package rules

import (
	"fmt"
	dfmengine "github.com/betterdfm/dfm-engine"
)

type AspectRatioRule struct{}

func NewAspectRatioRule() *AspectRatioRule { return &AspectRatioRule{} }

func (r *AspectRatioRule) ID() string { return "aspect-ratio" }

func (r *AspectRatioRule) Run(board dfmengine.BoardData, profile dfmengine.ProfileRules) []dfmengine.Violation {
	var violations []dfmengine.Violation
	if profile.MaxAspectRatio <= 0 || board.BoardThicknessMM <= 0 {
		return violations
	}
	check := func(x, y, diam float64) {
		if diam <= 0 {
			return
		}
		ratio := board.BoardThicknessMM / diam
		if ratio > profile.MaxAspectRatio {
			violations = append(violations, dfmengine.Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      "drill",
				X:          x,
				Y:          y,
				Message:    fmt.Sprintf("Drill aspect ratio %.1f:1 exceeds maximum %.1f:1 (board %.2f mm thick, drill %.4f mm)", ratio, profile.MaxAspectRatio, board.BoardThicknessMM, diam),
				Suggestion: fmt.Sprintf("Increase drill diameter or reduce board thickness. Target aspect ratio <= %.1f:1.", profile.MaxAspectRatio),
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
