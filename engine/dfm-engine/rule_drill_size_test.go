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

// Per-drill-layer attribution: when a Drill carries a Layer name (e.g.
// "D_1_10"), the violation must be tagged with that layer so the BoardViewer
// can highlight the offending drill span instead of grouping every drill
// violation under the legacy "drill" pseudo-layer.
func TestDrillSize_LayerAttribution(t *testing.T) {
	rule := &DrillSizeRule{}
	board := BoardData{
		Layers: []Layer{
			{Name: "D_1_10", Type: "DRILL"},
			{Name: "D_5_6", Type: "DRILL"},
		},
		Drills: []Drill{
			{X: 5, Y: 5, DiamMM: 0.2, Plated: true, Layer: "D_1_10"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinDrillDiamMM: 0.3}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	if viols[0].Layer != "D_1_10" {
		t.Errorf("expected violation Layer=%q, got %q", "D_1_10", viols[0].Layer)
	}
}

func TestDrillSize_LegacyLayerFallback(t *testing.T) {
	rule := &DrillSizeRule{}
	// Drill with no Layer attribution (older parser output) — must still
	// produce a violation tagged with the legacy "drill" pseudo-layer so
	// existing UI handling continues to work.
	board := drillBoard([]float64{0.2})
	profile := ProfileRules{MinDrillDiamMM: 0.3}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	if viols[0].Layer != "drill" {
		t.Errorf("expected fallback Layer=%q, got %q", "drill", viols[0].Layer)
	}
}
