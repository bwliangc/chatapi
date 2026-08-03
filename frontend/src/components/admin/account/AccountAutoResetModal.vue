<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.autoResetSettings')"
    width="narrow"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="account-auto-reset-form" class="space-y-5" @submit.prevent="save">
      <label class="flex items-center justify-between gap-4">
        <span>
          <span class="block text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.autoResetEnabled') }}
          </span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.autoResetEnabledHint') }}
          </span>
        </span>
        <Toggle v-model="enabled" :disabled="loading || saving" />
      </label>

      <fieldset :disabled="loading || saving || !enabled">
        <legend class="input-label">{{ t('admin.accounts.autoResetStrategy') }}</legend>
        <div class="grid grid-cols-2 rounded-md bg-gray-100 p-1 dark:bg-dark-800">
          <button
            type="button"
            class="min-h-10 rounded px-3 py-2 text-sm font-medium transition-colors"
            :class="strategy === 'weekly_threshold' ? activeStrategyClass : inactiveStrategyClass"
            @click="strategy = 'weekly_threshold'"
          >
            {{ t('admin.accounts.autoResetWeeklyStrategy') }}
          </button>
          <button
            type="button"
            class="min-h-10 rounded px-3 py-2 text-sm font-medium transition-colors"
            :class="strategy === 'credit_expiry' ? activeStrategyClass : inactiveStrategyClass"
            @click="strategy = 'credit_expiry'"
          >
            {{ t('admin.accounts.autoResetExpiryStrategy') }}
          </button>
        </div>
      </fieldset>

      <div v-if="strategy === 'weekly_threshold'">
        <label for="auto-reset-weekly-threshold" class="input-label">
          {{ t('admin.accounts.autoResetWeeklyThreshold') }}
        </label>
        <div class="relative">
          <input
            id="auto-reset-weekly-threshold"
            v-model.number="weeklyThreshold"
            type="number"
            min="1"
            max="100"
            step="1"
            class="input pr-10"
            :disabled="loading || saving || !enabled"
            required
          />
          <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500">%</span>
        </div>
        <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.autoResetWeeklyThresholdHint') }}
        </p>
      </div>

      <div v-else>
        <label for="auto-reset-expiry-hours" class="input-label">
          {{ t('admin.accounts.autoResetExpiryHours') }}
        </label>
        <div class="relative">
          <input
            id="auto-reset-expiry-hours"
            v-model.number="expiryHours"
            type="number"
            min="1"
            max="720"
            step="1"
            class="input pr-16"
            :disabled="loading || saving || !enabled"
            required
          />
          <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500">
            {{ t('admin.accounts.hours') }}
          </span>
        </div>
        <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.autoResetExpiryHoursHint') }}
        </p>
      </div>

      <div>
        <label for="auto-reset-email" class="input-label">
          {{ t('admin.accounts.autoResetEmail') }}
        </label>
        <input
          id="auto-reset-email"
          v-model.trim="email"
          type="email"
          class="input"
          :placeholder="t('admin.accounts.autoResetEmailPlaceholder')"
          :required="enabled"
          :disabled="loading || saving"
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.autoResetEmailHint') }}
        </p>
      </div>

      <p v-if="loadError" class="text-sm text-red-600 dark:text-red-400">{{ loadError }}</p>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="account-auto-reset-form"
          class="btn btn-primary"
          :disabled="loading || saving"
        >
          {{ saving ? t('common.processing') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { Account } from '@/types'
import type { AccountAutoResetStrategy } from '@/api/admin/accounts'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()
const { t } = useI18n()
const appStore = useAppStore()

const activeStrategyClass = 'bg-white text-primary-700 shadow-sm dark:bg-dark-700 dark:text-primary-300'
const inactiveStrategyClass = 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'
const enabled = ref(false)
const strategy = ref<AccountAutoResetStrategy>('weekly_threshold')
const weeklyThreshold = ref(90)
const expiryHours = ref(24)
const email = ref('')
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')
let loadSequence = 0

watch(
  () => [props.show, props.account?.id] as const,
  async ([show, accountID]) => {
    if (!show || !accountID) return
    const sequence = ++loadSequence
    loading.value = true
    loadError.value = ''
    try {
      const settings = await adminAPI.accounts.getAutoReset(accountID)
      if (sequence !== loadSequence) return
      enabled.value = settings.enabled
      strategy.value = settings.strategy
      weeklyThreshold.value = settings.weekly_threshold
      expiryHours.value = settings.expiry_hours
      email.value = settings.email
    } catch (error: any) {
      if (sequence !== loadSequence) return
      loadError.value = error?.message || t('admin.accounts.autoResetLoadFailed')
    } finally {
      if (sequence === loadSequence) loading.value = false
    }
  },
  { immediate: true }
)

const handleClose = () => {
  if (!saving.value) emit('close')
}

const save = async () => {
  const accountID = props.account?.id
  if (!accountID || saving.value) return
  saving.value = true
  try {
    await adminAPI.accounts.updateAutoReset(accountID, {
      enabled: enabled.value,
      strategy: strategy.value,
      weekly_threshold: weeklyThreshold.value,
      expiry_hours: expiryHours.value,
      email: email.value,
    })
    appStore.showSuccess(t('admin.accounts.autoResetSaved'))
    emit('saved')
    emit('close')
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.autoResetSaveFailed'))
  } finally {
    saving.value = false
  }
}
</script>
