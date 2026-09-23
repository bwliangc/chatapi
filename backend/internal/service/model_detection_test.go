package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

func TestModelDetectionStream(t *testing.T) {
	cases := []struct {
		name, protocol, stream string
		ok                     bool
	}{
		{"responses complete", "responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"1,2,3\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"model\":\"gpt-test\"}}\n\n", true},
		{"responses truncated", "responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"1,2,3\"}\n\n", false},
		{"responses incomplete", "responses", "data: {\"type\":\"response.incomplete\"}\n\n", false},
		{"responses tool", "responses", "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\"}}\n\n", false},
		{"claude complete", "anthropic", "data:{\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"1,2,3\"}}\r\n\r\ndata:{\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\r\n\r\ndata:{\"type\":\"message_stop\"}\r\n\r\n", true},
		{"claude length", "anthropic", "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", false},
		{"chat complete", "chat", "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"1,2,3\"},\"finish_reason\":\"stop\"}]}\n\n", true},
		{"chat length", "chat", "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"1,2,3\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n", false},
		{"chat done only", "chat", "data: [DONE]\n\n", false},
		{"malformed", "responses", "data: invalid\n\n", false},
		{"non-stream", "chat", "<html>an error with secrets</html>", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, _, err := parseModelDetectionStream(strings.NewReader(c.stream), c.protocol)
			if (err == nil) != c.ok {
				t.Fatalf("unexpected result %q %v", text, err)
			}
			if c.ok && text != "1,2,3" {
				t.Fatal(text)
			}
		})
	}
	raw, _ := json.Marshal(map[string]any{"type": "response.output_text.delta", "delta": strings.Repeat("1", modelDetectionOutputLimit+1)})
	if _, _, err := parseModelDetectionStream(strings.NewReader("data: "+string(raw)+"\n\n"), "responses"); err == nil {
		t.Fatal("accepted oversized output")
	}
}

type detectionSettingRepo struct {
	SettingRepository
	enabled atomic.Bool
}

func (r *detectionSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if key != SettingKeyModelDetectionEnabled {
		return "", ErrSettingNotFound
	}
	if r.enabled.Load() {
		return "true", nil
	}
	return "false", nil
}

type detectionAccountRepo struct {
	AccountRepository
	account *Account
}

func (r *detectionAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if id != r.account.ID {
		return nil, errors.New("wrong account")
	}
	return r.account, nil
}

type detectionHTTP struct {
	HTTPUpstream
	call func(*http.Request, string, int64) (*http.Response, error)
}

func (h detectionHTTP) DoWithTLS(r *http.Request, proxy string, id int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return h.call(r, proxy, id)
}

func TestModelDetectionFixedAccountAndFeatureGate(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			settings := &detectionSettingRepo{}
			settings.enabled.Store(true)
			account := &Account{ID: 42, Name: "test", Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "test-secret", "base_url": "https://models.example"}}
			calls := 0
			upstream := detectionHTTP{call: func(r *http.Request, _ string, id int64) (*http.Response, error) {
				calls++
				if id != 42 {
					t.Fatal("account failover")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(body)
				if !strings.Contains(string(raw), "355") || strings.Contains(string(raw), `"content":"hi"`) {
					t.Fatal("challenge was not forwarded")
				}
				text := strings.Repeat("7,", 310)
				var stream string
				if platform == PlatformAnthropic {
					if body["max_tokens"] != float64(4096) {
						t.Fatal("wrong token budget")
					}
					stream = fmt.Sprintf("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", text)
				} else {
					stream = fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"model\":\"gpt-6-sol\"}}\n\n", text)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(stream)), Header: http.Header{}}, nil
			}}
			svc := &AccountTestService{accountRepo: &detectionAccountRepo{account: account}, settingService: NewSettingService(settings, nil), httpUpstream: upstream, cfg: &config.Config{}}
			r, err := svc.DetectModel(context.Background(), 42, "gpt-6-sol", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 3 || r.Result == nil || r.Result.UsedOutputs != 3 {
				t.Fatalf("calls %d result %+v", calls, r)
			}
			raw, _ := json.Marshal(r)
			if strings.Contains(string(raw), "test-secret") || strings.Contains(string(raw), strings.Repeat("7,", 100)) {
				t.Fatal("sensitive/raw output retained")
			}
			settings.enabled.Store(false)
			if _, err := svc.DetectModel(context.Background(), 42, "gpt-6-sol", nil, nil); err == nil {
				t.Fatal("disabled feature accepted")
			}
			if calls != 3 {
				t.Fatal("disabled feature made a request")
			}
			settings.enabled.Store(true)
			_, err = svc.DetectModel(context.Background(), 42, "gpt-6-sol", nil, func(p ModelDetectionProgress) {
				if p.Accepted == 1 {
					settings.enabled.Store(false)
				}
			})
			if err == nil || calls != 4 {
				t.Fatalf("toggle did not stop subsequent probes: %d %v", calls, err)
			}
		})
	}
}

func TestModelDetectionRejectsUnsafeAccountsAndCancellation(t *testing.T) {
	a := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}
	if err := validateModelDetectionAccount(a, "gpt-image-2"); err == nil {
		t.Fatal("accepted image model")
	}
	a.Schedulable = false
	if err := validateModelDetectionAccount(a, "gpt-6-sol"); err == nil {
		t.Fatal("accepted unschedulable account")
	}
	settings := &detectionSettingRepo{}
	settings.enabled.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := &AccountTestService{settingService: NewSettingService(settings, nil)}
	if _, err := svc.DetectModel(ctx, 1, "gpt-6-sol", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestModelDetectionInsufficientSamplesBounded(t *testing.T) {
	settings := &detectionSettingRepo{}
	settings.enabled.Store(true)
	a := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test", "base_url": "https://models.example"}}
	var calls atomic.Int32
	upstream := detectionHTTP{call: func(*http.Request, string, int64) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"1,2\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")), Header: http.Header{}}, nil
	}}
	svc := &AccountTestService{accountRepo: &detectionAccountRepo{account: a}, settingService: NewSettingService(settings, nil), httpUpstream: upstream, cfg: &config.Config{}}
	result, err := svc.DetectModel(context.Background(), 42, "gpt-6-sol", nil, nil)
	if err != nil || calls.Load() != 6 || result.Verdict.Status != "insufficient" {
		t.Fatalf("calls %d result %+v error %v", calls.Load(), result, err)
	}
}

func TestModelDetectionOAuthUsesChallengeAndSanitizesErrors(t *testing.T) {
	a := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "test-secret"}}
	upstream := detectionHTTP{call: func(r *http.Request, _ string, id int64) (*http.Response, error) {
		if id != 7 || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Fatal("wrong account authentication")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(body)
		if !strings.Contains(string(raw), "test-challenge") || body["instructions"] == "" {
			t.Fatal("challenge or OAuth instructions missing")
		}
		if _, exists := body["max_output_tokens"]; exists {
			t.Fatal("Codex does not support max_output_tokens")
		}
		return nil, errors.New("network https://example/?api_key=test-secret")
	}}
	svc := &AccountTestService{httpUpstream: upstream}
	_, _, err := svc.probeModelIdentity(context.Background(), a, "gpt-6-sol", "test-challenge")
	if err == nil || strings.Contains(err.Error(), "test-secret") {
		t.Fatal("raw upstream error exposed")
	}
}
