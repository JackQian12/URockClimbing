import { listOrders } from '../../services/orders'
import type { MemberOrder } from '../../types/api'
import { formatCent } from '../../utils/money'

const statusLabels: Record<string, string> = { PENDING: '待支付', PAID: '已支付', CLOSED: '已关闭', REFUNDING: '退款中', REFUNDED: '已退款' }

type DisplayOrder = MemberOrder & { amount: string; statusLabel: string; createdLabel: string }

Page({
  data: { items: [] as DisplayOrder[], loading: true, error: '' },
  onLoad() { void this.loadOrders() },
  onPullDownRefresh() { void this.loadOrders().finally(() => wx.stopPullDownRefresh()) },
  async loadOrders() {
    this.setData({ loading: true, error: '' })
    try {
      const result = await listOrders()
      this.setData({ items: result.items.map((item) => ({ ...item, amount: formatCent(item.total_amount_cent), statusLabel: statusLabels[item.status] || item.status, createdLabel: new Date(item.created_at).toLocaleString('zh-CN') })) })
    } catch (reason) { this.setData({ error: reason instanceof Error ? reason.message : '订单加载失败' }) }
    finally { this.setData({ loading: false }) }
  },
  handleOrder(event: WechatMiniprogram.BaseEvent) {
    const orderNo = String(event.currentTarget.dataset.orderNo || '')
    if (orderNo) wx.navigateTo({ url: `/pages/orders/detail?order_no=${encodeURIComponent(orderNo)}` })
  },
})
