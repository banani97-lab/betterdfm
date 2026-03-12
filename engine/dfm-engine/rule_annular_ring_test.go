package dfmengine

import "testing"

func TestAnnularRing_BelowMin(t *testing.T) {
	rule := &AnnularRingRule{}
	// ring = (0.8 - 0.6) / 2 = 0.1mm, limit = 0.15mm → violation
	board := viaBoard(0.8, 0.6)
	profile := ProfileRules{MinAnnularRingMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	if viols[0].MeasuredMM < 0.099 || viols[0].MeasuredMM > 0.101 {
		t.Errorf("expected MeasuredMM≈0.1, got %f", viols[0].MeasuredMM)
	}
}

func TestAnnularRing_Passes(t *testing.T) {
	rule := &AnnularRingRule{}
	// ring = (1.0 - 0.6) / 2 = 0.2mm, limit = 0.15mm → no violation
	board := viaBoard(1.0, 0.6)
	profile := ProfileRules{MinAnnularRingMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(viols))
	}
}

func TestAnnularRing_ZeroOuterSkipped(t *testing.T) {
	rule := &AnnularRingRule{}
	board := BoardData{
		Vias: []Via{{X: 10, Y: 10, OuterDiamMM: 0, DrillDiamMM: 0.3}},
	}
	profile := ProfileRules{MinAnnularRingMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("zero outer diam should be skipped, got %d violations", len(viols))
	}
}
