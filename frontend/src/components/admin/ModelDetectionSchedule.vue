<template>
  <section class="card space-y-5 p-5 sm:p-6" aria-labelledby="detection-schedule-title">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h3 id="detection-schedule-title" class="font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.modelDetection.schedule.title') }}</h3>
      <label class="flex items-center gap-2 text-sm"><input v-model="config.enabled" type="checkbox" :disabled="loading || saving">{{ t('admin.modelDetection.schedule.enabled') }}</label>
    </div>
    <p class="text-sm text-gray-500">{{ t('admin.modelDetection.schedule.description') }}</p>
    <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    <p v-if="saved" role="status" class="text-sm text-green-600">{{ t('admin.modelDetection.schedule.saved') }}</p>
    <fieldset :disabled="loading || saving" class="space-y-4">
      <div class="grid gap-4 sm:grid-cols-3">
        <label class="space-y-2 text-sm"><span>{{ t('admin.modelDetection.platform') }}</span>
          <select v-model="platform" class="input w-full" @change="resetSearch"><option value="openai">OpenAI</option><option value="anthropic">Anthropic</option></select>
        </label>
        <label class="space-y-2 text-sm sm:col-span-2"><span>{{ t('admin.modelDetection.schedule.accounts') }}</span>
          <input v-model="query" class="input w-full" :placeholder="t('admin.modelDetection.selectAccount')" @input="search">
        </label>
      </div>
      <div class="max-h-56 overflow-auto rounded-xl border border-gray-200 p-3 dark:border-dark-600" :aria-busy="searching">
        <p v-if="searching" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
        <p v-else-if="!accounts.length" class="text-sm text-gray-500">{{ t('admin.modelDetection.noAccounts') }}</p>
        <label v-for="account in accounts" :key="account.id" class="flex cursor-pointer items-center gap-3 rounded-lg p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
          <input type="checkbox" :checked="selected(account.id)" :disabled="!selected(account.id) && config.targets.length >= 100" @change="toggleAccount(account)">
          <span class="min-w-0 break-all text-sm">{{ account.name }} <span class="text-gray-400">#{{ account.id }}</span></span>
        </label>
      </div>
      <div class="flex items-center justify-between gap-3 text-xs">
        <span>{{ t('admin.modelDetection.schedule.selected', { count: config.targets.length }) }}</span>
        <div class="flex gap-3">
          <button type="button" class="text-primary-600 disabled:opacity-40" :disabled="page <= 1 || searching" @click="changePage(-1)">{{ t('common.back') }}</button>
          <span>{{ page }} / {{ Math.max(1, Math.ceil(total / 50)) }}</span>
          <button type="button" class="text-primary-600 disabled:opacity-40" :disabled="page * 50 >= total || searching" @click="changePage(1)">{{ t('common.next') }}</button>
        </div>
      </div>
      <div class="grid items-end gap-3 sm:grid-cols-[1fr_auto_1fr]">
        <label class="space-y-2 text-sm"><span>{{ t('admin.modelDetection.schedule.batchModel') }}</span>
          <select v-model="batchModel" class="input w-full"><option value="">{{ t('admin.modelDetection.selectModel') }}</option><option v-for="model in info.models" :key="model" :value="model">{{ model }}</option></select>
        </label>
        <button type="button" class="btn btn-secondary" :disabled="!batchModel || !config.targets.length" @click="applyModel">{{ t('admin.modelDetection.schedule.applyModel') }}</button>
        <label class="space-y-2 text-sm"><span>{{ t('admin.modelDetection.schedule.interval') }}</span><input v-model.number="config.interval_minutes" type="number" min="15" max="10080" step="1" class="input w-full"></label>
      </div>
      <div v-if="config.targets.length" class="max-h-96 space-y-3 overflow-auto">
        <div v-for="target in config.targets" :key="target.account_id" class="flex flex-wrap items-center gap-3 rounded-xl bg-gray-50 p-3 dark:bg-dark-800">
          <div class="min-w-32 flex-1 text-sm"><p>{{ names[target.account_id] || (target.account_name ? `${target.account_name} · #${target.account_id}` : `#${target.account_id}`) }}</p><p v-if="target.next_run_at && config.enabled" class="mt-1 text-xs text-gray-500">{{ t('admin.modelDetection.schedule.nextRun') }} {{ formatTime(target.next_run_at) }}</p></div>
          <select v-model="target.model" class="input min-w-48 flex-1" :aria-label="`${t('admin.modelDetection.model')} #${target.account_id}`" @focus="loadModels(target.account_id)">
            <option value="">{{ t('admin.modelDetection.selectModel') }}</option><option v-for="model in modelOptions(target)" :key="model" :value="model">{{ model }}</option>
          </select>
          <button type="button" class="btn btn-secondary" :aria-label="`${t('common.remove')} #${target.account_id}`" @click="remove(target.account_id)">{{ t('common.remove') }}</button>
        </div>
      </div>
      <button class="btn btn-primary" :disabled="!valid || loading || saving" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</button>
    </fieldset>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import * as accountsAPI from '@/api/admin/accounts'
