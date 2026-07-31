import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import HomeView from '../HomeView.vue'

const { checkAuth, fetchPublicSettings, appStore } = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  fetchPublicSettings: vi.fn(),
  appStore: {
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
    fetchPublicSettings: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    isAuthenticated: false,
    isAdmin: false,
    user: null,
    checkAuth,
  }),
  useAppStore: () => ({ ...appStore, fetchPublicSettings }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('HomeView transparent pricing', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings.doc_url = ''
  })

  it('shows the configured documentation link below the primary action', () => {
    appStore.cachedPublicSettings.doc_url = 'https://docs.example.com/guide'

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          LocaleSwitcher: true,
          Icon: true,
        },
      },
    })

    const docLink = wrapper.get('[data-home-doc-link]')
    expect(docLink.attributes('href')).toBe('https://docs.example.com/guide')
    expect(docLink.attributes('target')).toBe('_blank')
    expect(docLink.attributes('rel')).toBe('noopener noreferrer')
  })

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
      { model: 'gpt-5.6-terra', prices: ['$2', '$0.2', '$2.5', '$12'] },
      { model: 'gpt-5.6-luna', prices: ['$0.2', '$0.02', '$0.25', '$1.2'] },
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
    ).toBe('¥0.375')
    expect(
      desktop.get('[data-model="gpt-5.6-luna"] [data-price-kind="cacheRead"] [data-price-currency="cny"]').text(),
    ).toBe('¥0.003')
  })
})
