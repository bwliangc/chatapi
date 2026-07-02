package handler

import (
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestNewLeaderboardSettlementUsesSnapshotTopN(t *testing.T) {
	settlement := newLeaderboardSettlement([]service.LeaderboardRewardLog{
		{
			UserID:              11,
			Rank:                1,
			RewardAmount:        7.5,
			PoolAmount:          20,
			TotalCost:           200,
			PoolRate:            10,
			TopN:                5,
			MinSpend:            12,
			DistributionMode:    service.LeaderboardRewardModeWeighted,
			DistributionWeights: "50,30,20,0,0",
		},
		{
			UserID:              12,
			Rank:                2,
			RewardAmount:        4.5,
			PoolAmount:          20,
			TotalCost:           200,
			PoolRate:            10,
			TopN:                5,
			MinSpend:            12,
			DistributionMode:    service.LeaderboardRewardModeWeighted,
			DistributionWeights: "50,30,20,0,0",
		},
	})
	if settlement == nil {
		t.Fatal("want settlement")
	}
	if settlement.topN != 5 {
		t.Fatalf("want snapshot topN 5, got %d", settlement.topN)
	}
	if settlement.minSpend != 12 {
		t.Fatalf("want snapshot min spend 12, got %v", settlement.minSpend)
	}
	if settlement.distributionMode != service.LeaderboardRewardModeWeighted {
		t.Fatalf("want weighted mode, got %q", settlement.distributionMode)
	}
	if settlement.distributionWeights != "50,30,20,0,0" {
		t.Fatalf("want snapshot weights, got %q", settlement.distributionWeights)
	}
}

func TestNewLeaderboardSettlementFallsBackToMaxRewardRankForLegacyRows(t *testing.T) {
	settlement := newLeaderboardSettlement([]service.LeaderboardRewardLog{
		{UserID: 21, Rank: 1, RewardAmount: 3, PoolAmount: 9, TotalCost: 90, DistributionMode: service.LeaderboardRewardModeAverage},
		{UserID: 22, Rank: 2, RewardAmount: 3, PoolAmount: 9, TotalCost: 90, DistributionMode: service.LeaderboardRewardModeAverage},
		{UserID: 23, Rank: 3, RewardAmount: 3, PoolAmount: 9, TotalCost: 90, DistributionMode: service.LeaderboardRewardModeAverage},
	})
	if settlement == nil {
		t.Fatal("want settlement")
	}
	if settlement.topN != 3 {
		t.Fatalf("want legacy topN fallback 3, got %d", settlement.topN)
	}
	if math.Abs(settlement.poolRate-10) > 1e-9 {
		t.Fatalf("want pool rate fallback 10, got %v", settlement.poolRate)
	}
	if settlement.rewardsByUser[23] != 3 {
		t.Fatalf("want reward amount for user 23, got %v", settlement.rewardsByUser[23])
	}
}
