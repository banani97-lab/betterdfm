package dfmengine

import (
	"fmt"
	"reflect"
	"testing"
)

// denseClearanceBoard builds a board with `nLayers` copper layers, each
// containing `nTraces` parallel horizontal traces at 0.15 mm centerline
// spacing. With width=0.1 and minC=0.18, every adjacent pair clears at 0.05
// mm — well under the limit — producing nTraces-1 raw violations per layer.
// Use nTraces large enough to hit the per-layer cap (500).
func denseClearanceBoard(nLayers, nTraces int) BoardData {
	board := BoardData{Outline: rectOutline(60, 200)}
	for li := 0; li < nLayers; li++ {
		layer := fmt.Sprintf("L%02d", li+1)
		board.Layers = append(board.Layers, Layer{Name: layer, Type: "COPPER"})
		for i := 0; i < nTraces; i++ {
			y := 5.0 + 0.15*float64(i)
			net := fmt.Sprintf("NET_%s_%d", layer, i)
			board.Traces = append(board.Traces, Trace{
				Layer:   layer,
				WidthMM: 0.1,
				StartX:  5,
				StartY:  y,
				EndX:    55,
				EndY:    y,
				NetName: net,
			})
		}
	}
	return board
}

// passivePairBoard builds n small-passive 2-pad components with asymmetric
// pad areas (ratio > 1.3) so the tombstoning rule fires once per component.
func passivePairBoard(n int) BoardData {
	board := BoardData{
		Layers:  []Layer{{Name: "L01_TOP", Type: "COPPER"}},
		Outline: rectOutline(200, 200),
	}
	for i := 0; i < n; i++ {
		ref := fmt.Sprintf("C%04d", i)
		x := 5.0 + float64(i%50)*3.0
		y := 5.0 + float64(i/50)*3.0
		// Big pad
		board.Pads = append(board.Pads, Pad{
			Layer: "L01_TOP", X: x, Y: y,
			WidthMM: 1.0, HeightMM: 1.0,
			RefDes: ref, PackageClass: "0402",
		})
		// Small pad — area ratio 1:0.4 = 2.5, well over the 1.3 limit
		board.Pads = append(board.Pads, Pad{
			Layer: "L01_TOP", X: x + 1.5, Y: y,
			WidthMM: 0.4, HeightMM: 0.4,
			RefDes: ref, PackageClass: "0402",
		})
	}
	return board
}

func TestClearanceDeterministic(t *testing.T) {
	board := denseClearanceBoard(3, 600)
	profile := ProfileRules{MinClearanceMM: 0.18}
	rule := &ClearanceRule{}

	first := rule.Run(board, profile)
	if len(first) == 0 {
		t.Fatal("expected violations from dense fixture, got none")
	}
	for i := 0; i < 9; i++ {
		next := rule.Run(board, profile)
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("clearance non-deterministic on run %d: len first=%d next=%d", i+2, len(first), len(next))
		}
	}
}

func TestClearancePerLayerCap(t *testing.T) {
	board := denseClearanceBoard(3, 600)
	profile := ProfileRules{MinClearanceMM: 0.18}
	rule := &ClearanceRule{}
	violations := rule.Run(board, profile)

	byLayer := map[string]int{}
	for _, v := range violations {
		byLayer[v.Layer]++
	}
	if len(byLayer) != 3 {
		t.Fatalf("expected violations on all 3 layers, got %d: %v", len(byLayer), byLayer)
	}
	for _, layer := range []string{"L01", "L02", "L03"} {
		c := byLayer[layer]
		if c == 0 {
			t.Errorf("layer %s contributed no violations — per-layer cap likely starved it", layer)
		}
		if c > maxClearanceViolations {
			t.Errorf("layer %s deduped count %d exceeds per-layer raw cap %d (unexpected)", layer, c, maxClearanceViolations)
		}
	}
}

func TestTombstoningDeterministic(t *testing.T) {
	// 600 small-passive pairs forces the 500 cap to bind.
	board := passivePairBoard(600)
	profile := ProfileRules{}
	rule := &TombstoningRiskRule{}

	first := rule.Run(board, profile)
	if len(first) == 0 {
		t.Fatal("expected tombstoning violations, got none")
	}
	for i := 0; i < 9; i++ {
		next := rule.Run(board, profile)
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("tombstoning non-deterministic on run %d: len first=%d next=%d", i+2, len(first), len(next))
		}
	}
}

func TestDedupDeterministicOrder(t *testing.T) {
	// Two cells per layer × two layers × two rules — input shuffled.
	in := []Violation{
		{RuleID: "clearance", Layer: "L02", X: 10, Y: 5, MeasuredMM: 0.1},
		{RuleID: "clearance", Layer: "L01", X: 5, Y: 5, MeasuredMM: 0.1},
		{RuleID: "drill-to-copper", Layer: "L01", X: 5, Y: 5, MeasuredMM: 0.05},
		{RuleID: "clearance", Layer: "L01", X: 20, Y: 5, MeasuredMM: 0.1},
	}
	first := dedupeViolations(in, 2.0)
	want := []struct{ layer, rule string }{
		{"L01", "clearance"},
		{"L01", "clearance"},
		{"L01", "drill-to-copper"},
		{"L02", "clearance"},
	}
	if len(first) != len(want) {
		t.Fatalf("expected %d deduped violations, got %d", len(want), len(first))
	}
	for i, w := range want {
		if first[i].Layer != w.layer || first[i].RuleID != w.rule {
			t.Errorf("position %d: want (%s,%s), got (%s,%s)", i, w.layer, w.rule, first[i].Layer, first[i].RuleID)
		}
	}
	// X-tiebreaker: L01/clearance entries should come back ordered by X (5 then 20).
	if first[0].X != 5 || first[1].X != 20 {
		t.Errorf("X tiebreaker broken: got X=%v then %v", first[0].X, first[1].X)
	}
	// Re-run to confirm stability
	for i := 0; i < 9; i++ {
		next := dedupeViolations(in, 2.0)
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("dedup output non-deterministic on run %d", i+2)
		}
	}
}
