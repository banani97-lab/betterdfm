'use client'

import { useState, useEffect, useCallback } from 'react'
import { X, Copy, Check, Link2, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  createShareLink,
  getShareLinks,
  deactivateShareLink,
  type ShareLink,
} from '@/lib/api'

interface ShareLinkModalProps {
  /** If provided, creates project-scoped share links */
  projectId?: string
  /** If provided, creates job-scoped share links */
  jobId?: string
  open: boolean
  onClose: () => void
}

export function ShareLinkModal({ projectId, jobId, open, onClose }: ShareLinkModalProps) {
  const [links, setLinks] = useState<ShareLink[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  // Form state
  const [label, setLabel] = useState('')
  const [expiresIn, setExpiresIn] = useState('7') // days, '' for no expiry
  const [allowUpload, setAllowUpload] = useState(false)
  const [newShareUrl, setNewShareUrl] = useState<string | null>(null)

  const loadLinks = useCallback(async () => {
    try {
      const allLinks = await getShareLinks()
      // Filter to links matching the current scope
      const filtered = allLinks.filter((l) => {
        if (projectId) return l.projectId === projectId
        if (jobId) return l.jobId === jobId
        return true
      })
      setLinks(filtered)
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [projectId, jobId])

  useEffect(() => {
    if (open) {
      setLoading(true)
      loadLinks()
      setNewShareUrl(null)
    }
  }, [open, loadLinks])

  const handleCreate = async () => {
    if (!label.trim()) return
    setCreating(true)
    try {
      let expiresAt: string | undefined
      if (expiresIn) {
        const d = new Date()
        d.setDate(d.getDate() + parseInt(expiresIn, 10))
        expiresAt = d.toISOString()
      }
      const result = await createShareLink({
        projectId: projectId || undefined,
        jobId: jobId || undefined,
        label: label.trim(),
        expiresAt,
        allowUpload,
      })
      const fullUrl = `${window.location.origin}/share/${result.token}`
      setNewShareUrl(fullUrl)
      setLabel('')
      setExpiresIn('7')
      setAllowUpload(false)
      await loadLinks()
    } catch {
      // ignore
    } finally {
      setCreating(false)
    }
  }

  const handleDeactivate = async (id: string) => {
    try {
      await deactivateShareLink(id)
      setLinks((prev) => prev.filter((l) => l.id !== id))
    } catch {
      // ignore
    }
  }

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-card border rounded-xl shadow-lg w-full max-w-lg mx-4 max-h-[80vh] overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b">
          <div className="flex items-center gap-2">
            <Link2 className="h-5 w-5 text-muted-foreground" />
            <h2 className="text-lg font-semibold text-foreground">Share Links</h2>
          </div>
          <button onClick={onClose} className="p-1 rounded hover:bg-muted transition-colors">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-6">
          {/* Newly created URL */}
          {newShareUrl && (
            <div className="bg-green-50 dark:bg-green-950/30 border border-green-200 dark:border-green-800 rounded-lg p-4">
              <p className="text-sm font-medium text-green-800 dark:text-green-200 mb-2">Share link created!</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 text-xs bg-white dark:bg-background p-2 rounded border break-all">
                  {newShareUrl}
                </code>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleCopy(newShareUrl, 'new')}
                >
                  {copiedId === 'new' ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>
          )}

          {/* Create form */}
          <div className="space-y-3">
            <h3 className="text-sm font-medium text-foreground">Create new link</h3>
            <input
              type="text"
              placeholder="Label (e.g. 'For customer review')"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              className="w-full px-3 py-2 text-sm border rounded-md bg-background"
            />
            <div className="flex items-center gap-3">
              <label className="text-sm text-muted-foreground">Expires in:</label>
              <select
                value={expiresIn}
                onChange={(e) => setExpiresIn(e.target.value)}
                className="px-2 py-1.5 text-sm border rounded bg-background"
              >
                <option value="1">1 day</option>
                <option value="7">7 days</option>
                <option value="30">30 days</option>
                <option value="90">90 days</option>
                <option value="">Never</option>
              </select>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={allowUpload}
                onChange={(e) => setAllowUpload(e.target.checked)}
                className="rounded"
              />
              Allow customer to upload revised files
            </label>
            <Button onClick={handleCreate} disabled={creating || !label.trim()} className="w-full">
              {creating ? 'Creating...' : 'Create Share Link'}
            </Button>
          </div>

          {/* Existing links */}
          <div className="space-y-2">
            <h3 className="text-sm font-medium text-foreground">
              Active links {!loading && `(${links.filter((l) => l.active).length})`}
            </h3>
            {loading ? (
              <p className="text-sm text-muted-foreground">Loading...</p>
            ) : links.filter((l) => l.active).length === 0 ? (
              <p className="text-sm text-muted-foreground">No active share links.</p>
            ) : (
              links
                .filter((l) => l.active)
                .map((link) => {
                  const shareUrl = `${window.location.origin}/share/${link.token}`
                  return (
                    <div
                      key={link.id}
                      className="flex items-center gap-2 p-3 border rounded-lg bg-background"
                    >
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate">{link.label}</p>
                        <p className="text-xs text-muted-foreground">
                          {link.expiresAt
                            ? `Expires ${new Date(link.expiresAt).toLocaleDateString()}`
                            : 'No expiry'}
                          {link.allowUpload && ' \u00b7 Uploads allowed'}
                        </p>
                      </div>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleCopy(shareUrl, link.id)}
                        title="Copy link"
                      >
                        {copiedId === link.id ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleDeactivate(link.id)}
                        title="Deactivate link"
                        className="text-red-500 hover:text-red-600"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  )
                })
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
