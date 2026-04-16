package dfmengine

// ComponentHeightRule flags SMT components taller than the profile's per-side
// maximum. CMs enforce these limits because reflow-oven, stencil-printer, and
// wave-soldering clearances are per-side: a 25 mm power inductor on the top
// side is fine if the line can accept it, but the same part on the bottom
// side collides with the wave-solder pallet or reflow conveyor.
//
// Only mountType=="smt" components are checked. Through-hole, press-fit, and
// manual-place parts have different clearance rules that the CM captures
// elsewhere. Components missing .comp_height metadata are counted and
// surfaced in a single INFO entry, mirroring how pad-size-for-package
// reports unclassified components.
type ComponentHeightRule struct{}

func (ComponentHeightRule) ID() string { return "component-height" }

func (ComponentHeightRule) Run(board BoardData, profile ProfileRules) []Violation {
	topLimit := profile.MaxComponentHeightTopMM
	botLimit := profile.MaxComponentHeightBottomMM
	if topLimit <= 0 && botLimit <= 0 {
		return nil
	}

	// Resolve outer solder-mask layer names so the viewer can narrow
	// visibility to the correct side when a violation is focused. Falls
	// back to outer copper when the board has no mask layers at all.
	topMask, botMask := outerSolderMaskLayerNames(board.Layers)
	if topMask == "" || botMask == "" {
		outer := outerCopperLayerSet(board.Layers)
		if topMask == "" {
			for name := range outer {
				if topMask == "" {
					topMask = name
				}
			}
		}
		if botMask == "" {
			for name := range outer {
				botMask = name
			}
		}
	}

	var violations []Violation
	var missingHeight int

	for _, c := range board.Components {
		if c.MountType != "smt" {
			continue
		}
		// Fiducials sometimes inherit a library-default height — ignore
		// them along with other non-assembly refdes classes.
		if isTestPoint(c.RefDes) {
			continue
		}
		if c.HeightMM <= 0 {
			if c.RefDes != "" {
				missingHeight++
			}
			continue
		}

		var limit float64
		var layer string
		switch c.Side {
		case "top":
			limit = topLimit
			layer = topMask
		case "bot":
			limit = botLimit
			layer = botMask
		default:
			// Side couldn't be determined — skip rather than guess.
			continue
		}
		if limit <= 0 {
			continue
		}
		if c.HeightMM <= limit {
			continue
		}
		violations = append(violations, Violation{
			RuleID:     "component-height",
			Severity:   "ERROR",
			Layer:      layer,
			X:          c.X,
			Y:          c.Y,
			Message:    msgComponentHeight(c.RefDes, c.Side, c.HeightMM, limit),
			Suggestion: "Move the part to the other side, replace with a shorter package, or loosen the profile's per-side height limit.",
			MeasuredMM: c.HeightMM,
			LimitMM:    limit,
			Unit:       "mm",
			RefDes:     c.RefDes,
			Count:      1,
		})
	}

	if missingHeight > 0 {
		violations = append(violations, Violation{
			RuleID:   "component-height",
			Severity: "INFO",
			Message:  msgComponentsMissingHeight(missingHeight),
			Suggestion: "ODB++ exports sometimes omit .comp_height for certain packages. Unlisted components are skipped.",
			Unit:     "mm",
		})
	}
	return violations
}
