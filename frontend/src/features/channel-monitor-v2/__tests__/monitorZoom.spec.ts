import { describe, expect, it } from 'vitest'
import {
  DEFAULT_ZOOM,
  MIN_ZOOM_SPAN,
  clampZoom,
  handleMonitorWheel,
  isZoomed,
  resetZoom,
  sliceByZoom,
  zoomAt,
} from '../monitorZoom'

describe('monitorZoom', () => {
  it('resets to full span', () => {
    expect(resetZoom()).toEqual(DEFAULT_ZOOM)
    expect(isZoomed(DEFAULT_ZOOM)).toBe(false)
  })

  it('zooms in around cursor without always pinning to the end', () => {
    const mid = zoomAt(DEFAULT_ZOOM, 'in')
    expect(mid.span).toBeLessThan(1)
    expect(mid.span).toBeGreaterThanOrEqual(MIN_ZOOM_SPAN)
    // Center zoom keeps window roughly centered
    expect(mid.offset).toBeGreaterThan(0)
    expect(mid.offset + mid.span).toBeLessThanOrEqual(1.001)
    expect(zoomAt(mid, 'out')).toEqual(DEFAULT_ZOOM)

    const left = zoomAt(DEFAULT_ZOOM, 'in', 0)
    expect(left.offset).toBeLessThanOrEqual(0.001)

    const right = zoomAt(DEFAULT_ZOOM, 'in', 1)
    expect(right.offset + right.span).toBeGreaterThan(0.99)
  })

  it.each([DEFAULT_ZOOM, { span: 0.4, offset: 0.1 }])(
    'keeps vertical trackpad gestures native at $span zoom',
    (state) => {
      for (const deltaY of [-100, 100]) {
        const event = new WheelEvent('wheel', { deltaY, deltaX: 2, cancelable: true })
        expect(handleMonitorWheel(state, event, null)).toBe(state)
        expect(event.defaultPrevented).toBe(false)
      }
    },
  )

  it('zooms deliberately with Ctrl or Command, but not on horizontal motion', () => {
    for (const modifier of ['ctrlKey', 'metaKey'] as const) {
      const event = new WheelEvent('wheel', { deltaY: -100, [modifier]: true, cancelable: true })
      expect(handleMonitorWheel(DEFAULT_ZOOM, event, null).span).toBeLessThan(1)
      expect(event.defaultPrevented).toBe(true)
    }
    const horizontal = new WheelEvent('wheel', { deltaX: 100, deltaY: -3, ctrlKey: true, cancelable: true })
    expect(handleMonitorWheel(DEFAULT_ZOOM, horizontal, null)).toBe(DEFAULT_ZOOM)
    expect(horizontal.defaultPrevented).toBe(false)
  })

  it('pans the zoomed timeline when a horizontal gesture can move it', () => {
    const zoomed = clampZoom({ span: 0.4, offset: 0.1 })
    const shift = new WheelEvent('wheel', { deltaX: 200, shiftKey: true, cancelable: true })
    const horizontal = new WheelEvent('wheel', { deltaX: 200, deltaY: 5, cancelable: true })
    for (const event of [shift, horizontal]) {
      const panned = handleMonitorWheel(zoomed, event, null)
      expect(panned.span).toBe(zoomed.span)
      expect(panned.offset).toBeGreaterThan(zoomed.offset)
      expect(event.defaultPrevented).toBe(true)
    }

    const atEnd = { span: 0.4, offset: 0.6 }
    const beyondEnd = new WheelEvent('wheel', { deltaX: 200, cancelable: true })
    expect(handleMonitorWheel(atEnd, beyondEnd, null)).toBe(atEnd)
    expect(beyondEnd.defaultPrevented).toBe(false)
  })

  it('lets the matrix scroll horizontally through its visible columns first', () => {
    const scroller = document.createElement('div')
    Object.defineProperties(scroller, { scrollWidth: { value: 600 }, clientWidth: { value: 450 } })
    const zoomed = { span: 0.4, offset: 0.1 }
    const scroll = new WheelEvent('wheel', { deltaX: 80, cancelable: true })
    expect(handleMonitorWheel(zoomed, scroll, null, scroller)).toBe(zoomed)
    expect(scroll.defaultPrevented).toBe(false)

    scroller.scrollLeft = 150
    const pan = new WheelEvent('wheel', { deltaX: 80, cancelable: true })
    expect(handleMonitorWheel(zoomed, pan, null, scroller).offset).toBeGreaterThan(zoomed.offset)
    expect(pan.defaultPrevented).toBe(true)
  })

  it('clamps span to MIN_ZOOM_SPAN and keeps window in range', () => {
    const tight = clampZoom({ span: 0.01, offset: 0.9 })
    expect(tight.span).toBe(MIN_ZOOM_SPAN)
    expect(tight.offset + tight.span).toBeLessThanOrEqual(1.001)
  })

  it('slices series by window, not always last N points', () => {
    const items = Array.from({ length: 100 }, (_, i) => i)
    const startWindow = sliceByZoom(items, { span: 0.2, offset: 0 })
    expect(startWindow[0]).toBe(0)
    expect(startWindow.length).toBe(20)

    const midWindow = sliceByZoom(items, { span: 0.2, offset: 0.4 })
    expect(midWindow[0]).toBe(40)
    expect(midWindow[midWindow.length - 1]).toBe(59)

    const full = sliceByZoom(items, DEFAULT_ZOOM)
    expect(full).toHaveLength(100)
  })
})
