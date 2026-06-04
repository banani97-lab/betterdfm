resource "aws_lb" "main" {
  name               = "${var.name_prefix}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  tags = { Name = "${var.name_prefix}-alb" }
}

resource "aws_lb_target_group" "web" {
  name        = "${var.name_prefix}-web"
  port        = local.container_port.web
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    path                = "/"
    matcher             = "200-399"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 5
  }

  tags = { Name = "${var.name_prefix}-web" }
}

resource "aws_lb_target_group" "api" {
  name        = "${var.name_prefix}-api"
  port        = local.container_port.api
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    path                = "/health"
    matcher             = "200"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 5
  }

  tags = { Name = "${var.name_prefix}-api" }
}

# ---------------------------------------------------------------------------
# Custom-domain path: ACM cert (DNS-validated) + HTTPS host-based routing.
# ---------------------------------------------------------------------------
resource "aws_acm_certificate" "main" {
  count = local.use_custom_domain ? 1 : 0

  domain_name               = local.app_fqdn
  subject_alternative_names = [local.api_fqdn]
  validation_method         = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.name_prefix}-cert" }
}

resource "aws_lb_listener" "https" {
  count = local.use_custom_domain ? 1 : 0

  load_balancer_arn = aws_lb.main.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate.main[0].arn

  # Default to web; api split out by host below.
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.web.arn
  }
}

resource "aws_lb_listener_rule" "api_host" {
  count = local.use_custom_domain ? 1 : 0

  listener_arn = aws_lb_listener.https[0].arn
  priority     = 10

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }

  condition {
    host_header {
      values = [local.api_fqdn]
    }
  }
}

# HTTP -> HTTPS redirect when a domain is set.
resource "aws_lb_listener" "http_redirect" {
  count = local.use_custom_domain ? 1 : 0

  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

# ---------------------------------------------------------------------------
# No-domain alpha path: web on :80, api on :8080 (plain HTTP).
# ---------------------------------------------------------------------------
resource "aws_lb_listener" "http_web" {
  count = local.use_custom_domain ? 0 : 1

  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.web.arn
  }
}

resource "aws_lb_listener" "http_api" {
  count = local.use_custom_domain ? 0 : 1

  load_balancer_arn = aws_lb.main.arn
  port              = 8080
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }
}

locals {
  # Base URLs the app uses, depending on whether a domain is configured.
  app_url = local.use_custom_domain ? "https://${local.app_fqdn}" : "http://${aws_lb.main.dns_name}"
  api_url = local.use_custom_domain ? "https://${local.api_fqdn}" : "http://${aws_lb.main.dns_name}:8080"
}
