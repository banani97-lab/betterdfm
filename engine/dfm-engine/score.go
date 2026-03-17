package dfmengine

import "math"

// ScoreResult holds the computed manufacturability score and supporting data.
type ScoreResult struct {
	Score            int
	Grade            string
	Verdict          string
	ByRule           map[string]float64 // penalty points per ruleId
	ByRuleCount      map[string]int     // violation count per ruleId
	AreaCM2          float64
	ViolationDensity float64
}

// ruleWeight returns the yield-impact weight for a given rule ID.
func ruleWeight(id string) float64 {
	switch id {
	case "clearance":
		return 3.0
	case "trace-width":
		return 2.5
	case "annular-ring":
		return 2.5
	case "drill-size":
		return 2.0
	case "drill-to-copper":
		return 2.0
	case "drill-to-drill":
		return 2.0
	case "aspect-ratio":
		return 1.5
	case "edge-clearance":
		return 1.5
	case "solder-mask-dam":
		return 1.0
	case "copper-sliver":
		return 1.0
	case "silkscreen-on-pad":
		return 1.0
	default:
		return 1.0
	}
}

// severityWeight returns the penalty multiplier for a severity string.
func severityWeight(sev string) float64 {
	switch sev {
	case "ERROR":
		return 10.0
	case "WARNING":
		return 3.0
	case "INFO":
		return 0.5
	default:
		return 1.0
	}
}

// marginMult returns a proportional severity factor in [0, 1] based on how far
// the measured value deviates from the limit, relative to the limit itself.
//
// Formula: √(excess) clamped to [0, 1], where:
//   - "too small" violations (trace width, clearance, etc.): excess = (limit - measured) / limit
//   - "too large" violations (aspect ratio): excess = (measured - limit) / limit
//   - measured == 0 (feature entirely absent): hard 1.0
//
// Representative values:
//
//	 5% off limit → ~0.22   25% off → 0.50   100%+ off → 1.00
func marginMult(v Violation) float64 {
	if v.MeasuredMM == 0 || v.LimitMM == 0 {
		return 1.0
	}
	var excess float64
	if v.MeasuredMM > v.LimitMM {
		// "too large" (e.g. aspect-ratio exceeds max)
		excess = (v.MeasuredMM - v.LimitMM) / v.LimitMM
	} else {
		// "too small" (e.g. trace width below min)
		excess = (v.LimitMM - v.MeasuredMM) / v.LimitMM
	}
	return math.Min(math.Sqrt(excess), 1.0)
}

// ruleMaxContribution returns the maximum normalized penalty points a single rule
// can contribute to the score, regardless of violation count. This prevents a
// dense board hitting the 500-violation cap from scoring 0 automatically.
//
// Calibration: all caps sum to exactly 100, so when every rule hits its cap the
// score reaches exactly 0 (grade F). Single-rule-maxed scores:
//
//	clearance alone maxed       → score 80  (grade B — still needs fixes)
//	trace-width alone maxed     → score 84  (grade B)
//	drill-to-copper alone maxed → score 91  (grade A — isolated drill risk)
//	all rules maxed             → score  0  (grade F — truly unmanufacturable)
func ruleMaxContribution(id string) float64 {
	switch id {
	case "clearance":
		return 20.0
	case "trace-width":
		return 16.0
	case "annular-ring":
		return 12.0
	case "drill-size":
		return 10.0
	case "drill-to-copper":
		return 9.0
	case "drill-to-drill":
		return 8.0
	case "aspect-ratio":
		return 7.0
	case "edge-clearance":
		return 7.0
	case "solder-mask-dam":
		return 5.0
	case "copper-sliver":
		return 3.0
	case "silkscreen-on-pad":
		return 3.0
	default:
		return 3.0
	}
}

// outlineBBox returns the width and height of the bounding box of outline points in mm.
func outlineBBox(outline []Point) (w, h float64) {
	if len(outline) == 0 {
		return 0, 0
	}
	minX, minY := outline[0].X, outline[0].Y
	maxX, maxY := outline[0].X, outline[0].Y
	for _, p := range outline[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return maxX - minX, maxY - minY
}

// scoreGrade returns the letter grade and verdict text for a numeric score.
func scoreGrade(score int) (string, string) {
	switch {
	case score >= 90:
		return "A", "Production Ready — no significant issues"
	case score >= 75:
		return "B", "Minor Issues — review recommended before submission"
	case score >= 60:
		return "C", "Moderate Issues — rework required"
	case score >= 40:
		return "D", "Significant Issues — major redesign required"
	default:
		return "F", "Not Manufacturable — critical failures present"
	}
}

// ComputeScore calculates the manufacturability score from violations and board outline.
//
// Penalty formula (per violation):
//   p_i = ruleWeight(ruleId) * severityWeight(severity) * marginMult(v)
//
// Per-rule normalization with cap:
//   raw_norm_r  = sum(p_i for rule r) / areaFactor
//   capped_r    = min(raw_norm_r, ruleMaxContribution(r))
//   P_norm      = sum(capped_r across all rules)
//   score       = clamp(round(100 - P_norm), 0, 100)
//
// The per-rule cap ensures that even a rule hitting the 500-violation ceiling
// (due to dense routing on a complex board) cannot single-handedly force the
// score to zero. It bounds each rule's maximum score impact to a calibrated
// value while still producing 0 when multiple rules are severely violated.
func ComputeScore(violations []Violation, outline []Point) ScoreResult {
	byRule := make(map[string]float64)
	byRuleCount := make(map[string]int)

	for _, v := range violations {
		// Each deduplicated violation represents one distinct spatial problem area.
		// Spatial concentration is already captured by dedup — a dense cluster
		// collapses into fewer cells, producing fewer violations and a lower penalty
		// naturally. Multiplying by count would double-penalize the same information.
		p := ruleWeight(v.RuleID) * severityWeight(v.Severity) * marginMult(v)
		byRule[v.RuleID] += p
		byRuleCount[v.RuleID]++
	}

	bboxW, bboxH := outlineBBox(outline)
	areaCM2 := bboxW * bboxH / 100.0
	areaFactor := math.Sqrt(math.Max(1.0, areaCM2))

	var pNorm float64
	for ruleID, rawPenalty := range byRule {
		rawNorm := rawPenalty / areaFactor
		cap := ruleMaxContribution(ruleID)
		if rawNorm > cap {
			rawNorm = cap
		}
		pNorm += rawNorm
	}

	rawScore := 100.0 - pNorm
	score := int(math.Round(math.Max(0, math.Min(100, rawScore))))

	density := 0.0
	if areaCM2 > 0 {
		density = float64(len(violations)) / areaCM2
	}

	grade, verdict := scoreGrade(score)
	return ScoreResult{
		Score:            score,
		Grade:            grade,
		Verdict:          verdict,
		ByRule:           byRule,
		ByRuleCount:      byRuleCount,
		AreaCM2:          areaCM2,
		ViolationDensity: density,
	}
}
