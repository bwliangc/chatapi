//go:build integration

package repository

import (
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *AccountRepoSuite) TestList_DefaultSortByNameAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "z-account"})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "a-account"})

	accounts, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("a-account", accounts[0].Name)
	s.Require().Equal("z-account", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByPriorityDesc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "low-priority", Priority: 10})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "high-priority", Priority: 90})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "priority",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("high-priority", accounts[0].Name)
	s.Require().Equal("low-priority", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByUpstreamBillingRateWithMissingLast() {
	makeAccount := func(name, status string, rate any) {
		extra := map[string]any{}
		if rate != nil {
			extra[service.UpstreamBillingProbeExtraKey] = map[string]any{
				"status": status,
				"data":   map[string]any{"effective_rate_multiplier": rate},
			}
		}
		mustCreateAccount(s.T(), s.client, &service.Account{Name: name, Extra: extra})
	}
	makeAccount("high-rate", service.UpstreamBillingProbeStatusOK, 0.8)
	makeAccount("low-rate", service.UpstreamBillingProbeStatusOK, 0.03)
	makeAccount("missing-rate", "", nil)
	makeAccount("unsupported-with-retained-rate", service.UpstreamBillingProbeStatusUnsupported, 0.01)

	for _, tc := range []struct {
		order string
		want  []string
	}{
		{order: "asc", want: []string{"low-rate", "high-rate", "missing-rate", "unsupported-with-retained-rate"}},
		{order: "desc", want: []string{"high-rate", "low-rate", "unsupported-with-retained-rate", "missing-rate"}},
	} {
		accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
			Page: 1, PageSize: 10, SortBy: "upstream_billing_rate", SortOrder: tc.order,
		}, "", "", "", "", 0, "")
		s.Require().NoError(err)
		s.Require().Len(accounts, 4)
		for i, name := range tc.want {
			s.Require().Equal(name, accounts[i].Name)
		}
	}
}

func (s *AccountRepoSuite) TestListWithFilters_SortByCurrentUpstreamBillingRateDuringPeak() {
	now := time.Now()
	locations := []string{"UTC", "Asia/Shanghai", "America/New_York", "Europe/London"}
	var timezone string
	var minute int
	for _, name := range locations {
		location, err := time.LoadLocation(name)
		s.Require().NoError(err)
		local := now.In(location)
		candidate := local.Hour()*60 + local.Minute()
		if candidate >= 2 && candidate <= 1436 {
			timezone = name
			minute = candidate
			break
		}
	}
	s.Require().NotEmpty(timezone)

	peakStart := fmt.Sprintf("%02d:%02d", (minute-2)/60, (minute-2)%60)
	peakEnd := fmt.Sprintf("%02d:%02d", (minute+3)/60, (minute+3)%60)
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "current-peak-rate",
		Extra: map[string]any{
			service.UpstreamBillingProbeExtraKey: map[string]any{
				"status": service.UpstreamBillingProbeStatusOK,
				"data": map[string]any{
					"billing_scope":             "token",
					"resolved_rate_multiplier":  1.0,
					"effective_rate_multiplier": 1.0,
					"peak_rate_enabled":         true,
					"peak_start":                peakStart,
					"peak_end":                  peakEnd,
					"peak_rate_multiplier":      10.0,
					"timezone":                  timezone,
				},
			},
		},
	})
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name: "current-off-peak-rate",
		Extra: map[string]any{
			service.UpstreamBillingProbeExtraKey: map[string]any{
				"status": service.UpstreamBillingProbeStatusOK,
				"data": map[string]any{
					"effective_rate_multiplier": 5.0,
				},
			},
		},
	})

	for _, tc := range []struct {
		order string
		want  []string
	}{
		{order: "asc", want: []string{"current-off-peak-rate", "current-peak-rate"}},
		{order: "desc", want: []string{"current-peak-rate", "current-off-peak-rate"}},
	} {
		accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
			Page: 1, PageSize: 10, SortBy: "upstream_billing_rate", SortOrder: tc.order,
		}, "", "", "", "", 0, "")
		s.Require().NoError(err)
		s.Require().Len(accounts, 2)
		for i, name := range tc.want {
			s.Require().Equal(name, accounts[i].Name)
		}
	}
}

func (s *AccountRepoSuite) TestListWithFilters_SortByDisplayedStatusAcrossPages() {
	now := time.Now()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	create := func(name, status string, rate, overload, temp *time.Time, paused bool, extra map[string]any) {
		a := mustCreateAccount(s.T(), s.client, &service.Account{Name: name, Status: status, Type: service.AccountTypeAPIKey, Extra: extra, RateLimitResetAt: rate, OverloadUntil: overload})
		update := s.client.Account.UpdateOneID(a.ID).SetSchedulable(!paused)
		if temp != nil {
			update.SetTempUnschedulableUntil(*temp)
		}
		s.Require().NoError(update.Exec(s.ctx))
	}
	// Interleave stored active rows and cooldowns so raw status/ID sorting fails.
	create("rate-limited", "active", &future, nil, nil, false, nil)
	create("healthy-a", "active", nil, nil, nil, false, nil)
	create("error", "error", nil, nil, &future, false, nil)
	create("quota", "active", nil, nil, nil, false, map[string]any{"quota_limit": 10, "quota_used": 10})
	create("inactive", "inactive", nil, nil, nil, false, nil)
	create("paused", "active", nil, nil, nil, true, nil)
	create("overloaded", "active", nil, &future, nil, false, nil)
	create("temporary", "active", nil, nil, &future, false, nil)
	create("healthy-expired", "active", &past, &past, &past, false, map[string]any{"quota_daily_limit": 10, "quota_daily_used": 10, "quota_daily_start": now.Add(-48 * time.Hour).Format(time.RFC3339)})
	create("daily-quota", "active", nil, nil, nil, false, map[string]any{"quota_daily_limit": "10", "quota_daily_used": 10, "quota_daily_start": now.Format(time.RFC3339), "ignored_payload": "not needed for sorting"})
	create("rate-limited-error", "error", &future, &future, &future, false, nil)
	wantAsc := []string{"healthy-a", "healthy-expired", "inactive", "paused", "quota", "daily-quota", "rate-limited", "rate-limited-error", "overloaded", "temporary", "error"}
	for _, order := range []string{"asc", "desc"} {
		want := append([]string(nil), wantAsc...)
		if order == "desc" {
			for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
				want[i], want[j] = want[j], want[i]
			}
		}
		var got []string
		for page := 1; page <= 4; page++ {
			accounts, total, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: page, PageSize: 3, SortBy: "status", SortOrder: order}, "", "", "", "", 0, "")
			s.Require().NoError(err)
			s.Require().EqualValues(len(want), total.Total)
			for _, a := range accounts {
				got = append(got, a.Name)
			}
		}
		s.Require().Equal(want, got, order)
	}
	accounts, total, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 3, SortBy: "status"}, "", "", "", "healthy", 0, "")
	s.Require().NoError(err)
	s.Require().EqualValues(2, total.Total)
	s.Require().Equal("healthy-a", accounts[0].Name)
	s.Require().Equal("healthy-expired", accounts[1].Name)
	accounts, _, err = s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 99, PageSize: 3, SortBy: "status"}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Empty(accounts)
}
