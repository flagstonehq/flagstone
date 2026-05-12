# =============================================================================
# Flagstone — Compute (EC2)
# =============================================================================
# A single t4g.small instance running Ubuntu 24.04 ARM64. We use the official
# Canonical AMI lookup (data source) instead of hardcoding an AMI ID — IDs
# differ per region and rotate as new images are published.
#
# Why t4g.small:
#   * Free until Dec 2026 (Graviton free trial)
#   * 2 vCPU, 2GB RAM — comfortable for Go server + Redis + small workload
#   * ARM Graviton: Go cross-compiles to ARM with `GOARCH=arm64`, no rewrites
#
# We're deliberately NOT using:
#   * Elastic IP (costs money if instance is stopped — public IP rotates on
#     reboot, but for dev that's fine; use Route 53 ALIAS for stable DNS later)
#   * Auto Scaling Group (overkill for v1; one instance is enough until traction)
#   * Application Load Balancer ($16/month, not free — terminate TLS in Caddy
#     on the box itself for now)
# =============================================================================

# Latest Ubuntu 24.04 LTS for ARM64 (Graviton)
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical's official account

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-arm64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# -----------------------------------------------------------------------------
# IAM role for the instance
# -----------------------------------------------------------------------------
# Attaching an IAM role to EC2 means the app can call AWS APIs (CloudWatch,
# S3 for backups, Secrets Manager) without storing access keys on the box.
# Right now the role has no policies — we'll attach them as we add features.
# -----------------------------------------------------------------------------

resource "aws_iam_role" "app" {
  name = "${var.project_name}-app-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
      Action = "sts:AssumeRole"
    }]
  })
}

# Allows the instance to send logs and metrics to CloudWatch.
# We'll plug OpenTelemetry into this later.
resource "aws_iam_role_policy_attachment" "cloudwatch_agent" {
  role       = aws_iam_role.app.name
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
}

# Allows SSH-less shell access via Session Manager (no port 22 needed).
# Backup plan if you ever lock yourself out of SSH.
resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.app.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "app" {
  name = "${var.project_name}-app-profile"
  role = aws_iam_role.app.name
}

# -----------------------------------------------------------------------------
# The instance itself
# -----------------------------------------------------------------------------

resource "aws_instance" "app" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = var.key_pair_name
  subnet_id              = aws_subnet.public[0].id
  vpc_security_group_ids = [aws_security_group.app.id]
  iam_instance_profile   = aws_iam_instance_profile.app.name

  # IMDSv2 only — defends against SSRF attacks that try to read instance
  # credentials. Should be the default but isn't, so we set it explicitly.
  metadata_options {
    http_tokens   = "required"
    http_endpoint = "enabled"
  }

  root_block_device {
    volume_type = "gp3" # cheaper and faster than gp2; default size 8GB is fine
    volume_size = 20    # under the 30GB free-tier ceiling
    encrypted   = true
  }

  # cloud-init script: installs Docker on first boot. The actual application
  # deploy happens via a separate process (CI/CD, Ansible, etc.). Keeping
  # provisioning minimal here means rebuilds are predictable.
  user_data = <<-EOF
    #!/bin/bash
    set -euxo pipefail

    apt-get update
    apt-get install -y ca-certificates curl gnupg

    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
      | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    echo "deb [arch=arm64 signed-by=/etc/apt/keyrings/docker.gpg] \
      https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
      > /etc/apt/sources.list.d/docker.list

    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

    usermod -aG docker ubuntu
    systemctl enable --now docker
  EOF

  # Force replacement (not in-place update) when user_data changes — otherwise
  # the script only runs on first boot and stale instances drift.
  user_data_replace_on_change = true

  tags = {
    Name = "${var.project_name}-app"
  }
}
