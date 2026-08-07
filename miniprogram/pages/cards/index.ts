import { request } from '../../services/http'
import type { CardProduct } from '../../types/api'

type DisplayProduct = CardProduct & { priceYuan: string; unitPriceYuan: string }

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
      const result = await request<{ items: CardProduct[]; total: number }>('/card-products', { authenticated: false })
      this.setData({
        products: result.items.map((item) => ({
          ...item,
          priceYuan: (item.price_cent / 100).toFixed(item.price_cent % 100 ? 2 : 0),
          unitPriceYuan: (item.price_cent / item.total_times / 100).toFixed(0),
        })),
      })
    } catch (reason) {
      this.setData({ error: reason instanceof Error ? reason.message : '加载失败，请稍后重试' })
    } finally {
      this.setData({ loading: false })
    }
  },
})
