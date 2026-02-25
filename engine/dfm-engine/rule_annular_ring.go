package dfmengine

import "fmt"

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
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "copper",
				X:          via.X,
				Y:          via.Y,
				Message:    fmt.Sprintf("Annular ring %.4f mm is below minimum %.4f mm", annularRing, profile.MinAnnularRingMM),
				Suggestion: fmt.Sprintf("Increase via pad diameter or reduce drill size to achieve annular ring of at least %.4f mm.", profile.MinAnnularRingMM),
				MeasuredMM: annularRing,
				LimitMM:    profile.MinAnnularRingMM,
				Unit:       "mm",
			})
		}
	}
	return violations
}
