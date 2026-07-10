package repository

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildCostCalculatorUsageCostExpressionUsesVersionTimeBounds(t *testing.T) {
	effectiveAt := time.Date(2026, 7, 10, 8, 30, 0, 0, time.UTC)
	periods := []service.CostCalculatorUsageRatePeriod{
		{AccountID: 12, UsageCostRate: 6.2, EffectiveFrom: &effectiveAt},
		{AccountID: 12, UsageCostRate: 5.4, EffectiveUntil: &effectiveAt},
	}

	expression, args := buildCostCalculatorUsageCostExpression("ul", []any{"start", "end"}, periods, 4.8)

	for _, part := range []string{
		"ul.account_id = $4::bigint AND ul.created_at < $5::timestamptz THEN $6::numeric",
		"ul.account_id = $7::bigint AND ul.created_at >= $8::timestamptz THEN $9::numeric",
		"ELSE $3::numeric",
	} {
		if !strings.Contains(expression, part) {
			t.Fatalf("expression %q does not contain %q", expression, part)
		}
	}
	wantArgs := []any{"start", "end", 4.8, int64(12), effectiveAt, 5.4, int64(12), effectiveAt, 6.2}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildCostCalculatorUsageCostExpressionFallsBackToDefaultRate(t *testing.T) {
	expression, args := buildCostCalculatorUsageCostExpression("ul", nil, nil, 4.8)

	if expression != "COALESCE(ul.account_stats_cost, ul.total_cost) * ($1::numeric)" {
		t.Fatalf("expression = %q", expression)
	}
	if !reflect.DeepEqual(args, []any{4.8}) {
		t.Fatalf("args = %#v", args)
	}
}
