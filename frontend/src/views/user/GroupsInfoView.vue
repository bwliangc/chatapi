<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <div class="flex flex-col justify-between gap-3 lg:flex-row lg:items-center">
            <div class="relative w-full sm:w-80">
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
              />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('groupsInfo.searchPlaceholder')"
                class="input pl-10"
              />
            </div>
            <button
              @click="loadGroups"
              :disabled="loading"
              class="btn btn-secondary flex-shrink-0"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>

          <!-- Platform filter chips -->
          <div v-if="availablePlatforms.length > 1" class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              @click="platformFilter = ''"
              :class="[
                'inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors',
                platformFilter === ''
                  ? 'border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                  : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
              ]"
            >
              {{ t('groupsInfo.allPlatforms') }}
            </button>
            <button
              v-for="p in availablePlatforms"
              :key="p"
              type="button"
              @click="platformFilter = p"
              :class="[
                'inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors',
                platformFilter === p
                  ? 'border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                  : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
              ]"
            >
              <PlatformIcon :platform="p as GroupPlatform" size="sm" />
              {{ platformLabel(p) }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <div v-if="loading" class="flex items-center justify-center py-20 text-gray-400">
          <Icon name="refresh" size="lg" class="animate-spin" />
        </div>

        <EmptyState
          v-else-if="filteredGroups.length === 0"
          :title="t('groupsInfo.empty')"
          :description="t('groupsInfo.emptyDescription')"
        />

        <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          <div
            v-for="group in filteredGroups"
            :key="group.id"
            class="flex flex-col overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm transition-shadow hover:shadow-md dark:border-dark-700 dark:bg-dark-800"
          >
            <!-- accent bar -->
            <div class="h-1 w-full" :class="platformAccentBarClass(group.platform)" />

            <!-- clickable header toggles model expansion -->
            <button
              type="button"
              @click="toggleExpanded(group.id)"
              class="flex w-full flex-col gap-3 px-4 py-4 text-left transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/50"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="flex min-w-0 items-center gap-2">
                  <PlatformIcon :platform="group.platform as GroupPlatform" size="md" />
                  <span class="truncate font-semibold text-gray-900 dark:text-white">{{
                    group.name
                  }}</span>
                </div>
                <div class="flex flex-shrink-0 items-center gap-1">
                  <span
                    v-if="group.subscription_type === 'subscription'"
                    class="rounded bg-violet-100 px-1.5 py-0.5 text-[10px] font-semibold text-violet-700 dark:bg-violet-900/30 dark:text-violet-300"
                  >
                    {{ t('groups.subscription') }}
                  </span>
                  <span
                    v-if="group.is_exclusive"
                    class="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                  >
                    {{ t('groupsInfo.exclusive') }}
                  </span>
                </div>
              </div>

              <p
                v-if="group.description"
                class="line-clamp-2 text-xs text-gray-500 dark:text-gray-400"
              >
                {{ group.description }}
              </p>

              <div class="flex items-center justify-between">
                <!-- rate -->
                <div class="flex items-center gap-1.5 text-sm">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('groupsInfo.rate') }}</span>
                  <template v-if="hasCustomRate(group)">
                    <span class="text-xs text-gray-400 line-through"
                      >{{ group.rate_multiplier }}x</span
                    >
                    <span class="font-semibold text-primary-600 dark:text-primary-400"
                      >{{ userGroupRates[group.id] }}x</span
                    >
                  </template>
                  <span v-else class="font-semibold text-gray-900 dark:text-white"
                    >{{ group.rate_multiplier }}x</span
                  >
                </div>

                <!-- model count + chevron -->
                <div class="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
                  <span>{{ t('groupsInfo.modelCount', { count: group.models.length }) }}</span>
                  <svg
                    class="h-4 w-4 transition-transform"
                    :class="expanded.has(group.id) ? 'rotate-180' : ''"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    stroke-width="2"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </div>
              </div>
            </button>

            <!-- expanded models -->
            <div
              v-if="expanded.has(group.id)"
              class="border-t border-gray-100 px-4 py-3 dark:border-dark-700"
            >
              <div v-if="group.models.length === 0" class="text-xs text-gray-400">
                {{ t('groupsInfo.noModels') }}
              </div>
              <div v-else class="flex flex-wrap gap-1.5">
                <SupportedModelChip
                  v-for="model in group.models"
                  :key="model.name"
                  :model="model"
                  :show-platform="false"
                  pricing-key-prefix="availableChannels.pricing"
                  :no-pricing-label="t('availableChannels.noPricing')"
                  :platform-hint="group.platform"
                />
              </div>
            </div>
          </div>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import SupportedModelChip from '@/components/channels/SupportedModelChip.vue'
import userGroupsAPI, { type GroupInfo } from '@/api/groups'
import type { GroupPlatform } from '@/types'
import { platformAccentBarClass, platformLabel } from '@/utils/platformColors'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<GroupInfo[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const platformFilter = ref<string>('')
const expanded = ref<Set<number>>(new Set())

const availablePlatforms = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const g of groups.value) {
    if (g.platform && !seen.has(g.platform)) {
      seen.add(g.platform)
      out.push(g.platform)
    }
  }
  return out
})

const filteredGroups = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return groups.value.filter((g) => {
    if (platformFilter.value && g.platform !== platformFilter.value) return false
    if (!q) return true
    return (
      g.name.toLowerCase().includes(q) ||
      (g.description || '').toLowerCase().includes(q) ||
      g.models.some((m) => m.name.toLowerCase().includes(q))
    )
  })
})

function hasCustomRate(group: GroupInfo): boolean {
  const rate = userGroupRates.value[group.id]
  return rate !== undefined && rate !== null && rate !== group.rate_multiplier
}

function toggleExpanded(id: number) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

async function loadGroups() {
  loading.value = true
  try {
    // 分组信息与用户专属倍率并发拉取。专属倍率失败不阻塞展示——降级为仅默认倍率。
    const [list, rates] = await Promise.all([
      userGroupsAPI.getAvailableGroupsInfo(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        return {} as Record<number, number>
      })
    ])
    groups.value = list
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadGroups)
</script>
