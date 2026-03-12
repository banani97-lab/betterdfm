package dfmengine

import "testing"

func TestSolderMaskDam_BelowMin(t *testing.T) {
	rule := &SolderMaskDamRule{}
	// Two 1mm pads with 0.05mm gap → dam = 0.05mm, limit = 0.1mm → violation
	board := padPairBoard(0.05, 1.0)
	profile := ProfileRules{MinSolderMaskDamMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation, got 0")
	}
	if viols[0].Severity != "WARNING" {
		t.Errorf("expected WARNING severity, got %s", viols[0].Severity)
	}
}

func TestSolderMaskDam_Passes(t *testing.T) {
	rule := &SolderMaskDamRule{}
	// Two 1mm pads with 0.5mm gap → dam = 0.5mm, limit = 0.1mm → no violation
	board := padPairBoard(0.5, 1.0)
	profile := ProfileRules{MinSolderMaskDamMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(viols))
	}
}

func TestSolderMaskDam_InnerLayerSkipped(t *testing.T) {
	rule := &SolderMaskDamRule{}
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "inner1", Type: "COPPER"},
			{Name: "bot_copper", Type: "COPPER"},
		},
		Pads: []Pad{
			// Two inner-layer pads very close together
			{Layer: "inner1", X: 10, Y: 20, WidthMM: 1.0, HeightMM: 1.0, Shape: "CIRCLE"},
			{Layer: "inner1", X: 11.05, Y: 20, WidthMM: 1.0, HeightMM: 1.0, Shape: "CIRCLE"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinSolderMaskDamMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("inner-layer pads should be skipped, got %d violations", len(viols))
	}
}
