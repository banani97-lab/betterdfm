import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { Violation } from './api'

// Mock auth module
vi.mock('./auth', () => ({
  getStoredToken: vi.fn(() => null),
  clearToken: vi.fn(),
}))

import { getStoredToken } from './auth'

describe('apiFetch', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('sends Authorization header when token is present', async () => {
    vi.mocked(getStoredToken).mockReturnValue('test-token-123')
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(
      new Response(JSON.stringify({ id: '1' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )

    const { getJob } = await import('./api')
    await getJob('job-1')

    expect(mockFetch).toHaveBeenCalledOnce()
    const [, init] = mockFetch.mock.calls[0]
    const headers = (init as RequestInit)?.headers as Record<string, string>
    expect(headers?.Authorization).toBe('Bearer test-token-123')
  })

  it('throws on non-2xx response with status and message', async () => {
    vi.mocked(getStoredToken).mockReturnValue(null)
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(
      new Response('Not found', { status: 404 })
    )

    const { getJob } = await import('./api')
    await expect(getJob('bad-id')).rejects.toThrow('404')
  })

  it('returns undefined for 204 No Content', async () => {
    vi.mocked(getStoredToken).mockReturnValue(null)
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(new Response(null, { status: 204 }))

    const { deleteProfile } = await import('./api')
    const result = await deleteProfile('profile-1')
    expect(result).toBeUndefined()
  })

  it('Violation type has x2 and y2 fields (compile-time check)', () => {
    // This is a compile-time check; if the type lacks x2/y2, TypeScript will error.
    const v: Violation = {
      id: '1', jobId: 'j1', ruleId: 'clearance', severity: 'ERROR',
      layer: 'top_copper', x: 1, y: 2, message: 'msg', suggestion: 'sug',
      count: 1, measuredMM: 0.05, limitMM: 0.1, unit: 'mm',
      netName: '', refDes: '', x2: 3, y2: 4, ignored: false,
    }
    expect(v.x2).toBe(3)
    expect(v.y2).toBe(4)
  })
})
