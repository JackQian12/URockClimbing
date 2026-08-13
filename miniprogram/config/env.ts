export type AppEnvironment = 'development' | 'trial' | 'production'

const baseUrls: Record<AppEnvironment, string> = {
  development: 'https://api.urockclimbing.cn/api/v1',
  trial: 'https://api.urockclimbing.cn/api/v1',
  production: 'https://api.urockclimbing.cn/api/v1',
}

const DEVELOPMENT_API_OVERRIDE_KEY = 'urock.development_api_base_url'

function currentEnvironment(): AppEnvironment {
  const envVersion = wx.getAccountInfoSync().miniProgram.envVersion
  if (envVersion === 'release') return 'production'
  if (envVersion === 'trial') return 'trial'
  return 'development'
}

export const appConfig = {
  environment: currentEnvironment(),
  get apiBaseUrl(): string {
	if (this.environment === 'development') {
	  const override = wx.getStorageSync<string>(DEVELOPMENT_API_OVERRIDE_KEY)
	  if (override) return override.replace(/\/$/, '')
	}
    return baseUrls[this.environment]
  },
  requestTimeoutMs: 10_000,
}
