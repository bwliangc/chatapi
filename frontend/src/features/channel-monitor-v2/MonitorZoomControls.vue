<template>
  <span v-if="!disabled" class="hidden text-[11px] text-gray-500 dark:text-gray-400 sm:inline">
    {{ t('channelMonitorV2.zoom.wheelHint') }}
  </span>
  <div class="inline-flex shrink-0 items-center gap-1" role="group" :aria-label="t('channelMonitorV2.zoom.label')">
    <button
      type="button"
      class="zoom-button w-8 text-base"
      :disabled="disabled || zoom.span <= MIN_ZOOM_SPAN"
      :aria-label="t('channelMonitorV2.zoom.in')"
      :title="t('channelMonitorV2.zoom.in')"
      @click="zoom = zoomAt(zoom, 'in')"
    >+</button>
    <button
      type="button"
      class="zoom-button w-8 text-base"
      :disabled="disabled || !zoomed"
      :aria-label="t('channelMonitorV2.zoom.out')"
      :title="t('channelMonitorV2.zoom.out')"
      @click="zoom = zoomAt(zoom, 'out')"
    >−</button>
    <button
      type="button"
      class="zoom-button px-2 text-[11px]"
      :disabled="disabled || !zoomed"
      @click="zoom = resetZoom()"
    >{{ t('channelMonitorV2.zoom.reset') }}</button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { isZoomed, MIN_ZOOM_SPAN, resetZoom, zoomAt, type ZoomState } from './monitorZoom'

defineProps<{ disabled?: boolean }>()
const zoom = defineModel<ZoomState>({ required: true })
const { t } = useI18n()
const zoomed = computed(() => isZoomed(zoom.value))
</script>

<style scoped>
.zoom-button {
  @apply inline-flex h-8 items-center justify-center rounded-lg border border-gray-200 bg-white font-semibold
    text-gray-600 hover:bg-gray-50 disabled:opacity-50 dark:border-dark-700 dark:bg-dark-900
    dark:text-gray-300 dark:hover:bg-dark-800;
}
</style>
