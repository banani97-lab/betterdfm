package dfmengine

import "testing"

func TestDrillSize_BelowMin(t *testing.T) {
	rule := &DrillSizeRule{}
	board := drillBoard([]float64{0.2})
	profile := ProfileRules{MinDrillDiamMM: 0.3}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	if viols[0].Severity != "ERROR" {
		t.Errorf("expected ERROR, got %s", viols[0].Severity)
	}
}

func TestDrillSize_AboveMax(t *testing.T) {
	rule := &DrillSizeRule{}
	board := drillBoard([]float64{4.0})
	profile := ProfileRules{MaxDrillDiamMM: 3.0}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
}

func TestDrillSize_InRange(t *testing.T) {
	rule := &DrillSizeRule{}
	board := drillBoard([]float64{0.5})
	profile := ProfileRules{MinDrillDiamMM: 0.3, MaxDrillDiamMM: 3.0}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(viols))
	}
}

func TestDrillSize_ViaDrillChecked(t *testing.T) {
	rule := &DrillSizeRule{}
	board := viaBoard(0.8, 0.2)
	board.Vias[0].DrillDiamMM = 0.2
	profile := ProfileRules{MinDrillDiamMM: 0.3}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation for via drill, got %d", len(viols))
	}
}
