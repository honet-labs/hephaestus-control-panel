#!/usr/bin/env bash
# ==============================================================================
# Hephaestus Control Panel (HCP) - Automated Universal Installer
# Compatible with: Ubuntu, Debian, Rocky Linux, RHEL, AlmaLinux
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
echo -e "${BOLD}Hephaestus Control Panel (HCP) - Universal Auto-Installer v2.0${NC}"
echo -e "Target Stack: Go 1.22 + PostgreSQL 16 + Vue 3 / Nginx (Multi-Container)"
echo "------------------------------------------------------------------------------"

# Check Root Privileges
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}[ERROR] This installation script must be run as root or with sudo.${NC}"
    exit 1
fi

# Detect Linux Distribution
echo -e "\n${BLUE}[1/6] Detecting Operating System...${NC}"
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS_ID=$ID
    OS_VERSION=$VERSION_ID
    OS_NAME=$PRETTY_NAME
else
    echo -e "${RED}[ERROR] Could not detect operating system (/etc/os-release not found).${NC}"
    exit 1
fi

echo -e "Detected OS: ${GREEN}${OS_NAME}${NC} (Family: ${OS_ID})"

# Function: Install Dependencies and Docker based on OS
install_dependencies() {
    echo -e "\n${BLUE}[2/6] Installing Package Dependencies & Docker Engine...${NC}"
    
    case "$OS_ID" in
        ubuntu|debian)
            echo "Updating APT repository..."
            apt-get update -y
            apt-get install -y ca-certificates curl gnupg lsb-release git openssl jq ufw

            # Install Docker if not present
            if ! command -v docker &> /dev/null; then
                echo "Installing Docker Engine via official APT repository..."
                install -m 0755 -d /etc/apt/keyrings
                curl -fsSL https://download.docker.com/linux/$OS_ID/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg --yes
                chmod a+r /etc/apt/keyrings/docker.gpg

                echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$OS_ID $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
                apt-get update -y
                apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            else
                echo -e "${GREEN}Docker is already installed.${NC}"
            fi
            ;;

        rocky|rhel|almalinux|centos)
            echo "Updating DNF/YUM repository..."
            if command -v dnf &> /dev/null; then
                PKG_MGR="dnf"
            else
                PKG_MGR="yum"
            fi

            $PKG_MGR install -y yum-utils git openssl jq curl

            # Install Docker if not present
            if ! command -v docker &> /dev/null; then
                echo "Installing Docker Engine via official DNF/YUM repository..."
                $PKG_MGR config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo || true
                $PKG_MGR install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            else
                echo -e "${GREEN}Docker is already installed.${NC}"
            fi
            ;;

        *)
            echo -e "${YELLOW}[WARN] Unknown distribution: $OS_ID. Attempting generic Docker install script...${NC}"
            if ! command -v docker &> /dev/null; then
                curl -fsSL https://get.docker.com -o get-docker.sh
                sh get-docker.sh
                rm -f get-docker.sh
            fi
            ;;
    esac

    # Ensure Docker is started and enabled
    systemctl daemon-reload
    systemctl enable docker
    systemctl start docker

    # Verify Docker and Compose
    if command -v docker &> /dev/null; then
        DOCKER_VER=$(docker --version)
        echo -e "${GREEN}[OK] ${DOCKER_VER}${NC}"
    else
        echo -e "${RED}[ERROR] Docker installation failed.${NC}"
        exit 1
    fi
}

install_dependencies

# If script is run via curl pipe (outside of cloned repo), clone repository to /opt/hephaestus-control-panel
INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ ! -f "$INSTALL_DIR/docker-compose.yml" ]; then
    echo -e "\n${BLUE}[Setup] Cloning Hephaestus Control Panel repository...${NC}"
    INSTALL_DIR="/opt/hephaestus-control-panel"
    if [ -d "$INSTALL_DIR" ]; then
        cd "$INSTALL_DIR" && git pull || true
    else
        git clone https://github.com/honet-labs/hephaestus-control-panel.git "$INSTALL_DIR"
    fi
fi
cd "$INSTALL_DIR"

