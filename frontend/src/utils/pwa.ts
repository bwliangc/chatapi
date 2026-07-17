import { computed, ref } from 'vue'

type InstallChoice = {
  outcome: 'accepted' | 'dismissed'
  platform: string
}

type BeforeInstallPromptEvent = Event & {
  prompt: () => Promise<void>
  userChoice: Promise<InstallChoice>
}

const deferredPrompt = ref<BeforeInstallPromptEvent | null>(null)
const installed = ref(false)
let initialized = false

function detectStandaloneMode() {
  const navigatorWithStandalone = navigator as Navigator & { standalone?: boolean }
  return window.matchMedia?.('(display-mode: standalone)').matches === true ||
    navigatorWithStandalone.standalone === true
}

export function initPwaInstall() {
  if (initialized || typeof window === 'undefined') return
  initialized = true
  installed.value = detectStandaloneMode()

  window.addEventListener('beforeinstallprompt', ((event: BeforeInstallPromptEvent) => {
    event.preventDefault()
    deferredPrompt.value = event
  }) as EventListener)

  window.addEventListener('appinstalled', () => {
    deferredPrompt.value = null
    installed.value = true
  })

  window.matchMedia?.('(display-mode: standalone)').addEventListener?.('change', (event) => {
    installed.value = event.matches
  })
}

export function registerPwaServiceWorker() {
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return

  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch((error) => {
      console.warn('Failed to register PWA service worker:', error)
    })
  }, { once: true })
}

export function usePwaInstall() {
  const canInstall = computed(() => deferredPrompt.value !== null && !installed.value)

  async function installApp() {
    const prompt = deferredPrompt.value
    if (!prompt || installed.value) return false

    await prompt.prompt()
    const choice = await prompt.userChoice
    deferredPrompt.value = null
    return choice.outcome === 'accepted'
  }

  return {
    canInstall,
    installed: computed(() => installed.value),
    installApp
  }
}
