#!/bin/sh
set -e

# Ensure certificates directory exists
mkdir -p /etc/nginx/certs

# Generate self-signed certificate if none exists (H-04 Origin TLS)
if [ ! -f /etc/nginx/certs/cert.pem ] || [ ! -f /etc/nginx/certs/key.pem ]; then
    echo "[HCP] No TLS certificate found in /etc/nginx/certs. Generating default self-signed Origin certificate..."
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout /etc/nginx/certs/key.pem \
        -out /etc/nginx/certs/cert.pem \
        -subj "/C=ID/ST=Jakarta/L=Jakarta/O=HONET Labs/CN=hephaestus-panel"
    chmod 600 /etc/nginx/certs/key.pem
    chmod 644 /etc/nginx/certs/cert.pem
    echo "[HCP] Default self-signed TLS certificate generated successfully."
fi

exec "$@"
