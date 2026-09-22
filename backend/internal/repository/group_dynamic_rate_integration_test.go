//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupDynamicRatePublication(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	at := time.Now().UTC().Truncate(5 * time.Minute)
	groupRepo := newGroupRepositoryWithSQL(client, tx)
	group := &service.Group{
		Name: "dynamic-rate-publication", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		RateMultiplier: 1, DynamicRate: service.GroupDynamicRate{Enabled: true, Min: .5, Max: 2},
	}
	require.NoError(t, groupRepo.Create(ctx, group))
	saved, err := groupRepo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, group.DynamicRate, saved.DynamicRate)
	_, err = tx.ExecContext(ctx, `UPDATE groups SET created_at = $2 WHERE id = $1`, group.ID, at.Add(-10*24*time.Hour))
	require.NoError(t, err)
	user := mustCreateUser(t, client, &service.User{Email: "dynamic-rate@example.com"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-dynamic-rate", Name: "dynamic-rate"})
	account := mustCreateAccount(t, client, &service.Account{Name: "dynamic-rate"})
	usageRepo := newUsageLogRepositoryWithSQL(client, tx)
	for i := 1; i <= 27; i++ {
		factor := 1
		if i == 1 {
			factor = 4
		}
		_, err := usageRepo.Create(ctx, &service.UsageLog{
			UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID,
			RequestID: fmt.Sprintf("dynamic-rate-%d", i), Model: "gpt-5",
			InputTokens: 10 * factor, OutputTokens: 20 * factor, CacheCreationTokens: 30 * factor, CacheReadTokens: 40 * factor,
			CreatedAt: at.Add(-time.Duration(i)*time.Hour + 15*time.Minute),
		})
		require.NoError(t, err)
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM scheduler_outbox WHERE group_id = $1`, group.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM auth_cache_invalidation_outbox`)
	require.NoError(t, err)
	repo := newDashboardAggregationRepositoryWithSQL(tx)
	history, current, err := repo.groupDynamicRateUsage(ctx, group.ID, at.Add(-26*time.Hour), at, 25)
	require.NoError(t, err)
	require.Equal(t, 400.0, current, "all four token categories count towards demand")
	require.Len(t, history, 25)
	for _, tokens := range history {
		require.Equal(t, 100.0, tokens)
	}
	require.NoError(t, repo.SyncGroupDynamicRates(ctx, at))
	saved, err = groupRepo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, 1.25, saved.RateMultiplier)
	require.NotNil(t, saved.DynamicRateUpdatedAt)
	require.True(t, at.Equal(*saved.DynamicRateUpdatedAt))
	countRows := func(query string, args ...any) int {
		rows, err := tx.QueryContext(ctx, query, args...)
		require.NoError(t, err)
		defer rows.Close()
		require.True(t, rows.Next())
		var count int
		require.NoError(t, rows.Scan(&count))
		require.NoError(t, rows.Err())
		return count
	}
	require.Equal(t, 1, countRows(`SELECT COUNT(*) FROM scheduler_outbox WHERE group_id = $1`, group.ID))
	require.Greater(t, countRows(`SELECT COUNT(*) FROM auth_cache_invalidation_outbox`), 0, "publication invalidates request pricing snapshots")
	// A second instance/startup tick must not smooth the rate a second time.
	require.NoError(t, repo.SyncGroupDynamicRates(ctx, at.Add(time.Minute)))
	saved, err = groupRepo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, 1.25, saved.RateMultiplier)
	// No recent traffic lowers the published base rate on the next interval.
	require.NoError(t, repo.SyncGroupDynamicRates(ctx, at.Add(2*time.Hour)))
	saved, err = groupRepo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.Less(t, saved.RateMultiplier, 1.25)
	saved.DynamicRate = service.GroupDynamicRate{}
	require.NoError(t, groupRepo.Update(ctx, saved))
	rate := saved.RateMultiplier
	require.NoError(t, repo.SyncGroupDynamicRates(ctx, at.Add(3*time.Hour)))
	saved, err = groupRepo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, rate, saved.RateMultiplier, "disabled groups remain fixed")

	// Simulate an admin disabling dynamic pricing after the worker reads usage
	// but before it publishes. Even unchanged transaction-time updated_at must
	// not let the stale configuration overwrite this edit.
	saved.DynamicRate = group.DynamicRate
	require.NoError(t, groupRepo.Update(ctx, saved))
	beforePublish := &dynamicRateConcurrentEditExecutor{sqlExecutor: tx, edit: func() {
		_, err := tx.ExecContext(ctx, `UPDATE groups SET dynamic_rate = '{}', rate_multiplier = 1.7 WHERE id = $1`, group.ID)
		require.NoError(t, err)
	}}
	require.NoError(t, newDashboardAggregationRepositoryWithSQL(beforePublish).SyncGroupDynamicRates(ctx, at.Add(4*time.Hour)))
	saved, err = groupRepo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.False(t, saved.DynamicRate.Enabled)
	require.Equal(t, 1.7, saved.RateMultiplier)
}

type dynamicRateConcurrentEditExecutor struct {
	sqlExecutor
	edit func()
}

func (e *dynamicRateConcurrentEditExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if e.edit != nil {
		edit := e.edit
		e.edit = nil
		edit()
	}
	return e.sqlExecutor.ExecContext(ctx, query, args...)
}
