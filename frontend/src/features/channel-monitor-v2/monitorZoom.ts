/**
 * Shared X-axis zoom/pan for the channel monitor pulse matrix and line chart.
 * No chartjs-plugin-zoom dependency — state is span + offset fractions.
 */

export interface ZoomState {
  /** Visible fraction of the full series (1 = full range). */
  span: number
  /** Start of the visible window as a fraction of the full domain [0, 1 - span]. */
  offset: number
}

export const DEFAULT_ZOOM: ZoomState = { span: 1, offset: 0 }
export const MIN_ZOOM_SPAN = 0.12

const ZOOM_FACTOR = 0.88

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

export function resetZoom(): ZoomState {
  return { ...DEFAULT_ZOOM }
}

export function isZoomed(state: ZoomState): boolean {
  return state.span < 0.999 || state.offset > 0.001
}

/** Normalize offset so the visible window stays within [0, 1]. */
export function clampZoom(state: ZoomState): ZoomState {
  const span = clamp(state.span, MIN_ZOOM_SPAN, 1)
  const maxOffset = Math.max(0, 1 - span)
  return {
    span,
    offset: clamp(state.offset, 0, maxOffset),
  }
}

export function zoomAt(state: ZoomState, direction: 'in' | 'out', xRatio = 0.5): ZoomState {
  const ratio = clamp(Number.isFinite(xRatio) ? xRatio : 0.5, 0, 1)
  const nextSpan = clamp(direction === 'in' ? state.span * ZOOM_FACTOR : state.span / ZOOM_FACTOR, MIN_ZOOM_SPAN, 1)
  if (nextSpan === state.span) return state

  // Keep the point under the cursor fixed in data space.
  return clampZoom({
    span: nextSpan,
    offset: state.offset + ratio * (state.span - nextSpan),
  })
}

export function handleMonitorWheel(
  state: ZoomState,
  event: WheelEvent,
  plot: HTMLElement | null,
  scroller?: HTMLElement | null,
): ZoomState {
  if (!event.cancelable || event.defaultPrevented) return state
  const { deltaX, deltaY, shiftKey, ctrlKey, metaKey } = event
  const horizontal = Math.abs(deltaX) > Math.abs(deltaY)

  if (ctrlKey || metaKey) {
    if (horizontal || !deltaY) return state
    event.preventDefault()
    return zoomAt(state, deltaY < 0 ? 'in' : 'out', clientXRatio(event.clientX, plot))
  }

  // Let ordinary vertical gestures scroll the matrix rows or the page.
  const panDelta = shiftKey ? (deltaX || deltaY) : horizontal ? deltaX : 0
  if (!panDelta || !isZoomed(state)) return state

  // The matrix can be wider than its viewport. Scroll the visible columns
  // first, then pan the time window at the edge.
  if (scroller) {
    const maxScrollLeft = scroller.scrollWidth - scroller.clientWidth
    const canScroll = panDelta < 0 ? scroller.scrollLeft > 0 : scroller.scrollLeft < maxScrollLeft - 1
    if (canScroll) return state
  }

  const next = clampZoom({
    span: state.span,
    offset: state.offset + (panDelta / 400) * state.span,
  })
  if (next.offset === state.offset) return state
  event.preventDefault()
  return next
}

/**
 * Slice an array using zoom state. Cursor-centered windowing — not always "last N".
 */
export function sliceByZoom<T>(items: readonly T[], state: ZoomState): T[] {
  if (!items.length) return []
  const zoom = clampZoom(state)
  if (zoom.span >= 0.999) return [...items]

  const count = Math.max(1, Math.ceil(items.length * zoom.span))
  const maxStart = Math.max(0, items.length - count)
  // offset is window start in [0, 1-span] domain → map to index
  const start = Math.min(maxStart, Math.max(0, Math.round(zoom.offset * items.length)))
  return items.slice(start, start + count)
}

/** Map a client X position within an element to 0–1 ratio. */
function clientXRatio(clientX: number, element: HTMLElement | null): number {
  if (!element) return 0.5
  const rect = element.getBoundingClientRect()
  if (rect.width <= 0) return 0.5
  return clamp((clientX - rect.left) / rect.width, 0, 1)
}
