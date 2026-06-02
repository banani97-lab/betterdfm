'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Save, Plus, Trash2, HelpCircle, ChevronDown } from 'lucide-react'
import {
  getProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  type CapabilityProfile,
  type ProfileRules,
} from '@/lib/api'
import { isLoggedIn } from '@/lib/auth'
import { useUsage } from '@/lib/useUsage'
import { AppBackButton } from '@/components/ui/app-back-button'
import { Button } from '@/components/ui/button'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { AppTaskbar } from '@/components/ui/app-taskbar'
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
  minDrillToDrillMM: 0.25,
  minDrillToCopperMM: 0.25,
  minCopperSliverMM: 0.1,
  smallestPackageClass: '',
  maxTraceImbalanceRatio: 2.0,
  enableSilkscreenOnPadCheck: true,
  maxComponentHeightTopMM: 10,
  maxComponentHeightBottomMM: 5,
  minComponentSpacingMM: 0.5,
  componentSpacing: { discreteMM: 0.254, leadedMM: 1.27, bgaMM: 3.175, throughHoleMM: 3.175 },
  flagThroughHoleOnBottom: true,
  minMountingHoleKeepoutMM: 0.5,
  enableFiducialPlacementCheck: true,
  enableFiducialCountCheck: true,
  enablePadSizeForPackageCheck: true,
  enableTombstoningRiskCheck: true,
  enableViaInPadCheck: true,
}

// A single configurable check. `number` and `select` carry a threshold; `toggle`
// is an on/off check with no numeric value; `spacing` is the per-package-class
// component-spacing sub-panel (a nested object, rendered full width).
const PACKAGE_CLASS_OPTIONS = ['01005', '0201', '0402', '0603', '0805', '1206', '1210', '1812', '2010', '2512']

type RuleField =
  | { kind: 'number'; key: keyof ProfileRules; label: string; unit: string; step: string; desc: string }
  | { kind: 'toggle'; key: keyof ProfileRules; label: string; desc: string }
  | { kind: 'select'; key: keyof ProfileRules; label: string; desc: string }
  | { kind: 'spacing'; label: string; desc: string }

