package service

import (
	"net/mail"
	"slices"
	"strings"
)

const (
	AccountExtraAbnormalNotifyEnabled  = "abnormal_notify_enabled"
	AccountExtraAbnormalNotifyEmail    = "abnormal_notify_email"
	AccountExtraAbnormalNotifyStatuses = "abnormal_notify_statuses"

	AccountNotifyStatusError             = "error"
	AccountNotifyStatusRateLimited       = "rate_limited"
	AccountNotifyStatusOverloaded        = "overloaded"
	AccountNotifyStatusTempUnschedulable = "temp_unschedulable"
)

type AccountAbnormalNotificationSettings struct {
	Enabled  bool     `json:"enabled"`
	Email    string   `json:"email"`
	Statuses []string `json:"statuses"`
}

func AccountAbnormalNotificationSettingsFrom(account *Account) AccountAbnormalNotificationSettings {
	// Missing selections belong to the original error-only notification feature.
	settings := AccountAbnormalNotificationSettings{Statuses: []string{AccountNotifyStatusError}}
	if account == nil || account.Extra == nil {
		return settings
	}
	settings.Enabled, _ = account.Extra[AccountExtraAbnormalNotifyEnabled].(bool)
	settings.Email, _ = account.Extra[AccountExtraAbnormalNotifyEmail].(string)
	settings.Email = strings.TrimSpace(settings.Email)
	if raw, exists := account.Extra[AccountExtraAbnormalNotifyStatuses]; exists {
		settings.Statuses = []string{}
		appendStatus := func(status string) {
			if IsAccountNotificationStatus(status) && !slices.Contains(settings.Statuses, status) {
				settings.Statuses = append(settings.Statuses, status)
			}
		}
		switch statuses := raw.(type) {
		case []string:
			for _, status := range statuses {
				appendStatus(status)
			}
		case []any:
			for _, value := range statuses {
				if status, ok := value.(string); ok {
					appendStatus(status)
				}
			}
		}
	}
	return settings
}

func IsAccountNotificationStatus(status string) bool {
	switch status {
	case AccountNotifyStatusError, AccountNotifyStatusRateLimited, AccountNotifyStatusOverloaded, AccountNotifyStatusTempUnschedulable:
		return true
	default:
		return false
	}
}

func (s AccountAbnormalNotificationSettings) Includes(status string) bool {
	return s.Enabled && slices.Contains(s.Statuses, status)
}

func AccountStoredEmail(account *Account) string {
	if account == nil {
		return ""
	}
	for _, source := range []map[string]any{account.Extra, account.Credentials} {
		for _, key := range []string{"email_address", "email"} {
			value, _ := source[key].(string)
			if email := NormalizeEmail(value); email != "" {
				return email
			}
		}
	}
	return ""
}

func NormalizeEmail(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(parsed.Address, value) {
		return ""
	}
	return parsed.Address
}

func NotificationRecipientName(email string) string {
	email = strings.TrimSpace(email)
	if at := strings.Index(email, "@"); at > 0 {
		return email[:at]
	}
	return email
}
