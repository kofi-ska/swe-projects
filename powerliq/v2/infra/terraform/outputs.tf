output "api_base_url" {
  value = "https://${aws_lb.this.dns_name}"
}
output "ecr_repositories" {
  value = { for name, repo in aws_ecr_repository.service : name => repo.repository_url }
}
output "estimated_baseline_monthly_usd" {
  value       = local.estimated_monthly_usd
  description = "Planning estimate, not a billing guarantee. AWS Budget provides the hard operational warning."
}
output "security_note" {
  value = "Confirm the SNS email subscription before relying on alarms."
}
output "oauth_token_endpoint" {
  value = "https://${aws_cognito_user_pool_domain.this.domain}.auth.${var.aws_region}.amazoncognito.com/oauth2/token"
}
output "oauth_client_id" {
  value = aws_cognito_user_pool_client.service.id
}
output "oauth_client_secret" {
  value     = aws_cognito_user_pool_client.service.client_secret
  sensitive = true
}
