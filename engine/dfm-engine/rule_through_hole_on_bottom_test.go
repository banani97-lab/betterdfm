package dfmengine

import "testing"

func runTHBottom(board BoardData, profile ProfileRules) []Violation {
	return (&ThroughHoleOnBottomRule{}).Run(board, profile)
}

func boolPtr(b bool) *bool { return &b }

func TestThroughHoleOnBottom_FlagsTHTAndPressfit(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "J1", Side: "bot", MountType: "thmt"},
		{RefDes: "P2", Side: "bot", MountType: "pressfit"},
	}}
	vs := runTHBottom(board, ProfileRules{})
	if len(vs) != 2 {
		t.Fatalf("expected 2 violations (default-on), got %d: %+v", len(vs), vs)
	}
	for _, v := range vs {
		if v.Severity != "WARNING" {
			t.Fatalf("expected WARNING, got %s", v.Severity)
		}
	}
}

func TestThroughHoleOnBottom_TopSideIgnored(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "J1", Side: "top", MountType: "thmt"},
	}}
	if vs := runTHBottom(board, ProfileRules{}); len(vs) != 0 {
		t.Fatalf("top-side THT should be ignored, got %+v", vs)
	}
}

func TestThroughHoleOnBottom_SMTBottomIgnored(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "C1", Side: "bot", MountType: "smt"},
	}}
	if vs := runTHBottom(board, ProfileRules{}); len(vs) != 0 {
		t.Fatalf("bottom-side SMT should be ignored (handled by component-height), got %+v", vs)
	}
}

func TestThroughHoleOnBottom_DisabledByFlag(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "J1", Side: "bot", MountType: "thmt"},
	}}
	if vs := runTHBottom(board, ProfileRules{FlagThroughHoleOnBottom: boolPtr(false)}); len(vs) != 0 {
		t.Fatalf("flag=false should disable the rule, got %+v", vs)
	}
}

func TestThroughHoleOnBottom_NilFlagEnabled(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "J1", Side: "bot", MountType: "thmt"},
	}}
	// nil pointer (default) means the check is on.
	if vs := runTHBottom(board, ProfileRules{FlagThroughHoleOnBottom: nil}); len(vs) != 1 {
		t.Fatalf("nil flag should enable the rule, got %+v", vs)
	}
}

func TestThroughHoleOnBottom_MountingHolesSkipped(t *testing.T) {
	board := BoardData{Components: []Component{
		{RefDes: "MH1", Side: "bot", MountType: "thmt"},
		{RefDes: "TP3", Side: "bot", MountType: "pressfit"},
	}}
	if vs := runTHBottom(board, ProfileRules{}); len(vs) != 0 {
		t.Fatalf("mounting holes / test points should be skipped, got %+v", vs)
	}
}
