################################################################################
# SNS Topic for Alerts
################################################################################

resource "aws_sns_topic" "alerts" {
  name = "${local.name_prefix}-alerts"

  kms_master_key_id = aws_kms_key.secrets.id

  tags = {
    Name = "${local.name_prefix}-alerts"
  }
}

resource "aws_sns_topic_subscription" "alerts_email" {
  count = var.alert_email != "" ? 1 : 0

  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

################################################################################
# CloudWatch Log Groups for Services
################################################################################

resource "aws_cloudwatch_log_group" "services" {
  for_each = toset(var.services)

  name              = "/garudapass/${var.environment}/${each.value}"
  retention_in_days = var.log_retention_days

  tags = {
    Name    = "${local.name_prefix}-${each.value}-logs"
    Service = each.value
  }
}

################################################################################
# CloudWatch Alarms - RDS
################################################################################

resource "aws_cloudwatch_metric_alarm" "rds_cpu" {
  alarm_name          = "${local.name_prefix}-rds-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "CPUUtilization"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "RDS CPU utilization exceeds 80%"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  ok_actions          = [aws_sns_topic.alerts.arn]

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.identifier
  }

  tags = {
    Name = "${local.name_prefix}-rds-cpu-alarm"
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_free_storage" {
  alarm_name          = "${local.name_prefix}-rds-storage-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 2
  metric_name         = "FreeStorageSpace"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = 5368709120 # 5 GB in bytes
  alarm_description   = "RDS free storage space below 5 GB"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  ok_actions          = [aws_sns_topic.alerts.arn]

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.identifier
  }

  tags = {
    Name = "${local.name_prefix}-rds-storage-alarm"
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_connections" {
  alarm_name          = "${local.name_prefix}-rds-connections-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "DatabaseConnections"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = 100
  alarm_description   = "RDS database connections exceed 100"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  ok_actions          = [aws_sns_topic.alerts.arn]

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.identifier
  }

  tags = {
    Name = "${local.name_prefix}-rds-connections-alarm"
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_read_latency" {
  alarm_name          = "${local.name_prefix}-rds-read-latency-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "ReadLatency"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = 0.02 # 20ms
  alarm_description   = "RDS read latency exceeds 20ms"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.identifier
  }

  tags = {
    Name = "${local.name_prefix}-rds-read-latency-alarm"
  }
}

################################################################################
# CloudWatch Alarms - ElastiCache Redis
################################################################################

resource "aws_cloudwatch_metric_alarm" "redis_cpu" {
  alarm_name          = "${local.name_prefix}-redis-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Average"
  threshold           = 75
  alarm_description   = "Redis CPU utilization exceeds 75%"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  ok_actions          = [aws_sns_topic.alerts.arn]

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.id
  }

  tags = {
    Name = "${local.name_prefix}-redis-cpu-alarm"
  }
}

resource "aws_cloudwatch_metric_alarm" "redis_memory" {
  alarm_name          = "${local.name_prefix}-redis-memory-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "DatabaseMemoryUsagePercentage"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "Redis memory usage exceeds 80%"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  ok_actions          = [aws_sns_topic.alerts.arn]

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.id
  }

  tags = {
    Name = "${local.name_prefix}-redis-memory-alarm"
  }
}

resource "aws_cloudwatch_metric_alarm" "redis_evictions" {
  alarm_name          = "${local.name_prefix}-redis-evictions-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "Evictions"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Sum"
  threshold           = 100
  alarm_description   = "Redis evictions exceed 100 in 5 minutes"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.id
  }

  tags = {
    Name = "${local.name_prefix}-redis-evictions-alarm"
  }
}

################################################################################
# CloudWatch Alarms - EKS
################################################################################

resource "aws_cloudwatch_metric_alarm" "eks_node_cpu" {
  alarm_name          = "${local.name_prefix}-eks-node-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "node_cpu_utilization"
  namespace           = "ContainerInsights"
  period              = 300
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "EKS node CPU utilization exceeds 80%"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  dimensions = {
    ClusterName = aws_eks_cluster.main.name
  }

  tags = {
    Name = "${local.name_prefix}-eks-cpu-alarm"
  }
}

################################################################################
# CloudWatch Dashboard
################################################################################

resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "${local.name_prefix}-overview"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title   = "RDS - CPU Utilization"
          metrics = [["AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", aws_db_instance.main.identifier]]
          period  = 300
          stat    = "Average"
          region  = var.aws_region
          view    = "timeSeries"
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title   = "RDS - Database Connections"
          metrics = [["AWS/RDS", "DatabaseConnections", "DBInstanceIdentifier", aws_db_instance.main.identifier]]
          period  = 300
          stat    = "Average"
          region  = var.aws_region
          view    = "timeSeries"
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "RDS - Free Storage Space"
          metrics = [["AWS/RDS", "FreeStorageSpace", "DBInstanceIdentifier", aws_db_instance.main.identifier]]
          period  = 300
          stat    = "Average"
          region  = var.aws_region
          view    = "timeSeries"
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "RDS - Read/Write Latency"
          metrics = [
            ["AWS/RDS", "ReadLatency", "DBInstanceIdentifier", aws_db_instance.main.identifier],
            ["AWS/RDS", "WriteLatency", "DBInstanceIdentifier", aws_db_instance.main.identifier],
          ]
          period = 300
          stat   = "Average"
          region = var.aws_region
          view   = "timeSeries"
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 12
        width  = 12
        height = 6
        properties = {
          title   = "Redis - CPU Utilization"
          metrics = [["AWS/ElastiCache", "CPUUtilization", "ReplicationGroupId", aws_elasticache_replication_group.main.id]]
          period  = 300
          stat    = "Average"
          region  = var.aws_region
          view    = "timeSeries"
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 12
        width  = 12
        height = 6
        properties = {
          title   = "Redis - Memory Usage"
          metrics = [["AWS/ElastiCache", "DatabaseMemoryUsagePercentage", "ReplicationGroupId", aws_elasticache_replication_group.main.id]]
          period  = 300
          stat    = "Average"
          region  = var.aws_region
          view    = "timeSeries"
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 18
        width  = 12
        height = 6
        properties = {
          title   = "Redis - Cache Hits vs Misses"
          metrics = [
            ["AWS/ElastiCache", "CacheHits", "ReplicationGroupId", aws_elasticache_replication_group.main.id],
            ["AWS/ElastiCache", "CacheMisses", "ReplicationGroupId", aws_elasticache_replication_group.main.id],
          ]
          period = 300
          stat   = "Sum"
          region = var.aws_region
          view   = "timeSeries"
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 18
        width  = 12
        height = 6
        properties = {
          title   = "Redis - Current Connections"
          metrics = [["AWS/ElastiCache", "CurrConnections", "ReplicationGroupId", aws_elasticache_replication_group.main.id]]
          period  = 300
          stat    = "Average"
          region  = var.aws_region
          view    = "timeSeries"
        }
      }
    ]
  })
}
