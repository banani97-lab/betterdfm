package dfmengine

// BoardData is the normalized representation of a parsed PCB file
type BoardData struct {
	Layers           []Layer   `json:"layers"`
	Traces           []Trace   `json:"traces"`
	Pads             []Pad     `json:"pads"`
	Vias             []Via     `json:"vias"`
	Drills           []Drill   `json:"drills"`
	Outline          []Point   `json:"outline"`
	BoardThicknessMM float64   `json:"boardThicknessMM"`
	Warnings         []string  `json:"warnings,omitempty"`
	Polygons         []Polygon `json:"polygons,omitempty"`
}

type Layer struct {
	Name string `json:"name"`
	Type string `json:"type"` // COPPER | SOLDER_MASK | SILK | DRILL | OUTLINE
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
	Shape    string  `json:"shape"` // RECT | CIRCLE | OVAL
	NetName  string  `json:"netName"`
	RefDes   string  `json:"refDes"`
}

type Via struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	OuterDiamMM float64 `json:"outerDiamMM"`
	DrillDiamMM float64 `json:"drillDiamMM"`
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
