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
			// 0805 expects minW 0.80 mm; this pad is 0.76 mm wide on the top layer.
			{Layer: "L01_TOP", X: 10, Y: 10, WidthMM: 0.76, HeightMM: 1.0,
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
			{Layer: "L02_GND", X: 10, Y: 10, WidthMM: 0.76, HeightMM: 1.0,
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
			{Layer: "L01_TOP", X: 10, Y: 10, WidthMM: 0.76, HeightMM: 1.0,
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

// With no layer metadata, fall back to checking all pads (preserves
// pre-existing behavior for any board that lacks a stack).
func TestPadSizeForPackage_NoLayerMetadataFallback(t *testing.T) {
	rule := &PadSizeForPackageRule{}
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Pads: []Pad{
			{Layer: "unknown", X: 0, Y: 0, WidthMM: 0.76, HeightMM: 1.0,
				Shape: "RECT", RefDes: "R1", PackageClass: "0805"},
		},
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 1 {
		t.Fatalf("expected fallback violation, got %d", len(viols))
	}
}
