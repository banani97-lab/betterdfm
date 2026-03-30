'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Building2, LogOut, BarChart3, FileText, Users, AlertTriangle, CheckCircle, XCircle, Info } from 'lucide-react'
import { isAdminLoggedIn, clearAdminToken, adminApiFetch } from '@/lib/adminAuth'
import { Button } from '@/components/ui/button'
import { ADMIN_APP_NAME } from '@/lib/branding'

interface PlatformStats {
  totalOrgs: number
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

function StatCard({ label, value, icon: Icon, sub }: { label: string; value: string | number; icon: React.ElementType; sub?: string }) {
  return (
    <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
      <div className="flex items-center gap-3 mb-2">
        <Icon className="h-5 w-5 text-slate-400" />
        <span className="text-sm text-slate-400">{label}</span>
      </div>
      <p className="text-2xl font-bold text-white">{value}</p>
      {sub && <p className="text-xs text-slate-500 mt-1">{sub}</p>}
    </div>
  )
}

function GradeBar({ grade, count, total }: { grade: string; count: number; total: number }) {
  const pct = total > 0 ? (count / total) * 100 : 0
  const colors: Record<string, string> = { A: 'bg-green-500', B: 'bg-blue-500', C: 'bg-yellow-500', D: 'bg-red-500' }
  return (
    <div className="flex items-center gap-3">
      <span className="w-6 text-sm font-semibold text-white">{grade}</span>
      <div className="flex-1 h-5 bg-slate-800 rounded overflow-hidden">
        <div className={`h-full ${colors[grade] || 'bg-slate-600'} rounded`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-sm text-slate-400 w-10 text-right">{count}</span>
    </div>
  )
}

function ruleLabel(ruleId: string): string {
  return ruleId.replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

export default function AdminDashboardPage() {
  const router = useRouter()
  const [stats, setStats] = useState<PlatformStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!isAdminLoggedIn()) { router.replace('/admin/login'); return }
    adminApiFetch<PlatformStats>('/admin/stats')
      .then(setStats)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [router])

  const handleLogout = () => {
    clearAdminToken()
    router.replace('/admin/login')
  }

  const totalGraded = stats ? Object.values(stats.gradeDistribution).reduce((a, b) => a + b, 0) : 0

  return (
    <div className="min-h-screen bg-slate-950">
      <header className="bg-slate-900 border-b border-slate-800 px-6 py-4 flex items-center justify-between gap-4 sticky top-0 z-30">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-orange-600 flex items-center justify-center">
            <svg viewBox="0 0 24 24" fill="none" className="w-4 h-4 text-white" stroke="currentColor" strokeWidth="2">
              <path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <h1 className="text-xl font-semibold text-white">{ADMIN_APP_NAME}</h1>
        </div>
        <Button variant="ghost" size="sm" onClick={handleLogout} className="text-slate-400 hover:text-white">
          <LogOut className="h-4 w-4 mr-1" /> Logout
        </Button>
      </header>

      <main className="max-w-5xl mx-auto px-6 py-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-2xl font-bold text-white mb-1">Dashboard</h2>
            <p className="text-slate-400 text-sm">Platform-wide statistics</p>
          </div>
          <Link href="/admin/organizations">
            <Button variant="outline" size="sm" className="border-slate-700 text-slate-300 hover:text-white hover:bg-slate-800">
              <Building2 className="h-4 w-4 mr-1" /> Organizations
            </Button>
          </Link>
        </div>

        {loading ? (
          <div className="flex items-center justify-center h-64 text-slate-500">Loading stats...</div>
        ) : !stats ? (
          <div className="flex items-center justify-center h-64 text-slate-500">Failed to load stats</div>
        ) : (
          <div className="space-y-6">
            {/* Top-level KPIs */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              <StatCard label="Organizations" value={stats.totalOrgs} icon={Building2} />
              <StatCard label="Users" value={stats.totalUsers} icon={Users} />
              <StatCard label="Submissions" value={stats.totalSubmissions} icon={FileText} />
              <StatCard label="Analysis Jobs" value={stats.totalJobs} icon={BarChart3} />
            </div>

            {/* Score + Grade + Job Status */}
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              {/* Average Score */}
              <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                <p className="text-sm text-slate-400 mb-2">Average MFG Score</p>
                <p className="text-4xl font-bold text-white">{Math.round(stats.avgScore)}</p>
                <p className="text-xs text-slate-500 mt-1">Across all completed jobs</p>
              </div>

              {/* Grade distribution */}
              <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                <p className="text-sm text-slate-400 mb-3">Grade Distribution</p>
                <div className="space-y-2">
                  {['A', 'B', 'C', 'D'].map(g => (
                    <GradeBar key={g} grade={g} count={stats.gradeDistribution[g] || 0} total={totalGraded} />
                  ))}
                </div>
              </div>

              {/* Job status breakdown */}
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
            </div>

            {/* Violations */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {/* By severity */}
              <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                <p className="text-sm text-slate-400 mb-3">Violations by Severity</p>
                <div className="flex items-end gap-6 mt-2">
                  {[
                    { key: 'ERROR', label: 'Errors', icon: XCircle, color: 'text-red-400' },
                    { key: 'WARNING', label: 'Warnings', icon: AlertTriangle, color: 'text-yellow-400' },
                    { key: 'INFO', label: 'Info', icon: Info, color: 'text-blue-400' },
                  ].map(({ key, label, icon: SevIcon, color }) => (
                    <div key={key} className="text-center">
                      <SevIcon className={`h-5 w-5 mx-auto mb-1 ${color}`} />
                      <p className="text-2xl font-bold text-white">{stats.violationsBySeverity[key] || 0}</p>
                      <p className="text-xs text-slate-500">{label}</p>
                    </div>
                  ))}
                </div>
                <p className="text-xs text-slate-500 mt-3">{stats.totalViolations.toLocaleString()} total violations</p>
              </div>

              {/* Top rules */}
              <div className="bg-slate-900 border border-slate-800 rounded-lg p-5">
                <p className="text-sm text-slate-400 mb-3">Most Triggered Rules</p>
                {stats.topRules.length === 0 ? (
                  <p className="text-xs text-slate-500">No violations recorded yet</p>
                ) : (
                  <div className="space-y-2">
                    {stats.topRules.slice(0, 6).map((r) => (
                      <div key={r.ruleId} className="flex items-center justify-between">
                        <span className="text-sm text-slate-300 truncate">{ruleLabel(r.ruleId)}</span>
                        <span className="text-sm font-medium text-white ml-2">{r.count.toLocaleString()}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}
