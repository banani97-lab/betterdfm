package dfmengine

import "math"

// FreeSpaceGrid is a coarse per-side occupancy bitmap of a board, used by
// component-scoped fix hints to suggest a target position near a
// violation. A cell is "occupied" if any feature on that side (pads,
// traces, polygons) or any through-feature (drills, vias) intersects it,
// with a small clearance margin. Cells outside the board outline (and
// inside any outline holes) are also occupied so NearestFree never
// returns an off-board location.
//
// Build cost is O(features × cells-per-feature-bbox). The grid is
// intended to be built lazily per side, the first time a rule needs it,
// and queried via NearestFree.
type FreeSpaceGrid struct {
	cellMM     float64
	minX, minY float64
	cols, rows int
	occupied   []bool
}

// freeSpaceCellMM is the default raster resolution for free-space queries.
// 0.5 mm balances precision against build cost: a 100x100 mm board has
// 40k cells per side, which builds in well under 10 ms in practice.
const freeSpaceCellMM = 0.5

// freeSpaceFeatureMarginMM is the clearance ring around each feature
// when rasterizing. Keeps cells immediately abutting a pad or trace
// classified as occupied so a suggested target doesn't land flush against
// existing copper.
const freeSpaceFeatureMarginMM = 0.2

// buildFreeSpaceGrid returns a free-space grid for the given side
// ("top" or "bot"). Through-features (drills, vias) occupy both sides.
// When layer metadata is missing, the side filter degrades to a wildcard
// — all pads/traces/polygons mark occupancy, which is conservative.
//
// Returns nil when the board outline is too sparse (<3 points) to define
// a rasterizable area.
func buildFreeSpaceGrid(board BoardData, side string, cellMM float64) *FreeSpaceGrid {
	if cellMM <= 0 {
		cellMM = freeSpaceCellMM
	}
	if len(board.Outline) < 3 {
		return nil
	}

	bbox := newBoardBBox(board.Outline, 0)
	if !bbox.valid {
		return nil
	}
	cols := int(math.Ceil((bbox.maxX-bbox.minX)/cellMM)) + 1
	rows := int(math.Ceil((bbox.maxY-bbox.minY)/cellMM)) + 1
	if cols <= 0 || rows <= 0 {
		return nil
	}

	g := &FreeSpaceGrid{
		cellMM:   cellMM,
		minX:     bbox.minX,
		minY:     bbox.minY,
		cols:     cols,
		rows:     rows,
		occupied: make([]bool, cols*rows),
	}

	// Mark every cell whose center is outside the outline (or inside a
	// cutout) as occupied. This is the board-edge keepout.
	for r := 0; r < rows; r++ {
		cy := bbox.minY + (float64(r)+0.5)*cellMM
		for c := 0; c < cols; c++ {
			cx := bbox.minX + (float64(c)+0.5)*cellMM
			if !pointInPolygon(cx, cy, board.Outline) {
				g.set(c, r)
				continue
			}
			for _, hole := range board.OutlineHoles {
				if pointInPolygon(cx, cy, hole) {
					g.set(c, r)
					break
				}
			}
		}
	}

	// Pick the outer copper layer name for this side, if known.
	sideLayer := ""
	var first, last string
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			if first == "" {
				first = l.Name
			}
			last = l.Name
		}
	}
	switch side {
	case "top":
		sideLayer = first
	case "bot":
		sideLayer = last
	}
	// If we can't identify the side's layer name (no stack metadata),
	// treat every layer as relevant — conservative.
	sideMatches := func(layerName string) bool {
		if sideLayer == "" {
			return true
		}
		return layerName == sideLayer
	}

	margin := freeSpaceFeatureMarginMM

	// Pads on this side's outer copper.
	for _, p := range board.Pads {
		if !sideMatches(p.Layer) {
			continue
		}
		halfW := p.WidthMM/2 + margin
		halfH := p.HeightMM/2 + margin
		g.markBBox(p.X-halfW, p.Y-halfH, p.X+halfW, p.Y+halfH)
	}

	// Traces on this side's outer copper. Treat as a rotated bbox
	// approximated by the axis-aligned bbox of the segment endpoints
	// expanded by half the trace width. Slightly over-occupies; fine
	// for v1 (conservative).
	for _, t := range board.Traces {
		if !sideMatches(t.Layer) {
			continue
		}
		halfW := t.WidthMM/2 + margin
		minX := math.Min(t.StartX, t.EndX) - halfW
		maxX := math.Max(t.StartX, t.EndX) + halfW
		minY := math.Min(t.StartY, t.EndY) - halfW
		maxY := math.Max(t.StartY, t.EndY) + halfW
		g.markBBox(minX, minY, maxX, maxY)
	}

	// Polygons on this side's outer copper. Mark each polygon's
	// axis-aligned bbox (cheap, conservative — true rasterization could
	// reclaim interior holes but isn't worth the complexity for v1).
	for _, poly := range board.Polygons {
		if !sideMatches(poly.Layer) {
			continue
		}
		if len(poly.Points) == 0 {
			continue
		}
		minX, minY := poly.Points[0].X, poly.Points[0].Y
		maxX, maxY := minX, minY
		for _, pt := range poly.Points[1:] {
			if pt.X < minX {
				minX = pt.X
			}
			if pt.X > maxX {
				maxX = pt.X
			}
			if pt.Y < minY {
				minY = pt.Y
			}
			if pt.Y > maxY {
				maxY = pt.Y
			}
		}
		g.markBBox(minX-margin, minY-margin, maxX+margin, maxY+margin)
	}

	// Drills and vias are through-features — they occupy both sides.
	for _, d := range board.Drills {
		r := d.DiamMM/2 + margin
		g.markBBox(d.X-r, d.Y-r, d.X+r, d.Y+r)
	}
	for _, v := range board.Vias {
		r := v.OuterDiamMM/2 + margin
		g.markBBox(v.X-r, v.Y-r, v.X+r, v.Y+r)
	}

	return g
}

