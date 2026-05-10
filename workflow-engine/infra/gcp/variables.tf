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
