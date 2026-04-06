#!/usr/bin/env bash
set -euo pipefail

KEY_DIR="./keys"
mkdir -p "$KEY_DIR"

echo "Generating EC P-256 key pair for BFF OIDC client..."
openssl ecparam -genkey -name prime256v1 -noout -out "$KEY_DIR/bff-private.pem"
openssl ec -in "$KEY_DIR/bff-private.pem" -pubout -out "$KEY_DIR/bff-public.pem"

openssl ec -in "$KEY_DIR/bff-private.pem" -pubout -outform DER 2>/dev/null | \
  openssl base64 -A > "$KEY_DIR/bff-public.b64"

echo "Keys generated in $KEY_DIR/"
echo "  Private: $KEY_DIR/bff-private.pem"
echo "  Public:  $KEY_DIR/bff-public.pem"
echo ""
echo "Add $KEY_DIR/ to .gitignore (NEVER commit private keys)"
