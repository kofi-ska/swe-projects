#!/bin/sh
set -e

echo "Waiting for Geth RPC to be ready..."
sleep 5

echo "Injecting random transactions into txpool..."
for i in $(seq 1 5); do
  VALUE=$((RANDOM % 9900 + 100))
  geth attach http://geth:8545 --exec "eth.sendTransaction({from: eth.accounts[0], to: eth.accounts[0], value: web3.toWei($VALUE, 'wei')})" >/dev/null 2>&1
  echo "Injected tx #$i (value: $VALUE wei)"
done

echo "Transaction seeding complete."
