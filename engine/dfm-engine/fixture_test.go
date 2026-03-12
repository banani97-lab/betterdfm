package dfmengine

// Shared synthetic BoardData builders for engine unit tests.

func rectOutline(w, h float64) []Point {
	return []Point{
		{X: 0, Y: 0},
		{X: w, Y: 0},
		{X: w, Y: h},
		{X: 0, Y: h},
	}
}

// twoTraceBoard returns a board with two parallel horizontal traces separated by gapMM.
// Both traces are on "top_copper" with width 0.1mm.
func twoTraceBoard(w1, w2, gapMM float64) BoardData {
	hw1, hw2 := w1/2, w2/2
	// Trace 1: y=10, Trace 2: y=10+hw1+gapMM+hw2
	y1 := 10.0
	y2 := y1 + hw1 + gapMM + hw2
	return BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: w1, StartX: 5, StartY: y1, EndX: 55, EndY: y1},
			{Layer: "top_copper", WidthMM: w2, StartX: 5, StartY: y2, EndX: 55, EndY: y2},
		},
		Outline: rectOutline(60, 40),
	}
}

// singleTraceBoard returns a board with one trace of the given width on top_copper.
func singleTraceBoard(widthMM float64) BoardData {
	return BoardData{
		Layers:  []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces:  []Trace{{Layer: "top_copper", WidthMM: widthMM, StartX: 5, StartY: 20, EndX: 55, EndY: 20}},
		Outline: rectOutline(60, 40),
	}
}

// viaBoard returns a board with a single via.
func viaBoard(outerDiam, drillDiam float64) BoardData {
	return BoardData{
		Layers:  []Layer{{Name: "top_copper", Type: "COPPER"}},
		Vias:    []Via{{X: 30, Y: 20, OuterDiamMM: outerDiam, DrillDiamMM: drillDiam}},
		Outline: rectOutline(60, 40),
	}
}

// drillBoard returns a board with multiple drills at x=i*5, y=0.
func drillBoard(diams []float64) BoardData {
	drills := make([]Drill, len(diams))
	for i, d := range diams {
		drills[i] = Drill{X: float64(i) * 5, Y: 0, DiamMM: d, Plated: true}
	}
	return BoardData{
		Layers:  []Layer{{Name: "top_copper", Type: "COPPER"}},
		Drills:  drills,
		Outline: rectOutline(60, 40),
	}
}

// edgeTraceBoard returns a board with a trace endpoint at distFromEdgeMM from the right edge.
// Board is 60×40mm so right edge is at x=60; trace endpoint is at x=60-distFromEdgeMM.
func edgeTraceBoard(distFromEdgeMM float64) BoardData {
	endX := 60.0 - distFromEdgeMM
	return BoardData{
		Layers: []Layer{{Name: "top_copper", Type: "COPPER"}},
		Traces: []Trace{
			{Layer: "top_copper", WidthMM: 0.1, StartX: 10, StartY: 20, EndX: endX, EndY: 20},
		},
		Outline: rectOutline(60, 40),
	}
}

// padPairBoard returns a board with two circular pads on top_copper separated by gapMM edge-to-edge.
func padPairBoard(gapMM, padDiamMM float64) BoardData {
	r := padDiamMM / 2
	x1, x2 := 10.0, 10.0+2*r+gapMM
	return BoardData{
		Layers: []Layer{
			{Name: "top_copper", Type: "COPPER"},
			{Name: "bot_copper", Type: "COPPER"},
		},
		Pads: []Pad{
			{Layer: "top_copper", X: x1, Y: 20, WidthMM: padDiamMM, HeightMM: padDiamMM, Shape: "CIRCLE"},
			{Layer: "top_copper", X: x2, Y: 20, WidthMM: padDiamMM, HeightMM: padDiamMM, Shape: "CIRCLE"},
		},
		Outline: rectOutline(60, 40),
	}
}
