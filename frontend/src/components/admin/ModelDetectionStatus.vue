<template>
  <div class="max-w-56 text-xs" :title="snapshot?.reason">
    <div class="flex items-center gap-1.5">
      <span class="inline-flex rounded-full px-2 py-1 font-medium" :class="tone">{{ t(`admin.modelDetection.statuses.${status}`) }}</span>
      <button v-if="showHistory" type="button" class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700 dark:hover:text-primary-400" :title="t('admin.modelDetection.history.open')" :aria-label="t('admin.modelDetection.history.open')" @click.stop="emit('history')">
        <Icon name="clock" size="sm" aria-hidden="true" />
      </button>
    </div>
    <template v-if="snapshot && !compact">
      <p class="mt-1 truncate text-gray-600 dark:text-gray-400" :title="snapshot.model">{{ snapshot.model }}</p>
      <p class="mt-0.5 text-gray-400">{{ new Date(snapshot.checked_at).toLocaleString() }}</p>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { DetectionSnapshot } from '@/api/admin/modelDetection'
const props = defineProps<{ snapshot?: DetectionSnapshot | null; showHistory?: boolean; compact?: boolean }>()
const emit = defineEmits<{ history: [] }>()
const { t } = useI18n()
const known = ['consistent', 'suspected_mismatch', 'inconclusive', 'unsupported', 'insufficient', 'error', 'cancelled']
const status = computed(() => known.includes(props.snapshot?.status ?? '') ? props.snapshot!.status : 'untested')
const tone = computed(() => status.value === 'consistent'
  ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  : status.value === 'suspected_mismatch'
    ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    : status.value === 'untested'
      ? 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
      : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400')
</script>
