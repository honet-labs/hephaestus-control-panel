#!/usr/bin/env bash
# ==============================================================================
# Hephaestus Control Panel (HCP) - Password Reset Utility
# ==============================================================================
set -e

USERNAME="${1:-admin}"
NEW_PASSWORD="$2"

echo "======================================================================"
echo " HEPHAESTUS CONTROL PANEL - USER PASSWORD RESET"
echo "======================================================================"

# 1. Check if running inside docker compose
if command -v docker &> /dev/null && docker compose ps --services 2>/dev/null | grep -q "engine"; then
    echo "[*] Connecting to Hephaestus Engine container..."
    if [ -n "$NEW_PASSWORD" ]; then
        docker compose exec engine /app/hephaestus reset-password -u "$USERNAME" -p "$NEW_PASSWORD"
    else
        docker compose exec -it engine /app/hephaestus reset-password -u "$USERNAME"
    fi
    exit 0
fi

# 2. Check if local compiled binaries exist
if [ -f "./hephaestus" ]; then
    ./hephaestus reset-password -u "$USERNAME" ${NEW_PASSWORD:+-p "$NEW_PASSWORD"}
elif [ -f "./hcp-cli" ]; then
    ./hcp-cli reset-password -u "$USERNAME" ${NEW_PASSWORD:+-p "$NEW_PASSWORD"}
elif command -v go &> /dev/null; then
    go run ./cmd/cli reset-password -u "$USERNAME" ${NEW_PASSWORD:+-p "$NEW_PASSWORD"}
else
    echo "[ERROR] Could not find running Docker container 'engine' or local Go environment."
    echo "Make sure the services are running with: docker compose up -d"
    echo "Or execute directly inside container:"
    echo "  docker compose exec -it engine /app/hephaestus reset-password -u $USERNAME"
    exit 1
fi
