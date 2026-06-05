/**
 * User Groups API endpoints (non-admin)
 * Handles group-related operations for regular users
 */

import { apiClient } from './client'
import type { Group } from '@/types'
import type { UserSupportedModel } from './channels'

/**
 * 公开分组信息条目：在可绑定分组基础上，额外带分组描述与可用模型列表。
 * 模型来自该分组关联渠道的支持模型并集（后端已按分组平台脱敏、按模型名去重）。
 * 受 available-channels 开关约束：未启用时返回空数组。
 */
export interface GroupInfo {
  id: number
  name: string
  description: string
  platform: string
  subscription_type: string
  rate_multiplier: number
  is_exclusive: boolean
  models: UserSupportedModel[]
}

/**
 * Get available groups that the current user can bind to API keys
 * This returns groups based on user's permissions:
 * - Standard groups: public (non-exclusive) or explicitly allowed
 * - Subscription groups: user has active subscription
 * @returns List of available groups
 */
export async function getAvailable(): Promise<Group[]> {
  const { data } = await apiClient.get<Group[]>('/groups/available')
  return data
}

/**
 * Get current user's custom group rate multipliers
 * @returns Map of group_id to custom rate_multiplier
 */
export async function getUserGroupRates(): Promise<Record<number, number>> {
  const { data } = await apiClient.get<Record<number, number> | null>('/groups/rates')
  return data || {}
}

/**
 * Get public info for groups the current user can bind (rate / platform / models).
 * Gated by the same available-channels setting; returns [] when disabled.
 */
export async function getAvailableGroupsInfo(options?: {
  signal?: AbortSignal
}): Promise<GroupInfo[]> {
  const { data } = await apiClient.get<GroupInfo[]>('/groups/available/info', {
    signal: options?.signal
  })
  return data
}

export const userGroupsAPI = {
  getAvailable,
  getUserGroupRates,
  getAvailableGroupsInfo
}

export default userGroupsAPI
