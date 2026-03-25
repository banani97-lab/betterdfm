const ADMIN_CLIENT_ID = process.env.NEXT_PUBLIC_ADMIN_COGNITO_CLIENT_ID || ''
const ADMIN_TOKEN_KEY = 'betterdfm_admin_token'

import { API_URL } from './api'

export function isAdminDevMode(): boolean {
  return !ADMIN_CLIENT_ID
}

export function getAdminToken(): string | null {
  if (typeof window === 'undefined') return null
  return localStorage.getItem(ADMIN_TOKEN_KEY)
}

export function setAdminToken(token: string): void {
  localStorage.setItem(ADMIN_TOKEN_KEY, token)
}

export function clearAdminToken(): void {
  localStorage.removeItem(ADMIN_TOKEN_KEY)
}

export function isAdminTokenValid(): boolean {
  if (isAdminDevMode()) return true
  const token = getAdminToken()
  if (!token) return false
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
    return typeof payload.exp === 'number' && payload.exp * 1000 > Date.now()
  } catch {
    return false
  }
}

export function isAdminLoggedIn(): boolean {
  if (isAdminDevMode()) return true
  return !!getAdminToken() && isAdminTokenValid()
}

export async function adminSignIn(email: string, password: string): Promise<void> {
  if (isAdminDevMode()) {
    setAdminToken('dev-admin-token')
    return
  }

  const res = await fetch('/api/auth/admin-signin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })

  const data = await res.json()

  if (!res.ok) {
    throw new Error(data.error || 'Admin sign in failed')
  }

  setAdminToken(data.token)
}

export async function adminApiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getAdminToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string>),
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(`${API_URL}${path}`, { ...init, headers })

  if (res.status === 401) {
    clearAdminToken()
    if (typeof window !== 'undefined') {
      window.location.href = '/admin/login'
    }
    throw new Error('Admin session expired')
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.message || `Request failed: ${res.status}`)
  }

  return res.json()
}
