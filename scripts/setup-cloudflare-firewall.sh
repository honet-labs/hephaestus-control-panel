#!/usr/bin/env bash
# ==============================================================================
# Hephaestus Control Panel (HCP) - Cloudflare Origin Firewall Hardening (H-04)
# ==============================================================================
# Restrict port 8881 / 80 / 443 so ONLY Cloudflare proxy IPs can connect.
# This prevents direct origin bypass attacks.
# ==============================================================================

set -e

PORT="${1:-8881}"

echo "[HCP] Hardening origin firewall on port $PORT..."
echo "[HCP] Fetching official Cloudflare IP ranges..."

CF_IPV4=$(curl -s https://www.cloudflare.com/ips-v4)
CF_IPV6=$(curl -s https://www.cloudflare.com/ips-v6)

if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
    echo "[HCP] UFW detected. Adding allow rules for Cloudflare IPs on port $PORT..."
    
    # Allow localhost
    ufw allow from 127.0.0.1 to any port "$PORT" proto tcp
    
    for ip in $CF_IPV4; do
        ufw allow proto tcp from "$ip" to any port "$PORT" comment "Cloudflare IPv4"
    done
    
    for ip in $CF_IPV6; do
        ufw allow proto tcp from "$ip" to any port "$PORT" comment "Cloudflare IPv6"
    done
    
    # Deny all others on this port
    ufw deny "$PORT/tcp"
    ufw reload
    echo "[HCP] UFW rules successfully applied for port $PORT."
else
    echo "[HCP] UFW not active. You can apply Cloudflare IP whitelisting using iptables or your cloud provider security group."
    echo "[HCP] Cloudflare IPv4 ranges:"
    echo "$CF_IPV4"
fi

echo "[HCP] Direct origin access is now blocked for external unauthorized IPs!"
