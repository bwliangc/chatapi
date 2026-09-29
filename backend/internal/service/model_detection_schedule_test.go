package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/modeltrace"
	"github.com/stretchr/testify/require"
)

type detectionScheduleRepo struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
	fail   bool
}

func (r *detectionScheduleRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (r *detectionScheduleRepo) CompareAndSwapValue(_ context.Context, key, previous, next string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return false, errors.New("db unavailable")
	}
	if r.values[key] != previous {
		return false, nil
	}
	r.values[key] = next
	return true, nil
}
func scheduleTestService() (*AccountTestService, *detectionScheduleRepo) {
	repo := &detectionScheduleRepo{values: map[string]string{SettingKeyModelDetectionEnabled: "true"}}
	return &AccountTestService{settingService: NewSettingService(repo, nil), accountRepo: &detectionAccountRepo{account: &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}}}, repo
}
func TestModelDetectionSchedulePersistsAndClaimsOnce(t *testing.T) {
	ctx := context.Background()
	s, repo := scheduleTestService()
	config, err := s.SaveModelDetectionSchedule(ctx, ModelDetectionSchedule{Enabled: true, IntervalMinutes: 60, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "gpt-6-sol"}}})
	require.NoError(t, err)
	// Simulate another server process loading the same durable configuration.
	other := &AccountTestService{settingService: NewSettingService(repo, nil), accountRepo: s.accountRepo}
	loaded, err := other.GetModelDetectionSchedule(ctx)
	require.NoError(t, err)
	require.Equal(t, config, loaded)
	now := time.Now().UTC()
	var claimed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			target, err := other.claimDetectionTarget(ctx, now)
			if err != nil {
				t.Error(err)
			}
			if target != nil {
				claimed.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, claimed.Load())
	next, err := s.GetModelDetectionSchedule(ctx)
	require.NoError(t, err)
	require.Equal(t, now.Add(time.Hour), next.Targets[0].NextRunAt)
	// Editing an existing target cannot reset its due time and trigger duplicate charges.
	config.Targets[0].Model = "gpt-6-astra"
	updated, err := s.SaveModelDetectionSchedule(ctx, config)
	require.NoError(t, err)
	require.Equal(t, next.Targets[0].NextRunAt, updated.Targets[0].NextRunAt)
	target, err := s.claimDetectionTarget(ctx, now.Add(time.Hour))
	require.NoError(t, err)
	require.NotNil(t, target)
	require.Equal(t, "gpt-6-astra", target.Model)
}
func TestModelDetectionScheduleGatesAndFailure(t *testing.T) {
	ctx := context.Background()
	for _, enabled := range []bool{false, true} {
		s, repo := scheduleTestService()
		_, err := s.SaveModelDetectionSchedule(ctx, ModelDetectionSchedule{Enabled: enabled, IntervalMinutes: 15, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "gpt-6-sol"}}})
		require.NoError(t, err)
		if enabled {
			repo.values[SettingKeyModelDetectionEnabled] = "false"
		}
		target, err := s.claimDetectionTarget(ctx, time.Now())
		require.NoError(t, err)
		require.Nil(t, target)
	}
	s, repo := scheduleTestService()
	_, err := s.SaveModelDetectionSchedule(ctx, ModelDetectionSchedule{Enabled: true, IntervalMinutes: 15, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "gpt-6-sol"}}})
	require.NoError(t, err)
	repo.fail = true
	target, err := s.claimDetectionTarget(ctx, time.Now())
	require.Error(t, err)
	require.Nil(t, target)
}
func TestModelDetectionScheduleValidation(t *testing.T) {
	for _, config := range []ModelDetectionSchedule{
		{Enabled: true, IntervalMinutes: 14}, {Enabled: true, IntervalMinutes: 10081}, {Enabled: true, IntervalMinutes: 60},
		{IntervalMinutes: 60, Targets: []ModelDetectionTarget{{AccountID: -1, Model: "x"}}},
		{IntervalMinutes: 60, Targets: []ModelDetectionTarget{{AccountID: 42, Model: " "}}},
		{IntervalMinutes: 60, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "gpt-*"}}},
		{IntervalMinutes: 60, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "a"}, {AccountID: 42, Model: "b"}}},
		{Enabled: true, IntervalMinutes: 60, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "gpt-image-2"}}},
	} {
		s, _ := scheduleTestService()
		_, err := s.SaveModelDetectionSchedule(context.Background(), config)
		require.Error(t, err)
	}
	s, _ := scheduleTestService()
	s.accountRepo = nil
	_, err := s.SaveModelDetectionSchedule(context.Background(), ModelDetectionSchedule{IntervalMinutes: 60, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "removed-account-model"}}})
	require.NoError(t, err, "disabling must not depend on account availability")
}
func TestModelDetectionLeaseAcrossInstancesAndExpiry(t *testing.T) {
	s, repo := scheduleTestService()
	other := &AccountTestService{settingService: NewSettingService(repo, nil), accountRepo: s.accountRepo}
	ctx := context.Background()
	release, err := s.acquireDetectionLease(ctx, 42)
	require.NoError(t, err)
	_, err = other.acquireDetectionLease(ctx, 42)
	require.Error(t, err)
	release()
	release, err = other.acquireDetectionLease(ctx, 42)
	require.NoError(t, err)
	release()
	repo.values["model_detection_lease_42"] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	release, err = s.acquireDetectionLease(ctx, 42)
	require.NoError(t, err)
	release()
}

