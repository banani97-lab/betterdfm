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

// msgClearancePadPairTooClose is the electrical pad-to-pad clearance message.
// Deliberately distinct from solder-mask-dam: that rule flags small gaps
// between any-net pads as a WARNING about the mask web between them; this is
// an ERROR about different-net copper below the fab's electrical clearance.
func msgClearancePadPairTooClose(measured, limit float64) (string, string) {
	return fmt.Sprintf("Pad-to-pad clearance %.4f mm between different nets is below minimum %.4f mm", measured, limit),
		fmt.Sprintf("Increase spacing between the pads to at least %.4f mm.", limit)
}

// msgClearanceShort reports different-net copper that touches or overlaps -
// a probable short, not a spacing issue.
func msgClearanceShort(kind, netA, netB string) (string, string) {
	a, b := netA, netB
	if a == "" {
		a = "?"
	}
	if b == "" {
		b = "?"
	}
	return fmt.Sprintf("Overlapping %s copper on different nets (%s, %s) - probable short", kind, a, b),
		"Separate the copper features; nets in contact will short during fabrication."
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

func msgEdgeClearanceComponentBelow(label, refDes string, measured, limit float64) string {
	if refDes != "" {
		return fmt.Sprintf("%s %s is %.4f mm from board edge, below minimum %.4f mm", label, refDes, measured, limit)
	}
	return fmt.Sprintf("%s is %.4f mm from board edge, below minimum %.4f mm", label, measured, limit)
}

func msgEdgeClearanceDrillBelow(measured, limit, diamMM float64) string {
	return fmt.Sprintf("Drill hole (%.2f mm) is %.4f mm from board edge, below minimum %.4f mm", diamMM, measured, limit)
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

func msgPadUndersizedForPackage(refDes, pkgClass string, measured, expected float64) (string, string) {
	prefix := "Pad"
	if refDes != "" {
		prefix = fmt.Sprintf("Pad on %s", refDes)
	}
	return fmt.Sprintf("%s dimension %.4f mm is below expected minimum %.4f mm for %s package", prefix, measured, expected, pkgClass),
		fmt.Sprintf("Increase pad size to at least %.4f mm per IPC-7351 guidelines for %s.", expected, pkgClass)
}

func msgPadOversizedForPackage(refDes, pkgClass string, measured, expected float64) (string, string) {
	prefix := "Pad"
	if refDes != "" {
		prefix = fmt.Sprintf("Pad on %s", refDes)
	}
	return fmt.Sprintf("%s dimension %.4f mm exceeds expected maximum %.4f mm for %s package", prefix, measured, expected, pkgClass),
		fmt.Sprintf("Reduce pad size to at most %.4f mm for %s to avoid bridging risk.", expected, pkgClass)
}

func msgTombstoningRisk(refDes, pkgClass string, ratio float64) (string, string) {
	return fmt.Sprintf("Component %s (%s) has asymmetric pads with area ratio %.2f:1, risking tombstoning", refDes, pkgClass, ratio),
		"Equalize pad sizes on both ends of the component to reduce tombstoning risk during reflow."
}

func msgPackageCapability(refDes, pkgClass, minClass string) (string, string) {
	return fmt.Sprintf("Component %s uses %s package, but CM minimum is %s", refDes, pkgClass, minClass),
		fmt.Sprintf("Use a larger package size (%s or above) or select a CM with %s capability.", minClass, pkgClass)
}

func msgUnclassifiedComponents(count int) (string, string) {
	return fmt.Sprintf("%d components could not be classified for pad-size checks", count),
		"Package classification is based on ODB++ metadata. Unclassified components are skipped."
}

func msgComponentHeight(refDes, side string, heightMM, limitMM float64) string {
	sideLabel := side
	if sideLabel == "bot" {
		sideLabel = "bottom"
	} else if sideLabel == "" {
		sideLabel = "?"
	}
	return fmt.Sprintf("Component %s on %s side is %.2f mm tall, exceeds %.2f mm limit",
		refDes, sideLabel, heightMM, limitMM)
}

func msgComponentsMissingHeight(count int) string {
	return fmt.Sprintf("%d SMT components skipped (no .comp_height in ODB++ metadata)", count)
}

func msgTraceImbalance(refDes string, wide, narrow, ratio float64) string {
	return fmt.Sprintf("Trace width imbalance on %s: %.3f mm vs %.3f mm (%.1f:1 ratio)", refDes, wide, narrow, ratio)
}

// spacingLabel renders a package class for spacing messages. Empty/unknown
// reads as a plain "component" so messages never show an internal token.
func spacingLabel(ptype string) string {
	switch ptype {
	case "discrete":
		return "discrete"
	case "bga":
		return "BGA"
	case "through_hole":
		return "through-hole"
	case "leaded":
		return "leaded"
	default:
		return ""
	}
}

// refWithType formats "BGA U7" / "discrete C4", or just the refdes when the
// class is unknown.
func refWithType(ref, ptype string) string {
	if lbl := spacingLabel(ptype); lbl != "" {
		return lbl + " " + ref
	}
	return ref
}

func msgComponentSpacing(refA, refB, ptypeA, ptypeB string, measured, limit float64) (string, string) {
	a, b := refWithType(refA, ptypeA), refWithType(refB, ptypeB)
	if measured <= geomEps {
		return fmt.Sprintf("%s and %s overlap (land patterns touch), below minimum spacing %.4f mm", a, b, limit),
			fmt.Sprintf("Separate %s and %s so their land patterns are at least %.4f mm apart for placement and rework access.", refA, refB, limit)
	}
	return fmt.Sprintf("%s and %s are %.4f mm apart, below minimum spacing %.4f mm", a, b, measured, limit),
		fmt.Sprintf("Increase spacing between %s and %s to at least %.4f mm for pick-and-place nozzle and rework access.", refA, refB, limit)
}

func msgMountingHoleKeepout(diamMM, gapMM, limitMM float64) (string, string) {
	if gapMM < 0 {
		return fmt.Sprintf("Copper overlaps the keepout of a %.1f mm mounting hole (needs %.2f mm clearance)", diamMM, limitMM),
			"Pull copper back from the mounting hole so the screw head, washer, or standoff cannot short or damage it."
	}
	return fmt.Sprintf("Copper is %.2f mm from a %.1f mm mounting hole, below the %.2f mm keepout", gapMM, diamMM, limitMM),
		"Pull copper back from the mounting hole so the screw head, washer, or standoff cannot short or damage it."
}

func msgFiducialsCollinear(count int) (string, string) {
	return fmt.Sprintf("All %d global fiducials are nearly collinear, giving poor pick-and-place registration", count),
		"Place at least 3 global fiducials in a wide triangle (e.g. opposite corners plus one offset) for reliable 2D alignment."
}

func msgNoLocalFiducial(refDes string) (string, string) {
	return fmt.Sprintf("Fine-pitch component %s has no fiducial near its land pattern", refDes),
		"Add local fiducials adjacent to fine-pitch / BGA devices for optical alignment per IPC-7351."
}

func msgThroughHoleOnBottom(refDes, mountType string) (string, string) {
	kind := "Through-hole"
	if mountType == "pressfit" {
		kind = "Press-fit"
	}
	return fmt.Sprintf("%s component %s is on the bottom side, which cannot be wave or reflow soldered normally", kind, refDes),
		"Move the part to the top side, or confirm the line supports bottom-side selective/hand soldering for this part."
}

func msgViaInPad(refDes string, finePitch bool) (string, string) {
	if finePitch {
		return fmt.Sprintf("Via-in-pad on fine-pitch component %s: an unfilled via in the land wicks solder during reflow, risking opens", refDes),
			"Fill and cap the via (IPC-4761 Type VII) or relocate it outside the land pattern."
	}
	return fmt.Sprintf("Via-in-pad on %s: confirm the via is filled and capped to prevent solder wicking during reflow", refDes),
		"Confirm via fill/cap (IPC-4761 Type VII) with the fabricator, or relocate the via outside the land."
}

func msgSilkscreenOnPad(refDes string) (string, string) {
	if refDes != "" {
		return fmt.Sprintf("Silkscreen overlaps copper pad for %s", refDes),
			"Move silkscreen features away from exposed copper pads to prevent solderability issues."
	}
	return "Silkscreen overlaps copper pad",
		"Move silkscreen features away from exposed copper pads to prevent solderability issues."
}
