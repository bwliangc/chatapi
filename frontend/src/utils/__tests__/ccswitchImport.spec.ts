import { describe, expect, it } from 'vitest'
import {
  CC_SWITCH_DEFAULT_MODELS,
  buildCcSwitchImportDeeplink
} from '@/utils/ccswitchImport'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('ccswitchImport utils', () => {
  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it('keeps the current Codex model as the dialog default', () => {
    expect(CC_SWITCH_DEFAULT_MODELS.codex).toBe('gpt-5.5')
  })

  it.each([
    { app: 'claude' as const, model: 'claude-sonnet-4-6' },
    { app: 'codex' as const, model: 'gpt-5.5' },
    { app: 'gemini' as const, model: 'gemini-3.1-pro-preview' }
  ])('uses the selected $app app and primary model', ({ app, model }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        app,
        model
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.get('model')).toBe(model)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
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
