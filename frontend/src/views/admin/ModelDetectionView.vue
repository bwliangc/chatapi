<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="border-b border-gray-200 pb-5 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.modelDetection.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.description') }}</p>
      </header>
      <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
        {{ t('admin.modelDetection.notice') }}
      </div>
      <section class="card space-y-5 p-6">
        <div class="grid gap-4 md:grid-cols-2">
          <label class="space-y-2 text-sm"><span>{{ t('admin.modelDetection.platform') }}</span>
            <select v-model="platform" class="input w-full" :disabled="running" @change="resetSearch">
              <option value="openai">OpenAI</option><option value="anthropic">Anthropic</option>
            </select>
          </label>
          <label class="space-y-2 text-sm"><span>{{ t('admin.modelDetection.search') }}</span>
            <div class="flex gap-2"><input v-model="search" class="input w-full" :disabled="running" @keyup.enter="resetSearch"><button class="btn btn-secondary shrink-0" :disabled="running || accountsLoading" @click="resetSearch">{{ t('common.search') }}</button></div>
          </label>
          <label class="space-y-2 text-sm"><span>{{ t('admin.modelDetection.account') }}</span>
            <select v-model="accountID" class="input w-full" :disabled="running || accountsLoading" @change="loadModels">
              <option value="">{{ t('admin.modelDetection.selectAccount') }}</option>
              <option v-for="account in accounts" :key="account.id" :value="String(account.id)">{{ account.name }} · #{{ account.id }} · {{ account.type }}</option>
            </select>
          </label>
          <label class="space-y-2 text-sm"><span>{{ t('admin.modelDetection.model') }}</span>
            <select v-model="model" class="input w-full" :disabled="running || modelsLoading || !accountID">
              <option value="">{{ modelsLoading ? t('common.loading') : t('admin.modelDetection.selectModel') }}</option>
              <option v-for="item in models" :key="item.id" :value="item.id">{{ item.id }}</option>
            </select>
          </label>
        </div>
        <div class="flex items-center gap-3 text-xs text-gray-500">
          <button class="btn btn-secondary" :disabled="running || accountsLoading || page <= 1" @click="changePage(-1)">←</button>
          <span>{{ t('admin.modelDetection.page', { page, total }) }}</span>
          <button class="btn btn-secondary" :disabled="running || accountsLoading || page * 50 >= total" @click="changePage(1)">→</button>
        </div>
        <p v-if="!accountsLoading && accounts.length === 0" class="text-sm text-gray-500">{{ t('admin.modelDetection.noAccounts') }}</p>
        <p v-if="model && info && !info.models.includes(model)" class="text-sm text-amber-600 dark:text-amber-400">{{ t('admin.modelDetection.unlisted') }}</p>
        <p class="text-xs text-gray-500">{{ t('admin.modelDetection.cost') }}</p>
        <div class="flex gap-3">
          <button class="btn btn-primary" :disabled="running || !accountID || !model || !info || modelsLoading" @click="run">{{ running ? t('admin.modelDetection.running') : t('admin.modelDetection.start') }}</button>
          <button v-if="running" class="btn btn-secondary" @click="cancel">{{ t('common.cancel') }}</button>
        </div>
        <div v-if="attempt > 0" class="space-y-2" role="status" aria-live="polite">
          <div class="h-2 overflow-hidden rounded bg-gray-100 dark:bg-dark-700"><div class="h-full bg-primary-500 transition-all" :style="{ width: `${accepted / 3 * 100}%` }" /></div>
          <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.modelDetection.progress', { attempt, accepted, active }) }} {{ progressMessage }}</p>
        </div>
      </section>
      <p v-if="error" class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300" role="alert">{{ error }}</p>
      <section v-if="result" class="card space-y-5 p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h3 class="font-semibold">{{ t('admin.modelDetection.result') }}</h3>
          <span class="rounded bg-gray-100 px-3 py-1 text-sm dark:bg-dark-700">{{ t(`admin.modelDetection.verdicts.${result.verdict.status}`) }}</span>
        </div>
        <p class="text-sm text-gray-500">{{ result.verdict.reason }}</p>
        <dl class="grid gap-4 text-sm md:grid-cols-3">
          <div><dt class="text-gray-500">{{ t('admin.modelDetection.requested') }}</dt><dd class="mt-1 break-all font-medium">{{ result.model }}</dd></div>
          <div><dt class="text-gray-500">{{ t('admin.modelDetection.mapped') }}</dt><dd class="mt-1 break-all font-medium">{{ result.mapped_model }}</dd></div>
          <div><dt class="text-gray-500">{{ t('admin.modelDetection.reported') }}</dt><dd class="mt-1 break-all font-medium">{{ [...new Set(result.actual_models.filter(Boolean))].join(', ') || '—' }}</dd></div>
        </dl>
        <div v-if="result.result" class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead><tr class="border-b dark:border-dark-700"><th class="py-3">{{ t('admin.modelDetection.candidate') }}</th><th class="py-3 text-right">{{ t('admin.modelDetection.probability') }}</th></tr></thead>
            <tbody><tr v-for="candidate in result.result.results.slice(0, 5)" :key="candidate.model" class="border-b dark:border-dark-700"><td class="py-3">{{ candidate.model }}</td><td class="py-3 text-right font-mono">{{ (candidate.probability * 100).toFixed(2) }}%</td></tr></tbody>
          </table>
        </div>
      </section>
      <p v-if="info" class="break-all text-xs text-gray-400">{{ t('admin.modelDetection.bank', { count: result?.result?.results.length ?? info.models.length, date: result?.result?.bank_built_at ?? info.bank_built_at }) }} · SHA-256 {{ result?.result?.bank_sha256 ?? info.bank_sha256 }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import * as accountsAPI from '@/api/admin/accounts'
import { detectAccountModel, getDetectionInfo, type DetectionInfo, type DetectionResult } from '@/api/admin/modelDetection'
import type { AccountListItem, ClaudeModel } from '@/types'

const { t } = useI18n()
const platform = ref('openai'), search = ref(''), accountID = ref(''), model = ref('')
const page = ref(1), total = ref(0)
const accounts = ref<AccountListItem[]>([]), models = ref<ClaudeModel[]>([])
const accountsLoading = ref(false), modelsLoading = ref(false), running = ref(false)
const info = ref<DetectionInfo | null>(null), result = ref<DetectionResult | null>(null)
const attempt = ref(0), accepted = ref(0), active = ref(0), error = ref(''), progressMessage = ref('')
let controller: AbortController | undefined
let accountRequest = 0, modelRequest = 0
let disposed = false
const errorText = (e: unknown) => e instanceof Error ? e.message : t('common.error')
async function loadAccounts() {
  const request = ++accountRequest
  accountsLoading.value = true
  accountID.value = ''; accounts.value = []; models.value = []; model.value = ''; modelsLoading.value = false; ++modelRequest
  try {
    const data = await accountsAPI.list(page.value, 50, { platform: platform.value, status: 'active', search: search.value, lite: 'true' })
    if (disposed || request !== accountRequest) return
    accounts.value = data.items.filter(a => a.type === 'oauth' || a.type === 'apikey')
    total.value = data.total
  } catch (e) { if (request === accountRequest) error.value = errorText(e) }
  finally { if (request === accountRequest) accountsLoading.value = false }
}
function resetSearch() { page.value = 1; error.value = ''; void loadAccounts() }
function changePage(delta: number) { page.value += delta; void loadAccounts() }
async function loadModels() {
  const request = ++modelRequest
  model.value = ''; models.value = []; error.value = ''
  if (!accountID.value) { modelsLoading.value = false; return }
  modelsLoading.value = true
  try {
    const data = await accountsAPI.getAvailableModels(Number(accountID.value))
    if (disposed || request !== modelRequest) return
    models.value = data.filter(m => !/image|audio|tts|whisper|embedding|realtime|video|\*/i.test(m.id))
  } catch (e) { if (request === modelRequest) error.value = errorText(e) }
  finally { if (request === modelRequest) modelsLoading.value = false }
}
async function run() {
  if (running.value || !accountID.value || !model.value) return
  running.value = true; error.value = ''; result.value = null; attempt.value = 0; accepted.value = 0; active.value = 0; progressMessage.value = ''
  controller = new AbortController()
  try {
    await detectAccountModel(Number(accountID.value), model.value, controller.signal, event => {
      if (event.type === 'progress') { attempt.value = event.attempt; accepted.value = event.accepted; active.value = event.in_flight ?? 0; progressMessage.value = event.message || '' }
      if (event.type === 'result') result.value = event.data
    })
  } catch (e) { error.value = controller.signal.aborted ? t('admin.modelDetection.cancelled') : errorText(e) }
  finally { running.value = false; active.value = 0; controller = undefined }
}
function cancel() { controller?.abort() }
onMounted(async () => {
  await Promise.all([loadAccounts(), getDetectionInfo().then(data => { info.value = data }).catch(e => { error.value = errorText(e) })])
})
onBeforeUnmount(() => { disposed = true; ++accountRequest; ++modelRequest; cancel() })
</script>
