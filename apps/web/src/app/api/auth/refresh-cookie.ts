import type { NextResponse } from 'next/server'

// The Cognito refresh token lives in an httpOnly cookie so it is never
// readable from JS, and is path-scoped to the auth routes so the browser
// only sends it to /api/auth/* (never to the Go API or page loads).
export const REFRESH_COOKIE_NAME = 'rapiddfm_refresh_token'

// Cognito's default refresh-token validity is 30 days.
const REFRESH_COOKIE_MAX_AGE_S = 60 * 60 * 24 * 30

function cookieOptions() {
  return {
    httpOnly: true,
    // Allow plain-http localhost in dev; require https everywhere else.
    secure: process.env.NODE_ENV !== 'development',
    sameSite: 'lax' as const,
    path: '/api/auth',
  }
}

export function setRefreshCookie(res: NextResponse, refreshToken: string): void {
  res.cookies.set(REFRESH_COOKIE_NAME, refreshToken, {
    ...cookieOptions(),
    maxAge: REFRESH_COOKIE_MAX_AGE_S,
  })
}

export function clearRefreshCookie(res: NextResponse): void {
  res.cookies.set(REFRESH_COOKIE_NAME, '', { ...cookieOptions(), maxAge: 0 })
}
