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
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
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
	require.Equal(t, map[string]any{"enabled": true, "email": "alerts@example.com"}, responseData(t, recorder))
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
	require.Equal(t, map[string]any{"enabled": false, "email": "owner@example.com"}, responseData(t, recorder))
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
		service.AccountExtraAbnormalNotifyEnabled: true,
		service.AccountExtraAbnormalNotifyEmail:   "alerts@example.com",
	}, adminSvc.updatedAccountExtra)
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
