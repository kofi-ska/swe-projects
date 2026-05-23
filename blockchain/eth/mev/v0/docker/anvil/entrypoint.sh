#!/bin/sh
set -e

CHAIN_NAME=${CHAIN_NAME:-anvil}
CHAIN_ID=${CHAIN_ID:-1337}
PORT=${PORT:-8545}
BLOCK_TIME=${BLOCK_TIME:-2}
MNEMONIC=${MNEMONIC:-"test test test test test test test test test test test junk"}

echo "Launching $CHAIN_NAME chain on port $PORT (id=$CHAIN_ID)..."

exec anvil \
  --chain-id $CHAIN_ID \
  --port $PORT \
  --block-time $BLOCK_TIME \
  --mnemonic "$MNEMONIC" \
  --host 0.0.0.0
