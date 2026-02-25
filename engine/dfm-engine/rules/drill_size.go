package rules

import (
	"fmt"
	dfmengine "github.com/betterdfm/dfm-engine"
)

type DrillSizeRule struct{}

func NewDrillSizeRule() *DrillSizeRule { return &DrillSizeRule{} }

func (r *DrillSizeRule) ID() string { return "drill-size" }

func (r *DrillSizeRule) Run(board dfmengine.BoardData, profile dfmengine.ProfileRules) []dfmengine.Violation {
	var violations []dfmengine.Violation
	check := func(x, y, diam float64, label string) {
		if profile.MinDrillDiamMM > 0 && diam < profile.MinDrillDiamMM {
			violations = append(violations, dfmengine.Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "drill",
				X:          x,
				Y:          y,
				Message:    fmt.Sprintf("%s diameter %.4f mm is below minimum %.4f mm", label, diam, profile.MinDrillDiamMM),
				Suggestion: fmt.Sprintf("Increase drill diameter to at least %.4f mm.", profile.MinDrillDiamMM),
			})
		}
		if profile.MaxDrillDiamMM > 0 && diam > profile.MaxDrillDiamMM {
			violations = append(violations, dfmengine.Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "drill",
				X:          x,
				Y:          y,
				Message:    fmt.Sprintf("%s diameter %.4f mm exceeds maximum %.4f mm", label, diam, profile.MaxDrillDiamMM),
				Suggestion: fmt.Sprintf("Reduce drill diameter to at most %.4f mm.", profile.MaxDrillDiamMM),
			})
		}
	}
	for _, d := range board.Drills {
		check(d.X, d.Y, d.DiamMM, "Drill")
	}
	for _, v := range board.Vias {
		check(v.X, v.Y, v.DrillDiamMM, "Via drill")
	}
	return violations
}
