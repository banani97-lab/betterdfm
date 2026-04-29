import { describe, it, expect } from 'vitest'
import { buildPaintList } from './boardPainter'
import type { Bounds } from './boardPainter'
import type { BoardData, Violation } from './api'
import type { FillRect, DrawLine, DrawViolationMarker, DrawPolygon } from './paint'

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeBounds(): Bounds {
  return { minX: 0, minY: 0, maxX: 60, maxY: 40, scale: 10, offX: 50, offY: 50 }
}

function makeBoard(overrides: Partial<BoardData> = {}): BoardData {
  return {
    layers: [
      { name: 'top_copper', type: 'COPPER' },
      { name: 'bottom_copper', type: 'COPPER' },
    ],
    traces: [
      { layer: 'top_copper', widthMM: 0.2, startX: 5, startY: 5, endX: 20, endY: 5, netName: '' },
    ],
    pads: [],
    vias: [],
    drills: [],
    outline: [
      { x: 0, y: 0 }, { x: 60, y: 0 }, { x: 60, y: 40 }, { x: 0, y: 40 },
    ],
    boardThicknessMM: 1.6,
    ...overrides,
  }
}

function makeViolation(overrides: Partial<Violation> = {}): Violation {
  return {
    id: 'v1', jobId: 'j1', ruleId: 'clearance',
    severity: 'ERROR', layer: 'top_copper',
    x: 10, y: 10, x2: 0, y2: 0,
    message: 'msg', suggestion: 'sug',
    count: 1, measuredMM: 0.1, limitMM: 0.15,
    unit: 'mm', netName: '', refDes: '', ignored: false,
    ...overrides,
  }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('buildPaintList', () => {

  it('returns empty list for null boardData', () => {
    const result = buildPaintList(null, makeBounds(), [], new Set(), undefined, false)
    expect(result).toHaveLength(0)
  })

  it('returns empty list for null bounds', () => {
    const result = buildPaintList(makeBoard(), null, [], new Set(), undefined, false)
    expect(result).toHaveLength(0)
  })

  it('emits background fillRect as first instruction', () => {
    const result = buildPaintList(makeBoard(), makeBounds(), [], new Set(), undefined, false)
    const first = result[0] as FillRect
    expect(first.type).toBe('fillRect')
    expect(first.x).toBe(0)
    expect(first.y).toBe(0)
    expect(first.w).toBe(1200)
    expect(first.h).toBe(800)
  })

  it('emits drawLine instructions for visible copper traces', () => {
    const result = buildPaintList(makeBoard(), makeBounds(), [], new Set(), undefined, false)
    const lines = result.filter(i => i.type === 'drawLine') as DrawLine[]
    expect(lines.length).toBeGreaterThan(0)
    // The trace from (5,5)→(20,5) with scale=10, offX=50, offY=50:
    // tx(5)=50+50=100, ty(5)=50+50=100, tx(20)=200+50=250
    const traceLine = lines.find(l => Math.abs(l.x1 - 100) < 1 && Math.abs(l.y1 - 100) < 1)
    expect(traceLine).toBeDefined()
    expect(traceLine!.x2).toBeCloseTo(250)
  })

  it('skips traces on hidden layers', () => {
    const board = makeBoard()
    const hiddenLayers = new Set(['top_copper'])
    const result = buildPaintList(board, makeBounds(), [], hiddenLayers, undefined, false)
    const lines = result.filter(i => i.type === 'drawLine') as DrawLine[]
    // No copper traces visible (top_copper hidden, bottom_copper has no traces)
    expect(lines.length).toBe(0)
  })

  it('emits violationMarker per violation with correct severity', () => {
    const violations = [
      makeViolation({ id: 'v1', severity: 'ERROR', x: 5, y: 5 }),
      makeViolation({ id: 'v2', severity: 'WARNING', x: 10, y: 10 }),
      makeViolation({ id: 'v3', severity: 'INFO', x: 20, y: 20 }),
    ]
    const result = buildPaintList(makeBoard(), makeBounds(), violations, new Set(), undefined, false)
    const markers = result.filter(i => i.type === 'drawViolationMarker') as DrawViolationMarker[]
    expect(markers).toHaveLength(3)
    expect(markers[0].severity).toBe('ERROR')
    expect(markers[1].severity).toBe('WARNING')
    expect(markers[2].severity).toBe('INFO')
  })

  it('sets selected=true only for selectedViolationId', () => {
    const violations = [
      makeViolation({ id: 'v1', severity: 'ERROR', x: 5, y: 5 }),
      makeViolation({ id: 'v2', severity: 'WARNING', x: 10, y: 10 }),
    ]
    const result = buildPaintList(makeBoard(), makeBounds(), violations, new Set(), 'v1', false)
    const markers = result.filter(i => i.type === 'drawViolationMarker') as DrawViolationMarker[]
    expect(markers[0].selected).toBe(true)
    expect(markers[0].pulseFraction).toBeDefined()
    expect(markers[1].selected).toBe(false)
    expect(markers[1].pulseFraction).toBeUndefined()
  })

  it('emits x2/y2 on marker for two-object violations', () => {
    const v = makeViolation({ id: 'v1', severity: 'ERROR', x: 5, y: 5, x2: 15, y2: 5 })
    const result = buildPaintList(makeBoard(), makeBounds(), [v], new Set(), undefined, false)
    const markers = result.filter(i => i.type === 'drawViolationMarker') as DrawViolationMarker[]
    expect(markers).toHaveLength(1)
    expect(markers[0].x2).toBeDefined()
    expect(markers[0].y2).toBeDefined()
  })

  it('does not emit x2/y2 when both are zero', () => {
    const v = makeViolation({ id: 'v1', severity: 'ERROR', x: 5, y: 5, x2: 0, y2: 0 })
    const result = buildPaintList(makeBoard(), makeBounds(), [v], new Set(), undefined, false)
    const markers = result.filter(i => i.type === 'drawViolationMarker') as DrawViolationMarker[]
    expect(markers[0].x2).toBeUndefined()
    expect(markers[0].y2).toBeUndefined()
  })

  it('emits no violation markers when violations list is empty', () => {
    const result = buildPaintList(makeBoard(), makeBounds(), [], new Set(), undefined, false)
    const markers = result.filter(i => i.type === 'drawViolationMarker')
    expect(markers).toHaveLength(0)
  })

  it('emits grid lines when gridEnabled is true', () => {
    const result = buildPaintList(makeBoard(), makeBounds(), [], new Set(), undefined, true, 1)
    // Grid lines are drawLine instructions after the board geometry
    const lines = result.filter(i => i.type === 'drawLine') as DrawLine[]
    // At least some grid lines (in addition to the trace line)
    expect(lines.length).toBeGreaterThan(1)
  })

  it('emits no extra grid lines when gridEnabled is false', () => {
    const withGrid    = buildPaintList(makeBoard(), makeBounds(), [], new Set(), undefined, true)
    const withoutGrid = buildPaintList(makeBoard(), makeBounds(), [], new Set(), undefined, false)
    const gridLines   = withGrid.filter(i => i.type === 'drawLine').length
    const noGridLines = withoutGrid.filter(i => i.type === 'drawLine').length
    expect(gridLines).toBeGreaterThan(noGridLines)
  })

  it('emits setComposite multiply + source-over when mask layer is visible', () => {
    const board = makeBoard({
      layers: [
        { name: 'top_copper', type: 'COPPER' },
        { name: 'gts', type: 'SOLDER_MASK' },
      ],
    })
    const result = buildPaintList(board, makeBounds(), [], new Set(), undefined, false)
    const composites = result.filter(i => i.type === 'setComposite')
    expect(composites.length).toBeGreaterThanOrEqual(2)
  })

  it('skips soldermask when mask layer is hidden', () => {
    const board = makeBoard({
      layers: [
        { name: 'top_copper', type: 'COPPER' },
        { name: 'gts', type: 'SOLDER_MASK' },
      ],
    })
    const hidden = new Set(['gts'])
    const result = buildPaintList(board, makeBounds(), [], hidden, undefined, false)
    const composites = result.filter(i => i.type === 'setComposite')
    expect(composites).toHaveLength(0)
  })

  it('passes outlineHoles to FR4 fill polygon as cutouts', () => {
    const board = makeBoard({
      outlineHoles: [
        [{ x: 10, y: 10 }, { x: 20, y: 10 }, { x: 20, y: 20 }, { x: 10, y: 20 }],
      ],
    })
    const result = buildPaintList(board, makeBounds(), [], new Set(), undefined, false)
    const fr4 = result.find(i => i.type === 'drawPolygon') as DrawPolygon | undefined
    expect(fr4).toBeDefined()
    expect(fr4!.holes).toHaveLength(1)
    expect(fr4!.holes![0]).toHaveLength(4)
  })

  it('omits holes field when outlineHoles is empty', () => {
    const board = makeBoard()
    const result = buildPaintList(board, makeBounds(), [], new Set(), undefined, false)
    const fr4 = result.find(i => i.type === 'drawPolygon') as DrawPolygon | undefined
    expect(fr4).toBeDefined()
    expect(fr4!.holes).toBeUndefined()
  })
})
