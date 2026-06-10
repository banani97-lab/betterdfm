'use client'

import { useEffect } from 'react'

// Global error boundary: replaces the root layout when it (or this boundary's
// children) throw, so it must render its own <html>/<body> and cannot rely on
// the app's stylesheet or components being present. Keep it plain.
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    console.error(error)
  }, [error])

  return (
    <html lang="en">
      <body
        style={{
          margin: 0,
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontFamily:
            'ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif',
          background: '#0a0a0a',
          color: '#fafafa',
        }}
      >
        <div style={{ textAlign: 'center', padding: '2rem', maxWidth: '32rem' }}>
          <h1 style={{ fontSize: '1.125rem', fontWeight: 500, margin: '0 0 0.5rem' }}>
            Something went wrong
          </h1>
          <p style={{ fontSize: '0.875rem', color: '#a3a3a3', margin: '0 0 1.5rem' }}>
            {error.message || 'An unexpected error occurred.'}
          </p>
          <button
            onClick={() => reset()}
            style={{
              padding: '0.5rem 1rem',
              borderRadius: '0.375rem',
              border: '1px solid #404040',
              background: 'transparent',
              color: '#fafafa',
              fontSize: '0.875rem',
              cursor: 'pointer',
              marginRight: '0.75rem',
            }}
          >
            Try again
          </button>
          <a href="/dashboard" style={{ fontSize: '0.875rem', color: '#a3a3a3' }}>
            Back to dashboard
          </a>
        </div>
      </body>
    </html>
  )
}
