'use client'

import { useEffect, useRef, useState, useMemo } from 'react'
import { AlertCircle, AlertTriangle, Info } from 'lucide-react'
import { Badge } from './badge'
import { cn } from '@/lib/utils'
import type { Violation } from '@/lib/api'

export type SeverityFilter = 'ERROR' | 'WARNING' | 'INFO' | 'NONE'

const RULE_LABELS: Record<string, string> = {
  'annular-ring':    'Annular Ring',
  'aspect-ratio':    'Aspect Ratio',
  'clearance':       'Clearance',
  'drill-size':      'Drill Size',
  'edge-clearance':  'Edge Clearance',
  'solder-mask-dam':       'Solder Mask',
  'trace-width':           'Trace Width',
  'pad-size-for-package':  'Pad Size',
  'tombstoning-risk':      'Tombstoning Risk',
  'package-capability':    'Package Capability',
}

interface ViolationListProps {
  violations: Violation[]
  allViolations: Violation[]  // used for tab counts — not affected by severity filter
  selectedId?: string
  onSelect?: (v: Violation) => void
  filter: SeverityFilter
  onFilterChange: (f: SeverityFilter) => void
  onIgnore?: (v: Violation, ignored: boolean) => void
}

const severityIcon = {
  ERROR: <AlertCircle className="h-4 w-4 text-red-500" />,
  WARNING: <AlertTriangle className="h-4 w-4 text-yellow-500" />,
  INFO: <Info className="h-4 w-4 text-blue-500" />,
}

const severityBadgeVariant: Record<string, 'destructive' | 'warning' | 'info'> = {
  ERROR: 'destructive',
  WARNING: 'warning',
  INFO: 'info',
}

