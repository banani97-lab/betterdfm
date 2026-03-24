package dfmengine

// padSizeRange defines expected pad dimension ranges for a passive package class
// based on IPC-7351B land pattern recommendations.
type padSizeRange struct {
	minW, maxW float64
	minH, maxH float64
}

var ipcPadRanges = map[string]padSizeRange{
	"0201": {0.20, 0.40, 0.20, 0.40},
	"0402": {0.40, 0.80, 0.40, 0.80},
	"0603": {0.60, 1.20, 0.60, 1.30},
	"0805": {0.80, 1.60, 0.80, 1.80},
	"1206": {1.00, 2.00, 1.20, 2.50},
	"1210": {1.00, 2.00, 1.80, 3.00},
	"1812": {1.60, 2.60, 2.80, 3.80},
	"2010": {1.80, 2.80, 2.00, 3.00},
	"2512": {2.20, 3.40, 2.80, 3.80},
}

// PadSizeForPackageRule checks that pad dimensions match IPC-7351 expected ranges
// for the detected passive package class.
type PadSizeForPackageRule struct{}

func (r *PadSizeForPackageRule) ID() string { return "pad-size-for-package" }

func (r *PadSizeForPackageRule) Run(board BoardData, _ ProfileRules) []Violation {
	const maxViolations = 500
	var violations []Violation

	// Track unclassified components (non-empty RefDes but empty PackageClass)
	unclassifiedRefs := map[string]struct{}{}

	for _, pad := range board.Pads {
		if len(violations) >= maxViolations {
			break
		}

		if pad.PackageClass == "" {
			if pad.RefDes != "" {
				unclassifiedRefs[pad.RefDes] = struct{}{}
			}
			continue
		}

		expected, ok := ipcPadRanges[pad.PackageClass]
		if !ok {
			continue
		}

		// Check width
		if pad.WidthMM < expected.minW {
			msg, sug := msgPadUndersizedForPackage(pad.RefDes, pad.PackageClass, pad.WidthMM, expected.minW)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: pad.WidthMM,
				LimitMM:    expected.minW,
				Unit:       "mm",
				RefDes:     pad.RefDes,
			})
		} else if pad.WidthMM > expected.maxW {
			msg, sug := msgPadOversizedForPackage(pad.RefDes, pad.PackageClass, pad.WidthMM, expected.maxW)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "INFO",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: pad.WidthMM,
				LimitMM:    expected.maxW,
				Unit:       "mm",
				RefDes:     pad.RefDes,
			})
		}

		if len(violations) >= maxViolations {
			break
		}

		// Check height
		if pad.HeightMM < expected.minH {
			msg, sug := msgPadUndersizedForPackage(pad.RefDes, pad.PackageClass, pad.HeightMM, expected.minH)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "WARNING",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: pad.HeightMM,
				LimitMM:    expected.minH,
				Unit:       "mm",
				RefDes:     pad.RefDes,
			})
		} else if pad.HeightMM > expected.maxH {
			msg, sug := msgPadOversizedForPackage(pad.RefDes, pad.PackageClass, pad.HeightMM, expected.maxH)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "INFO",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: pad.HeightMM,
				LimitMM:    expected.maxH,
				Unit:       "mm",
				RefDes:     pad.RefDes,
			})
		}
	}

	violations = dedupeViolations(violations, 2.0)

	// Emit a single INFO if some components could not be classified
	if len(unclassifiedRefs) > 0 {
		msg, sug := msgUnclassifiedComponents(len(unclassifiedRefs))
		violations = append(violations, Violation{
			RuleID:     r.ID(),
			Severity:   "INFO",
			Message:    msg,
			Suggestion: sug,
			Unit:       "mm",
		})
	}

	return violations
}
