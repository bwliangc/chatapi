<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { platformLabel } from '@/utils/platformColors'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
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
const HISTORY_LIMIT = 8
const autoRefresh = ref(true)
let controller: AbortController | null = null
let timer: ReturnType<typeof setInterval> | undefined

const count = (value: number) => new Intl.NumberFormat(locale.value, { notation: 'compact', maximumFractionDigits: 2 }).format(value)
const exact = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const rate = (value: number) => new Intl.NumberFormat(locale.value, { maximumFractionDigits: 4 }).format(value)
const time = (value: string) => new Date(value).toLocaleString(locale.value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
const platforms = computed(() => [...new Set(board.value?.groups.map(group => group.platform) ?? [])].sort())
const platformOptions = computed(() => [
  { value: 'all', label: t('groupRates.allPlatforms') },
  ...platforms.value.map(item => ({ value: item, label: platformLabel(item) })),
])
const groups = computed(() => {
  const term = search.value.trim().toLocaleLowerCase()
  return (board.value?.groups ?? []).filter(group =>
    (platform.value === 'all' || group.platform === platform.value) &&
    (!term || `${group.name} ${group.platform}`.toLocaleLowerCase().includes(term)),
  )
})
// 固定按近 24 小时 Token 用量从高到低排序
const sortedGroups = computed(() => [...groups.value].sort((a, b) => b.last_24_hours.total_tokens - a.last_24_hours.total_tokens))
const trendGroupOptions = computed(() => [
  { value: 'all', label: t('groupRates.allVisibleGroups') },
  ...sortedGroups.value.map(group => ({ value: String(group.id), label: group.name })),
])
// 领奖台：按近 24 小时 Token 用量取前三；暂无用量时也展示（按默认顺序占位）
const podium = computed(() => [...groups.value]
  .sort((a, b) => b.last_24_hours.total_tokens - a.last_24_hours.total_tokens)
  .slice(0, 3))
const showPodium = computed(() => podium.value.length >= 2)
const dayTokensTotal = computed(() => groups.value.reduce((sum, group) => sum + group.last_24_hours.total_tokens, 0))
const PODIUM_STYLES = [
  { label: 'champion', order: 'sm:order-2', badge: 'from-amber-300 to-amber-500 text-amber-950', ring: 'ring-2 ring-amber-400/70', glow: 'from-amber-100/80 dark:from-amber-500/15', bar: 'bg-amber-400', pedestal: 'h-24 from-amber-300/90 to-amber-500/90', value: 'text-4xl' },
  { label: 'runnerUp', order: 'sm:order-1', badge: 'from-slate-200 to-slate-400 text-slate-800', ring: 'ring-1 ring-slate-300/80 dark:ring-slate-500/50', glow: 'from-slate-100/80 dark:from-slate-400/10', bar: 'bg-slate-400', pedestal: 'h-16 from-slate-300/90 to-slate-400/90', value: 'text-3xl' },
  { label: 'third', order: 'sm:order-3', badge: 'from-orange-300 to-orange-600 text-orange-950', ring: 'ring-1 ring-orange-300/70 dark:ring-orange-500/40', glow: 'from-orange-100/70 dark:from-orange-500/10', bar: 'bg-orange-400', pedestal: 'h-10 from-orange-300/90 to-orange-500/90', value: 'text-3xl' },
] as const
const podiumSlots = computed(() => showPodium.value
  ? podium.value.map((group, index) => ({ group, rank: index + 1, style: PODIUM_STYLES[index], share: dayTokensTotal.value ? group.last_24_hours.total_tokens / dayTokensTotal.value * 100 : 0 }))
  : [])
const restGroups = computed(() => {
  const ids = new Set(podiumSlots.value.map(slot => slot.group.id))
  return sortedGroups.value.filter(group => !ids.has(group.id))
})
// 其他分组：页面上只放一行折叠卡片，数量按容器宽度铺满一行，其余收进「+N」，点击任意卡片打开全部分组弹窗
// 卡片最小宽 11rem(176px)，相邻重叠 3rem(48px)，「+N」卡 7rem(112px)
const FOLD_CARD = 176
const FOLD_STEP = 128
const FOLD_TILE = 64
const foldRow = ref<HTMLElement | null>(null)
const foldWidth = ref(0)
let foldObserver: ResizeObserver | undefined
watch(foldRow, el => {
  foldObserver?.disconnect()
  if (!el || typeof ResizeObserver === 'undefined') return
  foldObserver = new ResizeObserver(([entry]) => { foldWidth.value = entry.contentRect.width })
  foldObserver.observe(el)
})
const foldCapacity = computed(() => {
  const total = restGroups.value.length
  if (!foldWidth.value) return Math.min(total, 7)
  if (FOLD_CARD + (total - 1) * FOLD_STEP <= foldWidth.value) return total
  return Math.max(1, Math.floor((foldWidth.value - FOLD_CARD - FOLD_TILE) / FOLD_STEP) + 1)
})
const foldGroups = computed(() => restGroups.value.slice(0, foldCapacity.value))
const foldHiddenCount = computed(() => restGroups.value.length - foldGroups.value.length)
const allGroupsOpen = ref(false)
const focusGroupId = ref<number | null>(null)
// 点击卡片 → 弹窗从被点的卡片位置「长」出来（FLIP），卡片随后依次弹入
const MORPH_MS = 420
async function openAllGroups(groupId: number | undefined, event?: MouseEvent) {
  const origin = (event?.currentTarget as HTMLElement | null)?.getBoundingClientRect()
  focusGroupId.value = groupId ?? null
  allGroupsOpen.value = true
  await nextTick()
  const grid = document.querySelector<HTMLElement>('[data-all-groups-grid]')
  const panel = grid?.closest<HTMLElement>('.modal-content')
  if (panel && origin && !reduceMotion) {
    const target = panel.getBoundingClientRect()
    const dx = origin.left + origin.width / 2 - (target.left + target.width / 2)
    const dy = origin.top + origin.height / 2 - (target.top + target.height / 2)
    const sx = origin.width / target.width
    const sy = origin.height / target.height
    panel.animate([
      { transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})`, opacity: 0.4 },
      { transform: 'none', opacity: 1 },
    ], { duration: MORPH_MS, easing: 'cubic-bezier(0.2, 0.85, 0.25, 1)' })
  }
  if (groupId === undefined) return
  document.querySelector(`[data-modal-group-id="${groupId}"]`)?.scrollIntoView({ block: 'nearest' })
}
const chartGroups = computed(() => selectedGroup.value === 'all' ? groups.value : groups.value.filter(group => String(group.id) === selectedGroup.value))
// 倍率历史图分组过多时只画 24h 用量最高的几个，避免线条颜色重复、图例挤成一团
const groupsWithHistory = computed(() => chartGroups.value.filter(group => group.rate_trend.length > 0))
const historyGroups = computed(() => groupsWithHistory.value.length <= HISTORY_LIMIT
  ? groupsWithHistory.value
  : [...groupsWithHistory.value].sort((a, b) => b.last_24_hours.total_tokens - a.last_24_hours.total_tokens).slice(0, HISTORY_LIMIT))
watch(groups, value => {
  if (selectedGroup.value !== 'all' && !value.some(group => String(group.id) === selectedGroup.value)) selectedGroup.value = 'all'
})
// 趋势分析默认选中冠军分组（即用量排序第一的分组，无用量时也选它）；用户手动选过后不再自动切换
const trendPicked = ref(false)
const championId = computed(() => {
  const champion = sortedGroups.value[0]
  return champion ? String(champion.id) : null
})
watch(championId, id => {
  if (!trendPicked.value) selectedGroup.value = id ?? 'all'
}, { immediate: true })
function pickTrendGroup(value: unknown) {
  trendPicked.value = true
  selectedGroup.value = String(value ?? 'all')
}
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
const SPOT_COLORS = ['251, 191, 36', '148, 163, 184', '251, 146, 60']
const reduceMotion = typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
// 鼠标跟随光晕；tilt 为 true 时附带轻微 3D 倾斜（仅领奖台）
function onSpotMove(event: PointerEvent, tilt = false) {
  if (event.pointerType !== 'mouse') return
  const el = event.currentTarget as HTMLElement
  const rect = el.getBoundingClientRect()
  const x = event.clientX - rect.left
  const y = event.clientY - rect.top
  el.style.setProperty('--spot-x', `${x}px`)
  el.style.setProperty('--spot-y', `${y}px`)
  if (tilt && !reduceMotion) {
    const px = x / rect.width - 0.5
    const py = y / rect.height - 0.5
    el.style.transform = `perspective(900px) rotateX(${(-py * 5).toFixed(2)}deg) rotateY(${(px * 5).toFixed(2)}deg) translateY(-2px)`
  }
}
function onSpotLeave(event: PointerEvent) {
  (event.currentTarget as HTMLElement).style.transform = ''
}
function showTrend(group: GroupRateBoardItem) {
  trendPicked.value = true
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
  foldObserver?.disconnect()
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
            <Select v-model="platform" class="w-full sm:w-44" :options="platformOptions" :aria-label="t('groupRates.platform')" />
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
          <section v-if="showPodium" class="space-y-4" :aria-label="t('groupRates.podiumTitle')">
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <h2 class="text-lg font-semibold text-gray-950 dark:text-white">{{ t('groupRates.podiumTitle') }}</h2>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ dayTokensTotal ? t('groupRates.podiumHint') : t('groupRates.podiumEmptyHint') }}</p>
            </div>
            <div class="grid items-end gap-4 sm:grid-cols-3">
              <div v-for="slot in podiumSlots" :key="slot.group.id" class="podium-rise flex flex-col" :class="slot.style.order" :style="{ animationDelay: `${[2, 1, 3][slot.rank - 1] * 120}ms` }">
                <article class="spotlight card relative overflow-hidden p-5" :class="[slot.style.ring, { 'podium-champion': slot.rank === 1 }]" :style="{ '--spot-color': SPOT_COLORS[slot.rank - 1] }" :data-group-id="slot.group.id" @pointermove="onSpotMove($event, true)" @pointerleave="onSpotLeave">
                  <span class="spot-glow" aria-hidden="true" />
                  <div class="pointer-events-none absolute inset-x-0 top-0 h-28 bg-gradient-to-b to-transparent" :class="slot.style.glow" />
                  <div class="relative">
                    <div class="flex items-start justify-between gap-3">
                      <div class="flex min-w-0 items-center gap-3">
                        <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-br text-sm font-black shadow-sm" :class="slot.style.badge">
                          <svg v-if="slot.rank === 1" class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M3 7l4.5 4L12 4l4.5 7L21 7l-2 12H5L3 7z" /></svg>
                          <template v-else>{{ slot.rank }}</template>
                        </span>
                        <div class="min-w-0">
                          <p class="text-xs font-semibold" :class="slot.rank === 1 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-500 dark:text-gray-400'">{{ t(`groupRates.${slot.style.label}`) }}</p>
                          <h3 class="truncate font-semibold text-gray-950 dark:text-white" :title="slot.group.name">{{ slot.group.name }}</h3>
                        </div>
                      </div>
                      <span class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium" :class="slot.group.dynamic_rate.enabled ? 'bg-teal-50 text-teal-700 dark:bg-teal-950/40 dark:text-teal-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-gray-400'">{{ slot.group.dynamic_rate.enabled ? t('groupRates.dynamic') : t('groupRates.fixed') }}</span>
                    </div>
                    <p class="mt-1 pl-[3.25rem] text-xs text-gray-500 dark:text-gray-400">{{ platformLabel(slot.group.platform) }}<template v-if="slot.group.subscription_type === 'subscription'"> · {{ t('groupRates.subscription') }}</template></p>
                    <div class="mt-5">
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.dayTokens') }}</p>
                      <p v-if="slot.group.last_24_hours.total_tokens > 0" class="mt-1 font-semibold tabular-nums tracking-tight text-gray-950 dark:text-white" :class="slot.style.value" :title="exact(slot.group.last_24_hours.total_tokens)">{{ count(slot.group.last_24_hours.total_tokens) }}</p>
                      <p v-else class="mt-1 font-semibold tracking-tight text-gray-300 dark:text-dark-500" :class="slot.style.value">{{ t('groupRates.noUsage') }}</p>
                      <div class="mt-3 flex items-center gap-2">
                        <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="podium-bar h-full rounded-full" :class="slot.style.bar" :style="{ width: `${slot.share}%` }" /></div>
                        <span class="text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ t('groupRates.share', { pct: rate(Math.round(slot.share * 10) / 10) }) }}</span>
                      </div>
                    </div>
                    <dl class="mt-5 grid grid-cols-2 gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
                      <div>
                        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.currentRate') }}</dt>
                        <dd class="mt-0.5 font-semibold tabular-nums text-teal-700 dark:text-teal-300">{{ rate(slot.group.effective_multiplier) }}×
                          <span v-if="slot.group.effective_multiplier !== slot.group.rate_multiplier" class="ml-1 text-[11px] font-normal text-gray-400">({{ t('groupRates.colBaseRate') }} {{ rate(slot.group.rate_multiplier) }}×)</span>
                        </dd>
                      </div>
                      <div><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.dayRequests') }}</dt><dd class="mt-0.5 font-semibold tabular-nums text-gray-900 dark:text-gray-100" :title="exact(slot.group.last_24_hours.requests)">{{ count(slot.group.last_24_hours.requests) }}</dd></div>
                      <div><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.hourTokens') }}</dt><dd class="mt-0.5 font-semibold tabular-nums text-gray-900 dark:text-gray-100" :title="exact(slot.group.last_hour.total_tokens)">{{ count(slot.group.last_hour.total_tokens) }}</dd></div>
                      <div><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.hourRequests') }}</dt><dd class="mt-0.5 font-semibold tabular-nums text-gray-900 dark:text-gray-100" :title="exact(slot.group.last_hour.requests)">{{ count(slot.group.last_hour.requests) }}</dd></div>
                    </dl>
                    <button type="button" class="mt-4 text-xs font-semibold text-teal-700 hover:underline dark:text-teal-400" @click="showTrend(slot.group)">{{ t('groupRates.viewTrend') }} <span aria-hidden="true">↗</span></button>
                  </div>
                </article>
                <div class="mx-5 hidden items-center justify-center rounded-b-xl bg-gradient-to-b text-2xl font-black text-white/90 shadow-inner sm:flex" :class="slot.style.pedestal" aria-hidden="true">{{ slot.rank }}</div>
              </div>
            </div>
          </section>

          <section v-if="restGroups.length" :aria-label="t('groupRates.otherGroups')">
            <!-- 一行重叠的折叠卡片，数量按容器宽度铺满一行：悬停放大并把后面的卡片推开，点击打开全部分组弹窗。
                 不用 overflow 滚动容器，避免放大的卡片被裁切；右侧预留让位空间 -->
            <div ref="foldRow" class="fold-row flex pb-2 pr-11 pt-3">
              <button
                v-for="(group, index) in foldGroups"
                :key="group.id"
                type="button"
                class="fold-card spotlight card relative flex min-w-[11rem] max-w-[20rem] flex-1 flex-col overflow-hidden bg-white p-4 text-left dark:bg-dark-800"
                :class="{ 'pr-14': index < foldGroups.length - 1 || foldHiddenCount > 0 }"
                :style="{ zIndex: index + 1 }"
                :data-group-id="group.id"
                :title="group.name"
                @pointermove="onSpotMove($event)"
                @click="openAllGroups(group.id, $event)"
              >
                <span class="spot-glow" aria-hidden="true" />
                <span class="relative flex min-w-0 items-center gap-1.5">
                  <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="group.dynamic_rate.enabled ? 'fold-live bg-teal-500' : 'bg-gray-300 dark:bg-dark-500'" />
                  <span class="truncate text-sm font-semibold text-gray-950 dark:text-white">{{ group.name }}</span>
                </span>
                <span class="relative mt-0.5 truncate text-[11px] text-gray-500 dark:text-gray-400">{{ platformLabel(group.platform) }}</span>
                <span class="relative mt-3 text-2xl font-semibold tabular-nums tracking-tight text-gray-950 dark:text-white">{{ rate(group.effective_multiplier) }}<span class="ml-0.5 text-sm font-normal text-gray-400">×</span></span>
                <span class="relative mt-3 border-t border-gray-100 pt-2 text-[11px] text-gray-500 dark:border-dark-700 dark:text-gray-400">24h <b class="tabular-nums text-gray-800 dark:text-gray-200">{{ count(group.last_24_hours.total_tokens) }}</b> Tokens</span>
                <span class="relative text-[11px] text-gray-500 dark:text-gray-400">1h <b class="tabular-nums text-gray-800 dark:text-gray-200">{{ count(group.last_hour.total_tokens) }}</b> Tokens</span>
              </button>
              <button
                v-if="foldHiddenCount > 0"
                type="button"
                class="fold-card card relative flex w-28 shrink-0 flex-col items-center justify-center gap-1 bg-gray-50 p-4 text-teal-700 dark:bg-dark-800 dark:text-teal-300"
                :style="{ zIndex: foldGroups.length + 1 }"
                @click="openAllGroups(undefined, $event)"
              >
                <span class="text-2xl font-semibold tabular-nums">+{{ foldHiddenCount }}</span>
                <span class="text-xs">{{ t('groupRates.viewAll') }}</span>
              </button>
            </div>
          </section>

          <BaseDialog :show="allGroupsOpen" :title="showPodium ? t('groupRates.allGroupsTitle', { count: restGroups.length }) : `${t('groupRates.groups')} (${restGroups.length})`" width="full" close-on-click-outside @close="allGroupsOpen = false">
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4" data-all-groups-grid>
              <article v-for="(group, index) in restGroups" :key="group.id" class="modal-card-in spotlight card relative flex flex-col overflow-hidden p-4" :class="focusGroupId === group.id ? 'modal-card-focus ring-2 ring-teal-500/60' : ''" :style="{ animationDelay: `${(reduceMotion ? 0 : MORPH_MS - 120) + Math.min(index, 16) * 35}ms` }" :data-modal-group-id="group.id" @pointermove="onSpotMove($event)">
                <span class="spot-glow" aria-hidden="true" />
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <h4 class="truncate font-semibold text-gray-950 dark:text-white" :title="group.name">{{ group.name }}</h4>
                    <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{{ platformLabel(group.platform) }}<template v-if="group.subscription_type === 'subscription'"> · {{ t('groupRates.subscription') }}</template></p>
                  </div>
                  <span class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium" :class="group.dynamic_rate.enabled ? 'bg-teal-50 text-teal-700 dark:bg-teal-950/40 dark:text-teal-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-gray-400'">{{ group.dynamic_rate.enabled ? t('groupRates.dynamic') : t('groupRates.fixed') }}</span>
                </div>
                <div class="my-4">
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.currentRate') }}</p>
                  <p class="mt-1 text-3xl font-semibold tabular-nums tracking-tight text-gray-950 dark:text-white">{{ rate(group.effective_multiplier) }}<span class="ml-1 text-lg font-normal text-gray-400">×</span></p>
                  <div v-if="group.effective_multiplier !== group.rate_multiplier" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.baseRate') }} {{ rate(group.rate_multiplier) }}×</div>
                  <div class="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
                    <span v-if="group.user_rate_multiplier !== undefined" class="font-medium text-teal-700 dark:text-teal-300">{{ t('groupRates.customRate') }}</span>
                    <span v-if="group.peak_multiplier !== 1" class="text-amber-600 dark:text-amber-400">{{ t('groupRates.peak', { rate: rate(group.peak_multiplier) }) }}</span>
                  </div>
                </div>
                <div v-if="group.dynamic_rate.enabled" class="mb-4">
                  <div class="flex justify-between gap-2 text-xs text-gray-500 dark:text-gray-400"><span>{{ t('groupRates.range') }}</span><span class="tabular-nums">{{ rate(group.dynamic_rate.min) }}–{{ rate(group.dynamic_rate.max) }}×</span></div>
                  <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800"><div class="h-full rounded-full bg-teal-500" :style="{ width: `${rangePosition(group)}%` }" /></div>
                  <p class="mt-1.5 text-xs text-gray-400">{{ group.rate_updated_at ? t('groupRates.rateUpdated', { time: time(group.rate_updated_at) }) : t('groupRates.awaitingUpdate') }}</p>
                </div>
                <dl class="mt-auto grid grid-cols-2 gap-x-4 gap-y-3 border-t border-gray-100 pt-3 dark:border-dark-700">
                  <div v-for="stat in [
                    { key: 'hourTokens', value: group.last_hour.total_tokens },
                    { key: 'hourRequests', value: group.last_hour.requests },
                    { key: 'dayTokens', value: group.last_24_hours.total_tokens },
                    { key: 'dayRequests', value: group.last_24_hours.requests },
                  ]" :key="stat.key"><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t(`groupRates.${stat.key}`) }}</dt><dd class="mt-0.5 text-sm font-semibold tabular-nums text-gray-900 dark:text-gray-100" :title="exact(stat.value)">{{ count(stat.value) }}</dd></div>
                </dl>
                <button type="button" class="mt-4 self-start text-xs font-semibold text-teal-700 hover:underline dark:text-teal-400" @click="allGroupsOpen = false; showTrend(group)">{{ t('groupRates.viewTrend') }} <span aria-hidden="true">↗</span></button>
              </article>
            </div>
          </BaseDialog>

          <section id="group-rate-trend" class="scroll-mt-24 space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h2 class="text-lg font-semibold text-gray-950 dark:text-white">{{ t('groupRates.trendSection') }}</h2>
              <Select :model-value="selectedGroup" class="w-full sm:w-64" :options="trendGroupOptions" :aria-label="t('groupRates.trendGroup')" @update:model-value="pickTrendGroup" />
            </div>
            <div class="grid gap-4 xl:grid-cols-2">
              <div class="card p-4 sm:p-6">
                <div class="mb-6 flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h3 class="font-semibold text-gray-950 dark:text-white">{{ t('groupRates.trendTitle') }}</h3>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.trendHint') }}</p>
                  </div>
                  <div class="flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
                    <button v-for="value in (['tokens', 'requests'] as const)" :key="value" type="button" class="rounded-md px-3 py-1 text-xs font-medium" :class="metric === value ? 'bg-white text-teal-700 shadow-sm dark:bg-dark-700 dark:text-teal-300' : 'text-gray-500 dark:text-gray-400'" :aria-pressed="metric === value" @click="metric = value">{{ value === 'tokens' ? 'Tokens' : t('groupRates.requests') }}</button>
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
              </div>

              <div class="card p-4 sm:p-6">
                <div class="mb-6">
                  <h3 class="font-semibold text-gray-950 dark:text-white">{{ t('groupRates.rateTrendTitle') }}</h3>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('groupRates.rateTrendHint') }}</p>
                  <p v-if="historyGroups.length < groupsWithHistory.length" class="mt-1 text-xs text-amber-600 dark:text-amber-400">{{ t('groupRates.historyTruncated', { shown: historyGroups.length, total: groupsWithHistory.length }) }}</p>
                </div>
                <GroupRateHistoryChart :groups="historyGroups" />
              </div>
            </div>
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

<style scoped>
.podium-rise {
  animation: podium-rise 0.7s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}
@keyframes podium-rise {
  from { opacity: 0; transform: translateY(28px); }
  to { opacity: 1; transform: none; }
}
.podium-bar {
  transition: width 0.8s ease;
}
/* 冠军卡片：一道光扫过，只播放 3 次，避免常驻动画占用 GPU */
.podium-champion::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(105deg, transparent 35%, rgba(251, 191, 36, 0.22) 50%, transparent 65%);
  transform: translateX(-100%);
  animation: podium-shine 2.4s ease-in-out 0.9s 3;
}
@keyframes podium-shine {
  to { transform: translateX(100%); }
}
@media (prefers-reduced-motion: reduce) {
  .podium-rise, .podium-champion::after { animation: none; }
}
/* 鼠标跟随光效：只在悬停时显示，CSS 变量由 onSpotMove 写入，不常驻动画 */
.spotlight {
  --spot-x: 50%;
  --spot-y: 50%;
  --spot-color: 20, 184, 166;
  transition: transform 0.2s ease-out, box-shadow 0.3s ease;
}
.spotlight:hover {
  box-shadow: 0 18px 40px -20px rgba(var(--spot-color), 0.55);
}
.spotlight::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 1;
  padding: 1.5px;
  border-radius: inherit;
  background: radial-gradient(220px circle at var(--spot-x) var(--spot-y), rgba(var(--spot-color), 0.9), transparent 55%);
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#000 0 0) content-box exclude, linear-gradient(#000 0 0);
  opacity: 0;
  transition: opacity 0.3s ease;
  pointer-events: none;
}
.spot-glow {
  position: absolute;
  inset: 0;
  background: radial-gradient(320px circle at var(--spot-x) var(--spot-y), rgba(var(--spot-color), 0.12), transparent 45%);
  opacity: 0;
  transition: opacity 0.3s ease;
  pointer-events: none;
}
.spotlight:hover::before,
.spotlight:hover .spot-glow {
  opacity: 1;
}
/* 折叠卡片行：后一张压住前一张右侧，悬停时放大上浮、后面的卡片向右让位 */
.fold-card {
  box-shadow: -10px 0 18px -14px rgba(15, 23, 42, 0.45);
  transform-origin: center bottom;
  transition: transform 0.28s cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 0.28s ease;
}
.fold-card + .fold-card {
  margin-left: -3rem;
}
.fold-card:hover,
.fold-card:focus-visible {
  z-index: 50 !important;
  transform: translateY(-8px) scale(1.12);
  box-shadow: 0 22px 40px -18px rgba(13, 148, 136, 0.55);
}
.fold-card:hover ~ .fold-card,
.fold-card:focus-visible ~ .fold-card {
  transform: translateX(2.75rem);
}
.fold-live {
  animation: fold-pulse 2.4s ease-in-out infinite;
}
@keyframes fold-pulse {
  50% { opacity: 0.35; }
}
/* 展开全部弹窗：卡片依次从下方弹入；点击进入的那张卡片额外闪一圈高亮 */
.modal-card-in {
  animation: modal-card-in 0.45s cubic-bezier(0.2, 0.9, 0.3, 1.15) both;
}
@keyframes modal-card-in {
  from { opacity: 0; transform: translateY(18px) scale(0.94); }
  to { opacity: 1; transform: none; }
}
.modal-card-focus {
  animation: modal-card-in 0.45s cubic-bezier(0.2, 0.9, 0.3, 1.15) both, modal-card-focus 1.2s ease-out 0.45s 2;
}
@keyframes modal-card-focus {
  0% { box-shadow: 0 0 0 0 rgba(20, 184, 166, 0.5); }
  100% { box-shadow: 0 0 0 14px rgba(20, 184, 166, 0); }
}
@media (prefers-reduced-motion: reduce) {
  .fold-card, .fold-card:hover ~ .fold-card { transition: none; }
  .fold-card:hover, .fold-card:focus-visible { transform: none; }
  .fold-card:hover ~ .fold-card { transform: none; }
  .fold-live, .modal-card-in, .modal-card-focus { animation: none; }
}
</style>
