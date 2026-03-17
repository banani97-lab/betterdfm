package dfmengine

import "testing"

func TestDrillToDrill_TooClose(t *testing.T) {
	rule := &DrillToDrillRule{}
	// Two 1mm-diameter drills with centers 1.1mm apart → edge-to-edge = 0.1mm < 0.25mm limit.
	board := BoardData{
		Drills: []Drill{
			{X: 0, Y: 0, DiamMM: 1.0, Plated: true},
			{X: 1.1, Y: 0, DiamMM: 1.0, Plated: true},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinDrillToDrillMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected ≥1 violation, got 0")
	}
	v := viols[0]
	if v.RuleID != "drill-to-drill" {
		t.Errorf("expected RuleID=drill-to-drill, got %s", v.RuleID)
	}
	if v.MeasuredMM >= 0.25 {
		t.Errorf("MeasuredMM should be below 0.25, got %f", v.MeasuredMM)
	}
	if v.LimitMM != 0.25 {
		t.Errorf("LimitMM should be 0.25, got %f", v.LimitMM)
	}
}

func TestDrillToDrill_OK(t *testing.T) {
	rule := &DrillToDrillRule{}
	// Two 1mm-diameter drills with centers 2mm apart → edge-to-edge = 1mm > 0.25mm limit.
	board := BoardData{
		Drills: []Drill{
			{X: 0, Y: 0, DiamMM: 1.0, Plated: true},
			{X: 2.0, Y: 0, DiamMM: 1.0, Plated: true},
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinDrillToDrillMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(viols))
	}
}

func TestDrillToDrill_NoProfile(t *testing.T) {
	rule := &DrillToDrillRule{}
	board := BoardData{
		Drills: []Drill{
			{X: 0, Y: 0, DiamMM: 1.0, Plated: true},
			{X: 0.5, Y: 0, DiamMM: 1.0, Plated: true},
		},
		Outline: rectOutline(60, 40),
	}
	// MinDrillToDrillMM = 0 → rule disabled
	profile := ProfileRules{}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("rule should be disabled when MinDrillToDrillMM=0, got %d violations", len(viols))
	}
}

func TestDrillToDrill_ViaAndDrillTooClose(t *testing.T) {
	rule := &DrillToDrillRule{}
	// Via drill radius = 0.15mm, drill radius = 0.15mm, centers 0.2mm apart → gap = -0.1mm → skip.
	// Centers 0.4mm apart → gap = 0.1mm < 0.25mm → violation.
	board := BoardData{
		Drills: []Drill{{X: 0, Y: 0, DiamMM: 0.3, Plated: true}},
		Vias:   []Via{{X: 0.4, Y: 0, OuterDiamMM: 0.6, DrillDiamMM: 0.3}},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinDrillToDrillMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) == 0 {
		t.Fatal("expected violation for via+drill too close, got 0")
	}
}

func TestDrillToDrill_OverlappingSkipped(t *testing.T) {
	rule := &DrillToDrillRule{}
	// Overlapping drills (gap < 0) should be skipped — that's a DRC issue, not DFM.
	board := BoardData{
		Drills: []Drill{
			{X: 0, Y: 0, DiamMM: 1.0, Plated: true},
			{X: 0.3, Y: 0, DiamMM: 1.0, Plated: true}, // centers 0.3mm apart, radii sum 1mm → overlap
		},
		Outline: rectOutline(60, 40),
	}
	profile := ProfileRules{MinDrillToDrillMM: 0.25}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("overlapping drills should not generate DFM violations, got %d", len(viols))
	}
}
