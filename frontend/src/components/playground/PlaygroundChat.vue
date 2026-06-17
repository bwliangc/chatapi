<template>
  <div class="flex h-full flex-col gap-3 p-4">
    <!-- Controls: key + model -->
    <div class="shrink-0 space-y-3 rounded-lg border border-gray-100 bg-gray-50/60 p-3 dark:border-dark-700 dark:bg-dark-800/40">
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="input-label">{{ t('onlinePlayground.selectKey') }}</label>
          <select v-model="selectedKeyId" class="input" :disabled="streaming">
            <option v-if="keys.length === 0" :value="null" disabled>
              {{ t('onlinePlayground.noActiveKey') }}
            </option>
            <option v-for="k in keys" :key="k.id" :value="k.id">
              {{ keyLabel(k) }}
            </option>
          </select>
        </div>
        <div>
          <label class="input-label flex items-center gap-2">
            <span>{{ t('onlinePlayground.selectModel') }}</span>
            <button
              type="button"
              class="leading-none text-gray-400 transition-colors hover:text-gray-600 disabled:opacity-40 dark:hover:text-gray-200"
              :class="{ 'animate-spin': modelsLoading }"
              :title="t('onlinePlayground.refreshModels')"
              :disabled="!selectedKey || modelsLoading || streaming"
              @click="loadModels"
            >↻</button>
            <span v-if="modelsLoading" class="text-xs font-normal text-gray-400">{{ t('onlinePlayground.loadingModels') }}</span>
          </label>
          <select v-if="models.length > 0" v-model="selectedModel" class="input" :disabled="streaming">
            <option v-for="m in models" :key="m" :value="m">{{ m }}</option>
          </select>
          <input
            v-else
            v-model="selectedModel"
            class="input"
            :placeholder="t('onlinePlayground.modelPlaceholder')"
            :disabled="streaming"
          />
          <p v-if="models.length > 0" class="mt-1 text-xs text-gray-400">
            {{ t('onlinePlayground.modelsCount', { count: models.length }) }}
          </p>
          <p v-else-if="selectedKey && !modelsLoading" class="mt-1 text-xs text-gray-400">
            {{ t('onlinePlayground.modelsManualHint') }}
          </p>
        </div>
      </div>
      <p v-if="keys.length === 0 && !keysLoading" class="text-xs text-amber-600 dark:text-amber-400">
        {{ t('onlinePlayground.createKeyHint') }}
      </p>
    </div>

    <!-- Chat messages -->
    <div ref="scrollEl" class="flex-1 space-y-4 overflow-y-auto rounded-lg border border-gray-100 p-3 dark:border-dark-700">
      <div v-if="messages.length === 0" class="flex h-full items-center justify-center">
        <p class="px-6 text-center text-sm text-gray-400">{{ t('onlinePlayground.emptyState') }}</p>
      </div>
      <div v-for="(msg, i) in messages" :key="i" class="flex" :class="msg.role === 'user' ? 'justify-end' : 'justify-start'">
        <div class="max-w-[85%] rounded-2xl px-4 py-2.5 text-sm" :class="bubbleClass(msg)">
          <div v-if="msg.images && msg.images.length" class="mb-2 flex flex-wrap gap-2">
            <img v-for="(img, j) in msg.images" :key="j" :src="img" class="h-20 w-20 rounded-lg object-cover" />
          </div>
          <!-- Assistant output is markdown (sanitized); user/error text stays plain. -->
          <template v-if="msg.role === 'assistant' && !msg.error">
            <div class="md-body break-words" v-html="renderMarkdown(msg.text)"></div>
            <span v-if="msg.streaming" class="animate-pulse">▌</span>
          </template>
          <p v-else class="whitespace-pre-wrap break-words">{{ msg.text }}</p>
        </div>
      </div>
    </div>

    <!-- Composer -->
    <div class="shrink-0 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
      <div v-if="pendingImages.length" class="mb-2 flex flex-wrap gap-2">
        <div v-for="(img, i) in pendingImages" :key="i" class="relative">
          <img :src="img" class="h-14 w-14 rounded-lg object-cover" />
          <button
            type="button"
            class="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-gray-700 text-xs text-white"
            @click="pendingImages.splice(i, 1)"
          >×</button>
        </div>
      </div>

      <div class="flex items-end gap-2">
        <button type="button" class="btn btn-secondary shrink-0 px-2.5" :title="t('onlinePlayground.attachImage')" :disabled="streaming" @click="imageInput?.click()">🖼️</button>
        <textarea
          v-model="input"
          rows="1"
          class="input max-h-40 flex-1 resize-none"
          :placeholder="t('onlinePlayground.inputPlaceholder')"
          @keydown.enter.exact.prevent="send"
        ></textarea>
        <button v-if="!streaming" type="button" class="btn btn-primary shrink-0" :disabled="!canSend" @click="send">
          {{ t('onlinePlayground.send') }}
        </button>
        <button v-else type="button" class="btn btn-secondary shrink-0" @click="stop">
          {{ t('onlinePlayground.stop') }}
        </button>
      </div>
      <div class="mt-2 flex items-center justify-between">
        <p class="text-xs text-gray-400">{{ t('onlinePlayground.enterHint') }}</p>
        <button v-if="messages.length" type="button" class="text-xs text-gray-400 hover:text-gray-600" @click="clearChat">
          {{ t('onlinePlayground.clear') }}
        </button>
      </div>

      <input ref="imageInput" type="file" accept="image/*" multiple class="hidden" @change="onPickImages" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { keysAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import {
  listPlaygroundModels,
  streamChatCompletion,
  type ChatMessage,
  type ChatContentPart,
} from '@/api/playground'
import type { ApiKey } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

marked.setOptions({ breaks: true, gfm: true })

// 助手输出按 markdown 渲染；marked → DOMPurify 消毒（与项目其它 markdown 渲染一致），防 XSS。
function renderMarkdown(text: string): string {
  return DOMPurify.sanitize(marked.parse(text) as string)
}

const MAX_IMAGE_BYTES = 20 * 1024 * 1024 // 20MB per image
const MAX_IMAGE_LABEL = `${Math.round(MAX_IMAGE_BYTES / (1024 * 1024))}MB`

interface DisplayMessage {
  role: 'user' | 'assistant'
  text: string
  images?: string[] // data URLs (user messages)
  streaming?: boolean
  error?: boolean
}

const keys = ref<ApiKey[]>([])
const keysLoading = ref(false)
const selectedKeyId = ref<number | null>(null)

const models = ref<string[]>([])
const modelsLoading = ref(false)
const selectedModel = ref('')

const messages = ref<DisplayMessage[]>([])
const input = ref('')
const pendingImages = ref<string[]>([])

const streaming = ref(false)
let abortController: AbortController | null = null
let modelsAbort: AbortController | null = null

const imageInput = ref<HTMLInputElement | null>(null)
const scrollEl = ref<HTMLElement | null>(null)

const selectedKey = computed(() => keys.value.find(k => k.id === selectedKeyId.value))

const canSend = computed(
  () =>
    !streaming.value &&
    !!selectedKey.value &&
    selectedModel.value.trim().length > 0 &&
    (input.value.trim().length > 0 || pendingImages.value.length > 0),
)

function keyLabel(k: ApiKey): string {
  const name = k.name || `#${k.id}`
  return k.group?.name ? `${name} · ${k.group.name}` : name
}

function bubbleClass(msg: DisplayMessage): string {
  if (msg.error) return 'bg-red-50 text-red-700 dark:bg-red-500/10 dark:text-red-300'
  return msg.role === 'user'
    ? 'bg-primary-600 text-white'
    : 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-100'
}

async function loadKeys() {
  keysLoading.value = true
  try {
    const res = await keysAPI.list(1, 100, { status: 'active' })
    keys.value = (res.items || []).filter(k => k.status === 'active' && !!k.key)
    if (keys.value.length > 0 && selectedKeyId.value === null) {
      selectedKeyId.value = keys.value[0].id
    }
  } catch (e: unknown) {
    appStore.showError(errMsg(e, t('onlinePlayground.loadKeysFailed')))
  } finally {
    keysLoading.value = false
  }
}

async function loadModels() {
  const key = selectedKey.value?.key
  if (!key) {
    models.value = []
    return
  }
  modelsAbort?.abort()
  modelsAbort = new AbortController()
  modelsLoading.value = true
  try {
    const list = await listPlaygroundModels(key, modelsAbort.signal)
    models.value = list
    if (list.length > 0 && (!selectedModel.value || !list.includes(selectedModel.value))) {
      selectedModel.value = list[0]
    }
  } catch {
    // Model listing may be unsupported for some groups — keep the manual input usable.
    models.value = []
  } finally {
    modelsLoading.value = false
  }
}

watch(selectedKeyId, () => {
  void loadModels()
})

function onPickImages(e: Event) {
  const target = e.target as HTMLInputElement
  const files = target.files
  if (!files) return
  for (const file of Array.from(files)) {
    if (file.size > MAX_IMAGE_BYTES) {
      appStore.showError(t('onlinePlayground.fileTooLarge', { name: file.name, limit: MAX_IMAGE_LABEL }))
      continue
    }
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === 'string') pendingImages.value.push(reader.result)
    }
    reader.readAsDataURL(file)
  }
  target.value = ''
}

