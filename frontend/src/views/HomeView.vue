<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
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

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
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
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div class="hidden sm:block">
            <div class="text-sm font-black tracking-tight text-slate-950 dark:text-white">
              {{ siteName }}
            </div>
            <div class="max-w-48 truncate text-[11px] text-slate-500 dark:text-slate-400">
              {{ siteSubtitle }}
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

          <!-- Model Plaza Link -->
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="inline-flex items-center gap-1.5 rounded-xl p-2 text-sm text-slate-500 transition-colors hover:bg-slate-900/5 hover:text-slate-950 dark:text-slate-300 dark:hover:bg-white/10 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>

          <!-- Theme Toggle -->
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

            <div class="mt-8 flex flex-col items-start gap-3">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="group inline-flex items-center justify-center rounded-2xl bg-slate-950 px-6 py-3.5 text-sm font-black text-white shadow-2xl shadow-slate-950/20 transition-all hover:-translate-y-1 hover:shadow-slate-950/30 dark:bg-white dark:text-slate-950"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" class="ml-2 transition-transform group-hover:translate-x-1" :stroke-width="2.3" />
              </router-link>

              <a
                v-if="docUrl"
                data-home-doc-link
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-2 px-1 text-sm font-bold text-slate-600 transition-colors hover:text-slate-950 dark:text-slate-300 dark:hover:text-white"
              >
                <Icon name="book" size="sm" />
                {{ t('home.viewDocs') }}
              </a>
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
            <div
              class="flex flex-col gap-4 border-b border-slate-900/10 p-5 dark:border-white/10 md:flex-row md:items-end md:justify-between"
            >
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

              <div class="flex flex-wrap items-center gap-3 md:justify-end">
                <div class="flex items-baseline gap-2 text-xs text-slate-500 dark:text-slate-400">
                  <span>{{ t('home.pricingTable.uniformMultiplier') }}</span>
                  <strong class="font-mono text-sm text-cyan-700 dark:text-cyan-200">
                    {{ formatMultiplier(uniformMultiplier) }}
                  </strong>
                </div>
              </div>
            </div>

            <div data-pricing-layout="desktop" class="hidden overflow-x-auto lg:block">
              <table class="w-full min-w-[52rem] divide-y divide-slate-900/10 text-left text-sm dark:divide-white/10">
                <thead class="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                  <tr>
                    <th class="px-5 py-4 font-black">{{ t('home.pricingTable.model') }}</th>
                    <th
                      v-for="priceKind in priceKinds"
                      :key="priceKind.key"
                      class="px-5 py-4 text-right font-black"
                    >
                      {{ priceKind.label }}
                    </th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-900/10 dark:divide-white/10">
                  <tr
                    v-for="item in featuredPrices"
                    :key="item.model"
                    :data-model="item.model"
                    class="transition-colors hover:bg-slate-950/[0.03] dark:hover:bg-white/[0.04]"
                  >
                    <td class="px-5 py-4">
                      <div class="font-black text-slate-950 dark:text-white">{{ item.model }}</div>
                    </td>
                    <td
                      v-for="priceKind in priceKinds"
                      :key="priceKind.key"
                      :data-price-kind="priceKind.key"
                      class="px-5 py-4 text-right"
                    >
                      <div
                        data-price-currency="usd"
                        class="whitespace-nowrap font-mono text-sm font-bold text-slate-800 dark:text-slate-200"
                      >
                        {{ formatPrice(item.prices[priceKind.key]) }}
                      </div>
                      <div
                        data-price-currency="cny"
                        class="mt-1 whitespace-nowrap font-mono text-xs font-bold text-emerald-700 dark:text-emerald-300"
                      >
                        {{
                          formatCnyFromPrice(
                            item.prices[priceKind.key],
                            item.multiplier,
                          )
                        }}
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <ul data-pricing-layout="mobile" class="divide-y divide-slate-900/10 dark:divide-white/10 lg:hidden">
              <li
                v-for="item in featuredPrices"
                :key="`${item.model}-mobile`"
                :data-model="item.model"
                class="px-5 py-4"
              >
                <div class="break-words text-sm font-black text-slate-950 dark:text-white">
                  {{ item.model }}
                </div>

                <dl class="mt-2 divide-y divide-slate-900/[0.06] dark:divide-white/[0.06]">
                  <div
                    v-for="priceKind in priceKinds"
                    :key="priceKind.key"
                    :data-price-kind="priceKind.key"
                    class="grid grid-cols-[minmax(0,1fr)_auto_auto] items-baseline gap-3 py-1.5"
                  >
                    <dt class="min-w-0 text-xs text-slate-500 dark:text-slate-400">
                      {{ priceKind.label }}
                    </dt>
                    <dd
                      data-price-currency="usd"
                      class="whitespace-nowrap font-mono text-xs font-bold text-slate-800 dark:text-slate-200"
                    >
                      {{ formatPrice(item.prices[priceKind.key]) }}
                    </dd>
                    <dd
                      data-price-currency="cny"
                      class="min-w-[4.75rem] whitespace-nowrap text-right font-mono text-xs font-bold text-emerald-700 dark:text-emerald-300"
                    >
                      {{
                        formatCnyFromPrice(
                          item.prices[priceKind.key],
                          item.multiplier,
                        )
                      }}
                    </dd>
                  </div>
                </dl>
              </li>
            </ul>
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
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const EXCHANGE_CNY_PER_100_CREDITS = 15
const CNY_PER_CREDIT = EXCHANGE_CNY_PER_100_CREDITS / 100
const PRICE_SCALE = 1_000_000

