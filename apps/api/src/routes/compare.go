package routes

import (
	"errors"
	"math"
	"net/http"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CompareHandler handles design comparison between two analysis jobs.
type CompareHandler struct {
	db *gorm.DB
}

// NewCompareHandler creates a new CompareHandler.
func NewCompareHandler(database *gorm.DB) *CompareHandler {
	return &CompareHandler{db: database}
}

// matchThresholdMM is the maximum Euclidean distance (in mm) for two
// violations to be considered the same location across revisions.
const matchThresholdMM = 0.5

// comparisonJobSummary is the per-job metadata returned in the response.
type comparisonJobSummary struct {
	ID          string  `json:"id"`
	MfgScore    int     `json:"mfgScore"`
	MfgGrade    string  `json:"mfgGrade"`
	Filename    string  `json:"filename"`
	CompletedAt *string `json:"completedAt"`
}

// comparisonSummary holds aggregate counts.
type comparisonSummary struct {
	FixedCount     int `json:"fixedCount"`
	NewCount       int `json:"newCount"`
	UnchangedCount int `json:"unchangedCount"`
}

// unchangedPair groups matching violations from job A and job B.
type unchangedPair struct {
	A db.Violation `json:"a"`
	B db.Violation `json:"b"`
}

// comparisonResponse is the full JSON response.
type comparisonResponse struct {
	JobA       comparisonJobSummary `json:"jobA"`
	JobB       comparisonJobSummary `json:"jobB"`
	ScoreDelta int                  `json:"scoreDelta"`
	Summary    comparisonSummary    `json:"summary"`
	Fixed      []db.Violation       `json:"fixed"`
	New        []db.Violation       `json:"new"`
	Unchanged  []unchangedPair      `json:"unchanged"`
}

// Compare GET /compare?jobA=X&jobB=Y
// Compares two analysis jobs and returns fixed, new, and unchanged violations.
func (h *CompareHandler) Compare(c echo.Context) error {
	user := lib.GetUser(c)
	jobAID := c.QueryParam("jobA")
	jobBID := c.QueryParam("jobB")

	if jobAID == "" || jobBID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "jobA and jobB query params are required")
	}
	if jobAID == jobBID {
		return echo.NewHTTPError(http.StatusBadRequest, "jobA and jobB must be different")
	}

	// Fetch both jobs, verifying they belong to the same org as the user.
	var jobA, jobB db.AnalysisJob
	if err := h.db.First(&jobA, "id = ? AND org_id = ?", jobAID, user.OrgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "job A not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if err := h.db.First(&jobB, "id = ? AND org_id = ?", jobBID, user.OrgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "job B not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Both jobs must be DONE.
	if jobA.Status != "DONE" {
		return echo.NewHTTPError(http.StatusBadRequest, "job A is not complete")
	}
	if jobB.Status != "DONE" {
		return echo.NewHTTPError(http.StatusBadRequest, "job B is not complete")
	}

	// Fetch non-ignored violations for both jobs.
	var violationsA, violationsB []db.Violation
	if err := h.db.Where("job_id = ? AND ignored = false", jobAID).Find(&violationsA).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if err := h.db.Where("job_id = ? AND ignored = false", jobBID).Find(&violationsB).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Run violation matching algorithm.
	fixed, newViolations, unchanged := matchViolations(violationsA, violationsB)

	// Fetch submission filenames for context.
	filenameA := fetchFilename(h.db, jobA.SubmissionID)
	filenameB := fetchFilename(h.db, jobB.SubmissionID)

	// Build completed-at strings.
	var completedAtA, completedAtB *string
	if jobA.CompletedAt != nil {
		s := jobA.CompletedAt.Format("2006-01-02T15:04:05Z")
		completedAtA = &s
	}
	if jobB.CompletedAt != nil {
		s := jobB.CompletedAt.Format("2006-01-02T15:04:05Z")
		completedAtB = &s
	}

	return c.JSON(http.StatusOK, comparisonResponse{
		JobA: comparisonJobSummary{
			ID:          jobA.ID,
			MfgScore:    jobA.MfgScore,
			MfgGrade:    jobA.MfgGrade,
			Filename:    filenameA,
			CompletedAt: completedAtA,
		},
		JobB: comparisonJobSummary{
			ID:          jobB.ID,
			MfgScore:    jobB.MfgScore,
			MfgGrade:    jobB.MfgGrade,
			Filename:    filenameB,
			CompletedAt: completedAtB,
		},
		ScoreDelta: jobB.MfgScore - jobA.MfgScore,
		Summary: comparisonSummary{
			FixedCount:     len(fixed),
			NewCount:       len(newViolations),
			UnchangedCount: len(unchanged),
		},
		Fixed:     fixed,
		New:       newViolations,
		Unchanged: unchanged,
	})
}

// matchViolations implements the matching algorithm:
//   - Two violations "match" if same ruleId AND same layer AND Euclidean
//     distance between (x,y) < matchThresholdMM.
//   - For each violation in A, find the closest match in B.
//   - Unmatched A → "fixed". Unmatched B → "new". Matched → "unchanged".
func matchViolations(violationsA, violationsB []db.Violation) (fixed []db.Violation, newV []db.Violation, unchanged []unchangedPair) {
	// Track which B violations have been matched.
	matchedB := make([]bool, len(violationsB))

	for _, va := range violationsA {
		bestIdx := -1
		bestDist := math.MaxFloat64

		for j, vb := range violationsB {
			if matchedB[j] {
				continue
			}
			if va.RuleID != vb.RuleID || va.Layer != vb.Layer {
				continue
			}
			dist := math.Hypot(va.X-vb.X, va.Y-vb.Y)
			if dist < matchThresholdMM && dist < bestDist {
				bestDist = dist
				bestIdx = j
			}
		}

		if bestIdx >= 0 {
			matchedB[bestIdx] = true
			unchanged = append(unchanged, unchangedPair{A: va, B: violationsB[bestIdx]})
		} else {
			fixed = append(fixed, va)
		}
	}

	// Unmatched B violations are "new".
	for j, vb := range violationsB {
		if !matchedB[j] {
			newV = append(newV, vb)
		}
	}

	// Ensure non-nil slices for JSON serialization.
	if fixed == nil {
		fixed = []db.Violation{}
	}
	if newV == nil {
		newV = []db.Violation{}
	}
	if unchanged == nil {
		unchanged = []unchangedPair{}
	}

	return fixed, newV, unchanged
}

// fetchFilename looks up the submission filename for a given submission ID.
func fetchFilename(database *gorm.DB, submissionID string) string {
	var sub db.Submission
	if err := database.Select("filename").First(&sub, "id = ?", submissionID).Error; err != nil {
		return ""
	}
	return sub.Filename
}
