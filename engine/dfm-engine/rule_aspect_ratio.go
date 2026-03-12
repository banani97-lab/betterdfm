package dfmengine

// AspectRatioRule checks that drill aspect ratios do not exceed the maximum.
type AspectRatioRule struct{}

func (r *AspectRatioRule) ID() string { return "aspect-ratio" }

func (r *AspectRatioRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MaxAspectRatio <= 0 || board.BoardThicknessMM <= 0 {
		return violations
	}
	check := func(x, y, diam float64) {
		if diam <= 0 {
			return
		}
		ratio := board.BoardThicknessMM / diam
		if ratio > profile.MaxAspectRatio {
			msg, sug := msgAspectRatioExceeds(ratio, profile.MaxAspectRatio, board.BoardThicknessMM, diam)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      "drill",
				X:          x,
				Y:          y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: ratio,
				LimitMM:    profile.MaxAspectRatio,
				Unit:       "ratio",
			})
		}
	}
	for _, d := range board.Drills {
		check(d.X, d.Y, d.DiamMM)
	}
	for _, v := range board.Vias {
		check(v.X, v.Y, v.DrillDiamMM)
	}
	return violations
}
