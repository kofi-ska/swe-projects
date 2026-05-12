variable "project_id" {
  type        = string
  description = "GCP project id."
}

variable "region" {
  type        = string
  description = "GCP region."
  default     = "europe-west2"
}

variable "zone" {
  type        = string
  description = "GCP zone."
  default     = "europe-west2-b"
}

variable "name" {
  type        = string
  description = "Base name for resources."
  default     = "workflow-engine"
}

variable "machine_type" {
  type        = string
  description = "Compute Engine machine type."
  default     = "e2-standard-2"
}

variable "repo_url" {
  type        = string
  description = "Git repository URL to clone on the VM."
}

variable "repo_ref" {
  type        = string
  description = "Git ref to deploy."
  default     = "main"
}

variable "app_port" {
  type        = number
  description = "Public app port."
  default     = 8080
}

variable "public_domain" {
  type        = string
  description = "Public DNS name used by the reverse proxy."
}

variable "ssh_source_ranges" {
  type        = list(string)
  description = "CIDR blocks allowed to SSH to the VM."
  default     = ["0.0.0.0/0"]
}

variable "postgres_password" {
  type        = string
  description = "Postgres password used inside Compose."
  sensitive   = true
}

variable "v3_api_key" {
  type        = string
  description = "API key required by v3 HTTP endpoints."
  sensitive   = true
  validation {
    condition     = length(trimspace(var.v3_api_key)) > 0
    error_message = "v3_api_key must be set for public deployments."
  }
}
