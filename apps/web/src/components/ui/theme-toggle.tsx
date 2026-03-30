'use client'

import { useEffect, useState } from 'react'
import { Moon, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { getStoredValue, LEGACY_THEME_STORAGE_KEY, setStoredValue, THEME_STORAGE_KEY } from '@/lib/branding'
import { cn } from '@/lib/utils'

type Theme = 'light' | 'dark'

interface ThemeToggleProps {
  className?: string
}

export function ThemeToggle({ className }: ThemeToggleProps) {
  const [theme, setTheme] = useState<Theme>('light')
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    const stored = getStoredValue(THEME_STORAGE_KEY, LEGACY_THEME_STORAGE_KEY)
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    const initial: Theme = stored === 'dark' || stored === 'light' ? stored : prefersDark ? 'dark' : 'light'
    document.documentElement.classList.toggle('dark', initial === 'dark')
    setTheme(initial)
    setMounted(true)
  }, [])

  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark'
    setTheme(next)
    document.documentElement.classList.toggle('dark', next === 'dark')
    setStoredValue(THEME_STORAGE_KEY, next, LEGACY_THEME_STORAGE_KEY)
  }

  if (!mounted) return null

  return (
    <Button
      variant="outline"
      size="icon"
      onClick={toggleTheme}
      className={cn('shrink-0', className)}
      aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
      title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
    >
      {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  )
}