export function ViolationList({ violations, allViolations, selectedId, onSelect, filter, onFilterChange, onIgnore }: ViolationListProps) {
  const listRef = useRef<HTMLDivElement>(null)
  const [showIgnored, setShowIgnored] = useState(false)
  const [ruleFilter, setRuleFilter] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!selectedId || !listRef.current) return
    const el = listRef.current.querySelector(`[data-violation-id="${selectedId}"]`)
    el?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [selectedId])

  // Counts always reflect the full layer-filtered set, not the active severity tab
  const counts = {
    ERROR: allViolations.filter((v) => v.severity === 'ERROR' && !v.ignored).length,
    WARNING: allViolations.filter((v) => v.severity === 'WARNING' && !v.ignored).length,
    INFO: allViolations.filter((v) => v.severity === 'INFO' && !v.ignored).length,
  }

  const ignoredCount = allViolations.filter((v) => v.ignored).length

  // Rule IDs present in the current severity view (allViolations before severity split)
  const availableRules = useMemo(() => {
    const ids = new Set<string>()
    for (const v of allViolations) ids.add(v.ruleId)
    return Array.from(ids).sort()
  }, [allViolations])

  // Count per rule among the currently severity-filtered violations (non-ignored)
  const ruleCounts = useMemo(() => {
    const m = new Map<string, number>()
    for (const v of violations) {
      if (!v.ignored) m.set(v.ruleId, (m.get(v.ruleId) ?? 0) + 1)
    }
    return m
  }, [violations])

  const toggleRule = (ruleId: string) => {
    setRuleFilter((prev) => {
      const next = new Set(prev)
      if (next.has(ruleId)) next.delete(ruleId); else next.add(ruleId)
      return next
    })
  }

  // Apply rule filter then ignored filter
  const displayViolations = violations.filter((v) => {
    if (!showIgnored && v.ignored) return false
    if (ruleFilter.size > 0 && !ruleFilter.has(v.ruleId)) return false
    return true
  })

  return (
    <div className="flex flex-col h-full">
      {/* Severity filter tabs */}
      <div className="flex gap-1 p-2 border-b bg-muted/40 flex-shrink-0 flex-wrap">
        {(['ERROR', 'WARNING', 'INFO', 'NONE'] as SeverityFilter[]).map((f) => (
          <button
            key={f}
            onClick={() => onFilterChange(f)}
            className={cn(
              'px-3 py-1 rounded text-xs font-medium transition-colors',
              filter === f
                ? 'bg-card shadow-sm text-foreground border'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {f}
            {(f === 'ERROR' || f === 'WARNING' || f === 'INFO') && (
              <span className="ml-1 text-muted-foreground">({counts[f]})</span>
            )}
          </button>
        ))}
        {ignoredCount > 0 && (
          <button
            onClick={() => setShowIgnored((s) => !s)}
            className="ml-auto px-2 py-1 rounded text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            {showIgnored ? `Hide ignored ▴` : `Show ${ignoredCount} ignored ▾`}
          </button>
        )}
      </div>

      {/* Rule type filter pills */}
      {availableRules.length > 1 && filter !== 'NONE' && (
        <div className="flex gap-1 px-2 py-1.5 border-b bg-muted/20 flex-shrink-0 flex-wrap">
          {availableRules.map((ruleId) => {
            const active = ruleFilter.size === 0 || ruleFilter.has(ruleId)
            const count = ruleCounts.get(ruleId) ?? 0
            return (
              <button
                key={ruleId}
                onClick={() => toggleRule(ruleId)}
                title={ruleFilter.size === 0 ? `Filter to ${ruleId} only` : ruleFilter.has(ruleId) ? `Remove ${ruleId} filter` : `Add ${ruleId} to filter`}
                className={cn(
                  'px-2 py-0.5 rounded-full text-xs transition-colors border',
                  active
                    ? 'bg-card border-border text-foreground'
                    : 'bg-transparent border-transparent text-muted-foreground/50 hover:text-muted-foreground'
                )}
              >
                {RULE_LABELS[ruleId] ?? ruleId}
                <span className={cn('ml-1', active ? 'text-muted-foreground' : 'text-muted-foreground/40')}>
                  ({count})
                </span>
              </button>
            )
          })}
          {ruleFilter.size > 0 && (
            <button
              onClick={() => setRuleFilter(new Set())}
              className="ml-auto px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              clear
            </button>
          )}
        </div>
      )}

      {/* List */}
      <div ref={listRef} className="flex-1 overflow-y-auto divide-y">
        {filter === 'NONE' ? (
          <div className="flex flex-col items-center justify-center h-40 text-muted-foreground">
            <Info className="h-8 w-8 mb-2" />
            <p className="text-sm">Violations hidden</p>
          </div>
        ) : displayViolations.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-40 text-muted-foreground">
            <Info className="h-8 w-8 mb-2" />
            <p className="text-sm">No violations found</p>
          </div>
        ) : (
          displayViolations.map((v) => (
            <button
              key={v.id}
              data-violation-id={v.id}
              onClick={() => onSelect?.(v)}
              className={cn(
                'group w-full text-left p-3 hover:bg-muted/40 transition-colors',
                selectedId === v.id && 'bg-primary/15 border-l-2 border-primary',
                v.ignored && 'opacity-50'
              )}
            >
              <div className="flex items-start gap-2">
                <div className="mt-0.5 flex-shrink-0">
                  {severityIcon[v.severity as keyof typeof severityIcon]}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <Badge variant={severityBadgeVariant[v.severity] ?? 'default'} className="text-xs">
                      {v.severity}
                    </Badge>
                    <span className="text-xs font-mono text-muted-foreground">{v.ruleId}</span>
                    {v.count > 1 && (
                      <span className="text-xs font-mono text-muted-foreground" title={`${v.count} affected pairs in this area`}>×{v.count}</span>
                    )}
                    {v.ignored && (
                      <span className="text-xs text-muted-foreground italic">ignored</span>
                    )}
                  </div>
                  <p className={cn('text-sm text-foreground leading-snug', v.ignored && 'line-through')}>{v.message}</p>
                  {v.suggestion && !v.ignored && (
                    <p className="text-xs text-muted-foreground mt-1 leading-snug">{v.suggestion}</p>
                  )}
                  {v.measuredMM !== 0 && !v.ignored && (
                    <div className="mt-1 grid grid-cols-2 gap-x-3 text-xs font-mono text-muted-foreground">
                      <span>
                        Measured&nbsp;
                        <span className="text-foreground">
                          {v.unit === 'ratio'
                            ? v.measuredMM.toFixed(2)
                            : `${v.measuredMM.toFixed(3)} mm`}
                        </span>
                      </span>
                      <span>
                        Limit&nbsp;
                        <span className="text-foreground">
                          {v.unit === 'ratio'
                            ? v.limitMM.toFixed(2)
                            : `${v.limitMM.toFixed(3)} mm`}
                        </span>
                      </span>
                      {v.netName && (
                        <span>Net&nbsp;<span className="text-foreground">{v.netName}</span></span>
                      )}
                      {v.refDes && (
                        <span>Ref&nbsp;<span className="text-foreground">{v.refDes}</span></span>
                      )}
                    </div>
                  )}
                  <p className="text-xs text-muted-foreground mt-1 font-mono">
                    layer: {v.layer || '—'} | x: {v.x.toFixed(2)} y: {v.y.toFixed(2)}
                  </p>
                </div>
                {onIgnore && (
                  <button
                    onClick={(e) => { e.stopPropagation(); onIgnore(v, !v.ignored) }}
                    title={v.ignored ? 'Restore violation' : 'Ignore / waive this violation'}
                    className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity text-xs text-muted-foreground hover:text-foreground px-1 py-0.5 rounded hover:bg-muted"
                  >
                    {v.ignored ? 'restore' : 'ignore'}
                  </button>
                )}
              </div>
            </button>
          ))
        )}
      </div>
    </div>
  )
}
