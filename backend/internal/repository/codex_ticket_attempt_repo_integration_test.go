//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCodexTicketHistoryPersistenceRetentionAndLock(t *testing.T) {
	ctx := context.Background()
	repo := NewCodexTicketAttemptRepository(integrationDB)
	const accountID int64 = 9223372036854775000
	t.Cleanup(func() { _, _ = integrationDB.Exec("DELETE FROM codex_ticket_attempts WHERE account_id=$1", accountID) })
	require.NoError(t, repo.Cleanup(ctx))
	now := time.Now().UTC()
	for _, item := range []struct {
		outcome string
		at      time.Time
	}{
		{"success", now.Add(-90 * 24 * time.Hour)},
		{"success", now}, {"miss", now}, {"error", now},
	} {
		a := &service.CodexTicketAttempt{AccountID: accountID, Model: "gpt-6-astra", OccurredAt: item.at, Outcome: item.outcome, Trigger: "automatic"}
		require.NoError(t, repo.Insert(ctx, a))
		require.Positive(t, a.ID)
	}
	rows, total, err := repo.List(ctx, accountID, "gpt-6-astra", false, 1, 2)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, rows, 2)
	rows, total, err = repo.List(ctx, accountID, "gpt-6-astra", true, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, rows, 2, "successful history survives beyond 30 days")
	_, total, err = repo.List(ctx, accountID, "gpt-5.6-sol", false, 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)

	unlock, acquired, err := repo.TryLock(ctx, accountID, "gpt-6-astra")
	require.NoError(t, err)
	require.True(t, acquired)
	defer unlock()
	_, acquired, err = repo.TryLock(ctx, accountID, "gpt-6-astra")
	require.NoError(t, err)
	require.False(t, acquired, "another connection cannot harvest the same account/model")
	unlockOther, acquired, err := repo.TryLock(ctx, accountID, "gpt-5.6-sol")
	require.NoError(t, err)
	require.True(t, acquired)
	unlockOther()
}
