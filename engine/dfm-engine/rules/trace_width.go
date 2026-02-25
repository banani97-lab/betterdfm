package rules

import (
	"fmt"
	dfmengine "github.com/betterdfm/dfm-engine"
)

type TraceWidthRule struct{}

func NewTraceWidthRule() *TraceWidthRule { return &TraceWidthRule{} }

func (r *TraceWidthRule) ID() string { return "trace-width" }

func (r *TraceWidthRule) Run(board dfmengine.BoardData, profile dfmengine.ProfileRules) []dfmengine.Violation {
	var violations []dfmengine.Violation
	if profile.MinTraceWidthMM <= 0 {
		return violations
	}
	for _, trace := range board.Traces {
		if trace.WidthMM > 0 && trace.WidthMM < profile.MinTraceWidthMM {
			violations = append(violations, dfmengine.Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      trace.Layer,
				X:          trace.StartX,
				Y:          trace.StartY,
				Message:    fmt.Sprintf("Trace width %.4f mm is below minimum %.4f mm", trace.WidthMM, profile.MinTraceWidthMM),
				Suggestion: fmt.Sprintf("Increase trace width to at least %.4f mm or update the capability profile.", profile.MinTraceWidthMM),
			})
		}
	}
	return violations
}
