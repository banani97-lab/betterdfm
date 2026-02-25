'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { signIn, completeNewPassword, isLoggedIn, isDevMode } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function LoginPage() {
  const router = useRouter()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [newPasswordSession, setNewPasswordSession] = useState<string | null>(null)
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  useEffect(() => {
    if (isLoggedIn()) router.replace('/dashboard')
  }, [router])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const result = await signIn(email, password)
      if (result.kind === 'new_password_required') {
        setNewPasswordSession(result.session)
      } else {
        router.replace('/dashboard')
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Sign in failed')
    } finally {
      setLoading(false)
    }
  }

  const handleNewPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match')
      return
    }
    setLoading(true)
    try {
      await completeNewPassword(email, newPassword, newPasswordSession!)
      router.replace('/dashboard')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to set new password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-900 to-blue-950">
      <div className="w-full max-w-sm">
        {/* Logo + title */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-blue-600 mb-4">
            <svg viewBox="0 0 24 24" fill="none" className="w-8 h-8 text-white" stroke="currentColor" strokeWidth="2">
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <path d="M7 8h10M7 12h6M7 16h8" strokeLinecap="round" />
            </svg>
          </div>
          <h1 className="text-3xl font-bold text-white">BetterDFM</h1>
          <p className="text-blue-300 mt-1 text-sm">PCB Design-for-Manufacturability</p>
        </div>

        {/* Card */}
        <div className="bg-white/10 backdrop-blur-sm rounded-2xl p-8 border border-white/20">
          {newPasswordSession ? (
            <>
              <h2 className="text-xl font-semibold text-white mb-1">Set your password</h2>
              <p className="text-blue-200 text-sm mb-6">Your account requires a new password before continuing.</p>

              {error && (
                <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-400/20 text-red-300 text-sm">
                  {error}
                </div>
              )}

              <form onSubmit={handleNewPassword} className="flex flex-col gap-4">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="new-password" className="text-blue-100 text-sm">New password</Label>
                  <Input
                    id="new-password"
                    type="password"
                    autoComplete="new-password"
                    placeholder="••••••••"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    required
                    className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                  />
                </div>

                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="confirm-password" className="text-blue-100 text-sm">Confirm password</Label>
                  <Input
                    id="confirm-password"
                    type="password"
                    autoComplete="new-password"
                    placeholder="••••••••"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    required
                    className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                  />
                </div>

                <Button
                  type="submit"
                  disabled={loading}
                  className="w-full bg-blue-600 hover:bg-blue-500 text-white font-semibold h-11 mt-1"
                >
                  {loading ? 'Setting password…' : 'Set password & continue'}
                </Button>
              </form>
            </>
          ) : (
            <>
              <h2 className="text-xl font-semibold text-white mb-1">Welcome back</h2>
              <p className="text-blue-200 text-sm mb-6">Sign in to your DFM dashboard.</p>

              {isDevMode() && (
                <div className="mb-4 px-3 py-2 rounded-lg bg-yellow-500/10 border border-yellow-400/20 text-yellow-300 text-xs">
                  Dev mode — any credentials accepted
                </div>
              )}

              {error && (
                <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-400/20 text-red-300 text-sm">
                  {error}
                </div>
              )}

              <form onSubmit={handleSubmit} className="flex flex-col gap-4">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="email" className="text-blue-100 text-sm">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    autoComplete="email"
                    placeholder="you@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                    className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                  />
                </div>

                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="password" className="text-blue-100 text-sm">Password</Label>
                  <Input
                    id="password"
                    type="password"
                    autoComplete="current-password"
                    placeholder="••••••••"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                    className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                  />
                </div>

                <Button
                  type="submit"
                  disabled={loading}
                  className="w-full bg-blue-600 hover:bg-blue-500 text-white font-semibold h-11 mt-1"
                >
                  {loading ? 'Signing in…' : 'Sign in'}
                </Button>
              </form>
            </>
          )}

          <p className="text-center text-xs text-blue-300/60 mt-5">
            Secured by AWS Cognito
          </p>
        </div>
      </div>
    </div>
  )
}
