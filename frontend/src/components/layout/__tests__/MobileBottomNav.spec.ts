import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import MobileBottomNav from '../MobileBottomNav.vue'
import { useAppStore, useAuthStore } from '@/stores'

const messages = {
  en: {
    common: { more: () => 'More', navigation: () => 'Primary navigation' },
    nav: {
      dashboard: () => 'Dashboard',
      apiKeys: () => 'API Keys',
      usage: () => 'Usage',
      subscriptions: () => 'Subscriptions',
      accounts: () => 'Accounts',
      users: () => 'Users'
    }
  }
}

function makeRouter(initialPath: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      '/dashboard', '/keys', '/usage', '/purchase', '/profile',
      '/admin/dashboard', '/admin/accounts', '/admin/users', '/admin/usage'
    ].map((path) => ({ path, component: { template: '<div />' } }))
  })
  return router.push(initialPath).then(() => router)
}

async function mountNav(path: string, role: 'admin' | 'user' = 'user') {
  const pinia = createPinia()
  setActivePinia(pinia)
  const authStore = useAuthStore()
  authStore.user = {
    id: '1',
    username: 'mobile-user',
    email: 'mobile@example.com',
    role
  } as typeof authStore.user

  const router = await makeRouter(path)
  const i18n = createI18n({ legacy: false, locale: 'en', messages })
  const wrapper = mount(MobileBottomNav, {
    global: { plugins: [pinia, router, i18n] }
  })
  await router.isReady()
  return { wrapper, appStore: useAppStore() }
}

describe('MobileBottomNav', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('shows the user primary destinations and marks the current page', async () => {
    const { wrapper } = await mountNav('/keys')

    const links = wrapper.findAll('a')
    expect(links.map((link) => link.attributes('href'))).toEqual([
      '/dashboard', '/keys', '/usage', '/purchase'
    ])
    expect(links[1].attributes('aria-current')).toBe('page')
  })

  it('uses admin destinations for administrators', async () => {
    const { wrapper } = await mountNav('/admin/accounts', 'admin')

    expect(wrapper.findAll('a').map((link) => link.attributes('href'))).toEqual([
      '/admin/dashboard', '/admin/accounts', '/admin/users', '/admin/usage'
    ])
  })

  it('opens the complete navigation from More', async () => {
    const { wrapper, appStore } = await mountNav('/profile')

    const moreButton = wrapper.get('button')
    expect(moreButton.classes()).toContain('mobile-bottom-nav-item-active')
    await moreButton.trigger('click')
    expect(appStore.mobileOpen).toBe(true)
  })
})
