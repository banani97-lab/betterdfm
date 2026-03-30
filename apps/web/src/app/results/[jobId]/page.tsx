'use client'

import { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { Download, AlertCircle, AlertTriangle, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, GitCompareArrows, Info, ListFilter } from 'lucide-react'
import { API_URL, getJob, getViolations, getBoardData, getSubmissions, getProjectSubmissions, patchViolation, ignoreLayerViolations, type AnalysisJob, type Submission, type Violation, type BoardData } from '@/lib/api'
import { isLoggedIn, canWrite, getStoredToken } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ViolationList, type SeverityFilter } from '@/components/ui/ViolationList'
import { BoardViewer } from '@/components/ui/BoardViewer'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { cn } from '@/lib/utils'

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
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
  const [hiddenLayers, setHiddenLayers] = useState<Set<string>>(new Set())
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>('ERROR')
  const [isPortraitMobile, setIsPortraitMobile] = useState(false)
  const [violationsOpen, setViolationsOpen] = useState(true)
  const [compareOpen, setCompareOpen] = useState(false)
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [currentProjectId, setCurrentProjectId] = useState<string | null>(null)

  const toggleLayer = (name: string) => {
    setHiddenLayers((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name); else next.add(name)
      return next
    })
  }

  const handleIgnore = useCallback(async (v: Violation, ignored: boolean) => {
    try {
      const result = await patchViolation(v.id, { ignored })
      setViolations((prev) => prev.map((x) => x.id === v.id ? { ...x, ignored } : x))
      setJob((prev) => prev ? { ...prev, mfgScore: result.mfgScore, mfgGrade: result.mfgGrade } : prev)
    } catch {
      // ignore network errors silently — violation state stays unchanged
    }
  }, [])

  const handleIgnoreLayer = useCallback(async (layer: string, ignored: boolean, severity?: string) => {
    if (!job) return
    try {
      const result = await ignoreLayerViolations(job.id, layer, ignored, severity)
      setViolations((prev) => prev.map((x) =>
        x.layer === layer && (!severity || x.severity === severity) ? { ...x, ignored } : x
      ))
      setJob((prev) => prev ? { ...prev, mfgScore: result.mfgScore, mfgGrade: result.mfgGrade } : prev)
    } catch {
      // ignore network errors silently
    }
  }, [job])

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

  // Layer-filtered only — used for tab counts so they always show totals per severity.
  const layerFiltered = violations.filter((v) => !v.layer || !hiddenLayers.has(v.layer))

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
    const load = async () => {
      try {
        const [jobData, violationsData] = await Promise.all([
          getJob(jobId),
          getViolations(jobId),
        ])
        setJob(jobData)
        setViolations(violationsData ?? [])
        getBoardData(jobId).then(setBoardData).catch(() => {})
        // Find the current submission's projectId for compare scoping
        getSubmissions().then(subs => {
          const current = subs?.find(s => s.id === jobData.submissionId)
          if (current?.projectId) setCurrentProjectId(current.projectId)
        }).catch(() => {})
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : 'Failed to load results')
      } finally {
        setLoading(false)
      }
    }
    load()
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
        <Link href="/dashboard"><Button variant="outline">Back to Dashboard</Button></Link>
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
        <div className="order-2 md:order-3 ml-auto flex items-center gap-2">
          {currentProjectId && <div className="relative">
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

      {/* Body: board + collapsible issues panel */}
      <div className={cn('flex flex-1 min-h-0 overflow-hidden', collapseToBottom ? 'flex-col' : 'flex-row')}>
        <div className={cn('order-1 flex-1 min-h-0 min-w-0 overflow-hidden', collapseToBottom ? 'p-2 sm:p-3' : 'p-2 sm:p-3 md:p-4')}>
          <BoardViewer
            violations={visibleViolations.filter((v) => !v.ignored)}
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
                onIgnore={canWrite() ? handleIgnore : undefined}
              />
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
