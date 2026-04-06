# GarudaPass - Dev Environment
# Usage: terraform plan -var-file=environments/dev.tfvars

environment = "dev"
domain_name = "dev.garudapass.id"

# Networking - single NAT gateway to reduce cost
single_nat_gateway = true

# EKS - smaller nodes, fewer replicas
eks_cluster_version     = "1.29"
eks_node_instance_types = ["t3.medium"]
eks_node_min_size       = 2
eks_node_max_size       = 5
eks_node_desired_size   = 2
eks_node_disk_size      = 50

# RDS - single AZ, smaller instance
db_instance_class          = "db.t3.medium"
db_allocated_storage       = 50
db_max_allocated_storage   = 100
db_multi_az                = false
db_backup_retention_period = 3
db_deletion_protection     = false

# Redis - single node
redis_node_type          = "cache.t3.micro"
redis_num_cache_clusters = 1

# Monitoring
log_retention_days = 7
