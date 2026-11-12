import { computed } from 'vue'
import { useAppStore } from '@/stores/app'

function normalizeHostname(hostname: string): string {
  return hostname.trim().toLowerCase().replace(/\.$/, '')
}

export function useIngressNodeLabel() {
  const appStore = useAppStore()
  const namesByHostname = computed(() => {
    const names = new Map<string, string>()
    for (const { name, endpoint } of appStore.cachedPublicSettings?.custom_endpoints ?? []) {
      const label = name?.trim()
      if (!label) continue

      try {
        const url = new URL(endpoint)
        if (url.protocol !== 'http:' && url.protocol !== 'https:') continue
        const hostname = normalizeHostname(url.hostname)
        if (hostname && !names.has(hostname)) names.set(hostname, label)
      } catch {
        // Skip invalid endpoint URLs.
      }
    }
    return names
  })

  return (domain?: string | null): string => {
    if (!domain) return '-'
    return namesByHostname.value.get(normalizeHostname(domain)) ?? domain
  }
}
