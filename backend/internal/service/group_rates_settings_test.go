package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGroupRatesMenuSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.GroupRatesMenuEnabled)
	for _, enabled := range []bool{true, false} {
		settings.GroupRatesMenuEnabled = enabled
		require.NoError(t, svc.UpdateSettings(ctx, settings))
		stored, err := svc.GetAllSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, stored.GroupRatesMenuEnabled)
		public, err := svc.GetPublicSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, public.GroupRatesMenuEnabled)
		injected, err := svc.GetPublicSettingsForInjection(ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, injected.(*PublicSettingsInjectionPayload).GroupRatesMenuEnabled)
	}
}
