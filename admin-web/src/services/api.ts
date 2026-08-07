import type { ApiEnvelope } from '../types/api'

let csrfToken = ''

export class ApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status: number,
    readonly requestId?: string,
  ) {
    super(message)
  }
}

export function setCsrfToken(value: string): void {
  csrfToken = value
}

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  headers.set('X-Request-ID', crypto.randomUUID())
  if (csrfToken && init.method && init.method !== 'GET') {
    headers.set('X-CSRF-Token', csrfToken)
  }

  const response = await fetch(`/api/v1${path}`, {
    ...init,
    headers,
    credentials: 'include',
  })
  const envelope = (await response.json()) as ApiEnvelope<T>
  if (!response.ok || envelope.error) {
    throw new ApiError(
      envelope.error?.code ?? 'UNKNOWN_ERROR',
      envelope.error?.message ?? '请求失败，请稍后重试',
      response.status,
      envelope.request_id,
    )
  }
  if (envelope.data === undefined) throw new ApiError('INVALID_RESPONSE', '服务器返回无效数据', response.status)
  return envelope.data
}

