import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import HomeView from '../HomeView.vue'

const { checkAuth, fetchPublicSettings } = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  fetchPublicSettings: vi.fn(),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    isAuthenticated: false,
    isAdmin: false,
    user: null,
    checkAuth,
  }),
  useAppStore: () => ({
    publicSettingsLoaded: true,
    cachedPublicSettings: {
      site_name: 'Test Site',
      site_logo: '',
      site_subtitle: '',
      doc_url: '',
      home_content: '',
    },
    siteName: 'Test Site',
    siteLogo: '',
    docUrl: '',
    fetchPublicSettings,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('HomeView transparent pricing', () => {
  it('lists official short and long context prices including cache read and write', () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          LocaleSwitcher: true,
          Icon: true,
        },
      },
    })

    const expectedRows = [
      { model: 'gpt-5.6-sol', tier: 'short', prices: ['$5', '$0.5', '$6.25', '$30'] },
      { model: 'gpt-5.6-sol', tier: 'long', prices: ['$10', '$1', '$12.5', '$45'] },
      { model: 'gpt-5.6-terra', tier: 'short', prices: ['$2.5', '$0.25', '$3.125', '$15'] },
      { model: 'gpt-5.6-terra', tier: 'long', prices: ['$5', '$0.5', '$6.25', '$22.5'] },
      { model: 'gpt-5.6-luna', tier: 'short', prices: ['$1', '$0.1', '$1.25', '$6'] },
      { model: 'gpt-5.6-luna', tier: 'long', prices: ['$2', '$0.2', '$2.5', '$9'] },
      { model: 'gpt-5.5', tier: 'short', prices: ['$5', '$0.5', '-', '$30'] },
      { model: 'gpt-5.5', tier: 'long', prices: ['$10', '$1', '-', '$45'] },
      { model: 'gpt-5.4', tier: 'short', prices: ['$2.5', '$0.25', '-', '$15'] },
      { model: 'gpt-5.4', tier: 'long', prices: ['$5', '$0.5', '-', '$22.5'] },
    ]
    const priceKinds = ['input', 'cacheRead', 'cacheWrite', 'output']

    for (const layout of ['desktop', 'mobile']) {
      const pricingLayout = wrapper.get(`[data-pricing-layout="${layout}"]`)
      expectedRows.forEach(({ model, tier, prices }) => {
        const row = pricingLayout.get(`[data-model="${model}"][data-context-tier="${tier}"]`)
        expect(row.text()).toContain('1x')
        priceKinds.forEach((kind, index) => {
          expect(row.get(`[data-price-currency="usd"][data-price-kind="${kind}"]`).text()).toBe(prices[index])
        })
      })
    }

    const desktop = wrapper.get('[data-pricing-layout="desktop"]')
    const terraShort = desktop.get('[data-model="gpt-5.6-terra"][data-context-tier="short"]')
    const lunaShort = desktop.get('[data-model="gpt-5.6-luna"][data-context-tier="short"]')
    const solLong = desktop.get('[data-model="gpt-5.6-sol"][data-context-tier="long"]')
    expect(terraShort.get('[data-price-currency="cny"][data-price-kind="cacheWrite"]').text()).toBe('¥0.46875')
    expect(lunaShort.get('[data-price-currency="cny"][data-price-kind="cacheRead"]').text()).toBe('¥0.015')
    expect(solLong.get('[data-price-currency="cny"][data-price-kind="output"]').text()).toBe('¥6.75')
  })
})
