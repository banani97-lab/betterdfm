locals {
  container_port = {
    web       = 3000
    api       = 8080
    gerbonara = 8001
  }
}

# Public ALB: the only internet-facing component.
resource "aws_security_group" "alb" {
  name        = "${var.name_prefix}-alb"
  description = "Public ALB ingress"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HTTP: redirects to HTTPS when a domain is set; serves the web app directly
  # in the no-domain alpha case.
  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Used only in the no-domain alpha case to expose the api (browser calls it
  # directly via NEXT_PUBLIC_API_URL). Unused once a domain enables host routing.
  ingress {
    description = "api (no-domain alpha)"
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.name_prefix}-alb" }
}

# web + api accept traffic only from the ALB.
resource "aws_security_group" "web" {
  name        = "${var.name_prefix}-web"
  description = "web service tasks"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "from ALB"
    from_port       = local.container_port.web
    to_port         = local.container_port.web
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.name_prefix}-web" }
}

resource "aws_security_group" "api" {
  name        = "${var.name_prefix}-api"
  description = "api service tasks"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "from ALB"
    from_port       = local.container_port.api
    to_port         = local.container_port.api
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.name_prefix}-api" }
}

# worker has no inbound; it polls SQS and calls gerbonara.
resource "aws_security_group" "worker" {
  name        = "${var.name_prefix}-worker"
  description = "worker service tasks"
  vpc_id      = aws_vpc.main.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.name_prefix}-worker" }
}

# gerbonara is internal: only the worker calls it.
resource "aws_security_group" "gerbonara" {
  name        = "${var.name_prefix}-gerbonara"
  description = "gerbonara sidecar tasks"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "from worker"
    from_port       = local.container_port.gerbonara
    to_port         = local.container_port.gerbonara
    protocol        = "tcp"
    security_groups = [aws_security_group.worker.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.name_prefix}-gerbonara" }
}

# RDS: reachable only from api + worker.
resource "aws_security_group" "rds" {
  name        = "${var.name_prefix}-rds"
  description = "RDS Postgres"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Postgres from api + worker"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.api.id, aws_security_group.worker.id]
  }

  tags = { Name = "${var.name_prefix}-rds" }
}
