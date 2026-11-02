#!/usr/bin/env bash
set -e
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"
openssl req -x509 -newkey rsa:4096 -nodes -days 3650 \
  -keyout key.pem -out cert.pem \
  -subj "/C=XX/ST=Local/L=Local/O=FLARE/CN=127.0.0.1" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
chmod 600 key.pem
echo "[*] Generated key.pem/cert.pem"
