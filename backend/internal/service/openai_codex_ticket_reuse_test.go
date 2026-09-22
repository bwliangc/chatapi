package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCodexTicketReuseRequiresExpiryAndHonorsWindowBoundary(t *testing.T) {
	now := time.Now()
	ticket := &openAICodexTicket{State: fakeCodexTicketState(292), Length: 292}
	require.False(t, ticket.usable(now, 292, true, 0), "an incomplete ticket cannot be reused indefinitely")
	ticket.ExpiresAt = now.Add(-10 * time.Minute)
	require.False(t, ticket.usable(now, 292, true, 10*time.Minute), "the reuse window ends exactly at the boundary")
	require.True(t, ticket.usable(now.Add(-time.Nanosecond), 292, true, 10*time.Minute))
	require.True(t, ticket.usable(now, 292, true, 0))
	require.False(t, ticket.usable(now, 292, false, 0))
	ticket.State = fakeCodexTicketState(332)
	require.False(t, ticket.usable(now, 292, true, 0))
}

func TestCodexTicketExpiredReusePreservesAccountAndModelScope(t *testing.T) {
	for _, scope := range []string{"enabled", "global off", "account off", "model off", "other account", "other model"} {
		t.Run(scope, func(t *testing.T) {
			svc := ticketTestService(t, config.OpenAICodexTicketConfig{
				Enabled: scope != "global off", FailClosed: true, ReuseExpired: true, ReuseExpiredMaxSeconds: 600,
			}, nil)
			account := ticketTestAccount(41)
			svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
				Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292,
				Cookie: "ticket_cookie=account41", CapturedAt: time.Now().Add(-5 * time.Minute), ExpiresAt: time.Now().Add(-time.Minute),
			})
			model := "gpt-6-astra"
			switch scope {
			case "account off":
				account.Extra[codexTicketAccountEnabledKey] = false
			case "model off":
				account.Extra[codexTicketModelsEnabledKey] = map[string]any{model: false}
			case "other account":
				account = ticketTestAccount(42)
			case "other model":
				model = "gpt-5.6-sol"
			}
			h := http.Header{}
			h.Set("Cookie", "existing=1")
			err := svc.applyOpenAICodexTicket(context.Background(), account, model, h)
			blocked := scope == "other account" || scope == "other model"
			require.Equal(t, blocked, svc.openAICodexTicketBlocksAccount(account, model))
			if blocked {
				require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
			} else {
				require.NoError(t, err)
			}
			if scope == "enabled" {
				require.Equal(t, fakeCodexTicketState(292), h.Get(openAICodexTurnStateHeader))
				require.Equal(t, "existing=1; ticket_cookie=account41", h.Get("Cookie"))
			} else {
				require.Empty(t, h.Get(openAICodexTurnStateHeader))
				require.Equal(t, "existing=1", h.Get("Cookie"))
			}
		})
	}
}

func TestCodexTicketReuseLoadsLatestPersistedCookieAfterRestart(t *testing.T) {
	cfg := config.OpenAICodexTicketConfig{Enabled: true, FailClosed: true, ReuseExpired: true, ReuseExpiredMaxSeconds: 600}
	svc := ticketTestService(t, cfg, nil)
	account := ticketTestAccount(41)
	now := time.Now()
	svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
		Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292, Cookie: "version=old",
		CapturedAt: now.Add(-10 * time.Minute), ExpiresAt: now.Add(-8 * time.Minute),
	})
	// A newer ticket captured by another instance must replace the in-memory pair.
	account.Extra[openAICodexTicketExtraKey("gpt-6-astra")] = map[string]any{
		"state": fakeCodexTicketState(292), "length": 292, "cookie": "version=new",
		"captured_at": now.Add(-3 * time.Minute), "expires_at": now.Add(-time.Minute),
	}
	for _, gateway := range []*OpenAIGatewayService{svc, ticketTestService(t, cfg, nil)} {
		h := http.Header{}
		require.NoError(t, gateway.applyOpenAICodexTicket(context.Background(), account, "gpt-6-astra", h))
		require.Equal(t, "version=new", h.Get("Cookie"))
		require.False(t, gateway.openAICodexTicketBlocksAccount(account, "gpt-6-astra"))
	}
}
