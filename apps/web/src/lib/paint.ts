// Paint instruction types for the BoardViewer rendering pipeline.
// These are pure data — no canvas API calls, no React, no dependencies.

export type FillStyle = string
export type StrokeStyle = string
export type CompositeOp = GlobalCompositeOperation

export interface FillRect {
  type: 'fillRect'
  x: number; y: number; w: number; h: number
  fillStyle: FillStyle
  alpha?: number
}

export interface DrawLine {
  type: 'drawLine'
  x1: number; y1: number; x2: number; y2: number
  strokeStyle: StrokeStyle
  lineWidth: number
  alpha?: number
}

export interface DrawCircle {
  type: 'drawCircle'
  cx: number; cy: number; r: number
  fillStyle?: FillStyle
  strokeStyle?: StrokeStyle
  lineWidth?: number
  alpha?: number
}

export interface DrawEllipse {
  type: 'drawEllipse'
  cx: number; cy: number; rx: number; ry: number
  fillStyle?: FillStyle
  alpha?: number
}

export interface DrawPolygon {
  type: 'drawPolygon'
  points: Array<{ x: number; y: number }>
  fillStyle?: FillStyle
  strokeStyle?: StrokeStyle
  lineWidth?: number
  close?: boolean
}

export interface DrawPath {
  type: 'drawPath'
  points: Array<{ x: number; y: number }>
  strokeStyle: StrokeStyle
  lineWidth: number
  alpha?: number
}

export interface DrawViolationMarker {
  type: 'drawViolationMarker'
  cx: number; cy: number; r: number
  color: string
  severity: string
  selected: boolean
  pulseFraction?: number
  // Second-object endpoint for two-point violations
  x2?: number; y2?: number
}

export interface SetComposite {
  type: 'setComposite'
  operation: CompositeOp
}

export type PaintInstruction =
  | FillRect
  | DrawLine
  | DrawCircle
  | DrawEllipse
  | DrawPolygon
  | DrawPath
  | DrawViolationMarker
  | SetComposite
