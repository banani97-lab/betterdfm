package dfmengine

import (
	"math"
	"sort"
	"strings"
)

// ClearanceRule checks trace-to-trace and trace-to-pad minimum clearances.
// At most maxClearanceViolations are reported to prevent OOM on dense boards.
const maxClearanceViolations = 500

// clearanceCellMM is the spatial grid cell size used to deduplicate violations.
// Pairs of copper features within the same cell represent the same structural
// problem (e.g. a copper pour's many segments all too close to the same pads)
// and are collapsed into a single violation showing the worst-case clearance.
const clearanceCellMM = 2.0

type ClearanceRule struct{}

func (r *ClearanceRule) ID() string { return "clearance" }

// traceBB is a trace with its precomputed bounding box.
type traceBB struct {
	t          Trace
	minX, maxX float64
	minY, maxY float64
}

func newTraceBB(t Trace) traceBB {
	hw := t.WidthMM / 2
	return traceBB{
		t:    t,
		minX: math.Min(t.StartX, t.EndX) - hw,
		maxX: math.Max(t.StartX, t.EndX) + hw,
		minY: math.Min(t.StartY, t.EndY) - hw,
		maxY: math.Max(t.StartY, t.EndY) + hw,
	}
}

func (r *ClearanceRule) Run(board BoardData, profile ProfileRules) []Violation {
	var violations []Violation
	if profile.MinClearanceMM <= 0 {
		return violations
	}
	// Compute board outline bounding box. Features more than 2 mm outside it
	// are panel-level additions (fiducials, tooling marks, test coupons) that
	// should not be DFM-checked for trace clearance.
	const outlinePanelBuffer = 2.0
	var oMinX, oMaxX, oMinY, oMaxY float64
	if len(board.Outline) > 0 {
		oMinX, oMaxX = board.Outline[0].X, board.Outline[0].X
		oMinY, oMaxY = board.Outline[0].Y, board.Outline[0].Y
		for _, op := range board.Outline[1:] {
			if op.X < oMinX {
				oMinX = op.X
			}
			if op.X > oMaxX {
				oMaxX = op.X
			}
			if op.Y < oMinY {
				oMinY = op.Y
			}
			if op.Y > oMaxY {
				oMaxY = op.Y
			}
		}
	}
	inBoard := func(x, y float64) bool {
		return len(board.Outline) == 0 ||
			(x >= oMinX-outlinePanelBuffer && x <= oMaxX+outlinePanelBuffer &&
				y >= oMinY-outlinePanelBuffer && y <= oMaxY+outlinePanelBuffer)
	}

	// Build the set of copper layer names from layer metadata.
	// Clearance is an electrical rule — only copper and power-ground layers matter.
	// Silkscreen, solder mask, drill, outline, and rout layers are excluded.
	copperLayerNames := map[string]bool{}
	for _, l := range board.Layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			copperLayerNames[l.Name] = true
		}
	}
	isCopperLayer := func(name string) bool {
		if len(copperLayerNames) > 0 {
			return copperLayerNames[name]
		}
		// Fallback when layer metadata is absent: exclude known non-copper names.
		n := strings.ToLower(name)
		return !strings.Contains(n, "silk") && !strings.Contains(n, "legend") &&
			!strings.Contains(n, "overlay") && !strings.Contains(n, "mask") &&
			!strings.Contains(n, "drill") && !strings.Contains(n, "outline") &&
			n != "rout"
	}

	// Group traces and pads by layer, excluding panel-level features and non-copper layers.
	tracesByLayer := map[string][]traceBB{}
	for _, t := range board.Traces {
		if !isCopperLayer(t.Layer) {
			continue
		}
		mx := (t.StartX + t.EndX) / 2
		my := (t.StartY + t.EndY) / 2
		if !inBoard(mx, my) {
			continue
		}
		tracesByLayer[t.Layer] = append(tracesByLayer[t.Layer], newTraceBB(t))
	}
	padsByLayer := map[string][]Pad{}
	for _, p := range board.Pads {
		if !isCopperLayer(p.Layer) {
			continue
		}
		if !inBoard(p.X, p.Y) {
			continue
		}
		padsByLayer[p.Layer] = append(padsByLayer[p.Layer], p)
	}

	// P3: group copper pours by layer for the fill-aware pour pass below.
	// (Earlier versions injected pour edges as zero-width pseudo-traces into
	// the trace sweep; the dedicated pass knows about the filled interior, so
	// it can also detect features buried inside a different net's pour.)
	polysByLayer := map[string][]*Polygon{}
	for i := range board.Polygons {
		poly := &board.Polygons[i]
		if !isCopperLayer(poly.Layer) {
			continue
		}
		// Pours without a usable net are not checkable electrical features
		// (matches the trace/pad skip policy).
		if poly.NetName == "" || poly.NetName == "$NONE$" {
			continue
		}
		polysByLayer[poly.Layer] = append(polysByLayer[poly.Layer], poly)
	}

	minC := profile.MinClearanceMM

	// $NONE$ is the ODB++ convention for "this copper has no electrical net"
	// — mounting hole rings, mechanical markers, board-level annotations.
	// These features are not part of the electrical design and should not
	// participate in electrical clearance DRC.
	isNonElectrical := func(net string) bool {
		return net == "$NONE$"
	}

	// Short detection trusts net labels in proportion to the geometric
	// evidence. Inferred labels (BFS propagation, majority vote) are never
	// enough — they routinely disagree across a junction. Positionally
	// matched netlist labels are enough only when the geometry independently
	// corroborates a defect (a proper trace crossing, a trace passing
	// through a pad). Plain overlap/containment — pad-pad, pour containment,
	// pour-pour — needs authoritative .net= attrs on both sides: on real
	// boards those overlaps are dominated by intentional structures whose
	// netlist labels legitimately disagree (net-tie joins like
	// VBACKUP/VBACKUP-CON, keypad dome fingers).
	isConfidentNet := func(source string) bool {
		return source == "attr" || source == "netlist"
	}
	isAttrNet := func(source string) bool {
		return source == "attr"
	}

	// Iterate layers in sorted order for determinism. Combined with the
	// per-layer cap below, this guarantees identical output across runs on
	// identical input — the v1↔v2 diff feature relies on it.
	// Union of trace, pad, and pour layers: the pad-to-pad and pour sweeps
	// must also cover layers without traces.
	layerSet := map[string]bool{}
	for name := range tracesByLayer {
		layerSet[name] = true
	}
	for name := range padsByLayer {
		layerSet[name] = true
	}
	for name := range polysByLayer {
		layerSet[name] = true
	}
	layerNames := make([]string, 0, len(layerSet))
	for name := range layerSet {
		layerNames = append(layerNames, name)
	}
	sort.Strings(layerNames)

	for _, layer := range layerNames {
		traces := tracesByLayer[layer]
		// Per-layer cap: each copper layer gets its own maxClearanceViolations
		// budget. The previous global cap let whichever layer iterated first
		// (random map order) consume the entire budget, starving the others.
		layerViolations := 0
		// P4.3: 2D grid hash for trace-to-trace clearance.
		// Cell size = 2*minC ensures any violating pair occupies the same or adjacent cells.
		// This eliminates the O(n·k) worst case for vertically-dense designs.
		gridCell := minC * 2
		if gridCell < 0.1 {
			gridCell = 0.1
		}
		type gridKey = [2]int
		traceGrid := make(map[gridKey][]int, len(traces))
		for i, tb := range traces {
			cxMin := int(math.Floor(tb.minX / gridCell))
			cxMax := int(math.Floor(tb.maxX / gridCell))
			cyMin := int(math.Floor(tb.minY / gridCell))
			cyMax := int(math.Floor(tb.maxY / gridCell))
			for cx := cxMin; cx <= cxMax; cx++ {
				for cy := cyMin; cy <= cyMax; cy++ {
					traceGrid[gridKey{cx, cy}] = append(traceGrid[gridKey{cx, cy}], i)
				}
			}
		}

		for i, a := range traces {
			if layerViolations >= maxClearanceViolations {
				break
			}
			// Query all cells this trace's expanded bbox overlaps.
			cxMin := int(math.Floor((a.minX - minC) / gridCell))
			cxMax := int(math.Floor((a.maxX + minC) / gridCell))
			cyMin := int(math.Floor((a.minY - minC) / gridCell))
			cyMax := int(math.Floor((a.maxY + minC) / gridCell))
			seenJ := make(map[int]bool)
			for cx := cxMin; cx <= cxMax; cx++ {
				for cy := cyMin; cy <= cyMax; cy++ {
					for _, j := range traceGrid[gridKey{cx, cy}] {
						if j <= i || seenJ[j] {
							continue // each pair checked once; skip self
						}
						seenJ[j] = true
						if layerViolations >= maxClearanceViolations {
							break
						}
						b := traces[j]
						// Same-net traces are intentionally connected — no clearance check.
						if a.t.NetName != "" && a.t.NetName == b.t.NetName {
							continue
						}
						if isNonElectrical(a.t.NetName) || isNonElectrical(b.t.NetName) {
							continue
						}
						// Skip pairs where either trace has no net. After the
						// parser's net propagation pass, any trace still without
						// a net is orphaned copper — text markings, logos, board
						// IDs, or fab drawing remnants. We can't determine whether
						// it's on the same or a different net as its neighbour,
						// so flagging it is speculation, not a real DRC finding.
						if a.t.NetName == "" || b.t.NetName == "" {
							continue
						}
						dist := segToSegDist(
							a.t.StartX, a.t.StartY, a.t.EndX, a.t.EndY,
							b.t.StartX, b.t.StartY, b.t.EndX, b.t.EndY,
						)
						clearance := dist - (a.t.WidthMM+b.t.WidthMM)/2
						if clearance <= 0 {
							// Both nets are known and different (same-net and
							// unknown-net pairs were skipped above). Only a
							// proper segment crossing is a short with high
							// confidence: endpoint-chained or T-junction
							// contact is connected routing where one side of
							// the junction carries a stale/conflicting net
							// label (netlist vs attr namespaces, propagation
							// seeds), not a fab defect.
							if !segsIntersect(
								a.t.StartX, a.t.StartY, a.t.EndX, a.t.EndY,
								b.t.StartX, b.t.StartY, b.t.EndX, b.t.EndY,
							) {
								continue
							}
							if !isConfidentNet(a.t.NetSource) || !isConfidentNet(b.t.NetSource) {
								continue
							}
							msg, sug := msgClearanceShort("trace", a.t.NetName, b.t.NetName)
							violations = append(violations, Violation{
								RuleID:     r.ID(),
								Severity:   "ERROR",
								Layer:      layer,
								X:          (a.t.StartX + a.t.EndX) / 2,
								Y:          (a.t.StartY + a.t.EndY) / 2,
								Message:    msg,
								Suggestion: sug,
								MeasuredMM: 0,
								LimitMM:    minC,
								Unit:       "mm",
								NetName:    a.t.NetName,
								X2:         (b.t.StartX + b.t.EndX) / 2,
								Y2:         (b.t.StartY + b.t.EndY) / 2,
							})
							layerViolations++
							continue
						}
						if clearance < minC-geomEps {
							msg, sug := msgClearanceTraceTooClose(clearance, minC)
							violations = append(violations, Violation{
								RuleID:     r.ID(),
								Severity:   "ERROR",
								Layer:      layer,
								X:          (a.t.StartX + a.t.EndX) / 2,
								Y:          (a.t.StartY + a.t.EndY) / 2,
								Message:    msg,
								Suggestion: sug,
								MeasuredMM: clearance,
								LimitMM:    minC,
								Unit:       "mm",
								NetName:    a.t.NetName,
								X2:         (b.t.StartX + b.t.EndX) / 2,
								Y2:         (b.t.StartY + b.t.EndY) / 2,
							})
							layerViolations++
						}
					}
				}
			}
		}

		// Trace-to-pad clearance.
		pads := padsByLayer[layer]

		// Sort pads by X so we can binary-search into the window per trace.
		sort.Slice(pads, func(i, j int) bool { return pads[i].X < pads[j].X })

		for _, tb := range traces {
			if layerViolations >= maxClearanceViolations {
				break
			}
			t := tb.t
			// Binary search: first pad whose X >= tb.minX - minC - maxPadRadius(≈1mm buffer)
			lo := sort.Search(len(pads), func(k int) bool {
				return pads[k].X >= tb.minX-minC-1.0
			})
			for k := lo; k < len(pads); k++ {
				if layerViolations >= maxClearanceViolations {
					break
				}
				p := pads[k]
				padRadius := math.Max(p.WidthMM, p.HeightMM) / 2
				if p.X > tb.maxX+minC+padRadius {
					break
				}
				// Quick Y rejection.
				if p.Y+padRadius+minC < tb.minY || p.Y-padRadius-minC > tb.maxY {
					continue
				}
				// Same-net trace and pad are intentionally connected — skip.
				if t.NetName != "" && t.NetName == p.NetName {
					continue
				}
				if isNonElectrical(t.NetName) || isNonElectrical(p.NetName) {
					continue
				}
				if t.NetName == "" || p.NetName == "" {
					continue
				}
				// P2.1: closest-point + padEdgeDist for shape-aware clearance.
				cpX, cpY := closestPointOnSeg(p.X, p.Y, t.StartX, t.StartY, t.EndX, t.EndY)
				clearance := padEdgeDist(cpX, cpY, p) - t.WidthMM/2
				if clearance <= 0 {
					// A trace endpoint terminating inside the pad is the
					// intended connection (the net labels merely disagree —
					// netlist vs attr namespaces). Only a trace passing
					// through the pad with both ends outside is a probable
					// short.
					if padEdgeDist(t.StartX, t.StartY, p) <= t.WidthMM/2+geomEps ||
						padEdgeDist(t.EndX, t.EndY, p) <= t.WidthMM/2+geomEps {
						continue
					}
					if !isConfidentNet(t.NetSource) || !isConfidentNet(p.NetSource) {
						continue
					}
					msg, sug := msgClearanceShort("trace-pad", t.NetName, p.NetName)
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "ERROR",
						Layer:      layer,
						X:          p.X,
						Y:          p.Y,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: 0,
						LimitMM:    minC,
						Unit:       "mm",
						NetName:    t.NetName,
						RefDes:     p.RefDes,
						X2:         p.X,
						Y2:         p.Y,
					})
					layerViolations++
					continue
				}
				if clearance < minC-geomEps {
					msg, sug := msgClearancePadTooClose(clearance, minC)
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "ERROR",
						Layer:      layer,
						X:          p.X,
						Y:          p.Y,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: clearance,
						LimitMM:    minC,
						Unit:       "mm",
						NetName:    t.NetName,
						X2:         p.X,
						Y2:         p.Y,
					})
					layerViolations++
				}
			}
		}

		// P2: pad-to-pad clearance and overlap-as-short. Same sorted-X window
		// pattern as the trace-to-pad sweep; pads are already sorted by X.
		// Same skip policy as the other sweeps: same-net, $NONE$, and
		// unknown-net pairs are not checkable electrical pairs.
		maxPadRadius := 0.0
		for _, p := range pads {
			if r := math.Max(p.WidthMM, p.HeightMM) / 2; r > maxPadRadius {
				maxPadRadius = r
			}
		}
		for i := range pads {
			if layerViolations >= maxClearanceViolations {
				break
			}
			a := pads[i]
			if a.NetName == "" || isNonElectrical(a.NetName) {
				continue
			}
			aRadius := math.Max(a.WidthMM, a.HeightMM) / 2
			for j := i + 1; j < len(pads); j++ {
				if layerViolations >= maxClearanceViolations {
					break
				}
				b := pads[j]
				if b.X-a.X > minC+aRadius+maxPadRadius {
					break
				}
				if b.NetName == a.NetName || b.NetName == "" || isNonElectrical(b.NetName) {
					continue
				}
				bRadius := math.Max(b.WidthMM, b.HeightMM) / 2
				// Quick Y rejection.
				if math.Abs(b.Y-a.Y) > minC+aRadius+bRadius {
					continue
				}
				gap := padToPadGap(a, b)
				if gap <= 0 {
					if !isAttrNet(a.NetSource) || !isAttrNet(b.NetSource) {
						continue
					}
					// Full containment (either pad's center inside the other's
					// copper) is concentric/stacked design — dome-switch rings,
					// shield contacts — or a netlist-vs-geometry label conflict,
					// not a misplacement. Only partial edge overlap is a
					// credible short.
					if padEdgeDist(a.X, a.Y, b) <= geomEps || padEdgeDist(b.X, b.Y, a) <= geomEps {
						continue
					}
					msg, sug := msgClearanceShort("pad", a.NetName, b.NetName)
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "ERROR",
						Layer:      layer,
						X:          a.X,
						Y:          a.Y,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: 0,
						LimitMM:    minC,
						Unit:       "mm",
						NetName:    a.NetName,
						RefDes:     a.RefDes,
						X2:         b.X,
						Y2:         b.Y,
					})
					layerViolations++
					continue
				}
				if gap < minC-geomEps {
					msg, sug := msgClearancePadPairTooClose(gap, minC)
					violations = append(violations, Violation{
						RuleID:     r.ID(),
						Severity:   "ERROR",
						Layer:      layer,
						X:          a.X,
						Y:          a.Y,
						Message:    msg,
						Suggestion: sug,
						MeasuredMM: gap,
						LimitMM:    minC,
						Unit:       "mm",
						NetName:    a.NetName,
						RefDes:     a.RefDes,
						X2:         b.X,
						Y2:         b.Y,
					})
					layerViolations++
				}
			}
		}

		// P3: fill-aware pour pass. Each labeled copper pour is indexed once
		// (edge grid + hole bboxes), then checked against different-net
		// traces, pads, and other pours on the layer. Containment in the
		// filled region is a probable short (confidence-gated, like the
		// other short checks); boundary proximity below minC is a clearance
		// violation.
		polys := polysByLayer[layer]
		if len(polys) > 0 && layerViolations < maxClearanceViolations {
			indexes := make([]*indexedPolygon, len(polys))
			for pi, poly := range polys {
				indexes[pi] = newIndexedPolygon(poly, math.Max(0.5, minC*2))
			}
			for pi, ip := range indexes {
				if ip == nil {
					break
				}
				if layerViolations >= maxClearanceViolations {
					break
				}
				poly := polys[pi]
				pourConfident := isAttrNet(poly.NetSource)

				// Trace vs pour.
				for _, tb := range traces {
					if layerViolations >= maxClearanceViolations {
						break
					}
					t := tb.t
					if t.NetName == "" || isNonElectrical(t.NetName) || t.NetName == poly.NetName {
						continue
					}
					if tb.maxX < ip.minX-minC || tb.minX > ip.maxX+minC ||
						tb.maxY < ip.minY-minC || tb.minY > ip.maxY+minC {
						continue
					}
					mx, my := (t.StartX+t.EndX)/2, (t.StartY+t.EndY)/2
					if pourConfident && isAttrNet(t.NetSource) {
						sx, sy, hit := 0.0, 0.0, false
						switch {
						case ip.contains(t.StartX, t.StartY):
							sx, sy, hit = t.StartX, t.StartY, true
						case ip.contains(t.EndX, t.EndY):
							sx, sy, hit = t.EndX, t.EndY, true
						case ip.contains(mx, my):
							sx, sy, hit = mx, my, true
						}
						if hit {
							msg, sug := msgClearanceShort("trace-pour", t.NetName, poly.NetName)
							violations = append(violations, Violation{
								RuleID:     r.ID(),
								Severity:   "ERROR",
								Layer:      layer,
								X:          sx,
								Y:          sy,
								Message:    msg,
								Suggestion: sug,
								MeasuredMM: 0,
								LimitMM:    minC,
								Unit:       "mm",
								NetName:    t.NetName,
							})
							layerViolations++
							continue
						}
					}
					if d, ok := ip.segDistWithin(t.StartX, t.StartY, t.EndX, t.EndY, minC+t.WidthMM/2); ok {
						clearance := d - t.WidthMM/2
						if clearance > 0 && clearance < minC-geomEps {
							msg, sug := msgClearancePourTooClose("trace", clearance, minC)
							violations = append(violations, Violation{
								RuleID:     r.ID(),
								Severity:   "ERROR",
								Layer:      layer,
								X:          mx,
								Y:          my,
								Message:    msg,
								Suggestion: sug,
								MeasuredMM: clearance,
								LimitMM:    minC,
								Unit:       "mm",
								NetName:    t.NetName,
							})
							layerViolations++
						}
					}
				}

				// Pad vs pour.
				for _, p := range pads {
					if layerViolations >= maxClearanceViolations {
						break
					}
					if p.NetName == "" || isNonElectrical(p.NetName) || p.NetName == poly.NetName {
						continue
					}
					padRadius := math.Max(p.WidthMM, p.HeightMM) / 2
					if p.X < ip.minX-minC-padRadius || p.X > ip.maxX+minC+padRadius ||
						p.Y < ip.minY-minC-padRadius || p.Y > ip.maxY+minC+padRadius {
						continue
					}
					if pourConfident && isAttrNet(p.NetSource) && ip.contains(p.X, p.Y) {
						msg, sug := msgClearanceShort("pad-pour", p.NetName, poly.NetName)
						violations = append(violations, Violation{
							RuleID:     r.ID(),
							Severity:   "ERROR",
							Layer:      layer,
							X:          p.X,
							Y:          p.Y,
							Message:    msg,
							Suggestion: sug,
							MeasuredMM: 0,
							LimitMM:    minC,
							Unit:       "mm",
							NetName:    p.NetName,
							RefDes:     p.RefDes,
						})
						layerViolations++
						continue
					}
					if d, nx, ny, ok := ip.nearestEdgeWithin(p.X, p.Y, minC+padRadius); ok {
						// Approximation: pad extent toward the pour via the
						// support function. Exact for convex pads.
						clearance := d - padProjection(p, nx-p.X, ny-p.Y)
						if clearance > 0 && clearance < minC-geomEps {
							msg, sug := msgClearancePourTooClose("pad", clearance, minC)
							violations = append(violations, Violation{
								RuleID:     r.ID(),
								Severity:   "ERROR",
								Layer:      layer,
								X:          p.X,
								Y:          p.Y,
								Message:    msg,
								Suggestion: sug,
								MeasuredMM: clearance,
								LimitMM:    minC,
								Unit:       "mm",
								NetName:    p.NetName,
								RefDes:     p.RefDes,
								X2:         nx,
								Y2:         ny,
							})
							layerViolations++
						}
					}
				}

				// Pour vs pour (different nets), each pair once.
				for pj := pi + 1; pj < len(indexes); pj++ {
					if layerViolations >= maxClearanceViolations {
						break
					}
					jp := indexes[pj]
					if jp == nil {
						continue
					}
					polyJ := polys[pj]
					if polyJ.NetName == poly.NetName {
						continue
					}
					if jp.maxX < ip.minX-minC || jp.minX > ip.maxX+minC ||
						jp.maxY < ip.minY-minC || jp.minY > ip.maxY+minC {
						continue
					}
					// Overlap → short: any vertex of one fill inside the other.
					if pourConfident && isAttrNet(polyJ.NetSource) {
						sx, sy, hit := 0.0, 0.0, false
						for _, v := range polyJ.Points {
							if ip.contains(v.X, v.Y) {
								sx, sy, hit = v.X, v.Y, true
								break
							}
						}
						if !hit {
							for _, v := range poly.Points {
								if jp.contains(v.X, v.Y) {
									sx, sy, hit = v.X, v.Y, true
									break
								}
							}
						}
						if hit {
							msg, sug := msgClearanceShort("pour", poly.NetName, polyJ.NetName)
							violations = append(violations, Violation{
								RuleID:     r.ID(),
								Severity:   "ERROR",
								Layer:      layer,
								X:          sx,
								Y:          sy,
								Message:    msg,
								Suggestion: sug,
								MeasuredMM: 0,
								LimitMM:    minC,
								Unit:       "mm",
								NetName:    poly.NetName,
							})
							layerViolations++
							continue
						}
					}
					// Boundary distance: scan J's rings against I's edge grid.
					best := math.MaxFloat64
					bx, by := 0.0, 0.0
					scanRing := func(ring []Point) {
						n := len(ring)
						for k := 0; k < n; k++ {
							a := ring[k]
							b := ring[(k+1)%n]
							if d, ok := ip.segDistWithin(a.X, a.Y, b.X, b.Y, minC); ok && d < best {
								best = d
								bx, by = (a.X+b.X)/2, (a.Y+b.Y)/2
							}
						}
					}
					scanRing(polyJ.Points)
					for _, hole := range polyJ.Holes {
						scanRing(hole)
					}
					if best > 0 && best < minC-geomEps {
						msg, sug := msgClearancePourTooClose("pour", best, minC)
						violations = append(violations, Violation{
							RuleID:     r.ID(),
							Severity:   "ERROR",
							Layer:      layer,
							X:          bx,
							Y:          by,
							Message:    msg,
							Suggestion: sug,
							MeasuredMM: best,
							LimitMM:    minC,
							Unit:       "mm",
							NetName:    poly.NetName,
						})
						layerViolations++
					}
				}
			}
		}
	}

	return dedupeViolations(violations, clearanceCellMM)
}

