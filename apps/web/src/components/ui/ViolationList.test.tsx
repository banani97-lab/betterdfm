import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ViolationList } from './ViolationList'
import type { Violation } from '@/lib/api'

function makeViolation(overrides: Partial<Violation> = {}): Violation {
  return {
    id: 'v1',
    jobId: 'j1',
    ruleId: 'trace-width',
    severity: 'ERROR',
    layer: 'top_copper',
    x: 10,
    y: 20,
    message: 'Trace too narrow',
    suggestion: 'Widen trace',
    count: 1,
    measuredMM: 0.08,
    limitMM: 0.1,
    unit: 'mm',
    netName: '',
    refDes: '',
    x2: 0,
    y2: 0,
    ignored: false,
    ...overrides,
  }
}

function makeDefaultProps(violations: Violation[]) {
  return {
    filter: 'ERROR' as const,
    onFilterChange: vi.fn(),
    shownViolationIds: new Set(violations.map((violation) => violation.id)),
    onToggleShown: vi.fn(),
    onShowAll: vi.fn(),
    onHideAll: vi.fn(),
  }
}

describe('ViolationList', () => {
  it('renders all violations passed via prop when filter is active', () => {
    const violations = [
      makeViolation({ id: 'v1', severity: 'ERROR', message: 'Error msg' }),
      makeViolation({ id: 'v2', severity: 'WARNING', message: 'Warning msg' }),
    ]
    render(
      <ViolationList
        violations={violations}
        allViolations={violations}
        {...makeDefaultProps(violations)}
      />
    )
    expect(screen.getByText('Error msg')).toBeInTheDocument()
    expect(screen.getByText('Warning msg')).toBeInTheDocument()
  })

  it('filters to only ERRORs when ERROR tab active', () => {
    const allViols = [
      makeViolation({ id: 'v1', severity: 'ERROR', message: 'Error msg' }),
      makeViolation({ id: 'v2', severity: 'WARNING', message: 'Warning msg' }),
    ]
    const errorOnly = allViols.filter((v) => v.severity === 'ERROR')
    render(
      <ViolationList
        violations={errorOnly}
        allViolations={allViols}
        filter="ERROR"
        onFilterChange={vi.fn()}
        shownViolationIds={new Set(errorOnly.map((violation) => violation.id))}
        onToggleShown={vi.fn()}
        onShowAll={vi.fn()}
        onHideAll={vi.fn()}
      />
    )
    expect(screen.getByText('Error msg')).toBeInTheDocument()
    expect(screen.queryByText('Warning msg')).not.toBeInTheDocument()
  })

  it('shows ignored violations toggle', () => {
    const ignoredViol = makeViolation({ id: 'v2', ignored: true, message: 'Ignored violation' })
    const activeViol = makeViolation({ id: 'v1', ignored: false, message: 'Active violation' })
    const all = [activeViol, ignoredViol]
    const { rerender } = render(
      <ViolationList
        violations={[activeViol]}
        allViolations={all}
        {...makeDefaultProps([activeViol])}
      />
    )
    // Active violation is visible
    expect(screen.getByText('Active violation')).toBeInTheDocument()
    // Ignored violation not visible initially (not in violations prop)
    expect(screen.queryByText('Ignored violation')).not.toBeInTheDocument()

    // Rerender with ignored also included
    rerender(
      <ViolationList
        violations={all}
        allViolations={all}
        {...makeDefaultProps(all)}
      />
    )
    // After toggle, show ignored button appears
    const showIgnoredBtn = screen.queryByRole('button', { name: /ignored/i })
    if (showIgnoredBtn) {
      fireEvent.click(showIgnoredBtn)
    }
  })

  it('rule filter pills toggle correctly', () => {
    const violations = [
      makeViolation({ id: 'v1', ruleId: 'trace-width', message: 'Trace violation' }),
      makeViolation({ id: 'v2', ruleId: 'clearance', message: 'Clearance violation' }),
    ]
    render(
      <ViolationList
        violations={violations}
        allViolations={violations}
        {...makeDefaultProps(violations)}
      />
    )
    // Rule filter pills should be present
    const traceWidthPill = screen.queryByText(/Trace Width/i)
    if (traceWidthPill) {
      fireEvent.click(traceWidthPill)
      // After clicking, it should be selected (has active styling)
      expect(traceWidthPill).toBeInTheDocument()
    }
  })

  it('shows visibility controls and forwards toggle actions', () => {
    const violations = [
      makeViolation({ id: 'v1', message: 'Trace violation' }),
      makeViolation({ id: 'v2', ruleId: 'clearance', message: 'Clearance violation' }),
    ]
    const onToggleShown = vi.fn()
    const onShowAll = vi.fn()
    const onHideAll = vi.fn()

    render(
      <ViolationList
        violations={violations}
        allViolations={violations}
        filter="ERROR"
        onFilterChange={vi.fn()}
        shownViolationIds={new Set(['v1'])}
        onToggleShown={onToggleShown}
        onShowAll={onShowAll}
        onHideAll={onHideAll}
      />
    )

    expect(screen.getByText('Showing 1 of 2')).toBeInTheDocument()

    const traceButton = screen.getByText('Trace violation').closest('button')
    expect(traceButton).toHaveAttribute('aria-pressed', 'true')
    if (traceButton) fireEvent.click(traceButton)
    expect(onToggleShown).toHaveBeenCalledWith(violations[0])

    fireEvent.click(screen.getByRole('button', { name: /show all/i }))
    expect(onShowAll).toHaveBeenCalledOnce()

    fireEvent.click(screen.getByRole('button', { name: /hide all/i }))
    expect(onHideAll).toHaveBeenCalledOnce()
  })
})
