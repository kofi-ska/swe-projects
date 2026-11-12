#!/bin/sh
set -eu

queue_url="$(awslocal sqs create-queue \
  --queue-name powerliq-market-price \
  --attributes VisibilityTimeout=30,ReceiveMessageWaitTimeSeconds=10 \
  --query QueueUrl \
  --output text)"
queue_arn="$(awslocal sqs get-queue-attributes \
  --queue-url "$queue_url" \
  --attribute-names QueueArn \
  --query Attributes.QueueArn \
  --output text)"

awslocal events create-event-bus --name powerliq-local
awslocal events put-rule \
  --event-bus-name powerliq-local \
  --name powerliq-market-price \
  --event-pattern '{"detail-type":["market-price.revised"]}'
awslocal events put-targets \
  --event-bus-name powerliq-local \
  --rule powerliq-market-price \
  --targets "Id=market-data,Arn=$queue_arn"
