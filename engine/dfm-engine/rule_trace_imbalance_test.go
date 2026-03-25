package dfmengine

import "testing"

func TestTraceImbalance_Flagged(t *testing.T) {
	board := BoardData{
		Layers: []Layer{{Name: "F.Cu", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "F.Cu", X: 0, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET1"},
			{Layer: "F.Cu", X: 5, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET2"},
		},
		Traces: []Trace{
			{Layer: "F.Cu", WidthMM: 0.3, StartX: -1, StartY: 0, EndX: 0, EndY: 0, NetName: "NET1"},
			{Layer: "F.Cu", WidthMM: 0.1, StartX: 6, StartY: 0, EndX: 5, EndY: 0, NetName: "NET2"},
		},
	}
	profile := ProfileRules{MaxTraceImbalanceRatio: 2.0}
	violations := TraceImbalanceRule{}.Run(board, profile)
	if len(violations) == 0 {
		t.Fatal("expected violation for 3:1 trace width ratio")
	}
	if violations[0].RuleID != "trace-imbalance" {
		t.Fatalf("expected rule ID trace-imbalance, got %s", violations[0].RuleID)
	}
}

func TestTraceImbalance_UnderThreshold(t *testing.T) {
	board := BoardData{
		Layers: []Layer{{Name: "F.Cu", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "F.Cu", X: 0, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET1"},
			{Layer: "F.Cu", X: 5, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET2"},
		},
		Traces: []Trace{
			{Layer: "F.Cu", WidthMM: 0.2, StartX: -1, StartY: 0, EndX: 0, EndY: 0, NetName: "NET1"},
			{Layer: "F.Cu", WidthMM: 0.15, StartX: 6, StartY: 0, EndX: 5, EndY: 0, NetName: "NET2"},
		},
	}
	profile := ProfileRules{MaxTraceImbalanceRatio: 2.0}
	violations := TraceImbalanceRule{}.Run(board, profile)
	if len(violations) != 0 {
		t.Fatalf("expected no violations for 1.33:1 ratio, got %d", len(violations))
	}
}

func TestTraceImbalance_OnePadUnconnected(t *testing.T) {
	board := BoardData{
		Layers: []Layer{{Name: "F.Cu", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "F.Cu", X: 0, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET1"},
			{Layer: "F.Cu", X: 5, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET2"},
		},
		Traces: []Trace{
			{Layer: "F.Cu", WidthMM: 0.3, StartX: -1, StartY: 0, EndX: 0, EndY: 0, NetName: "NET1"},
			// No trace connects to the second pad
		},
	}
	profile := ProfileRules{MaxTraceImbalanceRatio: 2.0}
	violations := TraceImbalanceRule{}.Run(board, profile)
	if len(violations) != 0 {
		t.Fatalf("expected no violations when one pad has no connected trace, got %d", len(violations))
	}
}

func TestTraceImbalance_ThreePadComponent(t *testing.T) {
	board := BoardData{
		Layers: []Layer{{Name: "F.Cu", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "F.Cu", X: 0, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "U1", NetName: "NET1"},
			{Layer: "F.Cu", X: 5, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "U1", NetName: "NET2"},
			{Layer: "F.Cu", X: 10, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "U1", NetName: "NET3"},
		},
		Traces: []Trace{
			{Layer: "F.Cu", WidthMM: 0.5, StartX: -1, StartY: 0, EndX: 0, EndY: 0, NetName: "NET1"},
			{Layer: "F.Cu", WidthMM: 0.1, StartX: 6, StartY: 0, EndX: 5, EndY: 0, NetName: "NET2"},
		},
	}
	profile := ProfileRules{MaxTraceImbalanceRatio: 2.0}
	violations := TraceImbalanceRule{}.Run(board, profile)
	if len(violations) != 0 {
		t.Fatalf("expected no violations for 3-pad component, got %d", len(violations))
	}
}

func TestTraceImbalance_Disabled(t *testing.T) {
	board := BoardData{
		Layers: []Layer{{Name: "F.Cu", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "F.Cu", X: 0, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET1"},
			{Layer: "F.Cu", X: 5, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "NET2"},
		},
		Traces: []Trace{
			{Layer: "F.Cu", WidthMM: 0.5, StartX: -1, StartY: 0, EndX: 0, EndY: 0, NetName: "NET1"},
			{Layer: "F.Cu", WidthMM: 0.1, StartX: 6, StartY: 0, EndX: 5, EndY: 0, NetName: "NET2"},
		},
	}
	profile := ProfileRules{MaxTraceImbalanceRatio: 0} // disabled
	violations := TraceImbalanceRule{}.Run(board, profile)
	if len(violations) != 0 {
		t.Fatalf("expected no violations when rule disabled, got %d", len(violations))
	}
}
