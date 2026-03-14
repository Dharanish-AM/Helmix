output "registry_url" {
  description = "Container registry URL."
  value       = terraform_data.registry.output.registry_url
}
