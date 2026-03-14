output "cluster_endpoint" {
  description = "Kubernetes API server endpoint."
  value       = terraform_data.cluster.output.cluster_endpoint
}

output "cluster_ca" {
  description = "Base64 encoded cluster CA bundle."
  value       = terraform_data.cluster.output.cluster_ca
}

output "kubeconfig_command" {
  description = "Command to fetch or assemble kubeconfig for operators."
  value       = terraform_data.cluster.output.kubeconfig_cmd
}
