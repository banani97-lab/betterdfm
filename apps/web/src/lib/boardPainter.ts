// Pure function: board data → paint instructions.
// No canvas API calls, no React, no side effects.

import type { BoardData, Violation } from './api'
import type { PaintInstruction, DrawViolationMarker } from './paint'

export interface Bounds {
  minX: number; minY: number; maxX: number; maxY: number
  scale: number; offX: number; offY: number
}

// ── Layer classification (mirrors BoardViewer.tsx) ────────────────────────────

function isSilkLayer(n: string): boolean {
  return n.includes('silk') || n.includes('legend') || n.includes('gto') ||
         n.includes('gbo') || n.includes('overlay')
}
function isMaskLayer(n: string): boolean {
  return n.includes('mask') || n.includes('covertop') || n.includes('coverbottom') ||
         n.includes('cover') || n.includes('gts') || n.includes('gbs')
}
function isCopperLayer(n: string): boolean {
  return !isSilkLayer(n) && !isMaskLayer(n) &&
         !n.includes('outline') && !n.includes('gko') &&
         !n.includes('edge') && !n.includes('board') && n !== 'rout'
}
function getSilkColor(n: string): string {
  return (n.includes('bot') || n.includes('back') || n.includes('gbo') || n.includes('b.'))
    ? '#d0c060' : '#f0e8d8'
}

const SEV_COLOR: Record<string, string> = {
  ERROR:   '#ff3333',
  WARNING: '#ffaa00',
  INFO:    '#44aaff',
}

const SAFE = 1e5
function ok(n: number): boolean { return isFinite(n) && n > -SAFE && n < SAFE }

// ── Main export ───────────────────────────────────────────────────────────────

