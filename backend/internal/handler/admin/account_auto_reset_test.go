package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountAutoResetRouter(adminSvc *stubAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts/:id/auto-reset", handler.GetAutoReset)
	router.PUT("/api/v1/admin/accounts/:id/auto-reset", handler.UpdateAutoReset)
	return router
}

func TestUpdateAccountAutoResetPersistsStrategy(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{ID: 42, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}}
	router := setupAccountAutoResetRouter(adminSvc)
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"enabled":true,"strategy":"credit_expiry","weekly_threshold":85,"expiry_minutes":30,"email":" alerts@example.com "}`)
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42/auto-reset", body))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), adminSvc.updatedAccountExtraID)
	require.Equal(t, true, adminSvc.updatedAccountExtra[service.AccountExtraAutoResetEnabled])
	require.Equal(t, service.AccountAutoResetStrategyCreditExpiry, adminSvc.updatedAccountExtra[service.AccountExtraAutoResetStrategy])
	require.Equal(t, "alerts@example.com", adminSvc.updatedAccountExtra[service.AccountExtraAutoResetEmail])
}

func TestAccountAutoResetRejectsUnsupportedAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{ID: 42, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth}}
	router := setupAccountAutoResetRouter(adminSvc)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/42/auto-reset", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUpdateAccountAutoResetAcceptsLegacyExpiryHours(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{ID: 42, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}}
	router := setupAccountAutoResetRouter(adminSvc)
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"enabled":true,"strategy":"credit_expiry","weekly_threshold":85,"expiry_hours":2,"email":"alerts@example.com"}`)
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42/auto-reset", body))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 120, adminSvc.updatedAccountExtra[service.AccountExtraAutoResetExpiryMinutes])
}
