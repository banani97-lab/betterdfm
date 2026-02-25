package lib

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type AWSClients struct {
	S3        *s3.Client
	S3Presign *s3.PresignClient
	SQS       *sqs.Client
	Bucket    string
	QueueURL  string
}

func NewAWSClients(ctx context.Context, bucket, queueURL string) (*AWSClients, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	s3c := s3.NewFromConfig(cfg)
	return &AWSClients{
		S3:        s3c,
		S3Presign: s3.NewPresignClient(s3c),
		SQS:       sqs.NewFromConfig(cfg),
		Bucket:    bucket,
		QueueURL:  queueURL,
	}, nil
}

// PresignPutURL generates a presigned S3 PUT URL valid for 15 minutes.
func (a *AWSClients) PresignPutURL(ctx context.Context, key, contentType string) (string, error) {
	req, err := a.S3Presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return req.URL, nil
}

// EnqueueJob sends a job message to SQS.
func (a *AWSClients) EnqueueJob(ctx context.Context, jobID string) error {
	body := fmt.Sprintf(`{"jobId":%q}`, jobID)
	_, err := a.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(a.QueueURL),
		MessageBody: aws.String(body),
	})
	return err
}
