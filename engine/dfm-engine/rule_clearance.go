package dfmengine

import (
	"math"
	"sort"
)

// ClearanceRule checks trace-to-trace and trace-to-pad minimum clearances.
// At most maxClearanceViolations are reported to prevent OOM on dense boards.
const maxClearanceViolations = 500

// clearanceCellMM is the spatial grid cell size used to deduplicate violations.
// Pairs of copper features within the same cell represent the same structural
// problem (e.g. a copper pour's many segments all too close to the same pads)
// and are collapsed into a single violation showing the worst-case clearance.
const clearanceCellMM = 2.0

type ClearanceRule struct{}

func (r *ClearanceRule) ID() string { return "clearance" }

// traceBB is a trace with its precomputed bounding box.
type traceBB struct {
	t          Trace
	minX, maxX float64
	minY, maxY float64
}

func newTraceBB(t Trace) traceBB {
	hw := t.WidthMM / 2
	return traceBB{
		t:    t,
		minX: math.Min(t.StartX, t.EndX) - hw,
		maxX: math.Max(t.StartX, t.EndX) + hw,
		minY: math.Min(t.StartY, t.EndY) - hw,
		maxY: math.Max(t.StartY, t.EndY) + hw,
	}
}

func (r *ClearanceRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinClearanceMM <= 0 {
		return violations
	}

	// Compute board outline bounding box. Features more than 2 mm outside it
	// are panel-level additions (fiducials, tooling marks, test coupons) that
	// should not be DFM-checked for trace clearance.
	const outlinePanelBuffer = 2.0
	var oMinX, oMaxX, oMinY, oMaxY float64
	if len(board.Outline) > 0 {
		oMinX, oMaxX = board.Outline[0].X, board.Outline[0].X
		oMinY, oMaxY = board.Outline[0].Y, board.Outline[0].Y
		for _, op := range board.Outline[1:] {
			if op.X < oMinX {
				oMinX = op.X
			}
			if op.X > oMaxX {
				oMaxX = op.X
			}
			if op.Y < oMinY {
				oMinY = op.Y
			}
			if op.Y > oMaxY {
				oMaxY = op.Y
			}
		}
	}
	inBoard := func(x, y float64) bool {
		return len(board.Outline) == 0 ||
			(x >= oMinX-outlinePanelBuffer && x <= oMaxX+outlinePanelBuffer &&
				y >= oMinY-outlinePanelBuffer && y <= oMaxY+outlinePanelBuffer)
	}

	// Group traces and pads by layer, excluding panel-level features.
	tracesByLayer := map[string][]traceBB{}
	for _, t := range board.Traces {
		mx := (t.StartX + t.EndX) / 2
		my := (t.StartY + t.EndY) / 2
		if !inBoard(mx, my) {
			continue
		}
		tracesByLayer[t.Layer] = append(tracesByLayer[t.Layer], newTraceBB(t))
	}
	padsByLayer := map[string][]Pad{}
	for _, p := range board.Pads {
		if !inBoard(p.X, p.Y) {
			continue
		}
		padsByLayer[p.Layer] = append(padsByLayer[p.Layer], p)
	}

	minC := profile.MinClearanceMM

	for layer, traces := range tracesByLayer {
		// Sort by minX to enable a sweep-line approach: for each trace we
		// only need to consider later traces whose minX is still within
		// reach (maxX + combined half-widths + minClearance). This turns the
		// O(n²) pair enumeration into O(n·k) where k is the average number of
		// traces in the local neighbourhood — typically very small on real boards.
		sort.Slice(traces, func(i, j int) bool { return traces[i].minX < traces[j].minX })

		for i, a := range traces {
			if len(violations) >= maxClearanceViolations {
				break
			}
			// Any trace j with traces[j].minX > a.maxX + minC cannot be
			// close enough to violate clearance → stop the inner loop.
			xThreshold := a.maxX + minC

			for j := i + 1; j < len(traces); j++ {
				b := traces[j]
				if b.minX > xThreshold {
					break // all remaining are farther in X
				}
				// Quick Y rejection.
				if a.maxY+minC < b.minY || b.maxY+minC < a.minY {
					continue
				}
				// Full segment-to-segment distance.
				if len(violations) >= maxClearanceViolations {
					break
				}
				// Same-net traces are intentionally connected — no clearance check needed.
				if a.t.NetName != "" && a.t.NetName == b.t.NetName {
					continue
				}
				dist := segToSegDist(
					a.t.StartX, a.t.StartY, a.t.EndX, a.t.EndY,
					b.t.StartX, b.t.StartY, b.t.EndX, b.t.EndY,
				)
				clearance := dist - (a.t.WidthMM+b.t.WidthMM)/2
				// Overlapping copper (clearance < 0) is a DRC issue, not DFM.
				// Copper pours, pad stubs, and via rings routinely produce dist=0.
				if clearance < 0 {
					continue
				}
				if clearance < minC {
					msg, sug := msgClearanceTraceTooClose(clearance, minC)
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "ERROR",
						Layer:      layer,
						X:          (a.t.StartX + a.t.EndX) / 2,
						Y:          (a.t.StartY + a.t.EndY) / 2,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: clearance,
						LimitMM:    minC,
						Unit:       "mm",
						NetName:    a.t.NetName,
						X2:         (b.t.StartX + b.t.EndX) / 2,
						Y2:         (b.t.StartY + b.t.EndY) / 2,
					})
				}
			}
		}

		// Trace-to-pad clearance.
		pads := padsByLayer[layer]
		if len(pads) == 0 {
			continue
		}

		// Sort pads by X so we can binary-search into the window per trace.
		sort.Slice(pads, func(i, j int) bool { return pads[i].X < pads[j].X })

		for _, tb := range traces {
			if len(violations) >= maxClearanceViolations {
				break
			}
			t := tb.t
			// Binary search: first pad whose X >= tb.minX - minC - maxPadRadius(≈1mm buffer)
			lo := sort.Search(len(pads), func(k int) bool {
				return pads[k].X >= tb.minX-minC-1.0
			})
			for k := lo; k < len(pads); k++ {
				if len(violations) >= maxClearanceViolations {
					break
				}
				p := pads[k]
				padRadius := math.Max(p.WidthMM, p.HeightMM) / 2
				if p.X > tb.maxX+minC+padRadius {
					break
				}
				// Quick Y rejection.
				if p.Y+padRadius+minC < tb.minY || p.Y-padRadius-minC > tb.maxY {
					continue
				}
				// Same-net trace and pad are intentionally connected — skip.
				if t.NetName != "" && t.NetName == p.NetName {
					continue
				}
				dist := ptToSegDist(p.X, p.Y, t.StartX, t.StartY, t.EndX, t.EndY)
				clearance := dist - t.WidthMM/2 - padRadius
				if clearance < 0 {
					continue // overlapping copper -- DRC issue, not DFM
				}
				if clearance < minC {
					msg, sug := msgClearancePadTooClose(clearance, minC)
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "ERROR",
						Layer:      layer,
						X:          p.X,
						Y:          p.Y,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: clearance,
						LimitMM:    minC,
						Unit:       "mm",
						NetName:    t.NetName,
						X2:         p.X,
						Y2:         p.Y,
					})
				}
			}
		}
	}

	return dedupeViolations(violations, clearanceCellMM)
}

