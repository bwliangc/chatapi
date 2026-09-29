import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelDetectionSchedule from '../ModelDetectionSchedule.vue'
import ModelDetectionStatus from '../ModelDetectionStatus.vue'

const mock = vi.hoisted(() => ({ list: vi.fn(), models: vi.fn(), get: vi.fn(), save: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/accounts', () => ({ list: mock.list, getAvailableModels: mock.models }))
vi.mock('@/api/admin/modelDetection', () => ({ getDetectionSchedule: mock.get, saveDetectionSchedule: mock.save }))
const render = () => mount(ModelDetectionSchedule, { props: { info: { models: ['gpt-6-sol'], bank_sha256: '', bank_built_at: '' } } })
beforeEach(() => {
  vi.clearAllMocks()
  mock.list.mockImplementation((page: number) => Promise.resolve({ items: [{ id: page === 1 ? 42 : 99, name: page === 1 ? 'First account' : 'Other account', type: 'apikey' }], total: 51 }))
  mock.models.mockResolvedValue([{ id: 'gpt-6-sol' }, { id: 'gpt-image-2' }])
  mock.get.mockResolvedValue({ enabled: false, interval_minutes: 360, targets: [] })
  mock.save.mockImplementation(async (config) => structuredClone(config))
})
describe('ModelDetectionSchedule', () => {
  it('preserves selection across pages, applies a model, and saves the schedule', async () => {
    // Vue reactive proxies are not structured-cloneable; snapshot the API input as JSON.
    mock.save.mockImplementation(async config => JSON.parse(JSON.stringify(config)))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('input[type="checkbox"]')[0].setValue(true)
    await wrapper.findAll('input[type="checkbox"]')[1].setValue(true); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'common.next')!.trigger('click'); await flushPromises()
    await wrapper.findAll('input[type="checkbox"]')[1].setValue(true); await flushPromises()
    await wrapper.findAll('select')[1].setValue('gpt-6-sol')
    await wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.schedule.applyModel')!.trigger('click')
    await wrapper.find('input[type="number"]').setValue('60')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click'); await flushPromises()
    expect(mock.save).toHaveBeenCalledWith({ enabled: true, interval_minutes: 60, targets: [{ account_id: 42, model: 'gpt-6-sol' }, { account_id: 99, model: 'gpt-6-sol' }] })
    expect(wrapper.find('[role="status"]').text()).toBe('admin.modelDetection.schedule.saved')
    wrapper.unmount()
  })
  it('restores existing targets and permits disabling an unavailable account', async () => {
    mock.get.mockResolvedValue({ enabled: true, interval_minutes: 60, targets: [{ account_id: 777, model: 'custom-model', next_run_at: '2026-10-01T00:00:00Z' }] })
    mock.save.mockImplementation(async config => JSON.parse(JSON.stringify(config)))
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('#777')
    expect(wrapper.find('select[aria-label="admin.modelDetection.model #777"]').element.value).toBe('custom-model')
    await wrapper.find('input[type="checkbox"]').setValue(false)
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click'); await flushPromises()
    expect(mock.save).toHaveBeenCalledWith(expect.objectContaining({ enabled: false }))
    wrapper.unmount()
  })
  it('blocks an empty enabled schedule and surfaces save failures', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.find('input[type="checkbox"]').setValue(true)
    const save = wrapper.findAll('button').find(b => b.text() === 'common.save')!
    expect(save.attributes('disabled')).toBeDefined()
    await wrapper.findAll('input[type="checkbox"]')[1].setValue(true); await flushPromises()
    await wrapper.find('select[aria-label="admin.modelDetection.model #42"]').setValue('gpt-6-sol')
    expect(wrapper.find('select[aria-label="admin.modelDetection.model #42"]').text()).not.toContain('gpt-image-2')
    mock.save.mockRejectedValue(new Error('database unavailable'))
    await save.trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('database unavailable')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('does not allow overwriting a schedule whose initial load failed', async () => {
    mock.get.mockRejectedValue(new Error('read failed'))
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('fieldset').attributes('disabled')).toBeDefined()
    expect(mock.save).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
describe('ModelDetectionStatus', () => {
  it.each(['consistent', 'suspected_mismatch', 'inconclusive', 'unsupported', 'insufficient', 'error', 'cancelled'])('renders %s independently of other verdicts', status => {
    const wrapper = mount(ModelDetectionStatus, { props: { snapshot: { status, model: 'gpt-6-sol', checked_at: '2026-09-28T08:00:00Z', reason: 'reason' } } })
    expect(wrapper.text()).toContain(`admin.modelDetection.statuses.${status}`)
    expect(wrapper.text()).toContain('gpt-6-sol')
    expect(wrapper.attributes('title')).toBe('reason')
  })
  it('shows untested when there is no saved result', () => {
    expect(mount(ModelDetectionStatus).text()).toContain('admin.modelDetection.statuses.untested')
  })
})
