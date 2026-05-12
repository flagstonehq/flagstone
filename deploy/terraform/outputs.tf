# =============================================================================
# Flagstone — Outputs
# =============================================================================
# These print after `terraform apply` and are queryable with `terraform output`.
# Sensitive ones (like DB endpoint) are marked so they don't leak into logs.
# =============================================================================

output "app_public_ip" {
  description = "Public IPv4 of the EC2 instance. Use this to SSH in or hit the API."
  value       = aws_instance.app.public_ip
}

output "app_public_dns" {
  description = "AWS-assigned public DNS name. Stable while the instance is running, but changes on stop/start."
  value       = aws_instance.app.public_dns
}

output "ssh_command" {
  description = "Ready-to-paste SSH command (assumes your private key matches the configured key pair)."
  value       = "ssh -i ~/.ssh/${var.key_pair_name}.pem ubuntu@${aws_instance.app.public_ip}"
}

output "db_endpoint" {
  description = "RDS endpoint (host:port). Set this as DATABASE_URL host on the EC2."
  value       = aws_db_instance.main.endpoint
  sensitive   = true
}

output "db_address" {
  description = "RDS hostname only (no port)."
  value       = aws_db_instance.main.address
  sensitive   = true
}

output "vpc_id" {
  description = "VPC ID — useful if you add more resources outside this Terraform config."
  value       = aws_vpc.main.id
}
