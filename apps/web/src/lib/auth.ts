import {
  clearStoredValue,
  getStoredValue,
  LEGACY_TOKEN_STORAGE_KEY,
  setStoredValue,
  TOKEN_STORAGE_KEY,
} from './branding'
import { identify, reset as analyticsReset } from './analytics'

const CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''

// ── Dev mode ──────────────────────────────────────────────────────────────────

export function isDevMode(): boolean {
  // Never enter the auth bypass in a production build, even if the Cognito
  // client ID is missing. In production a missing client ID is a
  // misconfiguration, not an invitation to skip auth (the API is the
  // authoritative gate regardless). Build-tagged out of prod on the backend.
  if (process.env.NODE_ENV === 'production') return false
  return !CLIENT_ID
}

// ── Token storage ─────────────────────────────────────────────────────────────

function decodeJwtPayload(token: string): Record<string, unknown> {
  return JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
}

export function getStoredToken(): string | null {
  return getStoredValue(TOKEN_STORAGE_KEY, LEGACY_TOKEN_STORAGE_KEY)
}

export function setStoredToken(token: string): void {
  setStoredValue(TOKEN_STORAGE_KEY, token, LEGACY_TOKEN_STORAGE_KEY)
}

export function clearToken(): void {
  clearStoredValue(TOKEN_STORAGE_KEY, LEGACY_TOKEN_STORAGE_KEY)
  analyticsReset()
}

/**
 * User-initiated sign out: clears the local token and (best-effort) the
 * httpOnly refresh cookie. The localStorage clear never depends on the
 * server call succeeding.
 */
export function signOut(): void {
  if (!isDevMode()) {
    fetch('/api/auth/signout', { method: 'POST' }).catch(() => {
      // Fire-and-forget — the cookie expires on its own after 30 days.
    })
  }
  clearToken()
}

export function isTokenValid(): boolean {
  if (isDevMode()) return true
  const token = getStoredToken()
  if (!token) return false
  try {
    const payload = decodeJwtPayload(token)
    return typeof payload.exp === 'number' && payload.exp * 1000 > Date.now()
  } catch {
    return false
  }
}

export function isLoggedIn(): boolean {
  if (isDevMode()) return true
  return !!getStoredToken()
}

/** Extract the user's role from the JWT. Returns ADMIN in dev mode. */
export function getUserRole(): 'ADMIN' | 'ANALYST' | 'VIEWER' {
  if (isDevMode()) return 'ADMIN'
  const token = getStoredToken()
  if (!token) return 'VIEWER'
  try {
    const payload = decodeJwtPayload(token)
    const role = payload['custom:role']
    if (role === 'ADMIN' || role === 'ANALYST' || role === 'VIEWER') return role
    return 'ANALYST' // default matches backend
  } catch {
    return 'VIEWER'
  }
}

/** Returns true if the current user can perform write operations. */
export function canWrite(): boolean {
  const role = getUserRole()
  return role === 'ADMIN' || role === 'ANALYST'
}

// ── Silent token refresh ──────────────────────────────────────────────────────
// The Cognito refresh token lives in an httpOnly cookie scoped to /api/auth;
// POST /api/auth/refresh exchanges it for a fresh ID token. In dev mode
// (no client id) there is no real token to refresh, so everything no-ops.

const TOKEN_EXPIRY_SKEW_MS = 2 * 60 * 1000

/** True when the stored token expires within ~2 minutes (or already has). */
export function isTokenExpiringSoon(): boolean {
  if (isDevMode()) return false
  const token = getStoredToken()
  if (!token) return false
  try {
    const payload = decodeJwtPayload(token)
    return (
      typeof payload.exp === 'number' && payload.exp * 1000 - Date.now() < TOKEN_EXPIRY_SKEW_MS
    )
  } catch {
    return false
  }
}

let refreshInFlight: Promise<string | null> | null = null

/**
 * Exchange the httpOnly refresh cookie for a new ID token. Stores and returns
 * the new token on success; returns null on any failure (no cookie, revoked
 * refresh token, network error). Single-flight: concurrent callers share one
 * in-flight request so parallel 401s trigger exactly one refresh.
 */
