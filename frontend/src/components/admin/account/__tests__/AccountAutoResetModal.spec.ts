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
      conditions: [
        { type: 'weekly_threshold', value: 90 },
        { type: 'credit_expiry', value: 1440 },
      ],
      weekly_threshold: 90,
      expiry_minutes: 1440,
      email: 'alerts@example.com',
    })
    api.updateAutoReset.mockResolvedValue({})
  })

  it('loads and saves both trigger conditions', async () => {
    const wrapper = mount(AccountAutoResetModal, {
      props: { show: true, account },
      attachTo: document.body,
    })
    await flushPromises()

    const weeklyInput = document.body.querySelector<HTMLInputElement>('#auto-reset-weekly-threshold')
    const expiryInput = document.body.querySelector<HTMLInputElement>('#auto-reset-expiry-minutes')
    expect(weeklyInput?.value).toBe('90')
    expect(expiryInput?.value).toBe('1440')
    expect(weeklyInput).not.toBeNull()
    expect(expiryInput).not.toBeNull()
    expiryInput!.value = '30'
    expiryInput!.dispatchEvent(new Event('input', { bubbles: true }))
    document.body.querySelector<HTMLFormElement>('#account-auto-reset-form')!
      .dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(api.updateAutoReset).toHaveBeenCalledWith(42, {
      enabled: true,
      conditions: [
        { type: 'weekly_threshold', value: 90 },
        { type: 'credit_expiry', value: 30 },
      ],
      email: 'alerts@example.com',
    })
    expect(notifications.showSuccess).toHaveBeenCalled()
    expect(wrapper.emitted('close')).toBeTruthy()
    wrapper.unmount()
  })

  it('removes and adds trigger conditions independently', async () => {
    const wrapper = mount(AccountAutoResetModal, {
      props: { show: true, account },
      attachTo: document.body,
    })
    await flushPromises()

    const removeButtons = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button[aria-label="admin.accounts.autoResetRemoveCondition"]'))
    expect(removeButtons).toHaveLength(2)
    removeButtons[0].click()
    await wrapper.vm.$nextTick()
    expect(document.body.querySelector('#auto-reset-weekly-threshold')).toBeNull()
    expect(document.body.querySelector('#auto-reset-expiry-minutes')).not.toBeNull()

    const addButton = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent?.includes('admin.accounts.autoResetAddCondition'))
    expect(addButton).toBeDefined()
    addButton!.click()
    await wrapper.vm.$nextTick()
    expect(document.body.querySelector('#auto-reset-weekly-threshold')).not.toBeNull()

    wrapper.unmount()
  })
})
