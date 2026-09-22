package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupDynamicRateDemandAndBounds(t *testing.T) {
	cfg := GroupDynamicRate{Enabled: true, Min: .5, Max: 2}
	history := make([]float64, 100)
	for i := range history {
		history[i] = float64(i)
	}
	require.Equal(t, .875, CalculateGroupDynamicRate(cfg, 1, 0, history))
	require.Equal(t, 1.25, CalculateGroupDynamicRate(cfg, 1, 1000, history))
	last := cfg.Min
	for current := 0.0; current <= 200; current++ {
		rate := CalculateGroupDynamicRate(cfg, 1, current, history)
		require.GreaterOrEqual(t, rate, last, "demand must never lower the rate")
		require.GreaterOrEqual(t, rate, cfg.Min)
		require.LessOrEqual(t, rate, cfg.Max)
		last = rate
	}
	require.Equal(t, float64(0), history[0], "calculation must not change historical data")
	require.Equal(t, .5, CalculateGroupDynamicRate(cfg, 0, 0, nil))
	require.Equal(t, 2.0, CalculateGroupDynamicRate(cfg, 9, 1000, nil))
	require.Equal(t, 1.0, CalculateGroupDynamicRate(cfg, 1, 1000, history[:23]))
	cfg.Enabled = false
	require.Equal(t, 9.0, CalculateGroupDynamicRate(cfg, 9, 1000, history))
}

func TestGroupDynamicRateSparseAndConstantHistory(t *testing.T) {
	cfg := GroupDynamicRate{Enabled: true, Min: .5, Max: 1.5}
	history := make([]float64, 168)
	require.Equal(t, .875, CalculateGroupDynamicRate(cfg, 1, 0, history))
	require.Equal(t, 1.125, CalculateGroupDynamicRate(cfg, 1, 100, history))
	for i := range history {
		history[i] = 100
	}
	require.Equal(t, 1.0, CalculateGroupDynamicRate(cfg, 1, 100, history), "constant demand should settle at the midpoint")
	require.Equal(t, 1.125, CalculateGroupDynamicRate(cfg, 1, 200, history))
	for i := range history {
		history[i] = 0
	}
	history[0] = 1000
	require.Greater(t, CalculateGroupDynamicRate(cfg, 1, 1000, history), CalculateGroupDynamicRate(cfg, 1, 10, history))
}

func TestNormalizeGroupDynamicRate(t *testing.T) {
	for _, cfg := range []GroupDynamicRate{
		{Enabled: true, Min: -1, Max: 2},
		{Enabled: true, Min: 2, Max: 1},
		{Enabled: true, Min: math.NaN(), Max: 2},
		{Enabled: true, Min: 0, Max: math.Inf(1)},
		{Enabled: true, Min: 0, Max: 1e6},
		{Enabled: true, Min: .50001, Max: 1},
	} {
		_, err := NormalizeGroupDynamicRate(cfg)
		require.Error(t, err)
	}
	cfg, err := NormalizeGroupDynamicRate(GroupDynamicRate{Enabled: true, Min: 0, Max: 0})
	require.NoError(t, err)
	require.True(t, cfg.Enabled, "zero is a valid free multiplier")
	cfg, err = NormalizeGroupDynamicRate(GroupDynamicRate{Min: -1})
	require.NoError(t, err)
	require.Equal(t, GroupDynamicRate{}, cfg)
}

func TestUserGroupRateResolverDynamicDefaultAndZeroOverride(t *testing.T) {
	repo := &userGroupRateResolverRepoStub{}
	resolver := newUserGroupRateResolver(repo, nil, 0, nil, "")
	ctx := context.Background()
	require.Equal(t, 1.0, resolver.Resolve(ctx, 1, 2, 1))
	require.Equal(t, 1.25, resolver.Resolve(ctx, 1, 2, 1.25))
	require.Equal(t, 1, repo.calls, "a cached absence must use the latest group rate")
	zero := 0.0
	repo.rate = &zero
	resolver.cache.Flush()
	require.Zero(t, resolver.Resolve(ctx, 1, 2, 1))
	require.Zero(t, resolver.Resolve(ctx, 1, 2, 2), "free user overrides remain free after group rate changes")
}
