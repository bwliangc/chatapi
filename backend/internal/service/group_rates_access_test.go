//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGroupRatesBoardUsesExistingGroupAuthorization(t *testing.T) {
	user := &User{ID: 1, AllowedGroups: []int64{2}, RestrictPublicGroups: true}
	provider := &APIKeyService{
		userRepo: &visibilityUserRepo{user: user},
		groupRepo: &visibilityGroupRepo{groups: []Group{
			{ID: 1, Status: StatusActive},
			{ID: 2, Status: StatusActive, IsExclusive: true},
			{ID: 3, Status: StatusActive, IsExclusive: true, SubscriptionType: SubscriptionTypeSubscription},
			{ID: 4, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
			{ID: 5, Status: "inactive"},
		}},
		userSubRepo: &visibilitySubRepo{subscriptions: []UserSubscription{
			{UserID: 1, GroupID: 3, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)},
			{UserID: 1, GroupID: 4, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(-time.Hour)},
		}},
	}
	usage := &rateBoardUsageStub{}
	svc := NewGroupRatesService(provider, usage)
	board, err := svc.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, board.Groups, 2)
	require.Equal(t, []int64{2, 3}, usage.ids)
	user.RestrictPublicGroups = false
	board, err = svc.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, board.Groups, 3)
	require.Equal(t, []int64{1, 2, 3}, usage.ids)
}
