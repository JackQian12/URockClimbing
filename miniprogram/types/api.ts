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

export type MemberCardStatus = 'PENDING_ACTIVATION' | 'ACTIVE' | 'USED_UP' | 'EXPIRED' | 'REFUND_LOCKED' | 'REFUNDED'

export interface MemberCard {
  id: string
  card_no: string
  product_id: string
  product_name: string
  product_type: 'COUNT_CARD' | 'TIME_PASS'
  total_times: number | null
  remaining_times: number | null
  status: MemberCardStatus
  activated_at: string | null
  expires_at: string | null
  validity_days: number
  daily_use_limit: number
  redemption_count: number
  last_redemption_at: string | null
  created_at: string
}

export interface MemberRedemption {
  redemption_no: string
  times: number
  before_remaining: number | null
  after_remaining: number | null
  redeemed_at: string
  card_id: string
  card_no: string
  product_name: string
}

export interface RedemptionToken { token: string; expires_at: string }
export interface RedemptionPreview {
  member_no: string; nickname: string | null; phone_last4: string | null
  card_id: string; card_no: string; product_name: string; product_type: 'COUNT_CARD' | 'TIME_PASS'
  status: MemberCardStatus; remaining_times: number | null; activated_at: string | null; expires_at: string | null; token_expires_at: string
}
export interface RedemptionResult {
  redemption_no: string; card_id: string; card_no: string; product_name: string; product_type: 'COUNT_CARD' | 'TIME_PASS'
  before_remaining: number | null; after_remaining: number | null; card_status: MemberCardStatus
  activated_at: string | null; expires_at: string | null; redeemed_at: string
}
export interface StaffRedemption {
  redemption_no: string; card_no: string; product_name: string
  before_remaining: number | null; after_remaining: number | null; redeemed_at: string
}
