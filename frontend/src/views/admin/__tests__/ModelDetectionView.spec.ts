import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ModelDetectionView from '../ModelDetectionView.vue'
import type { DetectionEvent } from '@/api/admin/modelDetection'

const mock = vi.hoisted(() => ({ list: vi.fn(), models: vi.fn(), info: vi.fn(), detect: vi.fn() }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, values?: unknown) => key === 'admin.modelDetection.progress' ? `${key} ${JSON.stringify(values)}` : key }) }))
vi.mock('@/api/admin/accounts', () => ({ list: mock.list, getAvailableModels: mock.models }))
vi.mock('@/api/admin/modelDetection', () => ({ getDetectionInfo: mock.info, detectAccountModel: mock.detect }))
const render = () => mount(ModelDetectionView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })

afterEach(() => { vi.useRealTimers() })

const chooseAccount = async (wrapper: ReturnType<typeof render>) => {
  await wrapper.find('#detection-account').trigger('focus')
  await flushPromises()
  await wrapper.find('#detection-account').trigger('keydown', { key: 'Enter' })
  await flushPromises()
}

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
    await chooseAccount(wrapper)
    await wrapper.findAll('select')[1].setValue('gpt-6-sol')
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
    await chooseAccount(wrapper)
    expect(mock.models).toHaveBeenCalledWith(42)
    expect(wrapper.findAll('select')[1].text()).not.toContain('gpt-image-2')
    await wrapper.findAll('select')[1].setValue('gpt-6-sol')
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
    await chooseAccount(wrapper)
    await wrapper.findAll('select')[1].setValue('gpt-6-sol')
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
  it('searches remotely after typing and selects a matching account with the keyboard', async () => {
    vi.useFakeTimers()
    const wrapper = render(); await flushPromises()
    await wrapper.find('#detection-account').trigger('focus'); await flushPromises()
    mock.list.mockResolvedValue({ items: [{ id: 99, name: 'Production account', type: 'oauth' }], total: 1 })
    const calls = mock.list.mock.calls.length
    await wrapper.find('#detection-account').setValue('Prod')
    await vi.advanceTimersByTimeAsync(299)
    expect(mock.list).toHaveBeenCalledTimes(calls)
    await vi.advanceTimersByTimeAsync(1); await flushPromises()
    expect(mock.list).toHaveBeenLastCalledWith(1, 50, expect.objectContaining({ search: 'Prod', platform: 'openai' }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.find('[role="listbox"]').text()).toContain('Production account')
    await wrapper.find('#detection-account').trigger('keydown', { key: 'Enter' }); await flushPromises()
    expect(mock.models).toHaveBeenLastCalledWith(99)
    expect(wrapper.find('[role="listbox"]').exists()).toBe(false)
    await wrapper.findAll('select')[1].setValue('gpt-6-sol')
    await wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.start')!.trigger('click'); await flushPromises()
    expect(mock.detect).toHaveBeenCalledWith(99, 'gpt-6-sol', expect.any(AbortSignal), expect.any(Function))
    wrapper.unmount()
  })
  it('clears the selection immediately on editing and ignores stale search results', async () => {
    vi.useFakeTimers()
    const wrapper = render(); await flushPromises()
    await chooseAccount(wrapper)
    await wrapper.findAll('select')[1].setValue('gpt-6-sol')
    let resolveOld!: (data: unknown) => void
    mock.list.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    await wrapper.find('#detection-account').setValue('Old')
    expect(wrapper.findAll('button').find(b => b.text() === 'admin.modelDetection.start')!.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('select')[1].attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.find('#detection-account').setValue('New')
    resolveOld({ items: [{ id: 88, name: 'Old result', type: 'apikey' }], total: 1 })
    await flushPromises()
    expect(wrapper.find('[role="listbox"]').text()).not.toContain('Old result')
    mock.list.mockResolvedValue({ items: [{ id: 99, name: 'New result', type: 'oauth' }], total: 1 })
    await vi.advanceTimersByTimeAsync(300); await flushPromises()
    await wrapper.find('[role="option"]').trigger('click'); await flushPromises()
    expect(mock.models).toHaveBeenLastCalledWith(99)
    wrapper.unmount()
  })
  it('cancels pending search and clears account selection when changing platform', async () => {
    vi.useFakeTimers()
    const wrapper = render(); await flushPromises()
    await chooseAccount(wrapper)
    await wrapper.find('#detection-account').setValue('pending')
    await wrapper.findAll('select')[0].setValue('anthropic'); await flushPromises()
    const calls = mock.list.mock.calls.length
    await vi.advanceTimersByTimeAsync(300)
    expect(mock.list).toHaveBeenCalledTimes(calls)
    expect(mock.list).toHaveBeenLastCalledWith(1, 50, expect.objectContaining({ search: '', platform: 'anthropic' }), expect.anything())
    expect((wrapper.find('#detection-account').element as HTMLInputElement).value).toBe('')
    expect(wrapper.findAll('select')[1].attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

})
