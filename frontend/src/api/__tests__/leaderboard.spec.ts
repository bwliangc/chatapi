import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { getLeaderboard } from '@/api/leaderboard'

describe('leaderboard API', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({ data: { ranking: [] } })
  })

  it('sends the selected period and pagination parameters', async () => {
    await getLeaderboard('yesterday', 3, 10)

    expect(get).toHaveBeenCalledWith('/leaderboard', {
      params: { period: 'yesterday', page: 3, page_size: 10 },
    })
  })
})
