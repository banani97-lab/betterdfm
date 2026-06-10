import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { TOKEN_STORAGE_KEY } from './branding'

// Keep mixpanel out of the picture.
vi.mock('./analytics', () => ({
  identify: vi.fn(),
  reset: vi.fn(),
  track: vi.fn(),
}))

// auth.ts captures NEXT_PUBLIC_COGNITO_CLIENT_ID at module load, so each test
// stubs the env first and imports a fresh copy of the module.
async function importAuth() {
  vi.resetModules()
  return import('./auth')
}

function fakeJwt(payload: Record<string, unknown>): string {
  return `header.${btoa(JSON.stringify(payload))}.sig`
}

beforeEach(() => {
  vi.stubEnv('NEXT_PUBLIC_COGNITO_CLIENT_ID', 'test-client-id')
  vi.stubGlobal('fetch', vi.fn())
  localStorage.clear()
})

afterEach(() => {
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('refreshToken', () => {
  it('single-flight: concurrent callers share one in-flight request', async () => {
    const { refreshToken } = await importAuth()
    let resolveFetch!: (r: Response) => void
    vi.mocked(fetch).mockReturnValue(
      new Promise<Response>((resolve) => {
        resolveFetch = resolve
      })
    )

    const calls = [refreshToken(), refreshToken(), refreshToken(), refreshToken(), refreshToken()]
    expect(fetch).toHaveBeenCalledOnce()
    expect(fetch).toHaveBeenCalledWith('/api/auth/refresh', { method: 'POST' })

    resolveFetch(
      new Response(JSON.stringify({ token: 'fresh-token' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    const results = await Promise.all(calls)
    expect(results).toEqual(['fresh-token', 'fresh-token', 'fresh-token', 'fresh-token', 'fresh-token'])
    expect(localStorage.getItem(TOKEN_STORAGE_KEY)).toBe('fresh-token')
  })

  it('starts a new request once the previous one has settled', async () => {
    const { refreshToken } = await importAuth()
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ token: 't1' }), { status: 200 })
    )

    await refreshToken()
    await refreshToken()
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('returns null on a 401 response without storing anything', async () => {
    const { refreshToken } = await importAuth()
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: 'Session expired' }), { status: 401 })
    )

    expect(await refreshToken()).toBeNull()
    expect(localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
  })

  it('returns null on network errors', async () => {
    const { refreshToken } = await importAuth()
    vi.mocked(fetch).mockRejectedValue(new TypeError('network down'))

    expect(await refreshToken()).toBeNull()
  })

  it('no-ops in dev mode (empty client id): returns null without fetching', async () => {
    vi.stubEnv('NEXT_PUBLIC_COGNITO_CLIENT_ID', '')
    const { refreshToken } = await importAuth()

    expect(await refreshToken()).toBeNull()
    expect(fetch).not.toHaveBeenCalled()
  })
})

describe('isTokenExpiringSoon', () => {
  it('is true when exp is within the 2-minute skew', async () => {
    const auth = await importAuth()
    auth.setStoredToken(fakeJwt({ exp: Math.floor(Date.now() / 1000) + 60 }))
    expect(auth.isTokenExpiringSoon()).toBe(true)
  })

  it('is false when exp is comfortably in the future', async () => {
    const auth = await importAuth()
    auth.setStoredToken(fakeJwt({ exp: Math.floor(Date.now() / 1000) + 30 * 60 }))
    expect(auth.isTokenExpiringSoon()).toBe(false)
  })

  it('is false with no token, a malformed token, or in dev mode', async () => {
    const auth = await importAuth()
    expect(auth.isTokenExpiringSoon()).toBe(false)
    auth.setStoredToken('not-a-jwt')
    expect(auth.isTokenExpiringSoon()).toBe(false)

    vi.stubEnv('NEXT_PUBLIC_COGNITO_CLIENT_ID', '')
    const devAuth = await importAuth()
    devAuth.setStoredToken(fakeJwt({ exp: 0 }))
    expect(devAuth.isTokenExpiringSoon()).toBe(false)
  })
})

describe('signOut', () => {
  it('clears the local token even when the signout request fails', async () => {
    const auth = await importAuth()
    auth.setStoredToken('some-token')
    vi.mocked(fetch).mockRejectedValue(new TypeError('network down'))

    auth.signOut()

    expect(localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
    expect(fetch).toHaveBeenCalledWith('/api/auth/signout', { method: 'POST' })
  })

  it('does not call the signout route in dev mode', async () => {
    vi.stubEnv('NEXT_PUBLIC_COGNITO_CLIENT_ID', '')
    const auth = await importAuth()
    auth.setStoredToken('dev-token')

    auth.signOut()

    expect(localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
    expect(fetch).not.toHaveBeenCalled()
  })
})
