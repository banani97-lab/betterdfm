package dfmengine

import "testing"

func TestRunner_RunAll_CombinesViolations(t *testing.T) {
	// A board with both a narrow trace and a drill below min → both rules should fire
	board := BoardData{
		Layers:           []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces:           []Trace{{Layer: "top_copper", WidthMM: 0.05, StartX: 5, StartY: 20, EndX: 55, EndY: 20}},
		Drills:           []Drill{{X: 30, Y: 20, DiamMM: 0.15, Plated: true}},
		Outline:          rectOutline(60, 40),
		BoardThicknessMM: 1.6,
	}
	profile := ProfileRules{
		MinTraceWidthMM: 0.1,
		MinDrillDiamMM:  0.2,
	}
	runner := NewRunner()
	viols := runner.Run(board, profile)
	rulesSeen := map[string]bool{}
	for _, v := range viols {
		rulesSeen[v.RuleID] = true
	}
	if !rulesSeen["trace-width"] {
		t.Error("expected trace-width violation")
	}
	if !rulesSeen["drill-size"] {
		t.Error("expected drill-size violation")
	}
}

func TestRunner_EmptyBoard_ZeroViolations(t *testing.T) {
	runner := NewRunner()
	viols := runner.Run(BoardData{}, ProfileRules{
		MinTraceWidthMM:    0.1,
		MinClearanceMM:     0.1,
		MinDrillDiamMM:     0.2,
		MaxDrillDiamMM:     6.0,
		MinAnnularRingMM:   0.15,
		MaxAspectRatio:     6.0,
		MinSolderMaskDamMM: 0.1,
		MinEdgeClearanceMM: 0.2,
	})
	if len(viols) != 0 {
		t.Fatalf("empty board should produce 0 violations, got %d", len(viols))
	}
}
