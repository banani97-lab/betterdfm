package rules

import (
	"fmt"
	"math"
	dfmengine "github.com/betterdfm/dfm-engine"
)

type SolderMaskDamRule struct{}

func NewSolderMaskDamRule() *SolderMaskDamRule { return &SolderMaskDamRule{} }

func (r *SolderMaskDamRule) ID() string { return "solder-mask-dam" }

func (r *SolderMaskDamRule) Run(board dfmengine.BoardData, profile dfmengine.ProfileRules) []dfmengine.Violation {
	var violations []dfmengine.Violation
	if profile.MinSolderMaskDamMM <= 0 {
		return violations
	}
	pads := board.Pads
	for i := 0; i < len(pads); i++ {
		for j := i + 1; j < len(pads); j++ {
			a, b := pads[i], pads[j]
			if a.Layer != b.Layer {
				continue
			}
			centerDist := math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y))
			aRadius := math.Max(a.WidthMM, a.HeightMM) / 2
			bRadius := math.Max(b.WidthMM, b.HeightMM) / 2
			edgeDist := centerDist - aRadius - bRadius
			if edgeDist < profile.MinSolderMaskDamMM {
				violations = append(violations, dfmengine.Violation{
					RuleID:     r.ID(),
					Severity:   "WARNING",
					Layer:      a.Layer,
					X:          (a.X + b.X) / 2,
					Y:          (a.Y + b.Y) / 2,
					Message:    fmt.Sprintf("Solder mask dam %.4f mm is below minimum %.4f mm between pads", edgeDist, profile.MinSolderMaskDamMM),
					Suggestion: fmt.Sprintf("Increase pad spacing to achieve solder mask dam of at least %.4f mm.", profile.MinSolderMaskDamMM),
				})
			}
		}
	}
	return violations
}
