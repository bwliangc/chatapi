package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type updateAccountAutoResetRequest struct {
	Enabled         bool                                 `json:"enabled"`
	Strategy        string                               `json:"strategy"`
	Conditions      *[]service.AccountAutoResetCondition `json:"conditions"`
	WeeklyThreshold float64                              `json:"weekly_threshold"`
	ExpiryMinutes   int                                  `json:"expiry_minutes"`
	ExpiryHours     *int                                 `json:"expiry_hours,omitempty"`
	Email           string                               `json:"email"`
}

func (h *AccountHandler) GetAutoReset(c *gin.Context) {
	account, ok := h.accountForAutoReset(c)
	if !ok {
		return
	}
	settings := service.AccountAutoResetSettingsFrom(account)
	if settings.Email == "" {
		settings.Email = h.accountNotificationRecipientEmail(c.Request.Context(), account)
	}
	response.Success(c, settings)
}

func (h *AccountHandler) UpdateAutoReset(c *gin.Context) {
	account, ok := h.accountForAutoReset(c)
	if !ok {
		return
	}
	var req updateAccountAutoResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	expiryMinutes := req.ExpiryMinutes
	if expiryMinutes == 0 && req.ExpiryHours != nil {
		expiryMinutes = *req.ExpiryHours * 60
	}
	strategy := service.AccountAutoResetSettingsFrom(account).Strategy
	if requestedStrategy := strings.TrimSpace(req.Strategy); requestedStrategy != "" {
		strategy = requestedStrategy
	}
	settings := service.AccountAutoResetSettingsFrom(account)
	settings.Enabled = req.Enabled
	settings.Strategy = strategy
	settings.Email = strings.TrimSpace(req.Email)
	if req.Conditions != nil {
		settings.Conditions = *req.Conditions
		for _, condition := range settings.Conditions {
			switch strings.TrimSpace(condition.Type) {
			case service.AccountAutoResetStrategyWeeklyThreshold:
				settings.WeeklyThreshold = condition.Value
			case service.AccountAutoResetStrategyCreditExpiry:
				settings.ExpiryMinutes = int(condition.Value)
			}
		}
	} else {
		settings.WeeklyThreshold = req.WeeklyThreshold
		settings.ExpiryMinutes = expiryMinutes
		settings.Conditions = nil
	}
	if settings.Email != "" {
		settings.Email = service.NormalizeEmail(settings.Email)
	}
	if err := service.ValidateAccountAutoResetSettings(settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.adminService.UpdateAccountExtra(c.Request.Context(), account.ID, service.AccountAutoResetExtraUpdates(settings)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AccountHandler) accountForAutoReset(c *gin.Context) (*service.Account, bool) {
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
	if account.Platform != service.PlatformOpenAI || account.Type != service.AccountTypeOAuth || account.IsShadow() {
		response.BadRequest(c, "auto reset is only supported for parent OpenAI OAuth accounts")
		return nil, false
	}
	return account, true
}
