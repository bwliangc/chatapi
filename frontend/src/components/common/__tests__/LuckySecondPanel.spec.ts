import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LuckySecondPanel from '../LuckySecondPanel.vue'

const mocks = vi.hoisted(() => ({
  list: vi.fn(), slots: vi.fn(), create: vi.fn(), cancel: vi.fn(),
  app: { cachedPublicSettings: { lucky_second_enabled: false }, showSuccess: vi.fn() },
}))
vi.mock('@/api/luckySecond', () => ({ luckySecondAPI: mocks }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks.app }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const campaign = (id: number) => ({ id, name: `Campaign ${id}`, starts_at: '2090-10-01T00:00:00Z', ends_at: '2090-10-08T00:00:00Z', timezone: 'UTC', total_amount: '100', awarded_amount: '2', reward_count: 50, awarded_count: 1, expired_count: 0, pending_count: 0, cancelled_at: null })
const wrappers: ReturnType<typeof mount>[] = []
function render(admin = false) {
  const wrapper = mount(LuckySecondPanel, { props: { admin }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, Pagination: true } } })
  wrappers.push(wrapper); return wrapper
}
beforeEach(() => { vi.clearAllMocks(); mocks.list.mockResolvedValue({ items: [campaign(1), campaign(2)], total: 2, page: 1 }); mocks.slots.mockResolvedValue({ items: [], total: 0, page: 1 }) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()) })

describe('LuckySecondPanel', () => {
  it('shows history management even while participation is disabled', async () => {
    const wrapper = render(true); await flushPromises()
    expect(mocks.list).toHaveBeenCalledWith(true, 1)
    expect(wrapper.text()).toContain('luckySecond.disabledHint')
    expect(wrapper.text()).toContain('luckySecond.create')
    expect(wrapper.text()).toContain('Campaign 1')
    expect(wrapper.text()).not.toContain('luckySecond.myAwarded')
  })
  it('uses public winner endpoints without exposing admin controls', async () => {
    const wrapper = render(); await flushPromises()
    expect(mocks.list).toHaveBeenCalledWith(false, 1)
    expect(wrapper.text()).not.toContain('luckySecond.create')
    const button = wrapper.findAll('button').find(b => b.text() === 'luckySecond.awards')!
    await button.trigger('click'); await flushPromises()
    expect(mocks.slots).toHaveBeenCalledWith(false, 1, 1)
    expect(wrapper.text()).toContain('luckySecond.noAwards')
  })
  it('does not show a previous campaign response after switching details', async () => {
    let finishFirst!: (value: unknown) => void
    mocks.slots.mockImplementationOnce(() => new Promise(resolve => { finishFirst = resolve }))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('article')[0].find('button').trigger('click')
    await wrapper.findAll('article')[1].find('button').trigger('click'); await flushPromises()
    finishFirst({ items: [{ id: 3, name: 'wrong-winner', amount: '10', second_at: '2090-10-01T01:00:00Z', state: 'awarded' }], total: 1, page: 1 })
    await flushPromises(); expect(wrapper.text()).not.toContain('wrong-winner')
  })
  it('shows the personal total before opening records and zero for no rewards', async () => {
    mocks.list.mockResolvedValueOnce({ items: [{ ...campaign(1), my_awarded_amount: '5.12345600' }, { ...campaign(2), my_awarded_amount: '0' }], total: 2, page: 1 })
    const wrapper = render(); await flushPromises()
    const cards = wrapper.findAll('article')
    expect(cards[0].text()).toContain('luckySecond.myAwarded')
    expect(cards[0].text()).toContain('$5.123456')
    expect(cards[1].text()).toContain('$0.00')
    expect(mocks.slots).not.toHaveBeenCalled()
  })
})
