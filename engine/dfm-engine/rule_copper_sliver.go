package dfmengine

import "strings"

// CopperSliverRule finds very narrow un-netted copper features (slivers) that can
// peel or lift during PCB fabrication. Targets copper pour artifacts — thin traces
// with no net assignment — not intentional signal traces.
const maxCopperSliverViolations = 500

type CopperSliverRule struct{}

func (r *CopperSliverRule) ID() string { return "copper-sliver" }

func (r *CopperSliverRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinCopperSliverMM <= 0 {
		return violations
	}

	copperLayers := make(map[string]bool, len(board.Layers))
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			copperLayers[l.Name] = true
		}
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

	minW := profile.MinCopperSliverMM
	for _, t := range board.Traces {
		if len(violations) >= maxCopperSliverViolations {
			break
		}
		if !isCopperLayer(t.Layer) {
			continue
		}
		// Only flag un-netted copper — intentional signal traces have a net name.
		if t.NetName != "" {
			continue
		}
		if t.WidthMM < minW {
			msg, sug := msgCopperSliver(t.WidthMM, minW)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      t.Layer,
				X:          (t.StartX + t.EndX) / 2,
				Y:          (t.StartY + t.EndY) / 2,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: t.WidthMM,
				LimitMM:    minW,
				Unit:       "mm",
			})
		}
	}

	return dedupeViolations(violations, 2.0)
}
