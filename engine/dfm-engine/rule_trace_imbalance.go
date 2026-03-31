package dfmengine

import "math"

// TraceImbalanceRule flags 2-pad components where the traces connected to each
// pad differ in width by more than the configured ratio.
type TraceImbalanceRule struct{}

func (TraceImbalanceRule) ID() string { return "trace-imbalance" }

func (TraceImbalanceRule) Run(board BoardData, profile ProfileRules) []Violation {
	if board.SourceFormat == "GERBER" {
		return nil // requires component data (refDes)
	}
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

	// Group pads by RefDes + Layer. A 2-pad component may have pads on multiple
	// layers; we check each layer independently to avoid counting 4+ pads total.
	type refLayer struct{ ref, layer string }
	padsByRefLayer := map[refLayer][]Pad{}
	for _, p := range board.Pads {
		if p.RefDes == "" || !copperLayers[p.Layer] {
			continue
		}
		key := refLayer{p.RefDes, p.Layer}
		padsByRefLayer[key] = append(padsByRefLayer[key], p)
	}

	// Also track which refDes we've already flagged to avoid duplicates across layers
	flagged := map[string]bool{}
	var violations []Violation

	for key, pads := range padsByRefLayer {
		if len(pads) != 2 || flagged[key.ref] {
			continue
		}
		refDes := key.ref

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
			flagged[refDes] = true
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

	// Check if pad is connected to a copper polygon on the same layer.
	// A polygon connection represents a large copper pour — thermal relief
	// patterns mean the pad center may not be inside the polygon, so we also
	// check if any polygon edge passes near the pad (within pad radius + 0.5mm).
	padRadius := math.Max(pad.WidthMM, pad.HeightMM) / 2
	proximity := padRadius + 0.5 // thermal relief spokes are typically within this range
	for _, poly := range polygons {
		if poly.Layer != pad.Layer || !copperLayers[poly.Layer] {
			continue
		}
		connected := pointInPolygon(pad.X, pad.Y, poly.Points)
		if !connected {
			// Check if any polygon edge is near the pad
			n := len(poly.Points)
			for i := 0; i < n && !connected; i++ {
				a := poly.Points[i]
				b := poly.Points[(i+1)%n]
				if ptToSegDist(pad.X, pad.Y, a.X, a.Y, b.X, b.Y) <= proximity {
					connected = true
				}
			}
		}
		if connected {
			pourWidth := 10.0
			if pourWidth > maxWidth {
				maxWidth = pourWidth
			}
			break
		}
	}

	return maxWidth
}

