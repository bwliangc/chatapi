package service

import (
	"net/mail"
	"strings"
)

const (
	AccountExtraAbnormalNotifyEnabled = "abnormal_notify_enabled"
	AccountExtraAbnormalNotifyEmail   = "abnormal_notify_email"
)

type AccountAbnormalNotificationSettings struct {
	Enabled bool   `json:"enabled"`
	Email   string `json:"email"`
}

func AccountAbnormalNotificationSettingsFrom(account *Account) AccountAbnormalNotificationSettings {
	settings := AccountAbnormalNotificationSettings{}
	if account == nil || account.Extra == nil {
		return settings
	}
	settings.Enabled, _ = account.Extra[AccountExtraAbnormalNotifyEnabled].(bool)
	settings.Email, _ = account.Extra[AccountExtraAbnormalNotifyEmail].(string)
	settings.Email = strings.TrimSpace(settings.Email)
	return settings
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
