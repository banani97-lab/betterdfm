package dfmengine

// mountingHoleMinDiamMM is the smallest non-plated drill treated as a mounting
// hole. M2 clearance holes are ~2.2 mm, so 2.0 mm captures M2 and larger while
// excluding ordinary vias and component pin holes.
const mountingHoleMinDiamMM = 2.0

// MountingHoleKeepoutRule flags copper (traces and pads) that encroaches on the
// keepout ring around a non-plated mounting hole. Screw heads, washers, and
// standoffs sit on that ring; copper inside it risks shorting to the fastener
// or being damaged when the screw is torqued down. Gated by
// profile.MinMountingHoleKeepoutMM (off when <= 0). The keepout is measured
// from the hole edge, so it is independent of the screw size used.
//
// Only non-plated holes >= mountingHoleMinDiamMM are considered; plated holes
// (which sometimes intentionally carry copper for grounding) and small vias are
// excluded. One violation is emitted per mounting hole, reporting the worst
// (smallest) copper gap found.
type MountingHoleKeepoutRule struct{}

func (r *MountingHoleKeepoutRule) ID() string { return "mounting-hole-keepout" }

func (r *MountingHoleKeepoutRule) Run(board BoardData, profile ProfileRules) []Violation {
	keepout := profile.MinMountingHoleKeepoutMM
	if keepout <= 0 {
		return nil
	}

	copperLayers := map[string]bool{}
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			copperLayers[l.Name] = true
		}
	}

	const maxViolations = 500
	var violations []Violation

	for _, hole := range board.Drills {
		if len(violations) >= maxViolations {
			break
		}
		if hole.Plated || hole.DiamMM < mountingHoleMinDiamMM {
			continue
		}
		radius := hole.DiamMM / 2

		worstGap := keepout // only track features inside the keepout
		var found bool
		var wLayer string
		var wx2, wy2 float64

		// Pads.
		for _, pad := range board.Pads {
			if len(copperLayers) > 0 && !copperLayers[pad.Layer] {
				continue
			}
			gap := padEdgeDist(hole.X, hole.Y, pad) - radius
			if gap < worstGap-geomEps {
				worstGap = gap
				found = true
				wLayer = pad.Layer
				cx, cy := padClosestPoint(pad, hole.X, hole.Y)
				wx2, wy2 = cx, cy
			}
		}

		// Traces.
		for _, tr := range board.Traces {
			if len(copperLayers) > 0 && !copperLayers[tr.Layer] {
				continue
			}
			d := ptToSegDist(hole.X, hole.Y, tr.StartX, tr.StartY, tr.EndX, tr.EndY)
			gap := d - tr.WidthMM/2 - radius
			if gap < worstGap-geomEps {
				worstGap = gap
				found = true
				wLayer = tr.Layer
				cx, cy := closestPointOnSeg(hole.X, hole.Y, tr.StartX, tr.StartY, tr.EndX, tr.EndY)
				wx2, wy2 = cx, cy
			}
		}

		if !found {
			continue
		}
		// Attribute to the offending copper layer; fall back to the drill layer.
		layer := wLayer
		if layer == "" {
			layer = hole.Layer
		}
		if layer == "" {
			layer = "drill"
		}
		msg, sug := msgMountingHoleKeepout(hole.DiamMM, worstGap, keepout)
		violations = append(violations, Violation{
			RuleID:     r.ID(),
			Severity:   "WARNING",
			Layer:      layer,
			X:          hole.X,
			Y:          hole.Y,
			Message:    msg,
			Suggestion: sug,
			MeasuredMM: worstGap,
			LimitMM:    keepout,
			Unit:       "mm",
			X2:         wx2,
			Y2:         wy2,
		})
	}

	return dedupeViolations(violations, 2.0)
}
