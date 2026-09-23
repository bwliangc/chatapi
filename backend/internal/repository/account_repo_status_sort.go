package repository

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Status is a display value, not just accounts.status: cooldowns, pause state
// and quota windows override the stored status. Project only status inputs,
// rank the whole filtered set, then hydrate the requested page. Reusing the Go
// quota window rules avoids a second implementation of timezone/DST handling in SQL.
func (r *accountRepository) listAccountsByDisplayStatus(ctx context.Context, q *dbent.AccountQuery, params pagination.PaginationParams) ([]service.Account, *pagination.PaginationResult, error) {
	var rows []struct {
		ID                     int64           `json:"id"`
		Status                 string          `json:"status"`
		Type                   string          `json:"type"`
		Schedulable            bool            `json:"schedulable"`
		RateLimitResetAt       *time.Time      `json:"rate_limit_reset_at"`
		OverloadUntil          *time.Time      `json:"overload_until"`
		TempUnschedulableUntil *time.Time      `json:"temp_unschedulable_until"`
		Quota                  json.RawMessage `json:"quota"`
	}
	err := q.Clone().Select(dbaccount.FieldID, dbaccount.FieldStatus, dbaccount.FieldType, dbaccount.FieldSchedulable,
		dbaccount.FieldRateLimitResetAt, dbaccount.FieldOverloadUntil, dbaccount.FieldTempUnschedulableUntil).
		Aggregate(func(s *entsql.Selector) string {
			extra := s.C(dbaccount.FieldExtra)
			// Do not retrieve credentials or unrelated Extra payloads for off-page rows.
			return "COALESCE((SELECT jsonb_object_agg(key, value) FROM jsonb_each(CASE WHEN jsonb_typeof(" + extra + ") = 'object' THEN " + extra + " ELSE '{}'::jsonb END) WHERE key IN (" +
				"'quota_limit','quota_used','quota_daily_limit','quota_daily_used','quota_weekly_limit','quota_weekly_used'," +
				"'quota_daily_start','quota_weekly_start','quota_daily_reset_mode','quota_daily_reset_hour'," +
				"'quota_weekly_reset_mode','quota_weekly_reset_day','quota_weekly_reset_hour','quota_reset_timezone'" +
				")), '{}'::jsonb) AS quota"
		}).Scan(ctx, &rows)
	if err != nil {
		return nil, nil, err
	}
	type rankedAccount struct {
		id     int64
		rank   int
		status string
	}
	ranked := make([]rankedAccount, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		a := service.Account{ID: row.ID, Status: row.Status, Type: row.Type, Schedulable: row.Schedulable,
			RateLimitResetAt: row.RateLimitResetAt, OverloadUntil: row.OverloadUntil, TempUnschedulableUntil: row.TempUnschedulableUntil}
		if err := json.Unmarshal(row.Quota, &a.Extra); err != nil {
			return nil, nil, err
		}
		ranked = append(ranked, rankedAccount{id: a.ID, rank: accountDisplayStatusRank(&a, now), status: a.Status})
	}
	desc := params.NormalizedSortOrder(pagination.SortOrderAsc) == pagination.SortOrderDesc
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if desc {
			a, b = b, a
		}
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		// Unknown stored statuses remain separate groups rather than interleaving.
		if a.rank == 8 && a.status != b.status {
			return a.status < b.status
		}
		return a.id < b.id
	})
	result := paginationResultFromTotal(int64(len(ranked)), params)
	start := params.Offset()
	if start >= len(ranked) {
		return []service.Account{}, result, nil
	}
	end := min(start+params.Limit(), len(ranked))
	ids := make([]int64, 0, end-start)
	positions := make(map[int64]int, end-start)
	for _, a := range ranked[start:end] {
		positions[a.id] = len(ids)
		ids = append(ids, a.id)
	}
	entities, err := q.Clone().Where(dbaccount.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	accounts, err := r.accountsToService(ctx, entities)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(accounts, func(i, j int) bool { return positions[accounts[i].ID] < positions[accounts[j].ID] })
	return accounts, result, nil
}

// Match AccountStatusIndicator's main badge precedence exactly. Ascending puts
// healthy accounts first; descending puts errors first. Model-specific badges
// are supplementary and do not override the main account status.
func accountDisplayStatusRank(a *service.Account, now time.Time) int {
	if a.RateLimitResetAt != nil && a.RateLimitResetAt.After(now) {
		return 4
	}
	if a.OverloadUntil != nil && a.OverloadUntil.After(now) {
		return 5
	}
	if a.Status == service.StatusError {
		return 7
	}
	if a.TempUnschedulableUntil != nil && a.TempUnschedulableUntil.After(now) {
		return 6
	}
	if a.Status != service.StatusActive {
		if a.Status == "inactive" {
			return 1
		}
		return 8
	}
	if a.IsAPIKeyOrBedrock() {
		exceeded := func(used, limit float64) bool { return limit > 0 && used >= limit }
		if exceeded(a.GetQuotaUsed(), a.GetQuotaLimit()) ||
			(!a.IsDailyQuotaPeriodExpired() && exceeded(a.GetQuotaDailyUsed(), a.GetQuotaDailyLimit())) ||
			(!a.IsWeeklyQuotaPeriodExpired() && exceeded(a.GetQuotaWeeklyUsed(), a.GetQuotaWeeklyLimit())) {
			return 3
		}
	}
	if !a.Schedulable {
		return 2
	}
	return 0
}

func isAccountDisplayStatusSort(params pagination.PaginationParams) bool {
	return strings.EqualFold(strings.TrimSpace(params.SortBy), "status")
}
