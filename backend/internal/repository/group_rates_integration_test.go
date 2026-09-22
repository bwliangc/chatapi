//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupRateUsageWindowsAndGroupIsolation(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	at := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	group := mustCreateGroup(t, client, &service.Group{Name: "board-visible"})
	other := mustCreateGroup(t, client, &service.Group{Name: "board-hidden", IsExclusive: true})
	user := mustCreateUser(t, client, &service.User{Email: "board@example.com"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-board", Name: "board"})
	account := mustCreateAccount(t, client, &service.Account{Name: "board-account"})
	repo := newUsageLogRepositoryWithSQL(client, tx)
	for i, row := range []struct {
		offset  time.Duration
		factor  int
		groupID int64
	}{
		{-24 * time.Hour, 1, group.ID},
		{-24*time.Hour - time.Second, 99, group.ID},
		{-time.Hour, 2, group.ID},
		{-time.Hour - time.Second, 3, group.ID},
		{-5 * time.Minute, 4, group.ID},
		{0, 50, group.ID},
		{-5 * time.Minute, 100, other.ID},
	} {
		_, err := repo.Create(ctx, &service.UsageLog{
			UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &row.groupID,
			RequestID: fmt.Sprintf("board-request-%d", i), Model: "gpt-5", CreatedAt: at.Add(row.offset),
			InputTokens: 10 * row.factor, OutputTokens: 20 * row.factor,
			CacheCreationTokens: 30 * row.factor, CacheReadTokens: 40 * row.factor,
		})
		require.NoError(t, err)
	}
	buckets, err := repo.GetGroupRateUsage(ctx, []int64{group.ID}, at)
	require.NoError(t, err)
	var day, hour service.GroupRateUsage
	for _, b := range buckets {
		require.Equal(t, group.ID, b.GroupID)
		require.Equal(t, b.At.Truncate(time.Hour), b.At)
		day.Add(b.Usage)
		hour.Add(b.LastHour)
	}
	require.Equal(t, int64(4), day.Requests)
	require.Equal(t, int64(1000), day.TotalTokens)
	require.Equal(t, int64(100), day.InputTokens)
	require.Equal(t, int64(200), day.OutputTokens)
	require.Equal(t, int64(300), day.CacheCreationTokens)
	require.Equal(t, int64(400), day.CacheReadTokens)
	require.Equal(t, int64(2), hour.Requests)
	require.Equal(t, int64(600), hour.TotalTokens)
	empty, err := repo.GetGroupRateUsage(ctx, nil, at)
	require.NoError(t, err)
	require.Empty(t, empty)
}
