Page({
  data: {
    safeTop: 24,
    greeting: '周末好，岩友',
    cardName: '成人 10 次卡',
    remainingTimes: 7,
    expiresAt: '2026.12.31',
    lastVisit: '8月1日',
  },

  onLoad() {
    const windowInfo = wx.getWindowInfo()
    const hour = new Date().getHours()
    const greeting = hour < 11 ? '早上好，岩友' : hour < 18 ? '下午好，岩友' : '晚上好，岩友'
    this.setData({ safeTop: windowInfo.safeArea?.top ?? 24, greeting })
  },

  handleRedeem() {
    wx.showModal({
      title: '核销功能即将开放',
      content: '登录并持有有效次卡后，即可生成一次性核销码。',
      confirmText: '去登录',
      cancelText: '稍后',
      success: (result) => {
        if (result.confirm) wx.switchTab({ url: '/pages/profile/index' })
      },
    })
  },

  handleBuyCard() {
    wx.switchTab({ url: '/pages/cards/index' })
  },

  handleGymNews() {
    wx.showToast({ title: '新线路：抱石区 V2–V5', icon: 'none' })
  },

  onShareAppMessage() {
    return { title: 'U-Rock 遇岩 · 遇见向上的自己', path: '/pages/home/index' }
  },
})
