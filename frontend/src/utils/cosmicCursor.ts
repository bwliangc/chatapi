/**
 * 黑/白洞鼠标指针特效的开关状态（本地持久化 + 跨模块通知）。
 * main.ts 的特效引擎订阅变更；侧边栏左下角的开关按钮读写状态。
 */

const STORAGE_KEY = 'cosmicCursorEnabled'

type CosmicCursorListener = (enabled: boolean) => void

const listeners = new Set<CosmicCursorListener>()

/** 设备是否支持该特效（精确指针且未开启「减少动态效果」）。触屏设备上开关无意义，应隐藏。 */
export function isCosmicCursorSupported(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  return (
    window.matchMedia('(pointer: fine)').matches &&
    !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

/** 默认开启；仅当用户显式关闭过（存储为 'false'）时返回 false。 */
export function isCosmicCursorEnabled(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) !== 'false'
  } catch {
    return true
  }
}

export function setCosmicCursorEnabled(enabled: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, String(enabled))
  } catch {
    // 隐私模式等场景下存储不可用，仅在当前会话生效
  }
  for (const listener of listeners) listener(enabled)
}

export function onCosmicCursorEnabledChange(listener: CosmicCursorListener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}
