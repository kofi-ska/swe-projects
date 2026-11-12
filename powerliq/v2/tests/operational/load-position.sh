#!/bin/sh
set -eu

base_url="${POWERLIQ_POSITION_URL:-http://localhost:8081}"
requests="${POWERLIQ_LOAD_REQUESTS:-500}"
concurrency="${POWERLIQ_LOAD_CONCURRENCY:-20}"
max_error_percent="${POWERLIQ_LOAD_MAX_ERROR_PERCENT:-1}"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/powerliq-load.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT INT TERM

case "$requests:$concurrency:$max_error_percent" in
  *[!0-9:]*|0:*|*:0:*) echo "load controls must be positive integers" >&2; exit 2 ;;
esac

run_one() {
  sequence="$1"
  portfolio_id="$(printf '00000000-0000-0000-0000-%012d' "$sequence")"
  payload="{\"portfolioId\":\"$portfolio_id\",\"side\":\"LONG\",\"contractPriceGbpPerMWh\":50,\"volumeMWh\":1,\"settlementDate\":\"2027-01-15\",\"settlementPeriod\":1}"
  curl --silent --show-error --output /dev/null \
    --write-out '%{http_code}\n' \
    --connect-timeout 2 --max-time 10 \
    --header 'Content-Type: application/json' \
    --header "Idempotency-Key: load-$sequence" \
    --data "$payload" \
    "$base_url/api/v2/positions" > "$work_dir/$sequence.status" || printf '000\n' > "$work_dir/$sequence.status"
}
export base_url work_dir
export -f run_one 2>/dev/null || true

start="$(perl -MTime::HiRes=time -e 'printf "%.6f", time')"
sequence=1
active=0
while [ "$sequence" -le "$requests" ]; do
  run_one "$sequence" &
  active=$((active + 1))
  if [ "$active" -ge "$concurrency" ]; then
    wait
    active=0
  fi
  sequence=$((sequence + 1))
done
wait
end="$(perl -MTime::HiRes=time -e 'printf "%.6f", time')"

successes="$(awk '$1 == 201 {count++} END {print count+0}' "$work_dir"/*.status)"
failures=$((requests - successes))
error_percent="$(awk -v failures="$failures" -v total="$requests" 'BEGIN {printf "%.2f", failures*100/total}')"
elapsed="$(awk -v start="$start" -v end="$end" 'BEGIN {printf "%.3f", end-start}')"
throughput="$(awk -v total="$requests" -v elapsed="$elapsed" 'BEGIN {printf "%.2f", total/elapsed}')"

printf 'requests=%s successes=%s failures=%s error_percent=%s elapsed_seconds=%s throughput_rps=%s\n' \
  "$requests" "$successes" "$failures" "$error_percent" "$elapsed" "$throughput"

awk -v actual="$error_percent" -v maximum="$max_error_percent" 'BEGIN {exit !(actual <= maximum)}'
