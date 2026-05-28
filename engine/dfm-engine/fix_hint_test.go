package dfmengine

import (
	"math"
	"testing"
)

// Per-rule fix-hint regression tests. Each test asserts the structured
// FixHint fields rather than re-deriving the whole violation — the
// existing rule_*_test.go files already cover detection.

func TestEdgeClearance_FixHint_PadShiftsInward(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Component pad close to the right edge — the inward direction is -X.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 59.95, Y: 20, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE", RefDes: "C1"},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{MinEdgeClearanceMM: 0.2})
	if len(viols) == 0 {
		t.Fatal("expected a violation to attach a hint to")
	}
	v := viols[0]
	if v.FixAction != "shift" {
		t.Errorf("FixAction = %q, want shift", v.FixAction)
	}
	if v.FixTarget != "component" {
		t.Errorf("FixTarget = %q, want component", v.FixTarget)
	}
	if v.FixDX >= 0 {
		t.Errorf("FixDX = %v, want negative (pointing inward from right edge)", v.FixDX)
	}
	if v.FixMagnitudeMM <= 0 {
		t.Errorf("FixMagnitudeMM = %v, want > 0", v.FixMagnitudeMM)
	}
}

func TestEdgeClearance_FixHint_DrillShiftsInward(t *testing.T) {
	rule := &EdgeClearanceRule{}
	// Drill 0.05 mm from the bottom edge (y = 0). Inward = +Y.
	board := BoardData{
		Layers:  []Layer{{Name: "top_copper", Type: "COPPER"}},
		Drills:  []Drill{{X: 30, Y: 0.05, DiamMM: 0.5}},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{MinEdgeClearanceMM: 0.5})
	if len(viols) == 0 {
		t.Fatal("expected drill violation")
	}
	v := viols[0]
	if v.FixAction != "shift" || v.FixTarget != "drill" {
		t.Errorf("got action=%q target=%q, want shift/drill", v.FixAction, v.FixTarget)
	}
	if v.FixDY <= 0 {
		t.Errorf("FixDY = %v, want positive (inward from bottom)", v.FixDY)
	}
}

func TestDrillToDrill_FixHint_PointsAwayFromOther(t *testing.T) {
	rule := &DrillToDrillRule{}
	// Two drills 0.1 mm apart center-to-center along +X (limit 0.5).
	board := BoardData{
		Drills: []Drill{
			{X: 10, Y: 10, DiamMM: 0.3},
			{X: 10.5, Y: 10, DiamMM: 0.3},
		},
		Outline: rectOutline(20, 20),
	}
	viols := rule.Run(board, ProfileRules{MinDrillToDrillMM: 0.5})
	if len(viols) == 0 {
		t.Fatal("expected drill-to-drill violation")
	}
	v := viols[0]
	if v.FixAction != "shift" {
		t.Errorf("FixAction = %q, want shift", v.FixAction)
	}
	// Hint is attached to drill A (the earlier one in the sweep order),
	// so direction should point from A away from B — i.e. -X.
	if v.FixDX >= 0 {
		t.Errorf("FixDX = %v, want negative (A pushed away from B at +X)", v.FixDX)
	}
}

func TestDrillToCopper_FixHint_PointsAwayFromTrace(t *testing.T) {
	rule := &DrillToCopperRule{}
	// Drill above a horizontal trace at y=10 with a real gap (so it's a
	// DFM clearance issue, not an overlap). Distance center-to-trace =
	// 0.4 mm; with drill r=0.1 and trace half-w=0.1, gap = 0.2 < 0.3 mm
	// limit → violation. Inward = +Y.
	board := BoardData{
		Layers:  []Layer{{Name: "top_copper", Type: "COPPER"}},
		Drills:  []Drill{{X: 10, Y: 10.4, DiamMM: 0.2}},
		Traces:  []Trace{{Layer: "top_copper", StartX: 5, StartY: 10, EndX: 15, EndY: 10, WidthMM: 0.2, NetName: "VCC"}},
		Outline: rectOutline(20, 20),
	}
	viols := rule.Run(board, ProfileRules{MinDrillToCopperMM: 0.3})
	if len(viols) == 0 {
		t.Fatal("expected drill-to-copper trace violation")
	}
	v := viols[0]
	if v.FixAction != "shift" || v.FixTarget != "drill" {
		t.Errorf("got action=%q target=%q, want shift/drill", v.FixAction, v.FixTarget)
	}
	if v.FixDY <= 0 {
		t.Errorf("FixDY = %v, want positive (drill above trace)", v.FixDY)
	}
}

