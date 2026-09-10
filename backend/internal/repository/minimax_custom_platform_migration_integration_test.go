//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigration237PreservesCustomPlatforms(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	// Recreate the custom branch's pre-MiniMax constraints and existing data.
	for _, spec := range []struct{ table, column, constraint string }{
		{"user_platform_quotas", "platform", "user_platform_quotas_platform_check"},
		{"composite_model_routes", "target_platform", "composite_model_routes_target_platform_check"},
		{"channel_monitors", "provider", "channel_monitors_provider_check"},
		{"channel_monitor_request_templates", "provider", "channel_monitor_request_templates_provider_check"},
	} {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT %s;
ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s IN
('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'custom', 'kimi', 'zhipu', 'deepseek'));`,
			spec.table, spec.constraint, spec.table, spec.constraint, spec.column))
		require.NoError(t, err)
	}

	var userID, groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users (email, password_hash)
VALUES ('migration237@example.com', 'test-hash') RETURNING id`).Scan(&userID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO groups (name, platform)
VALUES ('migration237', 'composite') RETURNING id`).Scan(&groupID))

	insertPlatform := func(platform string) {
		t.Helper()
		_, err := tx.ExecContext(ctx, `INSERT INTO user_platform_quotas (user_id, platform) VALUES ($1, $2)`, userID, platform)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO composite_model_routes (group_id, public_model, target_platform)
VALUES ($1, $2, $2)`, groupID, platform)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO channel_monitors
(name, provider, endpoint, api_key_encrypted, primary_model, interval_seconds, created_by)
VALUES ($1, $1, 'https://example.com', 'test-key', 'test-model', 60, $2)`, platform, userID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO channel_monitor_request_templates (name, provider) VALUES ($1, $1)`, platform)
		require.NoError(t, err)
	}
	insertPlatform("custom")

	migration, err := dbmigrations.FS.ReadFile("237_add_minimax_platform.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err, "upgrade must accept existing custom rows")
	insertPlatform("minimax")

	// Replaying the migration must preserve both sets of rows.
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	for _, table := range []string{"user_platform_quotas", "composite_model_routes", "channel_monitors", "channel_monitor_request_templates"} {
		column := "provider"
		if table == "user_platform_quotas" {
			column = "platform"
		} else if table == "composite_model_routes" {
			column = "target_platform"
		}
		var count int
		require.NoError(t, tx.QueryRowContext(ctx, fmt.Sprintf("SELECT count(DISTINCT %s) FROM %s WHERE %s IN ('custom', 'minimax')", column, table, column)).Scan(&count))
		require.Equal(t, 2, count, table)
	}
}
