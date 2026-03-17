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
