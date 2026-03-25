'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Save, Plus, Building2 } from 'lucide-react'
import { isAdminLoggedIn, adminApiFetch } from '@/lib/adminAuth'
import type { Organization } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function AdminOrganizationsPage() {
  const router = useRouter()
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [selected, setSelected] = useState<Organization | null>(null)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [logoUrl, setLogoUrl] = useState('')
  const [saving, setSaving] = useState(false)
  const [creating, setCreating] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [newName, setNewName] = useState('')
  const [newSlug, setNewSlug] = useState('')

  useEffect(() => {
    if (!isAdminLoggedIn()) { router.replace('/admin/login'); return }
    loadOrgs()
  }, [router])

  const loadOrgs = async () => {
    try {
      const data = await adminApiFetch<Organization[]>('/admin/organizations')
      setOrgs(data ?? [])
      if (!selected && data?.length > 0) selectOrg(data[0])
    } catch (e: unknown) {
      if (e instanceof Error) setMessage({ type: 'error', text: e.message })
    }
  }

  const selectOrg = (org: Organization) => {
    setSelected(org)
    setName(org.name)
    setSlug(org.slug)
    setLogoUrl(org.logoUrl || '')
  }

  const handleSave = async () => {
    if (!selected) return
    setSaving(true)
    setMessage(null)
    try {
      const updated = await adminApiFetch<Organization>(`/admin/organizations/${selected.id}`, {
        method: 'PUT',
        body: JSON.stringify({ name, slug, logoUrl }),
      })
      setSelected(updated)
      await loadOrgs()
      setMessage({ type: 'success', text: 'Organization saved.' })
    } catch (e: unknown) {
      setMessage({ type: 'error', text: e instanceof Error ? e.message : String(e) })
    } finally {
      setSaving(false)
    }
  }

  const handleCreate = async () => {
    if (!newName.trim() || !newSlug.trim()) return
    setCreating(true)
    try {
      const org = await adminApiFetch<Organization>('/admin/organizations', {
        method: 'POST',
        body: JSON.stringify({ name: newName, slug: newSlug }),
      })
      await loadOrgs()
      selectOrg(org)
      setNewName('')
      setNewSlug('')
      setMessage({ type: 'success', text: 'Organization created.' })
    } catch (e: unknown) {
      setMessage({ type: 'error', text: e instanceof Error ? e.message : String(e) })
    } finally {
      setCreating(false)
    }
  }

  const autoSlug = (value: string) => {
    setNewName(value)
    setNewSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''))
  }

  return (
    <div className="min-h-screen bg-slate-950">
      <header className="bg-slate-900 border-b border-slate-800 px-6 py-4 flex items-center justify-between gap-4 sticky top-0 z-30">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-orange-600 flex items-center justify-center">
            <Building2 className="h-4 w-4 text-white" />
          </div>
          <h1 className="text-xl font-semibold text-white">Organizations</h1>
        </div>
        <a href="/admin" className="text-sm text-slate-400 hover:text-white">Back to Admin</a>
      </header>

      <main className="max-w-5xl mx-auto px-6 py-8 grid grid-cols-3 gap-6">
        {/* Org list */}
        <div className="col-span-1">
          <div className="bg-slate-900 rounded-lg border border-slate-800 p-4">
            <h2 className="font-semibold text-white mb-3">Companies</h2>
            <div className="space-y-1">
              {orgs.map((org) => (
                <div
                  key={org.id}
                  className={`flex items-center px-3 py-2 rounded cursor-pointer text-sm ${selected?.id === org.id ? 'bg-orange-600/20 text-orange-400 font-medium' : 'hover:bg-slate-800 text-slate-400'}`}
                  onClick={() => selectOrg(org)}
                >
                  <span className="truncate">{org.name}</span>
                  <span className="ml-auto text-xs text-slate-500">{org.slug}</span>
                </div>
              ))}
              {orgs.length === 0 && (
                <p className="text-xs text-slate-500 py-2">No organizations yet</p>
              )}
            </div>

            {/* Create new */}
            <div className="mt-4 pt-4 border-t border-slate-800">
              <p className="text-xs font-medium text-slate-400 mb-2">New Organization</p>
              <Input
                placeholder="Company name"
                value={newName}
                onChange={(e) => autoSlug(e.target.value)}
                className="mb-2 text-sm h-8 bg-slate-800 border-slate-700 text-white placeholder:text-slate-500"
              />
              <Input
                placeholder="slug"
                value={newSlug}
                onChange={(e) => setNewSlug(e.target.value)}
                className="mb-2 text-sm h-8 bg-slate-800 border-slate-700 text-white placeholder:text-slate-500"
              />
              <Button
                size="sm"
                onClick={handleCreate}
                disabled={creating || !newName.trim() || !newSlug.trim()}
                className="w-full bg-orange-600 hover:bg-orange-500 text-white"
              >
                <Plus className="h-3.5 w-3.5 mr-1" /> Create
              </Button>
            </div>
          </div>
        </div>

        {/* Org editor */}
        <div className="col-span-2">
          {selected ? (
            <div className="bg-slate-900 rounded-lg border border-slate-800 p-6">
              <h3 className="font-semibold text-white mb-4">Edit Organization</h3>

              <div className="space-y-4">
                <div>
                  <Label className="mb-1 block text-sm text-slate-300">Name</Label>
                  <Input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="bg-slate-800 border-slate-700 text-white"
                  />
                </div>
                <div>
                  <Label className="mb-1 block text-sm text-slate-300">Slug</Label>
                  <Input
                    value={slug}
                    onChange={(e) => setSlug(e.target.value)}
                    className="bg-slate-800 border-slate-700 text-white"
                  />
                  <p className="text-xs text-slate-500 mt-1">URL-safe identifier, must be unique</p>
                </div>
                <div>
                  <Label className="mb-1 block text-sm text-slate-300">Logo URL</Label>
                  <Input
                    value={logoUrl}
                    onChange={(e) => setLogoUrl(e.target.value)}
                    placeholder="https://..."
                    className="bg-slate-800 border-slate-700 text-white placeholder:text-slate-500"
                  />
                </div>
                <div>
                  <Label className="mb-1 block text-sm text-slate-300">Organization ID</Label>
                  <p className="text-sm text-slate-400 font-mono bg-slate-800 px-3 py-2 rounded border border-slate-700">{selected.id}</p>
                </div>
                <div>
                  <Label className="mb-1 block text-sm text-slate-300">Created</Label>
                  <p className="text-sm text-slate-400">{new Date(selected.createdAt).toLocaleString()}</p>
                </div>
              </div>

              {message && (
                <div className={`mt-4 p-3 rounded text-sm ${message.type === 'success' ? 'bg-green-900/30 text-green-400 border border-green-800' : 'bg-red-900/30 text-red-400 border border-red-800'}`}>
                  {message.text}
                </div>
              )}

              <div className="mt-6 flex justify-end">
                <Button
                  onClick={handleSave}
                  disabled={saving}
                  className="bg-orange-600 hover:bg-orange-500 text-white"
                >
                  <Save className="h-4 w-4 mr-1" />
                  {saving ? 'Saving…' : 'Save'}
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-center h-64 bg-slate-900 rounded-lg border border-slate-800 text-slate-500">
              Select or create an organization
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
