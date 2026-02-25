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

export function isLoggedIn(): boolean {
  if (isDevMode()) return true
  return !!getStoredToken()
}

// ── Sign in via server-side proxy ─────────────────────────────────────────────
// Routes through /api/auth/signin to avoid browser CORS issues with Cognito.

export async function signIn(email: string, password: string): Promise<void> {
  if (isDevMode()) {
    setStoredToken('dev-token')
    return
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

  setStoredToken(data.token)
}
