package dfmengine

import (
	"math"
	"sort"
)

const (
	// collinearFraction: if every global fiducial sits within this fraction of
	// the fiducial span from the line through the two extreme fiducials, the
	// set is effectively collinear and gives the placement machine poor 2D
	// (rotation/scale) registration. IPC-7351 recommends a wide triangle.
	collinearFraction = 0.1
	// localFiducialMarginMM: a fine-pitch / BGA device should have a fiducial
	// within this distance of its land-pattern bounding box for local optical
	// alignment (IPC-7351 / IPC-A-610). Beyond this it is treated as "no local
	// fiducial present" and surfaced as an advisory.
	localFiducialMarginMM = 5.0
)

// FiducialPlacementRule checks the *quality* of fiducial placement, complementing
// FiducialRule which only checks the count. It runs only when the parser found
// at least one fiducial. Two checks:
//
//   - Collinear global fiducials (3 or more, but nearly in a line) -> WARNING.
//   - Fine-pitch / BGA components with no fiducial near their land pattern ->
//     INFO advising local fiducials for optical alignment.
//
// Gated by profile.EnableFiducialPlacementCheck: nil or true enables the check,
// false disables it.
type FiducialPlacementRule struct{}

func (r *FiducialPlacementRule) ID() string { return "fiducial-placement" }

func (r *FiducialPlacementRule) Run(board BoardData, profile ProfileRules) []Violation {
	// Gated by profile.EnableFiducialPlacementCheck: nil or true enables the
	// check, false disables it.
	if profile.EnableFiducialPlacementCheck != nil && !*profile.EnableFiducialPlacementCheck {
		return nil
	}

	var fids []Point
	for _, p := range board.Pads {
		if p.IsFiducial {
			fids = append(fids, Point{X: p.X, Y: p.Y})
		}
	}
	// Only run when the parser provided fiducial data, mirroring FiducialRule.
	if len(fids) == 0 {
		return nil
	}

	var violations []Violation

	// 1. Collinearity of global fiducials (only meaningful with 3+; count
	//    sufficiency is FiducialRule's job).
	if len(fids) >= minFiducials {
		if span, perp, ax, ay := fiducialSpread(fids); span > geomEps && perp < span*collinearFraction {
			msg, sug := msgFiducialsCollinear(len(fids))
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				X:          ax,
				Y:          ay,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: perp,
				LimitMM:    span * collinearFraction,
				Unit:       "mm",
				Count:      1,
			})
		}
	}

	// 2. Local fiducials for fine-pitch / BGA devices.
	const maxViolations = 500
	outer := outerCopperLayerSet(board.Layers)

	// Bounding box per fine-pitch component from its outer-copper pads.
	type box struct {
		minX, minY, maxX, maxY float64
		seen                   bool
	}
	boxes := map[string]*box{}
	for _, pad := range board.Pads {
		if pad.RefDes == "" || isTestPoint(pad.RefDes) || pad.IsFiducial {
			continue
		}
		if len(outer) > 0 && !outer[pad.Layer] {
			continue
		}
		if !isFinePitchClass(pad.PackageClass) {
			continue
		}
		b := boxes[pad.RefDes]
		if b == nil {
			b = &box{minX: math.MaxFloat64, minY: math.MaxFloat64, maxX: -math.MaxFloat64, maxY: -math.MaxFloat64}
			boxes[pad.RefDes] = b
		}
		hw, hh := pad.WidthMM/2, pad.HeightMM/2
		b.minX = math.Min(b.minX, pad.X-hw)
		b.maxX = math.Max(b.maxX, pad.X+hw)
		b.minY = math.Min(b.minY, pad.Y-hh)
		b.maxY = math.Max(b.maxY, pad.Y+hh)
		b.seen = true
	}

	// Deterministic order over the component map.
	refs := make([]string, 0, len(boxes))
	for ref := range boxes {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	for _, ref := range refs {
		if len(violations) >= maxViolations {
			break
		}
		b := boxes[ref]
		if !b.seen {
			continue
		}
		if hasFiducialNearBox(fids, b.minX, b.minY, b.maxX, b.maxY, localFiducialMarginMM) {
			continue
		}
		cx, cy := (b.minX+b.maxX)/2, (b.minY+b.maxY)/2
		msg, sug := msgNoLocalFiducial(ref)
		violations = append(violations, Violation{
			RuleID:     r.ID(),
			Severity:   "INFO",
			X:          cx,
			Y:          cy,
			Message:    msg,
			Suggestion: sug,
			Unit:       "count",
			RefDes:     ref,
			Count:      1,
		})
	}

	return violations
}

// fiducialSpread returns the maximum pairwise distance (span) among fiducials,
// the maximum perpendicular distance of any fiducial from the line through the
// two extreme points (perp), and a representative anchor point (the midpoint of
// the extreme pair) for the violation marker.
func fiducialSpread(fids []Point) (span, perp, ax, ay float64) {
	var aIdx, bIdx int
	for i := 0; i < len(fids); i++ {
		for j := i + 1; j < len(fids); j++ {
			dx := fids[i].X - fids[j].X
			dy := fids[i].Y - fids[j].Y
			if d := math.Sqrt(dx*dx + dy*dy); d > span {
				span = d
				aIdx, bIdx = i, j
			}
		}
	}
	a, b := fids[aIdx], fids[bIdx]
	for _, p := range fids {
		if d := ptToSegDist(p.X, p.Y, a.X, a.Y, b.X, b.Y); d > perp {
			perp = d
		}
	}
	return span, perp, (a.X + b.X) / 2, (a.Y + b.Y) / 2
}

// hasFiducialNearBox reports whether any fiducial lies within margin of the
// axis-aligned box (inside the box counts as near).
func hasFiducialNearBox(fids []Point, minX, minY, maxX, maxY, margin float64) bool {
	for _, p := range fids {
		dx := math.Max(0, math.Max(minX-p.X, p.X-maxX))
		dy := math.Max(0, math.Max(minY-p.Y, p.Y-maxY))
		if math.Sqrt(dx*dx+dy*dy) <= margin {
			return true
		}
	}
	return false
}
