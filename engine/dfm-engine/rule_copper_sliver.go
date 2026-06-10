package dfmengine

import (
	"math"
	"strings"
)

// CopperSliverRule finds copper too thin to survive fabrication etching:
//
//  1. Un-netted thin trace segments (copper pour artifacts, the original
//     check): traces with no net assignment below minCopperSliverMM.
//  2. Pour webs and necks (P4): two boundary segments of the same copper
//     polygon closer than minCopperSliverMM with copper between them.
//     Outer-ring vs hole and hole vs hole approaches are the classic sliver
//     (a thin copper web between two clearance voids); same-ring approaches
//     are the neck / acid-trap case where the fill folds back on itself.
const maxCopperSliverViolations = 500

// minMeasurableSliverMM floors the pour-boundary check. Approaches narrower
// than ~20µm are not designable copper — they are boundary-encoding artifacts
// (fill seams, self-touching tessellation around thermal spokes) that no CAM
// tool would treat as printable geometry.
const minMeasurableSliverMM = 0.02

type CopperSliverRule struct{}

func (r *CopperSliverRule) ID() string { return "copper-sliver" }

func (r *CopperSliverRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinCopperSliverMM <= 0 {
		return violations
	}

	copperLayers := make(map[string]bool, len(board.Layers))
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			copperLayers[l.Name] = true
		}
	}
	isCopperLayer := func(name string) bool {
		if len(copperLayers) > 0 {
			return copperLayers[name]
		}
		n := strings.ToLower(name)
		return !strings.Contains(n, "silk") && !strings.Contains(n, "legend") &&
			!strings.Contains(n, "overlay") && !strings.Contains(n, "mask") &&
			!strings.Contains(n, "drill") && !strings.Contains(n, "outline") &&
			n != "rout"
	}

	minW := profile.MinCopperSliverMM
	for _, t := range board.Traces {
		if len(violations) >= maxCopperSliverViolations {
			break
		}
		if !isCopperLayer(t.Layer) {
			continue
		}
		// Only flag un-netted copper — intentional signal traces have a net name.
		if t.NetName != "" {
			continue
		}
		if t.WidthMM < minW {
			msg, sug := msgCopperSliver(t.WidthMM, minW)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      t.Layer,
				X:          (t.StartX + t.EndX) / 2,
				Y:          (t.StartY + t.EndY) / 2,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: t.WidthMM,
				LimitMM:    minW,
				Unit:       "mm",
			})
		}
	}

	// P4: pour boundary self-proximity. Slivers are pure geometry, so every
	// copper polygon participates regardless of net labeling.
	for i := range board.Polygons {
		if len(violations) >= maxCopperSliverViolations {
			break
		}
		poly := &board.Polygons[i]
		if !isCopperLayer(poly.Layer) {
			continue
		}
		violations = appendPourSliverViolations(violations, r.ID(), poly, minW)
	}

	return dedupeViolations(violations, 2.0)
}

// sliverEdge is one boundary segment of a pour, tagged with its ring identity
// and its midpoint's cumulative arc length along the ring.
type sliverEdge struct {
	ax, ay, bx, by float64
	ring           int
	arcMid         float64
}

