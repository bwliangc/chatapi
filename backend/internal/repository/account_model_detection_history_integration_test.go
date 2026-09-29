//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestModelDetectionHistoryPersistenceAndPagination(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newAccountRepositoryWithSQL(client, tx, nil)
	account := mustCreateAccount(t, client, &service.Account{Name: "history-account", Extra: map[string]any{"unrelated": "preserved"}})
	other := mustCreateAccount(t, client, &service.Account{Name: "other-account"})
	snapshot := service.ModelDetectionSnapshot{Status: "consistent", Model: "gpt-6-sol", CheckedAt: time.Now().UTC(), Reason: "test"}
	require.NoError(t, repo.SaveModelDetectionResult(ctx, other.ID, "manual", snapshot))
	for i := 0; i < 3; i++ {
		snapshot.CheckedAt = snapshot.CheckedAt.Add(time.Second)
		snapshot.Status = []string{"consistent", "suspected_mismatch", "error"}[i]
		require.NoError(t, repo.SaveModelDetectionResult(ctx, account.ID, "scheduled", snapshot))
	}
	items, total, err := repo.ListModelDetectionHistory(ctx, account.ID, 1, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 2)
	require.Equal(t, "error", items[0].Status)
	require.Equal(t, "suspected_mismatch", items[1].Status)
	require.Greater(t, items[0].ID, items[1].ID)
	require.Equal(t, "scheduled", items[0].Source)
	items, total, err = repo.ListModelDetectionHistory(ctx, account.ID, 2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 1)
	require.Equal(t, "consistent", items[0].Status)
	items, total, err = repo.ListModelDetectionHistory(ctx, account.ID, 3, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Empty(t, items)
	latest, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "preserved", latest.Extra["unrelated"])
	require.Equal(t, "error", latest.Extra[service.ModelDetectionSnapshotExtraKey].(map[string]any)["status"])
	// Force the history INSERT to fail. The latest snapshot must not change.
	_, err = client.ExecContext(ctx, "SAVEPOINT detection_failure")
	require.NoError(t, err)
	snapshot.Status = "consistent"
	require.Error(t, repo.SaveModelDetectionResult(ctx, account.ID, "invalid-source", snapshot))
	_, err = client.ExecContext(ctx, "ROLLBACK TO SAVEPOINT detection_failure")
	require.NoError(t, err)
	latest, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "error", latest.Extra[service.ModelDetectionSnapshotExtraKey].(map[string]any)["status"])
	_, total, err = repo.ListModelDetectionHistory(ctx, account.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.ErrorIs(t, repo.SaveModelDetectionResult(ctx, -1, "manual", snapshot), service.ErrAccountNotFound)
}

func TestModelDetectionHistoryMigrationPreservesLatestOnce(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newAccountRepositoryWithSQL(client, tx, nil)
	snapshot := service.ModelDetectionSnapshot{Status: "insufficient", Model: "gpt-6-sol", CheckedAt: time.Now().UTC(), Reason: "sample"}
	account := mustCreateAccount(t, client, &service.Account{Name: "legacy-history", Extra: map[string]any{service.ModelDetectionSnapshotExtraKey: snapshot}})
	without := mustCreateAccount(t, client, &service.Account{Name: "no-history"})
	migration, err := migrations.FS.ReadFile("242_model_detection_history.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = client.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	items, total, err := repo.ListModelDetectionHistory(ctx, account.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "legacy", items[0].Source)
	require.Equal(t, snapshot, items[0].ModelDetectionSnapshot)
	items, total, err = repo.ListModelDetectionHistory(ctx, without.ID, 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)
}
