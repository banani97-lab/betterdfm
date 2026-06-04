output "alb_dns_name" {
  description = "ALB DNS name. In no-domain mode: web on http://<dns>, api on http://<dns>:8080."
  value       = aws_lb.main.dns_name
}

output "app_url" {
  value = local.app_url
}

output "api_url" {
  description = "Base API URL for the browser (set as NEXT_PUBLIC_API_URL build arg in CI)."
  value       = local.api_url
}

output "ecr_repository_urls" {
  description = "ECR repo URLs per service (CI pushes here)."
  value       = { for s, r in aws_ecr_repository.service : s => r.repository_url }
}

output "ecs_cluster" {
  value = aws_ecs_cluster.main.name
}

output "uploads_bucket" {
  value = aws_s3_bucket.uploads.id
}

output "sqs_queue_url" {
  value = aws_sqs_queue.jobs.url
}

output "jwt_issuer" {
  description = "Cognito issuer URL for the API JWT_ISSUER env."
  value       = local.jwt_issuer
}

output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.main.id
}

output "cognito_app_client_id" {
  value = aws_cognito_user_pool_client.app.id
}

output "cognito_admin_client_id" {
  value = aws_cognito_user_pool_client.admin.id
}

output "cognito_hosted_domain" {
  value = aws_cognito_user_pool_domain.main.domain
}

output "acm_certificate_validation" {
  description = "DNS records to create to validate the ACM cert (only when domain_name is set)."
  value       = local.use_custom_domain ? aws_acm_certificate.main[0].domain_validation_options : null
}
