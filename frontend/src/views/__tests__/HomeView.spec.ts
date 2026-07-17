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
  it('shows one set of base prices for all context lengths', () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          LocaleSwitcher: true,
          Icon: true,
        },
      },
    })

    const rows = [
      { model: 'gpt-5.6-sol', prices: ['$5', '$0.5', '$6.25', '$30'] },
      { model: 'gpt-5.6-terra', prices: ['$2.5', '$0.25', '$3.125', '$15'] },
      { model: 'gpt-5.6-luna', prices: ['$1', '$0.1', '$1.25', '$6'] },
      { model: 'gpt-5.5', prices: ['$5', '$0.5', '-', '$30'] },
      { model: 'gpt-5.4', prices: ['$2.5', '$0.25', '-', '$15'] },
    ]
    const priceKinds = ['input', 'cacheRead', 'cacheWrite', 'output']

    const expectPrices = () => {
      for (const layout of ['desktop', 'mobile']) {
        const pricingLayout = wrapper.get(`[data-pricing-layout="${layout}"]`)
        expect(pricingLayout.findAll('[data-model]')).toHaveLength(5)
        rows.forEach(({ model, prices }) => {
          const row = pricingLayout.get(`[data-model="${model}"]`)
          priceKinds.forEach((kind, index) => {
            expect(row.get(`[data-price-kind="${kind}"] [data-price-currency="usd"]`).text()).toBe(prices[index])
          })
        })
      }
    }

    expect(wrapper.find('[data-context-select]').exists()).toBe(false)
    expectPrices()

    const desktop = wrapper.get('[data-pricing-layout="desktop"]')
    expect(
      desktop.get('[data-model="gpt-5.6-terra"] [data-price-kind="cacheWrite"] [data-price-currency="cny"]').text(),
    ).toBe('¥0.46875')
    expect(
      desktop.get('[data-model="gpt-5.6-luna"] [data-price-kind="cacheRead"] [data-price-currency="cny"]').text(),
    ).toBe('¥0.015')
  })
})
