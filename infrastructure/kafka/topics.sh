#!/usr/bin/env bash
set -euo pipefail

BROKER="${KAFKA_BROKERS:-localhost:19092}"

topics=(
  "audit.auth"
  "audit.consent"
  "audit.signing"
  "audit.corporate"
  "events.user.lifecycle"
  "events.entity.lifecycle"
)

for topic in "${topics[@]}"; do
  rpk topic create "$topic" \
    --brokers "$BROKER" \
    --partitions 3 \
    --replicas 1 \
    2>/dev/null || echo "Topic $topic already exists"
done

echo "All Kafka topics created."
