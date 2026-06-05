# Uploads bucket: holds customer ODB++ archives + derived board/violation data.
# SSE-KMS with the CMK, versioned, fully private, TLS-only. Per-tenant isolation
# is enforced by the submissions/<orgId>/... key prefix that the application
# already writes (see WS7) plus the scoped task-role policy in iam.tf.
resource "aws_s3_bucket" "uploads" {
  bucket = "${var.name_prefix}-uploads-${data.aws_caller_identity.current.account_id}"

  tags = {
    Name = "${var.name_prefix}-uploads"
  }
}

resource "aws_s3_bucket_public_access_block" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.main.arn
    }
    bucket_key_enabled = true
  }
}

# Deny non-TLS access and any upload that is not SSE-KMS with our CMK (SC-8/SC-28).
data "aws_iam_policy_document" "uploads" {
  statement {
    sid       = "DenyInsecureTransport"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.uploads.arn, "${aws_s3_bucket.uploads.arn}/*"]

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

  # Block PUTs that EXPLICITLY request a non-KMS encryption (e.g. SSE-S3/AES256).
  # The two conditions are ANDed: the Deny fires only when the encryption header
  # is present (Null = false) AND its value is not aws:kms. Header-less presigned
  # PUTs (no encryption header) are therefore allowed and fall through to the
  # bucket's default SSE-KMS (CMK) encryption, so objects are always CMK-encrypted
  # at rest, while an explicit AES256 upload is rejected.
  statement {
    sid       = "DenyNonKMSPuts"
    effect    = "Deny"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.uploads.arn}/*"]

    principals {
      type        = "AWS"
      identifiers = ["*"]
    }

    condition {
      test     = "StringNotEquals"
      variable = "s3:x-amz-server-side-encryption"
      values   = ["aws:kms"]
    }

    condition {
      test     = "Null"
      variable = "s3:x-amz-server-side-encryption"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "uploads" {
  bucket = aws_s3_bucket.uploads.id
  policy = data.aws_iam_policy_document.uploads.json
}

# CORS so the browser can PUT directly to S3 via presigned URLs. Locked to the
# app origin when a domain is set; permissive only in the no-domain alpha case.
resource "aws_s3_bucket_cors_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  cors_rule {
    allowed_methods = ["PUT", "GET"]
    allowed_origins = local.use_custom_domain ? ["https://${local.app_fqdn}"] : ["*"]
    allowed_headers = ["*"]
    expose_headers  = ["ETag"]
    max_age_seconds = 3000
  }
}
