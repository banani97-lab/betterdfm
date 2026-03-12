import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import { BoardViewer } from './BoardViewer'
import type { BoardData, Violation } from '@/lib/api'

// Mock canvas getContext
const mockCtx = {
  clearRect: vi.fn(),
  fillRect: vi.fn(),
  strokeRect: vi.fn(),
  beginPath: vi.fn(),
  moveTo: vi.fn(),
  lineTo: vi.fn(),
  arc: vi.fn(),
  stroke: vi.fn(),
  fill: vi.fn(),
  closePath: vi.fn(),
  setLineDash: vi.fn(),
  save: vi.fn(),
  restore: vi.fn(),
  scale: vi.fn(),
  translate: vi.fn(),
  measureText: vi.fn(() => ({ width: 50 })),
  fillText: vi.fn(),
  createLinearGradient: vi.fn(() => ({ addColorStop: vi.fn() })),
  canvas: { width: 800, height: 600 },
  globalAlpha: 1,
  globalCompositeOperation: 'source-over',
  lineWidth: 1,
  strokeStyle: '',
  fillStyle: '',
  lineCap: 'butt' as CanvasLineCap,
  font: '',
  textAlign: 'left' as CanvasTextAlign,
}

beforeEach(() => {
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(mockCtx as unknown as CanvasRenderingContext2D)
})

function syntheticBoardData(): BoardData {
  return {
    layers: [{ name: 'top_copper', type: 'COPPER' }],
    traces: [
      { layer: 'top_copper', widthMM: 0.2, startX: 5, startY: 5, endX: 55, endY: 5, netName: '' },
    ],
    pads: [],
    vias: [],
    drills: [],
    outline: [
      { x: 0, y: 0 }, { x: 60, y: 0 }, { x: 60, y: 40 }, { x: 0, y: 40 },
    ],
    boardThicknessMM: 1.6,
  }
}

function makeViolation(overrides: Partial<Violation> = {}): Violation {
  return {
    id: 'v1', jobId: 'j1', ruleId: 'trace-width', severity: 'ERROR',
    layer: 'top_copper', x: 10, y: 5, message: 'Too narrow', suggestion: 'Widen',
    count: 1, measuredMM: 0.08, limitMM: 0.1, unit: 'mm',
    netName: '', refDes: '', x2: 0, y2: 0, ignored: false,
    ...overrides,
  }
}

const defaultProps = {
  violations: [],
  selectedViolationId: undefined,
  onViolationClick: vi.fn(),
  hiddenLayers: new Set<string>(),
  onToggleLayer: vi.fn(),
}

describe('BoardViewer', () => {
  it('renders without crash with null boardData', () => {
    expect(() =>
      render(<BoardViewer boardData={null} {...defaultProps} />)
    ).not.toThrow()
  })

  it('renders without crash with synthetic BoardData', () => {
    expect(() =>
      render(
        <BoardViewer
          boardData={syntheticBoardData()}
          {...defaultProps}
          violations={[makeViolation()]}
        />
      )
    ).not.toThrow()
  })

  it('renders canvas element', () => {
    const { container } = render(
      <BoardViewer
        boardData={syntheticBoardData()}
        {...defaultProps}
        violations={[makeViolation()]}
      />
    )
    expect(container.querySelector('canvas')).toBeInTheDocument()
  })

  it('accepts onViolationClick prop without throwing', () => {
    const onViolationClick = vi.fn()
    expect(() =>
      render(
        <BoardViewer
          boardData={syntheticBoardData()}
          violations={[makeViolation()]}
          onViolationClick={onViolationClick}
          hiddenLayers={new Set()}
          onToggleLayer={vi.fn()}
        />
      )
    ).not.toThrow()
  })
})
