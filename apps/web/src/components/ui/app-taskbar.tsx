'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Cog, FolderOpen, LogOut, Plus, X } from 'lucide-react'
import { signOut, canWrite } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { ThemeToggle } from '@/components/ui/theme-toggle'
import { useUiSettings, type BackgroundStyle, type TableDensity } from '@/lib/useUiSettings'
import { cn } from '@/lib/utils'

const BACKGROUND_OPTIONS: Array<{ id: BackgroundStyle; label: string }> = [
  { id: 'studio', label: 'Studio' },
  { id: 'spotlight', label: 'Spotlight' },
  { id: 'grid', label: 'Grid' },
  { id: 'aurora', label: 'Aurora' },
]

const DENSITY_OPTIONS: Array<{ id: TableDensity; label: string }> = [
  { id: 'comfortable', label: 'Comfortable' },
  { id: 'compact', label: 'Compact' },
]

// AppTaskbar is the global navigation cluster: theme toggle, settings panel,
// sign out, projects, and upload. It is self-contained — drop it into the
// right side of any page header.
//
// By default the buttons reveal their labels on hover of the cluster
// (`group/taskbar`). On headers that also carry a page title there isn't room
// for the expanded labels, so pass `expandOnHover={false}` to keep the cluster
// at its compact icon width (labels remain available via tooltips).
export function AppTaskbar({ className, expandOnHover = true }: { className?: string; expandOnHover?: boolean }) {
  const router = useRouter()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const { settings, update } = useUiSettings()

  const handleLogout = () => {
    signOut()
    router.replace('/login')
  }

  return (
    <>
      <div className={cn('flex w-full md:w-auto flex-wrap md:flex-nowrap items-center justify-end gap-2', expandOnHover && 'group/taskbar', className)}>
        <ThemeToggle className="h-11 w-11" />

        <Button
          variant="ghost"
          size="icon"
          className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
          onClick={() => setSettingsOpen(true)}
          aria-label="Open settings"
          title="Open settings"
        >
          <span className="flex items-center justify-center w-full">
            <Cog className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
            <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
              Settings
            </span>
          </span>
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
          onClick={handleLogout}
          aria-label="Sign out"
          title="Sign out"
        >
          <span className="flex items-center justify-center w-full">
            <LogOut className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
            <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
              Sign out
            </span>
          </span>
        </Button>

        <Link href="/projects">
          <Button
            variant="ghost"
            size="icon"
            className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
            aria-label="Projects"
            title="Projects"
          >
            <span className="flex items-center justify-center w-full">
              <FolderOpen className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
              <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
                Projects
              </span>
            </span>
          </Button>
        </Link>

        {canWrite() && (
          <Link href="/upload">
            <Button
              size="icon"
              className="h-10 w-auto px-3 md:h-11 md:w-11 md:px-0 overflow-hidden transition-all duration-300 md:group-hover/taskbar:w-32"
              aria-label="Upload"
              title="Upload"
            >
              <span className="flex items-center justify-center w-full">
                <Plus className="h-5 w-5 shrink-0 transition-transform duration-300 md:group-hover/taskbar:-translate-x-0.5" />
                <span className="ml-2 whitespace-nowrap text-sm md:ml-0 md:max-w-0 md:opacity-0 md:overflow-hidden md:transition-all md:duration-300 md:group-hover/taskbar:max-w-20 md:group-hover/taskbar:opacity-100 md:group-hover/taskbar:ml-2">
                  Upload
                </span>
              </span>
            </Button>
          </Link>
        )}
      </div>

      {settingsOpen && (
        <div className="fixed inset-0 z-50">
          <button
            type="button"
            className="absolute inset-0 bg-black/45"
            onClick={() => setSettingsOpen(false)}
            aria-label="Close settings"
          />
          <aside
            className="absolute right-0 top-0 h-full w-full max-w-lg bg-card border-l border-border shadow-2xl p-6 overflow-y-auto"
            role="dialog"
            aria-modal="true"
            aria-label="Settings"
          >
            <div className="flex items-start justify-between gap-4 mb-6">
              <div>
                <p className="text-xs uppercase tracking-[0.12em] text-muted-foreground mb-2">General Settings</p>
                <h2 className="text-2xl font-semibold text-foreground">Workspace Preferences</h2>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-10 w-10"
                onClick={() => setSettingsOpen(false)}
                aria-label="Close settings panel"
              >
                <X className="h-5 w-5" />
              </Button>
            </div>

            <div className="space-y-6">
              <section className="rounded-xl border border-border/80 bg-muted/20 p-4">
                <h3 className="font-medium text-foreground mb-3">Background Style</h3>
                <div className="grid grid-cols-4 gap-3">
                  {BACKGROUND_OPTIONS.map((bg) => (
                    <button
                      key={bg.id}
                      type="button"
                      className={cn(
                        'rounded-lg border p-3 text-left transition-colors',
                        settings.background === bg.id ? 'border-primary bg-primary/10' : 'border-border hover:bg-muted/40'
                      )}
                      onClick={() => update({ background: bg.id })}
                    >
                      <p className="text-sm font-medium text-foreground">{bg.label}</p>
                    </button>
                  ))}
                </div>
              </section>

              <section className="rounded-xl border border-border/80 bg-muted/20 p-4">
                <h3 className="font-medium text-foreground mb-3">Submissions Layout</h3>
                <div className="grid grid-cols-2 gap-3">
                  {DENSITY_OPTIONS.map((density) => (
                    <button
                      key={density.id}
                      type="button"
                      className={cn(
                        'rounded-lg border p-3 text-left transition-colors',
                        settings.tableDensity === density.id ? 'border-primary bg-primary/10' : 'border-border hover:bg-muted/40'
                      )}
                      onClick={() => update({ tableDensity: density.id })}
                    >
                      <p className="text-sm font-medium text-foreground">{density.label}</p>
                    </button>
                  ))}
                </div>
              </section>

              <section className="rounded-xl border border-border/80 bg-muted/20 p-4">
                <h3 className="font-medium text-foreground mb-3">Quick Access</h3>
                <Link href="/admin/profile" onClick={() => setSettingsOpen(false)}>
                  <Button variant="outline" className="w-full justify-start">Capability Profiles</Button>
                </Link>
              </section>
            </div>
          </aside>
        </div>
      )}
    </>
  )
}
