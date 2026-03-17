package dfmengine

import "testing"

func TestDrillToCopper_TooClose(t *testing.T) {
	rule := &DrillToCopperRule{}
	// Drill at (10,10), radius=0.15mm (diam=0.3). Trace at y=10.3, width=0.1mm (half=0.05).
	// dist from drill center to trace = 0.3. gap = 0.3 - 0.15 - 0.05 = 0.1mm < 0.25mm → violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10.3, EndX: 30, EndY: 10.3},
		},
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 0.3, Plated: true}},
		Outline: rectOutline(40, 30),
	}
	profile := ProfileRules{MinDrillToCopperMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation, got 0")
	}
	v := viols[0]
	if v.RuleID != "drill-to-copper" {
		t.Errorf("expected RuleID=drill-to-copper, got %s", v.RuleID)
	}
	if v.MeasuredMM >= 0.25 {
		t.Errorf("MeasuredMM should be below 0.25, got %f", v.MeasuredMM)
	}
}

func TestDrillToCopper_OK(t *testing.T) {
	rule := &DrillToCopperRule{}
	// Drill at (10,10), radius=0.15mm. Trace at y=10.5, width=0.1mm.
	// gap = 0.5 - 0.15 - 0.05 = 0.3mm > 0.25mm → no violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10.5, EndX: 30, EndY: 10.5},
		},
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 0.3, Plated: true}},
		Outline: rectOutline(40, 30),
	}
	profile := ProfileRules{MinDrillToCopperMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(viols))
	}
}

func TestDrillToCopper_NoProfile(t *testing.T) {
	rule := &DrillToCopperRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10.2, EndX: 30, EndY: 10.2},
		},
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 0.3, Plated: true}},
		Outline: rectOutline(40, 30),
	}
	profile := ProfileRules{} // MinDrillToCopperMM = 0 → disabled
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("rule should be disabled when MinDrillToCopperMM=0, got %d violations", len(viols))
	}
}

func TestDrillToCopper_AnnularRingSkipped(t *testing.T) {
	rule := &DrillToCopperRule{}
	// Drill at (10,10), radius=0.15mm. Pad centered on same point (annular ring): overlapping → gap < 0 → skip.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 0.6, HeightMM: 0.6, Shape: "CIRCLE"},
		},
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 0.3, Plated: true}},
		Outline: rectOutline(40, 30),
	}
	profile := ProfileRules{MinDrillToCopperMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("annular ring pad (overlapping) should not generate DFM violations, got %d", len(viols))
	}
}

func TestDrillToCopper_PadTooClose(t *testing.T) {
	rule := &DrillToCopperRule{}
	// Drill at (10,10), radius=0.15mm. Nearby pad at (10.3, 10), radius=0.05mm.
	// dist = 0.3. gap = 0.3 - 0.15 - 0.05 = 0.1mm < 0.25mm → violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10.3, Y: 10, WidthMM: 0.1, HeightMM: 0.1, Shape: "CIRCLE"},
		},
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 0.3, Plated: true}},
		Outline: rectOutline(40, 30),
	}
	profile := ProfileRules{MinDrillToCopperMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected violation for pad too close to drill, got 0")
	}
}

func TestDrillToCopper_SilkSkipped(t *testing.T) {
	rule := &DrillToCopperRule{}
	// Silk trace very close to drill — should be ignored (silk is not copper).
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		Traces: []Trace{
			{Layer: "top_silk", WidthMM: 0.1, StartX: 0, StartY: 10.2, EndX: 30, EndY: 10.2},
		},
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 0.3, Plated: true}},
		Outline: rectOutline(40, 30),
	}
	profile := ProfileRules{MinDrillToCopperMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("silk traces should not trigger drill-to-copper rule, got %d violations", len(viols))
	}
}
