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
