package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountNotificationEmailSenderStub struct {
	input *service.NotificationEmailSendInput
	err   error
}

func (s *accountNotificationEmailSenderStub) Send(_ context.Context, input service.NotificationEmailSendInput) error {
	s.input = &input
	return s.err
}

func setupAccountAbnormalNoticeRouter(adminSvc *stubAdminService, sender accountNotificationEmailSender) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler.notificationEmailService = sender
	router.POST("/api/v1/admin/accounts/:id/send-abnormal-notice", handler.SendAbnormalNotice)
	return router
}

func TestAccountHandlerSendAbnormalNotice(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{
		ID:           42,
		Name:         "openai-main",
		Platform:     service.PlatformOpenAI,
		Status:       "error",
		ErrorMessage: "refresh token expired",
		Extra:        map[string]any{"email_address": "owner@example.com"},
	}}
	sender := &accountNotificationEmailSenderStub{}
	router := setupAccountAbnormalNoticeRouter(adminSvc, sender)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/send-abnormal-notice", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, sender.input)
	require.Equal(t, service.NotificationEmailEventAccountAbnormalNotice, sender.input.Event)
	require.Equal(t, "zh-CN,zh;q=0.9", sender.input.Locale)
	require.Equal(t, "owner@example.com", sender.input.RecipientEmail)
	require.Equal(t, "owner", sender.input.RecipientName)
	require.Equal(t, map[string]string{
		"account_id":     "42",
		"account_name":   "openai-main",
		"platform":       service.PlatformOpenAI,
		"account_status": "error",
		"error_message":  "refresh token expired",
	}, sender.input.Variables)
}

func TestAccountHandlerSendAbnormalNoticeUsesParentAccountEmail(t *testing.T) {
	parentID := int64(7)
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:              42,
			Name:            "openai-shadow",
			Platform:        service.PlatformOpenAI,
			Status:          "error",
			ParentAccountID: &parentID,
		},
		{
			ID:       parentID,
			Name:     "openai-main",
			Platform: service.PlatformOpenAI,
			Status:   service.StatusActive,
			Extra:    map[string]any{"email": "owner@example.com"},
		},
	}
	sender := &accountNotificationEmailSenderStub{}
	router := setupAccountAbnormalNoticeRouter(adminSvc, sender)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/send-abnormal-notice", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, sender.input)
	require.Equal(t, "owner@example.com", sender.input.RecipientEmail)
}

func TestAccountHandlerSendAbnormalNoticeAllowsManualNoticeForNormalAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{
		ID:       42,
		Name:     "openai-main",
		Platform: service.PlatformOpenAI,
		Status:   service.StatusActive,
		Extra:    map[string]any{"email_address": "owner@example.com"},
	}}
	sender := &accountNotificationEmailSenderStub{}
	router := setupAccountAbnormalNoticeRouter(adminSvc, sender)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/send-abnormal-notice", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, sender.input)
	require.Equal(t, service.StatusActive, sender.input.Variables["account_status"])
	require.Equal(t, "-", sender.input.Variables["error_message"])
}

func TestAccountHandlerSendAbnormalNoticeRejectsAccountWithoutEmail(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{
		ID:       42,
		Name:     "openai-main",
		Platform: service.PlatformOpenAI,
		Status:   "error",
	}}
	sender := &accountNotificationEmailSenderStub{}
	router := setupAccountAbnormalNoticeRouter(adminSvc, sender)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/send-abnormal-notice", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, sender.input)
}
