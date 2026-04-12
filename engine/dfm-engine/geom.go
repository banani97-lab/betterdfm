package dfmengine

import "math"

// geomEps is the floating-point tolerance used in geometric comparisons.
// Distances within geomEps of a limit are treated as "at the limit" (not a violation)
// to prevent float-drift false positives when a feature is exactly at the rule boundary.
const geomEps = 1e-6

// padEdgeDist returns the distance from point (px, py) to the nearest point on
// the pad's copper surface. Returns 0 when the point is inside or on the pad.
// Shape-aware: CIRCLE uses radius, RECT uses axis-aligned bounding box, OVAL uses
// stadium/capsule geometry. Unknown shapes fall back to the max(W,H)/2 circle.
func padEdgeDist(px, py float64, pad Pad) float64 {
	dx := px - pad.X
	dy := py - pad.Y
	switch pad.Shape {
	case "CIRCLE":
		r := pad.WidthMM / 2
		return math.Max(0, math.Sqrt(dx*dx+dy*dy)-r)
	case "RECT":
		ex := math.Max(0, math.Abs(dx)-pad.WidthMM/2)
		ey := math.Max(0, math.Abs(dy)-pad.HeightMM/2)
		return math.Sqrt(ex*ex + ey*ey)
	case "OVAL":
		// Stadium/capsule: a rectangle with semicircle caps on the two long ends.
		r := math.Min(pad.WidthMM, pad.HeightMM) / 2
		halfLen := math.Max(0, (math.Max(pad.WidthMM, pad.HeightMM)-2*r)/2)
		var d float64
		if pad.WidthMM >= pad.HeightMM {
			// Horizontal capsule axis.
			d = ptToSegDist(px, py, pad.X-halfLen, pad.Y, pad.X+halfLen, pad.Y)
		} else {
			// Vertical capsule axis.
			d = ptToSegDist(px, py, pad.X, pad.Y-halfLen, pad.X, pad.Y+halfLen)
		}
		return math.Max(0, d-r)
	case "POLYGON":
		// P4.2: Use exact polygon contour when available.
		if len(pad.Contour) >= 3 {
			if pointInPolygon(px, py, pad.Contour) {
				return 0
			}
			minD := math.MaxFloat64
			n := len(pad.Contour)
			for i := 0; i < n; i++ {
				a := pad.Contour[i]
				b := pad.Contour[(i+1)%n]
				if d := ptToSegDist(px, py, a.X, a.Y, b.X, b.Y); d < minD {
					minD = d
				}
			}
			return minD
		}
		// Fallback if contour not populated.
		r := math.Max(pad.WidthMM, pad.HeightMM) / 2
		return math.Max(0, math.Sqrt(dx*dx+dy*dy)-r)
	default:
		r := math.Max(pad.WidthMM, pad.HeightMM) / 2
		return math.Max(0, math.Sqrt(dx*dx+dy*dy)-r)
	}
}

// padToPadGap returns the minimum edge-to-edge distance between two pads.
// Returns 0 if the pads overlap. Shape-aware for all common pad types.
func padToPadGap(a, b Pad) float64 {
	// For two RECTs: exact axis-aligned bounding box distance.
	if a.Shape == "RECT" && b.Shape == "RECT" {
		gapX := math.Max(0, math.Max(
			(b.X-b.WidthMM/2)-(a.X+a.WidthMM/2),
			(a.X-a.WidthMM/2)-(b.X+b.WidthMM/2),
		))
		gapY := math.Max(0, math.Max(
			(b.Y-b.HeightMM/2)-(a.Y+a.HeightMM/2),
			(a.Y-a.HeightMM/2)-(b.Y+b.HeightMM/2),
		))
		return math.Sqrt(gapX*gapX + gapY*gapY)
	}
	// For two CIRCLEs: center distance minus both radii.
	if a.Shape == "CIRCLE" && b.Shape == "CIRCLE" {
		d := math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y))
		return math.Max(0, d-a.WidthMM/2-b.WidthMM/2)
	}
	// General case: find the closest point on pad A's surface to pad B's
	// center, then measure from that point to pad B's edge. This is exact
	// for CIRCLE-RECT and a very close approximation for other combos.
	cpx, cpy := padClosestPoint(a, b.X, b.Y)
	return padEdgeDist(cpx, cpy, b)
}

