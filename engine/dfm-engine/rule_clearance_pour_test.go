package dfmengine

// Fill-aware pour pass tests (P3).

import "testing"

// pourBoard returns a board with a 20x20 GND pour at (10..30, 10..30) with a
// 4x4 hole centered at (20, 20), netSource attr.
func pourBoard() BoardData {
	return BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Polygons: []Polygon{{
			Layer: "top_copper",
			Points: []Point{
				{X: 10, Y: 10}, {X: 30, Y: 10}, {X: 30, Y: 30}, {X: 10, Y: 30},
			},
			Holes: [][]Point{{
				{X: 18, Y: 18}, {X: 22, Y: 18}, {X: 22, Y: 22}, {X: 18, Y: 22},
			}},
			NetName:   "GND",
			NetSource: "attr",
		}},
		Outline: rectOutline(60, 40),
	}
}

func runPour(t *testing.T, board BoardData) []Violation {
	t.Helper()
	return (&ClearanceRule{}).Run(board, ProfileRules{MinClearanceMM: 0.15})
}

func TestPour_TraceInsideFillIsShort(t *testing.T) {
	board := pourBoard()
	board.Traces = []Trace{
		{Layer: "top_copper", WidthMM: 0.1, StartX: 12, StartY: 12, EndX: 15, EndY: 12, NetName: "SIG", NetSource: "attr"},
	}
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 short, got %d", len(viols))
	}
	if viols[0].MeasuredMM != 0 || !containsStr(viols[0].Message, "probable short") {
		t.Errorf("unexpected: %+v", viols[0])
	}
}

func TestPour_TraceInsideHoleNotShort(t *testing.T) {
	// A trace inside the pour's hole (anti-pad region) is not on the copper.
	// Its clearance to the hole edge (1.5mm at the closest) is fine too.
	board := pourBoard()
	board.Traces = []Trace{
		{Layer: "top_copper", WidthMM: 0.1, StartX: 19.5, StartY: 20, EndX: 20.5, EndY: 20, NetName: "SIG", NetSource: "attr"},
	}
	if viols := runPour(t, board); len(viols) != 0 {
		t.Fatalf("trace in hole must not violate, got %d: %+v", len(viols), viols[0])
	}
}

func TestPour_TraceNearHoleEdgeFlagged(t *testing.T) {
	// Inside the hole, 0.1mm from its edge: clearance = 0.1 - 0.05 = 0.05.
	board := pourBoard()
	board.Traces = []Trace{
		{Layer: "top_copper", WidthMM: 0.1, StartX: 18.1, StartY: 20, EndX: 18.2, EndY: 20, NetName: "SIG", NetSource: "attr"},
	}
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	v := viols[0]
	if v.MeasuredMM < 0.04 || v.MeasuredMM > 0.06 {
		t.Errorf("MeasuredMM = %f, want ~0.05", v.MeasuredMM)
	}
	if !containsStr(v.Message, "pour to trace") {
		t.Errorf("message %q should be a pour-to-trace finding", v.Message)
	}
}

func TestPour_TraceOutsideNearEdgeFlagged(t *testing.T) {
	// Outside the pour, 0.1mm left of x=10: clearance = 0.1 - 0.05 = 0.05.
	board := pourBoard()
	board.Traces = []Trace{
		{Layer: "top_copper", WidthMM: 0.1, StartX: 9.9, StartY: 15, EndX: 9.9, EndY: 18, NetName: "SIG", NetSource: "attr"},
	}
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
}

func TestPour_SameNetTraceSkipped(t *testing.T) {
	board := pourBoard()
	board.Traces = []Trace{
		{Layer: "top_copper", WidthMM: 0.1, StartX: 12, StartY: 12, EndX: 15, EndY: 12, NetName: "GND", NetSource: "attr"},
	}
	if viols := runPour(t, board); len(viols) != 0 {
		t.Fatalf("same-net trace in pour must not violate, got %d", len(viols))
	}
}

