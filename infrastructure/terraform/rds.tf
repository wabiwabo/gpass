################################################################################
# RDS KMS Key
################################################################################

resource "aws_kms_key" "rds" {
  description             = "KMS key for RDS encryption at rest"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = {
    Name = "${local.name_prefix}-rds-kms"
  }
}

resource "aws_kms_alias" "rds" {
  name          = "alias/${local.name_prefix}-rds"
  target_key_id = aws_kms_key.rds.key_id
}

################################################################################
# RDS Security Group
################################################################################

resource "aws_security_group" "rds" {
  name_prefix = "${local.name_prefix}-rds-"
  description = "Security group for RDS PostgreSQL"
  vpc_id      = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-rds-sg"
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_security_group_rule" "rds_ingress_eks" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.eks_cluster.id
  security_group_id        = aws_security_group.rds.id
  description              = "Allow PostgreSQL from EKS nodes"
}

resource "aws_security_group_rule" "rds_ingress_private_subnets" {
  count             = length(local.azs)
  type              = "ingress"
  from_port         = 5432
  to_port           = 5432
  protocol          = "tcp"
  cidr_blocks       = [cidrsubnet(var.vpc_cidr, 8, count.index + 10)]
  security_group_id = aws_security_group.rds.id
  description       = "Allow PostgreSQL from private subnet ${count.index}"
}

################################################################################
# RDS Parameter Group
################################################################################

resource "aws_db_parameter_group" "main" {
  name   = "${local.name_prefix}-pg16"
  family = "postgres16"

  parameter {
    name  = "log_connections"
    value = "1"
  }

  parameter {
    name  = "log_disconnections"
    value = "1"
  }

  parameter {
    name  = "log_duration"
    value = "1"
  }

  parameter {
    name  = "log_min_duration_statement"
    value = "1000"
  }

  parameter {
    name  = "log_statement"
    value = "ddl"
  }

  parameter {
    name         = "shared_preload_libraries"
    value        = "pg_stat_statements"
    apply_method = "pending-reboot"
  }

  parameter {
    name  = "pg_stat_statements.track"
    value = "all"
  }

  tags = {
    Name = "${local.name_prefix}-pg-params"
  }
}

################################################################################
# RDS Instance
################################################################################

resource "aws_db_instance" "main" {
  identifier = "${local.name_prefix}-postgres"

  engine         = "postgres"
  engine_version = var.db_engine_version
  instance_class = var.db_instance_class

  allocated_storage     = var.db_allocated_storage
  max_allocated_storage = var.db_max_allocated_storage
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id            = aws_kms_key.rds.arn

  db_name  = var.db_name
  username = "garudapass_admin"
  password = aws_secretsmanager_secret_version.db_password.secret_string
  port     = 5432

  multi_az               = var.db_multi_az
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  parameter_group_name   = aws_db_parameter_group.main.name
  publicly_accessible    = false

  backup_retention_period = var.db_backup_retention_period
  backup_window           = "03:00-04:00"
  maintenance_window      = "sun:04:00-sun:05:00"

  deletion_protection       = var.db_deletion_protection
  skip_final_snapshot       = var.environment == "dev"
  final_snapshot_identifier = var.environment != "dev" ? "${local.name_prefix}-final-snapshot" : null
  copy_tags_to_snapshot     = true

  performance_insights_enabled          = true
  performance_insights_retention_period = var.environment == "prod" ? 731 : 7
  performance_insights_kms_key_id       = aws_kms_key.rds.arn

  monitoring_interval = 60
  monitoring_role_arn = aws_iam_role.rds_monitoring.arn

  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  tags = {
    Name = "${local.name_prefix}-postgres"
  }
}

################################################################################
# RDS Enhanced Monitoring IAM Role
################################################################################

resource "aws_iam_role" "rds_monitoring" {
  name = "${local.name_prefix}-rds-monitoring-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "monitoring.rds.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.name_prefix}-rds-monitoring-role"
  }
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
  role       = aws_iam_role.rds_monitoring.name
}
