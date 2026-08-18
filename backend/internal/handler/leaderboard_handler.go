package handler

import (
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	// leaderboardScanLimit 取榜深度：用于定位「当前用户排名」（即便不在展示区内）。
	// total_actual_cost 由 SQL 的 SUM() OVER () 在 LIMIT 之前对全量分组求和，
	// 故无论取多深，总额都是全站口径；这里取较深仅为找到本人名次。
	leaderboardScanLimit = 1000
)

// LeaderboardHandler 面向用户的（只读）消费排行榜。
type LeaderboardHandler struct {
	dashboardService *service.DashboardService
	settingService   *service.SettingService
	rewardRepo       service.LeaderboardRewardRepository
}

// NewLeaderboardHandler 构造用户排行榜处理器。
func NewLeaderboardHandler(
	dashboardService *service.DashboardService,
	settingService *service.SettingService,
	rewardRepo service.LeaderboardRewardRepository,
) *LeaderboardHandler {
	return &LeaderboardHandler{
		dashboardService: dashboardService,
		settingService:   settingService,
		rewardRepo:       rewardRepo,
	}
}

type leaderboardEntry struct {
	Rank       int     `json:"rank"`
	Name       string  `json:"name"` // 脱敏后的标识（本人行为真实邮箱）
	ActualCost float64 `json:"actual_cost"`
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	IsWinner   bool    `json:"is_winner"` // 是否在中奖区（前 N 名且有消费且开启奖励）
	IsMe       bool    `json:"is_me"`
	// RewardAmount 昨日结算的实际发放金额（仅 period=yesterday 的中奖者，>0）。
	RewardAmount float64 `json:"reward_amount,omitempty"`
}

type leaderboardResponse struct {
	Period             string             `json:"period"` // "today" | "yesterday"
	RewardEnabled      bool               `json:"reward_enabled"`
	PoolRate           float64            `json:"pool_rate"`
	TopN               int                `json:"top_n"`
	DistributionMode   string             `json:"distribution_mode"`
	DistributionShares []float64          `json:"distribution_shares,omitempty"` // weighted 模式下前 N 名奖励占比(%)
	TotalCost          float64            `json:"total_cost"`
	PoolAmount         float64            `json:"pool_amount"`
	MinSpend           float64            `json:"min_spend"` // 每人参与门槛（0=无门槛）
	DisplayTopN        int                `json:"display_top_n"`
	Ranking            []leaderboardEntry `json:"ranking"`
	Me                 *leaderboardEntry  `json:"me"`
	Total              int                `json:"total"`
	Page               int                `json:"page"`
	PageSize           int                `json:"page_size"`
	Pages              int                `json:"pages"`
}

type leaderboardSettlement struct {
	rewardsByUser       map[int64]float64
	poolRate            float64
	topN                int
	minSpend            float64
	distributionMode    string
	distributionWeights string
	totalCost           float64
	poolAmount          float64
}

