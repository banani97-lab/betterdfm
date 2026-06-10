// Hand-rolled toast emitter. Deliberately framework-free (no React imports) so
// it can be called from anywhere — components, lib code, event handlers. The
// renderer (`src/components/ui/Toaster.tsx`) subscribes via subscribeToToasts.

export type ToastKind = 'success' | 'error' | 'info'

export interface ToastEvent {
  id: number
  kind: ToastKind
  message: string
  createdAt: number
}

type ToastListener = (toast: ToastEvent) => void

let nextId = 1
const listeners = new Set<ToastListener>()

function emit(kind: ToastKind, message: string): void {
  // No listeners (SSR, or Toaster not mounted) — drop the event, never crash.
  if (listeners.size === 0) return
  const event: ToastEvent = { id: nextId++, kind, message, createdAt: Date.now() }
  for (const listener of listeners) listener(event)
}

export const toast = {
  success: (message: string) => emit('success', message),
  error: (message: string) => emit('error', message),
  info: (message: string) => emit('info', message),
}

/** Register a toast listener. Returns an unsubscribe function. */
export function subscribeToToasts(fn: ToastListener): () => void {
  listeners.add(fn)
  return () => {
    listeners.delete(fn)
  }
}
