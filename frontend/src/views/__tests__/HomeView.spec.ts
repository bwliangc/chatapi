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
  it('lists every GPT-5.6 model with its official input and output price', () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          LocaleSwitcher: true,
          Icon: true,
        },
      },
    })

    const expected = [
      ['gpt-5.6-sol', '$5 / $30', '¥0.75 / ¥4.5'],
      ['gpt-5.6-terra', '$2.5 / $15', '¥0.38 / ¥2.25'],
      ['gpt-5.6-luna', '$1 / $6', '¥0.15 / ¥0.9'],
    ]
    const rows = wrapper.findAll('tbody tr')

    expected.forEach(([model, officialPrice, cnyPrice]) => {
      const row = rows.find((candidate) => candidate.text().includes(model))
      expect(row, `${model} pricing row`).toBeDefined()
      expect(row!.text()).toContain(officialPrice)
      expect(row!.text()).toContain('1x')
      expect(row!.text()).toContain(cnyPrice)
    })

    expect(rows.some((row) => row.text().includes('gpt-5.5'))).toBe(true)
    expect(rows.some((row) => row.text().includes('gpt-5.4'))).toBe(true)
  })
})
