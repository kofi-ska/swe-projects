#!/bin/sh
set -e

echo "Starting Geth dev node..."
exec geth "$@"
