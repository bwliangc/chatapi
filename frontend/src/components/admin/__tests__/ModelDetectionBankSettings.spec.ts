import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelDetectionBankSettings from '../ModelDetectionBankSettings.vue'

const api = vi.hoisted(() => ({ status: vi.fn(), check: vi.fn(), update: vi.fn(), rollback: vi.fn(), stepUp: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/modelDetection', () => ({ getBankStatus: api.status, checkBankUpdate: api.check, updateBank: api.update, rollbackBank: api.rollback }))
vi.mock('@/components/auth/TotpStepUpDialog.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/composables/useStepUp', () => ({ useStepUp: () => ({ run: api.stepUp }), isStepUpCancelled: () => false }))
const version = { commit: 'a'.repeat(40), sha256: 'bank-a', model_count: 3, built_at: '2026-09-23', algorithm_sha256: 'algorithm' }
const current = { current: version, source: 'embedded', revision: 'revision-a' }
const updated = { current: { ...version, commit: 'b'.repeat(40), sha256: 'bank-b' }, source: 'database', revision: 'revision-b', previous: version }
const render = () => mount(ModelDetectionBankSettings)

beforeEach(() => {
  vi.resetAllMocks()
  api.stepUp.mockImplementation((fn: () => Promise<unknown>) => fn())
  api.status.mockResolvedValue(current)
  api.check.mockResolvedValue({ status: current, remote: updated.current, compatible: true, update_available: true })
  api.update.mockResolvedValue(updated)
  api.rollback.mockResolvedValue({ ...current, revision: 'revision-c' })
})
describe('ModelDetectionBankSettings', () => {
  it('loads local status without contacting upstream and requires a check before update', async () => {
    const wrapper = render(); await flushPromises()
    expect(api.status).toHaveBeenCalledOnce()
    expect(api.check).not.toHaveBeenCalled()
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('updates the exact checked commit and revision through step-up, then rolls back', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button')[0].trigger('click'); await flushPromises()
    await wrapper.findAll('button')[1].trigger('click'); await flushPromises()
    expect(api.update).toHaveBeenCalledWith(updated.current.commit, current.revision)
    expect(api.stepUp).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('admin.modelDetection.bankUpdate.updated')
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeDefined()
    await wrapper.findAll('button')[2].trigger('click'); await flushPromises()
    expect(api.rollback).toHaveBeenCalledWith(updated.revision)
    expect(wrapper.findAll('button')[2].attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.modelDetection.bankUpdate.rolledBack')
    wrapper.unmount()
  })
  it('blocks algorithm changes', async () => {
    api.check.mockResolvedValue({ status: current, remote: updated.current, compatible: false, update_available: false })
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button')[0].trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('admin.modelDetection.bankUpdate.requiresUpgrade')
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeDefined()
    expect(api.update).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('refreshes after a failed mutation and requires a fresh check', async () => {
    api.update.mockRejectedValue(new Error('conflict'))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button')[0].trigger('click'); await flushPromises()
    api.status.mockResolvedValue(updated)
    await wrapper.findAll('button')[1].trigger('click'); await flushPromises()
    expect(api.status).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[role="alert"]').text()).toContain('conflict')
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('button')[2].attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})
