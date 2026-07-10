package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const SettingKeyCostCalculatorConfig = "cost_calculator_config"

type CostCalculatorAccountCost struct {
	AccountID         int64    `json:"account_id"`
	AccountName       string   `json:"account_name"`
	Platform          string   `json:"platform"`
	MonthlyCost       float64  `json:"monthly_cost"`
	UsageCostRate     *float64 `json:"usage_cost_rate,omitempty"`
	MonthlyCostLabel  string   `json:"monthly_cost_label,omitempty"`
	FixedCostStartsAt string   `json:"fixed_cost_starts_at,omitempty"`
	FixedCostEndsAt   string   `json:"fixed_cost_ends_at,omitempty"`
	EffectiveFrom     string   `json:"effective_from,omitempty"`
	EffectiveUntil    string   `json:"effective_until,omitempty"`
}

type CostCalculatorUsageRatePeriod struct {
	AccountID      int64
	UsageCostRate  float64
	EffectiveFrom  *time.Time
	EffectiveUntil *time.Time
}

type CostCalculatorBalanceRechargePackage struct {
	BalanceAmount float64 `json:"balance_amount"`
	ActualAmount  float64 `json:"actual_amount"`
}

type CostCalculatorConfig struct {
	BalanceExchangeRate     float64                                `json:"balance_exchange_rate"`
	UpstreamCostRate        float64                                `json:"upstream_cost_rate"`
	AccountCosts            []CostCalculatorAccountCost            `json:"account_costs"`
	AccountCostHistory      []CostCalculatorAccountCost            `json:"account_cost_history,omitempty"`
	BalanceRechargePackages []CostCalculatorBalanceRechargePackage `json:"balance_recharge_packages"`
}

type UpdateCostCalculatorConfigInput struct {
	BalanceExchangeRate     *float64
	UpstreamCostRate        *float64
	AccountCosts            *[]CostCalculatorAccountCost
	BalanceRechargePackages *[]CostCalculatorBalanceRechargePackage
}

const (
	defaultCostCalculatorBalanceExchangeRate = 1.0
	defaultCostCalculatorUpstreamCostRate    = 1.0
	maxCostCalculatorAccountCosts            = 1000
	maxCostCalculatorAccountCostHistory      = 10000
	maxCostCalculatorBalanceRechargePackages = 200
	maxCostCalculatorLabelLength             = 120
)

func DefaultCostCalculatorConfig() CostCalculatorConfig {
	return CostCalculatorConfig{
		BalanceExchangeRate:     defaultCostCalculatorBalanceExchangeRate,
		UpstreamCostRate:        defaultCostCalculatorUpstreamCostRate,
		AccountCosts:            []CostCalculatorAccountCost{},
		AccountCostHistory:      []CostCalculatorAccountCost{},
		BalanceRechargePackages: []CostCalculatorBalanceRechargePackage{},
	}
}

