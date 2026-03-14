variable "cloud_provider" {
  description = "Target cloud provider: aws, gcp, or azure."
  type        = string

  validation {
    condition     = contains(["aws", "gcp", "azure"], lower(var.cloud_provider))
    error_message = "cloud_provider must be one of aws, gcp, azure."
  }
}

variable "region" {
  description = "Cloud region for database resources."
  type        = string
}

variable "vpc_id" {
  description = "Network identifier where the database runs."
  type        = string
}

variable "subnet_ids" {
  description = "Subnets available to the database."
  type        = list(string)
}

variable "instance_class" {
  description = "Provider-specific database class or tier."
  type        = string
}

variable "storage_gb" {
  description = "Allocated storage size in GiB."
  type        = number

  validation {
    condition     = var.storage_gb >= 20
    error_message = "storage_gb must be at least 20 GiB."
  }
}

variable "multi_az" {
  description = "Enable multi-zone/high-availability deployment."
  type        = bool
  default     = false
}
