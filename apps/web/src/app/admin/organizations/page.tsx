'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Save, Plus, Building2, BarChart3, XCircle, AlertTriangle, Info, CheckCircle, UserPlus, Trash2 } from 'lucide-react'
import { isAdminLoggedIn, adminApiFetch } from '@/lib/adminAuth'
import { toast } from '@/lib/toast'
import { friendlyReason } from '@/lib/errors'
import type { Organization, User } from '@/lib/api'
import { AppBackButton } from '@/components/ui/app-back-button'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface OrgStats {
  totalJobs: number
  jobsByStatus: Record<string, number>
  totalSubmissions: number
  totalUsers: number
  totalViolations: number
  violationsBySeverity: Record<string, number>
  topRules: { ruleId: string; count: number }[]
  avgScore: number
  gradeDistribution: Record<string, number>
}

function ruleLabel(ruleId: string): string {
  return ruleId.replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

export default function AdminOrganizationsPage() {
  const router = useRouter()
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [selected, setSelected] = useState<Organization | null>(null)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [logoUrl, setLogoUrl] = useState('')
  const [saving, setSaving] = useState(false)
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [newSlug, setNewSlug] = useState('')
  const [stats, setStats] = useState<OrgStats | null>(null)
  const [statsLoading, setStatsLoading] = useState(false)
  const [users, setUsers] = useState<User[]>([])
  const [usersLoading, setUsersLoading] = useState(false)
  const [tier, setTier] = useState('STARTER')
  const [newEmail, setNewEmail] = useState('')
  const [newRole, setNewRole] = useState<string>('ANALYST')
  const [usAttested, setUsAttested] = useState(false)
  const [inviting, setInviting] = useState(false)

  useEffect(() => {
    if (!isAdminLoggedIn()) { router.replace('/admin/login'); return }
    loadOrgs()
  }, [router])

  const loadOrgs = async () => {
    try {
      const data = await adminApiFetch<Organization[]>('/admin/organizations')
      setOrgs(data ?? [])
      if (!selected && data?.length > 0) selectOrg(data[0])
    } catch (e: unknown) {
      toast.error(`Couldn't load organizations — ${friendlyReason(e)}`)
    }
  }

  const selectOrg = (org: Organization) => {
    setSelected(org)
    setName(org.name)
    setSlug(org.slug)
    setLogoUrl(org.logoUrl || '')
    setTier(org.subscriptionTier || 'STARTER')
    loadStats(org.id)
    loadUsers(org.id)
  }

  const loadUsers = async (orgId: string) => {
    setUsersLoading(true)
    try {
      const data = await adminApiFetch<User[]>(`/admin/organizations/${orgId}/users`)
      setUsers(data ?? [])
    } catch {
      setUsers([])
    } finally {
      setUsersLoading(false)
    }
  }

  const handleInviteUser = async () => {
    if (!selected || !newEmail.trim() || !usAttested) return
    setInviting(true)
    try {
      await adminApiFetch(`/admin/organizations/${selected.id}/users`, {
        method: 'POST',
        body: JSON.stringify({ email: newEmail, role: newRole, usPersonAttested: usAttested }),
      })
      await loadUsers(selected.id)
      setNewEmail('')
      setUsAttested(false)
      toast.success('User invited')
    } catch (e: unknown) {
      toast.error(`Couldn't invite user — ${friendlyReason(e)}`)
    } finally {
      setInviting(false)
    }
  }

  const handleUpdateRole = async (userId: string, role: string) => {
    if (!selected) return
    try {
      await adminApiFetch(`/admin/organizations/${selected.id}/users/${userId}`, {
        method: 'PUT',
        body: JSON.stringify({ role }),
      })
      await loadUsers(selected.id)
    } catch (e: unknown) {
      toast.error(`Couldn't update role — ${friendlyReason(e)}`)
    }
  }

  const handleDeleteUser = async (userId: string) => {
    if (!selected) return
    if (!window.confirm('Remove this user? They will lose access.')) return
    try {
      await adminApiFetch(`/admin/organizations/${selected.id}/users/${userId}`, {
        method: 'DELETE',
      })
      await loadUsers(selected.id)
      toast.success('User removed')
    } catch (e: unknown) {
      toast.error(`Couldn't remove user — ${friendlyReason(e)}`)
    }
  }

  const loadStats = async (orgId: string) => {
    setStatsLoading(true)
    setStats(null)
    try {
      const data = await adminApiFetch<OrgStats>(`/admin/organizations/${orgId}/stats`)
      setStats(data)
    } catch {
      setStats(null)
    } finally {
      setStatsLoading(false)
    }
  }

  const handleSave = async () => {
    if (!selected) return
    setSaving(true)
    try {
      const updated = await adminApiFetch<Organization>(`/admin/organizations/${selected.id}`, {
        method: 'PUT',
        body: JSON.stringify({ name, slug, logoUrl, subscriptionTier: tier }),
      })
      setSelected(updated)
      await loadOrgs()
      toast.success('Organization saved')
    } catch (e: unknown) {
      toast.error(`Couldn't save organization — ${friendlyReason(e)}`)
    } finally {
      setSaving(false)
    }
  }

  const handleCreate = async () => {
    if (!newName.trim() || !newSlug.trim()) return
    setCreating(true)
    try {
      const org = await adminApiFetch<Organization>('/admin/organizations', {
        method: 'POST',
        body: JSON.stringify({ name: newName, slug: newSlug }),
      })
      await loadOrgs()
      selectOrg(org)
      setNewName('')
      setNewSlug('')
      toast.success('Organization created')
    } catch (e: unknown) {
      toast.error(`Couldn't create organization — ${friendlyReason(e)}`)
    } finally {
      setCreating(false)
    }
  }

  const autoSlug = (value: string) => {
    setNewName(value)
    setNewSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''))
  }

  const totalGraded = stats ? Object.values(stats.gradeDistribution).reduce((a, b) => a + b, 0) : 0

  return (
    <div className="min-h-screen bg-slate-950">
      <header className="bg-slate-900 border-b border-slate-800 px-6 py-4 flex items-center justify-between gap-4 sticky top-0 z-30">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-orange-600 flex items-center justify-center">
            <Building2 className="h-4 w-4 text-white" />
          </div>
          <h1 className="text-xl font-semibold text-white">Organizations</h1>
        </div>
        <AppBackButton href="/admin" label="Dashboard" tone="inverse" />
      </header>

      <main className="max-w-6xl mx-auto px-6 py-8 grid grid-cols-4 gap-6">
        {/* Org list */}
        <div className="col-span-1">
          <div className="bg-slate-900 rounded-lg border border-slate-800 p-4">
            <h2 className="font-semibold text-white mb-3">Companies</h2>
            <div className="space-y-1">
              {orgs.map((org) => (
                <div
                  key={org.id}
                  className={`flex items-center px-3 py-2 rounded cursor-pointer text-sm ${selected?.id === org.id ? 'bg-orange-600/20 text-orange-400 font-medium' : 'hover:bg-slate-800 text-slate-400'}`}
                  onClick={() => selectOrg(org)}
                >
                  <span className="truncate">{org.name}</span>
                  <span className="ml-auto text-xs text-slate-500">{org.slug}</span>
                </div>
              ))}
              {orgs.length === 0 && (
                <p className="text-xs text-slate-500 py-2">No organizations yet</p>
              )}
            </div>

            {/* Create new */}
            <div className="mt-4 pt-4 border-t border-slate-800">
              <p className="text-xs font-medium text-slate-400 mb-2">New Organization</p>
              <Input
                placeholder="Company name"
                value={newName}
                onChange={(e) => autoSlug(e.target.value)}
                className="mb-2 text-sm h-8 bg-slate-800 border-slate-700 text-white placeholder:text-slate-500"
              />
              <Input
                placeholder="slug"
                value={newSlug}
                onChange={(e) => setNewSlug(e.target.value)}
                className="mb-2 text-sm h-8 bg-slate-800 border-slate-700 text-white placeholder:text-slate-500"
              />
              <Button
                size="sm"
                onClick={handleCreate}
                disabled={creating || !newName.trim() || !newSlug.trim()}
                className="w-full bg-orange-600 hover:bg-orange-500 text-white"
              >
                <Plus className="h-3.5 w-3.5 mr-1" /> Create
              </Button>
            </div>
          </div>
        </div>

        {/* Org detail + stats */}
        <div className="col-span-3 space-y-4">
          {selected ? (
            <>
              {/* Org editor */}
              <div className="bg-slate-900 rounded-lg border border-slate-800 p-6">
                <h3 className="font-semibold text-white mb-4">Edit Organization</h3>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <Label className="mb-1 block text-sm text-slate-300">Name</Label>
                    <Input
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      className="bg-slate-800 border-slate-700 text-white"
                    />
                  </div>
                  <div>
                    <Label className="mb-1 block text-sm text-slate-300">Slug</Label>
                    <Input
                      value={slug}
                      onChange={(e) => setSlug(e.target.value)}
                      className="bg-slate-800 border-slate-700 text-white"
                    />
                  </div>
                  <div>
                    <Label className="mb-1 block text-sm text-slate-300">Logo URL</Label>
                    <Input
                      value={logoUrl}
                      onChange={(e) => setLogoUrl(e.target.value)}
                      placeholder="https://..."
                      className="bg-slate-800 border-slate-700 text-white placeholder:text-slate-500"
                    />
                  </div>
                  <div>
                    <Label className="mb-1 block text-sm text-slate-300">Subscription Tier</Label>
                    <select
                      value={tier}
                      onChange={(e) => setTier(e.target.value)}
                      className="w-full h-10 px-3 rounded-md bg-slate-800 border border-slate-700 text-white text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      <option value="STARTER">Starter ($799/mo)</option>
                      <option value="PROFESSIONAL">Professional ($2,499/mo)</option>
                      <option value="ENTERPRISE">Enterprise (Custom)</option>
                    </select>
                  </div>
                  <div>
                    <Label className="mb-1 block text-sm text-slate-300">Organization ID</Label>
                    <p className="text-sm text-slate-400 font-mono bg-slate-800 px-3 py-2 rounded border border-slate-700 h-10 flex items-center">{selected.id}</p>
                  </div>
                </div>

                <div className="mt-4 flex items-center justify-between">
                  <p className="text-xs text-slate-500">Created {new Date(selected.createdAt).toLocaleDateString()}</p>
                  <Button
                    onClick={handleSave}
                    disabled={saving}
                    className="bg-orange-600 hover:bg-orange-500 text-white"
                  >
                    <Save className="h-4 w-4 mr-1" />
                    {saving ? 'Saving...' : 'Save'}
                  </Button>
                </div>
              </div>

              {/* Users */}
              <div className="bg-slate-900 rounded-lg border border-slate-800 p-5">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="font-semibold text-white">Users</h3>
                  <span className="text-xs text-slate-500">{users.length} member{users.length !== 1 ? 's' : ''}</span>
                </div>

                {/* Invite form */}
                <div className="mb-4">
                  <div className="flex gap-2">
                    <Input
                      placeholder="email@company.com"
                      type="email"
                      value={newEmail}
                      onChange={(e) => setNewEmail(e.target.value)}
                      className="flex-1 text-sm h-9 bg-slate-800 border-slate-700 text-white placeholder:text-slate-500"
                    />
                    <select
                      value={newRole}
                      onChange={(e) => setNewRole(e.target.value)}
                      className="h-9 px-3 rounded-md border border-slate-700 bg-slate-800 text-sm text-white"
                    >
                      <option value="ADMIN">Admin</option>
                      <option value="ANALYST">Analyst</option>
                      <option value="VIEWER">Viewer</option>
                    </select>
                    <Button
                      size="sm"
                      onClick={handleInviteUser}
                      disabled={inviting || !newEmail.trim() || !usAttested}
                      className="bg-orange-600 hover:bg-orange-500 text-white h-9 px-3"
                    >
                      <UserPlus className="h-4 w-4 mr-1" />
                      {inviting ? 'Inviting...' : 'Invite'}
                    </Button>
                  </div>
                  {/* US-persons attestation (NIST 800-171 3.9.1) */}
                  <label className="mt-2 flex items-start gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={usAttested}
                      onChange={(e) => setUsAttested(e.target.checked)}
                      className="mt-0.5 h-4 w-4 shrink-0 accent-orange-600"
                    />
                    <span className="text-xs text-slate-400">
                      I attest that this user is a U.S. person (U.S. citizen, lawful permanent
                      resident, or protected individual) authorized to access export-controlled
                      (ITAR/CUI) technical data.
                    </span>
                  </label>
                </div>

                {/* User list */}
                {usersLoading ? (
                  <p className="text-sm text-slate-500 py-4 text-center">Loading users...</p>
                ) : users.length === 0 ? (
                  <p className="text-sm text-slate-500 py-4 text-center">No users in this organization yet</p>
                ) : (
                  <div className="space-y-1">
                    {users.map((u) => {
                      const roleColors: Record<string, string> = {
                        ADMIN: 'bg-orange-500/20 text-orange-400',
                        ANALYST: 'bg-blue-500/20 text-blue-400',
                        VIEWER: 'bg-slate-500/20 text-slate-400',
                      }
                      return (
                        <div key={u.id} className="flex items-center gap-3 px-3 py-2 rounded hover:bg-slate-800/50">
                          <span className="text-sm text-white flex-1 truncate">{u.email}</span>
                          <select
                            value={u.role}
                            onChange={(e) => handleUpdateRole(u.id, e.target.value)}
                            className={`text-xs font-medium px-2 py-1 rounded border-0 cursor-pointer ${roleColors[u.role] || 'bg-slate-700 text-slate-300'}`}
                          >
                            <option value="ADMIN">Admin</option>
                            <option value="ANALYST">Analyst</option>
                            <option value="VIEWER">Viewer</option>
                          </select>
                          <span className="text-xs text-slate-500 w-20">{new Date(u.createdAt).toLocaleDateString()}</span>
                          <button
                            onClick={() => handleDeleteUser(u.id)}
                            className="text-slate-500 hover:text-red-400 p-1"
                            title="Remove user"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </div>
                      )
                    })}
                  </div>
                )}
              </div>

              {/* Org stats */}
              {statsLoading ? (
                <div className="bg-slate-900 rounded-lg border border-slate-800 p-6 text-center text-slate-500">
                  Loading stats...
                </div>
              ) : stats ? (
                <div className="space-y-4">
                  {/* KPIs row */}
                  <div className="grid grid-cols-4 gap-3">
                    {[
                      { label: 'Users', value: stats.totalUsers },
                      { label: 'Submissions', value: stats.totalSubmissions },
                      { label: 'Jobs Run', value: stats.totalJobs },
                      { label: 'Avg Score', value: Math.round(stats.avgScore) },
                    ].map((kpi) => (
                      <div key={kpi.label} className="bg-slate-900 border border-slate-800 rounded-lg p-4 text-center">
                        <p className="text-2xl font-bold text-white">{kpi.value}</p>
                        <p className="text-xs text-slate-400 mt-1">{kpi.label}</p>
                      </div>
                    ))}
                  </div>

                  <div className="grid grid-cols-3 gap-4">
                    {/* Job status */}
                    <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                      <p className="text-sm text-slate-400 mb-3">Jobs by Status</p>
                      <div className="space-y-2">
                        {[
                          { key: 'DONE', label: 'Completed', icon: CheckCircle, color: 'text-green-400' },
                          { key: 'FAILED', label: 'Failed', icon: XCircle, color: 'text-red-400' },
                          { key: 'PROCESSING', label: 'Processing', icon: BarChart3, color: 'text-blue-400' },
                          { key: 'PENDING', label: 'Pending', icon: Info, color: 'text-slate-400' },
                        ].map(({ key, label, icon: StatusIcon, color }) => (
                          <div key={key} className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <StatusIcon className={`h-4 w-4 ${color}`} />
                              <span className="text-sm text-slate-300">{label}</span>
                            </div>
                            <span className="text-sm font-medium text-white">{stats.jobsByStatus[key] || 0}</span>
                          </div>
                        ))}
                      </div>
                    </div>

                    {/* Violations by severity */}
                    <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                      <p className="text-sm text-slate-400 mb-3">Violations</p>
                      <div className="space-y-3">
                        {[
                          { key: 'ERROR', label: 'Errors', icon: XCircle, color: 'text-red-400' },
                          { key: 'WARNING', label: 'Warnings', icon: AlertTriangle, color: 'text-yellow-400' },
                          { key: 'INFO', label: 'Info', icon: Info, color: 'text-blue-400' },
                        ].map(({ key, label, icon: SevIcon, color }) => (
                          <div key={key} className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <SevIcon className={`h-4 w-4 ${color}`} />
                              <span className="text-sm text-slate-300">{label}</span>
                            </div>
                            <span className="text-sm font-medium text-white">{stats.violationsBySeverity[key] || 0}</span>
                          </div>
                        ))}
                      </div>
                      <p className="text-xs text-slate-500 mt-3">{stats.totalViolations.toLocaleString()} total</p>
                    </div>

                    {/* Grade distribution */}
                    <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                      <p className="text-sm text-slate-400 mb-3">Grades</p>
                      <div className="space-y-2">
                        {['A', 'B', 'C', 'D'].map(g => {
                          const count = stats.gradeDistribution[g] || 0
                          const pct = totalGraded > 0 ? (count / totalGraded) * 100 : 0
                          const colors: Record<string, string> = { A: 'bg-green-500', B: 'bg-blue-500', C: 'bg-yellow-500', D: 'bg-red-500' }
                          return (
                            <div key={g} className="flex items-center gap-3">
                              <span className="w-5 text-sm font-semibold text-white">{g}</span>
                              <div className="flex-1 h-4 bg-slate-800 rounded overflow-hidden">
                                <div className={`h-full ${colors[g]} rounded`} style={{ width: `${pct}%` }} />
                              </div>
                              <span className="text-sm text-slate-400 w-8 text-right">{count}</span>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  </div>

                  {/* Top rules */}
                  {stats.topRules.length > 0 && (
                    <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                      <p className="text-sm text-slate-400 mb-3">Most Triggered Rules</p>
                      <div className="grid grid-cols-2 gap-x-8 gap-y-2">
                        {stats.topRules.slice(0, 10).map((r) => (
                          <div key={r.ruleId} className="flex items-center justify-between">
                            <span className="text-sm text-slate-300 truncate">{ruleLabel(r.ruleId)}</span>
                            <span className="text-sm font-medium text-white ml-2">{r.count.toLocaleString()}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ) : null}
            </>
          ) : (
            <div className="flex items-center justify-center h-64 bg-slate-900 rounded-lg border border-slate-800 text-slate-500">
              Select or create an organization
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
