package dfmengine

import "testing"

func TestTraceWidth_BelowMin(t *testing.T) {
	rule := &TraceWidthRule{}
	board := singleTraceBoard(0.08)
	profile := ProfileRules{MinTraceWidthMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	if viols[0].Severity != "ERROR" {
		t.Errorf("expected ERROR severity, got %s", viols[0].Severity)
	}
	if viols[0].MeasuredMM != 0.08 {
		t.Errorf("expected MeasuredMM=0.08, got %f", viols[0].MeasuredMM)
	}
	if viols[0].LimitMM != 0.1 {
		t.Errorf("expected LimitMM=0.1, got %f", viols[0].LimitMM)
	}
}

func TestTraceWidth_AtMin(t *testing.T) {
	rule := &TraceWidthRule{}
	board := singleTraceBoard(0.1)
	profile := ProfileRules{MinTraceWidthMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations at min, got %d", len(viols))
	}
}

func TestTraceWidth_AboveMin(t *testing.T) {
	rule := &TraceWidthRule{}
	board := singleTraceBoard(0.2)
	profile := ProfileRules{MinTraceWidthMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations above min, got %d", len(viols))
	}
}

func TestTraceWidth_NonCopperLayerSkipped(t *testing.T) {
	rule := &TraceWidthRule{}
	board := BoardData{
		Layers: []Layer{{Name: "silk_top", Type: "SILK"}},
		Traces: []Trace{
			{Layer: "silk_top", WidthMM: 0.05, StartX: 0, StartY: 0, EndX: 10, EndY: 0},
		},
	}
	profile := ProfileRules{MinTraceWidthMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("silk layer traces should be skipped, got %d violations", len(viols))
	}
}

func TestTraceWidth_MaxViolationCap(t *testing.T) {
	rule := &TraceWidthRule{}
	traces := make([]Trace, 600)
	for i := range traces {
		traces[i] = Trace{Layer: "top_copper", WidthMM: 0.05, StartX: float64(i), StartY: 0, EndX: float64(i) + 1, EndY: 0}
	}
	board := BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: traces,
	}
	profile := ProfileRules{MinTraceWidthMM: 0.1}
	viols := rule.Run(board, profile)
	if len(viols) > 500 {
		t.Errorf("expected at most 500 violations, got %d", len(viols))
	}
}
