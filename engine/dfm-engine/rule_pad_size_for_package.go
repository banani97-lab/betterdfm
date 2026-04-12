package dfmengine

// padSizeRange defines the acceptable pad dimension envelope for a passive
// package class, based on IPC-7351B land pattern recommendations spanning
// Density C (Min land protrusion) through Density A (Max land protrusion),
// with a small tolerance on each end.
//
// Dimensions are expressed as (short, long) rather than (width, height) so
// the check is rotation-invariant: a 1206 placed at 0° has the same pad
// footprint as one placed at 90°, and the rule normalizes each pad to
// (min, max) of its two dimensions before comparing.
type padSizeRange struct {
	minShort, maxShort float64 // shorter pad dimension
	minLong, maxLong   float64 // longer pad dimension
}

// IPC-7351B nominal pad dimensions for rectangular passives. Values are
// generous enough to cover Density A/B/C variations plus fabrication
// tolerance; the goal is to catch genuinely misplaced pads (e.g. a 0402
// pad on an 0805 footprint) without false-flagging standard land patterns.
var ipcPadRanges = map[string]padSizeRange{
	// Package: short (across body)     long (along body axis)
	"01005": {0.10, 0.30, 0.15, 0.35},
	"0201":  {0.15, 0.40, 0.20, 0.45},
	"0402":  {0.40, 0.75, 0.45, 0.85},
	"0603":  {0.60, 1.05, 0.70, 1.30},
	"0805":  {0.75, 1.40, 0.90, 1.60},
	"1206":  {0.85, 1.35, 1.30, 2.00},
	"1210":  {0.85, 1.35, 1.90, 2.80},
	"1812":  {1.40, 2.20, 2.60, 3.80},
	"2010":  {1.50, 2.40, 2.40, 3.40},
	"2512":  {1.80, 2.80, 2.90, 4.20},
}

// PadSizeForPackageRule checks that pad dimensions match IPC-7351 expected ranges
// for the detected passive package class.
type PadSizeForPackageRule struct{}

func (r *PadSizeForPackageRule) ID() string { return "pad-size-for-package" }

func (r *PadSizeForPackageRule) Run(board BoardData, _ ProfileRules) []Violation {
	const maxViolations = 500
	var violations []Violation

	// Only check component mounting pads on outer copper layers. Internal
	// planes (e.g. L02_GND) can carry "pads" that are really plane features
	// like thermal reliefs or solid connections — they get a refdes via
	// spatial lookup to the component above, but they are not the physical
	// mounting lands we want to measure against IPC-7351.
	outerLayers := outerCopperLayerSet(board.Layers)


	// Track unclassified components (non-empty RefDes but empty PackageClass)
	unclassifiedRefs := map[string]struct{}{}

	for _, pad := range board.Pads {
		if len(violations) >= maxViolations {
			break
		}

		if len(outerLayers) > 0 && !outerLayers[pad.Layer] {
			continue
		}

		if pad.IsViaCatchPad {
			continue
		}
		if isTestPoint(pad.RefDes) {
			continue
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

		// Normalize (widthMM, heightMM) to (short, long) so rotated parts
		// are compared against the correct axes. Pad bounding-box width is
		// whichever axis is longer in canvas space, which flips depending
		// on whether the component is placed at 0°/180° vs 90°/270°.
		padShort, padLong := pad.WidthMM, pad.HeightMM
		if padShort > padLong {
			padShort, padLong = padLong, padShort
		}

		// Short dimension check
		if padShort < expected.minShort {
			msg, sug := msgPadUndersizedForPackage(pad.RefDes, pad.PackageClass, padShort, expected.minShort)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: padShort,
				LimitMM:    expected.minShort,
				Unit:       "mm",
				RefDes:     pad.RefDes,
			})
		} else if padShort > expected.maxShort {
			msg, sug := msgPadOversizedForPackage(pad.RefDes, pad.PackageClass, padShort, expected.maxShort)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "INFO",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: padShort,
				LimitMM:    expected.maxShort,
				Unit:       "mm",
				RefDes:     pad.RefDes,
			})
		}

		if len(violations) >= maxViolations {
			break
		}

		// Long dimension check
		if padLong < expected.minLong {
			msg, sug := msgPadUndersizedForPackage(pad.RefDes, pad.PackageClass, padLong, expected.minLong)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "ERROR",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: padLong,
				LimitMM:    expected.minLong,
				Unit:       "mm",
				RefDes:     pad.RefDes,
			})
		} else if padLong > expected.maxLong {
			msg, sug := msgPadOversizedForPackage(pad.RefDes, pad.PackageClass, padLong, expected.maxLong)
			violations = append(violations, Violation{
				RuleID:     r.ID(),
				Severity:   "INFO",
				Layer:      pad.Layer,
				X:          pad.X,
				Y:          pad.Y,
				Message:    msg,
				Suggestion: sug,
				MeasuredMM: padLong,
				LimitMM:    expected.maxLong,
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
