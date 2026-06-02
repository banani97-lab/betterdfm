package dfmengine

import "testing"

// These tests confirm each discrete check honors its profile on/off toggle:
// a board that produces a violation when the check is enabled must produce
// none when the toggle is set to false. boolPtr is defined in
// rule_through_hole_on_bottom_test.go.

func TestFiducialCount_DisabledByProfile(t *testing.T) {
	// 2 fiducials (< minFiducials) would normally WARNING.
	board := BoardData{
		Layers:  spacingLayers(),
		Pads:    []Pad{fiducialPad(0, 0), fiducialPad(20, 0)},
		Outline: rectOutline(60, 40),
	}
	if vs := (&FiducialRule{}).Run(board, ProfileRules{}); len(vs) != 1 {
		t.Fatalf("sanity: expected 1 violation when enabled, got %+v", vs)
	}
	vs := (&FiducialRule{}).Run(board, ProfileRules{EnableFiducialCountCheck: boolPtr(false)})
	if len(vs) != 0 {
		t.Fatalf("disabled fiducial-count check must produce no violations, got %+v", vs)
	}
}

func TestPadSizeForPackage_DisabledByProfile(t *testing.T) {
	// Undersized 0805 pad would normally produce an ERROR.
	board := BoardData{
		SourceFormat: "ODB_PLUS_PLUS",
		Layers:       fourLayerStack(),
		Pads: []Pad{
			{Layer: "L01_TOP", X: 10, Y: 10, WidthMM: 0.50, HeightMM: 0.50,
				Shape: "RECT", RefDes: "R169", PackageClass: "0805"},
		},
	}
	if vs := (&PadSizeForPackageRule{}).Run(board, ProfileRules{}); len(vs) == 0 {
		t.Fatalf("sanity: expected a violation when enabled, got none")
	}
	vs := (&PadSizeForPackageRule{}).Run(board, ProfileRules{EnablePadSizeForPackageCheck: boolPtr(false)})
	if len(vs) != 0 {
		t.Fatalf("disabled pad-size-for-package check must produce no violations, got %+v", vs)
	}
}

func TestTombstoningRisk_DisabledByProfile(t *testing.T) {
	// 0402 with grossly unbalanced pad areas (ratio 4.0 > 1.3) would ERROR.
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 0.5, HeightMM: 0.5, Shape: "RECT", RefDes: "C1", PackageClass: "0402"},
			{Layer: "top_copper", X: 11, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "RECT", RefDes: "C1", PackageClass: "0402"},
		},
		Outline: rectOutline(60, 40),
	}
	if vs := (&TombstoningRiskRule{}).Run(board, ProfileRules{}); len(vs) != 1 {
		t.Fatalf("sanity: expected 1 violation when enabled, got %+v", vs)
	}
	vs := (&TombstoningRiskRule{}).Run(board, ProfileRules{EnableTombstoningRiskCheck: boolPtr(false)})
	if len(vs) != 0 {
		t.Fatalf("disabled tombstoning-risk check must produce no violations, got %+v", vs)
	}
}

func TestViaInPad_DisabledByProfile(t *testing.T) {
	// Fine-pitch BGA via-in-pad would normally WARNING.
	board := BoardData{
		Layers:     spacingLayers(),
		Components: []Component{{RefDes: "U1", MountType: "smt", PackageClass: "BGA256"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "U1", PackageClass: "BGA256", IsViaCatchPad: true},
		},
		Outline: rectOutline(60, 40),
	}
	if vs := (&ViaInPadRule{}).Run(board, ProfileRules{}); len(vs) != 1 {
		t.Fatalf("sanity: expected 1 violation when enabled, got %+v", vs)
	}
	vs := (&ViaInPadRule{}).Run(board, ProfileRules{EnableViaInPadCheck: boolPtr(false)})
	if len(vs) != 0 {
		t.Fatalf("disabled via-in-pad check must produce no violations, got %+v", vs)
	}
}
