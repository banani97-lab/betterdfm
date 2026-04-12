package dfmengine

import (
	"math"
	"strings"
)

// SilkscreenOnPadRule detects silkscreen features that overlap exposed copper pads.
// Silk ink on a copper pad degrades solderability (ink doesn't wet solder).
const maxSilkscreenOnPadViolations = 500

type SilkscreenOnPadRule struct{}

func (r *SilkscreenOnPadRule) ID() string { return "silkscreen-on-pad" }

// bb is a 2D axis-aligned bounding box.
type bb struct {
	minX, maxX, minY, maxY float64
}

func (a bb) overlaps(b bb) bool {
	return a.minX <= b.maxX && b.minX <= a.maxX &&
		a.minY <= b.maxY && b.minY <= a.maxY
}

// boardSide returns "top", "bot", or "" (unknown) for a layer name.
func boardSide(layer string) string {
	n := strings.ToLower(layer)
	if strings.Contains(n, "top") || strings.Contains(n, "comp") || strings.Contains(n, "front") {
		return "top"
	}
	if strings.Contains(n, "bot") || strings.Contains(n, "bottom") || strings.Contains(n, "back") ||
		strings.Contains(n, "sol") {
		return "bot"
	}
	return ""
}

func sameSide(layerA, layerB string) bool {
	sa, sb := boardSide(layerA), boardSide(layerB)
	return sa == "" || sb == "" || sa == sb
}

