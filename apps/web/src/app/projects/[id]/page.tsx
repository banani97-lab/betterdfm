'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter, useParams } from 'next/navigation'
import Link from 'next/link'
import { ArrowLeft, Check, Edit2, Plus, Share2, Upload } from 'lucide-react'
import {
  getProject,
  getProjectSubmissions,
  updateProject,
  type Project,
  type Submission,
} from '@/lib/api'
import { isLoggedIn, canWrite } from '@/lib/auth'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ShareLinkModal } from '@/components/ui/ShareLinkModal'
import { cn } from '@/lib/utils'

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
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

export default function ProjectDetailPage() {
  const router = useRouter()
  const params = useParams()
  const id = params.id as string

  const [project, setProject] = useState<Project | null>(null)
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Inline editing
  const [editingName, setEditingName] = useState(false)
  const [editName, setEditName] = useState('')
  const [editingDesc, setEditingDesc] = useState(false)
  const [editDesc, setEditDesc] = useState('')
  const [editingRef, setEditingRef] = useState(false)
  const [editRef, setEditRef] = useState('')
  const [shareOpen, setShareOpen] = useState(false)

  const fetchData = useCallback(async () => {
    try {
      const [p, subs] = await Promise.all([
        getProject(id),
        getProjectSubmissions(id),
      ])
      setProject(p)
      setSubmissions(subs ?? [])
      setError(null)
    } catch (e: unknown) {
      if (e instanceof Error) setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    fetchData()
  }, [router, fetchData])

  const saveField = async (field: 'name' | 'description' | 'customerRef', value: string) => {
    if (!project) return
    try {
      const updated = await updateProject(project.id, { [field]: value })
      setProject((prev) => prev ? { ...prev, ...updated } : prev)
    } catch (e: unknown) {
      if (e instanceof Error) setError(e.message)
    }
  }

  // Score sparkline data
  const scoreData = submissions
    .filter((s) => s.status === 'DONE' && s.mfgScore > 0)
    .map((s) => s.mfgScore)

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin h-6 w-6 border-4 border-blue-600 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (!project) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center gap-4">
        <p className="text-muted-foreground">Project not found</p>
        <Link href="/projects"><Button variant="outline">Back to Projects</Button></Link>
      </div>
    )
  }

  return (
    <div className="min-h-screen">
      <header className="bg-card/65 border-b border-border/80 px-4 py-3 md:px-6 md:py-4 flex items-center justify-between gap-3 sticky top-0 z-30">
        <div className="flex items-center gap-4">
          <RapidDFMLogo className="shrink-0" />
          <Link href="/projects">
            <Button variant="ghost" size="icon" className="h-10 w-10" aria-label="Back to projects">
              <ArrowLeft className="h-5 w-5" />
            </Button>
          </Link>
        </div>
        <div className="flex items-center gap-2">
          {canWrite() && (
            <>
              <Button variant="outline" onClick={() => setShareOpen(true)}>
                <Share2 className="h-4 w-4 mr-2" /> Share
              </Button>
              <Link href={`/upload?projectId=${project.id}`}>
                <Button>
                  <Upload className="h-4 w-4 mr-2" /> Upload to Project
                </Button>
              </Link>
            </>
          )}
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
        {error && (
          <div className="mb-4 p-3 bg-destructive/10 border border-destructive/30 rounded text-sm text-destructive">
            {error}
          </div>
        )}

        {/* Project header */}
        <div className="rounded-2xl border border-border/70 bg-card/55 p-6 mb-6">
          <div className="flex flex-wrap items-start gap-4 mb-4">
            {/* Name */}
            <div className="flex-1 min-w-0">
              {editingName ? (
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    className="text-2xl font-semibold bg-background border border-input rounded-md px-2 py-1 focus:outline-none focus:ring-2 focus:ring-ring flex-1"
                    autoFocus
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        saveField('name', editName)
                        setEditingName(false)
                      }
                      if (e.key === 'Escape') setEditingName(false)
                    }}
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={() => { saveField('name', editName); setEditingName(false) }}
                  >
                    <Check className="h-4 w-4" />
                  </Button>
                </div>
              ) : (
                <div className="flex items-center gap-2 group">
                  <h1 className="text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
                    {project.name}
                  </h1>
                  {canWrite() && (
                    <button
                      onClick={() => { setEditName(project.name); setEditingName(true) }}
                      className="opacity-0 group-hover:opacity-100 transition-opacity"
                      aria-label="Edit name"
                    >
                      <Edit2 className="h-4 w-4 text-muted-foreground" />
                    </button>
                  )}
                </div>
              )}
            </div>

            {/* Stats */}
            {project.avgScore > 0 && (
              <div
                className="inline-flex items-center rounded-md px-3 py-1.5 font-mono text-sm font-bold text-white"
                style={{ background: scoreColor(project.avgScore) }}
              >
                {Math.round(project.avgScore)}
                <span className="ml-1 opacity-85">{project.latestGrade}</span>
              </div>
            )}
          </div>

          {/* Description */}
          <div className="mb-3 group">
            {editingDesc ? (
              <div className="flex items-start gap-2">
                <textarea
                  value={editDesc}
                  onChange={(e) => setEditDesc(e.target.value)}
                  rows={2}
                  className="flex-1 border border-input bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring resize-none"
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === 'Escape') setEditingDesc(false)
                  }}
                />
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => { saveField('description', editDesc); setEditingDesc(false) }}
                >
                  <Check className="h-4 w-4" />
                </Button>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <p className="text-sm text-muted-foreground">
                  {project.description || 'No description'}
                </p>
                {canWrite() && (
                  <button
                    onClick={() => { setEditDesc(project.description); setEditingDesc(true) }}
                    className="opacity-0 group-hover:opacity-100 transition-opacity"
                    aria-label="Edit description"
                  >
                    <Edit2 className="h-3 w-3 text-muted-foreground" />
                  </button>
                )}
              </div>
            )}
          </div>

          {/* Customer ref */}
          <div className="group flex items-center gap-2">
            {editingRef ? (
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  value={editRef}
                  onChange={(e) => setEditRef(e.target.value)}
                  className="border border-input bg-background rounded-md px-2 py-1 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-ring"
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') { saveField('customerRef', editRef); setEditingRef(false) }
                    if (e.key === 'Escape') setEditingRef(false)
                  }}
                />
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  onClick={() => { saveField('customerRef', editRef); setEditingRef(false) }}
                >
                  <Check className="h-3 w-3" />
                </Button>
              </div>
            ) : (
              <>
                {project.customerRef ? (
                  <span className="text-xs font-mono text-muted-foreground bg-muted/30 px-2 py-1 rounded">
                    {project.customerRef}
                  </span>
                ) : (
                  <span className="text-xs text-muted-foreground">No customer reference</span>
                )}
                {canWrite() && (
                  <button
                    onClick={() => { setEditRef(project.customerRef); setEditingRef(true) }}
                    className="opacity-0 group-hover:opacity-100 transition-opacity"
                    aria-label="Edit customer reference"
                  >
                    <Edit2 className="h-3 w-3 text-muted-foreground" />
                  </button>
                )}
              </>
            )}
          </div>

          {/* Score sparkline */}
          {scoreData.length > 1 && (
            <div className="mt-4 pt-4 border-t border-border/50">
              <p className="text-xs uppercase tracking-[0.1em] text-muted-foreground mb-2">Score Trend</p>
              <div className="flex items-end gap-1 h-10">
                {scoreData.map((score, i) => {
                  const height = Math.max(10, (score / 100) * 40)
                  return (
                    <div
                      key={i}
                      className="rounded-sm flex-1 max-w-[24px] transition-all"
                      style={{
                        height: `${height}px`,
                        background: scoreColor(score),
                        opacity: 0.5 + (i / scoreData.length) * 0.5,
                      }}
                      title={`Score: ${score}`}
                    />
                  )
                })}
              </div>
            </div>
          )}

          <div className="flex items-center gap-4 mt-4 pt-3 border-t border-border/50">
            <span className="text-xs text-muted-foreground">
              {project.submissionCount} submission{project.submissionCount === 1 ? '' : 's'}
            </span>
            <span className="text-xs text-muted-foreground">
              Created {formatDate(project.createdAt)}
            </span>
          </div>
        </div>

        {/* Submissions table */}
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-foreground">Submissions</h2>
        </div>

        {submissions.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-48 border-2 border-dashed border-border rounded-2xl bg-card/45">
            <Upload className="h-10 w-10 text-muted-foreground mb-3" />
            <p className="text-sm text-muted-foreground mb-3">No submissions in this project yet</p>
            {canWrite() && (
              <Link href={`/upload?projectId=${project.id}`}>
                <Button><Plus className="h-4 w-4 mr-2" /> Upload a file</Button>
              </Link>
            )}
          </div>
        ) : (
          <section className="relative overflow-hidden rounded-3xl border border-border/70 bg-card/55 shadow-[0_30px_90px_-40px_rgba(0,0,0,0.55)]">
            <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-primary/20 via-transparent to-primary/15" />
            <div className="relative border-b border-border/70 px-6 py-4">
              <div className="hidden md:grid items-center md:grid-cols-[minmax(0,1.6fr)_150px_260px] gap-4">
                <p className="text-xs uppercase tracking-[0.16em] font-semibold text-muted-foreground">Filename</p>
                <p className="text-xs uppercase tracking-[0.16em] font-semibold text-muted-foreground">Score</p>
                <p className="text-xs uppercase tracking-[0.16em] font-semibold text-muted-foreground text-right">Actions</p>
              </div>
            </div>

            <ul className="relative p-4 space-y-3">
              {submissions.map((s) => (
                <li
                  key={s.id}
                  className={cn(
                    'grid grid-cols-1 md:items-center rounded-2xl border border-border/70 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg',
                    'bg-background/12 hover:bg-background/18',
                    'md:grid-cols-[minmax(0,1.6fr)_150px_260px] gap-4 px-4 py-4 md:py-5'
                  )}
                >
                  <div className="min-w-0">
                    <p className="font-mono font-medium text-foreground break-all leading-snug md:truncate text-base">{s.filename}</p>
                    <p className="text-xs text-muted-foreground mt-1">{formatDate(s.createdAt)}</p>
                  </div>

                  <div className="flex items-center justify-between md:block">
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
                    {s.status === 'DONE' && s.latestJobId && (
                      <Link href={`/results/${s.latestJobId}`}>
                        <Button variant="outline" className="h-11 px-5 text-sm">View Results</Button>
                      </Link>
                    )}
                    {s.status !== 'DONE' && (
                      <Badge variant="info" className="text-sm px-3 py-1.5">{s.status}</Badge>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          </section>
        )}
      </main>

      {project && (
        <ShareLinkModal
          projectId={project.id}
          open={shareOpen}
          onClose={() => setShareOpen(false)}
        />
      )}
    </div>
  )
}
