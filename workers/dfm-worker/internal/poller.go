package internal

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

const workerPoolSize = 5

// sqsJob bundles a parsed job ID with the original SQS message so the worker
// goroutine can delete the message after processing.
type sqsJob struct {
	jobID         string
	receiptHandle *string
}

func (w *Worker) Poll(ctx context.Context) {
	// Recovery sweep: re-enqueue PENDING jobs that fell through SQS delivery.
	if w.sqsQueueURL != "" {
		go w.recoverStuckJobs(ctx)
	}

	if w.sqsQueueURL == "" {
		// Dev mode: no SQS, poll the DB directly (unchanged behavior).
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			w.pollDB(ctx)
		}
	}

	// SQS mode: concurrent worker pool.
	jobs := make(chan sqsJob, workerPoolSize*2)

	var wg sync.WaitGroup
	for i := 0; i < workerPoolSize; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.sqsWorker(ctx, id, jobs)
		}(i)
	}

	// Polling loop: receive up to 10 messages and dispatch to workers.
	w.sqsPollLoop(ctx, jobs)

	// Context cancelled — close channel and wait for in-flight jobs.
	close(jobs)
	wg.Wait()
	log.Println("all SQS workers shut down")
}

// sqsPollLoop receives batches of SQS messages and sends parsed jobs to the
// channel. It returns when ctx is cancelled.
func (w *Worker) sqsPollLoop(ctx context.Context, jobs chan<- sqsJob) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		out, err := w.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(w.sqsQueueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("SQS receive error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range out.Messages {
			j, ok := w.parseSQSMessage(ctx, msg)
			if !ok {
				continue
			}
			select {
			case jobs <- j:
			case <-ctx.Done():
				return
			}
		}
	}
}

// parseSQSMessage unmarshals a single SQS message. On unmarshal failure the
// message is deleted immediately and ok is false.
func (w *Worker) parseSQSMessage(ctx context.Context, msg sqstypes.Message) (sqsJob, bool) {
	var payload sqsMessage
	if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &payload); err != nil {
		log.Printf("failed to unmarshal SQS message: %v", err)
		w.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(w.sqsQueueURL),
			ReceiptHandle: msg.ReceiptHandle,
		})
		return sqsJob{}, false
	}
	return sqsJob{jobID: payload.JobID, receiptHandle: msg.ReceiptHandle}, true
}

// sqsWorker processes jobs from the channel, deleting each SQS message after
// processing (whether success or failure).
func (w *Worker) sqsWorker(ctx context.Context, id int, jobs <-chan sqsJob) {
	for j := range jobs {
		log.Printf("[worker-%d] processing job %s", id, j.jobID)
		if err := w.ProcessJob(ctx, j.jobID); err != nil {
			log.Printf("[worker-%d] job %s failed: %v", id, j.jobID, err)
			w.markJobFailedWithBatch(j.jobID, err)
		}
		if _, delErr := w.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(w.sqsQueueURL),
			ReceiptHandle: j.receiptHandle,
		}); delErr != nil {
			log.Printf("[worker-%d] failed to delete SQS message for job %s: %v", id, j.jobID, delErr)
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
			w.markJobFailedWithBatch(job.ID, err)
		}
	}
}
