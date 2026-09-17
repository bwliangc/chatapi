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
	settings := AccountAbnormalNotificationSettingsFrom(account)
	require.Equal(t, AccountAbnormalNotificationSettings{Enabled: true, Email: "alerts@example.com", Statuses: []string{AccountNotifyStatusError}}, settings)
	require.True(t, settings.Includes(AccountNotifyStatusError))
	require.False(t, settings.Includes(AccountNotifyStatusRateLimited))
}

func TestAccountAbnormalNotificationSelectedStatuses(t *testing.T) {
	for _, statuses := range []any{
		[]string{AccountNotifyStatusRateLimited, AccountNotifyStatusOverloaded, AccountNotifyStatusRateLimited, "unknown"},
		[]any{AccountNotifyStatusRateLimited, AccountNotifyStatusOverloaded, AccountNotifyStatusRateLimited, "unknown", 123},
	} {
		account := &Account{Extra: map[string]any{
			AccountExtraAbnormalNotifyEnabled:  true,
			AccountExtraAbnormalNotifyStatuses: statuses,
		}}
		settings := AccountAbnormalNotificationSettingsFrom(account)
		require.Equal(t, []string{AccountNotifyStatusRateLimited, AccountNotifyStatusOverloaded}, settings.Statuses)
		require.True(t, settings.Includes(AccountNotifyStatusRateLimited))
		require.False(t, settings.Includes(AccountNotifyStatusError))
		account.Extra[AccountExtraAbnormalNotifyEnabled] = false
		require.False(t, AccountAbnormalNotificationSettingsFrom(account).Includes(AccountNotifyStatusRateLimited))
	}
}

func TestAccountAbnormalNotificationEmptySelectionDoesNotDefaultToError(t *testing.T) {
	settings := AccountAbnormalNotificationSettingsFrom(&Account{Extra: map[string]any{
		AccountExtraAbnormalNotifyEnabled:  true,
		AccountExtraAbnormalNotifyStatuses: []any{},
	}})
	require.NotNil(t, settings.Statuses)
	require.Empty(t, settings.Statuses)
	require.False(t, settings.Includes(AccountNotifyStatusError))
	require.Equal(t, []string{AccountNotifyStatusError}, AccountAbnormalNotificationSettingsFrom(nil).Statuses)
}

func TestAccountStoredEmailRejectsInvalidValues(t *testing.T) {
	require.Empty(t, AccountStoredEmail(&Account{Extra: map[string]any{"email": "invalid"}}))
	require.Equal(t, "owner@example.com", AccountStoredEmail(&Account{Credentials: map[string]any{"email_address": " owner@example.com "}}))
}
