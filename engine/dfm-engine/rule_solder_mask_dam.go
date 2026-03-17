package dfmengine

import (
	"math"
	"sort"
)

// SolderMaskDamRule checks that solder mask bridges between adjacent pads are wide enough.
type SolderMaskDamRule struct{}

func (r *SolderMaskDamRule) ID() string { return "solder-mask-dam" }

func (r *SolderMaskDamRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinSolderMaskDamMM <= 0 {
		return violations
	}
	const (
		maxViol   = 2000 // raised — dedup will collapse the final count
		damCellMM = 2.0
	)

	// Solder mask is only applied to the outer copper layers (top and bottom).
	// Inner layers never have solder mask, so checking them produces false positives.
	// Identify the first and last COPPER layers in board order as the outer layers.
	outerLayers := map[string]bool{}
	firstCopper, lastCopper := "", ""
	for _, l := range board.Layers {
		if l.Type == "COPPER" {
			if firstCopper == "" {
				firstCopper = l.Name
			}
			lastCopper = l.Name
		}
	}
	if firstCopper != "" {
		outerLayers[firstCopper] = true
	}
	if lastCopper != "" {
		outerLayers[lastCopper] = true
	}

	// Group pads by layer, then use a sorted sweep to avoid O(n²).
	type padWithRadius struct {
		p      Pad
		radius float64
	}
	byLayer := map[string][]padWithRadius{}
	for _, p := range board.Pads {
		if !outerLayers[p.Layer] {
			continue // skip inner-layer pads — no solder mask there
		}
		r := math.Max(p.WidthMM, p.HeightMM) / 2
		byLayer[p.Layer] = append(byLayer[p.Layer], padWithRadius{p, r})
	}

	minDam := profile.MinSolderMaskDamMM

	for _, pads := range byLayer {
		if len(violations) >= maxViol {
			break
		}
		// Sort by X so we can break the inner loop early once pads are too far apart.
		sort.Slice(pads, func(i, j int) bool { return pads[i].p.X < pads[j].p.X })

		for i := 0; i < len(pads); i++ {
			if len(violations) >= maxViol {
				break
			}
			a := pads[i]
			xLimit := a.p.X + a.radius + minDam + 10.0 // 10mm = generous max pad radius

			for j := i + 1; j < len(pads); j++ {
				if len(violations) >= maxViol {
					break
				}
				b := pads[j]
				if b.p.X-b.radius > xLimit {
					break // all remaining pads are too far in X
				}
				centerDist := math.Sqrt((a.p.X-b.p.X)*(a.p.X-b.p.X) + (a.p.Y-b.p.Y)*(a.p.Y-b.p.Y))
				// P2.1: Use padProjection for shape-aware pad-to-pad gap.
				dx, dy := b.p.X-a.p.X, b.p.Y-a.p.Y
				edgeDist := centerDist - padProjection(a.p, dx, dy) - padProjection(b.p, -dx, -dy)
				// Overlapping openings (edgeDist < 0) are a DRC concern, not DFM -- skip.
				if edgeDist < 0 {
					continue
				}
				if edgeDist < minDam-geomEps {
					msg, sug := msgSolderMaskDamBelow(edgeDist, minDam)
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "WARNING",
						Layer:      a.p.Layer,
						X:          (a.p.X + b.p.X) / 2,
						Y:          (a.p.Y + b.p.Y) / 2,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: edgeDist,
						LimitMM:    minDam,
						Unit:       "mm",
						NetName:    a.p.NetName,
						X2:         b.p.X,
						Y2:         b.p.Y,
					})
				}
			}
		}
	}
	return dedupeViolations(violations, damCellMM)
}
