package dfmengine

// AspectRatioRule checks that drill aspect ratios do not exceed the maximum.
type AspectRatioRule struct{}

func (r *AspectRatioRule) ID() string { return "aspect-ratio" }

func (r *AspectRatioRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MaxAspectRatio <= 0 || board.BoardThicknessMM <= 0 {
		return violations
	}
	check := func(x, y, diam float64, layer string) {
		if diam <= 0 {
			return
		}
		ratio := board.BoardThicknessMM / diam
		if ratio > profile.MaxAspectRatio {
			msg, sug := msgAspectRatioExceeds(ratio, profile.MaxAspectRatio, board.BoardThicknessMM, diam)
			// Attribute the violation to the actual drill layer (e.g. "D_1_10",
			// "D_5_6") so the BoardViewer can highlight the offending hole on
			// the right layer toggle. Falls back to the legacy "drill" pseudo-
			// layer for older parser output without per-record layer info.
			if layer == "" {
				layer = "drill"
			}
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      layer,
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
	bbox := newBoardBBox(board.Outline, 2.0)
	for _, d := range board.Drills {
		if !bbox.contains(d.X, d.Y) {
			continue
		}
		check(d.X, d.Y, d.DiamMM, d.Layer)
	}
	for _, v := range board.Vias {
		if !bbox.contains(v.X, v.Y) {
			continue
		}
		check(v.X, v.Y, v.DrillDiamMM, v.Layer)
	}
	return violations
}
