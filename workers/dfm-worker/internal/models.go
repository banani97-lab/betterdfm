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
	BoardData    []byte     `gorm:"type:jsonb"`
	MfgScore     int
	MfgGrade     string
}

type Submission struct {
	ID       string `gorm:"primaryKey"`
	OrgID    string
	UserID   string
	BatchID  *string
	Filename string
	FileType string
	FileKey  string
	Status   string
}

type CapabilityProfile struct {
	ID        string
	OrgID     string
	Name      string
	IsDefault bool
	Rules     []byte `gorm:"type:jsonb"`
}

type Violation struct {
	ID         string `gorm:"primaryKey"`
	OrgID      string
	JobID      string
	RuleID     string
	Severity   string
	Layer      string
	X          float64
	Y          float64
	Message    string
	Suggestion string
	Count      int
	MeasuredMM float64
	LimitMM    float64
	Unit       string
	NetName    string
	RefDes     string
	X2         float64
	Y2         float64
}
