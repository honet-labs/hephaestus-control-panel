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
SCRIPT_SRC="${BASH_SOURCE[0]:-$0}"
INSTALL_DIR="$(cd "$(dirname "$SCRIPT_SRC")" && pwd)"
SCRIPT_FILE="$INSTALL_DIR/$(basename "$SCRIPT_SRC")"
cd "$INSTALL_DIR"

# Safe Self-Reload after git pull to ensure script executes new code from start
if [ -z "$HCP_UPDATE_RELOADED" ]; then
    export HCP_UPDATE_RELOADED=1
    echo -e "\n${BLUE}[1/4] Fetching latest release from GitHub...${NC}"
    git stash || true
    if git fetch origin main 2>/dev/null; then
        git reset --hard origin/main || true
        echo -e "${GREEN}[OK] Repository updated to latest release. Reloading update script...${NC}"
    else
        echo -e "${YELLOW}[!] Could not fetch from origin/main. Continuing with local version...${NC}"
    fi
    chmod +x "$SCRIPT_FILE" 2>/dev/null || true
    exec bash "$SCRIPT_FILE" "$@"
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

# Sanitize deprecated or dead mirrors from .env (mirror.gcr.io is dead and causes build hangs)
if [ -f .env ]; then
    CURRENT_MIRROR=$(grep -E "^REGISTRY_MIRROR=" .env | cut -d'=' -f2- || true)
    if echo "$CURRENT_MIRROR" | grep -qE "mirror\.gcr\.io|public\.ecr\.aws"; then
        echo -e "${YELLOW}[*] Removing deprecated/rate-limited REGISTRY_MIRROR (${CURRENT_MIRROR}) from .env...${NC}"
        sed -i 's|^REGISTRY_MIRROR=.*|REGISTRY_MIRROR=|g' .env
        export REGISTRY_MIRROR=""
    fi
fi

# Ensure security encryption key and performance tuning environment variables exist in .env
if [ -f .env ]; then
    if ! grep -q "APP_ENCRYPTION_KEY" .env; then
        echo -e "${CYAN}[*] Generating secure 64-char hex APP_ENCRYPTION_KEY in .env...${NC}"
        NEW_KEY=$(openssl rand -hex 32 2>/dev/null || cat /proc/sys/kernel/random/uuid | tr -d '-' | head -c 64)
        echo "" >> .env
        echo "# Security Key (64-character Hexadecimal for AES-256-GCM)" >> .env
        echo "APP_ENCRYPTION_KEY=${NEW_KEY}" >> .env
    fi
    if ! grep -q "GOMEMLIMIT" .env; then
        echo -e "${CYAN}[*] Applying recommended performance & resource tuning configurations to .env...${NC}"
        cat << 'EOF' >> .env

# Resource & Database Connection Pool Tuning
DB_MAX_CONNS=25
DB_MIN_CONNS=5
DB_MAX_CONN_IDLE_TIME=600
DB_MAX_CONN_LIFETIME=3600
POSTGRES_SHARED_BUFFERS=128MB
POSTGRES_WORK_MEM=8MB
POSTGRES_MAINTENANCE_WORK_MEM=32MB
POSTGRES_MAX_CONNECTIONS=100
POSTGRES_WAL_BUFFERS=8MB
GOMEMLIMIT=512MiB
GOGC=80
EOF
    else
        # Auto-tune old defaults if found in existing .env
        sed -i 's/^DB_MAX_CONNS=10$/DB_MAX_CONNS=25/' .env 2>/dev/null || true
        sed -i 's/^DB_MIN_CONNS=2$/DB_MIN_CONNS=5/' .env 2>/dev/null || true
        sed -i 's/^DB_MAX_CONN_IDLE_TIME=300$/DB_MAX_CONN_IDLE_TIME=600/' .env 2>/dev/null || true
        sed -i 's/^POSTGRES_MAX_CONNECTIONS=40$/POSTGRES_MAX_CONNECTIONS=100/' .env 2>/dev/null || true
        sed -i 's/^POSTGRES_SHARED_BUFFERS=64MB$/POSTGRES_SHARED_BUFFERS=128MB/' .env 2>/dev/null || true
        sed -i 's/^POSTGRES_WORK_MEM=4MB$/POSTGRES_WORK_MEM=8MB/' .env 2>/dev/null || true
        sed -i 's/^POSTGRES_MAINTENANCE_WORK_MEM=16MB$/POSTGRES_MAINTENANCE_WORK_MEM=32MB/' .env 2>/dev/null || true
        sed -i 's/^POSTGRES_WAL_BUFFERS=4MB$/POSTGRES_WAL_BUFFERS=8MB/' .env 2>/dev/null || true
        sed -i 's/^GOMEMLIMIT=256MiB$/GOMEMLIMIT=512MiB/' .env 2>/dev/null || true
    fi
fi

echo -e "\n${BLUE}[2/4] Rebuilding & Upgrading Container Stack...${NC}"
BUILD_SUCCESS=false

# Attempt 1: Standard build (fast, leverages local layer cache)
export REGISTRY_MIRROR=""
if $COMPOSE_CMD build; then
    BUILD_SUCCESS=true
else
    echo -e "${YELLOW}[!] Standard build failed. Retrying with --no-cache...${NC}"
fi

# Attempt 2 (Fallback): Clean build (--no-cache)
if [ "$BUILD_SUCCESS" = false ]; then
    echo -e "${YELLOW}[*] Retrying with clean build (--no-cache)...${NC}"
    if REGISTRY_MIRROR="" $COMPOSE_CMD build --no-cache; then
        BUILD_SUCCESS=true
    fi
fi

if [ "$BUILD_SUCCESS" = false ]; then
    echo -e "${RED}[ERROR] Docker build failed. Please inspect errors above.${NC}"
    exit 1
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

# Get Host IP and Web HTTP Port
SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$SERVER_IP" ]; then
    SERVER_IP="localhost"
fi

HTTP_PORT=$(grep -E "^HTTP_PORT=" .env 2>/dev/null | cut -d'=' -f2 | tr -d ' "' || echo "80")
HTTP_PORT=${HTTP_PORT:-80}

echo -e "\n${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}[OK] HEPHAESTUS CONTROL PANEL (HCP) UPDATED SUCCESSFULLY!${NC}"
echo -e "${GREEN}==============================================================================${NC}"
if [ "$HTTP_PORT" = "80" ]; then
    echo -e "Web Interface URL    : ${CYAN}http://${SERVER_IP}${NC} (or http://localhost)"
else
    echo -e "Web Interface URL    : ${CYAN}http://${SERVER_IP}:${HTTP_PORT}${NC} (or http://localhost:${HTTP_PORT})"
fi
echo -e "Active Containers    : hephaestus-panel, hephaestus-engine, hephaestus-database"
echo -e "Container Logs       : ${CYAN}docker compose logs -f${NC}"
echo -e "Reset Admin Password : ${YELLOW}./reset-password.sh admin '<password_baru>'${NC}"
echo "=============================================================================="
