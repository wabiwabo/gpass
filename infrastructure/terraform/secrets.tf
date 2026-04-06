################################################################################
# KMS Key for Secrets
################################################################################

resource "aws_kms_key" "secrets" {
  description             = "KMS key for Secrets Manager encryption"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = {
    Name = "${local.name_prefix}-secrets-kms"
  }
}

resource "aws_kms_alias" "secrets" {
  name          = "alias/${local.name_prefix}-secrets"
  target_key_id = aws_kms_key.secrets.key_id
}

################################################################################
# Database Credentials
################################################################################

resource "random_password" "db_password" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "aws_secretsmanager_secret" "db_password" {
  name       = "${local.name_prefix}/database/password"
  kms_key_id = aws_kms_key.secrets.arn

  tags = {
    Name = "${local.name_prefix}-db-password"
  }
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = random_password.db_password.result
}

resource "aws_secretsmanager_secret" "db_connection" {
  name       = "${local.name_prefix}/database/connection"
  kms_key_id = aws_kms_key.secrets.arn

  tags = {
    Name = "${local.name_prefix}-db-connection"
  }
}

resource "aws_secretsmanager_secret_version" "db_connection" {
  secret_id = aws_secretsmanager_secret.db_connection.id
  secret_string = jsonencode({
    host     = aws_db_instance.main.address
    port     = aws_db_instance.main.port
    dbname   = var.db_name
    username = "garudapass_admin"
    password = random_password.db_password.result
    engine   = "postgres"
  })
}

################################################################################
# Redis Auth Token
################################################################################

resource "random_password" "redis_auth_token" {
  length           = 64
  special          = false # Redis auth tokens do not support all special chars
}

resource "aws_secretsmanager_secret" "redis_auth_token" {
  name       = "${local.name_prefix}/redis/auth-token"
  kms_key_id = aws_kms_key.secrets.arn

  tags = {
    Name = "${local.name_prefix}-redis-auth-token"
  }
}

resource "aws_secretsmanager_secret_version" "redis_auth_token" {
  secret_id     = aws_secretsmanager_secret.redis_auth_token.id
  secret_string = random_password.redis_auth_token.result
}

################################################################################
# NIK Encryption Key
################################################################################

resource "random_password" "nik_encryption_key" {
  length  = 64
  special = false
}

resource "aws_secretsmanager_secret" "nik_encryption_key" {
  name       = "${local.name_prefix}/nik/encryption-key"
  kms_key_id = aws_kms_key.secrets.arn

  tags = {
    Name = "${local.name_prefix}-nik-encryption-key"
  }
}

resource "aws_secretsmanager_secret_version" "nik_encryption_key" {
  secret_id     = aws_secretsmanager_secret.nik_encryption_key.id
  secret_string = random_password.nik_encryption_key.result
}

################################################################################
# BFF Session Secret
################################################################################

resource "random_password" "session_secret" {
  length  = 64
  special = false
}

resource "aws_secretsmanager_secret" "session_secret" {
  name       = "${local.name_prefix}/bff/session-secret"
  kms_key_id = aws_kms_key.secrets.arn

  tags = {
    Name = "${local.name_prefix}-session-secret"
  }
}

resource "aws_secretsmanager_secret_version" "session_secret" {
  secret_id     = aws_secretsmanager_secret.session_secret.id
  secret_string = random_password.session_secret.result
}

################################################################################
# API Signing Keys
################################################################################

resource "random_password" "api_signing_key" {
  length  = 64
  special = false
}

resource "aws_secretsmanager_secret" "api_signing_key" {
  name       = "${local.name_prefix}/api/signing-key"
  kms_key_id = aws_kms_key.secrets.arn

  tags = {
    Name = "${local.name_prefix}-api-signing-key"
  }
}

resource "aws_secretsmanager_secret_version" "api_signing_key" {
  secret_id     = aws_secretsmanager_secret.api_signing_key.id
  secret_string = random_password.api_signing_key.result
}

################################################################################
# Keycloak Admin Credentials
################################################################################

resource "random_password" "keycloak_admin_password" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "aws_secretsmanager_secret" "keycloak_admin" {
  name       = "${local.name_prefix}/keycloak/admin"
  kms_key_id = aws_kms_key.secrets.arn

  tags = {
    Name = "${local.name_prefix}-keycloak-admin"
  }
}

resource "aws_secretsmanager_secret_version" "keycloak_admin" {
  secret_id = aws_secretsmanager_secret.keycloak_admin.id
  secret_string = jsonencode({
    username = "admin"
    password = random_password.keycloak_admin_password.result
  })
}

################################################################################
# IRSA Role for Secrets Access
################################################################################

resource "aws_iam_role" "secrets_access" {
  name = "${local.name_prefix}-secrets-access"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = aws_iam_openid_connect_provider.eks.arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringLike = {
            "${replace(aws_eks_cluster.main.identity[0].oidc[0].issuer, "https://", "")}:sub" = "system:serviceaccount:garudapass:*"
            "${replace(aws_eks_cluster.main.identity[0].oidc[0].issuer, "https://", "")}:aud" = "sts.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = {
    Name = "${local.name_prefix}-secrets-access-role"
  }
}

resource "aws_iam_role_policy" "secrets_access" {
  name = "${local.name_prefix}-secrets-access-policy"
  role = aws_iam_role.secrets_access.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret",
        ]
        Resource = "arn:aws:secretsmanager:${var.aws_region}:${data.aws_caller_identity.current.account_id}:secret:${local.name_prefix}/*"
      },
      {
        Effect = "Allow"
        Action = [
          "kms:Decrypt",
        ]
        Resource = aws_kms_key.secrets.arn
      }
    ]
  })
}
