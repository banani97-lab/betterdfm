'use client'

import { useEffect } from 'react'
import Link from 'next/link'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    // Surface the error in the console for debugging / error reporting.
    console.error(error)
  }, [error])

  return (
    <main className="min-h-screen flex items-center justify-center p-6">
      <div className="flex flex-col items-center justify-center max-w-lg w-full p-10 border-2 border-dashed border-border rounded-2xl bg-card/45 text-center">
        <AlertTriangle className="h-12 w-12 text-destructive mb-4" />
        <h1 className="text-lg font-medium text-foreground">Something went wrong</h1>
        <p className="text-sm text-muted-foreground mt-1 mb-6 break-words max-w-full">
          {error.message || 'An unexpected error occurred.'}
        </p>
        <div className="flex items-center gap-3">
          <Button onClick={() => reset()}>
            <RefreshCw className="h-4 w-4 mr-2" /> Try again
          </Button>
          <Link href="/dashboard">
            <Button variant="outline">Back to dashboard</Button>
          </Link>
        </div>
      </div>
    </main>
  )
}
