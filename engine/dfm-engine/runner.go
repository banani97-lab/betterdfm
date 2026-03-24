package dfmengine

// Runner holds all registered rules.
type Runner struct {
	ruleList []Rule
}

// NewRunner returns a Runner with all built-in rules registered.
// All rule types are defined in this package to avoid circular imports.
func NewRunner() *Runner {
	return &Runner{
		ruleList: []Rule{
			&TraceWidthRule{},
			&ClearanceRule{},
			&DrillSizeRule{},
			&AnnularRingRule{},
			&AspectRatioRule{},
			&SolderMaskDamRule{},
			&EdgeClearanceRule{},
			&DrillToDrillRule{},
			&DrillToCopperRule{},
			&CopperSliverRule{},
			&SilkscreenOnPadRule{},
			&PadSizeForPackageRule{},
			&TombstoningRiskRule{},
		},
	}
}

// Run executes all rules and returns per-instance violations.
func (r *Runner) Run(board BoardData, profile ProfileRules) []Violation {
	var all []Violation
	for _, rule := range r.ruleList {
		all = append(all, rule.Run(board, profile)...)
	}
	return all
}
