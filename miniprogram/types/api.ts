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
}

export interface CardProduct {
  id: string
  name: string
  short_description: string
  description: string
  total_times: number
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
