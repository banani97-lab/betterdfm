package dfmengine

import "fmt"

// TraceWidthRule checks that all traces meet the minimum width requirement.
type TraceWidthRule struct{}

func (r *TraceWidthRule) ID() string { return "trace-width" }

func (r *TraceWidthRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinTraceWidthMM <= 0 {
		return violations
	}
	// Build copper layer set — only routed copper traces are subject to width rules.
	// Non-copper layers (solder mask openings, silk) must never appear here, but
	// guard anyway in case a parser emits them.
	copperLayers := make(map[string]bool, len(board.Layers))
	for _, l := range board.Layers {
		if l.Type == "COPPER" {
			copperLayers[l.Name] = true
		}
	}
	const maxViol = 500
	for _, trace := range board.Traces {
		if len(violations) >= maxViol {
			break
		}
		if !copperLayers[trace.Layer] {
			continue
		}
		if trace.WidthMM > 0 && trace.WidthMM < profile.MinTraceWidthMM {
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      trace.Layer,
				X:          trace.StartX,
				Y:          trace.StartY,
				Message:    fmt.Sprintf("Trace width %.4f mm is below minimum %.4f mm", trace.WidthMM, profile.MinTraceWidthMM),
				Suggestion: fmt.Sprintf("Increase trace width to at least %.4f mm.", profile.MinTraceWidthMM),
				MeasuredMM: trace.WidthMM,
				LimitMM:    profile.MinTraceWidthMM,
				Unit:       "mm",
				NetName:    trace.NetName,
			})
		}
	}
	return violations
}