// idx returns the flat slice index for cell (c, r). Returns -1 when
// out of bounds.
func (g *FreeSpaceGrid) idx(c, r int) int {
	if c < 0 || c >= g.cols || r < 0 || r >= g.rows {
		return -1
	}
	return r*g.cols + c
}

func (g *FreeSpaceGrid) set(c, r int) {
	if i := g.idx(c, r); i >= 0 {
		g.occupied[i] = true
	}
}

// isOccupied returns true for cells outside the grid as well, so the
// rectangular fit test in NearestFree treats off-grid windows as failed.
func (g *FreeSpaceGrid) isOccupied(c, r int) bool {
	i := g.idx(c, r)
	if i < 0 {
		return true
	}
	return g.occupied[i]
}

// markBBox marks every cell whose center falls in the (minX, minY,
// maxX, maxY) board-space bbox as occupied. Cheap and conservative.
func (g *FreeSpaceGrid) markBBox(minX, minY, maxX, maxY float64) {
	cLo := int(math.Floor((minX - g.minX) / g.cellMM))
	cHi := int(math.Floor((maxX - g.minX) / g.cellMM))
	rLo := int(math.Floor((minY - g.minY) / g.cellMM))
	rHi := int(math.Floor((maxY - g.minY) / g.cellMM))
	if cLo < 0 {
		cLo = 0
	}
	if rLo < 0 {
		rLo = 0
	}
	if cHi >= g.cols {
		cHi = g.cols - 1
	}
	if rHi >= g.rows {
		rHi = g.rows - 1
	}
	for r := rLo; r <= rHi; r++ {
		row := r * g.cols
		for c := cLo; c <= cHi; c++ {
			g.occupied[row+c] = true
		}
	}
}

// windowFree returns true when the rectangular window of cells centered
// on (c, r) covering ±halfCols × ±halfRows is entirely free.
func (g *FreeSpaceGrid) windowFree(c, r, halfCols, halfRows int) bool {
	for dr := -halfRows; dr <= halfRows; dr++ {
		for dc := -halfCols; dc <= halfCols; dc++ {
			if g.isOccupied(c+dc, r+dr) {
				return false
			}
		}
	}
	return true
}

// NearestFree searches outward from (cx, cy) for the nearest cell whose
// surrounding ±halfWMM × ±halfHMM window is entirely free. Search is
// capped at maxSearchMM. Returns ok=false if no fit is found in budget.
//
// Intended for component-scoped fix hints: callers pass the component's
// bbox half-extents as halfW/halfH so the suggested target has room for
// the part. The result is a guidance position, not a routing-validated
// placement.
func (g *FreeSpaceGrid) NearestFree(cx, cy, halfWMM, halfHMM, maxSearchMM float64) (float64, float64, bool) {
	if g == nil {
		return 0, 0, false
	}
	c0 := int(math.Floor((cx - g.minX) / g.cellMM))
	r0 := int(math.Floor((cy - g.minY) / g.cellMM))
	halfCols := int(math.Ceil(halfWMM / g.cellMM))
	halfRows := int(math.Ceil(halfHMM / g.cellMM))
	maxRings := int(math.Ceil(maxSearchMM / g.cellMM))

	for ring := 0; ring <= maxRings; ring++ {
		if ring == 0 {
			if g.windowFree(c0, r0, halfCols, halfRows) {
				x, y := g.cellCenter(c0, r0)
				return x, y, true
			}
			continue
		}
		// Walk the perimeter of the ring at Chebyshev distance == ring.
		for dr := -ring; dr <= ring; dr++ {
			for dc := -ring; dc <= ring; dc++ {
				if abs(dc) != ring && abs(dr) != ring {
					continue // interior of ring already covered by smaller rings
				}
				c := c0 + dc
				r := r0 + dr
				if g.windowFree(c, r, halfCols, halfRows) {
					x, y := g.cellCenter(c, r)
					return x, y, true
				}
			}
		}
	}
	return 0, 0, false
}

// cellCenter returns the board-space center of cell (c, r).
func (g *FreeSpaceGrid) cellCenter(c, r int) (float64, float64) {
	return g.minX + (float64(c)+0.5)*g.cellMM, g.minY + (float64(r)+0.5)*g.cellMM
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
