package dfmengine

import "testing"

func TestCopperSliver_Violation(t *testing.T) {
	rule := &CopperSliverRule{}
	// Un-netted trace, 0.05mm wide → below 0.1mm limit → violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.05, StartX: 5, StartY: 10, EndX: 15, EndY: 10, NetName: ""},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinCopperSliverMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation for copper sliver, got 0")
	}
	v := viols[0]
	if v.RuleID != "copper-sliver" {
		t.Errorf("expected RuleID=copper-sliver, got %s", v.RuleID)
	}
	if v.Severity != "WARNING" {
		t.Errorf("expected WARNING severity, got %s", v.Severity)
	}
	if v.MeasuredMM != 0.05 {
		t.Errorf("expected MeasuredMM=0.05, got %f", v.MeasuredMM)
	}
}

func TestCopperSliver_NettedTraceSkipped(t *testing.T) {
	rule := &CopperSliverRule{}
	// Thin but netted trace — intentional signal, should not be flagged as sliver.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.05, StartX: 5, StartY: 10, EndX: 15, EndY: 10, NetName: "SIG1"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinCopperSliverMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("netted trace must not be flagged as copper sliver, got %d violations", len(viols))
	}
}

func TestCopperSliver_WideUnnetted_OK(t *testing.T) {
	rule := &CopperSliverRule{}
	// Wide un-netted trace (pour fill) — wide enough, no violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.5, StartX: 5, StartY: 10, EndX: 15, EndY: 10, NetName: ""},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinCopperSliverMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("wide un-netted trace is not a sliver, expected 0 violations, got %d", len(viols))
	}
}

func TestCopperSliver_NoProfile(t *testing.T) {
	rule := &CopperSliverRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.01, StartX: 5, StartY: 10, EndX: 15, EndY: 10, NetName: ""},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{} // MinCopperSliverMM = 0 → disabled
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("rule should be disabled when MinCopperSliverMM=0, got %d violations", len(viols))
	}
}

func TestCopperSliver_SilkLayerSkipped(t *testing.T) {
	rule := &CopperSliverRule{}
	// Very thin trace on silk layer — must not trigger copper sliver rule.
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		Traces: []Trace{
			{Layer: "top_silk", WidthMM: 0.01, StartX: 5, StartY: 10, EndX: 15, EndY: 10, NetName: ""},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinCopperSliverMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("silk layer traces must not trigger copper-sliver rule, got %d violations", len(viols))
	}
}
