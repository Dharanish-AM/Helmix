#!/usr/bin/env sh

set -eu

KEY_DIR="${1:-./certs}"
PRIVATE_KEY_PATH="$KEY_DIR/jwt-private.pem"
PUBLIC_KEY_PATH="$KEY_DIR/jwt-public.pem"

mkdir -p "$KEY_DIR"

if [ -f "$PRIVATE_KEY_PATH" ] && [ -f "$PUBLIC_KEY_PATH" ]; then
	echo "JWT key pair already exists in $KEY_DIR"
	exit 0
fi

umask 077
openssl genrsa -out "$PRIVATE_KEY_PATH" 2048 >/dev/null 2>&1
openssl rsa -in "$PRIVATE_KEY_PATH" -pubout -out "$PUBLIC_KEY_PATH" >/dev/null 2>&1

chmod 600 "$PRIVATE_KEY_PATH"
chmod 644 "$PUBLIC_KEY_PATH"

echo "Generated JWT key pair in $KEY_DIR"