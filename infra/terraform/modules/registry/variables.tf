variable "cloud_provider" {
  description = "Target cloud provider: aws, gcp, or azure."
  type        = string

  validation {
    condition     = contains(["aws", "gcp", "azure"], lower(var.cloud_provider))
    error_message = "cloud_provider must be one of aws, gcp, azure."
  }
}

variable "region" {
  description = "Cloud region for registry resources."
  type        = string
}

variable "project_slug" {
  description = "Project identifier used in registry naming."
  type        = string
  default     = "helmix"
}
