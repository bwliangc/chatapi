import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'
export const GROK_CC_SWITCH_MODEL = 'grok-4.5'

export type CcSwitchAppType = 'claude' | 'codex' | 'gemini' | 'grokbuild'

export const CC_SWITCH_DEFAULT_MODELS: Record<CcSwitchAppType, string> = {
  claude: '',
  codex: OPENAI_CC_SWITCH_CODEX_MODEL,
  gemini: '',
  grokbuild: GROK_CC_SWITCH_MODEL
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

function withV1Endpoint(baseUrl: string): string {
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, '')
  return normalizedBaseUrl.endsWith('/v1') ? normalizedBaseUrl : `${normalizedBaseUrl}/v1`
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  app: CcSwitchAppType,
  baseUrl: string
): CcSwitchImportConfig {
  return {
    app,
    endpoint:
      platform === 'antigravity'
        ? `${baseUrl}/antigravity`
        : platform === 'grok'
          ? withV1Endpoint(baseUrl)
          : baseUrl
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
