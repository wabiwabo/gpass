################################################################################
# VPC
################################################################################

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "IDs of the private subnets"
  value       = aws_subnet.private[*].id
}

output "database_subnet_ids" {
  description = "IDs of the database subnets"
  value       = aws_subnet.database[*].id
}

################################################################################
# EKS
################################################################################

output "eks_cluster_name" {
  description = "Name of the EKS cluster"
  value       = aws_eks_cluster.main.name
}

output "eks_cluster_endpoint" {
  description = "Endpoint for the EKS cluster API server"
  value       = aws_eks_cluster.main.endpoint
}

output "eks_cluster_certificate_authority" {
  description = "Base64 encoded certificate data for the EKS cluster"
  value       = aws_eks_cluster.main.certificate_authority[0].data
  sensitive   = true
}

output "eks_oidc_provider_arn" {
  description = "ARN of the OIDC provider for IRSA"
  value       = aws_iam_openid_connect_provider.eks.arn
}

output "eks_cluster_security_group_id" {
  description = "Security group ID of the EKS cluster"
  value       = aws_security_group.eks_cluster.id
}

output "cluster_autoscaler_role_arn" {
  description = "IAM role ARN for the cluster autoscaler"
  value       = aws_iam_role.cluster_autoscaler.arn
}

################################################################################
# RDS
################################################################################

output "rds_endpoint" {
  description = "Endpoint of the RDS PostgreSQL instance"
  value       = aws_db_instance.main.endpoint
}

output "rds_address" {
  description = "Hostname of the RDS PostgreSQL instance"
  value       = aws_db_instance.main.address
}

output "rds_port" {
  description = "Port of the RDS PostgreSQL instance"
  value       = aws_db_instance.main.port
}

output "rds_database_name" {
  description = "Name of the default database"
  value       = aws_db_instance.main.db_name
}

################################################################################
# ElastiCache Redis
################################################################################

output "redis_endpoint" {
  description = "Primary endpoint of the Redis replication group"
  value       = aws_elasticache_replication_group.main.primary_endpoint_address
}

output "redis_port" {
  description = "Port of the Redis cluster"
  value       = aws_elasticache_replication_group.main.port
}

################################################################################
# ECR
################################################################################

output "ecr_repository_urls" {
  description = "Map of service name to ECR repository URL"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}

################################################################################
# Secrets
################################################################################

output "db_secret_arn" {
  description = "ARN of the database connection secret"
  value       = aws_secretsmanager_secret.db_connection.arn
}

output "redis_auth_secret_arn" {
  description = "ARN of the Redis auth token secret"
  value       = aws_secretsmanager_secret.redis_auth_token.arn
}

output "secrets_access_role_arn" {
  description = "IAM role ARN for accessing secrets from EKS pods"
  value       = aws_iam_role.secrets_access.arn
}

################################################################################
# Monitoring
################################################################################

output "sns_alerts_topic_arn" {
  description = "ARN of the SNS topic for alerts"
  value       = aws_sns_topic.alerts.arn
}

output "cloudwatch_dashboard_url" {
  description = "URL of the CloudWatch dashboard"
  value       = "https://${var.aws_region}.console.aws.amazon.com/cloudwatch/home?region=${var.aws_region}#dashboards:name=${aws_cloudwatch_dashboard.main.dashboard_name}"
}
