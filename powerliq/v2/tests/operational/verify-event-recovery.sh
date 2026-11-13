#!/bin/sh
set -eu

market_url="${POWERLIQ_MARKET_DATA_URL:-http://localhost:8084}"
provider="RECOVERY-TEST"
delivery_start="2027-01-15T00:00:00Z"
delivery_end="2027-01-15T00:30:00Z"
event_suffix="$(printf '%012d' "$(date +%s)")"
event_id="11111111-1111-1111-1111-$event_suffix"
occurred_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
detail="{\"eventId\":\"$event_id\",\"eventType\":\"market-price.revised\",\"eventVersion\":1,\"occurredAt\":\"$occurred_at\",\"source\":\"recovery-test\",\"correlationId\":\"22222222-2222-2222-2222-222222222222\",\"causationId\":null,\"aggregateId\":\"$event_id\",\"partitionKey\":\"$delivery_start\",\"payload\":{\"provider\":\"$provider\",\"deliveryStart\":\"$delivery_start\",\"deliveryEnd\":\"$delivery_end\",\"priceGbpPerMWh\":73.25,\"sourceRevision\":\"1\",\"sourceTimestamp\":\"$occurred_at\",\"retrievedAt\":\"$occurred_at\"}}"
entry="[{\"Source\":\"com.powerliq.recovery-test\",\"DetailType\":\"market-price.revised\",\"Detail\":$(printf '%s' "$detail" | jq -Rs .),\"EventBusName\":\"powerliq-local\"}]"

docker compose stop localstack >/dev/null
curl --fail --silent --max-time 5 "$market_url/actuator/health/readiness" >/dev/null
docker compose start localstack >/dev/null

until docker compose exec -T localstack awslocal events describe-event-bus --name powerliq-local >/dev/null 2>&1; do sleep 1; done
until docker compose exec -T localstack awslocal sqs get-queue-url --queue-name powerliq-market-price >/dev/null 2>&1; do sleep 1; done
docker compose exec -T localstack awslocal sqs purge-queue --queue-url http://localstack:4566/000000000000/powerliq-market-price >/dev/null
docker compose exec -T localstack awslocal events put-events --entries "$entry" >/dev/null

attempt=0
until curl --fail --silent --get \
  --data-urlencode "provider=$provider" \
  --data-urlencode "deliveryStart=$delivery_start" \
  "$market_url/api/v2/market-prices/latest" | grep -q '73.25'; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 30 ] || { echo "event was not consumed after dependency recovery" >&2; exit 1; }
  sleep 1
done

echo "market-data remained healthy and consumed the queued event after LocalStack recovery"
