/* eslint-env serviceworker */

const CACHE_NAME = 'sub2api-pwa-v2'
const APP_SHELL_KEY = '/__pwa_shell__'
const PRECACHE_URLS = ['/logo.png']

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting())
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((names) => Promise.all(
        names.filter((name) => name !== CACHE_NAME).map((name) => caches.delete(name))
      ))
      .then(() => self.clients.claim())
  )
})

function isApiRequest(pathname) {
  return pathname === '/setup' ||
    pathname.startsWith('/setup/') ||
    pathname.startsWith('/api/') ||
    pathname.startsWith('/v1/')
}

function isStaticAsset(pathname) {
  return pathname.startsWith('/assets/') ||
    pathname.startsWith('/icons/') ||
    pathname === '/logo.png'
}

async function handleNavigation(request) {
  try {
    const response = await fetch(request)
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME)
      await cache.put(APP_SHELL_KEY, response.clone())
    }
    return response
  } catch {
    return (await caches.match(APP_SHELL_KEY)) || Response.error()
  }
}

async function handleStaticAsset(request) {
  const cached = await caches.match(request)
  if (cached) return cached

  const response = await fetch(request)
  if (response.ok) {
    const cache = await caches.open(CACHE_NAME)
    await cache.put(request, response.clone())
  }
  return response
}

self.addEventListener('fetch', (event) => {
  const { request } = event
  if (request.method !== 'GET') return

  const url = new URL(request.url)
  if (
    url.origin !== self.location.origin ||
    isApiRequest(url.pathname) ||
    url.pathname.startsWith('/pwa/') ||
    url.pathname === '/manifest.webmanifest'
  ) return

  if (request.mode === 'navigate') {
    event.respondWith(handleNavigation(request))
    return
  }

  if (isStaticAsset(url.pathname)) {
    event.respondWith(handleStaticAsset(request))
  }
})
