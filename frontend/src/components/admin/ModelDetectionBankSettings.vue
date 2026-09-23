<template>
  <div class="space-y-4 border-t border-gray-100 pt-5 dark:border-dark-700">
    <div>
      <h3 class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.modelDetection.bankUpdate.title') }}</h3>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelDetection.bankUpdate.description') }}</p>
    </div>
    <dl v-if="status" class="grid gap-3 text-xs sm:grid-cols-2">
      <div><dt class="text-gray-500">{{ t('admin.modelDetection.bankUpdate.current') }}</dt><dd class="mt-1 font-mono">{{ status.current.commit.slice(0, 12) }} · {{ t('admin.modelDetection.bankUpdate.models', { count: status.current.model_count }) }}</dd><dd class="mt-1 text-gray-500">{{ status.current.built_at }}</dd></div>
      <div><dt class="text-gray-500">{{ t('admin.modelDetection.bankUpdate.source') }}</dt><dd class="mt-1">{{ t(`admin.modelDetection.bankUpdate.${status.source}`) }}</dd></div>
      <div v-if="checked?.remote"><dt class="text-gray-500">{{ t('admin.modelDetection.bankUpdate.remote') }}</dt><dd class="mt-1 font-mono">{{ checked.remote.commit.slice(0, 12) }}<span v-if="checked.compatible"> · {{ t('admin.modelDetection.bankUpdate.models', { count: checked.remote.model_count }) }}</span></dd></div>
      <div v-if="status.previous"><dt class="text-gray-500">{{ t('admin.modelDetection.bankUpdate.previous') }}</dt><dd class="mt-1 font-mono">{{ status.previous.commit.slice(0, 12) }}</dd></div>
    </dl>
    <p v-if="status?.warning" class="text-xs text-amber-600 dark:text-amber-400">{{ status.warning }}</p>
    <p v-if="checked" class="text-sm" :class="checked.compatible ? 'text-gray-600 dark:text-gray-300' : 'text-amber-600 dark:text-amber-400'" role="status">
      {{ checked.compatible ? t(checked.update_available ? 'admin.modelDetection.bankUpdate.available' : 'admin.modelDetection.bankUpdate.latest') : t('admin.modelDetection.bankUpdate.requiresUpgrade') }}
    </p>
    <div class="flex flex-wrap gap-2">
      <button type="button" class="btn btn-secondary" :disabled="busy !== ''" @click="check">{{ t(busy === 'check' ? 'admin.modelDetection.bankUpdate.checking' : 'admin.modelDetection.bankUpdate.check') }}</button>
      <button type="button" class="btn btn-primary" :disabled="busy !== '' || !checked?.compatible || !checked?.update_available" @click="apply">{{ t(busy === 'update' ? 'admin.modelDetection.bankUpdate.updating' : 'admin.modelDetection.bankUpdate.update') }}</button>
      <button type="button" class="btn btn-secondary" :disabled="busy !== '' || !status?.previous" @click="rollback">{{ t(busy === 'rollback' ? 'admin.modelDetection.bankUpdate.rollingBack' : 'admin.modelDetection.bankUpdate.rollback') }}</button>
      <a href="https://github.com/xqy2006/ModelTrace" target="_blank" rel="noopener noreferrer" class="btn btn-secondary">
        <GitHubMark class="h-4 w-4 shrink-0" />
        {{ t('admin.modelDetection.bankUpdate.repository') }}
      </a>
    </div>
    <p v-if="message" class="text-xs text-green-600 dark:text-green-400" role="status">{{ message }}</p>
    <p v-if="error" class="text-xs text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <TotpStepUpDialog :controller="stepUp" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import GitHubMark from '@/components/auth/GitHubMark.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useStepUp, isStepUpCancelled } from '@/composables/useStepUp'
import { getBankStatus, checkBankUpdate, updateBank, rollbackBank, type BankStatus, type BankCheck } from '@/api/admin/modelDetection'

const { t } = useI18n()
const stepUp = useStepUp()
const status = ref<BankStatus | null>(null)
const checked = ref<BankCheck | null>(null)
const busy = ref(''), error = ref(''), message = ref('')
const errorText = (e: unknown) => e instanceof Error ? e.message : (e as { message?: string })?.message || t('common.error')
async function operation(name: string, action: () => Promise<void>) {
  if (busy.value) return
  busy.value = name; error.value = ''; message.value = ''
  try { await action() }
  catch (e) {
    if (!isStepUpCancelled(e)) error.value = errorText(e)
    // A request can fail after the DB commit. Refresh status before allowing a retry.
    checked.value = null
    try { status.value = await getBankStatus() } catch { /* preserve primary error */ }
  } finally { busy.value = '' }
}
async function check() {
  await operation('check', async () => {
    checked.value = await checkBankUpdate()
    status.value = checked.value.status
  })
}
async function apply() {
  const candidate = checked.value
  if (!candidate?.remote || !candidate.compatible || !candidate.update_available) return
  await operation('update', async () => {
    status.value = await stepUp.run(() => updateBank(candidate.remote!.commit, candidate.status.revision))
    checked.value = null
    message.value = t('admin.modelDetection.bankUpdate.updated')
  })
}
async function rollback() {
  if (!status.value?.previous) return
  const revision = status.value.revision
  await operation('rollback', async () => {
    status.value = await stepUp.run(() => rollbackBank(revision))
    checked.value = null
    message.value = t('admin.modelDetection.bankUpdate.rolledBack')
  })
}
onMounted(() => operation('load', async () => { status.value = await getBankStatus() }))
</script>
