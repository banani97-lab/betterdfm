'use client'

import { useRef, useState, useCallback, useMemo, useEffect } from 'react'
import { ZoomIn, ZoomOut, Home, Layers, Grid3X3, RotateCw } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Violation, BoardData } from '@/lib/api'

// ── Props ─────────────────────────────────────────────────────────────────────

export interface BoardViewerTransform {
  offsetX: number
  offsetY: number
  scale: number
}

interface BoardViewerProps {
  violations: Violation[]
  boardData?: BoardData | null
  selectedViolationId?: string
  onViolationClick?: (v: Violation | null) => void
  hiddenLayers: Set<string>
  onToggleLayer: (name: string) => void
  onSetHiddenLayers?: (layers: Set<string>) => void
  violationLayers?: Set<string>       // layers that have at least one violation
  allIgnoredLayers?: Set<string>      // layers where every violation is ignored
  onIgnoreLayer?: (name: string, ignored: boolean, severity?: string) => void
  /** When provided, the viewer uses this transform instead of its own internal state. */
  externalTransform?: BoardViewerTransform
  /** Called whenever the user pans/zooms. Parent can sync to another viewer. */
  onTransformChange?: (transform: BoardViewerTransform) => void
  /** Optional label displayed at top-left of the board viewer. */
  label?: string
}

interface Bounds {
  minX: number; minY: number; maxX: number; maxY: number
  scale: number; offX: number; offY: number
}

// ── Constants ─────────────────────────────────────────────────────────────────

const SAFE = 1e5
const ok = (n: number): boolean => isFinite(n) && n > -SAFE && n < SAFE

const SEV_COLOR: Record<string, string> = {
  ERROR:   '#ff3333',
  WARNING: '#ff6b00',
  INFO:    '#44aaff',
}

// ── Layer classification ──────────────────────────────────────────────────────

function isSilkLayer(n: string): boolean {
  return n.includes('silk') || n.includes('legend') || n.includes('gto') ||
         n.includes('gbo') || n.includes('overlay')
}
function isMaskLayer(n: string): boolean {
  return n.includes('mask') || n.includes('covertop') || n.includes('coverbottom') ||
         n.includes('cover') || n.includes('gts') || n.includes('gbs')
}
function isOutlineLayer(n: string): boolean {
  return n.includes('outline') || n.includes('gko') || n.includes('edge') ||
         n.includes('board') || n === 'rout'
}
function isDrillLayer(n: string): boolean {
  return n.includes('drill') || n.includes('drl') || n.includes('xln') ||
         n.includes('exc') || n.includes('ncd') || n === 'drills'
}
function isCopperLayer(n: string): boolean {
  return !isSilkLayer(n) && !isMaskLayer(n) && !isOutlineLayer(n) && !isDrillLayer(n)
}

function getSilkColor(n: string): string {
  return (n.includes('bot') || n.includes('back') || n.includes('gbo') || n.includes('b.'))
    ? '#d0c060' : '#f0e8d8'
}

function getLayerColor(layerName: string): string {
  const n = layerName.toLowerCase()
  if (isSilkLayer(n)) return getSilkColor(n)
  if (isMaskLayer(n)) return '#00dd66'
  if (isOutlineLayer(n)) return '#44ff88'
  if (n === 'signal_1' || n.includes('gtl') || n.includes('f.cu') || n.includes('top.cu')) return '#f0a830'
  if (n.includes('gbl') || n.includes('b.cu') || n.includes('bottom.cu')) return '#60b8f0'
  if (n.includes('flex')) return '#e8b840'
  if (n.includes('plane')) return '#c87820'
  if (n.includes('signal') || n.includes('copper') || n.includes('.cu')) return '#c07828'
  return '#7090a8'
}

// ── Bounds computation ────────────────────────────────────────────────────────

function computeBounds(boardData: BoardData): Bounds {
  const xs: number[] = []
  const ys: number[] = []

  if (boardData.outline?.length > 0) {
    for (const pt of boardData.outline) {
      if (ok(pt.x) && ok(pt.y)) { xs.push(pt.x); ys.push(pt.y) }
    }
  } else {
    for (const t of boardData.traces ?? []) {
      if (ok(t.startX) && ok(t.startY)) { xs.push(t.startX); ys.push(t.startY) }
      if (ok(t.endX)   && ok(t.endY))   { xs.push(t.endX);   ys.push(t.endY) }
    }
    for (const p of boardData.pads   ?? []) { if (ok(p.x) && ok(p.y)) { xs.push(p.x); ys.push(p.y) } }
    for (const v of boardData.vias   ?? []) { if (ok(v.x) && ok(v.y)) { xs.push(v.x); ys.push(v.y) } }
    for (const d of boardData.drills ?? []) { if (ok(d.x) && ok(d.y)) { xs.push(d.x); ys.push(d.y) } }
  }

  if (xs.length === 0) return { minX: 0, minY: 0, maxX: 100, maxY: 70, scale: 5, offX: 50, offY: 50 }

  const minX = Math.min(...xs)
  const minY = Math.min(...ys)
  const maxX = Math.max(...xs)
  const maxY = Math.max(...ys)
  const boardW = maxX - minX || 1
  const boardH = maxY - minY || 1
  const CANVAS_W = 1200
  const CANVAS_H = 800
  const scale = Math.min((CANVAS_W / boardW) * 0.9, (CANVAS_H / boardH) * 0.9)
  const offX = (CANVAS_W - boardW * scale) / 2
  const offY = (CANVAS_H - boardH * scale) / 2
  return { minX, minY, maxX, maxY, scale, offX, offY }
}

// ── Canvas drawing helpers ────────────────────────────────────────────────────

/** Builds the board outline path (polygon from outline pts or fallback rect). */
function boardOutlinePath(ctx: CanvasRenderingContext2D, boardData: BoardData, b: Bounds) {
  const { minX, minY, maxX, maxY, scale: s, offX, offY } = b
  const tx = (x: number) => (x - minX) * s + offX
  const ty = (y: number) => (maxY - y) * s + offY   // flip Y: ODB++ Y-up → canvas Y-down
  ctx.beginPath()
  if (boardData.outline?.length > 1) {
    const pts = boardData.outline.filter(p => ok(p.x) && ok(p.y))
    if (pts.length > 1) {
      ctx.moveTo(tx(pts[0].x), ty(pts[0].y))
      for (let i = 1; i < pts.length; i++) ctx.lineTo(tx(pts[i].x), ty(pts[i].y))
      ctx.closePath()
      return
    }
  }
  ctx.rect(tx(minX), ty(maxY), (maxX - minX) * s, (maxY - minY) * s)
}

