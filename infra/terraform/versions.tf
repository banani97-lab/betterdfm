terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.40"
    }
  }

  # Partial S3 backend. Concrete values (bucket, dynamodb_table, region, key)
  # are supplied at init time via -backend-config=backend.hcl so they are not
  # committed. The state bucket + lock table live in the GovCloud account and
  # must be encrypted with a US-person-controlled key. See backend.hcl.example.
  backend "s3" {}
}
