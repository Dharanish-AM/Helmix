terraform {
  required_version = ">= 1.7.0"
}

locals {
  provider_key = lower(var.cloud_provider)
  service_by_provider = {
    aws   = "ecr"
    gcp   = "artifact-registry"
    azure = "acr"
  }
  service_name      = local.service_by_provider[local.provider_key]
  normalized_region = replace(var.region, "-", "")
  registry_url      = format("%s-%s-%s.%s.helmix.registry", var.project_slug, local.provider_key, local.service_name, local.normalized_region)
}

resource "terraform_data" "registry" {
  input = {
    provider     = local.provider_key
    service_name = local.service_name
    project_slug = var.project_slug
    region       = var.region
    registry_url = local.registry_url
  }
}
