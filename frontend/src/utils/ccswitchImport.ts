import type { GroupPlatform } from '@/types'
import { buildCodexApiKeyAuthConfig } from '@/utils/codexAuthConfig'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-6.1-sol'
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
  model?: string
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

/**
 * Balance query CC Switch runs against the imported provider. CC Switch fills
 * `{{baseUrl}}` with the provider's base URL as stored — Codex and Grok imports
 * carry a trailing `/v1` (see `withV1Endpoint`), Claude ones do not, and users
 * may edit it either way afterwards — then evaluates the script, so the URL
 * strips an existing `/v1` instead of blindly appending one (`/v1/v1/usage`
 * is a 404 and CC Switch shows "query failed").
 */
export const CC_SWITCH_USAGE_SCRIPT = `({
    request: {
      url: "{{baseUrl}}".replace(/\\/+$/, "").replace(/\\/v1$/, "") + "/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`

function withV1Endpoint(baseUrl: string): string {
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, '')
  return normalizedBaseUrl.endsWith('/v1') ? normalizedBaseUrl : `${normalizedBaseUrl}/v1`
}

function withoutTrailingSlashes(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '')
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
        ? `${baseUrl.replace(/\/+$/, '')}/antigravity`
        : platform === 'grok'
          ? withV1Endpoint(baseUrl)
          : platform === 'openai' && app === 'codex'
            ? withoutTrailingSlashes(baseUrl)
          : baseUrl,
    model:
      platform === 'openai' && app === 'codex'
        ? OPENAI_CC_SWITCH_CODEX_MODEL
        : platform === 'grok' && app === 'grokbuild'
          ? GROK_CC_SWITCH_MODEL
          : undefined
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.app, input.baseUrl)
  const model = (input.model || config.model)?.trim()
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

  const modelEntries: [string, string | undefined][] = [['model', model]]
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

  if (config.app === 'codex') {
    // CC Switch's embedded-config import preserves provider auth and headers.
    // It deliberately leaves global [features] settings to the user.
    const modelConfig = model ? `model = ${JSON.stringify(model)}\n` : ''
    const codexConfig = `model_provider = "sub2api"
${modelConfig}web_search = "live"

[model_providers.sub2api]
name = "Sub2API"
base_url = ${JSON.stringify(config.endpoint)}
wire_api = "responses"
${buildCodexApiKeyAuthConfig(input.apiKey)}
`
    const payload = JSON.stringify({ config: codexConfig })
    const bytes = new TextEncoder().encode(payload)
    entries.push(['config', btoa(Array.from(bytes, (byte) => String.fromCharCode(byte)).join(''))])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
