<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5 sm:space-y-6">
      <header class="flex items-start gap-3 sm:gap-4">
        <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
          <Icon name="chart" size="lg" aria-hidden="true" />
        </div>
        <div class="min-w-0">
          <h2 class="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{{ t('admin.modelDetection.title') }}</h2>
          <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.description') }}</p>
        </div>
      </header>

      <div class="grid items-start gap-5 xl:grid-cols-[minmax(300px,360px)_minmax(0,1fr)] xl:gap-6">
        <section class="card min-w-0 overflow-hidden" aria-labelledby="detection-configuration">
          <div class="flex items-center gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
            <Icon name="cog" size="sm" class="text-gray-400" aria-hidden="true" />
            <h3 id="detection-configuration" class="font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.modelDetection.configuration') }}</h3>
          </div>
          <div class="space-y-5 p-5 sm:p-6">
            <label class="block space-y-2 text-sm font-medium text-gray-700 dark:text-gray-300">
              <span>{{ t('admin.modelDetection.platform') }}</span>
              <select v-model="platform" class="input w-full" :disabled="running" @change="resetSearch">
                <option value="openai">OpenAI</option><option value="anthropic">Anthropic</option>
              </select>
            </label>
            <div class="space-y-2">
              <label for="detection-search" class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.modelDetection.search') }}</label>
              <div class="flex gap-2">
                <input id="detection-search" v-model="search" class="input min-w-0 flex-1" :disabled="running" @keyup.enter="resetSearch">
                <button class="btn btn-secondary shrink-0 px-3" :disabled="running || accountsLoading" :aria-label="t('common.search')" @click="resetSearch"><Icon name="search" size="sm" aria-hidden="true" /></button>
              </div>
            </div>
            <div class="space-y-2">
              <label class="block space-y-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                <span>{{ t('admin.modelDetection.account') }}</span>
                <select v-model="accountID" class="input w-full" :disabled="running || accountsLoading" @change="loadModels">
                  <option value="">{{ t('admin.modelDetection.selectAccount') }}</option>
                  <option v-for="account in accounts" :key="account.id" :value="String(account.id)">{{ account.name }} · #{{ account.id }} · {{ account.type }}</option>
                </select>
              </label>
              <div class="flex items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ t('admin.modelDetection.page', { page, total }) }}</span>
                <div class="flex gap-1">
                  <button class="btn btn-secondary !rounded-lg !p-1.5" :disabled="running || accountsLoading || page <= 1" :aria-label="t('admin.modelDetection.previousPage')" @click="changePage(-1)"><Icon name="chevronLeft" size="sm" aria-hidden="true" /></button>
                  <button class="btn btn-secondary !rounded-lg !p-1.5" :disabled="running || accountsLoading || page * 50 >= total" :aria-label="t('admin.modelDetection.nextPage')" @click="changePage(1)"><Icon name="chevronRight" size="sm" aria-hidden="true" /></button>
                </div>
              </div>
            </div>
            <p v-if="!accountsLoading && accounts.length === 0" class="text-sm text-gray-500">{{ t('admin.modelDetection.noAccounts') }}</p>
            <label class="block space-y-2 text-sm font-medium text-gray-700 dark:text-gray-300">
              <span>{{ t('admin.modelDetection.model') }}</span>
              <select v-model="model" class="input w-full" :disabled="running || modelsLoading || !accountID">
                <option value="">{{ modelsLoading ? t('common.loading') : t('admin.modelDetection.selectModel') }}</option>
                <option v-for="item in models" :key="item.id" :value="item.id">{{ item.id }}</option>
              </select>
            </label>
            <p v-if="model && info && !info.models.includes(model)" class="rounded-xl bg-amber-50 p-3 text-xs leading-5 text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ t('admin.modelDetection.unlisted') }}</p>
            <div class="space-y-3 border-t border-gray-100 pt-5 dark:border-dark-700">
              <button class="btn btn-primary w-full" :disabled="running || !accountID || !model || !info || modelsLoading" @click="run">
                <Icon :name="running ? 'refresh' : 'play'" size="sm" :class="{ 'animate-spin motion-reduce:animate-none': running }" aria-hidden="true" />
                {{ running ? t('admin.modelDetection.running') : t('admin.modelDetection.start') }}
              </button>
              <button v-if="running" class="btn btn-secondary w-full" @click="cancel">{{ t('common.cancel') }}</button>
              <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.cost') }}</p>
            </div>
            <div v-if="attempt > 0" class="space-y-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-800" role="status" aria-live="polite">
              <div class="flex items-center justify-between gap-3 text-sm">
                <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.modelDetection.samplingProgress') }}</span>
                <span class="font-mono font-semibold text-primary-600 dark:text-primary-400">{{ accepted }}<span class="text-gray-400"> / 3</span></span>
              </div>
              <div class="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600" role="progressbar" :aria-label="t('admin.modelDetection.samplingProgress')" :aria-valuenow="accepted" :aria-valuemin="0" :aria-valuemax="3">
                <div class="h-full rounded-full bg-primary-500 transition-[width] motion-reduce:transition-none" :style="{ width: `${accepted / 3 * 100}%` }" />
              </div>
              <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.progress', { attempt, accepted, active }) }}</p>
              <p v-if="progressMessage" class="text-xs text-amber-700 dark:text-amber-400">{{ progressMessage }}</p>
            </div>
          </div>
        </section>

        <div class="min-w-0 space-y-5">
          <div v-if="error" class="flex items-start gap-3 rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300" role="alert">
            <Icon name="exclamationCircle" class="shrink-0" aria-hidden="true" /><p class="min-w-0 break-words leading-6">{{ error }}</p>
          </div>
          <section v-if="result" class="card min-w-0 overflow-hidden" aria-labelledby="detection-result" data-testid="detection-result" :data-verdict="result.verdict.status">
            <div class="result-summary border-b p-5 sm:p-6" :class="`result-${verdictAppearance.tone}`">
              <div class="flex items-center justify-between gap-3">
                <h3 id="detection-result" class="text-sm font-medium opacity-80">{{ t('admin.modelDetection.result') }}</h3>
                <span class="rounded-full border border-black/10 dark:border-white/10 px-2.5 py-1 font-mono text-xs">#{{ result.account_id }}</span>
              </div>
              <div class="mt-5 flex items-start gap-3 sm:gap-4">
                <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-white/70 dark:bg-white/5"><Icon :name="verdictAppearance.icon" size="xl" aria-hidden="true" /></div>
                <div class="min-w-0">
                  <p class="text-2xl font-semibold tracking-tight sm:text-3xl">{{ t(`admin.modelDetection.verdicts.${result.verdict.status}`) }}</p>
                  <p class="mt-2 text-sm leading-6 opacity-90">{{ result.verdict.reason }}</p>
                </div>
              </div>
            </div>
            <div class="space-y-6 p-5 sm:p-6">
              <dl class="grid gap-3 sm:grid-cols-2">
                <div class="rounded-xl border border-gray-100 bg-gray-50/80 p-4 dark:border-dark-700 dark:bg-dark-800/70"><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.requested') }}</dt><dd class="mt-2 break-all text-sm font-semibold text-gray-900 dark:text-gray-100">{{ result.model }}</dd></div>
                <div class="rounded-xl border border-gray-100 bg-gray-50/80 p-4 dark:border-dark-700 dark:bg-dark-800/70"><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.mapped') }}</dt><dd class="mt-2 break-all text-sm font-semibold text-gray-900 dark:text-gray-100">{{ result.mapped_model }}</dd></div>
                <div class="min-w-0 sm:col-span-2"><dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.reported') }}</dt><dd class="mt-1.5 break-all text-sm text-gray-700 dark:text-gray-300">{{ resultModels }}</dd></div>
              </dl>
              <div v-if="result.result" class="space-y-4 border-t border-gray-100 pt-5 dark:border-dark-700">
                <div class="flex flex-wrap items-baseline justify-between gap-2">
                  <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.modelDetection.candidateRanking') }}</h4>
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.probability') }}</span>
                </div>
                <ol class="space-y-4">
                  <li v-for="(candidate, index) in result.result.results.slice(0, 5)" :key="candidate.model" class="flex items-start gap-3">
                    <span class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-lg bg-gray-100 font-mono text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">{{ index + 1 }}</span>
                    <div class="min-w-0 flex-1">
                      <div class="mb-2 flex items-baseline justify-between gap-3 text-sm">
                        <span class="min-w-0 break-all font-medium text-gray-800 dark:text-gray-200">{{ candidate.model }}</span>
                        <span class="shrink-0 font-mono tabular-nums text-gray-700 dark:text-gray-300">{{ probabilityPercent(candidate.probability).toFixed(2) }}%</span>
                      </div>
                      <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700" aria-hidden="true"><div class="h-full rounded-full" :class="index === 0 ? `candidate-${verdictAppearance.tone}` : 'bg-gray-300 dark:bg-dark-500'" :style="{ width: `${probabilityPercent(candidate.probability)}%` }" /></div>
                    </div>
                  </li>
                </ol>
              </div>
            </div>
          </section>
          <section v-else class="card flex min-h-[360px] flex-col items-center justify-center px-6 py-12 text-center sm:min-h-[420px]">
            <div class="mb-5 flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-50 text-primary-500 ring-1 ring-gray-100 dark:bg-dark-800 dark:ring-dark-700"><Icon :name="running ? 'refresh' : 'chart'" size="xl" :class="{ 'animate-spin motion-reduce:animate-none': running }" aria-hidden="true" /></div>
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-100">{{ t(running ? 'admin.modelDetection.running' : 'admin.modelDetection.readyTitle') }}</h3>
            <p class="mt-2 max-w-sm text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t(running ? 'admin.modelDetection.runningDescription' : 'admin.modelDetection.readyDescription') }}</p>
          </section>
          <div class="flex items-start gap-2.5 px-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
            <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" aria-hidden="true" /><p>{{ t('admin.modelDetection.notice') }}</p>
          </div>
          <details v-if="info" class="rounded-xl border border-gray-200 px-4 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
            <summary class="cursor-pointer leading-5">{{ t('admin.modelDetection.bank', { count: result?.result?.results.length ?? info.models.length, date: result?.result?.bank_built_at ?? info.bank_built_at }) }}</summary>
            <p class="mt-3 break-all border-t border-gray-100 pt-3 font-mono leading-5 dark:border-dark-700">SHA-256 {{ result?.result?.bank_sha256 ?? info.bank_sha256 }}</p>
          </details>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import * as accountsAPI from '@/api/admin/accounts'
