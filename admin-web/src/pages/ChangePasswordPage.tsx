import { useState, type FormEvent } from 'react'
import { useAuth } from '../auth/AuthContext'

export function ChangePasswordPage() {
  const { session, changePassword, logout } = useAuth()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  if (!session?.user.must_change_password) return null

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (submitting) return
    if (newPassword !== confirmation) {
      setError('两次输入的新密码不一致')
      return
    }
    setError('')
    setSubmitting(true)
    try {
      await changePassword(currentPassword, newPassword)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '修改失败，请稍后重试')
    } finally {
      setSubmitting(false)
    }
  }

  return <div className="password-page">
    <form className="login-form password-form" onSubmit={submit}>
      <div className="login-brand"><span className="brand-logo brand-logo-full"><img src="/admin/logo.jpeg" alt="U-Rock 遇岩，遇见向上的自己" /></span></div>
      <p className="eyebrow">SECURITY CHECK</p>
      <h2>设置新密码</h2>
      <p className="form-intro">首次登录必须修改初始密码。新密码需为 12 至 72 个字符。</p>
      <label>当前密码<input autoFocus type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} /></label>
      <label>新密码<input type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></label>
      <label>确认新密码<input type="password" autoComplete="new-password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>
      {error && <div className="form-error">{error}</div>}
      <button className="primary-button" disabled={!currentPassword || newPassword.length < 12 || !confirmation || submitting}>{submitting ? '保存中…' : '保存并进入后台'}</button>
      <button type="button" className="secondary-button" onClick={() => void logout()}>退出登录</button>
    </form>
  </div>
}
