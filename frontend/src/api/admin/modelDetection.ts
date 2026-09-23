import apiClient, { buildApiUrl } from '../client'
import { ADMIN_UI_REQUEST_HEADER } from '../adminUIRequest'

export interface DetectionInfo { models: string[]; bank_sha256: string; bank_built_at: string }
export interface DetectionResult {
  account_id: string
  model: string
  mapped_model: string
  actual_models: string[]
  attempts: number
  verdict: { status: string; reason: string }
  result: null | {
    prediction: string
    probability: number
    used_outputs: number
    bank_sha256: string
    bank_built_at: string
    results: { model: string; probability: number; score: number }[]
  }
}
export type DetectionEvent =
  | { type: 'progress'; attempt: number; accepted: number; in_flight?: number; message?: string }
  | { type: 'result'; data: DetectionResult }
  | { type: 'error'; message: string }

export async function getDetectionInfo(): Promise<DetectionInfo> {
  return (await apiClient.get<DetectionInfo>('/admin/model-detection')).data
}

// Parse event boundaries rather than assuming network chunks align with UTF-8
// characters or SSE lines. An EOF without a result is not a successful test.
export async function readDetectionStream(body: ReadableStream<Uint8Array>, onEvent: (event: DetectionEvent) => void): Promise<void> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let completed = false
  const consume = (frame: string) => {
    const raw = frame.split('\n').filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart()).join('\n')
    if (!raw) return
    const event = JSON.parse(raw) as DetectionEvent
    if (event.type === 'error') throw new Error(event.message)
    if (event.type === 'result') completed = true
    if (event.type === 'progress' || event.type === 'result') onEvent(event)
  }
  try {
    while (true) {
      const { done, value } = await reader.read()
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true })
      buffer = buffer.replace(/\r\n/g, '\n')
      let boundary: number
      while ((boundary = buffer.indexOf('\n\n')) !== -1) {
        consume(buffer.slice(0, boundary))
        buffer = buffer.slice(boundary + 2)
      }
      if (buffer.length > 512 * 1024) throw new Error('检测响应过大')
      if (done) break
    }
    if (!completed) throw new Error('检测连接提前结束，请重新测试')
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}

export async function detectAccountModel(accountID: number, model: string, signal: AbortSignal, onEvent: (event: DetectionEvent) => void): Promise<void> {
  const response = await fetch(buildApiUrl(`/admin/accounts/${accountID}/model-detection`), {
    method: 'POST',
    headers: { Authorization: `Bearer ${localStorage.getItem('auth_token')}`, 'Content-Type': 'application/json', [ADMIN_UI_REQUEST_HEADER]: '1' },
    body: JSON.stringify({ model }),
    signal
  })
  if (!response.ok) {
    const error = await response.json().catch(() => ({}))
    throw new Error(error.message || `HTTP ${response.status}`)
  }
  if (!response.body) throw new Error('检测响应为空')
  await readDetectionStream(response.body, onEvent)
}

export interface BankVersion {
  commit: string
  sha256: string
  algorithm_sha256: string
  built_at: string
  model_count: number
  installed_at?: string
}
export interface BankStatus {
  current: BankVersion
  previous?: BankVersion
  revision: string
  source: 'embedded' | 'database'
  warning?: string
}
export interface BankCheck {
  status: BankStatus
  remote?: BankVersion
  compatible: boolean
  update_available: boolean
  message?: string
}
export async function getBankStatus(): Promise<BankStatus> {
  return (await apiClient.get<BankStatus>('/admin/model-detection/bank')).data
}
export async function checkBankUpdate(): Promise<BankCheck> {
  return (await apiClient.post<BankCheck>('/admin/model-detection/bank/check')).data
}
export async function updateBank(commit: string, revision: string): Promise<BankStatus> {
  return (await apiClient.post<BankStatus>('/admin/model-detection/bank/update', { commit, revision })).data
}
export async function rollbackBank(revision: string): Promise<BankStatus> {
  return (await apiClient.post<BankStatus>('/admin/model-detection/bank/rollback', { revision })).data
}
