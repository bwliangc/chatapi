import { describe, expect, it, vi } from 'vitest'
import { applyPwaBranding, initPwaInstall, usePwaInstall } from '../pwa'

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

  it('updates browser and installed-app names from site settings', () => {
    document.head.innerHTML = `
      <link rel="icon" href="/pwa/icon-192.png">
      <meta name="apple-mobile-web-app-title" content="Sub2API">
      <meta name="application-name" content="Sub2API">
    `

    applyPwaBranding('Configured Name', 'data:image/png;base64,configured-logo')

    expect(document.querySelector<HTMLMetaElement>('meta[name="apple-mobile-web-app-title"]')?.content)
      .toBe('Configured Name')
    expect(document.querySelector<HTMLMetaElement>('meta[name="application-name"]')?.content)
      .toBe('Configured Name')
    expect(document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.href)
      .toBe('data:image/png;base64,configured-logo')
  })

  it('restores the generated default icon when the configured logo is removed', () => {
    document.head.innerHTML = '<link rel="icon" href="data:image/png;base64,old-logo">'

    applyPwaBranding('Configured Name', '')

    expect(document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.getAttribute('href'))
      .toBe('/pwa/icon-192.png')
  })
})
