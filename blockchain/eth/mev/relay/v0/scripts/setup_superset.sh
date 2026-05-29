#!/usr/bin/env bash
set -e

echo "[Superset] Initializing database and admin user..."
superset db upgrade
superset fab create-admin \
    --username admin \
    --firstname relay \
    --lastname operator \
    --email admin@relay.local \
    --password admin123

echo "[Superset] Loading examples and setting up roles..."
superset init

echo "[Superset] Ready on port 8088"
