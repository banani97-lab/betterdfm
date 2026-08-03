data "aws_iam_policy_document" "ecs_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# ---------------------------------------------------------------------------
# Execution role: shared by all task definitions. Pulls images, writes logs,
# and reads the DB secret to inject DATABASE_URL.
# ---------------------------------------------------------------------------
resource "aws_iam_role" "execution" {
  name               = "${var.name_prefix}-ecs-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}

resource "aws_iam_role_policy_attachment" "execution_managed" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "execution_extra" {
  statement {
    sid       = "ReadDbSecret"
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [
      aws_secretsmanager_secret.database_url.arn,
      aws_secretsmanager_secret.internal_tls.arn,
    ]
  }

  statement {
    sid       = "DecryptSecret"
    effect    = "Allow"
    actions   = ["kms:Decrypt"]
    resources = [aws_kms_key.main.arn]
  }
}

resource "aws_iam_role_policy" "execution_extra" {
  name   = "secrets"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution_extra.json
}

# ---------------------------------------------------------------------------
# Per-service task roles (the role the container code runs as).
# ---------------------------------------------------------------------------
resource "aws_iam_role" "task" {
  for_each           = toset(local.services)
  name               = "${var.name_prefix}-task-${each.key}"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}

# api: presign/read/write uploads, enqueue jobs, manage Cognito users, use CMK.
data "aws_iam_policy_document" "task_api" {
  statement {
    sid       = "S3Objects"
    effect    = "Allow"
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["${aws_s3_bucket.uploads.arn}/*"]
  }
  statement {
    sid       = "S3List"
    effect    = "Allow"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.uploads.arn]
  }
  statement {
    sid       = "SQSSend"
    effect    = "Allow"
    actions   = ["sqs:SendMessage", "sqs:GetQueueAttributes", "sqs:GetQueueUrl"]
    resources = [aws_sqs_queue.jobs.arn]
  }
  statement {
    sid       = "Cognito"
    effect    = "Allow"
    actions   = ["cognito-idp:AdminCreateUser", "cognito-idp:AdminDeleteUser", "cognito-idp:AdminUpdateUserAttributes", "cognito-idp:AdminGetUser"]
    resources = [aws_cognito_user_pool.main.arn]
  }
  statement {
    sid       = "KMS"
    effect    = "Allow"
    actions   = ["kms:Encrypt", "kms:Decrypt", "kms:GenerateDataKey", "kms:DescribeKey"]
    resources = [aws_kms_key.main.arn]
  }
}

resource "aws_iam_role_policy" "task_api" {
  name   = "api"
  role   = aws_iam_role.task["api"].id
  policy = data.aws_iam_policy_document.task_api.json
}

# worker: read/write uploads, consume jobs queue + DLQ, use CMK.
data "aws_iam_policy_document" "task_worker" {
  statement {
    sid       = "S3Objects"
    effect    = "Allow"
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["${aws_s3_bucket.uploads.arn}/*"]
  }
  statement {
    sid       = "SQSConsume"
    effect    = "Allow"
    actions   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes", "sqs:GetQueueUrl"]
    resources = [aws_sqs_queue.jobs.arn, aws_sqs_queue.jobs_dlq.arn]
  }
  statement {
    sid       = "KMS"
    effect    = "Allow"
    actions   = ["kms:Encrypt", "kms:Decrypt", "kms:GenerateDataKey", "kms:DescribeKey"]
    resources = [aws_kms_key.main.arn]
  }
}

resource "aws_iam_role_policy" "task_worker" {
  name   = "worker"
  role   = aws_iam_role.task["worker"].id
  policy = data.aws_iam_policy_document.task_worker.json
}

# gerbonara: download uploads only, decrypt with CMK.
data "aws_iam_policy_document" "task_gerbonara" {
  statement {
    sid       = "S3Read"
    effect    = "Allow"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.uploads.arn}/*"]
  }
  statement {
    sid       = "KMS"
    effect    = "Allow"
    actions   = ["kms:Decrypt", "kms:DescribeKey"]
    resources = [aws_kms_key.main.arn]
  }
}

resource "aws_iam_role_policy" "task_gerbonara" {
  name   = "gerbonara"
  role   = aws_iam_role.task["gerbonara"].id
  policy = data.aws_iam_policy_document.task_gerbonara.json
}

# web has no direct AWS access (it talks to the api over HTTP); its task role
# exists for consistency and future use, with no attached policy.

# ---------------------------------------------------------------------------
# Deploy permissions for the GitHub OIDC role (created trust-only in WS0).
# ---------------------------------------------------------------------------
data "aws_iam_policy_document" "deploy" {
  statement {
    sid       = "ECRAuth"
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
  statement {
    sid    = "ECRPush"
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
      "ecr:PutImage",
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
    ]
    resources = [for r in aws_ecr_repository.service : r.arn]
  }
  # RegisterTaskDefinition does not support resource-level permissions.
  statement {
    sid       = "ECSRegister"
    effect    = "Allow"
    actions   = ["ecs:RegisterTaskDefinition", "ecs:DeregisterTaskDefinition", "ecs:DescribeTaskDefinition"]
    resources = ["*"]
  }
  statement {
    sid       = "ECSDeploy"
    effect    = "Allow"
    actions   = ["ecs:UpdateService", "ecs:DescribeServices"]
    resources = ["*"]
  }
  statement {
    sid       = "PassTaskRoles"
    effect    = "Allow"
    actions   = ["iam:PassRole"]
    resources = concat([aws_iam_role.execution.arn], [for r in aws_iam_role.task : r.arn])

    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "deploy" {
  count  = local.enable_github_oidc ? 1 : 0
  name   = "deploy"
  role   = aws_iam_role.github_deploy[0].id
  policy = data.aws_iam_policy_document.deploy.json
}
