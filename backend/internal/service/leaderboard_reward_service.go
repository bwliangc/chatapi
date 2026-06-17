package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// LeaderboardRewardGrantItem 表示发给单个用户的奖励项。
type LeaderboardRewardGrantItem struct {
	UserID int64
	Rank   int     // 排行榜名次，从 1 开始
	Amount float64 // 发放到余额的金额
	Email  string  // 中奖用户邮箱（用于可选的邮件通知）
}

// LeaderboardRewardGrant 表示一次每日激励发放的全部信息。
type LeaderboardRewardGrant struct {
	RewardDate       time.Time // 结算的消费日期（即“昨天”），仅日期部分有意义
	PoolAmount       float64   // 奖池总额（昨日总消费 × 比例）
	TotalCost        float64   // 当日全站总消费（actual_cost）
	DistributionMode string    // 分配模式
	Items            []LeaderboardRewardGrantItem
}

// LeaderboardRewardRepository 排行榜激励发放的持久化接口。
type LeaderboardRewardRepository interface {
	// HasSettled 报告给定消费日期是否已经发放过（幂等去重依据）。
	HasSettled(ctx context.Context, rewardDate time.Time) (bool, error)
	// GrantRewards 在单个事务内为每个用户加余额并写入发放记录；
	// (reward_date, user_id) 唯一约束保证重复发放会整体回滚。返回实际发放人数。
	GrantRewards(ctx context.Context, grant LeaderboardRewardGrant) (int, error)
}

// LeaderboardRewardService 周期性按昨日消费排行榜发放余额奖励。
type LeaderboardRewardService struct {
	rewardRepo               LeaderboardRewardRepository
	settingService           *SettingService
	dashboardService         *DashboardService
	billingCacheService      *BillingCacheService
	notificationEmailService *NotificationEmailService
	interval                 time.Duration
	stopCh                   chan struct{}
	stopOnce                 sync.Once
	wg                       sync.WaitGroup
}

// NewLeaderboardRewardService 构造排行榜激励服务。interval 为巡检间隔（建议 1 小时）。
func NewLeaderboardRewardService(
	rewardRepo LeaderboardRewardRepository,
	settingService *SettingService,
	dashboardService *DashboardService,
	billingCacheService *BillingCacheService,
	interval time.Duration,
	notificationEmailService *NotificationEmailService,
) *LeaderboardRewardService {
	return &LeaderboardRewardService{
		rewardRepo:               rewardRepo,
		settingService:           settingService,
		dashboardService:         dashboardService,
		billingCacheService:      billingCacheService,
		notificationEmailService: notificationEmailService,
		interval:                 interval,
		stopCh:                   make(chan struct{}),
	}
}

