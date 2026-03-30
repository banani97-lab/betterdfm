'use client'

import { useEffect } from 'react'
import { getStoredValue, LEGACY_THEME_STORAGE_KEY, THEME_STORAGE_KEY } from '@/lib/branding'

export function ThemeInit() {
  useEffect(() => {
    const stored = getStoredValue(THEME_STORAGE_KEY, LEGACY_THEME_STORAGE_KEY)
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    const useDark = stored === 'dark' || (stored !== 'light' && prefersDark)
    document.documentElement.classList.toggle('dark', useDark)
  }, [])

  return null
}
