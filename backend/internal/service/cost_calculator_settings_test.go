package service

import (
	"strings"
	"testing"
	"time"
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

func TestVersionCostCalculatorAccountCostsArchivesChangedValues(t *testing.T) {
	oldRate := 5.4
	newRate := 6.2
	effectiveAt := time.Date(2026, 7, 10, 8, 30, 0, 0, time.UTC)
	current := CostCalculatorConfig{
		AccountCosts: []CostCalculatorAccountCost{{
			AccountID:         12,
			AccountName:       "upstream-a",
			MonthlyCost:       300,
			UsageCostRate:     &oldRate,
			FixedCostStartsAt: "2026-07-01",
		}},
	}
	next := CostCalculatorConfig{
		AccountCosts: []CostCalculatorAccountCost{{
			AccountID:         12,
			AccountName:       "upstream-a",
			MonthlyCost:       450,
			UsageCostRate:     &newRate,
			FixedCostStartsAt: "2026-07-01",
		}},
	}

	versionCostCalculatorAccountCosts(current, &next, effectiveAt)

	if len(next.AccountCostHistory) != 1 {
		t.Fatalf("history length = %d, want 1", len(next.AccountCostHistory))
	}
	history := next.AccountCostHistory[0]
	if history.MonthlyCost != 300 || history.UsageCostRate == nil || *history.UsageCostRate != oldRate {
		t.Fatalf("unexpected archived cost: %+v", history)
	}
	if history.EffectiveFrom != "" {
		t.Fatalf("legacy history effective_from = %q, want empty", history.EffectiveFrom)
	}
	if history.EffectiveUntil != effectiveAt.Format(time.RFC3339Nano) {
		t.Fatalf("history effective_until = %q", history.EffectiveUntil)
	}
	if got := next.AccountCosts[0].EffectiveFrom; got != effectiveAt.Format(time.RFC3339Nano) {
		t.Fatalf("new effective_from = %q", got)
	}
	if next.AccountCosts[0].EffectiveUntil != "" {
		t.Fatalf("new effective_until = %q, want empty", next.AccountCosts[0].EffectiveUntil)
	}
}

func TestVersionCostCalculatorAccountCostsDoesNotDuplicateUnchangedValues(t *testing.T) {
	rate := 5.4
	originalEffectiveFrom := "2026-07-01T00:00:00Z"
	current := CostCalculatorConfig{
		AccountCosts: []CostCalculatorAccountCost{{
			AccountID:     12,
			MonthlyCost:   300,
			UsageCostRate: &rate,
			EffectiveFrom: originalEffectiveFrom,
		}},
	}
	next := CostCalculatorConfig{
		AccountCosts: []CostCalculatorAccountCost{{
			AccountID:     12,
			MonthlyCost:   300,
			UsageCostRate: &rate,
		}},
	}

	versionCostCalculatorAccountCosts(current, &next, time.Date(2026, 7, 10, 8, 30, 0, 0, time.UTC))

	if len(next.AccountCostHistory) != 0 {
		t.Fatalf("history length = %d, want 0", len(next.AccountCostHistory))
	}
	if got := next.AccountCosts[0].EffectiveFrom; got != originalEffectiveFrom {
		t.Fatalf("effective_from = %q, want %q", got, originalEffectiveFrom)
	}
}

func TestVersionCostCalculatorAccountCostsArchivesRemovedAccount(t *testing.T) {
	rate := 5.4
	effectiveAt := time.Date(2026, 7, 10, 8, 30, 0, 0, time.UTC)
	current := CostCalculatorConfig{
		AccountCosts: []CostCalculatorAccountCost{{
			AccountID:     12,
			MonthlyCost:   300,
			UsageCostRate: &rate,
		}},
	}
	next := CostCalculatorConfig{AccountCosts: []CostCalculatorAccountCost{}}

	versionCostCalculatorAccountCosts(current, &next, effectiveAt)

	if len(next.AccountCostHistory) != 1 {
		t.Fatalf("history length = %d, want 1", len(next.AccountCostHistory))
	}
	if got := next.AccountCostHistory[0].EffectiveUntil; got != effectiveAt.Format(time.RFC3339Nano) {
		t.Fatalf("effective_until = %q", got)
	}
}

func TestCostCalculatorAccountUsageRatePeriodsKeepsVersionBounds(t *testing.T) {
	oldRate := 5.4
	newRate := 6.2
	cfg := CostCalculatorConfig{
		AccountCostHistory: []CostCalculatorAccountCost{{
			AccountID:      12,
			UsageCostRate:  &oldRate,
			EffectiveUntil: "2026-07-10T08:30:00Z",
		}},
		AccountCosts: []CostCalculatorAccountCost{{
			AccountID:     12,
			UsageCostRate: &newRate,
			EffectiveFrom: "2026-07-10T08:30:00Z",
		}},
	}

	periods := cfg.AccountUsageRatePeriods()
	if len(periods) != 2 {
		t.Fatalf("periods length = %d, want 2", len(periods))
	}
	if periods[0].EffectiveFrom != nil || periods[0].EffectiveUntil == nil || periods[0].UsageCostRate != oldRate {
		t.Fatalf("unexpected old period: %+v", periods[0])
	}
	if periods[1].EffectiveFrom == nil || periods[1].EffectiveUntil != nil || periods[1].UsageCostRate != newRate {
		t.Fatalf("unexpected current period: %+v", periods[1])
	}
}
