package rules

import (
	"fmt"
	"math"
	dfmengine "github.com/betterdfm/dfm-engine"
)

type EdgeClearanceRule struct{}

func NewEdgeClearanceRule() *EdgeClearanceRule { return &EdgeClearanceRule{} }

func (r *EdgeClearanceRule) ID() string { return "edge-clearance" }

func (r *EdgeClearanceRule) Run(board dfmengine.BoardData, profile dfmengine.ProfileRules) []dfmengine.Violation {
	var violations []dfmengine.Violation
	if profile.MinEdgeClearanceMM <= 0 || len(board.Outline) < 2 {
		return violations
	}
	outline := board.Outline
	minDistToOutline := func(px, py float64) float64 {
		minD := math.MaxFloat64
		for i := 0; i < len(outline); i++ {
			a := outline[i]
			b := outline[(i+1)%len(outline)]
			d := pointToSegmentDist(px, py, a.X, a.Y, b.X, b.Y)
			if d < minD {
				minD = d
			}
		}
		return minD
	}
	for _, trace := range board.Traces {
		for _, pt := range [][2]float64{{trace.StartX, trace.StartY}, {trace.EndX, trace.EndY}} {
			dist := minDistToOutline(pt[0], pt[1])
			if dist < profile.MinEdgeClearanceMM {
				violations = append(violations, dfmengine.Violation{
					RuleID:     r.ID(),
					Severity:   "WARNING",
					Layer:      trace.Layer,
					X:          pt[0],
					Y:          pt[1],
					Message:    fmt.Sprintf("Trace is %.4f mm from board edge, below minimum %.4f mm", dist, profile.MinEdgeClearanceMM),
					Suggestion: fmt.Sprintf("Move trace at least %.4f mm away from board edge.", profile.MinEdgeClearanceMM),
				})
			}
		}
	}
	for _, pad := range board.Pads {
		dist := minDistToOutline(pad.X, pad.Y)
		if dist < profile.MinEdgeClearanceMM {
			violations = append(violations, dfmengine.Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    fmt.Sprintf("Pad is %.4f mm from board edge, below minimum %.4f mm", dist, profile.MinEdgeClearanceMM),
				Suggestion: fmt.Sprintf("Move pad at least %.4f mm away from board edge.", profile.MinEdgeClearanceMM),
			})
		}
	}
	return violations
}
