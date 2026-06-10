import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { AnalysisJob, BatchDetail } from './api'

// Mock only the fetchers; keep the real ApiError class so instanceof checks work.
vi.mock('./api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api')>()
  return { ...actual, getJob: vi.fn(), getBatch: vi.fn() }
})

import { getJob, getBatch, ApiError } from './api'
import { pollJobUntilDone, pollBatchUntilDone, PollTimeoutError } from './poll'

const job = (status: AnalysisJob['status']): AnalysisJob =>
  ({ id: 'j1', orgId: 'o1', submissionId: 's1', profileId: 'p1', status, mfgScore: 0, mfgGrade: '' })

const batch = (status: BatchDetail['batch']['status']): BatchDetail => ({
  batch: {
    id: 'b1', orgId: 'o1', userId: 'u1', status, total: 1, completed: 0, failed: 0,
    createdAt: '', updatedAt: '',
  },
  submissions: [],
  avgScore: null,
})

// Tiny real delays keep the tests fast without fake-timer plumbing.
const fastOpts = { intervalMs: 1, retryDelayMs: 1 }

describe('pollJobUntilDone', () => {
  beforeEach(() => {
    vi.mocked(getJob).mockReset()
  })

  it('resolves when the job reaches a terminal state', async () => {
    vi.mocked(getJob)
      .mockResolvedValueOnce(job('PENDING'))
      .mockResolvedValueOnce(job('PROCESSING'))
      .mockResolvedValueOnce(job('DONE'))

    const onUpdate = vi.fn()
    const result = await pollJobUntilDone('j1', { ...fastOpts, onUpdate })

    expect(result.status).toBe('DONE')
    expect(onUpdate).toHaveBeenCalledTimes(3)
  })

  it('retries transient network errors and recovers', async () => {
    vi.mocked(getJob)
      .mockResolvedValueOnce(job('PENDING'))
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce(job('DONE'))

    const result = await pollJobUntilDone('j1', fastOpts)
    expect(result.status).toBe('DONE')
    expect(getJob).toHaveBeenCalledTimes(4)
  })

  it('retries 5xx ApiErrors', async () => {
    vi.mocked(getJob)
      .mockRejectedValueOnce(new ApiError('API /jobs/j1: 502 Bad Gateway', 502))
      .mockResolvedValueOnce(job('DONE'))

    const result = await pollJobUntilDone('j1', fastOpts)
    expect(result.status).toBe('DONE')
    expect(getJob).toHaveBeenCalledTimes(2)
  })

  it('gives up after exhausting retries and rethrows the last error', async () => {
    vi.mocked(getJob).mockRejectedValue(new TypeError('Failed to fetch'))

    await expect(pollJobUntilDone('j1', fastOpts)).rejects.toThrow('Failed to fetch')
    // 1 initial attempt + 3 retries
    expect(getJob).toHaveBeenCalledTimes(4)
  })

  it('does not retry ApiError 404 — that is a real answer', async () => {
    vi.mocked(getJob).mockRejectedValue(new ApiError('API /jobs/j1: 404 not found', 404))

    await expect(pollJobUntilDone('j1', fastOpts)).rejects.toThrow('404')
    expect(getJob).toHaveBeenCalledTimes(1)
  })

  it('does not retry ApiError 403', async () => {
    vi.mocked(getJob).mockRejectedValue(new ApiError('forbidden', 403))

    await expect(pollJobUntilDone('j1', fastOpts)).rejects.toThrow('forbidden')
    expect(getJob).toHaveBeenCalledTimes(1)
  })

  it('throws PollTimeoutError (distinguishable from failures) when the job never finishes', async () => {
    vi.mocked(getJob).mockResolvedValue(job('PROCESSING'))

    const err = await pollJobUntilDone('j1', { ...fastOpts, timeoutMs: 10 }).catch((e) => e)
    expect(err).toBeInstanceOf(PollTimeoutError)
    expect(err.name).toBe('PollTimeoutError')
  })

  it('returns FAILED jobs without throwing (caller decides how to render)', async () => {
    vi.mocked(getJob).mockResolvedValue(job('FAILED'))

    const result = await pollJobUntilDone('j1', fastOpts)
    expect(result.status).toBe('FAILED')
  })
})

describe('pollBatchUntilDone', () => {
  beforeEach(() => {
    vi.mocked(getBatch).mockReset()
  })

  it('resolves when the batch reaches a terminal state', async () => {
    vi.mocked(getBatch)
      .mockResolvedValueOnce(batch('PROCESSING'))
      .mockResolvedValueOnce(batch('PARTIAL_FAIL'))

    const result = await pollBatchUntilDone('b1', fastOpts)
    expect(result.batch.status).toBe('PARTIAL_FAIL')
  })

  it('retries transient errors mid-poll', async () => {
    vi.mocked(getBatch)
      .mockResolvedValueOnce(batch('PENDING'))
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce(batch('DONE'))

    const result = await pollBatchUntilDone('b1', fastOpts)
    expect(result.batch.status).toBe('DONE')
    expect(getBatch).toHaveBeenCalledTimes(3)
  })

  it('does not retry 4xx and throws PollTimeoutError on timeout', async () => {
    vi.mocked(getBatch).mockRejectedValue(new ApiError('gone', 404))
    await expect(pollBatchUntilDone('b1', fastOpts)).rejects.toThrow('gone')
    expect(getBatch).toHaveBeenCalledTimes(1)

    vi.mocked(getBatch).mockReset()
    vi.mocked(getBatch).mockResolvedValue(batch('PENDING'))
    await expect(pollBatchUntilDone('b1', { ...fastOpts, timeoutMs: 10 })).rejects.toBeInstanceOf(PollTimeoutError)
  })
})
