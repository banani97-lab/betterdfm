package dfmengine

import (
	"math"
	"sort"
)

// ComponentSpacingRule flags adjacent same-side components placed too close
// together for the pick-and-place nozzle to reach, or for rework access. The
// limit (profile.MinComponentSpacingMM) is the minimum edge-to-edge gap
// between component land-pattern extents; IPC-7351B nominal-density courtyard
// excess (0.25 mm per side) implies roughly 0.5 mm between neighbors.
//
// A component's extent is derived from the bounding box of its outer-copper
// pads grouped by RefDes — the parser does not emit a courtyard, so the pad
// bounding box is the best available proxy for the land pattern. Components on
// opposite sides of the board never collide, so only same-side pairs are
// compared. Through-hole parts whose pads span both outer layers are skipped:
// their keepouts are governed by mechanical rules, not nozzle access.
type ComponentSpacingRule struct{}

func (r *ComponentSpacingRule) ID() string { return "component-spacing" }

type compBox struct {
	ref                    string
	side                   string // "top" | "bot"
	layer                  string
	minX, minY, maxX, maxY float64
	cx, cy                 float64
}

func (r *ComponentSpacingRule) Run(board BoardData, profile ProfileRules) []Violation {
	limit := profile.MinComponentSpacingMM
	if limit <= 0 {
		return nil
	}

	// Resolve the outer copper layer names in stack order so each pad can be
	// attributed to the top or bottom side.
	var topCu, botCu string
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			if topCu == "" {
				topCu = l.Name
			}
			botCu = l.Name
		}
	}

	// Accumulate per-component pad bounding boxes. sidesSeen tracks which outer
	// layers a RefDes appears on so we can drop through-hole parts.
	type acc struct {
		minX, minY, maxX, maxY float64
		topSeen, botSeen       bool
		hasPad                 bool
	}
	groups := map[string]*acc{}
	for _, pad := range board.Pads {
		if pad.RefDes == "" || pad.IsViaCatchPad || isTestPoint(pad.RefDes) {
			continue
		}
		var side string
		switch pad.Layer {
		case topCu:
			side = "top"
		case botCu:
			side = "bot"
		default:
			// Inner-layer pad (plane thermal, etc.) — not a placement land.
			continue
		}
		a := groups[pad.RefDes]
		if a == nil {
			a = &acc{minX: math.MaxFloat64, minY: math.MaxFloat64, maxX: -math.MaxFloat64, maxY: -math.MaxFloat64}
			groups[pad.RefDes] = a
		}
		// Expand by the pad's axis-aligned half-extents.
		hw, hh := pad.WidthMM/2, pad.HeightMM/2
		a.minX = math.Min(a.minX, pad.X-hw)
		a.maxX = math.Max(a.maxX, pad.X+hw)
		a.minY = math.Min(a.minY, pad.Y-hh)
		a.maxY = math.Max(a.maxY, pad.Y+hh)
		a.hasPad = true
		if side == "top" {
			a.topSeen = true
		} else {
			a.botSeen = true
		}
	}

	boxes := make([]compBox, 0, len(groups))
	for ref, a := range groups {
		if !a.hasPad {
			continue
		}
		// Skip parts spanning both outer layers (through-hole / press-fit).
		if a.topSeen == a.botSeen {
			continue
		}
		side := "top"
		if a.botSeen {
			side = "bot"
		}
		layer := topCu
		if side == "bot" {
			layer = botCu
		}
		boxes = append(boxes, compBox{
			ref: ref, side: side, layer: layer,
			minX: a.minX, minY: a.minY, maxX: a.maxX, maxY: a.maxY,
			cx: (a.minX + a.maxX) / 2, cy: (a.minY + a.maxY) / 2,
		})
	}

	// Sweepline: sort by minX, then for each box compare only with later boxes
	// whose minX is within the limit of this box's maxX. Deterministic ties by
	// RefDes keep the violation set stable across runs.
	sort.Slice(boxes, func(i, j int) bool {
		if boxes[i].minX != boxes[j].minX {
			return boxes[i].minX < boxes[j].minX
		}
		return boxes[i].ref < boxes[j].ref
	})

	const maxViolations = 500
	var violations []Violation
	for i := 0; i < len(boxes); i++ {
		if len(violations) >= maxViolations {
			break
		}
		a := boxes[i]
		for j := i + 1; j < len(boxes); j++ {
			b := boxes[j]
			if b.minX-a.maxX >= limit-geomEps {
				break // no later box can be within the limit in X
			}
			if a.side != b.side {
				continue
			}
			gap := bboxGap(a, b)
			if gap >= limit-geomEps {
				continue
			}
			sev := "WARNING"
			if gap <= geomEps {
				sev = "ERROR" // land patterns overlap
			}
			msg, sug := msgComponentSpacing(a.ref, b.ref, gap, limit)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   sev,
				Layer:      a.layer,
				X:          a.cx,
				Y:          a.cy,
				X2:         b.cx,
				Y2:         b.cy,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: gap,
				LimitMM:    limit,
				Unit:       "mm",
				RefDes:     a.ref,
				Count:      1,
			})
			if len(violations) >= maxViolations {
				break
			}
		}
	}
	return violations
}

// bboxGap returns the axis-aligned edge-to-edge distance between two boxes.
// Returns 0 when they overlap or touch.
func bboxGap(a, b compBox) float64 {
	dx := math.Max(0, math.Max(b.minX-a.maxX, a.minX-b.maxX))
	dy := math.Max(0, math.Max(b.minY-a.maxY, a.minY-b.maxY))
	return math.Sqrt(dx*dx + dy*dy)
}
