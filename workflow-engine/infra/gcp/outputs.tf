output "public_ip" {
  value       = google_compute_address.main.address
  description = "Public IP of the workflow-engine VM."
}

output "public_url" {
  value       = var.public_domain != "" ? "https://${var.public_domain}" : "http://${google_compute_address.main.address}"
  description = "HTTP URL for the live service."
}
