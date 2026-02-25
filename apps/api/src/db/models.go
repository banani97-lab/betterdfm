package db

import (
	"time"

	"gorm.io/datatypes"
)

// Organization is a CM tenant
type Organization struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Slug      string    `gorm:"uniqueIndex" json:"slug"`
	Name      string    `json:"name"`
	LogoURL   string    `json:"logoUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

// User is linked to a Cognito sub
type User struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	OrgID      string    `json:"orgId"`
	CognitoSub string    `gorm:"uniqueIndex" json:"cognitoSub"`
	Email      string    `json:"email"`
	Role       string    `json:"role"` // ADMIN | ANALYST | VIEWER
	CreatedAt  time.Time `json:"createdAt"`
}

// CapabilityProfile holds a CM's shop floor constraints
type CapabilityProfile struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	OrgID     string         `json:"orgId"`
	Name      string         `json:"name"`
	IsDefault bool           `json:"isDefault"`
	Rules     datatypes.JSON `json:"rules"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// ProfileRules is the Go struct serialized to JSON in CapabilityProfile.Rules
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

// Submission is an uploaded Gerber or ODB++ file
type Submission struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	OrgID     string    `json:"orgId"`
	UserID    string    `json:"userId"`
	Filename  string    `json:"filename"`
	FileType  string    `json:"fileType"` // GERBER | ODB_PLUS_PLUS
	FileKey   string    `json:"fileKey"`  // S3 key
	Status    string    `json:"status"`   // UPLOADED | ANALYZING | DONE | FAILED
	CreatedAt time.Time `json:"createdAt"`
}

// AnalysisJob is one analysis run
type AnalysisJob struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	SubmissionID string         `json:"submissionId"`
	ProfileID    string         `json:"profileId"`
	Status       string         `json:"status"` // PENDING | PROCESSING | DONE | FAILED
	StartedAt    *time.Time     `json:"startedAt"`
	CompletedAt  *time.Time     `json:"completedAt"`
	ErrorMsg     string         `json:"errorMsg"`
	BoardData    datatypes.JSON `json:"boardData" gorm:"type:jsonb"`
	MfgScore     int            `json:"mfgScore"`
	MfgGrade     string         `json:"mfgGrade"`
}

// Violation is a single DFM issue found.
type Violation struct {
	ID         string  `gorm:"primaryKey" json:"id"`
	JobID      string  `json:"jobId"`
	RuleID     string  `json:"ruleId"`
	Severity   string  `json:"severity"` // ERROR | WARNING | INFO
	Layer      string  `json:"layer"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Message    string  `json:"message"`
	Suggestion string  `json:"suggestion"`
	Count      int     `json:"count"`
	MeasuredMM float64 `json:"measuredMM"`
	LimitMM    float64 `json:"limitMM"`
	Unit       string  `json:"unit"`
	NetName    string  `json:"netName"`
	RefDes     string  `json:"refDes"`
	X2         float64 `json:"x2"`
	Y2         float64 `json:"y2"`
	Ignored    bool    `gorm:"default:false" json:"ignored"`
}
