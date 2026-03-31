package dfmengine

// packageSizeRank maps passive package classes to a numeric rank (smaller = smaller package).
var packageSizeRank = map[string]int{
	"01005": 1,
	"0201":  2,
	"0402":  3,
	"0603":  4,
	"0805":  5,
	"1206":  6,
	"1210":  7,
	"1812":  8,
	"2010":  9,
	"2512":  10,
}

// PackageCapabilityRule checks that no component uses a package class smaller
// than what the CM can place, as defined by profile.SmallestPackageClass.
type PackageCapabilityRule struct{}

func (r *PackageCapabilityRule) ID() string { return "package-capability" }

func (r *PackageCapabilityRule) Run(board BoardData, profile ProfileRules) []Violation {
	if board.SourceFormat == "GERBER" {
		return nil // requires component data (packageClass)
	}
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
