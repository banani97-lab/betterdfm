package dfmengine

import (
	"math"
	"testing"
)

// rectOutline is provided by fixture_test.go in this package.

func TestBuildFreeSpaceGrid_EmptyBoard_AllCellsFreeInside(t *testing.T) {
	board := BoardData{Outline: rectOutline(20, 20)}
	g := buildFreeSpaceGrid(board, "top", 0.5)
	if g == nil {
		t.Fatal("grid is nil")
	}
	// A point near the center of an empty rectangular board should be free.
	x, y, ok := g.NearestFree(10, 10, 1, 1, 2)
	if !ok {
		t.Fatal("expected to find a free cell at the board center")
	}
	if math.Abs(x-10) > 1 || math.Abs(y-10) > 1 {
		t.Errorf("free cell drifted from center: got (%v, %v), want near (10, 10)", x, y)
	}
}

func TestBuildFreeSpaceGrid_OffBoardOccupied(t *testing.T) {
	board := BoardData{Outline: rectOutline(20, 20)}
	g := buildFreeSpaceGrid(board, "top", 0.5)
	if g == nil {
		t.Fatal("grid is nil")
	}
	// (-5, -5) is well outside the outline — must not be picked.
	_, _, ok := g.NearestFree(-5, -5, 0.5, 0.5, 1)
	if ok {
		t.Error("NearestFree returned a hit for an off-board query with a tiny search budget")
	}
}

func TestBuildFreeSpaceGrid_PadOccupiesAdjacentCells(t *testing.T) {
	board := BoardData{
		Outline: rectOutline(20, 20),
		Layers:  []Layer{{Name: "TOP", Type: "COPPER"}, {Name: "BOT", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "TOP", X: 10, Y: 10, WidthMM: 2, HeightMM: 2, Shape: "RECT"},
		},
	}
	g := buildFreeSpaceGrid(board, "top", 0.5)
	if g == nil {
		t.Fatal("grid is nil")
	}
	// A 1mm window centered on the pad cannot fit — the pad + margin
	// blocks the cells around (10, 10).
	_, _, ok := g.NearestFree(10, 10, 0.5, 0.5, 0)
	if ok {
		t.Error("NearestFree returned a hit on top of an occupied pad with zero search budget")
	}
	// A larger search budget finds a nearby free cell beyond the pad
	// (pad half-width + margin ~= 1.2 mm, so something past that fits).
	x, y, ok := g.NearestFree(10, 10, 0.5, 0.5, 5)
	if !ok {
		t.Fatal("NearestFree failed to find a free cell within 5 mm of the pad")
	}
	dist := math.Hypot(x-10, y-10)
	if dist < 1 {
		t.Errorf("free cell suspiciously close to pad center: dist=%v", dist)
	}
}

func TestBuildFreeSpaceGrid_PadOnOtherSide_DoesNotBlock(t *testing.T) {
	// Top-side pad shouldn't occupy the bot-side grid.
	board := BoardData{
		Outline: rectOutline(20, 20),
		Layers:  []Layer{{Name: "TOP", Type: "COPPER"}, {Name: "BOT", Type: "COPPER"}},
		Pads: []Pad{
			{Layer: "TOP", X: 10, Y: 10, WidthMM: 2, HeightMM: 2, Shape: "RECT"},
		},
	}
	bot := buildFreeSpaceGrid(board, "bot", 0.5)
	if bot == nil {
		t.Fatal("bot grid is nil")
	}
	x, y, ok := bot.NearestFree(10, 10, 0.5, 0.5, 0)
	if !ok {
		t.Error("bot-side query at (10,10) should succeed — top-side pad does not occupy bot")
	}
	if math.Abs(x-10) > 1 || math.Abs(y-10) > 1 {
		t.Errorf("bot-side free cell drifted: got (%v, %v)", x, y)
	}
}

func TestBuildFreeSpaceGrid_DrillBlocksBothSides(t *testing.T) {
	// A through-drill should occupy the top AND bot grid.
	board := BoardData{
		Outline: rectOutline(20, 20),
		Layers:  []Layer{{Name: "TOP", Type: "COPPER"}, {Name: "BOT", Type: "COPPER"}},
		Drills:  []Drill{{X: 10, Y: 10, DiamMM: 1.0}},
	}
	for _, side := range []string{"top", "bot"} {
		g := buildFreeSpaceGrid(board, side, 0.5)
		if g == nil {
			t.Fatalf("%s grid is nil", side)
		}
		_, _, ok := g.NearestFree(10, 10, 0.5, 0.5, 0)
		if ok {
			t.Errorf("%s: through-drill at (10,10) should block the cell, but query succeeded", side)
		}
	}
}

func TestBuildFreeSpaceGrid_SaturatedBoard_FailsGracefully(t *testing.T) {
	// A board fully covered by one giant pad has no free cells.
	board := BoardData{
		Outline: rectOutline(10, 10),
		Layers:  []Layer{{Name: "TOP", Type: "COPPER"}},
		Pads:    []Pad{{Layer: "TOP", X: 5, Y: 5, WidthMM: 20, HeightMM: 20, Shape: "RECT"}},
	}
	g := buildFreeSpaceGrid(board, "top", 0.5)
	if g == nil {
		t.Fatal("grid is nil")
	}
	_, _, ok := g.NearestFree(5, 5, 0.5, 0.5, 20)
	if ok {
		t.Error("NearestFree should have returned ok=false on a saturated board")
	}
}

func TestBuildFreeSpaceGrid_DegenerateBoard_ReturnsNil(t *testing.T) {
	// Outline with <3 points is not rasterizable.
	if g := buildFreeSpaceGrid(BoardData{Outline: []Point{{X: 0, Y: 0}, {X: 10, Y: 0}}}, "top", 0.5); g != nil {
		t.Error("degenerate outline should return nil grid")
	}
}
