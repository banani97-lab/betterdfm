package dfmengine

import "math"

// indexedPolygon is a copper pour prepared for fast geometric queries:
// containment (is a point inside the filled region, holes subtracted) and
// proximity (nearest boundary edge within a search radius). Built once per
// polygon per rule run; the edge grid uses the same gridKey pattern as the
// trace sweep in rule_clearance.go so queries are O(cell), not O(vertices).
type indexedPolygon struct {
	poly                   *Polygon
	minX, maxX, minY, maxY float64
	cell                   float64
	edges                  map[[2]int][]ipEdge
	holes                  []ipHoleBB
}

type ipEdge struct{ ax, ay, bx, by float64 }

type ipHoleBB struct {
	minX, maxX, minY, maxY float64
	ring                   []Point
}

// newIndexedPolygon builds the index. cellMM controls grid granularity; pass
// something on the order of the largest query radius (e.g. 2*minClearance,
// floored at 0.5 mm) so a proximity query touches only a few cells.
func newIndexedPolygon(poly *Polygon, cellMM float64) *indexedPolygon {
	if len(poly.Points) < 3 {
		return nil
	}
	if cellMM < 0.5 {
		cellMM = 0.5
	}
	ip := &indexedPolygon{
		poly:  poly,
		cell:  cellMM,
		edges: map[[2]int][]ipEdge{},
		minX:  poly.Points[0].X, maxX: poly.Points[0].X,
		minY: poly.Points[0].Y, maxY: poly.Points[0].Y,
	}
	for _, p := range poly.Points[1:] {
		ip.minX = math.Min(ip.minX, p.X)
		ip.maxX = math.Max(ip.maxX, p.X)
		ip.minY = math.Min(ip.minY, p.Y)
		ip.maxY = math.Max(ip.maxY, p.Y)
	}
	ip.addRingEdges(poly.Points)
	for _, hole := range poly.Holes {
		if len(hole) < 3 {
			continue
		}
		hb := ipHoleBB{
			minX: hole[0].X, maxX: hole[0].X,
			minY: hole[0].Y, maxY: hole[0].Y,
			ring: hole,
		}
		for _, p := range hole[1:] {
			hb.minX = math.Min(hb.minX, p.X)
			hb.maxX = math.Max(hb.maxX, p.X)
			hb.minY = math.Min(hb.minY, p.Y)
			hb.maxY = math.Max(hb.maxY, p.Y)
		}
		ip.holes = append(ip.holes, hb)
		ip.addRingEdges(hole)
	}
	return ip
}

func (ip *indexedPolygon) addRingEdges(ring []Point) {
	n := len(ring)
	for i := 0; i < n; i++ {
		a := ring[i]
		b := ring[(i+1)%n]
		e := ipEdge{a.X, a.Y, b.X, b.Y}
		cxMin := int(math.Floor(math.Min(a.X, b.X) / ip.cell))
		cxMax := int(math.Floor(math.Max(a.X, b.X) / ip.cell))
		cyMin := int(math.Floor(math.Min(a.Y, b.Y) / ip.cell))
		cyMax := int(math.Floor(math.Max(a.Y, b.Y) / ip.cell))
		for cx := cxMin; cx <= cxMax; cx++ {
			for cy := cyMin; cy <= cyMax; cy++ {
				ip.edges[[2]int{cx, cy}] = append(ip.edges[[2]int{cx, cy}], e)
			}
		}
	}
}

// contains reports whether (x, y) lies inside the filled copper region:
// inside the outer ring and outside every hole. Hole lookup is accelerated
// by per-hole bboxes — anti-pads are small, so almost all are rejected
// without a ray cast.
func (ip *indexedPolygon) contains(x, y float64) bool {
	if x < ip.minX || x > ip.maxX || y < ip.minY || y > ip.maxY {
		return false
	}
	if !pointInPolygon(x, y, ip.poly.Points) {
		return false
	}
	for i := range ip.holes {
		h := &ip.holes[i]
		if x < h.minX || x > h.maxX || y < h.minY || y > h.maxY {
			continue
		}
		if pointInPolygon(x, y, h.ring) {
			return false
		}
	}
	return true
}

