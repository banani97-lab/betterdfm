provider "aws" {
  region = var.region

  # Every resource is tagged so the ITAR/CUI authorization boundary is
  # identifiable in inventory and billing.
  default_tags {
    tags = merge(
      {
        Project     = "rapiddfm"
        Environment = var.environment
        Compliance  = "ITAR-CUI"
        ManagedBy   = "terraform"
      },
      var.tags,
    )
  }
}

# Resolved at apply time from the caller's credentials, so the account ID and
# partition (aws-us-gov in GovCloud) never need to be hardcoded.
data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}
data "aws_region" "current" {}
