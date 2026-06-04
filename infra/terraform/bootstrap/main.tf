provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "rapiddfm"
      Environment = "gov"
      Compliance  = "ITAR-CUI"
      ManagedBy   = "terraform-bootstrap"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

variable "region" {
  description = "GovCloud region (must match the root module)."
  type        = string
  default     = "us-gov-west-1"

  validation {
    condition     = startswith(var.region, "us-gov-")
    error_message = "region must be a GovCloud region (us-gov-*)."
  }
}

variable "name_prefix" {
  description = "Prefix for the state bucket / key names."
  type        = string
  default     = "rapiddfm"
}

# Dedicated CMK for Terraform state at rest (state can contain secrets such as
# the generated DB password).
resource "aws_kms_key" "state" {
  description             = "${var.name_prefix} Terraform state encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true
}

resource "aws_kms_alias" "state" {
  name          = "alias/${var.name_prefix}-tfstate"
  target_key_id = aws_kms_key.state.key_id
}

resource "aws_s3_bucket" "state" {
  bucket = "${var.name_prefix}-tfstate-${data.aws_caller_identity.current.account_id}"

  tags = { Name = "${var.name_prefix}-tfstate" }
}

resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.state.arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "state" {
  bucket = aws_s3_bucket.state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

data "aws_iam_policy_document" "state" {
  statement {
    sid       = "DenyInsecureTransport"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.state.arn, "${aws_s3_bucket.state.arn}/*"]

    principals {
      type        = "AWS"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "state" {
  bucket = aws_s3_bucket.state.id
  policy = data.aws_iam_policy_document.state.json
}

output "state_bucket" {
  value = aws_s3_bucket.state.id
}

output "state_kms_key_arn" {
  value = aws_kms_key.state.arn
}

# Copy/paste-ready backend config for the root module.
output "backend_hcl" {
  description = "Write these lines into ../backend.hcl"
  value       = <<-EOT
    bucket       = "${aws_s3_bucket.state.id}"
    key          = "rapiddfm/govcloud/terraform.tfstate"
    region       = "${var.region}"
    kms_key_id   = "${aws_kms_key.state.arn}"
    encrypt      = true
    use_lockfile = true
  EOT
}
