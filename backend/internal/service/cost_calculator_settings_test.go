package service

import (
	"strings"
	"testing"
)

func TestNormalizeCostCalculatorConfigKeepsFixedCostDateRange(t *testing.T) {
	cfg, err := normalizeCostCalculatorConfig(CostCalculatorConfig{
		BalanceExchangeRate: 1,
		UpstreamCostRate:    1,
		AccountCosts: []CostCalculatorAccountCost{
			{
				AccountID:         1,
				MonthlyCost:       30,
				FixedCostStartsAt: "2026-07-01",
				FixedCostEndsAt:   "2026-07-31",
			},
		},
	})
	if err != nil {
		t.Fatalf("normalize config: %v", err)
	}
	if got := cfg.AccountCosts[0].FixedCostStartsAt; got != "2026-07-01" {
		t.Fatalf("FixedCostStartsAt = %q", got)
	}
	if got := cfg.AccountCosts[0].FixedCostEndsAt; got != "2026-07-31" {
		t.Fatalf("FixedCostEndsAt = %q", got)
	}
}

func TestNormalizeCostCalculatorConfigRejectsInvalidFixedCostDateRange(t *testing.T) {
	_, err := normalizeCostCalculatorConfig(CostCalculatorConfig{
		BalanceExchangeRate: 1,
		UpstreamCostRate:    1,
		AccountCosts: []CostCalculatorAccountCost{
			{
				AccountID:         1,
				MonthlyCost:       30,
				FixedCostStartsAt: "2026-08-01",
				FixedCostEndsAt:   "2026-07-31",
			},
		},
	})
	if err == nil {
		t.Fatal("expected invalid date range error")
	}
	if !strings.Contains(err.Error(), "fixed_cost_ends_at") {
		t.Fatalf("unexpected error: %v", err)
	}
}
