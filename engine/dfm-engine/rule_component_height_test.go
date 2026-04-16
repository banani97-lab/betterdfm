package dfmengine

import "testing"

func runHeight(board BoardData, profile ProfileRules) []Violation {
	return ComponentHeightRule{}.Run(board, profile)
}

func TestComponentHeight_BothLimitsDisabled(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "C1", Side: "top", MountType: "smt", HeightMM: 100},
	}}
	if got := runHeight(board, ProfileRules{}); len(got) != 0 {
		t.Fatalf("disabled rule should return no violations, got %d", len(got))
	}
}

func TestComponentHeight_FlagsTopOverLimit(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "C100", Side: "top", MountType: "smt", HeightMM: 22.0},
		{RefDes: "C101", Side: "top", MountType: "smt", HeightMM: 5.0},
	}}
	vs := runHeight(board, ProfileRules{
		MaxComponentHeightTopMM:    10,
		MaxComponentHeightBottomMM: 5,
	})
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for C100, got %d: %+v", len(vs), vs)
	}
	if vs[0].RefDes != "C100" || vs[0].Severity != "ERROR" {
		t.Fatalf("unexpected violation: %+v", vs[0])
	}
	if vs[0].MeasuredMM != 22.0 || vs[0].LimitMM != 10 {
		t.Fatalf("unexpected measurement/limit: %+v", vs[0])
	}
}

func TestComponentHeight_FlagsBottomOverLimit(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "L1", Side: "bot", MountType: "smt", HeightMM: 10.0},
		{RefDes: "L2", Side: "bot", MountType: "smt", HeightMM: 4.0},
	}}
	vs := runHeight(board, ProfileRules{
		MaxComponentHeightTopMM:    10,
		MaxComponentHeightBottomMM: 5,
	})
	if len(vs) != 1 || vs[0].RefDes != "L1" {
		t.Fatalf("expected 1 violation for L1, got %+v", vs)
	}
}

func TestComponentHeight_IgnoresThroughHole(t *testing.T) {
	// Tall THMT connectors shouldn't trip the SMT-only rule.
	board := BoardData{Components: []Component{
		{RefDes: "J1", Side: "top", MountType: "thmt", HeightMM: 30.0},
		{RefDes: "J2", Side: "bot", MountType: "thmt", HeightMM: 30.0},
	}}
	vs := runHeight(board, ProfileRules{
		MaxComponentHeightTopMM:    10,
		MaxComponentHeightBottomMM: 5,
	})
	if len(vs) != 0 {
		t.Fatalf("THMT must be ignored, got %d violations: %+v", len(vs), vs)
	}
}

func TestComponentHeight_IgnoresFiducialsAndTestPoints(t *testing.T) {
	// Fiducials sometimes inherit library-default heights we don't trust.
	board := BoardData{Components: []Component{
		{RefDes: "FID1", Side: "top", MountType: "smt", HeightMM: 20.0},
		{RefDes: "TP7", Side: "bot", MountType: "smt", HeightMM: 20.0},
		{RefDes: "MH3", Side: "top", MountType: "smt", HeightMM: 20.0},
	}}
	vs := runHeight(board, ProfileRules{
		MaxComponentHeightTopMM:    10,
		MaxComponentHeightBottomMM: 5,
	})
	if len(vs) != 0 {
		t.Fatalf("fiducials / test points / mounting holes must be ignored, got %+v", vs)
	}
}

func TestComponentHeight_OneSideDisabled(t *testing.T) {
	// Setting only top limit disables bottom checks entirely (and vice versa).
	board := BoardData{Components: []Component{
		{RefDes: "A", Side: "top", MountType: "smt", HeightMM: 12},
		{RefDes: "B", Side: "bot", MountType: "smt", HeightMM: 12},
	}}
	vs := runHeight(board, ProfileRules{MaxComponentHeightTopMM: 10})
	if len(vs) != 1 || vs[0].RefDes != "A" {
		t.Fatalf("expected only A flagged when bottom disabled, got %+v", vs)
	}
}

func TestComponentHeight_AggregatesMissingHeightAsInfo(t *testing.T) {
	// Components with no height metadata get rolled into a single INFO.
	board := BoardData{Components: []Component{
		{RefDes: "U1", Side: "top", MountType: "smt", HeightMM: 0},
		{RefDes: "U2", Side: "top", MountType: "smt", HeightMM: 0},
		{RefDes: "U3", Side: "bot", MountType: "smt", HeightMM: 0},
	}}
	vs := runHeight(board, ProfileRules{
		MaxComponentHeightTopMM:    10,
		MaxComponentHeightBottomMM: 5,
	})
	if len(vs) != 1 {
		t.Fatalf("expected 1 aggregate INFO, got %d: %+v", len(vs), vs)
	}
	if vs[0].Severity != "INFO" {
		t.Fatalf("expected INFO severity, got %s", vs[0].Severity)
	}
}

func TestComponentHeight_UnknownSideSkipped(t *testing.T) {
	// If side wasn't recovered we don't guess — skip.
	board := BoardData{Components: []Component{
		{RefDes: "X1", Side: "", MountType: "smt", HeightMM: 30},
	}}
	vs := runHeight(board, ProfileRules{
		MaxComponentHeightTopMM:    10,
		MaxComponentHeightBottomMM: 5,
	})
	if len(vs) != 0 {
		t.Fatalf("unknown side should skip, got %+v", vs)
	}
}
