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

func TestSolderMaskDam_RectPads(t *testing.T) {
	rule := &SolderMaskDamRule{}
	// Two RECT 2×1mm pads. padProjection along X = 1mm each.
	// Centers 2.05mm apart → edgeDist = 2.05 - 1 - 1 = 0.05mm < 0.1mm limit → violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 20, WidthMM: 2, HeightMM: 1, Shape: "RECT"},
			{Layer: "top_copper", X: 12.05, Y: 20, WidthMM: 2, HeightMM: 1, Shape: "RECT"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinSolderMaskDamMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("RECT pads with 0.05mm dam should fail solder mask dam check, got 0 violations")
	}
}

func TestSolderMaskDam_RectPadsClearEnough(t *testing.T) {
	rule := &SolderMaskDamRule{}
	// Two RECT 1×1mm pads at x=10 and x=11.5. padProjection along X = 0.5 each.
	// dam = 1.5 - 0.5 - 0.5 = 0.5mm > 0.1mm → pass.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 20, WidthMM: 1, HeightMM: 1, Shape: "RECT"},
			{Layer: "top_copper", X: 11.5, Y: 20, WidthMM: 1, HeightMM: 1, Shape: "RECT"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinSolderMaskDamMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("RECT pads with 0.5mm dam should pass, got %d violations", len(viols))
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
