package dfmengine

import "testing"

func runViaInPad(board BoardData) []Violation {
	return (&ViaInPadRule{}).Run(board, ProfileRules{})
}

func TestViaInPad_FinePitchBGAIsWarning(t *testing.T) {
	board := BoardData{
		Layers:     spacingLayers(),
		Components: []Component{{RefDes: "U1", MountType: "smt", PackageClass: "BGA256"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "U1", PackageClass: "BGA256", IsViaCatchPad: true},
		},
		Outline: rectOutline(60, 40),
	}
	vs := runViaInPad(board)
	if len(vs) != 1 || vs[0].Severity != "WARNING" {
		t.Fatalf("expected 1 WARNING for fine-pitch BGA via-in-pad, got %+v", vs)
	}
	if vs[0].RefDes != "U1" {
		t.Fatalf("unexpected refdes: %+v", vs[0])
	}
}

func TestViaInPad_CoarsePitchIsInfo(t *testing.T) {
	// A lone coarse land (no BGA class, no pitch info) is advisory only.
	board := BoardData{
		Layers:     spacingLayers(),
		Components: []Component{{RefDes: "TP_DUMMY", MountType: "smt"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "RECT",
				RefDes: "Q1", PackageClass: "SOT23", IsViaCatchPad: true},
		},
		Outline: rectOutline(60, 40),
	}
	vs := runViaInPad(board)
	if len(vs) != 1 || vs[0].Severity != "INFO" {
		t.Fatalf("expected 1 INFO for coarse via-in-pad, got %+v", vs)
	}
}

func TestViaInPad_PitchDrivenFinePitch(t *testing.T) {
	// Two lands 0.4mm apart (center-to-center) -> fine pitch even without a BGA class.
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			{Layer: "top_copper", X: 10.0, Y: 10, WidthMM: 0.2, HeightMM: 0.2, Shape: "RECT",
				RefDes: "U2", IsViaCatchPad: true},
			{Layer: "top_copper", X: 10.4, Y: 10, WidthMM: 0.2, HeightMM: 0.2, Shape: "RECT",
				RefDes: "U2"},
		},
		Outline: rectOutline(60, 40),
	}
	vs := runViaInPad(board)
	if len(vs) != 1 || vs[0].Severity != "WARNING" {
		t.Fatalf("expected 1 WARNING from 0.4mm pitch, got %+v", vs)
	}
}

func TestViaInPad_ThroughHoleExcluded(t *testing.T) {
	// THT pin pad sits on its own drill; must not be flagged.
	board := BoardData{
		Layers:     spacingLayers(),
		Components: []Component{{RefDes: "J1", MountType: "thmt"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1.5, HeightMM: 1.5, Shape: "CIRCLE",
				RefDes: "J1", IsViaCatchPad: true},
		},
		Outline: rectOutline(60, 40),
	}
	if vs := runViaInPad(board); len(vs) != 0 {
		t.Fatalf("THT pin pad must not be flagged, got %+v", vs)
	}
}

func TestViaInPad_DonutAndHoledPadsExcluded(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "DONUT",
				RefDes: "U3", IsViaCatchPad: true},
			{Layer: "top_copper", X: 20, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "CIRCLE",
				RefDes: "U4", HoleMM: 0.4, IsViaCatchPad: true},
		},
		Outline: rectOutline(60, 40),
	}
	if vs := runViaInPad(board); len(vs) != 0 {
		t.Fatalf("donut / holed pads must not be flagged, got %+v", vs)
	}
}

func TestViaInPad_NormalLandNotFlagged(t *testing.T) {
	// SMD land with no via under it (IsViaCatchPad false) is clean.
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "U5", PackageClass: "BGA256"},
		},
		Outline: rectOutline(60, 40),
	}
	if vs := runViaInPad(board); len(vs) != 0 {
		t.Fatalf("normal land without a via must not be flagged, got %+v", vs)
	}
}

func TestViaInPad_TestPointsSkipped(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "TP1", IsViaCatchPad: true},
		},
		Outline: rectOutline(60, 40),
	}
	if vs := runViaInPad(board); len(vs) != 0 {
		t.Fatalf("test points must be skipped, got %+v", vs)
	}
}