// GetLeaderboard 返回今日/昨日的消费排行榜（脱敏）。
// GET /api/v1/leaderboard?period=today|yesterday
func (h *LeaderboardHandler) GetLeaderboard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ctx := c.Request.Context()
	requestedPage, pageSize := response.ParsePagination(c)

	// 受「用户排行榜展示」开关控制（与是否发奖解耦）。
	if !h.settingService.IsLeaderboardRankingVisibleEnabled(ctx) {
		response.NotFound(c, "leaderboard not available")
		return
	}

	period := c.DefaultQuery("period", "today")
	now := timezone.NowInUserLocation("")
	startOfToday := timezone.StartOfDayInUserLocation(now, "")
	var windowStart, windowEnd time.Time
	switch period {
	case "yesterday":
		windowStart = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -1), "")
		windowEnd = startOfToday
	default:
		period = "today"
		windowStart = startOfToday
		windowEnd = now
	}

	rewardEnabled := h.settingService.IsLeaderboardRewardEnabled(ctx)
	poolRate := h.settingService.GetLeaderboardRewardPoolRate(ctx)
	topN := h.settingService.GetLeaderboardRewardTopN(ctx)
	displayTopN := h.settingService.GetLeaderboardDisplayTopN(ctx)
	minSpend := h.settingService.GetLeaderboardRewardMinSpend(ctx)
	currentMinSpend := minSpend
	mode := h.settingService.GetLeaderboardRewardDistributionMode(ctx)
	weights := h.settingService.GetLeaderboardRewardWeights(ctx)
	var settlement *leaderboardSettlement
	if period == "yesterday" && h.rewardRepo != nil {
		logs, err := h.rewardRepo.ListByDate(ctx, windowStart)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		settlement = newLeaderboardSettlement(logs)
		if settlement != nil {
			rewardEnabled = true
			poolRate = settlement.poolRate
			topN = settlement.topN
			minSpend = leaderboardDisplayMinSpend(settlement, currentMinSpend)
			mode = settlement.distributionMode
			weights = settlement.distributionWeights
		}
	}

	resp := &leaderboardResponse{
		Period:           period,
		RewardEnabled:    rewardEnabled,
		PoolRate:         poolRate,
		TopN:             topN,
		MinSpend:         roundLeaderboard(minSpend),
		DisplayTopN:      displayTopN,
		DistributionMode: mode,
		Ranking:          []leaderboardEntry{},
		Page:             1,
		PageSize:         pageSize,
		Pages:            1,
	}
	// weighted 模式：附带各名次奖励占比，供前端展示实际分配比例。
	if mode == service.LeaderboardRewardModeWeighted {
		resp.DistributionShares = service.LeaderboardWeightedShares(weights, topN)
	}

	scanLimit := leaderboardScanLimit
	if displayTopN > scanLimit {
		scanLimit = displayTopN
	}
	ranking, err := h.dashboardService.GetUserSpendingRanking(ctx, windowStart, windowEnd, scanLimit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if ranking != nil {
		// 排除名单：名单内用户不上榜、不发奖，且其消耗不计入奖池总额。
		// ranking.TotalActualCost 由 SQL 在 LIMIT 前对全站求和（含被排除者），
		// 故从中扣减榜单内可见的被排除者消耗，使总额/奖池口径与展示一致。
		excluded := h.settingService.GetLeaderboardExcludedEmailSet(ctx)
		totalCost := ranking.TotalActualCost
		filtered, totalCost := filterLeaderboardExcludedItems(ranking.Ranking, excluded, totalCost, settlement == nil)
		resp.Total = min(len(filtered), displayTopN)
		page, pages, pageStart, pageEnd := leaderboardPageWindow(requestedPage, pageSize, resp.Total)
		resp.Page = page
		resp.Pages = pages

		if settlement != nil {
			resp.TotalCost = roundLeaderboard(settlement.totalCost)
			resp.PoolAmount = roundLeaderboard(settlement.poolAmount)
		} else {
			resp.TotalCost = roundLeaderboard(totalCost)
		}
		if settlement == nil && rewardEnabled && poolRate > 0 {
			resp.PoolAmount = roundLeaderboard(totalCost * poolRate / 100.0)
		}
		// 展示范围与奖励资格相互独立：公开榜单展示配置的前 N 名并分页，
		// 奖励名额和参与门槛只决定 is_winner。本人仍带回全站名次供前端单独展示。
		winners := 0
		for i, item := range filtered {
			rank := i + 1
			isMe := item.UserID == subject.UserID
			name := maskLeaderboardEmail(item.Email)
			if isMe {
				name = item.Email // 本人行展示真实身份
			}
			qualified := item.ActualCost >= minSpend && item.ActualCost > 0
			rewardAmount := 0.0
			hasSettledReward := false
			if settlement != nil {
				rewardAmount, hasSettledReward = settlement.rewardsByUser[item.UserID]
			}
			isWinner := rewardEnabled && topN > 0 && rank <= topN && qualified
			if settlement != nil {
				isWinner = hasSettledReward
			}
			if settlement == nil && isWinner {
				winners++
			}
			entry := leaderboardEntry{
				Rank:       rank,
				Name:       name,
				ActualCost: roundLeaderboard(item.ActualCost),
				Requests:   item.Requests,
				Tokens:     item.Tokens,
				IsWinner:   isWinner,
				IsMe:       isMe,
			}
			if hasSettledReward {
				entry.RewardAmount = rewardAmount
			}
			if rank > pageStart && rank <= pageEnd {
				resp.Ranking = append(resp.Ranking, entry)
			}
			if isMe {
				meCopy := entry
				resp.Me = &meCopy // 本人始终带回（含全站名次），即使未上榜
			}
		}

		// 昨日结算：就地按发放算法算出各名次实际奖励（设置不变时与已发放一致），填到中奖者行。
		if period == "yesterday" && settlement == nil && rewardEnabled && winners > 0 && resp.PoolAmount > 0 {
			amounts := service.LeaderboardRewardAmounts(winners, mode, weights, resp.PoolAmount)
			for i := range resp.Ranking {
				if e := &resp.Ranking[i]; e.IsWinner && e.Rank-1 < len(amounts) {
					e.RewardAmount = amounts[e.Rank-1]
				}
			}
			if resp.Me != nil && resp.Me.IsWinner && resp.Me.Rank-1 < len(amounts) {
				resp.Me.RewardAmount = amounts[resp.Me.Rank-1]
			}
		}
	}

	response.Success(c, resp)
}