// Every check, bucketed by manufacturing stage (bare-board fabrication first,
// then assembly). Mixing numeric thresholds and on/off toggles within a bucket
// keeps everything about one stage in one place.
const RULE_GROUPS: Array<{ title: string; blurb?: string; fields: RuleField[] }> = [
  {
    title: 'Copper & Routing',
    blurb: 'Bare-board copper features: widths, spacings, and outline clearance.',
    fields: [
      { kind: 'number', key: 'minTraceWidthMM', label: 'Min Trace Width', unit: 'mm', step: '0.01',
        desc: 'Flag trace segments narrower than this. Thinner traces are harder to etch cleanly and carry less current.' },
      { kind: 'number', key: 'minClearanceMM', label: 'Min Clearance', unit: 'mm', step: '0.01',
        desc: 'Minimum spacing between different-net copper features. Below this, you risk shorts during fabrication or bridging during assembly.' },
      { kind: 'number', key: 'minCopperSliverMM', label: 'Min Copper Sliver', unit: 'mm', step: '0.005',
        desc: 'Thinnest copper feature width that will survive etching. Thinner slivers become acid traps or shorts.' },
      { kind: 'number', key: 'minEdgeClearanceMM', label: 'Min Edge Clearance', unit: 'mm', step: '0.01',
        desc: 'Minimum distance from copper features to the board outline. Features too close risk exposure on the routed edge.' },
    ],
  },
  {
    title: 'Drilling & Vias',
    blurb: 'Hole sizes, hole-to-feature spacings, and plating aspect ratio.',
    fields: [
      { kind: 'number', key: 'minDrillDiamMM', label: 'Min Drill Diameter', unit: 'mm', step: '0.01',
        desc: 'Smallest mechanical drill the fab can reliably produce. Below this you need laser drilling.' },
      { kind: 'number', key: 'maxDrillDiamMM', label: 'Max Drill Diameter', unit: 'mm', step: '0.1',
        desc: 'Largest drill the fab will accept before routing is required instead. Tooling holes commonly exceed this.' },
      { kind: 'number', key: 'minAnnularRingMM', label: 'Min Annular Ring', unit: 'mm', step: '0.01',
        desc: 'Copper ring remaining around a drill hit after drilling tolerances. Below this, drill breakout can sever the connection.' },
      { kind: 'number', key: 'minDrillToDrillMM', label: 'Min Drill-to-Drill', unit: 'mm', step: '0.01',
        desc: 'Edge-to-edge spacing between drill holes. Below this, adjacent drill walls can break through into each other.' },
      { kind: 'number', key: 'minDrillToCopperMM', label: 'Min Drill-to-Copper', unit: 'mm', step: '0.01',
        desc: 'Drill hole edge to nearest other-net copper. Protects against drill bit wander clipping a neighboring trace.' },
      { kind: 'number', key: 'maxAspectRatio', label: 'Max Aspect Ratio', unit: ':1', step: '0.5',
        desc: 'Ratio of board thickness to smallest drill diameter. Higher ratios require premium electroplating — 10:1 is standard, 12:1 is HDI.' },
    ],
  },
  {
    title: 'Solder Mask, Silkscreen & Mechanical',
    blurb: 'Surface-finish webs, silkscreen overlap, and mounting-hole keepout.',
    fields: [
      { kind: 'number', key: 'minSolderMaskDamMM', label: 'Min Solder Mask Dam', unit: 'mm', step: '0.01',
        desc: 'Narrowest solder mask web between two pad openings. Below this the mask flakes off during reflow and you lose solder bridge protection.' },
      { kind: 'number', key: 'minMountingHoleKeepoutMM', label: 'Min Mounting-Hole Keepout', unit: 'mm', step: '0.05',
        desc: 'Minimum copper keepout from the edge of a non-plated mounting hole. Protects copper from the screw head and washer footprint per IPC-2221B generic clearance.' },
      { kind: 'toggle', key: 'enableSilkscreenOnPadCheck', label: 'Silkscreen-on-Pad Check',
        desc: 'Flag silkscreen features overlapping copper pads, which can lift or contaminate the solder joint.' },
    ],
  },
  {
    title: 'Assembly: Placement',
    blurb: 'Component spacing, height limits, side restrictions, and fiducials.',
    fields: [
      { kind: 'number', key: 'minComponentSpacingMM', label: 'Min Component Spacing', unit: 'mm', step: '0.05',
        desc: 'Baseline minimum courtyard edge-to-edge gap between adjacent same-side components. Below this, the pick-and-place nozzle cannot reach the part and rework becomes difficult. IPC-7351B nominal density implies about 0.5 mm. Used as the fallback when the per-class radii below are unset.' },
      { kind: 'spacing', label: 'Component Spacing by Package Class',
        desc: 'Per-class keepout radii for the component-spacing check. The required gap between two parts is the larger of their two radii, so a discrete next to a BGA uses the BGA radius. A zero field falls back to Min Component Spacing. Defaults follow CM practice: discrete 0.254 mm (10 mil), leaded QFN/QFP/PLCC/connector 1.27 mm (50 mil), BGA and through-hole pin 3.175 mm (125 mil).' },
      { kind: 'number', key: 'maxComponentHeightTopMM', label: 'Max Component Height (Top)', unit: 'mm', step: '0.5',
        desc: 'SMT-only cap on top-side component height. Limited by stencil printer head clearance and reflow oven conveyor height. Typical is 10 mm; precision assembly lines may need lower.' },
      { kind: 'number', key: 'maxComponentHeightBottomMM', label: 'Max Component Height (Bottom)', unit: 'mm', step: '0.5',
        desc: 'SMT-only cap on bottom-side component height. Limited by wave-solder pallet clearance and reflow pallet support height. Typical is 5 mm.' },
      { kind: 'toggle', key: 'flagThroughHoleOnBottom', label: 'Through-Hole on Bottom Side',
        desc: 'Flag THT / press-fit parts on the bottom side, which cannot be wave or reflow soldered normally.' },
      { kind: 'toggle', key: 'enableFiducialCountCheck', label: 'Fiducial-Count Check',
        desc: 'Require at least 3 fiducials for pick-and-place alignment when the board has any.' },
      { kind: 'toggle', key: 'enableFiducialPlacementCheck', label: 'Fiducial-Placement Check',
        desc: 'Flag collinear global fiducials and fine-pitch / BGA parts missing a nearby local fiducial (IPC-7351).' },
    ],
  },
  {
    title: 'Assembly: Footprints & Soldering',
    blurb: 'Package capability, land geometry, and reflow solder-joint risks.',
    fields: [
      { kind: 'select', key: 'smallestPackageClass', label: 'Smallest Placeable Package',
        desc: 'The smallest passive package class your line can place. Components smaller than this are flagged by the package-capability check. Leave unset to skip.' },
      { kind: 'toggle', key: 'enablePadSizeForPackageCheck', label: 'Pad-Size-for-Package Check',
        desc: 'Flag passive pad geometry outside the IPC-7351 envelope for the detected package class.' },
      { kind: 'number', key: 'maxTraceImbalanceRatio', label: 'Max Trace Imbalance Ratio', unit: ':1', step: '0.1',
        desc: 'Flag when the two traces on a 2-pad component differ in width by more than this ratio. Thermal asymmetry is a major cause of tombstoning during reflow.' },
      { kind: 'toggle', key: 'enableTombstoningRiskCheck', label: 'Tombstoning-Risk Check',
        desc: 'Flag small 2-pad passives with unbalanced pad areas that can tombstone during reflow.' },
      { kind: 'toggle', key: 'enableViaInPadCheck', label: 'Via-in-Pad Check',
        desc: 'Flag vias landing in SMT lands, which can wick solder away from the joint (IPC-4761 / 7093).' },
    ],
  },
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
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const toggleGroup = (title: string) => setCollapsed((c) => ({ ...c, [title]: !c[title] }))
  const { usage } = useUsage()
  const profileLimitReached = usage ? (usage.profiles.limit !== -1 && usage.profiles.used >= usage.profiles.limit) : false

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

  const SPACING_DEFAULTS = { discreteMM: 0.254, leadedMM: 1.27, bgaMM: 3.175, throughHoleMM: 3.175 }

  const setSpacingClass = (key: keyof typeof SPACING_DEFAULTS, val: string) => {
    setRules((r) => ({
      ...r,
      componentSpacing: { ...SPACING_DEFAULTS, ...(r.componentSpacing ?? {}), [key]: parseFloat(val) || 0 },
    }))
  }

  const renderField = (f: RuleField) => {
    if (f.kind === 'spacing') {
      return (
        <div key="spacing" className="col-span-2 rounded-md border border-border/70 bg-muted/20 p-3">
          <div className="flex items-center gap-1.5 mb-1">
            <span className="text-xs font-semibold text-foreground">{f.label}</span>
            <RuleHelp text={f.desc} />
          </div>
          <div className="grid grid-cols-2 gap-3 mt-2">
            {([
              ['discreteMM', 'Discrete (R/C/L)'],
              ['leadedMM', 'Leaded (QFN/QFP/PLCC/connector)'],
              ['bgaMM', 'BGA'],
              ['throughHoleMM', 'Through-Hole Pin'],
            ] as Array<[keyof typeof SPACING_DEFAULTS, string]>).map(([key, label]) => (
              <div key={key}>
                <Label className="block text-xs mb-1">{label}</Label>
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    step="0.05"
                    min="0"
                    value={rules.componentSpacing?.[key] ?? SPACING_DEFAULTS[key]}
                    onChange={(e) => setSpacingClass(key, e.target.value)}
                    className="flex-1"
                  />
                  <span className="text-xs text-muted-foreground w-8 flex-shrink-0">mm</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )
    }
    return (
      <div key={f.key}>
        <div className="flex items-center gap-1.5 mb-1">
          <Label className="block text-xs">{f.label}</Label>
          <RuleHelp text={f.desc} />
        </div>
        {f.kind === 'number' && (
          <div className="flex items-center gap-2">
            <Input
              type="number"
              step={f.step}
              min="0"
              value={rules[f.key] as number ?? 0}
              onChange={(e) => setRuleValue(f.key, e.target.value)}
              className="flex-1"
            />
            <span className="text-xs text-muted-foreground w-8 flex-shrink-0">{f.unit}</span>
          </div>
        )}
        {f.kind === 'select' && (
          <select
            value={(rules[f.key] as string | undefined) ?? ''}
            onChange={(e) => setRules((r) => ({ ...r, [f.key]: e.target.value }))}
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          >
            <option value="">Not set (skip check)</option>
            {PACKAGE_CLASS_OPTIONS.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
        )}
        {f.kind === 'toggle' && (
          <label className="flex items-center gap-2 cursor-pointer h-9">
            <input
              type="checkbox"
              checked={(rules[f.key] as boolean | undefined) ?? true}
              onChange={(e) => setRules((r) => ({ ...r, [f.key]: e.target.checked }))}
              className="w-4 h-4"
            />
            <span className="text-xs text-muted-foreground">
              {((rules[f.key] as boolean | undefined) ?? true) ? 'Enabled' : 'Disabled'}
            </span>
          </label>
        )}
      </div>
    )
  }

  return (
    <div className="min-h-screen">
      <header className="bg-card/65 border-b border-border/80 px-6 py-4 flex items-center gap-4 sticky top-0 z-30">
        <RapidDFMLogo className="shrink-0" />
        <h1 className="text-xl font-semibold text-foreground truncate">Capability Profiles</h1>
        <AppTaskbar expandOnHover={false} className="w-auto ml-auto shrink-0" />
      </header>

      <main className="max-w-5xl mx-auto px-6 py-8 grid grid-cols-3 gap-6">
        <div className="col-span-3">
          <AppBackButton href="/dashboard" label="Dashboard" />
        </div>

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
              {profileLimitReached ? (
                <p className="text-xs text-muted-foreground">Profile limit reached ({usage!.profiles.used}/{usage!.profiles.limit})</p>
              ) : (
                <>
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
                </>
              )}
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

              <div className="flex items-center justify-between mb-4">
                <h3 className="font-semibold text-foreground">Manufacturing Rules</h3>
                <button
                  type="button"
                  onClick={() => {
                    const anyOpen = RULE_GROUPS.some((g) => !collapsed[g.title])
                    setCollapsed(Object.fromEntries(RULE_GROUPS.map((g) => [g.title, anyOpen])))
                  }}
                  className="text-xs text-muted-foreground hover:text-foreground"
                >
                  {RULE_GROUPS.some((g) => !collapsed[g.title]) ? 'Collapse all' : 'Expand all'}
                </button>
              </div>
              <div className="space-y-3">
                {RULE_GROUPS.map((group) => {
                  const open = !collapsed[group.title]
                  return (
                    <div key={group.title} className="border border-border rounded-lg overflow-hidden">
                      <button
                        type="button"
                        onClick={() => toggleGroup(group.title)}
                        aria-expanded={open}
                        className="w-full flex items-center justify-between px-4 py-3 bg-muted/40 hover:bg-muted/60 text-left transition-colors"
                      >
                        <span className="text-sm font-semibold text-foreground">{group.title}</span>
                        <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${open ? '' : '-rotate-90'}`} />
                      </button>
                      {open && (
                        <div className="p-4">
                          {group.blurb && <p className="text-xs text-muted-foreground mb-3">{group.blurb}</p>}
                          <div className="grid grid-cols-2 gap-4">
                            {group.fields.map((f) => renderField(f))}
                          </div>
                        </div>
                      )}
                    </div>
                  )
                })}
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

// RuleHelp renders a small ? icon that reveals a popover with the rule's
// explanation on hover/focus. CSS-only via Tailwind's `group-hover` — no
// portal/positioning library needed since the panel is short and the form
// has ample horizontal space.
function RuleHelp({ text }: { text: string }) {
  return (
    <span className="group relative inline-flex">
      <HelpCircle
        className="h-3.5 w-3.5 text-muted-foreground hover:text-foreground transition-colors cursor-help"
        tabIndex={0}
        role="button"
        aria-label="Rule details"
      />
      <span
        role="tooltip"
        className="invisible opacity-0 group-hover:visible group-hover:opacity-100 group-focus-within:visible group-focus-within:opacity-100 absolute left-5 top-1/2 -translate-y-1/2 z-20 w-64 rounded-md border bg-card text-foreground px-3 py-2 text-xs leading-relaxed shadow-lg transition-opacity"
      >
        {text}
      </span>
    </span>
  )
}
