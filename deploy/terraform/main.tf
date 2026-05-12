# =============================================================================
# Flagstone — Terraform root configuration
# =============================================================================
# This sets up the AWS provider and Terraform's required versions.
# State is local for now — see DESIGN.md for when to migrate to S3 backend.
# =============================================================================

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.70"
    }
  }

  # When the project gets serious, uncomment this and create an S3 bucket
  # for state. Local state is fine for solo dev — but lose the file and you
  # lose the ability to manage the infra (you'd have to import everything
  # back). See DESIGN.md > "State management".
  #
  # backend "s3" {
  #   bucket         = "flagstone-tfstate"
  #   key            = "infra/terraform.tfstate"
  #   region         = "us-east-1"
  #   encrypt        = true
  #   dynamodb_table = "flagstone-tfstate-lock"
  # }
}

provider "aws" {
  region = var.aws_region

  # default_tags propagates to every resource that supports tagging — saves
  # us from having to remember tags on each resource and makes cost
  # allocation in AWS Cost Explorer trivial.
  default_tags {
    tags = {
      Project     = var.project_name
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}
