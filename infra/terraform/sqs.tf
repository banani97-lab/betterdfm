# Analysis job queue (api enqueues, worker consumes) + dead-letter queue.
# Encrypted with the CMK.
resource "aws_sqs_queue" "jobs_dlq" {
  name                      = "${var.name_prefix}-jobs-dlq"
  kms_master_key_id         = aws_kms_key.main.arn
  message_retention_seconds = 1209600 # 14 days

  tags = {
    Name = "${var.name_prefix}-jobs-dlq"
  }
}

resource "aws_sqs_queue" "jobs" {
  name                       = "${var.name_prefix}-jobs"
  kms_master_key_id          = aws_kms_key.main.arn
  visibility_timeout_seconds = 900 # >= worst-case analysis time
  message_retention_seconds  = 345600

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.jobs_dlq.arn
    maxReceiveCount     = 5
  })

  tags = {
    Name = "${var.name_prefix}-jobs"
  }
}
