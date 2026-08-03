package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	AccountExtraAutoResetEnabled         = "auto_reset_enabled"
	AccountExtraAutoResetStrategy        = "auto_reset_strategy"
	AccountExtraAutoResetWeeklyThreshold = "auto_reset_weekly_threshold"
	AccountExtraAutoResetExpiryHours     = "auto_reset_expiry_hours"
	AccountExtraAutoResetEmail           = "auto_reset_email"
	AccountExtraAutoResetNextCheckAt     = "auto_reset_next_check_at"
	AccountExtraAutoResetWeeklyArmed     = "auto_reset_weekly_armed"
	AccountExtraAutoResetLastAt          = "auto_reset_last_at"
	AccountExtraAutoResetLastStrategy    = "auto_reset_last_strategy"
	AccountExtraAutoResetLastError       = "auto_reset_last_error"

	AccountAutoResetStrategyWeeklyThreshold = "weekly_threshold"
	AccountAutoResetStrategyCreditExpiry    = "credit_expiry"

	AccountAutoResetDefaultWeeklyThreshold = 90
	AccountAutoResetDefaultExpiryHours     = 24
)

type AccountAutoResetSettings struct {
	Enabled         bool    `json:"enabled"`
	Strategy        string  `json:"strategy"`
	WeeklyThreshold float64 `json:"weekly_threshold"`
	ExpiryHours     int     `json:"expiry_hours"`
	Email           string  `json:"email"`
	LastResetAt     string  `json:"last_reset_at,omitempty"`
	LastStrategy    string  `json:"last_strategy,omitempty"`
	LastError       string  `json:"last_error,omitempty"`
}

func AccountAutoResetSettingsFrom(account *Account) AccountAutoResetSettings {
	settings := AccountAutoResetSettings{
		Strategy:        AccountAutoResetStrategyWeeklyThreshold,
		WeeklyThreshold: AccountAutoResetDefaultWeeklyThreshold,
		ExpiryHours:     AccountAutoResetDefaultExpiryHours,
	}
	if account == nil || account.Extra == nil {
		return settings
	}
	settings.Enabled, _ = account.Extra[AccountExtraAutoResetEnabled].(bool)
	if strategy, ok := account.Extra[AccountExtraAutoResetStrategy].(string); ok {
		settings.Strategy = strings.TrimSpace(strategy)
	}
	if threshold, ok := accountExtraFloat64(account.Extra[AccountExtraAutoResetWeeklyThreshold]); ok {
		settings.WeeklyThreshold = threshold
	}
	if hours, ok := accountExtraInt(account.Extra[AccountExtraAutoResetExpiryHours]); ok {
		settings.ExpiryHours = hours
	}
	settings.Email, _ = account.Extra[AccountExtraAutoResetEmail].(string)
	settings.Email = strings.TrimSpace(settings.Email)
	settings.LastResetAt, _ = account.Extra[AccountExtraAutoResetLastAt].(string)
	settings.LastStrategy, _ = account.Extra[AccountExtraAutoResetLastStrategy].(string)
	settings.LastError, _ = account.Extra[AccountExtraAutoResetLastError].(string)
	return settings
}

func ValidateAccountAutoResetSettings(settings AccountAutoResetSettings) error {
	if settings.Strategy != AccountAutoResetStrategyWeeklyThreshold && settings.Strategy != AccountAutoResetStrategyCreditExpiry {
		return fmt.Errorf("invalid auto reset strategy")
	}
	if settings.WeeklyThreshold < 1 || settings.WeeklyThreshold > 100 {
		return fmt.Errorf("weekly threshold must be between 1 and 100")
	}
	if settings.ExpiryHours < 1 || settings.ExpiryHours > 24*30 {
		return fmt.Errorf("expiry hours must be between 1 and 720")
	}
	if settings.Email != "" && NormalizeEmail(settings.Email) == "" {
		return fmt.Errorf("invalid notification email")
	}
	if settings.Enabled && strings.TrimSpace(settings.Email) == "" {
		return fmt.Errorf("notification email is required when enabled")
	}
	return nil
}

func AccountAutoResetExtraUpdates(settings AccountAutoResetSettings) map[string]any {
	return map[string]any{
		AccountExtraAutoResetEnabled:         settings.Enabled,
		AccountExtraAutoResetStrategy:        settings.Strategy,
		AccountExtraAutoResetWeeklyThreshold: settings.WeeklyThreshold,
		AccountExtraAutoResetExpiryHours:     settings.ExpiryHours,
		AccountExtraAutoResetEmail:           strings.TrimSpace(settings.Email),
		AccountExtraAutoResetNextCheckAt:     nil,
		AccountExtraAutoResetWeeklyArmed:     true,
		AccountExtraAutoResetLastError:       "",
	}
}

func accountExtraFloat64(value any) (float64, bool) {
	switch value := value.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func accountExtraInt(value any) (int, bool) {
	parsed, ok := accountExtraFloat64(value)
	if !ok {
		return 0, false
	}
	return int(parsed), true
}

func accountExtraTime(value any) (time.Time, bool) {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, text)
	return parsed, err == nil
}
