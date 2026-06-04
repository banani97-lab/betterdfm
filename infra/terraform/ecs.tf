resource "aws_ecs_cluster" "main" {
  name = var.name_prefix

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = { Name = var.name_prefix }
}

# Internal service discovery so worker -> gerbonara and web -> api resolve by
# DNS inside the VPC (no public hop).
resource "aws_service_discovery_private_dns_namespace" "main" {
  name = "${var.name_prefix}.local"
  vpc  = aws_vpc.main.id
}

resource "aws_service_discovery_service" "internal" {
  for_each = toset(["api", "gerbonara"])

  name = each.key

  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.main.id

    dns_records {
      type = "A"
      ttl  = 10
    }

    routing_policy = "MULTIVALUE"
  }

  health_check_custom_config {
    failure_threshold = 1
  }
}

# CMK-encrypted log group per service (names match the CMK key-policy prefix).
resource "aws_cloudwatch_log_group" "service" {
  for_each = toset(local.services)

  name              = "/${var.name_prefix}/${each.key}"
  retention_in_days = var.log_retention_days
  kms_key_id        = aws_kms_key.main.arn
}

locals {
  internal_api_url = "http://api.${var.name_prefix}.local:8080"
  gerbonara_url    = "http://gerbonara.${var.name_prefix}.local:8001"

  image = { for s in local.services : s => "${aws_ecr_repository.service[s].repository_url}:${var.image_tag}" }

  # Shared env for services that talk to AWS.
  aws_env = [
    { name = "AWS_REGION", value = var.region },
    { name = "S3_BUCKET", value = aws_s3_bucket.uploads.id },
    { name = "SQS_QUEUE_URL", value = aws_sqs_queue.jobs.url },
  ]

  db_secret = [
    { name = "DATABASE_URL", valueFrom = aws_secretsmanager_secret.database_url.arn },
  ]
}

# Helper to build a standard log configuration block per service.
locals {
  log_config = { for s in local.services : s => {
    logDriver = "awslogs"
    options = {
      "awslogs-group"         = aws_cloudwatch_log_group.service[s].name
      "awslogs-region"        = var.region
      "awslogs-stream-prefix" = s
    }
  } }
}

# ---------------------------------------------------------------------------
# Task definitions
# ---------------------------------------------------------------------------
resource "aws_ecs_task_definition" "web" {
  family                   = "${var.name_prefix}-web"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.service_cpu["web"]
  memory                   = var.service_memory["web"]
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task["web"].arn

  container_definitions = jsonencode([{
    name      = "web"
    image     = local.image["web"]
    essential = true
    portMappings = [{
      containerPort = local.container_port.web
      protocol      = "tcp"
    }]
    # Server-side calls in the web app reach the api internally. NEXT_PUBLIC_*
    # (browser) vars are baked at docker build time in CI, not here.
    environment = [
      { name = "INTERNAL_API_URL", value = local.internal_api_url },
    ]
    logConfiguration = local.log_config["web"]
  }])
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${var.name_prefix}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.service_cpu["api"]
  memory                   = var.service_memory["api"]
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task["api"].arn

  container_definitions = jsonencode([{
    name      = "api"
    image     = local.image["api"]
    essential = true
    portMappings = [{
      containerPort = local.container_port.api
      protocol      = "tcp"
    }]
    environment = concat(local.aws_env, [
      { name = "JWT_ISSUER", value = local.jwt_issuer },
      { name = "COGNITO_USER_POOL_ID", value = aws_cognito_user_pool.main.id },
      { name = "COGNITO_CLIENT_ID", value = aws_cognito_user_pool_client.app.id },
      { name = "ADMIN_COGNITO_CLIENT_ID", value = aws_cognito_user_pool_client.admin.id },
      { name = "NON_CUI_ALPHA_MODE", value = "true" },
    ])
    secrets          = local.db_secret
    logConfiguration = local.log_config["api"]
  }])
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.name_prefix}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.service_cpu["worker"]
  memory                   = var.service_memory["worker"]
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task["worker"].arn

  container_definitions = jsonencode([{
    name      = "worker"
    image     = local.image["worker"]
    essential = true
    environment = concat(local.aws_env, [
      { name = "GERBONARA_URL", value = local.gerbonara_url },
    ])
    secrets          = local.db_secret
    logConfiguration = local.log_config["worker"]
  }])
}

resource "aws_ecs_task_definition" "gerbonara" {
  family                   = "${var.name_prefix}-gerbonara"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.service_cpu["gerbonara"]
  memory                   = var.service_memory["gerbonara"]
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task["gerbonara"].arn

  container_definitions = jsonencode([{
    name      = "gerbonara"
    image     = local.image["gerbonara"]
    essential = true
    portMappings = [{
      containerPort = local.container_port.gerbonara
      protocol      = "tcp"
    }]
    environment = [
      { name = "AWS_REGION", value = var.region },
      { name = "S3_BUCKET", value = aws_s3_bucket.uploads.id },
    ]
    logConfiguration = local.log_config["gerbonara"]
  }])
}

# ---------------------------------------------------------------------------
# Services. desired_count + task_definition are ignored so CI-driven deploys
# (register new task def, update service) do not fight Terraform.
# ---------------------------------------------------------------------------
resource "aws_ecs_service" "web" {
  name            = "${var.name_prefix}-web"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.web.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.web.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.web.arn
    container_name   = "web"
    container_port   = local.container_port.web
  }

  lifecycle {
    ignore_changes = [task_definition, desired_count]
  }

  depends_on = [aws_lb_listener.http_web, aws_lb_listener.https]
}

resource "aws_ecs_service" "api" {
  name            = "${var.name_prefix}-api"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.api.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = local.container_port.api
  }

  service_registries {
    registry_arn = aws_service_discovery_service.internal["api"].arn
  }

  lifecycle {
    ignore_changes = [task_definition, desired_count]
  }

  depends_on = [aws_lb_listener.http_api, aws_lb_listener.https]
}

resource "aws_ecs_service" "worker" {
  name            = "${var.name_prefix}-worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.worker.id]
    assign_public_ip = false
  }

  lifecycle {
    ignore_changes = [task_definition, desired_count]
  }
}

resource "aws_ecs_service" "gerbonara" {
  name            = "${var.name_prefix}-gerbonara"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.gerbonara.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.gerbonara.id]
    assign_public_ip = false
  }

  service_registries {
    registry_arn = aws_service_discovery_service.internal["gerbonara"].arn
  }

  lifecycle {
    ignore_changes = [task_definition, desired_count]
  }
}
