terraform {
  required_version = ">= 1.7.0"
}

locals {
  provider_key       = lower(var.cloud_provider)
  normalized_region  = replace(var.region, "-", "")
  vpc_id             = format("%s-%s-vpc", local.provider_key, local.normalized_region)
  public_subnet_ids  = [for i in range(var.az_count) : format("%s-public-%02d", local.vpc_id, i + 1)]
  private_subnet_ids = [for i in range(var.az_count) : format("%s-private-%02d", local.vpc_id, i + 1)]
}

resource "terraform_data" "topology" {
  input = {
    provider           = local.provider_key
    region             = var.region
    cidr_block         = var.cidr_block
    vpc_id             = local.vpc_id
    public_subnet_ids  = local.public_subnet_ids
    private_subnet_ids = local.private_subnet_ids
  }
}
