'use client'

import { useCallback, useEffect, useState, type MouseEvent } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Archive, ArchiveRestore, FolderOpen, Inbox, Plus, Search, X } from 'lucide-react'
import { getProjects, createProject, archiveProject, restoreProject, getSubmissions, type Project } from '@/lib/api'
import { isLoggedIn, canWrite } from '@/lib/auth'
import { useUsage } from '@/lib/useUsage'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { AppTaskbar } from '@/components/ui/app-taskbar'
import { AppBackButton } from '@/components/ui/app-back-button'
import { Button } from '@/components/ui/button'
import { track } from '@/lib/analytics'

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
  })
}

export default function ProjectsPage() {
  const router = useRouter()
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [showArchived, setShowArchived] = useState(false)
  const [unassignedCount, setUnassignedCount] = useState(0)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [newRef, setNewRef] = useState('')
  const [creating, setCreating] = useState(false)
  const { usage } = useUsage()
  const projectLimitReached = usage ? (usage.projects.limit !== -1 && usage.projects.used >= usage.projects.limit) : false

  const fetchProjects = useCallback(async (q?: string, archived = false) => {
    try {
      const data = await getProjects(q, archived)
      setProjects(data ?? [])
      if (!archived) {
        const unassigned = await getSubmissions({ unassigned: true })
        setUnassignedCount(unassigned?.length ?? 0)
      } else {
        setUnassignedCount(0)
      }
      setError(null)
    } catch (e: unknown) {
      if (e instanceof Error) setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    fetchProjects(undefined, false)
  }, [router, fetchProjects])

  useEffect(() => {
    const t = setTimeout(() => { fetchProjects(search || undefined, showArchived) }, 300)
    return () => clearTimeout(t)
  }, [search, showArchived, fetchProjects])

  const handleArchiveToggle = async (e: MouseEvent, id: string, archived: boolean) => {
    e.preventDefault()
    e.stopPropagation()
    try {
      if (archived) {
        await restoreProject(id)
      } else {
        await archiveProject(id)
      }
      fetchProjects(search || undefined, showArchived)
    } catch (err: unknown) {
      if (err instanceof Error) setError(err.message)
    }
  }

  const handleCreate = async () => {
    if (!newName.trim()) return
    setCreating(true)
    try {
      await createProject({
        name: newName.trim(),
        description: newDesc.trim() || undefined,
        customerRef: newRef.trim() || undefined,
      })
      track('Project Created', { projectName: newName.trim() })
      setShowCreate(false)
      setNewName('')
      setNewDesc('')
      setNewRef('')
      fetchProjects(search || undefined)
    } catch (e: unknown) {
      if (e instanceof Error) setError(e.message)
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="min-h-screen">
      <header className="bg-card/65 border-b border-border/80 px-4 py-3 md:px-6 md:py-4 flex items-center justify-between gap-3 sticky top-0 z-30">
        <RapidDFMLogo className="shrink-0" />
        <div className="flex items-center gap-2">
          {canWrite() && (
            projectLimitReached ? (
              <p className="text-xs text-muted-foreground">Project limit reached ({usage!.projects.used}/{usage!.projects.limit}). Upgrade to create more.</p>
            ) : (
              <Button onClick={() => setShowCreate(true)}>
                <Plus className="h-4 w-4 mr-2" /> New Project
              </Button>
            )
          )}
          <AppTaskbar />
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
        <div className="mb-8 flex flex-col gap-5">
          <AppBackButton href="/dashboard" label="Dashboard" />
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <h1 className="text-3xl md:text-4xl font-semibold tracking-tight text-foreground">Projects</h1>
              <p className="text-sm text-muted-foreground mt-2">
                {projects.length} project{projects.length === 1 ? '' : 's'}
              </p>
            </div>
          </div>
        </div>

        {/* Search + Active/Archived toggle */}
        <div className="mb-6 flex flex-wrap items-center gap-3">
          <div className="relative max-w-md flex-1 min-w-[200px]">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search projects..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full border border-input bg-background rounded-md pl-10 pr-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            />
          </div>
          <div className="inline-flex rounded-md border border-input overflow-hidden text-sm">
            <button
              type="button"
              onClick={() => setShowArchived(false)}
              className={`px-3 py-2 ${!showArchived ? 'bg-primary text-primary-foreground' : 'bg-background text-muted-foreground hover:text-foreground'}`}
            >
              Active
            </button>
            <button
              type="button"
              onClick={() => setShowArchived(true)}
              className={`px-3 py-2 border-l border-input ${showArchived ? 'bg-primary text-primary-foreground' : 'bg-background text-muted-foreground hover:text-foreground'}`}
            >
              Archived
            </button>
          </div>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-destructive/10 border border-destructive/30 rounded text-sm text-destructive">
            {error}
          </div>
        )}

        {!showArchived && !loading && unassignedCount > 0 && (
          <Link href="/projects/unassigned">
            <div className="mb-4 rounded-2xl border border-dashed border-border/70 bg-card/40 p-5 hover:bg-card/60 transition-colors cursor-pointer flex items-center gap-3">
              <Inbox className="h-5 w-5 text-muted-foreground shrink-0" />
              <div>
                <h3 className="text-base font-semibold text-foreground">Unassigned</h3>
                <p className="text-xs text-muted-foreground">
                  {unassignedCount} submission{unassignedCount === 1 ? '' : 's'} not in any project
                </p>
              </div>
            </div>
          </Link>
        )}

        {loading ? (
          <div className="flex items-center justify-center h-48">
            <div className="animate-spin h-6 w-6 border-4 border-blue-600 border-t-transparent rounded-full" />
          </div>
        ) : projects.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 border-2 border-dashed border-border rounded-2xl bg-card/45">
            <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-medium text-foreground">{showArchived ? 'No archived projects' : 'No projects yet'}</h3>
            {!showArchived && <p className="text-sm text-muted-foreground mb-4">Create a project to organize your submissions</p>}
            {!showArchived && canWrite() && !projectLimitReached && (
              <Button onClick={() => setShowCreate(true)}>
                <Plus className="h-4 w-4 mr-2" /> Create your first project
              </Button>
            )}
            {!showArchived && canWrite() && projectLimitReached && (
              <p className="text-xs text-muted-foreground">Project limit reached ({usage!.projects.used}/{usage!.projects.limit}). Upgrade to create more.</p>
            )}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {projects.map((p) => (
              <Link key={p.id} href={`/projects/${p.id}`}>
                <div className="rounded-2xl border border-border/70 bg-card/55 p-5 hover:bg-card/70 hover:-translate-y-0.5 hover:shadow-lg transition-all duration-200 cursor-pointer h-full">
                  <div className="flex items-start justify-between gap-2 mb-3">
                    <h3 className="text-base font-semibold text-foreground truncate">{p.name}</h3>
                    {p.latestGrade && p.avgScore > 0 && (
                      <div
                        className="inline-flex items-center rounded-md px-2 py-1 font-mono text-xs font-bold text-white shrink-0"
                        style={{ background: scoreColor(p.avgScore) }}
                      >
                        {Math.round(p.avgScore)}
                        <span className="ml-0.5 opacity-85">{p.latestGrade}</span>
                      </div>
                    )}
                  </div>
                  {p.customerRef && (
                    <p className="text-xs text-muted-foreground mb-1 font-mono">{p.customerRef}</p>
                  )}
                  {p.description && (
                    <p className="text-sm text-muted-foreground mb-3 line-clamp-2">{p.description}</p>
                  )}
                  <div className="flex items-center gap-4 mt-auto pt-2 border-t border-border/50">
                    <span className="text-xs text-muted-foreground">
                      {p.submissionCount} submission{p.submissionCount === 1 ? '' : 's'}
                    </span>
                    {p.lastActivityAt && (
                      <span className="text-xs text-muted-foreground">
                        {formatDate(p.lastActivityAt)}
                      </span>
                    )}
                    {canWrite() && (
                      <button
                        type="button"
                        onClick={(e) => handleArchiveToggle(e, p.id, showArchived)}
                        className="ml-auto inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                        title={showArchived ? 'Restore project' : 'Archive project'}
                      >
                        {showArchived ? <ArchiveRestore className="h-3.5 w-3.5" /> : <Archive className="h-3.5 w-3.5" />}
                        {showArchived ? 'Restore' : 'Archive'}
                      </button>
                    )}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}
      </main>

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50">
          <button
            type="button"
            className="absolute inset-0 bg-black/45"
            onClick={() => setShowCreate(false)}
            aria-label="Close create project"
          />
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <div className="relative bg-card border border-border rounded-2xl shadow-2xl p-6 w-full max-w-md">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-semibold text-foreground">New Project</h2>
                <Button variant="ghost" size="icon" className="h-9 w-9" onClick={() => setShowCreate(false)}>
                  <X className="h-5 w-5" />
                </Button>
              </div>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Name *</label>
                  <input
                    type="text"
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    className="w-full border border-input bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    placeholder="e.g. Motor Controller Rev B"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Description</label>
                  <textarea
                    value={newDesc}
                    onChange={(e) => setNewDesc(e.target.value)}
                    rows={2}
                    className="w-full border border-input bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring resize-none"
                    placeholder="Optional description"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Customer Reference</label>
                  <input
                    type="text"
                    value={newRef}
                    onChange={(e) => setNewRef(e.target.value)}
                    className="w-full border border-input bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    placeholder="e.g. PO-2024-1234"
                  />
                </div>
                <Button onClick={handleCreate} disabled={!newName.trim() || creating} className="w-full">
                  {creating ? 'Creating...' : 'Create Project'}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
