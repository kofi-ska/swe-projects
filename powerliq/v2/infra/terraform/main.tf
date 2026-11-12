data "aws_availability_zones" "available" {
  state = "available"
}
data "aws_ssm_parameter" "ecs_ami" {
  name = "/aws/service/ecs/optimized-ami/amazon-linux-2023/arm64/recommended/image_id"
}
data "aws_caller_identity" "current" {}
locals {
  services = {
    position = { port_env = "POSITION_HTTP_PORT", db_url_env = "POSITION_DB_URL", db_user_env = "POSITION_DB_USER", db_password_env = "POSITION_DB_PASSWORD", db_schema = "position", path = "/api/v2/positions*", priority = 10
    }
    portfolio = { port_env = "PORTFOLIO_HTTP_PORT", db_url_env = "PORTFOLIO_DB_URL", db_user_env = "PORTFOLIO_DB_USER", db_password_env = "PORTFOLIO_DB_PASSWORD", db_schema = "portfolio", path = "/api/v2/portfolios*", priority = 20
    }
    market-ingestion = { port_env = "INGESTION_HTTP_PORT", db_url_env = "INGESTION_DB_URL", db_user_env = "INGESTION_DB_USER", db_password_env = "INGESTION_DB_PASSWORD", db_schema = "ingestion", path = "/api/v2/ingestion-runs*", priority = 30
    }
    market-data = { port_env = "MARKET_DATA_HTTP_PORT", db_url_env = "MARKET_DATA_DB_URL", db_user_env = "MARKET_DATA_DB_USER", db_password_env = "MARKET_DATA_DB_PASSWORD", db_schema = "market_data", path = "/api/v2/market-prices*", priority = 40
    }
    valuation = { port_env = "VALUATION_HTTP_PORT", db_url_env = "VALUATION_DB_URL", db_user_env = "VALUATION_DB_USER", db_password_env = "VALUATION_DB_PASSWORD", db_schema = "valuation", path = "/api/v2/valuations*", priority = 50
    }

  }
  estimated_monthly_usd = 74
}
resource "terraform_data" "budget_guard" {
  lifecycle {
    precondition {
      condition     = local.estimated_monthly_usd <= var.monthly_budget_usd
      error_message = "Estimated baseline exceeds configured monthly budget."
    }
  }
}
resource "aws_vpc" "this" {
  cidr_block           = "10.42.0.0/16"
  enable_dns_hostnames = true
}
resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
}
resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.this.id
  cidr_block              = "10.42.${count.index}.0/24"
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true
}
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
}
resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}
resource "aws_security_group" "alb" {
  name_prefix = "powerliq-alb-"
  vpc_id      = aws_vpc.this.id
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
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
}
resource "aws_security_group" "ecs" {
  name_prefix = "powerliq-ecs-"
  vpc_id      = aws_vpc.this.id
  ingress {
    from_port       = 32768
    to_port         = 65535
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
resource "aws_security_group" "db" {
  name_prefix = "powerliq-db-"
  vpc_id      = aws_vpc.this.id
  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.ecs.id]
  }
}
resource "random_password" "db" {
  length           = 32
  special          = true
  override_special = "!#$%&*+-=?"
}
resource "aws_db_subnet_group" "this" {
  name       = "powerliq-${var.environment}"
  subnet_ids = aws_subnet.public[*].id
}
resource "aws_db_instance" "this" {
  identifier                   = "powerliq-${var.environment}"
  engine                       = "postgres"
  engine_version               = "16"
  instance_class               = var.db_instance_class
  allocated_storage            = 20
  max_allocated_storage        = 30
  storage_type                 = "gp3"
  db_name                      = "powerliq"
  username                     = "powerliq_admin"
  password                     = random_password.db.result
  db_subnet_group_name         = aws_db_subnet_group.this.name
  vpc_security_group_ids       = [aws_security_group.db.id]
  publicly_accessible          = false
  multi_az                     = false
  backup_retention_period      = 1
  deletion_protection          = false
  skip_final_snapshot          = true
  storage_encrypted            = true
  performance_insights_enabled = false
  apply_immediately            = true
}
resource "aws_ecr_repository" "service" {
  for_each = local.services
  name     = "powerliq/${each.key}-service"
  image_scanning_configuration {
    scan_on_push = true
  }
  image_tag_mutability = "IMMUTABLE"
}
resource "aws_ecr_lifecycle_policy" "service" {
  for_each   = local.services
  repository = aws_ecr_repository.service[each.key].name
  policy     = jsonencode({ rules = [{ rulePriority = 1, description = "Retain ten images", selection = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 10 }, action = { type = "expire" } }] })
}
resource "aws_cognito_user_pool" "this" {
  name                = "powerliq-${var.environment}"
  deletion_protection = "INACTIVE"
  password_policy {
    minimum_length    = 14
    require_lowercase = true
    require_numbers   = true
    require_symbols   = true
    require_uppercase = true
  }
}
resource "aws_cognito_resource_server" "api" {
  identifier   = "powerliq"
  name         = "PowerLiq API"
  user_pool_id = aws_cognito_user_pool.this.id
  scope {
    scope_name        = "read"
    scope_description = "Read PowerLiq data"
  }
  scope {
    scope_name        = "write"
    scope_description = "Mutate PowerLiq data"
  }
  scope {
    scope_name        = "internal"
    scope_description = "Deliver internal events"
  }
}
resource "aws_cognito_user_pool_client" "service" {
  name                                 = "powerliq-service-client"
  user_pool_id                         = aws_cognito_user_pool.this.id
  generate_secret                      = true
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["client_credentials"]
  allowed_oauth_scopes                 = ["powerliq/read", "powerliq/write", "powerliq/internal"]
  supported_identity_providers         = ["COGNITO"]
  depends_on                           = [aws_cognito_resource_server.api]
}
resource "aws_cognito_user_pool_domain" "this" {
  domain       = "powerliq-${var.environment}-${data.aws_caller_identity.current.account_id}"
  user_pool_id = aws_cognito_user_pool.this.id
}
resource "aws_ecs_cluster" "this" {
  name = "powerliq-${var.environment}"
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}
resource "aws_iam_role" "ecs_instance" {
  name_prefix        = "powerliq-ecs-instance-"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" }, Action = "sts:AssumeRole" }] })
}
resource "aws_iam_role_policy_attachment" "ecs_instance" {
  role       = aws_iam_role.ecs_instance.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEC2ContainerServiceforEC2Role"
}
resource "aws_iam_instance_profile" "ecs" {
  name_prefix = "powerliq-ecs-"
  role        = aws_iam_role.ecs_instance.name
}
resource "aws_launch_template" "ecs" {
  name_prefix   = "powerliq-"
  image_id      = data.aws_ssm_parameter.ecs_ami.value
  instance_type = var.instance_type
  iam_instance_profile {
    name = aws_iam_instance_profile.ecs.name
  }
  network_interfaces {
    associate_public_ip_address = true
    security_groups             = [aws_security_group.ecs.id]
  }
  user_data = base64encode("#!/bin/bash\necho ECS_CLUSTER=${aws_ecs_cluster.this.name} >> /etc/ecs/ecs.config\necho ECS_ENABLE_TASK_IAM_ROLE=true >> /etc/ecs/ecs.config\n")
  metadata_options {
    http_tokens   = "required"
    http_endpoint = "enabled"
  }
  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      encrypted   = true
      volume_size = 30
      volume_type = "gp3"
    }
  }
}
resource "aws_autoscaling_group" "ecs" {
  name_prefix         = "powerliq-"
  min_size            = 1
  max_size            = 1
  desired_capacity    = 1
  vpc_zone_identifier = aws_subnet.public[*].id
  launch_template {
    id      = aws_launch_template.ecs.id
    version = "$Latest"
  }
}
resource "aws_lb" "this" {
  name                       = "powerliq-${var.environment}"
  load_balancer_type         = "application"
  subnets                    = aws_subnet.public[*].id
  security_groups            = [aws_security_group.alb.id]
  enable_deletion_protection = false
  drop_invalid_header_fields = true
}
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"
  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}
resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.this.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.certificate_arn
  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = "application/json"
      message_body = "{\"code\":\"NOT_FOUND\"}"
      status_code  = "404"
    }
  }
}