// ptToSegDist returns the minimum distance from point (px,py) to segment (ax,ay)-(bx,by).
func ptToSegDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Sqrt((px-ax)*(px-ax) + (py-ay)*(py-ay))
	}
	t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/(dx*dx+dy*dy)))
	nx, ny := ax+t*dx, ay+t*dy
	return math.Sqrt((px-nx)*(px-nx) + (py-ny)*(py-ny))
}

// segToSegDist returns the minimum distance between two line segments.
func segToSegDist(ax1, ay1, ax2, ay2, bx1, by1, bx2, by2 float64) float64 {
	// Check proper intersection first
	if segsIntersect(ax1, ay1, ax2, ay2, bx1, by1, bx2, by2) {
		return 0
	}
	d1 := ptToSegDist(ax1, ay1, bx1, by1, bx2, by2)
	d2 := ptToSegDist(ax2, ay2, bx1, by1, bx2, by2)
	d3 := ptToSegDist(bx1, by1, ax1, ay1, ax2, ay2)
	d4 := ptToSegDist(bx2, by2, ax1, ay1, ax2, ay2)
	return math.Min(math.Min(d1, d2), math.Min(d3, d4))
}

func segsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	cross2D := func(ox, oy, ux, uy, vx, vy float64) float64 {
		return (ux-ox)*(vy-oy) - (uy-oy)*(vx-ox)
	}
	d1 := cross2D(cx, cy, dx, dy, ax, ay)
	d2 := cross2D(cx, cy, dx, dy, bx, by)
	d3 := cross2D(ax, ay, bx, by, cx, cy)
	d4 := cross2D(ax, ay, bx, by, dx, dy)
	return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))
}
