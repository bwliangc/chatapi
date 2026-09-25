import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export interface LuckySecondCampaign {
  id: number
  name: string
  starts_at: string
  ends_at: string
  timezone: string
  total_amount: string
  reward_count: number
  cancelled_at: string | null
  created_at: string
  awarded_count: number
  awarded_amount: string
  my_awarded_amount?: string
  expired_count: number
  pending_count: number
}
export interface LuckySecondSlot {
  id: number
  second_at: string
  amount: string
  state: 'waiting' | 'awarded' | 'expired' | 'cancelled'
  user_id?: number
  name?: string
  is_me: boolean
  request_id?: string
  awarded_at: string | null
}
export interface LuckySecondCreate {
  name: string
  starts_at: string
  ends_at: string
  timezone: string
  total_amount: string
  reward_count: number
}
export const luckySecondAPI = {
  async list(admin: boolean, page = 1) {
    return (await apiClient.get<BasePaginationResponse<LuckySecondCampaign>>(`${admin ? '/admin' : ''}/lucky-second`, { params: { page, page_size: 10 } })).data
  },
  async slots(admin: boolean, id: number, page = 1) {
    return (await apiClient.get<BasePaginationResponse<LuckySecondSlot>>(`${admin ? '/admin' : ''}/lucky-second/${id}/${admin ? 'slots' : 'awards'}`, { params: { page, page_size: 20 } })).data
  },
  async create(input: LuckySecondCreate) {
    return (await apiClient.post<{ id: number }>('/admin/lucky-second', input)).data
  },
  async update(id: number, input: Partial<Omit<LuckySecondCreate, 'timezone'>>) {
    return (await apiClient.patch<{ id: number }>(`/admin/lucky-second/${id}`, input)).data
  },
  async cancel(id: number) {
    await apiClient.post(`/admin/lucky-second/${id}/cancel`)
  },
}