// Start 启动后台巡检循环。启动时先立即执行一次（补发昨日未结算的奖励）。
func (s *LeaderboardRewardService) Start() {
	if s == nil || s.rewardRepo == nil || s.settingService == nil || s.dashboardService == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop 停止后台巡检并等待退出。
func (s *LeaderboardRewardService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

// runOnce 结算“昨天”的排行榜激励（若启用且尚未结算）。
func (s *LeaderboardRewardService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !s.settingService.IsLeaderboardRewardEnabled(ctx) {
		return
	}
	rate := s.settingService.GetLeaderboardRewardPoolRate(ctx)
	topN := s.settingService.GetLeaderboardRewardTopN(ctx)
	if rate <= 0 || topN <= 0 {
		return
	}
	mode := s.settingService.GetLeaderboardRewardDistributionMode(ctx)
	weights := s.settingService.GetLeaderboardRewardWeights(ctx)

	// 用服务器时区计算“昨天” [startOfYesterday, startOfToday)。
	now := timezone.NowInUserLocation("")
	startOfToday := timezone.StartOfDayInUserLocation(now, "")
	startOfYesterday := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -1), "")
	rewardDate := startOfYesterday

	settled, err := s.rewardRepo.HasSettled(ctx, rewardDate)
	if err != nil {
		log.Printf("[LeaderboardReward] check settled for %s failed: %v", rewardDate.Format("2006-01-02"), err)
		return
	}
	if settled {
		return
	}

	// 取昨日所有用户消费（按 actual_cost 降序，limit=0 不限），既算全站总消费，也选前 N 名。
	items, err := s.dashboardService.GetUserBreakdownStats(
		ctx, startOfYesterday, startOfToday,
		usagestats.UserBreakdownDimension{SortBy: "actual_cost"}, 0,
	)
	if err != nil {
		log.Printf("[LeaderboardReward] get user breakdown for %s failed: %v", rewardDate.Format("2006-01-02"), err)
		return
	}

	var totalCost float64
	for _, it := range items {
		if it.ActualCost > 0 {
			totalCost += it.ActualCost
		}
	}
	if totalCost <= 0 {
		return // 昨日无消费，无奖池
	}

	pool := roundTo(totalCost*rate/100.0, 6)
	if pool <= 0 {
		return
	}

	// 参与门槛：仅个人消费 ≥ 门槛的用户进入中奖区（门槛只过滤获奖资格，奖池仍按全站总消费计）。
	minSpend := s.settingService.GetLeaderboardRewardMinSpend(ctx)
	eligible := items
	if minSpend > 0 {
		eligible = make([]usagestats.UserBreakdownItem, 0, len(items))
		for _, it := range items {
			if it.ActualCost >= minSpend {
				eligible = append(eligible, it)
			}
		}
	}

	grantItems := buildLeaderboardRewardItems(eligible, topN, mode, weights, pool)
	if len(grantItems) == 0 {
		return
	}

	granted, err := s.rewardRepo.GrantRewards(ctx, LeaderboardRewardGrant{
		RewardDate:       rewardDate,
		PoolAmount:       pool,
		TotalCost:        roundTo(totalCost, 6),
		DistributionMode: mode,
		Items:            grantItems,
	})
	if err != nil {
		log.Printf("[LeaderboardReward] grant rewards for %s failed: %v", rewardDate.Format("2006-01-02"), err)
		return
	}

	// 失效受益用户的余额缓存，确保扣费立即看到新余额。
	if s.billingCacheService != nil {
		for _, it := range grantItems {
			if err := s.billingCacheService.InvalidateUserBalance(ctx, it.UserID); err != nil {
				log.Printf("[LeaderboardReward] invalidate balance cache for user %d failed: %v", it.UserID, err)
			}
		}
	}

	// 可选：向中奖用户发送邮件通知（受管理员开关控制，发送失败仅记录不影响发放）。
	if s.notificationEmailService != nil && s.settingService.IsLeaderboardRewardEmailNotifyEnabled(ctx) {
		for _, it := range grantItems {
			if strings.TrimSpace(it.Email) == "" {
				continue
			}
			if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
				Event:          NotificationEmailEventLeaderboardReward,
				RecipientEmail: it.Email,
				RecipientName:  it.Email,
				UserID:         it.UserID,
				Variables: map[string]string{
					"reward_amount": fmt.Sprintf("%.2f", it.Amount),
					"rank":          strconv.Itoa(it.Rank),
					"reward_date":   rewardDate.Format("2006-01-02"),
				},
			}); err != nil {
				log.Printf("[LeaderboardReward] send reward email to user %d failed: %v", it.UserID, err)
			}
		}
	}

	log.Printf("[LeaderboardReward] settled %s: total_cost=%.6f pool=%.6f winners=%d mode=%s",
		rewardDate.Format("2006-01-02"), totalCost, pool, granted, mode)
}

