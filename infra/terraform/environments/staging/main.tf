terraform {
  required_version = ">= 1.7.0"
}

variable "cloud_provider" {
  type    = string
  default = "aws"
}

variable "region" {
  type    = string
  default = "us-east-1"
}

module "vpc" {
  source         = "../../modules/vpc"
  cloud_provider = var.cloud_provider
  region         = var.region
  cidr_block     = "10.20.0.0/16"
  az_count       = 1
}

module "kubernetes" {
  source         = "../../modules/kubernetes"
  cloud_provider = var.cloud_provider
  region         = var.region
  vpc_id         = module.vpc.vpc_id
  subnet_ids     = module.vpc.private_subnet_ids
  node_type      = "small"
  min_nodes      = 1
  max_nodes      = 2
}

module "database" {
  source         = "../../modules/database"
  cloud_provider = var.cloud_provider
  region         = var.region
  vpc_id         = module.vpc.vpc_id
  subnet_ids     = module.vpc.private_subnet_ids
  instance_class = "db-small"
  storage_gb     = 30
  multi_az       = false
}

module "cache" {
  source         = "../../modules/cache"
  cloud_provider = var.cloud_provider
  region         = var.region
  vpc_id         = module.vpc.vpc_id
  subnet_ids     = module.vpc.private_subnet_ids
}

module "registry" {
  source         = "../../modules/registry"
  cloud_provider = var.cloud_provider
  region         = var.region
  project_slug   = "helmix"
}

output "kubeconfig_command" {
  value = module.kubernetes.kubeconfig_command
}

output "db_endpoint" {
  value = module.database.db_endpoint
}

output "redis_endpoint" {
  value = module.cache.redis_endpoint
}

output "registry_url" {
  value = module.registry.registry_url
}
