'use client'

import { Suspense, useEffect, useState, useCallback, useMemo } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import { AlertCircle, AlertTriangle, ArrowRight, ArrowUp, ArrowDown, Check, Info, Plus, Minus } from 'lucide-react'
import {
  compareJobs,
  getBoardData,
  type ComparisonResult,
  type Violation,
  type BoardData,
} from '@/lib/api'
import { isLoggedIn } from '@/lib/auth'
import { AppBackButton } from '@/components/ui/app-back-button'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { BoardViewer, type BoardViewerTransform } from '@/components/ui/BoardViewer'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { cn } from '@/lib/utils'
import { track } from '@/lib/analytics'

// ── Helpers ──────────────────────────────────────────────────────────────────

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
}

function severityIcon(severity: string) {
  switch (severity) {
    case 'ERROR':   return <AlertCircle className="h-4 w-4 text-red-500 shrink-0" />
    case 'WARNING': return <AlertTriangle className="h-4 w-4 text-yellow-500 shrink-0" />
    case 'INFO':    return <Info className="h-4 w-4 text-blue-500 shrink-0" />
    default:        return null
  }
}

// ── Violation card ───────────────────────────────────────────────────────────

function ViolationCard({ v }: { v: Violation }) {
  return (
    <div className="flex items-start gap-2 px-3 py-2 rounded-md bg-muted/40 text-sm">
      {severityIcon(v.severity)}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="font-medium text-foreground">{v.ruleId}</span>
          {v.layer && <Badge variant="outline" className="text-[10px] px-1.5 py-0">{v.layer}</Badge>}
        </div>
        <p className="text-muted-foreground text-xs mt-0.5 break-words">{v.message}</p>
        <p className="text-muted-foreground/60 text-[10px] mt-0.5 font-mono">
          ({v.x.toFixed(2)}, {v.y.toFixed(2)}) mm
        </p>
      </div>
    </div>
  )
}

// ── Main page ────────────────────────────────────────────────────────────────

type Tab = 'fixed' | 'new' | 'unchanged'

export default function ComparePage() {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin h-8 w-8 border-4 border-blue-600 border-t-transparent rounded-full" />
      </div>
    }>
      <ComparePageInner />
    </Suspense>
  )
}

