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
