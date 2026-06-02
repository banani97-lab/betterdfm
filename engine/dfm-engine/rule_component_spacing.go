package dfmengine

import (
	"math"
	"sort"
)

// ComponentSpacingRule flags adjacent same-side components placed too close
// together for the pick-and-place nozzle to reach, or for rework access. The
// limit is the minimum edge-to-edge gap between component land-pattern extents;
// IPC-7351B nominal-density courtyard excess (0.25 mm per side) implies roughly
// 0.5 mm between neighbors.
//
// Two modes:
//   - Flat (profile.ComponentSpacing == nil): a single threshold
//     profile.MinComponentSpacingMM applies to every same-side pair. Through-hole
//     parts (pads on both outer layers) are skipped — their keepouts are
//     mechanical, not nozzle-access.
//   - Per-class (profile.ComponentSpacing != nil): each part is classified by its
//     PackageType (discrete / leaded / bga / through_hole) and the pair threshold
//     is max(radius[a], radius[b]). Through-hole parts ARE included — a discrete
//     packed against a through-hole pin field needs the larger keepout — and they
//     contribute a box on each outer side they appear on.
//
// A component's extent is the bounding box of its outer-copper pads grouped by
// RefDes; the parser emits no courtyard, so the pad bounding box is the best
// available proxy. Opposite-side parts never collide, so only same-side pairs
// are compared.
type ComponentSpacingRule struct{}

func (r *ComponentSpacingRule) ID() string { return "component-spacing" }

type compBox struct {
	ref                    string
	side                   string // "top" | "bot"
	layer                  string
	ptype                  string // package class: discrete | leaded | bga | through_hole
	minX, minY, maxX, maxY float64
	cx, cy                 float64
}

func (r *ComponentSpacingRule) Run(board BoardData, profile ProfileRules) []Violation {
	classes := profile.ComponentSpacing
	flat := profile.MinComponentSpacingMM

	// radius returns the keepout a package class requires to a neighbor. In flat
	// mode every class resolves to MinComponentSpacingMM, so PackageType is
	// irrelevant and behavior matches the legacy single-threshold rule.
	radius := func(ptype string) float64 {
		if classes == nil {
			return flat
		}
		var v float64
		switch ptype {
		case "discrete":
			v = classes.DiscreteMM
		case "bga":
			v = classes.BGAMM
		case "through_hole":
			v = classes.ThroughHoleMM
		default: // "leaded", "", unknown -> mid keepout
			v = classes.LeadedMM
		}
		if v <= 0 {
			v = flat // per-field fallback
		}
		return v
	}

	// maxRadius bounds the sweepline X-window: no pair can be in range once their
	// X gap exceeds the largest possible threshold.
	maxRadius := flat
	if classes != nil {
		for _, v := range []float64{classes.DiscreteMM, classes.LeadedMM, classes.BGAMM, classes.ThroughHoleMM} {
			if v > maxRadius {
				maxRadius = v
			}
		}
	}
	if maxRadius <= 0 {
		return nil
	}

	// Outer copper layer names in stack order, for top/bottom attribution.
	var topCu, botCu string
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			if topCu == "" {
				topCu = l.Name
			}
			botCu = l.Name
		}
	}

	// PackageType per RefDes from the parsed component records.
	compType := make(map[string]string, len(board.Components))
	for _, c := range board.Components {
		if c.RefDes != "" {
			compType[c.RefDes] = c.PackageType
		}
	}

	// Accumulate per-component pad bounding boxes. sidesSeen tracks which outer
	// layers a RefDes appears on so we can detect through-hole parts.
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
		spanning := a.topSeen && a.botSeen
		if spanning {
			// Through-hole / press-fit: pads on both outer layers.
			if classes == nil {
				// Flat mode: keepouts here are mechanical, not nozzle access.
				continue
			}
			// Per-class mode: a through-hole pin field interacts with SMT
			// neighbors on both sides, so emit a box on each.
			for _, side := range []string{"top", "bot"} {
				layer := topCu
				if side == "bot" {
					layer = botCu
				}
				boxes = append(boxes, compBox{
					ref: ref, side: side, layer: layer, ptype: "through_hole",
					minX: a.minX, minY: a.minY, maxX: a.maxX, maxY: a.maxY,
					cx: (a.minX + a.maxX) / 2, cy: (a.minY + a.maxY) / 2,
				})
			}
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
			ref: ref, side: side, layer: layer, ptype: compType[ref],
			minX: a.minX, minY: a.minY, maxX: a.maxX, maxY: a.maxY,
			cx: (a.minX + a.maxX) / 2, cy: (a.minY + a.maxY) / 2,
		})
	}

	// Sweepline: sort by minX, compare each box only with later boxes whose minX
	// is within maxRadius of this box's maxX. Deterministic ties by RefDes (then
	// side, since through-hole parts emit two boxes) keep the set stable.
	sort.Slice(boxes, func(i, j int) bool {
		if boxes[i].minX != boxes[j].minX {
			return boxes[i].minX < boxes[j].minX
		}
		if boxes[i].ref != boxes[j].ref {
			return boxes[i].ref < boxes[j].ref
		}
		return boxes[i].side < boxes[j].side
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
			if b.minX-a.maxX >= maxRadius-geomEps {
				break // no later box can be within any threshold in X
			}
			if a.side != b.side {
				continue
			}
			if a.ref == b.ref {
				continue // same part (e.g. its own top/bot through-hole boxes)
			}
			limit := radius(a.ptype)
			if rb := radius(b.ptype); rb > limit {
				limit = rb
			}
			gap := bboxGap(a, b)
			if gap >= limit-geomEps {
				continue
			}
			sev := "WARNING"
			if gap <= geomEps {
				sev = "ERROR" // land patterns overlap
			}
			msg, sug := msgComponentSpacing(a.ref, b.ref, a.ptype, b.ptype, gap, limit)
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
