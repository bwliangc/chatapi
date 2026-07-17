<template>
  <nav
    v-if="shouldShow"
    class="mobile-bottom-nav lg:hidden"
    :aria-label="t('common.navigation')"
  >
    <div class="grid h-16 grid-cols-5">
      <router-link
        v-for="item in primaryItems"
        :key="item.path"
        :to="item.path"
        class="mobile-bottom-nav-item"
        :class="{ 'mobile-bottom-nav-item-active': isActive(item) }"
        :aria-current="isActive(item) ? 'page' : undefined"
        @click="closeMenu"
      >
        <Icon :name="item.icon" size="md" :stroke-width="isActive(item) ? 2 : 1.5" />
        <span>{{ t(item.labelKey) }}</span>
      </router-link>

      <button
        type="button"
        class="mobile-bottom-nav-item"
        :class="{ 'mobile-bottom-nav-item-active': menuIsActive }"
        :aria-expanded="appStore.mobileOpen"
        aria-controls="app-sidebar"
        @click="appStore.toggleMobileSidebar"
      >
        <Icon name="menu" size="md" :stroke-width="menuIsActive ? 2 : 1.5" />
        <span>{{ t('common.more') }}</span>
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'

type MobileNavItem = {
  path: string
  labelKey: string
  icon: 'home' | 'key' | 'chart' | 'creditCard' | 'server' | 'users'
  activePrefixes?: string[]
}

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const userItems: MobileNavItem[] = [
  { path: '/dashboard', labelKey: 'nav.dashboard', icon: 'home' },
  { path: '/keys', labelKey: 'nav.apiKeys', icon: 'key' },
  { path: '/usage', labelKey: 'nav.usage', icon: 'chart' },
  {
    path: '/purchase',
    labelKey: 'nav.subscriptions',
    icon: 'creditCard',
    activePrefixes: ['/purchase', '/subscriptions', '/orders', '/payment']
  }
]

const adminItems: MobileNavItem[] = [
  { path: '/admin/dashboard', labelKey: 'nav.dashboard', icon: 'home' },
  { path: '/admin/accounts', labelKey: 'nav.accounts', icon: 'server' },
  { path: '/admin/users', labelKey: 'nav.users', icon: 'users' },
  { path: '/admin/usage', labelKey: 'nav.usage', icon: 'chart' }
]

const primaryItems = computed(() => (authStore.isAdmin ? adminItems : userItems))
const shouldShow = computed(
  () => Boolean(authStore.user) && (authStore.isAdmin || !appStore.backendModeEnabled)
)

function matchesPath(path: string) {
  return route.path === path || route.path.startsWith(`${path}/`)
}

function isActive(item: MobileNavItem) {
  return [item.path, ...(item.activePrefixes || [])].some(matchesPath)
}

const menuIsActive = computed(
  () => appStore.mobileOpen || !primaryItems.value.some(isActive)
)

function closeMenu() {
  appStore.setMobileOpen(false)
}
</script>

<style scoped>
.mobile-bottom-nav {
  position: fixed;
  z-index: 35;
  right: 0;
  bottom: 0;
  left: 0;
  padding-bottom: env(safe-area-inset-bottom);
  border-top: 1px solid rgb(203 213 225 / 0.75);
  background: rgb(255 255 255 / 0.92);
  box-shadow: 0 -8px 30px rgb(31 81 137 / 0.1);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
}

.dark .mobile-bottom-nav {
  border-color: rgb(30 39 66 / 0.9);
  background: rgb(14 20 38 / 0.92);
  box-shadow: 0 -8px 30px rgb(0 0 0 / 0.24);
}

.mobile-bottom-nav-item {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 0.25rem;
  color: rgb(100 116 139);
  font-size: 0.6875rem;
  font-weight: 500;
  line-height: 1rem;
  transition: color 150ms ease, background-color 150ms ease;
  -webkit-tap-highlight-color: transparent;
}

.mobile-bottom-nav-item span {
  width: 100%;
  overflow: hidden;
  padding: 0 0.2rem;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-bottom-nav-item-active {
  color: rgb(43 92 206);
  background: rgb(238 244 254 / 0.72);
}

.dark .mobile-bottom-nav-item {
  color: rgb(159 173 200);
}

.dark .mobile-bottom-nav-item-active {
  color: rgb(147 184 245);
  background: rgb(34 56 107 / 0.34);
}

@media (prefers-reduced-motion: reduce) {
  .mobile-bottom-nav-item {
    transition: none;
  }
}
</style>
