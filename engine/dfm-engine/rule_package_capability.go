package dfmengine

// packageSizeRank maps passive package classes to a numeric rank (smaller = smaller package).
var packageSizeRank = map[string]int{
	"0201": 1,
	"0402": 2,
	"0603": 3,
	"0805": 4,
	"1206": 5,
	"1210": 6,
	"1812": 7,
	"2010": 8,
	"2512": 9,
}

// PackageCapabilityRule checks that no component uses a package class smaller
// than what the CM can place, as defined by profile.SmallestPackageClass.
type PackageCapabilityRule struct{}

func (r *PackageCapabilityRule) ID() string { return "package-capability" }

func (r *PackageCapabilityRule) Run(board BoardData, profile ProfileRules) []Violation {
	if profile.SmallestPackageClass == "" {
		return nil
	}

	minRank, ok := packageSizeRank[profile.SmallestPackageClass]
	if !ok {
		return nil
	}

	const maxViolations = 500
	var violations []Violation

	// Deduplicate by RefDes — one violation per component, not per pad
	seen := map[string]bool{}

	for _, pad := range board.Pads {
		if len(violations) >= maxViolations {
			break
		}
		if pad.PackageClass == "" || pad.RefDes == "" {
			continue
		}
		if seen[pad.RefDes] {
			continue
		}

		padRank, known := packageSizeRank[pad.PackageClass]
		if !known {
			continue
		}

		if padRank < minRank {
			seen[pad.RefDes] = true
			msg, sug := msgPackageCapability(pad.RefDes, pad.PackageClass, profile.SmallestPackageClass)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				RefDes:     pad.RefDes,
				Unit:       "mm",
			})
		}
	}

	return violations
}
