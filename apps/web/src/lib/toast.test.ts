import { describe, it, expect, vi } from 'vitest'
import { toast, subscribeToToasts, type ToastEvent } from './toast'
import { friendlyReason } from './errors'
import { ApiError } from './api'

describe('toast emitter', () => {
  it('delivers emitted toasts to subscribers with kind, message, and timestamps', () => {
    const received: ToastEvent[] = []
    const unsubscribe = subscribeToToasts((t) => received.push(t))

    toast.success('Project created')
    toast.error("Couldn't create project — server error, try again")
    toast.info('Re-analysis started')

    expect(received).toHaveLength(3)
    expect(received[0]).toMatchObject({ kind: 'success', message: 'Project created' })
    expect(received[1].kind).toBe('error')
    expect(received[2].kind).toBe('info')
    expect(received[0].createdAt).toBeTypeOf('number')
    unsubscribe()
  })

  it('assigns monotonically increasing ids', () => {
    const received: ToastEvent[] = []
    const unsubscribe = subscribeToToasts((t) => received.push(t))

    toast.info('first')
    toast.info('second')
    toast.info('third')

    expect(received[1].id).toBeGreaterThan(received[0].id)
    expect(received[2].id).toBeGreaterThan(received[1].id)
    unsubscribe()
  })

  it('stops delivering after unsubscribe', () => {
    const fn = vi.fn()
    const unsubscribe = subscribeToToasts(fn)

    toast.success('before')
    unsubscribe()
    toast.success('after')

    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('is a no-op with no listeners (never throws)', () => {
    expect(() => toast.error('nobody is listening')).not.toThrow()
  })

  it('fans out to multiple subscribers independently', () => {
    const a = vi.fn()
    const b = vi.fn()
    const unsubA = subscribeToToasts(a)
    const unsubB = subscribeToToasts(b)

    toast.info('hello')
    unsubA()
    toast.info('again')

    expect(a).toHaveBeenCalledTimes(1)
    expect(b).toHaveBeenCalledTimes(2)
    unsubB()
  })
})

describe('friendlyReason', () => {
  it('maps ApiError statuses to short phrases, never the raw message', () => {
    expect(friendlyReason(new ApiError('API /x: 402 quota', 402))).toBe('analysis limit reached')
    expect(friendlyReason(new ApiError('API /x: 403 nope', 403))).toBe("you don't have permission")
    expect(friendlyReason(new ApiError('API /x: 404 gone', 404))).toBe('not found')
    expect(friendlyReason(new ApiError('API /x: 429 slow down', 429))).toBe('too many requests, try again shortly')
    expect(friendlyReason(new ApiError('API /x: 502 Bad Gateway', 502))).toBe('server error, try again')
  })

  it('treats network failures as server errors and falls back generically', () => {
    expect(friendlyReason(new TypeError('Failed to fetch'))).toBe('server error, try again')
    expect(friendlyReason(new Error('whatever'))).toBe('something went wrong, try again')
    expect(friendlyReason('not even an error')).toBe('something went wrong, try again')
  })

  it('never leaks API paths or status codes', () => {
    const reasons = [402, 403, 404, 429, 500, 503].map((s) =>
      friendlyReason(new ApiError(`API /jobs/abc: ${s} internal detail`, s))
    )
    for (const r of reasons) {
      expect(r).not.toMatch(/API \//)
      expect(r).not.toMatch(/\d{3}/)
    }
  })
})
