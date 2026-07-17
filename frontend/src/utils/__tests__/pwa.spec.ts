import { describe, expect, it, vi } from 'vitest'
import { initPwaInstall, usePwaInstall } from '../pwa'

describe('PWA installation', () => {
  it('captures and consumes the Android install prompt', async () => {
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
    const prompt = vi.fn().mockResolvedValue(undefined)
    const event = new Event('beforeinstallprompt', { cancelable: true }) as Event & {
      prompt: () => Promise<void>
      userChoice: Promise<{ outcome: 'accepted'; platform: string }>
    }
    event.prompt = prompt
    event.userChoice = Promise.resolve({ outcome: 'accepted', platform: 'web' })

    initPwaInstall()
    window.dispatchEvent(event)

    const pwa = usePwaInstall()
    expect(event.defaultPrevented).toBe(true)
    expect(pwa.canInstall.value).toBe(true)
    await expect(pwa.installApp()).resolves.toBe(true)
    expect(prompt).toHaveBeenCalledOnce()
    expect(pwa.canInstall.value).toBe(false)
  })
})
