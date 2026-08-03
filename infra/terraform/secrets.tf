# DB credentials. The password is generated here and the full libpq DSN is
# stored in Secrets Manager (CMK-encrypted), injected into api/worker tasks as
# DATABASE_URL. sslmode=require enforces TLS from the client side too.
#
# NOTE: the generated password is present in Terraform state. State lives in the
# encrypted, access-controlled GovCloud backend. A later hardening step can move
# to an RDS-managed master secret with DSN assembly in the container entrypoint
# to keep the password out of state entirely.
resource "random_password" "db" {
  length = 32
  # No special chars: keeps the value safe to embed in the DSN without encoding.
  special = false
}

resource "aws_secretsmanager_secret" "database_url" {
  name       = "${var.name_prefix}/database-url"
  kms_key_id = aws_kms_key.main.arn

  tags = { Name = "${var.name_prefix}-database-url" }
}

resource "aws_secretsmanager_secret_version" "database_url" {
  secret_id     = aws_secretsmanager_secret.database_url.id
  secret_string = "postgres://${local.db_username}:${random_password.db.result}@${aws_db_instance.main.address}:5432/${local.db_name}?sslmode=require"
}

# Self-signed cert + key for internal service-to-service TLS (worker->gerbonara,
# ALB/web->api) so CUI is encrypted in transit inside the VPC (NIST 800-171
# 3.13.8). Value is a {tls_cert, tls_key} JSON, generated out of band (openssl)
# and set via put-secret-value, so the private key is not embedded in state.
resource "aws_secretsmanager_secret" "internal_tls" {
  name       = "${var.name_prefix}-internal-tls"
  kms_key_id = aws_kms_key.main.arn

  tags = { Name = "${var.name_prefix}-internal-tls" }
}
