import { useEffect, useState } from 'react'
import { useAuth } from './auth/AuthContext'
import { AdminLayout } from './layouts/AdminLayout'
import { DashboardPage } from './pages/DashboardPage'
import { LoginPage } from './pages/LoginPage'
import { ChangePasswordPage } from './pages/ChangePasswordPage'
import { CardProductsPage } from './pages/CardProductsPage'
import { MembersPage } from './pages/MembersPage'
import { OrdersPage } from './pages/OrdersPage'
import { AuditLogsPage, RedemptionsPage, StaffPage } from './pages/OperationsPages'
import { adminPath, navigate } from './services/navigation'

function Redirect({ to }: { to: string }) {
  useEffect(() => navigate(to, true), [to])
  return null
}

function protectedPage(path: string) {
  const pages: Record<string, React.ReactNode> = {
    '/': <DashboardPage />,
    '/card-products': <CardProductsPage />,
    '/members': <MembersPage />,
    '/orders': <OrdersPage />,
    '/redemptions': <RedemptionsPage />,
    '/staff': <StaffPage />,
    '/audit-logs': <AuditLogsPage />,
  }
  return <AdminLayout currentPath={path}>{pages[path] ?? pages['/']}</AdminLayout>
}

export function App() {
  const { session, loading } = useAuth()
  const [path, setPath] = useState(adminPath())
  useEffect(() => {
    const update = () => setPath(adminPath())
    window.addEventListener('popstate', update)
    return () => window.removeEventListener('popstate', update)
  }, [])
  if (loading) return <div className="app-loading">正在验证管理会话…</div>
  if (!session && path !== '/login') {
    return <Redirect to="/login" />
  }
  if (session?.user.must_change_password && path !== '/change-password') {
    return <Redirect to="/change-password" />
  }
  if (session && !session.user.must_change_password && (path === '/login' || path === '/change-password')) {
    return <Redirect to="/" />
  }
  if (path === '/login') return <LoginPage />
  if (path === '/change-password') return <ChangePasswordPage />
  return protectedPage(path)
}
