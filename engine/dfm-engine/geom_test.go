package dfmengine

import (
	"math"
	"testing"
)

// ── padEdgeDist ──────────────────────────────────────────────────────────────

func TestPadEdgeDist_Circle(t *testing.T) {
	pad := Pad{X: 10, Y: 10, WidthMM: 2, HeightMM: 2, Shape: "CIRCLE"} // radius = 1
	cases := []struct {
		px, py float64
		want   float64
		desc   string
	}{
		{10, 10, 0, "center → 0"},
		{10, 11, 0, "on edge → 0"},
		{10, 12, 1, "1mm outside"},
		{10, 9, 0, "on opposite edge → 0"},
		{12, 10, 1, "right of edge"},
	}
	for _, c := range cases {
		got := padEdgeDist(c.px, c.py, pad)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: padEdgeDist(%.1f,%.1f) = %f, want %f", c.desc, c.px, c.py, got, c.want)
		}
	}
}

func TestPadEdgeDist_Rect(t *testing.T) {
	// 4×2mm RECT centered at (0,0) → half-extents ±2, ±1
	pad := Pad{X: 0, Y: 0, WidthMM: 4, HeightMM: 2, Shape: "RECT"}
	cases := []struct {
		px, py float64
		want   float64
		desc   string
	}{
		{0, 0, 0, "center → 0"},
		{2, 0, 0, "on right edge → 0"},
		{3, 0, 1, "1mm right of edge"},
		{0, 1, 0, "on top edge → 0"},
		{0, 2, 1, "1mm above top edge"},
		{3, 2, math.Sqrt(2), "outside corner → sqrt(2)"},
	}
	for _, c := range cases {
		got := padEdgeDist(c.px, c.py, pad)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: padEdgeDist(%.1f,%.1f) = %f, want %f", c.desc, c.px, c.py, got, c.want)
		}
	}
}

func TestPadEdgeDist_Oval_Horizontal(t *testing.T) {
	// 4×2mm OVAL: r=1, halfLen=1, horizontal axis
	pad := Pad{X: 0, Y: 0, WidthMM: 4, HeightMM: 2, Shape: "OVAL"}
	cases := []struct {
		px, py float64
		want   float64
		desc   string
	}{
		{0, 0, 0, "center → 0"},
		{0, 1, 0, "on capsule side → 0"},
		{0, 2, 1, "1mm above capsule side"},
		{2, 0, 0, "on right cap edge → 0"},
		{3, 0, 1, "1mm right of cap"},
	}
	for _, c := range cases {
		got := padEdgeDist(c.px, c.py, pad)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: padEdgeDist(%.1f,%.1f) = %f, want %f", c.desc, c.px, c.py, got, c.want)
		}
	}
}

func TestPadEdgeDist_Oval_Vertical(t *testing.T) {
	// 2×4mm OVAL: r=1, halfLen=1, vertical axis
	pad := Pad{X: 0, Y: 0, WidthMM: 2, HeightMM: 4, Shape: "OVAL"}
	// 1mm right of the vertical capsule (side at x=1)
	got := padEdgeDist(2, 0, pad)
	if math.Abs(got-1) > 1e-9 {
		t.Errorf("1mm right of vertical capsule side: got %f, want 1.0", got)
	}
	// on capsule side
	got = padEdgeDist(1, 0, pad)
	if got > 1e-9 {
		t.Errorf("on vertical capsule side: got %f, want 0", got)
	}
}

func TestPadEdgeDist_Polygon(t *testing.T) {
	// Unit square polygon centered at origin: (±0.5, ±0.5)
	contour := []Point{{X: -0.5, Y: -0.5}, {X: 0.5, Y: -0.5}, {X: 0.5, Y: 0.5}, {X: -0.5, Y: 0.5}}
	pad := Pad{X: 0, Y: 0, WidthMM: 1, HeightMM: 1, Shape: "POLYGON", Contour: contour}
	cases := []struct {
		px, py float64
		want   float64
		desc   string
	}{
		{0, 0, 0, "inside → 0"},
		{0.5, 0, 0, "on edge → 0"},
		{1, 0, 0.5, "0.5mm outside right edge"},
		{0, -1, 0.5, "0.5mm outside bottom edge"},
	}
	for _, c := range cases {
		got := padEdgeDist(c.px, c.py, pad)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: padEdgeDist(%.1f,%.1f) = %f, want %f", c.desc, c.px, c.py, got, c.want)
		}
	}
}

func TestPadEdgeDist_Polygon_NoContourFallback(t *testing.T) {
	// POLYGON with empty Contour falls back to max(W,H)/2 circle
	pad := Pad{X: 0, Y: 0, WidthMM: 2, HeightMM: 2, Shape: "POLYGON"}
	got := padEdgeDist(2, 0, pad) // 1mm outside radius-1 circle
	if math.Abs(got-1) > 1e-9 {
		t.Errorf("empty contour fallback: got %f, want 1.0", got)
	}
}

// ── padProjection ─────────────────────────────────────────────────────────────

