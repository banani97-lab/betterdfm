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

// ── Pour sliver / neck detection (P4) ────────────────────────────────────────

func sliverProfile() ProfileRules { return ProfileRules{MinCopperSliverMM: 0.1} }

func TestPourSliver_NeckBetweenLobes(t *testing.T) {
	// Two 5x5 lobes joined by a 0.06mm-tall, 2mm-long neck: the outer ring
	// folds back on itself. Same ring, far apart in arc length.
	rule := &CopperSliverRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Polygons: []Polygon{{
			Layer: "top_copper",
			Points: []Point{
				{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 5, Y: 2.47}, {X: 7, Y: 2.47},
				{X: 7, Y: 0}, {X: 12, Y: 0}, {X: 12, Y: 5}, {X: 7, Y: 5},
				{X: 7, Y: 2.53}, {X: 5, Y: 2.53}, {X: 5, Y: 5}, {X: 0, Y: 5},
			},
		}},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, sliverProfile())
	if len(viols) == 0 {
		t.Fatal("expected neck violation(s), got 0")
	}
	// A long neck can be reported at both ends (dedup cells are 2mm); every
	// finding must be the ~0.06mm neck.
	for _, v := range viols {
		if v.MeasuredMM < 0.055 || v.MeasuredMM > 0.065 {
			t.Errorf("MeasuredMM = %f, want ~0.06", v.MeasuredMM)
		}
		if v.Severity != "WARNING" {
			t.Errorf("severity = %s, want WARNING", v.Severity)
		}
	}
}

func TestPourSliver_WebBetweenHoles(t *testing.T) {
	// Two anti-pads 0.05mm apart: classic copper web between voids.
	rule := &CopperSliverRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Polygons: []Polygon{{
			Layer: "top_copper",
			Points: []Point{
				{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 20}, {X: 0, Y: 20},
			},
			Holes: [][]Point{
				{{X: 5, Y: 5}, {X: 9.975, Y: 5}, {X: 9.975, Y: 10}, {X: 5, Y: 10}},
				{{X: 10.025, Y: 5}, {X: 15, Y: 5}, {X: 15, Y: 10}, {X: 10.025, Y: 10}},
			},
		}},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, sliverProfile())
	if len(viols) == 0 {
		t.Fatal("expected web violation(s), got 0")
	}
	for _, v := range viols {
		if v.MeasuredMM < 0.045 || v.MeasuredMM > 0.055 {
			t.Errorf("MeasuredMM = %f, want ~0.05", v.MeasuredMM)
		}
	}
}

func TestPourSliver_NarrowSlotNotFlagged(t *testing.T) {
	// A 0.06mm-wide slot cut into the pour: the thin thing is void, not
	// copper. Midpoint-in-fill check must reject it.
	rule := &CopperSliverRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Polygons: []Polygon{{
			Layer: "top_copper",
			Points: []Point{
				{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 2.47}, {X: 4, Y: 2.47},
				{X: 4, Y: 2.53}, {X: 10, Y: 2.53}, {X: 10, Y: 5}, {X: 0, Y: 5},
			},
		}},
		Outline: rectOutline(60, 40),
	}
	if viols := rule.Run(board, sliverProfile()); len(viols) != 0 {
		t.Fatalf("void slot must not be a copper sliver, got %d: %+v", len(viols), viols[0])
	}
}

func TestPourSliver_HealthyPolygonClean(t *testing.T) {
	rule := &CopperSliverRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Polygons: []Polygon{{
			Layer: "top_copper",
			Points: []Point{
				{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 20}, {X: 0, Y: 20},
			},
			Holes: [][]Point{
				{{X: 5, Y: 5}, {X: 8, Y: 5}, {X: 8, Y: 8}, {X: 5, Y: 8}},
			},
		}},
		Outline: rectOutline(60, 40),
	}
	if viols := rule.Run(board, sliverProfile()); len(viols) != 0 {
		t.Fatalf("healthy polygon must be clean, got %d", len(viols))
	}
}
