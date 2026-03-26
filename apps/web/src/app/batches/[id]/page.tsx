'use client'

import { useEffect, useState, useCallback } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { ArrowLeft, RefreshCw, CheckCircle, XCircle, Clock, Loader2 } from 'lucide-react'
import { getBatch, retryBatch, type BatchDetail, type BatchSubmission } from '@/lib/api'
import { isLoggedIn } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { BetterDFMLogo } from '@/components/ui/betterdfm-logo'
import { cn } from '@/lib/utils'

function gradeColor(grade: string): string {
  switch (grade) {
    case 'A': return 'text-green-500'
    case 'B': return 'text-blue-500'
    case 'C': return 'text-yellow-500'
    case 'D': return 'text-red-500'
    default: return 'text-muted-foreground'
  }
}

function statusBadge(status: string) {
  switch (status) {
    case 'DONE':
      return <span className="inline-flex items-center gap-1 text-xs font-medium text-green-600 bg-green-100 dark:bg-green-900/30 px-2 py-0.5 rounded-full"><CheckCircle className="h-3 w-3" /> Done</span>
    case 'FAILED':
      return <span className="inline-flex items-center gap-1 text-xs font-medium text-red-600 bg-red-100 dark:bg-red-900/30 px-2 py-0.5 rounded-full"><XCircle className="h-3 w-3" /> Failed</span>
    case 'ANALYZING':
    case 'PROCESSING':
      return <span className="inline-flex items-center gap-1 text-xs font-medium text-purple-600 bg-purple-100 dark:bg-purple-900/30 px-2 py-0.5 rounded-full"><Loader2 className="h-3 w-3 animate-spin" /> Analyzing</span>
    case 'PENDING':
    case 'UPLOADED':
      return <span className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground bg-muted px-2 py-0.5 rounded-full"><Clock className="h-3 w-3" /> Pending</span>
    case 'PARTIAL_FAIL':
      return <span className="inline-flex items-center gap-1 text-xs font-medium text-yellow-600 bg-yellow-100 dark:bg-yellow-900/30 px-2 py-0.5 rounded-full"><XCircle className="h-3 w-3" /> Partial Fail</span>
    default:
      return <span className="text-xs text-muted-foreground">{status}</span>
  }
}

export default function BatchDetailPage() {
  const params = useParams()
  const router = useRouter()
  const batchId = params.id as string
  const [data, setData] = useState<BatchDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [retrying, setRetrying] = useState(false)
  const [error, setError] = useState<string>('')

  const fetchBatch = useCallback(async () => {
    try {
      const result = await getBatch(batchId)
      setData(result)
      return result
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
      return null
    } finally {
      setLoading(false)
    }
  }, [batchId])

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }

    let cancelled = false
    const poll = async () => {
      const result = await fetchBatch()
      if (cancelled) return
      if (result && (result.batch.status === 'PENDING' || result.batch.status === 'PROCESSING')) {
        setTimeout(() => {
          if (!cancelled) poll()
        }, 3000)
      }
    }
    poll()
    return () => { cancelled = true }
  }, [router, fetchBatch])

  const handleRetry = async () => {
    setRetrying(true)
    try {
      await retryBatch(batchId)
      await fetchBatch()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setRetrying(false)
    }
  }

  const isProcessing = data?.batch.status === 'PENDING' || data?.batch.status === 'PROCESSING'
  const progressPct = data ? Math.round(((data.batch.completed + data.batch.failed) / data.batch.total) * 100) : 0

  return (
    <div className="min-h-screen bg-background">
      <header className="bg-card border-b px-6 py-5 flex items-center justify-between gap-4">
        <BetterDFMLogo />
        <h1 className="text-xl font-semibold text-foreground">Batch Results</h1>
      </header>

      <main className="max-w-4xl mx-auto px-6 py-10">
        <Link href="/dashboard" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6">
          <ArrowLeft className="h-4 w-4" /> Back to Dashboard
        </Link>

        {loading && (
          <div className="text-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground mx-auto" />
            <p className="text-sm text-muted-foreground mt-2">Loading batch...</p>
          </div>
        )}

        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 text-sm text-red-700 dark:text-red-400">
            {error}
          </div>
        )}

        {data && (
          <>
            {/* Summary cards */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
              <div className="bg-card border rounded-lg p-4">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Total Files</p>
                <p className="text-2xl font-bold text-foreground mt-1">{data.batch.total}</p>
              </div>
              <div className="bg-card border rounded-lg p-4">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Completed</p>
                <p className="text-2xl font-bold text-green-500 mt-1">{data.batch.completed}</p>
              </div>
              <div className="bg-card border rounded-lg p-4">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Failed</p>
                <p className={cn('text-2xl font-bold mt-1', data.batch.failed > 0 ? 'text-red-500' : 'text-foreground')}>
                  {data.batch.failed}
                </p>
              </div>
              <div className="bg-card border rounded-lg p-4">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Avg Score</p>
                <p className="text-2xl font-bold text-foreground mt-1">
                  {data.avgScore != null ? Math.round(data.avgScore) : '--'}
                </p>
              </div>
            </div>

            {/* Batch status + progress */}
            <div className="flex items-center gap-3 mb-6">
              {statusBadge(data.batch.status)}
              {isProcessing && (
                <div className="flex-1">
                  <div className="w-full bg-muted rounded-full h-2">
                    <div
                      className="bg-purple-600 h-2 rounded-full transition-all"
                      style={{ width: `${progressPct}%` }}
                    />
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    {data.batch.completed + data.batch.failed} / {data.batch.total} processed
                  </p>
                </div>
              )}
              {data.batch.failed > 0 && !isProcessing && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleRetry}
                  disabled={retrying}
                  className="ml-auto"
                >
                  <RefreshCw className={cn('h-4 w-4 mr-1', retrying && 'animate-spin')} />
                  Retry Failed ({data.batch.failed})
                </Button>
              )}
            </div>

            {/* Submissions table */}
            <div className="bg-card border rounded-lg overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b bg-muted/50">
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Filename</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Type</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Status</th>
                    <th className="text-right px-4 py-3 font-medium text-muted-foreground">Score</th>
                    <th className="text-right px-4 py-3 font-medium text-muted-foreground">Grade</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {data.submissions.map((sub: BatchSubmission) => (
                    <tr
                      key={sub.id}
                      className={cn(
                        'transition-colors',
                        sub.latestJobId ? 'cursor-pointer hover:bg-muted/50' : ''
                      )}
                      onClick={() => {
                        if (sub.latestJobId) router.push(`/results/${sub.latestJobId}`)
                      }}
                    >
                      <td className="px-4 py-3 font-medium truncate max-w-[200px]">{sub.filename}</td>
                      <td className="px-4 py-3 text-muted-foreground">
                        {sub.fileType === 'ODB_PLUS_PLUS' ? 'ODB++' : 'Gerber'}
                      </td>
                      <td className="px-4 py-3">{statusBadge(sub.jobStatus || sub.status)}</td>
                      <td className="px-4 py-3 text-right font-mono">
                        {sub.jobStatus === 'DONE' ? sub.mfgScore : '--'}
                      </td>
                      <td className={cn('px-4 py-3 text-right font-bold', gradeColor(sub.mfgGrade))}>
                        {sub.jobStatus === 'DONE' ? sub.mfgGrade : '--'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </main>
    </div>
  )
}