export function buildPaintList(
  boardData: BoardData | null | undefined,
  bounds: Bounds | null,
  violations: Violation[],
  hiddenLayers: Set<string>,
  selectedViolationId: string | undefined,
  gridEnabled: boolean,
  zoom = 1,
  now = Date.now(),
): PaintInstruction[] {
  if (!boardData || !bounds) return []

  const { minX, minY, maxX, maxY, scale: s, offX, offY } = bounds
  const tx = (x: number) => (x - minX) * s + offX
  const ty = (y: number) => (y - minY) * s + offY

  const out: PaintInstruction[] = []

  // 1. Viewbox background
  out.push({ type: 'fillRect', x: 0, y: 0, w: 1200, h: 800, fillStyle: '#060e06' })

  // 2. FR4 board fill
  const outlinePts = (boardData.outline ?? []).filter(p => ok(p.x) && ok(p.y))
  if (outlinePts.length > 1) {
    const outlineHoles = (boardData.outlineHoles ?? [])
      .map(ring => ring.filter(p => ok(p.x) && ok(p.y)).map(p => ({ x: tx(p.x), y: ty(p.y) })))
      .filter(ring => ring.length > 1)
    out.push({
      type: 'drawPolygon',
      points: outlinePts.map(p => ({ x: tx(p.x), y: ty(p.y) })),
      holes: outlineHoles.length > 0 ? outlineHoles : undefined,
      fillStyle: '#1a2e1a',
      close: true,
    })
  } else {
    out.push({
      type: 'fillRect',
      x: tx(minX), y: ty(minY),
      w: (maxX - minX) * s, h: (maxY - minY) * s,
      fillStyle: '#1a2e1a',
    })
  }

  // Group geometry by layer
  const tracesByLayer: Record<string, typeof boardData.traces> = {}
  for (const t of boardData.traces ?? []) (tracesByLayer[t.layer] ??= []).push(t)
  const padsByLayer: Record<string, typeof boardData.pads> = {}
  for (const p of boardData.pads ?? []) (padsByLayer[p.layer] ??= []).push(p)

  // 3. Copper traces
  for (const [layer, traces] of Object.entries(tracesByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (!isCopperLayer(layer.toLowerCase())) continue
    for (const t of traces) {
      const x1 = tx(t.startX), y1 = ty(t.startY)
      const x2 = tx(t.endX),   y2 = ty(t.endY)
      if (!ok(x1) || !ok(y1) || !ok(x2) || !ok(y2)) continue
      const lw = Math.max(0.5, isFinite(t.widthMM) ? t.widthMM * s : 0.5)
      out.push({ type: 'drawLine', x1, y1, x2, y2, strokeStyle: '#b47a22', lineWidth: lw })
    }
  }

  // 4. Copper pads
  for (const [layer, pads] of Object.entries(padsByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (!isCopperLayer(layer.toLowerCase())) continue
    for (const p of pads) {
      const cx = tx(p.x), cy = ty(p.y)
      if (!ok(cx) || !ok(cy)) continue
      const w = Math.max(1, p.widthMM * s)
      const h = Math.max(1, p.heightMM * s)
      if (p.shape === 'RECT') {
        out.push({ type: 'fillRect', x: cx - w / 2, y: cy - h / 2, w, h, fillStyle: '#e8c050' })
      } else if (p.shape === 'OVAL' && Math.abs(w - h) > 1) {
        out.push({ type: 'drawEllipse', cx, cy, rx: Math.max(1, w / 2), ry: Math.max(1, h / 2), fillStyle: '#e8c050' })
      } else if (p.shape === 'DONUT' && p.holeMM && p.holeMM > 0) {
        // Annular catch-pad: outer copper ring with the FR4-coloured hole punched
        // through. Draw as filled outer + filled inner overlay so the inner
        // shows the FR4 layer beneath, not the page background.
        const outerR = Math.max(1, Math.max(w, h) / 2)
        const innerR = Math.max(0.5, Math.min((p.holeMM / 2) * s, outerR * 0.95))
        out.push({ type: 'drawCircle', cx, cy, r: outerR, fillStyle: '#e8c050' })
        out.push({ type: 'drawCircle', cx, cy, r: innerR, fillStyle: '#1a2e1a' })
      } else {
        out.push({ type: 'drawCircle', cx, cy, r: Math.max(1, Math.max(w, h) / 2), fillStyle: '#e8c050' })
      }
    }
  }

  // 5–6. Vias and drills.
  //
  // Strict per-layer toggle: a Drill (or Via) is visible iff its drill-layer
  // toggle is on. The earlier span-aware variant (visible if any spanned
  // copper layer is visible) sounded physically right but in practice made
  // the drill-layer toggle a no-op whenever any copper layer was on, which
  // is almost always the case — the user couldn't see a difference between
  // toggling D_1_2 vs D_5_6. Strict mode makes the toggle do what the
  // panel says it does.
  //
  // Records with no layer attribution (older parser output, last-resort
  // synthesis) fall back to the legacy "any copper visible" gate.
  const MAX_VIA_MM = 15
  const anyCopperVisible = (boardData.layers ?? []).some(
    l => !hiddenLayers.has(l.name) && (l.type === 'COPPER' || l.type === 'DRILL')
  )
  const drillLayerVisible = (layer: string | undefined): boolean => {
    if (!layer) return anyCopperVisible
    return !hiddenLayers.has(layer)
  }

  for (const v of boardData.vias ?? []) {
    if (!drillLayerVisible(v.layer)) continue
    const cx = tx(v.x), cy = ty(v.y)
    if (!ok(cx) || !ok(cy)) continue
    const outerR = Math.max(2, Math.min((v.outerDiamMM / 2) * s, MAX_VIA_MM * s))
    const innerR = Math.max(0.8, Math.min((v.drillDiamMM / 2) * s, outerR * 0.85))
    out.push({ type: 'drawCircle', cx, cy, r: outerR, fillStyle: '#d4a840' })
    out.push({ type: 'drawCircle', cx, cy, r: innerR, fillStyle: '#060606' })
  }
  for (const d of boardData.drills ?? []) {
    if (!drillLayerVisible(d.layer)) continue
    const cx = tx(d.x), cy = ty(d.y)
    if (!ok(cx) || !ok(cy)) continue
    const r = Math.max(0.8, Math.min((d.diamMM / 2) * s, MAX_VIA_MM * s))
    out.push({ type: 'drawCircle', cx, cy, r, fillStyle: d.plated ? '#d4a840' : '#3a3a3a', alpha: 0.7 })
    out.push({ type: 'drawCircle', cx, cy, r: r * 0.6, fillStyle: '#060606' })
  }

  // 7. Soldermask (multiply composite tint + pad openings)
  const hasMaskVisible = (boardData.layers ?? []).some(
    l => isMaskLayer(l.name.toLowerCase()) && !hiddenLayers.has(l.name)
  )
  if (hasMaskVisible) {
    out.push({ type: 'setComposite', operation: 'multiply' })
    if (outlinePts.length > 1) {
      out.push({
        type: 'drawPolygon',
        points: outlinePts.map(p => ({ x: tx(p.x), y: ty(p.y) })),
        fillStyle: 'rgba(0,40,0,0.52)',
        close: true,
      })
    } else {
      out.push({
        type: 'fillRect',
        x: tx(minX), y: ty(minY),
        w: (maxX - minX) * s, h: (maxY - minY) * s,
        fillStyle: 'rgba(0,40,0,0.52)',
      })
    }
    out.push({ type: 'setComposite', operation: 'source-over' })

    for (const [layer, pads] of Object.entries(padsByLayer)) {
      if (!isMaskLayer(layer.toLowerCase())) continue
      if (hiddenLayers.has(layer)) continue
      for (const p of pads) {
        const cx = tx(p.x), cy = ty(p.y)
        if (!ok(cx) || !ok(cy)) continue
        const w = Math.max(1, p.widthMM * s)
        const h = Math.max(1, p.heightMM * s)
        if (p.shape === 'RECT') {
          out.push({ type: 'fillRect', x: cx - w / 2, y: cy - h / 2, w, h, fillStyle: '#e8c050', alpha: 0.9 })
        } else if (p.shape === 'OVAL' && Math.abs(w - h) > 1) {
          out.push({ type: 'drawEllipse', cx, cy, rx: Math.max(1, w / 2), ry: Math.max(1, h / 2), fillStyle: '#e8c050', alpha: 0.9 })
        } else {
          out.push({ type: 'drawCircle', cx, cy, r: Math.max(1, Math.max(w, h) / 2), fillStyle: '#e8c050', alpha: 0.9 })
        }
      }
    }
  }

  // 8. Silkscreen
  for (const [layer, traces] of Object.entries(tracesByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (!isSilkLayer(layer.toLowerCase())) continue
    const color = getSilkColor(layer.toLowerCase())
    for (const t of traces) {
      const x1 = tx(t.startX), y1 = ty(t.startY)
      const x2 = tx(t.endX),   y2 = ty(t.endY)
      if (!ok(x1) || !ok(y1) || !ok(x2) || !ok(y2)) continue
      const lw = Math.max(0.3, isFinite(t.widthMM) ? t.widthMM * s : 0.3)
      out.push({ type: 'drawLine', x1, y1, x2, y2, strokeStyle: color, lineWidth: lw, alpha: 0.7 })
    }
  }

  // 9. Board edge outline
  if (outlinePts.length > 1) {
    out.push({
      type: 'drawPolygon',
      points: outlinePts.map(p => ({ x: tx(p.x), y: ty(p.y) })),
      strokeStyle: '#50ff80',
      lineWidth: 1.5,
      close: true,
    })
  }

  // 10. Grid
  if (gridEnabled) {
    const pxPerMM = s * zoom
    const mmSteps = [0.5, 1, 2.5, 5, 10, 25, 50, 100]
    const spacingMM = mmSteps.find(step => step * pxPerMM >= 20) ?? 100
    const startX = Math.floor((minX - 5) / spacingMM) * spacingMM
    const endX   = maxX + 5
    const startY = Math.floor((minY - 5) / spacingMM) * spacingMM
    const endY   = maxY + 5
    const gridColor = 'rgba(100,200,100,0.12)'
    for (let gx = startX; gx <= endX; gx += spacingMM) {
      const vx = (gx - minX) * s + offX
      out.push({ type: 'drawLine', x1: vx, y1: (startY - minY) * s + offY, x2: vx, y2: (endY - minY) * s + offY, strokeStyle: gridColor, lineWidth: 0.5 })
    }
    for (let gy = startY; gy <= endY; gy += spacingMM) {
      const vy = (gy - minY) * s + offY
      out.push({ type: 'drawLine', x1: (startX - minX) * s + offX, y1: vy, x2: (endX - minX) * s + offX, y2: vy, strokeStyle: gridColor, lineWidth: 0.5 })
    }
  }

  // 11. Violation markers
  for (const v of violations) {
    if (!ok(v.x) || !ok(v.y)) continue
    const cx = tx(v.x), cy = ty(v.y)
    if (!ok(cx) || !ok(cy)) continue
    const color = SEV_COLOR[v.severity] ?? '#7090a8'
    const isSelected = v.id === selectedViolationId
    const pulseFraction = isSelected ? (now % 1800) / 1800 : undefined
    const marker: DrawViolationMarker = {
      type: 'drawViolationMarker',
      cx, cy, r: 8, color, severity: v.severity, selected: isSelected, pulseFraction,
    }
    if ((v.x2 !== 0 || v.y2 !== 0) && ok(tx(v.x2)) && ok(ty(v.y2))) {
      marker.x2 = tx(v.x2)
      marker.y2 = ty(v.y2)
    }
    out.push(marker)
  }

  return out
}
