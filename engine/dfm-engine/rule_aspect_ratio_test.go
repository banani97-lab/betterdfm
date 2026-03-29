package dfmengine

import "testing"

func TestAspectRatio_Exceeds(t *testing.T) {
	rule := &AspectRatioRule{}
	// ratio = 1.6/0.2 = 8:1, max = 6:1 → violation
	board := drillBoard([]float64{0.2})
	board.BoardThicknessMM = 1.6
	profile := ProfileRules{MaxAspectRatio: 6.0}
	viols := rule.Run(board, profile)
	if len(viols) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(viols))
	}
	if viols[0].Severity != "ERROR" {
		t.Errorf("expected ERROR severity, got %s", viols[0].Severity)
	}
}

func TestAspectRatio_Passes(t *testing.T) {
	rule := &AspectRatioRule{}
	// ratio = 1.6/0.4 = 4:1, max = 6:1 → no violation
	board := drillBoard([]float64{0.4})
	board.BoardThicknessMM = 1.6
	profile := ProfileRules{MaxAspectRatio: 6.0}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(viols))
	}
}

func TestAspectRatio_ZeroBoardThicknessSkipped(t *testing.T) {
	rule := &AspectRatioRule{}
	board := drillBoard([]float64{0.2})
	board.BoardThicknessMM = 0
	profile := ProfileRules{MaxAspectRatio: 6.0}
	viols := rule.Run(board, profile)
	if len(viols) != 0 {
		t.Fatalf("zero board thickness should be skipped, got %d violations", len(viols))
	}
}
