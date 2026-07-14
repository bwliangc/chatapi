package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorCustomProviderMigration(t *testing.T) {
	content, err := FS.ReadFile("177_channel_monitor_custom_provider.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "channel_monitors_provider_check")
	require.Contains(t, sql, "channel_monitor_request_templates_provider_check")
	require.Contains(t, sql, "CHECK (provider IN ('openai', 'anthropic', 'gemini', 'grok', 'custom'))")
	require.Contains(t, sql, "position('custom' IN monitor_constraint_def) = 0")
	require.Contains(t, sql, "position('custom' IN template_constraint_def) = 0")
}
