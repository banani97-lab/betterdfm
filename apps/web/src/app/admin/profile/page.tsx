'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Save, Plus, Trash2 } from 'lucide-react'
import {
  getProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  type CapabilityProfile,
  type ProfileRules,
} from '@/lib/api'
import { isLoggedIn } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { BetterDFMLogo } from '@/components/ui/betterdfm-logo'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const DEFAULT_RULES: ProfileRules = {
  minTraceWidthMM: 0.15,
  minClearanceMM: 0.15,
  minDrillDiamMM: 0.3,
  maxDrillDiamMM: 6.3,
  minAnnularRingMM: 0.15,
  maxAspectRatio: 10,
  minSolderMaskDamMM: 0.1,
  minEdgeClearanceMM: 0.3,
}

const RULE_FIELDS: Array<{ key: keyof ProfileRules; label: string; unit: string; step: string }> = [
  { key: 'minTraceWidthMM', label: 'Min Trace Width', unit: 'mm', step: '0.01' },
  { key: 'minClearanceMM', label: 'Min Clearance', unit: 'mm', step: '0.01' },
  { key: 'minDrillDiamMM', label: 'Min Drill Diameter', unit: 'mm', step: '0.01' },
  { key: 'maxDrillDiamMM', label: 'Max Drill Diameter', unit: 'mm', step: '0.1' },
  { key: 'minAnnularRingMM', label: 'Min Annular Ring', unit: 'mm', step: '0.01' },
  { key: 'maxAspectRatio', label: 'Max Aspect Ratio', unit: ':1', step: '0.5' },
  { key: 'minSolderMaskDamMM', label: 'Min Solder Mask Dam', unit: 'mm', step: '0.01' },
  { key: 'minEdgeClearanceMM', label: 'Min Edge Clearance', unit: 'mm', step: '0.01' },
]

