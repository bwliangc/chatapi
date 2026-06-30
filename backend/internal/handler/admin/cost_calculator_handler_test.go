package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestApplyCostCalculatorBalanceLiabilityValuationUsesMatchedRechargeHistory(t *testing.T) {
	summary := &service.CostCalculatorBalanceLiabilitySummary{TotalBalance: 100}
	rechargeSummary := &service.CostCalculatorBalanceRechargeSummary{
		ActualRevenue:          150,
		MatchedBalanceAmount:   1000,
		UnmatchedBalanceAmount: 50,
	}

	applyCostCalculatorBalanceLiabilityValuation(summary, rechargeSummary, []service.CostCalculatorBalanceRechargePackage{
		{BalanceAmount: 100, ActualAmount: 20},
	})

	require.Equal(t, "matched_recharge_history", summary.ValuationSource)
	require.InDelta(t, 0.15, summary.EstimatedUnitCost, 0.000001)
	require.InDelta(t, 15, summary.EstimatedActualLiability, 0.000001)
	require.InDelta(t, 50, summary.UnmatchedBalanceAmount, 0.000001)
}

func TestApplyCostCalculatorBalanceLiabilityValuationFallsBackToPackageTable(t *testing.T) {
	summary := &service.CostCalculatorBalanceLiabilitySummary{TotalBalance: 200}

	applyCostCalculatorBalanceLiabilityValuation(summary, &service.CostCalculatorBalanceRechargeSummary{}, []service.CostCalculatorBalanceRechargePackage{
		{BalanceAmount: 100, ActualAmount: 15},
		{BalanceAmount: 1000, ActualAmount: 135},
	})

	require.Equal(t, "package_table", summary.ValuationSource)
	require.InDelta(t, 150.0/1100.0, summary.EstimatedUnitCost, 0.000001)
	require.InDelta(t, 200.0*150.0/1100.0, summary.EstimatedActualLiability, 0.000001)
}

func TestApplyCostCalculatorBalanceLiabilityValuationUnavailable(t *testing.T) {
	summary := &service.CostCalculatorBalanceLiabilitySummary{TotalBalance: 200}

	applyCostCalculatorBalanceLiabilityValuation(summary, &service.CostCalculatorBalanceRechargeSummary{}, nil)

	require.Equal(t, "unavailable", summary.ValuationSource)
	require.Zero(t, summary.EstimatedUnitCost)
	require.Zero(t, summary.EstimatedActualLiability)
}

// 当全历史命中套餐（MatchedBalanceAmount > 0）但实收为 0 时，
// 不应采信 matched_recharge_history 而把单价/负债归零，应回退到 package_table 估值。
func TestApplyCostCalculatorBalanceLiabilityValuationFallsBackWhenMatchedRevenueZero(t *testing.T) {
	summary := &service.CostCalculatorBalanceLiabilitySummary{TotalBalance: 200}
	rechargeSummary := &service.CostCalculatorBalanceRechargeSummary{
		ActualRevenue:        0,
		MatchedBalanceAmount: 1000,
	}

	applyCostCalculatorBalanceLiabilityValuation(summary, rechargeSummary, []service.CostCalculatorBalanceRechargePackage{
		{BalanceAmount: 100, ActualAmount: 15},
		{BalanceAmount: 1000, ActualAmount: 135},
	})

	require.Equal(t, "package_table", summary.ValuationSource)
	require.InDelta(t, 150.0/1100.0, summary.EstimatedUnitCost, 0.000001)
	require.InDelta(t, 200.0*150.0/1100.0, summary.EstimatedActualLiability, 0.000001)
}
