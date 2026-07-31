import { API_URL } from './api'
import {
  ADMIN_TOKEN_STORAGE_KEY,
  clearStoredValue,
  getStoredValue,
  LEGACY_ADMIN_TOKEN_STORAGE_KEY,
  setStoredValue,
} from './branding'

const ADMIN_CLIENT_ID = process.env.NEXT_PUBLIC_ADMIN_COGNITO_CLIENT_ID || ''

export function isAdminDevMode(): boolean {
  // Never enter the admin auth bypass in a production build, even if the admin
  // Cognito client ID is missing. In production a missing client ID is a
  // misconfiguration, not an invitation to skip auth (the API is the
  // authoritative gate and fails closed). Mirrors isDevMode() in auth.ts.
  // Without this guard, a govcloud build with no admin client ID treats the
  // user as logged in, hits a 401 from the API, hard-redirects to /admin/login,
  // which bounces straight back to /admin — an endless reload loop.
  if (process.env.NODE_ENV === 'production') return false
  return !ADMIN_CLIENT_ID
}

export function getAdminToken(): string | null {
  return getStoredValue(ADMIN_TOKEN_STORAGE_KEY, LEGACY_ADMIN_TOKEN_STORAGE_KEY)
}

export function setAdminToken(token: string): void {
  setStoredValue(ADMIN_TOKEN_STORAGE_KEY, token, LEGACY_ADMIN_TOKEN_STORAGE_KEY)
}

export function clearAdminToken(): void {
  clearStoredValue(ADMIN_TOKEN_STORAGE_KEY, LEGACY_ADMIN_TOKEN_STORAGE_KEY)
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

export type AdminSignInResult =
  | { kind: 'ok' }
  | { kind: 'mfa_setup'; session: string }
  | { kind: 'mfa_challenge'; session: string }

export async function adminSignIn(email: string, password: string): Promise<AdminSignInResult> {
  if (isAdminDevMode()) {
    setAdminToken('dev-admin-token')
    return { kind: 'ok' }
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

  if (data.challenge === 'MFA_SETUP') {
    return { kind: 'mfa_setup', session: data.session }
  }
  if (data.challenge === 'SOFTWARE_TOKEN_MFA') {
    return { kind: 'mfa_challenge', session: data.session }
  }

  setAdminToken(data.token)
  return { kind: 'ok' }
}

/** Complete admin TOTP enrollment; stores the admin token on success. */
export async function adminVerifyMfaSetup(email: string, code: string, session: string): Promise<void> {
  const res = await fetch('/api/auth/mfa/verify-setup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, session, client: 'admin' }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Failed to verify authenticator')
  setAdminToken(data.token)
}

/** Answer a returning admin MFA challenge; stores the admin token. */
export async function adminRespondMfaChallenge(email: string, code: string, session: string): Promise<void> {
  const res = await fetch('/api/auth/mfa/challenge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, session, client: 'admin' }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Invalid authenticator code')
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
