output "public_ip" {
  value       = google_compute_address.main.address
  description = "Public IP of the workflow-engine VM."
}

output "public_url" {
  value       = "http://${google_compute_address.main.address}:${var.app_port}"
  description = "HTTP URL for the live service."
}
