package dfmengine

import (
	"strings"
	"testing"
)

func containsStr(s, sub string) bool { return strings.Contains(s, sub) }

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
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10, EndX: 50, EndY: 10, NetName: "SIG1"},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 25, Y: 10.2, WidthMM: 0.2, HeightMM: 0.2, Shape: "CIRCLE", NetName: "SIG2"},
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
		Traces: []Trace{
			{Layer: "gnd_plane", WidthMM: 0.1, StartX: 0, StartY: 10, EndX: 50, EndY: 10, NetName: "GND"},
			{Layer: "gnd_plane", WidthMM: 0.1, StartX: 0, StartY: 10.15, EndX: 50, EndY: 10.15, NetName: "VCC"},
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
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 11.1, EndX: 50, EndY: 11.1, NetName: "SIG1"},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 2, HeightMM: 1, Shape: "RECT", NetName: "SIG2"},
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
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 10.55, EndX: 50, EndY: 10.55, NetName: "SIG1"},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 2, HeightMM: 1, Shape: "RECT", NetName: "SIG2"},
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
		Layer:   "top_copper",
		NetName: "GND",
		Points: []Point{
			{X: 0, Y: 10}, {X: 50, Y: 10}, {X: 50, Y: 11}, {X: 0, Y: 11},
		},
	}
	board := BoardData{
		Layers:   []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces:   []Trace{{Layer: "top_copper", WidthMM: 0.1, StartX: 5, StartY: 10.1, EndX: 45, EndY: 10.1, NetName: "SIG1"}},
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

// ── Pad-to-pad clearance + overlap-as-short (P2) ─────────────────────────────

func padPairNetBoard(gapMM float64, netA, netB string) BoardData {
	x1 := 10.0
	x2 := x1 + 1.0 + gapMM // 1mm pads: edge-to-edge gap = centerDist - 1
	return BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: x1, Y: 20, WidthMM: 1, HeightMM: 1, Shape: "CIRCLE", NetName: netA, NetSource: "attr"},
			{Layer: "top_copper", X: x2, Y: 20, WidthMM: 1, HeightMM: 1, Shape: "CIRCLE", NetName: netB, NetSource: "attr"},
		},
		Outline: rectOutline(60, 40),
	}
}

func TestClearance_PadPairTooClose(t *testing.T) {
	rule := &ClearanceRule{}
	board := padPairNetBoard(0.05, "NET_A", "NET_B")
	viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1})
	if len(viols) != 1 {
		t.Fatalf("expected 1 pad-pad violation, got %d", len(viols))
	}
	v := viols[0]
	if v.Severity != "ERROR" {
		t.Errorf("severity = %s, want ERROR", v.Severity)
	}
	if v.MeasuredMM < 0.049 || v.MeasuredMM > 0.051 {
		t.Errorf("MeasuredMM = %f, want ~0.05", v.MeasuredMM)
	}
	if v.X2 == 0 && v.Y2 == 0 {
		t.Error("X2/Y2 should point at pad B")
	}
}

func TestClearance_PadPairSameNetSkipped(t *testing.T) {
	rule := &ClearanceRule{}
	board := padPairNetBoard(0.05, "GND", "GND")
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("same-net pad pair should be skipped, got %d violations", len(viols))
	}
}

func TestClearance_PadPairUnknownNetSkipped(t *testing.T) {
	rule := &ClearanceRule{}
	board := padPairNetBoard(0.05, "NET_A", "")
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("unknown-net pad pair should be skipped, got %d violations", len(viols))
	}
	board = padPairNetBoard(0.05, "NET_A", "$NONE$")
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("$NONE$ pad pair should be skipped, got %d violations", len(viols))
	}
}

func TestClearance_PadPairOkGapNoViolation(t *testing.T) {
	rule := &ClearanceRule{}
	board := padPairNetBoard(0.2, "NET_A", "NET_B")
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("0.2mm gap with 0.1mm limit should pass, got %d violations", len(viols))
	}
}

func TestClearance_PadOverlapIsShort(t *testing.T) {
	rule := &ClearanceRule{}
	board := padPairNetBoard(-0.2, "NET_A", "NET_B") // overlapping
	viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1})
	if len(viols) != 1 {
		t.Fatalf("expected 1 short violation, got %d", len(viols))
	}
	v := viols[0]
	if v.MeasuredMM != 0 {
		t.Errorf("short MeasuredMM = %f, want 0", v.MeasuredMM)
	}
	if v.Severity != "ERROR" {
		t.Errorf("severity = %s, want ERROR", v.Severity)
	}
	if want := "probable short"; !containsStr(v.Message, want) {
		t.Errorf("message %q should contain %q", v.Message, want)
	}
}

