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
