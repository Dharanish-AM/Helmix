output "vpc_id" {
  description = "VPC/network identifier."
  value       = terraform_data.topology.output.vpc_id
}

output "public_subnet_ids" {
  description = "Public subnet identifiers."
  value       = terraform_data.topology.output.public_subnet_ids
}

output "private_subnet_ids" {
  description = "Private subnet identifiers."
  value       = terraform_data.topology.output.private_subnet_ids
}
