<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('luckySecond.title') }}</h2>
        <p class="mt-1 max-w-2xl text-sm text-gray-500 dark:text-gray-400">{{ t('luckySecond.description') }}</p>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-secondary" :disabled="loading" @click="refresh">{{ t('common.refresh') }}</button>
        <button v-if="admin" class="btn btn-primary" @click="creating = !creating">{{ t('luckySecond.create') }}</button>
      </div>
    </div>
    <div v-if="admin && !enabled" class="rounded-lg bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
      {{ t('luckySecond.disabledHint') }}
      <RouterLink class="ml-2 underline" to="/admin/settings">{{ t('luckySecond.openSettings') }}</RouterLink>
    </div>
    <form v-if="admin && creating" class="card space-y-4 p-5" @submit.prevent="create">
      <p class="text-sm text-gray-500">{{ t('luckySecond.createHint', { timezone }) }}</p>
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="sm:col-span-2"><span class="input-label">{{ t('luckySecond.name') }}</span><input v-model="form.name" class="input" maxlength="120" required /></label>
        <label><span class="input-label">{{ t('luckySecond.start') }}</span><input v-model="form.start" class="input" type="datetime-local" step="1" required /></label>
        <label><span class="input-label">{{ t('luckySecond.end') }}</span><input v-model="form.end" class="input" type="datetime-local" step="1" required /></label>
        <label><span class="input-label">{{ t('luckySecond.pool') }}</span><input v-model="form.amount" class="input" type="number" min="0.000001" max="100000000" step="0.000001" required /></label>
        <label><span class="input-label">{{ t('luckySecond.count') }}</span><input v-model.number="form.count" class="input" type="number" min="1" max="10000" step="1" required /></label>
      </div>
      <p class="text-xs text-gray-500">{{ t('luckySecond.immutableHint') }}</p>
      <div class="flex gap-2"><button class="btn btn-primary" :disabled="saving" type="submit">{{ saving ? t('common.loading') : t('luckySecond.create') }}</button><button class="btn btn-secondary" type="button" :disabled="saving" @click="creating = false">{{ t('common.cancel') }}</button></div>
    </form>
    <p class="rounded-lg bg-gray-50 p-4 text-xs leading-relaxed text-gray-500 dark:bg-dark-800 dark:text-gray-400">{{ t('luckySecond.rules') }}</p>
    <div v-if="error" role="alert" class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>
    <div v-if="loading && !campaigns.length" class="card p-8 text-center text-gray-500">{{ t('common.loading') }}</div>
    <div v-else-if="!campaigns.length && !error" class="card p-10 text-center text-gray-500">{{ t('luckySecond.empty') }}</div>
    <article v-for="campaign in campaigns" :key="campaign.id" class="card overflow-hidden">
      <div class="space-y-4 p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div class="flex flex-wrap items-center gap-2"><h3 class="font-semibold text-gray-900 dark:text-white">{{ campaign.name }}</h3><span class="rounded-full bg-primary-50 px-2.5 py-1 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ status(campaign) }}</span></div>
            <p class="mt-2 text-xs text-gray-500">{{ date(campaign.starts_at, campaign.timezone) }} → {{ date(campaign.ends_at, campaign.timezone) }} ({{ campaign.timezone }})</p>
          </div>
          <button v-if="admin && !campaign.cancelled_at && new Date(campaign.ends_at).getTime() > now" class="text-sm text-red-600 hover:underline" @click="confirmCancel = campaign.id">{{ t('luckySecond.stop') }}</button>
        </div>
        <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <div><div class="text-xs text-gray-500">{{ t('luckySecond.pool') }}</div><div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ money(campaign.total_amount) }}</div></div>
          <div><div class="text-xs text-gray-500">{{ t('luckySecond.awarded') }}</div><div class="mt-1 text-xl font-semibold text-amber-600 dark:text-amber-400">{{ money(campaign.awarded_amount) }}</div></div>
          <div><div class="text-xs text-gray-500">{{ t('luckySecond.wonCount') }}</div><div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ campaign.awarded_count }} <span class="text-sm font-normal text-gray-400">/ {{ campaign.reward_count }}</span></div></div>
          <div><div class="text-xs text-gray-500">{{ t('luckySecond.unissued') }}</div><div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ money(Number(campaign.total_amount) - Number(campaign.awarded_amount)) }}</div></div>
        </div>
        <div v-if="!admin" class="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-primary-50 px-4 py-3 dark:bg-primary-900/20">
          <div>
            <div class="text-sm font-medium text-primary-700 dark:text-primary-300">{{ t('luckySecond.myAwarded') }}</div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('luckySecond.myAwardedHint') }}</p>
          </div>
          <div class="text-xl font-semibold text-primary-700 dark:text-primary-300">{{ money(campaign.my_awarded_amount ?? '0') }}</div>
        </div>
        <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full bg-amber-400" :style="{ width: `${100 * campaign.awarded_count / campaign.reward_count}%` }" /></div>
        <div v-if="confirmCancel === campaign.id" class="space-y-2 rounded-lg bg-red-50 p-3 text-sm dark:bg-red-900/10"><p>{{ t('luckySecond.stopHint') }}</p><button class="btn btn-danger mr-2" :disabled="saving" @click="cancel(campaign.id)">{{ t('luckySecond.confirmStop') }}</button><button class="btn btn-secondary" @click="confirmCancel = null">{{ t('common.cancel') }}</button></div>
        <button class="text-sm font-medium text-primary-600 hover:underline dark:text-primary-400" :aria-expanded="selected === campaign.id" @click="toggleDetails(campaign.id)">{{ selected === campaign.id ? t('luckySecond.hideDetails') : admin ? t('luckySecond.schedule') : t('luckySecond.awards') }}</button>
      </div>
      <div v-if="selected === campaign.id" class="border-t border-gray-100 dark:border-dark-700">
        <div v-if="slotsLoading" class="p-6 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else-if="slotsError" role="alert" class="p-6 text-sm text-red-600">{{ slotsError }}</div>
        <template v-else>
          <p v-if="!slots.length" class="p-6 text-center text-sm text-gray-500">{{ t('luckySecond.noAwards') }}</p>
          <div v-else class="overflow-x-auto"><table class="w-full whitespace-nowrap text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-5 py-3">{{ t('luckySecond.second') }}</th><th class="px-5 py-3">{{ t('luckySecond.amount') }}</th><th class="px-5 py-3">{{ t('luckySecond.winner') }}</th><th v-if="admin" class="px-5 py-3">{{ t('luckySecond.state') }}</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="slot in slots" :key="slot.id"><td class="px-5 py-3">{{ date(slot.second_at, campaign.timezone) }}</td><td class="px-5 py-3 font-medium text-amber-600">{{ money(slot.amount) }}</td><td class="px-5 py-3">{{ slot.name || '—' }} <span v-if="slot.is_me" class="text-primary-600">{{ t('leaderboard.youLabel') }}</span></td><td v-if="admin" class="px-5 py-3">{{ t(`luckySecond.slot.${slot.state}`) }}</td></tr></tbody></table></div>
          <Pagination v-if="slotsTotal > 20" :page="slotsPage" :total="slotsTotal" :page-size="20" :show-page-size-selector="false" @update:page="loadSlots" />
        </template>
      </div>
    </article>
    <Pagination v-if="total > 10" :page="page" :total="total" :page-size="10" :show-page-size-selector="false" @update:page="load" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { luckySecondAPI, type LuckySecondCampaign, type LuckySecondSlot } from '@/api/luckySecond'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import Pagination from './Pagination.vue'

