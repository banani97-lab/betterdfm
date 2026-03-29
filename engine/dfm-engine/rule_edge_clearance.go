package dfmengine

import "math"

// EdgeClearanceRule checks that copper features maintain minimum distance from the board edge.
type EdgeClearanceRule struct{}

func (r *EdgeClearanceRule) ID() string { return "edge-clearance" }

func (r *EdgeClearanceRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	// P2.4: Require at least 3 outline points for a valid closed polygon.
	if profile.MinEdgeClearanceMM <= 0 || len(board.Outline) < 3 {
		return violations
	}

	// Build spatial index for O(1)-ish point-to-outline distance queries.
	// Cell size = 2× the minimum clearance so any violating point is guaranteed
	// to have the nearest outline segment in its 3×3 neighbourhood.
	cellMM := profile.MinEdgeClearanceMM * 2
	// P4.1: Build index from all outline rings: outer boundary + inner cutouts.
	allRings := make([][]Point, 1, 1+len(board.OutlineHoles))
	allRings[0] = board.Outline
	allRings = append(allRings, board.OutlineHoles...)
	oidx := newOutlineIndexFromRings(allRings, cellMM)

	// Compute bounding box of the board outline to quickly reject flex-tail
	// features that extend far outside the rigid board region.
	const outsideBBoxBuffer = 5.0 // mm — flex features within 5 mm of bbox are still checked
	var minOX, maxOX, minOY, maxOY float64
	minOX, maxOX = board.Outline[0].X, board.Outline[0].X
	minOY, maxOY = board.Outline[0].Y, board.Outline[0].Y
	for _, op := range board.Outline[1:] {
		if op.X < minOX {
			minOX = op.X
		}
		if op.X > maxOX {
			maxOX = op.X
		}
		if op.Y < minOY {
			minOY = op.Y
		}
		if op.Y > maxOY {
			maxOY = op.Y
		}
	}
	inBBoxRegion := func(x, y float64) bool {
		return x >= minOX-outsideBBoxBuffer && x <= maxOX+outsideBBoxBuffer &&
			y >= minOY-outsideBBoxBuffer && y <= maxOY+outsideBBoxBuffer
	}

	copperLayers := make(map[string]bool, len(board.Layers))
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			copperLayers[l.Name] = true
		}
	}
	const (
		maxViol    = 2000 // raised — dedup will collapse the final count
		edgeCellMM = 2.0
	)

	limit := profile.MinEdgeClearanceMM

	for _, trace := range board.Traces {
		if len(violations) >= maxViol {
			break
		}
		if !copperLayers[trace.Layer] {
			continue
		}
		half := trace.WidthMM / 2
		// P2.2: Sample N points along the trace at limit/2 intervals so that a
		// trace running parallel and close to the edge is never missed even when
		// both endpoints are far away.
		tdx := trace.EndX - trace.StartX
		tdy := trace.EndY - trace.StartY
		traceLen := math.Sqrt(tdx*tdx + tdy*tdy)
		interval := limit / 2
		if interval < geomEps {
			interval = 0.1
		}
		nSteps := int(traceLen/interval) + 1
		if nSteps < 2 {
			nSteps = 2
		}
		for s := 0; s <= nSteps; s++ {
			if len(violations) >= maxViol {
				break
			}
			frac := float64(s) / float64(nSteps)
			ptX := trace.StartX + frac*tdx
			ptY := trace.StartY + frac*tdy
			if !inBBoxRegion(ptX, ptY) {
				continue
			}
			copperEdgeDist := oidx.minDist(ptX, ptY) - half
			// P2.3: Only flag if strictly below limit (geomEps tolerance).
			if copperEdgeDist < limit-geomEps {
				msg, sug := msgEdgeClearanceTraceBelow(copperEdgeDist, limit)
				violations = append(violations, Violation{
					RuleID:     r.ID(),
					Severity:   "ERROR",
					Layer:      trace.Layer,
					X:          ptX,
					Y:          ptY,
					Message:    msg,
					Suggestion: sug,
					MeasuredMM: copperEdgeDist,
					LimitMM:    limit,
					Unit:       "mm",
					NetName:    trace.NetName,
				})
			}
		}
	}
	for _, pad := range board.Pads {
		if len(violations) >= maxViol {
			break
		}
		if !copperLayers[pad.Layer] {
			continue
		}
		if !inBBoxRegion(pad.X, pad.Y) {
			continue
		}
		// P2.1: Use the closest outline point + padEdgeDist for shape-aware gap.
		_, cpX, cpY := oidx.minDistWithPoint(pad.X, pad.Y)
		copperEdgeDist := padEdgeDist(cpX, cpY, pad)
		// P2.3: Only flag if strictly below limit.
		if copperEdgeDist < limit-geomEps {
			msg, sug := msgEdgeClearancePadBelow(copperEdgeDist, limit)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: copperEdgeDist,
				LimitMM:    limit,
				Unit:       "mm",
				NetName:    pad.NetName,
				RefDes:     pad.RefDes,
			})
		}
	}
	// P3.1: Check copper polygon edges (fills/planes/pours) against board outline.
	for _, poly := range board.Polygons {
		if len(violations) >= maxViol {
			break
		}
		if !copperLayers[poly.Layer] {
			continue
		}
		// Build a list of rings: outer boundary + each hole.
		rings := make([][]Point, 0, 1+len(poly.Holes))
		rings = append(rings, poly.Points)
		rings = append(rings, poly.Holes...)

		for _, ring := range rings {
			if len(violations) >= maxViol {
				break
			}
			n := len(ring)
			if n < 2 {
				continue
			}
			for i := 0; i < n; i++ {
				if len(violations) >= maxViol {
					break
				}
				a := ring[i]
				b := ring[(i+1)%n]
				edx := b.X - a.X
				edy := b.Y - a.Y
				edgeLen := math.Sqrt(edx*edx + edy*edy)
				interval := limit / 2
				if interval < geomEps {
					interval = 0.1
				}
				nSteps := int(edgeLen/interval) + 1
				if nSteps < 2 {
					nSteps = 2
				}
				for s := 0; s <= nSteps; s++ {
					if len(violations) >= maxViol {
						break
					}
					frac := float64(s) / float64(nSteps)
					ptX := a.X + frac*edx
					ptY := a.Y + frac*edy
					if !inBBoxRegion(ptX, ptY) {
						continue
					}
					// Polygon edges have zero copper width — no half-width subtraction.
					copperEdgeDist := oidx.minDist(ptX, ptY)
					if copperEdgeDist < limit-geomEps {
						msg, sug := msgEdgeClearanceTraceBelow(copperEdgeDist, limit)
						violations = append(violations, Violation{
							RuleID:     r.ID(),
							Severity:   "ERROR",
							Layer:      poly.Layer,
							X:          ptX,
							Y:          ptY,
							Message:    msg,
							Suggestion: sug,
							MeasuredMM: copperEdgeDist,
							LimitMM:    limit,
							Unit:       "mm",
							NetName:    poly.NetName,
						})
					}
				}
			}
		}
	}
	return dedupeViolations(violations, edgeCellMM)
}
