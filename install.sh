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
echo -e "${BOLD}Hephaestus Control Panel (HCP)${NC}"
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

# Configure Docker Daemon (Uses host VPC DNS naturally)
configure_docker_daemon() {
    echo -e "\n${BLUE}[Setup] Configuring Docker Daemon Registry Mirrors...${NC}"
    mkdir -p /etc/docker
    cat <<EOF > /etc/docker/daemon.json
{
  "insecure-registries": ["registry-1.docker.io", "docker.io", "public.ecr.aws"]
}
EOF
    systemctl restart docker || true
}

configure_docker_daemon

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
# Helper: Check if a port is currently occupied
is_port_in_use() {
    local port=$1
    if command -v ss &> /dev/null; then
        ss -tuln 2>/dev/null | grep -qE "(:${port}|\[::\]:${port})\b" && return 0
    elif command -v netstat &> /dev/null; then
        netstat -tuln 2>/dev/null | grep -qE "(:${port}|\[::\]:${port})\b" && return 0
    elif command -v lsof &> /dev/null; then
        lsof -i :${port} &> /dev/null && return 0
    fi
    return 1
}

# Generate Cryptographic Keys and Interactive .env Configuration
setup_configuration() {
    echo -e "\n${BLUE}[3/6] Configuring Ports & Database Parameters...${NC}"

    # Default values
    DEF_HTTP_PORT=80
    DEF_DB_PORT=5432
    DEF_DB_USER="hephaestus"
    DEF_DB_NAME="hephaestus"
    
    # Check for existing .env
    RECONFIGURE="yes"
    if [ -f .env ]; then
        echo -e "${YELLOW}[INFO] An existing .env configuration file was found.${NC}"
        if [ -t 0 ]; then
            read -p "Do you want to keep existing configuration? [Y/n]: " KEEP_EXISTING
            KEEP_EXISTING=${KEEP_EXISTING:-Y}
            if [[ "$KEEP_EXISTING" =~ ^[Yy]$ ]]; then
                RECONFIGURE="no"
                source .env 2>/dev/null || true
            fi
        else
            RECONFIGURE="no"
            source .env 2>/dev/null || true
        fi
    fi

    if [ "$RECONFIGURE" = "yes" ]; then
        echo -e "\nPlease specify installation parameters (press Enter to accept default):"

        # 1. Web Panel Port
        while true; do
            if [ -t 0 ]; then
                read -p "1. Web Panel HTTP Port [$DEF_HTTP_PORT]: " INPUT_HTTP_PORT
            else
                INPUT_HTTP_PORT=""
            fi
            INPUT_HTTP_PORT=${INPUT_HTTP_PORT:-$DEF_HTTP_PORT}
            
            if ! [[ "$INPUT_HTTP_PORT" =~ ^[0-9]+$ ]] || [ "$INPUT_HTTP_PORT" -lt 1 ] || [ "$INPUT_HTTP_PORT" -gt 65535 ]; then
                echo -e "${RED}[WARN] Invalid port number. Please enter a value between 1 and 65535.${NC}"
                continue
            fi

            if is_port_in_use "$INPUT_HTTP_PORT"; then
                echo -e "${YELLOW}[WARNING] Port $INPUT_HTTP_PORT is already in use by another service on this host!${NC}"
                if [ -t 0 ]; then
                    read -p "Do you still want to proceed with port $INPUT_HTTP_PORT? [y/N]: " OVERRIDE_PORT
                    if [[ "$OVERRIDE_PORT" =~ ^[Yy]$ ]]; then
                        break
                    fi
                else
                    break
                fi
            else
                echo -e "${GREEN}[OK] Port $INPUT_HTTP_PORT is available.${NC}"
                break
            fi
        done

        # 2. Database External Port
        while true; do
            if [ -t 0 ]; then
                read -p "2. PostgreSQL External Port [$DEF_DB_PORT]: " INPUT_DB_PORT
            else
                INPUT_DB_PORT=""
            fi
            INPUT_DB_PORT=${INPUT_DB_PORT:-$DEF_DB_PORT}

            if ! [[ "$INPUT_DB_PORT" =~ ^[0-9]+$ ]] || [ "$INPUT_DB_PORT" -lt 1 ] || [ "$INPUT_DB_PORT" -gt 65535 ]; then
                echo -e "${RED}[WARN] Invalid port number. Please enter a value between 1 and 65535.${NC}"
                continue
            fi

            if is_port_in_use "$INPUT_DB_PORT"; then
                echo -e "${YELLOW}[WARNING] Port $INPUT_DB_PORT is already in use (e.g. host Postgres/another container)!${NC}"
                if [ -t 0 ]; then
                    read -p "Do you still want to proceed with port $INPUT_DB_PORT? [y/N]: " OVERRIDE_DB_PORT
                    if [[ "$OVERRIDE_DB_PORT" =~ ^[Yy]$ ]]; then
                        break
                    fi
                else
                    break
                fi
            else
                echo -e "${GREEN}[OK] Port $INPUT_DB_PORT is available.${NC}"
                break
            fi
        done

        # 3. Database Username
        if [ -t 0 ]; then
            read -p "3. Database Username [$DEF_DB_USER]: " INPUT_DB_USER
        else
            INPUT_DB_USER=""
        fi
        INPUT_DB_USER=${INPUT_DB_USER:-$DEF_DB_USER}

        # 4. Database Name
        if [ -t 0 ]; then
            read -p "4. Database Name [$DEF_DB_NAME]: " INPUT_DB_NAME
        else
            INPUT_DB_NAME=""
        fi
        INPUT_DB_NAME=${INPUT_DB_NAME:-$DEF_DB_NAME}

        # 5. Database Password
        RANDOM_DB_PASSWORD=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)
        if [ -t 0 ]; then
            read -p "5. Database Password (leave blank for random: $RANDOM_DB_PASSWORD): " INPUT_DB_PASS
        else
            INPUT_DB_PASS=""
        fi
        INPUT_DB_PASS=${INPUT_DB_PASS:-$RANDOM_DB_PASSWORD}

        # 6. Generate 64-char Hex Encryption Key for AES-256-GCM
        RANDOM_ENCRYPTION_KEY=$(openssl rand -hex 32)

        echo -e "\nWriting configuration to .env..."
        cat <<EOF > .env
