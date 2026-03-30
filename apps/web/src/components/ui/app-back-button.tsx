import type { ComponentPropsWithoutRef } from 'react'
import Link from 'next/link'
import { ArrowLeft } from 'lucide-react'
import { cn } from '@/lib/utils'

type AppBackButtonProps = Omit<ComponentPropsWithoutRef<typeof Link>, 'href'> & {
  href: string
  label: string
  caption?: string
  tone?: 'default' | 'inverse'
}

export function AppBackButton({
  href,
  label,
  caption = 'Back to',
  tone = 'default',
  className,
  ...props
}: AppBackButtonProps) {
  const inverse = tone === 'inverse'

  return (
    <Link
      href={href}
      className={cn(
        'group relative inline-flex w-fit items-center gap-3 overflow-hidden rounded-full px-3 py-2 pr-4 text-left transition-all duration-200 focus-visible:outline-none focus-visible:ring-2',
        inverse
          ? 'border border-white/10 bg-white/10 text-white shadow-[0_18px_40px_-28px_rgba(15,23,42,0.75)] ring-1 ring-white/10 focus-visible:ring-orange-400/60 hover:-translate-y-0.5 hover:border-orange-400/35 hover:bg-white/14 hover:shadow-[0_22px_48px_-30px_rgba(249,115,22,0.45)]'
          : 'border border-slate-200/80 bg-white/85 shadow-[0_16px_40px_-28px_rgba(15,23,42,0.55)] ring-1 ring-white/60 backdrop-blur-md focus-visible:ring-sky-400/60 hover:-translate-y-0.5 hover:border-sky-300/80 hover:shadow-[0_22px_48px_-30px_rgba(14,165,233,0.55)] dark:border-white/10 dark:bg-white/5 dark:shadow-none dark:ring-white/5 dark:backdrop-blur-none dark:hover:border-sky-400/30 dark:hover:bg-white/10',
        className
      )}
      {...props}
    >
      <span
        className={cn(
          'pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-200 group-hover:opacity-100',
          inverse
            ? 'bg-[radial-gradient(circle_at_left,rgba(249,115,22,0.16),transparent_48%)]'
            : 'bg-[radial-gradient(circle_at_left,rgba(56,189,248,0.14),transparent_48%)] dark:bg-[radial-gradient(circle_at_left,rgba(56,189,248,0.18),transparent_48%)]'
        )}
      />
      <span
        className={cn(
          'relative flex h-9 w-9 items-center justify-center rounded-full transition-transform duration-200 group-hover:-translate-x-0.5',
          inverse
            ? 'border border-white/15 bg-white/10 text-white'
            : 'border border-sky-500/20 bg-sky-500/10 text-sky-700 dark:border-sky-400/25 dark:bg-sky-400/10 dark:text-sky-300'
        )}
      >
        <ArrowLeft className="h-4 w-4" />
      </span>
      <span className="relative flex flex-col leading-tight">
        <span
          className={cn(
            'text-[10px] font-semibold uppercase tracking-[0.18em]',
            inverse ? 'text-slate-400' : 'text-slate-500 dark:text-slate-400'
          )}
        >
          {caption}
        </span>
        <span
          className={cn(
            'text-sm font-semibold',
            inverse ? 'text-white' : 'text-slate-950 dark:text-slate-50'
          )}
        >
          {label}
        </span>
      </span>
    </Link>
  )
}
