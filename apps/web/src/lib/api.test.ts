import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { Violation } from './api'

// Mock auth module
vi.mock('./auth', () => ({
  getStoredToken: vi.fn(() => null),
  clearToken: vi.fn(),
  isDevMode: vi.fn(() => false),
  isTokenExpiringSoon: vi.fn(() => false),
  refreshToken: vi.fn(async () => null),
}))

import { getStoredToken, clearToken, isDevMode, isTokenExpiringSoon, refreshToken } from './auth'

describe('apiFetch', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
    // jsdom's window.location is unforgeable; vitest copies it onto the
    // global object as a regular property, so stubGlobal works.
    vi.stubGlobal('location', { replace: vi.fn(), pathname: '/dashboard' })
    vi.mocked(isDevMode).mockReturnValue(false)
    vi.mocked(isTokenExpiringSoon).mockReturnValue(false)
    vi.mocked(refreshToken).mockResolvedValue(null)
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

  it('on 401, refreshes the token and retries the request once with the new token', async () => {
    vi.mocked(getStoredToken).mockReturnValue('stale-token')
    vi.mocked(refreshToken).mockResolvedValue('new-token')
    const mockFetch = vi.mocked(fetch)
    mockFetch
      .mockResolvedValueOnce(new Response('Unauthorized', { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: 'job-1' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      )

    const { getJob } = await import('./api')
    const job = await getJob('job-1')

    expect(job).toEqual({ id: 'job-1' })
    expect(refreshToken).toHaveBeenCalledOnce()
    expect(mockFetch).toHaveBeenCalledTimes(2)
    const [, retryInit] = mockFetch.mock.calls[1]
    const retryHeaders = (retryInit as RequestInit)?.headers as Record<string, string>
    expect(retryHeaders?.Authorization).toBe('Bearer new-token')
    expect(clearToken).not.toHaveBeenCalled()
    expect(window.location.replace).not.toHaveBeenCalled()
  })

  it('on 401, clears token and redirects to /login when refresh fails', async () => {
    vi.mocked(getStoredToken).mockReturnValue('stale-token')
    vi.mocked(refreshToken).mockResolvedValue(null)
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(new Response('Unauthorized', { status: 401 }))

    const { getJob } = await import('./api')
    await expect(getJob('job-1')).rejects.toMatchObject({ name: 'ApiError', status: 401 })

    expect(refreshToken).toHaveBeenCalledOnce()
    expect(mockFetch).toHaveBeenCalledTimes(1) // no retry without a new token
    expect(clearToken).toHaveBeenCalled()
    expect(window.location.replace).toHaveBeenCalledWith('/login')
  })

  it('on 401, retries only once even if the retry also 401s', async () => {
    vi.mocked(getStoredToken).mockReturnValue('stale-token')
    vi.mocked(refreshToken).mockResolvedValue('new-token')
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(new Response('Unauthorized', { status: 401 }))

    const { getJob } = await import('./api')
    await expect(getJob('job-1')).rejects.toMatchObject({ name: 'ApiError', status: 401 })

    expect(refreshToken).toHaveBeenCalledOnce()
    expect(mockFetch).toHaveBeenCalledTimes(2)
    expect(clearToken).toHaveBeenCalled()
    expect(window.location.replace).toHaveBeenCalledWith('/login')
  })

  it('does not attempt refresh in dev mode', async () => {
    vi.mocked(getStoredToken).mockReturnValue('dev-token')
    vi.mocked(isDevMode).mockReturnValue(true)
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(new Response('Unauthorized', { status: 401 }))

    const { getJob } = await import('./api')
    await expect(getJob('job-1')).rejects.toMatchObject({ name: 'ApiError', status: 401 })

    expect(refreshToken).not.toHaveBeenCalled()
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('proactively refreshes before the request when the token is expiring soon', async () => {
    vi.mocked(getStoredToken).mockReturnValue('almost-expired')
    vi.mocked(isTokenExpiringSoon).mockReturnValue(true)
    vi.mocked(refreshToken).mockResolvedValue('fresh-token')
    const mockFetch = vi.mocked(fetch)
    mockFetch.mockResolvedValue(
      new Response(JSON.stringify({ id: 'job-1' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )

    const { getJob } = await import('./api')
    await getJob('job-1')

    expect(refreshToken).toHaveBeenCalledOnce()
    expect(mockFetch).toHaveBeenCalledOnce()
    const [, init] = mockFetch.mock.calls[0]
    const headers = (init as RequestInit)?.headers as Record<string, string>
    expect(headers?.Authorization).toBe('Bearer fresh-token')
  })

  it('Violation type has x2 and y2 fields (compile-time check)', () => {
    // This is a compile-time check; if the type lacks x2/y2, TypeScript will error.
    const v: Violation = {
      id: '1', orgId: 'org1', jobId: 'j1', ruleId: 'clearance', severity: 'ERROR',
      layer: 'top_copper', x: 1, y: 2, message: 'msg', suggestion: 'sug',
      count: 1, measuredMM: 0.05, limitMM: 0.1, unit: 'mm',
      netName: '', refDes: '', x2: 3, y2: 4, ignored: false,
    }
    expect(v.x2).toBe(3)
    expect(v.y2).toBe(4)
  })
})
