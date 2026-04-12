package dfmengine

import "testing"

// fourLayerStack returns a typical 4-layer stack: top signal, two internal
// ground planes, bottom signal.
func fourLayerStack() []Layer {
	return []Layer{
		{Name: "L01_TOP", Type: "COPPER"},
		{Name: "L02_GND", Type: "POWER_GROUND"},
		{Name: "L03_PWR", Type: "POWER_GROUND"},
		{Name: "L04_BOT", Type: "COPPER"},
	}
}

func TestPadSizeForPackage_UndersizedOnTop(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Layers:       fourLayerStack(),
		Pads: []Pad{
			// 0805 short-dim min is 0.75 mm; a 0.50 × 0.50 pad is
			// unambiguously undersized on both axes.
			{Layer: "L01_TOP", X: 10, Y: 10, WidthMM: 0.50, HeightMM: 0.50,
				Shape: "RECT", RefDes: "R169", PackageClass: "0805"},
		},
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	if viols[0].Layer != "L01_TOP" {
		t.Errorf("expected layer L01_TOP, got %q", viols[0].Layer)
	}
	if viols[0].RefDes != "R169" {
		t.Errorf("expected refDes R169, got %q", viols[0].RefDes)
	}
}

// Regression: pads on internal POWER_GROUND planes (e.g. L02_GND) are not
// component mounting pads and must not produce pad-size-for-package
// violations, even if the parser's spatial lookup assigned them a refdes
// and package class from a component on the outer layer.
func TestPadSizeForPackage_IgnoresInternalPlanePads(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Layers:       fourLayerStack(),
		Pads: []Pad{
			// Plane feature under R169 — carries R169/0805 metadata from
			// spatial refdes lookup but lives on the internal ground plane.
			{Layer: "L02_GND", X: 10, Y: 10, WidthMM: 0.50, HeightMM: 0.50,
				Shape: "RECT", RefDes: "R169", PackageClass: "0805"},
		},
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations for internal plane pad, got %d: %+v", len(viols), viols)
	}
}

// A component's real outer-layer pad should still trigger the rule even if
// an internal-plane phantom pad with the same refdes is also present.
func TestPadSizeForPackage_OuterFlaggedInternalIgnored(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Layers:       fourLayerStack(),
		Pads: []Pad{
			{Layer: "L01_TOP", X: 10, Y: 10, WidthMM: 0.50, HeightMM: 0.50,
				Shape: "RECT", RefDes: "R169", PackageClass: "0805"},
			{Layer: "L02_GND", X: 10, Y: 10, WidthMM: 0.50, HeightMM: 0.50,
				Shape: "RECT", RefDes: "R169", PackageClass: "0805"},
		},
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation (outer only), got %d", len(viols))
	}
	if viols[0].Layer != "L01_TOP" {
		t.Errorf("expected outer-layer violation, got layer %q", viols[0].Layer)
	}
}

// Regression: IPC-7351B Density A nominal 1206 pad (1.80 × 1.15 mm) is a
// standard land pattern and must NOT be flagged as undersized. The previous
// table set minH=1.20 which wrongly rejected this.
func TestPadSizeForPackage_1206DensityANominal(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Layers:       fourLayerStack(),
		Pads: []Pad{
			{Layer: "L01_TOP", X: 0, Y: 0, WidthMM: 1.80, HeightMM: 1.15,
				Shape: "RECT", RefDes: "R32", PackageClass: "1206"},
		},
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations for 1206 Density A nominal pad (1.80×1.15), got %d: %+v", len(viols), viols)
	}
}

// Regression: the check must be rotation-invariant. A 1206 pad rotated 90°
// has (widthMM, heightMM) swapped — (1.15, 1.80) instead of (1.80, 1.15) —
// and must still pass. The old rule compared widthMM → minW and heightMM →
// minH directly, so a rotated part would fail on whichever axis the table
// happened to assign as "long".
func TestPadSizeForPackage_RotationInvariant(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	for _, pad := range []Pad{
		{WidthMM: 1.80, HeightMM: 1.15}, // 0° / 180°
		{WidthMM: 1.15, HeightMM: 1.80}, // 90° / 270°
	} {
		pad.Layer = "L01_TOP"
		pad.Shape = "RECT"
		pad.RefDes = "R32"
		pad.PackageClass = "1206"
		board := BoardData{
			SourceFormat: "ODB_PLUS_PLUS",
			Layers:       fourLayerStack(),
			Pads:         []Pad{pad},
		}
		viols := rule.Run(board, ProfileRules{})
		if len(viols) != 0 {
			t.Errorf("rotated 1206 pad (%.2f × %.2f) should pass, got %d violations: %+v",
				pad.WidthMM, pad.HeightMM, len(viols), viols)
		}
	}
}

// Regression: via catch-pads (IsViaCatchPad=true) are not SMT land patterns
// and must not be checked against IPC-7351 pad-size envelopes.
func TestPadSizeForPackage_IgnoresViaCatchPads(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Layers:       fourLayerStack(),
		Pads: []Pad{
			{Layer: "L01_TOP", X: 10.0, Y: 10.0, WidthMM: 0.762, HeightMM: 0.762,
				Shape: "CIRCLE", RefDes: "R169", PackageClass: "0805",
				IsViaCatchPad: true},
		},
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations for via catch-pad, got %d: %+v", len(viols), viols)
	}
}

// With no layer metadata, fall back to checking all pads (preserves
// pre-existing behavior for any board that lacks a stack).
func TestPadSizeForPackage_NoLayerMetadataFallback(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Pads: []Pad{
			{Layer: "unknown", X: 0, Y: 0, WidthMM: 0.50, HeightMM: 0.50,
				Shape: "RECT", RefDes: "R1", PackageClass: "0805"},
		},
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 1 {
		t.Fatalf("expected fallback violation, got %d", len(viols))
	}
}
