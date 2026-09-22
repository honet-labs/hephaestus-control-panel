#!/usr/bin/env bash
# ==============================================================================
# Hephaestus Control Panel (HCP) - Origin TLS & Cloudflare Origin CA Setup (H-04)
# ==============================================================================

set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CERTS_DIR="$DIR/certs"
mkdir -p "$CERTS_DIR"

echo "[HCP] Setting up Origin TLS Certificate in $CERTS_DIR..."

if [ -f "$CERTS_DIR/cert.pem" ] && [ -f "$CERTS_DIR/key.pem" ]; then
    echo "[HCP] Existing TLS certificate and key found in $CERTS_DIR."
else
    echo "[HCP] Generating robust self-signed Origin CA certificate (Valid 10 years)..."
    openssl req -x509 -nodes -days 3650 -newkey rsa:4096 \
        -keyout "$CERTS_DIR/key.pem" \
        -out "$CERTS_DIR/cert.pem" \
        -subj "/C=ID/ST=Jakarta/L=Jakarta/O=HONET Labs/OU=HCP Infrastructure/CN=hephaestus-origin" \
        -addext "subjectAltName=DNS:localhost,IP:127.0.0.1,IP:10.20.3.29"

    chmod 600 "$CERTS_DIR/key.pem"
    chmod 644 "$CERTS_DIR/cert.pem"
    echo "[HCP] Certificate generated successfully."
fi

echo ""
echo "=============================================================================="
echo "Origin TLS Setup Complete for H-04"
echo "1. Place Cloudflare Origin CA certificate into: $CERTS_DIR/cert.pem"
echo "2. Place Cloudflare Origin CA private key into:   $CERTS_DIR/key.pem"
echo "3. Restart HCP containers via: ./update-versi.sh"
echo "=============================================================================="
