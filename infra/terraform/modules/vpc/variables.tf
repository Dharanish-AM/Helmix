variable "cloud_provider" {
  description = "Target cloud provider: aws, gcp, or azure."
  type        = string

  validation {
    condition     = contains(["aws", "gcp", "azure"], lower(var.cloud_provider))
    error_message = "cloud_provider must be one of aws, gcp, azure."
  }
}

variable "region" {
  description = "Cloud region for networking resources."
  type        = string
}

variable "cidr_block" {
  description = "Primary CIDR for the virtual network."
  type        = string
}

variable "az_count" {
  description = "Number of availability zones (or equivalent zones)."
  type        = number
  default     = 2

  validation {
    condition     = var.az_count >= 1
    error_message = "az_count must be at least 1."
  }
}
