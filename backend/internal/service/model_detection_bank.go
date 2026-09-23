package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/modeltrace"
)

const modelDetectionBankStateKey = "model_detection_bank_state_v1"

var modelDetectionCommitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var ErrModelDetectionBankConflict = errors.New("指纹库已被其他操作更新，请刷新后重试")
var ErrModelDetectionAlgorithmChanged = errors.New("上游检测算法已变化，请先升级系统，再更新指纹库")

type modelDetectionBankCAS interface {
	CompareAndSwapValue(context.Context, string, string, string) (bool, error)
}
type ModelDetectionBankVersion struct {
	Commit          string `json:"commit"`
	SHA256          string `json:"sha256"`
	AlgorithmSHA256 string `json:"algorithm_sha256"`
	BuiltAt         string `json:"built_at"`
	ModelCount      int    `json:"model_count"`
	InstalledAt     string `json:"installed_at,omitempty"`
}
type modelDetectionBankSnapshot struct {
	ModelDetectionBankVersion
	Data []byte `json:"data"` // base64 preserves the original bytes and SHA across JSON storage.
}
type modelDetectionBankState struct {
	Current  modelDetectionBankSnapshot  `json:"current"`
	Previous *modelDetectionBankSnapshot `json:"previous,omitempty"`
}
type ModelDetectionBankStatus struct {
	Current  ModelDetectionBankVersion  `json:"current"`
	Previous *ModelDetectionBankVersion `json:"previous,omitempty"`
	Revision string                     `json:"revision"`
	Source   string                     `json:"source"`
	Warning  string                     `json:"warning,omitempty"`
}
type ModelDetectionBankCheck struct {
	Status          ModelDetectionBankStatus   `json:"status"`
	Remote          *ModelDetectionBankVersion `json:"remote,omitempty"`
	Compatible      bool                       `json:"compatible"`
	UpdateAvailable bool                       `json:"update_available"`
	Message         string                     `json:"message,omitempty"`
}
type modelDetectionBankManager struct {
	repo    SettingRepository
	client  *http.Client
	cacheMu sync.Mutex
	cache   map[string]*modeltrace.Bank
}

