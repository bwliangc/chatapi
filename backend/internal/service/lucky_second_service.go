package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type LuckySecondCampaign struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          time.Time  `json:"ends_at"`
	Timezone        string     `json:"timezone"`
	TotalAmount     string     `json:"total_amount"`
	RewardCount     int        `json:"reward_count"`
	CancelledAt     *time.Time `json:"cancelled_at"`
	CreatedAt       time.Time  `json:"created_at"`
	AwardedCount    int        `json:"awarded_count"`
	AwardedAmount   string     `json:"awarded_amount"`
	ExpiredCount    int        `json:"expired_count"`
	PendingCount    int        `json:"pending_count"`
	MyAwardedAmount *string    `json:"my_awarded_amount,omitempty"`
}

type LuckySecondCreate struct {
	Name        string    `json:"name"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"` // Exclusive; date-only UIs send midnight after the last day.
	Timezone    string    `json:"timezone"`
	TotalAmount string    `json:"total_amount"`
	RewardCount int       `json:"reward_count"`
}

type LuckySecondSlot struct {
	ID        int64      `json:"id"`
	SecondAt  time.Time  `json:"second_at"`
	Amount    string     `json:"amount"`
	State     string     `json:"state"`
	UserID    *int64     `json:"user_id,omitempty"`
	Name      string     `json:"name,omitempty"`
	IsMe      bool       `json:"is_me"`
	RequestID *string    `json:"request_id,omitempty"`
	AwardedAt *time.Time `json:"awarded_at"`
}

type LuckySecondRepository interface {
	Create(context.Context, LuckySecondCreate, []LuckySecondSlot) (int64, error)
	List(context.Context, int, int, int64) ([]LuckySecondCampaign, int64, error)
	Slots(context.Context, int64, bool, int, int) ([]LuckySecondSlot, int64, error)
	Cancel(context.Context, int64) error
	Begin(context.Context, string, string) (bool, error)
	Finish(context.Context, string) error
	Tick(context.Context, string) error
	PendingCacheInvalidations(context.Context) (map[int64][]int64, error)
	MarkCacheInvalidated(context.Context, []int64) error
}

// GenerateLuckySecondSchedule uses integer micro-units, then persists all random
// choices once. No float arithmetic, zero prizes, duplicated seconds or drift.
func GenerateLuckySecondSchedule(in LuckySecondCreate, now time.Time) ([]LuckySecondSlot, error) {
	if strings.TrimSpace(in.Name) == "" || len([]rune(in.Name)) > 120 {
		return nil, fmt.Errorf("活动名称不能为空且不能超过 120 个字符")
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return nil, fmt.Errorf("请选择有效时区")
	}
	if !in.StartsAt.After(now) || !in.EndsAt.After(in.StartsAt) || in.StartsAt.Nanosecond() != 0 || in.EndsAt.Nanosecond() != 0 {
		return nil, fmt.Errorf("活动必须在未来开始，结束时间晚于开始时间，时间精确到秒")
	}
	seconds := in.EndsAt.Unix() - in.StartsAt.Unix()
	if seconds > 366*86400 || in.RewardCount < 1 || in.RewardCount > 10000 || int64(in.RewardCount) > seconds {
		return nil, fmt.Errorf("活动最长 366 天，份数为 1–10000 且不能超过活动秒数")
	}
	amount, err := decimal.NewFromString(in.TotalAmount)
	if err != nil || !amount.IsPositive() || amount.GreaterThan(decimal.NewFromInt(100000000)) || !amount.Equal(amount.Truncate(6)) {
		return nil, fmt.Errorf("奖池须为正数，最多 6 位小数，上限 100000000")
	}
	units := amount.Shift(6).IntPart()
	if units < int64(in.RewardCount) {
		return nil, fmt.Errorf("每份奖励至少为 0.000001")
	}
	// Floyd sampling is O(count), even when every second is selected.
	selected := make(map[int64]bool, in.RewardCount)
	offsets := make([]int64, 0, in.RewardCount)
	for j := seconds - int64(in.RewardCount); j < seconds; j++ {
		n, e := rand.Int(rand.Reader, big.NewInt(j+1))
		if e != nil {
			return nil, e
		}
		k := n.Int64()
		if selected[k] {
			k = j
		}
		selected[k] = true
		offsets = append(offsets, k)
	}
	// Randomize the allocation order so the remainder has no chronological bias.
	for i := len(offsets) - 1; i > 0; i-- {
		n, e := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if e != nil {
			return nil, e
		}
		j := int(n.Int64())
		offsets[i], offsets[j] = offsets[j], offsets[i]
	}
	slots := make([]LuckySecondSlot, 0, len(offsets))
	for i, offset := range offsets {
		remaining := int64(len(offsets) - i)
		prize := units
		if remaining > 1 {
			maximum := 2 * (units / remaining)
			if cap := units - (remaining - 1); maximum > cap {
				maximum = cap
			}
			n, e := rand.Int(rand.Reader, big.NewInt(maximum))
			if e != nil {
				return nil, e
			}
			prize = n.Int64() + 1
		}
		units -= prize
		slots = append(slots, LuckySecondSlot{SecondAt: in.StartsAt.Add(time.Duration(offset) * time.Second), Amount: decimal.New(prize, -6).StringFixed(6), State: "waiting"})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].SecondAt.Before(slots[j].SecondAt) })
	return slots, nil
}

