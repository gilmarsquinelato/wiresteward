data "aws_ami" "flatcar_beta" {
  most_recent      = true
  executable_users = ["all"]
  owners           = ["075585003325"] // this is the account id that Flatcar use to release AMI images

  filter {
    name   = "name"
    values = ["Flatcar-beta-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  name_regex = "^Flatcar-beta-\\d{4}.\\d+.\\d+-hvm$"
}

resource "aws_security_group" "wiresteward" {
  name        = local.name
  description = "Allows wireguard and SSH traffic from anywhere, oauth2-proxy traffic from ALB"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 50620
    to_port     = 50620
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 51820
    to_port     = 51820
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # oauth2-proxy
  ingress {
    from_port       = 4180
    to_port         = 4180
    protocol        = "tcp"
    security_groups = [aws_security_group.wiresteward-lb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = local.name
  }
}

resource "aws_eip" "peer" {
  count = local.instance_count
  vpc   = true

  tags = {
    Name = local.name
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_eip_association" "peer" {
  count         = local.instance_count
  instance_id   = aws_instance.peer[count.index].id
  allocation_id = aws_eip.peer[count.index].id
}

module "wiresteward_ignition" {
  source = "../ignition"

  oauth2_proxy_client_id     = var.oauth2_proxy_client_id
  oauth2_proxy_cookie_secret = var.oauth2_proxy_cookie_secret
  oauth2_proxy_issuer_url    = var.oauth2_proxy_issuer_url
  ssh_key_agent_uri          = var.ssh_key_agent_uri
  ssh_key_agent_groups       = var.ssh_key_agent_groups
  wireguard_cidrs            = var.wireguard_cidrs
  wireguard_endpoints        = aws_route53_record.peer.*.name
  wireguard_exposed_subnets  = var.wireguard_exposed_subnets
}


resource "aws_instance" "peer" {
  count                  = local.instance_count
  ami                    = data.aws_ami.flatcar_beta.id
  instance_type          = "t2.micro"
  vpc_security_group_ids = [aws_security_group.wiresteward.id]
  subnet_id              = var.subnet_ids[count.index]
  source_dest_check      = false

  user_data = module.wiresteward_ignition.ignition[count.index]

  root_block_device {
    volume_type = "gp2"
    volume_size = "10"
  }

  credit_specification {
    cpu_credits = "unlimited"
  }

  tags = {
    Name = local.name
  }
}

resource "aws_route53_record" "peer" {
  count   = local.instance_count
  zone_id = var.dns_zone_id
  name    = "${count.index}.${var.role_name}.${var.dns_zone_name}"
  type    = "A"
  ttl     = "60"
  records = [aws_eip.peer[count.index].public_ip]
}