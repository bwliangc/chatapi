package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	AccountExtraAutoResetEnabled         = "auto_reset_enabled"
	AccountExtraAutoResetStrategy        = "auto_reset_strategy"
	AccountExtraAutoResetConditions      = "auto_reset_conditions"
	AccountExtraAutoResetWeeklyThreshold = "auto_reset_weekly_threshold"
	AccountExtraAutoResetExpiryMinutes   = "auto_reset_expiry_minutes"
	accountExtraAutoResetExpiryHours     = "auto_reset_expiry_hours"
	AccountExtraAutoResetEmail           = "auto_reset_email"
	AccountExtraAutoResetNextCheckAt     = "auto_reset_next_check_at"
	AccountExtraAutoResetWeeklyArmed     = "auto_reset_weekly_armed"
	AccountExtraAutoResetLastAt          = "auto_reset_last_at"
	AccountExtraAutoResetLastStrategy    = "auto_reset_last_strategy"
	AccountExtraAutoResetLastError       = "auto_reset_last_error"

	AccountAutoResetStrategyWeeklyThreshold = "weekly_threshold"
	AccountAutoResetStrategyCreditExpiry    = "credit_expiry"
	AccountAutoResetStrategyBothConditions  = "weekly_threshold+credit_expiry"

	AccountAutoResetDefaultWeeklyThreshold = 90
	AccountAutoResetDefaultExpiryMinutes   = 24 * 60
)

type AccountAutoResetCondition struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

type AccountAutoResetSettings struct {
	Enabled         bool                        `json:"enabled"`
	Strategy        string                      `json:"strategy"`
	Conditions      []AccountAutoResetCondition `json:"conditions"`
	WeeklyThreshold float64                     `json:"weekly_threshold"`
	ExpiryMinutes   int                         `json:"expiry_minutes"`
	Email           string                      `json:"email"`
	LastResetAt     string                      `json:"last_reset_at,omitempty"`
	LastStrategy    string                      `json:"last_strategy,omitempty"`
	LastError       string                      `json:"last_error,omitempty"`
}

func AccountAutoResetSettingsFrom(account *Account) AccountAutoResetSettings {
	settings := AccountAutoResetSettings{
		Strategy:        AccountAutoResetStrategyWeeklyThreshold,
		WeeklyThreshold: AccountAutoResetDefaultWeeklyThreshold,
		ExpiryMinutes:   AccountAutoResetDefaultExpiryMinutes,
	}
	settings.Conditions = legacyAccountAutoResetConditions(settings)
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
	if minutes, ok := accountExtraInt(account.Extra[AccountExtraAutoResetExpiryMinutes]); ok {
		settings.ExpiryMinutes = minutes
	} else if hours, ok := accountExtraInt(account.Extra[accountExtraAutoResetExpiryHours]); ok {
		settings.ExpiryMinutes = hours * 60
	}
	settings.Email, _ = account.Extra[AccountExtraAutoResetEmail].(string)
	settings.Email = strings.TrimSpace(settings.Email)
	settings.LastResetAt, _ = account.Extra[AccountExtraAutoResetLastAt].(string)
	settings.LastStrategy, _ = account.Extra[AccountExtraAutoResetLastStrategy].(string)
	settings.LastError, _ = account.Extra[AccountExtraAutoResetLastError].(string)
	if rawConditions, ok := account.Extra[AccountExtraAutoResetConditions]; ok {
		settings.Conditions = parseAccountAutoResetConditions(rawConditions)
	} else {
		settings.Conditions = legacyAccountAutoResetConditions(settings)
	}
	return settings
}

func ValidateAccountAutoResetSettings(settings AccountAutoResetSettings) error {
	conditions := effectiveAccountAutoResetConditions(settings)
	if settings.Enabled && len(conditions) == 0 {
		return fmt.Errorf("at least one auto reset condition is required when enabled")
	}
	seen := make(map[string]struct{}, len(conditions))
	for _, condition := range conditions {
		conditionType := strings.TrimSpace(condition.Type)
		if _, exists := seen[conditionType]; exists {
			return fmt.Errorf("duplicate auto reset condition: %s", conditionType)
		}
		seen[conditionType] = struct{}{}
		switch conditionType {
		case AccountAutoResetStrategyWeeklyThreshold:
			if condition.Value < 1 || condition.Value > 100 {
				return fmt.Errorf("weekly threshold must be between 1 and 100")
			}
		case AccountAutoResetStrategyCreditExpiry:
			if condition.Value < 1 || condition.Value > 30*24*60 || math.Trunc(condition.Value) != condition.Value {
				return fmt.Errorf("expiry minutes must be an integer between 1 and 43200")
			}
		default:
			return fmt.Errorf("unsupported auto reset condition: %s", conditionType)
		}
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
		AccountExtraAutoResetConditions:      effectiveAccountAutoResetConditions(settings),
		AccountExtraAutoResetWeeklyThreshold: settings.WeeklyThreshold,
		AccountExtraAutoResetExpiryMinutes:   settings.ExpiryMinutes,
		AccountExtraAutoResetEmail:           strings.TrimSpace(settings.Email),
		AccountExtraAutoResetNextCheckAt:     nil,
		AccountExtraAutoResetWeeklyArmed:     true,
		AccountExtraAutoResetLastError:       "",
	}
}

func legacyAccountAutoResetConditions(settings AccountAutoResetSettings) []AccountAutoResetCondition {
	return []AccountAutoResetCondition{
		{Type: AccountAutoResetStrategyWeeklyThreshold, Value: settings.WeeklyThreshold},
		{Type: AccountAutoResetStrategyCreditExpiry, Value: float64(settings.ExpiryMinutes)},
	}
}

func effectiveAccountAutoResetConditions(settings AccountAutoResetSettings) []AccountAutoResetCondition {
	if settings.Conditions == nil {
		return legacyAccountAutoResetConditions(settings)
	}
	return settings.Conditions
}

func accountAutoResetConditionValue(settings AccountAutoResetSettings, conditionType string) (float64, bool) {
	for _, condition := range effectiveAccountAutoResetConditions(settings) {
		if strings.TrimSpace(condition.Type) == conditionType {
			return condition.Value, true
		}
	}
	return 0, false
}

func parseAccountAutoResetConditions(value any) []AccountAutoResetCondition {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []AccountAutoResetCondition{}
	}
	var conditions []AccountAutoResetCondition
	if err := json.Unmarshal(encoded, &conditions); err != nil || conditions == nil {
		return []AccountAutoResetCondition{}
	}
	for index := range conditions {
		conditions[index].Type = strings.TrimSpace(conditions[index].Type)
	}
	return conditions
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
