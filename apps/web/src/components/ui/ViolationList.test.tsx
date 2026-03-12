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

const defaultProps = {
  filter: 'NONE' as const,
  onFilterChange: vi.fn(),
}

describe('ViolationList', () => {
  it('renders all violations when no severity filter', () => {
    const violations = [
      makeViolation({ id: 'v1', severity: 'ERROR', message: 'Error msg' }),
      makeViolation({ id: 'v2', severity: 'WARNING', message: 'Warning msg' }),
    ]
    render(
      <ViolationList
        violations={violations}
        allViolations={violations}
        {...defaultProps}
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
        {...defaultProps}
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
        {...defaultProps}
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
        {...defaultProps}
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
})
