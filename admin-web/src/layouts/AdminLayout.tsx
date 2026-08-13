import type { PropsWithChildren } from 'react'
import { useAuth } from '../auth/AuthContext'
import { navigate } from '../services/navigation'

const navigation = [
  ['/', '数据概览', '⌂'],
  ['/card-products', '会员卡商品', '◫'],
  ['/members', '会员管理', '◎'],
  ['/orders', '订单退款', '¥'],
  ['/redemptions', '核销记录', '✓'],
  ['/staff', '员工管理', '♙'],
  ['/audit-logs', '审计日志', '≡'],
]

export function AdminLayout({ children, currentPath }: PropsWithChildren<{ currentPath: string }>) {
  const { session, logout } = useAuth()
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-logo brand-logo-compact"><img src="/admin/logo.jpeg" alt="U-Rock 遇岩" /></span><div><b>U-Rock 遇岩</b><small>攀岩馆管理后台</small></div></div>
        <nav>
          {navigation.map(([to, label, icon]) => (
            <a key={to} href={`/admin${to}`} className={currentPath === to ? 'nav-link active' : 'nav-link'} onClick={(event) => { event.preventDefault(); navigate(to) }}>
              <span>{icon}</span>{label}
            </a>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div><small>当前管理员</small><strong>{session?.user.nickname || session?.user.username}</strong></div>
          <button className="text-button" onClick={() => void logout()}>退出</button>
        </div>
      </aside>
      <main className="main-content">{children}</main>
    </div>
  )
}
