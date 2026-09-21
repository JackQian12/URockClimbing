import { useEffect, useState, type FormEvent } from 'react'
import QRCode from 'qrcode'
import { apiRequest } from '../services/api'
import type { CheckinCode, CheckinCodeList } from '../types/api'

const formatTime = (value: string) => new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
}).format(new Date(value))

export function CheckinCodesPage() {
  const [items, setItems] = useState<CheckinCode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [name, setName] = useState('前台签到码')
  const [saving, setSaving] = useState(false)
  const [printCode, setPrintCode] = useState<CheckinCode | null>(null)
  const [qrDataURL, setQRDataURL] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const result = await apiRequest<CheckinCodeList>('/admin/checkin-codes')
      setItems(result.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : '签到码列表加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  async function showPrintable(code: CheckinCode) {
    setNotice('')
    try {
      const detail = code.qr_payload ? code : await apiRequest<CheckinCode>(`/admin/checkin-codes/${code.id}`)
      if (!detail.qr_payload) throw new Error('签到码内容缺失')
      const image = await QRCode.toDataURL(detail.qr_payload, { width: 720, margin: 2, errorCorrectionLevel: 'H' })
      setPrintCode(detail)
      setQRDataURL(image)
    } catch (e) {
      setNotice(e instanceof Error ? e.message : '签到码加载失败')
    }
  }

  async function create(event: FormEvent) {
    event.preventDefault()
    if (name.trim().length < 2) return
    setSaving(true)
    setNotice('')
    try {
      const created = await apiRequest<CheckinCode>('/admin/checkin-codes', { method: 'POST', body: JSON.stringify({ name: name.trim() }) })
      setItems(current => [created, ...current])
      setNotice('签到码已创建，请打印并张贴在场馆前台')
      await showPrintable(created)
    } catch (e) {
      setNotice(e instanceof Error ? e.message : '签到码创建失败')
    } finally {
      setSaving(false)
    }
  }

  async function toggle(code: CheckinCode) {
    const status = code.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE'
    const message = status === 'INACTIVE'
      ? `停用“${code.name}”后，已打印的二维码将立即无法签到。确认停用？`
      : `确认重新启用“${code.name}”？`
    if (!window.confirm(message)) return
    setNotice('')
    try {
      const updated = await apiRequest<CheckinCode>(`/admin/checkin-codes/${code.id}/status`, { method: 'PUT', body: JSON.stringify({ status, version: code.version }) })
      setItems(current => current.map(item => item.id === updated.id ? updated : item))
      setNotice(status === 'ACTIVE' ? '签到码已启用' : '签到码已停用')
    } catch (e) {
      setNotice(e instanceof Error ? e.message : '状态修改失败')
    }
  }

  async function regenerate(code: CheckinCode) {
    if (!window.confirm(`确认为“${code.name}”更换二维码？原来已打印的二维码将立即失效，需要重新打印。`)) return
    setNotice('')
    try {
      const updated = await apiRequest<CheckinCode>(`/admin/checkin-codes/${code.id}/regenerate`, { method: 'POST', body: JSON.stringify({ version: code.version }) })
      setItems(current => current.map(item => item.id === updated.id ? updated : item))
      setNotice('二维码已更换，原码已失效，请重新打印')
      await showPrintable(updated)
    } catch (e) {
      setNotice(e instanceof Error ? e.message : '二维码更换失败')
    }
  }

  return <div className="page-content checkin-page">
    <header className="page-header"><div><p className="eyebrow">VENUE CHECK-IN</p><h1>签到码生成器</h1><p>生成可打印、可重复使用的场馆码，由后台统一启停。</p></div><div className="date-chip">{items.filter(item => item.status === 'ACTIVE').length} 个启用中</div></header>
    <section className="checkin-generator">
      <div><h2>创建新签到码</h2><p>建议按张贴位置命名，例如“前台签到码”。</p></div>
      <form onSubmit={create}><input value={name} maxLength={100} onChange={event => setName(event.target.value)} placeholder="签到码名称"/><button className="primary-button compact" disabled={saving || name.trim().length < 2}>{saving ? '正在生成…' : '生成并打印'}</button></form>
    </section>
    {notice && <div className="page-notice">{notice}</div>}
    {loading ? <section className="content-card empty-state"><div className="loading-ring"/><h2>正在加载签到码</h2></section> : error ? <section className="content-card empty-state"><div className="empty-icon">!</div><h2>加载失败</h2><p>{error}</p><button className="secondary-button compact-secondary" onClick={() => void load()}>重新加载</button></section> : items.length === 0 ? <section className="content-card empty-state"><div className="empty-icon">▦</div><h2>还没有签到码</h2><p>生成后打印张贴，会员即可在小程序扫码。</p></section> : <section className="checkin-code-grid">{items.map(code => <article className="checkin-code-card" key={code.id}><header><span className={`checkin-status ${code.status.toLowerCase()}`}>{code.status === 'ACTIVE' ? '已启用' : '已停用'}</span><small>v{code.version}</small></header><div className="checkin-code-icon">▦</div><h2>{code.name}</h2><code>{code.code_no}</code><dl><div><dt>创建时间</dt><dd>{formatTime(code.created_at)}</dd></div><div><dt>最后更新</dt><dd>{formatTime(code.updated_at)}</dd></div></dl><footer><button className="detail-button" onClick={() => void showPrintable(code)}>查看·打印</button><button className="text-action" onClick={() => void regenerate(code)}>更换码</button><button className={code.status === 'ACTIVE' ? 'danger-action' : 'publish-action'} onClick={() => void toggle(code)}>{code.status === 'ACTIVE' ? '停用' : '启用'}</button></footer></article>)}</section>}
    {printCode && <div className="modal-backdrop checkin-print-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setPrintCode(null) }}><section className="checkin-print-modal"><header className="no-print"><div><p className="eyebrow">PRINTABLE QR CODE</p><h2>预览并打印</h2></div><button className="modal-close" onClick={() => setPrintCode(null)}>×</button></header><div className="print-sheet"><img className="print-logo" src="/admin/logo.jpeg" alt="U-Rock 遇岩"/><h1>{printCode.name}</h1><p>打开 U-Rock 遇岩小程序·扫码签到</p><img className="print-qr" src={qrDataURL} alt={`${printCode.name}二维码`}/><strong>次卡核销 1 次 · 期限卡当日签到</strong><small>{printCode.code_no}</small><em>本码可重复使用，仅由场馆后台启停</em></div><footer className="no-print"><button className="secondary-button compact-secondary" onClick={() => setPrintCode(null)}>关闭</button><button className="primary-button compact" onClick={() => window.print()}>打印签到码</button></footer></section></div>}
  </div>
}
