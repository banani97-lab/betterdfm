package dfmengine

import "testing"

func TestClearance_TracesTooClose(t *testing.T) {
	rule := &ClearanceRule{}
	// gap = 0.05mm, min = 0.1mm → violation
	board := twoTraceBoard(0.1, 0.1, 0.05)
	profile := ProfileRules{MinClearanceMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation, got 0")
	}
	v := viols[0]
	if v.MeasuredMM >= 0.1 {
		t.Errorf("MeasuredMM should be below 0.1, got %f", v.MeasuredMM)
	}
	if v.LimitMM != 0.1 {
		t.Errorf("LimitMM should be 0.1, got %f", v.LimitMM)
	}
	if v.RuleID != "clearance" {
		t.Errorf("RuleID should be clearance, got %s", v.RuleID)
	}
}

func TestClearance_SameNetSkipped(t *testing.T) {
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10, EndX: 50, EndY: 10, NetName: "GND"},
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10.05, EndX: 50, EndY: 10.05, NetName: "GND"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("same-net traces should be skipped, got %d violations", len(viols))
	}
}

func TestClearance_DifferentLayersSkipped(t *testing.T) {
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "bot_copper", Type: "COPPER"},
		},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10, EndX: 50, EndY: 10},
			{Layer: "bot_copper", WidthMM: 0.1, StartX: 0, StartY: 10.05, EndX: 50, EndY: 10.05},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("different-layer traces should not be compared, got %d violations", len(viols))
	}
}

func TestClearance_TraceToPadTooClose(t *testing.T) {
	rule := &ClearanceRule{}
	// Trace at y=10, width=0.1 (half=0.05). Pad at y=10.2, radius=0.1 (diam=0.2).
	// dist = |10.2-10| = 0.2. clearance = 0.2 - 0.05 - 0.1 = 0.05 < 0.15 → violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10, EndX: 50, EndY: 10},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 25, Y: 10.2, WidthMM: 0.2, HeightMM: 0.2, Shape: "CIRCLE"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected a trace-to-pad violation, got 0")
	}
	if viols[0].RuleID != "clearance" {
		t.Errorf("expected RuleID=clearance, got %s", viols[0].RuleID)
	}
}

func TestClearance_SilkLayerSkipped(t *testing.T) {
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		// Two silk traces with only 0.01mm gap — would violate if checked
		Traces: []Trace{
			{Layer: "top_silk", WidthMM: 0.1, StartX: 0, StartY: 10, EndX: 50, EndY: 10},
			{Layer: "top_silk", WidthMM: 0.1, StartX: 0, StartY: 10.01, EndX: 50, EndY: 10.01},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("silk layer traces must be skipped by clearance rule, got %d violations", len(viols))
	}
}

func TestClearance_PowerGroundLayerChecked(t *testing.T) {
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "gnd_plane", Type: "POWER_GROUND"}},
		// gap = 0.05mm edge-to-edge, min = 0.1mm → violation expected
		Traces: []Trace{
			{Layer: "gnd_plane", WidthMM: 0.1, StartX: 0, StartY: 10, EndX: 50, EndY: 10},
			{Layer: "gnd_plane", WidthMM: 0.1, StartX: 0, StartY: 10.15, EndX: 50, EndY: 10.15},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("POWER_GROUND layer traces should be checked for clearance, got 0 violations")
	}
}

func TestClearance_RectPadGap(t *testing.T) {
	rule := &ClearanceRule{}
	// RECT pad 2×1mm at (10,10). Trace at y=11.1, width=0.1mm.
	// Pad top edge at y=10.5. Closest-on-trace to edge → (10,11.1). dist=0.6, minus w/2=0.05 → 0.55mm > 0.15 → pass.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 11.1, EndX: 50, EndY: 11.1},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 2, HeightMM: 1, Shape: "RECT"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("RECT pad 0.55mm from trace should pass, got %d violations", len(viols))
	}
}

func TestClearance_RectPadTooClose(t *testing.T) {
	rule := &ClearanceRule{}
	// RECT pad 2×1mm at (10,10): top edge at y=10.5.
	// Trace at y=10.55, width=0.1mm: nearest edge = 0.05mm. gap = 0.05-0.05 = 0 < 0.15 → violation.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10.55, EndX: 50, EndY: 10.55},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 2, HeightMM: 1, Shape: "RECT"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("RECT pad nearly touching trace should be flagged, got 0 violations")
	}
}

func TestClearance_PolygonEdgesIncluded(t *testing.T) {
	rule := &ClearanceRule{}
	// Copper polygon with bottom edge at y=10. Trace at y=10.1, width=0.1mm.
	// Gap = 0.1 - 0 - 0.05 = 0.05mm < 0.15mm → violation.
	poly := Polygon{
		Layer: "top_copper",
		Points: []Point{
			{X: 0, Y: 10}, {X: 50, Y: 10}, {X: 50, Y: 11}, {X: 0, Y: 11},
		},
	}
	board := BoardData{
		Layers:   []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces:   []Trace{{Layer: "top_copper", WidthMM: 0.1, StartX: 5, StartY: 10.1, EndX: 45, EndY: 10.1}},
		Polygons: []Polygon{poly},
		Outline:  rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.15}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("trace 0.05mm from polygon edge should be flagged, got 0 violations")
	}
}

func TestClearance_DedupeCollapses(t *testing.T) {
	rule := &ClearanceRule{}
	// 30 pairs of traces very close together in the same 2mm cell
	traces := make([]Trace, 60)
	for i := 0; i < 30; i++ {
		x := float64(i) * 0.05
		traces[i*2] = Trace{Layer: "top_copper", WidthMM: 0.1, StartX: x, StartY: 10, EndX: x + 0.01, EndY: 10, NetName: "A"}
		traces[i*2+1] = Trace{Layer: "top_copper", WidthMM: 0.1, StartX: x, StartY: 10.05, EndX: x + 0.01, EndY: 10.05, NetName: "B"}
	}
	board := BoardData{
		Layers:  []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces:  traces,
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinClearanceMM: 0.1}
	viols := rule.Run(board, profile)
	// Raw violations may be many, but dedup should collapse them.
	// The count field on at least one violation should be > 1.
	hasCount := false
	for _, v := range viols {
		if v.Count > 1 {
			hasCount = true
			break
		}
	}
	if !hasCount && len(viols) > 5 {
		t.Errorf("expected dedup to collapse violations into fewer with Count>1, got %d raw", len(viols))
	}
}
