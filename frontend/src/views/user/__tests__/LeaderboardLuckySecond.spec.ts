import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import LeaderboardView from '../LeaderboardView.vue'

const mocks = vi.hoisted(() => ({ getLeaderboard: vi.fn(), app: { cachedPublicSettings: { lucky_second_enabled: true, leaderboard_ranking_visible_enabled: false }, showError: vi.fn() } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks.app }))
vi.mock('@/api/leaderboard', () => ({ getLeaderboard: mocks.getLeaderboard }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))

describe('Leaderboard Lucky Second tab', () => {
  it('remains available when the spending leaderboard is disabled', async () => {
    const wrapper = mount(LeaderboardView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, LuckySecondPanel: { template: '<div>campaign-panel</div>' }, Pagination: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('campaign-panel')
    expect(wrapper.text()).not.toContain('leaderboard.tabToday')
    expect(mocks.getLeaderboard).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
