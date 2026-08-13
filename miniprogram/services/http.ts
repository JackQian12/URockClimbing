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

let refreshInFlight: Promise<boolean> | null = null

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
	return send<T>(path, options, true)
}

async function send<T>(path: string, options: RequestOptions, allowRefresh: boolean): Promise<T> {
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

	const response = await rawRequest(path, options, header)
	if (response.statusCode === 401 && allowRefresh && options.authenticated !== false && tokenStore.getRefreshToken()) {
		const refreshed = await refreshAccessTokenOnce()
		if (refreshed) return send<T>(path, options, false)
	}

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

async function rawRequest(path: string, options: RequestOptions, header: Record<string, string>): Promise<WechatMiniprogram.RequestSuccessCallbackResult> {
	return new Promise<WechatMiniprogram.RequestSuccessCallbackResult>((resolve, reject) => {
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
}

async function refreshAccessToken(): Promise<boolean> {
	const refreshToken = tokenStore.getRefreshToken()
	if (!refreshToken) return false
	try {
		const header = { 'Content-Type': 'application/json', 'X-Request-ID': createRequestId() }
		const response = await rawRequest('/auth/refresh', { method: 'POST', authenticated: false, data: { refresh_token: refreshToken } }, header)
		const envelope = response.data as ApiEnvelope<{ access_token: string; refresh_token: string }>
		if (response.statusCode < 200 || response.statusCode >= 300 || !envelope.data) {
			tokenStore.clear()
			return false
		}
		tokenStore.set(envelope.data.access_token, envelope.data.refresh_token)
		return true
	} catch {
		tokenStore.clear()
		return false
	}
}

function refreshAccessTokenOnce(): Promise<boolean> {
	if (refreshInFlight) return refreshInFlight
	refreshInFlight = refreshAccessToken().finally(() => {
		refreshInFlight = null
	})
	return refreshInFlight
}

function createRequestId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 12)}`
}
