package internal

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func (w *Worker) Poll(ctx context.Context) {
	// Recovery sweep: re-enqueue PENDING jobs that fell through SQS delivery.
	if w.sqsQueueURL != "" {
		go w.recoverStuckJobs(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if w.sqsQueueURL == "" {
			w.pollDB(ctx)
			continue
		}

		out, err := w.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(w.sqsQueueURL),
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			log.Printf("SQS receive error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range out.Messages {
			var payload sqsMessage
			if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &payload); err != nil {
				log.Printf("failed to unmarshal SQS message: %v", err)
				w.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(w.sqsQueueURL),
					ReceiptHandle: msg.ReceiptHandle,
				})
				continue
			}
			log.Printf("processing job %s", payload.JobID)
			if err := w.ProcessJob(ctx, payload.JobID); err != nil {
				log.Printf("job %s failed: %v", payload.JobID, err)
				w.db.Model(&AnalysisJob{}).Where("id = ?", payload.JobID).Updates(map[string]interface{}{
					"status":    "FAILED",
					"error_msg": err.Error(),
				})
			}
			_, delErr := w.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(w.sqsQueueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			if delErr != nil {
				log.Printf("failed to delete SQS message: %v", delErr)
			}
		}
	}
}

// recoverStuckJobs runs a repeating sweep every 10 minutes, re-enqueuing any PENDING
// jobs older than 5 minutes — recovering from silent SQS delivery failures.
func (w *Worker) recoverStuckJobs(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	// Initial delay before the first sweep so the worker has time to start processing.
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Minute):
	}
	w.doRecovery(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.doRecovery(ctx)
		}
	}
}

func (w *Worker) doRecovery(ctx context.Context) {
	staleThreshold := time.Now().Add(-5 * time.Minute)
	var stuck []AnalysisJob
	if err := w.db.Where("status = ? AND created_at < ?", "PENDING", staleThreshold).Find(&stuck).Error; err != nil {
		log.Printf("[recovery] sweep query error: %v", err)
		return
	}
	for _, job := range stuck {
		log.Printf("[recovery] re-enqueuing stuck job %s", job.ID)
		body := `{"jobId":"` + job.ID + `"}`
		if _, err := w.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    aws.String(w.sqsQueueURL),
			MessageBody: aws.String(body),
		}); err != nil {
			log.Printf("[recovery] re-enqueue failed for job %s: %v", job.ID, err)
		}
	}
}

// pollDB is used in dev mode (no SQS) — picks up PENDING jobs directly from the DB.
func (w *Worker) pollDB(ctx context.Context) {
	var jobs []AnalysisJob
	if err := w.db.Where("status = ?", "PENDING").Find(&jobs).Error; err != nil {
		log.Printf("dev poll error: %v", err)
		time.Sleep(5 * time.Second)
		return
	}

	if len(jobs) == 0 {
		time.Sleep(3 * time.Second)
		return
	}

	for _, job := range jobs {
		log.Printf("[dev] processing job %s", job.ID)
		if err := w.ProcessJob(ctx, job.ID); err != nil {
			log.Printf("[dev] job %s failed: %v", job.ID, err)
			w.db.Model(&AnalysisJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
				"status":    "FAILED",
				"error_msg": err.Error(),
			})
		}
	}
}
