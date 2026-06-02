import { getJob, getBatch, type AnalysisJob, type BatchDetail } from './api'

const POLL_INTERVAL_MS = 3000
const POLL_TIMEOUT_MS = 5 * 60 * 1000 // 5 minutes

/** Thrown when a poll exceeds its timeout without reaching a terminal state. */
export class PollTimeoutError extends Error {
  constructor(message = 'Timed out waiting for analysis to finish. It may still complete in the background.') {
    super(message)
    this.name = 'PollTimeoutError'
  }
}

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))

interface PollOpts<T> {
  onUpdate?: (value: T) => void
  intervalMs?: number
  timeoutMs?: number
}

/**
 * Polls a job until it reaches a terminal state (DONE or FAILED), invoking
 * onUpdate on every fetch so callers can show live status. Throws
 * PollTimeoutError if the job is still pending past the timeout, so a stuck
 * job surfaces an error instead of spinning the UI forever.
 */
export async function pollJobUntilDone(jobId: string, opts: PollOpts<AnalysisJob> = {}): Promise<AnalysisJob> {
  const interval = opts.intervalMs ?? POLL_INTERVAL_MS
  const timeout = opts.timeoutMs ?? POLL_TIMEOUT_MS
  const start = Date.now()
  let job = await getJob(jobId)
  opts.onUpdate?.(job)
  while (job.status === 'PENDING' || job.status === 'PROCESSING') {
    if (Date.now() - start > timeout) throw new PollTimeoutError()
    await sleep(interval)
    job = await getJob(jobId)
    opts.onUpdate?.(job)
  }
  return job
}

/** Polls a batch until terminal (DONE, PARTIAL_FAIL, or FAILED). See pollJobUntilDone. */
export async function pollBatchUntilDone(batchId: string, opts: PollOpts<BatchDetail> = {}): Promise<BatchDetail> {
  const interval = opts.intervalMs ?? POLL_INTERVAL_MS
  const timeout = opts.timeoutMs ?? POLL_TIMEOUT_MS
  const start = Date.now()
  let data = await getBatch(batchId)
  opts.onUpdate?.(data)
  while (data.batch.status === 'PENDING' || data.batch.status === 'PROCESSING') {
    if (Date.now() - start > timeout) throw new PollTimeoutError()
    await sleep(interval)
    data = await getBatch(batchId)
    opts.onUpdate?.(data)
  }
  return data
}
