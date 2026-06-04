# Customer-managed CMK for ITAR/CUI data at rest (S3, RDS, CloudWatch Logs,
# SQS). Holding the key under a US-person-restricted policy is what underpins
# the 22 CFR 120.54(a)(5) encryption position: data encrypted with this key,
# decrypted only by US persons in-boundary, is not an export.
#
# Single-CMK model (decided): one key, with per-tenant isolation enforced at
# the S3 prefix + IAM/key-policy layer (see WS7). Key administrators and users
# must be US persons; that is enforced via IAM and personnel controls (PS/AC)
# out of band, since IAM principals are not US-person-aware by themselves.
resource "aws_kms_key" "main" {
  description             = "${var.name_prefix} CMK for ITAR/CUI data at rest"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  policy                  = data.aws_iam_policy_document.kms.json

  tags = {
    Name = "${var.name_prefix}-cmk"
  }
}

resource "aws_kms_alias" "main" {
  name          = "alias/${var.name_prefix}-cmk"
  target_key_id = aws_kms_key.main.key_id
}

data "aws_iam_policy_document" "kms" {
  # Root retains administrative control; day-to-day key admin is delegated to
  # US-person IAM principals out of band.
  statement {
    sid       = "EnableRootAccountAdmin"
    effect    = "Allow"
    actions   = ["kms:*"]
    resources = ["*"]

    principals {
      type        = "AWS"
      identifiers = ["arn:${data.aws_partition.current.partition}:iam::${data.aws_caller_identity.current.account_id}:root"]
    }
  }

  # Storage/queue services in this account may use the key, scoped by the
  # calling account so cross-account use is impossible.
  statement {
    sid    = "AllowServiceUse"
    effect = "Allow"
    actions = [
      "kms:Encrypt",
      "kms:Decrypt",
      "kms:ReEncrypt*",
      "kms:GenerateDataKey*",
      "kms:DescribeKey",
      "kms:CreateGrant",
    ]
    resources = ["*"]

    principals {
      type = "Service"
      identifiers = [
        "s3.amazonaws.com",
        "rds.amazonaws.com",
        "sqs.amazonaws.com",
      ]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }

  # CloudWatch Logs uses a distinct condition (encryption context carries the
  # log-group ARN) rather than aws:SourceAccount.
  statement {
    sid    = "AllowCloudWatchLogsUse"
    effect = "Allow"
    actions = [
      "kms:Encrypt",
      "kms:Decrypt",
      "kms:ReEncrypt*",
      "kms:GenerateDataKey*",
      "kms:DescribeKey",
    ]
    resources = ["*"]

    principals {
      type        = "Service"
      identifiers = ["logs.${var.region}.amazonaws.com"]
    }

    condition {
      test     = "ArnLike"
      variable = "kms:EncryptionContext:aws:logs:arn"
      values   = ["arn:${data.aws_partition.current.partition}:logs:${var.region}:${data.aws_caller_identity.current.account_id}:log-group:/${var.name_prefix}/*"]
    }
  }
}
