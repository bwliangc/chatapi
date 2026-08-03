import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountAutoResetModal from '../AccountAutoResetModal.vue'
import type { Account } from '@/types'

const api = vi.hoisted(() => ({
  getAutoReset: vi.fn(),
  updateAutoReset: vi.fn(),
}))
const notifications = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { accounts: api } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => notifications }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const account = {
  id: 42,
  name: 'codex-account',
  platform: 'openai',
  type: 'oauth',
} as Account

describe('AccountAutoResetModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getAutoReset.mockResolvedValue({
      enabled: true,
      strategy: 'credit_expiry',
      weekly_threshold: 90,
      expiry_hours: 24,
      email: 'alerts@example.com',
    })
    api.updateAutoReset.mockResolvedValue({})
  })

  it('loads the selected strategy and saves edited settings', async () => {
    const wrapper = mount(AccountAutoResetModal, {
      props: { show: true, account },
      attachTo: document.body,
    })
    await flushPromises()

    const expiryInput = document.body.querySelector<HTMLInputElement>('#auto-reset-expiry-hours')
    expect(expiryInput?.value).toBe('24')
    expect(expiryInput).not.toBeNull()
    expiryInput!.value = '12'
    expiryInput!.dispatchEvent(new Event('input', { bubbles: true }))
    document.body.querySelector<HTMLFormElement>('#account-auto-reset-form')!
      .dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(api.updateAutoReset).toHaveBeenCalledWith(42, {
      enabled: true,
      strategy: 'credit_expiry',
      weekly_threshold: 90,
      expiry_hours: 12,
      email: 'alerts@example.com',
    })
    expect(notifications.showSuccess).toHaveBeenCalled()
    expect(wrapper.emitted('close')).toBeTruthy()
    wrapper.unmount()
  })
})
