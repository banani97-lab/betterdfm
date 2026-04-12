package dfmengine

// BoardData is the normalized representation of a parsed PCB file
type BoardData struct {
	Layers           []Layer   `json:"layers"`
	Traces           []Trace   `json:"traces"`
	Pads             []Pad     `json:"pads"`
	Vias             []Via     `json:"vias"`
	Drills           []Drill   `json:"drills"`
	Outline          []Point    `json:"outline"`
	OutlineHoles     [][]Point  `json:"outlineHoles,omitempty"` // inner cutout boundaries (slots, step-outs)
	BoardThicknessMM float64    `json:"boardThicknessMM"`
	Warnings         []string   `json:"warnings,omitempty"`
	Polygons         []Polygon  `json:"polygons,omitempty"`
	SourceFormat     string     `json:"sourceFormat,omitempty"` // "ODB_PLUS_PLUS"
}

type Layer struct {
	Name string `json:"name"`
	Type string `json:"type"` // COPPER | POWER_GROUND | SOLDER_MASK | SOLDER_PASTE | SILK | DRILL | OUTLINE | ROUT
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Trace struct {
	Layer   string  `json:"layer"`
	WidthMM float64 `json:"widthMM"`
	StartX  float64 `json:"startX"`
	StartY  float64 `json:"startY"`
	EndX    float64 `json:"endX"`
	EndY    float64 `json:"endY"`
	NetName string  `json:"netName"`
}

type Pad struct {
	Layer    string  `json:"layer"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	WidthMM  float64 `json:"widthMM"`
	HeightMM float64 `json:"heightMM"`
	Shape    string  `json:"shape"`             // RECT | CIRCLE | OVAL | POLYGON
	NetName      string  `json:"netName"`
	RefDes       string  `json:"refDes"`
	PackageClass string  `json:"packageClass,omitempty"` // e.g. "0201", "0402", "0603"
	Contour      []Point `json:"contour,omitempty"`      // polygon contour points when Shape == "POLYGON"
	IsFiducial   bool    `json:"isFiducial,omitempty"`
}

type Via struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	OuterDiamMM float64 `json:"outerDiamMM"`
	DrillDiamMM float64 `json:"drillDiamMM"`
	NetName     string  `json:"netName,omitempty"`
}

type Drill struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	DiamMM float64 `json:"diamMM"`
	Plated bool    `json:"plated"`
}

type Polygon struct {
	Layer   string    `json:"layer"`
	Points  []Point   `json:"points"`
	Holes   [][]Point `json:"holes,omitempty"`
	NetName string    `json:"netName,omitempty"`
}

// ProfileRules defines the CM's manufacturing capabilities
type ProfileRules struct {
	MinTraceWidthMM    float64 `json:"minTraceWidthMM"`
	MinClearanceMM     float64 `json:"minClearanceMM"`
	MinDrillDiamMM     float64 `json:"minDrillDiamMM"`
	MaxDrillDiamMM     float64 `json:"maxDrillDiamMM"`
	MinAnnularRingMM   float64 `json:"minAnnularRingMM"`
	MaxAspectRatio     float64 `json:"maxAspectRatio"`
	MinSolderMaskDamMM float64 `json:"minSolderMaskDamMM"`
	MinEdgeClearanceMM float64 `json:"minEdgeClearanceMM"`
	MinDrillToDrillMM  float64 `json:"minDrillToDrillMM"`
	MinDrillToCopperMM float64 `json:"minDrillToCopperMM"`
	MinCopperSliverMM    float64 `json:"minCopperSliverMM"`
	SmallestPackageClass string  `json:"smallestPackageClass,omitempty"` // e.g. "0402" — smallest passive the CM can place
	MaxTraceImbalanceRatio    float64 `json:"maxTraceImbalanceRatio"`
	EnableSilkscreenOnPadCheck *bool  `json:"enableSilkscreenOnPadCheck"`
}

// Violation is a single DFM issue found.
type Violation struct {
	RuleID     string  `json:"ruleId"`
	Severity   string  `json:"severity"` // ERROR | WARNING | INFO
	Layer      string  `json:"layer"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Message    string  `json:"message"`
	Suggestion string  `json:"suggestion"`
	Count      int     `json:"count"`
	MeasuredMM float64 `json:"measuredMM"` // actual measured value (e.g. 0.08 mm trace width)
	LimitMM    float64 `json:"limitMM"`    // the rule limit that was violated (e.g. 0.10 mm)
	Unit       string  `json:"unit"`       // "mm" | "ratio"
	NetName    string  `json:"netName"`    // net name, "" if unknown
	RefDes     string  `json:"refDes"`     // reference designator, "" if not a component pad
	X2         float64 `json:"x2"`         // second object X (clearance/dam rules), 0 otherwise
	Y2         float64 `json:"y2"`         // second object Y (clearance/dam rules), 0 otherwise
}

// Rule interface
type Rule interface {
	ID() string
	Run(board BoardData, profile ProfileRules) []Violation
}

// isTestPoint returns true for refdes prefixes that denote test points,
// mounting holes, or other non-component features. These should not be
// subjected to IPC-7351 pad-size checks, package-capability checks, or
// tombstoning analysis — their pads are not SMT land patterns.
func isTestPoint(refDes string) bool {
	if len(refDes) < 2 {
		return false
	}
	// TP = test point, MH = mounting hole, FID = fiducial
	c0, c1 := refDes[0], refDes[1]
	if c0 == 'T' && c1 == 'P' {
		return true
	}
	if c0 == 'M' && c1 == 'H' {
		return true
	}
	if len(refDes) >= 3 && c0 == 'F' && c1 == 'I' && refDes[2] == 'D' {
		return true
	}
	return false
}

// outerCopperLayerSet returns the set of outermost copper layer names from
// the board stack — the first and last layer with type COPPER or POWER_GROUND
// in stack order. Component mounting pads can only exist on these layers;
// internal planes (e.g. L02_GND) and inner signal layers cannot host SMT
// lands, so rules that check component pads should filter by this set.
//
// Returns an empty map when no layer metadata is available, in which case
// callers should treat all pads as eligible (current behavior preserved).
func outerCopperLayerSet(layers []Layer) map[string]bool {
	var first, last string
	for _, l := range layers {
		if l.Type == "COPPER" || l.Type == "POWER_GROUND" {
			if first == "" {
				first = l.Name
			}
			last = l.Name
		}
	}
	set := map[string]bool{}
	if first != "" {
		set[first] = true
	}
	if last != "" {
		set[last] = true
	}
	return set
}

// drillLocationSet is a grid-based spatial set of drill hit coordinates.
// Used by rules that check "component mounting pads" to reject pads that
// sit over a drill — those are through-hole via catch-pads or through-hole
// leaded component pads, not SMT land patterns, and applying IPC-7351
// passive pad-size envelopes to them is meaningless.
//
// Stored as a grid of 2 mm cells. Query tolerance is configurable per call
// and should be ≥ cell pitch × half the neighbourhood checked.
type drillLocationSet struct {
	cells  map[[2]int][][2]float64
	cellMM float64
}

func newDrillLocationSet(drills []Drill) *drillLocationSet {
	s := &drillLocationSet{
		cells:  make(map[[2]int][][2]float64),
		cellMM: 2.0,
	}
	for _, d := range drills {
		key := [2]int{int(d.X / s.cellMM), int(d.Y / s.cellMM)}
		s.cells[key] = append(s.cells[key], [2]float64{d.X, d.Y})
	}
	return s
}

// Has reports whether any drill hit lies within tolMM of (x, y).
func (s *drillLocationSet) Has(x, y, tolMM float64) bool {
	if s == nil || len(s.cells) == 0 {
		return false
	}
	tol2 := tolMM * tolMM
	cx := int(x / s.cellMM)
	cy := int(y / s.cellMM)
	// Scan the 3×3 neighbourhood so a drill just across a cell boundary
	// is still found. Safe as long as tolMM ≤ cellMM.
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			bucket := s.cells[[2]int{cx + dx, cy + dy}]
			for _, p := range bucket {
				ddx := p[0] - x
				ddy := p[1] - y
				if ddx*ddx+ddy*ddy <= tol2 {
					return true
				}
			}
		}
	}
	return false
}
