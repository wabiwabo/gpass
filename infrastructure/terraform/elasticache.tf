################################################################################
# ElastiCache Security Group
################################################################################

resource "aws_security_group" "redis" {
  name_prefix = "${local.name_prefix}-redis-"
  description = "Security group for ElastiCache Redis"
  vpc_id      = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-redis-sg"
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_security_group_rule" "redis_ingress_eks" {
  type                     = "ingress"
  from_port                = 6379
  to_port                  = 6379
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.eks_cluster.id
  security_group_id        = aws_security_group.redis.id
  description              = "Allow Redis from EKS cluster"
}

################################################################################
# ElastiCache Redis Replication Group
################################################################################

resource "aws_elasticache_replication_group" "main" {
  replication_group_id = "${local.name_prefix}-redis"
  description          = "GarudaPass Redis cluster for sessions and caching"

  engine               = "redis"
  engine_version       = var.redis_engine_version
  node_type            = var.redis_node_type
  num_cache_clusters   = var.redis_num_cache_clusters
  port                 = 6379
  parameter_group_name = aws_elasticache_parameter_group.main.name

  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = aws_secretsmanager_secret_version.redis_auth_token.secret_string

  automatic_failover_enabled = var.redis_num_cache_clusters > 1
  multi_az_enabled           = var.redis_num_cache_clusters > 1

  snapshot_retention_limit = var.environment == "prod" ? 7 : 1
  snapshot_window          = "02:00-03:00"
  maintenance_window       = "sun:03:00-sun:04:00"

  notification_topic_arn = aws_sns_topic.alerts.arn

  tags = {
    Name = "${local.name_prefix}-redis"
  }
}

################################################################################
# ElastiCache Parameter Group
################################################################################

resource "aws_elasticache_parameter_group" "main" {
  name   = "${local.name_prefix}-redis7"
  family = "redis7"

  parameter {
    name  = "maxmemory-policy"
    value = "volatile-lru"
  }

  parameter {
    name  = "notify-keyspace-events"
    value = "Ex"
  }

  tags = {
    Name = "${local.name_prefix}-redis-params"
  }
}
