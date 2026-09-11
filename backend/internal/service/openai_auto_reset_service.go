package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	openAIAutoResetCycleInterval = time.Minute
	openAIAutoResetLockKey       = "openai:auto-reset:leader"
	// Cover both quota processing and the bounded email batch, including SMTP timeouts.
	openAIAutoResetLockTTL        = 30 * time.Minute
	openAIAutoResetMaxPerCycle    = 10
	openAIAutoResetRetryDelay     = 10 * time.Minute
	openAIAutoResetWeeklyPoll     = 5 * time.Minute
	openAIAutoResetPersistTimeout = 5 * time.Second
	openAIAutoResetRefreshTimeout = 8 * time.Second
)

type openAIAutoResetAccountRepository interface {
	FindByExtraField(ctx context.Context, key string, value any) ([]Account, error)
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
}

type openAIAutoResetQuotaWorkflow interface {
	QueryUsage(ctx context.Context, accountID int64) (*OpenAIQuotaUsage, error)
	CacheResetCreditsSnapshot(ctx context.Context, accountID int64, credits *OpenAIRateLimitResetCredits) error
	ResetCredit(ctx context.Context, accountID int64) (*OpenAIQuotaResetResult, error)
}

type openAIAutoResetAccountRecoverer interface {
	RecoverAccountState(ctx context.Context, accountID int64, options AccountRecoveryOptions) (*SuccessfulTestRecoveryResult, error)
}

type openAIAutoResetEmailSender interface {
	Send(ctx context.Context, input NotificationEmailSendInput) error
}

type OpenAIAutoResetService struct {
	accountRepo  openAIAutoResetAccountRepository
	quota        openAIAutoResetQuotaWorkflow
	recoverer    openAIAutoResetAccountRecoverer
	emailSender  openAIAutoResetEmailSender
	lockCache    LeaderLockCache
	db           *sql.DB
	instanceID   string
	now          func() time.Time
	parentCtx    context.Context
	parentCancel context.CancelFunc
	mu           sync.Mutex
	cycleMu      sync.Mutex
	started      bool
	stopped      bool
	wg           sync.WaitGroup
}

func NewOpenAIAutoResetService(
	accountRepo openAIAutoResetAccountRepository,
	quota openAIAutoResetQuotaWorkflow,
	recoverer openAIAutoResetAccountRecoverer,
	emailSender openAIAutoResetEmailSender,
) *OpenAIAutoResetService {
	ctx, cancel := context.WithCancel(context.Background())
	return &OpenAIAutoResetService{
		accountRepo:  accountRepo,
		quota:        quota,
		recoverer:    recoverer,
		emailSender:  emailSender,
		instanceID:   uuid.NewString(),
		now:          time.Now,
		parentCtx:    ctx,
		parentCancel: cancel,
	}
}

func ProvideOpenAIAutoResetService(
	accountRepo AccountRepository,
	quota *OpenAIQuotaService,
	recoverer *RateLimitService,
	emailSender *NotificationEmailService,
	lockCache LeaderLockCache,
	db *sql.DB,
) *OpenAIAutoResetService {
	service := NewOpenAIAutoResetService(accountRepo, quota, recoverer, emailSender)
	service.lockCache = lockCache
	service.db = db
	service.Start()
	return service
}

func (s *OpenAIAutoResetService) Start() {
	if s == nil || s.accountRepo == nil || s.quota == nil {
		return
	}
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.wg.Add(1)
	s.mu.Unlock()
	go s.runLoop()
}

func (s *OpenAIAutoResetService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.parentCancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *OpenAIAutoResetService) runLoop() {
	defer s.wg.Done()
	_ = s.RunDue(s.parentCtx)
	ticker := time.NewTicker(openAIAutoResetCycleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.parentCtx.Done():
			return
		case <-ticker.C:
			if err := s.RunDue(s.parentCtx); err != nil {
				slog.Error("openai_auto_reset_cycle_failed", "error", err)
			}
		}
	}
}

func (s *OpenAIAutoResetService) RunDue(ctx context.Context) error {
	if s == nil || s.accountRepo == nil || s.quota == nil {
		return nil
	}
	s.cycleMu.Lock()
	defer s.cycleMu.Unlock()

	release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, openAIAutoResetLockKey, s.instanceID, openAIAutoResetLockTTL)
	if !acquired {
		return nil
	}
	defer release()

	accounts, err := s.accountRepo.FindByExtraField(ctx, AccountExtraAutoResetEnabled, true)
	if err != nil {
		return fmt.Errorf("list enabled accounts: %w", err)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	now := s.now().UTC()
	processed := 0
	for i := range accounts {
		if ctx.Err() != nil {
			break
		}
		account := &accounts[i]
		if !openAIAutoResetAccountEligible(account) || !openAIAutoResetDue(account, now) {
			continue
		}
		if processed >= openAIAutoResetMaxPerCycle {
			break
		}
		processed++
		accountCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		if err := s.processAccount(accountCtx, account, now); err != nil {
			slog.Warn("openai_auto_reset_account_failed", "account_id", account.ID, "error", err)
			s.recordFailure(accountCtx, account.ID, now, err)
		}
		cancel()
	}
	return s.deliverPendingEmails(ctx)
}

