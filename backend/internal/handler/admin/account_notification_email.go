package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type accountNotificationEmailSender interface {
	Send(context.Context, service.NotificationEmailSendInput) error
}

func (h *AccountHandler) SetNotificationEmailService(notificationEmailService *service.NotificationEmailService) {
	h.notificationEmailService = notificationEmailService
}

type updateAccountAbnormalNotificationRequest struct {
	Enabled  bool     `json:"enabled"`
	Email    string   `json:"email"`
	Statuses []string `json:"statuses"`
}

func (h *AccountHandler) GetAbnormalNotification(c *gin.Context) {
	account, ok := h.accountForAbnormalNotification(c)
	if !ok {
		return
	}
	settings := service.AccountAbnormalNotificationSettingsFrom(account)
	if settings.Email == "" {
		settings.Email = h.accountNotificationRecipientEmail(c.Request.Context(), account)
	}
	response.Success(c, settings)
}

func (h *AccountHandler) UpdateAbnormalNotification(c *gin.Context) {
	account, ok := h.accountForAbnormalNotification(c)
	if !ok {
		return
	}
	var req updateAccountAbnormalNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email != "" {
		normalized := service.NormalizeEmail(req.Email)
		if normalized == "" {
			response.BadRequest(c, "invalid notification email")
			return
		}
		req.Email = normalized
	}
	if req.Enabled && req.Email == "" {
		response.BadRequest(c, "notification email is required when enabled")
		return
	}
	// Older clients omit statuses; preserve the account's current selection.
	if req.Statuses == nil {
		req.Statuses = service.AccountAbnormalNotificationSettingsFrom(account).Statuses
	}
	statuses := make([]string, 0, len(req.Statuses))
	seen := make(map[string]bool, len(req.Statuses))
	for _, status := range req.Statuses {
		if !service.IsAccountNotificationStatus(status) {
			response.BadRequest(c, "invalid notification status")
			return
		}
		if !seen[status] {
			statuses = append(statuses, status)
			seen[status] = true
		}
	}
	if req.Enabled && len(statuses) == 0 {
		response.BadRequest(c, "at least one notification status is required when enabled")
		return
	}
	if err := h.adminService.UpdateAccountExtra(c.Request.Context(), account.ID, map[string]any{
		service.AccountExtraAbnormalNotifyEnabled:  req.Enabled,
		service.AccountExtraAbnormalNotifyEmail:    req.Email,
		service.AccountExtraAbnormalNotifyStatuses: statuses,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.AccountAbnormalNotificationSettings{Enabled: req.Enabled, Email: req.Email, Statuses: statuses})
}

func (h *AccountHandler) accountForAbnormalNotification(c *gin.Context) (*service.Account, bool) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return nil, false
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	return account, true
}

func (h *AccountHandler) accountNotificationRecipientEmail(ctx context.Context, account *service.Account) string {
	if email := service.AccountStoredEmail(account); email != "" {
		return email
	}
	if account == nil || account.ParentAccountID == nil || h.adminService == nil {
		return ""
	}
	parent, err := h.adminService.GetAccount(ctx, *account.ParentAccountID)
	if err != nil {
		return ""
	}
	return service.AccountStoredEmail(parent)
}
