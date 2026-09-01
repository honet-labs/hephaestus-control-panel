#!/usr/bin/env bash
# ==============================================================================
# Hephaestus Control Panel (HCP) - System & Version Updater
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
echo -e "${BOLD}Hephaestus Control Panel (HCP) - Updater${NC}"
echo "------------------------------------------------------------------------------"

# Check Root Privileges
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}[ERROR] This update script must be run as root or with sudo.${NC}"
    exit 1
fi

# Locate Installation Directory
INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$INSTALL_DIR"

echo -e "\n${BLUE}[1/4] Fetching latest release from GitHub...${NC}"
# Stash any local uncommitted files to prevent git merge conflicts
git stash || true
git fetch origin main
git reset --hard origin/main

# Detect Compose Command
if docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    echo -e "${RED}[ERROR] Docker Compose command not found.${NC}"
    exit 1
fi

echo -e "\n${BLUE}[2/4] Rebuilding & Upgrading Container Stack...${NC}"
$COMPOSE_CMD build
$COMPOSE_CMD up -d

echo -e "\n${BLUE}[3/4] Cleaning Up Dangling Docker Images...${NC}"
docker image prune -f || true

echo -e "\n${BLUE}[4/4] Verifying Service Health...${NC}"
sleep 4
$COMPOSE_CMD ps

# Get Host IP
SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$SERVER_IP" ]; then
    SERVER_IP="localhost"
fi

echo -e "\n${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}[OK] HEPHAESTUS CONTROL PANEL (HCP) UPDATED SUCCESSFULLY!${NC}"
echo -e "${GREEN}==============================================================================${NC}"
echo -e "Web Interface URL    : ${CYAN}http://${SERVER_IP}${NC}"
echo -e "Active Containers    : Frontend (Nginx), Backend (Go), PostgreSQL 16"
echo -e "Container Logs       : ${CYAN}docker compose logs -f${NC}"
echo "=============================================================================="
