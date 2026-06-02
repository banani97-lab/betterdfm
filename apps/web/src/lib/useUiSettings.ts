'use client'

import { useCallback, useEffect, useState } from 'react'
import { UI_SETTINGS_STORAGE_KEY } from '@/lib/branding'

export type BackgroundStyle = 'spotlight' | 'studio' | 'grid' | 'aurora'
export type TableDensity = 'comfortable' | 'compact'

export interface UiSettings {
  background: BackgroundStyle
  tableDensity: TableDensity
}

export const DEFAULT_UI_SETTINGS: UiSettings = {
  background: 'studio',
  tableDensity: 'comfortable',
}

// Dispatched on the window when settings change so every mounted hook instance
// (e.g. the taskbar's modal and the dashboard's layout) re-reads in sync,
// without a global provider. `storage` covers cross-tab updates.
const CHANGE_EVENT = 'rapiddfm:ui-settings-changed'

function normalize(raw: unknown): UiSettings {
  const parsed = (raw ?? {}) as Partial<UiSettings> & { background?: string }
  const bg = parsed.background
  const background: BackgroundStyle =
    bg === 'spotlight' || bg === 'studio' || bg === 'grid' || bg === 'aurora'
      ? bg
      : bg === 'default'
        ? 'spotlight' // legacy value
        : DEFAULT_UI_SETTINGS.background
  return {
    background,
    tableDensity: parsed.tableDensity === 'compact' ? 'compact' : 'comfortable',
  }
}

function readSettings(): UiSettings {
  if (typeof window === 'undefined') return DEFAULT_UI_SETTINGS
  try {
    const raw = localStorage.getItem(UI_SETTINGS_STORAGE_KEY)
    return raw ? normalize(JSON.parse(raw)) : DEFAULT_UI_SETTINGS
  } catch {
    return DEFAULT_UI_SETTINGS
  }
}

function applyBackground(background: BackgroundStyle) {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-ui-bg', background)
  }
}

// useUiSettings centralizes the workspace preferences (background + submissions
// layout) shared by the taskbar settings panel and any page that reacts to
// them. The root layout already applies the saved background on first paint;
// this hook keeps it in sync after changes and on navigation. Starts from the
// default and reconciles with localStorage on mount to avoid hydration drift.
export function useUiSettings() {
  const [settings, setSettings] = useState<UiSettings>(DEFAULT_UI_SETTINGS)

  useEffect(() => {
    const sync = () => {
      const next = readSettings()
      setSettings(next)
      applyBackground(next.background)
    }
    sync()
    window.addEventListener(CHANGE_EVENT, sync)
    window.addEventListener('storage', sync)
    return () => {
      window.removeEventListener(CHANGE_EVENT, sync)
      window.removeEventListener('storage', sync)
    }
  }, [])

  const update = useCallback((patch: Partial<UiSettings>) => {
    const next = { ...readSettings(), ...patch }
    try {
      localStorage.setItem(UI_SETTINGS_STORAGE_KEY, JSON.stringify(next))
    } catch {
      // localStorage unavailable (private mode) — keep the in-memory value.
    }
    applyBackground(next.background)
    setSettings(next)
    window.dispatchEvent(new Event(CHANGE_EVENT))
  }, [])

  return { settings, update }
}
