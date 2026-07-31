'use client'

import { QRCodeSVG } from 'qrcode.react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface MfaPanelProps {
  mode: 'setup' | 'challenge'
  issuer: string
  account: string
  secret?: string
  code: string
  onCodeChange: (v: string) => void
  onSubmit: (e: React.FormEvent) => void
  loading: boolean
  error: string | null
  buttonClassName: string
  inputClassName: string
}

/** Insert a space every 4 chars so the TOTP secret is readable for manual entry. */
function formatSecret(s: string): string {
  return s.replace(/(.{4})/g, '$1 ').trim()
}

/** Build the otpauth:// URI an authenticator app scans. Rendered to a QR
 *  entirely client-side (qrcode.react), so the secret never leaves the browser. */
function otpauthUri(issuer: string, account: string, secret: string): string {
  const label = `${encodeURIComponent(issuer)}:${encodeURIComponent(account)}`
  const params = new URLSearchParams({ secret, issuer })
  return `otpauth://totp/${label}?${params.toString()}`
}

/**
 * Shared TOTP MFA UI for the app and admin login pages. In 'setup' mode it
 * shows the shared secret for manual entry into an authenticator app; in
 * 'challenge' mode it just collects the 6-digit code. Colours are passed in
 * (buttonClassName / inputClassName) so each page keeps its own theme.
 */
export function MfaPanel({
  mode,
  issuer,
  account,
  secret,
  code,
  onCodeChange,
  onSubmit,
  loading,
  error,
  buttonClassName,
  inputClassName,
}: MfaPanelProps) {
  return (
    <>
      <h2 className="text-xl font-semibold text-white mb-1">
        {mode === 'setup' ? 'Set up authenticator' : 'Two-factor authentication'}
      </h2>
      <p className="text-white/70 text-sm mb-6">
        {mode === 'setup'
          ? 'This account requires multi-factor authentication. Add the key below to an authenticator app (Google Authenticator, 1Password, Authy…), then enter the 6-digit code it shows.'
          : 'Enter the 6-digit code from your authenticator app.'}
      </p>

      {error && (
        <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-400/20 text-red-300 text-sm">
          {error}
        </div>
      )}

      {mode === 'setup' && secret && (
        <div className="mb-4 rounded-lg border border-white/20 bg-white/5 p-4">
          <p className="text-xs text-white/70 mb-3">
            Scan this with your authenticator app (Google Authenticator, 1Password, Authy…):
          </p>
          <div className="flex justify-center mb-3">
            <div className="rounded-lg bg-white p-3">
              <QRCodeSVG value={otpauthUri(issuer, account, secret)} size={160} />
            </div>
          </div>
          <details className="text-xs text-white/60">
            <summary className="cursor-pointer select-none hover:text-white/90">
              Can&rsquo;t scan? Enter a setup key instead
            </summary>
            <code className="mt-2 block text-sm font-mono text-white break-all tracking-wide select-all">
              {formatSecret(secret)}
            </code>
            <p className="text-[11px] text-white/50 mt-1">
              Account: {account} &middot; Issuer: {issuer}
            </p>
          </details>
        </div>
      )}

      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="mfa-code" className="text-white/90 text-sm">
            Authenticator code
          </Label>
          <Input
            id="mfa-code"
            inputMode="numeric"
            autoComplete="one-time-code"
            placeholder="123456"
            value={code}
            onChange={(e) => onCodeChange(e.target.value)}
            required
            className={inputClassName}
          />
        </div>
        <Button type="submit" disabled={loading} className={buttonClassName}>
          {loading ? 'Verifying…' : mode === 'setup' ? 'Verify & continue' : 'Verify'}
        </Button>
      </form>
    </>
  )
}
