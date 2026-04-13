package db

import (
	"time"

	"gorm.io/datatypes"
)

// Organization is a CM tenant
type Organization struct {
	ID                 string    `gorm:"primaryKey" json:"id"`
	Slug               string    `gorm:"uniqueIndex" json:"slug"`
	Name               string    `json:"name"`
	LogoURL            string    `json:"logoUrl"`
	SubscriptionTier   string    `gorm:"default:STARTER" json:"subscriptionTier"` // STARTER | PROFESSIONAL | ENTERPRISE
	BillingCycleAnchor time.Time `json:"billingCycleAnchor"`
	CreatedAt          time.Time `json:"createdAt"`
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
	MinDrillToDrillMM  float64 `json:"minDrillToDrillMM"`
	MinDrillToCopperMM float64 `json:"minDrillToCopperMM"`
	MinCopperSliverMM          float64 `json:"minCopperSliverMM"`
	MaxTraceImbalanceRatio     float64 `json:"maxTraceImbalanceRatio"`
	EnableSilkscreenOnPadCheck *bool   `json:"enableSilkscreenOnPadCheck"`
}

// Project groups related submissions
type Project struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	OrgID       string    `gorm:"index" json:"orgId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CustomerRef string    `json:"customerRef"`
	CreatedBy   string    `json:"createdBy"`
	Archived    bool      `gorm:"default:false" json:"archived"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Batch groups multiple submissions uploaded together
type Batch struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	OrgID     string    `gorm:"index" json:"orgId"`
	ProjectID *string   `json:"projectId"`
	UserID    string    `json:"userId"`
	ProfileID *string   `json:"profileId"`
	Status    string    `gorm:"default:PENDING" json:"status"` // PENDING | PROCESSING | DONE | PARTIAL_FAIL
	Total     int       `json:"total"`
	Completed int       `gorm:"default:0" json:"completed"`
	Failed    int       `gorm:"default:0" json:"failed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Submission is an uploaded ODB++ file
type Submission struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	OrgID     string    `json:"orgId"`
	UserID    string    `json:"userId"`
	ProjectID *string   `json:"projectId"`
	BatchID   *string   `json:"batchId"`
	Filename  string    `json:"filename"`
	FileType  string    `json:"fileType"` // ODB_PLUS_PLUS
	FileKey   string    `json:"fileKey"`  // S3 key
	Status    string    `json:"status"`   // UPLOADED | ANALYZING | DONE | FAILED
	CreatedAt time.Time `json:"createdAt"`
}

// AnalysisJob is one analysis run
type AnalysisJob struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	OrgID        string         `json:"orgId"`
	SubmissionID string         `json:"submissionId"`
	ProfileID    string         `json:"profileId"`
	Status       string         `json:"status"` // PENDING | PROCESSING | DONE | FAILED
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	StartedAt    *time.Time     `json:"startedAt"`
	CompletedAt  *time.Time     `json:"completedAt"`
	ErrorMsg     string         `json:"errorMsg"`
	BoardData     datatypes.JSON `json:"boardData" gorm:"type:jsonb"`
	BoardDataKey  string         `json:"-" gorm:"column:board_data_key"`
	ViolationsKey string         `json:"-" gorm:"column:violations_key"`
	BoardOutline  datatypes.JSON `json:"-" gorm:"type:jsonb;column:board_outline"`
	MfgScore      int            `json:"mfgScore"`
	MfgGrade      string         `json:"mfgGrade"`
}

// ShareLink is a token-based share link for customer portal access
type ShareLink struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	OrgID       string     `gorm:"index" json:"orgId"`
	Token       string     `gorm:"uniqueIndex" json:"token"`
	ProjectID   *string    `json:"projectId"`   // nullable — share a whole project
	JobID       *string    `json:"jobId"`        // nullable — share a single job
	CreatedBy   string     `json:"createdBy"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	AllowUpload bool       `gorm:"default:false" json:"allowUpload"`
	Active      bool       `gorm:"default:true" json:"active"`
	Label       string     `json:"label"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// ShareUpload tracks files uploaded by customers through a share link
type ShareUpload struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	ShareLinkID   string    `json:"shareLinkId"`
	SubmissionID  string    `json:"submissionId"`
	UploaderName  string    `json:"uploaderName"`
	UploaderEmail string    `json:"uploaderEmail"`
	CreatedAt     time.Time `json:"createdAt"`
}

// ContactSubmission stores contact form submissions from the landing page
type ContactSubmission struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Company   string    `json:"company"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// UsageEvent tracks billable events per org
type UsageEvent struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	OrgID     string    `gorm:"index:idx_usage_org_created" json:"orgId"`
	EventType string    `json:"eventType"` // ANALYSIS
	RefID     string    `json:"refId"`
	Overage   bool      `gorm:"default:false" json:"overage"`
	CreatedAt time.Time `gorm:"index:idx_usage_org_created" json:"createdAt"`
}

// Violation is a single DFM issue found.
type Violation struct {
	ID         string  `gorm:"primaryKey" json:"id"`
	OrgID      string  `json:"orgId"`
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
