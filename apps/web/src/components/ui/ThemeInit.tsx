'use client'

import { useEffect } from 'react'

const STORAGE_KEY = 'betterdfm-theme'

export function ThemeInit() {
  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    const useDark = stored === 'dark' || (stored !== 'light' && prefersDark)
    document.documentElement.classList.toggle('dark', useDark)
  }, [])

  return null
}
