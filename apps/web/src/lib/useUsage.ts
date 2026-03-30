'use client'
import { useState, useEffect } from 'react'
import { getUsage, type UsageSummary } from './api'

let cached: UsageSummary | null = null

export function useUsage() {
  const [usage, setUsage] = useState<UsageSummary | null>(cached)
  const [loading, setLoading] = useState(!cached)

  useEffect(() => {
    if (cached) { setUsage(cached); setLoading(false); return }
    getUsage()
      .then(u => { cached = u; setUsage(u) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const refresh = () => {
    getUsage()
      .then(u => { cached = u; setUsage(u) })
      .catch(() => {})
  }

  return { usage, loading, refresh }
}