import { detectAccountModel, getDetectionInfo, type DetectionInfo, type DetectionResult } from '@/api/admin/modelDetection'
import type { AccountListItem, ClaudeModel } from '@/types'

const { t } = useI18n()
const platform = ref('openai'), search = ref(''), accountID = ref(''), model = ref('')
const page = ref(1), total = ref(0)
const accounts = ref<AccountListItem[]>([]), models = ref<ClaudeModel[]>([])
const accountsLoading = ref(false), modelsLoading = ref(false), running = ref(false)
const info = ref<DetectionInfo | null>(null), result = ref<DetectionResult | null>(null)
const verdictAppearance = computed(() => {
  switch (result.value?.verdict.status) {
    case 'consistent': return { tone: 'success', icon: 'checkCircle' as const }
    case 'suspected_mismatch': return { tone: 'danger', icon: 'xCircle' as const }
    case 'inconclusive':
    case 'insufficient': return { tone: 'warning', icon: 'exclamationCircle' as const }
    default: return { tone: 'neutral', icon: 'questionCircle' as const }
  }
})
const resultModels = computed(() => [...new Set(result.value?.actual_models.filter(Boolean) ?? [])].join(', ') || '—')
const probabilityPercent = (value: number) => Math.min(100, Math.max(0, value * 100))
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

<style scoped>
.result-success { @apply border-green-200 bg-green-50 text-green-800 dark:border-green-900/60 dark:bg-green-950/30 dark:text-green-300; }
.result-danger { @apply border-red-200 bg-red-50 text-red-800 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300; }
.result-warning { @apply border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300; }
.result-neutral { @apply border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300; }
.candidate-success { @apply bg-green-500 dark:bg-green-400; }
.candidate-danger { @apply bg-red-500 dark:bg-red-400; }
.candidate-warning { @apply bg-amber-500 dark:bg-amber-400; }
.candidate-neutral { @apply bg-gray-400 dark:bg-gray-500; }
</style>
