package dfmengine

import (
	"math"
	"sort"
	"strings"
)

// DrillToCopperRule checks minimum distance from a drill/via hole edge to the
// nearest copper trace or pad. Prevents accidental shorts if the drill wanders.
const maxDrillToCopperViolations = 500

type DrillToCopperRule struct{}

func (r *DrillToCopperRule) ID() string { return "drill-to-copper" }

func (r *DrillToCopperRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinDrillToCopperMM <= 0 {
		return violations
	}

	// Build copper layer set.
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

	// Collect all drills and vias as (x, y, radius, netName) tuples.
	type hole struct {
		x, y, radius float64
		netName      string
	}
	bbox := newBoardBBox(board.Outline, 2.0)
	holes := make([]hole, 0, len(board.Drills)+len(board.Vias))
	for _, d := range board.Drills {
		if !bbox.contains(d.X, d.Y) { continue }
		holes = append(holes, hole{d.X, d.Y, d.DiamMM / 2, ""})
	}
	for _, v := range board.Vias {
		if !bbox.contains(v.X, v.Y) { continue }
		holes = append(holes, hole{v.X, v.Y, v.DrillDiamMM / 2, v.NetName})
	}
	if len(holes) == 0 {
		return violations
	}

	minD := profile.MinDrillToCopperMM

	// Collect copper traces (sorted by minX for sweep).
	type traceEntry struct {
		t          Trace
		minX, maxX float64
		minY, maxY float64
	}
	var copperTraces []traceEntry
	for _, t := range board.Traces {
		if !isCopperLayer(t.Layer) {
			continue
		}
		hw := t.WidthMM / 2
		copperTraces = append(copperTraces, traceEntry{
			t:    t,
			minX: math.Min(t.StartX, t.EndX) - hw,
			maxX: math.Max(t.StartX, t.EndX) + hw,
			minY: math.Min(t.StartY, t.EndY) - hw,
			maxY: math.Max(t.StartY, t.EndY) + hw,
		})
	}
	sort.Slice(copperTraces, func(i, j int) bool { return copperTraces[i].minX < copperTraces[j].minX })

	// Collect copper pads (sorted by X for sweep).
	var copperPads []Pad
	for _, p := range board.Pads {
		if isCopperLayer(p.Layer) {
			copperPads = append(copperPads, p)
		}
	}
	sort.Slice(copperPads, func(i, j int) bool { return copperPads[i].X < copperPads[j].X })

	for _, h := range holes {
		if len(violations) >= maxDrillToCopperViolations {
			break
		}

		reach := h.radius + minD + 1.0 // extra 1mm pad-radius buffer for search

		// Check traces.
		lo := sort.Search(len(copperTraces), func(k int) bool {
			return copperTraces[k].maxX >= h.x-reach
		})
		for k := lo; k < len(copperTraces); k++ {
			if len(violations) >= maxDrillToCopperViolations {
				break
			}
			te := copperTraces[k]
			if te.minX > h.x+reach {
				break
			}
			// Quick Y rejection.
			if te.maxY+reach < h.y || te.minY-reach > h.y {
				continue
			}
			t := te.t
			dist := ptToSegDist(h.x, h.y, t.StartX, t.StartY, t.EndX, t.EndY)
			gap := dist - h.radius - t.WidthMM/2
			if gap < 0 {
				continue // overlapping (annular ring or DRC issue)
			}
			if gap < minD-geomEps {
				msg, sug := msgDrillToCopperBelow(gap, minD)
				violations = append(violations, Violation{
					RuleID:     r.ID(),
					Severity:   "ERROR",
					Layer:      t.Layer,
					X:          h.x,
					Y:          h.y,
					X2:         (t.StartX + t.EndX) / 2,
					Y2:         (t.StartY + t.EndY) / 2,
					Message:    msg,
					Suggestion: sug,
					MeasuredMM: gap,
					LimitMM:    minD,
					Unit:       "mm",
					NetName:    t.NetName,
				})
			}
		}

		// Check pads.
		lo = sort.Search(len(copperPads), func(k int) bool {
			return copperPads[k].X >= h.x-reach
		})
		for k := lo; k < len(copperPads); k++ {
			if len(violations) >= maxDrillToCopperViolations {
				break
			}
			p := copperPads[k]
			padRadius := math.Max(p.WidthMM, p.HeightMM) / 2
			if p.X > h.x+h.radius+minD+padRadius {
				break
			}
			// Skip via's own annular ring pad (same net).
			if h.netName != "" && p.NetName == h.netName {
				continue
			}
			// Quick Y rejection.
			if p.Y+padRadius+h.radius+minD < h.y || p.Y-padRadius-h.radius-minD > h.y {
				continue
			}
			// P2.1: padEdgeDist for shape-aware drill-to-pad gap.
			
			gap := padEdgeDist(h.x, h.y, p) - h.radius
			if gap < 0 {
				continue // overlapping (annular ring pad or DRC issue)
			}
			if gap < minD-geomEps {
				msg, sug := msgDrillToCopperBelow(gap, minD)
				violations = append(violations, Violation{
					RuleID:     r.ID(),
					Severity:   "ERROR",
					Layer:      p.Layer,
					X:          h.x,
					Y:          h.y,
					X2:         p.X,
					Y2:         p.Y,
					Message:    msg,
					Suggestion: sug,
					MeasuredMM: gap,
					LimitMM:    minD,
					Unit:       "mm",
					NetName:    p.NetName,
					RefDes:     p.RefDes,
				})
			}
		}
	}

	return dedupeViolations(violations, 2.0)
}
