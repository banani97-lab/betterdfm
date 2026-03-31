package dfmengine

import "fmt"

// FiducialRule checks that the board has at least 3 fiducial markers
// for pick-and-place machine alignment.
const minFiducials = 3

type FiducialRule struct{}

func (r *FiducialRule) ID() string { return "fiducial-count" }

func (r *FiducialRule) Run(board BoardData, _ ProfileRules) []Violation {
	if board.SourceFormat == "GERBER" {
		return nil // requires pad_usage attribute (ODB++ only)
	}
	count := 0
	for _, p := range board.Pads {
		if p.IsFiducial {
			count++
		}
	}

	// Only run this rule if the parser provided fiducial data (at least one
	// pad tagged). Gerber files don't carry fiducial attributes, so we skip
	// rather than always failing.
	if count == 0 {
		return nil
	}
	if count >= minFiducials {
		return nil
	}

	msg := fmt.Sprintf("Board has %d fiducial(s), minimum %d required for pick-and-place alignment.", count, minFiducials)
	sug := "Add global fiducial markers (typically 1mm round pads with 2-3mm solder mask opening) on at least 3 corners of the board."

	return []Violation{{
		RuleID:     r.ID(),
		Severity:   "WARNING",
		Layer:      "",
		X:          0,
		Y:          0,
		Message:    msg,
		Suggestion: sug,
		MeasuredMM: float64(count),
		LimitMM:    float64(minFiducials),
		Unit:       "count",
	}}
}
