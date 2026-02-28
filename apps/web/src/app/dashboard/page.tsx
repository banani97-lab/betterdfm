'use client'

import { Fragment, useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Plus, Upload, RefreshCw, LogOut, Info } from 'lucide-react'
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
  const [expandedInfoId, setExpandedInfoId] = useState<string | null>(null)
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

      <main className="max-w-5xl mx-auto px-6 py-8">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold text-foreground">Submissions</h2>
          <Button variant="outline" size="sm" onClick={fetchSubmissions}>
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
          <div className="bg-card rounded-lg border">
            <table className="w-full text-sm">
              <thead className="bg-muted/40 border-b">
                <tr>
                  <th className="text-left px-3 py-2.5 font-medium text-muted-foreground">Filename</th>
                  <th className="text-left px-3 py-2.5 font-medium text-muted-foreground">Score</th>
                  <th className="text-right px-3 py-2.5 font-medium text-muted-foreground">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {submissions.map((s) => (
                  <Fragment key={s.id}>
                    <tr className="hover:bg-muted/30">
                      <td className="px-3 py-2.5">
                        <p className="font-mono text-xs truncate max-w-sm">{s.filename}</p>
                      </td>
                      <td className="px-3 py-2.5">
                        {s.status === 'DONE' && s.mfgScore > 0 ? (
                          <div
                            className="inline-block px-2 py-0.5 rounded font-mono text-xs font-bold text-white"
                            style={{ background: scoreColor(s.mfgScore) }}
                          >
                            {s.mfgScore} <span className="opacity-80">{s.mfgGrade}</span>
                          </div>
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </td>
                      <td className="px-3 py-2.5">
                        <div className="flex justify-end items-center gap-2">
                          <Button
                            variant="outline"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => setExpandedInfoId(expandedInfoId === s.id ? null : s.id)}
                            aria-label={`Show details for ${s.filename}`}
                            title={`Show details for ${s.filename}`}
                          >
                            <Info className="h-4 w-4" />
                          </Button>
                          {s.status === 'DONE' && s.latestJobId && (
                            <Link href={`/results/${s.latestJobId}`}>
                              <Button variant="outline" size="sm">View Results</Button>
                            </Link>
                          )}
                          {s.status === 'UPLOADED' && (
                            <Link href={`/upload?submissionId=${s.id}&step=analyze`}>
                              <Button variant="outline" size="sm">Analyze</Button>
                            </Link>
                          )}
                          {s.status !== 'DONE' && s.status !== 'UPLOADED' && (
                            <Badge variant="info">{s.status}</Badge>
                          )}
                        </div>
                      </td>
                    </tr>
                    {expandedInfoId === s.id && (
                      <tr className="bg-muted/20">
                        <td colSpan={3} className="px-3 py-3">
                          <div className="grid gap-3 sm:grid-cols-3">
                            <div>
                              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">File Type</p>
                              <p className="text-sm font-medium text-foreground">{s.fileType}</p>
                            </div>
                            <div>
                              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Created</p>
                              <p className="text-sm text-foreground">{s.createdAt ? formatDate(s.createdAt) : '-'}</p>
                            </div>
                            <div>
                              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Overview</p>
                              <p className="text-sm text-muted-foreground">Coming soon - add summary content here.</p>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>
    </div>
  )
}