func (s *SettingService) GetCostCalculatorConfig(ctx context.Context) (*CostCalculatorConfig, error) {
	cfg := DefaultCostCalculatorConfig()
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyCostCalculatorConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("get cost calculator config: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return &cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("parse cost calculator config: %w", err)
	}
	normalized, err := normalizeCostCalculatorConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (s *SettingService) UpdateCostCalculatorConfig(ctx context.Context, input UpdateCostCalculatorConfigInput) (*CostCalculatorConfig, error) {
	current, err := s.GetCostCalculatorConfig(ctx)
	if err != nil {
		return nil, err
	}
	next := *current
	if input.BalanceExchangeRate != nil {
		next.BalanceExchangeRate = *input.BalanceExchangeRate
	}
	if input.UpstreamCostRate != nil {
		next.UpstreamCostRate = *input.UpstreamCostRate
	}
	if input.AccountCosts != nil {
		next.AccountCosts = append([]CostCalculatorAccountCost(nil), (*input.AccountCosts)...)
	}
	if input.BalanceRechargePackages != nil {
		next.BalanceRechargePackages = append([]CostCalculatorBalanceRechargePackage(nil), (*input.BalanceRechargePackages)...)
	}
	normalized, err := normalizeCostCalculatorConfig(next)
	if err != nil {
		return nil, err
	}
	if input.AccountCosts != nil {
		versionCostCalculatorAccountCosts(*current, &normalized, time.Now().UTC())
		if normalized, err = normalizeCostCalculatorConfig(normalized); err != nil {
			return nil, err
		}
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal cost calculator config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyCostCalculatorConfig, string(data)); err != nil {
		return nil, fmt.Errorf("save cost calculator config: %w", err)
	}
	return &normalized, nil
}

func normalizeCostCalculatorConfig(cfg CostCalculatorConfig) (CostCalculatorConfig, error) {
	if !isFinitePositive(cfg.BalanceExchangeRate) {
		return CostCalculatorConfig{}, fmt.Errorf("balance_exchange_rate must be > 0")
	}
	if !isFiniteNonNegative(cfg.UpstreamCostRate) {
		return CostCalculatorConfig{}, fmt.Errorf("upstream_cost_rate must be >= 0")
	}
	if len(cfg.AccountCosts) > maxCostCalculatorAccountCosts {
		return CostCalculatorConfig{}, fmt.Errorf("account_costs cannot exceed %d", maxCostCalculatorAccountCosts)
	}
	if len(cfg.AccountCostHistory) > maxCostCalculatorAccountCostHistory {
		return CostCalculatorConfig{}, fmt.Errorf("account_cost_history cannot exceed %d", maxCostCalculatorAccountCostHistory)
	}
	if len(cfg.BalanceRechargePackages) > maxCostCalculatorBalanceRechargePackages {
		return CostCalculatorConfig{}, fmt.Errorf("balance_recharge_packages cannot exceed %d", maxCostCalculatorBalanceRechargePackages)
	}
	seen := make(map[int64]struct{}, len(cfg.AccountCosts))
	costs := make([]CostCalculatorAccountCost, 0, len(cfg.AccountCosts))
	for _, item := range cfg.AccountCosts {
		var err error
		item, err = normalizeCostCalculatorAccountCost(item, false, cfg.UpstreamCostRate)
		if err != nil {
			return CostCalculatorConfig{}, err
		}
		if _, ok := seen[item.AccountID]; ok {
			return CostCalculatorConfig{}, fmt.Errorf("duplicate account_id: %d", item.AccountID)
		}
		seen[item.AccountID] = struct{}{}
		costs = append(costs, item)
	}
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].AccountID < costs[j].AccountID
	})

	history := make([]CostCalculatorAccountCost, 0, len(cfg.AccountCostHistory))
	for _, item := range cfg.AccountCostHistory {
		var err error
		item, err = normalizeCostCalculatorAccountCost(item, true, cfg.UpstreamCostRate)
		if err != nil {
			return CostCalculatorConfig{}, err
		}
		history = append(history, item)
	}
	sort.SliceStable(history, func(i, j int) bool {
		if history[i].AccountID != history[j].AccountID {
			return history[i].AccountID < history[j].AccountID
		}
		return history[i].EffectiveUntil < history[j].EffectiveUntil
	})

	seenPackages := make(map[int64]struct{}, len(cfg.BalanceRechargePackages))
	packages := make([]CostCalculatorBalanceRechargePackage, 0, len(cfg.BalanceRechargePackages))
	for _, item := range cfg.BalanceRechargePackages {
		if !isFinitePositive(item.BalanceAmount) {
			return CostCalculatorConfig{}, fmt.Errorf("balance_recharge_packages.balance_amount must be > 0")
		}
		if !isFiniteNonNegative(item.ActualAmount) {
			return CostCalculatorConfig{}, fmt.Errorf("balance_recharge_packages.actual_amount must be >= 0")
		}
		key := costCalculatorAmountKey(item.BalanceAmount)
		if _, ok := seenPackages[key]; ok {
			return CostCalculatorConfig{}, fmt.Errorf("duplicate balance_recharge_packages.balance_amount: %g", item.BalanceAmount)
		}
		seenPackages[key] = struct{}{}
		packages = append(packages, item)
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].BalanceAmount < packages[j].BalanceAmount
	})
	return CostCalculatorConfig{
		BalanceExchangeRate:     cfg.BalanceExchangeRate,
		UpstreamCostRate:        cfg.UpstreamCostRate,
		AccountCosts:            costs,
		AccountCostHistory:      history,
		BalanceRechargePackages: packages,
	}, nil
}

