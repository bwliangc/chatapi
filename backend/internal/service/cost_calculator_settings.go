package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

const SettingKeyCostCalculatorConfig = "cost_calculator_config"

type CostCalculatorAccountCost struct {
	AccountID        int64    `json:"account_id"`
	AccountName      string   `json:"account_name"`
	Platform         string   `json:"platform"`
	MonthlyCost      float64  `json:"monthly_cost"`
	UsageCostRate    *float64 `json:"usage_cost_rate,omitempty"`
	MonthlyCostLabel string   `json:"monthly_cost_label,omitempty"`
}

type CostCalculatorBalanceRechargePackage struct {
	BalanceAmount float64 `json:"balance_amount"`
	ActualAmount  float64 `json:"actual_amount"`
}

type CostCalculatorConfig struct {
	BalanceExchangeRate     float64                                `json:"balance_exchange_rate"`
	UpstreamCostRate        float64                                `json:"upstream_cost_rate"`
	AccountCosts            []CostCalculatorAccountCost            `json:"account_costs"`
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
	maxCostCalculatorBalanceRechargePackages = 200
	maxCostCalculatorLabelLength             = 120
)

func DefaultCostCalculatorConfig() CostCalculatorConfig {
	return CostCalculatorConfig{
		BalanceExchangeRate:     defaultCostCalculatorBalanceExchangeRate,
		UpstreamCostRate:        defaultCostCalculatorUpstreamCostRate,
		AccountCosts:            []CostCalculatorAccountCost{},
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
	if len(cfg.BalanceRechargePackages) > maxCostCalculatorBalanceRechargePackages {
		return CostCalculatorConfig{}, fmt.Errorf("balance_recharge_packages cannot exceed %d", maxCostCalculatorBalanceRechargePackages)
	}
	seen := make(map[int64]struct{}, len(cfg.AccountCosts))
	costs := make([]CostCalculatorAccountCost, 0, len(cfg.AccountCosts))
	for _, item := range cfg.AccountCosts {
		if item.AccountID <= 0 {
			return CostCalculatorConfig{}, fmt.Errorf("account_id must be > 0")
		}
		if _, ok := seen[item.AccountID]; ok {
			return CostCalculatorConfig{}, fmt.Errorf("duplicate account_id: %d", item.AccountID)
		}
		seen[item.AccountID] = struct{}{}
		if !isFiniteNonNegative(item.MonthlyCost) {
			return CostCalculatorConfig{}, fmt.Errorf("monthly_cost must be >= 0")
		}
		if item.UsageCostRate != nil && !isFiniteNonNegative(*item.UsageCostRate) {
			return CostCalculatorConfig{}, fmt.Errorf("usage_cost_rate must be >= 0")
		}
		if item.UsageCostRate == nil {
			rate := cfg.UpstreamCostRate
			item.UsageCostRate = &rate
		}
		item.AccountName = trimCostCalculatorText(item.AccountName)
		item.Platform = trimCostCalculatorText(item.Platform)
		item.MonthlyCostLabel = trimCostCalculatorText(item.MonthlyCostLabel)
		if len(item.MonthlyCostLabel) > maxCostCalculatorLabelLength {
			return CostCalculatorConfig{}, fmt.Errorf("monthly_cost_label is too long")
		}
		costs = append(costs, item)
	}
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].AccountID < costs[j].AccountID
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
		BalanceRechargePackages: packages,
	}, nil
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
