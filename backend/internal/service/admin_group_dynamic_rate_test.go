//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminGroupDynamicRateCreateAndUpdate(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 1}
	svc := &adminServiceImpl{groupRepo: repo}
	ctx := context.Background()
	cfg := GroupDynamicRate{Enabled: true, Min: .5, Max: 2}
	created, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "dynamic", Platform: PlatformOpenAI, RateMultiplier: 3, DynamicRate: cfg})
	require.NoError(t, err)
	require.Equal(t, 2.0, created.RateMultiplier)
	require.Equal(t, cfg, created.DynamicRate)
	repo.getByID = created
	updated, err := svc.UpdateGroup(ctx, 1, &UpdateGroupInput{Name: "renamed"})
	require.NoError(t, err)
	require.Equal(t, cfg, updated.DynamicRate, "unrelated edits preserve dynamic configuration")
	cfg.Max = 1
	updated, err = svc.UpdateGroup(ctx, 1, &UpdateGroupInput{DynamicRate: &cfg})
	require.NoError(t, err)
	require.Equal(t, 1.0, updated.RateMultiplier, "changing bounds clamps the currently published rate")
	disabled := GroupDynamicRate{}
	updated, err = svc.UpdateGroup(ctx, 1, &UpdateGroupInput{DynamicRate: &disabled})
	require.NoError(t, err)
	require.False(t, updated.DynamicRate.Enabled)
	require.Equal(t, 1.0, updated.RateMultiplier)
	cfg.Min, cfg.Max = 2, 1
	_, err = svc.UpdateGroup(ctx, 1, &UpdateGroupInput{DynamicRate: &cfg})
	require.Error(t, err)
	_, err = svc.CreateGroup(ctx, &CreateGroupInput{Name: "invalid", Platform: PlatformOpenAI, DynamicRate: cfg})
	require.Error(t, err)
}
