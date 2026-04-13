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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	dfmengine "github.com/betterdfm/dfm-engine"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Worker struct {
	db           *gorm.DB
	sqsClient    *sqs.Client
	s3Client     *s3.Client
	s3Bucket     string
	sqsQueueURL  string
	gerbonaraURL string
	httpClient   *http.Client
}

func NewWorker(db *gorm.DB, sqsClient *sqs.Client, s3Client *s3.Client, s3Bucket, sqsQueueURL, gerbonaraURL string) *Worker {
	return &Worker{
		db:           db,
		sqsClient:    sqsClient,
		s3Client:     s3Client,
		s3Bucket:     s3Bucket,
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

	Track("Analysis Started", submission.OrgID, map[string]any{
		"jobId":        job.ID,
		"submissionId": submission.ID,
		"orgId":        submission.OrgID,
		"fileType":     submission.FileType,
	})

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
	board = sanitizeBoard(board, jobID)

	// 5.5 Upload board data to S3 and store outline in DB for scoring
	boardJSON, err := json.Marshal(board)
	if err != nil {
		return fmt.Errorf("failed to marshal board data: %w", err)
	}
	boardKey := fmt.Sprintf("results/%s/board.json", jobID)
	if w.s3Client != nil {
		if _, err := w.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(w.s3Bucket),
			Key:         aws.String(boardKey),
			Body:        bytes.NewReader(boardJSON),
			ContentType: aws.String("application/json"),
		}); err != nil {
			return fmt.Errorf("failed to upload board data to S3: %w", err)
		}
		job.BoardDataKey = boardKey
		log.Printf("job %s: uploaded board.json to S3 (%d bytes)", jobID, len(boardJSON))
	} else {
		// Dev mode: no S3, store inline in DB
		job.BoardData = boardJSON
	}
	// Store outline separately for score recalculation (small, ~1 KB)
	outlineData := struct {
		Outline      []dfmengine.Point   `json:"outline"`
		OutlineHoles [][]dfmengine.Point `json:"outlineHoles,omitempty"`
	}{Outline: board.Outline, OutlineHoles: board.OutlineHoles}
	if outlineJSON, err := json.Marshal(outlineData); err == nil {
		job.BoardOutline = outlineJSON
	}
	if err := w.db.Save(&job).Error; err != nil {
		log.Printf("WARN: failed to persist job metadata for %s: %v", job.ID, err)
	}

	// 6. Run DFM rules
	runner := dfmengine.NewRunner()
	engineViolations := runner.Run(board, profileRules)

	// 6.5 Compute manufacturability score
	scoreResult := dfmengine.ComputeScore(engineViolations, board.Outline)
	job.MfgScore = scoreResult.Score
	job.MfgGrade = scoreResult.Grade
	log.Printf("job %s: mfg_score=%d grade=%s", jobID, scoreResult.Score, scoreResult.Grade)

	// 7. Build violation records + bulk insert into DB (for ignore/waive)
	var dbViolations []Violation
	for _, v := range engineViolations {
		dbViolations = append(dbViolations, Violation{
			ID:         uuid.New().String(),
			OrgID:      job.OrgID,
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
	const batchSize = 2000
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

	// 7.5 Upload violations JSON to S3 for fast bulk reads
	if w.s3Client != nil {
		violJSON, err := json.Marshal(dbViolations)
		if err == nil {
			violKey := fmt.Sprintf("results/%s/violations.json", jobID)
			if _, err := w.s3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket:      aws.String(w.s3Bucket),
				Key:         aws.String(violKey),
				Body:        bytes.NewReader(violJSON),
				ContentType: aws.String("application/json"),
			}); err == nil {
				job.ViolationsKey = violKey
				log.Printf("job %s: uploaded violations.json to S3 (%d bytes)", jobID, len(violJSON))
			} else {
				log.Printf("WARN: failed to upload violations to S3 for job %s: %v", jobID, err)
			}
		}
	}

	// 8. Mark job DONE
	done := time.Now()
	job.Status = "DONE"
	job.CompletedAt = &done
	if err := w.db.Save(&job).Error; err != nil {
		return fmt.Errorf("failed to finalize job: %w", err)
	}

	Track("Analysis Completed", submission.OrgID, map[string]any{
		"jobId":          job.ID,
		"submissionId":   submission.ID,
		"orgId":          submission.OrgID,
		"fileType":       submission.FileType,
		"status":         "DONE",
		"durationMs":     time.Since(now).Milliseconds(),
		"score":          job.MfgScore,
		"grade":          job.MfgGrade,
		"violationCount": len(dbViolations),
	})

	// Update submission status
	w.db.Model(&Submission{}).Where("id = ?", submission.ID).Update("status", "DONE")

	// Update batch counters if this submission belongs to a batch
	w.updateBatchProgress(submission.BatchID, "completed")

	return nil
}