// Build the OpenAI-format messages array from the display history for sending.
function buildApiMessages(): ChatMessage[] {
  return messages.value
    .filter(m => !m.error)
    .map<ChatMessage>(m => {
      if (m.role === 'assistant') {
        return { role: 'assistant', content: m.text }
      }
      if (m.images && m.images.length > 0) {
        const parts: ChatContentPart[] = []
        if (m.text.trim().length > 0) parts.push({ type: 'text', text: m.text })
        for (const url of m.images) parts.push({ type: 'image_url', image_url: { url } })
        return { role: 'user', content: parts }
      }
      return { role: 'user', content: m.text }
    })
}

async function send() {
  if (!canSend.value || !selectedKey.value) return

  messages.value.push({
    role: 'user',
    text: input.value.trim(),
    images: [...pendingImages.value],
  })

  input.value = ''
  pendingImages.value = []

  // Push first, then grab the reactive proxy back out of the array. Mutating the
  // raw pushed object directly would NOT trigger Vue reactivity (the proxy wraps
  // the array element), so streamed deltas wouldn't render until the next ref
  // change — that was why streaming looked like it appeared all at once.
  messages.value.push({ role: 'assistant', text: '', streaming: true })
  const assistant = messages.value[messages.value.length - 1]
  scrollToBottom()

  streaming.value = true
  abortController = new AbortController()
  const apiMessages = buildApiMessages()

  await streamChatCompletion(
    selectedKey.value.key,
    selectedModel.value.trim(),
    apiMessages,
    {
      onDelta: (delta) => {
        assistant.text += delta
        scrollToBottom()
      },
      onDone: () => {
        assistant.streaming = false
        streaming.value = false
        scrollToBottom()
      },
      onError: (err) => {
        assistant.streaming = false
        if (assistant.text.length === 0) {
          assistant.text = err.message
          assistant.error = true
        } else {
          assistant.text += `\n\n[${t('onlinePlayground.streamError')}: ${err.message}]`
        }
        streaming.value = false
        scrollToBottom()
      },
    },
    abortController.signal,
  )
}

