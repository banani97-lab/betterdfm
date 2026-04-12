package dfmengine

// EdgeClearanceRule checks that component pads, drill holes, and fiducials
// maintain minimum distance from the board edge.
type EdgeClearanceRule struct{}

func (r *EdgeClearanceRule) ID() string { return "edge-clearance" }

func (r *EdgeClearanceRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinEdgeClearanceMM <= 0 || len(board.Outline) < 3 {
		return violations
	}

	cellMM := profile.MinEdgeClearanceMM * 2
	allRings := make([][]Point, 1, 1+len(board.OutlineHoles))
	allRings[0] = board.Outline
	allRings = append(allRings, board.OutlineHoles...)
	oidx := newOutlineIndexFromRings(allRings, cellMM)

	// Bounding box for rejecting features far outside the board
	const outsideBBoxBuffer = 5.0
	var minOX, maxOX, minOY, maxOY float64
	minOX, maxOX = board.Outline[0].X, board.Outline[0].X
	minOY, maxOY = board.Outline[0].Y, board.Outline[0].Y
	for _, op := range board.Outline[1:] {
		if op.X < minOX {
			minOX = op.X
		}
		if op.X > maxOX {
			maxOX = op.X
		}
		if op.Y < minOY {
			minOY = op.Y
		}
		if op.Y > maxOY {
			maxOY = op.Y
		}
	}
	inBBoxRegion := func(x, y float64) bool {
		return x >= minOX-outsideBBoxBuffer && x <= maxOX+outsideBBoxBuffer &&
			y >= minOY-outsideBBoxBuffer && y <= maxOY+outsideBBoxBuffer
	}

	copperLayers := make(map[string]bool, len(board.Layers))
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			copperLayers[l.Name] = true
		}
	}

	const (
		maxViol    = 2000
		edgeCellMM = 2.0
	)
	limit := profile.MinEdgeClearanceMM

	// Reject pads that sit on a drill hit — those are via catch-pads, not
	// component pads. The parser stores via catch-pads as Pads on every
	// copper layer, and the refdes spatial lookup tags them with a nearby
	// component's refdes (e.g. mounting holes MH10/MH11), so the RefDes
	// filter below can't distinguish them from real component pads.
	drillSet := newDrillLocationSet(board.Drills)
	const drillCoincidenceTolMM = 0.05

	// Check component pads and fiducials only (skip anonymous pads like pour thermals)
	for _, pad := range board.Pads {
		if len(violations) >= maxViol {
			break
		}
		if !copperLayers[pad.Layer] {
			continue
		}
		// Only check pads that belong to a component or are fiducials
		if pad.RefDes == "" && !pad.IsFiducial {
			continue
		}
		if drillSet.Has(pad.X, pad.Y, drillCoincidenceTolMM) {
			continue
		}
		if !inBBoxRegion(pad.X, pad.Y) {
			continue
		}
		_, cpX, cpY := oidx.minDistWithPoint(pad.X, pad.Y)
		copperEdgeDist := padEdgeDist(cpX, cpY, pad)
		if copperEdgeDist < limit-geomEps {
			label := "Pad"
			if pad.IsFiducial {
				label = "Fiducial"
			}
			msg := msgEdgeClearanceComponentBelow(label, pad.RefDes, copperEdgeDist, limit)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: "Move component or fiducial further from the board edge.",
				MeasuredMM: copperEdgeDist,
				LimitMM:    limit,
				Unit:       "mm",
				NetName:    pad.NetName,
				RefDes:     pad.RefDes,
			})
		}
	}

	// Check drill holes
	for _, drill := range board.Drills {
		if len(violations) >= maxViol {
			break
		}
		if !inBBoxRegion(drill.X, drill.Y) {
			continue
		}
		halfDiam := drill.DiamMM / 2
		edgeDist := oidx.minDist(drill.X, drill.Y) - halfDiam
		if edgeDist < limit-geomEps {
			msg := msgEdgeClearanceDrillBelow(edgeDist, limit, drill.DiamMM)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      "drill",
				X:          drill.X,
				Y:          drill.Y,
				Message:    msg,
				Suggestion: "Move drill hole further from the board edge to prevent breakout.",
				MeasuredMM: edgeDist,
				LimitMM:    limit,
				Unit:       "mm",
			})
		}
	}

	return dedupeViolations(violations, edgeCellMM)
}
