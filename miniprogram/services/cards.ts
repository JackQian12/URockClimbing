import { request } from './http'
import type { MemberCard, MemberRedemption } from '../types/api'

export function listMyCards(status?: string): Promise<{ items: MemberCard[]; total: number }> {
  const query = status ? `?status=${encodeURIComponent(status)}` : ''
  return request<{ items: MemberCard[]; total: number }>(`/me/cards${query}`)
}

export function getMyCard(id: string): Promise<MemberCard> {
  return request<MemberCard>(`/me/cards/${encodeURIComponent(id)}`)
}

export function listCardRedemptions(id: string): Promise<{ items: MemberRedemption[]; total: number }> {
  return request<{ items: MemberRedemption[]; total: number }>(`/me/cards/${encodeURIComponent(id)}/redemptions`)
}

export function listMyRedemptions(): Promise<{ items: MemberRedemption[]; total: number }> {
  return request<{ items: MemberRedemption[]; total: number }>('/me/redemptions')
}