func TestClearance_FixHint_PadShiftsAwayFromTrace(t *testing.T) {
	rule := &ClearanceRule{}
	// Pad to the right of a vertical trace. Real gap so it's a DFM
	// clearance issue, not an overlap: pad-edge to trace-edge ~= 0.05 mm,
	// which is below the 0.3 mm limit.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{{Layer: "top_copper", StartX: 10, StartY: 5, EndX: 10, EndY: 15, WidthMM: 0.2, NetName: "SIGA"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 10.3, Y: 10, WidthMM: 0.3, HeightMM: 0.3, Shape: "RECT", NetName: "SIGB", RefDes: "C2"},
		},
		Outline: rectOutline(20, 20),
	}
	viols := rule.Run(board, ProfileRules{MinClearanceMM: 0.3})
	if len(viols) == 0 {
		t.Fatal("expected trace-to-pad clearance violation")
	}
	v := viols[0]
	if v.FixAction != "shift" || v.FixTarget != "pad" {
		t.Errorf("got action=%q target=%q, want shift/pad", v.FixAction, v.FixTarget)
	}
	if v.FixDX <= 0 {
		t.Errorf("FixDX = %v, want positive (pad pushed away from trace at -X)", v.FixDX)
	}
}

func TestSilkscreenOnPad_FixHint_PushesSilkOffPad(t *testing.T) {
	rule := &SilkscreenOnPadRule{}
	// Silk feature center sitting right on a pad. Direction should point
	// from pad center to silk center (which is the same point here — the
	// helper guards zero vectors). Use an off-center silk so the vector
	// has a definite direction.
	board := BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "top_silk", Type: "SILK"},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: 10, Y: 10, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "C3"},
			// silk pad overlapping the copper pad, offset +X by 0.2
			{Layer: "top_silk", X: 10.2, Y: 10, WidthMM: 1, HeightMM: 1, Shape: "RECT"},
		},
		Outline: rectOutline(20, 20),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) == 0 {
		t.Fatal("expected silk-on-pad violation")
	}
	v := viols[0]
	if v.FixAction != "shift" || v.FixTarget != "silk" {
		t.Errorf("got action=%q target=%q, want shift/silk", v.FixAction, v.FixTarget)
	}
	if v.FixDX <= 0 {
		t.Errorf("FixDX = %v, want positive (silk shifted +X off pad)", v.FixDX)
	}
}

func TestTombstoning_FixHint_ResizeTowardLargerPad(t *testing.T) {
	rule := &TombstoningRiskRule{}
	// 0402 with one pad 2x larger than the other on a 2-pad component.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 9, Y: 10, WidthMM: 0.5, HeightMM: 0.5, Shape: "RECT", RefDes: "C5", PackageClass: "0402"},
			{Layer: "top_copper", X: 11, Y: 10, WidthMM: 1.0, HeightMM: 1.0, Shape: "RECT", RefDes: "C5", PackageClass: "0402"},
		},
		Outline: rectOutline(20, 20),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) == 0 {
		t.Fatal("expected tombstoning violation")
	}
	v := viols[0]
	if v.FixAction != "resize" {
		t.Errorf("FixAction = %q, want resize", v.FixAction)
	}
	if v.FixTarget != "pad" {
		t.Errorf("FixTarget = %q, want pad", v.FixTarget)
	}
	// Smaller pad is at x=9, larger at x=11 → vector points +X.
	if v.FixDX <= 0 {
		t.Errorf("FixDX = %v, want positive (toward larger pad)", v.FixDX)
	}
	if v.FixMagnitudeMM <= 0 {
		t.Errorf("FixMagnitudeMM = %v, want > 0", v.FixMagnitudeMM)
	}
}

