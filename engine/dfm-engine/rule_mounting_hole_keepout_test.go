package dfmengine

import "testing"

func runMHKeepout(board BoardData, profile ProfileRules) []Violation {
	return (&MountingHoleKeepoutRule{}).Run(board, profile)
}

func mhProfile(keepout float64) ProfileRules {
	return ProfileRules{MinMountingHoleKeepoutMM: keepout}
}

func TestMountingHoleKeepout_Disabled(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 3.2, Plated: false}},
		Traces: []Trace{{Layer: "top_copper", WidthMM: 0.2, StartX: 12, StartY: 8, EndX: 12, EndY: 12}},
	}
	if vs := runMHKeepout(board, mhProfile(0)); len(vs) != 0 {
		t.Fatalf("keepout=0 disables the rule, got %+v", vs)
	}
}

func TestMountingHoleKeepout_TraceTooClose(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 3.2, Plated: false}},
		// vertical trace at x=12: center distance 2.0, gap = 2.0 - 0.1 - 1.6 = 0.3
		Traces: []Trace{{Layer: "top_copper", WidthMM: 0.2, StartX: 12, StartY: 8, EndX: 12, EndY: 12}},
	}
	vs := runMHKeepout(board, mhProfile(0.5))
	if len(vs) != 1 || vs[0].Severity != "WARNING" {
		t.Fatalf("expected 1 WARNING for trace in keepout, got %+v", vs)
	}
	if vs[0].MeasuredMM <= 0 || vs[0].MeasuredMM >= 0.5 {
		t.Fatalf("expected positive gap below limit, got measured=%v", vs[0].MeasuredMM)
	}
}

func TestMountingHoleKeepout_TraceFarEnough(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 3.2, Plated: false}},
		Traces: []Trace{{Layer: "top_copper", WidthMM: 0.2, StartX: 20, StartY: 8, EndX: 20, EndY: 12}},
	}
	if vs := runMHKeepout(board, mhProfile(0.5)); len(vs) != 0 {
		t.Fatalf("trace well clear of keepout should not be flagged, got %+v", vs)
	}
}

func TestMountingHoleKeepout_PlatedHoleIgnored(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 3.2, Plated: true}},
		Traces: []Trace{{Layer: "top_copper", WidthMM: 0.2, StartX: 12, StartY: 8, EndX: 12, EndY: 12}},
	}
	if vs := runMHKeepout(board, mhProfile(0.5)); len(vs) != 0 {
		t.Fatalf("plated holes (may carry copper intentionally) should be ignored, got %+v", vs)
	}
}

func TestMountingHoleKeepout_SmallHoleIgnored(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		// 0.6mm non-plated hole is a via, not a mounting hole.
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 0.6, Plated: false}},
		Traces: []Trace{{Layer: "top_copper", WidthMM: 0.2, StartX: 10.5, StartY: 8, EndX: 10.5, EndY: 12}},
	}
	if vs := runMHKeepout(board, mhProfile(0.5)); len(vs) != 0 {
		t.Fatalf("sub-2mm holes should not be treated as mounting holes, got %+v", vs)
	}
}

func TestMountingHoleKeepout_PadOverlapNegativeGap(t *testing.T) {
	board := BoardData{
		Layers: spacingLayers(),
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 3.2, Plated: false}},
		Pads:   []Pad{{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1, HeightMM: 1, Shape: "RECT"}},
	}
	vs := runMHKeepout(board, mhProfile(0.5))
	if len(vs) != 1 || vs[0].MeasuredMM >= 0 {
		t.Fatalf("copper overlapping the hole should report a negative gap, got %+v", vs)
	}
}

func TestMountingHoleKeepout_OnePerHole(t *testing.T) {
	// Two offending traces near the same hole -> a single (worst) violation.
	board := BoardData{
		Layers: spacingLayers(),
		Drills: []Drill{{X: 10, Y: 10, DiamMM: 3.2, Plated: false}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.2, StartX: 12, StartY: 8, EndX: 12, EndY: 12},
			{Layer: "top_copper", WidthMM: 0.2, StartX: 8, StartY: 8, EndX: 8, EndY: 12},
		},
	}
	if vs := runMHKeepout(board, mhProfile(0.5)); len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation per hole, got %d: %+v", len(vs), vs)
	}
}
