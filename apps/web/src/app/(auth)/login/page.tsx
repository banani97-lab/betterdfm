'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { signIn, completeNewPassword, forgotPassword, resetPassword, isLoggedIn, isDevMode } from '@/lib/auth'
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
  const [forgotStep, setForgotStep] = useState<'off' | 'email' | 'code'>('off')
  const [resetCode, setResetCode] = useState('')
  const [resetEmail, setResetEmail] = useState('')
  const [resetNewPassword, setResetNewPassword] = useState('')
  const [resetConfirm, setResetConfirm] = useState('')
  const [resetSuccess, setResetSuccess] = useState(false)

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

  const handleForgotSubmitEmail = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await forgotPassword(resetEmail || email)
      if (!resetEmail) setResetEmail(email)
      setForgotStep('code')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to send reset code')
    } finally {
      setLoading(false)
    }
  }

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    if (resetNewPassword !== resetConfirm) {
      setError('Passwords do not match')
      return
    }
    setLoading(true)
    try {
      await resetPassword(resetEmail, resetCode, resetNewPassword)
      setResetSuccess(true)
      setTimeout(() => {
        setForgotStep('off')
        setResetSuccess(false)
        setResetCode('')
        setResetNewPassword('')
        setResetConfirm('')
      }, 2000)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to reset password')
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
          {forgotStep === 'email' ? (
            <>
              <h2 className="text-xl font-semibold text-white mb-1">Reset password</h2>
              <p className="text-blue-200 text-sm mb-6">Enter your email and we&apos;ll send a verification code.</p>

              {error && (
                <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-400/20 text-red-300 text-sm">
                  {error}
                </div>
              )}

              <form onSubmit={handleForgotSubmitEmail} className="flex flex-col gap-4">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="reset-email" className="text-blue-100 text-sm">Email</Label>
                  <Input
                    id="reset-email"
                    type="email"
                    autoComplete="email"
                    placeholder="you@example.com"
                    value={resetEmail || email}
                    onChange={(e) => setResetEmail(e.target.value)}
                    required
                    className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                  />
                </div>
                <Button type="submit" disabled={loading} className="w-full bg-blue-600 hover:bg-blue-500 text-white font-semibold h-11 mt-1">
                  {loading ? 'Sending…' : 'Send verification code'}
                </Button>
              </form>
              <button onClick={() => { setForgotStep('off'); setError(null) }} className="mt-4 text-sm text-blue-300 hover:text-white transition-colors w-full text-center">
                Back to sign in
              </button>
            </>
          ) : forgotStep === 'code' ? (
            <>
              <h2 className="text-xl font-semibold text-white mb-1">Enter verification code</h2>
              <p className="text-blue-200 text-sm mb-6">Check your email for a code from AWS.</p>

              {resetSuccess && (
                <div className="mb-4 px-3 py-2 rounded-lg bg-green-500/10 border border-green-400/20 text-green-300 text-sm">
                  Password reset successful! Redirecting to sign in…
                </div>
              )}

              {error && (
                <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-400/20 text-red-300 text-sm">
                  {error}
                </div>
              )}

              {!resetSuccess && (
                <form onSubmit={handleResetPassword} className="flex flex-col gap-4">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="code" className="text-blue-100 text-sm">Verification code</Label>
                    <Input
                      id="code"
                      type="text"
                      autoComplete="one-time-code"
                      placeholder="123456"
                      value={resetCode}
                      onChange={(e) => setResetCode(e.target.value)}
                      required
                      className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="reset-new-pw" className="text-blue-100 text-sm">New password</Label>
                    <Input
                      id="reset-new-pw"
                      type="password"
                      autoComplete="new-password"
                      placeholder="••••••••"
                      value={resetNewPassword}
                      onChange={(e) => setResetNewPassword(e.target.value)}
                      required
                      className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="reset-confirm-pw" className="text-blue-100 text-sm">Confirm password</Label>
                    <Input
                      id="reset-confirm-pw"
                      type="password"
                      autoComplete="new-password"
                      placeholder="••••••••"
                      value={resetConfirm}
                      onChange={(e) => setResetConfirm(e.target.value)}
                      required
                      className="bg-white/10 border-white/20 text-white placeholder:text-blue-300/50 focus:border-blue-400"
                    />
                  </div>
                  <Button type="submit" disabled={loading} className="w-full bg-blue-600 hover:bg-blue-500 text-white font-semibold h-11 mt-1">
                    {loading ? 'Resetting…' : 'Reset password'}
                  </Button>
                </form>
              )}
              <button onClick={() => { setForgotStep('off'); setError(null) }} className="mt-4 text-sm text-blue-300 hover:text-white transition-colors w-full text-center">
                Back to sign in
              </button>
            </>
          ) : newPasswordSession ? (
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
              <button
                onClick={() => { setForgotStep('email'); setError(null) }}
                className="mt-3 text-sm text-blue-300 hover:text-white transition-colors w-full text-center"
              >
                Forgot password?
              </button>
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
