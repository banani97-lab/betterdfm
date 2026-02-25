'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

// This route is no longer used — auth is handled via the login form directly.
// Kept to redirect stale bookmarks gracefully.
export default function CallbackPage() {
  const router = useRouter()
  useEffect(() => { router.replace('/login') }, [router])
  return null
}