// padClosestPoint returns the point on pad's surface nearest to (px, py).
// If (px, py) is inside the pad, returns (px, py) itself.
func padClosestPoint(pad Pad, px, py float64) (float64, float64) {
	switch pad.Shape {
	case "RECT":
		cx := math.Max(pad.X-pad.WidthMM/2, math.Min(px, pad.X+pad.WidthMM/2))
		cy := math.Max(pad.Y-pad.HeightMM/2, math.Min(py, pad.Y+pad.HeightMM/2))
		return cx, cy
	case "CIRCLE":
		dx, dy := px-pad.X, py-pad.Y
		d := math.Sqrt(dx*dx + dy*dy)
		r := pad.WidthMM / 2
		if d <= r {
			return px, py
		}
		return pad.X + dx/d*r, pad.Y + dy/d*r
	case "OVAL":
		r := math.Min(pad.WidthMM, pad.HeightMM) / 2
		halfLen := math.Max(0, (math.Max(pad.WidthMM, pad.HeightMM)-2*r)/2)
		// Closest point on capsule axis segment
		var sx, sy, ex, ey float64
		if pad.WidthMM >= pad.HeightMM {
			sx, sy = pad.X-halfLen, pad.Y
			ex, ey = pad.X+halfLen, pad.Y
		} else {
			sx, sy = pad.X, pad.Y-halfLen
			ex, ey = pad.X, pad.Y+halfLen
		}
		// Project onto axis segment
		adx, ady := ex-sx, ey-sy
		l2 := adx*adx + ady*ady
		var t float64
		if l2 > 0 {
			t = math.Max(0, math.Min(1, ((px-sx)*adx+(py-sy)*ady)/l2))
		}
		nearX, nearY := sx+t*adx, sy+t*ady
		dx, dy := px-nearX, py-nearY
		d := math.Sqrt(dx*dx + dy*dy)
		if d <= r {
			return px, py
		}
		return nearX + dx/d*r, nearY + dy/d*r
	default:
		// Fallback: treat as circle with max dimension
		r := math.Max(pad.WidthMM, pad.HeightMM) / 2
		dx, dy := px-pad.X, py-pad.Y
		d := math.Sqrt(dx*dx + dy*dy)
		if d <= r {
			return px, py
		}
		return pad.X + dx/d*r, pad.Y + dy/d*r
	}
}

// padProjection returns the pad's half-extent projected onto the unit vector (dx, dy).
// Use this to compute the pad-to-pad gap: gap = centerDist - projA - projB where
// projA = padProjection(padA, bX-aX, bY-aY) and projB = padProjection(padB, aX-bX, aY-bY).
func padProjection(pad Pad, dx, dy float64) float64 {
	l := math.Sqrt(dx*dx + dy*dy)
	if l < geomEps {
		return math.Max(pad.WidthMM, pad.HeightMM) / 2
	}
	ux, uy := dx/l, dy/l
	switch pad.Shape {
	case "CIRCLE":
		return pad.WidthMM / 2
	case "RECT":
		return math.Abs(ux)*pad.WidthMM/2 + math.Abs(uy)*pad.HeightMM/2
	case "OVAL":
		r := math.Min(pad.WidthMM, pad.HeightMM) / 2
		halfLen := math.Max(0, (math.Max(pad.WidthMM, pad.HeightMM)-2*r)/2)
		if pad.WidthMM >= pad.HeightMM {
			return math.Abs(ux)*halfLen + r
		}
		return math.Abs(uy)*halfLen + r
	default:
		return math.Max(pad.WidthMM, pad.HeightMM) / 2
	}
}

// pointInPolygon reports whether (px,py) is inside the given polygon using
// the ray-casting algorithm. The polygon need not be closed (last==first).
func pointInPolygon(px, py float64, pts []Point) bool {
	inside := false
	n := len(pts)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := pts[i].X, pts[i].Y
		xj, yj := pts[j].X, pts[j].Y
		if ((yi > py) != (yj > py)) && (px < (xj-xi)*(py-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

// closestPointOnSeg returns the point on segment (ax,ay)-(bx,by) nearest to (px,py).
func closestPointOnSeg(px, py, ax, ay, bx, by float64) (float64, float64) {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return ax, ay
	}
	t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/(dx*dx+dy*dy)))
	return ax + t*dx, ay + t*dy
}
