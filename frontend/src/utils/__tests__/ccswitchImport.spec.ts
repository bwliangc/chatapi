import { describe, expect, it } from 'vitest'
import {
  CC_SWITCH_DEFAULT_MODELS,
  CC_SWITCH_USAGE_SCRIPT,
  GROK_CC_SWITCH_MODEL,
  OPENAI_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink
} from '@/utils/ccswitchImport'
import type { GroupPlatform } from '@/types'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('ccswitchImport utils', () => {
  it('defaults OpenAI CC Switch imports to the current Codex model', () => {
    expect(OPENAI_CC_SWITCH_CODEX_MODEL).toBe('gpt-5.5')
  })

  it('defaults Grok Build imports to the current Grok model', () => {
    expect(GROK_CC_SWITCH_MODEL).toBe('grok-4.5')
  })

  it('keeps the current Codex model as the dialog default', () => {
    expect(CC_SWITCH_DEFAULT_MODELS.codex).toBe('gpt-5.5')
  })

  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it.each([
    { app: 'claude' as const, platform: 'anthropic' as GroupPlatform, model: 'claude-sonnet-4-6' },
    { app: 'codex' as const, platform: 'openai' as GroupPlatform, model: 'gpt-5.5' },
    { app: 'gemini' as const, platform: 'gemini' as GroupPlatform, model: 'gemini-3.1-pro-preview' },
    { app: 'grokbuild' as const, platform: 'grok' as GroupPlatform, model: GROK_CC_SWITCH_MODEL }
  ])('uses the selected $app app and primary model', ({ app, platform, model }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform,
        app,
        model
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(
      app === 'codex' ? baseInput.baseUrl : app === 'grokbuild' ? `${baseInput.baseUrl}/v1` : baseInput.baseUrl
    )
    expect(params.get('model')).toBe(model)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it.each([
    ['https://api.example.com', 'https://api.example.com'],
    ['https://api.example.com/', 'https://api.example.com'],
    ['https://api.example.com/v1', 'https://api.example.com/v1'],
    ['https://api.example.com/v1/', 'https://api.example.com/v1']
  ])('keeps Codex imports on the configured endpoint for base URL %s', (baseUrl, endpoint) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        baseUrl,
        platform: 'openai',
        app: 'codex'
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(endpoint)
    expect(params.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it.each([
    'https://api.example.com',
    'https://api.example.com/',
    'https://api.example.com/v1',
    'https://api.example.com/v1/'
  ])('imports Grok Build with one /v1 suffix for base URL %s', (baseUrl) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        baseUrl,
        platform: 'grok',
        app: 'grokbuild',
        model: GROK_CC_SWITCH_MODEL
      })
    )

    expect(params.get('app')).toBe('grokbuild')
    expect(params.get('endpoint')).toBe('https://api.example.com/v1')
    expect(params.get('model')).toBe(GROK_CC_SWITCH_MODEL)
  })

  it('adds Claude model aliases and trims model values', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        app: 'claude',
        model: ' claude-sonnet-4-6 ',
        haikuModel: ' claude-haiku-4-5 ',
        sonnetModel: 'claude-sonnet-4-6',
        opusModel: ' claude-opus-4-8 '
      })
    )

    expect(params.get('model')).toBe('claude-sonnet-4-6')
    expect(params.get('haikuModel')).toBe('claude-haiku-4-5')
    expect(params.get('sonnetModel')).toBe('claude-sonnet-4-6')
    expect(params.get('opusModel')).toBe('claude-opus-4-8')
  })

  it('does not include Claude aliases for other apps', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        app: 'codex',
        model: 'gpt-5.5',
        haikuModel: 'ignored-haiku',
        sonnetModel: 'ignored-sonnet',
        opusModel: 'ignored-opus'
      })
    )

    expect(params.has('haikuModel')).toBe(false)
    expect(params.has('sonnetModel')).toBe(false)
    expect(params.has('opusModel')).toBe(false)
  })

  it('omits blank model values', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        app: 'claude',
        model: ' ',
        haikuModel: '',
        sonnetModel: ' ',
        opusModel: undefined
      })
    )

    expect(params.has('model')).toBe(false)
    expect(params.has('haikuModel')).toBe(false)
    expect(params.has('sonnetModel')).toBe(false)
    expect(params.has('opusModel')).toBe(false)
  })

  it('keeps Antigravity imports on the dedicated endpoint for any selected app', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'antigravity',
        app: 'gemini',
        model: 'gemini-3.1-pro-preview'
      })
    )

    expect(params.get('app')).toBe('gemini')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/antigravity`)
  })
})

describe('CC Switch usage script', () => {
  const usageBaseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: CC_SWITCH_USAGE_SCRIPT
  }

  function usageUrlFor(baseUrl: string): string {
    const script = CC_SWITCH_USAGE_SCRIPT.split('{{baseUrl}}').join(baseUrl).split('{{apiKey}}').join('sk-test')
    // eslint-disable-next-line no-new-func
    const config = new Function(`return ${script}`)() as { request: { url: string } }
    return config.request.url
  }

  it.each([
    'https://api.example.com',
    'https://api.example.com/',
    'https://api.example.com/v1',
    'https://api.example.com/v1/'
  ])('queries exactly one /v1/usage for base URL %s', (baseUrl) => {
    expect(usageUrlFor(baseUrl)).toBe('https://api.example.com/v1/usage')
  })

  it('works against the endpoint every platform import stores', () => {
    const appByPlatform: Record<string, 'claude' | 'codex' | 'gemini' | 'grokbuild'> = {
      anthropic: 'claude',
      openai: 'codex',
      grok: 'grokbuild',
      gemini: 'gemini'
    }
    for (const platform of ['anthropic', 'openai', 'grok', 'gemini'] as GroupPlatform[]) {
      const endpoint = paramsFromDeeplink(
        buildCcSwitchImportDeeplink({
          ...usageBaseInput,
          platform,
          app: appByPlatform[platform]
        })
      ).get('endpoint') as string
      expect(usageUrlFor(endpoint)).toBe('https://api.example.com/v1/usage')
    }
  })
})
