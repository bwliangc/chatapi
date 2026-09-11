package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type autoResetFaultQuota struct {
	autoResetQuotaStub
	query      func(context.Context, int) error
	afterReset func()
	result     *OpenAIQuotaResetResult
}

func (q *autoResetFaultQuota) QueryUsage(ctx context.Context, id int64) (*OpenAIQuotaUsage, error) {
	if q.query != nil {
		if err := q.query(ctx, q.queryCalls); err != nil {
			q.queryCalls++
			return nil, err
		}
	}
	return q.autoResetQuotaStub.QueryUsage(ctx, id)
}

func (q *autoResetFaultQuota) ResetCredit(ctx context.Context, id int64) (*OpenAIQuotaResetResult, error) {
	result, err := q.autoResetQuotaStub.ResetCredit(ctx, id)
	if q.afterReset != nil {
		q.afterReset()
	}
	if q.result != nil {
		result = q.result
	}
	return result, err
}

type autoResetFaultEmail struct {
	inputs []NotificationEmailSendInput
	err    error
}

func (e *autoResetFaultEmail) Send(ctx context.Context, input NotificationEmailSendInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.inputs = append(e.inputs, input)
	return e.err
}

func autoResetNotificationFixture() (*autoResetRepoStub, *autoResetFaultQuota, *autoResetFaultEmail) {
	account := Account{ID: 42, Name: "weekly-account", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Extra: AccountAutoResetExtraUpdates(AccountAutoResetSettings{
			Enabled: true, Email: "alerts@example.com",
			Conditions: []AccountAutoResetCondition{{Type: AccountAutoResetStrategyWeeklyThreshold, Value: 90}},
		})}
	usage := func(used float64, credits int) *OpenAIQuotaUsage {
		return &OpenAIQuotaUsage{
			RateLimit: &OpenAIRateLimit{SecondaryWindow: &OpenAIRateLimitWindow{
				UsedPercent: used, LimitWindowSeconds: 7 * 24 * 60 * 60,
			}},
			RateLimitResetCredits: &OpenAIRateLimitResetCredits{AvailableCount: credits},
		}
	}
	return &autoResetRepoStub{accounts: []Account{account}},
		&autoResetFaultQuota{autoResetQuotaStub: autoResetQuotaStub{usage: []*OpenAIQuotaUsage{usage(95, 2), usage(0, 1)}}},
		&autoResetFaultEmail{}
}

func TestOpenAIAutoResetRefreshFailureDoesNotLoseEmail(t *testing.T) {
	repo, quota, email := autoResetNotificationFixture()
	quota.query = func(ctx context.Context, call int) error {
		if call == 1 {
			// The successful reset and notification must already be durable before refresh.
			require.NotEmpty(t, repo.accounts[0].Extra[AccountExtraAutoResetLastAt])
			require.Equal(t, true, repo.accounts[0].Extra[AccountExtraAutoResetEmailPending])
			require.NoError(t, ctx.Err())
			return context.DeadlineExceeded
		}
		return nil
	}
	svc := NewOpenAIAutoResetService(repo, quota, nil, email)
	defer svc.Stop()
	require.NoError(t, svc.RunDue(context.Background()))
	require.Equal(t, 1, quota.resetCalls)
	require.Len(t, email.inputs, 1)
	require.Equal(t, "1", email.inputs[0].Variables["remaining_credits"])
	require.Equal(t, false, repo.accounts[0].Extra[AccountExtraAutoResetEmailPending])
}

func TestOpenAIAutoResetSuccessSurvivesExpiredOperationContext(t *testing.T) {
	repo, quota, email := autoResetNotificationFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	quota.afterReset = cancel
	quota.query = func(ctx context.Context, call int) error {
		if call > 0 {
			require.NoError(t, ctx.Err(), "refresh has its own budget")
		}
		return nil
	}
	svc := NewOpenAIAutoResetService(repo, quota, nil, email)
	defer svc.Stop()
	require.NoError(t, svc.processAccount(ctx, &repo.accounts[0], time.Now().UTC()))
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.NotEmpty(t, repo.accounts[0].Extra[AccountExtraAutoResetLastAt])
	require.Equal(t, true, repo.accounts[0].Extra[AccountExtraAutoResetEmailPending])
	require.NoError(t, svc.deliverPendingEmails(context.Background()))
	require.Len(t, email.inputs, 1)
	require.Equal(t, 1, quota.resetCalls)
}

func TestOpenAIAutoResetFailureRecordedAfterContextCancellation(t *testing.T) {
	repo, quota, email := autoResetNotificationFixture()
	svc := NewOpenAIAutoResetService(repo, quota, nil, email)
	defer svc.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.recordFailure(ctx, 42, time.Now().UTC(), context.DeadlineExceeded)
	require.Contains(t, AccountAutoResetSettingsFrom(&repo.accounts[0]).LastError, "deadline exceeded")
}

