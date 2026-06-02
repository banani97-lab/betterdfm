package dfmengine

import "testing"

func runSpacing(board BoardData, profile ProfileRules) []Violation {
	return (&ComponentSpacingRule{}).Run(board, profile)
}

// spacingLayers returns a two-sided copper stack: top_copper is the outer top,
// bot_copper the outer bottom.
func spacingLayers() []Layer {
	return []Layer{
		{Name: "top_copper", Type: "COPPER"},
		{Name: "bot_copper", Type: "COPPER"},
	}
}

func smtPad(ref, layer string, x, y, w, h float64) Pad {
	return Pad{Layer: layer, X: x, Y: y, WidthMM: w, HeightMM: h, Shape: "RECT", RefDes: ref}
}

func TestComponentSpacing_DisabledWhenZero(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1),
			smtPad("R2", "top_copper", 1.0, 0.5, 1, 1), // overlapping
		},
		Outline: rectOutline(60, 40),
	}
	if got := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0}); len(got) != 0 {
		t.Fatalf("disabled rule should return no violations, got %d", len(got))
	}
}

func TestComponentSpacing_FlagsCloseSameSide(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1), // bbox [0,1]x[0,1]
			smtPad("R2", "top_copper", 1.8, 0.5, 1, 1), // bbox [1.3,2.3] -> gap 0.3
		},
		Outline: rectOutline(60, 40),
	}
	vs := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0.5})
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(vs), vs)
	}
	if vs[0].Severity != "WARNING" {
		t.Fatalf("expected WARNING, got %s", vs[0].Severity)
	}
	if vs[0].RefDes != "R1" || vs[0].LimitMM != 0.5 {
		t.Fatalf("unexpected violation: %+v", vs[0])
	}
	if vs[0].MeasuredMM < 0.29 || vs[0].MeasuredMM > 0.31 {
		t.Fatalf("expected gap ~0.30, got %.4f", vs[0].MeasuredMM)
	}
}

func TestComponentSpacing_OverlapIsError(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1), // bbox [0,1]
			smtPad("R2", "top_copper", 1.0, 0.5, 1, 1), // bbox [0.5,1.5] -> overlap
		},
		Outline: rectOutline(60, 40),
	}
	vs := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0.5})
	if len(vs) != 1 || vs[0].Severity != "ERROR" {
		t.Fatalf("expected 1 ERROR for overlap, got %+v", vs)
	}
}

func TestComponentSpacing_OppositeSidesIgnored(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1),
			smtPad("R2", "bot_copper", 1.0, 0.5, 1, 1), // overlapping in XY but other side
		},
		Outline: rectOutline(60, 40),
	}
	if got := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0.5}); len(got) != 0 {
		t.Fatalf("opposite-side parts must not be compared, got %+v", got)
	}
}

func TestComponentSpacing_FarApartOK(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1),
			smtPad("R2", "top_copper", 5.0, 0.5, 1, 1),
		},
		Outline: rectOutline(60, 40),
	}
	if got := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0.5}); len(got) != 0 {
		t.Fatalf("well-separated parts should not be flagged, got %+v", got)
	}
}

func TestComponentSpacing_ThroughHoleSkipped(t *testing.T) {
	// J1 has pads on both outer layers (through-hole) and sits right next to R1.
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1),
			smtPad("J1", "top_copper", 1.6, 0.5, 1, 1),
			smtPad("J1", "bot_copper", 1.6, 0.5, 1, 1),
		},
		Outline: rectOutline(60, 40),
	}
	if got := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0.5}); len(got) != 0 {
		t.Fatalf("through-hole part should be skipped, got %+v", got)
	}
}

func TestComponentSpacing_TestPointsSkipped(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1),
			smtPad("TP1", "top_copper", 1.6, 0.5, 1, 1),
			smtPad("MH1", "top_copper", 1.6, 0.5, 1, 1),
		},
		Outline: rectOutline(60, 40),
	}
	if got := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0.5}); len(got) != 0 {
		t.Fatalf("test points / mounting holes should be skipped, got %+v", got)
	}
}
