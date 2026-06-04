output "cloudtrail_name" {
  value = aws_cloudtrail.main.name
}

output "cloudtrail_bucket" {
  value = aws_s3_bucket.cloudtrail.id
}

output "cloudtrail_log_group" {
  value = aws_cloudwatch_log_group.cloudtrail.name
}

output "security_alerts_topic_arn" {
  description = "Subscribe responders (email/PagerDuty) to this SNS topic out of band."
  value       = aws_sns_topic.security_alerts.arn
}
