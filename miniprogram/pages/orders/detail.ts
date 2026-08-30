import { getOrder } from '../../services/orders'
import type { MemberOrder } from '../../types/api'
import { formatCent } from '../../utils/money'

const statusLabels: Record<string, string> = { PENDING: '待支付', PAID: '已支付', CLOSED: '已关闭', REFUNDING: '退款中', REFUNDED: '已退款' }

Page({
  data: { order: null as MemberOrder | null, amount: '', statusLabel: '', createdLabel: '', expiresLabel: '', loading: true, error: '' },
  onLoad(options: Record<string, string | undefined>) { void this.loadOrder(options.order_no || '') },
  async loadOrder(orderNo: string) {
    if (!orderNo) { this.setData({ loading: false, error: '订单号无效' }); return }
    try {
      const order = await getOrder(orderNo)
      this.setData({ order, amount: formatCent(order.total_amount_cent), statusLabel: statusLabels[order.status] || order.status, createdLabel: new Date(order.created_at).toLocaleString('zh-CN'), expiresLabel: new Date(order.expires_at).toLocaleString('zh-CN') })
    } catch (reason) { this.setData({ error: reason instanceof Error ? reason.message : '订单加载失败' }) }
    finally { this.setData({ loading: false }) }
  },
})
