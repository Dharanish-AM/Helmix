terraform {
  required_version = ">= 1.7.0"
}

locals {
  provider_key = lower(var.cloud_provider)
  service_by_provider = {
    aws   = "elasticache"
    gcp   = "memorystore"
    azure = "azure-cache-redis"
  }
  cache_service     = local.service_by_provider[local.provider_key]
  normalized_region = replace(var.region, "-", "")
  redis_endpoint    = format("%s-%s-redis.%s.%s.helmix.internal", local.provider_key, local.normalized_region, local.cache_service, var.region)
}

resource "terraform_data" "cache" {
  input = {
    provider       = local.provider_key
    cache_service  = local.cache_service
    redis_endpoint = local.redis_endpoint
    vpc_id         = var.vpc_id
    subnet_ids     = var.subnet_ids
  }
}
