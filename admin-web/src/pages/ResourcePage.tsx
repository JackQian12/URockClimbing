type Props = { eyebrow: string; title: string; description: string; action?: string }
export function ResourcePage({ eyebrow, title, description, action }: Props) {
  return <div className="page-content">
    <header className="page-header"><div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p>{description}</p></div>{action && <button className="primary-button compact" disabled>{action}</button>}</header>
    <section className="content-card empty-state"><div className="empty-icon">◇</div><h2>模块等待业务 API</h2><p>页面结构已经建立，相应 Go 后端用例完成后即可接入。</p></section>
  </div>
}

