resource "aws_security_group" "wiresteward-lb" {
  name        = "${local.name}-lb"
  description = "Allows HTTPS traffic from anywhere"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${local.name}-lb"
  }
}

resource "aws_lb" "wiresteward" {
  name               = local.name
  load_balancer_type = "application"
  subnets            = var.subnet_ids
  security_groups    = [aws_security_group.wiresteward-lb.id]

  tags = {
    Name = local.name
  }
}

resource "aws_acm_certificate" "cert" {
  domain_name       = var.wiresteward_endpoint
  validation_method = "DNS"
}

# Since `domain_validation_options` is a Set, we convert it to a list to be
# able to reference 0 index entry.
# This is safe enough with a single element, but we had had >1 elements, we
# would need something else here.
locals {
  validation_options_list = tolist(aws_acm_certificate.cert.domain_validation_options)
}

resource "aws_route53_record" "cert_validation" {
  name    = local.validation_options_list.0.resource_record_name
  type    = local.validation_options_list.0.resource_record_type
  zone_id = var.dns_zone_id
  records = [local.validation_options_list.0.resource_record_value]
  ttl     = 60
}

resource "aws_acm_certificate_validation" "cert" {
  certificate_arn         = aws_acm_certificate.cert.arn
  validation_record_fqdns = [aws_route53_record.cert_validation.fqdn]
}

resource "aws_lb_listener" "wiresteward_443" {
  load_balancer_arn = aws_lb.wiresteward.arn
  port              = "443"
  protocol          = "HTTPS"
  certificate_arn   = aws_acm_certificate_validation.cert.certificate_arn

  default_action {
    target_group_arn = aws_lb_target_group.wiresteward_8080.arn
    type             = "forward"
  }
}

resource "aws_lb_target_group" "wiresteward_8080" {
  vpc_id   = var.vpc_id
  port     = 8080
  protocol = "HTTP"

  health_check {
    matcher = "404"
  }

  # https://github.com/terraform-providers/terraform-provider-aws/issues/636#issuecomment-397459646
  lifecycle {
    create_before_destroy = true
  }

  tags = {
    Name = local.name
  }
}

resource "aws_lb_target_group_attachment" "peer" {
  count            = local.instance_count
  target_group_arn = aws_lb_target_group.wiresteward_8080.arn
  target_id        = aws_instance.peer[count.index].id
}

resource "aws_route53_record" "wiresteward" {
  zone_id = var.dns_zone_id
  name    = var.wiresteward_endpoint
  type    = "CNAME"
  ttl     = "600"
  records = [aws_lb.wiresteward.dns_name]
}
