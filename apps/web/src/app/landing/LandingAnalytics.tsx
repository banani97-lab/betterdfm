'use client'

import { useEffect, useRef, useCallback } from 'react'
import { track } from '@/lib/analytics'

/**
 * Tracks section views via IntersectionObserver and button clicks via delegation.
 * Drop this component anywhere in the landing page — it observes all
 * [data-section] elements and listens for clicks on [data-track-click] elements.
 */
export function LandingAnalytics() {
  const tracked = useRef<Set<string>>(new Set())

  // Section view tracking
  useEffect(() => {
    const sections = document.querySelectorAll('[data-section]')
    if (!sections.length) return

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          const name = (entry.target as HTMLElement).dataset.section
          if (!name || tracked.current.has(name)) continue
          tracked.current.add(name)
          track('Section Viewed', { section: name })
        }
      },
      { threshold: 0.3 }
    )

    sections.forEach((el) => observer.observe(el))
    return () => observer.disconnect()
  }, [])

  // Button click tracking via event delegation
  const handleClick = useCallback((e: MouseEvent) => {
    const el = (e.target as HTMLElement).closest('[data-track-click]') as HTMLElement | null
    if (!el) return
    const label = el.dataset.trackClick
    if (label) track('Button Clicked', { label })
  }, [])

  useEffect(() => {
    document.addEventListener('click', handleClick)
    return () => document.removeEventListener('click', handleClick)
  }, [handleClick])

  return null
}