/** Step 2: FR4 substrate fill. */
function drawBoardFill(ctx: CanvasRenderingContext2D, boardData: BoardData, b: Bounds) {
  boardOutlinePath(ctx, boardData, b)
  ctx.fillStyle = '#1a2e1a'
  ctx.fill()
}

/** Steps 3–6: copper traces, pads, vias, drills. */
function drawCopper(
  ctx: CanvasRenderingContext2D,
  b: Bounds,
  tracesByLayer: Record<string, NonNullable<BoardData['traces']>>,
  padsByLayer:   Record<string, NonNullable<BoardData['pads']>>,
  boardData: BoardData | null | undefined,
  hiddenLayers: Set<string>,
  polygonsByLayer: Record<string, NonNullable<BoardData['polygons']>>,
) {
  const { minX, minY, maxY, scale: s } = b
  const offX = b.offX, offY = b.offY
  const tx = (x: number) => (x - minX) * s + offX
  const ty = (y: number) => (maxY - y) * s + offY

  // Copper fill polygons (surfaces) — use evenodd so hole contours cut through
  ctx.fillStyle = '#c88020'
  for (const [layer, polys] of Object.entries(polygonsByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (!isCopperLayer(layer.toLowerCase())) continue
    for (const poly of polys) {
      if (poly.points.length < 3) continue
      ctx.beginPath()
      // Outer island
      ctx.moveTo(tx(poly.points[0].x), ty(poly.points[0].y))
      for (let i = 1; i < poly.points.length; i++) {
        ctx.lineTo(tx(poly.points[i].x), ty(poly.points[i].y))
      }
      ctx.closePath()
      // Hole contours (pad/via clearances in the copper pour)
      for (const hole of poly.holes ?? []) {
        if (hole.length < 3) continue
        ctx.moveTo(tx(hole[0].x), ty(hole[0].y))
        for (let i = 1; i < hole.length; i++) {
          ctx.lineTo(tx(hole[i].x), ty(hole[i].y))
        }
        ctx.closePath()
      }
      ctx.fill('evenodd')
    }
  }

  // Traces
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.strokeStyle = '#b47a22'
  for (const [layer, traces] of Object.entries(tracesByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (!isCopperLayer(layer.toLowerCase())) continue
    for (const t of traces) {
      const x1 = tx(t.startX), y1 = ty(t.startY)
      const x2 = tx(t.endX),   y2 = ty(t.endY)
      if (!ok(x1) || !ok(y1) || !ok(x2) || !ok(y2)) continue
      const sw = Math.max(0.5, isFinite(t.widthMM) ? t.widthMM * s : 0.5)
      ctx.lineWidth = sw
      ctx.beginPath()
      ctx.moveTo(x1, y1)
      ctx.lineTo(x2, y2)
      ctx.stroke()
    }
  }

  // Pads
  ctx.fillStyle = '#e8c050'
  for (const [layer, pads] of Object.entries(padsByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (!isCopperLayer(layer.toLowerCase())) continue
    for (const p of pads) {
      const cx = tx(p.x), cy = ty(p.y)
      if (!ok(cx) || !ok(cy)) continue
      const w = Math.max(1, p.widthMM * s)
      const h = Math.max(1, p.heightMM * s)
      ctx.beginPath()
      if (p.shape === 'RECT') {
        ctx.rect(cx - w / 2, cy - h / 2, w, h)
      } else if (p.shape === 'OVAL' && Math.abs(w - h) > 1) {
        ctx.ellipse(cx, cy, Math.max(1, w / 2), Math.max(1, h / 2), 0, 0, Math.PI * 2)
      } else {
        ctx.arc(cx, cy, Math.max(1, Math.max(w, h) / 2), 0, Math.PI * 2)
      }
      ctx.fill()
    }
  }

  // Vias and drills — only render when copper or drill layers are visible
  const allLayers = boardData?.layers ?? []
  const anyCopperVisible = allLayers.some(
    l => !hiddenLayers.has(l.name) && (l.type === 'COPPER' || l.type === 'DRILL')
  )
  if (!anyCopperVisible) return

  // Vias
  const MAX_VIA_MM = 15  // cap to guard against parser unit artifacts
  for (const v of boardData?.vias ?? []) {
    const cx = tx(v.x), cy = ty(v.y)
    if (!ok(cx) || !ok(cy)) continue
    const outerR = Math.max(2, Math.min((v.outerDiamMM / 2) * s, MAX_VIA_MM * s))
    const innerR = Math.max(0.8, Math.min((v.drillDiamMM / 2) * s, outerR * 0.85))
    ctx.fillStyle = '#d4a840'
    ctx.beginPath(); ctx.arc(cx, cy, outerR, 0, Math.PI * 2); ctx.fill()
    ctx.fillStyle = '#060606'
    ctx.beginPath(); ctx.arc(cx, cy, innerR, 0, Math.PI * 2); ctx.fill()
  }

  // Drills
  for (const d of boardData?.drills ?? []) {
    const cx = tx(d.x), cy = ty(d.y)
    if (!ok(cx) || !ok(cy)) continue
    const r = Math.max(0.8, Math.min((d.diamMM / 2) * s, MAX_VIA_MM * s))
    ctx.fillStyle = d.plated ? '#d4a840' : '#3a3a3a'
    ctx.globalAlpha = 0.7
    ctx.beginPath(); ctx.arc(cx, cy, r, 0, Math.PI * 2); ctx.fill()
    ctx.globalAlpha = 1
    ctx.fillStyle = '#060606'
    ctx.beginPath(); ctx.arc(cx, cy, r * 0.6, 0, Math.PI * 2); ctx.fill()
  }
}

/** Step 6b: routing cutouts (ROUT layer traces = router tool paths). */
function drawRouting(
  ctx: CanvasRenderingContext2D,
  b: Bounds,
  tracesByLayer: Record<string, NonNullable<BoardData['traces']>>,
  hiddenLayers: Set<string>,
) {
  const { minX, minY, maxY, scale: s, offX, offY } = b
  const tx = (x: number) => (x - minX) * s + offX
  const ty = (y: number) => (maxY - y) * s + offY

  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.strokeStyle = '#ffffff'
  ctx.globalAlpha = 0.85

  for (const [layer, traces] of Object.entries(tracesByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (layer.toLowerCase() !== 'rout') continue
    for (const t of traces) {
      const x1 = tx(t.startX), y1 = ty(t.startY)
      const x2 = tx(t.endX),   y2 = ty(t.endY)
      if (!ok(x1) || !ok(y1) || !ok(x2) || !ok(y2)) continue
      ctx.lineWidth = Math.max(1, isFinite(t.widthMM) ? t.widthMM * s : 1)
      ctx.beginPath()
      ctx.moveTo(x1, y1)
      ctx.lineTo(x2, y2)
      ctx.stroke()
    }
  }
  ctx.globalAlpha = 1
}

/** Step 7: soldermask overlay + pad openings.
 *  Respects hiddenLayers: if every mask layer is hidden the overlay is suppressed.
 *  Pad openings (exposed copper) are rendered as gold spots from the mask-layer pads. */
function drawSoldermask(
  ctx: CanvasRenderingContext2D,
  boardData: BoardData,
  b: Bounds,
  padsByLayer: Record<string, NonNullable<BoardData['pads']>>,
  hiddenLayers: Set<string>,
) {
  const hasMaskVisible = boardData.layers?.some(
    l => isMaskLayer(l.name.toLowerCase()) && !hiddenLayers.has(l.name)
  ) ?? false
  if (!hasMaskVisible) return

  // Green multiply tint over the board outline
  ctx.save()
  ctx.globalCompositeOperation = 'multiply'
  boardOutlinePath(ctx, boardData, b)
  ctx.fillStyle = 'rgba(0,40,0,0.52)'
  ctx.fill()
  ctx.restore()

  // Pad openings — render exposed copper for each visible mask layer's pads
  const { minX, minY, maxY, scale: s, offX, offY } = b
  const tx = (x: number) => (x - minX) * s + offX
  const ty = (y: number) => (maxY - y) * s + offY

  ctx.fillStyle = '#e8c050'
  ctx.globalAlpha = 0.9
  for (const [layer, pads] of Object.entries(padsByLayer)) {
    if (!isMaskLayer(layer.toLowerCase())) continue
    if (hiddenLayers.has(layer)) continue
    for (const p of pads) {
      const cx = tx(p.x), cy = ty(p.y)
      if (!ok(cx) || !ok(cy)) continue
      const w = Math.max(1, p.widthMM * s)
      const h = Math.max(1, p.heightMM * s)
      ctx.beginPath()
      if (p.shape === 'RECT') {
        ctx.rect(cx - w / 2, cy - h / 2, w, h)
      } else if (p.shape === 'OVAL' && Math.abs(w - h) > 1) {
        ctx.ellipse(cx, cy, Math.max(1, w / 2), Math.max(1, h / 2), 0, 0, Math.PI * 2)
      } else {
        ctx.arc(cx, cy, Math.max(1, Math.max(w, h) / 2), 0, Math.PI * 2)
      }
      ctx.fill()
    }
  }
  ctx.globalAlpha = 1
}

/** Step 8: silkscreen traces (cream top / yellow-green bottom). */
function drawSilkscreen(
  ctx: CanvasRenderingContext2D,
  b: Bounds,
  tracesByLayer: Record<string, NonNullable<BoardData['traces']>>,
  hiddenLayers: Set<string>,
) {
  const { minX, minY, maxY, scale: s } = b
  const offX = b.offX, offY = b.offY
  const tx = (x: number) => (x - minX) * s + offX
  const ty = (y: number) => (maxY - y) * s + offY

  ctx.lineCap = 'round'
  for (const [layer, traces] of Object.entries(tracesByLayer)) {
    if (hiddenLayers.has(layer)) continue
    if (!isSilkLayer(layer.toLowerCase())) continue
    ctx.strokeStyle = getSilkColor(layer.toLowerCase())
    ctx.globalAlpha = 0.7
    for (const t of traces) {
      const x1 = tx(t.startX), y1 = ty(t.startY)
      const x2 = tx(t.endX),   y2 = ty(t.endY)
      if (!ok(x1) || !ok(y1) || !ok(x2) || !ok(y2)) continue
      ctx.lineWidth = Math.max(0.3, isFinite(t.widthMM) ? t.widthMM * s : 0.3)
      ctx.beginPath()
      ctx.moveTo(x1, y1)
      ctx.lineTo(x2, y2)
      ctx.stroke()
    }
    ctx.globalAlpha = 1
  }
}

/** Step 9: board edge outline with green glow. */
function drawBoardEdge(ctx: CanvasRenderingContext2D, boardData: BoardData, b: Bounds) {
  ctx.save()
  ctx.shadowColor = '#50ff80'
  ctx.shadowBlur = 8
  boardOutlinePath(ctx, boardData, b)
  ctx.strokeStyle = '#50ff80'
  ctx.lineWidth = 1.5
  ctx.stroke()
  ctx.restore()
}

/** Step 9b: glow highlight around the component responsible for the selected violation. */
function drawComponentHighlight(
  ctx: CanvasRenderingContext2D,
  b: Bounds,
  pads: NonNullable<BoardData['pads']>,
  violation: Violation,
) {
  if (!violation.refDes) return
  const { minX, maxY, scale: s, offX, offY } = b
  const tx = (x: number) => (x - minX) * s + offX
  const ty = (y: number) => (maxY - y) * s + offY
  const color = SEV_COLOR[violation.severity] ?? '#7090a8'

  const matchingPads = pads.filter(p => p.refDes === violation.refDes)
  if (!matchingPads.length) return

  // Bounding box of all component pads in canvas coordinates
  let minCX = Infinity, minCY = Infinity, maxCX = -Infinity, maxCY = -Infinity
  for (const p of matchingPads) {
    const cx = tx(p.x), cy = ty(p.y)
    if (!ok(cx) || !ok(cy)) continue
    const hw = Math.max(3, (p.widthMM * s) / 2)
    const hh = Math.max(3, (p.heightMM * s) / 2)
    minCX = Math.min(minCX, cx - hw)
    minCY = Math.min(minCY, cy - hh)
    maxCX = Math.max(maxCX, cx + hw)
    maxCY = Math.max(maxCY, cy + hh)
  }
  if (!isFinite(minCX)) return

  const pad = 8
  ctx.save()
  ctx.strokeStyle = color
  ctx.lineWidth = 1.5
  ctx.shadowColor = color
  ctx.shadowBlur = 14
  ctx.globalAlpha = 0.75
  ctx.setLineDash([5, 3])
  ctx.beginPath()
  ctx.rect(minCX - pad, minCY - pad, maxCX - minCX + pad * 2, maxCY - minCY + pad * 2)
  ctx.stroke()

  // RefDes label above the bounding box
  ctx.setLineDash([])
  ctx.shadowBlur = 6
  ctx.globalAlpha = 0.95
  ctx.fillStyle = color
  ctx.font = 'bold 10px monospace'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'bottom'
  ctx.fillText(violation.refDes, (minCX + maxCX) / 2, minCY - pad - 2)
  ctx.restore()
}

/** Step 10: adaptive mm grid. */
function drawGrid(ctx: CanvasRenderingContext2D, b: Bounds, zoom: number) {
  const { minX, minY, maxX, maxY, scale, offX, offY } = b
  const pxPerMM = scale * zoom

  const mmSteps = [0.5, 1, 2.5, 5, 10, 25, 50, 100]
  const spacingMM = mmSteps.find(s => s * pxPerMM >= 20) ?? 100

  const startX = Math.floor((minX - 5) / spacingMM) * spacingMM
  const endX   = maxX + 5
  const startY = Math.floor((minY - 5) / spacingMM) * spacingMM
  const endY   = maxY + 5

  ctx.save()
  ctx.strokeStyle = 'rgba(100,200,100,0.12)'
  ctx.lineWidth = 0.5 / zoom
  ctx.setLineDash([])

  for (let gx = startX; gx <= endX; gx += spacingMM) {
    const vx = (gx - minX) * scale + offX
    ctx.beginPath()
    ctx.moveTo(vx, (maxY - startY) * scale + offY)
    ctx.lineTo(vx, (maxY - endY)   * scale + offY)
    ctx.stroke()
  }
  for (let gy = startY; gy <= endY; gy += spacingMM) {
    const vy = (maxY - gy) * scale + offY
    ctx.beginPath()
    ctx.moveTo((startX - minX) * scale + offX, vy)
    ctx.lineTo((endX   - minX) * scale + offX, vy)
    ctx.stroke()
  }
  ctx.restore()
}

/** Step 11: EDA-style violation markers — triangle/diamond/circle. */
function drawViolations(
  ctx: CanvasRenderingContext2D,
  b: Bounds | null,
  violations: Violation[],
  selectedViolationId: string | undefined,
  focusMode: boolean,
) {
  // In focus mode only render the selected marker — hides all the noise
  const toRender = focusMode
    ? violations.filter(v => v.id === selectedViolationId)
    : violations
  if (!toRender.length) return
  const now = Date.now()

  if (b) {
    const { minX, minY, maxY, scale: s, offX, offY } = b
    const tx = (x: number) => (x - minX) * s + offX
    const ty = (y: number) => (maxY - y) * s + offY

    for (const v of toRender) {
      const cx = tx(v.x), cy = ty(v.y)
      if (!ok(cx) || !ok(cy)) continue
      const isSelected = v.id === selectedViolationId
      const color = SEV_COLOR[v.severity] ?? '#7090a8'

      if (isSelected) {
        ctx.save()
        ctx.strokeStyle = color
        ctx.lineWidth = 2
        // Crosshair rectangle around the violation point. Suppressed when
        // the violation is component-scoped (has a refDes) because
        // drawComponentHighlight will draw a dashed bounding box around
        // the whole component — two stacked red squares is just noise.
        if (!v.refDes) {
          const r = 12
          ctx.strokeRect(cx - r, cy - r, r * 2, r * 2)
        }
        // Dashed line to secondary object (clearance/dam rules).
        if (v.x2 !== 0 || v.y2 !== 0) {
          const cx2 = tx(v.x2), cy2 = ty(v.y2)
          if (ok(cx2) && ok(cy2)) {
            ctx.globalAlpha = 0.5
            ctx.lineWidth = 1
            ctx.setLineDash([4, 3])
            ctx.beginPath(); ctx.moveTo(cx, cy); ctx.lineTo(cx2, cy2); ctx.stroke()
            ctx.setLineDash([])
            ctx.beginPath(); ctx.arc(cx2, cy2, 4, 0, Math.PI * 2); ctx.stroke()
          }
        }
        ctx.restore()
      } else {
        // Unselected: small colored dot with a thin ring — minimal footprint
        ctx.save()
        ctx.fillStyle = color
        ctx.globalAlpha = 0.85
        ctx.beginPath(); ctx.arc(cx, cy, 3, 0, Math.PI * 2); ctx.fill()
        ctx.globalAlpha = 0.5
        ctx.strokeStyle = color; ctx.lineWidth = 1
        ctx.beginPath(); ctx.arc(cx, cy, 6, 0, Math.PI * 2); ctx.stroke()
        ctx.restore()
      }
    }
  } else {
    // Fallback: no board data — scatter plot on 1200×800 canvas space
    const maxVX = toRender.reduce((m, v) => Math.max(m, Math.abs(v.x)), 1)
    const maxVY = toRender.reduce((m, v) => Math.max(m, Math.abs(v.y)), 1)
    const sx = 1160 / (maxVX * 2 + 1)
    const sy =  760 / (maxVY * 2 + 1)

    ctx.strokeStyle = 'rgba(45,74,45,0.5)'
    ctx.lineWidth = 0.5
    for (let i = 0; i <= 10; i++) {
      const x = 20 + i * 116
      const y = 20 + i * 76
      ctx.beginPath(); ctx.moveTo(x, 20);  ctx.lineTo(x, 780); ctx.stroke()
      ctx.beginPath(); ctx.moveTo(20, y);  ctx.lineTo(1180, y); ctx.stroke()
    }

    for (const v of toRender) {
      const cx = 20 + (v.x + maxVX) * sx
      const cy = 20 + (v.y + maxVY) * sy
      ctx.fillStyle = SEV_COLOR[v.severity] ?? '#6b7280'
      ctx.globalAlpha = 0.85
      ctx.beginPath(); ctx.arc(cx, cy, 6, 0, Math.PI * 2); ctx.fill()
      ctx.globalAlpha = 1
    }
  }
}

// ── Component ─────────────────────────────────────────────────────────────────

export function BoardViewer({
  violations,
  boardData,
  selectedViolationId,
  onViolationClick,
  hiddenLayers,
  onToggleLayer,
  onSetHiddenLayers,
  violationLayers,
  allIgnoredLayers,
  onIgnoreLayer,
  externalTransform,
  onTransformChange,
  label,
}: BoardViewerProps) {
  const canvasRef    = useRef<HTMLCanvasElement>(null)
  const zoomRef      = useRef(1)
  const panRef       = useRef({ x: 0, y: 0 })
  const rotationRef  = useRef(0)  // 0 | 90 | 180 | 270
  const lastMouseRef = useRef({ x: 0, y: 0 })
  const lastTouchRef = useRef<{ x: number; y: number } | null>(null)
  const pinchStartRef = useRef<{
    distance: number
    startZoom: number
    startPan: { x: number; y: number }
    startMid: { x: number; y: number }
  } | null>(null)
  const draggingRef  = useRef(false)
  const didMoveRef   = useRef(false)
  const hasInitRef   = useRef(false)

  const [dragging,          setDragging]          = useState(false)
  const [layerPanelOpen,    setLayerPanelOpen]    = useState(true)
  const [gridEnabled,       setGridEnabled]       = useState(false)
  const [mouseCoords,       setMouseCoords]       = useState<{ x: number; y: number } | null>(null)
  const [zoomPct,           setZoomPct]           = useState(100)
  const [rotation,          setRotation]          = useState(0)
  const [openDropdownLayer, setOpenDropdownLayer] = useState<string | null>(null)

  // Snapshot of hiddenLayers captured when a violation focus session starts.
  // Restored when the user deselects. `null` = not currently in a focus session.
  const preFocusHiddenRef = useRef<Set<string> | null>(null)
  // Mirror of the latest hiddenLayers prop so the focus effect can read it
  // without taking a dependency (which would cause the effect to re-run and
  // clobber its own updates).
  const hiddenLayersRef = useRef(hiddenLayers)
  useEffect(() => { hiddenLayersRef.current = hiddenLayers })

  // ── External transform synchronization ────────────────────────────────────
  // When controlled externally, sync refs from the prop so draw() uses them.
  useEffect(() => {
    if (!externalTransform) return
    zoomRef.current = externalTransform.scale
    panRef.current  = { x: externalTransform.offsetX, y: externalTransform.offsetY }
    setZoomPct(Math.round(externalTransform.scale * 100))
    draw()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [externalTransform?.offsetX, externalTransform?.offsetY, externalTransform?.scale])

  /** Notify parent of transform changes (no-op when uncontrolled). */
  const notifyTransform = useCallback(() => {
    onTransformChange?.({
      offsetX: panRef.current.x,
      offsetY: panRef.current.y,
      scale: zoomRef.current,
    })
  }, [onTransformChange])

  // ── Derived data ────────────────────────────────────────────────────────────

  const bounds = useMemo(() => boardData ? computeBounds(boardData) : null, [boardData])
  const layers = useMemo(() => boardData?.layers ?? [], [boardData])

  const tracesByLayer = useMemo(() => {
    const m: Record<string, NonNullable<BoardData['traces']>> = {}
    for (const t of boardData?.traces ?? []) (m[t.layer] ??= []).push(t)
    return m
  }, [boardData])

  const padsByLayer = useMemo(() => {
    const m: Record<string, NonNullable<BoardData['pads']>> = {}
    for (const p of boardData?.pads ?? []) (m[p.layer] ??= []).push(p)
    return m
  }, [boardData])

  const polygonsByLayer = useMemo(() => {
    const m: Record<string, NonNullable<BoardData['polygons']>> = {}
    for (const p of boardData?.polygons ?? []) (m[p.layer] ??= []).push(p)
    return m
  }, [boardData])

  // ── Imperative draw ─────────────────────────────────────────────────────────

  const draw = useCallback(() => {
    const canvas = canvasRef.current
    const ctx    = canvas?.getContext('2d')
    if (!canvas || !ctx || !canvas.width || !canvas.height) return

    const dpr = window.devicePixelRatio || 1
    const { width, height } = canvas

    // 1. Background (outside board)
    ctx.clearRect(0, 0, width, height)
    ctx.fillStyle = '#060e06'
    ctx.fillRect(0, 0, width, height)

    ctx.save()
    // Map viewBox (1200×800) coordinates to physical canvas pixels
    ctx.setTransform(
      zoomRef.current * dpr, 0,
      0, zoomRef.current * dpr,
      panRef.current.x * dpr, panRef.current.y * dpr,
    )
    // Apply board rotation around the viewBox centre (600, 400)
    if (rotationRef.current !== 0) {
      const rad = (rotationRef.current * Math.PI) / 180
      ctx.translate(600, 400)
      ctx.rotate(rad)
      ctx.translate(-600, -400)
    }

    if (bounds && boardData) {
      drawBoardFill(ctx, boardData, bounds)                               // 2: FR4
      drawCopper(ctx, bounds, tracesByLayer, padsByLayer, boardData, hiddenLayers, polygonsByLayer) // 3–6
      drawRouting(ctx, bounds, tracesByLayer, hiddenLayers)               // 6b: rout cutouts
      drawSoldermask(ctx, boardData, bounds, padsByLayer, hiddenLayers)   // 7: multiply
      drawSilkscreen(ctx, bounds, tracesByLayer, hiddenLayers)            // 8: silk
      drawBoardEdge(ctx, boardData, bounds)                               // 9: edge glow
    }

    if (gridEnabled && bounds) drawGrid(ctx, bounds, zoomRef.current)    // 10: grid

    const selectedViolation = violations.find(v => v.id === selectedViolationId)
    const focusMode = !!selectedViolation
    drawViolations(ctx, bounds, violations, selectedViolationId, focusMode) // 11: markers
    if (selectedViolation && bounds) {
      drawComponentHighlight(ctx, bounds, boardData?.pads ?? [], selectedViolation) // 11b: component
    }

    ctx.restore()
  }, [bounds, boardData, tracesByLayer, padsByLayer, polygonsByLayer, hiddenLayers, violations, selectedViolationId, gridEnabled])

  // ── ResizeObserver — keeps canvas px matched to container at DPR ───────────

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas?.parentElement) return

    const observer = new ResizeObserver(() => {
      const dpr = window.devicePixelRatio || 1
      const el = canvas.parentElement!
      const w = el.clientWidth
      const h = el.clientHeight
      canvas.width        = w * dpr
      canvas.height       = h * dpr
      canvas.style.width  = w + 'px'
      canvas.style.height = h + 'px'

      // Fit 1200×800 viewBox in the container on first mount
      if (!hasInitRef.current && w > 0 && h > 0) {
        const z = Math.min(w / 1200, h / 800)
        zoomRef.current = z
        panRef.current  = { x: (w - 1200 * z) / 2, y: (h - 800 * z) / 2 }
        hasInitRef.current = true
        setZoomPct(Math.round(z * 100))
      }

      draw()
    })

    observer.observe(canvas.parentElement)
    return () => observer.disconnect()
  }, [draw])

  // Redraw whenever data/settings change
  useEffect(() => { draw() }, [draw])

  // ── Center on selected violation ─────────────────────────────────────────────

  useEffect(() => {
    if (!selectedViolationId) { draw(); return }

    // Center the viewport on the selected violation
    const v = violations.find(vi => vi.id === selectedViolationId)
    const canvas = canvasRef.current
    if (v && bounds && canvas) {
      const canvasX = (v.x - bounds.minX) * bounds.scale + bounds.offX
      const canvasY = (bounds.maxY - v.y) * bounds.scale + bounds.offY
      const rect = canvas.getBoundingClientRect()
      const viewW = rect.width
      const viewH = rect.height
      const targetZoom = Math.max(3, zoomRef.current)
      zoomRef.current = targetZoom
      panRef.current = {
        x: viewW / 2 - canvasX * targetZoom,
        y: viewH / 2 - canvasY * targetZoom,
      }
      onTransformChange?.({ offsetX: panRef.current.x, offsetY: panRef.current.y, scale: targetZoom })
    }

    draw()
  }, [selectedViolationId, draw])

  // ── Filter visible layers to the selected violation's layer ─────────────────
  // When a violation is selected and carries a layer, narrow the toggled-on
  // layers to just that one so the zoomed-in view isn't cluttered with
  // unrelated copper/silk/etc. On deselect, restore the user's prior toggles.
  // Violations without a layer (e.g. fiducial-count) leave the toggles alone.
  useEffect(() => {
    if (!onSetHiddenLayers) return

    if (!selectedViolationId) {
      if (preFocusHiddenRef.current !== null) {
        onSetHiddenLayers(preFocusHiddenRef.current)
        preFocusHiddenRef.current = null
      }
      return
    }

    const v = violations.find(vi => vi.id === selectedViolationId)
    if (!v || !v.layer || layers.length === 0) return

    // Capture the pre-focus snapshot exactly once per focus session so that
    // clicking from one violation straight to another doesn't treat the
    // already-filtered state as "what the user wanted".
    if (preFocusHiddenRef.current === null) {
      preFocusHiddenRef.current = new Set(hiddenLayersRef.current)
    }

    // For drill/outline/rout violations, show the violation's layer PLUS
    // outer copper layers for context — a drill layer alone shows holes
    // but no surrounding copper, making it hard to orient.
    const vLayerLower = v.layer.toLowerCase()
    const needsContext = vLayerLower === 'drill' || vLayerLower.includes('drill') ||
      vLayerLower.includes('outline') || vLayerLower === 'rout'

    const visible = new Set<string>([v.layer])
    if (needsContext) {
      for (const l of layers) {
        // Add outer copper (first and last COPPER layers) for context
        if (l.type === 'COPPER' || l.type === 'POWER_GROUND') {
          visible.add(l.name)
          break
        }
      }
      for (let i = layers.length - 1; i >= 0; i--) {
        if (layers[i].type === 'COPPER' || layers[i].type === 'POWER_GROUND') {
          visible.add(layers[i].name)
          break
        }
      }
      // Also show all drill-type layers so holes are visible in the panel
      for (const l of layers) {
        if (l.type === 'DRILL') visible.add(l.name)
      }
    }

    const next = new Set<string>()
    for (const l of layers) {
      if (!visible.has(l.name)) next.add(l.name)
    }
    onSetHiddenLayers(next)
    // Depending only on selectedViolationId keeps this from re-firing when
    // the effect itself updates hiddenLayers via onSetHiddenLayers.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedViolationId])

  // ── Wheel — non-passive, zoom to cursor ─────────────────────────────────────

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const handler = (e: WheelEvent) => {
      e.preventDefault()
      const rect = canvas.getBoundingClientRect()
      const mx = e.clientX - rect.left
      const my = e.clientY - rect.top
      const factor  = e.deltaY < 0 ? 1.1 : 0.9
      const newZoom = Math.max(0.2, Math.min(20, zoomRef.current * factor))
      panRef.current.x = mx - (mx - panRef.current.x) * (newZoom / zoomRef.current)
      panRef.current.y = my - (my - panRef.current.y) * (newZoom / zoomRef.current)
      zoomRef.current  = newZoom
      setZoomPct(Math.round(newZoom * 100))
      notifyTransform()
      draw()
    }
    canvas.addEventListener('wheel', handler, { passive: false })
    return () => canvas.removeEventListener('wheel', handler)
  }, [draw, notifyTransform])

  // Touch gestures: one-finger pan and two-finger pinch zoom.
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const getTouchPoint = (touch: Touch) => {
      const rect = canvas.getBoundingClientRect()
      return { x: touch.clientX - rect.left, y: touch.clientY - rect.top }
    }

    const onTouchStart = (e: TouchEvent) => {
      if (e.touches.length === 1) {
        lastTouchRef.current = getTouchPoint(e.touches[0])
        pinchStartRef.current = null
        return
      }
      if (e.touches.length < 2) return

      const p1 = getTouchPoint(e.touches[0])
      const p2 = getTouchPoint(e.touches[1])
      const mid = { x: (p1.x + p2.x) / 2, y: (p1.y + p2.y) / 2 }
      const dist = Math.hypot(p2.x - p1.x, p2.y - p1.y)

      pinchStartRef.current = {
        distance: Math.max(1, dist),
        startZoom: zoomRef.current,
        startPan: { ...panRef.current },
        startMid: mid,
      }
      lastTouchRef.current = null
      e.preventDefault()
    }

    const onTouchMove = (e: TouchEvent) => {
      if (e.touches.length === 1 && !pinchStartRef.current) {
        const point = getTouchPoint(e.touches[0])
        const last = lastTouchRef.current
        if (last) {
          panRef.current = {
            x: panRef.current.x + point.x - last.x,
            y: panRef.current.y + point.y - last.y,
          }
          notifyTransform()
          draw()
        }
        lastTouchRef.current = point
        e.preventDefault()
        return
      }

      if (e.touches.length < 2) return

      const p1 = getTouchPoint(e.touches[0])
      const p2 = getTouchPoint(e.touches[1])
      const mid = { x: (p1.x + p2.x) / 2, y: (p1.y + p2.y) / 2 }
      const dist = Math.hypot(p2.x - p1.x, p2.y - p1.y)

      const start = pinchStartRef.current ?? {
        distance: Math.max(1, dist),
        startZoom: zoomRef.current,
        startPan: { ...panRef.current },
        startMid: mid,
      }
      pinchStartRef.current = start

      const nextZoom = Math.max(0.2, Math.min(20, start.startZoom * (dist / start.distance)))
      const zoomRatio = nextZoom / start.startZoom
      panRef.current = {
        x: mid.x - (start.startMid.x - start.startPan.x) * zoomRatio,
        y: mid.y - (start.startMid.y - start.startPan.y) * zoomRatio,
      }
      zoomRef.current = nextZoom
      setZoomPct(Math.round(nextZoom * 100))
      notifyTransform()
      draw()
      e.preventDefault()
    }

    const onTouchEnd = (e: TouchEvent) => {
      if (e.touches.length === 0) {
        lastTouchRef.current = null
        pinchStartRef.current = null
        return
      }
      if (e.touches.length === 1) {
        lastTouchRef.current = getTouchPoint(e.touches[0])
        pinchStartRef.current = null
      }
    }

    canvas.addEventListener('touchstart', onTouchStart, { passive: false })
    canvas.addEventListener('touchmove', onTouchMove, { passive: false })
    canvas.addEventListener('touchend', onTouchEnd, { passive: false })
    canvas.addEventListener('touchcancel', onTouchEnd, { passive: false })

    return () => {
      canvas.removeEventListener('touchstart', onTouchStart)
      canvas.removeEventListener('touchmove', onTouchMove)
      canvas.removeEventListener('touchend', onTouchEnd)
      canvas.removeEventListener('touchcancel', onTouchEnd)
    }
  }, [draw, notifyTransform])

  // ── Zoom / reset controls ───────────────────────────────────────────────────

  const rotateBoard = useCallback(() => {
    const next = (rotationRef.current + 90) % 360
    rotationRef.current = next
    setRotation(next)
    draw()
  }, [draw])

  const zoomBy = useCallback((factor: number) => {
    const canvas = canvasRef.current
    if (!canvas) return
    const cx = canvas.clientWidth  / 2
    const cy = canvas.clientHeight / 2
    const newZoom = Math.max(0.2, Math.min(20, zoomRef.current * factor))
    panRef.current.x = cx - (cx - panRef.current.x) * (newZoom / zoomRef.current)
    panRef.current.y = cy - (cy - panRef.current.y) * (newZoom / zoomRef.current)
    zoomRef.current  = newZoom
    setZoomPct(Math.round(newZoom * 100))
    notifyTransform()
    draw()
  }, [draw, notifyTransform])

  const resetView = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas?.parentElement) return
    const w = canvas.parentElement.clientWidth
    const h = canvas.parentElement.clientHeight
    const z = Math.min(w / 1200, h / 800)
    zoomRef.current     = z
    panRef.current      = { x: (w - 1200 * z) / 2, y: (h - 800 * z) / 2 }
    rotationRef.current = 0
    setZoomPct(Math.round(z * 100))
    setRotation(0)
    notifyTransform()
    draw()
  }, [draw, notifyTransform])

  // ── Mouse event handlers ────────────────────────────────────────────────────

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    draggingRef.current    = true
    didMoveRef.current     = false
    lastMouseRef.current   = { x: e.clientX, y: e.clientY }
    setDragging(true)
  }, [])

  const onMouseMove = useCallback((e: React.MouseEvent) => {
    if (draggingRef.current) {
      didMoveRef.current = true
      panRef.current = {
        x: panRef.current.x + e.clientX - lastMouseRef.current.x,
        y: panRef.current.y + e.clientY - lastMouseRef.current.y,
      }
      lastMouseRef.current = { x: e.clientX, y: e.clientY }
      notifyTransform()
      draw()
    }

    // Coordinate readout in board mm
    if (bounds && canvasRef.current) {
      const rect = canvasRef.current.getBoundingClientRect()
      const sx = e.clientX - rect.left
      const sy = e.clientY - rect.top
      const vx = (sx - panRef.current.x) / zoomRef.current
      const vy = (sy - panRef.current.y) / zoomRef.current
      const { minX, maxY, scale: s, offX, offY } = bounds
      setMouseCoords({
        x: (vx - offX) / s + minX,
        y: maxY - (vy - offY) / s,
      })
    }
  }, [bounds, draw, notifyTransform])

  const onMouseUp = useCallback(() => {
    draggingRef.current = false; setDragging(false)
  }, [])

  const onMouseLeave = useCallback(() => {
    draggingRef.current = false
    setDragging(false)
    setMouseCoords(null)
  }, [])

  // ── Click → nearest violation hit test ─────────────────────────────────────

  const handleClick = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!onViolationClick || !bounds) return
    // Don't fire if the mouse moved significantly (was a pan, not a click)
    if (didMoveRef.current) return

    const canvas = canvasRef.current!
    const rect   = canvas.getBoundingClientRect()
    const sx = e.clientX - rect.left
    const sy = e.clientY - rect.top
    const vx = (sx - panRef.current.x) / zoomRef.current
    const vy = (sy - panRef.current.y) / zoomRef.current

    const { minX, maxY, scale: s, offX, offY } = bounds
    const tx = (x: number) => (x - minX) * s + offX
    const ty = (y: number) => (maxY - y) * s + offY

    let best: Violation | null = null
    let bestDist = 14 / zoomRef.current   // 14 screen px → viewBox units
    for (const v of violations) {
      if (!ok(v.x) || !ok(v.y)) continue
      const d = Math.hypot(vx - tx(v.x), vy - ty(v.y))
      if (d < bestDist) { bestDist = d; best = v }
    }
    // Pass null to deselect when clicking empty space
    onViolationClick(best)
  }, [onViolationClick, bounds, violations])

  // ── JSX ─────────────────────────────────────────────────────────────────────

  return (
    <div className="relative flex flex-col h-full bg-gray-900 rounded-lg overflow-hidden">

      {/* Label (e.g. "Rev A (Before)") */}
      {label && (
        <div className="absolute top-2 left-2 z-20 px-2 py-1 bg-black/60 rounded text-xs text-white font-medium select-none">
          {label}
        </div>
      )}

      {/* Toolbar */}
      <div className="absolute top-2 right-2 z-20 flex items-center gap-1">
        <span className="text-xs text-gray-500 font-mono px-1 select-none">{zoomPct}%</span>
        {rotation !== 0 && (
          <span className="text-xs text-gray-500 font-mono px-1 select-none">{rotation}°</span>
        )}
        {[
          { icon: <Layers className="h-5 w-5" />, title: 'Layers', onClick: () => setLayerPanelOpen(o => !o), emphasize: true },
          { icon: <Grid3X3 className="h-5 w-5" />, title: 'Grid', onClick: () => setGridEnabled(g => !g), emphasize: true },
          { icon: <RotateCw className="h-4 w-4" />, title: 'Rotate 90°', onClick: rotateBoard },
          { icon: <ZoomIn className="h-4 w-4" />, title: 'Zoom in', onClick: () => zoomBy(1.3), mobileHidden: true },
          { icon: <ZoomOut className="h-4 w-4" />, title: 'Zoom out', onClick: () => zoomBy(1 / 1.3), mobileHidden: true },
          { icon: <Home className="h-4 w-4" />, title: 'Reset view', onClick: resetView },
        ].map(({ icon, title, onClick, emphasize, mobileHidden }) => (
          <button key={title} onClick={onClick} title={title}
            className={cn(
              'inline-flex items-center justify-center rounded border transition-colors',
              emphasize ? 'p-2' : 'p-1.5',
              mobileHidden && 'hidden md:inline-flex',
              title === 'Grid' && gridEnabled
                ? 'bg-green-900/60 border-green-500/40 text-green-400'
                : 'bg-black/60 backdrop-blur-sm border-white/10 text-gray-400 hover:text-white hover:border-white/30',
            )}>
            {icon}
          </button>
        ))}
      </div>

      {/* Layer panel */}
      {layerPanelOpen && layers.length > 0 && (
        <div
          className="absolute top-2 left-2 z-20 bg-black/70 backdrop-blur-sm border border-white/10 rounded-lg p-2 min-w-[160px] max-h-[80%] overflow-y-auto"
          onClick={() => setOpenDropdownLayer(null)}
        >
          <p className="text-xs text-gray-500 font-semibold mb-1 px-1 uppercase tracking-wider">Layers</p>
          {onSetHiddenLayers && (
            <div className="flex gap-1 mb-2 px-1 flex-wrap">
              {[
                { label: 'All', action: () => onSetHiddenLayers(new Set()) },
                { label: 'None', action: () => onSetHiddenLayers(new Set(layers.map(l => l.name))) },
                { label: 'Top', action: () => {
                  const bottom = new Set(layers.filter(l => {
                    const n = l.name.toLowerCase()
                    return n.includes('bot') || n.includes('back') || n.includes('gbo') || n.includes('gbs') || n.includes('gbl') || n.includes('b.') || n.includes('_bot')
                  }).map(l => l.name))
                  onSetHiddenLayers(bottom)
                }},
                { label: 'Bottom', action: () => {
                  const top = new Set(layers.filter(l => {
                    const n = l.name.toLowerCase()
                    return n.includes('top') || n.includes('front') || n.includes('gto') || n.includes('gts') || n.includes('gtl') || n.includes('f.') || n.includes('_top')
                  }).map(l => l.name))
                  onSetHiddenLayers(top)
                }},
              ].map(({ label, action }) => (
                <button key={label} onClick={action}
                  className="text-[10px] px-1.5 py-0.5 rounded border border-white/15 text-gray-400 hover:text-white hover:bg-white/10 transition-colors">
                  {label}
                </button>
              ))}
            </div>
          )}
          {layers.map((layer) => {
            const color      = getLayerColor(layer.name)
            const hidden     = hiddenLayers.has(layer.name)
            const hasViol    = violationLayers?.has(layer.name) ?? false
            const isOpen     = openDropdownLayer === layer.name
            return (
              <div key={layer.name} className="group relative flex items-center gap-1 w-full rounded hover:bg-white/10 transition-colors">
                <button onClick={() => onToggleLayer(layer.name)}
                  className="flex items-center gap-2 flex-1 min-w-0 px-1 py-0.5 text-left">
                  <div className="w-2.5 h-2.5 rounded-sm flex-shrink-0 border border-white/20 transition-colors"
                    style={{
                      backgroundColor: hidden ? 'transparent' : color,
                      boxShadow: hidden ? 'none' : `0 0 4px ${color}66`,
                    }} />
                  <span className={cn("text-xs truncate transition-colors", hidden ? "text-white/25 group-hover:text-white/70" : "text-[#ccc]")}>
                    {layer.name}
                  </span>
                </button>

                {/* Ignore dropdown trigger — visible on row hover */}
                {hasViol && onIgnoreLayer && (
                  <div className="relative mr-1">
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        setOpenDropdownLayer(isOpen ? null : layer.name)
                      }}
                      title="Ignore violations on this layer"
                      className={cn(
                        'transition-opacity text-[10px] px-1.5 py-0.5 rounded border border-white/10 hover:border-white/30 hover:bg-white/10',
                        isOpen ? 'opacity-100 text-white' : 'opacity-0 group-hover:opacity-100 text-gray-400 hover:text-white'
                      )}
                    >
                      ignore ▾
                    </button>

                    {/* Dropdown menu */}
                    {isOpen && (
                      <div
                        onClick={(e) => e.stopPropagation()}
                        className="absolute right-0 top-full mt-0.5 z-30 bg-gray-900 border border-white/20 rounded shadow-xl py-1 min-w-[150px]"
                      >
                        {[
                          { label: 'Ignore all',      ignored: true,  severity: undefined },
                          { label: 'Ignore errors',   ignored: true,  severity: 'ERROR'   },
                          { label: 'Ignore warnings', ignored: true,  severity: 'WARNING' },
                        ].map(({ label, ignored, severity }) => (
                          <button
                            key={label}
                            onClick={() => {
                              onIgnoreLayer(layer.name, ignored, severity)
                              setOpenDropdownLayer(null)
                            }}
                            className="w-full text-left px-3 py-1.5 text-xs text-gray-300 hover:text-white hover:bg-white/10 transition-colors"
                          >
                            {label}
                          </button>
                        ))}
                        <div className="border-t border-white/10 my-1" />
                        <button
                          onClick={() => {
                            onIgnoreLayer(layer.name, false)
                            setOpenDropdownLayer(null)
                          }}
                          className="w-full text-left px-3 py-1.5 text-xs text-gray-400 hover:text-white hover:bg-white/10 transition-colors"
                        >
                          Restore all
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Legend */}
      <div className="absolute bottom-2 left-2 z-10 hidden md:flex gap-3 bg-black/60 backdrop-blur-sm border border-white/10 rounded-lg px-3 py-1.5">
        <div className="flex items-center gap-1.5">
          <svg width="14" height="14" viewBox="0 0 14 14">
            <polygon points="7,1 13,12 1,12" fill="none" stroke="#ff3333" strokeWidth="1.5" />
          </svg>
          <span className="text-xs font-medium" style={{ color: '#ff3333' }}>ERROR</span>
        </div>
        <div className="flex items-center gap-1.5">
          <svg width="14" height="14" viewBox="0 0 14 14">
            <polygon points="7,1 13,7 7,13 1,7" fill="none" stroke="#ffaa00" strokeWidth="1.5" />
          </svg>
          <span className="text-xs font-medium" style={{ color: '#ffaa00' }}>WARNING</span>
        </div>
        <div className="flex items-center gap-1.5">
          <svg width="14" height="14" viewBox="0 0 14 14">
            <circle cx="7" cy="7" r="5.5" fill="none" stroke="#44aaff" strokeWidth="1.5" />
          </svg>
          <span className="text-xs font-medium" style={{ color: '#44aaff' }}>INFO</span>
        </div>
      </div>

      {/* Coordinate readout */}
      {mouseCoords && (
        <div className="absolute bottom-2 right-2 z-10 font-mono text-xs text-green-400
                        bg-black/70 backdrop-blur-sm border border-white/10 rounded px-2 py-1
                        pointer-events-none select-none">
          X&nbsp;{mouseCoords.x.toFixed(2)}&nbsp;&nbsp;Y&nbsp;{mouseCoords.y.toFixed(2)}&nbsp;&nbsp;mm
        </div>
      )}

      {/* Canvas viewport */}
      <div
        className={cn('flex-1 overflow-hidden', dragging ? 'cursor-grabbing' : 'cursor-crosshair')}
        style={{ minHeight: 0, touchAction: 'none' }}
        onMouseDown={onMouseDown}
        onMouseMove={onMouseMove}
        onMouseUp={onMouseUp}
        onMouseLeave={onMouseLeave}
      >
        <canvas
          ref={canvasRef}
          style={{ display: 'block' }}
          onClick={handleClick}
        />
      </div>
    </div>
  )
}
