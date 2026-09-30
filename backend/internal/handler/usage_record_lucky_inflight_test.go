package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type usageLuckySettings struct{ service.SettingRepository }

func (usageLuckySettings) GetValue(context.Context, string) (string, error) { return "true", nil }

type usageLuckyRepo struct {
	service.LuckySecondRepository
	finished atomic.Int32
}

func (*usageLuckyRepo) Begin(context.Context, string, string) (bool, error) { return true, nil }
func (*usageLuckyRepo) Tick(context.Context, string) error                  { return nil }
func (*usageLuckyRepo) PendingCacheInvalidations(context.Context) (map[int64][]int64, error) {
	return nil, nil
}
func (r *usageLuckyRepo) Finish(context.Context, string) error {
	r.finished.Add(1)
	return nil
}

// Both references must survive the handler and be released exactly once on every
// task outcome, without settling Lucky Second while another task is still pending.
func TestUsageTaskReleasesLuckySecondAndInflightReservation(t *testing.T) {
	for _, outcome := range []string{"completed", "dropped", "panicked"} {
		t.Run(outcome, func(t *testing.T) {
			repo := &usageLuckyRepo{}
			lucky := service.NewLuckySecondService(repo, service.NewSettingService(usageLuckySettings{}, nil), nil)
			t.Cleanup(lucky.Stop)
			var parent context.Context
			require.Eventually(t, func() bool {
				parent = lucky.Begin(context.Background())
				return service.LuckySecondAttempt(parent) != ""
			}, time.Second, time.Millisecond)
			lucky.Stop()

			cache := newHandlerInflightCache(1)
			cfg := &config.Config{}
			cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
			billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			key := &service.APIKey{User: &service.User{ID: 5}}
			parent, handlerDone, err := reserveInflightBalanceCtx(parent, billing, &countingEstimator{cost: 0.9, priced: true}, key, nil, tokenInflightEstimate("m", nil))
			require.NoError(t, err)
			attempt := service.LuckySecondAttempt(parent)
			ran := false
			task, abandon := wrapUsageRecordTaskContext(parent, func(ctx context.Context) {
				ran = true
				require.Equal(t, attempt, service.LuckySecondAttempt(ctx))
				require.Equal(t, 1, cache.count())
				require.Zero(t, repo.finished.Load())
				if outcome == "panicked" {
					panic("billing task failed")
				}
			})
			_, abandonOther := wrapUsageRecordTaskContext(parent, func(context.Context) {})
			handlerDone()
			service.ReleaseLuckySecond(parent)
			require.Equal(t, 1, cache.count())
			require.Zero(t, repo.finished.Load())
			switch outcome {
			case "completed":
				task(context.Background())
			case "panicked":
				require.Panics(t, func() { task(context.Background()) })
			case "dropped":
				abandon()
			}
			abandon()
			require.Equal(t, outcome != "dropped", ran)
			require.Equal(t, 1, cache.count(), "other billing task still holds the reservation")
			require.Zero(t, repo.finished.Load(), "other billing task still holds participation")
			abandonOther()
			abandonOther()
			require.Zero(t, cache.count())
			require.EqualValues(t, 1, repo.finished.Load())
		})
	}
}
