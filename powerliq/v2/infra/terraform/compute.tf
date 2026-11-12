resource "aws_secretsmanager_secret" "db" {
  name_prefix             = "powerliq/${var.environment}/db-"
  recovery_window_in_days = 0
}
resource "aws_secretsmanager_secret_version" "db" {
  secret_id     = aws_secretsmanager_secret.db.id
  secret_string = random_password.db.result
}
resource "aws_iam_role" "task_execution" {
  name_prefix        = "powerliq-task-execution-"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "ecs-tasks.amazonaws.com" }, Action = "sts:AssumeRole" }] })
}
resource "aws_iam_role_policy_attachment" "task_execution" {
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}
resource "aws_iam_role_policy" "task_secret" {
  role   = aws_iam_role.task_execution.id
  policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Action = ["secretsmanager:GetSecretValue"], Resource = aws_secretsmanager_secret.db.arn }] })
}
resource "aws_iam_role" "task" {
  name_prefix        = "powerliq-task-"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "ecs-tasks.amazonaws.com" }, Action = "sts:AssumeRole" }] })
}
resource "aws_cloudwatch_log_group" "service" {
  for_each          = local.services
  name              = "/powerliq/${var.environment}/${each.key}"
  retention_in_days = var.log_retention_days
  kms_key_id        = aws_kms_key.logs.arn
}
resource "aws_kms_key" "logs" {
  description             = "PowerLiq CloudWatch log encryption"
  enable_key_rotation     = true
  deletion_window_in_days = 7
}
resource "aws_ecs_task_definition" "service" {
  for_each                 = local.services
  family                   = "powerliq-${each.key}"
  requires_compatibilities = ["EC2"]
  network_mode             = "bridge"
  cpu                      = "128"
  memory                   = "512"
  execution_role_arn       = aws_iam_role.task_execution.arn
  task_role_arn            = aws_iam_role.task.arn
  container_definitions    = jsonencode([{ name = each.key, image = "${aws_ecr_repository.service[each.key].repository_url}:${var.image_tag}", essential = true, cpu = 128, memory = 512, readonlyRootFilesystem = true, portMappings = [{ containerPort = 8080, hostPort = 0, protocol = "tcp" }], environment = [{ name = each.value.port_env, value = "8080" }, { name = each.value.db_url_env, value = "jdbc:postgresql://${aws_db_instance.this.address}:5432/powerliq?currentSchema=${each.value.db_schema}" }, { name = each.value.db_user_env, value = aws_db_instance.this.username }, { name = "SPRING_FLYWAY_SCHEMAS", value = each.value.db_schema }, { name = "SPRING_FLYWAY_CREATE_SCHEMAS", value = "true" }, { name = "SPRING_PROFILES_ACTIVE", value = "prod" }, { name = "POWERLIQ_JWT_ISSUER_URI", value = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.this.id}" }], secrets = [{ name = each.value.db_password_env, valueFrom = aws_secretsmanager_secret.db.arn }], healthCheck = { command = ["CMD-SHELL", "wget -q -O - http://localhost:8080/actuator/health/readiness || exit 1"], interval = 30, timeout = 5, retries = 3, startPeriod = 60 }, logConfiguration = { logDriver = "awslogs", options = { "awslogs-group" = aws_cloudwatch_log_group.service[each.key].name, "awslogs-region" = var.aws_region, "awslogs-stream-prefix" = "service" } } }])
}
resource "aws_lb_target_group" "service" {
  for_each             = local.services
  name                 = "pl-${replace(each.key, "market-", "m-")}"
  port                 = 8080
  protocol             = "HTTP"
  vpc_id               = aws_vpc.this.id
  target_type          = "instance"
  deregistration_delay = 30
  health_check {
    path                = "/actuator/health/readiness"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    matcher             = "200"
  }
}
resource "aws_lb_listener_rule" "service" {
  for_each     = local.services
  listener_arn = aws_lb_listener.https.arn
  priority     = each.value.priority
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.service[each.key].arn
  }
  condition {
    path_pattern {
      values = [each.value.path]
    }
  }
}
resource "aws_ecs_service" "service" {
  for_each                           = local.services
  name                               = each.key
  cluster                            = aws_ecs_cluster.this.id
  task_definition                    = aws_ecs_task_definition.service[each.key].arn
  desired_count                      = 1
  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100
  health_check_grace_period_seconds  = 90
  load_balancer {
    target_group_arn = aws_lb_target_group.service[each.key].arn
    container_name   = each.key
    container_port   = 8080
  }
  depends_on = [aws_autoscaling_group.ecs, aws_lb_listener_rule.service]
  lifecycle {
    ignore_changes = [desired_count]
  }
}
