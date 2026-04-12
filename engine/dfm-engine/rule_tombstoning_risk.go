package dfmengine

// smallPassiveClasses are package classes susceptible to tombstoning.
var smallPassiveClasses = map[string]bool{
	"01005": true,
	"0201":  true,
	"0402":  true,
	"0603":  true,
}

// TombstoningRiskRule checks for asymmetric pad sizes on small 2-pad passives
// which can cause tombstoning during reflow soldering.
type TombstoningRiskRule struct{}

func (r *TombstoningRiskRule) ID() string { return "tombstoning-risk" }

func (r *TombstoningRiskRule) Run(board BoardData, _ ProfileRules) []Violation {
	const maxViolations = 500
	const maxRatio = 1.3

	// Only consider component mounting pads (outer copper layers,
	// not coincident with a drill hit — see pad-size-for-package for
	// the full rationale).
	outerLayers := outerCopperLayerSet(board.Layers)
	drillSet := newDrillLocationSet(board.Drills)
	const drillCoincidenceTolMM = 0.05

	// Group pads by RefDes, only for small passive package classes
	type padInfo struct {
		area  float64
		pad   Pad
	}
	type refLayer struct{ ref, layer string }
	groups := map[refLayer][]padInfo{}

	for _, pad := range board.Pads {
		if len(outerLayers) > 0 && !outerLayers[pad.Layer] {
			continue
		}
		if drillSet.Has(pad.X, pad.Y, drillCoincidenceTolMM) {
			continue
		}
		if isTestPoint(pad.RefDes) {
			continue
		}
		if pad.RefDes == "" || !smallPassiveClasses[pad.PackageClass] {
			continue
		}
		area := pad.WidthMM * pad.HeightMM
		key := refLayer{pad.RefDes, pad.Layer}
		groups[key] = append(groups[key], padInfo{area: area, pad: pad})
	}

	flagged := map[string]bool{}
	var violations []Violation
	for key, padInfos := range groups {
		if len(violations) >= maxViolations {
			break
		}

		// Only check 2-pad components, deduplicate across layers
		if len(padInfos) != 2 || flagged[key.ref] {
			continue
		}
		refDes := key.ref

		a1 := padInfos[0].area
		a2 := padInfos[1].area
		if a1 <= 0 || a2 <= 0 {
			continue
		}

		ratio := a1 / a2
		if a2 > a1 {
			ratio = a2 / a1
		}

		if ratio > maxRatio {
			// Use the smaller pad's location for the violation
			vPad := padInfos[0].pad
			if padInfos[1].area < padInfos[0].area {
				vPad = padInfos[1].pad
			}

			flagged[refDes] = true
			msg, sug := msgTombstoningRisk(refDes, vPad.PackageClass, ratio)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      vPad.Layer,
				X:          vPad.X,
				Y:          vPad.Y,
				X2:         padInfos[0].pad.X + padInfos[1].pad.X - vPad.X,
				Y2:         padInfos[0].pad.Y + padInfos[1].pad.Y - vPad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: ratio,
				LimitMM:    maxRatio,
				Unit:       "ratio",
				RefDes:     refDes,
			})
		}
	}

	return violations
}
