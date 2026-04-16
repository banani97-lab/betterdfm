package dfmengine

import "sync"

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
			&PackageCapabilityRule{},
			&TraceImbalanceRule{},
			&FiducialRule{},
			&ComponentHeightRule{},
		},
	}
}

// Run executes all rules in parallel and returns per-instance violations.
// Rules are read-only on board data, so concurrent execution is safe.
// Violation order is deterministic: rule 0 results first, then rule 1, etc.
func (r *Runner) Run(board BoardData, profile ProfileRules) []Violation {
	results := make([][]Violation, len(r.ruleList))
	var wg sync.WaitGroup
	for i, rule := range r.ruleList {
		wg.Add(1)
		go func(idx int, rl Rule) {
			defer wg.Done()
			results[idx] = rl.Run(board, profile)
		}(i, rule)
	}
	wg.Wait()

	var all []Violation
	for _, v := range results {
		all = append(all, v...)
	}
	return all
}
