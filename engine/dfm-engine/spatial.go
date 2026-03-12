package dfmengine

import "math"

// outlineIndex is a spatial grid that accelerates point-to-outline distance
// queries. Pre-building the index is O(n_segments) and each query is O(k)
// where k is the number of segments in the neighbourhood — typically << n.
type outlineIndex struct {
	segments [][4]float64 // [ax, ay, bx, by]
	cells    map[[2]int][]int
	cellMM   float64
}

func newOutlineIndex(outline []Point, cellMM float64) *outlineIndex {
	if cellMM <= 0 {
		cellMM = 2.0
	}
	idx := &outlineIndex{
		cells:  make(map[[2]int][]int),
		cellMM: cellMM,
	}
	n := len(outline)
	for i := 0; i < n; i++ {
		a := outline[i]
		b := outline[(i+1)%n]
		segIdx := len(idx.segments)
		idx.segments = append(idx.segments, [4]float64{a.X, a.Y, b.X, b.Y})
		// Bucket segment into every grid cell it overlaps.
		minX := math.Min(a.X, b.X)
		maxX := math.Max(a.X, b.X)
		minY := math.Min(a.Y, b.Y)
		maxY := math.Max(a.Y, b.Y)
		cxMin := int(math.Floor(minX / cellMM))
		cxMax := int(math.Floor(maxX / cellMM))
		cyMin := int(math.Floor(minY / cellMM))
		cyMax := int(math.Floor(maxY / cellMM))
		for cx := cxMin; cx <= cxMax; cx++ {
			for cy := cyMin; cy <= cyMax; cy++ {
				key := [2]int{cx, cy}
				idx.cells[key] = append(idx.cells[key], segIdx)
			}
		}
	}
	return idx
}

// minDist returns the minimum distance from point (px, py) to any outline segment.
func (idx *outlineIndex) minDist(px, py float64) float64 {
	cx := int(math.Floor(px / idx.cellMM))
	cy := int(math.Floor(py / idx.cellMM))

	// Collect candidate segment indices from this cell and its 1-cell margin.
	seen := make(map[int]bool)
	var candidates []int
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			key := [2]int{cx + dx, cy + dy}
			for _, si := range idx.cells[key] {
				if !seen[si] {
					seen[si] = true
					candidates = append(candidates, si)
				}
			}
		}
	}

	// Fall back to full scan if no candidates found (degenerate outlines).
	if len(candidates) == 0 {
		candidates = make([]int, len(idx.segments))
		for i := range idx.segments {
			candidates[i] = i
		}
	}

	minD := math.MaxFloat64
	for _, si := range candidates {
		s := idx.segments[si]
		d := ptToSegDist(px, py, s[0], s[1], s[2], s[3])
		if d < minD {
			minD = d
		}
	}
	return minD
}
