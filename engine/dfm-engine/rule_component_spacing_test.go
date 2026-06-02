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

// ---- Per-class (ComponentSpacing != nil) mode ----

func defaultSpacingClasses() *ComponentSpacingClasses {
	return &ComponentSpacingClasses{DiscreteMM: 0.254, LeadedMM: 1.27, BGAMM: 3.175, ThroughHoleMM: 3.175}
}

// twoBoxesGap1 places R1 at bbox [0,1]x[0,1] and X1 at [2,3]x[0,1] -> gap 1.0mm.
func twoBoxesGap1(refB string) []Pad {
	return []Pad{
		smtPad("R1", "top_copper", 0.5, 0.5, 1, 1),
		smtPad(refB, "top_copper", 2.5, 0.5, 1, 1),
	}
}

func TestComponentSpacing_PerClass_BGAUsesLargerKeepout(t *testing.T) {
	board := BoardData{
		Layers:     spacingLayers(),
		Pads:       twoBoxesGap1("U1"),
		Components: []Component{{RefDes: "R1", PackageType: "discrete"}, {RefDes: "U1", PackageType: "bga"}},
		Outline:    rectOutline(60, 40),
	}
	vs := runSpacing(board, ProfileRules{ComponentSpacing: defaultSpacingClasses()})
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for discrete<->BGA at 1mm, got %d: %+v", len(vs), vs)
	}
	if vs[0].LimitMM != 3.175 {
		t.Fatalf("expected BGA keepout 3.175, got %.4f", vs[0].LimitMM)
	}
	if vs[0].MeasuredMM < 0.99 || vs[0].MeasuredMM > 1.01 {
		t.Fatalf("expected gap ~1.0, got %.4f", vs[0].MeasuredMM)
	}
}

func TestComponentSpacing_PerClass_DiscretePairUnaffected(t *testing.T) {
	// Same 1mm geometry but both discrete -> 0.254 keepout, well clear.
	board := BoardData{
		Layers:     spacingLayers(),
		Pads:       twoBoxesGap1("R2"),
		Components: []Component{{RefDes: "R1", PackageType: "discrete"}, {RefDes: "R2", PackageType: "discrete"}},
		Outline:    rectOutline(60, 40),
	}
	if got := runSpacing(board, ProfileRules{ComponentSpacing: defaultSpacingClasses()}); len(got) != 0 {
		t.Fatalf("discrete pair 1mm apart should pass at 0.254mm keepout, got %+v", got)
	}
}

func TestComponentSpacing_PerClass_ThroughHoleIncluded(t *testing.T) {
	// J1 is through-hole (pads on both outer layers) 1mm from discrete R1.
	// Flat mode skips it; per-class mode flags it at the 3.175 keepout.
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1),
			smtPad("J1", "top_copper", 2.5, 0.5, 1, 1),
			smtPad("J1", "bot_copper", 2.5, 0.5, 1, 1),
		},
		Components: []Component{{RefDes: "R1", PackageType: "discrete"}, {RefDes: "J1", PackageType: "through_hole"}},
		Outline:    rectOutline(60, 40),
	}
	if got := runSpacing(board, ProfileRules{MinComponentSpacingMM: 0.5}); len(got) != 0 {
		t.Fatalf("flat mode must still skip through-hole, got %+v", got)
	}
	vs := runSpacing(board, ProfileRules{ComponentSpacing: defaultSpacingClasses()})
	if len(vs) != 1 || vs[0].LimitMM != 3.175 {
		t.Fatalf("expected 1 through-hole violation at 3.175, got %+v", vs)
	}
}

func TestComponentSpacing_PerClass_ZeroFieldFallsBackToFlat(t *testing.T) {
	// DiscreteMM unset (0) -> falls back to MinComponentSpacingMM (0.5).
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			smtPad("R1", "top_copper", 0.5, 0.5, 1, 1), // [0,1]
			smtPad("R2", "top_copper", 1.8, 0.5, 1, 1), // [1.3,2.3] -> gap 0.3
		},
		Components: []Component{{RefDes: "R1", PackageType: "discrete"}, {RefDes: "R2", PackageType: "discrete"}},
		Outline:    rectOutline(60, 40),
	}
	vs := runSpacing(board, ProfileRules{
		MinComponentSpacingMM: 0.5,
		ComponentSpacing:      &ComponentSpacingClasses{BGAMM: 3.175}, // discrete/leaded/TH all 0
	})
	if len(vs) != 1 || vs[0].LimitMM != 0.5 {
		t.Fatalf("expected fallback to flat 0.5 keepout, got %+v", vs)
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
