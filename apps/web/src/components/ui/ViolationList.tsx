'use client'

import { useEffect, useRef, useState } from 'react'
import { AlertCircle, AlertTriangle, Info } from 'lucide-react'
import { Badge } from './badge'
import { cn } from '@/lib/utils'
import type { Violation } from '@/lib/api'

export type SeverityFilter = 'ERROR' | 'WARNING' | 'INFO' | 'NONE'

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

  // When showIgnored is off, hide ignored violations from the list
  const displayViolations = violations.filter((v) => showIgnored || !v.ignored)

  return (
    <div className="flex flex-col h-full">
      {/* Filter tabs */}
      <div className="flex gap-1 p-2 border-b bg-gray-50 flex-shrink-0 flex-wrap">
        {(['ERROR', 'WARNING', 'INFO', 'NONE'] as SeverityFilter[]).map((f) => (
          <button
            key={f}
            onClick={() => onFilterChange(f)}
            className={cn(
              'px-3 py-1 rounded text-xs font-medium transition-colors',
              filter === f
                ? 'bg-white shadow-sm text-gray-900 border'
                : 'text-gray-600 hover:text-gray-900'
            )}
          >
            {f}
            {(f === 'ERROR' || f === 'WARNING' || f === 'INFO') && (
              <span className="ml-1 text-gray-400">({counts[f]})</span>
            )}
          </button>
        ))}
        {ignoredCount > 0 && (
          <button
            onClick={() => setShowIgnored((s) => !s)}
            className="ml-auto px-2 py-1 rounded text-xs text-gray-400 hover:text-gray-600 transition-colors"
          >
            {showIgnored ? `Hide ignored ▴` : `Show ${ignoredCount} ignored ▾`}
          </button>
        )}
      </div>

      {/* List */}
      <div ref={listRef} className="flex-1 overflow-y-auto divide-y">
        {filter === 'NONE' ? (
          <div className="flex flex-col items-center justify-center h-40 text-gray-400">
            <Info className="h-8 w-8 mb-2" />
            <p className="text-sm">Violations hidden</p>
          </div>
        ) : displayViolations.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-40 text-gray-400">
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
                'group w-full text-left p-3 hover:bg-gray-50 transition-colors',
                selectedId === v.id && 'bg-blue-50 border-l-2 border-blue-500',
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
                    <span className="text-xs font-mono text-gray-500">{v.ruleId}</span>
                    {v.ignored && (
                      <span className="text-xs text-gray-400 italic">ignored</span>
                    )}
                  </div>
                  <p className={cn('text-sm text-gray-900 leading-snug', v.ignored && 'line-through')}>{v.message}</p>
                  {v.suggestion && !v.ignored && (
                    <p className="text-xs text-gray-500 mt-1 leading-snug">{v.suggestion}</p>
                  )}
                  {v.measuredMM !== 0 && !v.ignored && (
                    <div className="mt-1 grid grid-cols-2 gap-x-3 text-xs font-mono text-gray-500">
                      <span>
                        Measured&nbsp;
                        <span className="text-gray-700">
                          {v.unit === 'ratio'
                            ? v.measuredMM.toFixed(2)
                            : `${v.measuredMM.toFixed(3)} mm`}
                        </span>
                      </span>
                      <span>
                        Limit&nbsp;
                        <span className="text-gray-700">
                          {v.unit === 'ratio'
                            ? v.limitMM.toFixed(2)
                            : `${v.limitMM.toFixed(3)} mm`}
                        </span>
                      </span>
                      {v.netName && (
                        <span>Net&nbsp;<span className="text-gray-700">{v.netName}</span></span>
                      )}
                      {v.refDes && (
                        <span>Ref&nbsp;<span className="text-gray-700">{v.refDes}</span></span>
                      )}
                    </div>
                  )}
                  <p className="text-xs text-gray-400 mt-1 font-mono">
                    layer: {v.layer || '—'} | x: {v.x.toFixed(2)} y: {v.y.toFixed(2)}
                  </p>
                </div>
                {onIgnore && (
                  <button
                    onClick={(e) => { e.stopPropagation(); onIgnore(v, !v.ignored) }}
                    title={v.ignored ? 'Restore violation' : 'Ignore / waive this violation'}
                    className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity text-xs text-gray-400 hover:text-gray-700 px-1 py-0.5 rounded hover:bg-gray-100"
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
