package dfmengine

import (
	"math"
	"testing"
)

func TestEdgeClearance_TooClose(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Trace endpoint at 0.05mm from right edge (60mm), limit = 0.2mm → violation
	board := edgeTraceBoard(0.05)
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation, got 0")
	}
	if viols[0].MeasuredMM >= 0.2 {
		t.Errorf("MeasuredMM should be below 0.2, got %f", viols[0].MeasuredMM)
	}
}

func TestEdgeClearance_Passes(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Trace endpoint at 0.5mm from right edge, limit = 0.2mm → no violation
	board := edgeTraceBoard(0.5)
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(viols))
	}
}

func TestEdgeClearance_NoOutlineSkipped(t *testing.T) {
	rule := &EdgeClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{{Layer: "top_copper", WidthMM: 0.1, StartX: 0.01, StartY: 0.01, EndX: 1, EndY: 1}},
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("no outline → should skip, got %d violations", len(viols))
	}
}

func TestEdgeClearance_FarOutsideSkipped(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Trace far outside the board bounding box (flex tail region)
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 500, StartY: 500, EndX: 600, EndY: 500},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("far-outside features should be skipped, got %d violations", len(viols))
	}
}

func TestEdgeClearance_PadTooClose(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Copper pad centre at x=59.9, right edge at x=60 → dist≈0.1mm < limit=0.2mm
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 59.9, Y: 20, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation for copper pad too close to board edge, got 0")
	}
	if viols[0].RuleID != "edge-clearance" {
		t.Errorf("expected RuleID=edge-clearance, got %s", viols[0].RuleID)
	}
}

func TestEdgeClearance_PadPasses(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Pad centre at x=59.5, right edge at x=60 → dist≈0.5mm > limit=0.2mm
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 59.5, Y: 20, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("pad far enough from edge should pass, got %d violations", len(viols))
	}
}

func TestEdgeClearance_NonCopperPadSkipped(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Silk pad right at the board edge — should be ignored
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		Pads: []Pad{
			{Layer: "top_silk", X: 59.9, Y: 20, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("silk pad near board edge must be skipped, got %d violations", len(viols))
	}
}

func TestEdgeClearance_PowerGroundTraceChecked(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// POWER_GROUND trace endpoint 0.05mm from right edge, limit=0.2mm → violation
	board := BoardData{
		Layers: []Layer{{Name: "gnd_plane", Type: "POWER_GROUND"}},
		Traces: []Trace{
			{Layer: "gnd_plane", WidthMM: 0.1, StartX: 10, StartY: 20, EndX: 59.95, EndY: 20},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("POWER_GROUND trace too close to board edge should be flagged, got 0 violations")
	}
}

// TestEdgeClearance_OutlineHoles checks that a trace near an inner board cutout
// (slot/step-out) is caught even when it is far from the outer boundary.
func TestEdgeClearance_OutlineHoles(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// 60×40 board with a 10×10 inner cutout in the centre.
	// Cutout corners: (25,15)–(35,15)–(35,25)–(25,25).
	// Trace runs at y=15.05 (only 0.05mm from the top edge of the cutout).
	// Trace is well within the outer board boundary.
	innerHole := []Point{
		{X: 25, Y: 15}, {X: 35, Y: 15}, {X: 35, Y: 25}, {X: 25, Y: 25},
	}
	board := BoardData{
		Layers:       []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces:       []Trace{{Layer: "top_copper", WidthMM: 0.1, StartX: 26, StartY: 15.05, EndX: 34, EndY: 15.05}},
		Outline:      rectOutline(60, 40),
		OutlineHoles: [][]Point{innerHole},
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation: trace too close to inner board cutout, got 0")
	}
}

// TestEdgeClearance_OutlineHoles_Safe ensures a trace well clear of an inner cutout passes.
func TestEdgeClearance_OutlineHoles_Safe(t *testing.T) {
	rule := &EdgeClearanceRule{}
	innerHole := []Point{
		{X: 25, Y: 15}, {X: 35, Y: 15}, {X: 35, Y: 25}, {X: 25, Y: 25},
	}
	board := BoardData{
		Layers:       []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces:       []Trace{{Layer: "top_copper", WidthMM: 0.1, StartX: 26, StartY: 16, EndX: 34, EndY: 16}},
		Outline:      rectOutline(60, 40),
		OutlineHoles: [][]Point{innerHole},
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("trace 1mm from cutout edge should pass (limit=0.2mm), got %d violations", len(viols))
	}
}

// TestOutlineIndexFromRings_MultipleRings checks that newOutlineIndexFromRings
// produces independent ring segments (no inter-ring wrap-around edges).
func TestOutlineIndexFromRings_MultipleRings(t *testing.T) {
	outer := rectOutline(60, 40)
	inner := []Point{{X: 25, Y: 15}, {X: 35, Y: 15}, {X: 35, Y: 25}, {X: 25, Y: 25}}
	rings := [][]Point{outer, inner}
	idx := newOutlineIndexFromRings(rings, 2.0)

	// A point 0.1mm inside the inner ring's top edge (y≈15) should be ~0.1mm from that edge.
	d := idx.minDist(30, 15.1)
	if d > 0.15 || d < 0.05 {
		t.Errorf("expected ~0.1mm from inner ring top edge, got %f", d)
	}

	// A point at the outer ring bottom edge (y=0) should be 0mm away.
	d = idx.minDist(30, 0)
	if d > 1e-9 {
		t.Errorf("point on outer ring bottom edge should be 0mm away, got %f", d)
	}
}

// TestOutlineIndex_MatchesBruteForce checks that the spatial index agrees with
// the naive brute-force scan to within 1e-9mm for 50 points around a 60×40 board.
func TestOutlineIndex_MatchesBruteForce(t *testing.T) {
	outline := rectOutline(60, 40)
	oidx := newOutlineIndex(outline, 0.4)

	bruteForce := func(px, py float64) float64 {
		minD := math.MaxFloat64
		n := len(outline)
		for i := 0; i < n; i++ {
			a := outline[i]
			b := outline[(i+1)%n]
			d := ptToSegDist(px, py, a.X, a.Y, b.X, b.Y)
			if d < minD {
				minD = d
			}
		}
		return minD
	}

	testPoints := [][2]float64{
		{0, 0}, {30, 0}, {60, 0}, {0, 20}, {30, 20}, {60, 20},
		{0, 40}, {30, 40}, {60, 40}, {5, 5}, {55, 35},
		{-1, -1}, {61, 41}, {30, -5}, {30, 45},
	}
	for _, pt := range testPoints {
		got := oidx.minDist(pt[0], pt[1])
		want := bruteForce(pt[0], pt[1])
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("point (%.1f,%.1f): index=%v brute=%v diff=%e", pt[0], pt[1], got, want, math.Abs(got-want))
		}
	}
}

// TestOutlineIndex_CenterOfBoard verifies center of 60×40 board is ~20mm from outline.
func TestOutlineIndex_CenterOfBoard(t *testing.T) {
	outline := rectOutline(60, 40)
	oidx := newOutlineIndex(outline, 1.0)
	got := oidx.minDist(30, 20)
	// Center of 60×40: nearest edge is 20mm (top/bottom) or 30mm (left/right) → 20mm
	if math.Abs(got-20.0) > 1e-9 {
		t.Errorf("expected 20.0mm from center to outline, got %f", got)
	}
}
