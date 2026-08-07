import { useEffect, useState } from 'react'
import { apiRequest } from '../services/api'
import type { CardProduct } from '../types/api'
import type { AdminOrderList, MemberList, RedemptionList } from '../types/api'

export function DashboardPage() {
  const [onSale, setOnSale] = useState<string>('—')
  const [memberTotal, setMemberTotal] = useState('—')
  const [todayOrders, setTodayOrders] = useState('—')
  const [todayRedemptions, setTodayRedemptions] = useState('—')
  useEffect(() => {
    apiRequest<{ items: CardProduct[]; total: number }>('/admin/card-products')
      .then((result) => setOnSale(String(result.items.filter((item) => item.status === 'ON_SALE').length)))
      .catch(() => setOnSale('—'))
    apiRequest<MemberList>('/admin/members?page_size=1').then((result) => setMemberTotal(String(result.total))).catch(() => setMemberTotal('—'))
    apiRequest<AdminOrderList>('/admin/orders?page_size=50').then((result) => {
      const today = new Date(); setTodayOrders(String(result.items.filter((item) => {
        const created = new Date(item.created_at)
        return created.getFullYear() === today.getFullYear() && created.getMonth() === today.getMonth() && created.getDate() === today.getDate()
      }).length))
    }).catch(() => setTodayOrders('—'))
    apiRequest<RedemptionList>('/admin/redemptions?page_size=50').then((result) => {
      const today = new Date(); setTodayRedemptions(String(result.items.filter((item) => {
        const redeemed = new Date(item.redeemed_at)
        return redeemed.getFullYear() === today.getFullYear() && redeemed.getMonth() === today.getMonth() && redeemed.getDate() === today.getDate()
      }).length))
    }).catch(() => setTodayRedemptions('—'))
  }, [])
  const stats = [['会员总数', memberTotal, '微信注册会员'], ['今日订单', todayOrders, '今天创建的订单'], ['今日核销', todayRedemptions, '今天完成的核销'], ['在售次卡', onSale, '当前可售商品']]
  return <div className="page-content">
    <header className="page-header"><div><p className="eyebrow">OVERVIEW</p><h1>数据概览</h1><p>掌握场馆今天的关键运营信息。</p></div><span className="date-chip">URock · 单门店</span></header>
    <section className="stats-grid">{stats.map(([label, value, hint]) => <article className="stat-card" key={label}><span>{label}</span><strong>{value}</strong><small>{hint}</small></article>)}</section>
    <section className="content-card empty-state"><div className="empty-icon">↗</div><h2>后台已准备就绪</h2><p>管理模块将按 PRD 里程碑逐步接入真实业务数据。</p></section>
  </div>
}
