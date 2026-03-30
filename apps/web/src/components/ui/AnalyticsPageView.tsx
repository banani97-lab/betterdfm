'use client'
import { useEffect } from 'react'
import { usePathname } from 'next/navigation'
import { track } from '@/lib/analytics'

export function AnalyticsPageView() {
  const pathname = usePathname()
  useEffect(() => {
    track('Page Viewed', { path: pathname })
  }, [pathname])
  return null
}
