package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	InFlight int    `json:"in_flight"`
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bank, err := s.CurrentModelDetectionBank(ctx)
	if err != nil {
		return nil, errors.New("指纹库不可用")
	}
	challenges, err := modeltrace.Challenges(6)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		workers.Wait() // Keep the account task lock until all requests/slots are released.
	}()
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, errors.New("读取账号失败")
	}
	if err := validateModelDetectionAccount(account, model); err != nil {
		return nil, err
	}
	parallelism := 3
	if account.Concurrency > 0 && account.Concurrency < parallelism {
		parallelism = account.Concurrency
	}
	result := &ModelDetectionResult{AccountID: accountID, Model: model, MappedModel: account.GetMappedModel(model), ActualModels: []string{}}
	type sample struct {
		index        int
		text, actual string
		err          error
	}
	completed := make(chan sample, parallelism)
	accepted := make(map[int]sample)
	inFlight := 0
	emit := func(message string) {
		if progress != nil {
			progress(ModelDetectionProgress{Type: "progress", Attempt: result.Attempts, Accepted: len(accepted), InFlight: inFlight, Message: message})
		}
	}
	for len(accepted) < 3 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !s.ModelDetectionEnabled(ctx) {
			return nil, errors.New("模型检测功能已关闭")
		}
		// Never launch more requests than the missing samples. A short response
		// opens one replacement slot; valid responses cannot cause oversampling.
		for inFlight < parallelism && len(accepted)+inFlight < 3 && result.Attempts < len(challenges) {
			index := result.Attempts
			result.Attempts++
			inFlight++
			emit("")
			workers.Add(1)
			go func() {
				defer workers.Done()
				text, actual, err := s.collectModelDetectionSample(ctx, accountID, model, result.MappedModel, challenges[index], concurrency)
				select {
				case completed <- sample{index: index, text: text, actual: actual, err: err}:
				case <-ctx.Done():
				}
			}()
		}
		if inFlight == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case item := <-completed:
			inFlight--
			if item.err != nil {
				return nil, fmt.Errorf("第 %d 次探测失败：%w", item.index+1, item.err)
			}
			if len(modeltrace.ParseNumbers(item.text)) < modeltrace.Minimum(challenges[item.index].Expected) {
				emit("有效数字不足，将重新采样")
				continue
			}
			accepted[item.index] = item
			emit("")
		}
	}
	// Preserve challenge order regardless of network completion order.
	outputs := make([]modeltrace.Output, 0, len(accepted))
	for index, challenge := range challenges {
		if item, ok := accepted[index]; ok {
			outputs = append(outputs, modeltrace.Output{Text: item.text, Expected: challenge.Expected})
			result.ActualModels = append(result.ActualModels, item.actual)
		}
	}
	result.Result, _ = bank.Analyze(outputs)
	result.Verdict = modeltrace.Classify(bank, config, result.Result)
	return result, nil
}

func (s *AccountTestService) collectModelDetectionSample(ctx context.Context, accountID int64, model, mapped string, challenge modeltrace.Challenge, concurrency *ConcurrencyService) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if !s.ModelDetectionEnabled(ctx) {
		return "", "", errors.New("模型检测功能已关闭")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return "", "", errors.New("读取账号失败")
	}
	if err := validateModelDetectionAccount(account, model); err != nil {
		return "", "", err
	}
	if account.GetMappedModel(model) != mapped {
		return "", "", errors.New("检测期间模型映射发生变化，请重新测试")
	}
	if concurrency != nil {
		slot, err := concurrency.AcquireAccountSlot(ctx, account.ID, account.Concurrency)
		if err != nil || !slot.Acquired {
			return "", "", errors.New("账号并发已满或暂不可用")
		}
		defer slot.ReleaseFunc()
	}
	return s.probeModelIdentity(ctx, account, model, challenge.Prompt)
}
