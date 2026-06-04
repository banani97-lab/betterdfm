output "region" {
  description = "GovCloud region these resources live in."
  value       = var.region
}

output "partition" {
  description = "AWS partition (aws-us-gov in GovCloud)."
  value       = data.aws_partition.current.partition
}

output "account_id" {
  description = "AWS account ID resolved from the apply credentials."
  value       = data.aws_caller_identity.current.account_id
}

output "vpc_id" {
  value = aws_vpc.main.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  value = aws_subnet.private[*].id
}

output "kms_key_arn" {
  description = "CMK ARN for S3/RDS/Logs/SQS encryption (consumed by WS2/WS4)."
  value       = aws_kms_key.main.arn
}

output "kms_alias" {
  value = aws_kms_alias.main.name
}

output "github_oidc_provider_arn" {
  value = local.enable_github_oidc ? aws_iam_openid_connect_provider.github[0].arn : null
}

output "github_deploy_role_arn" {
  description = "Role ARN the deploy workflow assumes via OIDC (null when github_org is unset)."
  value       = local.enable_github_oidc ? aws_iam_role.github_deploy[0].arn : null
}
