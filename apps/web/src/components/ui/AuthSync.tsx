'use client'

import { useEffect } from 'react'
import { TOKEN_STORAGE_KEY } from '@/lib/branding'
import { isDevMode } from '@/lib/auth'

// Paths where a missing token is normal — never redirect from these.
// (/admin has its own token under a different storage key.)
const PUBLIC_PATH_PREFIXES = ['/login', '/landing', '/share', '/admin']

function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATH_PREFIXES.some((p) => pathname === p || pathname.startsWith(`${p}/`))
}

/**
 * Cross-tab logout sync. Invisible client component mounted once in the root
 * layout: when another tab removes the auth token from localStorage (sign
 * out), redirect this tab to /login too. Token *changes* (another tab
 * refreshed it) need no action — the next getStoredToken() picks them up.
 */
export function AuthSync() {
  useEffect(() => {
    if (isDevMode()) return

    const onStorage = (e: StorageEvent) => {
      if (e.key !== TOKEN_STORAGE_KEY) return
      // Only react to a real token being removed — ignore writes/refreshes
      // and ignore events when this tab was never authenticated.
      if (!e.oldValue || e.newValue !== null) return
      if (isPublicPath(window.location.pathname)) return
      window.location.replace('/login')
    }

    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  return null
}
