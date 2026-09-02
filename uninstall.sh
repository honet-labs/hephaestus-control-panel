#!/usr/bin/env bash
# ==============================================================================
# Hephaestus Control Panel (HCP) - Uninstallation Script
# ==============================================================================

set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Print Banner
echo -e "${CYAN}"
echo "██╗  ██╗███████╗██████╗ ██╗  ██╗ █████╗ ███████╗███████╗████████╗██╗   ██╗███████╗"
echo "██║  ██║██╔════╝██╔══██╗██║  ██║██╔══██╗██╔════╝██╔════╝╚══██╔══╝██║   ██║██╔════╝"
echo "███████║█████╗  ██████╔╝███████║███████║█████╗  ███████╗   ██║   ██║   ██║███████╗"
echo "██╔══██║██╔══╝  ██╔═══╝ ██╔══██║██╔══██║██╔══╝  ╚════██║   ██║   ██║   ██║╚════██║"
echo "██║  ██║███████╗██║     ██║  ██║██║  ██║███████╗███████║   ██║   ╚██████╔╝███████║"
echo "╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝   ╚═╝    ╚═════╝ ╚══════╝"
echo -e "${NC}"
echo -e "${BOLD}Hephaestus Control Panel (HCP) - Uninstaller${NC}"
echo "------------------------------------------------------------------------------"

# Check Root Privileges
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}[ERROR] This uninstallation script must be run as root or with sudo.${NC}"
    exit 1
fi

INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$INSTALL_DIR"

# Detect Compose Command
if docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    COMPOSE_CMD=""
fi

echo -e "\n${BLUE}[1/3] Stopping and removing all Hephaestus containers...${NC}"
if [ -n "$COMPOSE_CMD" ] && [ -f docker-compose.yml ]; then
    $COMPOSE_CMD down --volumes --remove-orphans || true
fi

# Explicitly ensure any leftover containers are stopped
docker stop hephaestus-frontend hephaestus-backend hephaestus-postgres 2>/dev/null || true
docker rm -f hephaestus-frontend hephaestus-backend hephaestus-postgres 2>/dev/null || true

echo -e "\n${BLUE}[2/3] Cleaning up Docker images and networks...${NC}"
docker rmi -f hephaestus-control-panel-frontend hephaestus-control-panel-backend 2>/dev/null || true
docker network rm hephaestus-control-panel_hephaestus_net 2>/dev/null || true
docker volume rm hephaestus-control-panel_pg_data hephaestus-control-panel_app_data hephaestus-control-panel_app_logs hephaestus-control-panel_app_backups 2>/dev/null || true

echo -e "\n${BLUE}[3/3] Cleaning up local environment files...${NC}"
rm -f .env 2>/dev/null || true

echo -e "\n${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}[OK] HEPHAESTUS CONTROL PANEL (HCP) UNINSTALLED SUCCESSFULLY!${NC}"
echo -e "${GREEN}==============================================================================${NC}"
echo -e "All containers, volumes, and temporary networks have been removed."
echo -e "To completely delete the repository directory, run: ${CYAN}rm -rf $INSTALL_DIR${NC}"
echo "=============================================================================="
