export type AdminUser = {
  id: string
  username: string
  nickname: string | null
  role: 'ADMIN'
  must_change_password: boolean
}

export type AdminSession = {
  user: AdminUser
  csrf_token: string
}

export type ApiEnvelope<T> = {
  data?: T
  error?: { code: string; message: string }
  request_id: string
}

export type CardProductStatus = 'DRAFT' | 'ON_SALE' | 'OFF_SALE'
export type ActivationMode = 'PURCHASE' | 'FIRST_USE'

export type CardProduct = {
  id: string
  name: string
  short_description: string
  description: string
  total_times: number
  validity_days: number
  activation_mode: ActivationMode
  price_cent: number
  list_price_cent: number | null
  purchase_limit: number | null
  daily_use_limit: number
  transferable: boolean
  rules: string
  badge: string | null
  theme_color: string
  status: CardProductStatus
  sort_order: number
  version: number
  created_at: string
  updated_at: string
}

export type CardProductInput = Omit<CardProduct,
  'id' | 'status' | 'created_at' | 'updated_at'
>

export type MemberStatus = 'ACTIVE' | 'DISABLED'

export type Member = {
  id: string
  member_no: string
  nickname: string | null
  avatar_url: string | null
  phone: string | null
  phone_authorized: boolean
  wechat_bound: boolean
  status: MemberStatus
  role: 'MEMBER'
  tags: string[]
  admin_note: string
  active_card_count: number
  remaining_times: number
  paid_order_count: number
  total_spent_cent: number
  redemption_count: number
  last_redemption_at: string | null
  last_login_at: string | null
  registered_at: string
  privacy_consent_version: string | null
  privacy_consent_at: string | null
  marketing_consent: boolean
  version: number
}

export type MemberList = {
  items: Member[]
  total: number
  page: number
  page_size: number
}

export type OrderStatus = 'PENDING' | 'PAID' | 'CLOSED' | 'REFUNDING' | 'REFUNDED'
export type OrderProductSnapshot = {
  name?: string
  product_name?: string
  total_times?: number
  validity_days?: number
  price_cent?: number
  [key: string]: unknown
}
export type AdminOrder = {
  id: string
  order_no: string
  status: OrderStatus
  biz_type: string
  total_amount_cent: number
  paid_amount_cent: number
  product_snapshot: OrderProductSnapshot
  member_id: string
  member_no: string
  member_nickname: string | null
  member_phone: string | null
  payment_status: string | null
  merchant_order_no: string | null
  provider_transaction_id: string | null
  card_no: string | null
  card_status: string | null
  card_total_times: number | null
  card_remaining_times: number | null
  card_expires_at: string | null
  redemption_count: number
  refund_no: string | null
  refund_status: string | null
  refund_reason: string | null
  refund_created_at: string | null
  refund_eligible: boolean
  refund_ineligible_reason: string
  expires_at: string
  paid_at: string | null
  closed_at: string | null
  created_at: string
  updated_at: string
}
export type AdminOrderList = { items: AdminOrder[]; total: number; page: number; page_size: number }

export type RedemptionRecord = { id:string; redemption_no:string; times:number; before_remaining:number; after_remaining:number; redeemed_at:string; request_id:string; member_id:string; member_no:string; member_nickname:string|null; member_phone:string|null; card_no:string; product_name:string; card_status:string; operator_id:string; operator_member_no:string; operator_nickname:string|null }
export type RedemptionList = { items:RedemptionRecord[]; total:number; page:number; page_size:number }
export type StaffUser = { id:string; member_no:string; nickname:string|null; avatar_url:string|null; phone:string|null; role:'MEMBER'|'STAFF'; status:MemberStatus; wechat_bound:boolean; last_login_at:string|null; created_at:string; version:number }
export type StaffList = { items:StaffUser[]; total:number; page:number; page_size:number }
export type AuditLog = { id:string; operator_id:string|null; operator_name:string|null; action:string; resource_type:string; resource_id:string; before:unknown|null; after:unknown|null; request_ip:string|null; request_id:string; created_at:string }
export type AuditLogList = { items:AuditLog[]; total:number; page:number; page_size:number }
