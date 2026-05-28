package dfmengine

import (
	"fmt"
	"math"
)

// FiducialRule checks that the board has at least 3 fiducial markers
// for pick-and-place machine alignment.
const minFiducials = 3

// fiducialInsetMM is the offset from the board outline bbox corner for a
// suggested fiducial placement. Keeps the suggestion away from the edge
// keepout while remaining at the corner where p&p cameras expect.
const fiducialInsetMM = 5.0

type FiducialRule struct{}

func (r *FiducialRule) ID() string { return "fiducial-count" }

func (r *FiducialRule) Run(board BoardData, _ ProfileRules) []Violation {
	count := 0
	var existing []Point
	for _, p := range board.Pads {
		if p.IsFiducial {
			count++
			existing = append(existing, Point{X: p.X, Y: p.Y})
		}
	}

	// Only run this rule if the parser provided fiducial data (at least one
	// pad tagged). If no fiducials are found in the data, we skip
	// rather than always failing.
	if count == 0 {
		return nil
	}
	if count >= minFiducials {
		return nil
	}

	msg := fmt.Sprintf("Board has %d fiducial(s), minimum %d required for pick-and-place alignment.", count, minFiducials)
	sug := "Add global fiducial markers (typically 1mm round pads with 2-3mm solder mask opening) on at least 3 corners of the board."

	v := Violation{
		RuleID:     r.ID(),
		Severity:   "WARNING",
		Layer:      "",
		X:          0,
		Y:          0,
		Message:    msg,
		Suggestion: sug,
		MeasuredMM: float64(count),
		LimitMM:    float64(minFiducials),
		Unit:       "count",
	}

	// Fix hint: suggest a corner placement furthest from existing
	// fiducials, refined to a free cell via the top-side free-space grid.
	// When the board outline is missing, we skip the hint entirely.
	if len(board.Outline) >= 3 {
		bbox := newBoardBBox(board.Outline, 0)
		if bbox.valid {
			cx := (bbox.minX + bbox.maxX) / 2
			cy := (bbox.minY + bbox.maxY) / 2
			corners := []Point{
				{X: bbox.minX + fiducialInsetMM, Y: bbox.minY + fiducialInsetMM},
				{X: bbox.maxX - fiducialInsetMM, Y: bbox.minY + fiducialInsetMM},
				{X: bbox.maxX - fiducialInsetMM, Y: bbox.maxY - fiducialInsetMM},
				{X: bbox.minX + fiducialInsetMM, Y: bbox.maxY - fiducialInsetMM},
			}
			// Pick the corner whose minimum distance to any existing
			// fiducial is largest — spreads coverage to a fresh corner.
			bestCorner := corners[0]
			bestScore := -1.0
			for _, c := range corners {
				minDist := math.MaxFloat64
				for _, e := range existing {
					d := math.Hypot(c.X-e.X, c.Y-e.Y)
					if d < minDist {
						minDist = d
					}
				}
				if minDist > bestScore {
					bestScore = minDist
					bestCorner = c
				}
			}
			// Refine to a free cell (top-side; through-holes block both).
			grid := buildFreeSpaceGrid(board, "top", freeSpaceCellMM)
			target := bestCorner
			if grid != nil {
				if fx, fy, ok := grid.NearestFree(bestCorner.X, bestCorner.Y, 1.5, 1.5, 8.0); ok {
					target = Point{X: fx, Y: fy}
				}
			}
			setAddHint(&v, cx, cy, target.X, target.Y, "fiducial")
		}
	}

	return []Violation{v}
}
