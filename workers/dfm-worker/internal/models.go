package internal

import "time"

// Batch groups multiple submissions uploaded together
type Batch struct {
	ID        string    `gorm:"primaryKey"`
	OrgID     string
	ProjectID *string
	UserID    string
	ProfileID *string
	Status    string `gorm:"default:PENDING"`
	Total     int
	Completed int `gorm:"default:0"`
	Failed    int `gorm:"default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DB models mirroring the API's models
type AnalysisJob struct {
	ID           string     `gorm:"primaryKey"`
	OrgID        string
	SubmissionID string
	ProfileID    string
	Status       string
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ErrorMsg     string
	BoardData     []byte `gorm:"type:jsonb"`
	BoardDataKey  string `gorm:"column:board_data_key"`
	ViolationsKey string `gorm:"column:violations_key"`
	BoardOutline  []byte `gorm:"type:jsonb;column:board_outline"`
	MfgScore      int
	MfgGrade      string
}

type Submission struct {
	ID        string  `gorm:"primaryKey"`
	OrgID     string
	UserID    string
	ProjectID *string
	BatchID   *string
	Filename  string
	FileType  string
	FileKey   string
	Status    string
}

type CapabilityProfile struct {
	ID        string
	OrgID     string
	Name      string
	IsDefault bool
	Rules     []byte `gorm:"type:jsonb"`
}

type Violation struct {
	ID         string  `gorm:"primaryKey" json:"id"`
	OrgID      string  `json:"orgId"`
	JobID      string  `json:"jobId"`
	RuleID     string  `json:"ruleId"`
	Severity   string  `json:"severity"`
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
