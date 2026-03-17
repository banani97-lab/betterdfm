package dfmengine

import "fmt"

func msgTraceWidthBelow(measured, limit float64) (string, string) {
	return fmt.Sprintf("Trace width %.4f mm is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Increase trace width to at least %.4f mm.", limit)
}

func msgClearanceTraceTooClose(measured, limit float64) (string, string) {
	return fmt.Sprintf("Trace-to-trace clearance %.4f mm is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Increase spacing between traces to at least %.4f mm.", limit)
}

func msgClearancePadTooClose(measured, limit float64) (string, string) {
	return fmt.Sprintf("Trace-to-pad clearance %.4f mm is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Increase spacing between trace and pad to at least %.4f mm.", limit)
}

func msgDrillSizeBelow(label string, measured, limit float64) (string, string) {
	return fmt.Sprintf("%s diameter %.4f mm is below minimum %.4f mm", label, measured, limit),
		fmt.Sprintf("Increase %s diameter to at least %.4f mm.", label, limit)
}

func msgDrillSizeAbove(label string, measured, limit float64) (string, string) {
	return fmt.Sprintf("%s diameter %.4f mm exceeds maximum %.4f mm", label, measured, limit),
		fmt.Sprintf("Reduce %s diameter to at most %.4f mm.", label, limit)
}

func msgAnnularRingBelow(measured, limit float64) (string, string) {
	return fmt.Sprintf("Annular ring %.4f mm is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Increase via pad diameter or reduce drill size to achieve annular ring of at least %.4f mm.", limit)
}

func msgAspectRatioExceeds(ratio, maxRatio, boardThickness, drillDiam float64) (string, string) {
	return fmt.Sprintf("Drill aspect ratio %.1f:1 exceeds maximum %.1f:1 (board %.2f mm, drill %.4f mm)",
			ratio, maxRatio, boardThickness, drillDiam),
		fmt.Sprintf("Increase drill diameter or reduce board thickness. Target aspect ratio ≤ %.1f:1.", maxRatio)
}

func msgSolderMaskDamBelow(measured, limit float64) (string, string) {
	return fmt.Sprintf("Solder mask dam %.4f mm is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Increase pad spacing to achieve solder mask dam of at least %.4f mm.", limit)
}

func msgEdgeClearanceTraceBelow(measured, limit float64) (string, string) {
	return fmt.Sprintf("Trace is %.4f mm from board edge, below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Move trace at least %.4f mm away from board edge.", limit)
}

func msgEdgeClearancePadBelow(measured, limit float64) (string, string) {
	return fmt.Sprintf("Pad is %.4f mm from board edge, below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Move pad at least %.4f mm away from board edge.", limit)
}

func msgDrillToDrillBelow(measured, limit float64) (string, string) {
	return fmt.Sprintf("Drill-to-drill clearance %.4f mm is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Increase spacing between holes to at least %.4f mm edge-to-edge.", limit)
}

func msgDrillToCopperBelow(measured, limit float64) (string, string) {
	return fmt.Sprintf("Drill-to-copper clearance %.4f mm is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Move copper feature at least %.4f mm from the drill hole edge.", limit)
}

func msgCopperSliver(measured, limit float64) (string, string) {
	return fmt.Sprintf("Copper sliver %.4f mm wide is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Remove or merge copper slivers thinner than %.4f mm.", limit)
}

func msgSilkscreenOnPad(refDes string) (string, string) {
	if refDes != "" {
		return fmt.Sprintf("Silkscreen overlaps copper pad for %s", refDes),
			"Move silkscreen features away from exposed copper pads to prevent solderability issues."
	}
	return "Silkscreen overlaps copper pad",
		"Move silkscreen features away from exposed copper pads to prevent solderability issues."
}
