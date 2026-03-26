const CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''

const TOKEN_KEY = 'betterdfm_token'

// ── Dev mode ──────────────────────────────────────────────────────────────────

export function isDevMode(): boolean {
  return !CLIENT_ID
}

// ── Token storage ─────────────────────────────────────────────────────────────

export function getStoredToken(): string | null {
  if (typeof window === 'undefined') return null
  return localStorage.getItem(TOKEN_KEY)
}

export function setStoredToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export function isTokenValid(): boolean {
  if (isDevMode()) return true
  const token = getStoredToken()
  if (!token) return false
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
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
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
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

// ── Sign in via server-side proxy ─────────────────────────────────────────────
// Routes through /api/auth/signin to avoid browser CORS issues with Cognito.

export type SignInResult =
  | { kind: 'ok' }
  | { kind: 'new_password_required'; session: string }

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

  setStoredToken(data.token)
  return { kind: 'ok' }
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

export async function completeNewPassword(
  email: string,
  newPassword: string,
  session: string,
): Promise<void> {
  const res = await fetch('/api/auth/new-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, newPassword, session }),
  })

  const data = await res.json()

  if (!res.ok) {
    throw new Error(data.error || 'Failed to set new password')
  }

  setStoredToken(data.token)
}
