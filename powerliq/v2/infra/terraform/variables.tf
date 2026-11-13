variable "aws_region" {
  type    = string
  default = "eu-west-2"
}
variable "environment" {
  type    = string
  default = "sandbox"
}
variable "monthly_budget_usd" {
  type    = number
  default = 100
  validation {
    condition     = var.monthly_budget_usd <= 100
    error_message = "PowerLiq sandbox budget must not exceed USD 100/month."
  }
}
variable "alert_email" {
  type        = string
  description = "Email subscribed to operational and budget alarms."
}
variable "certificate_arn" {
  type        = string
  description = "ACM certificate ARN for the public HTTPS listener."
}
variable "image_tag" {
  type    = string
  default = "latest"
}
variable "instance_type" {
  type    = string
  default = "t4g.medium"
  validation {
    condition     = contains(["t4g.small", "t4g.medium"], var.instance_type)
    error_message = "Only budget-approved Graviton instance types are allowed."
  }
}
variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"
  validation {
    condition     = var.db_instance_class == "db.t4g.micro"
    error_message = "Sandbox RDS is constrained to db.t4g.micro."
  }
}
variable "log_retention_days" {
  type    = number
  default = 14
  validation {
    condition     = contains([7, 14, 30], var.log_retention_days)
    error_message = "Log retention must remain bounded."
  }
}