func TestClearance_TraceOverlapIsShort(t *testing.T) {
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.2, StartX: 0, StartY: 10, EndX: 50, EndY: 10, NetName: "NET_A", NetSource: "attr"},
			{Layer: "top_copper", WidthMM: 0.2, StartX: 25, StartY: 0, EndX: 25, EndY: 20, NetName: "NET_B", NetSource: "attr"},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1})
	if len(viols) != 1 {
		t.Fatalf("expected 1 short violation for crossing traces, got %d", len(viols))
	}
	if viols[0].MeasuredMM != 0 {
		t.Errorf("short MeasuredMM = %f, want 0", viols[0].MeasuredMM)
	}
	if !containsStr(viols[0].Message, "probable short") {
		t.Errorf("message %q should mention probable short", viols[0].Message)
	}
}

func TestClearance_TraceOverlapUnknownNetStillSkipped(t *testing.T) {
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.2, StartX: 0, StartY: 10, EndX: 50, EndY: 10, NetName: ""},
			{Layer: "top_copper", WidthMM: 0.2, StartX: 25, StartY: 0, EndX: 25, EndY: 20, NetName: "NET_B"},
		},
		Outline: rectOutline(60, 40),
	}
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("unknown-net overlap must not be flagged as a short, got %d", len(viols))
	}
}

func TestClearance_ChainedTraceOverlapNotShort(t *testing.T) {
	// Two segments sharing an endpoint (a routed chain) with conflicting net
	// labels must NOT be flagged as a short — that's a label disagreement,
	// not a crossing.
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.2, StartX: 0, StartY: 10, EndX: 25, EndY: 10, NetName: "NET_A"},
			{Layer: "top_copper", WidthMM: 0.2, StartX: 25, StartY: 10, EndX: 50, EndY: 10, NetName: "NET_B"},
		},
		Outline: rectOutline(60, 40),
	}
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("chained segments must not short, got %d violations", len(viols))
	}
}

func TestClearance_TraceEndingInPadNotShort(t *testing.T) {
	// A trace terminating inside a pad is the intended connection even when
	// the labels disagree.
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.2, StartX: 5, StartY: 20, EndX: 30, EndY: 20, NetName: "NET_A"},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 30, Y: 20, WidthMM: 1, HeightMM: 1, Shape: "RECT", NetName: "NET_B"},
		},
		Outline: rectOutline(60, 40),
	}
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("trace ending in pad must not short, got %d: %+v", len(viols), viols[0])
	}
}

func TestClearance_TraceThroughPadIsShort(t *testing.T) {
	// A trace passing straight through a pad with both endpoints outside is
	// a probable short.
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.2, StartX: 5, StartY: 20, EndX: 55, EndY: 20, NetName: "NET_A", NetSource: "attr"},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 30, Y: 20, WidthMM: 1, HeightMM: 1, Shape: "RECT", NetName: "NET_B", NetSource: "netlist"},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1})
	if len(viols) != 1 {
		t.Fatalf("expected 1 short, got %d", len(viols))
	}
	if viols[0].MeasuredMM != 0 || !containsStr(viols[0].Message, "probable short") {
		t.Errorf("unexpected violation: %+v", viols[0])
	}
}

func TestClearance_InferredNetOverlapNotShort(t *testing.T) {
	// Crossing different-net traces whose labels came from inference must not
	// be reported as shorts — inferred labels routinely disagree and would
	// flood real boards with false positives.
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.2, StartX: 0, StartY: 10, EndX: 50, EndY: 10, NetName: "NET_A", NetSource: "attr"},
			{Layer: "top_copper", WidthMM: 0.2, StartX: 25, StartY: 0, EndX: 25, EndY: 20, NetName: "NET_B", NetSource: "inferred"},
		},
		Outline: rectOutline(60, 40),
	}
	if viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1}); len(viols) != 0 {
		t.Fatalf("inferred-net crossing must not short, got %d violations", len(viols))
	}
}

func TestClearance_ContainedPadOverlapNotShort(t *testing.T) {
	// A small contact dot whose center sits on/inside a large ring pad
	// (dome-switch geometry) is concentric design or a label conflict, not a
	// misplacement short.
	rule := &ClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 20, WidthMM: 4.8, HeightMM: 4.8, Shape: "DONUT", HoleMM: 3.0, NetName: "KEYOUT", NetSource: "netlist"},
			{Layer: "top_copper", X: 11.95, Y: 20, WidthMM: 0.275, HeightMM: 0.275, Shape: "CIRCLE", NetName: "KEYIN", NetSource: "netlist"},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.1})
	for _, v := range viols {
		if containsStr(v.Message, "probable short") {
			t.Fatalf("contained pad overlap must not short: %+v", v)
		}
	}
}
