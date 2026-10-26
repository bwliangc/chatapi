//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiOpenAICompatForwards_ImageGenerationGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"messages", "chat/completions", "native"} {
		for _, tc := range []struct {
			name     string
			model    string
			platform string
			allowed  bool
			wantDeny bool
		}{
			{name: "image model", model: "gemini-3-pro-image", platform: PlatformGemini, wantDeny: true},
			{name: "mapped image alias", model: "my-draw", platform: PlatformGemini, wantDeny: true},
			{name: "composite group", model: "my-draw", platform: PlatformComposite, wantDeny: true},
			{name: "enabled group", model: "my-draw", platform: PlatformGemini, allowed: true},
			{name: "text model", model: "gemini-2.5-flash", platform: PlatformGemini},
			{name: "custom group", model: "my-draw", platform: PlatformCustom},
		} {
			t.Run(endpoint+"/"+tc.name, func(t *testing.T) {
				svc, httpStub := newGeminiVertexAPIKeyTestService()
				httpStub.response.Body = io.NopCloser(strings.NewReader(`{"id":"chat-test","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`))
				account := &Account{
					ID: 811, Platform: PlatformGemini, Type: AccountTypeAPIKey,
					Credentials: map[string]any{
						"api_key": "test-key", "base_url": "https://example.com/v1",
						"upstream_mode": GeminiUpstreamModeOpenAIChatCompletions,
						"model_mapping": map[string]any{"my-draw": "gemini-3-pro-image"},
					},
				}
				body := []byte(`{"model":"` + tc.model + `","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
				if endpoint == "native" {
					body = []byte(`{"contents":[{"parts":[{"text":"hello"}]}]}`)
				}
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, bytes.NewReader(body))
				withGeminiImageGateAPIKey(c, &Group{Platform: tc.platform, AllowImageGeneration: tc.allowed})

				var result *ForwardResult
				var err error
				switch endpoint {
				case "messages":
					result, err = svc.Forward(context.Background(), c, account, body)
				case "chat/completions":
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
				case "native":
					result, err = svc.ForwardNative(context.Background(), c, account, tc.model, "generateContent", false, body)
				}
				if tc.wantDeny {
					require.Error(t, err)
					require.Nil(t, result)
					require.Zero(t, httpStub.calls)
					googleStatus := ""
					if endpoint == "native" {
						googleStatus = "INVALID_ARGUMENT"
					}
					requireGeminiImagePermissionError(t, rec, googleStatus)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 1, httpStub.calls)
				require.Equal(t, "/v1/chat/completions", httpStub.lastReq.URL.Path)
				require.Equal(t, 7, result.Usage.OutputTokens)
			})
		}
	}
}

func TestGeminiOpenAICompatNative_ImageModalitiesAndCountTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, action := range []string{"generateContent", "countTokens"} {
		t.Run(action, func(t *testing.T) {
			svc, httpStub := newGeminiVertexAPIKeyTestService()
			account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{
				"upstream_mode": GeminiUpstreamModeOpenAIChatCompletions,
			}}
			c, rec := newGeminiNativeTestContext(t)
			withGeminiImageGateAPIKey(c, imageDisabledGroup())
			body := []byte(`{"contents":[{"parts":[{"text":"hello"}]}],"generationConfig":{"responseModalities":["IMAGE"]}}`)
			_, err := svc.ForwardNative(context.Background(), c, account, "gemini-2.5-flash", action, false, body)
			require.Zero(t, httpStub.calls)
			if action == "countTokens" {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)
				return
			}
			require.Error(t, err)
			requireGeminiImagePermissionError(t, rec, "INVALID_ARGUMENT")
		})
	}
}
