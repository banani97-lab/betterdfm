'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Plus, Upload, RefreshCw, LogOut, Info, X } from 'lucide-react'
import { getSubmissions, type Submission } from '@/lib/api'
import { isLoggedIn, clearToken } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ThemeToggle } from '@/components/ui/theme-toggle'
import { BetterDFMLogo } from '@/components/ui/betterdfm-logo'

function formatDate(iso: string) {
  return new Date(iso).toLocaleString()
}

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
}

export default function DashboardPage() {
  const router = useRouter()
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [infoSubmissionId, setInfoSubmissionId] = useState<string | null>(null)
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

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="bg-card border-b px-6 py-5 flex items-center justify-between">
        <BetterDFMLogo />
        <div className="flex items-center gap-3">
          <ThemeToggle className="h-11 w-11" />
          <Link href="/admin/profile">
            <Button variant="ghost" size="lg">Profile Settings</Button>
          </Link>
          <Button variant="ghost" size="lg" onClick={handleLogout}>
            <LogOut className="h-4 w-4 mr-1" /> Sign out
          </Button>
          <Link href="/upload">
            <Button size="lg">
              <Plus className="h-4 w-4 mr-1" /> Upload
            </Button>
          </Link>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-6 py-8">
        <div className="flex flex-wrap items-end justify-between gap-4 mb-6">
          <div>
            <h1 className="text-3xl font-semibold tracking-tight text-foreground">Submissions</h1>
            <p className="text-sm text-muted-foreground mt-1">
              {submissions.length} file{submissions.length === 1 ? '' : 's'} in your workspace
            </p>
          </div>
          <Button variant="outline" size="lg" onClick={fetchSubmissions}>
            <RefreshCw className="h-4 w-4 mr-1" /> Refresh
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
          <div className="flex flex-col items-center justify-center h-64 border-2 border-dashed border-border rounded-lg">
            <Upload className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-medium text-foreground">No submissions yet</h3>
            <p className="text-sm text-muted-foreground mb-4">Upload a Gerber or ODB++ file to get started</p>
            <Link href="/upload">
              <Button><Plus className="h-4 w-4 mr-1" /> Upload your first file</Button>
            </Link>
          </div>
        ) : (
          <div className="bg-card rounded-xl border border-border/70 shadow-sm overflow-hidden">
            <table className="w-full">
              <thead className="bg-muted/50 border-b">
                <tr>
                  <th className="text-left px-6 py-4 text-xs uppercase tracking-[0.12em] font-semibold text-muted-foreground">Filename</th>
                  <th className="text-left px-6 py-4 text-xs uppercase tracking-[0.12em] font-semibold text-muted-foreground">Score</th>
                  <th className="text-right px-6 py-4 text-xs uppercase tracking-[0.12em] font-semibold text-muted-foreground">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/70">
                {submissions.map((s) => (
                  <tr key={s.id} className="hover:bg-muted/30">
                    <td className="px-6 py-5">
                      <p className="font-mono text-sm md:text-base truncate max-w-xl font-medium">{s.filename}</p>
                    </td>
                    <td className="px-6 py-5">
                      {s.status === 'DONE' && s.mfgScore > 0 ? (
                        <div
                          className="inline-block px-3 py-1 rounded-md font-mono text-sm font-bold text-white"
                          style={{ background: scoreColor(s.mfgScore) }}
                        >
                          {s.mfgScore} <span className="opacity-80">{s.mfgGrade}</span>
                        </div>
                      ) : (
                        <span className="text-base text-muted-foreground">-</span>
                      )}
                    </td>
                    <td className="px-6 py-5">
                      <div className="flex justify-end items-center gap-3">
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-10 w-10"
                          onClick={() => setInfoSubmissionId(s.id)}
                          aria-label={`Show details for ${s.filename}`}
                          title={`Show details for ${s.filename}`}
                        >
                          <Info className="h-5 w-5" />
                        </Button>
                        {s.status === 'DONE' && s.latestJobId && (
                          <Link href={`/results/${s.latestJobId}`}>
                            <Button variant="outline" className="h-10 px-4 text-sm">View Results</Button>
                          </Link>
                        )}
                        {s.status === 'UPLOADED' && (
                          <Link href={`/upload?submissionId=${s.id}&step=analyze`}>
                            <Button variant="outline" className="h-10 px-4 text-sm">Analyze</Button>
                          </Link>
                        )}
                        {s.status !== 'DONE' && s.status !== 'UPLOADED' && (
                          <Badge variant="info" className="text-sm px-3 py-1">{s.status}</Badge>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
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
                <p className="text-base text-foreground">{infoSubmission.createdAt ? formatDate(infoSubmission.createdAt) : '-'}</p>
              </div>
              <div className="rounded-lg border border-border/80 bg-muted/20 p-4">
                <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-1">Overview</p>
                <p className="text-base text-muted-foreground">Placeholder for summary by you and your coworker.</p>
              </div>
            </div>
          </aside>
        </div>
      )}
    </div>
  )
}
