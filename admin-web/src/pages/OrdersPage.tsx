import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { apiRequest } from '../services/api'
import type { AdminOrder, AdminOrderList, OrderStatus } from '../types/api'

type Filter = 'ALL' | OrderStatus
const PAGE_SIZE = 20
const statusMeta: Record<OrderStatus, { label: string; className: string }> = {
  PENDING: { label: '待支付', className: 'pending' }, PAID: { label: '已支付', className: 'paid' },
  CLOSED: { label: '已关闭', className: 'closed' }, REFUNDING: { label: '退款中', className: 'refunding' },
  REFUNDED: { label: '已退款', className: 'refunded' },
}

function money(cent: number) { return new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(cent / 100) }
function dateTime(value: string | null) { return value ? new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) : '—' }
function productName(order: AdminOrder) { return order.product_snapshot.name || order.product_snapshot.product_name || '攀岩次卡' }
function memberName(order: AdminOrder) { return order.member_nickname?.trim() || '微信会员' }

export function OrdersPage() {
  const [orders, setOrders] = useState<AdminOrder[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [filter, setFilter] = useState<Filter>('ALL')
  const [searchInput, setSearchInput] = useState('')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<AdminOrder | null>(null)
  const [refundOrder, setRefundOrder] = useState<AdminOrder | null>(null)
  const [refundReason, setRefundReason] = useState('')
  const [refunding, setRefunding] = useState(false)
  const [refundError, setRefundError] = useState('')
  const [notice, setNotice] = useState('')

  async function load() {
    setLoading(true); setError('')
    const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) })
    if (query) params.set('q', query); if (filter !== 'ALL') params.set('status', filter)
    try { const result = await apiRequest<AdminOrderList>(`/admin/orders?${params}`); setOrders(result.items); setTotal(result.total) }
    catch (reason) { setError(reason instanceof Error ? reason.message : '订单加载失败') }
    finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [page, query, filter])

  const summary = useMemo(() => ({
    paid: orders.filter((item) => item.status === 'PAID').length,
    revenue: orders.filter((item) => item.status === 'PAID').reduce((sum, item) => sum + item.paid_amount_cent, 0),
    refunding: orders.filter((item) => item.status === 'REFUNDING').length,
  }), [orders])
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))
  function search(event: FormEvent) { event.preventDefault(); setPage(1); setQuery(searchInput.trim()) }

  async function submitRefund(event: FormEvent) {
    event.preventDefault(); if (!refundOrder || refunding) return
    if (!refundReason.trim()) { setRefundError('请填写退款原因'); return }
    if (!window.confirm(`确认向“${memberName(refundOrder)}”全额退回 ${money(refundOrder.paid_amount_cent)} 吗？`)) return
    setRefunding(true); setRefundError('')
    try {
      await apiRequest(`/admin/orders/${refundOrder.order_no}/refund`, { method: 'POST', body: JSON.stringify({ reason: refundReason.trim(), client_request_id: crypto.randomUUID() }) })
      setRefundOrder(null); setSelected(null); setNotice('退款申请已提交，请关注退款最终状态'); await load()
    } catch (reason) { setRefundError(reason instanceof Error ? reason.message : '退款申请失败') }
    finally { setRefunding(false) }
  }

  return <div className="page-content order-page">
    <header className="page-header"><div><p className="eyebrow">ORDERS & REFUNDS</p><h1>订单管理</h1><p>查询订单、核对支付与次卡发放，并处理符合条件的全额退款。</p></div><div className="date-chip">共 {total} 笔订单</div></header>
    <section className="member-summary"><div><span>符合条件</span><strong>{total}</strong><small>笔</small></div><div><span>本页已支付</span><strong>{summary.paid}</strong><small>笔</small></div><div><span>本页实收</span><strong className="summary-money">{money(summary.revenue)}</strong></div><div><span>本页退款中</span><strong>{summary.refunding}</strong><small>笔</small></div></section>
    <section className="member-toolbar"><div className="filter-tabs">{([['ALL','全部'],['PENDING','待支付'],['PAID','已支付'],['REFUNDING','退款中'],['REFUNDED','已退款'],['CLOSED','已关闭']] as const).map(([value,label]) => <button key={value} className={filter===value?'active':''} onClick={()=>{setPage(1);setFilter(value)}}>{label}</button>)}</div><form className="member-search" onSubmit={search}><input className="search-input" value={searchInput} onChange={(e)=>setSearchInput(e.target.value)} placeholder="订单号、会员号、昵称或手机后四位"/><button className="secondary-button compact-secondary">搜索</button></form></section>
    {notice&&<div className="page-notice">{notice}</div>}
    {loading&&<section className="content-card empty-state"><div className="loading-ring"/><h2>正在加载订单</h2></section>}
    {!loading&&error&&<section className="content-card empty-state"><div className="empty-icon">!</div><h2>加载失败</h2><p>{error}</p><button className="secondary-button compact-secondary" onClick={()=>void load()}>重新加载</button></section>}
    {!loading&&!error&&orders.length===0&&<section className="content-card empty-state"><div className="empty-icon">◇</div><h2>{query||filter!=='ALL'?'没有符合条件的订单':'还没有订单'}</h2><p>{query||filter!=='ALL'?'请更换搜索条件后重试。':'会员购买次卡后，订单会自动出现在这里。'}</p></section>}
    {!loading&&!error&&orders.length>0&&<section className="member-table-card"><div className="member-table-wrap"><table className="member-table order-table"><thead><tr><th>订单 / 商品</th><th>会员</th><th>金额</th><th>支付</th><th>次卡</th><th>下单时间</th><th>状态</th><th/></tr></thead><tbody>{orders.map(order=>{const status=statusMeta[order.status];return <tr key={order.id}>
      <td><strong className="table-primary">{productName(order)}</strong><small>{order.order_no}</small></td><td><strong className="table-primary">{memberName(order)}</strong><small>{order.member_no} · {order.member_phone||'未授权手机'}</small></td><td><strong className="table-primary">{money(order.paid_amount_cent||order.total_amount_cent)}</strong><small>{order.paid_amount_cent?'实付':'应付'}</small></td><td><strong className="table-primary">{order.payment_status||'未创建'}</strong><small>{order.provider_transaction_id?'微信交易已确认':'暂无微信交易号'}</small></td><td><strong className="table-primary">{order.card_no||'未发卡'}</strong><small>{order.card_no?`${order.card_remaining_times}/${order.card_total_times} 次 · ${order.card_status}`:'—'}</small></td><td><strong className="table-primary">{dateTime(order.created_at)}</strong><small>{order.paid_at?`支付 ${dateTime(order.paid_at)}`:'尚未支付'}</small></td><td><span className={`order-status ${status.className}`}>{status.label}</span>{order.refund_status&&<small>退款单：{order.refund_status}</small>}</td><td><button className="detail-button" onClick={()=>setSelected(order)}>查看</button></td>
    </tr>})}</tbody></table></div><footer className="pagination"><span>第 {page} / {pageCount} 页</span><div><button disabled={page<=1} onClick={()=>setPage(v=>v-1)}>上一页</button><button disabled={page>=pageCount} onClick={()=>setPage(v=>v+1)}>下一页</button></div></footer></section>}

    {selected&&<div className="modal-backdrop member-backdrop" role="presentation" onMouseDown={e=>{if(e.target===e.currentTarget)setSelected(null)}}><section className="member-drawer order-drawer" role="dialog" aria-modal="true"><header><div><p className="eyebrow">ORDER DETAIL</p><h2>{productName(selected)}</h2><small>{selected.order_no}</small></div><button className="modal-close" onClick={()=>setSelected(null)}>×</button></header><div className="member-detail-body">
      <section className="detail-metrics"><div><span>订单金额</span><strong>{money(selected.total_amount_cent)}</strong></div><div><span>实付金额</span><strong>{money(selected.paid_amount_cent)}</strong></div><div><span>订单状态</span><strong>{statusMeta[selected.status].label}</strong></div><div><span>核销次数</span><strong>{selected.redemption_count} 次</strong></div></section>
      <section className="detail-section"><h3>会员信息</h3><dl className="detail-list"><div><dt>会员</dt><dd>{memberName(selected)} · {selected.member_no}</dd></div><div><dt>手机号</dt><dd>{selected.member_phone||'未授权'}</dd></div></dl></section>
      <section className="detail-section"><h3>支付交易</h3><dl className="detail-list"><div><dt>支付状态</dt><dd>{selected.payment_status||'未创建'}</dd></div><div><dt>商户支付单号</dt><dd className="break-value">{selected.merchant_order_no||'—'}</dd></div><div><dt>微信交易号</dt><dd className="break-value">{selected.provider_transaction_id||'—'}</dd></div><div><dt>支付时间</dt><dd>{dateTime(selected.paid_at)}</dd></div></dl></section>
      <section className="detail-section"><h3>次卡发放</h3><dl className="detail-list"><div><dt>次卡号</dt><dd>{selected.card_no||'未发放'}</dd></div><div><dt>卡状态</dt><dd>{selected.card_status||'—'}</dd></div><div><dt>剩余次数</dt><dd>{selected.card_remaining_times==null?'—':`${selected.card_remaining_times} / ${selected.card_total_times} 次`}</dd></div><div><dt>到期时间</dt><dd>{dateTime(selected.card_expires_at)}</dd></div></dl></section>
      {selected.refund_no&&<section className="detail-section"><h3>退款记录</h3><dl className="detail-list"><div><dt>退款单号</dt><dd>{selected.refund_no}</dd></div><div><dt>退款状态</dt><dd>{selected.refund_status}</dd></div><div><dt>退款原因</dt><dd>{selected.refund_reason}</dd></div><div><dt>申请时间</dt><dd>{dateTime(selected.refund_created_at)}</dd></div></dl></section>}
      <section className={`refund-eligibility ${selected.refund_eligible?'eligible':'ineligible'}`}><strong>{selected.refund_eligible?'可以申请全额退款':'当前不可退款'}</strong><p>{selected.refund_eligible?'次卡尚未核销，退款后将立即锁定权益，等待微信最终结果。':selected.refund_ineligible_reason}</p></section>
    </div><footer><button className="secondary-button compact-secondary" onClick={()=>setSelected(null)}>关闭</button>{selected.refund_eligible&&<button className="danger-solid" onClick={()=>{setRefundReason('');setRefundError('');setRefundOrder(selected)}}>全额退款</button>}</footer></section></div>}

    {refundOrder&&<div className="modal-backdrop refund-backdrop"><section className="refund-modal" role="dialog" aria-modal="true"><header><div><p className="eyebrow">FULL REFUND</p><h2>确认全额退款</h2></div><button className="modal-close" onClick={()=>setRefundOrder(null)}>×</button></header><form onSubmit={submitRefund}><div className="refund-amount"><span>原路退回</span><strong>{money(refundOrder.paid_amount_cent)}</strong><small>{memberName(refundOrder)} · {refundOrder.order_no}</small></div><label>退款原因<textarea rows={4} maxLength={255} value={refundReason} onChange={e=>setRefundReason(e.target.value)} placeholder="例如：顾客误购，次卡尚未使用"/><small>{refundReason.length} / 255，该原因将提交至微信支付</small></label><div className="refund-warning"><strong>退款安全检查</strong><p>仅支持整单全额退款。提交后次卡会锁定；微信受理不等于退款成功，需等待最终状态。</p></div>{refundError&&<div className="form-error">{refundError}</div>}<footer><button type="button" className="secondary-button compact-secondary" disabled={refunding} onClick={()=>setRefundOrder(null)}>取消</button><button className="danger-solid" disabled={refunding}>{refunding?'提交中…':'确认全额退款'}</button></footer></form></section></div>}
  </div>
}
