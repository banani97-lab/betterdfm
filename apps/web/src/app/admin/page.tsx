'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Building2, LogOut } from 'lucide-react'
import { isAdminLoggedIn, clearAdminToken } from '@/lib/adminAuth'
import { Button } from '@/components/ui/button'

export default function AdminDashboardPage() {
  const router = useRouter()

  useEffect(() => {
    if (!isAdminLoggedIn()) router.replace('/admin/login')
  }, [router])

  const handleLogout = () => {
    clearAdminToken()
    router.replace('/admin/login')
  }

  return (
    <div className="min-h-screen bg-slate-950">
      <header className="bg-slate-900 border-b border-slate-800 px-6 py-4 flex items-center justify-between gap-4 sticky top-0 z-30">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-orange-600 flex items-center justify-center">
            <svg viewBox="0 0 24 24" fill="none" className="w-4 h-4 text-white" stroke="currentColor" strokeWidth="2">
              <path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <h1 className="text-xl font-semibold text-white">BetterDFM Admin</h1>
        </div>
        <Button variant="ghost" size="sm" onClick={handleLogout} className="text-slate-400 hover:text-white">
          <LogOut className="h-4 w-4 mr-1" /> Logout
        </Button>
      </header>

      <main className="max-w-3xl mx-auto px-6 py-12">
        <h2 className="text-2xl font-bold text-white mb-2">Administration</h2>
        <p className="text-slate-400 mb-8">Manage BetterDFM platform resources.</p>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Link href="/admin/organizations">
            <div className="bg-slate-900 border border-slate-800 rounded-lg p-6 hover:border-orange-600/50 transition-colors cursor-pointer">
              <Building2 className="h-8 w-8 text-orange-500 mb-3" />
              <h3 className="text-lg font-semibold text-white">Organizations</h3>
              <p className="text-sm text-slate-400 mt-1">Create and manage customer companies</p>
            </div>
          </Link>
        </div>
      </main>
    </div>
  )
}
