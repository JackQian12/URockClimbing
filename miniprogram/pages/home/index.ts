import { tokenStore } from '../../store/token'
import { getMe } from '../../services/auth'
import { listMyCards } from '../../services/cards'
import type { MemberCard } from '../../types/api'

Page({
  data: {
    safeTop: 24,
    greeting: '周末好，岩友',
	isLoggedIn: false,
	isRegistered: false,
	primaryCard: null as MemberCard | null,
	cardBenefit: '',
	cardMeta: '',
  },

  onLoad() {
    const windowInfo = wx.getWindowInfo()
    const hour = new Date().getHours()
    const greeting = hour < 11 ? '早上好，岩友' : hour < 18 ? '下午好，岩友' : '晚上好，岩友'
    this.setData({ safeTop: windowInfo.safeArea?.top ?? 24, greeting })
  },

	async onShow() {
		const isLoggedIn = Boolean(tokenStore.getAccessToken())
		this.setData({ isLoggedIn, isRegistered: false })
		if (!isLoggedIn) return
		try {
			const profile = await getMe()
			this.setData({ isRegistered: profile.registered, primaryCard: null })
			if (!profile.registered) return
			try {
				const result = await listMyCards()
				const card = result.items.find((item) => item.status === 'ACTIVE' || item.status === 'PENDING_ACTIVATION') || null
				this.setData({ primaryCard: card, cardBenefit: card ? (card.product_type === 'COUNT_CARD' ? `${card.remaining_times ?? 0} 次` : `${card.validity_days} 天畅爬`) : '', cardMeta: card ? (card.status === 'PENDING_ACTIVATION' ? '首次使用后激活' : card.expires_at ? `有效至 ${new Date(card.expires_at).toLocaleDateString('zh-CN')}` : '') : '' })
			} catch { this.setData({ primaryCard: null }) }
		} catch {
			this.setData({ isLoggedIn: Boolean(tokenStore.getAccessToken()), isRegistered: false })
		}
	},

	handleLogin() {
		wx.switchTab({ url: '/pages/profile/index' })
	},

  handleRedeem() {
	if (!this.data.isRegistered) {
		this.handleLogin()
		return
	}
	  const card = this.data.primaryCard
	  if (!card) { wx.navigateTo({ url: '/pages/my-cards/index' }); return }
	  wx.navigateTo({ url: `/pages/my-cards/detail?id=${encodeURIComponent(card.id)}` })
  },

  handleBuyCard() {
    wx.switchTab({ url: '/pages/cards/index' })
  },

	handleMyCards() {
		wx.navigateTo({ url: '/pages/my-cards/index' })
	},

  handleGymNews() {
    wx.showToast({ title: '新线路：抱石区 V2–V5', icon: 'none' })
  },

  onShareAppMessage() {
    return { title: 'U-Rock 遇岩 · 遇见向上的自己', path: '/pages/home/index' }
  },
})
