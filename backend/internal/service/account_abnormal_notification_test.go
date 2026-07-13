package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountAbnormalNotificationSettingsFrom(t *testing.T) {
	account := &Account{Extra: map[string]any{
		AccountExtraAbnormalNotifyEnabled: true,
		AccountExtraAbnormalNotifyEmail:   " alerts@example.com ",
	}}
	require.Equal(t, AccountAbnormalNotificationSettings{Enabled: true, Email: "alerts@example.com"}, AccountAbnormalNotificationSettingsFrom(account))
}

func TestAccountStoredEmailRejectsInvalidValues(t *testing.T) {
	require.Empty(t, AccountStoredEmail(&Account{Extra: map[string]any{"email": "invalid"}}))
	require.Equal(t, "owner@example.com", AccountStoredEmail(&Account{Credentials: map[string]any{"email_address": " owner@example.com "}}))
}
