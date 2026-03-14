terraform {
  required_version = ">= 1.7.0"
}

locals {
  provider_key = lower(var.cloud_provider)
  platform_by_provider = {
    aws   = "eks"
    gcp   = "gke"
    azure = "aks"
  }
  platform          = local.platform_by_provider[local.provider_key]
  normalized_region = replace(var.region, "-", "")
  cluster_name      = format("helmix-%s-%s", local.platform, local.normalized_region)
  cluster_endpoint  = format("https://%s.%s.%s.helmix.local", local.cluster_name, var.region, local.provider_key)
  cluster_ca        = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0="
  kubeconfig_cmd    = format("helmixctl kubeconfig --provider=%s --cluster=%s --region=%s", local.provider_key, local.cluster_name, var.region)
}

resource "terraform_data" "cluster" {
  input = {
    provider         = local.provider_key
    platform         = local.platform
    cluster_name     = local.cluster_name
    cluster_endpoint = local.cluster_endpoint
    cluster_ca       = local.cluster_ca
    kubeconfig_cmd   = local.kubeconfig_cmd
    vpc_id           = var.vpc_id
    subnet_ids       = var.subnet_ids
    node_type        = var.node_type
    min_nodes        = var.min_nodes
    max_nodes        = var.max_nodes
  }
}

check "node_bounds" {
  assert {
    condition     = var.max_nodes >= var.min_nodes
    error_message = "max_nodes must be greater than or equal to min_nodes."
  }
}
