package dfmengine

import "fmt"

// DrillSizeRule checks that all drills/vias are within the allowed diameter range.
type DrillSizeRule struct{}

func (r *DrillSizeRule) ID() string { return "drill-size" }

func (r *DrillSizeRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	checkDiam := func(x, y, diam float64, label string) {
		if profile.MinDrillDiamMM > 0 && diam < profile.MinDrillDiamMM {
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "drill",
				X:          x,
				Y:          y,
				Message:    fmt.Sprintf("%s diameter %.4f mm is below minimum %.4f mm", label, diam, profile.MinDrillDiamMM),
				Suggestion: fmt.Sprintf("Increase %s diameter to at least %.4f mm.", label, profile.MinDrillDiamMM),
				MeasuredMM: diam,
				LimitMM:    profile.MinDrillDiamMM,
				Unit:       "mm",
			})
		}
		if profile.MaxDrillDiamMM > 0 && diam > profile.MaxDrillDiamMM {
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "drill",
				X:          x,
				Y:          y,
				Message:    fmt.Sprintf("%s diameter %.4f mm exceeds maximum %.4f mm", label, diam, profile.MaxDrillDiamMM),
				Suggestion: fmt.Sprintf("Reduce %s diameter to at most %.4f mm.", label, profile.MaxDrillDiamMM),
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
		checkDiam(d.X, d.Y, d.DiamMM, "Drill")
	}
	for _, v := range board.Vias {
		if len(violations) >= maxViol {
			break
		}
		checkDiam(v.X, v.Y, v.DrillDiamMM, "Via drill")
	}
	return violations
}
