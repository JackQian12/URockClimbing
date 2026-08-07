import { useState, type FormEvent } from 'react'
import { useAuth } from '../auth/AuthContext'

export function LoginPage() {
  const { session, login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  if (session) return null

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (submitting) return
    setError('')
    setSubmitting(true)
    try {
      await login(username.trim(), password)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '登录失败，请稍后重试')
    } finally {
      setSubmitting(false)
    }
  }

  return <div className="login-page">
    <section className="login-visual">
      <div className="visual-copy"><span>URock Climbing</span><h1>管理每一次<br />向上的力量</h1><p>会员、次卡、订单与核销，在一个清晰的后台完成。</p></div>
      <div className="route-art" aria-hidden="true"><i /><i /><i /><i /><i /></div>
    </section>
    <section className="login-panel">
      <form className="login-form" onSubmit={submit}>
        <div className="login-brand"><span className="brand-logo brand-logo-full"><img src="/admin/logo.jpeg" alt="U-Rock 遇岩，遇见向上的自己" /></span></div>
        <p className="eyebrow">ADMIN PORTAL</p><h2>欢迎回来</h2><p className="form-intro">使用管理员账号登录 URock 运营后台</p>
        <label>用户名<input autoFocus autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="请输入用户名" /></label>
        <label>密码<input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="请输入密码" /></label>
        {error && <div className="form-error">{error}</div>}
        <button className="primary-button" disabled={!username || !password || submitting}>{submitting ? '登录中…' : '登录后台'}</button>
        <small className="security-note">登录行为将被安全审计。请勿共享管理员账号。</small>
      </form>
    </section>
  </div>
}
