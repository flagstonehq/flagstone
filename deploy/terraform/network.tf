# =============================================================================
# Flagstone — Networking
# =============================================================================
# Layout:
#
#   VPC (10.0.0.0/16)
#     |-- Public subnets  (10.0.1.0/24, 10.0.2.0/24)  --> Internet Gateway
#     |     EC2 lives here. Reachable from internet for HTTP/HTTPS.
#     |
#     `-- Private subnets (10.0.10.0/24, 10.0.11.0/24)
#           RDS lives here. Never reachable from internet.
#
# We deliberately do NOT create a NAT Gateway. NAT costs ~$32/month and is
# the #1 bill-shock culprit on AWS. RDS in private subnets without NAT means
# the DB has no outbound internet — which is a feature, not a bug, for a DB.
#
# Two AZs because RDS DB Subnet Groups REQUIRE at least two subnets in
# different AZs even if you only deploy single-AZ. No extra cost.
# =============================================================================

# Discover available AZs in the region — avoids hardcoding "us-east-1a" etc.
data "aws_availability_zones" "available" {
  state = "available"
}

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true # required for RDS connectivity by hostname
  enable_dns_support   = true

  tags = {
    Name = "${var.project_name}-vpc"
  }
}

# -----------------------------------------------------------------------------
# Public subnets — for EC2
# -----------------------------------------------------------------------------

resource "aws_subnet" "public" {
  count = length(var.public_subnet_cidrs)

  vpc_id                  = aws_vpc.main.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true # auto-assign public IPs to instances launched here

  tags = {
    Name = "${var.project_name}-public-${count.index + 1}"
    Tier = "public"
  }
}

# -----------------------------------------------------------------------------
# Private subnets — for RDS
# -----------------------------------------------------------------------------

resource "aws_subnet" "private" {
  count = length(var.private_subnet_cidrs)

  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_subnet_cidrs[count.index]
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "${var.project_name}-private-${count.index + 1}"
    Tier = "private"
  }
}

# -----------------------------------------------------------------------------
# Internet gateway + route table for public subnets
# -----------------------------------------------------------------------------

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project_name}-igw"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "${var.project_name}-public-rt"
  }
}

resource "aws_route_table_association" "public" {
  count = length(aws_subnet.public)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Note: private subnets get the VPC's default route table, which has no
# 0.0.0.0/0 route. That's intentional — DB has no outbound internet.

# -----------------------------------------------------------------------------
# Security groups
# -----------------------------------------------------------------------------
# Two SGs:
#   * app_sg: open to internet on 80/443, SSH only from your IP
#   * db_sg:  Postgres only from app_sg (no IP-based rules)
#
# Reference SGs by ID, not by IP. AWS evaluates this at packet time, so
# rotating EC2 IPs doesn't break DB access.
# -----------------------------------------------------------------------------

resource "aws_security_group" "app" {
  name        = "${var.project_name}-app-sg"
  description = "EC2 application server"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "SSH from operator IP"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.ssh_allowed_cidr]
  }

  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-app-sg"
  }
}

resource "aws_security_group" "db" {
  name        = "${var.project_name}-db-sg"
  description = "RDS Postgres — only reachable from app SG"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Postgres from app"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.app.id]
  }

  # No egress rules → DB can't initiate outbound connections.
  # This is a security control, not a limitation.

  tags = {
    Name = "${var.project_name}-db-sg"
  }
}
