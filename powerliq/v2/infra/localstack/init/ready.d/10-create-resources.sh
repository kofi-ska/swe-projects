#!/bin/sh
set -eu

awslocal kinesis create-stream --stream-name powerliq-market-events --shard-count 2
awslocal sqs create-queue --queue-name powerliq-ingestion-retry
awslocal sqs create-queue --queue-name powerliq-ingestion-dlq
awslocal sqs create-queue --queue-name powerliq-projection-retry
awslocal sqs create-queue --queue-name powerliq-projection-dlq
