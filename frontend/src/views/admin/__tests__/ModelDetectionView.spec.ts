import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelDetectionView from '../ModelDetectionView.vue'
import type { DetectionEvent } from '@/api/admin/modelDetection'

const mock = vi.hoisted(() => ({ list: vi.fn(), models: vi.fn(), info: vi.fn(), detect: vi.fn() }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, values?: unknown) => key === 'admin.modelDetection.progress' ? `${key} ${JSON.stringify(values)}` : key }) }))
vi.mock('@/api/admin/accounts', () => ({ list: mock.list, getAvailableModels: mock.models }))
vi.mock('@/api/admin/modelDetection', () => ({ getDetectionInfo: mock.info, detectAccountModel: mock.detect }))
const render = () => mount(ModelDetectionView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })

beforeEach(() => {
  vi.clearAllMocks()
  mock.list.mockResolvedValue({ items: [{ id: 42, name: 'Test account', type: 'apikey' }], total: 1 })
  mock.models.mockResolvedValue([{ id: 'gpt-6-sol' }, { id: 'gpt-image-2' }])
  mock.info.mockResolvedValue({ models: ['gpt-6-sol'], bank_sha256: 'sample', bank_built_at: '2026-09-23' })
  mock.detect.mockResolvedValue(undefined)
})
describe('ModelDetectionView', () => {
  it('shows concurrent requests in progress and clears the count when the request ends', async () => {
    let emit!: (event: DetectionEvent) => void
    let finish!: () => void
    mock.detect.mockImplementation((_id: number, _model: string, _signal: AbortSignal, onEvent: (event: DetectionEvent) => void) => new Promise<void>(resolve => {
      emit = onEvent
      finish = resolve
    }))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('select')[1].setValue('42'); await flushPromises()
    await wrapper.findAll('select')[2].setValue('gpt-6-sol')
    await wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.start')!.trigger('click')
    emit({ type: 'progress', attempt: 3, accepted: 0, in_flight: 3 })
    await flushPromises()
    expect(wrapper.find('[role="status"]').text()).toContain('"active":3')
    emit({ type: 'progress', attempt: 3, accepted: 1, in_flight: 2 })
    await flushPromises()
    expect(wrapper.find('[role="status"]').text()).toContain('"active":2')
    finish(); await flushPromises()
    expect(wrapper.find('[role="status"]').text()).toContain('"active":0')
    wrapper.unmount()
  })
  it('loads the selected account models and submits exactly that account and model', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('select')[1].setValue('42'); await flushPromises()
    expect(mock.models).toHaveBeenCalledWith(42)
    expect(wrapper.findAll('select')[2].text()).not.toContain('gpt-image-2')
    await wrapper.findAll('select')[2].setValue('gpt-6-sol')
    const start = wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.start')!
    await start.trigger('click'); await flushPromises()
    expect(mock.detect).toHaveBeenCalledWith(42, 'gpt-6-sol', expect.any(AbortSignal), expect.any(Function))
    wrapper.unmount()
  })
  it('aborts the active request when leaving the page', async () => {
    mock.detect.mockImplementation((_id: number, _model: string, signal: AbortSignal) => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('cancelled', 'AbortError')))
    }))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('select')[1].setValue('42'); await flushPromises()
    await wrapper.findAll('select')[2].setValue('gpt-6-sol')
    await wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.start')!.trigger('click')
    const signal = mock.detect.mock.calls[0][2] as AbortSignal
    expect(signal.aborted).toBe(false)
    wrapper.unmount(); await flushPromises()
    expect(signal.aborted).toBe(true)
  })
  it('prevents starting when metadata endpoint rejects a disabled feature', async () => {
    mock.info.mockRejectedValue(new Error('disabled'))
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('disabled')
    expect(wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.start')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
})
