package dfmengine

// DrillSizeRule checks that all drills/vias are within the allowed diameter range.
type DrillSizeRule struct{}

func (r *DrillSizeRule) ID() string { return "drill-size" }

func (r *DrillSizeRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	bbox := newBoardBBox(board.Outline, 2.0)
	checkDiam := func(x, y, diam float64, label string) {
		if profile.MinDrillDiamMM > 0 && diam < profile.MinDrillDiamMM {
			msg, sug := msgDrillSizeBelow(label, diam, profile.MinDrillDiamMM)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "drill",
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
				Layer:      "drill",
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
		checkDiam(d.X, d.Y, d.DiamMM, "Drill")
	}
	for _, v := range board.Vias {
		if len(violations) >= maxViol {
			break
		}
		if !bbox.contains(v.X, v.Y) {
			continue
		}
		checkDiam(v.X, v.Y, v.DrillDiamMM, "Via drill")
	}
	return violations
}
