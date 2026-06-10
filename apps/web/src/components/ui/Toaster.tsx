'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { AlertCircle, CheckCircle, Info, X } from 'lucide-react'
import { subscribeToToasts, type ToastEvent, type ToastKind } from '@/lib/toast'
import { cn } from '@/lib/utils'

const MAX_VISIBLE = 4
const DISMISS_MS: Record<ToastKind, number> = { success: 4000, info: 4000, error: 8000 }

const ICONS: Record<ToastKind, typeof Info> = {
  success: CheckCircle,
  error: AlertCircle,
  info: Info,
}

const ICON_CLASSES: Record<ToastKind, string> = {
  success: 'text-emerald-500',
  error: 'text-destructive',
  info: 'text-blue-500',
}

/**
 * Renders toasts emitted via `toast.*()` from src/lib/toast.ts. Mounted once
 * in the root layout. Newest at the bottom, max 4 visible, auto-dismissing
 * (4s for success/info, 8s for errors). An emit matching a visible toast's
 * kind+message refreshes that toast's timer instead of stacking a duplicate.
 */
export function Toaster() {
  const [toasts, setToasts] = useState<ToastEvent[]>([])
  // Mirror of `toasts` so the subscription callback and timers see current
  // state without re-subscribing on every change.
  const toastsRef = useRef<ToastEvent[]>([])
  const timersRef = useRef(new Map<number, ReturnType<typeof setTimeout>>())

  const update = useCallback((next: ToastEvent[]) => {
    toastsRef.current = next
    setToasts(next)
  }, [])

  const clearTimer = useCallback((id: number) => {
    const timer = timersRef.current.get(id)
    if (timer) {
      clearTimeout(timer)
      timersRef.current.delete(id)
    }
  }, [])

  const dismiss = useCallback(
    (id: number) => {
      clearTimer(id)
      update(toastsRef.current.filter((t) => t.id !== id))
    },
    [clearTimer, update]
  )

  const startTimer = useCallback(
    (id: number, kind: ToastKind) => {
      clearTimer(id)
      timersRef.current.set(id, setTimeout(() => dismiss(id), DISMISS_MS[kind]))
    },
    [clearTimer, dismiss]
  )

  useEffect(() => {
    const unsubscribe = subscribeToToasts((t) => {
      const dup = toastsRef.current.find((x) => x.kind === t.kind && x.message === t.message)
      if (dup) {
        // Duplicate of a visible toast — refresh its timer instead of stacking.
        startTimer(dup.id, dup.kind)
        return
      }
      let next = [...toastsRef.current, t]
      if (next.length > MAX_VISIBLE) {
        for (const dropped of next.slice(0, next.length - MAX_VISIBLE)) clearTimer(dropped.id)
        next = next.slice(next.length - MAX_VISIBLE)
      }
      update(next)
      startTimer(t.id, t.kind)
    })
    const timers = timersRef.current
    return () => {
      unsubscribe()
      for (const timer of timers.values()) clearTimeout(timer)
      timers.clear()
    }
  }, [startTimer, clearTimer, update])

  return (
    <div
      aria-live="polite"
      className="fixed bottom-4 right-4 z-[100] flex w-[min(24rem,calc(100vw-2rem))] flex-col gap-2 pointer-events-none"
    >
      {toasts.map((t) => {
        const Icon = ICONS[t.kind]
        return (
          <div
            key={t.id}
            role={t.kind === 'error' ? 'alert' : 'status'}
            className={cn(
              'pointer-events-auto flex items-start gap-2.5 bg-card border border-border rounded-lg shadow-lg px-4 py-3',
              t.kind === 'error' && 'border-destructive/40'
            )}
          >
            <Icon className={cn('h-4 w-4 mt-0.5 shrink-0', ICON_CLASSES[t.kind])} />
            <p className="flex-1 text-sm text-foreground leading-snug break-words">{t.message}</p>
            <button
              type="button"
              onClick={() => dismiss(t.id)}
              aria-label="Dismiss notification"
              className="shrink-0 rounded p-0.5 text-muted-foreground hover:text-foreground transition-colors"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
