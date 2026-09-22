<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { platformLabel } from '@/utils/platformColors'
import AppLayout from '@/components/layout/AppLayout.vue'
import GroupRateTrendChart from '@/components/charts/GroupRateTrendChart.vue'
import GroupRateHistoryChart from '@/components/charts/GroupRateHistoryChart.vue'
import { getGroupRateBoard, type GroupRateBoard, type GroupRateBoardItem, type GroupRateTrendPoint, type GroupRateUsage } from '@/api/groupRates'

const { t, locale } = useI18n()
const board = ref<GroupRateBoard | null>(null)
const loading = ref(false)
const failed = ref(false)
const search = ref('')
const platform = ref('all')
const selectedGroup = ref('all')
const metric = ref<'tokens' | 'requests'>('tokens')
const autoRefresh = ref(true)
let controller: AbortController | null = null
let timer: ReturnType<typeof setInterval> | undefined

const count = (value: number) => new Intl.NumberFormat(locale.value, { notation: 'compact', maximumFractionDigits: 2 }).format(value)
const exact = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const rate = (value: number) => new Intl.NumberFormat(locale.value, { maximumFractionDigits: 4 }).format(value)
const time = (value: string) => new Date(value).toLocaleString(locale.value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
const platforms = computed(() => [...new Set(board.value?.groups.map(group => group.platform) ?? [])].sort())
const groups = computed(() => {
  const term = search.value.trim().toLocaleLowerCase()
  return (board.value?.groups ?? []).filter(group =>
    (platform.value === 'all' || group.platform === platform.value) &&
    (!term || `${group.name} ${group.platform}`.toLocaleLowerCase().includes(term)),
  )
})
const chartGroups = computed(() => selectedGroup.value === 'all' ? groups.value : groups.value.filter(group => String(group.id) === selectedGroup.value))
watch(groups, value => {
  if (selectedGroup.value !== 'all' && !value.some(group => String(group.id) === selectedGroup.value)) selectedGroup.value = 'all'
})
const trend = computed<GroupRateTrendPoint[]>(() => {
  const points = new Map<string, GroupRateTrendPoint>()
  for (const group of chartGroups.value) for (const point of group.trend) {
    const previous = points.get(point.at) ?? { at: point.at, tokens: 0, requests: 0 }
    previous.tokens += point.tokens
    previous.requests += point.requests
    points.set(point.at, previous)
  }
  return [...points.values()].sort((a, b) => a.at.localeCompare(b.at))
})
function total(items: GroupRateBoardItem[], window: 'last_hour' | 'last_24_hours'): GroupRateUsage {
  const result: GroupRateUsage = { requests: 0, total_tokens: 0, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0 }
  for (const group of items) for (const key of Object.keys(result) as (keyof GroupRateUsage)[]) result[key] += group[window][key]
  return result
}
const lastHour = computed(() => total(groups.value, 'last_hour'))
const chartUsage = computed(() => total(chartGroups.value, 'last_24_hours'))
const dynamicCount = computed(() => groups.value.filter(group => group.dynamic_rate.enabled).length)
const breakdown = computed(() => [
  { key: 'input', value: chartUsage.value.input_tokens, color: 'bg-teal-500' },
  { key: 'output', value: chartUsage.value.output_tokens, color: 'bg-blue-500' },
  { key: 'cacheRead', value: chartUsage.value.cache_read_tokens, color: 'bg-violet-500' },
  { key: 'cacheWrite', value: chartUsage.value.cache_creation_tokens, color: 'bg-amber-500' },
])
function rangePosition(group: GroupRateBoardItem): number {
  const { min, max } = group.dynamic_rate
  if (max <= min) return 100
  return Math.max(0, Math.min(100, (group.rate_multiplier - min) / (max - min) * 100))
}
function showTrend(group: GroupRateBoardItem) {
  selectedGroup.value = String(group.id)
  document.getElementById('group-rate-trend')?.scrollIntoView({ block: 'start' })
}
async function reload() {
  if (loading.value) return
  const request = new AbortController()
  controller = request
  loading.value = true
  try {
    const data = await getGroupRateBoard(request.signal)
    if (request.signal.aborted) return
    board.value = data
    failed.value = false
  } catch (error) {
    if (request.signal.aborted) return
    const status = (error as { status?: number })?.status
    if (status === 401 || status === 403) board.value = null
    failed.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}
function onVisible() {
  if (!document.hidden && autoRefresh.value) void reload()
}
onMounted(() => {
  void reload()
  timer = setInterval(() => { if (!document.hidden && autoRefresh.value) void reload() }, 30000)
  document.addEventListener('visibilitychange', onVisible)
})
onBeforeUnmount(() => {
  controller?.abort()
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisible)
})
</script>

<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6" :aria-busy="loading">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-teal-600 dark:text-teal-400">
            <span class="h-2 w-2 rounded-full" :class="failed ? 'bg-amber-500' : 'bg-teal-500'" />
            {{ t('groupRates.live') }}
          </div>
          <h1 class="text-2xl font-bold tracking-tight text-gray-950 sm:text-3xl dark:text-white">{{ t('groupRates.title') }}</h1>
          <p class="mt-2 max-w-2xl text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('groupRates.description') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <label class="flex cursor-pointer items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="autoRefresh" type="checkbox" class="rounded border-gray-300 text-teal-600 focus:ring-teal-500" />
            {{ t('groupRates.autoRefresh') }}
          </label>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="reload">{{ loading ? t('common.loading') : t('groupRates.refresh') }}</button>
        </div>
      </header>

      <div v-if="failed" role="alert" class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
        {{ board ? t('groupRates.stale') : t('groupRates.loadError') }}
        <button type="button" class="ml-2 font-semibold underline" :disabled="loading" @click="reload">{{ t('common.tryAgain') }}</button>
      </div>

      <template v-if="board">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex w-full flex-wrap gap-2 sm:w-auto">
            <input v-model="search" class="input w-full sm:w-64" type="search" :placeholder="t('groupRates.search')" :aria-label="t('groupRates.search')" />
            <select v-model="platform" class="input w-full sm:w-44" :aria-label="t('groupRates.platform')">
              <option value="all">{{ t('groupRates.allPlatforms') }}</option>
              <option v-for="item in platforms" :key="item" :value="item">{{ platformLabel(item) }}</option>
            </select>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.updatedAt', { time: time(board.rates_at) }) }}</p>
        </div>

        <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <div v-for="stat in [
            { label: t('groupRates.availableGroups'), value: String(groups.length) },
            { label: t('groupRates.dynamicGroups'), value: String(dynamicCount) },
            { label: t('groupRates.hourTokens'), value: count(lastHour.total_tokens), title: exact(lastHour.total_tokens) },
            { label: t('groupRates.hourRequests'), value: count(lastHour.requests), title: exact(lastHour.requests) },
          ]" :key="stat.label" class="card p-4 sm:p-5">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
            <p class="mt-2 text-2xl font-semibold tabular-nums tracking-tight text-gray-950 sm:text-3xl dark:text-white" :title="stat.title">{{ stat.value }}</p>
          </div>
        </div>

        <div v-if="!groups.length" class="card px-6 py-14 text-center">
          <h2 class="font-semibold text-gray-800 dark:text-gray-200">{{ board.groups.length ? t('groupRates.noMatch') : t('groupRates.empty') }}</h2>
          <p class="mt-2 text-sm text-gray-500">{{ board.groups.length ? t('groupRates.noMatchHint') : t('groupRates.emptyHint') }}</p>
        </div>

        <template v-else>
          <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" :aria-label="t('groupRates.groups')">
            <article v-for="group in groups" :key="group.id" class="card flex flex-col overflow-hidden p-5" :data-group-id="group.id">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <h2 class="break-words font-semibold text-gray-950 dark:text-white">{{ group.name }}</h2>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ platformLabel(group.platform) }} · {{ group.subscription_type === 'subscription' ? t('groupRates.subscription') : t('groupRates.balance') }}</p>
                </div>
                <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium" :class="group.dynamic_rate.enabled ? 'bg-teal-50 text-teal-700 dark:bg-teal-950/40 dark:text-teal-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-gray-400'">{{ group.dynamic_rate.enabled ? t('groupRates.dynamic') : t('groupRates.fixed') }}</span>
              </div>
              <div class="my-5">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.baseRate') }}</p>
                <p class="mt-1 text-4xl font-semibold tabular-nums tracking-tight text-gray-950 dark:text-white">{{ rate(group.rate_multiplier) }}<span class="ml-1 text-xl font-normal text-gray-400">×</span></p>
                <div class="mt-3 flex flex-wrap items-center gap-2 text-xs">
                  <span class="font-medium text-teal-700 dark:text-teal-300">{{ t('groupRates.yourRate', { rate: rate(group.effective_multiplier) }) }}</span>
                  <span v-if="group.user_rate_multiplier !== undefined" class="text-gray-500 dark:text-gray-400">{{ t('groupRates.customRate') }}</span>
                  <span v-if="group.peak_multiplier !== 1" class="text-amber-600 dark:text-amber-400">{{ t('groupRates.peak', { rate: rate(group.peak_multiplier) }) }}</span>
                </div>
              </div>
              <div v-if="group.dynamic_rate.enabled" class="mb-5">
                <div class="flex justify-between gap-2 text-xs text-gray-500 dark:text-gray-400"><span>{{ t('groupRates.range') }}</span><span class="tabular-nums">{{ rate(group.dynamic_rate.min) }}–{{ rate(group.dynamic_rate.max) }}×</span></div>
                <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800"><div class="h-full rounded-full bg-teal-500" :style="{ width: `${rangePosition(group)}%` }" /></div>
                <p class="mt-2 text-xs text-gray-400">{{ group.rate_updated_at ? t('groupRates.rateUpdated', { time: time(group.rate_updated_at) }) : t('groupRates.awaitingUpdate') }}</p>
              </div>
              <dl class="mt-auto grid grid-cols-2 gap-x-4 gap-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
                <div v-for="stat in [
                  { key: 'hourTokens', value: group.last_hour.total_tokens },
                  { key: 'hourRequests', value: group.last_hour.requests },
                  { key: 'dayTokens', value: group.last_24_hours.total_tokens },
                  { key: 'dayRequests', value: group.last_24_hours.requests },
                ]" :key="stat.key"><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t(`groupRates.${stat.key}`) }}</dt><dd class="mt-1 font-semibold tabular-nums text-gray-900 dark:text-gray-100" :title="exact(stat.value)">{{ count(stat.value) }}</dd></div>
              </dl>
              <button type="button" class="mt-5 self-start text-xs font-semibold text-teal-700 hover:underline dark:text-teal-400" @click="showTrend(group)">{{ t('groupRates.viewTrend') }} <span aria-hidden="true">↗</span></button>
            </article>
          </section>

          <section id="group-rate-trend" class="card scroll-mt-24 p-4 sm:p-6">
            <div class="mb-6 flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 class="font-semibold text-gray-950 dark:text-white">{{ t('groupRates.trendTitle') }}</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.trendHint') }}</p>
              </div>
              <div class="flex max-w-full flex-wrap gap-2">
                <select v-model="selectedGroup" class="input max-w-full sm:max-w-60" :aria-label="t('groupRates.trendGroup')">
                  <option value="all">{{ t('groupRates.allVisibleGroups') }}</option>
                  <option v-for="group in groups" :key="group.id" :value="String(group.id)">{{ group.name }}</option>
                </select>
                <div class="flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
                  <button v-for="value in (['tokens', 'requests'] as const)" :key="value" type="button" class="rounded-md px-3 py-1 text-xs font-medium" :class="metric === value ? 'bg-white text-teal-700 shadow-sm dark:bg-dark-700 dark:text-teal-300' : 'text-gray-500 dark:text-gray-400'" :aria-pressed="metric === value" @click="metric = value">{{ value === 'tokens' ? 'Tokens' : t('groupRates.requests') }}</button>
                </div>
              </div>
            </div>
            <GroupRateTrendChart :points="trend" :metric="metric" />
            <div class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700">
              <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ t('groupRates.breakdown') }}</span>
                <span :title="exact(chartUsage.total_tokens)">{{ count(chartUsage.total_tokens) }} Tokens</span>
              </div>
              <div class="mt-3 flex h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800" aria-hidden="true">
                <div v-for="item in breakdown" :key="item.key" :class="item.color" :style="{ width: `${chartUsage.total_tokens ? item.value / chartUsage.total_tokens * 100 : 0}%` }" />
              </div>
              <dl class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div v-for="item in breakdown" :key="item.key">
                  <dt class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400"><span class="h-2 w-2 rounded-full" :class="item.color" />{{ t(`groupRates.${item.key}`) }}</dt>
                  <dd class="mt-1 text-sm font-medium tabular-nums text-gray-800 dark:text-gray-200" :title="exact(item.value)">{{ count(item.value) }}</dd>
                </div>
              </dl>
            </div>
          </section>

          <section class="card p-4 sm:p-6">
            <div class="mb-6 flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 class="font-semibold text-gray-950 dark:text-white">{{ t('groupRates.rateTrendTitle') }}</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.rateTrendHint') }}</p>
              </div>
              <select v-model="selectedGroup" class="input max-w-full sm:max-w-60" :aria-label="t('groupRates.trendGroup')">
                <option value="all">{{ t('groupRates.allVisibleGroups') }}</option>
                <option v-for="group in groups" :key="group.id" :value="String(group.id)">{{ group.name }}</option>
              </select>
            </div>
            <GroupRateHistoryChart :groups="chartGroups" />
          </section>
        </template>
        <footer class="space-y-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
          <p>{{ t('groupRates.usageAt', { time: time(board.usage_at) }) }}</p>
          <p>{{ t('groupRates.usageNote') }}</p>
          <p>{{ t('groupRates.rateNote') }}</p>
        </footer>
      </template>
      <div v-else-if="loading" class="grid gap-4 sm:grid-cols-3" role="status" :aria-label="t('common.loading')">
        <div v-for="index in 3" :key="index" class="card h-52 animate-pulse bg-gray-100 dark:bg-dark-800" />
      </div>
    </div>
  </AppLayout>
</template>
