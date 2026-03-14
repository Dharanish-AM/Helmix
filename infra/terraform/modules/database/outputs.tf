output "db_endpoint" {
  description = "Database endpoint hostname."
  value       = terraform_data.database.output.db_endpoint
}

output "db_port" {
  description = "Database endpoint port."
  value       = terraform_data.database.output.db_port
}

output "db_name" {
  description = "Default database name."
  value       = terraform_data.database.output.db_name
}
