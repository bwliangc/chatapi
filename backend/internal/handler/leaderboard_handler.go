package handler

import (
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	// leaderboardDisplayLimit 榜单对外展示的最大名次数。
	leaderboardDisplayLimit = 20
	// leaderboardScanLimit 取榜深度：用于定位「当前用户排名」（即便不在展示区内）。
	// total_actual_cost 由 SQL 的 SUM() OVER () 在 LIMIT 之前对全量分组求和，
	// 故无论取多深，总额都是全站口径；这里取较深仅为找到本人名次。
	leaderboardScanLimit = 1000
)

// LeaderboardHandler 面向用户的（只读）消费排行榜。
type LeaderboardHandler struct {
	dashboardService *service.DashboardService
	settingService   *service.SettingService
}

// NewLeaderboardHandler 构造用户排行榜处理器。
func NewLeaderboardHandler(dashboardService *service.DashboardService, settingService *service.SettingService) *LeaderboardHandler {
	return &LeaderboardHandler{
		dashboardService: dashboardService,
		settingService:   settingService,
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
	Ranking            []leaderboardEntry `json:"ranking"`
	Me                 *leaderboardEntry  `json:"me"`
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
	minSpend := h.settingService.GetLeaderboardRewardMinSpend(ctx)
	mode := h.settingService.GetLeaderboardRewardDistributionMode(ctx)
	weights := h.settingService.GetLeaderboardRewardWeights(ctx)

	resp := &leaderboardResponse{
		Period:           period,
		RewardEnabled:    rewardEnabled,
		PoolRate:         poolRate,
		TopN:             topN,
		MinSpend:         roundLeaderboard(minSpend),
		DistributionMode: mode,
		Ranking:          []leaderboardEntry{},
	}
	// weighted 模式：附带各名次奖励占比，供前端展示实际分配比例。
	if mode == service.LeaderboardRewardModeWeighted {
		resp.DistributionShares = service.LeaderboardWeightedShares(weights, topN)
	}

	ranking, err := h.dashboardService.GetUserSpendingRanking(ctx, windowStart, windowEnd, leaderboardScanLimit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if ranking != nil {
		resp.TotalCost = roundLeaderboard(ranking.TotalActualCost)
		if rewardEnabled && poolRate > 0 {
			resp.PoolAmount = roundLeaderboard(ranking.TotalActualCost * poolRate / 100.0)
		}
		// 榜单按消费降序：达标用户（actual_cost ≥ 门槛）必然是顶部连续的一段，未达门槛者
		// 一定排在更后、金额更小，不存在「跳过未达者再由后面递补」的情况。
		// 故中奖区 = 前 topN 名且达标；榜单只展示达标用户；本人始终带回全站名次供前端单独展示。
		applyThreshold := rewardEnabled && minSpend > 0
		winners := 0
		for i, item := range ranking.Ranking {
			rank := i + 1
			isMe := item.UserID == subject.UserID
			name := maskLeaderboardEmail(item.Email)
			if isMe {
				name = item.Email // 本人行展示真实身份
			}
			qualified := item.ActualCost >= minSpend && item.ActualCost > 0
			isWinner := rewardEnabled && topN > 0 && rank <= topN && qualified
			if isWinner {
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
			if (!applyThreshold || qualified) && len(resp.Ranking) < leaderboardDisplayLimit {
				resp.Ranking = append(resp.Ranking, entry)
			}
			if isMe {
				meCopy := entry
				resp.Me = &meCopy // 本人始终带回（含全站名次），即使未上榜
			}
		}

		// 昨日结算：就地按发放算法算出各名次实际奖励（设置不变时与已发放一致），填到中奖者行。
		if period == "yesterday" && rewardEnabled && winners > 0 && resp.PoolAmount > 0 {
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
