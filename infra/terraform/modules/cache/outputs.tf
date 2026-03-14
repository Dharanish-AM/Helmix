output "redis_endpoint" {
  description = "Redis endpoint hostname."
  value       = terraform_data.cache.output.redis_endpoint
}
