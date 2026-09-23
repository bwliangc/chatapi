package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type detectionSlotCache struct {
	ConcurrencyCache
	active   atomic.Int32
	acquired atomic.Int32
	released atomic.Int32
}

func (c *detectionSlotCache) AcquireAccountSlot(_ context.Context, _ int64, maximum int, _ string) (bool, error) {
	if n := c.active.Add(1); n > int32(maximum) {
		c.active.Add(-1)
		return false, nil
	}
	c.acquired.Add(1)
	return true, nil
}
func (c *detectionSlotCache) ReleaseAccountSlot(context.Context, int64, string) error {
	c.active.Add(-1)
	c.released.Add(1)
	return nil
}

type detectionSampleReply struct {
	text string
	err  error
}
type detectionParallelRun struct {
	svc       *AccountTestService
	ctx       context.Context
	cancel    context.CancelFunc
	started   chan chan detectionSampleReply
	done      chan struct{}
	settings  *detectionSettingRepo
	slots     *detectionSlotCache
	result    *ModelDetectionResult
	err       error
	cancelled atomic.Int32
	calls     atomic.Int32
	progress  []ModelDetectionProgress
}

func startParallelDetection(t *testing.T, limit int, onProgress func(*detectionParallelRun, ModelDetectionProgress)) *detectionParallelRun {
	t.Helper()
	r := &detectionParallelRun{started: make(chan chan detectionSampleReply, 6), done: make(chan struct{}), settings: &detectionSettingRepo{}, slots: &detectionSlotCache{}}
	r.ctx, r.cancel = context.WithTimeout(context.Background(), 5*time.Second)
	r.settings.enabled.Store(true)
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: limit, Credentials: map[string]any{"api_key": "test", "base_url": "https://models.example"}}
	upstream := detectionHTTP{call: func(req *http.Request, _ string, _ int64) (*http.Response, error) {
		r.calls.Add(1)
		reply := make(chan detectionSampleReply, 1)
		r.started <- reply
		select {
		case <-req.Context().Done():
			r.cancelled.Add(1)
			return nil, req.Context().Err()
		case response := <-reply:
			if response.err != nil {
				return nil, response.err
			}
			stream := fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"model\":\"gpt-6-sol\"}}\n\n", response.text)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(stream)), Header: http.Header{}}, nil
		}
	}}
	r.svc = &AccountTestService{accountRepo: &detectionAccountRepo{account: account}, settingService: NewSettingService(r.settings, nil), httpUpstream: upstream, cfg: &config.Config{}}
	go func() {
		defer close(r.done)
		r.result, r.err = r.svc.DetectModel(r.ctx, 42, "gpt-6-sol", NewConcurrencyService(r.slots), func(p ModelDetectionProgress) {
			r.progress = append(r.progress, p)
			if onProgress != nil {
				onProgress(r, p)
			}
		})
	}()
	t.Cleanup(func() {
		r.cancel()
		select {
		case <-r.done:
		case <-time.After(6 * time.Second):
			t.Error("detection workers did not stop")
		}
	})
	return r
}
func (r *detectionParallelRun) next(t *testing.T) chan detectionSampleReply {
	t.Helper()
	select {
	case c := <-r.started:
		return c
	case <-r.done:
		t.Fatalf("detection ended before expected request: %v", r.err)
	case <-r.ctx.Done():
		t.Fatal("parallel requests did not start")
	}
	return nil
}
func (r *detectionParallelRun) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(6 * time.Second):
		t.Fatal("detection did not finish")
	}
	if r.slots.active.Load() != 0 || r.slots.acquired.Load() != r.slots.released.Load() {
		t.Fatal("concurrency slots leaked")
	}
}
func validDetectionReply() detectionSampleReply {
	return detectionSampleReply{text: strings.Repeat("7,", 340)}
}

func TestModelDetectionParallelRespectsAccountLimit(t *testing.T) {
	for _, limit := range []int{0, 1, 2, 3, 8} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			r := startParallelDetection(t, limit, nil)
			width := 3
			if limit > 0 && limit < width {
				width = limit
			}
			initial := make([]chan detectionSampleReply, width)
			for i := range initial {
				initial[i] = r.next(t)
			}
			// Every initial call is blocked: reaching this point proves actual overlap.
			select {
			case <-r.started:
				t.Fatal("exceeded account parallelism")
			default:
			}
			for _, reply := range initial {
				reply <- validDetectionReply()
			}
			for i := width; i < 3; i++ {
				r.next(t) <- validDetectionReply()
			}
			r.wait(t)
			if r.err != nil || r.result.Result == nil || r.result.Result.UsedOutputs != 3 || r.calls.Load() != 3 {
				t.Fatalf("result %+v calls %d err %v", r.result, r.calls.Load(), r.err)
			}
			previousAttempt, previousAccepted := 0, 0
			for _, p := range r.progress {
				if p.InFlight < 0 || p.InFlight > width || p.Accepted+p.InFlight > 3 || p.Attempt < previousAttempt || p.Accepted < previousAccepted {
					t.Fatalf("invalid progress %+v", p)
				}
				previousAttempt, previousAccepted = p.Attempt, p.Accepted
			}
		})
	}
}
func TestModelDetectionParallelReplacesOnlyMissingSample(t *testing.T) {
	r := startParallelDetection(t, 3, nil)
	first, second, third := r.next(t), r.next(t), r.next(t)
	first <- detectionSampleReply{text: "1,2"}
	fourth := r.next(t)
	fourth <- validDetectionReply()
	second <- validDetectionReply()
	third <- validDetectionReply()
	r.wait(t)
	if r.err != nil || r.result.Attempts != 4 || r.result.Result.UsedOutputs != 3 || r.calls.Load() != 4 {
		t.Fatalf("unexpected replacement %+v %v", r.result, r.err)
	}
}
func TestModelDetectionParallelStopsPeers(t *testing.T) {
	for _, mode := range []string{"cancel", "failure", "feature-off"} {
		t.Run(mode, func(t *testing.T) {
			r := startParallelDetection(t, 3, func(r *detectionParallelRun, p ModelDetectionProgress) {
				if mode == "feature-off" && p.Accepted == 1 {
					r.settings.enabled.Store(false)
				}
			})
			first := r.next(t)
			r.next(t)
			r.next(t)
			if _, err := r.svc.DetectModel(context.Background(), 42, "gpt-6-sol", nil, nil); err == nil {
				t.Fatal("duplicate detection accepted")
			}
			switch mode {
			case "cancel":
				r.cancel()
			case "failure":
				first <- detectionSampleReply{err: errors.New("failed upstream")}
			case "feature-off":
				first <- validDetectionReply()
			}
			r.wait(t)
			wantCancelled := int32(2)
			if mode == "cancel" {
				wantCancelled = 3
			}
			if r.err == nil || r.cancelled.Load() != wantCancelled || r.calls.Load() != 3 {
				t.Fatalf("peers not stopped: %v cancelled %d calls %d", r.err, r.cancelled.Load(), r.calls.Load())
			}
			if _, busy := r.svc.modelDetectionActive.Load(int64(42)); busy {
				t.Fatal("account task lock leaked")
			}
		})
	}
}
