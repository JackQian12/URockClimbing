export interface ApiErrorBody {
  code: string
  message: string
}

export interface ApiEnvelope<T> {
  data?: T
  error?: ApiErrorBody
  request_id: string
}

export interface AuthTokens {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface MemberProfile {
  id: string
  member_no: string
  nickname: string | null
  avatar_url: string | null
  role: 'MEMBER' | 'STAFF' | 'ADMIN'
  status: 'ACTIVE' | 'DISABLED'
  version: number
  registered: boolean
  phone_masked: string | null
}

export interface CardProduct {
  id: string
  name: string
  short_description: string
  description: string
  product_type: 'COUNT_CARD' | 'TIME_PASS'
  total_times: number | null
  validity_days: number
  activation_mode: 'PURCHASE' | 'FIRST_USE'
  price_cent: number
  list_price_cent: number | null
  purchase_limit: number | null
  daily_use_limit: number
  transferable: boolean
  rules: string
  badge: string | null
  theme_color: string
}

export type OrderStatus = 'PENDING' | 'PAID' | 'CLOSED' | 'REFUNDING' | 'REFUNDED'

export interface OrderProductSnapshot {
  product_id: string
  name: string
  product_type: 'COUNT_CARD' | 'TIME_PASS'
  total_times: number | null
  validity_days: number
  activation_mode: 'PURCHASE' | 'FIRST_USE'
  price_cent: number
  daily_use_limit: number
  transferable: boolean
}

export interface MemberOrder {
  order_no: string
  status: OrderStatus
  biz_type: 'CARD_PURCHASE'
  total_amount_cent: number
  paid_amount_cent: number
  product_snapshot: OrderProductSnapshot
  expires_at: string
  paid_at: string | null
  closed_at: string | null
  created_at: string
  updated_at: string
}

export interface WechatPayParameters {
  timeStamp: string
  nonceStr: string
  package: string
  signType: 'RSA'
  paySign: string
}
