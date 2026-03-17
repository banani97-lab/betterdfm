package dfmengine

import (
	"math"
	"sort"
)

// DrillToDrillRule checks minimum edge-to-edge distance between any two drill holes
// (including vias). Prevents drill bit deflection and breakage during fabrication.
const maxDrillToDrillViolations = 500

type DrillToDrillRule struct{}

func (r *DrillToDrillRule) ID() string { return "drill-to-drill" }

func (r *DrillToDrillRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinDrillToDrillMM <= 0 {
		return violations
	}

	type hole struct {
		x, y, radius float64
	}

	holes := make([]hole, 0, len(board.Drills)+len(board.Vias))
	for _, d := range board.Drills {
		holes = append(holes, hole{d.X, d.Y, d.DiamMM / 2})
	}
	for _, v := range board.Vias {
		holes = append(holes, hole{v.X, v.Y, v.DrillDiamMM / 2})
	}

	if len(holes) < 2 {
		return violations
	}

	minD := profile.MinDrillToDrillMM

	// Sweep line on X: sort holes by (x - radius) so we can prune early.
	sort.Slice(holes, func(i, j int) bool { return holes[i].x < holes[j].x })

	for i, a := range holes {
		if len(violations) >= maxDrillToDrillViolations {
			break
		}
		// Any hole j whose left edge (x-radius) is beyond a's right edge + minD
		// cannot be close enough.
		xThreshold := a.x + a.radius + minD
		for j := i + 1; j < len(holes); j++ {
			b := holes[j]
			if b.x-b.radius > xThreshold {
				break
			}
			dx, dy := b.x-a.x, b.y-a.y
			dist := math.Sqrt(dx*dx + dy*dy)
			gap := dist - a.radius - b.radius
			if gap < 0 {
				continue // overlapping drills — DRC issue, not DFM
			}
			if gap < minD {
				msg, sug := msgDrillToDrillBelow(gap, minD)
				violations = append(violations, Violation{
					RuleID:     r.ID(),
					Severity:   "ERROR",
					Layer:      "drill",
					X:          a.x,
					Y:          a.y,
					X2:         b.x,
					Y2:         b.y,
					Message:    msg,
					Suggestion: sug,
					MeasuredMM: gap,
					LimitMM:    minD,
					Unit:       "mm",
				})
				if len(violations) >= maxDrillToDrillViolations {
					break
				}
			}
		}
	}

	return dedupeViolations(violations, 2.0)
}