func openAIAutoResetAccountEligible(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth && !account.IsShadow()
}

func openAIAutoResetDue(account *Account, now time.Time) bool {
	if account == nil || account.Extra == nil {
		return true
	}
	next, ok := accountExtraTime(account.Extra[AccountExtraAutoResetNextCheckAt])
	return !ok || !next.After(now)
}

func (s *OpenAIAutoResetService) processAccount(ctx context.Context, account *Account, now time.Time) error {
	settings := AccountAutoResetSettingsFrom(account)
	if !settings.Enabled {
		return nil
	}
	if err := ValidateAccountAutoResetSettings(settings); err != nil {
		return err
	}

	usage, err := s.quota.QueryUsage(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("query quota: %w", err)
	}
	if usage == nil {
		return fmt.Errorf("query quota returned empty result")
	}
	if usage.RateLimitResetCredits != nil {
		_ = s.quota.CacheResetCreditsSnapshot(ctx, account.ID, usage.RateLimitResetCredits)
	}

	triggerStrategy, triggerValue, updates := openAIAutoResetShouldTrigger(account, settings, usage, now)
	if triggerStrategy == "" {
		updates[AccountExtraAutoResetNextCheckAt] = openAIAutoResetNextCheck(settings, usage, now).Format(time.RFC3339)
		updates[AccountExtraAutoResetLastError] = ""
		return s.accountRepo.UpdateExtra(ctx, account.ID, updates)
	}

	pending, err := openAIAutoResetPendingEmails(account)
	if err != nil {
		return err
	}
	credits := usage.RateLimitResetCredits
	if credits == nil || credits.AvailableCount <= 0 {
		return fmt.Errorf("no reset credits available")
	}
	result, err := s.quota.ResetCredit(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("consume reset credit: %w", err)
	}
	if result == nil {
		return fmt.Errorf("consume reset credit returned empty result")
	}
	if strings.EqualFold(strings.TrimSpace(result.Code), "no_credit") {
		return fmt.Errorf("no reset credit consumed")
	}

	// Persist the successful side effect and its notification before any optional
	// upstream refresh. A spent credit must not lose its email to the quota deadline.
	remaining := max(credits.AvailableCount-1, 0)
	pending = append(pending, buildOpenAIAutoResetEmail(account, settings, result, triggerStrategy, triggerValue, remaining, now))
	updates[AccountExtraAutoResetLastAt] = now.Format(time.RFC3339)
	updates[AccountExtraAutoResetLastStrategy] = triggerStrategy
	updates[AccountExtraAutoResetLastError] = ""
	updates[AccountExtraAutoResetNextCheckAt] = now.Add(openAIAutoResetWeeklyPoll).Format(time.RFC3339)
	updates[AccountExtraAutoResetPendingEmails] = pending
	updates[AccountExtraAutoResetEmailPending] = true
	if strings.Contains(triggerStrategy, AccountAutoResetStrategyWeeklyThreshold) {
		updates[AccountExtraAutoResetWeeklyArmed] = false
	}
	if err := s.persistResetExtra(ctx, account.ID, updates); err != nil {
		return fmt.Errorf("save automatic reset result and notification: %w", err)
	}

	refreshCtx, cancelRefresh := context.WithTimeout(context.WithoutCancel(ctx), openAIAutoResetRefreshTimeout)
	defer cancelRefresh()
	if s.recoverer != nil {
		if _, err := s.recoverer.RecoverAccountState(refreshCtx, account.ID, AccountRecoveryOptions{InvalidateToken: true}); err != nil {
			slog.Warn("openai_auto_reset_recovery_failed", "account_id", account.ID, "error", err)
		}
	}
	if refreshed, refreshErr := s.quota.QueryUsage(refreshCtx, account.ID); refreshErr == nil && refreshed != nil {
		if refreshed.RateLimitResetCredits != nil {
			remaining = refreshed.RateLimitResetCredits.AvailableCount
			_ = s.quota.CacheResetCreditsSnapshot(refreshCtx, account.ID, refreshed.RateLimitResetCredits)
		}
		pending[len(pending)-1].Variables["remaining_credits"] = fmt.Sprint(remaining)
		if err := s.persistResetExtra(ctx, account.ID, map[string]any{
			AccountExtraAutoResetPendingEmails: pending,
			AccountExtraAutoResetNextCheckAt:   openAIAutoResetNextCheck(settings, refreshed, now).Format(time.RFC3339),
		}); err != nil {
			slog.Warn("openai_auto_reset_refresh_save_failed", "account_id", account.ID, "error", err)
		}
	} else {
		slog.Warn("openai_auto_reset_refresh_failed", "account_id", account.ID, "error", refreshErr)
	}

	return nil
}

