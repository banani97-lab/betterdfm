# Cognito user pool. MFA mandatory (IA-2), invite-only (no self-signup; US-person
# provisioning happens admin-side), with the custom orgId/role attributes the API
# JWT middleware reads (apps/api/src/lib/auth.go).
resource "aws_cognito_user_pool" "main" {
  name = "${var.name_prefix}-users"

  mfa_configuration = "ON"

  software_token_mfa_configuration {
    enabled = true
  }

  password_policy {
    minimum_length    = 14
    require_lowercase = true
    require_uppercase = true
    require_numbers   = true
    require_symbols   = true
  }

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  schema {
    name                     = "orgId"
    attribute_data_type      = "String"
    mutable                  = true
    developer_only_attribute = false

    string_attribute_constraints {
      min_length = 1
      max_length = 64
    }
  }

  schema {
    name                     = "role"
    attribute_data_type      = "String"
    mutable                  = true
    developer_only_attribute = false

    string_attribute_constraints {
      min_length = 1
      max_length = 16
    }
  }

  tags = { Name = "${var.name_prefix}-users" }
}

# Hosted-UI domain (prefix-based; a custom auth.<domain> can be added later).
resource "aws_cognito_user_pool_domain" "main" {
  domain       = "${var.name_prefix}-${data.aws_caller_identity.current.account_id}"
  user_pool_id = aws_cognito_user_pool.main.id
}

# App users client (audience for the API's app JWT middleware).
resource "aws_cognito_user_pool_client" "app" {
  name         = "${var.name_prefix}-app"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret                      = false
  explicit_auth_flows                  = ["ALLOW_USER_SRP_AUTH", "ALLOW_REFRESH_TOKEN_AUTH"]
  supported_identity_providers         = ["COGNITO"]
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_scopes                 = ["openid", "email", "profile"]

  callback_urls = local.use_custom_domain ? ["https://${local.app_fqdn}/auth/callback"] : ["http://localhost:3000/auth/callback"]
  logout_urls   = local.use_custom_domain ? ["https://${local.app_fqdn}"] : ["http://localhost:3000"]
}

# Admin client (separate audience for the API's admin JWT middleware).
resource "aws_cognito_user_pool_client" "admin" {
  name         = "${var.name_prefix}-admin"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret                      = false
  explicit_auth_flows                  = ["ALLOW_USER_SRP_AUTH", "ALLOW_REFRESH_TOKEN_AUTH"]
  supported_identity_providers         = ["COGNITO"]
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_scopes                 = ["openid", "email", "profile"]

  callback_urls = local.use_custom_domain ? ["https://${local.app_fqdn}/auth/callback"] : ["http://localhost:3000/auth/callback"]
  logout_urls   = local.use_custom_domain ? ["https://${local.app_fqdn}"] : ["http://localhost:3000"]
}

locals {
  jwt_issuer = "https://cognito-idp.${var.region}.amazonaws.com/${aws_cognito_user_pool.main.id}"
}