type detectionSnapshotRepo struct {
	AccountRepository
	snapshot ModelDetectionSnapshot
	source   string
	err      error
}

func (r *detectionSnapshotRepo) UpdateExtra(ctx context.Context, _ int64, updates map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.snapshot = updates[ModelDetectionSnapshotExtraKey].(ModelDetectionSnapshot)
	return r.err
}
func TestModelDetectionSnapshotVerdictsAndErrors(t *testing.T) {
	repo := &detectionSnapshotRepo{}
	s := &AccountTestService{accountRepo: repo}
	for _, status := range []string{"consistent", "suspected_mismatch", "inconclusive", "unsupported", "insufficient"} {
		require.NoError(t, s.saveDetectionSnapshot(42, "gpt-6-sol", &ModelDetectionResult{Verdict: modeltrace.Verdict{Status: status, Reason: "reason"}}, nil))
		require.Equal(t, status, repo.snapshot.Status)
		require.Equal(t, "gpt-6-sol", repo.snapshot.Model)
		require.False(t, repo.snapshot.CheckedAt.IsZero())
	}
	require.NoError(t, s.saveDetectionSnapshot(42, "gpt-6-sol", nil, errors.New("upstream secret")))
	require.Equal(t, "error", repo.snapshot.Status)
	require.NotContains(t, repo.snapshot.Reason, "secret")
	require.NoError(t, s.saveDetectionSnapshot(42, "gpt-6-sol", nil, context.Canceled))
	require.Equal(t, "cancelled", repo.snapshot.Status)
	repo.err = errors.New("write failed")
	require.Error(t, s.saveDetectionSnapshot(42, "gpt-6-sol", nil, context.DeadlineExceeded))
}
func TestModelDetectionScheduleChoosesOldestDueTarget(t *testing.T) {
	s, repo := scheduleTestService()
	now := time.Now().UTC()
	config := ModelDetectionSchedule{Enabled: true, IntervalMinutes: 15, Targets: []ModelDetectionTarget{{AccountID: 1, NextRunAt: now.Add(-time.Minute)}, {AccountID: 2, NextRunAt: now.Add(-time.Hour)}}}
	raw, err := json.Marshal(config)
	require.NoError(t, err)
	repo.values[modelDetectionScheduleKey] = string(raw)
	target, err := s.claimDetectionTarget(context.Background(), now)
	require.NoError(t, err)
	require.EqualValues(t, 2, target.AccountID)
}

func TestModelDetectionScheduleIntervalChange(t *testing.T) {
	s, _ := scheduleTestService()
	ctx := context.Background()
	config := ModelDetectionSchedule{Enabled: true, IntervalMinutes: 360, Targets: []ModelDetectionTarget{{AccountID: 42, Model: "gpt-6-sol"}}}
	_, err := s.SaveModelDetectionSchedule(ctx, config)
	require.NoError(t, err)
	now := time.Now().UTC()
	_, err = s.claimDetectionTarget(ctx, now)
	require.NoError(t, err)
	config.IntervalMinutes = 15
	updated, err := s.SaveModelDetectionSchedule(ctx, config)
	require.NoError(t, err)
	require.Equal(t, now.Add(15*time.Minute), updated.Targets[0].NextRunAt)
}

func (r *detectionSnapshotRepo) SaveModelDetectionResult(ctx context.Context, id int64, source string, snapshot ModelDetectionSnapshot) error {
	r.source = source
	return r.UpdateExtra(ctx, id, map[string]any{ModelDetectionSnapshotExtraKey: snapshot})
}
func (r *detectionSnapshotRepo) ListModelDetectionHistory(context.Context, int64, int, int) ([]ModelDetectionHistoryRecord, int64, error) {
	return nil, 0, nil
}
func TestModelDetectionHistorySource(t *testing.T) {
	repo := &detectionSnapshotRepo{}
	s := &AccountTestService{accountRepo: repo}
	require.NoError(t, s.saveDetectionSnapshot(42, "gpt-6-sol", nil, context.Canceled, "scheduled"))
	require.Equal(t, "scheduled", repo.source)
	require.NoError(t, s.saveDetectionSnapshot(42, "gpt-6-sol", nil, context.Canceled))
	require.Equal(t, "manual", repo.source)
}
