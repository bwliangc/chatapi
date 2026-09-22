<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import type { GroupDynamicRate } from '@/types'

const props = defineProps<{ modelValue: GroupDynamicRate }>()
const emit = defineEmits<{ 'update:modelValue': [value: GroupDynamicRate] }>()
const { t } = useI18n()
function update(patch: Partial<GroupDynamicRate>) {
  emit('update:modelValue', { ...props.modelValue, ...patch })
}
function numberValue(event: Event): number {
  return (event.target as HTMLInputElement).valueAsNumber
}
</script>

<template>
  <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
    <label class="flex items-center justify-between gap-3 text-sm font-medium">
      {{ t('admin.groups.dynamicRate.title') }}
      <Toggle :model-value="modelValue.enabled" @update:model-value="update({ enabled: $event })" />
    </label>
    <template v-if="modelValue.enabled">
      <div class="mt-3 grid grid-cols-2 gap-3">
        <label class="input-label">
          {{ t('admin.groups.dynamicRate.min') }}
          <input :value="modelValue.min" type="number" min="0" :max="modelValue.max" step="0.0001" required class="input mt-1" @input="update({ min: numberValue($event) })" />
        </label>
        <label class="input-label">
          {{ t('admin.groups.dynamicRate.max') }}
          <input :value="modelValue.max" type="number" :min="modelValue.min" max="999999.9999" step="0.0001" required class="input mt-1" @input="update({ max: numberValue($event) })" />
        </label>
      </div>
      <p class="input-hint mt-2">{{ t('admin.groups.dynamicRate.hint') }}</p>
      <p class="input-hint">{{ t('admin.groups.dynamicRate.rules') }}</p>
    </template>
  </div>
</template>
