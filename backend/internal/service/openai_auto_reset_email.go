package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const openAIAutoResetEmailTimeout = 45 * time.Second

// A durable notification snapshot. Never re-evaluate quota or consume a credit
// to retry delivery. Keeping the key stable lets NotificationEmailService dedupe
// a retry when SMTP succeeded but clearing this snapshot failed.
type openAIAutoResetPendingEmail struct {
	RecipientEmail string            `json:"recipient_email"`
	ReminderKey    string            `json:"reminder_key"`
	Variables      map[string]string `json:"variables"`
}

func openAIAutoResetPendingEmails(account *Account) ([]openAIAutoResetPendingEmail, error) {
	if account == nil || account.Extra[AccountExtraAutoResetPendingEmails] == nil {
		return nil, nil
	}
	raw, err := json.Marshal(account.Extra[AccountExtraAutoResetPendingEmails])
	if err != nil {
		return nil, fmt.Errorf("encode pending reset notifications: %w", err)
	}
	var pending []openAIAutoResetPendingEmail
	if err := json.Unmarshal(raw, &pending); err != nil {
		return nil, fmt.Errorf("decode pending reset notifications: %w", err)
	}
	for _, item := range pending {
		if NormalizeEmail(item.RecipientEmail) == "" || item.ReminderKey == "" {
			return nil, fmt.Errorf("invalid pending reset notification")
		}
	}
	return pending, nil
}

func buildOpenAIAutoResetEmail(account *Account, settings AccountAutoResetSettings, result *OpenAIQuotaResetResult, strategy, triggerValue string, remaining int, now time.Time) openAIAutoResetPendingEmail {
	strategyLabel := "Weekly usage threshold"
	if strategy == AccountAutoResetStrategyCreditExpiry {
		strategyLabel = "Reset credit nearing expiry"
	} else if strategy == AccountAutoResetStrategyBothConditions {
		strategyLabel = "Weekly usage threshold or reset credit nearing expiry"
	}
	return openAIAutoResetPendingEmail{
		RecipientEmail: settings.Email,
		// The persisted notification ID does not expose the upstream credit ID.
		ReminderKey: uuid.NewString(),
		Variables: map[string]string{
			"account_id":        strconv.FormatInt(account.ID, 10),
			"account_name":      account.Name,
			"strategy":          strategyLabel,
			"trigger_value":     triggerValue,
			"windows_reset":     strconv.Itoa(result.WindowsReset),
			"remaining_credits": strconv.Itoa(remaining),
			"reset_time":        now.Format("2006-01-02 15:04:05 MST"),
		},
	}
}

// Called under the same leader lock as reset processing. Scan pending deliveries
// separately so disabling automatic resets does not discard an already-spent
// credit's notification. Failure here never causes another quota reset.
func (s *OpenAIAutoResetService) deliverPendingEmails(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	accounts, err := s.accountRepo.FindByExtraField(ctx, AccountExtraAutoResetEmailPending, true)
	if err != nil {
		return fmt.Errorf("list pending reset notifications: %w", err)
	}
	// Oldest retries first, so a failing low-ID account cannot starve the queue.
	sort.Slice(accounts, func(i, j int) bool {
		left, _ := accountExtraTime(accounts[i].Extra[AccountExtraAutoResetEmailNextRetry])
		right, _ := accountExtraTime(accounts[j].Extra[AccountExtraAutoResetEmailNextRetry])
		if left.Equal(right) {
			return accounts[i].ID < accounts[j].ID
		}
		return left.Before(right)
	})
	attempted := 0
	for i := range accounts {
		account := &accounts[i]
		if !openAIAutoResetAccountEligible(account) {
			continue
		}
		now := s.now().UTC()
		if next, ok := accountExtraTime(account.Extra[AccountExtraAutoResetEmailNextRetry]); ok && next.After(now) {
			continue
		}
		pending, err := openAIAutoResetPendingEmails(account)
		if err != nil {
			slog.Warn("openai_auto_reset_pending_email_invalid", "account_id", account.ID, "error", err)
			continue
		}
		for len(pending) > 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if attempted >= openAIAutoResetMaxPerCycle {
				return nil
			}
			attempted++
			item := pending[0]
			emailCtx, cancel := context.WithTimeout(ctx, openAIAutoResetEmailTimeout)
			var sendErr error
			if s.emailSender == nil {
				sendErr = fmt.Errorf("email sender is not configured")
			} else {
				sendErr = s.emailSender.Send(emailCtx, NotificationEmailSendInput{
					Event:          NotificationEmailEventAccountAutoReset,
					RecipientEmail: item.RecipientEmail,
					RecipientName:  NotificationRecipientName(item.RecipientEmail),
					SourceType:     "account",
					SourceID:       strconv.FormatInt(account.ID, 10),
					ReminderKey:    item.ReminderKey,
					Variables:      item.Variables,
				})
			}
			cancel()
			updates := map[string]any{
				AccountExtraAutoResetEmailLastError: "",
				AccountExtraAutoResetEmailNextRetry: nil,
			}
			if sendErr == nil {
				pending = pending[1:]
			} else {
				message := []rune("automatic reset email: " + strings.TrimSpace(sendErr.Error()))
				if len(message) > 240 {
					message = message[:240]
				}
				updates[AccountExtraAutoResetEmailLastError] = string(message)
				updates[AccountExtraAutoResetEmailNextRetry] = s.now().UTC().Add(openAIAutoResetRetryDelay).Format(time.RFC3339)
				slog.Warn("openai_auto_reset_email_failed", "account_id", account.ID, "error", sendErr)
			}
			updates[AccountExtraAutoResetPendingEmails] = pending
			updates[AccountExtraAutoResetEmailPending] = len(pending) > 0
			if err := s.persistResetExtra(ctx, account.ID, updates); err != nil {
				slog.Warn("openai_auto_reset_email_state_save_failed", "account_id", account.ID, "error", err)
				break
			}
			if sendErr != nil {
				break
			}
			slog.Info("openai_auto_reset_email_sent", "account_id", account.ID)
		}
	}
	return nil
}
