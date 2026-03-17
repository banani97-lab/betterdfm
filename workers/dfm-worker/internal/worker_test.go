package internal

import (
	"testing"

	dfmengine "github.com/betterdfm/dfm-engine"
)

func TestSanitizeBoard_DropsZeroWidthTraces(t *testing.T) {
	board := dfmengine.BoardData{
		Traces: []dfmengine.Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 0, StartY: 0, EndX: 10, EndY: 0},
			{Layer: "top_copper", WidthMM: 0, StartX: 0, StartY: 1, EndX: 10, EndY: 1},
			{Layer: "top_copper", WidthMM: 0.2, StartX: 0, StartY: 2, EndX: 10, EndY: 2},
		},
	}
	out := sanitizeBoard(board, "test-job")
	if len(out.Traces) != 2 {
		t.Fatalf("expected 2 valid traces, got %d", len(out.Traces))
	}
	for _, tr := range out.Traces {
		if tr.WidthMM <= 0 {
			t.Errorf("zero-width trace leaked through sanitize: %+v", tr)
		}
	}
}

func TestSanitizeBoard_DropsZeroSizePads(t *testing.T) {
	board := dfmengine.BoardData{
		Pads: []dfmengine.Pad{
			{Layer: "top_copper", X: 1, Y: 1, WidthMM: 0.5, HeightMM: 0.5, Shape: "CIRCLE"},
			{Layer: "top_copper", X: 2, Y: 1, WidthMM: 0, HeightMM: 0.5, Shape: "CIRCLE"},
			{Layer: "top_copper", X: 3, Y: 1, WidthMM: 0.5, HeightMM: 0, Shape: "CIRCLE"},
			{Layer: "top_copper", X: 4, Y: 1, WidthMM: 0, HeightMM: 0, Shape: "CIRCLE"},
		},
	}
	out := sanitizeBoard(board, "test-job")
	if len(out.Pads) != 1 {
		t.Fatalf("expected 1 valid pad, got %d", len(out.Pads))
	}
	if out.Pads[0].X != 1 {
		t.Errorf("wrong pad survived: %+v", out.Pads[0])
	}
}

func TestSanitizeBoard_DropsZeroDiamDrills(t *testing.T) {
	board := dfmengine.BoardData{
		Drills: []dfmengine.Drill{
			{X: 5, Y: 5, DiamMM: 0.3, Plated: true},
			{X: 6, Y: 5, DiamMM: 0, Plated: true},
		},
	}
	out := sanitizeBoard(board, "test-job")
	if len(out.Drills) != 1 {
		t.Fatalf("expected 1 valid drill, got %d", len(out.Drills))
	}
	if out.Drills[0].DiamMM <= 0 {
		t.Error("zero-diameter drill leaked through")
	}
}

func TestSanitizeBoard_PreservesValidGeometry(t *testing.T) {
	board := dfmengine.BoardData{
		Traces: []dfmengine.Trace{
			{Layer: "top_copper", WidthMM: 0.15, StartX: 1, StartY: 1, EndX: 20, EndY: 1},
		},
		Pads: []dfmengine.Pad{
			{Layer: "top_copper", X: 5, Y: 5, WidthMM: 0.8, HeightMM: 0.8, Shape: "CIRCLE"},
		},
		Drills: []dfmengine.Drill{
			{X: 10, Y: 10, DiamMM: 0.3, Plated: true},
		},
		Outline: []dfmengine.Point{{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 30}, {X: 0, Y: 30}},
	}
	out := sanitizeBoard(board, "test-job")
	if len(out.Traces) != 1 || len(out.Pads) != 1 || len(out.Drills) != 1 {
		t.Errorf("valid geometry should not be dropped: traces=%d pads=%d drills=%d",
			len(out.Traces), len(out.Pads), len(out.Drills))
	}
}

func TestSanitizeBoard_ShortOutlineWarned(t *testing.T) {
	// Under 3 outline points — should NOT panic, just log; outline remains
	board := dfmengine.BoardData{
		Outline: []dfmengine.Point{{X: 0, Y: 0}, {X: 10, Y: 0}},
	}
	// Should not panic
	out := sanitizeBoard(board, "test-job")
	// Outline is preserved as-is (sanitize only warns)
	if len(out.Outline) != 2 {
		t.Errorf("outline should be preserved even if short; got %d pts", len(out.Outline))
	}
}

func TestSanitizeBoard_EmptyBoardNoOp(t *testing.T) {
	out := sanitizeBoard(dfmengine.BoardData{}, "empty-job")
	if len(out.Traces) != 0 || len(out.Pads) != 0 || len(out.Drills) != 0 {
		t.Error("empty board should remain empty after sanitize")
	}
}
