package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const modelDetectionScheduleKey = "model_detection_schedule_v1"
const ModelDetectionSnapshotExtraKey = "model_detection_snapshot"

type ModelDetectionSnapshot struct {
	Status     string    `json:"status"`
	Model      string    `json:"model"`
	CheckedAt  time.Time `json:"checked_at"`
	Reason     string    `json:"reason"`
	Prediction string    `json:"prediction,omitempty"`
}
type ModelDetectionTarget struct {
	AccountName string    `json:"account_name,omitempty"`
	AccountID   int64     `json:"account_id"`
	Model       string    `json:"model"`
	NextRunAt   time.Time `json:"next_run_at"`
}
type ModelDetectionSchedule struct {
	Enabled         bool                   `json:"enabled"`
	IntervalMinutes int                    `json:"interval_minutes"`
	Targets         []ModelDetectionTarget `json:"targets"`
}

func (s *AccountTestService) readDetectionSchedule(ctx context.Context) (ModelDetectionSchedule, string, error) {
	config := ModelDetectionSchedule{IntervalMinutes: 360, Targets: []ModelDetectionTarget{}}
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return config, "", errors.New("模型检测服务不可用")
	}
	raw, err := s.settingService.settingRepo.GetValue(ctx, modelDetectionScheduleKey)
	if errors.Is(err, ErrSettingNotFound) {
		return config, "", nil
	}
	if err != nil {
		return config, "", err
	}
	if raw != "" {
		err = json.Unmarshal([]byte(raw), &config)
	}
	if config.Targets == nil {
		config.Targets = []ModelDetectionTarget{}
	}
	return config, raw, err
}
func (s *AccountTestService) GetModelDetectionSchedule(ctx context.Context) (ModelDetectionSchedule, error) {
	config, _, err := s.readDetectionSchedule(ctx)
	if err != nil || len(config.Targets) == 0 {
		return config, err
	}
	ids := make([]int64, 0, len(config.Targets))
	for _, target := range config.Targets {
		ids = append(ids, target.AccountID)
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return config, err
	}
	names := make(map[int64]string, len(accounts))
	for _, account := range accounts {
		names[account.ID] = account.Name
	}
	for i := range config.Targets {
		config.Targets[i].AccountName = names[config.Targets[i].AccountID]
	}
	return config, nil
}
func (s *AccountTestService) SaveModelDetectionSchedule(ctx context.Context, config ModelDetectionSchedule) (ModelDetectionSchedule, error) {
	if config.Targets == nil {
		config.Targets = []ModelDetectionTarget{}
	}
	if config.IntervalMinutes < 15 || config.IntervalMinutes > 10080 {
		return config, errors.New("检测间隔须为 15 至 10080 分钟")
	}
	if len(config.Targets) > 100 || (config.Enabled && len(config.Targets) == 0) {
		return config, errors.New("请选择 1 至 100 个账号")
	}
	seen := map[int64]bool{}
	for i := range config.Targets {
		target := &config.Targets[i]
		target.AccountName = ""
		target.Model = strings.TrimSpace(target.Model)
		if target.AccountID <= 0 || seen[target.AccountID] || target.Model == "" || len(target.Model) > 160 || strings.Contains(target.Model, "*") {
			return config, errors.New("账号或模型无效，账号不可重复")
		}
		seen[target.AccountID] = true
		// Disabling a schedule must still work if an account was removed or is unavailable.
		if !config.Enabled {
			continue
		}
		account, err := s.accountRepo.GetByID(ctx, target.AccountID)
		if err != nil {
			return config, fmt.Errorf("读取账号 #%d 失败", target.AccountID)
		}
		if err := validateModelDetectionAccountModel(account, target.Model); err != nil {
			return config, fmt.Errorf("账号 #%d: %w", target.AccountID, err)
		}
	}
	store, ok := s.settingService.settingRepo.(modelDetectionBankCAS)
	if !ok {
		return config, errors.New("定时检测存储不可用")
	}
	for attempt := 0; attempt < 5; attempt++ {
		previous, raw, err := s.readDetectionSchedule(ctx)
		if err != nil {
			return config, err
		}
		for i := range config.Targets {
			config.Targets[i].NextRunAt = time.Now().UTC()
			for _, old := range previous.Targets {
				if old.AccountID == config.Targets[i].AccountID {
					config.Targets[i].NextRunAt = old.NextRunAt
					if previous.IntervalMinutes != config.IntervalMinutes && old.NextRunAt.After(time.Now()) {
						// Apply interval changes relative to the previous scheduled start.
						config.Targets[i].NextRunAt = old.NextRunAt.Add(time.Duration(config.IntervalMinutes-previous.IntervalMinutes) * time.Minute)
					}
					break
				}
			}
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			return config, err
		}
		updated, err := store.CompareAndSwapValue(ctx, modelDetectionScheduleKey, raw, string(encoded))
		if err != nil {
			return config, err
		}
		if updated {
			return config, nil
		}
	}
	return config, errors.New("配置正在更新，请重试")
}