func normalizeCostCalculatorAccountCost(item CostCalculatorAccountCost, history bool, defaultUsageRate float64) (CostCalculatorAccountCost, error) {
	if item.AccountID <= 0 {
		return CostCalculatorAccountCost{}, fmt.Errorf("account_id must be > 0")
	}
	if !isFiniteNonNegative(item.MonthlyCost) {
		return CostCalculatorAccountCost{}, fmt.Errorf("monthly_cost must be >= 0")
	}
	if item.UsageCostRate != nil && !isFiniteNonNegative(*item.UsageCostRate) {
		return CostCalculatorAccountCost{}, fmt.Errorf("usage_cost_rate must be >= 0")
	}
	if item.UsageCostRate == nil {
		rate := defaultUsageRate
		item.UsageCostRate = &rate
	}
	item.AccountName = trimCostCalculatorText(item.AccountName)
	item.Platform = trimCostCalculatorText(item.Platform)
	item.MonthlyCostLabel = trimCostCalculatorText(item.MonthlyCostLabel)
	item.FixedCostStartsAt = trimCostCalculatorText(item.FixedCostStartsAt)
	item.FixedCostEndsAt = trimCostCalculatorText(item.FixedCostEndsAt)
	item.EffectiveFrom = trimCostCalculatorText(item.EffectiveFrom)
	item.EffectiveUntil = trimCostCalculatorText(item.EffectiveUntil)
	if len(item.MonthlyCostLabel) > maxCostCalculatorLabelLength {
		return CostCalculatorAccountCost{}, fmt.Errorf("monthly_cost_label is too long")
	}
	if err := validateCostCalculatorDateRange(item.FixedCostStartsAt, item.FixedCostEndsAt); err != nil {
		return CostCalculatorAccountCost{}, err
	}
	from, err := parseCostCalculatorEffectiveTime("effective_from", item.EffectiveFrom, false)
	if err != nil {
		return CostCalculatorAccountCost{}, err
	}
	until, err := parseCostCalculatorEffectiveTime("effective_until", item.EffectiveUntil, history)
	if err != nil {
		return CostCalculatorAccountCost{}, err
	}
	if !history {
		item.EffectiveUntil = ""
		until = time.Time{}
	}
	if !from.IsZero() && !until.IsZero() && !from.Before(until) {
		return CostCalculatorAccountCost{}, fmt.Errorf("effective_from must be before effective_until")
	}
	return item, nil
}

func parseCostCalculatorEffectiveTime(field, value string, required bool) (time.Time, error) {
	if value == "" {
		if required {
			return time.Time{}, fmt.Errorf("%s is required for account cost history", field)
		}
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use RFC3339", field)
	}
	return parsed, nil
}

func versionCostCalculatorAccountCosts(current CostCalculatorConfig, next *CostCalculatorConfig, effectiveAt time.Time) {
	if next == nil {
		return
	}
	effectiveAt = effectiveAt.UTC()
	effectiveAtText := effectiveAt.Format(time.RFC3339Nano)
	currentByID := make(map[int64]CostCalculatorAccountCost, len(current.AccountCosts))
	for _, item := range current.AccountCosts {
		currentByID[item.AccountID] = item
	}
	nextByID := make(map[int64]struct{}, len(next.AccountCosts))
	history := append([]CostCalculatorAccountCost(nil), current.AccountCostHistory...)
	for i := range next.AccountCosts {
		item := &next.AccountCosts[i]
		nextByID[item.AccountID] = struct{}{}
		old, exists := currentByID[item.AccountID]
		switch {
		case !exists:
			item.EffectiveFrom = effectiveAtText
		case costCalculatorAccountCostValuesEqual(old, *item):
			item.EffectiveFrom = old.EffectiveFrom
		case exists:
			history = appendCostCalculatorAccountCostHistory(history, old, effectiveAtText)
			item.EffectiveFrom = effectiveAtText
		}
		item.EffectiveUntil = ""
	}
	for _, old := range current.AccountCosts {
		if _, exists := nextByID[old.AccountID]; !exists {
			history = appendCostCalculatorAccountCostHistory(history, old, effectiveAtText)
		}
	}
	next.AccountCostHistory = history
}