func leaderboardPageWindow(requestedPage, pageSize, total int) (page, pages, start, end int) {
	if pageSize <= 0 {
		pageSize = 20
	}
	pages = max(1, (total+pageSize-1)/pageSize)
	page = requestedPage
	if page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}
	start = (page - 1) * pageSize
	end = min(start+pageSize, total)
	return page, pages, start, end
}

func newLeaderboardSettlement(logs []service.LeaderboardRewardLog) *leaderboardSettlement {
	if len(logs) == 0 {
		return nil
	}
	settlement := &leaderboardSettlement{
		rewardsByUser:    make(map[int64]float64, len(logs)),
		distributionMode: service.LeaderboardRewardModeAverage,
	}
	maxRank := 0
	for i, log := range logs {
		if log.UserID > 0 && log.RewardAmount > 0 {
			settlement.rewardsByUser[log.UserID] = roundLeaderboard(log.RewardAmount)
		}
		if log.Rank > maxRank {
			maxRank = log.Rank
		}
		if i == 0 {
			settlement.poolRate = log.PoolRate
			settlement.topN = log.TopN
			settlement.minSpend = log.MinSpend
			settlement.distributionMode = strings.TrimSpace(log.DistributionMode)
			settlement.distributionWeights = strings.TrimSpace(log.DistributionWeights)
			settlement.totalCost = log.TotalCost
			settlement.poolAmount = log.PoolAmount
			continue
		}
		if settlement.poolRate <= 0 && log.PoolRate > 0 {
			settlement.poolRate = log.PoolRate
		}
		if settlement.topN <= 0 && log.TopN > 0 {
			settlement.topN = log.TopN
		}
		if settlement.minSpend <= 0 && log.MinSpend > 0 {
			settlement.minSpend = log.MinSpend
		}
		if settlement.distributionMode == "" && strings.TrimSpace(log.DistributionMode) != "" {
			settlement.distributionMode = strings.TrimSpace(log.DistributionMode)
		}
		if settlement.distributionWeights == "" && strings.TrimSpace(log.DistributionWeights) != "" {
			settlement.distributionWeights = strings.TrimSpace(log.DistributionWeights)
		}
		if settlement.totalCost <= 0 && log.TotalCost > 0 {
			settlement.totalCost = log.TotalCost
		}
		if settlement.poolAmount <= 0 && log.PoolAmount > 0 {
			settlement.poolAmount = log.PoolAmount
		}
	}
	if settlement.topN <= 0 {
		settlement.topN = maxRank
	}
	if settlement.poolRate <= 0 && settlement.totalCost > 0 && settlement.poolAmount > 0 {
		settlement.poolRate = roundLeaderboard(settlement.poolAmount / settlement.totalCost * 100)
	}
	if settlement.distributionMode == "" {
		settlement.distributionMode = service.LeaderboardRewardModeAverage
	}
	return settlement
}

func leaderboardDisplayMinSpend(settlement *leaderboardSettlement, currentMinSpend float64) float64 {
	if settlement == nil {
		return currentMinSpend
	}
	if settlement.minSpend > 0 {
		return settlement.minSpend
	}
	// 旧结算记录在规则快照迁移前没有 min_spend，只能从当前设置做展示兜底。
	// 新结算记录的非 0 快照仍优先，避免后续配置变更影响昨日榜。
	if currentMinSpend > 0 {
		return currentMinSpend
	}
	return 0
}

func filterLeaderboardExcludedItems(
	items []usagestats.UserSpendingRankingItem,
	excluded map[string]struct{},
	totalCost float64,
	subtractExcludedCost bool,
) ([]usagestats.UserSpendingRankingItem, float64) {
	if len(excluded) == 0 {
		return items, totalCost
	}

	filtered := make([]usagestats.UserSpendingRankingItem, 0, len(items))
	for _, item := range items {
		if service.IsLeaderboardEmailExcluded(item.Email, excluded) {
			if subtractExcludedCost {
				totalCost -= item.ActualCost
			}
			continue
		}
		filtered = append(filtered, item)
	}
	if totalCost < 0 {
		totalCost = 0
	}
	return filtered, totalCost
}

// maskLeaderboardEmail 对邮箱脱敏：保留本地名前 1-2 位 + "***"，域名保留。
// 空邮箱返回 "***"（前端按本地化占位展示）。
func maskLeaderboardEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "***"
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return maskLeaderboardLocal(email)
	}
	return maskLeaderboardLocal(email[:at]) + email[at:]
}

func maskLeaderboardLocal(local string) string {
	r := []rune(local)
	if len(r) <= 2 {
		return string(r[:1]) + "***"
	}
	return string(r[:2]) + "***"
}

func roundLeaderboard(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}