// updateBatchProgress atomically increments the completed or failed counter on
// the parent batch (if any) and finalises the batch status when all submissions
// have been processed.
func (w *Worker) updateBatchProgress(batchID *string, field string) {
	if batchID == nil || *batchID == "" {
		return
	}
	bid := *batchID

	// Atomically increment the appropriate counter
	if err := w.db.Model(&Batch{}).Where("id = ?", bid).
		Update(field, gorm.Expr(field+" + 1")).Error; err != nil {
		log.Printf("WARN: failed to increment batch %s %s: %v", bid, field, err)
		return
	}

	// Check if all submissions are now processed
	var batch Batch
	if err := w.db.First(&batch, "id = ?", bid).Error; err != nil {
		log.Printf("WARN: failed to fetch batch %s: %v", bid, err)
		return
	}

	if batch.Completed+batch.Failed >= batch.Total {
		newStatus := "DONE"
		if batch.Failed > 0 {
			newStatus = "PARTIAL_FAIL"
		}
		w.db.Model(&batch).Update("status", newStatus)
		log.Printf("batch %s finalised: status=%s completed=%d failed=%d", bid, newStatus, batch.Completed, batch.Failed)
	}
}

// markJobFailedWithBatch marks a job as FAILED, updates the submission status,
// and increments the batch failed counter.
func (w *Worker) markJobFailedWithBatch(jobID string, jobErr error) {
	w.db.Model(&AnalysisJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":    "FAILED",
		"error_msg": jobErr.Error(),
	})

	// Update submission status and batch counter
	var job AnalysisJob
	if err := w.db.First(&job, "id = ?", jobID).Error; err != nil {
		return
	}
	w.db.Model(&Submission{}).Where("id = ?", job.SubmissionID).Update("status", "FAILED")

	var sub Submission
	if err := w.db.First(&sub, "id = ?", job.SubmissionID).Error; err != nil {
		return
	}

	Track("Analysis Completed", sub.OrgID, map[string]any{
		"jobId":        job.ID,
		"submissionId": sub.ID,
		"orgId":        sub.OrgID,
		"status":       "FAILED",
		"errorMsg":     truncate(jobErr.Error(), 200),
	})

	w.updateBatchProgress(sub.BatchID, "failed")
}

// sanitizeBoard drops degenerate geometry and warns when the outline looks invalid.
func sanitizeBoard(board dfmengine.BoardData, jobID string) dfmengine.BoardData {
	// Drop zero-width traces.
	validTraces := board.Traces[:0]
	for _, t := range board.Traces {
		if t.WidthMM > 0 {
			validTraces = append(validTraces, t)
		}
	}
	dropped := len(board.Traces) - len(validTraces)
	if dropped > 0 {
		log.Printf("job %s: sanitize dropped %d zero-width traces", jobID, dropped)
	}
	board.Traces = validTraces

	// Drop zero-size pads.
	validPads := board.Pads[:0]
	for _, p := range board.Pads {
		if p.WidthMM > 0 && p.HeightMM > 0 {
			validPads = append(validPads, p)
		}
	}
	dropped = len(board.Pads) - len(validPads)
	if dropped > 0 {
		log.Printf("job %s: sanitize dropped %d zero-size pads", jobID, dropped)
	}
	board.Pads = validPads

	// Drop zero-diameter drills.
	validDrills := board.Drills[:0]
	for _, d := range board.Drills {
		if d.DiamMM > 0 {
			validDrills = append(validDrills, d)
		}
	}
	dropped = len(board.Drills) - len(validDrills)
	if dropped > 0 {
		log.Printf("job %s: sanitize dropped %d zero-diameter drills", jobID, dropped)
	}
	board.Drills = validDrills

	if len(board.Outline) < 3 {
		log.Printf("job %s: WARN outline has %d points (< 3); edge-clearance will be skipped", jobID, len(board.Outline))
	}

	return board
}

// truncate returns the first n characters of s, or all of s if shorter.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
