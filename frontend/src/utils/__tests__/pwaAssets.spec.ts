import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const frontendRoot = resolve(__dirname, '../../..')

describe('PWA assets', () => {
  it('declares an installable standalone web app', () => {
    const manifest = JSON.parse(
      readFileSync(resolve(frontendRoot, 'public/manifest.webmanifest'), 'utf8')
    )

    expect(manifest.display).toBe('standalone')
    expect(manifest.start_url).toBe('/home?source=pwa')
    expect(manifest.icons).toEqual(expect.arrayContaining([
      expect.objectContaining({ sizes: '192x192' }),
      expect.objectContaining({ sizes: '512x512', purpose: 'maskable' })
    ]))
  })

  it('keeps authenticated APIs out of the service worker cache', () => {
    const serviceWorker = readFileSync(resolve(frontendRoot, 'public/sw.js'), 'utf8')

    expect(serviceWorker).toContain("pathname.startsWith('/api/')")
    expect(serviceWorker).toContain("pathname.startsWith('/v1/')")
    expect(serviceWorker).toContain("pathname.startsWith('/setup/')")
  })
})
