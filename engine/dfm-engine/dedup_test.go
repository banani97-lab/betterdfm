package dfmengine

import "testing"

func TestDedup_SameCellCollapsed(t *testing.T) {
	viols := []Violation{
		{RuleID: "trace-width", Layer: "top_copper", X: 0.5, Y: 0.5, MeasuredMM: 0.08},
		{RuleID: "trace-width", Layer: "top_copper", X: 0.6, Y: 0.6, MeasuredMM: 0.07},
	}
	result := dedupeViolations(viols, 2.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 deduplicated violation, got %d", len(result))
	}
	if result[0].Count != 2 {
		t.Errorf("expected Count=2, got %d", result[0].Count)
	}
}

func TestDedup_WorstMeasuredKept(t *testing.T) {
	viols := []Violation{
		{RuleID: "clearance", Layer: "top_copper", X: 0.5, Y: 0.5, MeasuredMM: 0.09},
		{RuleID: "clearance", Layer: "top_copper", X: 0.6, Y: 0.6, MeasuredMM: 0.05},
	}
	result := dedupeViolations(viols, 2.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 deduplicated violation, got %d", len(result))
	}
	if result[0].MeasuredMM != 0.05 {
		t.Errorf("expected worst MeasuredMM=0.05, got %f", result[0].MeasuredMM)
	}
}

func TestDedup_DifferentLayerNotMerged(t *testing.T) {
	viols := []Violation{
		{RuleID: "trace-width", Layer: "top_copper", X: 0.5, Y: 0.5, MeasuredMM: 0.08},
		{RuleID: "trace-width", Layer: "bot_copper", X: 0.5, Y: 0.5, MeasuredMM: 0.08},
	}
	result := dedupeViolations(viols, 2.0)
	if len(result) != 2 {
		t.Fatalf("different layers should not be merged, got %d violations", len(result))
	}
}

func TestDedup_EmptyInput(t *testing.T) {
	result := dedupeViolations(nil, 2.0)
	if len(result) != 0 {
		t.Fatalf("expected empty output for nil input, got %d", len(result))
	}
}
