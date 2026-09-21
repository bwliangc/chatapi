package service

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCodexTicketScopeDefaultsAndDisableRestoreOriginalForwarding(t *testing.T) {
	for _, optIn := range []any{nil, false, "true", true} {
		for _, valid := range []bool{false, true} {
			t.Run(fmt.Sprintf("enabled=%v/ticket=%v", optIn, valid), func(t *testing.T) {
				cfg := config.OpenAICodexTicketConfig{Enabled: true, FailClosed: true}
				svc := ticketTestService(t, cfg, nil)
				account := ticketTestAccount(41)
				account.Extra = map[string]any{codexTicketAccountEnabledKey: optIn}
				if valid {
					svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
						Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292,
						CapturedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
					})
				}
				h := http.Header{}
				h.Set(openAICodexTurnStateHeader, "client-original")
				err := svc.applyOpenAICodexTicket(context.Background(), account, "gpt-6-astra", h)
				blocked := optIn == true && !valid
				require.Equal(t, blocked, svc.openAICodexTicketBlocksAccount(account, "gpt-6-astra"))
				if blocked {
					require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
				} else {
					require.NoError(t, err)
				}
				if optIn == true && valid {
					require.Equal(t, fakeCodexTicketState(292), h.Get(openAICodexTurnStateHeader))
				} else {
					require.Equal(t, "client-original", h.Get(openAICodexTurnStateHeader))
				}
				// Turning off an enabled account also ignores its cached ticket.
				account.Extra[codexTicketAccountEnabledKey] = false
				h.Set(openAICodexTurnStateHeader, "client-original")
				require.NoError(t, svc.applyOpenAICodexTicket(context.Background(), account, "gpt-6-astra", h))
				require.Equal(t, "client-original", h.Get(openAICodexTurnStateHeader))
				require.False(t, svc.openAICodexTicketBlocksAccount(account, "gpt-6-astra"))
				for _, status := range OpenAICodexTicketStatuses(account, cfg, time.Now()) {
					require.False(t, status.Blocked)
					require.False(t, status.HarvestEnabled)
				}
			})
		}
	}
}

func TestCodexTicketScopeOnlyHarvestsOptedInAccountAndModel(t *testing.T) {
	selected, untouched := ticketTestAccount(41), ticketTestAccount(42)
	selected.Status, untouched.Status = StatusActive, StatusActive
	selected.Extra[codexTicketModelsEnabledKey] = map[string]any{"gpt-5.6-sol": false}
	untouched.Extra = nil
	repo := &codexTicketRefreshRepo{accounts: []Account{*selected, *untouched}}
	var calls atomic.Int32
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, FailClosed: true, HarvestProxyURL: "http://proxy.example:8080",
	}, &codexTicketFuncUpstream{do: func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return codexTicketResponse(), nil
	}})
	svc.accountRepo = repo
	svc.refreshOpenAICodexTickets(context.Background())
	require.Equal(t, int32(1), calls.Load())
	require.NotNil(t, svc.lookupOpenAICodexTicket(selected, "gpt-6-astra"))
	require.Nil(t, svc.lookupOpenAICodexTicket(selected, "gpt-5.6-sol"))
	require.Nil(t, svc.lookupOpenAICodexTicket(untouched, "gpt-6-astra"))
	require.False(t, svc.openAICodexTicketBlocksAccount(selected, "gpt-5.6-sol"))
	require.False(t, svc.openAICodexTicketBlocksAccount(untouched, "gpt-6-astra"))
}

func TestCodexTicketScopeHTTPAndWSBuilders(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, mode := range []string{"responses", "passthrough", "websocket"} {
			t.Run(fmt.Sprintf("enabled=%v/%s", enabled, mode), func(t *testing.T) {
				svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, FailClosed: true}, nil)
				account := ticketTestAccount(41)
				account.Extra[codexTicketAccountEnabledKey] = enabled
				c, _ := newTurnStateTestContext(t, 7, "ticket-scope")
				c.Request.Header.Set(openAICodexTurnStateHeader, "client-original")
				build := func() (http.Header, error) {
					if mode == "websocket" {
						h, _, err := svc.buildOpenAIWSHeaders(context.Background(), c, account, "token", OpenAIWSProtocolDecision{}, true, "client-original", "", "", "gpt-6-astra", "")
						return h, err
					}
					body := []byte(`{"model":"gpt-6-astra","input":"hello"}`)
					var req *http.Request
					var err error
					if mode == "passthrough" {
						req, err = svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "token")
					} else {
						req, err = svc.buildUpstreamRequest(context.Background(), c, account, body, "token", true, "", true)
					}
					if err != nil {
						return nil, err
					}
					return req.Header, nil
				}
				h, err := build()
				if enabled {
					require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
				} else {
					require.NoError(t, err)
					require.Equal(t, "client-original", h.Get(openAICodexTurnStateHeader))
				}
				svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
					Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292,
					CapturedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
				})
				h, err = build()
				require.NoError(t, err)
				want := "client-original"
				if enabled {
					want = fakeCodexTicketState(292)
				}
				require.Equal(t, want, h.Get(openAICodexTurnStateHeader))
			})
		}
	}
}

func TestCodexTicketHarvestBoundsConcurrencyAndCancels(t *testing.T) {
	accounts := make([]Account, 8)
	for i := range accounts {
		accounts[i] = *ticketTestAccount(int64(i + 1))
		accounts[i].Status = StatusActive
	}
	started := make(chan struct{}, 16)
	var active, peak atomic.Int32
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, HarvestProxyURL: "http://proxy.example:8080", HarvestConcurrency: 2,
	}, &codexTicketFuncUpstream{do: func(req *http.Request) (*http.Response, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		started <- struct{}{}
		<-req.Context().Done()
		return nil, req.Context().Err()
	}})
	svc.accountRepo = &codexTicketRefreshRepo{accounts: accounts}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); svc.refreshOpenAICodexTickets(ctx) }()
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("probe did not start")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("harvester did not stop")
	}
	require.Equal(t, int32(2), peak.Load())
	require.Zero(t, active.Load())
}