func TestOpenAIAutoResetEmailRetriesAfterRestartWithoutConsumingCredit(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled", true: "disabled"}[disabled], func(t *testing.T) {
			repo, quota, email := autoResetNotificationFixture()
			email.err = errors.New("SMTP temporarily unavailable")
			now := time.Now().UTC()
			svc := NewOpenAIAutoResetService(repo, quota, nil, email)
			svc.now = func() time.Time { return now }
			require.NoError(t, svc.RunDue(context.Background()))
			require.Len(t, email.inputs, 1)
			require.Contains(t, AccountAutoResetSettingsFrom(&repo.accounts[0]).LastError, "SMTP temporarily unavailable")
			key := email.inputs[0].ReminderKey
			svc.Stop()

			email.err = nil
			repo.accounts[0].Extra[AccountExtraAutoResetEnabled] = !disabled
			restarted := NewOpenAIAutoResetService(repo, quota, nil, email)
			defer restarted.Stop()
			restarted.now = func() time.Time { return now }
			now = now.Add(openAIAutoResetRetryDelay - time.Second)
			require.NoError(t, restarted.RunDue(context.Background()))
			require.Len(t, email.inputs, 1, "retry respects backoff even when a quota poll is due")
			require.Contains(t, AccountAutoResetSettingsFrom(&repo.accounts[0]).LastError, "SMTP temporarily unavailable")
			now = now.Add(time.Second)
			require.NoError(t, restarted.RunDue(context.Background()))
			require.Len(t, email.inputs, 2)
			require.Equal(t, key, email.inputs[1].ReminderKey)
			require.Equal(t, 1, quota.resetCalls, "retry must not consume another credit")
			require.Empty(t, AccountAutoResetSettingsFrom(&repo.accounts[0]).LastError)
			require.Equal(t, false, repo.accounts[0].Extra[AccountExtraAutoResetEmailPending])
			now = now.Add(openAIAutoResetRetryDelay)
			require.NoError(t, restarted.RunDue(context.Background()))
			require.Len(t, email.inputs, 2, "completed notification is removed")
		})
	}
}

func TestOpenAIAutoResetPendingEmailDoesNotBlockNextReset(t *testing.T) {
	repo, quota, email := autoResetNotificationFixture()
	email.err = errors.New("SMTP unavailable")
	now := time.Now().UTC()
	svc := NewOpenAIAutoResetService(repo, quota, nil, email)
	defer svc.Stop()
	svc.now = func() time.Time { return now }
	require.NoError(t, svc.RunDue(context.Background()))
	now = now.Add(openAIAutoResetWeeklyPoll)
	require.NoError(t, svc.RunDue(context.Background())) // rearm below the threshold
	quota.usage = append(quota.usage, quota.usage[1], quota.usage[0], quota.usage[1])
	now = now.Add(openAIAutoResetWeeklyPoll)
	require.NoError(t, svc.RunDue(context.Background()))
	require.Equal(t, 2, quota.resetCalls)
	pending, err := openAIAutoResetPendingEmails(&repo.accounts[0])
	require.NoError(t, err)
	require.Len(t, pending, 2, "both successful resets retain their notifications")
	require.NotEqual(t, pending[0].ReminderKey, pending[1].ReminderKey)
	email.err = nil
	now = now.Add(openAIAutoResetRetryDelay)
	require.NoError(t, svc.RunDue(context.Background()))
	pending, err = openAIAutoResetPendingEmails(&repo.accounts[0])
	require.NoError(t, err)
	require.Empty(t, pending)
	require.Equal(t, 2, quota.resetCalls)
}

func TestOpenAIAutoResetNoCreditDoesNotQueueSuccessEmail(t *testing.T) {
	repo, quota, email := autoResetNotificationFixture()
	quota.result = &OpenAIQuotaResetResult{Code: "no_credit"}
	svc := NewOpenAIAutoResetService(repo, quota, nil, email)
	defer svc.Stop()
	require.NoError(t, svc.RunDue(context.Background()))
	require.Empty(t, email.inputs)
	require.Empty(t, repo.accounts[0].Extra[AccountExtraAutoResetLastAt])
	require.Contains(t, AccountAutoResetSettingsFrom(&repo.accounts[0]).LastError, "no reset credit consumed")
}

type autoResetEmailAckFailureRepo struct {
	*autoResetRepoStub
	failAck bool
}

func (r *autoResetEmailAckFailureRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	if pending, ok := updates[AccountExtraAutoResetEmailPending].(bool); ok && !pending && r.failAck {
		return errors.New("notification acknowledgement write failed")
	}
	return r.autoResetRepoStub.UpdateExtra(ctx, id, updates)
}

func TestOpenAIAutoResetEmailDedupeAfterAcknowledgementFailure(t *testing.T) {
	baseRepo, quota, _ := autoResetNotificationFixture()
	repo := &autoResetEmailAckFailureRepo{autoResetRepoStub: baseRepo, failAck: true}
	smtp := startNotificationEmailTestSMTPServer(t)
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.SetMultiple(context.Background(), smtp.settings()))
	sender := NewNotificationEmailService(settings, NewEmailService(settings, nil))
	svc := NewOpenAIAutoResetService(repo, quota, nil, sender)
	require.NoError(t, svc.RunDue(context.Background()))
	require.EqualValues(t, 1, smtp.messageCount())
	require.Equal(t, true, repo.accounts[0].Extra[AccountExtraAutoResetEmailPending])
	svc.Stop()

	repo.failAck = false
	restarted := NewOpenAIAutoResetService(repo, quota, nil, sender)
	defer restarted.Stop()
	require.NoError(t, restarted.RunDue(context.Background()))
	require.EqualValues(t, 1, smtp.messageCount(), "persisted delivery key prevents a second SMTP delivery")
	require.Equal(t, false, repo.accounts[0].Extra[AccountExtraAutoResetEmailPending])
	require.Equal(t, 1, quota.resetCalls)
}
