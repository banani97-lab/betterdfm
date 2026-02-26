'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Plus, Upload, RefreshCw, LogOut } from 'lucide-react'
import { getSubmissions, type Submission } from '@/lib/api'
import { isLoggedIn, clearToken } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ThemeToggle } from '@/components/ui/theme-toggle'

const statusVariant: Record<string, 'gray' | 'info' | 'success' | 'destructive'> = {
  UPLOADED: 'gray',
  ANALYZING: 'info',
  DONE: 'success',
  FAILED: 'destructive',
}

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
      <header className="bg-card border-b px-6 py-4 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-foreground">BetterDFM</h1>
          <p className="text-xs text-muted-foreground">PCB Design-for-Manufacturability</p>
        </div>
        <div className="flex items-center gap-3">
          <ThemeToggle />
          <Link href="/admin/profile">
            <Button variant="ghost" size="sm">Profile Settings</Button>
          </Link>
          <Button variant="ghost" size="sm" onClick={handleLogout}>
            <LogOut className="h-4 w-4 mr-1" /> Sign out
          </Button>
          <Link href="/upload">
            <Button size="sm">
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
          <div className="bg-card rounded-lg border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/40 border-b">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Filename</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Type</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Status</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Score</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Created</th>
                  <th className="text-right px-4 py-3 font-medium text-muted-foreground">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {submissions.map((s) => (
                  <tr key={s.id} className="hover:bg-muted/40">
                    <td className="px-4 py-3 font-mono text-xs truncate max-w-xs">{s.filename}</td>
                    <td className="px-4 py-3 text-muted-foreground">{s.fileType}</td>
                    <td className="px-4 py-3">
                      <Badge variant={statusVariant[s.status] ?? 'gray'}>
                        {s.status === 'ANALYZING' && (
                          <span className="inline-block w-2 h-2 bg-blue-500 rounded-full animate-pulse mr-1" />
                        )}
                        {s.status}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">
                      {s.status === 'DONE' && s.mfgScore > 0 ? (
                        <div
                          className="inline-block px-2 py-0.5 rounded font-mono text-xs font-bold text-white"
                          style={{ background: scoreColor(s.mfgScore) }}
                        >
                          {s.mfgScore} <span className="opacity-80">{s.mfgGrade}</span>
                        </div>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground text-xs">{s.createdAt ? formatDate(s.createdAt) : '—'}</td>
                    <td className="px-4 py-3 text-right">
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
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>
    </div>
  )
}