# Setup Firewall Rules
configure_firewall() {
    echo -e "\n${BLUE}[3/6] Checking Firewall Configuration...${NC}"
    HTTP_PORT=${HTTP_PORT:-80}

    # UFW (Ubuntu/Debian)
    if command -v ufw &> /dev/null && ufw status | grep -q "Status: active"; then
        echo "Opening port $HTTP_PORT in UFW..."
        ufw allow $HTTP_PORT/tcp comment "Hephaestus Control Panel (HCP)" || true
        ufw reload || true
    fi

    # Firewalld (RHEL/Rocky/AlmaLinux)
    if command -v firewall-cmd &> /dev/null && systemctl is-active --quiet firewalld; then
        echo "Opening port $HTTP_PORT in Firewalld..."
        firewall-cmd --permanent --add-port=$HTTP_PORT/tcp || true
        firewall-cmd --reload || true
    fi
}

# Generate Cryptographic Keys and .env Configuration
setup_configuration() {
    echo -e "\n${BLUE}[4/6] Generating Secure Cryptographic Keys & Configuration...${NC}"

    # Generate 64-char Hex Encryption Key for AES-256-GCM
    RANDOM_ENCRYPTION_KEY=$(openssl rand -hex 32)
    RANDOM_DB_PASSWORD=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)

    if [ ! -f .env ]; then
        echo "Creating new .env file with generated credentials..."
        cat <<EOF > .env
# ==============================================================================
# Hephaestus Control Panel (HCP) - Environment Configuration
# ==============================================================================

# Web & Server Ports
HTTP_PORT=80
DB_EXTERNAL_PORT=5432
APP_ENV=production

# Database Credentials
DB_USER=hephaestus
DB_PASSWORD=${RANDOM_DB_PASSWORD}
DB_NAME=hephaestus

# Security Key (64-character Hexadecimal for AES-256-GCM)
APP_ENCRYPTION_KEY=${RANDOM_ENCRYPTION_KEY}

# Directories
LOGS_DIR=/app/logs
DATA_DIR=/app/data
EOF
        echo -e "${GREEN}[OK] Generated .env file successfully.${NC}"
    else
        echo -e "${YELLOW}[INFO] Existing .env file found. Preserving existing configuration.${NC}"
    fi
}

setup_configuration
configure_firewall

# Build and Deploy Containers
deploy_containers() {
    echo -e "\n${BLUE}[5/6] Building & Starting Multi-Container Stack (Frontend, Backend, Postgres)...${NC}"
    
    # Check if docker compose or docker-compose is available
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        echo -e "${RED}[ERROR] Neither 'docker compose' nor 'docker-compose' command found.${NC}"
        exit 1
    fi

    echo "Executing: $COMPOSE_CMD up -d --build"
    if ! $COMPOSE_CMD up -d --build; then
        echo -e "\n${YELLOW}[WARN] Standard build encountered a network/TLS issue. Retrying with legacy Docker builder (DOCKER_BUILDKIT=0)...${NC}"
        DOCKER_BUILDKIT=0 COMPOSE_DOCKER_CLI_BUILD=0 $COMPOSE_CMD up -d --build
    fi

    echo -e "\n${BLUE}[6/6] Verifying Service Health...${NC}"
    sleep 5
    $COMPOSE_CMD ps
}

deploy_containers

# Get Host IP Address
SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$SERVER_IP" ]; then
    SERVER_IP="localhost"
fi

# Final Output
echo -e "\n${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}[OK] HEPHAESTUS CONTROL PANEL (HCP) DEPLOYED SUCCESSFULLY!${NC}"
echo -e "${GREEN}==============================================================================${NC}"
echo -e "Web Interface URL    : ${CYAN}http://${SERVER_IP}${NC} (or http://localhost)"
echo -e "Architecture         : Multi-Container (Frontend Nginx, Backend Go, PostgreSQL 16)"
echo -e "PostgreSQL Database  : hephaestus (Port 5432)"
echo -e "Installation Path    : ${INSTALL_DIR}"
echo -e "Configuration File   : ${INSTALL_DIR}/.env"
echo -e "Container Logs       : ${CYAN}docker compose logs -f${NC}"
echo -e "Restart Stack        : ${CYAN}docker compose restart${NC}"
echo "------------------------------------------------------------------------------"
echo -e "${BOLD}Next Steps:${NC}"
echo -e "1. Open ${CYAN}http://${SERVER_IP}${NC} in your web browser."
echo -e "2. The initial ${BOLD}Setup Wizard${NC} will open automatically."
echo -e "3. Create your Master Administrator credentials to start managing infrastructure."
echo "=============================================================================="