// nearestEdgeWithin returns the distance to and location of the nearest
// boundary edge point within radius r of (x, y). ok is false when no edge
// is that close.
func (ip *indexedPolygon) nearestEdgeWithin(x, y, r float64) (dist, nx, ny float64, ok bool) {
	if x < ip.minX-r || x > ip.maxX+r || y < ip.minY-r || y > ip.maxY+r {
		return 0, 0, 0, false
	}
	best := r * r
	cxMin := int(math.Floor((x - r) / ip.cell))
	cxMax := int(math.Floor((x + r) / ip.cell))
	cyMin := int(math.Floor((y - r) / ip.cell))
	cyMax := int(math.Floor((y + r) / ip.cell))
	for cx := cxMin; cx <= cxMax; cx++ {
		for cy := cyMin; cy <= cyMax; cy++ {
			for _, e := range ip.edges[[2]int{cx, cy}] {
				px, py := closestPointOnSeg(x, y, e.ax, e.ay, e.bx, e.by)
				d2 := (x-px)*(x-px) + (y-py)*(y-py)
				if d2 <= best {
					best = d2
					nx, ny = px, py
					ok = true
				}
			}
		}
	}
	if !ok {
		return 0, 0, 0, false
	}
	return math.Sqrt(best), nx, ny, true
}

// segDistWithin returns the minimum distance from segment (x1,y1)-(x2,y2) to
// any boundary edge within radius r of the segment. ok is false when no edge
// is that close.
func (ip *indexedPolygon) segDistWithin(x1, y1, x2, y2, r float64) (float64, bool) {
	sMinX, sMaxX := math.Min(x1, x2), math.Max(x1, x2)
	sMinY, sMaxY := math.Min(y1, y2), math.Max(y1, y2)
	if sMaxX < ip.minX-r || sMinX > ip.maxX+r || sMaxY < ip.minY-r || sMinY > ip.maxY+r {
		return 0, false
	}
	best := math.MaxFloat64
	ok := false
	cxMin := int(math.Floor((sMinX - r) / ip.cell))
	cxMax := int(math.Floor((sMaxX + r) / ip.cell))
	cyMin := int(math.Floor((sMinY - r) / ip.cell))
	cyMax := int(math.Floor((sMaxY + r) / ip.cell))
	for cx := cxMin; cx <= cxMax; cx++ {
		for cy := cyMin; cy <= cyMax; cy++ {
			for _, e := range ip.edges[[2]int{cx, cy}] {
				d := segToSegDist(x1, y1, x2, y2, e.ax, e.ay, e.bx, e.by)
				if d < best {
					best = d
					ok = true
				}
			}
		}
	}
	if !ok || best > r {
		return 0, false
	}
	return best, true
}

// segClosestPoints returns the closest pair of points between non-intersecting
// segments AB and CD (one point on each). For intersecting segments it returns
// one of the endpoint projections, which callers guard against by checking
// distance > 0 first.
func segClosestPoints(ax, ay, bx, by, cx, cy, dx, dy float64) (px, py, qx, qy float64) {
	best := math.MaxFloat64
	try := func(sx, sy, tx, ty float64) {
		d := (sx-tx)*(sx-tx) + (sy-ty)*(sy-ty)
		if d < best {
			best = d
			px, py, qx, qy = sx, sy, tx, ty
		}
	}
	x, y := closestPointOnSeg(ax, ay, cx, cy, dx, dy)
	try(ax, ay, x, y)
	x, y = closestPointOnSeg(bx, by, cx, cy, dx, dy)
	try(bx, by, x, y)
	x, y = closestPointOnSeg(cx, cy, ax, ay, bx, by)
	try(x, y, cx, cy)
	x, y = closestPointOnSeg(dx, dy, ax, ay, bx, by)
	try(x, y, dx, dy)
	return
}

// polygonContainsWithHoles reports whether (x, y) is inside poly's outer ring
// and outside all of its holes. Un-indexed convenience form for single-shot
// queries; rules doing repeated queries should use newIndexedPolygon.
func polygonContainsWithHoles(x, y float64, poly Polygon) bool {
	if len(poly.Points) < 3 || !pointInPolygon(x, y, poly.Points) {
		return false
	}
	for _, hole := range poly.Holes {
		if len(hole) >= 3 && pointInPolygon(x, y, hole) {
			return false
		}
	}
	return true
}
