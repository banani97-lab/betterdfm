package dfmengine

import "math"

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
// A pad sitting on a copper polygon returns the polygon's shortest bounding-box
// dimension as a synthetic width, representing the thermal mass of the pour.
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

	// Check if pad is connected to a same-net copper polygon on the same layer.
	// Only same-net polygons can be thermally connected — a GND pour's anti-pad
	// edge near a signal pad does NOT constitute a thermal connection.
	//
	// Prefer polygons the pad sits *inside*. Only fall back to "pad near an
	// edge" proximity when no inside match exists. An adjacent same-net pour
	// whose edge grazes the pad (within 0.5 mm) is rarely a real thermal
	// connection — it's typically the next island of the same net on the
	// other side of a clearance gap. Treating it as equivalent to the pour
	// the pad actually lives in produces false positives (see Dalsa C421:
	// a neighboring 48VIN island 21.5 mm wide outranked the 10.6 mm pour
	// the pad was inside, flagging a 2:1 imbalance that didn't exist).
	padRadius := math.Max(pad.WidthMM, pad.HeightMM) / 2
	proximity := padRadius + 0.5
	var insideW, nearEdgeW float64
	for _, poly := range polygons {
		if poly.Layer != pad.Layer || !copperLayers[poly.Layer] {
			continue
		}
		if poly.NetName == "" || poly.NetName != pad.NetName {
			continue
		}
		if pointInPolygon(pad.X, pad.Y, poly.Points) {
			if s := polyShortDim(poly); s > insideW {
				insideW = s
			}
			continue
		}
		// Skip the edge scan if this pour couldn't improve the running
		// near-edge candidate — avoids the O(segments) loop in the hot path.
		if polyShortDim(poly) <= nearEdgeW {
			continue
		}
		n := len(poly.Points)
		for i := 0; i < n; i++ {
			a := poly.Points[i]
			b := poly.Points[(i+1)%n]
			if ptToSegDist(pad.X, pad.Y, a.X, a.Y, b.X, b.Y) <= proximity {
				if s := polyShortDim(poly); s > nearEdgeW {
					nearEdgeW = s
				}
				break
			}
		}
	}

	// Use the polygon's shortest bbox dimension as equivalent width
	// (a 2 mm routing island and a 200 mm ground plane have very
	// different thermal mass). Inside match wins outright; near-edge
	// is the fallback only when the pad is not inside any pour.
	pourWidth := insideW
	if pourWidth == 0 {
		pourWidth = nearEdgeW
	}
	if pourWidth > maxWidth {
		maxWidth = pourWidth
	}
	return maxWidth
}

// polyShortDim returns the shortest bounding-box dimension of a polygon,
// capped at 50 mm. Used as a proxy for the thermal mass a copper pour
// contributes relative to a thin trace.
func polyShortDim(poly Polygon) float64 {
	if len(poly.Points) < 3 {
		return 1.0
	}
	minX, maxX := poly.Points[0].X, poly.Points[0].X
	minY, maxY := poly.Points[0].Y, poly.Points[0].Y
	for _, p := range poly.Points[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	w := maxX - minX
	h := maxY - minY
	short := math.Min(w, h)
	if short < 0.1 {
		short = 0.1
	}
	if short > 50.0 {
		short = 50.0
	}
	return short
}