import { getDetectionSchedule, saveDetectionSchedule, type DetectionInfo, type DetectionSchedule, type DetectionTarget } from '@/api/admin/modelDetection'
import type { AccountListItem } from '@/types'
defineProps<{ info: DetectionInfo }>()
const { t } = useI18n()
const config = ref<DetectionSchedule>({ enabled: false, interval_minutes: 360, targets: [] })
const loading = ref(true), saving = ref(false), searching = ref(false), saved = ref(false), error = ref('')
const platform = ref('openai'), query = ref(''), page = ref(1), total = ref(0), batchModel = ref('')
const accounts = ref<AccountListItem[]>([]), names = ref<Record<number, string>>({}), models = ref<Record<number, string[]>>({})
let timer: ReturnType<typeof setTimeout> | undefined
let searchController: AbortController | undefined
let request = 0, disposed = false
const loadingModels = new Set<number>()
const selected = (id: number) => config.value.targets.some(target => target.account_id === id)
const valid = computed(() => Number.isInteger(config.value.interval_minutes) && config.value.interval_minutes >= 15 && config.value.interval_minutes <= 10080 && (!config.value.enabled || config.value.targets.length > 0) && config.value.targets.every(target => target.model.trim()))
const message = (e: unknown) => e instanceof Error ? e.message : t('common.error')
const formatTime = (time: string) => new Date(time).toLocaleString()
async function loadAccounts() {
  searchController?.abort(); searchController = new AbortController()
  const sequence = ++request
  searching.value = true
  try {
    const data = await accountsAPI.list(page.value, 50, { platform: platform.value, search: query.value.trim(), status: 'active', lite: 'true' }, { signal: searchController.signal })
    if (disposed || sequence !== request) return
    accounts.value = data.items.filter(a => (a.type === 'oauth' || a.type === 'apikey') && !a.parent_account_id && !a.extra?.synthetic_ui_test)
    total.value = data.total
    for (const account of accounts.value) names.value[account.id] = `${account.name} · #${account.id}`
  } catch (e) { if (!disposed && sequence === request) error.value = message(e) }
  finally { if (sequence === request) searching.value = false }
}
function search() {
  clearTimeout(timer); searchController?.abort(); ++request
  accounts.value = []; searching.value = true; page.value = 1
  timer = setTimeout(() => { void loadAccounts() }, 300)
}
function resetSearch() { query.value = ''; search() }
function changePage(delta: number) { page.value += delta; void loadAccounts() }
function remove(id: number) { config.value.targets = config.value.targets.filter(t => t.account_id !== id); saved.value = false }
function toggleAccount(account: AccountListItem) {
  if (selected(account.id)) { remove(account.id); return }
  if (config.value.targets.length >= 100) return
  config.value.targets.push({ account_id: account.id, model: batchModel.value })
  saved.value = false
  void loadModels(account.id)
}
async function loadModels(id: number) {
  if (models.value[id] || loadingModels.has(id)) return
  loadingModels.add(id)
  try {
    const available = await accountsAPI.getAvailableModels(id)
    if (!disposed) models.value[id] = available.map(m => m.id).filter(id => !/image|audio|tts|whisper|embedding|realtime|video|\*/i.test(id))
  } catch (e) { if (!disposed) error.value = message(e) }
  finally { loadingModels.delete(id) }
}
function modelOptions(target: DetectionTarget) { return [...new Set([target.model, ...(models.value[target.account_id] || [])])].filter(Boolean) }
function applyModel() { config.value.targets.forEach(target => { target.model = batchModel.value }); saved.value = false }
async function save() {
  if (!valid.value || saving.value) return
  saving.value = true; saved.value = false; error.value = ''
  try { config.value = await saveDetectionSchedule(config.value); saved.value = true }
  catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
onMounted(async () => {
  void loadAccounts()
  try { config.value = await getDetectionSchedule(); loading.value = false }
  catch (e) { error.value = message(e) }
})
onBeforeUnmount(() => { disposed = true; clearTimeout(timer); searchController?.abort(); ++request })
</script>
