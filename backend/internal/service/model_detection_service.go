package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/modeltrace"
)

func (s *SettingService) IsModelDetectionEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyModelDetectionEnabled)
	return err == nil && value == "true"
}
func (s *AccountTestService) ModelDetectionEnabled(ctx context.Context) bool {
	return s != nil && s.settingService.IsModelDetectionEnabled(ctx)
}

type ModelDetectionProgress struct {
	Type     string `json:"type"`
	Attempt  int    `json:"attempt"`
	Accepted int    `json:"accepted"`
	Message  string `json:"message,omitempty"`
}
type ModelDetectionResult struct {
	AccountID    int64              `json:"account_id,string"`
	Model        string             `json:"model"`
	MappedModel  string             `json:"mapped_model"`
	ActualModels []string           `json:"actual_models"`
	Attempts     int                `json:"attempts"`
	Result       *modeltrace.Result `json:"result"`
	Verdict      modeltrace.Verdict `json:"verdict"`
}

func validateModelDetectionAccount(account *Account, model string) error {
	if account == nil {
		return errors.New("账号不存在")
	}
	if account.IsSyntheticUITest() || account.IsCredentialShadow() {
		return errors.New("合成账号和影子账号暂不支持模型检测，请选择实际账号")
	}
	if account.Platform != PlatformOpenAI && account.Platform != PlatformAnthropic {
		return errors.New("目前支持 OpenAI 和 Anthropic 账号")
	}
	if account.Type != AccountTypeAPIKey && account.Type != AccountTypeOAuth {
		return errors.New("目前支持 API Key 和 OAuth 账号")
	}
	if !account.IsSchedulable() {
		return errors.New("账号当前不可调度，请检查账号状态、额度及限流")
	}
	if !account.IsModelSupported(model) {
		return errors.New("所选账号不支持该模型")
	}
	target := strings.ToLower(account.GetMappedModel(model))
	for _, kind := range []string{"image", "audio", "tts", "whisper", "embedding", "realtime", "video"} {
		if strings.Contains(target, kind) {
			return errors.New("请选择文本生成模型")
		}
	}
	return nil
}

// DetectModel is tied to the HTTP request: cancellation/closing the page stops
// collection. Every probe is pinned to this account, with no scheduler failover.
func (s *AccountTestService) DetectModel(ctx context.Context, accountID int64, model string, concurrency *ConcurrencyService, progress func(ModelDetectionProgress)) (*ModelDetectionResult, error) {
	if !s.ModelDetectionEnabled(ctx) {
		return nil, errors.New("模型检测功能未启用")
	}
	model = strings.TrimSpace(model)
	config := modeltrace.DefaultConfig()
	config.Model = model
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if accountID <= 0 {
		return nil, errors.New("无效账号")
	}
	if _, busy := s.modelDetectionActive.LoadOrStore(accountID, true); busy {
		return nil, errors.New("该账号已有检测任务正在运行")
	}
	defer s.modelDetectionActive.Delete(accountID)
	bank, err := s.CurrentModelDetectionBank(ctx)
	if err != nil {
		return nil, errors.New("指纹库不可用")
	}
	challenges, err := modeltrace.Challenges(6)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	result := &ModelDetectionResult{AccountID: accountID, Model: model, ActualModels: []string{}}
	outputs := []modeltrace.Output{}
	for _, challenge := range challenges {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !s.ModelDetectionEnabled(ctx) {
			return nil, errors.New("模型检测功能已关闭")
		}
		account, err := s.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return nil, errors.New("读取账号失败")
		}
		if err = validateModelDetectionAccount(account, model); err != nil {
			return nil, err
		}
		mapped := account.GetMappedModel(model)
		if result.MappedModel != "" && result.MappedModel != mapped {
			return nil, errors.New("检测期间模型映射发生变化，请重新测试")
		}
		result.MappedModel = mapped
		result.Attempts++
		if progress != nil {
			progress(ModelDetectionProgress{Type: "progress", Attempt: result.Attempts, Accepted: len(outputs)})
		}
		callCtx, stop := context.WithTimeout(ctx, 90*time.Second)
		text, actual, err := func() (string, string, error) {
			if concurrency != nil {
				slot, e := concurrency.AcquireAccountSlot(callCtx, account.ID, account.Concurrency)
				if e != nil || !slot.Acquired {
					return "", "", errors.New("账号并发已满或暂不可用")
				}
				defer slot.ReleaseFunc()
			}
			return s.probeModelIdentity(callCtx, account, model, challenge.Prompt)
		}()
		stop()
		if err != nil {
			return nil, fmt.Errorf("第 %d 次探测失败：%w", result.Attempts, err)
		}
		if len(modeltrace.ParseNumbers(text)) < modeltrace.Minimum(challenge.Expected) {
			if progress != nil {
				progress(ModelDetectionProgress{Type: "progress", Attempt: result.Attempts, Accepted: len(outputs), Message: "有效数字不足，将重新采样"})
			}
			continue
		}
		outputs = append(outputs, modeltrace.Output{Text: text, Expected: challenge.Expected})
		result.ActualModels = append(result.ActualModels, actual)
		if progress != nil {
			progress(ModelDetectionProgress{Type: "progress", Attempt: result.Attempts, Accepted: len(outputs)})
		}
		if len(outputs) == 3 {
			break
		}
	}
	result.Result, _ = bank.Analyze(outputs)
	result.Verdict = modeltrace.Classify(bank, config, result.Result)
	return result, nil
}
