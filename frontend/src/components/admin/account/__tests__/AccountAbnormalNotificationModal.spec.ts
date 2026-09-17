import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountAbnormalNotificationModal from '../AccountAbnormalNotificationModal.vue'
import type { Account } from '@/types'

const api = vi.hoisted(() => ({ getAbnormalNotification: vi.fn(), updateAbnormalNotification: vi.fn() }))
const notifications = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: api } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => notifications }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

enableAutoUnmount(afterEach)

const account = { id: 42, name: 'codex-account', platform: 'openai', type: 'oauth' } as Account
const mountModal = () => mount(AccountAbnormalNotificationModal, {
  props: { show: true, account },
  global: {
    stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } },
  },
})

describe('AccountAbnormalNotificationModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getAbnormalNotification.mockResolvedValue({
      enabled: true, email: 'alerts@example.com', statuses: ['error', 'rate_limited'],
    })
    api.updateAbnormalNotification.mockResolvedValue({})
  })

  it('loads saved choices and saves only selected states', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('input[value="error"]').element.checked).toBe(true)
    expect(wrapper.get<HTMLInputElement>('input[value="rate_limited"]').element.checked).toBe(true)
    expect(wrapper.get<HTMLInputElement>('input[value="overloaded"]').element.checked).toBe(false)

    await wrapper.get('input[value="error"]').setValue(false)
    await wrapper.get('input[value="overloaded"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.updateAbnormalNotification).toHaveBeenCalledWith(42, {
      enabled: true, email: 'alerts@example.com', statuses: ['rate_limited', 'overloaded'],
    })
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })

  it('prevents enabled notifications with no states and permits turning them off', async () => {
    const wrapper = mountModal()
    await flushPromises()
    await wrapper.get('input[value="error"]').setValue(false)
    await wrapper.get('input[value="rate_limited"]').setValue(false)
    expect(wrapper.get<HTMLButtonElement>('button[type="submit"]').element.disabled).toBe(true)
    await wrapper.get('form').trigger('submit')
    expect(api.updateAbnormalNotification).not.toHaveBeenCalled()

    await wrapper.get('input[type="checkbox"]:not([value])').setValue(false)
    expect(wrapper.get<HTMLFieldSetElement>('fieldset').element.disabled).toBe(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.updateAbnormalNotification).toHaveBeenCalledWith(42, {
      enabled: false, email: 'alerts@example.com', statuses: [],
    })
  })

  it('keeps legacy settings limited to account errors', async () => {
    api.getAbnormalNotification.mockResolvedValue({ enabled: true, email: 'alerts@example.com' })
    const wrapper = mountModal()
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(api.updateAbnormalNotification).toHaveBeenCalledWith(42, {
      enabled: true, email: 'alerts@example.com', statuses: ['error'],
    })
  })

  it('does not overwrite settings when loading the next account fails', async () => {
    const wrapper = mountModal()
    await flushPromises()
    api.getAbnormalNotification.mockRejectedValue(new Error('load failed'))
    await wrapper.setProps({ account: { ...account, id: 43 } })
    await flushPromises()
    expect(wrapper.text()).toContain('load failed')
    expect(wrapper.get<HTMLButtonElement>('button[type="submit"]').element.disabled).toBe(true)
    await wrapper.get('form').trigger('submit')
    expect(api.updateAbnormalNotification).not.toHaveBeenCalled()
  })
})
