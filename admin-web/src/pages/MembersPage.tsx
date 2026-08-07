import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { apiRequest } from '../services/api'
import type { Member, MemberList, MemberStatus } from '../types/api'

type StatusFilter = 'ALL' | MemberStatus

const PAGE_SIZE = 20

function dateTime(value: string | null): string {
  if (!value) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(value))
}

function money(cent: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency', currency: 'CNY', minimumFractionDigits: cent % 100 ? 2 : 0,
  }).format(cent / 100)
}

function memberName(member: Member): string {
  return member.nickname?.trim() || '微信会员'
}

export function MembersPage() {
  const [members, setMembers] = useState<Member[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<StatusFilter>('ALL')
  const [searchInput, setSearchInput] = useState('')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<Member | null>(null)
  const [tags, setTags] = useState('')
  const [note, setNote] = useState('')
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) })
    if (query) params.set('q', query)
    if (status !== 'ALL') params.set('status', status)
    try {
      const result = await apiRequest<MemberList>(`/admin/members?${params}`)
      setMembers(result.items)
      setTotal(result.total)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '会员加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, query, status])

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const summary = useMemo(() => ({
    cards: members.reduce((sum, item) => sum + item.active_card_count, 0),
    remaining: members.reduce((sum, item) => sum + item.remaining_times, 0),
    wechat: members.filter((item) => item.wechat_bound).length,
  }), [members])

  function search(event: FormEvent) {
    event.preventDefault()
    setPage(1)
    setQuery(searchInput.trim())
  }

  function openMember(member: Member) {
    setSelected(member)
    setTags(member.tags.join('，'))
    setNote(member.admin_note)
    setFormError('')
  }

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!selected || saving) return
    const normalizedTags = tags.split(/[,，]/).map((item) => item.trim()).filter(Boolean)
    if (normalizedTags.length > 10) {
      setFormError('最多设置 10 个标签')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      const updated = await apiRequest<Member>(`/admin/members/${selected.id}`, {
        method: 'PUT',
        body: JSON.stringify({ status: selected.status, tags: normalizedTags, admin_note: note.trim(), version: selected.version }),
      })
      setMembers((current) => current.map((item) => item.id === updated.id ? updated : item))
      setSelected(null)
      setNotice('会员资料已保存')
    } catch (reason) {
      setFormError(reason instanceof Error ? reason.message : '保存失败，请稍后重试')
    } finally {
      setSaving(false)
    }
  }

  return <div className="page-content member-page">
    <header className="page-header">
      <div><p className="eyebrow">MEMBERS</p><h1>会员管理</h1><p>查询微信会员、查看次卡与到店数据，并维护运营标签。</p></div>
      <div className="date-chip">共 {total} 位会员</div>
    </header>

    <section className="member-summary">
      <div><span>符合条件</span><strong>{total}</strong><small>位会员</small></div>
      <div><span>本页有效次卡</span><strong>{summary.cards}</strong><small>张</small></div>
      <div><span>本页剩余次数</span><strong>{summary.remaining}</strong><small>次</small></div>
      <div><span>本页微信绑定</span><strong>{summary.wechat}</strong><small>人</small></div>
    </section>

    <section className="member-toolbar">
      <div className="filter-tabs">
        {([['ALL', '全部'], ['ACTIVE', '正常'], ['DISABLED', '已禁用']] as const).map(([value, label]) =>
          <button key={value} className={status === value ? 'active' : ''} onClick={() => { setPage(1); setStatus(value) }}>{label}</button>)}
      </div>
      <form className="member-search" onSubmit={search}>
        <input className="search-input" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="会员号、昵称或手机号后四位" />
        <button className="secondary-button compact-secondary">搜索</button>
      </form>
    </section>

    {notice && <div className="page-notice">{notice}</div>}
    {loading && <section className="content-card empty-state"><div className="loading-ring" /><h2>正在加载会员</h2></section>}
    {!loading && error && <section className="content-card empty-state"><div className="empty-icon">!</div><h2>加载失败</h2><p>{error}</p><button className="secondary-button compact-secondary" onClick={() => void load()}>重新加载</button></section>}
    {!loading && !error && members.length === 0 && <section className="content-card empty-state"><div className="empty-icon">◇</div><h2>{query || status !== 'ALL' ? '没有符合条件的会员' : '还没有微信会员'}</h2><p>{query || status !== 'ALL' ? '请更换搜索条件后重试。' : '用户首次通过微信小程序登录后，会自动出现在这里。'}</p></section>}

    {!loading && !error && members.length > 0 && <section className="member-table-card">
      <div className="member-table-wrap"><table className="member-table">
        <thead><tr><th>会员</th><th>联系方式</th><th>次卡权益</th><th>累计消费</th><th>到店核销</th><th>状态 / 标签</th><th /></tr></thead>
        <tbody>{members.map((member) => <tr key={member.id}>
          <td><div className="member-identity">
            {member.avatar_url ? <img src={member.avatar_url} alt="" referrerPolicy="no-referrer" /> : <span>{memberName(member).slice(0, 1)}</span>}
            <div><strong>{memberName(member)}</strong><small>{member.member_no}</small></div>
          </div></td>
          <td><strong className="table-primary">{member.phone || '未授权手机号'}</strong><small className={member.wechat_bound ? 'bound-dot' : ''}>{member.wechat_bound ? '微信已绑定' : '微信未绑定'}</small></td>
          <td><strong className="table-primary">{member.active_card_count} 张 · {member.remaining_times} 次</strong><small>当前有效</small></td>
          <td><strong className="table-primary">{money(member.total_spent_cent)}</strong><small>{member.paid_order_count} 笔已支付</small></td>
          <td><strong className="table-primary">{member.redemption_count} 次</strong><small>{member.last_redemption_at ? `最近 ${dateTime(member.last_redemption_at)}` : '暂无到店记录'}</small></td>
          <td><span className={`member-status ${member.status.toLowerCase()}`}>{member.status === 'ACTIVE' ? '正常' : '已禁用'}</span><div className="tag-row">{member.tags.slice(0, 2).map((tag) => <span key={tag}>{tag}</span>)}</div></td>
          <td><button className="detail-button" onClick={() => openMember(member)}>查看</button></td>
        </tr>)}</tbody>
      </table></div>
      <footer className="pagination"><span>第 {page} / {pageCount} 页</span><div><button disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>上一页</button><button disabled={page >= pageCount} onClick={() => setPage((value) => value + 1)}>下一页</button></div></footer>
    </section>}

    {selected && <div className="modal-backdrop member-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !saving) setSelected(null) }}>
      <section className="member-drawer" role="dialog" aria-modal="true" aria-labelledby="member-detail-title">
        <header><div><p className="eyebrow">MEMBER PROFILE</p><h2 id="member-detail-title">{memberName(selected)}</h2><small>{selected.member_no}</small></div><button className="modal-close" aria-label="关闭" disabled={saving} onClick={() => setSelected(null)}>×</button></header>
        <div className="member-detail-body">
          <section className="detail-metrics"><div><span>有效次卡</span><strong>{selected.active_card_count} 张</strong></div><div><span>剩余次数</span><strong>{selected.remaining_times} 次</strong></div><div><span>累计消费</span><strong>{money(selected.total_spent_cent)}</strong></div><div><span>核销次数</span><strong>{selected.redemption_count} 次</strong></div></section>
          <section className="detail-section"><h3>账号与授权</h3><dl className="detail-list"><div><dt>手机号</dt><dd>{selected.phone || '未授权'}</dd></div><div><dt>微信账号</dt><dd>{selected.wechat_bound ? '已绑定' : '未绑定'}</dd></div><div><dt>隐私同意</dt><dd>{selected.privacy_consent_at ? `${selected.privacy_consent_version || '已同意'} · ${dateTime(selected.privacy_consent_at)}` : '暂无记录'}</dd></div><div><dt>营销授权</dt><dd>{selected.marketing_consent ? '已同意' : '未同意'}</dd></div></dl></section>
          <section className="detail-section"><h3>会员活动</h3><dl className="detail-list"><div><dt>注册时间</dt><dd>{dateTime(selected.registered_at)}</dd></div><div><dt>最近登录</dt><dd>{dateTime(selected.last_login_at)}</dd></div><div><dt>最近核销</dt><dd>{dateTime(selected.last_redemption_at)}</dd></div><div><dt>支付订单</dt><dd>{selected.paid_order_count} 笔</dd></div></dl></section>
          <form id="member-editor" onSubmit={save}>
            <section className="detail-section"><h3>运营管理</h3><div className="member-form">
              <label>会员状态<select value={selected.status} onChange={(event) => setSelected({ ...selected, status: event.target.value as MemberStatus })}><option value="ACTIVE">正常</option><option value="DISABLED">禁用登录与使用权益</option></select></label>
              <label>会员标签<input value={tags} maxLength={220} onChange={(event) => setTags(event.target.value)} placeholder="例如：常客，抱石，重点跟进" /><small>使用逗号分隔，最多 10 个；仅后台可见</small></label>
              <label>内部备注<textarea rows={4} maxLength={500} value={note} onChange={(event) => setNote(event.target.value)} placeholder="记录需要门店员工了解的信息" /><small>{note.length} / 500，仅后台可见</small></label>
            </div></section>
            {formError && <div className="form-error">{formError}</div>}
          </form>
        </div>
        <footer><button className="secondary-button compact-secondary" disabled={saving} onClick={() => setSelected(null)}>取消</button><button form="member-editor" className="primary-button compact" disabled={saving}>{saving ? '保存中…' : '保存会员资料'}</button></footer>
      </section>
    </div>}
  </div>
}
