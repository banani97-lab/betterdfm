package main // rapiddfm worker v3

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/betterdfm/dfm-worker/internal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// fipsRequired reports whether AWS clients must use FIPS 140-validated
// endpoints. Forced on in any GovCloud region (us-gov-*) so an ITAR/CUI
// deployment cannot reach non-FIPS endpoints; also settable via
// AWS_USE_FIPS_ENDPOINT. Commercial/dev regions stay on standard endpoints.
func fipsRequired() bool {
	if strings.HasPrefix(strings.ToLower(os.Getenv("AWS_REGION")), "us-gov-") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AWS_USE_FIPS_ENDPOINT"))) {
	case "true", "1", "on", "yes", "enabled":
		return true
	}
	return false
}

// awsLoadOptions returns region/partition-aware LoadDefaultConfig options.
// Region and partition resolve from AWS_REGION; FIPS is layered on when required.
func awsLoadOptions() []func(*config.LoadOptions) error {
	if fipsRequired() {
		return []func(*config.LoadOptions) error{
			config.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled),
		}
	}
	return nil
}

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

	cfg, err := config.LoadDefaultConfig(context.Background(), awsLoadOptions()...)
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
