package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	dfmengine "github.com/betterdfm/dfm-engine"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Worker struct {
	db           *gorm.DB
	sqsClient    *sqs.Client
	sqsQueueURL  string
	gerbonaraURL string
	httpClient   *http.Client
}

func NewWorker(db *gorm.DB, sqsClient *sqs.Client, sqsQueueURL, gerbonaraURL string) *Worker {
	return &Worker{
		db:           db,
		sqsClient:    sqsClient,
		sqsQueueURL:  sqsQueueURL,
		gerbonaraURL: gerbonaraURL,
		httpClient:   &http.Client{Timeout: 5 * time.Minute},
	}
}

type sqsMessage struct {
	JobID string `json:"jobId"`
}

type parseRequest struct {
	FileKey  string `json:"fileKey"`
	FileType string `json:"fileType"`
	Bucket   string `json:"bucket"`
}

func (w *Worker) ProcessJob(ctx context.Context, jobID string) error {
	now := time.Now()

	// 1. Fetch job
	var job AnalysisJob
	if err := w.db.First(&job, "id = ?", jobID).Error; err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	// 2. Mark as PROCESSING
	job.Status = "PROCESSING"
	job.StartedAt = &now
	if err := w.db.Save(&job).Error; err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// 3. Fetch submission
	var submission Submission
	if err := w.db.First(&submission, "id = ?", job.SubmissionID).Error; err != nil {
		return fmt.Errorf("submission not found: %w", err)
	}

	// 4. Fetch capability profile
	var profile CapabilityProfile
	if err := w.db.First(&profile, "id = ?", job.ProfileID).Error; err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}
	var profileRules dfmengine.ProfileRules
	if err := json.Unmarshal(profile.Rules, &profileRules); err != nil {
		return fmt.Errorf("failed to unmarshal profile rules: %w", err)
	}

	// 5. Call gerbonara sidecar
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		bucket = "betterdfm-uploads"
	}
	reqBody, _ := json.Marshal(parseRequest{
		FileKey:  submission.FileKey,
		FileType: submission.FileType,
		Bucket:   bucket,
	})
	resp, err := w.httpClient.Post(w.gerbonaraURL+"/parse", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("gerbonara request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading gerbonara response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gerbonara returned %d: %s", resp.StatusCode, string(body))
	}

	var board dfmengine.BoardData
	if err := json.Unmarshal(body, &board); err != nil {
		return fmt.Errorf("failed to unmarshal board data: %w", err)
	}

	// 5.5 Persist board data on the job record for visualization
	if boardJSON, err := json.Marshal(board); err == nil {
		job.BoardData = boardJSON
		if err := w.db.Save(&job).Error; err != nil {
			log.Printf("WARN: failed to persist board_data for job %s: %v", job.ID, err)
			// Non-fatal: DFM analysis can still proceed without stored board geometry
		}
	}

	// 6. Run DFM rules
	runner := dfmengine.NewRunner()
	engineViolations := runner.Run(board, profileRules)

	// 6.5 Compute manufacturability score
	scoreResult := dfmengine.ComputeScore(engineViolations, board.Outline)
	job.MfgScore = scoreResult.Score
	job.MfgGrade = scoreResult.Grade
	log.Printf("job %s: mfg_score=%d grade=%s", jobID, scoreResult.Score, scoreResult.Grade)

	// 7. Bulk insert violations
	var dbViolations []Violation
	for _, v := range engineViolations {
		dbViolations = append(dbViolations, Violation{
			ID:         uuid.New().String(),
			JobID:      jobID,
			RuleID:     v.RuleID,
			Severity:   v.Severity,
			Layer:      v.Layer,
			X:          v.X,
			Y:          v.Y,
			Message:    v.Message,
			Suggestion: v.Suggestion,
			Count:      v.Count,
			MeasuredMM: v.MeasuredMM,
			LimitMM:    v.LimitMM,
			Unit:       v.Unit,
			NetName:    v.NetName,
			RefDes:     v.RefDes,
			X2:         v.X2,
			Y2:         v.Y2,
		})
	}
	const batchSize = 500
	for i := 0; i < len(dbViolations); i += batchSize {
		end := i + batchSize
		if end > len(dbViolations) {
			end = len(dbViolations)
		}
		batch := dbViolations[i:end]
		if err := w.db.Create(&batch).Error; err != nil {
			return fmt.Errorf("failed to insert violations: %w", err)
		}
	}
	log.Printf("job %s: inserted %d violations", jobID, len(dbViolations))

	// 8. Mark job DONE
	done := time.Now()
	job.Status = "DONE"
	job.CompletedAt = &done
	if err := w.db.Save(&job).Error; err != nil {
		return fmt.Errorf("failed to finalize job: %w", err)
	}

	// Update submission status
	w.db.Model(&Submission{}).Where("id = ?", submission.ID).Update("status", "DONE")
	return nil
}
