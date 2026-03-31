package dfmengine

import (
	"math"
	"testing"
)

func TestEdgeClearance_ComponentPadTooClose(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Component pad 0.05mm from right edge (60mm), limit = 0.2mm → violation
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 59.95, Y: 20, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE", RefDes: "C1"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation for component pad too close to board edge, got 0")
	}
}

func TestEdgeClearance_ComponentPadPasses(t *testing.T) {
	rule := &EdgeClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 59.5, Y: 20, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE", RefDes: "C1"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("component pad far enough from edge should pass, got %d violations", len(viols))
	}
}

func TestEdgeClearance_AnonymousPadSkipped(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Pad with no refDes and not a fiducial — should be skipped
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 59.95, Y: 20, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE"},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("anonymous pad should be skipped, got %d violations", len(viols))
	}
}

func TestEdgeClearance_FiducialTooClose(t *testing.T) {
	rule := &EdgeClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 59.95, Y: 20, WidthMM: 1.0, HeightMM: 1.0, Shape: "CIRCLE", IsFiducial: true},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation for fiducial too close to board edge, got 0")
	}
}

func TestEdgeClearance_DrillTooClose(t *testing.T) {
	rule := &EdgeClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Drills: []Drill{
			{X: 59.95, Y: 20, DiamMM: 0.8, Plated: true},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.3}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation for drill too close to board edge, got 0")
	}
}

func TestEdgeClearance_DrillPasses(t *testing.T) {
	rule := &EdgeClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Drills: []Drill{
			{X: 59, Y: 20, DiamMM: 0.8, Plated: true},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.3}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("drill far from edge should pass, got %d violations", len(viols))
	}
}

func TestEdgeClearance_NoOutlineSkipped(t *testing.T) {
	rule := &EdgeClearanceRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 0.01, Y: 0.01, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE", RefDes: "R1"},
		},
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("no outline → should skip, got %d violations", len(viols))
	}
}

func TestEdgeClearance_TracesNotChecked(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Trace very close to edge — should NOT be flagged (we only check pads/drills/fiducials)
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 10, StartY: 20, EndX: 59.95, EndY: 20},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinEdgeClearanceMM: 0.2}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("traces should not be checked for edge clearance, got %d violations", len(viols))
	}
}

// --- Outline index tests (unchanged) ---

func TestOutlineIndexFromRings_MultipleRings(t *testing.T) {
	outer := rectOutline(60, 40)
	inner := []Point{{X: 25, Y: 15}, {X: 35, Y: 15}, {X: 35, Y: 25}, {X: 25, Y: 25}}
	rings := [][]Point{outer, inner}
	idx := newOutlineIndexFromRings(rings, 2.0)

	d := idx.minDist(30, 15.1)
	if d > 0.15 || d < 0.05 {
		t.Errorf("expected ~0.1mm from inner ring top edge, got %f", d)
	}

	d = idx.minDist(30, 0)
	if d > 1e-9 {
		t.Errorf("point on outer ring bottom edge should be 0mm away, got %f", d)
	}
}

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

func TestOutlineIndex_CenterOfBoard(t *testing.T) {
	outline := rectOutline(60, 40)
	oidx := newOutlineIndex(outline, 1.0)
	got := oidx.minDist(30, 20)
	if math.Abs(got-20.0) > 1e-9 {
		t.Errorf("expected 20.0mm from center to outline, got %f", got)
	}
}
