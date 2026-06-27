<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="cosmic-shell cosmic-distortion-surface relative min-h-screen overflow-hidden text-slate-950 dark:text-white"
  >
    <!-- 星空与鼠标黑洞透镜背景 -->
    <div class="cosmic-backdrop cosmic-backdrop--absolute">
      <div class="starfield">
        <div class="stars-sm"></div>
        <div class="stars-md"></div>
        <div class="stars-lg"></div>
      </div>
      <div class="meteors">
        <i class="meteor" style="--a: 18deg; --dur: 5.5s; --delay: 0s"></i>
        <i class="meteor" style="--a: 75deg; --dur: 7s; --delay: 1.6s"></i>
        <i class="meteor" style="--a: 140deg; --dur: 6s; --delay: 3.2s"></i>
        <i class="meteor" style="--a: 205deg; --dur: 8s; --delay: 0.9s"></i>
        <i class="meteor" style="--a: 268deg; --dur: 6.5s; --delay: 4.4s"></i>
        <i class="meteor" style="--a: 325deg; --dur: 7.5s; --delay: 2.5s"></i>
      </div>
    </div>

    <!-- Extra tint for the pricing cockpit panels -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute left-[-18rem] top-[-14rem] h-[34rem] w-[34rem] rounded-full bg-amber-300/25 blur-3xl dark:bg-amber-500/10"></div>
      <div class="absolute right-[-12rem] top-24 h-[30rem] w-[30rem] rounded-full bg-cyan-400/20 blur-3xl dark:bg-cyan-400/10"></div>
      <div class="absolute inset-x-0 top-0 h-32 bg-gradient-to-b from-white/35 to-transparent dark:from-white/5"></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 px-5 py-4 sm:px-6">
      <nav
        class="mx-auto flex max-w-7xl items-center justify-between rounded-[1.75rem] border border-white/55 bg-white/55 px-4 py-3 shadow-[0_20px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl dark:border-white/10 dark:bg-white/[0.06] dark:shadow-[0_20px_80px_rgba(0,0,0,0.35)]"
      >
        <!-- Logo -->
        <div class="flex items-center gap-3">
          <div
            class="flex h-11 w-11 items-center justify-center overflow-hidden rounded-2xl border border-slate-900/10 bg-white shadow-lg shadow-amber-900/10 dark:border-white/10 dark:bg-white/10"
          >
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div class="hidden sm:block">
            <div class="text-sm font-black tracking-tight text-slate-950 dark:text-white">
              {{ siteName }}
            </div>
            <div class="text-[11px] uppercase tracking-[0.28em] text-slate-500 dark:text-slate-400">
              {{ t('home.navTagline') }}
            </div>
          </div>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-2 sm:gap-3">
          <LocaleSwitcher />

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-xl p-2 text-slate-500 transition-colors hover:bg-slate-900/5 hover:text-slate-950 dark:text-slate-300 dark:hover:bg-white/10 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <button
            @click="toggleTheme"
            class="rounded-xl p-2 text-slate-500 transition-colors hover:bg-slate-900/5 hover:text-slate-950 dark:text-slate-300 dark:hover:bg-white/10 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-2 rounded-full bg-slate-950 py-1.5 pl-1.5 pr-3 text-xs font-bold text-white shadow-lg shadow-slate-950/20 transition-transform hover:-translate-y-0.5 dark:bg-white dark:text-slate-950"
          >
            <span
              class="flex h-6 w-6 items-center justify-center rounded-full bg-gradient-to-br from-amber-300 to-cyan-300 text-[10px] font-black text-slate-950"
            >
              {{ userInitial }}
            </span>
            {{ t('home.dashboard') }}
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-slate-950 px-4 py-2 text-xs font-bold text-white shadow-lg shadow-slate-950/20 transition-transform hover:-translate-y-0.5 dark:bg-white dark:text-slate-950"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10 px-5 pb-16 pt-8 sm:px-6 lg:pb-24">
      <div class="mx-auto max-w-7xl">
        <!-- Hero -->
        <section class="grid gap-8 lg:grid-cols-[1.05fr_0.95fr] lg:items-center">
          <div>
            <div
              class="mb-6 inline-flex items-center gap-2 rounded-full border border-slate-900/10 bg-white/70 px-3 py-1.5 text-xs font-bold uppercase tracking-[0.22em] text-slate-600 shadow-sm backdrop-blur dark:border-white/10 dark:bg-white/[0.07] dark:text-slate-300"
            >
              <span class="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_20px_rgba(52,211,153,0.9)]"></span>
              {{ t('home.hero.kicker') }}
            </div>

            <h1
              class="max-w-4xl text-5xl font-black leading-[0.95] tracking-[-0.04em] text-slate-950 dark:text-white sm:text-6xl lg:text-6xl xl:text-7xl"
            >
              {{ t('home.hero.title') }}
              <span class="block whitespace-nowrap bg-gradient-to-r from-amber-500 via-orange-500 to-cyan-500 bg-clip-text text-transparent">
                {{ t('home.hero.titleAccent') }}
              </span>
            </h1>

            <div class="mt-8 flex flex-col gap-3 sm:flex-row">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="group inline-flex items-center justify-center rounded-2xl bg-slate-950 px-6 py-3.5 text-sm font-black text-white shadow-2xl shadow-slate-950/20 transition-all hover:-translate-y-1 hover:shadow-slate-950/30 dark:bg-white dark:text-slate-950"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" class="ml-2 transition-transform group-hover:translate-x-1" :stroke-width="2.3" />
              </router-link>
            </div>

          </div>

          <!-- Activity -->
          <aside
            class="relative overflow-hidden rounded-[2rem] border border-slate-900/10 bg-slate-950 p-6 text-white shadow-xl shadow-slate-900/20 dark:border-white/10"
          >
            <div class="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(251,191,36,0.28),transparent_36%),radial-gradient(circle_at_bottom_left,rgba(20,184,166,0.22),transparent_40%)]"></div>
            <div class="relative">
              <div class="inline-flex items-center gap-2 rounded-full bg-amber-300 px-3 py-1 text-xs font-black text-slate-950">
                <Icon name="fire" size="xs" :stroke-width="2.2" />
                {{ t('home.activity.badge') }}
              </div>
              <h2 class="mt-5 text-3xl font-black tracking-tight">
                {{ t('home.activity.title') }}
              </h2>
              <p class="mt-3 text-sm leading-7 text-slate-300">
                {{ t('home.activity.description') }}
              </p>

              <div class="mt-6 space-y-3">
                <div
                  v-for="item in activityHighlights"
                  :key="item.label"
                  class="flex items-center justify-between rounded-2xl border border-white/10 bg-white/[0.07] p-4"
                >
                  <div class="text-sm text-slate-300">{{ item.label }}</div>
                  <div class="text-lg font-black text-white">{{ item.value }}</div>
                </div>
              </div>

              <router-link
                :to="isAuthenticated ? '/usage' : '/login'"
                class="mt-6 inline-flex w-full items-center justify-center rounded-2xl bg-white px-5 py-3 text-sm font-black text-slate-950 transition-transform hover:-translate-y-1"
              >
                {{ isAuthenticated ? t('home.activity.ctaAuthed') : t('home.activity.ctaGuest') }}
                <Icon name="arrowRight" size="sm" class="ml-2" :stroke-width="2.3" />
              </router-link>
            </div>
          </aside>
        </section>

        <!-- Pricing Table -->
        <section class="mt-10">
          <div
            class="overflow-hidden rounded-[2rem] border border-slate-900/10 bg-white/70 shadow-xl shadow-slate-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-white/[0.06]"
          >
            <div class="flex flex-col gap-3 border-b border-slate-900/10 p-5 dark:border-white/10 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <div class="text-xs font-black uppercase tracking-[0.24em] text-amber-600 dark:text-amber-300">
                  {{ t('home.pricingTable.kicker') }}
                </div>
                <h2 class="mt-2 text-2xl font-black tracking-tight text-slate-950 dark:text-white">
                  {{ t('home.pricingTable.title') }}
                </h2>
                <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
                  {{ pricingTableSubtitle }}
                </p>
              </div>
            </div>

            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-slate-900/10 text-left text-sm dark:divide-white/10">
                <thead class="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                  <tr>
                    <th class="px-5 py-4 font-black">{{ t('home.pricingTable.model') }}</th>
                    <th class="px-5 py-4 font-black">{{ t('home.pricingTable.official') }}</th>
                    <th class="px-5 py-4 font-black">{{ t('home.pricingTable.multiplier') }}</th>
                    <th class="px-5 py-4 font-black">{{ t('home.pricingTable.cny') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-900/10 dark:divide-white/10">
                  <tr
                    v-for="item in featuredPrices"
                    :key="`${item.model}-${item.platform}`"
                    class="transition-colors hover:bg-slate-950/[0.03] dark:hover:bg-white/[0.04]"
                  >
                    <td class="px-5 py-4">
                      <div class="font-black text-slate-950 dark:text-white">{{ item.model }}</div>
                      <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                        {{ item.platform }}
                      </div>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 font-mono text-xs text-slate-700 dark:text-slate-300">
                      {{ formatPricePair(item.officialInput, item.officialOutput) }}
                    </td>
                    <td class="px-5 py-4">
                      <span
                        class="inline-flex rounded-full bg-cyan-400/15 px-2.5 py-1 text-xs font-black text-cyan-700 dark:text-cyan-200"
                      >
                        {{ formatMultiplier(item.multiplier) }}
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <div class="font-mono text-xs font-black text-emerald-700 dark:text-emerald-300">
                        {{ formatCnyPair(item.actualInput, item.actualOutput) }}
                      </div>
                      <div class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
                        {{ t('home.pricingTable.exchangeApplied') }}
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

        </section>

        <!-- Bottom CTA -->
        <section
          class="mt-10 overflow-hidden rounded-[2rem] border border-slate-900/10 bg-white/65 p-6 shadow-xl shadow-slate-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-white/[0.06] sm:p-8"
        >
          <div class="grid gap-6 lg:grid-cols-[1fr_auto] lg:items-center">
            <div>
              <div class="text-xs font-black uppercase tracking-[0.24em] text-slate-500 dark:text-slate-400">
                {{ t('home.cta.kicker') }}
              </div>
              <h2 class="mt-2 text-3xl font-black tracking-tight text-slate-950 dark:text-white">
                {{ t('home.cta.title') }}
              </h2>
            </div>
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="inline-flex items-center justify-center rounded-2xl bg-slate-950 px-6 py-3.5 text-sm font-black text-white shadow-xl shadow-slate-950/20 transition-transform hover:-translate-y-1 dark:bg-white dark:text-slate-950"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.cta.button') }}
            </router-link>
          </div>
        </section>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-slate-900/10 px-6 py-8 dark:border-white/10">
      <div class="mx-auto flex max-w-7xl flex-col items-center justify-between gap-4 text-center sm:flex-row sm:text-left">
        <p class="text-sm text-slate-500 dark:text-slate-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatScaled } from '@/utils/pricing'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const EXCHANGE_CNY_PER_100_CREDITS = 15
const CNY_PER_CREDIT = EXCHANGE_CNY_PER_100_CREDITS / 100
const PRICE_SCALE = 1_000_000

interface FeaturedPrice {
  model: string
  platform: string
  multiplier: number
  officialInput: number | null
  officialOutput: number | null
  actualInput: number | null
  actualOutput: number | null
}

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const currentYear = computed(() => new Date().getFullYear())

const featuredPrices: FeaturedPrice[] = [
  {
    model: 'gpt-5.5',
    platform: 'OpenAI',
    multiplier: 1,
    officialInput: 0.000005,
    officialOutput: 0.00003,
    actualInput: 0.000005,
    actualOutput: 0.00003
  },
  {
    model: 'gpt-5.4',
    platform: 'OpenAI',
    multiplier: 1,
    officialInput: 0.0000025,
    officialOutput: 0.000015,
    actualInput: 0.0000025,
    actualOutput: 0.000015
  }
]

const activityHighlights = computed(() => [
  { label: t('home.activity.items.period'), value: t('home.activity.values.period') },
  { label: t('home.activity.items.pool'), value: t('home.activity.values.pool') },
  { label: t('home.activity.items.reward'), value: t('home.activity.values.reward') },
  { label: t('home.activity.items.threshold'), value: t('home.activity.values.threshold') }
])

const pricingTableSubtitle = computed(() => t('home.pricingTable.fixedSubtitle'))

function formatMultiplier(rate: number): string {
  return `${Number(rate || 0).toFixed(2).replace(/\.00$/, '').replace(/0$/, '')}x`
}

function formatPricePair(input: number | null, output: number | null): string {
  return `${formatScaled(input, PRICE_SCALE)} / ${formatScaled(output, PRICE_SCALE)}`
}

function formatCnyPair(input: number | null, output: number | null): string {
  return `${formatCnyFromPrice(input)} / ${formatCnyFromPrice(output)}`
}

function formatCnyFromPrice(value: number | null): string {
  if (value == null) return '-'
  const cny = value * PRICE_SCALE * CNY_PER_CREDIT
  if (cny > 0 && cny < 0.01) return `¥${cny.toFixed(4)}`
  return `¥${cny.toFixed(2).replace(/\.00$/, '').replace(/0$/, '')}`
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
