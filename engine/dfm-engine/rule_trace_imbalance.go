package dfmengine

// TraceImbalanceRule flags 2-pad components where the traces connected to each
// pad differ in width by more than the configured ratio.
type TraceImbalanceRule struct{}

func (TraceImbalanceRule) ID() string { return "trace-imbalance" }

func (TraceImbalanceRule) Run(board BoardData, profile ProfileRules) []Violation {
	maxRatio := profile.MaxTraceImbalanceRatio
	if maxRatio <= 0 {
		return nil
	}

	// Build layer type lookup
	copperLayers := map[string]bool{}
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			copperLayers[l.Name] = true
		}
	}

	// Group pads by RefDes (only components with exactly 2 pads)
	padsByRef := map[string][]Pad{}
	for _, p := range board.Pads {
		if p.RefDes == "" || !copperLayers[p.Layer] {
			continue
		}
		padsByRef[p.RefDes] = append(padsByRef[p.RefDes], p)
	}

	var violations []Violation

	for refDes, pads := range padsByRef {
		if len(pads) != 2 {
			continue
		}

		w0 := widestConnectedTrace(pads[0], board.Traces, copperLayers)
		w1 := widestConnectedTrace(pads[1], board.Traces, copperLayers)

		// Both pads must have at least one connected trace
		if w0 <= 0 || w1 <= 0 {
			continue
		}

		wide := w0
		narrow := w1
		if w1 > w0 {
			wide = w1
			narrow = w0
		}

		ratio := wide / narrow
		if ratio > maxRatio {
			violations = append(violations, Violation{
				RuleID:     "trace-imbalance",
				Severity:   "WARNING",
				Layer:      pads[0].Layer,
				X:          pads[0].X,
				Y:          pads[0].Y,
				X2:         pads[1].X,
				Y2:         pads[1].Y,
				Message:    msgTraceImbalance(refDes, wide, narrow, ratio),
				Suggestion: "Balance trace widths entering component pads to reduce thermal and electrical asymmetry",
				MeasuredMM: ratio,
				LimitMM:    maxRatio,
				Unit:       "ratio",
				RefDes:     refDes,
				NetName:    pads[0].NetName,
				Count:      1,
			})
		}
	}

	return dedupeViolations(violations, 2.0)
}

// widestConnectedTrace finds the widest trace that touches a pad.
// A trace touches a pad if one of its endpoints falls within/on the pad boundary.
func widestConnectedTrace(pad Pad, traces []Trace, copperLayers map[string]bool) float64 {
	var maxWidth float64
	for _, t := range traces {
		if !copperLayers[t.Layer] || t.Layer != pad.Layer {
			continue
		}
		// Check if either endpoint of the trace touches the pad
		if padEdgeDist(t.StartX, t.StartY, pad) <= 0.01 || padEdgeDist(t.EndX, t.EndY, pad) <= 0.01 {
			if t.WidthMM > maxWidth {
				maxWidth = t.WidthMM
			}
		}
	}
	return maxWidth
}
