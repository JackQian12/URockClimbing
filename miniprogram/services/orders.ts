import { request } from './http'
import type { MemberOrder, WechatPayParameters } from '../types/api'

export function createOrder(cardProductId: string, idempotencyKey: string): Promise<MemberOrder> {
  return request<MemberOrder>('/orders', {
    method: 'POST',
    idempotencyKey,
    data: { card_product_id: cardProductId },
  })
}

export function listOrders(page = 1): Promise<{ items: MemberOrder[]; total: number }> {
  return request<{ items: MemberOrder[]; total: number }>(`/me/orders?page=${page}&page_size=20`)
}

export function getOrder(orderNo: string): Promise<MemberOrder> {
  return request<MemberOrder>(`/me/orders/${encodeURIComponent(orderNo)}`)
}

export function prepareWechatPay(orderNo: string): Promise<WechatPayParameters> {
  return request<WechatPayParameters>(`/orders/${encodeURIComponent(orderNo)}/wechat-pay`, { method: 'POST' })
}

export function syncWechatPay(orderNo: string): Promise<{ trade_state: string }> {
  return request<{ trade_state: string }>(`/orders/${encodeURIComponent(orderNo)}/sync`, { method: 'POST' })
}

export function createIdempotencyKey(): string {
  return `order-${Date.now()}-${Math.random().toString(36).slice(2, 14)}`
}
