/**
 * Online Playground API.
 *
 * Unlike the rest of the app (which talks to /api/v1 with the session JWT via
 * the axios apiClient), the playground calls the **gateway** endpoints
 * `/v1/models` and `/v1/chat/completions` directly, authenticated with the
 * user's own API key as `Authorization: Bearer <key>`. We therefore use raw
 * fetch (axios would inject the session token and can't stream SSE cleanly).
 *
 * `/v1` is same-origin in production and proxied to the backend by Vite in dev
 * (see vite.config.ts), so a relative URL works in both.
 */

/** One OpenAI-format content part of a chat message. */
export type ChatContentPart =
  | { type: 'text'; text: string }
  | { type: 'image_url'; image_url: { url: string } }

export interface ChatMessage {
  role: 'system' | 'user' | 'assistant'
  content: string | ChatContentPart[]
}

export interface StreamCallbacks {
  onDelta: (text: string) => void
  onDone: () => void
  onError: (err: Error) => void
}

/** GET /v1/models with the user's key → list of model ids the key can use. */
export async function listPlaygroundModels(apiKey: string, signal?: AbortSignal): Promise<string[]> {
  const res = await fetch('/v1/models', {
    headers: { Authorization: `Bearer ${apiKey}` },
    signal,
  })
  if (!res.ok) {
    throw new Error(await extractError(res))
  }
  const data = await res.json()
  const list = Array.isArray(data?.data) ? data.data : []
  return list
    .map((m: { id?: unknown }) => (m && typeof m.id === 'string' ? m.id : ''))
    .filter((id: string) => id.length > 0)
}

/**
 * POST /v1/chat/completions with stream:true, parsing the SSE response and
 * forwarding each content delta to callbacks.onDelta. Resolves when the stream
 * finishes (onDone) or fails (onError). Pass an AbortSignal to stop early.
 */
export async function streamChatCompletion(
  apiKey: string,
  model: string,
  messages: ChatMessage[],
  callbacks: StreamCallbacks,
  signal?: AbortSignal,
): Promise<void> {
  let res: Response
  try {
    res = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${apiKey}`,
      },
      body: JSON.stringify({ model, messages, stream: true }),
      signal,
    })
  } catch (e) {
    if (isAbort(e)) return callbacks.onDone()
    return callbacks.onError(toError(e))
  }

  if (!res.ok || !res.body) {
    return callbacks.onError(new Error(await extractError(res)))
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      // SSE frames are separated by newlines; keep the trailing partial line.
      const lines = buffer.split('\n')
      buffer = lines.pop() ?? ''
      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed.startsWith('data:')) continue
        const payload = trimmed.slice(5).trim()
        if (payload === '[DONE]') {
          callbacks.onDone()
          return
        }
        try {
          const json = JSON.parse(payload)
          const delta = json?.choices?.[0]?.delta?.content
          if (typeof delta === 'string' && delta.length > 0) {
            callbacks.onDelta(delta)
          }
          const errMsg = json?.error?.message
          if (typeof errMsg === 'string' && errMsg.length > 0) {
            callbacks.onError(new Error(errMsg))
            return
          }
        } catch {
          // ignore keep-alive / malformed lines
        }
      }
    }
    callbacks.onDone()
  } catch (e) {
    if (isAbort(e)) return callbacks.onDone()
    callbacks.onError(toError(e))
  }
}

async function extractError(res: Response): Promise<string> {
  try {
    const data = await res.json()
    return data?.error?.message || data?.message || `HTTP ${res.status}`
  } catch {
    return `HTTP ${res.status}`
  }
}

function isAbort(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError'
}

function toError(e: unknown): Error {
  return e instanceof Error ? e : new Error(String(e))
}
