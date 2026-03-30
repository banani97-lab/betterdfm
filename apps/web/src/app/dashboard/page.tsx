'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { AlertCircle, AlertTriangle, Cog, FolderOpen, Info, LogOut, Plus, RefreshCw, Upload, X } from 'lucide-react'
import { getSubmissions, getViolations, startAnalysis, getProjects, type Submission, type Project } from '@/lib/api'
import { clearToken, canWrite, isLoggedIn } from '@/lib/auth'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ThemeToggle } from '@/components/ui/theme-toggle'
import { getStoredValue, LEGACY_UI_SETTINGS_STORAGE_KEY, setStoredValue, UI_SETTINGS_STORAGE_KEY } from '@/lib/branding'
import { cn } from '@/lib/utils'

type BackgroundStyle = 'spotlight' | 'studio' | 'grid' | 'aurora'
type TableDensity = 'comfortable' | 'compact'

interface UiSettings {
  background: BackgroundStyle
  tableDensity: TableDensity
}

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

interface ViolationCounts { errors: number; warnings: number; infos: number }
interface OverviewEntry { counts: ViolationCounts; loading: boolean }

function generateBlurb(counts: ViolationCounts, score: number): string {
  const { errors, warnings, infos } = counts
  if (errors === 0 && warnings === 0) {
    if (infos === 0) return 'No manufacturing issues detected. This design appears ready for fabrication against the active capability profile.'
    return `No critical or warning-level issues found. ${infos} informational ${infos === 1 ? 'note' : 'notes'} flagged — review before release but these are not blockers.`
  }
  const parts: string[] = []
  if (errors > 0) {
    parts.push(score < 60
      ? `${errors} ${errors === 1 ? 'error' : 'errors'} detected that will likely cause fabrication rejection.`
      : `${errors} ${errors === 1 ? 'error' : 'errors'} require attention before this design can be manufactured.`)
  }
  if (warnings > 0) {
    parts.push(`${warnings} ${warnings === 1 ? 'warning' : 'warnings'} flagged that may affect yield or require negotiation with the fab.`)
  }
  if (infos > 0) parts.push(`${infos} informational ${infos === 1 ? 'note' : 'notes'} also present.`)
  if (score >= 75) parts.push('Most issues appear addressable with minor layout adjustments.')
  else if (score >= 60) parts.push('Address errors before submitting to your fab.')
  else parts.push('Significant rework recommended before fabrication.')
  return parts.join(' ')
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
  const [projects, setProjects] = useState<Project[]>([])
  const [settings, setSettings] = useState<UiSettings>(DEFAULT_UI_SETTINGS)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [infoSubmissionId, setInfoSubmissionId] = useState<string | null>(null)
  const [overviewCache, setOverviewCache] = useState<Record<string, OverviewEntry>>({})
  const [retrying, setRetrying] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchSubmissions = useCallback(async () => {
    try {
      const [subsResult, projResult] = await Promise.allSettled([
        getSubmissions(),
        getProjects(undefined, false),
      ])
      setSubmissions(subsResult.status === 'fulfilled' ? (subsResult.value ?? []) : [])
      setProjects(projResult.status === 'fulfilled' ? (projResult.value ?? []) : [])
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
      const raw = getStoredValue(UI_SETTINGS_STORAGE_KEY, LEGACY_UI_SETTINGS_STORAGE_KEY)
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
    setStoredValue(UI_SETTINGS_STORAGE_KEY, JSON.stringify(settings), LEGACY_UI_SETTINGS_STORAGE_KEY)
  }, [settings])

  // Auto-refresh when any submission is ANALYZING
  useEffect(() => {
    const hasAnalyzing = submissions.some((s) => s.status === 'ANALYZING')
    if (!hasAnalyzing) return
    const t = setInterval(fetchSubmissions, 5000)
    return () => clearInterval(t)
  }, [submissions, fetchSubmissions])

  // Fetch violations when info panel opens for a completed submission
  useEffect(() => {
    if (!infoSubmissionId) return
    if (overviewCache[infoSubmissionId]) return
    const sub = submissions.find((s) => s.id === infoSubmissionId)
    if (!sub?.latestJobId || sub.status !== 'DONE') return
    setOverviewCache((prev) => ({ ...prev, [infoSubmissionId]: { counts: { errors: 0, warnings: 0, infos: 0 }, loading: true } }))
    getViolations(sub.latestJobId)
      .then((violations) => {
        const counts = (violations ?? []).reduce<ViolationCounts>(
          (acc, v) => {
            if (v.ignored) return acc
            if (v.severity === 'ERROR') acc.errors++
            else if (v.severity === 'WARNING') acc.warnings++
            else if (v.severity === 'INFO') acc.infos++
            return acc
          },
          { errors: 0, warnings: 0, infos: 0 }
        )
        setOverviewCache((prev) => ({ ...prev, [infoSubmissionId]: { counts, loading: false } }))
      })
      .catch(() => {
        setOverviewCache((prev) => ({ ...prev, [infoSubmissionId]: { counts: { errors: 0, warnings: 0, infos: 0 }, loading: false } }))
      })
  }, [infoSubmissionId, submissions, overviewCache])

  const handleRetry = async (submissionId: string) => {
    setRetrying((prev) => new Set(prev).add(submissionId))
    try {
      await startAnalysis(submissionId)
      await fetchSubmissions()
    } catch {
      // swallow — submission list will reflect state on next refresh
    } finally {
      setRetrying((prev) => { const n = new Set(prev); n.delete(submissionId); return n })
    }
  }

  const handleLogout = () => {
    clearToken()
    router.replace('/login')
  }

  const infoSubmission = submissions.find((s) => s.id === infoSubmissionId) ?? null
  const isCompact = settings.tableDensity === 'compact'
  const rowPadding = settings.tableDensity === 'compact' ? 'py-3 md:py-3' : 'py-4 md:py-5'
  const filenameSize = settings.tableDensity === 'compact' ? 'text-sm' : 'text-base'
  const tableCols = isCompact
    ? 'md:grid-cols-[minmax(0,1.25fr)_112px_210px]'
    : 'md:grid-cols-[minmax(0,1.6fr)_150px_260px]'
  const rowSurfaceClass = isCompact
    ? 'bg-background/30 hover:bg-muted/25'
    : 'bg-background/12 hover:bg-background/18'

  return (
    <div className="min-h-screen">
      <header className="group/taskbar bg-card/65 border-b border-border/80 px-4 py-3 md:px-6 md:py-4 flex flex-wrap md:flex-nowrap items-center justify-between gap-3 md:gap-4 sticky top-0 z-30">
        <RapidDFMLogo className="shrink-0" />
        <div className="flex w-full md:w-auto flex-wrap md:flex-nowrap items-center justify-end gap-2">
          <ThemeToggle className="h-11 w-11" />

          <Button
            variant="ghost"
            size="icon"
            className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
            onClick={() => setSettingsOpen(true)}
            aria-label="Open settings"
            title="Open settings"
          >
            <span className="flex items-center justify-center w-full">
              <Cog className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
              <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
                Settings
              </span>
            </span>
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
            onClick={handleLogout}
            aria-label="Sign out"
            title="Sign out"
          >
            <span className="flex items-center justify-center w-full">
              <LogOut className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
              <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
                Sign out
              </span>
            </span>
          </Button>

          <Link href="/projects">
            <Button
              variant="ghost"
              size="icon"
              className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
              aria-label="Projects"
              title="Projects"
            >
              <span className="flex items-center justify-center w-full">
                <FolderOpen className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
                <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
                  Projects
                </span>
              </span>
            </Button>
          </Link>

          {canWrite() && (
            <Link href="/upload">
              <Button
                size="icon"
                className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
                aria-label="Upload"
                title="Upload"
              >
                <span className="flex items-center justify-center w-full">
                  <Plus className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
                  <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
                    Upload
                  </span>
                </span>
              </Button>
            </Link>
          )}
        </div>
      </header>

      <main className={cn('mx-auto py-8', isCompact ? 'max-w-5xl px-4 sm:px-5' : 'max-w-7xl px-4 sm:px-6')}>
        {/* Projects section */}
        {projects.length > 0 && (
          <div className="mb-8">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-xl font-semibold text-foreground">Projects</h2>
              <Link href="/projects">
                <Button variant="ghost" size="sm" className="text-sm">
                  View all <FolderOpen className="h-4 w-4 ml-1" />
                </Button>
              </Link>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {projects.slice(0, 6).map((p) => (
                <Link key={p.id} href={`/projects/${p.id}`}>
                  <div className="rounded-xl border border-border/70 bg-card/55 p-4 hover:bg-card/70 hover:-translate-y-0.5 hover:shadow-md transition-all duration-200 cursor-pointer">
                    <div className="flex items-start justify-between gap-2 mb-1">
                      <h3 className="text-sm font-semibold text-foreground truncate">{p.name}</h3>
                      {p.latestGrade && p.avgScore > 0 && (
                        <div
                          className="inline-flex items-center rounded px-1.5 py-0.5 font-mono text-xs font-bold text-white shrink-0"
                          style={{ background: scoreColor(p.avgScore) }}
                        >
                          {Math.round(p.avgScore)}{p.latestGrade}
                        </div>
                      )}
                    </div>
                    {p.customerRef && (
                      <p className="text-xs font-mono text-muted-foreground mb-1">{p.customerRef}</p>
                    )}
                    <p className="text-xs text-muted-foreground">
                      {p.submissionCount} submission{p.submissionCount === 1 ? '' : 's'}
                    </p>
                  </div>
                </Link>
              ))}
            </div>
          </div>
        )}

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
            {canWrite() && (
              <Link href="/upload">
                <Button><Plus className="h-4 w-4 mr-2" /> Upload your first file</Button>
              </Link>
            )}
          </div>
        ) : (
          <section className="relative overflow-hidden rounded-3xl border border-border/70 bg-card/55 shadow-[0_30px_90px_-40px_rgba(0,0,0,0.55)]">
            <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-primary/20 via-transparent to-primary/15" />
            <div className={cn('relative border-b border-border/70', isCompact ? 'px-4 py-3' : 'px-6 py-4')}>
              <div className={cn('hidden md:grid items-center', tableCols, isCompact ? 'gap-3' : 'gap-4')}>
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
                    'grid grid-cols-1 md:items-center rounded-2xl border border-border/70 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg',
                    rowSurfaceClass,
                    tableCols,
                    isCompact ? 'gap-3 px-3' : 'gap-4 px-4',
                    rowPadding
                  )}
                >
                  <div className="min-w-0">
                    <p className={cn('font-mono font-medium text-foreground break-all leading-snug md:truncate', filenameSize)}>{s.filename}</p>
                  </div>

                  <div className="flex items-center justify-between md:block">
                    <p className="text-sm font-medium text-muted-foreground md:hidden">Score</p>
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

                  <div className="flex flex-wrap items-center gap-2 md:justify-end md:gap-3">
                    <p className="text-sm font-medium text-muted-foreground md:hidden">Actions</p>
                    <Button
                      variant="outline"
                      size="icon"
                      className={isCompact ? 'h-10 w-10 md:h-9 md:w-9' : 'h-11 w-11'}
                      onClick={() => setInfoSubmissionId(s.id)}
                      aria-label={`Show details for ${s.filename}`}
                      title="Show details"
                    >
                      <Info className={isCompact ? 'h-4 w-4' : 'h-5 w-5'} />
                    </Button>
                    {s.status === 'DONE' && canWrite() && (
                      <Button
                        variant="outline"
                        size="icon"
                        className={isCompact ? 'h-10 w-10 md:h-9 md:w-9' : 'h-11 w-11'}
                        onClick={() => handleRetry(s.id)}
                        disabled={retrying.has(s.id)}
                        aria-label="Retry analysis"
                        title="Retry analysis with latest capability profile"
                      >
                        <RefreshCw className={cn(isCompact ? 'h-4 w-4' : 'h-5 w-5', retrying.has(s.id) && 'animate-spin')} />
                      </Button>
                    )}
                    {s.status === 'DONE' && s.latestJobId && (
                      <Link href={`/results/${s.latestJobId}`}>
                        <Button variant="outline" className={isCompact ? 'h-10 px-3 text-sm md:h-9 md:text-xs' : 'h-11 px-5 text-sm'}>View Results</Button>
                      </Link>
                    )}
                    {s.status === 'UPLOADED' && canWrite() && (
                      <Link href={`/upload?submissionId=${s.id}&step=analyze`}>
                        <Button variant="outline" className={isCompact ? 'h-10 px-3 text-sm md:h-9 md:text-xs' : 'h-11 px-5 text-sm'}>Analyze</Button>
                      </Link>
                    )}
                    {s.status !== 'DONE' && s.status !== 'UPLOADED' && (
                      <Badge variant="info" className={isCompact ? 'text-sm px-2.5 py-1.5 md:text-xs md:py-1' : 'text-sm px-3 py-1.5'}>{s.status}</Badge>
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
              {infoSubmission.status === 'DONE' && (() => {
                const entry = overviewCache[infoSubmission.id]
                return (
                  <>
                    <div className="rounded-lg border border-border/80 bg-muted/20 p-4">
                      <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-3">Findings</p>
                      {entry?.loading ? (
                        <div className="flex items-center gap-2 text-sm text-muted-foreground">
                          <div className="animate-spin h-4 w-4 border-2 border-muted-foreground border-t-transparent rounded-full" />
                          Loading…
                        </div>
                      ) : entry ? (
                        <div className="flex items-center gap-4">
                          <span className="flex items-center gap-1.5 text-sm font-medium text-red-500">
                            <AlertCircle className="h-4 w-4" />{entry.counts.errors}
                          </span>
                          <span className="flex items-center gap-1.5 text-sm font-medium text-yellow-500">
                            <AlertTriangle className="h-4 w-4" />{entry.counts.warnings}
                          </span>
                          <span className="flex items-center gap-1.5 text-sm font-medium text-blue-500">
                            <Info className="h-4 w-4" />{entry.counts.infos}
                          </span>
                        </div>
                      ) : (
                        <p className="text-sm text-muted-foreground">—</p>
                      )}
                    </div>
                    <div className="rounded-lg border border-border/80 bg-muted/20 p-4">
                      <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-2">Overview</p>
                      {entry?.loading ? (
                        <div className="animate-pulse h-4 bg-muted rounded w-3/4" />
                      ) : entry ? (
                        <p className="text-sm text-foreground leading-relaxed">
                          {generateBlurb(entry.counts, infoSubmission.mfgScore)}
                        </p>
                      ) : (
                        <p className="text-sm text-muted-foreground">—</p>
                      )}
                    </div>
                  </>
                )
              })()}
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
