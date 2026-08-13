import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { apiRequest } from '../services/api'
import type { ActivationMode, CardProduct, CardProductInput, CardProductStatus, ProductType } from '../types/api'

type ProductList = { items: CardProduct[]; total: number }
type Filter = 'ALL' | CardProductStatus
type ProductFilter = 'ALL' | ProductType

type Editor = {
  id?: string
  name: string
  shortDescription: string
  description: string
  productType: ProductType
  totalTimes: string
  validityDays: string
  activationMode: ActivationMode
  priceYuan: string
  listPriceYuan: string
  purchaseLimit: string
  dailyUseLimit: string
  transferable: boolean
  rules: string
  badge: string
  themeColor: string
  sortOrder: string
  version: number
}

const emptyEditor: Editor = {
  name: '', shortDescription: '', description: '', productType: 'COUNT_CARD', totalTimes: '10', validityDays: '365',
  activationMode: 'PURCHASE', priceYuan: '', listPriceYuan: '', purchaseLimit: '',
  dailyUseLimit: '1', transferable: false,
  rules: '仅限本人使用；每次入场核销 1 次；特殊活动及课程除外。',
  badge: '推荐', themeColor: '#6F4A2E', sortOrder: '0', version: 0,
}

const statusMeta: Record<CardProductStatus, { label: string; className: string }> = {
  DRAFT: { label: '草稿', className: 'draft' },
  ON_SALE: { label: '在售', className: 'on-sale' },
  OFF_SALE: { label: '已下架', className: 'off-sale' },
}

function yuan(cent: number): string {
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY', minimumFractionDigits: cent % 100 ? 2 : 0 }).format(cent / 100)
}

function toEditor(product: CardProduct): Editor {
  return {
    id: product.id,
    name: product.name,
    shortDescription: product.short_description,
    description: product.description,
    productType: product.product_type,
    totalTimes: product.total_times == null ? '' : String(product.total_times),
    validityDays: String(product.validity_days),
    activationMode: product.activation_mode,
    priceYuan: String(product.price_cent / 100),
    listPriceYuan: product.list_price_cent == null ? '' : String(product.list_price_cent / 100),
    purchaseLimit: product.purchase_limit == null ? '' : String(product.purchase_limit),
    dailyUseLimit: String(product.daily_use_limit),
    transferable: product.transferable,
    rules: product.rules,
    badge: product.badge ?? '',
    themeColor: product.theme_color,
    sortOrder: String(product.sort_order),
    version: product.version,
  }
}

function toInput(editor: Editor): CardProductInput {
  return {
    name: editor.name.trim(),
    short_description: editor.shortDescription.trim(),
    description: editor.description.trim(),
    product_type: editor.productType,
    total_times: editor.productType === 'COUNT_CARD' ? Number(editor.totalTimes) : null,
    validity_days: Number(editor.validityDays),
    activation_mode: editor.activationMode,
    price_cent: Math.round(Number(editor.priceYuan) * 100),
    list_price_cent: editor.listPriceYuan ? Math.round(Number(editor.listPriceYuan) * 100) : null,
    purchase_limit: editor.purchaseLimit ? Number(editor.purchaseLimit) : null,
    daily_use_limit: Number(editor.dailyUseLimit),
    transferable: editor.transferable,
    rules: editor.rules.trim(),
    badge: editor.badge.trim() || null,
    theme_color: editor.themeColor.toUpperCase(),
    sort_order: Number(editor.sortOrder),
    version: editor.version,
  }
}

