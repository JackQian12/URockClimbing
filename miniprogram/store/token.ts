const ACCESS_TOKEN_KEY = 'urock.access_token'
const REFRESH_TOKEN_KEY = 'urock.refresh_token'

export const tokenStore = {
  getAccessToken(): string | null {
    return wx.getStorageSync<string>(ACCESS_TOKEN_KEY) || null
  },
  getRefreshToken(): string | null {
    return wx.getStorageSync<string>(REFRESH_TOKEN_KEY) || null
  },
  set(accessToken: string, refreshToken: string): void {
    wx.setStorageSync(ACCESS_TOKEN_KEY, accessToken)
    wx.setStorageSync(REFRESH_TOKEN_KEY, refreshToken)
    getApp<IAppOption>().globalData.accessToken = accessToken
  },
  clear(): void {
    wx.removeStorageSync(ACCESS_TOKEN_KEY)
    wx.removeStorageSync(REFRESH_TOKEN_KEY)
    const app = getApp<IAppOption>()
    if (app) app.globalData.accessToken = null
  },
}

