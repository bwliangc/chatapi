package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountAbnormalNotificationRouter(adminSvc *stubAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts/:id/abnormal-notification", handler.GetAbnormalNotification)
	router.PUT("/api/v1/admin/accounts/:id/abnormal-notification", handler.UpdateAbnormalNotification)
	return router
}

func responseData(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope.Data
}

func TestGetAccountAbnormalNotificationUsesSavedSettings(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{
		ID: 42,
		Extra: map[string]any{
			service.AccountExtraAbnormalNotifyEnabled: true,
			service.AccountExtraAbnormalNotifyEmail:   "alerts@example.com",
		},
	}}
	router := setupAccountAbnormalNotificationRouter(adminSvc)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/42/abnormal-notification", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, map[string]any{"enabled": true, "email": "alerts@example.com", "statuses": []any{"error"}}, responseData(t, recorder))
}

func TestGetAccountAbnormalNotificationDefaultsToParentEmail(t *testing.T) {
	parentID := int64(7)
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{ID: 42, ParentAccountID: &parentID},
		{ID: parentID, Extra: map[string]any{"email": "owner@example.com"}},
	}
	router := setupAccountAbnormalNotificationRouter(adminSvc)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/42/abnormal-notification", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, map[string]any{"enabled": false, "email": "owner@example.com", "statuses": []any{"error"}}, responseData(t, recorder))
}

func TestUpdateAccountAbnormalNotificationPersistsSettings(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{ID: 42}}
	router := setupAccountAbnormalNotificationRouter(adminSvc)
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"enabled":true,"email":" alerts@example.com "}`)
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42/abnormal-notification", body))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), adminSvc.updatedAccountExtraID)
	require.Equal(t, map[string]any{
		service.AccountExtraAbnormalNotifyEnabled:  true,
		service.AccountExtraAbnormalNotifyEmail:    "alerts@example.com",
		service.AccountExtraAbnormalNotifyStatuses: []string{"error"},
	}, adminSvc.updatedAccountExtra)
}

func TestUpdateAccountAbnormalNotificationSelectedStatuses(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
		code int
	}{
		{"selected", `{"enabled":true,"email":"alerts@example.com","statuses":["rate_limited","overloaded","rate_limited"]}`, []string{"rate_limited", "overloaded"}, http.StatusOK},
		{"legacy request preserves selection", `{"enabled":true,"email":"alerts@example.com"}`, []string{"temp_unschedulable"}, http.StatusOK},
		{"empty enabled selection", `{"enabled":true,"email":"alerts@example.com","statuses":[]}`, nil, http.StatusBadRequest},
		{"unknown status", `{"enabled":true,"email":"alerts@example.com","statuses":["active"]}`, nil, http.StatusBadRequest},
		{"disabled empty selection", `{"enabled":false,"email":"","statuses":[]}`, []string{}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adminSvc := newStubAdminService()
			adminSvc.accounts = []service.Account{{ID: 42, Extra: map[string]any{
				service.AccountExtraAbnormalNotifyStatuses: []any{"temp_unschedulable"},
			}}}
			router := setupAccountAbnormalNotificationRouter(adminSvc)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42/abnormal-notification", strings.NewReader(tt.body)))
			require.Equal(t, tt.code, recorder.Code, recorder.Body.String())
			if tt.code == http.StatusOK {
				require.Equal(t, tt.want, adminSvc.updatedAccountExtra[service.AccountExtraAbnormalNotifyStatuses])
				var response struct {
					Data service.AccountAbnormalNotificationSettings `json:"data"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, tt.want, response.Data.Statuses)
			} else {
				require.Zero(t, adminSvc.updatedAccountExtraID)
			}
		})
	}
}

func TestUpdateAccountAbnormalNotificationRequiresValidEmailWhenEnabled(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{ID: 42}}
	router := setupAccountAbnormalNotificationRouter(adminSvc)

	for _, body := range []string{`{"enabled":true,"email":""}`, `{"enabled":true,"email":"invalid"}`} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42/abnormal-notification", strings.NewReader(body)))
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	}
}
