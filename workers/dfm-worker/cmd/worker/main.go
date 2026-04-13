package main // rapiddfm worker v3

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/betterdfm/dfm-worker/internal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	internal.InitAnalytics()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:secret@localhost:5432/betterdfm"
	}
	sqsQueueURL := os.Getenv("SQS_QUEUE_URL")
	gerbonaraURL := os.Getenv("GERBONARA_URL")
	if gerbonaraURL == "" {
		gerbonaraURL = "http://localhost:8001"
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("connected to database")

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}
	sqsClient := sqs.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		s3Bucket = "betterdfm-uploads"
	}

	w := internal.NewWorker(db, sqsClient, s3Client, s3Bucket, sqsQueueURL, gerbonaraURL)

	// Wait for the gerbonara sidecar to be healthy before consuming jobs.
	// Both containers start simultaneously in Fargate; without this check
	// the worker grabs a job from SQS before gerbonara has finished booting
	// and fails with "connection refused".
	internal.WaitForSidecar(gerbonaraURL+"/health", 120)

	log.Println("starting SQS polling loop...")
	w.Poll(context.Background())
}
