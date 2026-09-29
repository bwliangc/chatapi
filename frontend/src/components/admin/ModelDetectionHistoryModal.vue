<template>
  <BaseDialog :show="show" :title="t('admin.modelDetection.history.title')" width="extra-wide" @close="emit('close')">
    <div class="space-y-4">
      <p v-if="account" class="break-all text-sm font-medium text-gray-800 dark:text-gray-200">{{ account.name }} <span class="text-gray-400">#{{ account.id }}</span></p>
      <div v-if="loading" role="status" class="py-12 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="error" role="alert" class="space-y-3 rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
        <p>{{ error }}</p>
        <button type="button" class="btn btn-secondary" @click="load">{{ t('admin.modelDetection.history.retry') }}</button>
      </div>
      <p v-else-if="!records.length" class="py-12 text-center text-sm text-gray-500">{{ t('admin.modelDetection.history.empty') }}</p>
      <div v-else class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
        <table class="w-full text-left text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="whitespace-nowrap px-4 py-3">{{ t('admin.modelDetection.history.time') }}</th>
              <th class="px-4 py-3">{{ t('admin.modelDetection.model') }}</th>
              <th class="px-4 py-3">{{ t('common.status') }}</th>
              <th class="px-4 py-3">{{ t('admin.modelDetection.history.source') }}</th>
              <th class="px-4 py-3">{{ t('admin.modelDetection.history.details') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="record in records" :key="record.id">
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500">{{ new Date(record.checked_at).toLocaleString() }}</td>
              <td class="max-w-60 break-all px-4 py-3 font-medium text-gray-800 dark:text-gray-200">{{ record.model }}</td>
              <td class="whitespace-nowrap px-4 py-3"><ModelDetectionStatus :snapshot="record" compact /></td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500">{{ t(`admin.modelDetection.history.sources.${record.source}`) }}</td>
              <td class="min-w-56 px-4 py-3 text-xs leading-5 text-gray-600 dark:text-gray-400">
                <p class="break-words">{{ record.reason }}</p>
                <p v-if="record.prediction" class="mt-1 break-all">{{ t('admin.modelDetection.candidate') }}: {{ record.prediction }}</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="text-xs leading-5 text-gray-500">{{ t('admin.modelDetection.notice') }}</p>
    </div>
    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-between gap-3">
        <span class="text-xs text-gray-500">{{ t('admin.modelDetection.history.pagination', { page, pages, total }) }}</span>
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading || page <= 1" @click="changePage(-1)">{{ t('admin.modelDetection.history.previous') }}</button>
          <button type="button" class="btn btn-secondary" :disabled="loading || page >= pages" @click="changePage(1)">{{ t('common.next') }}</button>
          <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ModelDetectionStatus from './ModelDetectionStatus.vue'
import { getDetectionHistory, type DetectionHistoryRecord } from '@/api/admin/modelDetection'
const props = defineProps<{ show: boolean; account: { id: number; name: string } | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const records = ref<DetectionHistoryRecord[]>([])
const loading = ref(false), error = ref(''), page = ref(1), pages = ref(1), total = ref(0)
let controller: AbortController | undefined
let sequence = 0
function cancel() { controller?.abort(); ++sequence }
async function load() {
  cancel()
  if (!props.show || !props.account) return
  controller = new AbortController()
  const request = sequence
  loading.value = true; error.value = ''; records.value = []
  try {
    const data = await getDetectionHistory(props.account.id, page.value, 20, controller.signal)
    if (request !== sequence) return
    records.value = data.items; total.value = data.total; pages.value = data.pages
  } catch (e) {
    if (request === sequence) error.value = e instanceof Error ? e.message : t('common.error')
  } finally { if (request === sequence) loading.value = false }
}
function changePage(delta: number) { page.value += delta; void load() }
watch(() => [props.show, props.account?.id], () => {
  cancel(); records.value = []; error.value = ''; page.value = 1; pages.value = 1; total.value = 0; loading.value = false
  if (props.show && props.account) void load()
}, { immediate: true })
onBeforeUnmount(cancel)
</script>