function stop() {
  abortController?.abort()
  abortController = null
  streaming.value = false
  const last = messages.value[messages.value.length - 1]
  if (last && last.role === 'assistant') last.streaming = false
}

function clearChat() {
  if (streaming.value) stop()
  messages.value = []
}

function scrollToBottom() {
  nextTick(() => {
    if (scrollEl.value) scrollEl.value.scrollTop = scrollEl.value.scrollHeight
  })
}

function errMsg(e: unknown, fallback: string): string {
  if (e && typeof e === 'object' && 'message' in e && typeof (e as { message: unknown }).message === 'string') {
    return (e as { message: string }).message
  }
  return fallback
}

onMounted(() => {
  void loadKeys()
})

onBeforeUnmount(() => {
  abortController?.abort()
  modelsAbort?.abort()
})
</script>

<style scoped>
/* Styling for the sanitized markdown HTML injected via v-html (not scoped by
   Vue, hence :deep). Kept compact to fit chat bubbles. */
.md-body :deep(p) {
  margin: 0.3rem 0;
}
.md-body :deep(p:first-child) {
  margin-top: 0;
}
.md-body :deep(p:last-child) {
  margin-bottom: 0;
}
.md-body :deep(h1),
.md-body :deep(h2),
.md-body :deep(h3),
.md-body :deep(h4) {
  font-weight: 600;
  margin: 0.6rem 0 0.3rem;
  line-height: 1.3;
}
.md-body :deep(ul) {
  list-style: disc;
  padding-left: 1.25rem;
  margin: 0.3rem 0;
}
.md-body :deep(ol) {
  list-style: decimal;
  padding-left: 1.25rem;
  margin: 0.3rem 0;
}
.md-body :deep(li) {
  margin: 0.1rem 0;
}
.md-body :deep(a) {
  text-decoration: underline;
}
.md-body :deep(pre) {
  background: rgba(0, 0, 0, 0.06);
  padding: 0.6rem 0.75rem;
  border-radius: 0.5rem;
  overflow-x: auto;
  margin: 0.4rem 0;
}
.md-body :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.85em;
}
.md-body :deep(:not(pre) > code) {
  background: rgba(0, 0, 0, 0.06);
  padding: 0.1em 0.35em;
  border-radius: 0.25rem;
}
.md-body :deep(blockquote) {
  border-left: 3px solid rgba(0, 0, 0, 0.15);
  padding-left: 0.6rem;
  margin: 0.3rem 0;
  opacity: 0.85;
}
.md-body :deep(table) {
  border-collapse: collapse;
  margin: 0.4rem 0;
}
.md-body :deep(th),
.md-body :deep(td) {
  border: 1px solid rgba(0, 0, 0, 0.15);
  padding: 0.25rem 0.5rem;
}

/* Dark mode: lighten code/quote backgrounds against the dark bubble. */
:global(.dark) .md-body :deep(pre),
:global(.dark) .md-body :deep(:not(pre) > code) {
  background: rgba(255, 255, 255, 0.1);
}
:global(.dark) .md-body :deep(blockquote),
:global(.dark) .md-body :deep(th),
:global(.dark) .md-body :deep(td) {
  border-color: rgba(255, 255, 255, 0.18);
}
</style>
