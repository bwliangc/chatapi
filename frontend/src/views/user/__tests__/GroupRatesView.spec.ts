import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import GroupRatesView from '../GroupRatesView.vue'
import type { GroupRateBoard } from '@/api/groupRates'

const { getBoard } = vi.hoisted(() => ({ getBoard: vi.fn() }))
vi.mock('@/api/groupRates', () => ({ getGroupRateBoard: getBoard }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, locale: ref('en') }),
}))

const usage = { requests: 2, input_tokens: 10, output_tokens: 20, cache_read_tokens: 30, cache_creation_tokens: 40, total_tokens: 100 }
const fixture: GroupRateBoard = {
  rates_at: '2026-09-22T08:30:00Z', usage_at: '2026-09-22T08:30:00Z', refresh_seconds: 30,
  groups: [1, 2].map(id => ({
    id, name: `Group ${id}`, platform: id === 1 ? 'openai' : 'anthropic', subscription_type: 'standard',
    rate_multiplier: id, peak_multiplier: 1, effective_multiplier: id === 1 ? 0 : id,
    ...(id === 1 ? { user_rate_multiplier: 0 } : {}),
    dynamic_rate: { enabled: id === 1, min: .5, max: 2 },
    last_hour: usage, last_24_hours: usage,
    trend: [{ at: '2026-09-22T08:00:00Z', tokens: 100, requests: 2 }],
  })),
}
const mountView = () => shallowMount(GroupRatesView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
let wrapper: ReturnType<typeof mountView> | undefined
beforeEach(() => {
  vi.useFakeTimers()
  getBoard.mockReset().mockResolvedValue(structuredClone(fixture))
  Object.defineProperty(document, 'hidden', { configurable: true, value: false })
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers() })

describe('GroupRatesView', () => {
  it('filters groups and aggregated chart data without losing a zero personal override', async () => {
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('[data-group-id]')).toHaveLength(2)
    expect(wrapper.find('[data-group-id="1"]').text()).toContain('groupRates.customRate')
    expect(wrapper.findComponent({ name: 'GroupRateTrendChart' }).props('points')[0].tokens).toBe(200)
    await wrapper.get('input[type="search"]').setValue('Group 2')
    expect(wrapper.findAll('[data-group-id]')).toHaveLength(1)
    expect(wrapper.findComponent({ name: 'GroupRateTrendChart' }).props('points')[0].tokens).toBe(100)
    await wrapper.get('input[type="search"]').setValue('missing')
    expect(wrapper.text()).toContain('groupRates.noMatch')
  })

  it('refreshes automatically, labels stale data and clears it when access is denied', async () => {
    wrapper = mountView()
    await flushPromises()
    getBoard.mockRejectedValueOnce(new Error('network'))
    await vi.advanceTimersByTimeAsync(30000)
    expect(getBoard).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[role="alert"]').text()).toContain('groupRates.stale')
    expect(wrapper.findAll('[data-group-id]')).toHaveLength(2)
    getBoard.mockRejectedValueOnce({ status: 403 })
    await vi.advanceTimersByTimeAsync(30000)
    expect(wrapper.findAll('[data-group-id]')).toHaveLength(0)
    expect(wrapper.get('[role="alert"]').text()).toContain('groupRates.loadError')
  })

  it('pauses in a hidden tab and aborts pending work on unmount', async () => {
    wrapper = mountView()
    await flushPromises()
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    await vi.advanceTimersByTimeAsync(60000)
    expect(getBoard).toHaveBeenCalledTimes(1)
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    getBoard.mockImplementationOnce(() => new Promise(() => {}))
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(getBoard).toHaveBeenCalledTimes(2)
    const signal = getBoard.mock.calls[1][0] as AbortSignal
    wrapper.unmount(); wrapper = undefined
    expect(signal.aborted).toBe(true)
    await vi.advanceTimersByTimeAsync(60000)
    expect(getBoard).toHaveBeenCalledTimes(2)
  })
})
