package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	openAIAutoResetCycleInterval = time.Minute
	openAIAutoResetLockKey       = "openai:auto-reset:leader"
	openAIAutoResetLockTTL       = 10 * time.Minute
	openAIAutoResetMaxPerCycle   = 10
	openAIAutoResetRetryDelay    = 10 * time.Minute
	openAIAutoResetWeeklyPoll    = 5 * time.Minute
	openAIAutoResetExpiryPoll    = time.Hour
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
	return nil
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

	triggered, triggerValue, updates := openAIAutoResetShouldTrigger(account, settings, usage, now)
	if !triggered {
		updates[AccountExtraAutoResetNextCheckAt] = openAIAutoResetNextCheck(settings, usage, now).Format(time.RFC3339)
		updates[AccountExtraAutoResetLastError] = ""
		return s.accountRepo.UpdateExtra(ctx, account.ID, updates)
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
	if s.recoverer != nil {
		if _, err := s.recoverer.RecoverAccountState(ctx, account.ID, AccountRecoveryOptions{InvalidateToken: true}); err != nil {
			slog.Warn("openai_auto_reset_recovery_failed", "account_id", account.ID, "error", err)
		}
	}

	remaining := max(credits.AvailableCount-1, 0)
	if refreshed, refreshErr := s.quota.QueryUsage(ctx, account.ID); refreshErr == nil && refreshed != nil {
		usage = refreshed
		if refreshed.RateLimitResetCredits != nil {
			remaining = refreshed.RateLimitResetCredits.AvailableCount
			_ = s.quota.CacheResetCreditsSnapshot(ctx, account.ID, refreshed.RateLimitResetCredits)
		}
	}

	updates[AccountExtraAutoResetLastAt] = now.Format(time.RFC3339)
	updates[AccountExtraAutoResetLastStrategy] = settings.Strategy
	updates[AccountExtraAutoResetLastError] = ""
	updates[AccountExtraAutoResetNextCheckAt] = openAIAutoResetNextCheck(settings, usage, now).Format(time.RFC3339)
	if settings.Strategy == AccountAutoResetStrategyWeeklyThreshold {
		updates[AccountExtraAutoResetWeeklyArmed] = false
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
		return fmt.Errorf("save automatic reset result: %w", err)
	}

	s.sendSuccessEmail(ctx, account, settings, result, triggerValue, remaining, now)
	return nil
}

func openAIAutoResetShouldTrigger(account *Account, settings AccountAutoResetSettings, usage *OpenAIQuotaUsage, now time.Time) (bool, string, map[string]any) {
	updates := make(map[string]any)
	switch settings.Strategy {
	case AccountAutoResetStrategyWeeklyThreshold:
		used, ok := openAIWeeklyUsedPercent(usage)
		if !ok {
			return false, "", updates
		}
		armed := true
		if account != nil && account.Extra != nil {
			if stored, ok := account.Extra[AccountExtraAutoResetWeeklyArmed].(bool); ok {
				armed = stored
			}
		}
		if used < settings.WeeklyThreshold {
			updates[AccountExtraAutoResetWeeklyArmed] = true
			return false, fmt.Sprintf("%.2f%%", used), updates
		}
		return armed, fmt.Sprintf("%.2f%%", used), updates
	case AccountAutoResetStrategyCreditExpiry:
		expiresAt, ok := earliestOpenAIResetCreditExpiry(usage, now)
		if !ok {
			return false, "", updates
		}
		triggerAt := expiresAt.Add(-time.Duration(settings.ExpiryHours) * time.Hour)
		return !now.Before(triggerAt), expiresAt.Format(time.RFC3339), updates
	default:
		return false, "", updates
	}
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
	if settings.Strategy == AccountAutoResetStrategyCreditExpiry {
		if expiresAt, ok := earliestOpenAIResetCreditExpiry(usage, now); ok {
			triggerAt := expiresAt.Add(-time.Duration(settings.ExpiryHours) * time.Hour)
			if triggerAt.After(now.Add(time.Minute)) && triggerAt.Before(now.Add(openAIAutoResetExpiryPoll)) {
				return triggerAt
			}
		}
		return now.Add(openAIAutoResetExpiryPoll)
	}
	return now.Add(openAIAutoResetWeeklyPoll)
}

func (s *OpenAIAutoResetService) recordFailure(ctx context.Context, accountID int64, now time.Time, err error) {
	if s == nil || s.accountRepo == nil {
		return
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 240 {
		message = message[:240]
	}
	_ = s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{
		AccountExtraAutoResetLastError:   message,
		AccountExtraAutoResetNextCheckAt: now.Add(openAIAutoResetRetryDelay).Format(time.RFC3339),
	})
}

func (s *OpenAIAutoResetService) sendSuccessEmail(
	ctx context.Context,
	account *Account,
	settings AccountAutoResetSettings,
	result *OpenAIQuotaResetResult,
	triggerValue string,
	remaining int,
	now time.Time,
) {
	if s == nil || s.emailSender == nil || account == nil {
		return
	}
	strategyLabel := "Weekly usage threshold"
	if settings.Strategy == AccountAutoResetStrategyCreditExpiry {
		strategyLabel = "Reset credit nearing expiry"
	}
	reminderKey := now.Format(time.RFC3339Nano)
	if result.Credit != nil {
		reminderKey = firstNonEmpty(result.Credit.ID, result.Credit.RedeemedAt, reminderKey)
	}
	if err := s.emailSender.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventAccountAutoReset,
		RecipientEmail: settings.Email,
		RecipientName:  NotificationRecipientName(settings.Email),
		SourceType:     "account",
		SourceID:       strconv.FormatInt(account.ID, 10),
		ReminderKey:    reminderKey,
		Variables: map[string]string{
			"account_id":        strconv.FormatInt(account.ID, 10),
			"account_name":      account.Name,
			"strategy":          strategyLabel,
			"trigger_value":     triggerValue,
			"windows_reset":     strconv.Itoa(result.WindowsReset),
			"remaining_credits": strconv.Itoa(remaining),
			"reset_time":        now.Format("2006-01-02 15:04:05 MST"),
		},
	}); err != nil {
		slog.Warn("openai_auto_reset_email_failed", "account_id", account.ID, "error", err)
	}
}
