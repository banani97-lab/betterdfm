'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { AlertCircle, AlertTriangle, Cog, Info, LogOut, Plus, RefreshCw, Upload, X } from 'lucide-react'
import { getSubmissions, getViolations, type Submission } from '@/lib/api'
import { clearToken, isLoggedIn } from '@/lib/auth'
import { BetterDFMLogo } from '@/components/ui/betterdfm-logo'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ThemeToggle } from '@/components/ui/theme-toggle'
import { cn } from '@/lib/utils'

type BackgroundStyle = 'spotlight' | 'studio' | 'grid' | 'aurora'
type TableDensity = 'comfortable' | 'compact'

interface UiSettings {
  background: BackgroundStyle
  tableDensity: TableDensity
}

const UI_SETTINGS_KEY = 'betterdfm-ui-settings'

const DEFAULT_UI_SETTINGS: UiSettings = {
  background: 'studio',
  tableDensity: 'comfortable',
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString([], {
    hour12: true,
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
}

function applyBackground(background: BackgroundStyle) {
  document.documentElement.setAttribute('data-ui-bg', background)
}

export default function DashboardPage() {
  const router = useRouter()
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [settings, setSettings] = useState<UiSettings>(DEFAULT_UI_SETTINGS)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [infoSubmissionId, setInfoSubmissionId] = useState<string | null>(null)
  const [issueCountsByJobId, setIssueCountsByJobId] = useState<Record<string, { errors: number; warnings: number }>>({})
  const [issueCountsLoadingJobId, setIssueCountsLoadingJobId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchSubmissions = useCallback(async () => {
    try {
      const data = await getSubmissions()
      setSubmissions(data ?? [])
      setError(null)
    } catch (e: unknown) {
      if (e instanceof Error) setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!isLoggedIn()) {
      router.replace('/login')
      return
    }
    fetchSubmissions()
  }, [router, fetchSubmissions])

  useEffect(() => {
    try {
      const raw = localStorage.getItem(UI_SETTINGS_KEY)
      if (!raw) {
        applyBackground(DEFAULT_UI_SETTINGS.background)
        return
      }
      const parsed = JSON.parse(raw) as Partial<UiSettings>
      const parsedBackground = parsed.background
      const next: UiSettings = {
        background: (
          parsedBackground === 'spotlight' ||
          parsedBackground === 'studio' ||
          parsedBackground === 'grid' ||
          parsedBackground === 'aurora'
            ? parsedBackground
            : parsedBackground === 'default'
              ? 'spotlight'
              : DEFAULT_UI_SETTINGS.background
        ),
        tableDensity: parsed.tableDensity === 'compact' ? 'compact' : 'comfortable',
      }
      setSettings(next)
      applyBackground(next.background)
    } catch {
      applyBackground(DEFAULT_UI_SETTINGS.background)
    }
  }, [])

  useEffect(() => {
    applyBackground(settings.background)
    localStorage.setItem(UI_SETTINGS_KEY, JSON.stringify(settings))
  }, [settings])

  // Auto-refresh when any submission is ANALYZING
  useEffect(() => {
    const hasAnalyzing = submissions.some((s) => s.status === 'ANALYZING')
    if (!hasAnalyzing) return
    const t = setInterval(fetchSubmissions, 5000)
    return () => clearInterval(t)
  }, [submissions, fetchSubmissions])

  const handleLogout = () => {
    clearToken()
    router.replace('/login')
  }

  const infoSubmission = submissions.find((s) => s.id === infoSubmissionId) ?? null
  const infoJobId = infoSubmission?.latestJobId ?? null
  const infoIssueCounts = infoJobId ? issueCountsByJobId[infoJobId] : undefined
  const infoIssueCountsLoading = infoJobId ? issueCountsLoadingJobId === infoJobId : false

  useEffect(() => {
    if (!infoJobId || infoSubmission?.status !== 'DONE') return
    if (issueCountsByJobId[infoJobId] || issueCountsLoadingJobId === infoJobId) return

    let cancelled = false
    setIssueCountsLoadingJobId(infoJobId)

    getViolations(infoJobId)
      .then((violations) => {
        if (cancelled) return
        const counts = (violations ?? []).reduce(
          (acc, v) => {
            if (v.ignored) return acc
            if (v.severity === 'ERROR') acc.errors += 1
            if (v.severity === 'WARNING') acc.warnings += 1
            return acc
          },
          { errors: 0, warnings: 0 }
        )
        setIssueCountsByJobId((prev) => ({ ...prev, [infoJobId]: counts }))
      })
      .catch(() => {
        if (cancelled) return
        setIssueCountsByJobId((prev) => ({ ...prev, [infoJobId]: { errors: 0, warnings: 0 } }))
      })
      .finally(() => {
        setIssueCountsLoadingJobId((current) => (current === infoJobId ? null : current))
      })

    return () => {
      cancelled = true
    }
  }, [infoJobId, infoSubmission?.status, issueCountsByJobId, issueCountsLoadingJobId])

  const isCompact = settings.tableDensity === 'compact'
  const rowPadding = settings.tableDensity === 'compact' ? 'py-3' : 'py-5'
  const filenameSize = settings.tableDensity === 'compact' ? 'text-sm' : 'text-base'
  const tableCols = isCompact
    ? 'grid-cols-[minmax(0,1.25fr)_112px_210px]'
    : 'grid-cols-[minmax(0,1.6fr)_150px_260px]'
  const rowSurfaceClass = isCompact
    ? 'bg-background/30 hover:bg-muted/25'
    : 'bg-background/12 hover:bg-background/18'

  return (
    <div className="min-h-screen">
      <header className="group/taskbar bg-card/65 border-b border-border/80 px-6 py-4 flex items-center justify-between gap-4 sticky top-0 z-30">
        <BetterDFMLogo className="shrink-0" />
        <div className="flex items-center gap-2">
          <ThemeToggle className="h-11 w-11" />

          <Button
            variant="ghost"
            size="icon"
            className="h-11 w-11 overflow-hidden transition-all duration-300 group-hover/taskbar:w-32"
            onClick={() => setSettingsOpen(true)}
            aria-label="Open settings"
            title="Open settings"
          >
            <span className="flex items-center justify-center w-full">
              <Cog className="h-5 w-5 shrink-0 transition-transform duration-300 group-hover/taskbar:-translate-x-0.5" />
              <span className="whitespace-nowrap max-w-0 opacity-0 overflow-hidden transition-all duration-300 group-hover/taskbar:max-w-20 group-hover/taskbar:opacity-100 group-hover/taskbar:ml-2">
                Settings
              </span>
            </span>
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="h-11 w-11 overflow-hidden transition-all duration-300 group-hover/taskbar:w-32"
            onClick={handleLogout}
            aria-label="Sign out"
            title="Sign out"
          >
            <span className="flex items-center justify-center w-full">
              <LogOut className="h-5 w-5 shrink-0 transition-transform duration-300 group-hover/taskbar:-translate-x-0.5" />
              <span className="whitespace-nowrap max-w-0 opacity-0 overflow-hidden transition-all duration-300 group-hover/taskbar:max-w-20 group-hover/taskbar:opacity-100 group-hover/taskbar:ml-2">
                Sign out
              </span>
            </span>
          </Button>

          <Link href="/upload">
            <Button
              size="icon"
              className="h-11 w-11 overflow-hidden transition-all duration-300 group-hover/taskbar:w-32"
              aria-label="Upload"
              title="Upload"
            >
              <span className="flex items-center justify-center w-full">
                <Plus className="h-5 w-5 shrink-0 transition-transform duration-300 group-hover/taskbar:-translate-x-0.5" />
                <span className="whitespace-nowrap max-w-0 opacity-0 overflow-hidden transition-all duration-300 group-hover/taskbar:max-w-20 group-hover/taskbar:opacity-100 group-hover/taskbar:ml-2">
                  Upload
                </span>
              </span>
            </Button>
          </Link>
        </div>
      </header>

      <main className={cn('mx-auto py-8', isCompact ? 'max-w-5xl px-4' : 'max-w-7xl px-6')}>
        <div className="flex flex-wrap items-end justify-between gap-4 mb-6">
          <div>
            <h1 className="text-3xl md:text-4xl font-semibold tracking-tight text-foreground">Submissions</h1>
            <p className="text-sm text-muted-foreground mt-2">
              {submissions.length} file{submissions.length === 1 ? '' : 's'} in your workspace
            </p>
          </div>
          <Button variant="outline" size="lg" onClick={fetchSubmissions}>
            <RefreshCw className="h-4 w-4 mr-2" /> Refresh
          </Button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-destructive/10 border border-destructive/30 rounded text-sm text-destructive">
            {error}
          </div>
        )}

        {loading ? (
          <div className="flex items-center justify-center h-48">
            <div className="animate-spin h-6 w-6 border-4 border-blue-600 border-t-transparent rounded-full" />
          </div>
        ) : submissions.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 border-2 border-dashed border-border rounded-2xl bg-card/45">
            <Upload className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-medium text-foreground">No submissions yet</h3>
            <p className="text-sm text-muted-foreground mb-4">Upload a Gerber or ODB++ file to get started</p>
            <Link href="/upload">
              <Button><Plus className="h-4 w-4 mr-2" /> Upload your first file</Button>
            </Link>
          </div>
        ) : (
          <section
            className="relative overflow-hidden rounded-3xl border border-border/70 shadow-[0_30px_90px_-40px_rgba(0,0,0,0.55)]"
            style={{
              backgroundImage:
                'radial-gradient(120% 70% at 50% -18%, hsl(var(--primary) / 0.20), transparent 58%), linear-gradient(180deg, hsl(var(--primary) / 0.16) 0%, hsl(var(--primary) / 0.10) 54%, hsl(var(--primary) / 0.06) 100%)',
            }}
          >
            <div className={cn('relative border-b border-border/70', isCompact ? 'px-4 py-3' : 'px-6 py-4')}>
              <div className={cn('grid items-center', tableCols, isCompact ? 'gap-3' : 'gap-4')}>
                <p className="text-xs uppercase tracking-[0.16em] font-semibold text-muted-foreground">Filename</p>
                <p className="text-xs uppercase tracking-[0.16em] font-semibold text-muted-foreground">Score</p>
                <p className="text-xs uppercase tracking-[0.16em] font-semibold text-muted-foreground text-right">Actions</p>
              </div>
            </div>

            <ul className={cn('relative', isCompact ? 'p-3 space-y-2' : 'p-4 space-y-3')}>
              {submissions.map((s) => (
                <li
                  key={s.id}
                  className={cn(
                    'grid items-center rounded-2xl border border-border/70 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg',
                    rowSurfaceClass,
                    tableCols,
                    isCompact ? 'gap-3 px-3' : 'gap-4 px-4',
                    rowPadding
                  )}
                >
                  <div className="min-w-0">
                    <p className={cn('font-mono truncate font-medium text-foreground', filenameSize)}>{s.filename}</p>
                  </div>

                  <div>
                    {s.status === 'DONE' && s.mfgScore > 0 ? (
                      <div
                        className="inline-flex items-center rounded-md px-3 py-1.5 font-mono text-sm font-bold text-white"
                        style={{ background: scoreColor(s.mfgScore) }}
                      >
                        {s.mfgScore}
                        <span className="ml-1 opacity-85">{s.mfgGrade}</span>
                      </div>
                    ) : (
                      <span className="text-base text-muted-foreground">-</span>
                    )}
                  </div>

                  <div className="flex justify-end items-center gap-3">
                    <Button
                      variant="outline"
                      size="icon"
                      className={isCompact ? 'h-9 w-9' : 'h-11 w-11'}
                      onClick={() => setInfoSubmissionId(s.id)}
                      aria-label={`Show details for ${s.filename}`}
                      title={`Show details for ${s.filename}`}
                    >
                      <Info className={isCompact ? 'h-4 w-4' : 'h-5 w-5'} />
                    </Button>
                    {s.status === 'DONE' && s.latestJobId && (
                      <Link href={`/results/${s.latestJobId}`}>
                        <Button variant="outline" className={isCompact ? 'h-9 px-3 text-xs' : 'h-11 px-5 text-sm'}>View Results</Button>
                      </Link>
                    )}
                    {s.status === 'UPLOADED' && (
                      <Link href={`/upload?submissionId=${s.id}&step=analyze`}>
                        <Button variant="outline" className={isCompact ? 'h-9 px-3 text-xs' : 'h-11 px-5 text-sm'}>Analyze</Button>
                      </Link>
                    )}
                    {s.status !== 'DONE' && s.status !== 'UPLOADED' && (
                      <Badge variant="info" className={isCompact ? 'text-xs px-2.5 py-1' : 'text-sm px-3 py-1.5'}>{s.status}</Badge>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          </section>
        )}
      </main>

      {infoSubmission && (
        <div className="fixed inset-0 z-50">
          <button
            type="button"
            className="absolute inset-0 bg-black/45"
            onClick={() => setInfoSubmissionId(null)}
            aria-label="Close submission details"
          />
          <aside
            className="absolute right-0 top-0 h-full w-full max-w-md bg-card border-l border-border shadow-2xl p-6 overflow-y-auto"
            role="dialog"
            aria-modal="true"
            aria-label="Submission details"
          >
            <div className="flex items-start justify-between gap-4 mb-6">
              <div>
                <p className="text-xs uppercase tracking-[0.12em] text-muted-foreground mb-2">Submission Details</p>
                <h2 className="text-xl font-semibold text-foreground truncate">{infoSubmission.filename}</h2>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-10 w-10"
                onClick={() => setInfoSubmissionId(null)}
                aria-label="Close details panel"
              >
                <X className="h-5 w-5" />
              </Button>
            </div>

            <div className="space-y-5">
              <div className="rounded-lg border border-border/80 bg-muted/20 p-4">
                <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-1">File Type</p>
                <p className="text-base font-medium text-foreground">{infoSubmission.fileType}</p>
              </div>
              <div className="rounded-lg border border-border/80 bg-muted/20 p-4">
                <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-1">Created</p>
                <p className="text-base text-foreground">
                  {infoSubmission.createdAt ? formatDate(infoSubmission.createdAt) : '-'}
                </p>
              </div>
              <div className="rounded-lg border border-border/80 bg-muted/20 p-4">
                <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-2">Issue Counts</p>
                <div className="flex items-center gap-4">
                  <div className="inline-flex items-center gap-2">
                    <AlertCircle className="h-4 w-4 text-red-500" />
                    <span className="text-sm font-medium text-foreground">
                      {infoIssueCounts ? infoIssueCounts.errors : infoIssueCountsLoading ? '...' : '-'}
                    </span>
                    <span className="text-sm text-muted-foreground">Errors</span>
                  </div>
                  <div className="inline-flex items-center gap-2">
                    <AlertTriangle className="h-4 w-4 text-amber-500" />
                    <span className="text-sm font-medium text-foreground">
                      {infoIssueCounts ? infoIssueCounts.warnings : infoIssueCountsLoading ? '...' : '-'}
                    </span>
                    <span className="text-sm text-muted-foreground">Warnings</span>
                  </div>
                </div>
              </div>
              <div className="rounded-lg border border-border/80 bg-muted/20 p-4">
                <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-1">Overview</p>
                <p className="text-base text-muted-foreground">Placeholder for summary by you and your coworker.</p>
              </div>
            </div>
          </aside>
        </div>
      )}

      {settingsOpen && (
        <div className="fixed inset-0 z-50">
          <button
            type="button"
            className="absolute inset-0 bg-black/45"
            onClick={() => setSettingsOpen(false)}
            aria-label="Close settings"
          />
          <aside
            className="absolute right-0 top-0 h-full w-full max-w-lg bg-card border-l border-border shadow-2xl p-6 overflow-y-auto"
            role="dialog"
            aria-modal="true"
            aria-label="Settings"
          >
            <div className="flex items-start justify-between gap-4 mb-6">
              <div>
                <p className="text-xs uppercase tracking-[0.12em] text-muted-foreground mb-2">General Settings</p>
                <h2 className="text-2xl font-semibold text-foreground">Workspace Preferences</h2>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-10 w-10"
                onClick={() => setSettingsOpen(false)}
                aria-label="Close settings panel"
              >
                <X className="h-5 w-5" />
              </Button>
            </div>

            <div className="space-y-6">
              <section className="rounded-xl border border-border/80 bg-muted/20 p-4">
                <h3 className="font-medium text-foreground mb-3">Background Style</h3>
                <div className="grid grid-cols-4 gap-3">
                  {[
                    { id: 'studio', label: 'Studio' },
                    { id: 'spotlight', label: 'Spotlight' },
                    { id: 'grid', label: 'Grid' },
                    { id: 'aurora', label: 'Aurora' },
                  ].map((bg) => (
                    <button
                      key={bg.id}
                      type="button"
                      className={cn(
                        'rounded-lg border p-3 text-left transition-colors',
                        settings.background === bg.id ? 'border-primary bg-primary/10' : 'border-border hover:bg-muted/40'
                      )}
                      onClick={() => setSettings((prev) => ({ ...prev, background: bg.id as BackgroundStyle }))}
                    >
                      <p className="text-sm font-medium text-foreground">{bg.label}</p>
                    </button>
                  ))}
                </div>
              </section>

              <section className="rounded-xl border border-border/80 bg-muted/20 p-4">
                <h3 className="font-medium text-foreground mb-3">Submissions Layout</h3>
                <div className="grid grid-cols-2 gap-3">
                  {[
                    { id: 'comfortable', label: 'Comfortable' },
                    { id: 'compact', label: 'Compact' },
                  ].map((density) => (
                    <button
                      key={density.id}
                      type="button"
                      className={cn(
                        'rounded-lg border p-3 text-left transition-colors',
                        settings.tableDensity === density.id ? 'border-primary bg-primary/10' : 'border-border hover:bg-muted/40'
                      )}
                      onClick={() => setSettings((prev) => ({ ...prev, tableDensity: density.id as TableDensity }))}
                    >
                      <p className="text-sm font-medium text-foreground">{density.label}</p>
                    </button>
                  ))}
                </div>
              </section>

              <section className="rounded-xl border border-border/80 bg-muted/20 p-4">
                <h3 className="font-medium text-foreground mb-3">Quick Access</h3>
                <Link href="/admin/profile" onClick={() => setSettingsOpen(false)}>
                  <Button variant="outline" className="w-full justify-start">Capability Profiles</Button>
                </Link>
              </section>
            </div>
          </aside>
        </div>
      )}
    </div>
  )
}
