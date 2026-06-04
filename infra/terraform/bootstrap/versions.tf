terraform {
  # >= 1.10 for S3 native state locking (use_lockfile) in the root module.
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.40"
    }
  }

  # Local state by design: this module CREATES the remote backend that the root
  # module then uses, so it cannot itself use that backend (chicken-and-egg).
  # The local terraform.tfstate here is small and rarely changes; back it up
  # (or run `terraform init -migrate-state` into the bucket it creates later).
}
