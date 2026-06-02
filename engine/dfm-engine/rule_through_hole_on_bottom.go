package dfmengine

// ThroughHoleOnBottomRule flags through-hole and press-fit components placed on
// the bottom side of the board. Wave and selective soldering reach only the
// bottom side, so a through-hole part there forces awkward second-operation
// hand or selective soldering and often cannot share a reflow pass with the
// top-side SMT. The SMT component-height rule deliberately skips non-SMT parts,
// so without this rule bottom-side through-hole placement is checked by nothing.
//
// Gated by profile.FlagThroughHoleOnBottom: nil or true enables the check
// (the common-case default), false disables it for shops that accept the
// process. Mirrors the *bool nil-means-on convention used by the
// silkscreen-on-pad rule.
type ThroughHoleOnBottomRule struct{}

func (r *ThroughHoleOnBottomRule) ID() string { return "through-hole-on-bottom" }

func (r *ThroughHoleOnBottomRule) Run(board BoardData, profile ProfileRules) []Violation {
	if profile.FlagThroughHoleOnBottom != nil && !*profile.FlagThroughHoleOnBottom {
		return nil
	}

	const maxViolations = 500
	_, botMask := outerSolderMaskLayerNames(board.Layers)

	var violations []Violation
	for _, c := range board.Components {
		if len(violations) >= maxViolations {
			break
		}
		if c.Side != "bot" {
			continue
		}
		if c.MountType != "thmt" && c.MountType != "pressfit" {
			continue
		}
		if isTestPoint(c.RefDes) {
			continue
		}
		msg, sug := msgThroughHoleOnBottom(c.RefDes, c.MountType)
		violations = append(violations, Violation{
			RuleID:     r.ID(),
			Severity:   "WARNING",
			Layer:      botMask,
			X:          c.X,
			Y:          c.Y,
			Message:    msg,
			Suggestion: sug,
			Unit:       "count",
			RefDes:     c.RefDes,
			Count:      1,
		})
	}
	return violations
}
