import { describe, expect, it } from 'vitest'
import {
  ACCOUNT_PLATFORM_OPTIONS,
  CONCRETE_PLATFORM_OPTIONS,
  GROUP_PLATFORM_OPTIONS
} from '@/constants/platforms'

const concretePlatforms = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek'
]
const accountPlatforms = [
  ...concretePlatforms.slice(0, 5),
  'custom',
  ...concretePlatforms.slice(5)
]

describe('platform option catalogs', () => {
  it('exposes every concrete account platform', () => {
    expect(CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(concretePlatforms)
  })

  it('includes custom OpenAI-compatible accounts', () => {
    expect(ACCOUNT_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(accountPlatforms)
  })

  it('adds composite for group-backed filters', () => {
    expect(GROUP_PLATFORM_OPTIONS.map((option) => option.value)).toEqual([
      ...accountPlatforms,
      'composite'
    ])
  })
})