func TestTraceImbalance_FixHint_ResizesTrace(t *testing.T) {
	rule := TraceImbalanceRule{}
	// 2-pad component with one pad on a 1mm trace and the other on a
	// 0.1mm trace — 10:1 imbalance, well over default ratio.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 5, Y: 10, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "A"},
			{Layer: "top_copper", X: 15, Y: 10, WidthMM: 1, HeightMM: 1, Shape: "RECT", RefDes: "R1", NetName: "B"},
		},
		Traces: []Trace{
			{Layer: "top_copper", StartX: 5, StartY: 10, EndX: 0, EndY: 10, WidthMM: 1.0, NetName: "A"},
			{Layer: "top_copper", StartX: 15, StartY: 10, EndX: 20, EndY: 10, WidthMM: 0.1, NetName: "B"},
		},
		Outline: rectOutline(20, 20),
	}
	viols := rule.Run(board, ProfileRules{MaxTraceImbalanceRatio: 2.0})
	if len(viols) == 0 {
		t.Fatal("expected trace-imbalance violation")
	}
	v := viols[0]
	if v.FixAction != "resize" || v.FixTarget != "trace" {
		t.Errorf("got action=%q target=%q, want resize/trace", v.FixAction, v.FixTarget)
	}
	// Direction is from pads[0] toward pads[1] (the partner). pads[0] is
	// at x=5, pads[1] at x=15 → vector +X.
	if v.FixDX <= 0 {
		t.Errorf("FixDX = %v, want positive (along pair axis)", v.FixDX)
	}
}

func TestFiducialCount_FixHint_AddAtFreeCorner(t *testing.T) {
	rule := &FiducialRule{}
	// One existing fiducial — count is 1, below the minimum 3. The rule
	// should emit a single violation with an add hint pointing to a
	// corner inside the board outline.
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "top_copper", X: 5, Y: 5, WidthMM: 1, HeightMM: 1, Shape: "CIRCLE", IsFiducial: true},
		},
		Outline: rectOutline(60, 40),
	}
	viols := rule.Run(board, ProfileRules{})
	if len(viols) != 1 {
		t.Fatalf("expected exactly 1 fiducial violation, got %d", len(viols))
	}
	v := viols[0]
	if v.FixAction != "add" {
		t.Errorf("FixAction = %q, want add", v.FixAction)
	}
	if v.FixTarget != "fiducial" {
		t.Errorf("FixTarget = %q, want fiducial", v.FixTarget)
	}
	// Anchor (X, Y) is the board center. The bbox is [0,60] x [0,40] so
	// center is (30, 20).
	if math.Abs(v.X-30) > 1 || math.Abs(v.Y-20) > 1 {
		t.Errorf("anchor: got (%v,%v), want near (30,20)", v.X, v.Y)
	}
	// Target (X2, Y2) must be inside the outline bbox.
	if v.X2 < 0 || v.X2 > 60 || v.Y2 < 0 || v.Y2 > 40 {
		t.Errorf("target (%v,%v) outside outline bbox", v.X2, v.Y2)
	}
	// And must not coincide with the existing fiducial at (5, 5) — the
	// "furthest corner" heuristic should pick another corner.
	dist := math.Hypot(v.X2-5, v.Y2-5)
	if dist < 10 {
		t.Errorf("suggested fiducial position (%v,%v) too close to existing fiducial at (5,5): dist=%v", v.X2, v.Y2, dist)
	}
}

func TestTraceWidth_FixHint_AbsentByDesign(t *testing.T) {
	// Non-spatial rules don't get hints in v1. Regression guard against
	// accidentally adding a hint to a rule that doesn't have a natural
	// directional fix.
	rule := &TraceWidthRule{}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{{Layer: "top_copper", StartX: 0, StartY: 0, EndX: 10, EndY: 0, WidthMM: 0.05, NetName: "X"}},
	}
	viols := rule.Run(board, ProfileRules{MinTraceWidthMM: 0.15})
	if len(viols) == 0 {
		t.Fatal("expected trace-width violation")
	}
	if viols[0].FixAction != "" {
		t.Errorf("trace-width should not emit a fix hint in v1, got action=%q", viols[0].FixAction)
	}
}
