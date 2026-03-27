terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region  = var.region
  profile = "experiment"
}

variable "region" {
  default = "us-west-2"
}

variable "key_name" {
  description = "Name for the EC2 key pair"
  type        = string
  default     = "earthworm-ebpf"
}

variable "public_key_path" {
  description = "Path to your local SSH public key"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

variable "my_ip" {
  description = "Your public IP for SSH access (IPv4 CIDR like 203.0.113.10/32, or IPv6 like 2605:59c8::/128)"
  type        = string
}

variable "instance_type" {
  default = "t3.medium"
}

# Create a simple VPC for eBPF development
resource "aws_vpc" "ebpf_dev" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name    = "earthworm-ebpf-dev"
    Project = "earthworm"
  }
}

resource "aws_internet_gateway" "ebpf_dev" {
  vpc_id = aws_vpc.ebpf_dev.id

  tags = {
    Name    = "earthworm-ebpf-dev"
    Project = "earthworm"
  }
}

resource "aws_subnet" "ebpf_dev" {
  vpc_id                  = aws_vpc.ebpf_dev.id
  cidr_block              = "10.0.1.0/24"
  map_public_ip_on_launch = true
  availability_zone       = "${var.region}a"

  tags = {
    Name    = "earthworm-ebpf-dev"
    Project = "earthworm"
  }
}

resource "aws_route_table" "ebpf_dev" {
  vpc_id = aws_vpc.ebpf_dev.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.ebpf_dev.id
  }

  tags = {
    Name    = "earthworm-ebpf-dev"
    Project = "earthworm"
  }
}

resource "aws_route_table_association" "ebpf_dev" {
  subnet_id      = aws_subnet.ebpf_dev.id
  route_table_id = aws_route_table.ebpf_dev.id
}

# Latest Ubuntu 22.04 LTS AMI
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# Import your local SSH public key into AWS
resource "aws_key_pair" "ebpf_dev" {
  key_name   = var.key_name
  public_key = file(var.public_key_path)
}

# Security group: SSH from your IP only
resource "aws_security_group" "ebpf_dev" {
  name        = "earthworm-ebpf-dev"
  description = "SSH access for eBPF development"
  vpc_id      = aws_vpc.ebpf_dev.id

  ingress {
    description      = "SSH from my IP"
    from_port        = 22
    to_port          = 22
    protocol         = "tcp"
    cidr_blocks      = can(regex(":", var.my_ip)) ? [] : [var.my_ip]
    ipv6_cidr_blocks = can(regex(":", var.my_ip)) ? [var.my_ip] : []
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "earthworm-ebpf-dev"
    Project = "earthworm"
  }
}

# EC2 instance with eBPF toolchain installed via user_data
resource "aws_instance" "ebpf_dev" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = aws_key_pair.ebpf_dev.key_name
  subnet_id              = aws_subnet.ebpf_dev.id
  vpc_security_group_ids = [aws_security_group.ebpf_dev.id]

  associate_public_ip_address = true

  root_block_device {
    volume_size = 30
    volume_type = "gp3"
  }

  user_data = <<-EOF
    #!/bin/bash
    set -e

    # Update system
    apt-get update -y
    apt-get upgrade -y

    # Install eBPF toolchain (bpftool is part of linux-tools on Ubuntu 22.04)
    apt-get install -y clang llvm libbpf-dev linux-tools-$(uname -r) linux-tools-common git make

    # Install Go 1.22
    wget -q https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
    rm go1.22.5.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh

    # Generate vmlinux.h from running kernel BTF
    mkdir -p /opt/earthworm
    bpftool btf dump file /sys/kernel/btf/vmlinux format c > /opt/earthworm/vmlinux.h

    # Signal setup complete
    touch /opt/earthworm/.setup-complete
  EOF

  tags = {
    Name    = "earthworm-ebpf-dev"
    Project = "earthworm"
  }
}

output "instance_id" {
  value = aws_instance.ebpf_dev.id
}

output "public_ip" {
  value = aws_instance.ebpf_dev.public_ip
}

output "ssh_command" {
  value = "ssh ubuntu@${aws_instance.ebpf_dev.public_ip}"
}

output "setup_instructions" {
  value = <<-EOT
    After SSH:
    1. Wait for setup: tail -f /var/log/cloud-init-output.log
    2. Check: ls /opt/earthworm/.setup-complete
    3. Clone: git clone https://github.com/mercadoalex/earthworm.git
    4. Copy vmlinux.h: cp /opt/earthworm/vmlinux.h earthworm/src/ebpf/headers/
    5. Generate BPF: cd earthworm && go generate ./src/agent/...
    6. Run agent: sudo go run ./src/agent/ --ebpf
  EOT
}
