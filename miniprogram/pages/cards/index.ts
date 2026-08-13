import { request } from '../../services/http'
import type { CardProduct } from '../../types/api'
import { ensureWechatLogin, getMe } from '../../services/auth'

type DisplayProduct = CardProduct & {
  priceYuan: string
  benefitValue: string
  benefitUnit: string
  priceHint: string
  validityLabel: string
}

const passNames: Record<number, string> = { 7: '周卡', 30: '月卡', 90: '季卡', 365: '年卡' }

Page({
  data: {
    products: [] as DisplayProduct[],
    loading: true,
    error: '',
  },

  onLoad() {
    void this.loadProducts()
  },

  onPullDownRefresh() {
    void this.loadProducts().finally(() => wx.stopPullDownRefresh())
  },

  async loadProducts() {
    this.setData({ loading: true, error: '' })
    try {
      await ensureWechatLogin()
	  const profile = await getMe()
	  if (!profile.registered) {
		wx.showToast({ title: '请先授权手机号完成注册', icon: 'none' })
		setTimeout(() => wx.switchTab({ url: '/pages/profile/index' }), 500)
		this.setData({ products: [] })
		return
	  }
      const result = await request<{ items: CardProduct[]; total: number }>('/card-products')
      this.setData({
        products: result.items.map((item) => ({
          ...item,
          priceYuan: (item.price_cent / 100).toFixed(item.price_cent % 100 ? 2 : 0),
          benefitValue: item.product_type === 'TIME_PASS' ? '不限' : String(item.total_times),
          benefitUnit: item.product_type === 'TIME_PASS' ? '次' : '次',
          priceHint: item.product_type === 'TIME_PASS' || !item.total_times
            ? '有效期内不限次'
            : `约 ¥${(item.price_cent / item.total_times / 100).toFixed(0)}/次`,
          validityLabel: item.product_type === 'TIME_PASS'
            ? `${passNames[item.validity_days] || `${item.validity_days} 天卡`} · 首次使用起算`
            : `${item.validity_days} 天有效`,
        })),
      })
    } catch (reason) {
      this.setData({ error: reason instanceof Error ? reason.message : '加载失败，请稍后重试' })
    } finally {
      this.setData({ loading: false })
    }
  },
})
