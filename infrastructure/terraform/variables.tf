################################################################################
# General
################################################################################

variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "ap-southeast-1"
}

variable "environment" {
  description = "Deployment environment (dev, staging, prod)"
  type        = string
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, staging, prod."
  }
}

variable "domain_name" {
  description = "Primary domain name for GarudaPass (e.g., garudapass.id)"
  type        = string
}

################################################################################
# Networking
################################################################################

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "single_nat_gateway" {
  description = "Use a single NAT gateway (cost saving for dev). Set false for HA in prod."
  type        = bool
  default     = true
}

################################################################################
# EKS
################################################################################

variable "eks_cluster_version" {
  description = "Kubernetes version for the EKS cluster"
  type        = string
  default     = "1.29"
}

variable "eks_node_instance_types" {
  description = "Instance types for EKS managed node group"
  type        = list(string)
  default     = ["t3.medium"]
}

variable "eks_node_min_size" {
  description = "Minimum number of nodes in the EKS node group"
  type        = number
  default     = 2
}

variable "eks_node_max_size" {
  description = "Maximum number of nodes in the EKS node group"
  type        = number
  default     = 10
}

variable "eks_node_desired_size" {
  description = "Desired number of nodes in the EKS node group"
  type        = number
  default     = 2
}

variable "eks_node_disk_size" {
  description = "Disk size in GB for EKS nodes"
  type        = number
  default     = 50
}

################################################################################
# RDS
################################################################################

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.medium"
}

variable "db_allocated_storage" {
  description = "Allocated storage for RDS in GB"
  type        = number
  default     = 50
}

variable "db_max_allocated_storage" {
  description = "Maximum allocated storage for RDS autoscaling in GB"
  type        = number
  default     = 200
}

variable "db_engine_version" {
  description = "PostgreSQL engine version"
  type        = string
  default     = "16.2"
}

variable "db_multi_az" {
  description = "Enable Multi-AZ for RDS"
  type        = bool
  default     = false
}

variable "db_backup_retention_period" {
  description = "Number of days to retain RDS backups"
  type        = number
  default     = 7
}

variable "db_deletion_protection" {
  description = "Enable deletion protection for RDS"
  type        = bool
  default     = true
}

variable "db_name" {
  description = "Name of the default database"
  type        = string
  default     = "garudapass"
}

################################################################################
# ElastiCache (Redis)
################################################################################

variable "redis_node_type" {
  description = "ElastiCache Redis node type"
  type        = string
  default     = "cache.t3.medium"
}

variable "redis_num_cache_clusters" {
  description = "Number of cache clusters (nodes) in the replication group"
  type        = number
  default     = 1
}

variable "redis_engine_version" {
  description = "Redis engine version"
  type        = string
  default     = "7.1"
}

################################################################################
# Monitoring
################################################################################

variable "alert_email" {
  description = "Email address for CloudWatch alarm notifications"
  type        = string
  default     = ""
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 30
}

################################################################################
# Services
################################################################################

variable "services" {
  description = "List of GarudaPass microservice names for ECR repositories"
  type        = list(string)
  default = [
    "bff",
    "identity-service",
    "nik-service",
    "document-service",
    "verification-service",
    "notification-service",
    "audit-service",
    "consent-service",
    "admin-service",
    "citizen-portal",
    "admin-portal",
  ]
}