func (r *SilkscreenOnPadRule) Run(board BoardData, profile ProfileRules) []Violation {
	if profile.EnableSilkscreenOnPadCheck != nil && !*profile.EnableSilkscreenOnPadCheck {
		return nil
	}

	var violations []Violation

	// Build silk layer and copper layer sets from metadata.
	silkLayers := make(map[string]bool, len(board.Layers))
	copperLayers := make(map[string]bool, len(board.Layers))
	for _, l := range board.Layers {
		switch l.Type {
		case "SILK":
			silkLayers[l.Name] = true
		case "COPPER", "POWER_GROUND":
			copperLayers[l.Name] = true
		}
	}

	// Fallback layer-type detection when metadata is absent.
	isSilkLayer := func(name string) bool {
		if len(silkLayers) > 0 {
			return silkLayers[name]
		}
		n := strings.ToLower(name)
		return strings.Contains(n, "silk") || strings.Contains(n, "legend") ||
			strings.Contains(n, "overlay")
	}
	isCopperLayer := func(name string) bool {
		if len(copperLayers) > 0 {
			return copperLayers[name]
		}
		n := strings.ToLower(name)
		return !strings.Contains(n, "silk") && !strings.Contains(n, "legend") &&
			!strings.Contains(n, "overlay") && !strings.Contains(n, "mask") &&
			!strings.Contains(n, "drill") && !strings.Contains(n, "outline") &&
			n != "rout"
	}

	// Collect copper pad bounding boxes and build a 2D grid hash index.
	// Cell size of 2.0 mm matches typical pad pitch; pads spanning multiple
	// cells are stored in all overlapping cells (same pattern as outlineIndex
	// in spatial.go).  Query cost is O(cells_covered) instead of O(n_pads).
	const gridCellMM = 2.0


	type padEntry struct {
		box    bb
		refDes string
		layer  string
	}
	var padEntries []padEntry
	padGrid := make(map[[2]int][]int) // grid cell → []index into padEntries
	for _, p := range board.Pads {
		if !isCopperLayer(p.Layer) {
			continue
		}
		if p.IsViaCatchPad {
			continue
		}
		hw, hh := p.WidthMM/2, p.HeightMM/2
		pe := padEntry{
			box:    bb{p.X - hw, p.X + hw, p.Y - hh, p.Y + hh},
			refDes: p.RefDes,
			layer:  p.Layer,
		}
		idx := len(padEntries)
		padEntries = append(padEntries, pe)
		cxMin := int(math.Floor(pe.box.minX / gridCellMM))
		cxMax := int(math.Floor(pe.box.maxX / gridCellMM))
		cyMin := int(math.Floor(pe.box.minY / gridCellMM))
		cyMax := int(math.Floor(pe.box.maxY / gridCellMM))
		for cx := cxMin; cx <= cxMax; cx++ {
			for cy := cyMin; cy <= cyMax; cy++ {
				key := [2]int{cx, cy}
				padGrid[key] = append(padGrid[key], idx)
			}
		}
	}
	if len(padEntries) == 0 {
		return violations
	}

	// checkOverlap tests a silk feature against nearby copper pads.
	// silkBox is used for grid bucketing (AABB pre-filter only).
	// exactOK, if non-nil, is called after the AABB pre-filter to confirm
	// actual geometric overlap — this prevents false positives from diagonal
	// silk segments (e.g. octagon courtyard outlines) whose bounding boxes
	// extend inward over a pad that doesn't touch the actual line stroke.
	checkOverlap := func(silkLayer string, silkBox bb, exactOK func(pe padEntry) bool) {
		if len(violations) >= maxSilkscreenOnPadViolations {
			return
		}
		cxMin := int(math.Floor(silkBox.minX / gridCellMM))
		cxMax := int(math.Floor(silkBox.maxX / gridCellMM))
		cyMin := int(math.Floor(silkBox.minY / gridCellMM))
		cyMax := int(math.Floor(silkBox.maxY / gridCellMM))
		seen := make(map[int]bool)
	outer:
		for cx := cxMin; cx <= cxMax; cx++ {
			for cy := cyMin; cy <= cyMax; cy++ {
				for _, padIdx := range padGrid[[2]int{cx, cy}] {
					if seen[padIdx] {
						continue
					}
					seen[padIdx] = true
					pe := padEntries[padIdx]
					if !sameSide(silkLayer, pe.layer) {
						continue
					}
					if !silkBox.overlaps(pe.box) {
						continue
					}
					if exactOK != nil && !exactOK(pe) {
						continue
					}
					msg, sug := msgSilkscreenOnPad(pe.refDes)
					scx := (silkBox.minX + silkBox.maxX) / 2
					scy := (silkBox.minY + silkBox.maxY) / 2
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "ERROR",
						Layer:      silkLayer,
						X:          scx,
						Y:          scy,
						X2:         (pe.box.minX + pe.box.maxX) / 2,
						Y2:         (pe.box.minY + pe.box.maxY) / 2,
						Message:    msg,
						Suggestion: sug,
						RefDes:     pe.refDes,
					})
					break outer
				}
			}
		}
	}

	// Check silk traces.
	// Use an exact capsule-vs-pad check so that diagonal segments (e.g. the
	// 45° sides of an octagon courtyard) don't produce false positives from
	// their oversized bounding boxes.
	for _, t := range board.Traces {
		if len(violations) >= maxSilkscreenOnPadViolations {
			break
		}
		if !isSilkLayer(t.Layer) {
			continue
		}
		hw := t.WidthMM / 2
		box := bb{
			math.Min(t.StartX, t.EndX) - hw,
			math.Max(t.StartX, t.EndX) + hw,
			math.Min(t.StartY, t.EndY) - hw,
			math.Max(t.StartY, t.EndY) + hw,
		}
		t := t // capture for closure
		exactOK := func(pe padEntry) bool {
			padCx := (pe.box.minX + pe.box.maxX) / 2
			padCy := (pe.box.minY + pe.box.maxY) / 2
			padR := math.Max(pe.box.maxX-pe.box.minX, pe.box.maxY-pe.box.minY) / 2
			return ptToSegDist(padCx, padCy, t.StartX, t.StartY, t.EndX, t.EndY) < hw+padR
		}
		checkOverlap(t.Layer, box, exactOK)
	}

	// Check silk pads. AABB is a good enough approximation for rectangular
	// pad-shaped silk features so no exact check is needed.
	for _, p := range board.Pads {
		if len(violations) >= maxSilkscreenOnPadViolations {
			break
		}
		if !isSilkLayer(p.Layer) {
			continue
		}
		hw, hh := p.WidthMM/2, p.HeightMM/2
		box := bb{p.X - hw, p.X + hw, p.Y - hh, p.Y + hh}
		checkOverlap(p.Layer, box, nil)
	}

	return dedupeViolations(violations, 2.0)
}
