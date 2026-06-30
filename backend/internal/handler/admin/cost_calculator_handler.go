package admin

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type costCalculatorUsageSummaryReader interface {
	GetCostCalculatorUsageSummary(ctx context.Context, startTime, endTime time.Time, accountUsageRates map[int64]float64, defaultUsageRate float64, excludeAdmins bool) (*service.CostCalculatorUsageSummary, error)
}

type costCalculatorBalanceRechargeSummaryReader interface {
	GetCostCalculatorBalanceRechargeSummaryWithPackages(ctx context.Context, startTime, endTime time.Time, packages []service.CostCalculatorBalanceRechargePackage, excludeAdmins bool) (*service.CostCalculatorBalanceRechargeSummary, error)
}

type CostCalculatorHandler struct {
	settingService  *service.SettingService
	usageSummary    costCalculatorUsageSummaryReader
	rechargeSummary costCalculatorBalanceRechargeSummaryReader
}

func NewCostCalculatorHandler(settingService *service.SettingService, usageRepo service.UsageLogRepository, redeemRepo service.RedeemCodeRepository) *CostCalculatorHandler {
	var summaryReader costCalculatorUsageSummaryReader
	if repo, ok := usageRepo.(costCalculatorUsageSummaryReader); ok {
		summaryReader = repo
	}
	var rechargeReader costCalculatorBalanceRechargeSummaryReader
	if repo, ok := redeemRepo.(costCalculatorBalanceRechargeSummaryReader); ok {
		rechargeReader = repo
	}
	return &CostCalculatorHandler{settingService: settingService, usageSummary: summaryReader, rechargeSummary: rechargeReader}
}

type costCalculatorConfigRequest struct {
	BalanceExchangeRate     *float64                                        `json:"balance_exchange_rate"`
	UpstreamCostRate        *float64                                        `json:"upstream_cost_rate"`
	AccountCosts            *[]service.CostCalculatorAccountCost            `json:"account_costs"`
	BalanceRechargePackages *[]service.CostCalculatorBalanceRechargePackage `json:"balance_recharge_packages"`
}

func (h *CostCalculatorHandler) GetConfig(c *gin.Context) {
	cfg, err := h.settingService.GetCostCalculatorConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *CostCalculatorHandler) UpdateConfig(c *gin.Context) {
	var req costCalculatorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.settingService.UpdateCostCalculatorConfig(c.Request.Context(), service.UpdateCostCalculatorConfigInput{
		BalanceExchangeRate:     req.BalanceExchangeRate,
		UpstreamCostRate:        req.UpstreamCostRate,
		AccountCosts:            req.AccountCosts,
		BalanceRechargePackages: req.BalanceRechargePackages,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, cfg)
}

func (h *CostCalculatorHandler) GetUsageSummary(c *gin.Context) {
	if h.usageSummary == nil {
		response.Error(c, 500, "Cost calculator usage summary is unavailable")
		return
	}

	startTime, endTime := parseTimeRange(c)
	cfg, err := h.settingService.GetCostCalculatorConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	defaultUsageRate := cfg.UpstreamCostRate
	accountUsageRates := make(map[int64]float64, len(cfg.AccountCosts))
	for _, item := range cfg.AccountCosts {
		if item.UsageCostRate != nil {
			accountUsageRates[item.AccountID] = *item.UsageCostRate
		}
	}

	excludeAdmins := parseBoolQueryWithDefault(c.Query("exclude_admins"), true)
	summary, err := h.usageSummary.GetCostCalculatorUsageSummary(c.Request.Context(), startTime, endTime, accountUsageRates, defaultUsageRate, excludeAdmins)
	if err != nil {
		logger.LegacyPrintf("handler.admin.cost_calculator", "get usage summary failed: start=%s end=%s exclude_admins=%v err=%v", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), excludeAdmins, err)
		response.Error(c, 500, "Failed to get cost calculator usage summary")
		return
	}
	summary.StartDate = startTime.Format("2006-01-02")
	summary.EndDate = endTime.Add(-24 * time.Hour).Format("2006-01-02")
	response.Success(c, summary)
}

func (h *CostCalculatorHandler) GetBalanceRechargeSummary(c *gin.Context) {
	if h.rechargeSummary == nil {
		response.Error(c, 500, "Cost calculator balance recharge summary is unavailable")
		return
	}

	startTime, endTime := parseTimeRange(c)
	cfg, err := h.settingService.GetCostCalculatorConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	excludeAdmins := parseBoolQueryWithDefault(c.Query("exclude_admins"), true)
	summary, err := h.rechargeSummary.GetCostCalculatorBalanceRechargeSummaryWithPackages(c.Request.Context(), startTime, endTime, cfg.BalanceRechargePackages, excludeAdmins)
	if err != nil {
		logger.LegacyPrintf("handler.admin.cost_calculator", "get balance recharge summary failed: start=%s end=%s exclude_admins=%v err=%v", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), excludeAdmins, err)
		response.Error(c, 500, "Failed to get cost calculator balance recharge summary")
		return
	}
	response.Success(c, summary)
}
