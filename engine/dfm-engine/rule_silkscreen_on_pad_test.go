package dfmengine

import "testing"

func TestSilkscreenOnPad_Violation(t *testing.T) {
	rule := &SilkscreenOnPadRule{}
	// Silk trace centered at y=10 overlaps a copper pad at (10,10).
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		Traces: []Trace{
			{Layer: "top_silk", WidthMM: 0.12, StartX: 5, StartY: 10, EndX: 15, EndY: 10},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "RECT", RefDes: "U1"},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) == 0 {
		t.Fatal("expected ≥1 silkscreen-on-pad violation, got 0")
	}
	v := viols[0]
	if v.RuleID != "silkscreen-on-pad" {
		t.Errorf("expected RuleID=silkscreen-on-pad, got %s", v.RuleID)
	}
	if v.Severity != "WARNING" {
		t.Errorf("expected WARNING severity, got %s", v.Severity)
	}
	if v.RefDes != "U1" {
		t.Errorf("expected RefDes=U1, got %s", v.RefDes)
	}
}

func TestSilkscreenOnPad_NoOverlap(t *testing.T) {
	rule := &SilkscreenOnPadRule{}
	// Silk trace at y=20, copper pad at y=10 — no overlap.
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		Traces: []Trace{
			{Layer: "top_silk", WidthMM: 0.12, StartX: 5, StartY: 20, EndX: 15, EndY: 20},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "RECT", RefDes: "U1"},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 0 {
		t.Fatalf("no overlap expected, got %d violations", len(viols))
	}
}

func TestSilkscreenOnPad_DifferentSideSkipped(t *testing.T) {
	rule := &SilkscreenOnPadRule{}
	// Bottom silk trace overlapping a top copper pad — different sides, should be skipped.
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "bot_silk", Type: "SILK"},
		},
		Traces: []Trace{
			{Layer: "bot_silk", WidthMM: 0.12, StartX: 5, StartY: 10, EndX: 15, EndY: 10},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "RECT", RefDes: "U1"},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 0 {
		t.Fatalf("different-side silk/copper should not be flagged, got %d violations", len(viols))
	}
}

func TestSilkscreenOnPad_NoCopperPads(t *testing.T) {
	rule := &SilkscreenOnPadRule{}
	// Silk trace but no copper pads — should return no violations.
	board := BoardData{
		Layers: []Layer{
			{Name: "top_silk", Type: "SILK"},
		},
		Traces: []Trace{
			{Layer: "top_silk", WidthMM: 0.12, StartX: 5, StartY: 10, EndX: 15, EndY: 10},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 0 {
		t.Fatalf("no copper pads means no violations, got %d", len(viols))
	}
}

func TestSilkscreenOnPad_CopperTraceNotChecked(t *testing.T) {
	rule := &SilkscreenOnPadRule{}
	// Copper traces are not pads — silk overlapping a copper trace is NOT flagged by this rule.
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		Traces: []Trace{
			{Layer: "top_silk", WidthMM: 0.12, StartX: 5, StartY: 10, EndX: 15, EndY: 10},
			{Layer: "top_copper", WidthMM: 0.15, StartX: 5, StartY: 10, EndX: 15, EndY: 10},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 0 {
		t.Fatalf("silk over copper trace should not be flagged (only pads matter), got %d violations", len(viols))
	}
}