type LuckySecondService struct {
	Repo       LuckySecondRepository
	settings   *SettingService
	cache      *BillingCacheService
	worker     string
	stop       chan struct{}
	once       sync.Once
	wg         sync.WaitGroup
	enabled    atomic.Bool
	ready      atomic.Bool
	unfinished sync.Map
}

func NewLuckySecondService(repo LuckySecondRepository, settings *SettingService, cache *BillingCacheService) *LuckySecondService {
	s := &LuckySecondService{Repo: repo, settings: settings, cache: cache, worker: uuid.NewString(), stop: make(chan struct{})}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.tick()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
	return s
}
func (s *LuckySecondService) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.stop) })
	s.wg.Wait()
}
func (s *LuckySecondService) Enabled(ctx context.Context) bool {
	return s != nil && s.settings.IsLuckySecondEnabled(ctx)
}
func (s *LuckySecondService) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.enabled.Store(s.Enabled(ctx))
	s.unfinished.Range(func(key, _ any) bool {
		if err := s.Repo.Finish(ctx, key.(string)); err == nil {
			s.unfinished.Delete(key)
		}
		return ctx.Err() == nil
	})
	// Finish obligations accepted before disabling the switch; do not admit new ones.
	if err := s.Repo.Tick(ctx, s.worker); err != nil {
		s.ready.Store(false)
		slog.Error("lucky second settlement failed", "error", err)
		return
	}
	s.ready.Store(true)
	pending, err := s.Repo.PendingCacheInvalidations(ctx)
	if err != nil {
		slog.Error("lucky second cache queue failed", "error", err)
		return
	}
	for user, ids := range pending {
		if s.cache != nil {
			if err = s.cache.InvalidateUserBalance(ctx, user); err != nil {
				continue
			}
		}
		if err = s.Repo.MarkCacheInvalidated(ctx, ids); err != nil {
			slog.Error("lucky second cache ack failed", "error", err)
		}
	}
}

type luckySecondContextKey struct{}
type luckySecondParticipation struct {
	attempt string
	refs    atomic.Int64
	service *LuckySecondService
}

func (s *LuckySecondService) Begin(ctx context.Context) context.Context {
	if s == nil || !s.ready.Load() || !s.enabled.Load() {
		return ctx
	}
	attempt := uuid.NewString()
	admitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	ok, err := s.Repo.Begin(admitCtx, s.worker, attempt)
	if err != nil {
		slog.Error("lucky second admission failed", "error", err)
		return ctx
	}
	if !ok {
		return ctx
	}
	p := &luckySecondParticipation{attempt: attempt, service: s}
	p.refs.Store(1)
	return context.WithValue(ctx, luckySecondContextKey{}, p)
}
func LuckySecondAttempt(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if p, ok := ctx.Value(luckySecondContextKey{}).(*luckySecondParticipation); ok {
		return p.attempt
	}
	return ""
}
func CopyLuckySecondContext(parent, base context.Context) context.Context {
	if parent != nil {
		if p, ok := parent.Value(luckySecondContextKey{}).(*luckySecondParticipation); ok {
			return context.WithValue(base, luckySecondContextKey{}, p)
		}
	}
	return base
}
func RetainLuckySecond(ctx context.Context) {
	if ctx != nil {
		if p, ok := ctx.Value(luckySecondContextKey{}).(*luckySecondParticipation); ok {
			p.refs.Add(1)
		}
	}
}
func ReleaseLuckySecond(ctx context.Context) {
	if ctx == nil {
		return
	}
	p, ok := ctx.Value(luckySecondContextKey{}).(*luckySecondParticipation)
	if !ok || p.refs.Add(-1) != 0 {
		return
	}
	done, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.service.Repo.Finish(done, p.attempt); err != nil {
		p.service.unfinished.Store(p.attempt, true)
		slog.Error("lucky second completion failed", "attempt", p.attempt, "error", err)
	}
}
