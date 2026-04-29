package dfmengine

// DrillSizeRule checks that all drills/vias are within the allowed diameter range.
type DrillSizeRule struct{}

func (r *DrillSizeRule) ID() string { return "drill-size" }

func (r *DrillSizeRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	bbox := newBoardBBox(board.Outline, 2.0)
	// Attribute violations to the actual drill layer (e.g. "D_1_10") so the
	// viewer's layer toggle can highlight only the offending span. Empty
	// Layer falls back to the legacy "drill" pseudo-layer for boards parsed
	// before the per-record attribution was added.
	checkDiam := func(x, y, diam float64, label, layer string) {
		if layer == "" {
			layer = "drill"
		}
		if profile.MinDrillDiamMM > 0 && diam < profile.MinDrillDiamMM {
			msg, sug := msgDrillSizeBelow(label, diam, profile.MinDrillDiamMM)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      layer,
				X:          x,
				Y:          y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: diam,
				LimitMM:    profile.MinDrillDiamMM,
				Unit:       "mm",
			})
		}
		if profile.MaxDrillDiamMM > 0 && diam > profile.MaxDrillDiamMM {
			msg, sug := msgDrillSizeAbove(label, diam, profile.MaxDrillDiamMM)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      layer,
				X:          x,
				Y:          y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: diam,
				LimitMM:    profile.MaxDrillDiamMM,
				Unit:       "mm",
			})
		}
	}
	const maxViol = 500
	for _, d := range board.Drills {
		if len(violations) >= maxViol {
			break
		}
		if !bbox.contains(d.X, d.Y) {
			continue
		}
		checkDiam(d.X, d.Y, d.DiamMM, "Drill", d.Layer)
	}
	for _, v := range board.Vias {
		if len(violations) >= maxViol {
			break
		}
		if !bbox.contains(v.X, v.Y) {
			continue
		}
		checkDiam(v.X, v.Y, v.DrillDiamMM, "Via drill", v.Layer)
	}
	return violations
}
