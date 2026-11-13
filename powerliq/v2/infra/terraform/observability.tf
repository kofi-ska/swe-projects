resource "aws_sns_topic" "alerts" {
  name              = "powerliq-${var.environment}-alerts"
  kms_master_key_id = "alias/aws/sns"
}
resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}
resource "aws_cloudwatch_metric_alarm" "service_cpu" {
  for_each            = local.services
  alarm_name          = "powerliq-${each.key}-high-cpu"
  namespace           = "AWS/ECS"
  metric_name         = "CPUUtilization"
  dimensions          = { ClusterName = aws_ecs_cluster.this.name, ServiceName = aws_ecs_service.service[each.key].name }
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 3
  datapoints_to_alarm = 2
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
resource "aws_cloudwatch_metric_alarm" "service_memory" {
  for_each            = local.services
  alarm_name          = "powerliq-${each.key}-high-memory"
  namespace           = "AWS/ECS"
  metric_name         = "MemoryUtilization"
  dimensions          = { ClusterName = aws_ecs_cluster.this.name, ServiceName = aws_ecs_service.service[each.key].name }
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 3
  datapoints_to_alarm = 2
  threshold           = 85
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
resource "aws_cloudwatch_metric_alarm" "unhealthy_hosts" {
  for_each            = local.services
  alarm_name          = "powerliq-${each.key}-unhealthy"
  namespace           = "AWS/ApplicationELB"
  metric_name         = "UnHealthyHostCount"
  dimensions          = { LoadBalancer = aws_lb.this.arn_suffix, TargetGroup = aws_lb_target_group.service[each.key].arn_suffix }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 3
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
resource "aws_cloudwatch_metric_alarm" "target_5xx" {
  for_each            = local.services
  alarm_name          = "powerliq-${each.key}-5xx"
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_Target_5XX_Count"
  dimensions          = { LoadBalancer = aws_lb.this.arn_suffix, TargetGroup = aws_lb_target_group.service[each.key].arn_suffix }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 2
  threshold           = 5
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
resource "aws_cloudwatch_metric_alarm" "db_connections" {
  alarm_name          = "powerliq-db-connections"
  namespace           = "AWS/RDS"
  metric_name         = "DatabaseConnections"
  dimensions          = { DBInstanceIdentifier = aws_db_instance.this.identifier }
  statistic           = "Maximum"
  period              = 300
  evaluation_periods  = 3
  threshold           = 70
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
resource "aws_cloudwatch_metric_alarm" "db_storage" {
  alarm_name          = "powerliq-db-low-storage"
  namespace           = "AWS/RDS"
  metric_name         = "FreeStorageSpace"
  dimensions          = { DBInstanceIdentifier = aws_db_instance.this.identifier }
  statistic           = "Minimum"
  period              = 300
  evaluation_periods  = 2
  threshold           = 5368709120
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
resource "aws_cloudwatch_dashboard" "operations" {
  dashboard_name = "PowerLiq-${var.environment}"
  dashboard_body = jsonencode({ widgets = concat([for name, service in local.services : { type = "metric", x = 0, y = service.priority / 10 * 6, width = 12, height = 6, properties = { title = "${name} CPU / Memory", region = var.aws_region, period = 300, metrics = [["AWS/ECS", "CPUUtilization", "ClusterName", aws_ecs_cluster.this.name, "ServiceName", aws_ecs_service.service[name].name], [".", "MemoryUtilization", ".", ".", ".", "."]] }
  }], [{ type = "metric", x = 12, y = 0, width = 12, height = 6, properties = { title = "RDS", region = var.aws_region, metrics = [["AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", aws_db_instance.this.identifier], [".", "DatabaseConnections", ".", "."]] } }, { type = "log", x = 12, y = 6, width = 12, height = 12, properties = { title = "Recent errors", region = var.aws_region, query = "SOURCE '/powerliq/${var.environment}/position' | SOURCE '/powerliq/${var.environment}/portfolio' | SOURCE '/powerliq/${var.environment}/market-ingestion' | SOURCE '/powerliq/${var.environment}/market-data' | SOURCE '/powerliq/${var.environment}/valuation' | fields @timestamp, @log, @message | filter @message like /ERROR|Exception/ | sort @timestamp desc | limit 100", view = "table" } }]) })
}
resource "aws_budgets_budget" "monthly" {
  name         = "powerliq-${var.environment}-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.monthly_budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"
  cost_filter {
    name   = "TagKeyValue"
    values = ["user:Project$PowerLiq"]
  }
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.alert_email]
  }
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.alert_email]
  }
}
