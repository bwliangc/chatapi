import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelDetectionHistoryModal from '../ModelDetectionHistoryModal.vue'
import ModelDetectionStatus from '../ModelDetectionStatus.vue'
const { history } = vi.hoisted(() => ({ history: vi.fn() }))
vi.mock('@/api/admin/modelDetection', () => ({ getDetectionHistory: history }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const record = { id: '12', status: 'suspected_mismatch', model: 'gpt-6-sol', checked_at: '2026-09-29T00:00:00Z', source: 'scheduled', reason: 'Compare with another candidate', prediction: 'candidate-model' }
const render = (show = true) => mount(ModelDetectionHistoryModal, {
  props: { show, account: { id: 42, name: 'Account A' } },
  global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } }
})
beforeEach(() => { history.mockReset().mockResolvedValue({ items: [record], total: 21, page: 1, page_size: 20, pages: 2 }) })
describe('ModelDetectionHistoryModal', () => {
  it('loads only when opened and shows source, verdict, model, reason and candidate', async () => {
    const wrapper = render(false); await flushPromises(); expect(history).not.toHaveBeenCalled()
    await wrapper.setProps({ show: true }); await flushPromises()
    expect(history).toHaveBeenCalledWith(42, 1, 20, expect.any(AbortSignal))
    for (const value of ['Account A', 'gpt-6-sol', 'Compare with another candidate', 'candidate-model', 'admin.modelDetection.statuses.suspected_mismatch', 'admin.modelDetection.history.sources.scheduled']) expect(wrapper.text()).toContain(value)
    expect(wrapper.find('[aria-label="admin.modelDetection.history.open"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('requests subsequent pages and resets when switching accounts', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'common.next')!.trigger('click'); await flushPromises()
    expect(history).toHaveBeenLastCalledWith(42, 2, 20, expect.any(AbortSignal))
    await wrapper.setProps({ account: { id: 99, name: 'Account B' } }); await flushPromises()
    expect(history).toHaveBeenLastCalledWith(99, 1, 20, expect.any(AbortSignal))
    wrapper.unmount()
  })
  it('cancels on close and ignores responses from a previous account', async () => {
    let resolveOld!: (value: unknown) => void
    history.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const wrapper = render(); await flushPromises()
    const signal = history.mock.calls[0][3] as AbortSignal
    history.mockResolvedValueOnce({ items: [], total: 0, pages: 1 })
    await wrapper.setProps({ account: { id: 99, name: 'Account B' } }); await flushPromises()
    expect(signal.aborted).toBe(true)
    resolveOld({ items: [record], total: 21, pages: 2 }); await flushPromises()
    expect(wrapper.text()).toContain('admin.modelDetection.history.empty')
    expect(wrapper.text()).not.toContain('candidate-model')
    await wrapper.setProps({ show: false })
    expect((history.mock.calls[1][3] as AbortSignal).aborted).toBe(true)
    wrapper.unmount()
  })
  it('shows load failures and allows retrying', async () => {
    history.mockRejectedValueOnce(new Error('network failed'))
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('network failed')
    await wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.history.retry')!.trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('candidate-model')
    wrapper.unmount()
  })
  it('emits a history event beside an untested status', async () => {
    const wrapper = mount(ModelDetectionStatus, { props: { showHistory: true } })
    await wrapper.find('button[aria-label="admin.modelDetection.history.open"]').trigger('click')
    expect(wrapper.emitted('history')).toHaveLength(1)
    expect(wrapper.text()).toContain('admin.modelDetection.statuses.untested')
  })
})
