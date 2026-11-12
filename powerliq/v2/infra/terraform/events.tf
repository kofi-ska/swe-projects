resource "aws_cloudwatch_event_bus" "domain" {
  name = "powerliq-${var.environment}"
}
resource "aws_sqs_queue" "market_price_dlq" {
  name                      = "powerliq-${var.environment}-market-price-dlq"
  message_retention_seconds = 1209600
  sqs_managed_sse_enabled   = true
}
resource "aws_sqs_queue" "market_price" {
  name                       = "powerliq-${var.environment}-market-price"
  visibility_timeout_seconds = 60
  message_retention_seconds  = 345600
  receive_wait_time_seconds  = 10
  sqs_managed_sse_enabled    = true
  redrive_policy = jsonencode({ deadLetterTargetArn = aws_sqs_queue.market_price_dlq.arn, maxReceiveCount = 5
  })
}
resource "aws_cloudwatch_event_rule" "market_price" {
  name           = "powerliq-${var.environment}-market-price"
  event_bus_name = aws_cloudwatch_event_bus.domain.name
  event_pattern = jsonencode({ "detail-type" = ["market-price.revised"]
  })
}
resource "aws_cloudwatch_event_target" "market_price" {
  rule           = aws_cloudwatch_event_rule.market_price.name
  event_bus_name = aws_cloudwatch_event_bus.domain.name
  arn            = aws_sqs_queue.market_price.arn
  target_id      = "market-data"
  dead_letter_config {
    arn = aws_sqs_queue.market_price_dlq.arn
  }
}
data "aws_iam_policy_document" "queue" {
  statement {
    effect    = "Allow"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.market_price.arn]
    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_cloudwatch_event_rule.market_price.arn]
    }
  }
}
resource "aws_sqs_queue_policy" "market_price" {
  queue_url = aws_sqs_queue.market_price.id
  policy    = data.aws_iam_policy_document.queue.json
}
resource "aws_cloudwatch_metric_alarm" "market_price_dlq" {
  alarm_name  = "powerliq-market-price-dlq"
  namespace   = "AWS/SQS"
  metric_name = "ApproximateNumberOfMessagesVisible"
  dimensions = { QueueName = aws_sqs_queue.market_price_dlq.name
  }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}
