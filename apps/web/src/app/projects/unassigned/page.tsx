'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Inbox } from 'lucide-react'
import { getSubmissions, getProjects, moveSubmissionToProject, type Submission, type Project } from '@/lib/api'
import { isLoggedIn, canWrite } from '@/lib/auth'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { AppTaskbar } from '@/components/ui/app-taskbar'
import { AppBackButton } from '@/components/ui/app-back-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString([], {
    hour12: true, month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

export default function UnassignedSubmissionsPage() {
  const router = useRouter()
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchData = useCallback(async () => {
    try {
      const [subs, projs] = await Promise.all([
        getSubmissions({ unassigned: true }),
        getProjects(undefined, false),
      ])
      setSubmissions(subs ?? [])
      setProjects(projs ?? [])
      setError(null)
    } catch (e: unknown) {
      if (e instanceof Error) setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    fetchData()
  }, [router, fetchData])

  const handleAssign = async (submissionId: string, projectId: string) => {
    if (!projectId) return
    try {
      await moveSubmissionToProject(projectId, submissionId)
      setSubmissions((prev) => prev.filter((s) => s.id !== submissionId))
    } catch (e: unknown) {
      if (e instanceof Error) setError(e.message)
    }
  }

  return (
    <div className="min-h-screen">
      <header className="bg-card/65 border-b border-border/80 px-4 py-3 md:px-6 md:py-4 flex items-center justify-between gap-3 sticky top-0 z-30">
        <RapidDFMLogo className="shrink-0" />
        <AppTaskbar />
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
        <div className="mb-8 flex flex-col gap-5">
          <AppBackButton href="/projects" label="Projects" />
          <div>
            <h1 className="text-3xl md:text-4xl font-semibold tracking-tight text-foreground flex items-center gap-3">
              <Inbox className="h-7 w-7 text-muted-foreground" /> Unassigned
            </h1>
            <p className="text-sm text-muted-foreground mt-2">
              {submissions.length} submission{submissions.length === 1 ? '' : 's'} not in any project
            </p>
          </div>
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
            <Inbox className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-medium text-foreground">No unassigned submissions</h3>
            <p className="text-sm text-muted-foreground">Every submission is assigned to a project.</p>
          </div>
        ) : (
          <ul className="space-y-3">
            {submissions.map((s) => (
              <li
                key={s.id}
                className="grid grid-cols-1 md:grid-cols-[minmax(0,1.6fr)_120px_320px] gap-4 md:items-center rounded-2xl border border-border/70 bg-card/55 px-4 py-4"
              >
                <div className="min-w-0">
                  <p className="font-mono font-medium text-foreground break-all md:truncate text-base">{s.filename}</p>
                  <p className="text-xs text-muted-foreground mt-1">{formatDate(s.createdAt)}</p>
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
                    <Badge variant="info" className="text-sm px-3 py-1.5">{s.status}</Badge>
                  )}
                </div>

                <div className="flex flex-wrap items-center gap-2 md:justify-end">
                  {s.status === 'DONE' && s.latestJobId && (
                    <Link href={`/results/${s.latestJobId}`}>
                      <Button variant="outline" className="h-10 px-4 text-sm">View Results</Button>
                    </Link>
                  )}
                  {canWrite() && projects.length > 0 && (
                    <select
                      defaultValue=""
                      onChange={(e) => handleAssign(s.id, e.target.value)}
                      className="h-10 border border-input bg-background rounded-md px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      <option value="" disabled>Assign to project...</option>
                      {projects.map((p) => (
                        <option key={p.id} value={p.id}>{p.name}</option>
                      ))}
                    </select>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </main>
    </div>
  )
}
