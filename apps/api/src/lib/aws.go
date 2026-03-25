package lib

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

type AWSClients struct {
	S3         *s3.Client
	S3Presign  *s3.PresignClient
	SQS        *sqs.Client
	CognitoIDP *cognitoidentityprovider.Client
	Bucket     string
	QueueURL   string
	UserPoolID string
}

func NewAWSClients(ctx context.Context, bucket, queueURL, userPoolID string) (*AWSClients, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	s3c := s3.NewFromConfig(cfg)
	return &AWSClients{
		S3:         s3c,
		S3Presign:  s3.NewPresignClient(s3c),
		SQS:        sqs.NewFromConfig(cfg),
		CognitoIDP: cognitoidentityprovider.NewFromConfig(cfg),
		Bucket:     bucket,
		QueueURL:   queueURL,
		UserPoolID: userPoolID,
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

// CreateCognitoUser creates a user in Cognito and sends an email invitation.
func (a *AWSClients) CreateCognitoUser(ctx context.Context, email, orgID, role string) (string, error) {
	if a.UserPoolID == "" {
		// Dev mode
		return uuid.New().String(), nil
	}
	out, err := a.CognitoIDP.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId: aws.String(a.UserPoolID),
		Username:   aws.String(email),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(email)},
			{Name: aws.String("email_verified"), Value: aws.String("true")},
			{Name: aws.String("custom:orgId"), Value: aws.String(orgID)},
			{Name: aws.String("custom:role"), Value: aws.String(role)},
		},
		DesiredDeliveryMediums: []types.DeliveryMediumType{types.DeliveryMediumTypeEmail},
	})
	if err != nil {
		return "", fmt.Errorf("create cognito user: %w", err)
	}
	// Extract sub from attributes
	for _, attr := range out.User.Attributes {
		if aws.ToString(attr.Name) == "sub" {
			return aws.ToString(attr.Value), nil
		}
	}
	return "", fmt.Errorf("sub not found in cognito response")
}

// DeleteCognitoUser removes a user from Cognito.
func (a *AWSClients) DeleteCognitoUser(ctx context.Context, username string) error {
	if a.UserPoolID == "" {
		return nil
	}
	_, err := a.CognitoIDP.AdminDeleteUser(ctx, &cognitoidentityprovider.AdminDeleteUserInput{
		UserPoolId: aws.String(a.UserPoolID),
		Username:   aws.String(username),
	})
	return err
}

// UpdateCognitoUserAttributes updates custom attributes for a Cognito user.
func (a *AWSClients) UpdateCognitoUserAttributes(ctx context.Context, username, orgID, role string) error {
	if a.UserPoolID == "" {
		return nil
	}
	_, err := a.CognitoIDP.AdminUpdateUserAttributes(ctx, &cognitoidentityprovider.AdminUpdateUserAttributesInput{
		UserPoolId: aws.String(a.UserPoolID),
		Username:   aws.String(username),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("custom:orgId"), Value: aws.String(orgID)},
			{Name: aws.String("custom:role"), Value: aws.String(role)},
		},
	})
	return err
}