export default function AdminProfilePage() {
  const router = useRouter()
  const [profiles, setProfiles] = useState<CapabilityProfile[]>([])
  const [selected, setSelected] = useState<CapabilityProfile | null>(null)
  const [rules, setRules] = useState<ProfileRules>(DEFAULT_RULES)
  const [name, setName] = useState('')
  const [isDefault, setIsDefault] = useState(false)
  const [saving, setSaving] = useState(false)
  const [creating, setCreating] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [newName, setNewName] = useState('')

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    loadProfiles()
  }, [router])

  const loadProfiles = async () => {
    try {
      const ps = await getProfiles()
      setProfiles(ps ?? [])
      if (!selected && ps?.length > 0) selectProfile(ps[0])
    } catch (e: unknown) {
      if (e instanceof Error) setMessage({ type: 'error', text: e.message })
    }
  }

  const selectProfile = (p: CapabilityProfile) => {
    setSelected(p)
    setName(p.name)
    setIsDefault(p.isDefault)
    setRules(p.rules ?? DEFAULT_RULES)
  }

  const handleSave = async () => {
    if (!selected) return
    setSaving(true)
    setMessage(null)
    try {
      const updated = await updateProfile(selected.id, { name, isDefault, rules })
      setSelected(updated)
      await loadProfiles()
      setMessage({ type: 'success', text: 'Profile saved successfully.' })
    } catch (e: unknown) {
      setMessage({ type: 'error', text: e instanceof Error ? e.message : String(e) })
    } finally {
      setSaving(false)
    }
  }

  const handleCreate = async () => {
    if (!newName.trim()) return
    setCreating(true)
    try {
      const p = await createProfile({ name: newName, isDefault: profiles.length === 0, rules: DEFAULT_RULES })
      await loadProfiles()
      selectProfile(p)
      setNewName('')
      setMessage({ type: 'success', text: 'Profile created.' })
    } catch (e: unknown) {
      setMessage({ type: 'error', text: e instanceof Error ? e.message : String(e) })
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this profile?')) return
    try {
      await deleteProfile(id)
      const remaining = profiles.filter((p) => p.id !== id)
      setProfiles(remaining)
      if (selected?.id === id) {
        if (remaining.length > 0) selectProfile(remaining[0])
        else setSelected(null)
      }
    } catch (e: unknown) {
      setMessage({ type: 'error', text: e instanceof Error ? e.message : String(e) })
    }
  }

  const setRuleValue = (key: keyof ProfileRules, val: string) => {
    setRules((r) => ({ ...r, [key]: parseFloat(val) || 0 }))
  }

  return (
    <div className="min-h-screen">
      <header className="bg-card/65 border-b border-border/80 px-6 py-4 flex items-center justify-between gap-4 sticky top-0 z-30">
        <BetterDFMLogo />
        <h1 className="text-xl font-semibold text-foreground">Capability Profiles</h1>
      </header>

      <main className="max-w-5xl mx-auto px-6 py-8 grid grid-cols-3 gap-6">
        {/* Profile list */}
        <div className="col-span-1">
          <div className="bg-card rounded-lg border p-4">
            <h2 className="font-semibold text-foreground mb-3">Profiles</h2>
            <div className="space-y-1">
              {profiles.map((p) => (
                <div
                  key={p.id}
                  className={`flex items-center justify-between px-3 py-2 rounded cursor-pointer text-sm ${selected?.id === p.id ? 'bg-primary/15 text-primary font-medium' : 'hover:bg-muted/40 text-muted-foreground'}`}
                  onClick={() => selectProfile(p)}
                >
                  <span className="truncate">{p.name}{p.isDefault ? ' ★' : ''}</span>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDelete(p.id) }}
                    className="text-muted-foreground hover:text-destructive ml-2 flex-shrink-0"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
              {profiles.length === 0 && (
                <p className="text-xs text-muted-foreground py-2">No profiles yet</p>
              )}
            </div>

            {/* Create new */}
            <div className="mt-4 pt-4 border-t">
              <p className="text-xs font-medium text-muted-foreground mb-2">New Profile</p>
              <Input
                placeholder="Profile name"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                className="mb-2 text-sm h-8"
                onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              />
              <Button size="sm" onClick={handleCreate} disabled={creating || !newName.trim()} className="w-full">
                <Plus className="h-3.5 w-3.5 mr-1" /> Create
              </Button>
            </div>
          </div>
        </div>

        {/* Rule editor */}
        <div className="col-span-2">
          {selected ? (
            <div className="bg-card rounded-lg border p-6">
              <div className="flex items-start justify-between mb-6">
                <div className="flex-1 mr-4">
                  <Label className="mb-1 block">Profile Name</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>
                <label className="flex items-center gap-2 mt-6 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={isDefault}
                    onChange={(e) => setIsDefault(e.target.checked)}
                    className="w-4 h-4"
                  />
                  <span className="text-sm text-foreground">Default</span>
                </label>
              </div>

              <h3 className="font-semibold text-foreground mb-4">Manufacturing Rules</h3>
              <div className="grid grid-cols-2 gap-4">
                {RULE_FIELDS.map(({ key, label, unit, step }) => (
                  <div key={key}>
                    <Label className="mb-1 block text-xs">{label}</Label>
                    <div className="flex items-center gap-2">
                      <Input
                        type="number"
                        step={step}
                        min="0"
                        value={rules[key]}
                        onChange={(e) => setRuleValue(key, e.target.value)}
                        className="flex-1"
                      />
                      <span className="text-xs text-muted-foreground w-8 flex-shrink-0">{unit}</span>
                    </div>
                  </div>
                ))}
              </div>

              {message && (
                <div className={`mt-4 p-3 rounded text-sm ${message.type === 'success' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
                  {message.text}
                </div>
              )}

              <div className="mt-6 flex justify-end">
                <Button onClick={handleSave} disabled={saving}>
                  <Save className="h-4 w-4 mr-1" />
                  {saving ? 'Saving…' : 'Save Profile'}
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-center h-64 bg-card rounded-lg border text-muted-foreground">
              Select or create a profile
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
