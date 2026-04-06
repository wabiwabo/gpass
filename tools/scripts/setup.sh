#!/usr/bin/env bash
set -euo pipefail

echo "=== GarudaPass Development Setup ==="

for cmd in docker pnpm go openssl; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "ERROR: $cmd is required but not installed."
    exit 1
  fi
done

if [ ! -f .env ]; then
  cp .env.example .env
  echo "Created .env from .env.example"
fi

echo "Installing Node.js dependencies..."
pnpm install

echo "Installing Go dependencies..."
cd apps/bff && go mod download && cd ../..

echo "Generating OIDC client keys..."
./tools/scripts/generate-keys.sh

echo "Starting Docker services..."
docker compose up -d

echo "Waiting for services to be healthy..."
sleep 10

echo "Creating Kafka topics..."
chmod +x infrastructure/kafka/topics.sh
KAFKA_BROKERS=localhost:19092 ./infrastructure/kafka/topics.sh

echo ""
echo "=== Setup complete! ==="
echo "Keycloak Admin: http://localhost:8080 (admin/admin)"
echo "Kong Proxy:     http://localhost:8000"
echo "PostgreSQL:     localhost:5432"
echo "Redis:          localhost:6379"
echo "Kafka:          localhost:19092"