// appendPourSliverViolations scans one polygon's boundary (outer ring +
// holes) for pairs of segments closer than minW with copper between them.
func appendPourSliverViolations(violations []Violation, ruleID string, poly *Polygon, minW float64) []Violation {
	if len(poly.Points) < 3 {
		return violations
	}
	ip := newIndexedPolygon(poly, math.Max(0.5, 2*minW))
	if ip == nil {
		return violations
	}

	var edges []sliverEdge
	var ringLen []float64
	addRing := func(ring []Point, ringID int) {
		n := len(ring)
		arc := 0.0
		for k := 0; k < n; k++ {
			a := ring[k]
			b := ring[(k+1)%n]
			segLen := math.Hypot(b.X-a.X, b.Y-a.Y)
			edges = append(edges, sliverEdge{
				ax: a.X, ay: a.Y, bx: b.X, by: b.Y,
				ring:   ringID,
				arcMid: arc + segLen/2,
			})
			arc += segLen
		}
		ringLen = append(ringLen, arc)
	}
	addRing(poly.Points, 0)
	for _, hole := range poly.Holes {
		if len(hole) >= 3 {
			addRing(hole, len(ringLen))
		}
	}

	// Grid over edge indexes; cell sized to the query radius so a violating
	// pair always shares a cell or sits in adjacent cells.
	cell := math.Max(0.5, 2*minW)
	grid := map[[2]int][]int{}
	for idx := range edges {
		e := &edges[idx]
		cxMin := int(math.Floor(math.Min(e.ax, e.bx) / cell))
		cxMax := int(math.Floor(math.Max(e.ax, e.bx) / cell))
		cyMin := int(math.Floor(math.Min(e.ay, e.by) / cell))
		cyMax := int(math.Floor(math.Max(e.ay, e.by) / cell))
		for cx := cxMin; cx <= cxMax; cx++ {
			for cy := cyMin; cy <= cyMax; cy++ {
				grid[[2]int{cx, cy}] = append(grid[[2]int{cx, cy}], idx)
			}
		}
	}

	// Same-ring adjacency cutoff: neighbors along the boundary are always
	// "close" without enclosing thin copper. Anything within 2×minW of
	// boundary arc length is adjacency (tessellated arc neighbors included);
	// a genuine neck folds back, so its two sides are far apart along the
	// ring even when spatially touching.
	arcCutoff := 2 * minW

	seen := map[[2]int]bool{}
	for idx := range edges {
		if len(violations) >= maxCopperSliverViolations {
			break
		}
		e := &edges[idx]
		cxMin := int(math.Floor((math.Min(e.ax, e.bx) - minW) / cell))
		cxMax := int(math.Floor((math.Max(e.ax, e.bx) + minW) / cell))
		cyMin := int(math.Floor((math.Min(e.ay, e.by) - minW) / cell))
		cyMax := int(math.Floor((math.Max(e.ay, e.by) + minW) / cell))
		for cx := cxMin; cx <= cxMax; cx++ {
			for cy := cyMin; cy <= cyMax; cy++ {
				for _, jdx := range grid[[2]int{cx, cy}] {
					if jdx <= idx || seen[[2]int{idx, jdx}] {
						continue
					}
					seen[[2]int{idx, jdx}] = true
					f := &edges[jdx]
					if e.ring == f.ring {
						arcDist := math.Abs(e.arcMid - f.arcMid)
						if rl := ringLen[e.ring]; arcDist > rl/2 {
							arcDist = rl - arcDist
						}
						if arcDist <= arcCutoff {
							continue
						}
					}
					d := segToSegDist(e.ax, e.ay, e.bx, e.by, f.ax, f.ay, f.bx, f.by)
					if d < minMeasurableSliverMM || d >= minW-geomEps {
						continue
					}
					px, py, qx, qy := segClosestPoints(e.ax, e.ay, e.bx, e.by, f.ax, f.ay, f.bx, f.by)
					mx, my := (px+qx)/2, (py+qy)/2
					// Copper must lie between the two boundary approaches —
					// otherwise the thin thing is the void (a routing slot or
					// notch), not an etchable sliver.
					if !ip.contains(mx, my) {
						continue
					}
					msg, sug := msgCopperSliver(d, minW)
					violations = append(violations, Violation{
						RuleID:     ruleID,
						Severity:   "WARNING",
						Layer:      poly.Layer,
						X:          mx,
						Y:          my,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: d,
						LimitMM:    minW,
						Unit:       "mm",
						NetName:    poly.NetName,
						X2:         qx,
						Y2:         qy,
					})
					if len(violations) >= maxCopperSliverViolations {
						return violations
					}
				}
			}
		}
	}
	return violations
}
