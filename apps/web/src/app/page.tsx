'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { isTokenValid } from '@/lib/auth'

export default function RootPage() {
  const router = useRouter()

  useEffect(() => {
    router.replace(isTokenValid() ? '/dashboard' : '/login')
  }, [router])

  return null
}
