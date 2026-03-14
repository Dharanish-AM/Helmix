terraform {
  required_version = ">= 1.7.0"
}

locals {
  provider_key = lower(var.cloud_provider)
  engine_by_provider = {
    aws   = "rds-postgres"
    gcp   = "cloudsql-postgres"
    azure = "azure-postgres-flex"
  }
  engine            = local.engine_by_provider[local.provider_key]
  normalized_region = replace(var.region, "-", "")
  db_name           = "helmix"
  db_port           = 5432
  db_endpoint       = format("%s-%s-db.%s.%s.helmix.internal", local.provider_key, local.normalized_region, local.engine, var.region)
}

resource "terraform_data" "database" {
  input = {
    provider       = local.provider_key
    engine         = local.engine
    db_name        = local.db_name
    db_port        = local.db_port
    db_endpoint    = local.db_endpoint
    vpc_id         = var.vpc_id
    subnet_ids     = var.subnet_ids
    instance_class = var.instance_class
    storage_gb     = var.storage_gb
    multi_az       = var.multi_az
  }
}