const props = withDefaults(defineProps<{ admin?: boolean }>(), { admin: false })
const { t } = useI18n()
const appStore = useAppStore()
const enabled = computed(() => appStore.cachedPublicSettings?.lucky_second_enabled === true)
const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
const campaigns = ref<LuckySecondCampaign[]>([])
const slots = ref<LuckySecondSlot[]>([])
const page = ref(1), total = ref(0), slotsPage = ref(1), slotsTotal = ref(0)
const selected = ref<number | null>(null), confirmCancel = ref<number | null>(null)
const creating = ref(false), loading = ref(false), slotsLoading = ref(false), saving = ref(false)
const error = ref(''), slotsError = ref(''), now = ref(Date.now())
const form = reactive({ name: '', start: '', end: '', amount: '100', count: 50 })
let listVersion = 0, slotsVersion = 0
let timer: ReturnType<typeof setInterval> | undefined
const money = (value: string | number) => '$' + Number(value).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })
const date = (value: string, zone: string) => new Date(value).toLocaleString(undefined, { timeZone: zone, hour12: false })
function status(c: LuckySecondCampaign) {
  if (c.cancelled_at) return t('luckySecond.cancelled')
  if (new Date(c.starts_at).getTime() > now.value) return t('luckySecond.upcoming')
  if (new Date(c.ends_at).getTime() > now.value) return t('luckySecond.active')
  return t(c.pending_count ? 'luckySecond.settling' : 'luckySecond.ended')
}
async function load(target = page.value) {
  const version = ++listVersion
  loading.value = true; error.value = ''
  try { const data = await luckySecondAPI.list(props.admin, target); if (version !== listVersion) return; campaigns.value = data.items; total.value = data.total; page.value = data.page }
  catch (err) { if (version === listVersion) error.value = extractApiErrorMessage(err, t('common.error')) }
  finally { if (version === listVersion) loading.value = false }
}
async function loadSlots(target = 1) {
  if (selected.value === null) return
  const version = ++slotsVersion
  slotsLoading.value = true; slotsError.value = ''
  try { const data = await luckySecondAPI.slots(props.admin, selected.value, target); if (version !== slotsVersion) return; slots.value = data.items; slotsTotal.value = data.total; slotsPage.value = data.page }
  catch (err) { if (version === slotsVersion) slotsError.value = extractApiErrorMessage(err, t('common.error')) }
  finally { if (version === slotsVersion) slotsLoading.value = false }
}
function toggleDetails(id: number) { slotsVersion++; selected.value = selected.value === id ? null : id; slots.value = []; slotsTotal.value = 0; if (selected.value !== null) void loadSlots() }
async function refresh() { await load(); if (selected.value !== null) await loadSlots(slotsPage.value) }
async function create() {
  saving.value = true; error.value = ''
  try {
    const start = new Date(form.start), end = new Date(form.end)
    if (!form.name.trim() || !Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start.getTime() <= Date.now() || end <= start) throw new Error(t('luckySecond.invalidRange'))
    await luckySecondAPI.create({ name: form.name.trim(), starts_at: start.toISOString(), ends_at: end.toISOString(), timezone, total_amount: String(form.amount), reward_count: form.count })
    creating.value = false; form.name = ''; appStore.showSuccess(t('luckySecond.created')); await load(1)
  } catch (err) { error.value = extractApiErrorMessage(err, t('common.error')) }
  finally { saving.value = false }
}
async function cancel(id: number) { saving.value = true; error.value = ''; try { await luckySecondAPI.cancel(id); confirmCancel.value = null; await refresh() } catch (err) { error.value = extractApiErrorMessage(err, t('common.error')) } finally { saving.value = false } }
onMounted(() => { void load(); timer = setInterval(() => { now.value = Date.now() }, 1000) })
onUnmounted(() => { listVersion++; slotsVersion++; if (timer) clearInterval(timer) })
</script>
