package dfmengine

import "testing"

func runFidPlacement(board BoardData) []Violation {
	return (&FiducialPlacementRule{}).Run(board, ProfileRules{})
}

func fiducialPad(x, y float64) Pad {
	return Pad{Layer: "top_copper", X: x, Y: y, WidthMM: 1, HeightMM: 1, Shape: "CIRCLE", IsFiducial: true}
}

func TestFiducialPlacement_NoFiducialsSkips(t *testing.T) {
	board := BoardData{Layers: spacingLayers(), Outline: rectOutline(60, 40)}
	if vs := runFidPlacement(board); len(vs) != 0 {
		t.Fatalf("rule must skip when no fiducials present, got %+v", vs)
	}
}

func TestFiducialPlacement_WideTriangleOK(t *testing.T) {
	board := BoardData{
		Layers:  spacingLayers(),
		Pads:    []Pad{fiducialPad(0, 0), fiducialPad(50, 0), fiducialPad(25, 40)},
		Outline: rectOutline(60, 40),
	}
	if vs := runFidPlacement(board); len(vs) != 0 {
		t.Fatalf("a wide fiducial triangle should not be flagged, got %+v", vs)
	}
}

func TestFiducialPlacement_CollinearWarning(t *testing.T) {
	board := BoardData{
		Layers:  spacingLayers(),
		Pads:    []Pad{fiducialPad(0, 0), fiducialPad(10, 0), fiducialPad(20, 0)},
		Outline: rectOutline(60, 40),
	}
	vs := runFidPlacement(board)
	if len(vs) != 1 || vs[0].Severity != "WARNING" {
		t.Fatalf("expected 1 WARNING for collinear fiducials, got %+v", vs)
	}
}

func TestFiducialPlacement_TwoFiducialsNoCollinearCheck(t *testing.T) {
	// Count insufficiency is FiducialRule's concern; this rule should stay quiet.
	board := BoardData{
		Layers:  spacingLayers(),
		Pads:    []Pad{fiducialPad(0, 0), fiducialPad(20, 0)},
		Outline: rectOutline(60, 40),
	}
	if vs := runFidPlacement(board); len(vs) != 0 {
		t.Fatalf("two fiducials should not trigger collinearity, got %+v", vs)
	}
}

func TestFiducialPlacement_FinePitchMissingLocalFiducial(t *testing.T) {
	// One global fiducial far from a BGA -> INFO advising a local fiducial.
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			fiducialPad(0, 0),
			{Layer: "top_copper", X: 30, Y: 30, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "U1", PackageClass: "BGA256"},
			{Layer: "top_copper", X: 31, Y: 31, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "U1", PackageClass: "BGA256"},
		},
		Outline: rectOutline(60, 40),
	}
	vs := runFidPlacement(board)
	if len(vs) != 1 || vs[0].Severity != "INFO" || vs[0].RefDes != "U1" {
		t.Fatalf("expected 1 INFO for U1 missing local fiducial, got %+v", vs)
	}
}

func TestFiducialPlacement_FinePitchWithLocalFiducialOK(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Pads: []Pad{
			fiducialPad(30, 30), // sits on the BGA, within margin
			{Layer: "top_copper", X: 30, Y: 30, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "U1", PackageClass: "BGA256"},
			{Layer: "top_copper", X: 31, Y: 31, WidthMM: 0.3, HeightMM: 0.3, Shape: "CIRCLE",
				RefDes: "U1", PackageClass: "BGA256"},
		},
		Outline: rectOutline(60, 40),
	}
	if vs := runFidPlacement(board); len(vs) != 0 {
		t.Fatalf("BGA with a nearby fiducial should not be flagged, got %+v", vs)
	}
}
