import { request } from './http'
import { tokenStore } from '../store/token'
import type { AuthTokens } from '../types/api'
import type { MemberProfile } from '../types/api'

export async function loginWithWechat(): Promise<AuthTokens> {
  const loginResult = await wx.login()
  if (!loginResult.code) throw new Error('微信登录失败，请重试')

  const tokens = await request<AuthTokens>('/auth/wechat/login', {
    method: 'POST',
    authenticated: false,
    data: { code: loginResult.code },
  })
  tokenStore.set(tokens.access_token, tokens.refresh_token)
  return tokens
}

export async function ensureWechatLogin(): Promise<void> {
  if (tokenStore.getAccessToken()) return
  await loginWithWechat()
}

export async function getMe(): Promise<MemberProfile> {
  await ensureWechatLogin()
  return request<MemberProfile>('/me')
}

export async function updateMe(nickname: string, version: number): Promise<MemberProfile> {
  return request<MemberProfile>('/me', { method: 'PUT', data: { nickname, version } })
}

export async function registerWithPhone(code: string): Promise<MemberProfile> {
  return request<MemberProfile>('/me/phone', { method: 'POST', data: { code } })
}

export async function logout(): Promise<void> {
  const refreshToken = tokenStore.getRefreshToken()
  try {
    if (refreshToken) {
      await request<{ logged_out: boolean }>('/auth/logout', {
        method: 'POST',
        authenticated: false,
        data: { refresh_token: refreshToken },
      })
    }
  } finally {
    tokenStore.clear()
  }
}
