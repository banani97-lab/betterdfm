import Link from 'next/link'
import { FileQuestion } from 'lucide-react'
import { Button } from '@/components/ui/button'

export default function NotFound() {
  return (
    <main className="min-h-screen flex items-center justify-center p-6">
      <div className="flex flex-col items-center justify-center max-w-lg w-full p-10 border-2 border-dashed border-border rounded-2xl bg-card/45 text-center">
        <FileQuestion className="h-12 w-12 text-muted-foreground mb-4" />
        <h1 className="text-lg font-medium text-foreground">Page not found</h1>
        <p className="text-sm text-muted-foreground mt-1 mb-6">
          The page you are looking for does not exist or may have been moved.
        </p>
        <Link href="/dashboard">
          <Button variant="outline">Back to dashboard</Button>
        </Link>
      </div>
    </main>
  )
}
