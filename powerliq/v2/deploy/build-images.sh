#!/usr/bin/env sh
set -eu
registry="${1:-powerliq}"
tag="${2:-local}"
for service in position-service portfolio-service market-ingestion-service market-data-service valuation-service; do
  docker build --build-arg "SERVICE=${service}" -t "${registry}/${service}:${tag}" .
done
