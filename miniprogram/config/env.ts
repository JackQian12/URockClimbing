export type AppEnvironment = 'development' | 'trial' | 'production'

const baseUrls: Record<AppEnvironment, string> = {
  development: 'http://127.0.0.1:8080/api/v1',
  trial: 'https://api.urockclimbing.cn/api/v1',
  production: 'https://api.urockclimbing.cn/api/v1',
}

function currentEnvironment(): AppEnvironment {
  const envVersion = wx.getAccountInfoSync().miniProgram.envVersion
  if (envVersion === 'release') return 'production'
  if (envVersion === 'trial') return 'trial'
  return 'development'
}

export const appConfig = {
  environment: currentEnvironment(),
  get apiBaseUrl(): string {
    return baseUrls[this.environment]
  },
  requestTimeoutMs: 10_000,
}