// buildLeaderboardRewardItems 按分配模式把奖池分给前 N 名（仅纳入昨日有消费的用户）。
// 金额四舍五入到 6 位小数，因取整产生的余数补给第 1 名，保证发放总额 == pool。
func buildLeaderboardRewardItems(items []usagestats.UserBreakdownItem, topN int, mode, weightsRaw string, pool float64) []LeaderboardRewardGrantItem {
	winners := make([]usagestats.UserBreakdownItem, 0, topN)
	for _, it := range items {
		if it.ActualCost <= 0 || it.UserID <= 0 {
			continue
		}
		winners = append(winners, it)
		if len(winners) >= topN {
			break
		}
	}
	n := len(winners)
	if n == 0 {
		return nil
	}

	amounts := make([]float64, n)
	switch mode {
	case LeaderboardRewardModeWeighted:
		weights := parseLeaderboardRewardWeights(weightsRaw)
		ws := make([]float64, n)
		var sumW float64
		for i := 0; i < n; i++ {
			if i < len(weights) && weights[i] > 0 {
				ws[i] = weights[i]
			}
			sumW += ws[i]
		}
		if sumW <= 0 {
			fillAverage(amounts, pool)
		} else {
			for i := 0; i < n; i++ {
				amounts[i] = pool * ws[i] / sumW
			}
		}
	default: // average
		fillAverage(amounts, pool)
	}

	// 四舍五入并把余数补给第 1 名，保证总额守恒。
	rounded := make([]float64, n)
	var sumRounded float64
	for i := 0; i < n; i++ {
		rounded[i] = roundTo(amounts[i], 6)
		sumRounded += rounded[i]
	}
	rounded[0] = roundTo(rounded[0]+roundTo(pool-sumRounded, 6), 6)

	result := make([]LeaderboardRewardGrantItem, 0, n)
	for i := 0; i < n; i++ {
		if rounded[i] <= 0 {
			continue
		}
		result = append(result, LeaderboardRewardGrantItem{
			UserID: winners[i].UserID,
			Rank:   i + 1,
			Amount: rounded[i],
			Email:  winners[i].Email,
		})
	}
	return result
}

func fillAverage(amounts []float64, pool float64) {
	n := len(amounts)
	if n == 0 {
		return
	}
	share := pool / float64(n)
	for i := range amounts {
		amounts[i] = share
	}
}

// parseLeaderboardRewardWeights 解析逗号分隔的权重字符串，非法项按 0 处理。
func parseLeaderboardRewardWeights(raw string) []float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	weights := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			v = 0
		}
		weights = append(weights, v)
	}
	return weights
}

// LeaderboardWeightedShares 返回 weighted 模式下前 topN 名各自的奖励占比（百分比，0-100，
// 保留 2 位小数），算法与 buildLeaderboardRewardItems 的 weighted 分支一致：
// ws[i] = weights[i]（越界或 <=0 记 0），share[i] = ws[i] / Σws * 100。
// 若 topN<=0 或权重合计 <=0（实际退化为平均分配），返回 nil。
func LeaderboardWeightedShares(weightsRaw string, topN int) []float64 {
	if topN <= 0 {
		return nil
	}
	weights := parseLeaderboardRewardWeights(weightsRaw)
	ws := make([]float64, topN)
	var sumW float64
	for i := 0; i < topN; i++ {
		if i < len(weights) && weights[i] > 0 {
			ws[i] = weights[i]
		}
		sumW += ws[i]
	}
	if sumW <= 0 {
		return nil
	}
	shares := make([]float64, topN)
	for i := 0; i < topN; i++ {
		shares[i] = roundTo(ws[i]/sumW*100, 2)
	}
	return shares
}

// LeaderboardRewardAmounts 返回前 n 名各自的发放金额（按名次 1..n 顺序），按分配模式分配奖池，
// 与 buildLeaderboardRewardItems 完全一致（四舍五入到 6 位，取整余数补给第 1 名，总额 == pool）。
// 用于「昨日结算」展示各名次实际奖励（设置不变时与已发放金额一致）。
func LeaderboardRewardAmounts(n int, mode, weightsRaw string, pool float64) []float64 {
	if n <= 0 || pool <= 0 {
		return nil
	}
	amounts := make([]float64, n)
	if mode == LeaderboardRewardModeWeighted {
		weights := parseLeaderboardRewardWeights(weightsRaw)
		ws := make([]float64, n)
		var sumW float64
		for i := 0; i < n; i++ {
			if i < len(weights) && weights[i] > 0 {
				ws[i] = weights[i]
			}
			sumW += ws[i]
		}
		if sumW <= 0 {
			fillAverage(amounts, pool)
		} else {
			for i := 0; i < n; i++ {
				amounts[i] = pool * ws[i] / sumW
			}
		}
	} else {
		fillAverage(amounts, pool)
	}
	var sumRounded float64
	for i := 0; i < n; i++ {
		amounts[i] = roundTo(amounts[i], 6)
		sumRounded += amounts[i]
	}
	amounts[0] = roundTo(amounts[0]+roundTo(pool-sumRounded, 6), 6)
	return amounts
}
