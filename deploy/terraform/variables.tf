# =============================================================================
# Flagstone — Input variables
# =============================================================================
# All knobs go here. Override via terraform.tfvars or -var flags.
# Defaults are tuned for the AWS Free Tier — change at your wallet's risk.
# =============================================================================

variable "aws_region" {
  description = "AWS region. us-east-1 is the cheapest and has the most free-tier coverage."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Used as a prefix for all resource names. Keep it short — some AWS resources have name length limits."
  type        = string
  default     = "flagstone"
}

variable "environment" {
  description = "Logical environment (dev/staging/prod). Used in tags and resource names."
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

# -----------------------------------------------------------------------------
# Networking
# -----------------------------------------------------------------------------

variable "vpc_cidr" {
  description = "CIDR for the VPC. /16 is overkill for one server but costs nothing and gives us room to grow."
  type        = string
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "Public subnets, one per AZ. Two AZs satisfy RDS's multi-AZ requirement even if we only deploy in one."
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "Private subnets for RDS. Database is never reachable from the internet."
  type        = list(string)
  default     = ["10.0.10.0/24", "10.0.11.0/24"]
}

variable "ssh_allowed_cidr" {
  description = "Your public IP for SSH access. Find it with `curl ifconfig.me` and set it as YOUR_IP/32. NEVER use 0.0.0.0/0 in production."
  type        = string
  # No default — forces the operator to explicitly set their IP.
  # The previous default of 0.0.0.0/0 was a security risk: an accidental
  # `terraform apply` without tfvars would open SSH to the entire internet.

  validation {
    condition     = can(cidrhost(var.ssh_allowed_cidr, 0))
    error_message = "ssh_allowed_cidr must be a valid CIDR block (e.g. 203.0.113.50/32)."
  }

  validation {
    condition     = var.ssh_allowed_cidr != "0.0.0.0/0"
    error_message = "ssh_allowed_cidr must not be 0.0.0.0/0 — restrict SSH to your IP (use `curl ifconfig.me` to find it)."
  }
}

# -----------------------------------------------------------------------------
# Compute
# -----------------------------------------------------------------------------

variable "instance_type" {
  description = "EC2 instance type. t4g.small is free until Dec 2026 (ARM Graviton2, 2 vCPU, 2GB)."
  type        = string
  default     = "t4g.small"
}

variable "key_pair_name" {
  description = "Name of an existing EC2 key pair for SSH. Create one in the AWS console first."
  type        = string
}

# -----------------------------------------------------------------------------
# Database
# -----------------------------------------------------------------------------

variable "db_instance_class" {
  description = "RDS instance class. db.t3.micro is free for 12 months."
  type        = string
  default     = "db.t3.micro"
}

variable "db_name" {
  description = "Postgres database name."
  type        = string
  default     = "flagstone"
}

variable "db_username" {
  description = "Postgres master username."
  type        = string
  default     = "flagstone"
}

variable "db_password" {
  description = "Postgres master password. Pass via -var or environment variable, NEVER commit to git."
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.db_password) >= 16
    error_message = "db_password must be at least 16 characters. Generate one with: openssl rand -base64 32"
  }
}
