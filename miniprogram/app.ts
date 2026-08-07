import { tokenStore } from './store/token'

App<IAppOption>({
  globalData: {
    accessToken: tokenStore.getAccessToken(),
  },
})

