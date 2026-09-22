import { apiClient } from './client'
import type { GroupDynamicRate } from '@/types'

export interface GroupRateUsage {
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
}

export interface GroupRateTrendPoint {
  at: string
  tokens: number
  requests: number
}

export interface GroupRateHistoryPoint {
  at: string
  rate_multiplier: number
}

export interface GroupRateBoardItem {
  id: number
  name: string
  platform: string
  subscription_type: string
  rate_multiplier: number
  peak_multiplier: number
  effective_multiplier: number
  user_rate_multiplier?: number
  dynamic_rate: GroupDynamicRate
  rate_updated_at?: string
  last_hour: GroupRateUsage
  last_24_hours: GroupRateUsage
  trend: GroupRateTrendPoint[]
  rate_trend: GroupRateHistoryPoint[]
}

export interface GroupRateBoard {
  rates_at: string
  usage_at: string
  refresh_seconds: number
  groups: GroupRateBoardItem[]
}

export async function getGroupRateBoard(signal?: AbortSignal): Promise<GroupRateBoard> {
  const { data } = await apiClient.get<GroupRateBoard>('/groups/board', { signal })
  return data
}
