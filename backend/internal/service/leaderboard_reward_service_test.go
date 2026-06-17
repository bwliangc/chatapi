package service

import (
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func items(costs ...float64) []usagestats.UserBreakdownItem {
	out := make([]usagestats.UserBreakdownItem, 0, len(costs))
	for i, c := range costs {
		out = append(out, usagestats.UserBreakdownItem{UserID: int64(i + 1), ActualCost: c})
	}
	return out
}

func sumAmounts(items []LeaderboardRewardGrantItem) float64 {
	var s float64
	for _, it := range items {
		s += it.Amount
	}
	return s
}

func TestBuildLeaderboardRewardItems_Average(t *testing.T) {
	got := buildLeaderboardRewardItems(items(40, 30, 20, 10), 4, LeaderboardRewardModeAverage, "", 100)
	if len(got) != 4 {
		t.Fatalf("want 4 winners, got %d", len(got))
	}
	for _, it := range got {
		if math.Abs(it.Amount-25) > 1e-9 {
			t.Errorf("average: want 25, got %v (rank %d)", it.Amount, it.Rank)
		}
	}
	if math.Abs(sumAmounts(got)-100) > 1e-9 {
		t.Errorf("average: pool not conserved, sum=%v", sumAmounts(got))
	}
}

func TestBuildLeaderboardRewardItems_Proportional(t *testing.T) {
	got := buildLeaderboardRewardItems(items(40, 30, 20, 10), 4, LeaderboardRewardModeProportional, "", 100)
	want := []float64{40, 30, 20, 10}
	if len(got) != 4 {
		t.Fatalf("want 4 winners, got %d", len(got))
	}
	for i, it := range got {
		if math.Abs(it.Amount-want[i]) > 1e-9 {
			t.Errorf("proportional rank %d: want %v, got %v", it.Rank, want[i], it.Amount)
		}
	}
	if math.Abs(sumAmounts(got)-100) > 1e-9 {
		t.Errorf("proportional: pool not conserved, sum=%v", sumAmounts(got))
	}
}

func TestBuildLeaderboardRewardItems_Weighted(t *testing.T) {
	got := buildLeaderboardRewardItems(items(40, 30, 20), 3, LeaderboardRewardModeWeighted, "50,30,20", 100)
	want := []float64{50, 30, 20}
	if len(got) != 3 {
		t.Fatalf("want 3 winners, got %d", len(got))
	}
	for i, it := range got {
		if math.Abs(it.Amount-want[i]) > 1e-9 {
			t.Errorf("weighted rank %d: want %v, got %v", it.Rank, want[i], it.Amount)
		}
	}
}

func TestBuildLeaderboardRewardItems_WeightedFewerWeights(t *testing.T) {
	// 只有两档权重，第 3 名权重为 0 → 不发放；总额仍守恒。
	got := buildLeaderboardRewardItems(items(40, 30, 20), 3, LeaderboardRewardModeWeighted, "50,30", 100)
	if len(got) != 2 {
		t.Fatalf("want 2 winners (3rd dropped), got %d", len(got))
	}
	if math.Abs(sumAmounts(got)-100) > 1e-9 {
		t.Errorf("weighted-fewer: pool not conserved, sum=%v", sumAmounts(got))
	}
}

func TestBuildLeaderboardRewardItems_TopNExceedsWinners(t *testing.T) {
	// topN=10 但只有 2 个有消费的用户 → 全奖池在这 2 人间平分。
	got := buildLeaderboardRewardItems(items(40, 10, 0, 0), 10, LeaderboardRewardModeAverage, "", 100)
	if len(got) != 2 {
		t.Fatalf("want 2 winners, got %d", len(got))
	}
	if math.Abs(sumAmounts(got)-100) > 1e-9 {
		t.Errorf("topN-exceeds: pool not conserved, sum=%v", sumAmounts(got))
	}
}

func TestBuildLeaderboardRewardItems_ZeroCostExcluded(t *testing.T) {
	// 昨日无消费的用户不应成为获奖者。
	got := buildLeaderboardRewardItems(items(0, 0), 2, LeaderboardRewardModeAverage, "", 100)
	if len(got) != 0 {
		t.Fatalf("want 0 winners, got %d", len(got))
	}
}

func TestBuildLeaderboardRewardItems_RemainderConserved(t *testing.T) {
	// 3 人平分 100 → 33.333333 * 3 = 99.999999，余数补给第 1 名。
	got := buildLeaderboardRewardItems(items(30, 30, 30), 3, LeaderboardRewardModeAverage, "", 100)
	if len(got) != 3 {
		t.Fatalf("want 3 winners, got %d", len(got))
	}
	if math.Abs(sumAmounts(got)-100) > 1e-9 {
		t.Errorf("remainder: pool not conserved, sum=%v", sumAmounts(got))
	}
	if got[0].Rank != 1 {
		t.Errorf("remainder should go to rank 1, got rank %d first", got[0].Rank)
	}
}

func TestParseLeaderboardRewardWeights(t *testing.T) {
	cases := []struct {
		in   string
		want []float64
	}{
		{"50,30,20", []float64{50, 30, 20}},
		{"50, ,20", []float64{50, 0, 20}},
		{"", nil},
		{"abc,10", []float64{0, 10}},
		{"-5,10", []float64{0, 10}},
	}
	for _, c := range cases {
		got := parseLeaderboardRewardWeights(c.in)
		if len(got) != len(c.want) {
			t.Errorf("parse %q: len want %d got %d (%v)", c.in, len(c.want), len(got), got)
			continue
		}
		for i := range got {
			if math.Abs(got[i]-c.want[i]) > 1e-9 {
				t.Errorf("parse %q[%d]: want %v got %v", c.in, i, c.want[i], got[i])
			}
		}
	}
}
