//go:build unit

package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rawChatUsageUpstream struct {
	service.HTTPUpstream
	calls int
}

func (u *rawChatUsageUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	u.calls++
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"raw-chat-usage"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chat-test","choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`)),
	}, nil
}

func TestGatewayChatCompletions_RawForwardUsageRecordsChatEndpointOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{service.PlatformCustom, service.PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			cfg := &config.Config{}
			groupID := int64(9400)
			group := &service.Group{ID: groupID, Hydrated: true, Platform: platform, Status: service.StatusActive, RateMultiplier: 1}
			account := &service.Account{
				ID: 9401, Platform: platform, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{
					"api_key": "test-key", "base_url": "https://example.com/v1",
					"upstream_mode": service.GeminiUpstreamModeOpenAIChatCompletions,
				},
				AccountGroups: []service.AccountGroup{{AccountID: 9401, GroupID: groupID}},
			}
			snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
			usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
			billingRepo := &compatStreamUsageBillingRepo{}
			upstream := &rawChatUsageUpstream{}
			billingCache := service.NewBillingCacheService(newHandlerInflightCache(100), nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billingCache.Stop)
			gw := service.NewGatewayService(
				nil, &fakeGroupRepo{group: group}, usageRepo, billingRepo, nil, nil, nil, nil, cfg,
				snapshot, nil, service.NewBillingService(cfg, nil), nil, nil, nil, upstream,
				&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			)
			h := &GatewayHandler{
				gatewayService: gw, billingCacheService: billingCache, cfg: cfg, maxAccountSwitches: 1,
				concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
				geminiCompatService: service.NewGeminiMessagesCompatService(nil, nil, nil, snapshot, nil, nil, upstream, nil, cfg),
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			body := []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}]}`)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			apiKey := &service.APIKey{ID: 9402, UserID: 9403, GroupID: &groupID, Group: group, User: &service.User{ID: 9403, Balance: 100}}
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9403})

			h.ChatCompletions(c)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, 1, upstream.calls)
			require.Len(t, usageRepo.created, 1)
			usage := <-usageRepo.created
			require.NotNil(t, usage.UpstreamEndpoint)
			require.Equal(t, EndpointChatCompletions, *usage.UpstreamEndpoint)
			require.Equal(t, 12, usage.InputTokens)
			require.Equal(t, 7, usage.OutputTokens)
			billingRepo.mu.Lock()
			defer billingRepo.mu.Unlock()
			require.Len(t, billingRepo.commands, 1)
		})
	}
}
