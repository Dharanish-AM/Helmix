variable "cloud_provider" {
  description = "Target cloud provider: aws, gcp, or azure."
  type        = string

  validation {
    condition     = contains(["aws", "gcp", "azure"], lower(var.cloud_provider))
    error_message = "cloud_provider must be one of aws, gcp, azure."
  }
}

variable "region" {
  description = "Cloud region for cluster resources."
  type        = string
}

variable "vpc_id" {
  description = "Network identifier where the cluster runs."
  type        = string
}

variable "subnet_ids" {
  description = "Subnets for cluster nodes."
  type        = list(string)
}

variable "node_type" {
  description = "Node machine type for the default node group."
  type        = string
}

variable "min_nodes" {
  description = "Minimum autoscaling node count."
  type        = number
}

variable "max_nodes" {
  description = "Maximum autoscaling node count."
  type        = number
}