export function refreshToken(): Promise<string | null> {
  if (isDevMode()) return Promise.resolve(null)
  if (refreshInFlight) return refreshInFlight
  refreshInFlight = (async () => {
    try {
      const res = await fetch('/api/auth/refresh', { method: 'POST' })
      if (!res.ok) return null
      const data = await res.json().catch(() => null)
      const token = data?.token
      if (typeof token !== 'string' || !token) return null
      setStoredToken(token)
      return token
    } catch {
      return null
    } finally {
      refreshInFlight = null
    }
  })()
  return refreshInFlight
}

// ── Sign in via server-side proxy ─────────────────────────────────────────────
// Routes through /api/auth/signin to avoid browser CORS issues with Cognito.

export type SignInResult =
  | { kind: 'ok' }
  | { kind: 'new_password_required'; session: string }
  | { kind: 'mfa_setup'; session: string }
  | { kind: 'mfa_challenge'; session: string }

export async function signIn(email: string, password: string): Promise<SignInResult> {
  if (isDevMode()) {
    setStoredToken('dev-token')
    return { kind: 'ok' }
  }

  const res = await fetch('/api/auth/signin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })

  const data = await res.json()

  if (!res.ok) {
    throw new Error(data.error || 'Sign in failed')
  }

  if (data.challenge === 'NEW_PASSWORD_REQUIRED') {
    return { kind: 'new_password_required', session: data.session }
  }
  if (data.challenge === 'MFA_SETUP') {
    return { kind: 'mfa_setup', session: data.session }
  }
  if (data.challenge === 'SOFTWARE_TOKEN_MFA') {
    return { kind: 'mfa_challenge', session: data.session }
  }

  finishSignIn(data.token)
  return { kind: 'ok' }
}

/** Store the ID token and fire analytics identify (no-op in gov). */
function finishSignIn(token: string): void {
  setStoredToken(token)
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
    identify(payload.sub, { email: payload.email, orgId: payload['custom:orgId'], role: payload['custom:role'] })
  } catch { /* ignore parse errors */ }
}

// ── TOTP MFA ────────────────────────────────────────────────────────────────

/**
 * Begin TOTP enrollment for an MFA_SETUP challenge. Returns the shared secret
 * (for manual entry into an authenticator app) and a fresh session to pass to
 * verifyMfaSetup. Shared by the app and admin flows (client-agnostic).
 */
export async function associateSoftwareToken(
  session: string,
): Promise<{ secretCode: string; session: string }> {
  const res = await fetch('/api/auth/mfa/associate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ session }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Failed to start MFA setup')
  return { secretCode: data.secretCode, session: data.session }
}

/** Complete TOTP enrollment (app flow); stores the ID token on success. */
export async function verifyMfaSetup(email: string, code: string, session: string): Promise<void> {
  const res = await fetch('/api/auth/mfa/verify-setup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, session, client: 'app' }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Failed to verify authenticator')
  finishSignIn(data.token)
}

/** Answer a returning-user MFA challenge (app flow); stores the token. */
export async function respondMfaChallenge(email: string, code: string, session: string): Promise<void> {
  const res = await fetch('/api/auth/mfa/challenge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, session, client: 'app' }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Invalid authenticator code')
  finishSignIn(data.token)
}

export async function forgotPassword(email: string): Promise<void> {
  if (isDevMode()) return
  const res = await fetch('/api/auth/forgot-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || 'Failed to send reset code')
  }
}

export async function resetPassword(
  email: string,
  code: string,
  newPassword: string,
): Promise<void> {
  if (isDevMode()) return
  const res = await fetch('/api/auth/reset-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, newPassword }),
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || 'Failed to reset password')
  }
}

export type NewPasswordResult =
  | { kind: 'ok' }
  | { kind: 'mfa_setup'; session: string }
  | { kind: 'mfa_challenge'; session: string }

export async function completeNewPassword(
  email: string,
  newPassword: string,
  session: string,
): Promise<NewPasswordResult> {
  const res = await fetch('/api/auth/new-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, newPassword, session }),
  })

  const data = await res.json()

  if (!res.ok) {
    throw new Error(data.error || 'Failed to set new password')
  }

  // First login can chain straight into MFA enrollment.
  if (data.challenge === 'MFA_SETUP') {
    return { kind: 'mfa_setup', session: data.session }
  }
  if (data.challenge === 'SOFTWARE_TOKEN_MFA') {
    return { kind: 'mfa_challenge', session: data.session }
  }

  setStoredToken(data.token)
  return { kind: 'ok' }
}