// claimDetectionTarget advances the persisted due time atomically before any
// upstream request. Multiple server instances cannot claim the same occurrence.
func (s *AccountTestService) claimDetectionTarget(ctx context.Context, now time.Time) (*ModelDetectionTarget, error) {
	if !s.ModelDetectionEnabled(ctx) {
		return nil, nil
	}
	config, raw, err := s.readDetectionSchedule(ctx)
	if err != nil || !config.Enabled {
		return nil, err
	}
	if config.IntervalMinutes < 15 || config.IntervalMinutes > 10080 {
		return nil, errors.New("无效检测间隔")
	}
	due := -1
	for i := range config.Targets {
		if !config.Targets[i].NextRunAt.After(now) && (due == -1 || config.Targets[i].NextRunAt.Before(config.Targets[due].NextRunAt)) {
			due = i
		}
	}
	if due >= 0 {
		i := due
		target := config.Targets[i]
		config.Targets[i].NextRunAt = now.Add(time.Duration(config.IntervalMinutes) * time.Minute)
		next, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		store, ok := s.settingService.settingRepo.(modelDetectionBankCAS)
		if !ok {
			return nil, errors.New("定时检测存储不可用")
		}
		claimed, err := store.CompareAndSwapValue(ctx, modelDetectionScheduleKey, raw, string(next))
		if err != nil || !claimed {
			return nil, err
		}
		return &target, nil
	}
	return nil, nil
}

// The durable lease also prevents manual and scheduled runs from overlapping
// across instances. It outlives the hard ten-minute detection timeout.
func (s *AccountTestService) acquireDetectionLease(ctx context.Context, id int64) (func(), error) {
	store, ok := s.settingService.settingRepo.(modelDetectionBankCAS)
	if !ok {
		return nil, errors.New("检测锁存储不可用")
	}
	key := fmt.Sprintf("model_detection_lease_%d", id)
	raw, err := s.settingService.settingRepo.GetValue(ctx, key)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return nil, err
	}
	if raw != "" {
		until, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil || until.After(time.Now()) {
			return nil, errors.New("该账号已有检测任务正在运行")
		}
	}
	next := time.Now().Add(11 * time.Minute).UTC().Format(time.RFC3339Nano)
	ok, err = store.CompareAndSwapValue(ctx, key, raw, next)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("该账号已有检测任务正在运行")
	}
	return func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := store.CompareAndSwapValue(cleanup, key, next, ""); err != nil {
			log.Printf("[ModelDetection] release lease account=%d: %v", id, err)
		}
	}, nil
}

func (s *AccountTestService) saveDetectionSnapshot(id int64, model string, result *ModelDetectionResult, detectionErr error, sources ...string) error {
	snapshot := ModelDetectionSnapshot{Model: model, CheckedAt: time.Now().UTC(), Status: "error", Reason: "检测失败，请重新检测"}
	if result != nil {
		snapshot.Status = result.Verdict.Status
		snapshot.Reason = result.Verdict.Reason
		if result.Result != nil {
			snapshot.Prediction = result.Result.Prediction
		}
	} else if errors.Is(detectionErr, context.Canceled) {
		snapshot.Status = "cancelled"
		snapshot.Reason = "检测已取消"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	source := "manual"
	if len(sources) > 0 && sources[0] == "scheduled" {
		source = "scheduled"
	}
	repo, ok := s.accountRepo.(ModelDetectionHistoryRepository)
	if !ok {
		return errors.New("检测历史存储不可用")
	}
	return repo.SaveModelDetectionResult(ctx, id, source, snapshot)
}

type ModelDetectionScheduler struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func ProvideModelDetectionScheduler(s *AccountTestService, concurrency *ConcurrencyService) *ModelDetectionScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &ModelDetectionScheduler{cancel: cancel}
	runner.wg.Add(1)
	go func() {
		defer runner.wg.Done()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			// Bound each pass; a fresh configuration is read before every account.
			for i := 0; i < 100 && ctx.Err() == nil; i++ {
				target, err := s.claimDetectionTarget(ctx, time.Now().UTC())
				if err != nil {
					log.Printf("[ModelDetection] schedule: %v", err)
					break
				}
				if target == nil {
					break
				}
				if _, err := s.DetectModel(context.WithValue(ctx, modelDetectionSourceKey{}, "scheduled"), target.AccountID, target.Model, concurrency, nil); err != nil {
					log.Printf("[ModelDetection] scheduled account=%d: %v", target.AccountID, err)
				}
			}
		}
	}()
	return runner
}
func (r *ModelDetectionScheduler) Stop() {
	if r == nil {
		return
	}
	r.cancel()
	r.wg.Wait()
}
