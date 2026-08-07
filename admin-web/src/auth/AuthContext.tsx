import { createContext, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react'
import { apiRequest, setCsrfToken } from '../services/api'
import type { AdminSession } from '../types/api'

type AuthState = {
  session: AdminSession | null
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: PropsWithChildren) {
  const [session, setSession] = useState<AdminSession | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    apiRequest<AdminSession>('/admin/me')
      .then((value) => {
        setCsrfToken(value.csrf_token)
        setSession(value)
      })
      .catch(() => setSession(null))
      .finally(() => setLoading(false))
  }, [])

  const value = useMemo<AuthState>(() => ({
    session,
    loading,
    async login(username, password) {
      const next = await apiRequest<AdminSession>('/admin/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      })
      setCsrfToken(next.csrf_token)
      setSession(next)
    },
    async changePassword(currentPassword, newPassword) {
      await apiRequest('/admin/auth/change-password', {
        method: 'POST',
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      })
      setSession((current) => current ? {
        ...current,
        user: { ...current.user, must_change_password: false },
      } : current)
    },
    async logout() {
      await apiRequest('/admin/auth/logout', { method: 'POST', body: '{}' })
      setCsrfToken('')
      setSession(null)
    },
  }), [loading, session])

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used within AuthProvider')
  return value
}
