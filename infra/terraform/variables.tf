variable "region" {
  description = "GovCloud region. Must be us-gov-*; the application auto-enables FIPS endpoints there."
  type        = string
  default     = "us-gov-west-1"

  validation {
    condition     = startswith(var.region, "us-gov-")
    error_message = "region must be a GovCloud region (us-gov-*)."
  }
}

variable "name_prefix" {
  description = "Prefix applied to all resource names."
  type        = string
  default     = "rapiddfm"
}

variable "environment" {
  description = "Environment label used in tags."
  type        = string
  default     = "gov"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
  default     = "10.60.0.0/16"
}

variable "az_count" {
  description = "Number of availability zones to spread subnets across."
  type        = number
  default     = 2

  validation {
    condition     = var.az_count >= 2 && var.az_count <= 3
    error_message = "az_count must be 2 or 3."
  }
}

variable "single_nat_gateway" {
  description = "Use one shared NAT gateway (cheaper, fine for alpha) instead of one per AZ (HA)."
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "Retention for CloudWatch log groups created here (audit / AU controls)."
  type        = number
  default     = 365
}

variable "github_org" {
  description = "GitHub repo OWNER (personal username or org login) allowed to assume the deploy role. Empty = skip GitHub OIDC / the CI deploy role entirely (fine for manual deploys)."
  type        = string
  default     = ""
}

variable "github_repo" {
  description = "GitHub repository name (only used when github_org is set)."
  type        = string
  default     = "betterdfm"
}

variable "github_deploy_branch" {
  description = "Branch whose workflow runs may assume the deploy role."
  type        = string
  default     = "main"
}

variable "tags" {
  description = "Additional tags merged into the provider default tags."
  type        = map(string)
  default     = {}
}
