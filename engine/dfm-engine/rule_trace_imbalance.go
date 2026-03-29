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

		w0 := connectedCopperWidth(pads[0], board.Traces, board.Polygons, copperLayers)
		w1 := connectedCopperWidth(pads[1], board.Traces, board.Polygons, copperLayers)

		// Both pads must have at least one copper connection
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
				Severity:   "ERROR",
				Layer:      pads[0].Layer,
				X:          pads[0].X,
				Y:          pads[0].Y,
				X2:         pads[1].X,
				Y2:         pads[1].Y,
				Message:    msgTraceImbalance(refDes, wide, narrow, ratio),
				Suggestion: "Balance copper connections to component pads to reduce tombstoning risk from thermal asymmetry",
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

// connectedCopperWidth returns an effective copper width connected to a pad.
// It checks both traces (by endpoint proximity) and polygons (pad inside pour).
// A pad sitting on a copper polygon returns a large synthetic width to represent
// the thermal mass of the pour — this correctly flags imbalance when one pad
// has a thin trace and the other sits on a ground/power plane.
func connectedCopperWidth(pad Pad, traces []Trace, polygons []Polygon, copperLayers map[string]bool) float64 {
	var maxWidth float64

	// Check traces
	for _, t := range traces {
		if !copperLayers[t.Layer] || t.Layer != pad.Layer {
			continue
		}
		if padEdgeDist(t.StartX, t.StartY, pad) <= 0.01 || padEdgeDist(t.EndX, t.EndY, pad) <= 0.01 {
			if t.WidthMM > maxWidth {
				maxWidth = t.WidthMM
			}
		}
	}

	// Check if pad center is inside a copper polygon on the same layer.
	// A polygon connection represents a large copper pour — use a synthetic
	// width equal to the pad's largest dimension to indicate massive thermal
	// coupling (always larger than a typical trace).
	for _, poly := range polygons {
		if poly.Layer != pad.Layer || !copperLayers[poly.Layer] {
			continue
		}
		if pointInPolygon(pad.X, pad.Y, poly.Points) {
			pourWidth := 10.0 // synthetic large width for copper pour connection
			if pourWidth > maxWidth {
				maxWidth = pourWidth
			}
			break
		}
	}

	return maxWidth
}

