'use client'

import { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { ArrowLeft, Download, AlertCircle, AlertTriangle, Info } from 'lucide-react'
import { API_URL, getJob, getViolations, getBoardData, patchViolation, ignoreLayerViolations, type AnalysisJob, type Violation, type BoardData } from '@/lib/api'
import { isLoggedIn, getStoredToken } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ViolationList, type SeverityFilter } from '@/components/ui/ViolationList'
import { BoardViewer } from '@/components/ui/BoardViewer'
import { BetterDFMLogo } from '@/components/ui/betterdfm-logo'

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

  // Sets used by BoardViewer layer panel to show ignore buttons
  const violationLayers = useMemo(() => {
    const s = new Set<string>()
    for (const v of violations) { if (v.layer) s.add(v.layer) }
    return s
  }, [violations])

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
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : 'Failed to load results')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [jobId, router])

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
    <div className="flex flex-col h-screen bg-background">
      {/* Header */}
      <header className="bg-card border-b px-6 py-3 flex items-center gap-4 flex-shrink-0">
        <BetterDFMLogo className="shrink-0" />
        <div className="flex items-center gap-4 flex-1 min-w-0">
          <Link href="/dashboard">
            <Button variant="ghost" size="sm"><ArrowLeft className="h-4 w-4 mr-1" />Dashboard</Button>
          </Link>
          <div className="min-w-0">
            <h1 className="text-base font-semibold text-foreground">DFM Results</h1>
            <p className="text-xs text-muted-foreground font-mono truncate">{jobId}</p>
          </div>
        </div>
        {/* Summary badges */}
        <div className="flex items-center gap-2">
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
        <Button variant="outline" size="sm" onClick={exportCSV} disabled={violations.length === 0}>
          <Download className="h-4 w-4 mr-1" />CSV
        </Button>
        <Button variant="outline" size="sm" onClick={downloadPDF}>
          <Download className="h-4 w-4 mr-1" />PDF
        </Button>
      </header>

      {/* Body: split panel */}
      <div className="flex flex-1 overflow-hidden">
        {/* Left: violation list */}
        <div className="w-96 flex-shrink-0 border-r bg-card overflow-hidden flex flex-col">
          <ViolationList
            violations={visibleViolations}
            allViolations={layerFiltered}
            selectedId={selectedId}
            onSelect={(v) => setSelectedId(v.id)}
            filter={severityFilter}
            onFilterChange={setSeverityFilter}
            onIgnore={handleIgnore}
          />
        </div>

        {/* Right: board viewer */}
        <div className="flex-1 p-4 overflow-hidden">
          <BoardViewer
            violations={visibleViolations.filter((v) => !v.ignored)}
            boardData={boardData}
            selectedViolationId={selectedId}
            onViolationClick={(v) => setSelectedId(v.id)}
            hiddenLayers={hiddenLayers}
            onToggleLayer={toggleLayer}
            violationLayers={violationLayers}
            allIgnoredLayers={allIgnoredLayers}
            onIgnoreLayer={handleIgnoreLayer}
          />
        </div>
      </div>
    </div>
  )
}
