// Canvas renderer: executes PaintInstruction[] against a 2D rendering context.
// All coordinate and state management lives here; boardPainter.ts is canvas-free.

import type { PaintInstruction } from './paint'

export function executeInstructions(
  ctx: CanvasRenderingContext2D,
  instructions: PaintInstruction[],
): void {
  for (const inst of instructions) {
    switch (inst.type) {

      case 'fillRect':
        ctx.globalAlpha = inst.alpha ?? 1
        ctx.fillStyle = inst.fillStyle
        ctx.fillRect(inst.x, inst.y, inst.w, inst.h)
        ctx.globalAlpha = 1
        break

      case 'drawLine':
        ctx.globalAlpha = inst.alpha ?? 1
        ctx.strokeStyle = inst.strokeStyle
        ctx.lineWidth   = inst.lineWidth
        ctx.lineCap     = 'round'
        ctx.lineJoin    = 'round'
        ctx.beginPath()
        ctx.moveTo(inst.x1, inst.y1)
        ctx.lineTo(inst.x2, inst.y2)
        ctx.stroke()
        ctx.globalAlpha = 1
        break

      case 'drawCircle':
        ctx.globalAlpha = inst.alpha ?? 1
        ctx.beginPath()
        ctx.arc(inst.cx, inst.cy, inst.r, 0, Math.PI * 2)
        if (inst.fillStyle)   { ctx.fillStyle   = inst.fillStyle;   ctx.fill() }
        if (inst.strokeStyle) { ctx.strokeStyle = inst.strokeStyle; ctx.lineWidth = inst.lineWidth ?? 1; ctx.stroke() }
        ctx.globalAlpha = 1
        break

      case 'drawEllipse':
        ctx.globalAlpha = inst.alpha ?? 1
        ctx.beginPath()
        ctx.ellipse(inst.cx, inst.cy, inst.rx, inst.ry, 0, 0, Math.PI * 2)
        if (inst.fillStyle) { ctx.fillStyle = inst.fillStyle; ctx.fill() }
        ctx.globalAlpha = 1
        break

      case 'drawPolygon': {
        if (inst.points.length < 2) break
        ctx.beginPath()
        ctx.moveTo(inst.points[0].x, inst.points[0].y)
        for (let i = 1; i < inst.points.length; i++) ctx.lineTo(inst.points[i].x, inst.points[i].y)
        if (inst.close) ctx.closePath()
        for (const hole of inst.holes ?? []) {
          if (hole.length < 2) continue
          ctx.moveTo(hole[0].x, hole[0].y)
          for (let i = 1; i < hole.length; i++) ctx.lineTo(hole[i].x, hole[i].y)
          ctx.closePath()
        }
        if (inst.fillStyle) {
          ctx.fillStyle = inst.fillStyle
          ctx.fill(inst.holes && inst.holes.length > 0 ? 'evenodd' : 'nonzero')
        }
        if (inst.strokeStyle) { ctx.strokeStyle = inst.strokeStyle; ctx.lineWidth = inst.lineWidth ?? 1; ctx.stroke() }
        break
      }

      case 'drawPath':
        if (inst.points.length === 0) break
        ctx.globalAlpha  = inst.alpha ?? 1
        ctx.strokeStyle  = inst.strokeStyle
        ctx.lineWidth    = inst.lineWidth
        ctx.lineCap      = 'round'
        ctx.beginPath()
        ctx.moveTo(inst.points[0].x, inst.points[0].y)
        for (let i = 1; i < inst.points.length; i++) ctx.lineTo(inst.points[i].x, inst.points[i].y)
        ctx.stroke()
        ctx.globalAlpha = 1
        break

      case 'setComposite':
        ctx.globalCompositeOperation = inst.operation
        break

      case 'drawViolationMarker': {
        const { cx, cy, r: baseR, color, severity, selected, pulseFraction, x2, y2 } = inst

        // Dashed line to second object
        if (x2 !== undefined && y2 !== undefined) {
          ctx.save()
          ctx.globalAlpha = 0.4
          ctx.strokeStyle = color
          ctx.lineWidth   = 1
          ctx.setLineDash([4, 3])
          ctx.beginPath(); ctx.moveTo(cx, cy); ctx.lineTo(x2, y2); ctx.stroke()
          ctx.setLineDash([])
          ctx.beginPath(); ctx.arc(x2, y2, 4, 0, Math.PI * 2); ctx.stroke()
          ctx.restore()
        }

        // Pulsing halo + crosshairs for selected violation
        if (selected && pulseFraction !== undefined) {
          const pulseR = baseR * (1.6 + Math.sin(pulseFraction * Math.PI * 2) * 0.8)
          ctx.save()
          ctx.beginPath(); ctx.arc(cx, cy, pulseR, 0, Math.PI * 2)
          ctx.strokeStyle = color + '80'; ctx.lineWidth = 1; ctx.stroke()
          ctx.strokeStyle = color + 'bf'; ctx.lineWidth = 0.8
          ctx.beginPath()
          ctx.moveTo(cx - baseR * 1.9, cy); ctx.lineTo(cx - baseR * 1.2, cy)
          ctx.moveTo(cx + baseR * 1.2, cy); ctx.lineTo(cx + baseR * 1.9, cy)
          ctx.moveTo(cx, cy - baseR * 1.9); ctx.lineTo(cx, cy - baseR * 1.2)
          ctx.moveTo(cx, cy + baseR * 1.2); ctx.lineTo(cx, cy + baseR * 1.9)
          ctx.stroke()
          ctx.restore()
        }

        // Marker shape + label
        ctx.fillStyle   = color + (selected ? '40' : '28')
        ctx.strokeStyle = color
        ctx.lineWidth   = selected ? 1.5 : 1.2
        ctx.beginPath()
        if (severity === 'ERROR') {
          const h = baseR * 1.5
          ctx.moveTo(cx,          cy - h * 0.7)
          ctx.lineTo(cx + baseR,  cy + h * 0.3)
          ctx.lineTo(cx - baseR,  cy + h * 0.3)
          ctx.closePath()
        } else if (severity === 'WARNING') {
          ctx.moveTo(cx,         cy - baseR)
          ctx.lineTo(cx + baseR, cy)
          ctx.lineTo(cx,         cy + baseR)
          ctx.lineTo(cx - baseR, cy)
          ctx.closePath()
        } else {
          ctx.arc(cx, cy, baseR, 0, Math.PI * 2)
        }
        ctx.fill()
        ctx.stroke()

        ctx.fillStyle    = 'white'
        ctx.font         = `bold 6px monospace`
        ctx.textAlign    = 'center'
        ctx.textBaseline = 'middle'
        ctx.fillText(severity[0], cx, cy)
        break
      }
    }
  }

  // Ensure composite is reset to default after rendering
  ctx.globalCompositeOperation = 'source-over'
}
