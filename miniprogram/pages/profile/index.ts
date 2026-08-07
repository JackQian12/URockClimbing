import { loginWithWechat } from '../../services/auth'

Page({
  data: { loggingIn: false },
  async handleLogin() {
    if (this.data.loggingIn) return
    this.setData({ loggingIn: true })
    try {
      await loginWithWechat()
      wx.showToast({ title: '登录成功', icon: 'success' })
    } catch (error) {
      const message = error instanceof Error ? error.message : '登录失败，请重试'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ loggingIn: false })
    }
  },
})

