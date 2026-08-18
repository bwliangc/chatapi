import { apiClient } from './client'

export type LeaderboardPeriod = 'today' | 'yesterday'

export interface LeaderboardEntry {
  rank: number
  name: string // 脱敏标识（本人行为真实邮箱）
  actual_cost: number
  requests: number
  tokens: number
  is_winner: boolean
  is_me: boolean
  reward_amount?: number // 昨日结算的实际发放金额（仅中奖者，>0）
}

export interface LeaderboardResponse {
  period: LeaderboardPeriod
  reward_enabled: boolean
  pool_rate: number
  top_n: number
  distribution_mode: string
  distribution_shares?: number[] // weighted 模式下前 N 名奖励占比(%)，按名次顺序；非 weighted 为空
  total_cost: number
  pool_amount: number
  min_spend: number
  display_top_n: number
  ranking: LeaderboardEntry[]
  me: LeaderboardEntry | null
  total: number
  page: number
  page_size: number
  pages: number
}

/** 分页获取今日/昨日消费排行榜（脱敏，受展示开关控制）。 */
export async function getLeaderboard(
  period: LeaderboardPeriod,
  page: number = 1,
  pageSize: number = 10,
): Promise<LeaderboardResponse> {
  const { data } = await apiClient.get<LeaderboardResponse>(
    '/leaderboard',
    { params: { period, page, page_size: pageSize } },
  )
  return data
}
