package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type rateBoardGroupsStub struct {
	groups map[int64][]Group
	rates  map[int64]map[int64]float64
	err    error
	calls  int
}

func (p *rateBoardGroupsStub) GetAvailableGroups(_ context.Context, userID int64) ([]Group, error) {
	p.calls++
	return p.groups[userID], p.err
}
func (p *rateBoardGroupsStub) GetUserGroupRates(_ context.Context, userID int64) (map[int64]float64, error) {
	return p.rates[userID], nil
}

type rateBoardUsageStub struct {
	ids   []int64
	rows  []GroupRateUsageBucket
	err   error
	calls int
}

func (r *rateBoardUsageStub) GetGroupRateUsage(_ context.Context, ids []int64, _ time.Time) ([]GroupRateUsageBucket, error) {
	r.calls++
	r.ids = append([]int64(nil), ids...)
	return r.rows, r.err
}

func TestGroupRatesBoardPermissionsCacheAndPersonalRates(t *testing.T) {
	at := time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)
	shared := Group{ID: 1, Name: "Shared", Status: StatusActive, RateMultiplier: 1.2}
	private := Group{ID: 2, Name: "Private", Status: StatusActive, IsExclusive: true, RateMultiplier: .8}
	provider := &rateBoardGroupsStub{
		groups: map[int64][]Group{1: {shared, private}, 2: {shared}, 3: {}},
		rates:  map[int64]map[int64]float64{1: {1: 0}, 2: {1: .6}},
	}
	usage := &rateBoardUsageStub{rows: []GroupRateUsageBucket{
		{GroupID: 1, At: at.Truncate(time.Hour), Usage: GroupRateUsage{Requests: 3, TotalTokens: 100}, LastHour: GroupRateUsage{Requests: 2, TotalTokens: 90}},
		{GroupID: 2, At: at.Truncate(time.Hour), Usage: GroupRateUsage{Requests: 10, TotalTokens: 999}},
		{GroupID: 999, At: at.Truncate(time.Hour), Usage: GroupRateUsage{Requests: 10, TotalTokens: 9999}},
	}}
	svc := NewGroupRatesService(provider, usage)
	svc.now = func() time.Time { return at }
	ctx := context.Background()
	first, err := svc.Get(ctx, 1)
	require.NoError(t, err)
	require.Len(t, first.Groups, 2)
	require.Zero(t, first.Groups[0].EffectiveMultiplier, "a zero-price override is not absent")
	require.Equal(t, int64(100), first.Groups[0].Last24Hours.TotalTokens)
	require.Equal(t, int64(90), first.Groups[0].LastHour.TotalTokens)
	require.Len(t, first.Groups[0].Trend, 25)
	require.Equal(t, int64(0), first.Groups[0].Trend[0].Tokens, "idle hours are explicitly represented")
	require.Equal(t, []int64{1, 2}, usage.ids)
	second, err := svc.Get(ctx, 2)
	require.NoError(t, err)
	require.Len(t, second.Groups, 1)
	require.Equal(t, .6, second.Groups[0].EffectiveMultiplier)
	require.Equal(t, []int64{1}, usage.ids, "query only the caller's visible groups")
	require.Equal(t, 2, usage.calls)
	// Revocation is checked even while the shared usage cache is warm.
	provider.groups[1] = []Group{shared}
	provider.groups[1][0].RateMultiplier = 1.5
	third, err := svc.Get(ctx, 1)
	require.NoError(t, err)
	require.Len(t, third.Groups, 1)
	require.Equal(t, 1.5, third.Groups[0].RateMultiplier, "rates are not cached with usage")
	require.Zero(t, third.Groups[0].EffectiveMultiplier, "another viewer's override cannot leak through the cache")
	require.Equal(t, 2, usage.calls, "reuse aggregate usage for the same group set")
	empty, err := svc.Get(ctx, 3)
	require.NoError(t, err)
	require.NotNil(t, empty.Groups)
	require.Empty(t, empty.Groups)
	require.Equal(t, 2, usage.calls)
	provider.err = errors.New("permission lookup unavailable")
	_, err = svc.Get(ctx, 1)
	require.Error(t, err, "never serve cached data when authorization cannot be checked")
}

func TestGroupRatesBoardDoesNotCacheFailures(t *testing.T) {
	provider := &rateBoardGroupsStub{groups: map[int64][]Group{1: {{ID: 1, Status: StatusActive}}}}
	usage := &rateBoardUsageStub{err: errors.New("stats unavailable")}
	svc := NewGroupRatesService(provider, usage)
	_, err := svc.Get(context.Background(), 1)
	require.Error(t, err)
	usage.err = nil
	board, err := svc.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, board.Groups, 1)
	require.Zero(t, board.Groups[0].LastHour.TotalTokens)
	require.Equal(t, 2, usage.calls)
}
