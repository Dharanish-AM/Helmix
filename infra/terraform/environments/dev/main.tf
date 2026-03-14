terraform {
  required_version = ">= 1.7.0"
}

locals {
  deployment_mode = "k3d-local"
  notes = [
    "Dev uses local k3d and skips cloud infrastructure provisioning.",
    "Use this stack for local iteration and API contract checks.",
  ]
}

output "environment" {
  value = "dev"
}

output "deployment_mode" {
  value = local.deployment_mode
}

output "notes" {
  value = local.notes
}
