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

// Per-layer coverage: when a multi-layer through-hole has different catch-pad
// sizes on different copper layers, the inner-layer pad with the smaller ring
// must be flagged independently. This is the regression case Option B was
// designed for — previously the rule iterated stacked Vias which gave coverage
// "by accident"; now it walks per-layer DONUT pads.
func TestAnnularRing_PerLayerCatchPad_InnerLayerFlagged(t *testing.T) {
	rule := &AnnularRingRule{}
	// Drill 0.6mm at (30, 20). Top catch-pad 1.0mm DONUT (ring 0.2mm, passes).
	// Inner layer catch-pad 0.8mm DONUT (ring 0.1mm, fails 0.15mm limit).
	board := BoardData{
		Layers: []Layer{
			{Name: "signal_1", Type: "COPPER"},
			{Name: "signal_5", Type: "COPPER"},
		},
		Drills: []Drill{{X: 30, Y: 20, DiamMM: 0.6, Plated: true}},
		Pads: []Pad{
			{Layer: "signal_1", X: 30, Y: 20, WidthMM: 1.0, HeightMM: 1.0,
				Shape: "DONUT", HoleMM: 0.6, IsViaCatchPad: true},
			{Layer: "signal_5", X: 30, Y: 20, WidthMM: 0.8, HeightMM: 0.8,
				Shape: "DONUT", HoleMM: 0.6, IsViaCatchPad: true},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinAnnularRingMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation (inner layer only), got %d", len(viols))
	}
	if viols[0].Layer != "signal_5" {
		t.Errorf("expected violation on signal_5 (inner layer), got %q", viols[0].Layer)
	}
	if viols[0].MeasuredMM < 0.099 || viols[0].MeasuredMM > 0.101 {
		t.Errorf("expected MeasuredMM ≈ 0.1, got %f", viols[0].MeasuredMM)
	}
}

// Boards that don't carry per-layer catch-pad records (older parser output, or
// drill-only fixtures) must still be checked via the Via fallback.
func TestAnnularRing_ViaFallback_WhenNoCatchPads(t *testing.T) {
	rule := &AnnularRingRule{}
	// No Pads — the rule must fall back to checking Vias directly.
	board := viaBoard(0.7, 0.6) // ring = 0.05mm
	profile := ProfileRules{MinAnnularRingMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation from Via fallback, got %d", len(viols))
	}
}

// When per-layer catch-pads are present for a via, the rule must NOT also
// fire the Via fallback for that location — that would double-count.
func TestAnnularRing_NoDoubleCounting_WhenBothPadsAndViasPresent(t *testing.T) {
	rule := &AnnularRingRule{}
	board := BoardData{
		Layers: []Layer{{Name: "signal_1", Type: "COPPER"}},
		Drills: []Drill{{X: 30, Y: 20, DiamMM: 0.6, Plated: true}},
		Pads: []Pad{
			{Layer: "signal_1", X: 30, Y: 20, WidthMM: 0.8, HeightMM: 0.8,
				Shape: "DONUT", HoleMM: 0.6, IsViaCatchPad: true},
		},
		// A redundant Via at the same location, e.g. from drill-layer attrs.
		Vias:    []Via{{X: 30, Y: 20, OuterDiamMM: 0.8, DrillDiamMM: 0.6}},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinAnnularRingMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation (no double-counting), got %d", len(viols))
	}
	if viols[0].Layer != "signal_1" {
		t.Errorf("expected per-layer attribution, got Layer=%q", viols[0].Layer)
	}
}
