# GarudaPass - Production Environment
# Usage: terraform plan -var-file=environments/prod.tfvars

environment = "prod"
domain_name = "garudapass.id"

# Networking - HA NAT gateways across all AZs
single_nat_gateway = false

# EKS - larger nodes, more capacity
eks_cluster_version     = "1.29"
eks_node_instance_types = ["m5.large"]
eks_node_min_size       = 3
eks_node_max_size       = 10
eks_node_desired_size   = 3
eks_node_disk_size      = 100

# RDS - Multi-AZ, larger instance, longer retention
db_instance_class          = "db.r6g.large"
db_allocated_storage       = 100
db_max_allocated_storage   = 500
db_multi_az                = true
db_backup_retention_period = 14
db_deletion_protection     = true

# Redis - replication for HA
redis_node_type          = "cache.r6g.large"
redis_num_cache_clusters = 3
redis_engine_version     = "7.1"

# Monitoring
alert_email        = "ops@garudapass.id"
log_retention_days = 90
