package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

// GroupRateUsage contains only group-level totals, never user or account data.
type GroupRateUsage struct {
	Requests            int64 `json:"requests"`
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
}

func (u *GroupRateUsage) Add(other GroupRateUsage) {
	u.Requests += other.Requests
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheCreationTokens += other.CacheCreationTokens
	u.CacheReadTokens += other.CacheReadTokens
	u.TotalTokens += other.TotalTokens
}

type GroupRateUsageBucket struct {
	GroupID  int64
	At       time.Time
	Usage    GroupRateUsage
	LastHour GroupRateUsage
}

type GroupRateUsageRepository interface {
	GetGroupRateUsage(context.Context, []int64, time.Time) ([]GroupRateUsageBucket, error)
}

type GroupRateTrendPoint struct {
	At       time.Time `json:"at"`
	Tokens   int64     `json:"tokens"`
	Requests int64     `json:"requests"`
}

type GroupRateBoardItem struct {
	ID                  int64                 `json:"id"`
	Name                string                `json:"name"`
	Platform            string                `json:"platform"`
	SubscriptionType    string                `json:"subscription_type"`
	RateMultiplier      float64               `json:"rate_multiplier"`
	PeakMultiplier      float64               `json:"peak_multiplier"`
	EffectiveMultiplier float64               `json:"effective_multiplier"`
	UserRateMultiplier  *float64              `json:"user_rate_multiplier,omitempty"`
	DynamicRate         GroupDynamicRate      `json:"dynamic_rate"`
	RateUpdatedAt       *time.Time            `json:"rate_updated_at,omitempty"`
	LastHour            GroupRateUsage        `json:"last_hour"`
	Last24Hours         GroupRateUsage        `json:"last_24_hours"`
	Trend               []GroupRateTrendPoint `json:"trend"`
}

type GroupRateBoard struct {
	RatesAt        time.Time            `json:"rates_at"`
	UsageAt        time.Time            `json:"usage_at"`
	RefreshSeconds int                  `json:"refresh_seconds"`
	Groups         []GroupRateBoardItem `json:"groups"`
}

type GroupRatesGroupProvider interface {
	GetAvailableGroups(context.Context, int64) ([]Group, error)
	GetUserGroupRates(context.Context, int64) (map[int64]float64, error)
}

type GroupRatesService struct {
	groups GroupRatesGroupProvider
	usage  GroupRateUsageRepository
	cache  *gocache.Cache
	sf     singleflight.Group
	now    func() time.Time
}

func NewGroupRatesService(groups GroupRatesGroupProvider, usage GroupRateUsageRepository) *GroupRatesService {
	return &GroupRatesService{groups: groups, usage: usage, cache: gocache.New(30*time.Second, time.Minute), now: time.Now}
}

type groupRateUsageSnapshot struct {
	at      time.Time
	buckets []GroupRateUsageBucket
}

func (s *GroupRatesService) Get(ctx context.Context, userID int64) (*GroupRateBoard, error) {
	// Always recheck visibility and published rates. A cached usage result must
	// never restore access to a group after the caller's permission is revoked.
	groups, err := s.groups.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	visible := make([]Group, 0, len(groups))
	ids := make([]int64, 0, len(groups))
	for _, g := range groups {
		if !g.IsActive() {
			continue
		}
		visible = append(visible, g)
		ids = append(ids, g.ID)
	}
	now := s.now().UTC()
	out := &GroupRateBoard{RatesAt: now, UsageAt: now, RefreshSeconds: 30, Groups: make([]GroupRateBoardItem, 0, len(visible))}
	if len(visible) == 0 {
		return out, nil
	}
	userRates, err := s.groups.GetUserGroupRates(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.usage == nil {
		return nil, fmt.Errorf("group rate usage repository unavailable")
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	key := strings.Join(parts, ",")
	value, err, _ := s.sf.Do(key, func() (any, error) {
		if cached, ok := s.cache.Get(key); ok {
			return cached, nil
		}
		at := s.now().UTC()
		buckets, err := s.usage.GetGroupRateUsage(ctx, ids, at)
		if err != nil {
			return nil, err
		}
		snapshot := groupRateUsageSnapshot{at: at, buckets: buckets}
		s.cache.SetDefault(key, snapshot)
		return snapshot, nil
	})
	if err != nil {
		return nil, err
	}
	snapshot := value.(groupRateUsageSnapshot)
	out.UsageAt = snapshot.at
	byGroup := make(map[int64][]GroupRateUsageBucket)
	for _, bucket := range snapshot.buckets {
		byGroup[bucket.GroupID] = append(byGroup[bucket.GroupID], bucket)
	}
	start := snapshot.at.Add(-24 * time.Hour).Truncate(time.Hour)
	end := snapshot.at.Truncate(time.Hour)
	for _, g := range visible {
		peak := g.PeakMultiplierAt(now)
		item := GroupRateBoardItem{
			ID: g.ID, Name: g.Name, Platform: g.Platform, SubscriptionType: g.SubscriptionType,
			RateMultiplier: g.RateMultiplier, PeakMultiplier: peak, EffectiveMultiplier: g.RateMultiplier * peak,
			DynamicRate: g.DynamicRate, RateUpdatedAt: g.DynamicRateUpdatedAt,
			Trend: make([]GroupRateTrendPoint, 0, 25),
		}
		if override, ok := userRates[g.ID]; ok {
			item.UserRateMultiplier = &override
			item.EffectiveMultiplier = override * peak
		}
		for at := start; !at.After(end); at = at.Add(time.Hour) {
			item.Trend = append(item.Trend, GroupRateTrendPoint{At: at})
		}
		for _, bucket := range byGroup[g.ID] {
			item.Last24Hours.Add(bucket.Usage)
			item.LastHour.Add(bucket.LastHour)
			index := int(bucket.At.Sub(start) / time.Hour)
			if index >= 0 && index < len(item.Trend) {
				item.Trend[index].Tokens += bucket.Usage.TotalTokens
				item.Trend[index].Requests += bucket.Usage.Requests
			}
		}
		out.Groups = append(out.Groups, item)
	}
	return out, nil
}
