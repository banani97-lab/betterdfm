package dfmengine

import "math"

// dedupeViolations collapses violations that fall within the same spatial grid
// cell (keyed by layer + rule + grid coords) into a single representative
// violation. The one with the worst (smallest) MeasuredMM is kept; Count is set
// to the number of raw violations merged into that cell.
//
// This is used by rules that check individual features (traces, pads) where a
// copper pour or dense cluster produces hundreds of identical-location violations
// that all represent the same underlying design problem.
func dedupeViolations(violations []Violation, cellMM float64) []Violation {
	type cellKey struct {
		layer, ruleID string
		cx, cy        int
	}

	type cellBest struct {
		v     Violation
		count int
	}

	cells := make(map[cellKey]*cellBest, len(violations))
	for _, v := range violations {
		key := cellKey{
			layer:  v.Layer,
			ruleID: v.RuleID,
			cx:     int(math.Floor(v.X / cellMM)),
			cy:     int(math.Floor(v.Y / cellMM)),
		}
		if b, ok := cells[key]; !ok {
			cells[key] = &cellBest{v: v, count: 1}
		} else {
			b.count++
			if v.MeasuredMM < b.v.MeasuredMM {
				b.v = v
			}
		}
	}

	result := make([]Violation, 0, len(cells))
	for _, b := range cells {
		b.v.Count = b.count
		result = append(result, b.v)
	}
	return result
}
