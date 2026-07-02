import { apiClient } from '../client'

export interface CostCalculatorAccountCost {
  account_id: number
  account_name: string
  platform: string
  monthly_cost: number
  usage_cost_rate?: number
  monthly_cost_label?: string
  fixed_cost_starts_at?: string
  fixed_cost_ends_at?: string
}

export interface CostCalculatorBalanceRechargePackage {
  balance_amount: number
  actual_amount: number
}

export interface CostCalculatorConfig {
  balance_exchange_rate: number
  upstream_cost_rate: number
  account_costs: CostCalculatorAccountCost[]
  balance_recharge_packages: CostCalculatorBalanceRechargePackage[]
}

export interface UpdateCostCalculatorConfigRequest {
  balance_exchange_rate?: number
  upstream_cost_rate?: number
  account_costs?: CostCalculatorAccountCost[]
  balance_recharge_packages?: CostCalculatorBalanceRechargePackage[]
}

export interface CostCalculatorUsageParams {
  start_date?: string
  end_date?: string
  exclude_admins?: boolean
}

export interface CostCalculatorGroupUsage {
  group_id: number
  group_name: string
  requests: number
  total_tokens: number
  standard_cost: number
  actual_cost: number
  usage_cost: number
}

export interface CostCalculatorModelUsage {
  model: string
  requests: number
  total_tokens: number
  standard_cost: number
  actual_cost: number
  usage_cost: number
}

export interface CostCalculatorAccountUsage {
  account_id: number
  account_name: string
  platform: string
  requests: number
  total_tokens: number
  standard_cost: number
  actual_cost: number
  usage_cost: number
}

export interface CostCalculatorUsageSummary {
  start_date: string
  end_date: string
  total_requests: number
  total_tokens: number
  total_standard_cost: number
  total_actual_cost: number
  total_usage_cost: number
  groups: CostCalculatorGroupUsage[]
  models: CostCalculatorModelUsage[]
  accounts: CostCalculatorAccountUsage[]
}

export interface CostCalculatorBalanceRechargeSummary {
  total_amount: number
  redeem_amount: number
  admin_net_amount: number
  admin_added_amount: number
  admin_deducted_amount: number
  actual_revenue: number
  redeem_actual_revenue: number
  admin_actual_revenue: number
  matched_balance_amount: number
  unmatched_balance_amount: number
  matched_record_count: number
  unmatched_record_count: number
  leaderboard_reward_amount: number
  leaderboard_reward_count: number
  record_count: number
  redeem_count: number
  admin_count: number
  package_stats: Array<CostCalculatorBalanceRechargePackage & {
    count: number
    balance_total: number
    actual_total: number
  }>
}

export interface CostCalculatorBalanceLiabilitySummary {
  total_balance: number
  positive_user_count: number
  estimated_actual_liability: number
  estimated_unit_cost: number
  valuation_source: 'matched_recharge_history' | 'package_table' | 'unavailable' | string
  matched_balance_amount: number
  matched_actual_revenue: number
  unmatched_balance_amount: number
  leaderboard_reward_amount: number
  leaderboard_reward_count: number
}

export async function getConfig(): Promise<CostCalculatorConfig> {
  const { data } = await apiClient.get<CostCalculatorConfig>('/admin/cost-calculator/config')
  return data
}

export async function updateConfig(payload: UpdateCostCalculatorConfigRequest): Promise<CostCalculatorConfig> {
  const { data } = await apiClient.put<CostCalculatorConfig>('/admin/cost-calculator/config', payload)
  return data
}

export async function getUsageSummary(params?: CostCalculatorUsageParams): Promise<CostCalculatorUsageSummary> {
  const { data } = await apiClient.get<CostCalculatorUsageSummary>('/admin/cost-calculator/usage-summary', { params })
  return data
}

export async function getBalanceRechargeSummary(params?: CostCalculatorUsageParams): Promise<CostCalculatorBalanceRechargeSummary> {
  const { data } = await apiClient.get<CostCalculatorBalanceRechargeSummary>('/admin/cost-calculator/balance-recharge-summary', { params })
  return data
}

export async function getBalanceLiabilitySummary(params?: CostCalculatorUsageParams): Promise<CostCalculatorBalanceLiabilitySummary> {
  const { data } = await apiClient.get<CostCalculatorBalanceLiabilitySummary>('/admin/cost-calculator/balance-liability-summary', { params })
  return data
}

export const costCalculatorAPI = {
  getConfig,
  updateConfig,
  getUsageSummary,
  getBalanceRechargeSummary,
  getBalanceLiabilitySummary
}

export default costCalculatorAPI