type PriceKind = 'input' | 'cacheRead' | 'cacheWrite' | 'output'

interface TokenPrices {
  input: number | null
  cacheRead: number | null
  cacheWrite: number | null
  output: number | null
}

interface FeaturedPrice {
  model: string
  multiplier: number
  prices: TokenPrices
}

type LeaderboardActivityPublicSettings = {
  leaderboard_reward_pool_rate?: unknown
  leaderboard_reward_top_n?: unknown
}

const priceKinds = computed<Array<{ key: PriceKind; label: string }>>(() => [
  { key: 'input', label: t('home.pricingTable.priceInput') },
  { key: 'cacheRead', label: t('home.pricingTable.priceCacheRead') },
  { key: 'cacheWrite', label: t('home.pricingTable.priceCacheWrite') },
  { key: 'output', label: t('home.pricingTable.priceOutput') }
])

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const currentYear = computed(() => new Date().getFullYear())

const uniformMultiplier = 1

const featuredPrices: FeaturedPrice[] = [
  {
    model: 'gpt-5.6-sol',
    multiplier: uniformMultiplier,
    prices: {
      input: 0.000005,
      cacheRead: 0.0000005,
      cacheWrite: 0.00000625,
      output: 0.00003
    }
  },
  {
    model: 'gpt-5.6-terra',
    multiplier: uniformMultiplier,
    prices: {
      input: 0.000002,
      cacheRead: 0.0000002,
      cacheWrite: 0.0000025,
      output: 0.000012
    }
  },
  {
    model: 'gpt-5.6-luna',
    multiplier: uniformMultiplier,
    prices: {
      input: 0.0000002,
      cacheRead: 0.00000002,
      cacheWrite: 0.00000025,
      output: 0.0000012
    }
  },
  {
    model: 'gpt-5.5',
    multiplier: uniformMultiplier,
    prices: {
      input: 0.000005,
      cacheRead: 0.0000005,
      cacheWrite: null,
      output: 0.00003
    }
  },
  {
    model: 'gpt-5.4',
    multiplier: uniformMultiplier,
    prices: {
      input: 0.0000025,
      cacheRead: 0.00000025,
      cacheWrite: null,
      output: 0.000015
    }
  }
]

const activityHighlights = computed(() => [
  { label: t('home.activity.items.period'), value: t('home.activity.values.period') },
  { label: t('home.activity.items.pool'), value: activityPoolValue.value },
  { label: t('home.activity.items.reward'), value: activityRewardValue.value },
  { label: t('home.activity.items.threshold'), value: t('home.activity.values.threshold') }
])

const pricingTableSubtitle = computed(() => t('home.pricingTable.fixedSubtitle'))

const leaderboardActivitySettings = computed(
  () => appStore.cachedPublicSettings as LeaderboardActivityPublicSettings | null,
)

const leaderboardPoolRate = computed(() => {
  const value = Number(leaderboardActivitySettings.value?.leaderboard_reward_pool_rate)
  return Number.isFinite(value) ? value : null
})

const leaderboardTopN = computed(() => {
  const value = Number(leaderboardActivitySettings.value?.leaderboard_reward_top_n)
  return Number.isFinite(value) ? Math.max(0, Math.floor(value)) : null
})

const activityPoolValue = computed(() => {
  if (leaderboardPoolRate.value == null) return t('home.activity.values.pool')
  return t('home.activity.values.configuredPool', { rate: formatActivityNumber(leaderboardPoolRate.value) })
})

const activityRewardValue = computed(() => {
  if (leaderboardTopN.value == null) return t('home.activity.values.reward')
  return t('home.activity.values.configuredReward', { topN: leaderboardTopN.value })
})

function formatMultiplier(rate: number): string {
  return `${Number(rate || 0).toFixed(2).replace(/\.00$/, '').replace(/0$/, '')}x`
}

function formatActivityNumber(value: number): string {
  return value.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1')
}

function formatPrice(value: number | null): string {
  return formatScaled(value, PRICE_SCALE)
}

function formatCnyFromPrice(value: number | null, multiplier: number): string {
  if (value == null) return '-'
  const cny = value * PRICE_SCALE * multiplier * CNY_PER_CREDIT
  return `¥${cny.toFixed(5).replace(/\.?0+$/, '')}`
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
