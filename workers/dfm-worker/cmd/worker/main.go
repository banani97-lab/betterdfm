package main // rapiddfm worker v2

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
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

	w := internal.NewWorker(db, sqsClient, sqsQueueURL, gerbonaraURL)
	log.Println("starting SQS polling loop...")
	w.Poll(context.Background())
}
