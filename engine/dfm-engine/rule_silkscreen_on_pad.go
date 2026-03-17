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

	// Collect copper pad bounding boxes keyed by layer.
	type padEntry struct {
		box    bb
		refDes string
		layer  string
	}
	var copperPadBBs []padEntry
	for _, p := range board.Pads {
		if !isCopperLayer(p.Layer) {
			continue
		}
		hw, hh := p.WidthMM/2, p.HeightMM/2
		copperPadBBs = append(copperPadBBs, padEntry{
			box:    bb{p.X - hw, p.X + hw, p.Y - hh, p.Y + hh},
			refDes: p.RefDes,
			layer:  p.Layer,
		})
	}
	if len(copperPadBBs) == 0 {
		return violations
	}

	checkOverlap := func(silkLayer string, silkBox bb) {
		if len(violations) >= maxSilkscreenOnPadViolations {
			return
		}
		for _, pe := range copperPadBBs {
			if !sameSide(silkLayer, pe.layer) {
				continue
			}
			if silkBox.overlaps(pe.box) {
				msg, sug := msgSilkscreenOnPad(pe.refDes)
				// Report at the center of the silk feature.
				cx := (silkBox.minX + silkBox.maxX) / 2
				cy := (silkBox.minY + silkBox.maxY) / 2
				violations = append(violations, Violation{
					RuleID:     r.ID(),
					Severity:   "WARNING",
					Layer:      silkLayer,
					X:          cx,
					Y:          cy,
					X2:         (pe.box.minX + pe.box.maxX) / 2,
					Y2:         (pe.box.minY + pe.box.maxY) / 2,
					Message:    msg,
					Suggestion: sug,
					RefDes:     pe.refDes,
				})
				if len(violations) >= maxSilkscreenOnPadViolations {
					return
				}
				// Only report the first overlapping pad per silk feature.
				break
			}
		}
	}

	// Check silk traces.
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
		checkOverlap(t.Layer, box)
	}

	// Check silk pads (e.g. silkscreen courtyard pad markings).
	for _, p := range board.Pads {
		if len(violations) >= maxSilkscreenOnPadViolations {
			break
		}
		if !isSilkLayer(p.Layer) {
			continue
		}
		hw, hh := p.WidthMM/2, p.HeightMM/2
		box := bb{p.X - hw, p.X + hw, p.Y - hh, p.Y + hh}
		checkOverlap(p.Layer, box)
	}

	return dedupeViolations(violations, 2.0)
}
