package rules

import (
	"fmt"
	"math"
	dfmengine "github.com/betterdfm/dfm-engine"
)

type ClearanceRule struct{}

func NewClearanceRule() *ClearanceRule { return &ClearanceRule{} }

func (r *ClearanceRule) ID() string { return "clearance" }

func (r *ClearanceRule) Run(board dfmengine.BoardData, profile dfmengine.ProfileRules) []dfmengine.Violation {
	var violations []dfmengine.Violation
	if profile.MinClearanceMM <= 0 {
		return violations
	}
	traces := board.Traces
	// Check trace-to-trace clearance
	for i := 0; i < len(traces); i++ {
		for j := i + 1; j < len(traces); j++ {
			a, b := traces[i], traces[j]
			if a.Layer != b.Layer {
				continue
			}
			dist := segmentToSegmentDist(a.StartX, a.StartY, a.EndX, a.EndY,
				b.StartX, b.StartY, b.EndX, b.EndY)
			// subtract half-widths for edge-to-edge distance
			clearance := dist - (a.WidthMM+b.WidthMM)/2
			if clearance < profile.MinClearanceMM {
				mx := (a.StartX + a.EndX) / 2
				my := (a.StartY + a.EndY) / 2
				violations = append(violations, dfmengine.Violation{
					RuleID:     r.ID(),
					Severity:   "ERROR",
					Layer:      a.Layer,
					X:          mx,
					Y:          my,
					Message:    fmt.Sprintf("Trace clearance %.4f mm is below minimum %.4f mm", clearance, profile.MinClearanceMM),
					Suggestion: fmt.Sprintf("Increase spacing between traces to at least %.4f mm.", profile.MinClearanceMM),
				})
			}
		}
	}
	// Check trace-to-pad clearance
	for _, trace := range traces {
		for _, pad := range board.Pads {
			if trace.Layer != pad.Layer {
				continue
			}
			dist := pointToSegmentDist(pad.X, pad.Y, trace.StartX, trace.StartY, trace.EndX, trace.EndY)
			padRadius := math.Max(pad.WidthMM, pad.HeightMM) / 2
			clearance := dist - trace.WidthMM/2 - padRadius
			if clearance < profile.MinClearanceMM {
				violations = append(violations, dfmengine.Violation{
					RuleID:     r.ID(),
					Severity:   "ERROR",
					Layer:      trace.Layer,
					X:          pad.X,
					Y:          pad.Y,
					Message:    fmt.Sprintf("Trace-to-pad clearance %.4f mm is below minimum %.4f mm", clearance, profile.MinClearanceMM),
					Suggestion: fmt.Sprintf("Increase spacing between trace and pad to at least %.4f mm.", profile.MinClearanceMM),
				})
			}
		}
	}
	return violations
}

// pointToSegmentDist returns the distance from point (px,py) to segment (ax,ay)-(bx,by)
func pointToSegmentDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Sqrt((px-ax)*(px-ax) + (py-ay)*(py-ay))
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	nx := ax + t*dx
	ny := ay + t*dy
	return math.Sqrt((px-nx)*(px-nx) + (py-ny)*(py-ny))
}

// segmentToSegmentDist returns minimum distance between two line segments
func segmentToSegmentDist(ax1, ay1, ax2, ay2, bx1, by1, bx2, by2 float64) float64 {
	d1 := pointToSegmentDist(ax1, ay1, bx1, by1, bx2, by2)
	d2 := pointToSegmentDist(ax2, ay2, bx1, by1, bx2, by2)
	d3 := pointToSegmentDist(bx1, by1, ax1, ay1, ax2, ay2)
	d4 := pointToSegmentDist(bx2, by2, ax1, ay1, ax2, ay2)
	return math.Min(math.Min(d1, d2), math.Min(d3, d4))
}