func TestPour_InferredPourNoShortButEdgeFinding(t *testing.T) {
	// Inferred pour labels can't justify a short, but boundary findings stand.
	board := pourBoard()
	board.Polygons[0].NetSource = "inferred"
	board.Traces = []Trace{
		{Layer: "top_copper", WidthMM: 0.1, StartX: 12, StartY: 12, EndX: 15, EndY: 12, NetName: "SIG", NetSource: "attr"},
		{Layer: "top_copper", WidthMM: 0.1, StartX: 9.9, StartY: 35, EndX: 9.9, EndY: 38, NetName: "SIG2", NetSource: "attr"},
	}
	viols := runPour(t, board)
	for _, v := range viols {
		if containsStr(v.Message, "probable short") {
			t.Fatalf("inferred pour must not produce shorts: %+v", v)
		}
	}
}

func TestPour_PadBuriedIsShort(t *testing.T) {
	board := pourBoard()
	board.Pads = []Pad{
		{Layer: "top_copper", X: 14, Y: 14, WidthMM: 1, HeightMM: 1, Shape: "RECT", NetName: "SIG", NetSource: "attr"},
	}
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 short, got %d", len(viols))
	}
	if viols[0].MeasuredMM != 0 || !containsStr(viols[0].Message, "probable short") {
		t.Errorf("unexpected: %+v", viols[0])
	}
}

func TestPour_PadNearEdgeFlagged(t *testing.T) {
	// 1x1 RECT pad centered 0.55mm left of the pour edge: gap = 0.05.
	board := pourBoard()
	board.Pads = []Pad{
		{Layer: "top_copper", X: 9.45, Y: 20, WidthMM: 1, HeightMM: 1, Shape: "RECT", NetName: "SIG", NetSource: "netlist"},
	}
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	v := viols[0]
	if v.MeasuredMM < 0.04 || v.MeasuredMM > 0.06 {
		t.Errorf("MeasuredMM = %f, want ~0.05", v.MeasuredMM)
	}
	if !containsStr(v.Message, "pour to pad") {
		t.Errorf("message %q should be a pour-to-pad finding", v.Message)
	}
}

func TestPour_PourPourOverlapIsShort(t *testing.T) {
	board := pourBoard()
	board.Polygons = append(board.Polygons, Polygon{
		Layer: "top_copper",
		Points: []Point{
			{X: 28, Y: 12}, {X: 35, Y: 12}, {X: 35, Y: 16}, {X: 28, Y: 16},
		},
		NetName:   "VCC",
		NetSource: "attr",
	})
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 short, got %d", len(viols))
	}
	if viols[0].MeasuredMM != 0 || !containsStr(viols[0].Message, "probable short") {
		t.Errorf("unexpected: %+v", viols[0])
	}
}

func TestPour_DegeneratePolygonDoesNotAbortPass(t *testing.T) {
	// Regression: a degenerate pour (<3 points → nil index) sorted ahead of a
	// real pour used to `break` out of the whole pour pass, silently skipping
	// every valid pour on the layer.
	board := pourBoard()
	board.Polygons = append([]Polygon{{
		Layer:     "top_copper",
		Points:    []Point{{X: 0, Y: 0}, {X: 1, Y: 0}},
		NetName:   "VCC",
		NetSource: "attr",
	}}, board.Polygons...)
	board.Traces = []Trace{
		{Layer: "top_copper", WidthMM: 0.1, StartX: 12, StartY: 12, EndX: 15, EndY: 12, NetName: "SIG", NetSource: "attr"},
	}
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 short from the valid pour, got %d", len(viols))
	}
	if !containsStr(viols[0].Message, "probable short") {
		t.Errorf("unexpected: %+v", viols[0])
	}
}

func TestPour_PourPourGapFlagged(t *testing.T) {
	// Second pour 0.05mm right of the first: boundary clearance finding.
	board := pourBoard()
	board.Polygons = append(board.Polygons, Polygon{
		Layer: "top_copper",
		Points: []Point{
			{X: 30.05, Y: 12}, {X: 35, Y: 12}, {X: 35, Y: 16}, {X: 30.05, Y: 16},
		},
		NetName:   "VCC",
		NetSource: "attr",
	})
	viols := runPour(t, board)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	v := viols[0]
	if v.MeasuredMM < 0.045 || v.MeasuredMM > 0.055 {
		t.Errorf("MeasuredMM = %f, want ~0.05", v.MeasuredMM)
	}
	if !containsStr(v.Message, "pour to pour") {
		t.Errorf("message %q should be a pour-to-pour finding", v.Message)
	}
}
