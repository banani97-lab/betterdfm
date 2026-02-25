package rules

import (
	"fmt"
	dfmengine "github.com/betterdfm/dfm-engine"
)

type AnnularRingRule struct{}

func NewAnnularRingRule() *AnnularRingRule { return &AnnularRingRule{} }

func (r *AnnularRingRule) ID() string { return "annular-ring" }

func (r *AnnularRingRule) Run(board dfmengine.BoardData, profile dfmengine.ProfileRules) []dfmengine.Violation {
	var violations []dfmengine.Violation
	if profile.MinAnnularRingMM <= 0 {
		return violations
	}
	for _, via := range board.Vias {
		if via.OuterDiamMM <= 0 || via.DrillDiamMM <= 0 {
			continue
		}
		annularRing := (via.OuterDiamMM - via.DrillDiamMM) / 2
		if annularRing < profile.MinAnnularRingMM {
			violations = append(violations, dfmengine.Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "copper",
				X:          via.X,
				Y:          via.Y,
				Message:    fmt.Sprintf("Annular ring %.4f mm is below minimum %.4f mm", annularRing, profile.MinAnnularRingMM),
				Suggestion: fmt.Sprintf("Increase via pad diameter or reduce drill size to achieve annular ring of at least %.4f mm.", profile.MinAnnularRingMM),
			})
		}
	}
	return violations
}
