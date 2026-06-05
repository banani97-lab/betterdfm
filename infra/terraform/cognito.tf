# Cognito user pool. Invite-only (no self-signup; US-person provisioning happens
# admin-side), with the custom orgId/role attributes the API JWT middleware reads
# (apps/api/src/lib/auth.go).
#
# TEMPORARY (non-CUI alpha): mfa_configuration is OPTIONAL, not ON. The frontend
# sign-in flow does not yet implement TOTP enrollment (it only handles
# NEW_PASSWORD_REQUIRED), so mandatory MFA would stall login at MFA_SETUP. This
# MUST return to "ON" before the environment handles anything real or is
# assessed, once TOTP enrollment is built into the sign-in flow. Tracked in
# docs/ITAR-GOVCLOUD-MIGRATION.md. TOTP remains available (software token) so
# users can opt in now. (IA-2)
resource "aws_cognito_user_pool" "main" {
  name = "${var.name_prefix}-users"

  mfa_configuration = "OPTIONAL"

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

  generate_secret = false
  # USER_PASSWORD_AUTH is what the app's /api/auth/signin routes use (InitiateAuth).
  explicit_auth_flows                  = ["ALLOW_USER_PASSWORD_AUTH", "ALLOW_USER_SRP_AUTH", "ALLOW_REFRESH_TOKEN_AUTH"]
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

  generate_secret = false
  # USER_PASSWORD_AUTH is what the app's /api/auth/signin routes use (InitiateAuth).
  explicit_auth_flows                  = ["ALLOW_USER_PASSWORD_AUTH", "ALLOW_USER_SRP_AUTH", "ALLOW_REFRESH_TOKEN_AUTH"]
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
