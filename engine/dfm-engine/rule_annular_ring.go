package dfmengine

import "math"

// AnnularRingRule checks that the copper catch-pad around each plated hole has
// at least MinAnnularRingMM of copper on every layer it passes through.
//
// Modelling note: a through-hole consists of one drill bit (diameter from the
// Drill record) and N catch-pads — one per copper layer it intersects, each
// captured as a Pad with IsViaCatchPad=true. Inner-layer catch-pads are often
// smaller than top/bottom cover pads, so the rule must check every layer
// independently — checking only the largest pad would silently miss
// inner-layer violations and pass boards that aren't actually manufacturable.
//
// The previous implementation iterated Vias and relied on the parser
// emitting one Via per copper layer; the rewrite uses per-layer Pad records
// directly, which is also what the renderer uses, so both views stay in sync.
type AnnularRingRule struct{}

func (r *AnnularRingRule) ID() string { return "annular-ring" }

func (r *AnnularRingRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinAnnularRingMM <= 0 {
		return violations
	}
	const maxViol = 500
	bbox := newBoardBBox(board.Outline, 2.0)

	// Index drills by 1µm-rounded (x,y) for O(1) catch-pad lookup. 1µm is
	// well below the resolution of any DFM check; pad coords are rounded
	// the same way so colocation is exact.
	type key struct{ x, y int64 }
	keyOf := func(x, y float64) key {
		return key{int64(math.Round(x * 1000)), int64(math.Round(y * 1000))}
	}
	drillByXY := make(map[key]float64, len(board.Drills))
	for _, d := range board.Drills {
		if d.DiamMM <= 0 {
			continue
		}
		k := keyOf(d.X, d.Y)
		// If multiple drill records share a location (e.g. through-hole +
		// co-located microvia), keep the larger diameter — annular ring is
		// most strained against the bigger drill.
		if prev, ok := drillByXY[k]; !ok || d.DiamMM > prev {
			drillByXY[k] = d.DiamMM
		}
	}

	// For pads with no colocated drill (parser didn't tag the drill, or it
	// was below the 50µm filter), fall back to the Via record's drill
	// diameter at the same location.
	viaByXY := make(map[key]float64, len(board.Vias))
	for _, v := range board.Vias {
		if v.DrillDiamMM <= 0 {
			continue
		}
		k := keyOf(v.X, v.Y)
		if prev, ok := viaByXY[k]; !ok || v.DrillDiamMM > prev {
			viaByXY[k] = v.DrillDiamMM
		}
	}

	// catchPadOuter returns the smaller cross-section of a catch-pad — the
	// dimension that limits annular ring. For DONUT and CIRCLE shapes
	// w == h, so this is just the diameter.
	catchPadOuter := func(p Pad) float64 {
		if p.Shape == "DONUT" || p.Shape == "CIRCLE" {
			return p.WidthMM
		}
		return math.Min(p.WidthMM, p.HeightMM)
	}

	// Dedupe by (layer, drill location) — the parser can attach >1 catch-pad
	// to the same via on the same layer (e.g. a coverlay relief and the
	// underlying copper pad both tagged), and we only want one violation
	// per layer per via.
	type seenKey struct {
		layer string
		k     key
	}
	seen := make(map[seenKey]bool)

	for _, p := range board.Pads {
		if len(violations) >= maxViol {
			break
		}
		if !p.IsViaCatchPad {
			continue
		}
		if !bbox.contains(p.X, p.Y) {
			continue
		}
		k := keyOf(p.X, p.Y)
		drillDiam, ok := drillByXY[k]
		if !ok {
			drillDiam, ok = viaByXY[k]
		}
		if !ok || drillDiam <= 0 {
			continue
		}
		outer := catchPadOuter(p)
		if outer <= 0 || outer <= drillDiam {
			// outer <= drill is a DRC issue (overlapping) — skip rather
			// than report a negative annular ring.
			continue
		}
		annularRing := (outer - drillDiam) / 2
		if annularRing >= profile.MinAnnularRingMM {
			continue
		}
		sk := seenKey{p.Layer, k}
		if seen[sk] {
			continue
		}
		seen[sk] = true

		msg, sug := msgAnnularRingBelow(annularRing, profile.MinAnnularRingMM)
		violations = append(violations, Violation{
			RuleID:     r.ID(),
			Severity:   "ERROR",
			Layer:      p.Layer,
			X:          p.X,
			Y:          p.Y,
			Message:    msg,
			Suggestion: sug,
			MeasuredMM: annularRing,
			LimitMM:    profile.MinAnnularRingMM,
			Unit:       "mm",
			NetName:    p.NetName,
		})
	}

	// Fallback: vias whose catch-pads the parser never tagged at all
	// (boards with no copper-layer pad records — rare, but possible for
	// drill-only test fixtures). Check each Via's outer/hole directly so
	// we don't silently pass these boards.
	covered := make(map[key]bool, len(seen))
	for _, p := range board.Pads {
		if p.IsViaCatchPad {
			covered[keyOf(p.X, p.Y)] = true
		}
	}
	for _, via := range board.Vias {
		if len(violations) >= maxViol {
			break
		}
		if !bbox.contains(via.X, via.Y) {
			continue
		}
		if via.OuterDiamMM <= 0 || via.DrillDiamMM <= 0 {
			continue
		}
		k := keyOf(via.X, via.Y)
		if covered[k] {
			continue
		}
		annularRing := (via.OuterDiamMM - via.DrillDiamMM) / 2
		if annularRing >= profile.MinAnnularRingMM {
			continue
		}
		msg, sug := msgAnnularRingBelow(annularRing, profile.MinAnnularRingMM)
		// Prefer the via's drill-layer attribution so the viewer can highlight
		// the offending span (e.g. D_1_10). Falls back to the legacy "copper"
		// pseudo-layer for boards without per-record layer info.
		layer := via.Layer
		if layer == "" {
			layer = "copper"
		}
		violations = append(violations, Violation{
			RuleID:     r.ID(),
			Severity:   "ERROR",
			Layer:      layer,
			X:          via.X,
			Y:          via.Y,
			Message:    msg,
			Suggestion: sug,
			MeasuredMM: annularRing,
			LimitMM:    profile.MinAnnularRingMM,
			Unit:       "mm",
			NetName:    via.NetName,
		})
	}
	return violations
}
