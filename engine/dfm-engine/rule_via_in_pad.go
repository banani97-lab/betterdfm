package dfmengine

import (
	"math"
	"strings"
)

// finePitchThresholdMM is the land-to-land pitch at or below which a via-in-pad
// is treated as a hard reflow risk (WARNING) rather than an advisory (INFO).
// 0.5 mm is the conventional fine-pitch / BGA boundary in IPC-7093.
const finePitchThresholdMM = 0.5

// ViaInPadRule flags vias that land inside an SMT component pad. An unfilled,
// uncapped via in a land wicks solder away from the joint during reflow and
// causes opens or insufficient fillets, most dangerously on fine-pitch and BGA
// devices (IPC-7093). The geometry can be detected but fill/cap status cannot,
// so this rule flags for review: WARNING on fine-pitch lands, INFO otherwise.
// Designs that intentionally use filled-and-capped via-in-pad (IPC-4761 Type
// VII) should treat the INFO entries as confirmation items.
//
// Detection relies on the parser tagging a pad with IsViaCatchPad when a drill
// hit falls inside it. A standalone via's own catch pad has no RefDes, so
// requiring a RefDes isolates the via-in-component-land case. Intended
// through-holes (THT pins) are excluded via the SMT mount-type filter and a
// DONUT/HoleMM shape guard so component pin pads are never flagged.
type ViaInPadRule struct{}

func (r *ViaInPadRule) ID() string { return "via-in-pad" }

func (r *ViaInPadRule) Run(board BoardData, _ ProfileRules) []Violation {
	const maxViolations = 500

	outer := outerCopperLayerSet(board.Layers)

	// mountType by RefDes so THT / press-fit pin pads (which legitimately have
	// holes) are not mistaken for via-in-pad.
	mount := map[string]string{}
	for _, c := range board.Components {
		if c.RefDes != "" {
			mount[c.RefDes] = c.MountType
		}
	}

	// Per-RefDes land centers, for estimating pitch (fine-pitch classification).
	landCenters := map[string][]Point{}
	for _, pad := range board.Pads {
		if pad.RefDes == "" || isTestPoint(pad.RefDes) {
			continue
		}
		if len(outer) > 0 && !outer[pad.Layer] {
			continue
		}
		landCenters[pad.RefDes] = append(landCenters[pad.RefDes], Point{X: pad.X, Y: pad.Y})
	}

	var violations []Violation
	for _, pad := range board.Pads {
		if len(violations) >= maxViolations {
			break
		}
		if pad.RefDes == "" || isTestPoint(pad.RefDes) {
			continue
		}
		if !pad.IsViaCatchPad {
			continue
		}
		if len(outer) > 0 && !outer[pad.Layer] {
			continue
		}
		// Exclude intended through-holes: THT/press-fit parts and donut/holed pads.
		if mt, ok := mount[pad.RefDes]; ok && mt != "" && mt != "smt" {
			continue
		}
		if pad.Shape == "DONUT" || pad.HoleMM > 0 {
			continue
		}

		finePitch := isFinePitchClass(pad.PackageClass) ||
			minLandPitch(landCenters[pad.RefDes]) <= finePitchThresholdMM
		sev := "INFO"
		if finePitch {
			sev = "WARNING"
		}
		msg, sug := msgViaInPad(pad.RefDes, finePitch)
		violations = append(violations, Violation{
			RuleID:     r.ID(),
			Severity:   sev,
			Layer:      pad.Layer,
			X:          pad.X,
			Y:          pad.Y,
			Message:    msg,
			Suggestion: sug,
			Unit:       "count",
			NetName:    pad.NetName,
			RefDes:     pad.RefDes,
			Count:      1,
		})
	}
	return violations
}

// isFinePitchClass reports whether a package class denotes a fine-pitch area-array
// device whose via-in-pad joints are reflow-critical.
func isFinePitchClass(pkg string) bool {
	p := strings.ToUpper(pkg)
	return strings.HasPrefix(p, "BGA") || strings.HasPrefix(p, "CSP") || strings.HasPrefix(p, "WLCSP")
}

// minLandPitch returns the smallest center-to-center distance among a
// component's lands, or +Inf when fewer than two lands are present (pitch
// undefined, so the caller falls back to package-class classification).
func minLandPitch(centers []Point) float64 {
	if len(centers) < 2 {
		return math.MaxFloat64
	}
	min := math.MaxFloat64
	for i := 0; i < len(centers); i++ {
		for j := i + 1; j < len(centers); j++ {
			dx := centers[i].X - centers[j].X
			dy := centers[i].Y - centers[j].Y
			if d := math.Sqrt(dx*dx + dy*dy); d < min {
				min = d
			}
		}
	}
	return min
}
