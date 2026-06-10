'use client'

import { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Download, AlertCircle, AlertTriangle, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, GitCompareArrows, Info, ListFilter } from 'lucide-react'
import { API_URL, ApiError, getJob, getViolations, getBoardData, fetchBoardFromUrl, fetchViolationsFromUrl, getSubmissions, getProjectSubmissions, patchViolation, ignoreLayerViolations, type AnalysisJob, type Submission, type Violation, type BoardData } from '@/lib/api'
import { isLoggedIn, canWrite, getStoredToken } from '@/lib/auth'
import { useUsage } from '@/lib/useUsage'
import { AppBackButton } from '@/components/ui/app-back-button'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ViolationList, type SeverityFilter } from '@/components/ui/ViolationList'
import { BoardViewer } from '@/components/ui/BoardViewer'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { AppTaskbar } from '@/components/ui/app-taskbar'
import { cn } from '@/lib/utils'
import { track } from '@/lib/analytics'

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
}

const JOB_POLL_MS = 4000

function describeLoadError(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.status === 404) return "This analysis doesn't exist or was deleted."
    if (e.status === 403) return "You don't have permission to view this analysis."
  }
  return e instanceof Error ? `Failed to load results: ${e.message}` : 'Failed to load results'
}

export default function ResultsPage() {
  const { jobId } = useParams<{ jobId: string }>()
  const router = useRouter()
  const [job, setJob] = useState<AnalysisJob | null>(null)
  const [violations, setViolations] = useState<Violation[]>([])
  const [boardData, setBoardData] = useState<BoardData | null>(null)
  const [selectedId, setSelectedId] = useState<string | undefined>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [boardError, setBoardError] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [hiddenLayers, setHiddenLayers] = useState<Set<string>>(new Set())
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>('ERROR')
  const [ruleFilter, setRuleFilter] = useState<Set<string>>(new Set())
  const [isPortraitMobile, setIsPortraitMobile] = useState(false)
  const [violationsOpen, setViolationsOpen] = useState(true)
  const [compareOpen, setCompareOpen] = useState(false)
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [currentProjectId, setCurrentProjectId] = useState<string | null>(null)
  const { usage } = useUsage()

  const toggleLayer = (name: string) => {
    setHiddenLayers((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name); else next.add(name)
      return next
    })
  }

  const handleIgnore = useCallback(async (v: Violation, ignored: boolean) => {
    setActionError(null)
    // Optimistic update — reverted below if the API call fails.
    setViolations((prev) => prev.map((x) => x.id === v.id ? { ...x, ignored } : x))
    try {
      const result = await patchViolation(v.id, { ignored })
      setJob((prev) => prev ? { ...prev, mfgScore: result.mfgScore, mfgGrade: result.mfgGrade } : prev)
    } catch {
      setViolations((prev) => prev.map((x) => x.id === v.id ? { ...x, ignored: v.ignored } : x))
      setActionError(`Couldn't ${ignored ? 'ignore' : 'restore'} the violation — the change was not saved. Please try again.`)
    }
  }, [])

  const handleIgnoreLayer = useCallback(async (layer: string, ignored: boolean, severity?: string) => {
    if (!job) return
    setActionError(null)
    const affects = (x: Violation) => x.layer === layer && (!severity || x.severity === severity)
    // Snapshot pre-change ignored flags so a failure can restore mixed states.
    const previous = new Map(violations.filter(affects).map((x) => [x.id, x.ignored]))
    // Optimistic update — reverted below if the API call fails.
    setViolations((prev) => prev.map((x) => affects(x) ? { ...x, ignored } : x))
    try {
      const result = await ignoreLayerViolations(job.id, layer, ignored, severity)
      setJob((prev) => prev ? { ...prev, mfgScore: result.mfgScore, mfgGrade: result.mfgGrade } : prev)
    } catch {
      setViolations((prev) => prev.map((x) => previous.has(x.id) ? { ...x, ignored: previous.get(x.id)! } : x))
      setActionError(`Couldn't update violations on layer "${layer}" — the change was not saved. Please try again.`)
    }
  }, [job, violations])

  const allIgnoredLayers = useMemo(() => {
    const counts = new Map<string, { total: number; ignored: number }>()
    for (const v of violations) {
      if (!v.layer) continue
      const c = counts.get(v.layer) ?? { total: 0, ignored: 0 }
      c.total++
      if (v.ignored) c.ignored++
      counts.set(v.layer, c)
    }
    const s = new Set<string>()
    for (const [layer, c] of counts) {
      if (c.total > 0 && c.total === c.ignored) s.add(layer)
    }
    return s
  }, [violations])

  // Known layer names from board data — used to check if a violation's layer is real
  const knownLayers = useMemo(() => {
    const s = new Set<string>()
    for (const l of boardData?.layers ?? []) s.add(l.name)
    return s
  }, [boardData])

  // Layer-filtered only — used for tab counts so they always show totals per severity.
  // Violations on non-real layers (e.g. "drill") hide when all real layers are hidden.
  const allHidden = knownLayers.size > 0 && hiddenLayers.size >= knownLayers.size
  const layerFiltered = violations.filter((v) => {
    if (!v.layer) return !allHidden
    if (knownLayers.has(v.layer)) return !hiddenLayers.has(v.layer)
    return !allHidden // non-real layer like "drill" — hide when everything is hidden
  })

  // Severity + layer filtered — used for display and board markers.
  const visibleViolations = layerFiltered.filter((v) => {
    if (severityFilter === 'NONE') return false
    return v.severity === severityFilter
  })

  // Sets used by BoardViewer layer panel to show ignore buttons.
  // Only layers with visible (non-ignored, current-severity) violations get the indicator.
  const violationLayers = useMemo(() => {
    const s = new Set<string>()
    for (const v of visibleViolations) {
      if (v.layer && !v.ignored) s.add(v.layer)
    }
    return s
  }, [visibleViolations])

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    let cancelled = false
    let pollTimer: ReturnType<typeof setInterval> | undefined

    // Full results load — only runs once the job is DONE.
    const loadResults = async (jobData: AnalysisJob) => {
      try {
        track('Analysis Viewed', { jobId, score: jobData.mfgScore, grade: jobData.mfgGrade })
        track('BoardViewer Started', { jobId })
        const boardStart = Date.now()
        // Kick off the board S3 fetch in parallel with violations so the
        // viewer isn't blocked waiting for the violations list. When the job
        // response inlines presigned URLs we skip the /board and /violations
        // round-trips entirely and go straight to S3.
        const boardP = jobData.boardUrl
          ? fetchBoardFromUrl(jobData.boardUrl)
          : getBoardData(jobId)
        boardP.then((bd) => {
          if (cancelled) return
          setBoardData(bd)
          setBoardError(false)
          track('BoardViewer Loaded', { jobId, durationMs: Date.now() - boardStart })
        }).catch(() => {
          if (cancelled) return
          // Board preview failure shouldn't block violations/scores — render a
          // placeholder in the viewer area instead of an infinite spinner.
          setBoardError(true)
          track('BoardViewer Failed', { jobId, durationMs: Date.now() - boardStart })
        })
        const violationsData = await (jobData.violationsUrl
          ? fetchViolationsFromUrl(jobData.violationsUrl)
          : getViolations(jobId))
        if (cancelled) return
        setViolations(violationsData ?? [])
      } catch (e: unknown) {
        if (!cancelled) setError(describeLoadError(e))
      }
    }

    const startPolling = () => {
      pollTimer = setInterval(async () => {
        try {
          const j = await getJob(jobId)
          if (cancelled) return
          setJob(j)
          if (j.status === 'DONE' || j.status === 'FAILED') {
            if (pollTimer) { clearInterval(pollTimer); pollTimer = undefined }
            if (j.status === 'DONE') loadResults(j)
          }
        } catch {
          // Transient poll error — keep polling; the next tick may succeed.
        }
      }, JOB_POLL_MS)
    }

    const load = async () => {
      try {
        const jobData = await getJob(jobId)
        if (cancelled) return
        setJob(jobData)
        // Find the current submission's projectId for compare scoping / back link
        getSubmissions().then(subs => {
          if (cancelled) return
          const current = subs?.find(s => s.id === jobData.submissionId)
          if (current?.projectId) setCurrentProjectId(current.projectId)
        }).catch(() => {})

        if (jobData.status === 'DONE') {
          await loadResults(jobData)
        } else if (jobData.status !== 'FAILED') {
          // PENDING / PROCESSING — poll until the job reaches a terminal state.
          startPolling()
        }
      } catch (e: unknown) {
        if (!cancelled) setError(describeLoadError(e))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
      if (pollTimer) clearInterval(pollTimer)
    }
  }, [jobId, router])

  // Load project submissions for compare dropdown
  useEffect(() => {
    if (!compareOpen || submissions.length > 0 || !currentProjectId) return
    getProjectSubmissions(currentProjectId).then(setSubmissions).catch(() => {})
  }, [compareOpen, submissions.length, currentProjectId])

  useEffect(() => {
    const orientationQuery = window.matchMedia('(orientation: portrait)')
    const mobileWidthQuery = window.matchMedia('(max-width: 900px)')
    const update = () => setIsPortraitMobile(orientationQuery.matches && mobileWidthQuery.matches)

    update()

    if (orientationQuery.addEventListener) {
      orientationQuery.addEventListener('change', update)
      mobileWidthQuery.addEventListener('change', update)
    } else {
      orientationQuery.addListener(update)
      mobileWidthQuery.addListener(update)
    }
    window.addEventListener('resize', update)

    return () => {
      if (orientationQuery.removeEventListener) {
        orientationQuery.removeEventListener('change', update)
        mobileWidthQuery.removeEventListener('change', update)
      } else {
        orientationQuery.removeListener(update)
        mobileWidthQuery.removeListener(update)
      }
      window.removeEventListener('resize', update)
    }
  }, [])

  useEffect(() => {
    setViolationsOpen(!isPortraitMobile)
  }, [isPortraitMobile])

  const downloadPDF = async () => {
    track('Report Exported', { jobId, format: 'pdf' })
    const token = getStoredToken()
    const res = await fetch(`${API_URL}/jobs/${jobId}/report.pdf`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (!res.ok) return
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `dfm-report-${jobId}.pdf`
    a.click()
    URL.revokeObjectURL(url)
  }

  const exportCSV = () => {
    track('Report Exported', { jobId, format: 'csv' })
    const header = 'id,ruleId,severity,layer,x,y,message,suggestion,measuredMM,limitMM,unit,netName,refDes,x2,y2\n'
    const rows = violations
      .map((v) =>
        [v.id, v.ruleId, v.severity, v.layer, v.x, v.y, `"${v.message}"`, `"${v.suggestion}"`,
         v.measuredMM, v.limitMM, v.unit, v.netName, v.refDes, v.x2, v.y2].join(',')
      )
      .join('\n')
    const blob = new Blob([header + rows], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `violations-${jobId}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  const errorCount = violations.filter((v) => v.severity === 'ERROR' && !v.ignored).length
  const warningCount = violations.filter((v) => v.severity === 'WARNING' && !v.ignored).length
  const infoCount = violations.filter((v) => v.severity === 'INFO' && !v.ignored).length
  const collapseToBottom = isPortraitMobile
  const backHref = currentProjectId ? `/projects/${currentProjectId}` : '/dashboard'
  const backLabel = currentProjectId ? 'Project' : 'Dashboard'

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin h-8 w-8 border-4 border-blue-600 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-4">
        <AlertCircle className="h-12 w-12 text-red-400" />
        <p className="text-muted-foreground">{error}</p>
        <AppBackButton href={backHref} label={backLabel} />
      </div>
    )
  }

  if (job?.status === 'FAILED') {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-4 px-4">
        <AlertCircle className="h-12 w-12 text-red-400" />
        <h1 className="text-lg font-semibold text-foreground">Analysis failed</h1>
        {job.errorMsg ? (
          <p className="text-sm text-muted-foreground max-w-md text-center font-mono break-words">{job.errorMsg}</p>
        ) : (
          <p className="text-sm text-muted-foreground max-w-md text-center">
            Something went wrong while analyzing this design. Try re-running the analysis from the dashboard.
          </p>
        )}
        <AppBackButton href={backHref} label={backLabel} />
      </div>
    )
  }

  if (job && job.status !== 'DONE') {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-4 px-4">
        <div className="animate-spin h-8 w-8 border-4 border-blue-600 border-t-transparent rounded-full" />
        <div className="text-center">
          <h1 className="text-lg font-semibold text-foreground">Analysis in progress</h1>
          <p className="text-sm text-muted-foreground mt-1">
            This page will update automatically when the analysis completes.
          </p>
        </div>
        <AppBackButton href={backHref} label={backLabel} />
      </div>
    )
  }

  return (
    <div className="flex flex-col min-h-screen h-[100dvh] md:h-screen bg-background">
      {/* Header */}
      <header className="bg-card border-b px-4 py-3 md:px-6 md:py-5 flex flex-wrap md:flex-nowrap items-center gap-3 md:gap-4 flex-shrink-0">
        <RapidDFMLogo className="shrink-0" />
        <div className="flex items-center gap-4 flex-1 min-w-0">
          <div className="min-w-0">
            <h1 className="text-lg font-semibold text-foreground">DFM Results</h1>
            <p className="text-xs text-muted-foreground font-mono truncate">{jobId}</p>
          </div>
        </div>
        <div className="order-2 md:order-3 ml-auto flex flex-wrap items-center justify-end gap-2">
          <AppBackButton href={backHref} label={backLabel} />
          {currentProjectId && usage?.features.compare !== false && <div className="relative">
            <Button variant="outline" className="h-10 px-3 md:h-11 md:px-4" onClick={() => setCompareOpen(o => !o)}>
              <GitCompareArrows className="h-4 w-4 mr-1" />Compare
            </Button>
            {compareOpen && (
              <div className="absolute right-0 top-full mt-1 z-50 w-72 bg-card border border-border rounded-lg shadow-lg py-1 max-h-64 overflow-y-auto">
                <p className="px-3 py-1.5 text-xs font-medium text-muted-foreground">Compare with another analysis:</p>
                {submissions
                  .filter(s => s.status === 'DONE' && s.latestJobId && s.latestJobId !== jobId)
                  .map(s => (
                    <button
                      key={s.id}
                      type="button"
                      className="w-full text-left px-3 py-2 hover:bg-muted/60 transition-colors flex items-center justify-between"
                      onClick={() => {
                        setCompareOpen(false)
                        router.push(`/compare?jobA=${jobId}&jobB=${s.latestJobId}`)
                      }}
                    >
                      <span className="text-sm truncate">{s.filename}</span>
                      {s.mfgScore > 0 && (
                        <span className="text-xs font-mono font-bold ml-2 shrink-0" style={{ color: scoreColor(s.mfgScore) }}>
                          {s.mfgScore} {s.mfgGrade}
                        </span>
                      )}
                    </button>
                  ))
                }
                {submissions.filter(s => s.status === 'DONE' && s.latestJobId && s.latestJobId !== jobId).length === 0 && (
                  <p className="px-3 py-2 text-xs text-muted-foreground">No other completed analyses in this project.</p>
                )}
              </div>
            )}
          </div>}
          <Button variant="outline" className="h-10 px-3 md:h-11 md:px-8" onClick={exportCSV} disabled={violations.length === 0}>
            <Download className="h-4 w-4 mr-1" />CSV
          </Button>
          <Button variant="outline" className="h-10 px-3 md:h-11 md:px-8" onClick={downloadPDF}>
            <Download className="h-4 w-4 mr-1" />PDF
          </Button>
          <AppTaskbar expandOnHover={false} />
        </div>
        {/* Summary badges */}
        <div className="order-3 md:order-2 w-full md:w-auto flex items-center gap-2">
          <div className="flex items-center gap-1">
            <AlertCircle className="h-4 w-4 text-red-500" />
            <Badge variant="destructive">{errorCount}</Badge>
          </div>
          <div className="flex items-center gap-1">
            <AlertTriangle className="h-4 w-4 text-yellow-500" />
            <Badge variant="warning">{warningCount}</Badge>
          </div>
          <div className="flex items-center gap-1">
            <Info className="h-4 w-4 text-blue-500" />
            <Badge variant="info">{infoCount}</Badge>
          </div>
          {job && job.mfgScore > 0 && (
            <div
              className="px-2 py-0.5 rounded font-mono text-sm font-bold text-white"
              style={{ background: scoreColor(job.mfgScore) }}
            >
              {job.mfgScore} <span className="text-xs opacity-80">{job.mfgGrade}</span>
            </div>
          )}
        </div>
      </header>

      {/* Inline action error — a patch/ignore call failed and was rolled back */}
      {actionError && (
        <div className="flex items-center gap-2 px-4 py-2 bg-red-500/10 border-b border-red-500/20 flex-shrink-0">
          <AlertCircle className="h-4 w-4 text-red-500 shrink-0" />
          <p className="flex-1 text-sm text-red-600 dark:text-red-400">{actionError}</p>
          <button
            type="button"
            onClick={() => setActionError(null)}
            className="text-xs text-red-600 dark:text-red-400 underline hover:no-underline shrink-0"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Body: board + collapsible issues panel */}
      <div className={cn('flex flex-1 min-h-0 overflow-hidden', collapseToBottom ? 'flex-col' : 'flex-row')}>
        <div className={cn('order-1 flex-1 min-h-0 min-w-0 overflow-hidden', collapseToBottom ? 'p-2 sm:p-3' : 'p-2 sm:p-3 md:p-4')}>
          {boardError && !boardData ? (
            <div className="flex flex-col items-center justify-center h-full gap-2 bg-gray-900 rounded-lg">
              <AlertTriangle className="h-8 w-8 text-yellow-500" />
              <p className="text-sm font-medium text-gray-200">Board preview unavailable</p>
              <p className="text-xs text-gray-400 text-center px-6">
                The board visualization couldn&apos;t be loaded. Violations and scores are still available.
              </p>
            </div>
          ) : !boardData ? (
            <div className="flex flex-col items-center justify-center h-full gap-3">
              <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full" />
              <p className="text-sm text-muted-foreground">Loading Board Visualizer</p>
            </div>
          ) : (
            <BoardViewer
              violations={visibleViolations.filter((v) => !v.ignored && (ruleFilter.size === 0 || ruleFilter.has(v.ruleId)))}
              boardData={boardData}
              selectedViolationId={selectedId}
              onViolationClick={(v) => setSelectedId(v?.id)}
              hiddenLayers={hiddenLayers}
              onToggleLayer={toggleLayer}
              onSetHiddenLayers={setHiddenLayers}
              violationLayers={violationLayers}
              allIgnoredLayers={allIgnoredLayers}
              onIgnoreLayer={canWrite() ? handleIgnoreLayer : undefined}
            />
          )}
        </div>

        <section
          className={cn(
            'order-2 bg-card overflow-hidden flex',
            collapseToBottom ? 'flex-col border-t border-border/80' : 'flex-row border-l border-border/80',
            collapseToBottom
              ? (violationsOpen ? 'h-[44dvh] min-h-56 max-h-[68dvh]' : 'h-10')
              : (violationsOpen ? 'w-[18.5rem] md:w-96' : 'w-12')
          )}
          aria-label="Violations panel"
        >
          <div
            className={cn(
              'bg-card/85 backdrop-blur supports-[backdrop-filter]:bg-card/75',
              collapseToBottom
                ? 'h-10 px-3 border-b border-border/70 flex items-center justify-between'
                : 'w-12 border-r border-border/70 flex flex-col items-center gap-2 py-2'
            )}
          >
            {collapseToBottom ? (
              <div className="inline-flex items-center gap-2 text-sm font-medium text-foreground">
                <ListFilter className="h-4 w-4 text-muted-foreground" />
                Issues
                <span className="text-xs text-muted-foreground">{layerFiltered.length}</span>
              </div>
            ) : (
              <>
                <ListFilter className="h-4 w-4 text-muted-foreground" />
                <span className="[writing-mode:vertical-rl] rotate-180 text-[11px] tracking-wide uppercase text-muted-foreground select-none">
                  Issues
                </span>
              </>
            )}

            <button
              type="button"
              onClick={() => setViolationsOpen((v) => !v)}
              aria-label={violationsOpen ? 'Collapse issues panel' : 'Expand issues panel'}
              title={violationsOpen ? 'Collapse issues panel' : 'Expand issues panel'}
              className="inline-flex items-center justify-center h-8 w-8 rounded-md border border-border/70 bg-background/70 text-foreground hover:bg-muted/60 transition-colors"
            >
              {collapseToBottom ? (
                violationsOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />
              ) : (
                violationsOpen ? <ChevronLeft className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />
              )}
            </button>
          </div>

          {violationsOpen && (
            <div className="flex-1 min-h-0 min-w-0">
              <ViolationList
                violations={visibleViolations}
                allViolations={layerFiltered}
                selectedId={selectedId}
                onSelect={(v) => setSelectedId(prev => prev === v.id ? undefined : v.id)}
                filter={severityFilter}
                onFilterChange={setSeverityFilter}
                ruleFilter={ruleFilter}
                onRuleFilterChange={setRuleFilter}
                onIgnore={canWrite() ? handleIgnore : undefined}
              />
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
