#!/usr/bin/env bash
# ==============================================================================
# Hephaestus Control Panel (HCP) - Cloudflare Tunnel Setup (H-04 Recommended)
# ==============================================================================

set -e

TUNNEL_TOKEN="${1:-$CLOUDFLARE_TUNNEL_TOKEN}"

if [ -z "$TUNNEL_TOKEN" ]; then
    echo "Usage: $0 <CLOUDFLARE_TUNNEL_TOKEN>"
    echo "Or set CLOUDFLARE_TUNNEL_TOKEN environment variable."
    echo ""
    echo "Cloudflare Tunnel eliminates open public ports (like 8881) completely."
    echo "Steps:"
    echo "  1. In Cloudflare Zero Trust Dashboard -> Networks -> Tunnels -> Create Tunnel."
    echo "  2. Choose 'Cloudflared' and copy the tunnel token."
    echo "  3. Run: ./scripts/setup-cloudflare-tunnel.sh <token>"
    exit 1
fi

echo "[HCP] Setting up Cloudflare Tunnel container with token..."
docker run -d --name hcp-cloudflared \
    --restart unless-stopped \
    --network host \
    cloudflare/cloudflared:latest tunnel --no-autoupdate run --token "$TUNNEL_TOKEN"

echo "[HCP] Cloudflare Tunnel successfully running. You can now close port 8881 in your public firewall!"
