# =============================================================================
# Flagstone — Database (RDS Postgres)
# =============================================================================
# db.t3.micro is free for 12 months. After that ~$13/month for the instance
# plus storage. When the trial ends, options are:
#   1. Migrate to self-hosted Postgres on the EC2 box (cheapest, less reliable)
#   2. Stay on RDS (managed backups, point-in-time recovery — worth $13/mo)
#   3. Migrate to Hetzner managed Postgres or a Postgres-as-a-service like Neon
#
# Single-AZ on purpose: Multi-AZ doubles the cost and isn't justified at MVP.
# We DO take automated backups (7 days retention) so a hardware failure is
# recoverable, just with downtime.
# =============================================================================

# RDS needs a "DB Subnet Group" — a group of subnets in different AZs where
# it can place the primary (and replicas if Multi-AZ).
resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "${var.project_name}-db-subnet-group"
  }
}

# Custom parameter group: lets us tweak Postgres config without touching the
# instance. For now we just enable pg_stat_statements (essential for slow
# query analysis) — adding it later requires a reboot, so we set it upfront.
resource "aws_db_parameter_group" "main" {
  name   = "${var.project_name}-pg16"
  family = "postgres16"

  parameter {
    name         = "shared_preload_libraries"
    value        = "pg_stat_statements"
    apply_method = "pending-reboot" # required for libraries
  }

  parameter {
    name  = "log_min_duration_statement"
    value = "1000" # log queries slower than 1 second
  }

  tags = {
    Name = "${var.project_name}-pg16"
  }
}

resource "aws_db_instance" "main" {
  identifier     = "${var.project_name}-db"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = var.db_instance_class

  # Storage: 20GB gp3, with autoscaling up to 100GB. gp3 is cheaper than gp2
  # at the same performance, and the autoscaling means we don't get paged
  # at 3am because the disk filled up.
  allocated_storage     = 20
  max_allocated_storage = 100
  storage_type          = "gp3"
  storage_encrypted     = true

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password # in production, fetch from Secrets Manager

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.db.id]
  parameter_group_name   = aws_db_parameter_group.main.name

  publicly_accessible = false # only EC2 in our VPC can reach it
  multi_az            = false # single-AZ to stay in free tier

  backup_retention_period = 7
  backup_window           = "03:00-04:00" # UTC; AR is UTC-3 → midnight local
  maintenance_window      = "sun:04:00-sun:05:00"

  # Without `skip_final_snapshot=true`, terraform destroy will fail demanding
  # a final snapshot name. For dev that's annoying; for prod, set to false.
  skip_final_snapshot = var.environment == "dev"

  # Required if final_snapshot is taken. Uses a static name to avoid noisy
  # diffs on every `terraform plan` (timestamp() changes every run).
  # If you destroy and recreate, the old snapshot must be deleted first or
  # this name changed — acceptable tradeoff vs. plan noise.
  final_snapshot_identifier = var.environment == "dev" ? null : "${var.project_name}-db-final"

  # Performance Insights: free for 7 days of retention. Lifesaver for
  # debugging slow queries.
  performance_insights_enabled          = true
  performance_insights_retention_period = 7

  # Send Postgres logs to CloudWatch — needed for any decent observability.
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  # Apply changes immediately rather than waiting for the maintenance window.
  # In dev that's what we want; in prod, set to false to avoid surprises.
  apply_immediately = var.environment == "dev"

  tags = {
    Name = "${var.project_name}-db"
  }
}
