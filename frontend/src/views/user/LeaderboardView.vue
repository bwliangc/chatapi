<template>
  <AppLayout>
    <div class="mx-auto w-[90%] max-w-4xl">
      <div class="mb-4">
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
          {{ t('leaderboard.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('leaderboard.description') }}
        </p>
      </div>

      <!-- 今日/昨日切换 -->
      <div class="mb-4 inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-800">
        <button
          type="button"
          class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors"
          :class="period === 'today'
            ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-400'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
          @click="switchPeriod('today')"
        >
          {{ t('leaderboard.tabToday') }}
        </button>
        <button
          type="button"
          class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors"
          :class="period === 'yesterday'
            ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-400'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
          @click="switchPeriod('yesterday')"
        >
          {{ t('leaderboard.tabYesterday') }}
        </button>
      </div>

      <!-- 奖池/规则信息 -->
      <div v-if="data" class="card mb-4 p-5">
        <div v-if="showReward" class="grid grid-cols-2 gap-4">
          <div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('leaderboard.poolAmount') }}</div>
            <div class="mt-0.5 text-lg font-semibold text-amber-600 dark:text-amber-400">{{ money(data.pool_amount) }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('leaderboard.totalSpend') }}</div>
            <div class="mt-0.5 text-sm font-medium text-gray-900 dark:text-white">{{ money(data.total_cost) }}</div>
          </div>
        </div>
        <div v-else class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('leaderboard.rewardDisabledNote') }}
          <span class="ml-1 text-gray-700 dark:text-gray-300">{{ t('leaderboard.totalSpend') }}: {{ money(data.total_cost) }}</span>
        </div>
        <p v-if="hasThreshold" class="mt-3 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/15 dark:text-amber-400">
          {{ t('leaderboard.thresholdNote', { amount: money(data.min_spend), topN: data.top_n }) }}
        </p>
        <p v-if="showReward && distributionText" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('leaderboard.distributionRatio') }}{{ distributionText }}
        </p>
        <p class="mt-2 text-xs text-gray-400">{{ periodHint }}</p>
      </div>

      <!-- 榜单 -->
      <div class="card overflow-hidden">
        <div v-if="loading" class="p-8 text-center text-sm text-gray-400">{{ t('common.loading') }}</div>
        <div v-else-if="!data || (data.ranking.length === 0 && !data.me)" class="p-8 text-center text-sm text-gray-400">
          {{ t('leaderboard.empty') }}
        </div>
        <table v-else class="w-full text-sm">
          <thead>
            <tr class="border-b border-gray-100 text-left text-xs text-gray-400 dark:border-dark-700">
              <th class="px-4 py-2 font-medium">{{ t('leaderboard.rankHeader') }}</th>
              <th class="px-4 py-2 font-medium">{{ t('leaderboard.userHeader') }}</th>
              <th class="px-4 py-2 text-right font-medium">{{ t('leaderboard.spendHeader') }}</th>
              <th v-if="period === 'yesterday'" class="px-4 py-2 text-right font-medium">{{ t('leaderboard.rewardResult') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="entry in data.ranking"
              :key="entry.rank"
              class="border-b border-gray-50 last:border-0 dark:border-dark-800"
              :class="[
                entry.is_me ? 'bg-primary-50/50 dark:bg-primary-900/10' : '',
                entry.is_winner ? 'bg-amber-50/40 dark:bg-amber-900/5' : '',
              ]"
            >
              <td class="px-4 py-2.5">
                <span class="inline-flex h-6 w-6 items-center justify-center font-semibold" :class="rankClass(entry.rank, entry.is_winner)">
                  {{ medal(entry.rank, entry.is_winner) }}
                </span>
              </td>
              <td class="px-4 py-2.5">
                <span class="text-gray-900 dark:text-gray-100">{{ entry.name }}</span>
                <span
                  v-if="entry.is_me"
                  class="ml-2 rounded bg-primary-100 px-1.5 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
                >{{ t('leaderboard.youLabel') }}</span>
                <span
                  v-if="entry.is_winner"
                  class="ml-2 rounded bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                >🏆</span>
              </td>
              <td class="px-4 py-2.5 text-right font-medium text-gray-900 dark:text-gray-100">{{ money(entry.actual_cost) }}</td>
              <td v-if="period === 'yesterday'" class="px-4 py-2.5 text-right font-medium">
                <span v-if="entry.reward_amount" class="text-amber-600 dark:text-amber-400">{{ money(entry.reward_amount) }}</span>
                <span v-else class="text-gray-400">—</span>
              </td>
            </tr>
            <!-- 当前榜单没有消费数据时的占位提示 -->
            <tr v-if="data.ranking.length === 0">
              <td :colspan="period === 'yesterday' ? 4 : 3" class="px-4 py-6 text-center text-sm text-gray-400">{{ t('leaderboard.noWinnersYet') }}</td>
            </tr>
            <!-- 本人未出现在当前页时，用虚线分隔后单独展示全站排名。 -->
            <template v-if="data.me && !meInList">
              <tr aria-hidden="true">
                <td :colspan="period === 'yesterday' ? 4 : 3" class="px-4 py-1.5">
                  <div class="border-t-2 border-dashed border-gray-300 dark:border-gray-600"></div>
                </td>
              </tr>
              <tr class="bg-primary-50/50 dark:bg-primary-900/10">
                <td class="px-4 py-2.5">
                  <span class="inline-flex h-6 w-6 items-center justify-center font-semibold" :class="rankClass(data.me.rank, data.me.is_winner)">
                    {{ medal(data.me.rank, data.me.is_winner) }}
                  </span>
                </td>
                <td class="px-4 py-2.5">
                  <span class="text-gray-900 dark:text-gray-100">{{ data.me.name }}</span>
                  <span class="ml-2 rounded bg-primary-100 px-1.5 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ t('leaderboard.youLabel') }}</span>
                  <span v-if="meBelowThreshold" class="ml-2 rounded bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500 dark:bg-dark-700 dark:text-gray-400">{{ t('leaderboard.belowThreshold') }}</span>
                  <span v-if="data.me.is_winner" class="ml-2 rounded bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">🏆</span>
                </td>
                <td class="px-4 py-2.5 text-right font-medium text-gray-900 dark:text-gray-100">{{ money(data.me.actual_cost) }}</td>
                <td v-if="period === 'yesterday'" class="px-4 py-2.5 text-right font-medium">
                  <span v-if="data.me.reward_amount" class="text-amber-600 dark:text-amber-400">{{ money(data.me.reward_amount) }}</span>
                  <span v-else class="text-gray-400">—</span>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
        <Pagination
          v-if="data && data.total > data.page_size"
          :page="data.page"
          :total="data.total"
          :page-size="data.page_size"
          :show-page-size-selector="false"
          @update:page="handlePageChange"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import {
  getLeaderboard,
  type LeaderboardResponse,
  type LeaderboardPeriod,
} from '@/api/leaderboard'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const period = ref<LeaderboardPeriod>('today')
const pageSize = 10
const pageByPeriod = reactive<Record<LeaderboardPeriod, number>>({ today: 1, yesterday: 1 })
// 按周期和页码缓存，切换 tab 或翻回已访问页时可立即显示。
const cache = ref<Record<string, LeaderboardResponse>>({})
const loadingKeys = reactive(new Set<string>())
const cacheKey = (p: LeaderboardPeriod, page: number) => `${p}:${page}`
const currentCacheKey = computed(() => cacheKey(period.value, pageByPeriod[period.value]))
const data = computed<LeaderboardResponse | null>(
  () => cache.value[currentCacheKey.value] ?? null,
)
const loading = computed(() => loadingKeys.has(currentCacheKey.value))

const showReward = computed(() => !!data.value?.reward_enabled)
// 本人是否已出现在当前页中（用于决定是否在表底用虚线单独补一行本人排名）。
const meInList = computed(() => !!data.value?.ranking?.some((e) => e.is_me))
// 本人是否未达参与门槛（用于在本人行标注「未达门槛」）。
const meBelowThreshold = computed(() => {
  const d = data.value
  return !!(d?.me && d.reward_enabled && (d.min_spend || 0) > 0 && d.me.actual_cost < d.min_spend)
})
const hasThreshold = computed(
  () => showReward.value && (data.value?.min_spend || 0) > 0,
)
const periodHint = computed(() =>
  period.value === 'today'
    ? t('leaderboard.hintToday')
    : t('leaderboard.hintYesterday'),
)

// 分配比例文案：weighted 显示各名次实际占比；其余按平均分配显示。
const distributionText = computed(() => {
  const d = data.value
  if (!d) return ''
  if (d.distribution_mode === 'weighted') {
    const shares = d.distribution_shares
    if (shares && shares.length) {
      return shares
        .map((pct, i) => ({ rank: i + 1, pct }))
        .filter((s) => s.pct > 0)
        .map((s) => t('leaderboard.rankShare', { rank: s.rank, pct: s.pct }))
        .join(' · ')
    }
    return t('leaderboard.modeAverage') // 权重无效时实际按平均分配
  }
  return t('leaderboard.modeAverage')
})

async function load(
  p: LeaderboardPeriod = period.value,
  page: number = pageByPeriod[p],
) {
  const key = cacheKey(p, page)
  // 仅在该页尚无缓存时显示加载态；有缓存则静默刷新。
  if (!cache.value[key]) loadingKeys.add(key)
  try {
    const res = await getLeaderboard(p, page, pageSize)
    pageByPeriod[p] = res.page
    cache.value = { ...cache.value, [cacheKey(p, res.page)]: res }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loadingKeys.delete(key)
  }
}

function switchPeriod(p: LeaderboardPeriod) {
  if (period.value === p) return
  period.value = p // 立即切到该周期缓存，无需等待
  const page = pageByPeriod[p]
  // 昨日已结算（静态）：有缓存则不重复请求；今日实时：始终后台刷新。
  if (p === 'today' || !cache.value[cacheKey(p, page)]) load(p, page)
}

function handlePageChange(page: number) {
  pageByPeriod[period.value] = page
  load(period.value, page)
}

function money(n: number): string {
  const v = Number(n) || 0
  if (v > 0 && v < 1) return '$' + v.toFixed(4)
  return '$' + v.toFixed(2)
}

// 奖牌只对中奖者显示；未中奖（含未达门槛）只显示纯名次数字，避免误导。
function medal(rank: number, isWinner: boolean): string {
  if (isWinner) {
    if (rank === 1) return '🥇'
    if (rank === 2) return '🥈'
    if (rank === 3) return '🥉'
  }
  return String(rank)
}

function rankClass(rank: number, isWinner: boolean): string {
  if (isWinner && rank <= 3) return 'text-base'
  return 'text-gray-400'
}

onMounted(() => load())
</script>