func openAIAutoResetShouldTrigger(account *Account, settings AccountAutoResetSettings, usage *OpenAIQuotaUsage, now time.Time) (string, string, map[string]any) {
	updates := make(map[string]any)
	strategies := make([]string, 0, 2)
	values := make([]string, 0, 2)

	weeklyThreshold, weeklyEnabled := accountAutoResetConditionValue(settings, AccountAutoResetStrategyWeeklyThreshold)
	if used, ok := openAIWeeklyUsedPercent(usage); weeklyEnabled && ok {
		armed := true
		if account != nil && account.Extra != nil {
			if stored, ok := account.Extra[AccountExtraAutoResetWeeklyArmed].(bool); ok {
				armed = stored
			}
		}
		if used < weeklyThreshold {
			updates[AccountExtraAutoResetWeeklyArmed] = true
		} else if armed {
			strategies = append(strategies, AccountAutoResetStrategyWeeklyThreshold)
			values = append(values, fmt.Sprintf("weekly_used=%.2f%%", used))
		}
	}

	expiryMinutes, expiryEnabled := accountAutoResetConditionValue(settings, AccountAutoResetStrategyCreditExpiry)
	if expiresAt, ok := earliestOpenAIResetCreditExpiry(usage, now); expiryEnabled && ok {
		triggerAt := expiresAt.Add(-time.Duration(expiryMinutes) * time.Minute)
		if !now.Before(triggerAt) {
			strategies = append(strategies, AccountAutoResetStrategyCreditExpiry)
			values = append(values, "credit_expires_at="+expiresAt.Format(time.RFC3339))
		}
	}

	return strings.Join(strategies, "+"), strings.Join(values, "; "), updates
}

func openAIWeeklyUsedPercent(usage *OpenAIQuotaUsage) (float64, bool) {
	if usage == nil || usage.RateLimit == nil {
		return 0, false
	}
	windows := []*OpenAIRateLimitWindow{usage.RateLimit.PrimaryWindow, usage.RateLimit.SecondaryWindow}
	var weekly *OpenAIRateLimitWindow
	for _, window := range windows {
		if window == nil {
			continue
		}
		if weekly == nil || window.LimitWindowSeconds > weekly.LimitWindowSeconds {
			weekly = window
		}
	}
	if weekly == nil {
		return 0, false
	}
	return weekly.UsedPercent, true
}

func earliestOpenAIResetCreditExpiry(usage *OpenAIQuotaUsage, now time.Time) (time.Time, bool) {
	if usage == nil || usage.RateLimitResetCredits == nil || usage.RateLimitResetCredits.AvailableCount <= 0 {
		return time.Time{}, false
	}
	var earliest time.Time
	for _, credit := range usage.RateLimitResetCredits.Credits {
		expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(credit.ExpiresAt))
		if err != nil || !expiresAt.After(now) {
			continue
		}
		if earliest.IsZero() || expiresAt.Before(earliest) {
			earliest = expiresAt
		}
	}
	return earliest, !earliest.IsZero()
}

func openAIAutoResetNextCheck(settings AccountAutoResetSettings, usage *OpenAIQuotaUsage, now time.Time) time.Time {
	next := now.Add(openAIAutoResetWeeklyPoll)
	expiryMinutes, expiryEnabled := accountAutoResetConditionValue(settings, AccountAutoResetStrategyCreditExpiry)
	if expiresAt, ok := earliestOpenAIResetCreditExpiry(usage, now); expiryEnabled && ok {
		triggerAt := expiresAt.Add(-time.Duration(expiryMinutes) * time.Minute)
		if triggerAt.After(now) && triggerAt.Before(next) {
			return triggerAt
		}
	}
	return next
}

func (s *OpenAIAutoResetService) recordFailure(ctx context.Context, accountID int64, now time.Time, err error) {
	if s == nil || s.accountRepo == nil {
		return
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 240 {
		message = message[:240]
	}
	if persistErr := s.persistResetExtra(ctx, accountID, map[string]any{
		AccountExtraAutoResetLastError:   message,
		AccountExtraAutoResetNextCheckAt: now.Add(openAIAutoResetRetryDelay).Format(time.RFC3339),
	}); persistErr != nil {
		slog.Warn("openai_auto_reset_failure_save_failed", "account_id", accountID, "error", persistErr)
	}
}

// Give result persistence its own bounded budget after an upstream side effect,
// even if the operation context has expired or shutdown has started.
func (s *OpenAIAutoResetService) persistResetExtra(ctx context.Context, accountID int64, updates map[string]any) error {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAutoResetPersistTimeout)
	defer cancel()
	return s.accountRepo.UpdateExtra(persistCtx, accountID, updates)
}