function ComparePageInner() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const jobAId = searchParams.get('jobA') ?? ''
  const jobBId = searchParams.get('jobB') ?? ''
  const backHref = jobAId ? `/results/${jobAId}` : '/dashboard'
  const backLabel = jobAId ? 'Results' : 'Dashboard'

  const [result, setResult] = useState<ComparisonResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<Tab>('fixed')

  // Board data for dual viewer
  const [boardDataA, setBoardDataA] = useState<BoardData | null>(null)
  const [boardDataB, setBoardDataB] = useState<BoardData | null>(null)

  // Synchronized transform for dual board viewers
  const [syncTransform, setSyncTransform] = useState<BoardViewerTransform | undefined>(undefined)

  // Hidden layers (shared across both viewers)
  const [hiddenLayers, setHiddenLayers] = useState<Set<string>>(new Set())
  const toggleLayer = useCallback((name: string) => {
    setHiddenLayers((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name); else next.add(name)
      return next
    })
  }, [])

  const handleTransformChange = useCallback((t: BoardViewerTransform) => {
    setSyncTransform(t)
  }, [])

  // Color-coded violations for each board
  const violationsA = useMemo(() => {
    if (!result) return []
    // Fixed violations shown as green on left board, unchanged as gray
    return [
      ...result.fixed.map(v => ({ ...v, severity: 'INFO' as const })),      // will render as blue/info, but we override color below
      ...result.unchanged.map(p => p.a),
    ]
  }, [result])

  const violationsB = useMemo(() => {
    if (!result) return []
    // New violations shown on right board, unchanged as gray
    return [
      ...result.new,
      ...result.unchanged.map(p => p.b),
    ]
  }, [result])

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    if (!jobAId || !jobBId) {
      setError('Both jobA and jobB query parameters are required.')
      setLoading(false)
      return
    }

    const load = async () => {
      try {
        const [comparisonData] = await Promise.all([
          compareJobs(jobAId, jobBId),
          getBoardData(jobAId).then(setBoardDataA).catch(() => {}),
          getBoardData(jobBId).then(setBoardDataB).catch(() => {}),
        ])
        setResult(comparisonData)
        track('Comparison Viewed', { jobAId, jobBId, scoreDelta: comparisonData.scoreDelta })
        // Default to tab with most items
        if (comparisonData.summary.newCount > 0 && comparisonData.summary.newCount >= comparisonData.summary.fixedCount) {
          setActiveTab('new')
        } else if (comparisonData.summary.fixedCount > 0) {
          setActiveTab('fixed')
        }
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : 'Failed to load comparison')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [jobAId, jobBId, router])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin h-8 w-8 border-4 border-blue-600 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (error || !result) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-4">
        <AlertCircle className="h-12 w-12 text-red-400" />
        <p className="text-muted-foreground">{error ?? 'No data'}</p>
        <AppBackButton href={backHref} label={backLabel} />
      </div>
    )
  }

  const { jobA, jobB, scoreDelta, summary } = result
  const improved = scoreDelta > 0
  const regressed = scoreDelta < 0

  const tabs: { key: Tab; label: string; count: number; variant: 'success' | 'destructive' | 'gray' }[] = [
    { key: 'fixed', label: 'Fixed', count: summary.fixedCount, variant: 'success' },
    { key: 'new', label: 'New', count: summary.newCount, variant: 'destructive' },
    { key: 'unchanged', label: 'Unchanged', count: summary.unchangedCount, variant: 'gray' },
  ]

  return (
    <div className="flex flex-col min-h-screen h-[100dvh] bg-background">
      {/* ── Header ─────────────────────────────────────────────────── */}
      <header className="bg-card border-b px-4 py-3 md:px-6 md:py-4 flex-shrink-0">
        <div className="flex flex-wrap items-center gap-3 mb-3">
          <RapidDFMLogo className="shrink-0" />
          <h1 className="text-lg font-semibold text-foreground">Design Comparison</h1>
          <AppBackButton href={backHref} label={backLabel} className="ml-auto shrink-0" />
        </div>

        {/* Comparison header: Job A → delta → Job B */}
        <div className="flex items-center justify-between gap-4 flex-wrap">
          {/* Job A */}
          <div className="flex items-center gap-3 min-w-0">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground truncate">{jobA.filename || 'Rev A'}</p>
              <p className="text-xs text-muted-foreground">{jobA.completedAt ? new Date(jobA.completedAt).toLocaleDateString() : 'N/A'}</p>
            </div>
            <div
              className="px-2 py-0.5 rounded font-mono text-sm font-bold text-white shrink-0"
              style={{ background: scoreColor(jobA.mfgScore) }}
            >
              {jobA.mfgScore} <span className="text-xs opacity-80">{jobA.mfgGrade}</span>
            </div>
          </div>

          {/* Delta */}
          <div className="flex items-center gap-1.5 shrink-0">
            <ArrowRight className="h-4 w-4 text-muted-foreground" />
            <span
              className={cn(
                'flex items-center gap-0.5 font-mono text-sm font-bold px-2 py-0.5 rounded',
                improved ? 'bg-green-500/20 text-green-600 dark:text-green-400' :
                regressed ? 'bg-red-500/20 text-red-600 dark:text-red-400' :
                'bg-muted text-muted-foreground'
              )}
            >
              {improved && <ArrowUp className="h-3.5 w-3.5" />}
              {regressed && <ArrowDown className="h-3.5 w-3.5" />}
              {scoreDelta > 0 ? '+' : ''}{scoreDelta}
            </span>
            <ArrowRight className="h-4 w-4 text-muted-foreground" />
          </div>

          {/* Job B */}
          <div className="flex items-center gap-3 min-w-0">
            <div
              className="px-2 py-0.5 rounded font-mono text-sm font-bold text-white shrink-0"
              style={{ background: scoreColor(jobB.mfgScore) }}
            >
              {jobB.mfgScore} <span className="text-xs opacity-80">{jobB.mfgGrade}</span>
            </div>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground truncate">{jobB.filename || 'Rev B'}</p>
              <p className="text-xs text-muted-foreground">{jobB.completedAt ? new Date(jobB.completedAt).toLocaleDateString() : 'N/A'}</p>
            </div>
          </div>
        </div>

        {/* Summary stat cards */}
        <div className="flex items-center gap-3 mt-3">
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-green-500/10 border border-green-500/20">
            <Check className="h-4 w-4 text-green-600 dark:text-green-400" />
            <span className="text-sm font-semibold text-green-700 dark:text-green-300">{summary.fixedCount}</span>
            <span className="text-xs text-green-600/80 dark:text-green-400/80">Fixed</span>
          </div>
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-red-500/10 border border-red-500/20">
            <Plus className="h-4 w-4 text-red-600 dark:text-red-400" />
            <span className="text-sm font-semibold text-red-700 dark:text-red-300">{summary.newCount}</span>
            <span className="text-xs text-red-600/80 dark:text-red-400/80">New</span>
          </div>
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-muted border border-border">
            <Minus className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-semibold text-muted-foreground">{summary.unchangedCount}</span>
            <span className="text-xs text-muted-foreground/80">Unchanged</span>
          </div>
        </div>
      </header>

      {/* ── Body ───────────────────────────────────────────────────── */}
      <div className="flex flex-1 min-h-0 overflow-hidden">

        {/* Dual Board Viewer */}
        <div className="flex-1 flex min-h-0 min-w-0">
          <div className="flex-1 min-h-0 min-w-0 p-2">
            <BoardViewer
              violations={violationsA}
              boardData={boardDataA}
              hiddenLayers={hiddenLayers}
              onToggleLayer={toggleLayer}
              externalTransform={syncTransform}
              onTransformChange={handleTransformChange}
              label="Rev A (Before)"
            />
          </div>
          <div className="w-px bg-border" />
          <div className="flex-1 min-h-0 min-w-0 p-2">
            <BoardViewer
              violations={violationsB}
              boardData={boardDataB}
              hiddenLayers={hiddenLayers}
              onToggleLayer={toggleLayer}
              externalTransform={syncTransform}
              onTransformChange={handleTransformChange}
              label="Rev B (After)"
            />
          </div>
        </div>

        {/* Violation diff panel */}
        <aside className="w-80 lg:w-96 border-l border-border bg-card overflow-hidden flex flex-col shrink-0">
          {/* Tabs */}
          <div className="flex border-b border-border">
            {tabs.map(({ key, label, count, variant }) => (
              <button
                key={key}
                type="button"
                onClick={() => setActiveTab(key)}
                className={cn(
                  'flex-1 flex items-center justify-center gap-1.5 px-3 py-2.5 text-sm font-medium transition-colors',
                  activeTab === key
                    ? 'border-b-2 border-primary text-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {label}
                <Badge variant={variant} className="text-[10px] px-1.5 py-0">{count}</Badge>
              </button>
            ))}
          </div>

          {/* Tab content */}
          <div className="flex-1 overflow-y-auto p-3 space-y-2">
            {activeTab === 'fixed' && (
              result.fixed.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">No fixed violations</p>
              ) : (
                result.fixed.map(v => <ViolationCard key={v.id} v={v} />)
              )
            )}
            {activeTab === 'new' && (
              result.new.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">No new violations</p>
              ) : (
                result.new.map(v => <ViolationCard key={v.id} v={v} />)
              )
            )}
            {activeTab === 'unchanged' && (
              result.unchanged.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">No unchanged violations</p>
              ) : (
                result.unchanged.map(({ a, b }) => (
                  <div key={a.id} className="space-y-1">
                    <ViolationCard v={a} />
                  </div>
                ))
              )
            )}
          </div>
        </aside>
      </div>
    </div>
  )
}
