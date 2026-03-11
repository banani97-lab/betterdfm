package dfmengine

import (
	"fmt"
	"math"
)

// EdgeClearanceRule checks that copper features maintain minimum distance from the board edge.
type EdgeClearanceRule struct{}

func (r *EdgeClearanceRule) ID() string { return "edge-clearance" }

func (r *EdgeClearanceRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinEdgeClearanceMM <= 0 || len(board.Outline) < 2 {
		return violations
	}
	minDistToOutline := func(px, py float64) float64 {
		minD := math.MaxFloat64
		n := len(board.Outline)
		for i := 0; i < n; i++ {
			a := board.Outline[i]
			b := board.Outline[(i+1)%n]
			d := ptToSegDist(px, py, a.X, a.Y, b.X, b.Y)
			if d < minD {
				minD = d
			}
		}
		return minD
	}

	// Compute bounding box of the board outline to quickly reject flex-tail
	// features that extend far outside the rigid board region.
	const outsideBBoxBuffer = 5.0 // mm — flex features within 5 mm of bbox are still checked
	var minOX, maxOX, minOY, maxOY float64
	if len(board.Outline) > 0 {
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
	}
	inBBoxRegion := func(x, y float64) bool {
		return x >= minOX-outsideBBoxBuffer && x <= maxOX+outsideBBoxBuffer &&
			y >= minOY-outsideBBoxBuffer && y <= maxOY+outsideBBoxBuffer
	}

	copperLayers := make(map[string]bool, len(board.Layers))
	for _, l := range board.Layers {
		if l.Type == "COPPER" {
			copperLayers[l.Name] = true
		}
	}
	const (
		maxViol    = 2000 // raised — dedup will collapse the final count
		edgeCellMM = 2.0
	)
	for _, trace := range board.Traces {
		if len(violations) >= maxViol {
			break
		}
		if !copperLayers[trace.Layer] {
			continue
		}
		for _, pt := range [2][2]float64{{trace.StartX, trace.StartY}, {trace.EndX, trace.EndY}} {
			if len(violations) >= maxViol {
				break
			}
			// Skip features far outside the board outline bounding box (e.g. flex tails).
			if !inBBoxRegion(pt[0], pt[1]) {
				continue
			}
			dist := minDistToOutline(pt[0], pt[1])
			if dist < profile.MinEdgeClearanceMM {
				violations = append(violations, Violation{
					RuleID:     r.ID(),
					Severity:   "WARNING",
					Layer:      trace.Layer,
					X:          pt[0],
					Y:          pt[1],
					Message:    fmt.Sprintf("Trace is %.4f mm from board edge, below minimum %.4f mm", dist, profile.MinEdgeClearanceMM),
					Suggestion: fmt.Sprintf("Move trace at least %.4f mm away from board edge.", profile.MinEdgeClearanceMM),
					MeasuredMM: dist,
					LimitMM:    profile.MinEdgeClearanceMM,
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
		if !inBBoxRegion(pad.X, pad.Y) {
			continue
		}
		dist := minDistToOutline(pad.X, pad.Y)
		if dist < profile.MinEdgeClearanceMM {
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    fmt.Sprintf("Pad is %.4f mm from board edge, below minimum %.4f mm", dist, profile.MinEdgeClearanceMM),
				Suggestion: fmt.Sprintf("Move pad at least %.4f mm away from board edge.", profile.MinEdgeClearanceMM),
				MeasuredMM: dist,
				LimitMM:    profile.MinEdgeClearanceMM,
				Unit:       "mm",
				NetName:    pad.NetName,
				RefDes:     pad.RefDes,
			})
		}
	}
	return dedupeViolations(violations, edgeCellMM)
}