func newModelDetectionBankManager(repo SettingRepository) *modelDetectionBankManager {
	return &modelDetectionBankManager{repo: repo, cache: map[string]*modeltrace.Bank{}, client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (s *SettingService) modelDetectionBankManager() *modelDetectionBankManager {
	s.modelDetectionBankOnce.Do(func() { s.modelDetectionBankState = newModelDetectionBankManager(s.settingRepo) })
	return s.modelDetectionBankState
}
func (s *AccountTestService) CurrentModelDetectionBank(ctx context.Context) (*modeltrace.Bank, error) {
	if s == nil || s.settingService == nil {
		return nil, errors.New("指纹库服务不可用")
	}
	_, snapshot, _, err := s.settingService.modelDetectionBankManager().load(ctx)
	if err != nil {
		return nil, err
	}
	return s.settingService.modelDetectionBankManager().parse(snapshot)
}
func (s *AccountTestService) ModelDetectionBankStatus(ctx context.Context) (ModelDetectionBankStatus, error) {
	if s == nil || s.settingService == nil {
		return ModelDetectionBankStatus{}, errors.New("指纹库服务不可用")
	}
	status, _, _, err := s.settingService.modelDetectionBankManager().load(ctx)
	return status, err
}
func (s *AccountTestService) CheckModelDetectionBank(ctx context.Context) (ModelDetectionBankCheck, error) {
	if s == nil || s.settingService == nil {
		return ModelDetectionBankCheck{}, errors.New("指纹库服务不可用")
	}
	return s.settingService.modelDetectionBankManager().check(ctx)
}
func (s *AccountTestService) UpdateModelDetectionBank(ctx context.Context, commit, revision string) (ModelDetectionBankStatus, error) {
	if s == nil || s.settingService == nil {
		return ModelDetectionBankStatus{}, errors.New("指纹库服务不可用")
	}
	return s.settingService.modelDetectionBankManager().update(ctx, commit, revision)
}
func (s *AccountTestService) RollbackModelDetectionBank(ctx context.Context, revision string) (ModelDetectionBankStatus, error) {
	if s == nil || s.settingService == nil {
		return ModelDetectionBankStatus{}, errors.New("指纹库服务不可用")
	}
	return s.settingService.modelDetectionBankManager().rollback(ctx, revision)
}
func modelDetectionDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func embeddedModelDetectionSnapshot() modelDetectionBankSnapshot {
	source := modeltrace.EmbeddedSource()
	return modelDetectionBankSnapshot{ModelDetectionBankVersion: ModelDetectionBankVersion{Commit: source.Commit, AlgorithmSHA256: source.AlgorithmSHA256, SHA256: source.BankSHA256}, Data: modeltrace.EmbeddedData()}
}
func (m *modelDetectionBankManager) parse(snapshot modelDetectionBankSnapshot) (*modeltrace.Bank, error) {
	if snapshot.AlgorithmSHA256 != modeltrace.EmbeddedSource().AlgorithmSHA256 {
		return nil, ErrModelDetectionAlgorithmChanged
	}
	if len(snapshot.Data) > modeltrace.MaxBankBytes || modelDetectionDigest(snapshot.Data) != snapshot.SHA256 {
		return nil, errors.New("指纹库校验失败")
	}
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if b := m.cache[snapshot.SHA256]; b != nil {
		return b, nil
	}
	bank, err := modeltrace.ParseBank(snapshot.Data)
	if err != nil {
		return nil, errors.New("指纹库结构不兼容或内容无效")
	}
	if len(m.cache) >= 4 {
		clear(m.cache)
	}
	m.cache[snapshot.SHA256] = bank
	return bank, nil
}
func (m *modelDetectionBankManager) read(ctx context.Context) (modelDetectionBankState, string, error) {
	var state modelDetectionBankState
	if m.repo == nil {
		return state, "", errors.New("指纹库存储不可用")
	}
	raw, err := m.repo.GetValue(ctx, modelDetectionBankStateKey)
	if errors.Is(err, ErrSettingNotFound) {
		return state, "", nil
	}
	if err != nil {
		return state, "", errors.New("读取指纹库失败")
	}
	if raw != "" && (len(raw) > 12*1024*1024 || json.Unmarshal([]byte(raw), &state) != nil) {
		// Keep the original value for revision/CAS recovery, but never use a
		// partially decoded snapshot. load falls back to the embedded baseline.
		return modelDetectionBankState{}, raw, nil
	}
	return state, raw, nil
}
func (m *modelDetectionBankManager) load(ctx context.Context) (ModelDetectionBankStatus, modelDetectionBankSnapshot, string, error) {
	state, raw, err := m.read(ctx)
	status := ModelDetectionBankStatus{Revision: modelDetectionDigest([]byte(raw)), Source: "embedded"}
	if err != nil {
		return status, modelDetectionBankSnapshot{}, raw, err
	}
	current := state.Current
	if raw == "" {
		current = embeddedModelDetectionSnapshot()
	} else {
		status.Source = "database"
	}
	bank, err := m.parse(current)
	if err != nil {
		// A newer application can have a different scoring algorithm. Never run an
		// incompatible persisted bank with it; the shipped bank is the safe baseline.
		current = embeddedModelDetectionSnapshot()
		bank, err = m.parse(current)
		status.Source = "embedded"
		status.Warning = "保存的指纹库与当前程序不兼容或校验失败，正在使用内置版本"
	}
	if err != nil {
		return status, current, raw, err
	}
	current.BuiltAt = bank.BuiltAt
	current.ModelCount = len(bank.Models)
	status.Current = current.ModelDetectionBankVersion
	if state.Previous != nil && state.Previous.SHA256 != current.SHA256 {
		if previous, err := m.parse(*state.Previous); err == nil {
			meta := state.Previous.ModelDetectionBankVersion
			meta.BuiltAt = previous.BuiltAt
			meta.ModelCount = len(previous.Models)
			status.Previous = &meta
		}
	}
	return status, current, raw, nil
}
func (m *modelDetectionBankManager) get(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("创建更新请求失败")
	}
	req.Header.Set("User-Agent", "ChatAPI-ModelTrace-Updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, errors.New("无法连接 ModelTrace 更新源，请检查网络后重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ModelTrace 更新源返回 HTTP %d（403/429 可能为 GitHub 限流）", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil || int64(len(raw)) > maxBytes {
		return nil, errors.New("更新文件读取失败或超过大小限制")
	}
	return raw, nil
}
func (m *modelDetectionBankManager) fetch(ctx context.Context, commit string) (modelDetectionBankSnapshot, error) {
	snapshot := modelDetectionBankSnapshot{}
	if !modelDetectionCommitPattern.MatchString(commit) {
		return snapshot, errors.New("无效的上游版本")
	}
	base := "https://raw.githubusercontent.com/xqy2006/ModelTrace/" + commit + "/"
	code, err := m.get(ctx, base+"static/fingerprint-core.js", 256*1024)
	if err != nil {
		return snapshot, err
	}
	snapshot.Commit = commit
	snapshot.AlgorithmSHA256 = modelDetectionDigest(code)
	if snapshot.AlgorithmSHA256 != modeltrace.EmbeddedSource().AlgorithmSHA256 {
		return snapshot, ErrModelDetectionAlgorithmChanged
	}
	data, err := m.get(ctx, base+"data/unified_bank.json", modeltrace.MaxBankBytes)
	if err != nil {
		return snapshot, err
	}
	snapshot.Data = data
	snapshot.SHA256 = modelDetectionDigest(data)
	bank, err := m.parse(snapshot)
	if err != nil {
		return snapshot, err
	}
	snapshot.BuiltAt = bank.BuiltAt
	snapshot.ModelCount = len(bank.Models)
	return snapshot, nil
}
func (m *modelDetectionBankManager) check(ctx context.Context) (ModelDetectionBankCheck, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	result := ModelDetectionBankCheck{}
	var err error
	result.Status, _, _, err = m.load(ctx)
	if err != nil {
		return result, err
	}
	raw, err := m.get(ctx, "https://api.github.com/repos/xqy2006/ModelTrace/commits/main", 512*1024)
	if err != nil {
		return result, err
	}
	var head struct {
		SHA string `json:"sha"`
	}
	if json.Unmarshal(raw, &head) != nil || !modelDetectionCommitPattern.MatchString(head.SHA) {
		return result, errors.New("更新源返回无效版本信息")
	}
	snapshot, err := m.fetch(ctx, head.SHA)
	result.Remote = &snapshot.ModelDetectionBankVersion
	if errors.Is(err, ErrModelDetectionAlgorithmChanged) {
		result.Message = err.Error()
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Compatible = true
	result.UpdateAvailable = snapshot.SHA256 != result.Status.Current.SHA256
	return result, nil
}
func (m *modelDetectionBankManager) save(ctx context.Context, raw string, state modelDetectionBankState) (ModelDetectionBankStatus, error) {
	store, ok := m.repo.(modelDetectionBankCAS)
	if !ok {
		return ModelDetectionBankStatus{}, errors.New("指纹库存储不支持原子更新")
	}
	next, err := json.Marshal(state)
	if err != nil {
		return ModelDetectionBankStatus{}, errors.New("保存指纹库失败")
	}
	ok, err = store.CompareAndSwapValue(ctx, modelDetectionBankStateKey, raw, string(next))
	if err != nil {
		return ModelDetectionBankStatus{}, errors.New("保存指纹库失败，请刷新确认当前版本")
	}
	if !ok {
		return ModelDetectionBankStatus{}, ErrModelDetectionBankConflict
	}
	status, _, _, err := m.load(ctx)
	return status, err
}
func (m *modelDetectionBankManager) update(ctx context.Context, commit, revision string) (ModelDetectionBankStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	status, current, raw, err := m.load(ctx)
	if err != nil {
		return status, err
	}
	if revision != status.Revision {
		return status, ErrModelDetectionBankConflict
	}
	next, err := m.fetch(ctx, commit)
	if err != nil {
		return status, err
	}
	if next.SHA256 == current.SHA256 {
		return status, nil
	}
	next.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	return m.save(ctx, raw, modelDetectionBankState{Current: next, Previous: &current})
}
func (m *modelDetectionBankManager) rollback(ctx context.Context, revision string) (ModelDetectionBankStatus, error) {
	status, _, raw, err := m.load(ctx)
	if err != nil {
		return status, err
	}
	if revision != status.Revision {
		return status, ErrModelDetectionBankConflict
	}
	state, rawAgain, err := m.read(ctx)
	if err != nil {
		return status, err
	}
	if rawAgain != raw {
		return status, ErrModelDetectionBankConflict
	}
	if status.Previous == nil || state.Previous == nil {
		return status, errors.New("没有可回滚的兼容版本")
	}
	// Consume the one-step rollback slot; do not toggle indefinitely between banks.
	return m.save(ctx, raw, modelDetectionBankState{Current: *state.Previous})
}