# ==============================================================================
# Hephaestus Control Panel (HCP) - Environment Configuration
# ==============================================================================

# Web & Server Ports
HTTP_PORT=${INPUT_HTTP_PORT}
DB_EXTERNAL_PORT=${INPUT_DB_PORT}
APP_ENV=production

# Registry Mirror (Uses Amazon ECR Public mirror to avoid Docker Hub TLS proxy blocks)
REGISTRY_MIRROR=public.ecr.aws/docker/library/

# Database Credentials
DB_USER=${INPUT_DB_USER}
DB_PASSWORD=${INPUT_DB_PASS}
DB_NAME=${INPUT_DB_NAME}

# Security Key (64-character Hexadecimal for AES-256-GCM)
APP_ENCRYPTION_KEY=${RANDOM_ENCRYPTION_KEY}

# Directories
LOGS_DIR=/app/logs
DATA_DIR=/app/data
EOF
        HTTP_PORT=${INPUT_HTTP_PORT}
        DB_EXTERNAL_PORT=${INPUT_DB_PORT}
        DB_USER=${INPUT_DB_USER}
        DB_NAME=${INPUT_DB_NAME}
        echo -e "${GREEN}[OK] Configuration generated successfully.${NC}"
    fi
}

# Setup Firewall Rules
configure_firewall() {
    echo -e "\n${BLUE}[4/6] Checking Firewall Configuration...${NC}"
    HTTP_PORT=${HTTP_PORT:-80}

    # UFW (Ubuntu/Debian)
    if command -v ufw &> /dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
        echo "Opening Web port $HTTP_PORT in UFW..."
        ufw allow $HTTP_PORT/tcp comment "Hephaestus Control Panel (HCP) Web" || true
        ufw reload || true
    fi

    # Firewalld (RHEL/Rocky/AlmaLinux)
    if command -v firewall-cmd &> /dev/null && systemctl is-active --quiet firewalld; then
        echo "Opening Web port $HTTP_PORT in Firewalld..."
        firewall-cmd --permanent --add-port=$HTTP_PORT/tcp || true
        firewall-cmd --reload || true
    fi
}

setup_configuration
configure_firewall

# Build and Deploy Containers
deploy_containers() {
    echo -e "\n${BLUE}[5/6] Building & Starting Multi-Container Stack (Panel, Engine, Database)...${NC}"
    
    # Check if docker compose or docker-compose is available
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        echo -e "${RED}[ERROR] Neither 'docker compose' nor 'docker-compose' command found.${NC}"
        exit 1
    fi

    echo "Building container images with host networking..."
    $COMPOSE_CMD build
    
    echo "Starting container stack..."
    $COMPOSE_CMD up -d

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
if [ "$HTTP_PORT" = "80" ]; then
    echo -e "Web Interface URL    : ${CYAN}http://${SERVER_IP}${NC} (or http://localhost)"
else
    echo -e "Web Interface URL    : ${CYAN}http://${SERVER_IP}:${HTTP_PORT}${NC} (or http://localhost:${HTTP_PORT})"
fi
echo -e "Architecture         : Multi-Container (hephaestus-panel, hephaestus-engine, hephaestus-database)"
echo -e "PostgreSQL Database  : ${DB_NAME:-hephaestus} (External Port: ${DB_EXTERNAL_PORT:-5432})"
echo -e "Database User        : ${DB_USER:-hephaestus}"
echo -e "Installation Path    : ${INSTALL_DIR}"
echo -e "Configuration File   : ${INSTALL_DIR}/.env"
echo -e "Container Logs       : ${CYAN}docker compose logs -f${NC}"
echo -e "Restart Stack        : ${CYAN}docker compose restart${NC}"
echo "------------------------------------------------------------------------------"
echo -e "${BOLD}Next Steps:${NC}"
if [ "$HTTP_PORT" = "80" ]; then
    echo -e "1. Open ${CYAN}http://${SERVER_IP}${NC} in your web browser."
else
    echo -e "1. Open ${CYAN}http://${SERVER_IP}:${HTTP_PORT}${NC} in your web browser."
fi
echo -e "2. The initial ${BOLD}Setup Wizard${NC} will open automatically."
echo -e "3. Create your Master Administrator credentials to start managing infrastructure."
echo "=============================================================================="
