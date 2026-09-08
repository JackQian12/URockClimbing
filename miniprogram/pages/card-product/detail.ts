import { appConfig } from '../../config/env'
import { request } from '../../services/http'
import { createIdempotencyKey, createOrder, prepareWechatPay, syncWechatPay } from '../../services/orders'
import type { CardProduct } from '../../types/api'
import { formatCent } from '../../utils/money'

Page({
  data: {
    product: null as CardProduct | null,
    price: '',
    benefit: '',
    validity: '',
    loading: true,
    submitting: false,
    error: '',
    paymentEntryEnabled: appConfig.paymentEntryEnabled,
  },

  onLoad(options: Record<string, string | undefined>) {
    void this.loadProduct(options.id || '')
  },

  async loadProduct(id: string) {
    if (!id) {
      this.setData({ loading: false, error: '会员卡商品无效' })
      return
    }
    this.setData({ loading: true, error: '' })
    try {
      const product = await request<CardProduct>(`/card-products/${encodeURIComponent(id)}`)
      this.setData({
        product,
        price: formatCent(product.price_cent),
        benefit: product.product_type === 'TIME_PASS' ? '有效期内不限总次数，每日限用 1 次' : `共 ${product.total_times} 次入场权益`,
        validity: product.activation_mode === 'FIRST_USE' ? `首次使用起 ${product.validity_days} 天有效` : `购买后 ${product.validity_days} 天有效`,
      })
    } catch (reason) {
      this.setData({ error: reason instanceof Error ? reason.message : '商品加载失败' })
    } finally {
      this.setData({ loading: false })
    }
  },

  async handleBuy() {
    const product = this.data.product
    if (!product || this.data.submitting) return
    if (!appConfig.paymentEntryEnabled) {
      wx.showModal({ title: '微信支付配置中', content: '支付密钥配置完成后即可在线购买，当前不会创建订单或扣款。', showCancel: false })
      return
    }
    const confirmation = await wx.showModal({ title: '确认购买', content: `${product.name}，应付 ${formatCent(product.price_cent)}` })
    if (!confirmation.confirm) return
    this.setData({ submitting: true })
    try {
      const order = await createOrder(product.id, createIdempotencyKey())
      const pay = await prepareWechatPay(order.order_no)
      await wx.requestPayment(pay)
      try {
        await syncWechatPay(order.order_no)
      } catch {
        // The signed server callback remains authoritative. The order page will refresh it.
      }
      wx.redirectTo({ url: `/pages/orders/detail?order_no=${encodeURIComponent(order.order_no)}` })
    } catch (reason) {
      wx.showToast({ title: reason instanceof Error ? reason.message : '支付发起失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },
})
