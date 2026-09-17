//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type accountStateNotificationCase struct {
	name   string
	status string
	enter  func(*accountRepository, context.Context, int64, time.Time) error
	clear  func(*accountRepository, context.Context, int64) error
}

func accountStateNotificationCases() []accountStateNotificationCase {
	return []accountStateNotificationCase{
		{"weekly rate limit", service.AccountNotifyStatusRateLimited, (*accountRepository).SetRateLimited, (*accountRepository).ClearRateLimit},
		{"extended rate limit", service.AccountNotifyStatusRateLimited, (*accountRepository).SetRateLimitedIfLater, (*accountRepository).ClearRateLimit},
		{"overload", service.AccountNotifyStatusOverloaded, (*accountRepository).SetOverloaded, (*accountRepository).ClearRateLimit},
		{"temporary block", service.AccountNotifyStatusTempUnschedulable, func(r *accountRepository, ctx context.Context, id int64, until time.Time) error {
			return r.SetTempUnschedulable(ctx, id, until, "consecutive failures")
		}, (*accountRepository).ClearTempUnschedulable},
		{"model rate limit", service.AccountNotifyStatusRateLimited, func(r *accountRepository, ctx context.Context, id int64, until time.Time) error {
			return r.SetModelRateLimit(ctx, id, "spark", until, "weekly quota exhausted")
		}, (*accountRepository).ClearModelRateLimits},
	}
}

func TestAccountStateNotifications(t *testing.T) {
	tests := accountStateNotificationCases()
	tests = append(tests, accountStateNotificationCase{
		"conditional rate limit", service.AccountNotifyStatusRateLimited,
		func(r *accountRepository, ctx context.Context, id int64, until time.Time) error {
			account, err := r.GetByID(ctx, id)
			if err != nil {
				return err
			}
			updated, err := r.SetRateLimitedIfUnchanged(ctx, id, account.UpdatedAt, account.RateLimitedAt, account.RateLimitResetAt, until)
			if err == nil && !updated {
				return fmt.Errorf("expected matching rate-limit generation")
			}
			return err
		}, (*accountRepository).ClearRateLimit,
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tx := testEntTx(t)
			repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
			recorder := &abnormalNotificationSenderRecorder{}
			repo.abnormalNotificationSender = recorder
			account := mustCreateAccount(t, tx.Client(), &service.Account{
				Name: tt.name, Platform: service.PlatformOpenAI, Status: service.StatusActive, Schedulable: true,
				Extra: map[string]any{
					service.AccountExtraAbnormalNotifyEnabled:  true,
					service.AccountExtraAbnormalNotifyEmail:    "alerts@example.com",
					service.AccountExtraAbnormalNotifyStatuses: []string{tt.status},
				},
			})
			until := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(time.Second)
			require.NoError(t, tt.enter(repo, ctx, account.ID, until))
			require.Len(t, recorder.inputs, 1)
			require.Equal(t, tt.status, recorder.inputs[0].Variables["account_status"])
			require.Equal(t, until.Format(time.RFC3339), recorder.inputs[0].Variables["reset_time"])
			require.Equal(t, "alerts@example.com", recorder.inputs[0].RecipientEmail)
			require.Equal(t, service.NotificationEmailEventAccountAbnormalNotice, recorder.inputs[0].Event)

			require.NoError(t, tt.enter(repo, ctx, account.ID, until))
			require.NoError(t, tt.enter(repo, ctx, account.ID, until.Add(time.Hour)))
			require.Len(t, recorder.inputs, 1, "repeated or extended cooldowns must not send another notice")
			require.NoError(t, tt.clear(repo, ctx, account.ID))
			require.NoError(t, tt.enter(repo, ctx, account.ID, until))
			require.Len(t, recorder.inputs, 2, "recovery must re-arm the notification")

			require.NoError(t, tt.clear(repo, ctx, account.ID))
			require.NoError(t, tt.enter(repo, ctx, account.ID, time.Now().Add(-time.Hour)))
			require.Len(t, recorder.inputs, 2, "expired cooldowns must not send notifications")
			require.NoError(t, tt.enter(repo, ctx, account.ID, until))
			require.Len(t, recorder.inputs, 3, "a naturally expired cooldown must also re-arm the notification")

			require.NoError(t, tt.clear(repo, ctx, account.ID))
			require.NoError(t, repo.UpdateExtra(ctx, account.ID, map[string]any{
				service.AccountExtraAbnormalNotifyStatuses: []string{service.AccountNotifyStatusError},
			}))
			require.NoError(t, tt.enter(repo, ctx, account.ID, until))
			require.Len(t, recorder.inputs, 3, "unselected states must not notify")
		})
	}
}

type concurrentAccountNotificationRecorder struct {
	inputs chan service.NotificationEmailSendInput
}

func (r *concurrentAccountNotificationRecorder) Send(_ context.Context, input service.NotificationEmailSendInput) error {
	r.inputs <- input
	return nil
}

func TestAccountStateNotificationsConcurrentTransitions(t *testing.T) {
	for _, tt := range accountStateNotificationCases() {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
			recorder := &concurrentAccountNotificationRecorder{inputs: make(chan service.NotificationEmailSendInput, 16)}
			repo.abnormalNotificationSender = recorder
			account := mustCreateAccount(t, integrationEntClient, &service.Account{
				Name: "concurrent " + tt.name, Platform: service.PlatformOpenAI,
				Extra: map[string]any{
					service.AccountExtraAbnormalNotifyEnabled:  true,
					service.AccountExtraAbnormalNotifyEmail:    "alerts@example.com",
					service.AccountExtraAbnormalNotifyStatuses: []string{tt.status},
				},
			})
			t.Cleanup(func() { require.NoError(t, integrationEntClient.Account.DeleteOneID(account.ID).Exec(ctx)) })
			start := make(chan struct{})
			errs := make(chan error, 12)
			var wg sync.WaitGroup
			until := time.Now().Add(7 * 24 * time.Hour)
			for range 12 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					errs <- tt.enter(repo, ctx, account.ID, until)
				}()
			}
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
			require.Len(t, recorder.inputs, 1, "concurrent writes must claim one state transition")
		})
	}
}

func TestAccountStateNotificationsLegacyAndDisabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			ctx := context.Background()
			tx := testEntTx(t)
			repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
			recorder := &abnormalNotificationSenderRecorder{}
			repo.abnormalNotificationSender = recorder
			account := mustCreateAccount(t, tx.Client(), &service.Account{Name: "legacy notification", Extra: map[string]any{
				service.AccountExtraAbnormalNotifyEnabled: enabled,
				service.AccountExtraAbnormalNotifyEmail:   "alerts@example.com",
			}})
			for _, tt := range accountStateNotificationCases() {
				require.NoError(t, tt.enter(repo, ctx, account.ID, time.Now().Add(time.Hour)))
			}
			require.Empty(t, recorder.inputs, "legacy accounts must not opt into new categories")
			require.NoError(t, repo.SetError(ctx, account.ID, "expired token"))
			if enabled {
				require.Len(t, recorder.inputs, 1)
			} else {
				require.Empty(t, recorder.inputs)
			}
		})
	}
}