export function CardProductsPage() {
  const [products, setProducts] = useState<CardProduct[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [filter, setFilter] = useState<Filter>('ALL')
  const [productFilter, setProductFilter] = useState<ProductFilter>('ALL')
  const [keyword, setKeyword] = useState('')
  const [editor, setEditor] = useState<Editor | null>(null)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const result = await apiRequest<ProductList>('/admin/card-products')
      setProducts(result.items)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '商品加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const counts = useMemo(() => ({
    ALL: products.length,
    ON_SALE: products.filter((item) => item.status === 'ON_SALE').length,
    DRAFT: products.filter((item) => item.status === 'DRAFT').length,
    OFF_SALE: products.filter((item) => item.status === 'OFF_SALE').length,
  }), [products])

  const visibleProducts = useMemo(() => {
    const query = keyword.trim().toLowerCase()
    return products.filter((item) => (filter === 'ALL' || item.status === filter) &&
      (productFilter === 'ALL' || item.product_type === productFilter) &&
      (!query || item.name.toLowerCase().includes(query) || item.short_description.toLowerCase().includes(query)))
  }, [filter, keyword, productFilter, products])

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!editor || saving) return
    setFormError('')
    const input = toInput(editor)
    if (!input.name || (input.product_type === 'COUNT_CARD' && (!input.total_times || input.total_times < 1)) || input.validity_days < 1 || input.price_cent < 1) {
      setFormError('请完整填写商品名称、权益、有效期和售价')
      return
    }
    if (input.list_price_cent != null && input.list_price_cent < input.price_cent) {
      setFormError('划线价不能低于售价')
      return
    }
    setSaving(true)
    try {
      const saved = editor.id
        ? await apiRequest<CardProduct>(`/admin/card-products/${editor.id}`, { method: 'PUT', body: JSON.stringify(input) })
        : await apiRequest<CardProduct>('/admin/card-products', { method: 'POST', body: JSON.stringify(input) })
      setProducts((current) => editor.id
        ? current.map((item) => item.id === saved.id ? saved : item)
        : [saved, ...current])
      setEditor(null)
      setNotice(editor.id ? '商品已保存' : '草稿已创建')
    } catch (reason) {
      setFormError(reason instanceof Error ? reason.message : '保存失败，请稍后重试')
    } finally {
      setSaving(false)
    }
  }

  async function changeStatus(product: CardProduct) {
    const nextStatus: CardProductStatus = product.status === 'ON_SALE' ? 'OFF_SALE' : 'ON_SALE'
    const verb = nextStatus === 'ON_SALE' ? '上架' : '下架'
    if (!window.confirm(`确认${verb}“${product.name}”吗？`)) return
    setNotice('')
    try {
      const updated = await apiRequest<CardProduct>(`/admin/card-products/${product.id}/status`, {
        method: 'POST', body: JSON.stringify({ status: nextStatus, version: product.version }),
      })
      setProducts((current) => current.map((item) => item.id === updated.id ? updated : item))
      setNotice(`商品已${verb}`)
    } catch (reason) {
      setNotice(reason instanceof Error ? reason.message : `${verb}失败`)
    }
  }

  return <div className="page-content card-product-page">
    <header className="page-header">
      <div><p className="eyebrow">MEMBERSHIP PRODUCTS</p><h1>会员卡商品</h1><p>统一管理次卡、周卡、月卡、季卡和年卡。</p></div>
      <button className="primary-button compact" onClick={() => { setFormError(''); setEditor({ ...emptyEditor }) }}>新建会员卡</button>
    </header>

    <section className="product-summary">
      <div><span>商品总数</span><strong>{counts.ALL}</strong></div>
      <div><span>在售</span><strong>{counts.ON_SALE}</strong></div>
      <div><span>草稿</span><strong>{counts.DRAFT}</strong></div>
      <div><span>已下架</span><strong>{counts.OFF_SALE}</strong></div>
    </section>

    <section className="product-toolbar">
      <div className="filter-tabs">
        {([['ALL', '全部'], ['ON_SALE', '在售'], ['DRAFT', '草稿'], ['OFF_SALE', '已下架']] as const).map(([value, label]) =>
          <button key={value} className={filter === value ? 'active' : ''} onClick={() => setFilter(value)}>{label}<small>{counts[value]}</small></button>)}
      </div>
      <div className="product-toolbar-fields">
        <select className="search-input product-type-filter" value={productFilter} onChange={(event) => setProductFilter(event.target.value as ProductFilter)}>
          <option value="ALL">全部卡类型</option>
          <option value="COUNT_CARD">次卡</option>
          <option value="TIME_PASS">周卡 / 月卡 / 季卡 / 年卡</option>
        </select>
        <input className="search-input" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="搜索商品名称" />
      </div>
    </section>

    {notice && <div className="page-notice">{notice}</div>}
    {loading && <section className="content-card empty-state"><div className="loading-ring" /><h2>正在加载商品</h2></section>}
    {!loading && error && <section className="content-card empty-state"><div className="empty-icon">!</div><h2>加载失败</h2><p>{error}</p><button className="secondary-button compact-secondary" onClick={() => void load()}>重新加载</button></section>}
    {!loading && !error && visibleProducts.length === 0 && <section className="content-card empty-state"><div className="empty-icon">◇</div><h2>{products.length ? '没有符合条件的商品' : '创建第一张攀岩会员卡'}</h2><p>{products.length ? '尝试切换筛选条件或搜索词。' : '可以从次卡或月卡开始，清楚写明有效期和使用规则。'}</p></section>}

    {!loading && !error && visibleProducts.length > 0 && <section className="product-grid">
      {visibleProducts.map((product) => {
        const status = statusMeta[product.status]
        return <article className="product-admin-card" key={product.id}>
          <div className="product-color" style={{ background: product.theme_color }} />
          <div className="product-card-body">
            <div className="product-card-top"><div><span className="product-kind-badge">{product.product_type === 'COUNT_CARD' ? '次卡' : ({ 7: '周卡', 30: '月卡', 90: '季卡', 365: '年卡' }[product.validity_days] || '期限卡')}</span>{product.badge && <span className="product-badge">{product.badge}</span>}<span className={`status-badge ${status.className}`}>{status.label}</span></div><span className="product-id">#{product.id}</span></div>
            <h2>{product.name}</h2><p className="product-short">{product.short_description || '暂无一句话介绍'}</p>
            <div className="product-price"><strong>{yuan(product.price_cent)}</strong>{product.list_price_cent && <del>{yuan(product.list_price_cent)}</del>}<small>{product.product_type === 'COUNT_CARD' && product.total_times ? `约 ${yuan(Math.round(product.price_cent / product.total_times))}/次` : '有效期内不限次'}</small></div>
            <div className="product-facts"><span><b>{product.product_type === 'COUNT_CARD' ? product.total_times : '不限'}</b> {product.product_type === 'COUNT_CARD' ? '次入场' : '总次数'}</span><span><b>{product.validity_days}</b> 天有效</span><span>{product.activation_mode === 'PURCHASE' ? '购买即生效' : '首次使用生效'}</span><span>{product.transferable ? '允许共享' : '仅限本人'}</span></div>
            <div className="product-actions"><button className="text-action" onClick={() => { setFormError(''); setEditor(toEditor(product)) }}>编辑</button><button className={product.status === 'ON_SALE' ? 'danger-action' : 'publish-action'} onClick={() => void changeStatus(product)}>{product.status === 'ON_SALE' ? '下架' : '上架'}</button></div>
          </div>
        </article>
      })}
    </section>}

    {editor && <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !saving) setEditor(null) }}>
      <section className="product-modal" role="dialog" aria-modal="true" aria-labelledby="product-editor-title">
        <header><div><p className="eyebrow">PRODUCT EDITOR</p><h2 id="product-editor-title">{editor.id ? '编辑会员卡' : '新建会员卡'}</h2></div><button className="modal-close" aria-label="关闭" disabled={saving} onClick={() => setEditor(null)}>×</button></header>
        <form onSubmit={save}>
          <div className="form-section"><h3>基础信息</h3><div className="form-grid">
            <label>会员卡类型<select value={editor.productType} onChange={(event) => {
              const productType = event.target.value as ProductType
              setEditor({ ...editor, productType, totalTimes: productType === 'COUNT_CARD' ? (editor.totalTimes || '10') : '', validityDays: productType === 'TIME_PASS' ? '30' : editor.validityDays, activationMode: productType === 'TIME_PASS' ? 'FIRST_USE' : editor.activationMode, dailyUseLimit: productType === 'TIME_PASS' ? '1' : editor.dailyUseLimit, transferable: productType === 'TIME_PASS' ? false : editor.transferable })
            }}><option value="COUNT_CARD">次卡</option><option value="TIME_PASS">期限卡</option></select></label>
            <label>商品名称<input maxLength={100} value={editor.name} onChange={(event) => setEditor({ ...editor, name: event.target.value })} placeholder={editor.productType === 'COUNT_CARD' ? '例如：成人 10 次攀岩卡' : '例如：成人攀岩月卡'} /></label>
            <label className="span-2">一句话介绍<input maxLength={255} value={editor.shortDescription} onChange={(event) => setEditor({ ...editor, shortDescription: event.target.value })} placeholder="灵活到店，适合每月攀爬 1–2 次" /></label>
            <label>商品标签<input maxLength={32} value={editor.badge} onChange={(event) => setEditor({ ...editor, badge: event.target.value })} placeholder="推荐 / 新客专享" /></label>
            <label>卡面主题色<input type="color" value={editor.themeColor} onChange={(event) => setEditor({ ...editor, themeColor: event.target.value })} /></label>
            <label className="span-2">详细介绍<textarea rows={3} value={editor.description} onChange={(event) => setEditor({ ...editor, description: event.target.value })} placeholder="介绍适合人群和包含的权益" /></label>
          </div></div>
          <div className="form-section"><h3>权益与有效期</h3><div className="form-grid form-grid-3">
            {editor.productType === 'COUNT_CARD' ? <label>可用次数<input type="number" min="1" max="1000" value={editor.totalTimes} onChange={(event) => setEditor({ ...editor, totalTimes: event.target.value })} /></label> : <label>总使用次数<input value="有效期内不限次" disabled /></label>}
            {editor.productType === 'TIME_PASS' ? <label>期限卡周期<select value={editor.validityDays} onChange={(event) => setEditor({ ...editor, validityDays: event.target.value })}><option value="7">周卡（7 天）</option><option value="30">月卡（30 天）</option><option value="90">季卡（90 天）</option><option value="365">年卡（365 天）</option></select></label> : <label>有效期（天）<input type="number" min="1" max="3650" value={editor.validityDays} onChange={(event) => setEditor({ ...editor, validityDays: event.target.value })} /></label>}
            <label>生效方式<select value={editor.activationMode} disabled={editor.productType === 'TIME_PASS'} onChange={(event) => setEditor({ ...editor, activationMode: event.target.value as ActivationMode })}><option value="PURCHASE">购买后立即生效</option><option value="FIRST_USE">首次核销时生效</option></select></label>
            <label>每日最多核销<input type="number" min="1" max="20" disabled={editor.productType === 'TIME_PASS'} value={editor.dailyUseLimit} onChange={(event) => setEditor({ ...editor, dailyUseLimit: event.target.value })} /></label>
            <label>每人限购（留空不限）<input type="number" min="1" max="100" value={editor.purchaseLimit} onChange={(event) => setEditor({ ...editor, purchaseLimit: event.target.value })} /></label>
            <label className="checkbox-label"><input type="checkbox" disabled={editor.productType === 'TIME_PASS'} checked={editor.transferable} onChange={(event) => setEditor({ ...editor, transferable: event.target.checked })} /><span>{editor.productType === 'TIME_PASS' ? '期限卡仅限本人使用' : '允许持卡人分享给同行人使用'}</span></label>
          </div></div>
          <div className="form-section"><h3>价格与展示</h3><div className="form-grid form-grid-3">
            <label>售价（元）<input type="number" min="0.01" step="0.01" value={editor.priceYuan} onChange={(event) => setEditor({ ...editor, priceYuan: event.target.value })} placeholder="1680" /></label>
            <label>划线价（元）<input type="number" min="0.01" step="0.01" value={editor.listPriceYuan} onChange={(event) => setEditor({ ...editor, listPriceYuan: event.target.value })} placeholder="2000" /></label>
            <label>排序权重<input type="number" value={editor.sortOrder} onChange={(event) => setEditor({ ...editor, sortOrder: event.target.value })} /><small>数值越大越靠前</small></label>
            <label className="span-3">使用规则<textarea rows={4} value={editor.rules} onChange={(event) => setEditor({ ...editor, rules: event.target.value })} placeholder="写清适用范围、是否含装备、退款和特殊活动规则" /></label>
          </div></div>
          {formError && <div className="form-error">{formError}</div>}
          <footer><button type="button" className="secondary-button compact-secondary" disabled={saving} onClick={() => setEditor(null)}>取消</button><button className="primary-button compact" disabled={saving}>{saving ? '保存中…' : editor.id ? '保存修改' : '保存为草稿'}</button></footer>
        </form>
      </section>
    </div>}
  </div>
}
