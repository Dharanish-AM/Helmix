variable "cloud_provider" {
  description = "Target cloud provider: aws, gcp, or azure."
  type        = string

  validation {
    condition     = contains(["aws", "gcp", "azure"], lower(var.cloud_provider))
    error_message = "cloud_provider must be one of aws, gcp, azure."
  }
}

variable "region" {
  description = "Cloud region for cache resources."
  type        = string
}

variable "vpc_id" {
  description = "Network identifier where cache runs."
  type        = string
}

variable "subnet_ids" {
  description = "Subnets available to cache resources."
  type        = list(string)
}
