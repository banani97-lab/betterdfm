package dfmengine

import "testing"

func TestScore_NoViolations(t *testing.T) {
	result := ComputeScore(nil, rectOutline(60, 40))
	if result.Score != 100 {
		t.Errorf("expected score=100 with no violations, got %d", result.Score)
	}
	if result.Grade != "A" {
		t.Errorf("expected grade=A, got %s", result.Grade)
	}
}

func TestScore_SevereClearance(t *testing.T) {
	// Many clearance errors should max out the clearance cap (15 pts) → score ≤ 85.
	viols := make([]Violation, 50)
	for i := range viols {
		viols[i] = Violation{
			RuleID:     "clearance",
			Severity:   "ERROR",
			Layer:      "top_copper",
			X:          float64(i) * 2,
			Y:          0,
			MeasuredMM: 0.01,
			LimitMM:    0.1,
			Count:      10,
		}
	}
	result := ComputeScore(viols, rectOutline(60, 40))
	if result.Score >= 86 {
		t.Errorf("expected score < 86 with many clearance errors (cap=15), got %d", result.Score)
	}
}

func TestScore_MultipleRules_AtMost100(t *testing.T) {
	viols := []Violation{
		{RuleID: "trace-width", Severity: "ERROR", MeasuredMM: 0.05, LimitMM: 0.1},
		{RuleID: "clearance", Severity: "ERROR", MeasuredMM: 0.03, LimitMM: 0.1},
	}
	result := ComputeScore(viols, rectOutline(60, 40))
	if result.Score > 100 {
		t.Errorf("score should never exceed 100, got %d", result.Score)
	}
	if result.Score < 0 {
		t.Errorf("score should never be negative, got %d", result.Score)
	}
}

func TestScore_GradeA(t *testing.T) {
	// One minor info violation on a large board → grade A
	viols := []Violation{
		{RuleID: "edge-clearance", Severity: "INFO", MeasuredMM: 0.19, LimitMM: 0.2, Count: 1},
	}
	result := ComputeScore(viols, rectOutline(100, 100))
	if result.Score < 90 {
		t.Errorf("expected grade A (score≥90) for minor violation on large board, got %d", result.Score)
	}
}

func TestScore_GradeF(t *testing.T) {
	// Max violations for many rules → should score very low (F)
	var viols []Violation
	rules := []string{"clearance", "trace-width", "annular-ring", "drill-size",
		"drill-to-copper", "drill-to-drill", "aspect-ratio", "edge-clearance",
		"solder-mask-dam", "package-capability", "trace-imbalance"}
	for _, ruleID := range rules {
		for i := 0; i < 100; i++ {
			viols = append(viols, Violation{
				RuleID:     ruleID,
				Severity:   "ERROR",
				MeasuredMM: 0.001,
				LimitMM:    0.1,
				Count:      500,
				X:          float64(i) * 5,
				Y:          float64(i) * 5,
			})
		}
	}
	result := ComputeScore(viols, rectOutline(60, 40))
	if result.Score >= 40 {
		t.Errorf("expected grade F (score<40) with max violations, got %d", result.Score)
	}
}
