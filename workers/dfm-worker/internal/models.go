package internal

import "time"

// DB models mirroring the API's models
type AnalysisJob struct {
	ID           string     `gorm:"primaryKey"`
	SubmissionID string
	ProfileID    string
	Status       string
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