func appendCostCalculatorAccountCostHistory(history []CostCalculatorAccountCost, item CostCalculatorAccountCost, effectiveUntil string) []CostCalculatorAccountCost {
	if item.EffectiveFrom != "" {
		from, fromErr := time.Parse(time.RFC3339Nano, item.EffectiveFrom)
		until, untilErr := time.Parse(time.RFC3339Nano, effectiveUntil)
		if fromErr == nil && untilErr == nil && !from.Before(until) {
			return history
		}
	}
	item.EffectiveUntil = effectiveUntil
	return append(history, item)
}

func costCalculatorAccountCostValuesEqual(a, b CostCalculatorAccountCost) bool {
	return a.AccountID == b.AccountID &&
		a.AccountName == b.AccountName &&
		a.Platform == b.Platform &&
		a.MonthlyCost == b.MonthlyCost &&
		costCalculatorOptionalFloatEqual(a.UsageCostRate, b.UsageCostRate) &&
		a.MonthlyCostLabel == b.MonthlyCostLabel &&
		a.FixedCostStartsAt == b.FixedCostStartsAt &&
		a.FixedCostEndsAt == b.FixedCostEndsAt
}

func costCalculatorOptionalFloatEqual(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (cfg CostCalculatorConfig) AccountUsageRatePeriods() []CostCalculatorUsageRatePeriod {
	items := make([]CostCalculatorAccountCost, 0, len(cfg.AccountCostHistory)+len(cfg.AccountCosts))
	items = append(items, cfg.AccountCostHistory...)
	items = append(items, cfg.AccountCosts...)
	periods := make([]CostCalculatorUsageRatePeriod, 0, len(items))
	for _, item := range items {
		if item.AccountID <= 0 || item.UsageCostRate == nil || !isFiniteNonNegative(*item.UsageCostRate) {
			continue
		}
		period := CostCalculatorUsageRatePeriod{AccountID: item.AccountID, UsageCostRate: *item.UsageCostRate}
		if parsed, err := parseCostCalculatorEffectiveTime("effective_from", item.EffectiveFrom, false); err == nil && !parsed.IsZero() {
			period.EffectiveFrom = &parsed
		}
		if parsed, err := parseCostCalculatorEffectiveTime("effective_until", item.EffectiveUntil, false); err == nil && !parsed.IsZero() {
			period.EffectiveUntil = &parsed
		}
		periods = append(periods, period)
	}
	return periods
}

func costCalculatorAmountKey(value float64) int64 {
	return int64(math.Round(value * 1_000_000))
}

func isFinitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func isFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func trimCostCalculatorText(value string) string {
	return strings.TrimSpace(value)
}

func validateCostCalculatorDateRange(startDate, endDate string) error {
	var start time.Time
	var end time.Time
	var hasStart, hasEnd bool
	if startDate != "" {
		parsed, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			return fmt.Errorf("fixed_cost_starts_at must use YYYY-MM-DD")
		}
		start = parsed
		hasStart = true
	}
	if endDate != "" {
		parsed, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			return fmt.Errorf("fixed_cost_ends_at must use YYYY-MM-DD")
		}
		end = parsed
		hasEnd = true
	}
	if hasStart && hasEnd && end.Before(start) {
		return fmt.Errorf("fixed_cost_ends_at must be on or after fixed_cost_starts_at")
	}
	return nil
}
