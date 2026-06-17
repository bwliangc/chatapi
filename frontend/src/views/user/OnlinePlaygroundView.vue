<template>
  <AppLayout>
    <div class="mx-auto w-[90%]">
      <div class="mb-4">
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
          {{ t('onlinePlayground.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('onlinePlayground.description') }}
        </p>
      </div>

      <!-- Renders in the main content area like any other page; fixed height so
           the message list scrolls internally instead of growing the page. -->
      <div class="card h-[calc(100vh-13rem)] overflow-hidden">
        <PlaygroundChat class="h-full" />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlaygroundChat from '@/components/playground/PlaygroundChat.vue'

const { t } = useI18n()
</script>
