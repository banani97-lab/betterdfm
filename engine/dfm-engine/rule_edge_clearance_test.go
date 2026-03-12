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