func TestPadProjection_Circle(t *testing.T) {
	// Circle projection is always radius regardless of direction.
	pad := Pad{WidthMM: 2, HeightMM: 2, Shape: "CIRCLE"}
	dirs := [][2]float64{{1, 0}, {0, 1}, {1, 1}, {0.6, 0.8}}
	for _, d := range dirs {
		got := padProjection(pad, d[0], d[1])
		if math.Abs(got-1.0) > 1e-9 {
			t.Errorf("circle projection dir(%.1f,%.1f) = %f, want 1.0", d[0], d[1], got)
		}
	}
}

func TestPadProjection_Rect(t *testing.T) {
	// 4×2 RECT: projection along X = 2, along Y = 1, along 45° = (2+1)/sqrt(2)
	pad := Pad{WidthMM: 4, HeightMM: 2, Shape: "RECT"}
	if got := padProjection(pad, 1, 0); math.Abs(got-2) > 1e-9 {
		t.Errorf("projection along X: %f, want 2", got)
	}
	if got := padProjection(pad, 0, 1); math.Abs(got-1) > 1e-9 {
		t.Errorf("projection along Y: %f, want 1", got)
	}
	want45 := (2 + 1) / math.Sqrt2
	if got := padProjection(pad, 1, 1); math.Abs(got-want45) > 1e-9 {
		t.Errorf("projection along 45°: %f, want %f", got, want45)
	}
}

func TestPadProjection_Oval_Horizontal(t *testing.T) {
	// 4×2 OVAL, horizontal: halfLen=1, r=1
	pad := Pad{WidthMM: 4, HeightMM: 2, Shape: "OVAL"}
	// Along X: halfLen * |ux| + r = 1*1 + 1 = 2
	if got := padProjection(pad, 1, 0); math.Abs(got-2) > 1e-9 {
		t.Errorf("oval horizontal proj X: %f, want 2", got)
	}
	// Along Y: halfLen * 0 + r = 1
	if got := padProjection(pad, 0, 1); math.Abs(got-1) > 1e-9 {
		t.Errorf("oval horizontal proj Y: %f, want 1", got)
	}
}

func TestPadProjection_ZeroLength(t *testing.T) {
	// Zero-length direction → fallback to max(W,H)/2
	pad := Pad{WidthMM: 4, HeightMM: 2, Shape: "RECT"}
	got := padProjection(pad, 0, 0)
	if math.Abs(got-2) > 1e-9 {
		t.Errorf("zero-length dir fallback: %f, want 2", got)
	}
}

// ── pointInPolygon ───────────────────────────────────────────────────────────

func TestPointInPolygon(t *testing.T) {
	// Unit square: (0,0)–(1,0)–(1,1)–(0,1)
	sq := []Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	cases := []struct {
		px, py float64
		want   bool
		desc   string
	}{
		{0.5, 0.5, true, "center"},
		{2, 0.5, false, "outside right"},
		{-1, 0.5, false, "outside left"},
		{0.5, 2, false, "outside top"},
		{0.5, -1, false, "outside bottom"},
	}
	for _, c := range cases {
		got := pointInPolygon(c.px, c.py, sq)
		if got != c.want {
			t.Errorf("%s: pointInPolygon(%.1f,%.1f) = %v, want %v", c.desc, c.px, c.py, got, c.want)
		}
	}
}

func TestPointInPolygon_LShape(t *testing.T) {
	// L-shaped polygon
	l := []Point{
		{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {X: 0, Y: 2},
	}
	if !pointInPolygon(0.5, 0.5, l) {
		t.Error("(0.5,0.5) should be inside L-shape")
	}
	if !pointInPolygon(1.5, 0.5, l) {
		t.Error("(1.5,0.5) should be inside L-shape")
	}
	if pointInPolygon(1.5, 1.5, l) {
		t.Error("(1.5,1.5) should be outside L-shape (notch region)")
	}
}

// ── closestPointOnSeg ────────────────────────────────────────────────────────

func TestClosestPointOnSeg(t *testing.T) {
	cases := []struct {
		px, py, ax, ay, bx, by float64
		wantX, wantY           float64
		desc                   string
	}{
		// Interior projection
		{5, 1, 0, 0, 10, 0, 5, 0, "perpendicular from interior"},
		// Before start → clamp to A
		{-1, 0, 0, 0, 10, 0, 0, 0, "before start clamps to A"},
		// After end → clamp to B
		{11, 0, 0, 0, 10, 0, 10, 0, "after end clamps to B"},
		// Degenerate segment (A==B)
		{3, 4, 5, 5, 5, 5, 5, 5, "degenerate returns A"},
		// Diagonal segment
		{0, 1, 0, 0, 1, 1, 0.5, 0.5, "diagonal midpoint"},
	}
	for _, c := range cases {
		gx, gy := closestPointOnSeg(c.px, c.py, c.ax, c.ay, c.bx, c.by)
		if math.Abs(gx-c.wantX) > 1e-9 || math.Abs(gy-c.wantY) > 1e-9 {
			t.Errorf("%s: got (%.4f,%.4f), want (%.4f,%.4f)", c.desc, gx, gy, c.wantX, c.wantY)
		}
	}
}
