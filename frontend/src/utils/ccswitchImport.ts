import type { GroupPlatform } from '@/types'

export type CcSwitchAppType = 'claude' | 'codex' | 'gemini'

export const CC_SWITCH_DEFAULT_MODELS: Record<CcSwitchAppType, string> = {
  claude: '',
  codex: 'gpt-5.5',
  gemini: ''
}

export interface CcSwitchImportConfig {
  app: CcSwitchAppType
  endpoint: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  app: CcSwitchAppType
  model?: string
  haikuModel?: string
  sonnetModel?: string
  opusModel?: string
  providerName: string
  apiKey: string
  usageScript: string
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  app: CcSwitchAppType,
  baseUrl: string
): CcSwitchImportConfig {
  return {
    app,
    endpoint: platform === 'antigravity' ? `${baseUrl}/antigravity` : baseUrl
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.app, input.baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  const modelEntries: [string, string | undefined][] = [['model', input.model]]
  if (config.app === 'claude') {
    modelEntries.push(
      ['haikuModel', input.haikuModel],
      ['sonnetModel', input.sonnetModel],
      ['opusModel', input.opusModel]
    )
  }

  let insertAt = 2
  for (const [key, value] of modelEntries) {
    const trimmedValue = value?.trim()
    if (trimmedValue) {
      entries.splice(insertAt, 0, [key, trimmedValue])
      insertAt += 1
    }
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
