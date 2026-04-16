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

func TestTraceImbalance_PrefersInsidePourOverNearEdgePour(t *testing.T) {
	// Regression: Dalsa C421. A pad sits inside a same-net pour, but a
	// larger same-net pour has an edge within 0.5 mm of the pad. The
	// inside pour must win; the near-edge pour must be ignored when an
	// inside match exists, otherwise the rule compares the neighboring
	// island (not the one the pad lives in) and flags a false imbalance.
	insidePour := Polygon{
		Layer: "F.Cu", NetName: "VIN",
		Points: []Point{{X: -5, Y: -5}, {X: 5, Y: -5}, {X: 5, Y: 5}, {X: -5, Y: 5}}, // 10x10 bbox, short=10
	}
	// A separate same-net pour whose nearest edge (x=1.2) is within
	// padRadius(1)+0.5 = 1.5 mm of the pad centre, so the near-edge branch
	// would match it if we let it.
	nearEdgePour := Polygon{
		Layer: "F.Cu", NetName: "VIN",
		Points: []Point{{X: 1.2, Y: -10.5}, {X: 22.2, Y: -10.5}, {X: 22.2, Y: 10.5}, {X: 1.2, Y: 10.5}}, // 21x21, short=21
	}
	board := BoardData{
		Layers: []Layer{{Name: "F.Cu", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "F.Cu", X: 0, Y: 0, WidthMM: 2, HeightMM: 2, Shape: "RECT", RefDes: "C1", NetName: "VIN"},
			{Layer: "F.Cu", X: 0, Y: -7, WidthMM: 2, HeightMM: 2, Shape: "RECT", RefDes: "C1", NetName: "GND"},
		},
		// Iteration order is intentionally hostile: near-edge first.
		Polygons: []Polygon{nearEdgePour, insidePour,
			{Layer: "F.Cu", NetName: "GND",
				Points: []Point{{X: -5, Y: -12}, {X: 5, Y: -12}, {X: 5, Y: -2}, {X: -5, Y: -2}}}, // 10x10, short=10
		},
	}
	profile := ProfileRules{MaxTraceImbalanceRatio: 2.0}
	violations := TraceImbalanceRule{}.Run(board, profile)
	if len(violations) != 0 {
		t.Fatalf("expected no violation when pad sits inside a same-size pour on both sides, got %d: %+v",
			len(violations), violations)
	}
}

func TestTraceImbalance_FallsBackToNearEdgeWhenNotInside(t *testing.T) {
	// When the pad isn't inside any same-net pour, near-edge proximity is
	// still the best signal we have. Make sure legitimate thermal-mass
	// imbalance detection via edge-proximity still fires.
	board := BoardData{
		Layers: []Layer{{Name: "F.Cu", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "F.Cu", X: 0, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "A"},
			{Layer: "F.Cu", X: 10, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "B"},
		},
		Polygons: []Polygon{
			{Layer: "F.Cu", NetName: "A",
				Points: []Point{{X: 0.6, Y: -1}, {X: 2.6, Y: -1}, {X: 2.6, Y: 1}, {X: 0.6, Y: 1}}}, // 2x2, short=2
			{Layer: "F.Cu", NetName: "B",
				Points: []Point{{X: 10.6, Y: -15}, {X: 40.6, Y: -15}, {X: 40.6, Y: 15}, {X: 10.6, Y: 15}}}, // 30x30, short=30
		},
	}
	profile := ProfileRules{MaxTraceImbalanceRatio: 2.0}
	violations := TraceImbalanceRule{}.Run(board, profile)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation from near-edge fallback, got %d", len(violations))
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
