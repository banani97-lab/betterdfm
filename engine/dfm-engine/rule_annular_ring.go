package dfmengine

// AnnularRingRule checks that via annular rings meet the minimum requirement.
type AnnularRingRule struct{}

func (r *AnnularRingRule) ID() string { return "annular-ring" }

func (r *AnnularRingRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinAnnularRingMM <= 0 {
		return violations
	}
	const maxViol = 500
	for _, via := range board.Vias {
		if len(violations) >= maxViol {
			break
		}
		if via.OuterDiamMM <= 0 || via.DrillDiamMM <= 0 {
			continue
		}
		annularRing := (via.OuterDiamMM - via.DrillDiamMM) / 2
		if annularRing < profile.MinAnnularRingMM {
			msg, sug := msgAnnularRingBelow(annularRing, profile.MinAnnularRingMM)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "copper",
				X:          via.X,
				Y:          via.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: annularRing,
				LimitMM:    profile.MinAnnularRingMM,
				Unit:       "mm",
			})
		}
	}
	return violations
}
