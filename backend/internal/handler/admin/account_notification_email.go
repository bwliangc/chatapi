package admin

import (
	"context"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type accountNotificationEmailSender interface {
	Send(context.Context, service.NotificationEmailSendInput) error
}

// SetNotificationEmailService attaches the account-notification sender without
// changing the constructor signature used throughout the existing test suite.
func (h *AccountHandler) SetNotificationEmailService(notificationEmailService *service.NotificationEmailService) {
	h.notificationEmailService = notificationEmailService
}

// SendAbnormalNotice sends a manually confirmed abnormal-account notice to the
// email address stored with the upstream account.
// POST /api/v1/admin/accounts/:id/send-abnormal-notice
func (h *AccountHandler) SendAbnormalNotice(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if h.notificationEmailService == nil {
		response.InternalError(c, "notification email service is not configured")
		return
	}

	ctx := c.Request.Context()
	account, err := h.adminService.GetAccount(ctx, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	status, reason, abnormal := accountAbnormalNoticeState(account, time.Now().UTC())
	if !abnormal {
		status = firstAccountAbnormalReason(account.Status, "unknown")
		reason = "-"
	}

	recipient := h.accountNotificationRecipientEmail(ctx, account)
	if recipient == "" {
		response.BadRequest(c, "account email is not available")
		return
	}

	if err := h.notificationEmailService.Send(ctx, service.NotificationEmailSendInput{
		Event:          service.NotificationEmailEventAccountAbnormalNotice,
		Locale:         c.GetHeader("Accept-Language"),
		RecipientEmail: recipient,
		RecipientName:  accountNotificationRecipientName(recipient),
		Variables: map[string]string{
			"account_id":     strconv.FormatInt(account.ID, 10),
			"account_name":   account.Name,
			"platform":       account.Platform,
			"account_status": status,
			"error_message":  reason,
		},
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"message":         "account abnormal notification sent",
		"recipient_email": recipient,
	})
}

func (h *AccountHandler) accountNotificationRecipientEmail(ctx context.Context, account *service.Account) string {
	if email := accountStoredEmail(account); email != "" {
		return email
	}
	if account == nil || account.ParentAccountID == nil || h.adminService == nil {
		return ""
	}
	parent, err := h.adminService.GetAccount(ctx, *account.ParentAccountID)
	if err != nil {
		return ""
	}
	return accountStoredEmail(parent)
}

func accountStoredEmail(account *service.Account) string {
	if account == nil {
		return ""
	}
	for _, source := range []map[string]any{account.Extra, account.Credentials} {
		for _, key := range []string{"email_address", "email"} {
			email, ok := source[key].(string)
			if !ok {
				continue
			}
			email = strings.TrimSpace(email)
			parsed, err := mail.ParseAddress(email)
			if err == nil && strings.EqualFold(parsed.Address, email) {
				return email
			}
		}
	}
	return ""
}

func accountAbnormalNoticeState(account *service.Account, now time.Time) (status, reason string, abnormal bool) {
	if account == nil {
		return "", "", false
	}
	if account.Status == "error" {
		return "error", firstAccountAbnormalReason(account.ErrorMessage, "account is in an error state"), true
	}
	if strings.TrimSpace(account.ErrorMessage) != "" {
		return "error", strings.TrimSpace(account.ErrorMessage), true
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return "rate_limited", "the account is currently rate limited", true
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		return "overloaded", "the upstream service is currently overloaded", true
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		return "temporarily_unschedulable", firstAccountAbnormalReason(account.TempUnschedulableReason, "the account is temporarily unavailable"), true
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		return "expired", "the account has expired", true
	}
	return "", "", false
}

func firstAccountAbnormalReason(reason, fallback string) string {
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		return trimmed
	}
	return fallback
}

func accountNotificationRecipientName(email string) string {
	email = strings.TrimSpace(email)
	if at := strings.Index(email, "@"); at > 0 {
		return email[:at]
	}
	return email
}
