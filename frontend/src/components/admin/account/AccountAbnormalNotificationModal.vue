<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.abnormalNotificationSettings')"
    width="narrow"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="account-abnormal-notification-form" class="space-y-5" @submit.prevent="save">
      <label class="flex items-center justify-between gap-4">
        <span>
          <span class="block text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.abnormalNotificationEnabled') }}
          </span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.abnormalNotificationEnabledHint') }}
          </span>
        </span>
        <input
          v-model="enabled"
          type="checkbox"
          class="h-5 w-5 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          :disabled="loading || saving"
        />
      </label>

      <fieldset :disabled="!enabled || loading || saving || !!loadError" class="space-y-3">
        <legend class="input-label mb-2">{{ t('admin.accounts.abnormalNotificationStatuses') }}</legend>
        <label v-for="status in statusOptions" :key="status" class="flex items-start gap-3">
          <input
            v-model="statuses"
            type="checkbox"
            :value="status"
            class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <span>
            <span class="block text-sm text-gray-900 dark:text-white">
              {{ t(`admin.accounts.abnormalNotificationStatusLabels.${status}`) }}
            </span>
            <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
              {{ t(`admin.accounts.abnormalNotificationStatusHints.${status}`) }}
            </span>
          </span>
        </label>
      </fieldset>
      <p v-if="enabled && statuses.length === 0" class="text-sm text-red-600 dark:text-red-400">
        {{ t('admin.accounts.abnormalNotificationStatusRequired') }}
      </p>

      <div>
        <label for="abnormal-notification-email" class="input-label">
          {{ t('admin.accounts.abnormalNotificationEmail') }}
        </label>
        <input
          id="abnormal-notification-email"
          v-model.trim="email"
          type="email"
          class="input"
          :placeholder="t('admin.accounts.abnormalNotificationEmailPlaceholder')"
          :required="enabled"
          :disabled="loading || saving || !!loadError"
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.abnormalNotificationEmailHint') }}
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
          form="account-abnormal-notification-form"
          class="btn btn-primary"
          :disabled="loading || saving || !!loadError || (enabled && statuses.length === 0)"
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
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { Account } from '@/types'
import type { AccountNotificationStatus } from '@/api/admin/accounts'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()
const { t } = useI18n()
const appStore = useAppStore()

const enabled = ref(false)
const email = ref('')
const statusOptions: AccountNotificationStatus[] = ['error', 'rate_limited', 'temp_unschedulable', 'overloaded']
const statuses = ref<AccountNotificationStatus[]>(['error'])
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')
let loadSequence = 0

watch(
  () => [props.show, props.account?.id] as const,
  async ([show, accountID]) => {
    const sequence = ++loadSequence
    if (!show || !accountID) return
    loading.value = true
    loadError.value = ''
    enabled.value = false
    email.value = ''
    statuses.value = ['error']
    try {
      const settings = await adminAPI.accounts.getAbnormalNotification(accountID)
      if (sequence !== loadSequence) return
      enabled.value = settings.enabled
      email.value = settings.email
      statuses.value = [...(settings.statuses ?? ['error'])]
    } catch (error: any) {
      if (sequence !== loadSequence) return
      loadError.value = error?.message || t('admin.accounts.abnormalNotificationLoadFailed')
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
  if (!accountID || loading.value || saving.value || loadError.value) return
  if (enabled.value && statuses.value.length === 0) {
    appStore.showError(t('admin.accounts.abnormalNotificationStatusRequired'))
    return
  }
  saving.value = true
  try {
    await adminAPI.accounts.updateAbnormalNotification(accountID, {
      enabled: enabled.value,
      email: email.value,
      statuses: [...statuses.value],
    })
    appStore.showSuccess(t('admin.accounts.abnormalNotificationSaved'))
    emit('saved')
    emit('close')
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.abnormalNotificationSaveFailed'))
  } finally {
    saving.value = false
  }
}
</script>
