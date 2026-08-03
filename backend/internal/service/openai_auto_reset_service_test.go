package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type autoResetRepoStub struct {
	accounts []Account
	updates  []map[string]any
}

func (s *autoResetRepoStub) FindByExtraField(context.Context, string, any) ([]Account, error) {
	return s.accounts, nil
}

func (s *autoResetRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	copyUpdates := make(map[string]any, len(updates))
	for key, value := range updates {
		copyUpdates[key] = value
	}
	s.updates = append(s.updates, copyUpdates)
	return nil
}

type autoResetQuotaStub struct {
	usage      []*OpenAIQuotaUsage
	queryCalls int
	resetCalls int
}

func (s *autoResetQuotaStub) QueryUsage(context.Context, int64) (*OpenAIQuotaUsage, error) {
	index := s.queryCalls
	s.queryCalls++
	if index >= len(s.usage) {
		index = len(s.usage) - 1
	}
	return s.usage[index], nil
}

func (s *autoResetQuotaStub) CacheResetCreditsSnapshot(context.Context, int64, *OpenAIRateLimitResetCredits) error {
	return nil
}

func (s *autoResetQuotaStub) ResetCredit(context.Context, int64) (*OpenAIQuotaResetResult, error) {
	s.resetCalls++
	return &OpenAIQuotaResetResult{
		Code:         "success",
		WindowsReset: 2,
		Credit:       &OpenAIQuotaResetCredit{ID: "credit-1", RedeemedAt: "2026-08-03T00:00:00Z"},
	}, nil
}

type autoResetRecovererStub struct{ calls int }

func (s *autoResetRecovererStub) RecoverAccountState(context.Context, int64, AccountRecoveryOptions) (*SuccessfulTestRecoveryResult, error) {
	s.calls++
	return &SuccessfulTestRecoveryResult{}, nil
}

type autoResetEmailStub struct{ inputs []NotificationEmailSendInput }

func (s *autoResetEmailStub) Send(_ context.Context, input NotificationEmailSendInput) error {
	s.inputs = append(s.inputs, input)
	return nil
}

func TestOpenAIAutoResetWeeklyThresholdRunsOnceAndSendsEmail(t *testing.T) {
	now := time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)
	account := Account{
		ID:       42,
		Name:     "codex-account",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			AccountExtraAutoResetEnabled:         true,
			AccountExtraAutoResetStrategy:        AccountAutoResetStrategyWeeklyThreshold,
			AccountExtraAutoResetWeeklyThreshold: 90.0,
			AccountExtraAutoResetExpiryHours:     24.0,
			AccountExtraAutoResetEmail:           "alerts@example.com",
			AccountExtraAutoResetWeeklyArmed:     true,
		},
	}
	weekly := func(used float64, credits int) *OpenAIQuotaUsage {
		return &OpenAIQuotaUsage{
			RateLimit: &OpenAIRateLimit{
				PrimaryWindow:   &OpenAIRateLimitWindow{UsedPercent: 30, LimitWindowSeconds: 5 * 60 * 60},
				SecondaryWindow: &OpenAIRateLimitWindow{UsedPercent: used, LimitWindowSeconds: 7 * 24 * 60 * 60},
			},
			RateLimitResetCredits: &OpenAIRateLimitResetCredits{AvailableCount: credits},
		}
	}
	repo := &autoResetRepoStub{accounts: []Account{account}}
	quota := &autoResetQuotaStub{usage: []*OpenAIQuotaUsage{weekly(92, 2), weekly(4, 1)}}
	recoverer := &autoResetRecovererStub{}
	email := &autoResetEmailStub{}
	svc := NewOpenAIAutoResetService(repo, quota, recoverer, email)
	svc.now = func() time.Time { return now }

	require.NoError(t, svc.RunDue(context.Background()))
	require.Equal(t, 1, quota.resetCalls)
	require.Equal(t, 1, recoverer.calls)
	require.Len(t, email.inputs, 1)
	require.Equal(t, NotificationEmailEventAccountAutoReset, email.inputs[0].Event)
	require.Equal(t, "1", email.inputs[0].Variables["remaining_credits"])
	require.NotEmpty(t, repo.updates)
	last := repo.updates[len(repo.updates)-1]
	require.Equal(t, false, last[AccountExtraAutoResetWeeklyArmed])
	require.Equal(t, AccountAutoResetStrategyWeeklyThreshold, last[AccountExtraAutoResetLastStrategy])
}

func TestOpenAIAutoResetCreditExpiryTriggerBoundary(t *testing.T) {
	now := time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)
	settings := AccountAutoResetSettings{Strategy: AccountAutoResetStrategyCreditExpiry, ExpiryHours: 24}
	usage := &OpenAIQuotaUsage{RateLimitResetCredits: &OpenAIRateLimitResetCredits{
		AvailableCount: 1,
		Credits:        []OpenAIRateLimitResetCreditDetail{{ExpiresAt: now.Add(24 * time.Hour).Format(time.RFC3339)}},
	}}

	triggered, expiresAt, _ := openAIAutoResetShouldTrigger(&Account{}, settings, usage, now)
	require.True(t, triggered)
	require.Equal(t, now.Add(24*time.Hour).Format(time.RFC3339), expiresAt)

	usage.RateLimitResetCredits.Credits[0].ExpiresAt = now.Add(24*time.Hour + time.Second).Format(time.RFC3339)
	triggered, _, _ = openAIAutoResetShouldTrigger(&Account{}, settings, usage, now)
	require.False(t, triggered)
}

func TestAccountAutoResetSettingsValidation(t *testing.T) {
	valid := AccountAutoResetSettings{
		Enabled:         true,
		Strategy:        AccountAutoResetStrategyWeeklyThreshold,
		WeeklyThreshold: 90,
		ExpiryHours:     24,
		Email:           "alerts@example.com",
	}
	require.NoError(t, ValidateAccountAutoResetSettings(valid))

	invalid := valid
	invalid.WeeklyThreshold = 101
	require.Error(t, ValidateAccountAutoResetSettings(invalid))
	invalid = valid
	invalid.Email = "invalid"
	require.Error(t, ValidateAccountAutoResetSettings(invalid))
}
