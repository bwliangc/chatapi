package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// --- mock repo ---

type userBreakdownRepoCapture struct {
	service.UsageLogRepository
	capturedDim   usagestats.UserBreakdownDimension
	capturedLimit int
	result        []usagestats.UserBreakdownItem
}

func (r *userBreakdownRepoCapture) GetUserBreakdownStats(
	_ context.Context, _, _ time.Time,
	dim usagestats.UserBreakdownDimension, limit int,
) ([]usagestats.UserBreakdownItem, error) {
	r.capturedDim = dim
	r.capturedLimit = limit
	if r.result != nil {
		return r.result, nil
	}
	return []usagestats.UserBreakdownItem{}, nil
}

func newUserBreakdownRouter(repo *userBreakdownRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewDashboardService(repo, nil, nil, nil)
	h := NewDashboardHandler(svc, nil)
	router := gin.New()
	router.GET("/admin/dashboard/user-breakdown", h.GetUserBreakdown)
	return router
}

// --- tests ---

func TestGetUserBreakdown_GroupIDFilter(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&group_id=42", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(42), repo.capturedDim.GroupID)
	require.Empty(t, repo.capturedDim.Model)
	require.Empty(t, repo.capturedDim.Endpoint)
	require.Equal(t, 50, repo.capturedLimit) // default limit
	require.Equal(t, "actual_cost", repo.capturedDim.SortBy)
}

func TestGetUserBreakdown_SortBy(t *testing.T) {
	tests := []struct {
		name       string
		sortBy     string
		wantStatus int
		wantSortBy string
		wantError  string
	}{
		{name: "default", wantStatus: http.StatusOK, wantSortBy: "actual_cost"},
		{name: "actual cost", sortBy: "actual_cost", wantStatus: http.StatusOK, wantSortBy: "actual_cost"},
		{name: "tokens", sortBy: "tokens", wantStatus: http.StatusOK, wantSortBy: "tokens"},
		{name: "total tokens alias", sortBy: "total_tokens", wantStatus: http.StatusOK, wantSortBy: "tokens"},
		{name: "input tokens", sortBy: "input_tokens", wantStatus: http.StatusOK, wantSortBy: "input_tokens"},
		{name: "output tokens", sortBy: "output_tokens", wantStatus: http.StatusOK, wantSortBy: "output_tokens"},
		{name: "cache tokens", sortBy: "cache_tokens", wantStatus: http.StatusOK, wantSortBy: "cache_tokens"},
		{name: "requests", sortBy: "requests", wantStatus: http.StatusOK, wantSortBy: "requests"},
		{name: "cost", sortBy: "cost", wantStatus: http.StatusOK, wantSortBy: "cost"},
		{name: "account cost", sortBy: "account_cost", wantStatus: http.StatusOK, wantSortBy: "account_cost"},
		{
			name:       "invalid",
			sortBy:     "unknown",
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid sort_by, use actual_cost/tokens/input_tokens/output_tokens/cache_tokens/requests/cost/account_cost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &userBreakdownRepoCapture{}
			router := newUserBreakdownRouter(repo)
			path := "/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16"
			if tt.sortBy != "" {
				path += "&sort_by=" + tt.sortBy
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
			require.Equal(t, tt.wantSortBy, repo.capturedDim.SortBy)
			if tt.wantError != "" {
				require.Contains(t, w.Body.String(), tt.wantError)
			}
		})
	}
}

func TestGetUserBreakdown_ModelFilter(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&model=claude-opus-4-6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "claude-opus-4-6", repo.capturedDim.Model)
	require.Equal(t, usagestats.ModelSourceRequested, repo.capturedDim.ModelType)
	require.Equal(t, int64(0), repo.capturedDim.GroupID)
}

func TestGetUserBreakdown_ModelSourceFilter(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&model=claude-opus-4-6&model_source=upstream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, usagestats.ModelSourceUpstream, repo.capturedDim.ModelType)
}

func TestGetUserBreakdown_InvalidModelSource(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&model_source=foobar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetUserBreakdown_EndpointFilter(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&endpoint=/v1/messages&endpoint_type=upstream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "/v1/messages", repo.capturedDim.Endpoint)
	require.Equal(t, "upstream", repo.capturedDim.EndpointType)
}

func TestGetUserBreakdown_DefaultEndpointType(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&endpoint=/chat", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "inbound", repo.capturedDim.EndpointType)
}

func TestGetUserBreakdown_CustomLimit(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&model=test&limit=100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 100, repo.capturedLimit)
}

func TestGetUserBreakdown_LimitClamped(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	// limit > 200 should fall back to default 50
	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&model=test&limit=999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 50, repo.capturedLimit)
}

func TestGetUserBreakdown_ResponseFormat(t *testing.T) {
	repo := &userBreakdownRepoCapture{
		result: []usagestats.UserBreakdownItem{
			{UserID: 1, Email: "alice@test.com", Requests: 100, TotalTokens: 50000, Cost: 1.5, ActualCost: 1.2},
			{UserID: 2, Email: "bob@test.com", Requests: 50, TotalTokens: 25000, Cost: 0.8, ActualCost: 0.6},
		},
	}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&group_id=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Users     []usagestats.UserBreakdownItem `json:"users"`
			StartDate string                         `json:"start_date"`
			EndDate   string                         `json:"end_date"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Users, 2)
	require.Equal(t, int64(1), resp.Data.Users[0].UserID)
	require.Equal(t, "alice@test.com", resp.Data.Users[0].Email)
	require.Equal(t, int64(100), resp.Data.Users[0].Requests)
	require.InDelta(t, 1.2, resp.Data.Users[0].ActualCost, 0.001)
	require.Equal(t, "2026-03-01", resp.Data.StartDate)
	require.Equal(t, "2026-03-16", resp.Data.EndDate)
}

func TestGetUserBreakdown_EmptyResult(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&group_id=999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Users []usagestats.UserBreakdownItem `json:"users"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Empty(t, resp.Data.Users)
}

func TestGetUserBreakdown_NoFilters(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(0), repo.capturedDim.GroupID)
	require.Empty(t, repo.capturedDim.Model)
	require.Empty(t, repo.capturedDim.Endpoint)
}

func TestGetUserBreakdown_RequestTypeFilter(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  int16
	}{
		{"ws_v2", "ws_v2", int16(service.RequestTypeWSV2)},
		{"stream", "stream", int16(service.RequestTypeStream)},
		{"sync", "sync", int16(service.RequestTypeSync)},
		{"numeric compatibility", "3", int16(service.RequestTypeWSV2)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &userBreakdownRepoCapture{}
			router := newUserBreakdownRouter(repo)

			req := httptest.NewRequest(http.MethodGet,
				"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&request_type="+tc.value, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.NotNil(t, repo.capturedDim.RequestType, "request_type=%s should set filter", tc.value)
			require.Equal(t, tc.want, *repo.capturedDim.RequestType)
		})
	}
}

func TestGetUserBreakdown_InvalidRequestType(t *testing.T) {
	repo := &userBreakdownRepoCapture{}
	router := newUserBreakdownRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/user-breakdown?start_date=2026-03-01&end_date=2026-03-16&request_type=bogus", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
