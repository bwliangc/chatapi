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

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('HomeView default home', () => {
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
})
