# Security alerting (AU-6 / IR): notify on high-signal events from the
# CloudTrail log stream. Subscriptions (email/PagerDuty/etc.) are added out of
# band so they are not committed.
resource "aws_sns_topic" "security_alerts" {
  name              = "${var.name_prefix}-security-alerts"
  kms_master_key_id = aws_kms_key.main.arn

  tags = { Name = "${var.name_prefix}-security-alerts" }
}

locals {
  security_metric_ns = "${var.name_prefix}/Security"

  # CloudTrail metric-filter alarms: pattern -> human label.
  security_filters = {
    root-usage = {
      pattern = "{ $.userIdentity.type = \"Root\" && $.userIdentity.invokedBy NOT EXISTS && $.eventType != \"AwsServiceEvent\" }"
      metric  = "RootAccountUsage"
    }
    unauthorized-api = {
      pattern = "{ ($.errorCode = \"*UnauthorizedOperation\") || ($.errorCode = \"AccessDenied*\") }"
      metric  = "UnauthorizedApiCalls"
    }
    console-no-mfa = {
      pattern = "{ ($.eventName = \"ConsoleLogin\") && ($.additionalEventData.MFAUsed != \"Yes\") }"
      metric  = "ConsoleSignInWithoutMFA"
    }
  }
}

resource "aws_cloudwatch_log_metric_filter" "security" {
  for_each = local.security_filters

  name           = "${var.name_prefix}-${each.key}"
  log_group_name = aws_cloudwatch_log_group.cloudtrail.name
  pattern        = each.value.pattern

  metric_transformation {
    name          = each.value.metric
    namespace     = local.security_metric_ns
    value         = "1"
    default_value = "0"
  }
}

resource "aws_cloudwatch_metric_alarm" "security" {
  for_each = local.security_filters

  alarm_name          = "${var.name_prefix}-${each.key}"
  alarm_description   = "Security event: ${each.key}"
  namespace           = local.security_metric_ns
  metric_name         = each.value.metric
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.security_alerts.arn]

  depends_on = [aws_cloudwatch_log_metric_filter.security]
}

# Audit-process-failure alarm (NIST 800-171 3.3.4): fire if the CloudTrail log
# stream stops delivering. A metric filter cannot detect stoppage (no events =
# no match), so alarm on the log group's IncomingLogEvents dropping to zero.
# treat_missing_data = breaching so an absence of logs trips the alarm.
resource "aws_cloudwatch_metric_alarm" "audit_delivery_stalled" {
  alarm_name          = "${var.name_prefix}-audit-delivery-stalled"
  alarm_description   = "Audit failure: no CloudTrail events delivered to CloudWatch in the last hour"
  namespace           = "AWS/Logs"
  metric_name         = "IncomingLogEvents"
  dimensions          = { LogGroupName = aws_cloudwatch_log_group.cloudtrail.name }
  statistic           = "Sum"
  period              = 3600
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.security_alerts.arn]
}
