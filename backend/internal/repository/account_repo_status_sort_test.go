package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountDisplayStatusRank(t *testing.T) {
	now := time.Now()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	for _, tc := range []struct {
		name    string
		account service.Account
		want    int
	}{
		{"normal", service.Account{Status: "active", Schedulable: true}, 0},
		{"paused", service.Account{Status: "active"}, 2},
		{"inactive", service.Account{Status: "inactive"}, 1},
		{"error beats temporary cooldown", service.Account{Status: "error", TempUnschedulableUntil: &future}, 7},
		{"rate limit beats error and overload", service.Account{Status: "error", RateLimitResetAt: &future, OverloadUntil: &future}, 4},
		{"overload beats error", service.Account{Status: "error", OverloadUntil: &future}, 5},
		{"temporary cooldown beats inactive", service.Account{Status: "inactive", TempUnschedulableUntil: &future}, 6},
		{"expired cooldowns", service.Account{Status: "active", Schedulable: true, RateLimitResetAt: &past, OverloadUntil: &past, TempUnschedulableUntil: &past}, 0},
		{"total quota beats pause", service.Account{Status: "active", Type: "apikey", Extra: map[string]any{"quota_limit": "10", "quota_used": 10}}, 3},
		{"oauth does not display API key quota", service.Account{Status: "active", Type: "oauth", Schedulable: true, Extra: map[string]any{"quota_limit": 10, "quota_used": 10}}, 0},
		{"daily quota", service.Account{Status: "active", Type: "bedrock", Extra: map[string]any{"quota_daily_limit": 10, "quota_daily_used": 10, "quota_daily_start": now.Format(time.RFC3339)}}, 3},
		{"expired daily quota", service.Account{Status: "active", Type: "apikey", Schedulable: true, Extra: map[string]any{"quota_daily_limit": 10, "quota_daily_used": 10, "quota_daily_start": now.Add(-48 * time.Hour).Format(time.RFC3339)}}, 0},
		{"weekly quota", service.Account{Status: "active", Type: "apikey", Extra: map[string]any{"quota_weekly_limit": 10, "quota_weekly_used": 10, "quota_weekly_start": now.Format(time.RFC3339)}}, 3},
		{"expired fixed quota", service.Account{Status: "active", Type: "apikey", Schedulable: true, Extra: map[string]any{"quota_weekly_limit": 10, "quota_weekly_used": 10, "quota_weekly_start": now.Add(-14 * 24 * time.Hour).Format(time.RFC3339), "quota_weekly_reset_mode": "fixed", "quota_reset_timezone": "Asia/Shanghai"}}, 0},
		{"invalid quota", service.Account{Status: "active", Type: "apikey", Schedulable: true, Extra: map[string]any{"quota_limit": "bad", "quota_used": 100}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, accountDisplayStatusRank(&tc.account, now)) })
	}
}