// ptToSegDist returns the minimum distance from point (px,py) to segment (ax,ay)-(bx,by).
func ptToSegDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Sqrt((px-ax)*(px-ax) + (py-ay)*(py-ay))
	}
	t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/(dx*dx+dy*dy)))
	nx, ny := ax+t*dx, ay+t*dy
	return math.Sqrt((px-nx)*(px-nx) + (py-ny)*(py-ny))
}

// segToSegDist returns the minimum distance between two line segments.
func segToSegDist(ax1, ay1, ax2, ay2, bx1, by1, bx2, by2 float64) float64 {
	// Check proper intersection first
	if segsIntersect(ax1, ay1, ax2, ay2, bx1, by1, bx2, by2) {
		return 0
	}
	d1 := ptToSegDist(ax1, ay1, bx1, by1, bx2, by2)
	d2 := ptToSegDist(ax2, ay2, bx1, by1, bx2, by2)
	d3 := ptToSegDist(bx1, by1, ax1, ay1, ax2, ay2)
	d4 := ptToSegDist(bx2, by2, ax1, ay1, ax2, ay2)
	return math.Min(math.Min(d1, d2), math.Min(d3, d4))
}

func segsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	cross2D := func(ox, oy, ux, uy, vx, vy float64) float64 {
		return (ux-ox)*(vy-oy) - (uy-oy)*(vx-ox)
	}
	d1 := cross2D(cx, cy, dx, dy, ax, ay)
	d2 := cross2D(cx, cy, dx, dy, bx, by)
	d3 := cross2D(ax, ay, bx, by, cx, cy)
	d4 := cross2D(ax, ay, bx, by, dx, dy)
	// P2.3: Treat near-zero cross products as collinear (not intersecting).
	pos := func(v float64) bool { return v > geomEps }
	neg := func(v float64) bool { return v < -geomEps }
	return ((pos(d1) && neg(d2)) || (neg(d1) && pos(d2))) &&
		((pos(d3) && neg(d4)) || (neg(d3) && pos(d4)))
}
