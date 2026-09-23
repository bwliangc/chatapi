import { describe, expect, it, vi } from 'vitest'
import { ReadableStream } from 'node:stream/web'
import { readDetectionStream } from '../modelDetection'

function stream(text: string, split = 3): globalThis.ReadableStream<Uint8Array> {
  const encoded = new TextEncoder().encode(text)
  return new ReadableStream<Uint8Array>({ start(controller) {
    for (let i = 0; i < encoded.length; i += split) controller.enqueue(encoded.slice(i, i + split))
    controller.close()
  } }) as globalThis.ReadableStream<Uint8Array>
}
describe('model detection stream', () => {
  it('handles split UTF-8, heartbeats and CRLF event boundaries', async () => {
    const receive = vi.fn()
    await readDetectionStream(stream(': keepalive\r\n\r\ndata: {"type":"progress","attempt":1,"accepted":0,"message":"正在采样"}\r\n\r\ndata: {"type":"result","data":{"model":"gpt-test"}}\r\n\r\n', 1), receive)
    expect(receive).toHaveBeenCalledTimes(2)
    expect(receive.mock.calls[0][0].message).toBe('正在采样')
  })
  it('rejects an EOF without a completed result', async () => {
    await expect(readDetectionStream(stream('data: {"type":"progress","attempt":1,"accepted":0}\n\n'), vi.fn())).rejects.toThrow('提前结束')
  })
  it('surfaces backend errors and malformed events', async () => {
    await expect(readDetectionStream(stream('data: {"type":"error","message":"功能已关闭"}\n\n'), vi.fn())).rejects.toThrow('功能已关闭')
    await expect(readDetectionStream(stream('data: broken\n\n'), vi.fn())).rejects.toThrow()
  })
})
