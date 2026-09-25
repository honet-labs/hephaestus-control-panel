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

# Safe Self-Reload after git pull to ensure script executes new code from start
if [ -z "$HCP_UPDATE_RELOADED" ]; then
    export HCP_UPDATE_RELOADED=1
    echo -e "\n${BLUE}[1/4] Fetching latest release from GitHub...${NC}"
    git stash || true
    git fetch origin main
    git reset --hard origin/main
    echo -e "${GREEN}[OK] Repository updated to latest release. Reloading update script...${NC}"
    exec "$0" "$@"
fi

# Detect Compose Command
if docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    echo -e "${RED}[ERROR] Docker Compose command not found.${NC}"
    exit 1
fi

# Verify REGISTRY_MIRROR health (bypass AWS Public ECR 429 Too Many Requests)
if [ -f .env ]; then
    CURRENT_MIRROR=$(grep -E "^REGISTRY_MIRROR=" .env | cut -d'=' -f2- || true)
    if echo "$CURRENT_MIRROR" | grep -q "public.ecr.aws"; then
        echo -e "${YELLOW}[*] Testing AWS Public ECR mirror connectivity...${NC}"
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -m 5 "https://public.ecr.aws/v2/docker/library/alpine/manifests/3.20" 2>/dev/null || echo "000")
        if [ "$HTTP_CODE" = "429" ] || [ "$HTTP_CODE" = "000" ] || [ "$HTTP_CODE" = "403" ]; then
            echo -e "${YELLOW}[WARNING] AWS Public ECR returned HTTP ${HTTP_CODE} (Too Many Requests / Rate Limited).${NC}"
            echo -e "${CYAN}[*] Switching REGISTRY_MIRROR in .env to standard Docker Hub to prevent build errors...${NC}"
            sed -i 's|^REGISTRY_MIRROR=.*|REGISTRY_MIRROR=|g' .env
            export REGISTRY_MIRROR=""
        else
            echo -e "${GREEN}[OK] AWS Public ECR mirror is accessible.${NC}"
        fi
    fi
fi

# Ensure performance and resource tuning environment variables exist in .env
if [ -f .env ]; then
    if ! grep -q "GOMEMLIMIT" .env; then
        echo -e "${CYAN}[*] Applying recommended performance & resource tuning configurations to .env...${NC}"
        cat << 'EOF' >> .env

# Resource & Database Connection Pool Tuning
DB_MAX_CONNS=10
DB_MIN_CONNS=2
DB_MAX_CONN_IDLE_TIME=300
DB_MAX_CONN_LIFETIME=3600
POSTGRES_SHARED_BUFFERS=64MB
POSTGRES_WORK_MEM=4MB
POSTGRES_MAINTENANCE_WORK_MEM=16MB
POSTGRES_MAX_CONNECTIONS=40
POSTGRES_WAL_BUFFERS=4MB
GOMEMLIMIT=256MiB
GOGC=80
EOF
    fi
fi

echo -e "\n${BLUE}[2/4] Rebuilding & Upgrading Container Stack...${NC}"
BUILD_SUCCESS=false

# Attempt 1: Standard build (fast, leverages local layer cache)
if $COMPOSE_CMD build; then
    BUILD_SUCCESS=true
else
    echo -e "${YELLOW}[!] Standard build failed or encountered registry rate limit.${NC}"
fi

# Attempt 2 (Fallback): Try standard Docker Hub (REGISTRY_MIRROR="")
if [ "$BUILD_SUCCESS" = false ]; then
    echo -e "${YELLOW}[*] Fallback Attempt 1: Retrying with standard Docker Hub registry...${NC}"
    export REGISTRY_MIRROR=""
    if [ -f .env ]; then
        sed -i 's|^REGISTRY_MIRROR=.*|REGISTRY_MIRROR=|g' .env || true
    fi
    if REGISTRY_MIRROR="" $COMPOSE_CMD build; then
        BUILD_SUCCESS=true
    fi
fi

# Attempt 3 (Fallback): Try Google Container Registry mirror (mirror.gcr.io)
if [ "$BUILD_SUCCESS" = false ]; then
    echo -e "${YELLOW}[*] Fallback Attempt 2: Retrying with Google Container Registry mirror (mirror.gcr.io)...${NC}"
    export REGISTRY_MIRROR="mirror.gcr.io/library/"
    if [ -f .env ]; then
        sed -i 's|^REGISTRY_MIRROR=.*|REGISTRY_MIRROR=mirror.gcr.io/library/|g' .env || true
    fi
    if REGISTRY_MIRROR="mirror.gcr.io/library/" $COMPOSE_CMD build; then
        BUILD_SUCCESS=true
    fi
fi

# Attempt 4 (Fallback): Clean build (--no-cache)
if [ "$BUILD_SUCCESS" = false ]; then
    echo -e "${YELLOW}[*] Fallback Attempt 3: Retrying with clean build (--no-cache)...${NC}"
    $COMPOSE_CMD build --no-cache
fi

$COMPOSE_CMD up -d

echo -e "\n${BLUE}[3/4] Cleaning Up Dangling Docker Images...${NC}"
docker image prune -f || true

echo -e "\n${BLUE}[4/4] Verifying Service Health & Migrations...${NC}"
sleep 5
$COMPOSE_CMD ps

# Optional: If password passed as argument (e.g. ./update-version.sh 'P@ssw0rd294!' or ./update-versi.sh 'P@ssw0rd294!')
NEW_PASS="$1"
TARGET_USER="${2:-admin}"
if [ -n "$NEW_PASS" ]; then
    echo -e "\n${CYAN}[*] Setting password for user '${TARGET_USER}'...${NC}"
    docker exec -i hephaestus-engine /app/hephaestus reset-password -u "$TARGET_USER" -p "$NEW_PASS" || true
fi

# Get Host IP
SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$SERVER_IP" ]; then
    SERVER_IP="localhost"
fi

echo -e "\n${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}[OK] HEPHAESTUS CONTROL PANEL (HCP) UPDATED SUCCESSFULLY!${NC}"
echo -e "${GREEN}==============================================================================${NC}"
echo -e "Web Interface URL    : ${CYAN}http://${SERVER_IP}${NC}"
echo -e "Active Containers    : hephaestus-panel, hephaestus-engine, hephaestus-database"
echo -e "Container Logs       : ${CYAN}docker compose logs -f${NC}"
echo -e "Reset Admin Password : ${YELLOW}./reset-password.sh admin '<password_baru>'${NC}"
echo "=============================================================================="
