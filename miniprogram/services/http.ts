import { appConfig } from '../config/env'
import { tokenStore } from '../store/token'
import type { ApiEnvelope } from '../types/api'

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly requestId?: string,
    public readonly statusCode?: number,
  ) {
    super(message)
  }
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  data?: WechatMiniprogram.IAnyObject
  authenticated?: boolean
  idempotencyKey?: string
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const token = tokenStore.getAccessToken()
  const header: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Request-ID': createRequestId(),
  }

  if (options.authenticated !== false && token) {
    header.Authorization = `Bearer ${token}`
  }
  if (options.idempotencyKey) {
    header['Idempotency-Key'] = options.idempotencyKey
  }

  const response = await new Promise<WechatMiniprogram.RequestSuccessCallbackResult>((resolve, reject) => {
    wx.request({
      url: `${appConfig.apiBaseUrl}${path}`,
      method: options.method ?? 'GET',
      data: options.data,
      header,
      timeout: appConfig.requestTimeoutMs,
      success: resolve,
      fail: reject,
    })
  }).catch(() => {
    throw new ApiError('NETWORK_ERROR', '网络连接失败，请稍后重试')
  })

  const envelope = response.data as ApiEnvelope<T>
  if (response.statusCode < 200 || response.statusCode >= 300 || envelope.error) {
    if (response.statusCode === 401) tokenStore.clear()
    throw new ApiError(
      envelope.error?.code ?? 'UNKNOWN_ERROR',
      envelope.error?.message ?? '请求失败，请稍后重试',
      envelope.request_id,
      response.statusCode,
    )
  }
  if (envelope.data === undefined) {
    throw new ApiError('INVALID_RESPONSE', '服务器返回了无效数据', envelope.request_id)
  }
  return envelope.data
}

function createRequestId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 12)}`
}
