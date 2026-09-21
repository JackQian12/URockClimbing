import { request } from './http'
import type { CheckinPreview, CheckinResult, RedemptionPreview, RedemptionResult, RedemptionToken, StaffRedemption } from '../types/api'

let pendingScannedToken = ''

export function createRedemptionToken(cardId: string): Promise<RedemptionToken> {
  return request<RedemptionToken>(`/me/cards/${encodeURIComponent(cardId)}/redemption-token`, { method: 'POST' })
}
export function previewRedemption(token: string): Promise<RedemptionPreview> {
  return request<RedemptionPreview>('/staff/redemptions/preview', { method: 'POST', data: { token } })
}
export function confirmRedemption(token: string, idempotencyKey: string): Promise<RedemptionResult> {
  return request<RedemptionResult>('/staff/redemptions/confirm', { method: 'POST', data: { token }, idempotencyKey })
}
export function listTodayRedemptions(): Promise<{items: StaffRedemption[]; total: number}> {
  return request<{items: StaffRedemption[]; total: number}>('/staff/redemptions/today')
}
export function setPendingScannedToken(token: string) { pendingScannedToken = token }
export function takePendingScannedToken(): string { const token = pendingScannedToken; pendingScannedToken = ''; return token }
export function createRedemptionRequestID(): string { return `redeem-${Date.now()}-${Math.random().toString(36).slice(2, 14)}` }
export function previewCheckin(token: string): Promise<CheckinPreview> {
  return request<CheckinPreview>('/me/checkins/preview', { method: 'POST', data: { token } })
}
export function confirmCheckin(token: string, cardId: string, idempotencyKey: string): Promise<CheckinResult> {
  return request<CheckinResult>('/me/checkins/confirm', { method: 'POST', data: { token, card_id: cardId }, idempotencyKey })
}
