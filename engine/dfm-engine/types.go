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
	SourceFormat     string     `json:"sourceFormat,omitempty"` // "GERBER" | "ODB_PLUS_PLUS"
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
	NetName    string  `json:"netName"`    // net name, "" for Gerber or unknown
	RefDes     string  `json:"refDes"`     // reference designator, "" if not a component pad
	X2         float64 `json:"x2"`         // second object X (clearance/dam rules), 0 otherwise
	Y2         float64 `json:"y2"`         // second object Y (clearance/dam rules), 0 otherwise
}

// Rule interface
type Rule interface {
	ID() string
	Run(board BoardData, profile ProfileRules) []Violation
}
