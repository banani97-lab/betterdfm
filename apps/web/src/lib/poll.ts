import { getJob, getBatch, ApiError, type AnalysisJob, type BatchDetail } from './api'

const POLL_INTERVAL_MS = 3000
const POLL_TIMEOUT_MS = 5 * 60 * 1000 // 5 minutes
const MAX_FETCH_RETRIES = 3
const RETRY_BASE_DELAY_MS = 1000

/**
 * Thrown when a poll exceeds its timeout without reaching a terminal state.
 * Callers should treat this as "still running in the background", not as a
 * failure — the job may well complete after we stop watching it.
 */
export class PollTimeoutError extends Error {
  constructor(message = 'Timed out waiting for analysis to finish. It may still complete in the background.') {
    super(message)
    this.name = 'PollTimeoutError'
  }
}

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))

/**
 * 4xx responses are real answers (404 = gone, 403 = no access) — retrying
 * won't change them. Everything else (network blips, 5xx) is worth retrying.
 */
function isRetryable(err: unknown): boolean {
  if (err instanceof ApiError) return err.status >= 500
  return true
}

/**
 * Run a status fetch, retrying transient failures (network errors, 5xx) up to
 * MAX_FETCH_RETRIES times with linear backoff before rethrowing, so a single
 * dropped request mid-poll doesn't kill the whole flow.
 */
async function fetchWithRetry<T>(fn: () => Promise<T>, retryDelayMs: number): Promise<T> {
  let attempt = 0
  for (;;) {
    try {
      return await fn()
    } catch (err) {
      if (!isRetryable(err) || attempt >= MAX_FETCH_RETRIES) throw err
      attempt++
      await sleep(retryDelayMs * attempt)
    }
  }
}

interface PollOpts<T> {
  onUpdate?: (value: T) => void
  intervalMs?: number
  timeoutMs?: number
  /** Base delay between fetch retries; exposed for tests. */
  retryDelayMs?: number
}

/**
 * Polls a job until it reaches a terminal state (DONE or FAILED), invoking
 * onUpdate on every fetch so callers can show live status. Transient fetch
 * failures are retried (see fetchWithRetry). Throws PollTimeoutError if the
 * job is still pending past the timeout, so a stuck job surfaces a
 * distinguishable "still running" signal instead of spinning the UI forever.
 */
export async function pollJobUntilDone(jobId: string, opts: PollOpts<AnalysisJob> = {}): Promise<AnalysisJob> {
  const interval = opts.intervalMs ?? POLL_INTERVAL_MS
  const timeout = opts.timeoutMs ?? POLL_TIMEOUT_MS
  const retryDelay = opts.retryDelayMs ?? RETRY_BASE_DELAY_MS
  const start = Date.now()
  let job = await fetchWithRetry(() => getJob(jobId), retryDelay)
  opts.onUpdate?.(job)
  while (job.status === 'PENDING' || job.status === 'PROCESSING') {
    if (Date.now() - start > timeout) throw new PollTimeoutError()
    await sleep(interval)
    job = await fetchWithRetry(() => getJob(jobId), retryDelay)
    opts.onUpdate?.(job)
  }
  return job
}

/** Polls a batch until terminal (DONE, PARTIAL_FAIL, or FAILED). See pollJobUntilDone. */
export async function pollBatchUntilDone(batchId: string, opts: PollOpts<BatchDetail> = {}): Promise<BatchDetail> {
  const interval = opts.intervalMs ?? POLL_INTERVAL_MS
  const timeout = opts.timeoutMs ?? POLL_TIMEOUT_MS
  const retryDelay = opts.retryDelayMs ?? RETRY_BASE_DELAY_MS
  const start = Date.now()
  let data = await fetchWithRetry(() => getBatch(batchId), retryDelay)
  opts.onUpdate?.(data)
  while (data.batch.status === 'PENDING' || data.batch.status === 'PROCESSING') {
    if (Date.now() - start > timeout) throw new PollTimeoutError()
    await sleep(interval)
    data = await fetchWithRetry(() => getBatch(batchId), retryDelay)
    opts.onUpdate?.(data)
  }
  return data
}
