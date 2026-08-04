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

      <div>
        <div class="mb-2">
          <span class="input-label">{{ t('admin.accounts.autoResetConditions') }}</span>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.autoResetConditionsHint') }}
          </p>
        </div>

        <div v-if="conditions.length" class="divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700">
          <div v-for="condition in conditions" :key="condition.type" class="py-4 first:pt-3 last:pb-3">
            <div class="mb-2 flex items-center justify-between gap-3">
              <label :for="conditionInputID(condition.type)" class="text-sm font-medium text-gray-800 dark:text-gray-200">
                {{ conditionLabel(condition.type) }}
              </label>
              <button
                type="button"
                class="inline-flex h-8 w-8 flex-none items-center justify-center rounded text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-red-950/30 dark:hover:text-red-400"
                :disabled="loading || saving || !enabled"
                :title="t('admin.accounts.autoResetRemoveCondition')"
                :aria-label="t('admin.accounts.autoResetRemoveCondition')"
                @click="removeCondition(condition.type)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
            <div class="relative">
              <input
                :id="conditionInputID(condition.type)"
                v-model.number="condition.value"
                type="number"
                min="1"
                :max="condition.type === 'weekly_threshold' ? 100 : 43200"
                step="1"
                class="input"
                :class="condition.type === 'weekly_threshold' ? 'pr-10' : 'pr-16'"
                :disabled="loading || saving || !enabled"
                required
              />
              <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500">
                {{ condition.type === 'weekly_threshold' ? '%' : t('admin.accounts.minutes') }}
              </span>
            </div>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
              {{ conditionHint(condition.type) }}
            </p>
          </div>
        </div>

        <p v-else class="border-y border-gray-200 py-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
          {{ t('admin.accounts.autoResetNoConditions') }}
        </p>

        <div v-if="availableConditionTypes.length" class="mt-3 flex flex-wrap gap-2">
          <button
            v-for="conditionType in availableConditionTypes"
            :key="conditionType"
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="loading || saving || !enabled"
            @click="addCondition(conditionType)"
          >
            <Icon name="plus" size="sm" />
            <span>{{ t('admin.accounts.autoResetAddCondition', { condition: conditionLabel(conditionType) }) }}</span>
          </button>
        </div>
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
          :disabled="loading || saving || (enabled && conditions.length === 0)"
        >
          {{ saving ? t('common.processing') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { Account } from '@/types'
import type { AccountAutoResetCondition, AccountAutoResetConditionType } from '@/api/admin/accounts'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()
const { t } = useI18n()
const appStore = useAppStore()

const enabled = ref(false)
const conditions = ref<AccountAutoResetCondition[]>([])
const email = ref('')
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')
let loadSequence = 0

const allConditionTypes: AccountAutoResetConditionType[] = ['weekly_threshold', 'credit_expiry']
const availableConditionTypes = computed(() =>
  allConditionTypes.filter((conditionType) => !conditions.value.some((condition) => condition.type === conditionType))
)

const conditionLabel = (conditionType: AccountAutoResetConditionType) =>
  t(conditionType === 'weekly_threshold'
    ? 'admin.accounts.autoResetWeeklyThreshold'
    : 'admin.accounts.autoResetExpiryMinutes')

const conditionHint = (conditionType: AccountAutoResetConditionType) =>
  t(conditionType === 'weekly_threshold'
    ? 'admin.accounts.autoResetWeeklyThresholdHint'
    : 'admin.accounts.autoResetExpiryMinutesHint')

const conditionInputID = (conditionType: AccountAutoResetConditionType) =>
  conditionType === 'weekly_threshold' ? 'auto-reset-weekly-threshold' : 'auto-reset-expiry-minutes'

const addCondition = (conditionType: AccountAutoResetConditionType) => {
  if (conditions.value.some((condition) => condition.type === conditionType)) return
  conditions.value.push({
    type: conditionType,
    value: conditionType === 'weekly_threshold' ? 90 : 1440,
  })
}

const removeCondition = (conditionType: AccountAutoResetConditionType) => {
  conditions.value = conditions.value.filter((condition) => condition.type !== conditionType)
}

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
      conditions.value = (settings.conditions ?? [
        { type: 'weekly_threshold', value: settings.weekly_threshold },
        { type: 'credit_expiry', value: settings.expiry_minutes },
      ]).map((condition) => ({ ...condition }))
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
      conditions: conditions.value.map((condition) => ({ ...condition })),
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
