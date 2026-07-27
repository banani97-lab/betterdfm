# ---------------------------------------------------------------------------
# WS2 application-tier inputs (ECS/Fargate, RDS, S3, SQS, Cognito, ALB).
# ---------------------------------------------------------------------------

variable "domain_name" {
  description = "Optional base domain for the deployment. Empty = serve over the ALB DNS name on HTTP. When set, app.<domain> -> web and api.<domain> -> api over HTTPS (ACM), and Cognito callbacks use it."
  type        = string
  default     = ""
}

variable "image_tag" {
  description = "Container image tag the ECS task definitions reference initially. CI updates task defs thereafter."
  type        = string
  default     = "latest"
}

variable "db_engine_version" {
  description = "RDS Postgres engine version."
  type        = string
  default     = "16"
}

variable "db_instance_class" {
  description = "RDS instance class (single-AZ is fine for alpha)."
  type        = string
  default     = "db.t3.medium"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage in GB."
  type        = number
  default     = 50
}

variable "db_multi_az" {
  description = "Run RDS multi-AZ (HA). Off for alpha to save cost."
  type        = bool
  default     = false
}

variable "service_cpu" {
  description = "Fargate task CPU units per service."
  type        = map(number)
  default = {
    web       = 512
    api       = 512
    worker    = 1024 # 1 vCPU: engine run over large boards
    gerbonara = 2048 # 2 vCPU: parsing large (200MB+) ODB++ archives
  }
}

variable "service_memory" {
  description = "Fargate task memory (MiB) per service."
  type        = map(number)
  default = {
    web       = 1024
    api       = 1024
    worker    = 4096 # holds full board JSON (unmarshal + re-marshal) + spatial grids
    gerbonara = 8192 # peak parse memory for 200MB+ ODB++ (verbose text -> Python objects)
  }
}

locals {
  use_custom_domain = var.domain_name != ""
  app_fqdn          = local.use_custom_domain ? "app.${var.domain_name}" : ""
  api_fqdn          = local.use_custom_domain ? "api.${var.domain_name}" : ""

  services = ["web", "api", "worker", "gerbonara"]
}
