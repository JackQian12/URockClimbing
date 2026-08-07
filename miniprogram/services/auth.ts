import { request } from './http'
import { tokenStore } from '../store/token'
import type { AuthTokens } from '../types/api'

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

