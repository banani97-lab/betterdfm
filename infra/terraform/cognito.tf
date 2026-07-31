# Cognito user pool. Invite-only (no self-signup; US-person provisioning happens
# admin-side), with the custom orgId/role attributes the API JWT middleware reads
# (apps/api/src/lib/auth.go).
#
# MFA is enforced pool-wide (mfa_configuration = ON) using TOTP (software
# token). The app and admin sign-in flows implement the full MFA_SETUP +
# SOFTWARE_TOKEN_MFA challenge handling, so an invited user enrolls an
# authenticator on first login. Verified end-to-end on both flows. NIST
# 800-171 IA-2. (Applied live via set-user-pool-mfa-config; this keeps
# terraform in sync.)
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

    # Invitation email sent by AdminCreateUser. {username} and {####} (the
    # temporary password) are required placeholders. Spells out the three steps
    # so an invited user knows what the account is and how to get in.
    invite_message_template {
      email_subject = "Your RapidDFM account is ready"
      email_message = <<-EOT
        <p>You've been given access to <strong>RapidDFM</strong>, the PCB design-for-manufacturability analysis platform.</p>
        <p><strong>To get started:</strong></p>
        <ol>
          <li>Go to <a href="https://app.gov.rapiddfm.com/login">app.gov.rapiddfm.com/login</a></li>
          <li>Sign in with your email and this temporary password: <strong>{####}</strong></li>
          <li>You'll be prompted to create your own password.</li>
        </ol>
        <p>Your sign-in username is <strong>{username}</strong>.</p>
        <p>If you weren't expecting this invitation, you can safely ignore this email.</p>
      EOT
    }
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
