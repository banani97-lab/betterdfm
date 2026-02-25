module github.com/betterdfm/dfm-worker

go 1.23

require (
	github.com/aws/aws-sdk-go-v2 v1.41.1
	github.com/aws/aws-sdk-go-v2/config v1.32.9
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.21
	github.com/betterdfm/dfm-engine v0.0.0
	github.com/google/uuid v1.6.0
	gorm.io/datatypes v1.2.0
	gorm.io/driver/postgres v1.5.7
	gorm.io/gorm v1.25.9
)

replace github.com/betterdfm/dfm-engine => ../../engine/dfm-engine
