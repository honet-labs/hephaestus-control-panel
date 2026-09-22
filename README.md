# Hephaestus Control Panel (HCP)

> **Unified DevOps, Server, Database & Network Infrastructure Control Panel**  
> High-performance Go backend, PostgreSQL 16 ACID persistence, native in-memory worker pool, and sleek modern Vue 3 + Tailwind CSS web interface.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![Vue 3](https://img.shields.io/badge/Vue-3.x-4FC08D?style=flat-square&logo=vuedotjs)](https://vuejs.org/)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-3.x-38B2AC?style=flat-square&logo=tailwind-css)](https://tailwindcss.com/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org/)
[![Docker](https://img.shields.io/badge/Docker-Multi--Container-2496ED?style=flat-square&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](./LICENSE)

---

## Overview

**Hephaestus Control Panel (HCP)** is a centralized, enterprise-ready infrastructure management platform designed to eliminate toolchain fragmentation for DevOps engineers, SREs, NOC network operators, and Database Administrators (DBAs).

Instead of juggling disparate desktop SSH clients, standalone crontab backup scripts, isolated network scanners, and multiple observability tabs, Hephaestus consolidates your operational workflow into a single, high-performance web dashboard:

- **Ultra-low Resource Footprint**: Backend baseline RAM usage < 50MB, interactive WebSocket terminal keystroke latency < 15ms, and sub-10ms REST API response times.
- **Zero Third-Party Queue Dependencies**: Native Go Goroutine Worker Pool with real-time job progress percentages, cancellation (`context.WithCancel`), and automatic retries without requiring external Redis or RabbitMQ brokers.
- **Enterprise-Grade Security**: AES-256-GCM authenticated credential encryption, bcrypt password hashing, token-bucket API rate limiting, IP brute-force lockout, and granular feature-level Role-Based Access Control (RBAC).
- **Turnkey Linux Deployment**: Fully automated universal installer (`install.sh`) supporting Ubuntu, Debian, Rocky Linux, RHEL, and AlmaLinux with zero-downtime containerized updates (`update-version.sh`).

---

## Core Capabilities

### 1. Multi-Tab SSH Terminal & Host Management
- **Interactive Browser Terminal**: High-performance terminal powered by `@xterm/xterm` over full-duplex WebSockets connected to Go PTY (`xterm-256color`).
- **Split-Screen Grid Layouts**: Dynamic multi-pane arrangements including `1x1` (Single), `1x2` (Dual Column), `2x1` (Dual Row), and `2x2` (Quad Screen).
- **Multi-Cast Keyboard Broadcast**: Synchronized input broadcasting from the active terminal pane to all open split sessions simultaneously.
- **Integrated SFTP File Explorer**: File and directory manager supporting multipart uploads, streaming chunked downloads, directory navigation, deletions, and remote-to-remote file transfers.
- **Host Telemetry & Systemd Control**: Real-time CPU, RAM, disk, and load averages via SSH, interactive process manager with kill capabilities, systemd service management (`start`, `stop`, `restart`, `status`), and remote firewall rules management (UFW / iptables).
- **Multi-User Host Sharing**: Granular host-sharing permissions across administrative and operator accounts.
- **Broad Cipher Compatibility**: Built-in support for standard modern SSH ciphers as well as legacy ciphers (`3des-cbc`, `aes128-cbc`, `diffie-hellman-group1-sha1`) for enterprise routers, switches, and legacy appliances.

### 2. Docker & Container Management
- **Multi-Host Docker Engine Support**: Manage containers across local UNIX sockets (`/var/run/docker.sock`), remote TCP sockets, or remote SSH tunnels.
- **Container Lifecycle Operations**: Start, stop, restart, pause, unpause, delete, and deploy new containers directly from the control panel.
- **Live Container Metrics & Logs**: Real-time CPU, RAM, and network I/O statistics coupled with live streaming container logs.
- **Image & Network Management**: Pull images from Docker Hub or private registries, remove unused images, and inspect or create container bridge networks.

### 3. Network Topology & Auto-Discovery
- **Visual Topology Canvas**: Interactive node-and-edge network graph with multi-tab sheets and real-time device status indicators.
- **Dual Auto-Discovery Engines**:
  - **Prometheus Target Sync**: Ingests active targets directly from Prometheus `/api/v1/targets`.
  - **Subnet ICMP Sweeper**: High-concurrency ping discovery sweep across CIDR blocks (e.g. `192.168.1.0/24`) to auto-discover active network devices.
- **Automated Health Monitoring**: Background 60-second ICMP ping worker cycle continuously validating device reachability and round-trip latency.

### 4. Database Disaster Recovery & Automated Backups
- **Multi-Database Engine Support**: Automated and on-demand dumps for PostgreSQL (`pg_dump`), MySQL / MariaDB (`mysqldump`), and Microsoft SQL Server (`sqlcmd`).
- **Flexible Execution Modes**: Native local execution or remote SSH dump execution for remote database nodes.
- **Automated Gzip Compression**: Real-time streaming compression minimizing storage footprint and network transfer overhead.
- **Multi-Destination Uploaders**: Replicate backups across Local Storage, Cloudflare R2, AWS S3, and NAS storage via SFTP/SCP.
- **Cron Scheduling & History**: Standard cron expression scheduling (`robfig/cron/v3`) with complete audit trails, file size tracking, execution durations, and error reporting.

### 5. Visual Custom Metrics & Reporting Engine (`/reports`)
- **Document-Style Metric Reports**: Interactive custom metric dashboard and report generator supporting portrait and landscape orientations.
- **Flexible Chart Widgets**: Line charts, bar charts, area charts, and tabular metric summaries.
- **Multi-Datasource Ingestion**: Unified querying from OpenSearch clusters and Prometheus instances.
- **Multi-Host Metric Comparison**: Compare metrics across multiple servers simultaneously with dynamic multi-color series and auto-wrapped legends.
- **High-Resolution Chart Exports**: One-click download of rendered metric charts for documentation, post-mortems, and executive reporting.

### 6. Observability Hub & Remote Configuration
- **OpenTelemetry Remote Collector Management**: Multi-host OTel collector configuration editor, remote YAML syntax validation, instant service restart, pre-built collector presets, and revision history.
- **OpenSearch Cluster Telemetry**: Real-time cluster health, node JVM metrics, shard allocation status, and index recovery tracking.
- **Prometheus Management**: Interactive PromQL query execution, configuration editor, and remote configuration reload trigger (`/-/reload`).
- **Data Prepper Pipelines**: Visual YAML editor and validator for OpenSearch Data Prepper log parsing pipelines.
- **Interactive Grok Debugger**: In-browser regular expression pattern tester with built-in patterns for standard Linux and web server logs.

### 7. NOC Wallboard Slide Show & Kiosk Mode
- **Multi-Dashboard Carousel**: Automated cyclic rotation between internal and external dashboards (Grafana, Kibana, Prometheus, Uptime Kuma, OpenSearch).
- **Custom Display Scaling**: Density zoom transforms (`50%`, `67%`, `75%`, `80%`, `90%`, `100%`) for dense NOC monitor walls.
- **Immersive Auto-Hide Controls**: Floating controls and headers automatically fade out after 3 seconds of inactivity and reappear on cursor movement or touch.
- **Private Network Access (PNA) Helper**: Built-in detection and clipboard utility to bypass modern browser mixed-network restrictions.

### 8. Enterprise Security, RBAC & Credential Vault
- **Initial Setup Wizard**: Automatic redirection to `/setup` on initial deployment for secure master administrator provisioning.
- **Granular Feature-Level RBAC**: Permission matrix enforcing `none`, `read`, or `manage` rights across 15 distinct system modules (Terminal, Backups, Topology, Docker, Queue, Logs, Reports, etc.) across `ADMIN`, `OPERATOR`, `VIEWER`, or custom roles.
- **Vaultwarden / Bitwarden E2EE Integration**: Direct synchronization with Vaultwarden instances and cached cipher management.
- **Brute-Force & Rate Limiting Defense**: Sliding-window rate limiter locking out offending IPs after consecutive failed logins; token-bucket limiter on all API routes.
- **AES-256-GCM Cryptography**: Zero plaintext secret storage. Database passwords, SSH private keys, and S3 credentials are encrypted at rest with authentication tags.
- **Comprehensive Audit Trail**: All system actions logged to `activity_logs` with actor usernames, timestamps, and IP addresses.

### 9. Native In-Memory Background Worker Queue
- **Goroutine Worker Pool**: Concurrency-controlled worker channels (default 5 concurrent workers) with real-time job state transitions (`pending`, `running`, `completed`, `failed`, `cancelled`).
- **Live Progress Reporting**: Background tasks dispatch percentage progress and status updates directly to the web UI.
- **Queue Dashboard (`/queue`)**: Centralized interface to monitor active jobs, view job history, trigger manual runs, or cancel running tasks.

### 10. Structured Logging & Live Web Stream
- **High-Performance Logging**: Structured JSON logging powered by Zerolog with automatic Lumberjack file rotation (`app.log`, `error.log`).
- **Live Log WebSocket (`/ws/logs`)**: In-memory Pub/Sub channel broadcasting real-time logs to the web interface with pause/resume, level filters (`INFO`, `WARN`, `ERROR`, `DEBUG`), and text search.

---

## Screenshoot
### Overview
<img width="1919" height="945" alt="image" src="https://github.com/user-attachments/assets/ea821510-60db-4bbb-9c51-1ab107911e7d" />

### Connections
<img width="1919" height="954" alt="image" src="https://github.com/user-attachments/assets/855d8af1-35a6-4b2c-a615-6c51ce136529" />

### Menu Infrastruktur
- Remote Server
<img width="1919" height="948" alt="Remote-Server" src="https://github.com/user-attachments/assets/6df047e2-930e-49f9-bc62-a9ac47b207a0" />

- Management Container
<img width="1919" height="945" alt="image" src="https://github.com/user-attachments/assets/2813a564-bacf-4d1c-b63b-9752805b13b9" />

### Menu Networking
- Network Topology
<img width="1919" height="944" alt="image" src="https://github.com/user-attachments/assets/7e4b0031-b18b-484f-ba55-cb78fe56c553" />

### Remote Config
- Data Prepper Pipelines
<img width="1919" height="947" alt="image" src="https://github.com/user-attachments/assets/45ccb1f2-275d-48f2-a716-4262cad76b2b" />

- Prometheus Config
<img width="1919" height="946" alt="image" src="https://github.com/user-attachments/assets/81ff85cc-8f95-463e-b15e-0cf5a48462a9" />

- OpenTelemetry Config
<img width="1919" height="949" alt="image" src="https://github.com/user-attachments/assets/508793ee-d98b-480a-9f11-41cfe8a32af6" />

### Tools
- SNMP Browser
<img width="1919" height="949" alt="image" src="https://github.com/user-attachments/assets/1745672f-951b-467b-9670-3771d5dcdd0b" />

- Backup Manager
<img width="1919" height="949" alt="image" src="https://github.com/user-attachments/assets/4078fb7e-e638-4c05-81a7-5206ac1475c0" />
<img width="1919" height="948" alt="image" src="https://github.com/user-attachments/assets/c9b77afc-07b8-47c1-9614-c9c00eecc179" />
<img width="1919" height="951" alt="image" src="https://github.com/user-attachments/assets/175f5dbd-3eb6-4308-be45-c3e14b09c759" />
<img width="1919" height="942" alt="image" src="https://github.com/user-attachments/assets/585e3154-6deb-4553-bc6b-cb0b1eb7a0cb" />
<img width="1919" height="851" alt="image" src="https://github.com/user-attachments/assets/ef900199-b1a9-417a-85e8-46961c3db3bf" />
<img width="1695" height="667" alt="image" src="https://github.com/user-attachments/assets/9d3429dc-df83-4a17-8406-1f6101b51277" />

### Security
- Vaultwarden
<img width="1919" height="940" alt="vaultwarden-home" src="https://github.com/user-attachments/assets/1bf6ef8a-08e8-48ef-b383-e1dbe4270a64" />
<img width="1919" height="911" alt="vaultwarden" src="https://github.com/user-attachments/assets/b41a2d5c-c3cb-4892-90d0-345991d322fb" />


### Monitoring
- OpenSearch Cluster Monitor
<img width="1919" height="955" alt="image" src="https://github.com/user-attachments/assets/0c5b9b76-0aae-4b83-9268-a84b4c1630a8" />
<img width="1919" height="945" alt="image" src="https://github.com/user-attachments/assets/6dff276b-9e72-4bf8-aeac-44e4c0a9e49e" />
<img width="1919" height="942" alt="image" src="https://github.com/user-attachments/assets/7747cf20-9760-404a-8f05-224156998d2b" />
<img width="1919" height="944" alt="image" src="https://github.com/user-attachments/assets/819b59c0-20ee-4cac-a0fc-df209457ed05" />
<img width="1919" height="939" alt="image" src="https://github.com/user-attachments/assets/60b659ca-0f6c-44df-af39-b1054cff2845" />
<img width="1919" height="947" alt="image" src="https://github.com/user-attachments/assets/1d77cfd7-8993-45da-9b05-d5c041e27fd1" />
<img width="1919" height="943" alt="image" src="https://github.com/user-attachments/assets/67632973-7d0e-436d-bc11-5eff56ad8f8a" />
<img width="1920" height="1080" alt="Screenshot (5880)" src="https://github.com/user-attachments/assets/9f43a7e1-a01c-457e-8154-5a9d31930b0f" />
<img width="1919" height="946" alt="Screenshot 2026-09-21 110017" src="https://github.com/user-attachments/assets/f8e925f3-128c-4c09-8750-cec0a954f0bd" />
<img width="1919" height="950" alt="Screenshot 2026-09-21 110029" src="https://github.com/user-attachments/assets/c213a440-5ca8-4874-8e54-1bec1dac31d7" />
<img width="1919" height="943" alt="Screenshot 2026-09-21 110053" src="https://github.com/user-attachments/assets/06389434-dfd6-4a4f-8a3c-0635b3e75d39" />

- Slide Show
<img width="1919" height="943" alt="image" src="https://github.com/user-attachments/assets/5b0ea24e-6b66-4e5a-9218-34c40316c8f1" />
<img width="1919" height="948" alt="image" src="https://github.com/user-attachments/assets/deb6ea29-224b-46da-9e65-6869035ced54" />

### Settings
- RBAC
<img width="1919" height="938" alt="image" src="https://github.com/user-attachments/assets/69059cc2-51f5-4c0d-80ce-857fcad155a1" />
<img width="1919" height="944" alt="image" src="https://github.com/user-attachments/assets/38e31ad4-06ba-4894-be49-f8e03436b547" />
<img width="1919" height="942" alt="image" src="https://github.com/user-attachments/assets/8d6caa6c-88ed-4857-b0b9-fb02dc29c305" />

---

## Multi-Container Architecture

Hephaestus is deployed as an isolated, multi-container Docker Compose stack engineered for high availability and minimal resource contention:

```
                  ┌────────────────────────────────────────────────────────┐
                  │                 Client Web Browser                     │
                  │ (Vue 3 Single Page Application + xterm.js WebSockets)  │
                  └───────────────────────────┬────────────────────────────┘
                                              │ HTTP / WS (Port 80 / 443)
                                              ▼
                  ┌────────────────────────────────────────────────────────┐
                  │         hephaestus-panel (Nginx 1.27 Alpine)           │
                  │  - Gzip asset compression, caching, SPA routing        │
                  │  - Reverse proxy: /api/* -> engine:5000                │
                  │  - WebSocket proxy: /ws/* -> engine:5000               │
                  └───────────────────────────┬────────────────────────────┘
                                              │ Internal Bridge Network
                                              ▼
                  ┌────────────────────────────────────────────────────────┐
                  │         hephaestus-engine (Go 1.22+ Runtime)           │
                  │  - Gin REST API & WebSocket Session Multiplexer        │
                  │  - Native Goroutine Worker Pool & Cron Schedulers      │
                  │  - SSH / SFTP / GoSNMP / Backup Dump Engines           │
                  └──────────────┬──────────────────────────┬──────────────┘
                                 │                          │
                 PostgreSQL pgx  │                          │ Docker Socket
                 Connection Pool │                          │ /var/run/docker.sock
                                 ▼                          ▼
                  ┌────────────────────────┐      ┌────────────────────────┐
                  │   hephaestus-database  │      │  Host & Remote Docker  │
                  │  (PostgreSQL 16 DB)    │      │  Engines & Containers  │
                  │  - 29 Relational Tables│      └────────────────────────┘
                  │  - AES-256-GCM Storage │
                  └────────────────────────┘
```

| Service | Container Name | Base Image | Role & Responsibilities |
|---|---|---|---|
| **Panel** | `hephaestus-panel` | `nginx:1.27-alpine` | Serves compiled Vue 3 SPA assets, Gzip compression, and reverse-proxies `/api` and `/ws` to the engine. |
| **Engine** | `hephaestus-engine` | `alpine:3.20` + Go binary | High-throughput Go REST API, WebSocket terminal/log multiplexer, in-memory worker queue, cron scheduler, and SSH/SFTP engine. |
| **Database** | `hephaestus-database` | `postgres:16-alpine` | PostgreSQL 16 ACID persistence storing 29 relational tables, JSONB configurations, and encrypted credentials. |

---

## Database Architecture & Schema

HCP utilizes **PostgreSQL 16+** with connection pooling managed via `pgxpool.Pool`. The database schema comprises 29 tables categorized into six primary domains:

1. **Authentication & Identity**:
   - `users`: User accounts with bcrypt password hashes and roles.
   - `user_sessions`: Cryptographically secure 64-character hex session tokens with idle expiration.
   - `system_roles`: Configurable roles storing JSONB feature-level permission matrices.
   - `app_config`: Key-value application state store (setup status, notification flags).
   - `activity_logs`: Immutable audit trails detailing user actions, modules, timestamps, and client IPs.
2. **Infrastructure & Server Management**:
   - `remote_host_configs`: SSH host profiles, ports, groups, tags, and encrypted credentials.
   - `remote_host_shares`: Multi-user access delegations per server profile.
   - `remote_host_firewall_rules`: Host-level firewall rule definitions (UFW / iptables).
   - `docker_connections`: Multi-host Docker engine configurations (Local socket, TCP, SSH).
3. **Network Topology & Telemetry**:
   - `topology_sheets`: Organizational workspace tabs for network maps.
   - `topology_devices`: Discovered and manually mapped infrastructure nodes with coordinate positions.
   - `topology_edges`: Device interconnects, interface labels, and link styling.
   - `topology_pending`: Staged discovery scan results awaiting canvas inclusion.
   - `device_ping_results`: Historical and current ICMP reachability and round-trip latency data.
   - `imported_mibs` & `oid_registry`: Parsed MIB files and indexed OID definitions for fast prefix lookups.
4. **Disaster Recovery & Database Backups**:
   - `backup_database_configs`: Target database credentials and connection parameters.
   - `backup_destinations`: Storage target configurations (Local, Cloudflare R2, AWS S3, NAS).
   - `backup_schedules`: Cron schedule configurations linking database targets to storage destinations.
   - `backup_history`: Historical execution logs including dump status, archive size, and durations.
5. **Observability, Connectors & Monitoring Views**:
   - `opentelemetry_configs` & `opentelemetry_config_history`: Remote OpenTelemetry collector profiles and revision history.
   - `prometheus_configs`, `opensearch_configs`, `dataprepper_configs`, `grafana_configs`, `uptime_kuma_configs`: Connection parameters for external telemetry tools.
   - `monitoring_views`: Wallboard presentation carousels and slide interval configurations.
   - `query_panels`: Saved PromQL and OpenSearch metric queries.
6. **Visual Reporting**:
   - `visual_reports`: Metric dashboard document layouts and metadata.
   - `visual_report_widgets`: Metric widget configurations, data sources, time ranges, and chart types.

---

## Automated Linux Operations Scripts

Hephaestus includes a suite of battle-tested shell utilities for complete lifecycle management:

### 1. Universal Automated Installer (`install.sh`)
Deploys Hephaestus Control Panel in under 3 minutes on fresh Linux distributions (**Ubuntu, Debian, Rocky Linux, RHEL, AlmaLinux**):

```bash
# 1. Clone repository
git clone https://github.com/honet-labs/hephaestus-control-panel.git
cd hephaestus-control-panel

# 2. Grant execution rights and run as root
chmod +x install.sh
sudo ./install.sh
```

**Automated Steps Performed:**
1. Automatically detects Linux distribution family (`apt` or `dnf/yum`).
2. Installs package dependencies (`curl`, `git`, `openssl`, `jq`, `ufw`/`firewalld`).
3. Installs and configures official Docker Engine and Docker Compose.
4. Interactively configures HTTP ports and database credentials with conflict detection.
5. Generates a cryptographically strong 64-character hexadecimal `APP_ENCRYPTION_KEY` and random PostgreSQL password.
6. Automatically opens firewall ports in UFW or Firewalld.
7. Builds and starts the multi-container stack (`hephaestus-panel`, `hephaestus-engine`, `hephaestus-database`).

### 2. Zero-Downtime Updater (`update-version.sh`)
Pulls the latest release, mitigates Docker registry rate limits, and rebuilds the stack:

```bash
chmod +x update-version.sh
sudo ./update-version.sh
```

- Features safe self-reloading after `git pull` to ensure updated script code is executed cleanly.
- Tests connectivity to Docker registries and automatically falls back if rate limits (e.g. AWS Public ECR 429) are detected.
- Safely rebuilds and restarts containers while preserving all persistent database and backup volumes.

### 3. Administrator Password Reset (`reset-password.sh`)
Instantly resets an administrator or user password directly via Docker container or CLI:

```bash
chmod +x reset-password.sh
sudo ./reset-password.sh <username> [optional_new_password]

# Example:
sudo ./reset-password.sh admin MyNewSecurePassword123!
```

### 4. Clean Uninstaller (`uninstall.sh`)
Safely stops and purges all containers, volumes, networks, and environment files:

```bash
chmod +x uninstall.sh
sudo ./uninstall.sh
```

---

## Manual Docker Compose Deployment

If you prefer deploying via standard Docker Compose commands:

```bash
# 1. Clone repository and navigate to directory
git clone https://github.com/honet-labs/hephaestus-control-panel.git
cd hephaestus-control-panel

# 2. Copy and customize environment variables
cp .env.example .env
# Edit .env and set your APP_ENCRYPTION_KEY and DB credentials

# 3. Build and launch all containers
docker compose up -d --build

# 4. Monitor startup and container health
docker compose ps
docker compose logs -f
```

Access the panel in your browser at `http://<YOUR_SERVER_IP>` or `http://localhost`.

---

## Configuration & Environment Reference

All application parameters are declared in the `.env` file at the root of the project:

| Variable | Default Value | Description |
|---|---|---|
| `HTTP_PORT` | `80` | Host port exposed for the Web Panel UI and reverse proxy. |
| `DB_EXTERNAL_PORT` | `5432` | Host port exposed for external PostgreSQL database connections. |
| `APP_ENV` | `production` | Application runtime environment (`production` or `development`). |
| `DB_USER` | `hephaestus` | PostgreSQL master username. |
| `DB_PASSWORD` | *(Generated)* | PostgreSQL master password. |
| `DB_NAME` | `hephaestus` | PostgreSQL database name. |
| `APP_ENCRYPTION_KEY` | *(64 Hex Chars)* | 256-bit hexadecimal key used for AES-256-GCM secret encryption. |
| `LOGS_DIR` | `/app/logs` | Persistent storage directory for structured JSON application logs. |
| `DATA_DIR` | `/app/data` | Storage directory for imported MIB files and application cache. |
| `BACKUP_DIR` | `./backups` | Host mount directory for automated and manual database dump archives. |
| `REGISTRY_MIRROR` | *(Empty)* | Optional container registry mirror prefix (e.g. `mirror.gcr.io/library/`). |

---

## Project Structure

```
go-hephaestus/
├── cmd/
│   ├── cli/                      # Dedicated CLI administrative commands
│   └── server/
│       └── main.go               # Application entry point, router & shutdown
├── internal/
│   ├── cli/                      # CLI subcommand handlers (reset-password, users)
│   ├── config/                   # Config loader, crypto (AES-256-GCM), env parser
│   ├── core/domain/              # Pure domain entities, structs & DTO models
│   ├── database/                 # PostgreSQL pool (pgxpool) & schema migrations
│   │   └── migrations/           # Versioned SQL migration scripts
│   ├── handlers/                 # Gin HTTP REST & WebSocket endpoint handlers
│   ├── logger/                   # Zerolog + Lumberjack rotation + Pub/Sub broadcaster
│   ├── middleware/               # Auth, RBAC, Request Logger, Rate Limiters
│   ├── queue/                    # Native in-memory Goroutine Worker Pool
│   ├── repository/               # PostgreSQL data access layer (29 tables)
│   ├── scheduler/                # Cron engine (Backups, Ping monitor, Cleanup)
│   └── services/                 # SSH, SFTP, Docker, Backup, SNMP, OTel, Topology
├── web/                          # Modern Vue 3 + Vite + Tailwind CSS frontend
│   ├── nginx.conf                # Nginx SPA caching & reverse proxy configuration
│   └── src/
│       ├── components/           # Reusable UI widgets, modals & navbars
│       ├── layouts/              # AppLayout (navigation, theme, header)
│       ├── stores/               # Pinia reactive state stores
│       ├── router/               # Vue Router route definitions & auth guards
│       └── views/                # 23 functional views (Terminal, Backup, Topology, etc.)
├── docs/                         # Comprehensive engineering documentation
│   ├── PRD.md                    # Product Requirement Document
│   ├── DEVELOPMENT_GUIDE.md      # Developer & UI/UX Engineering Guide
│   ├── 01-architecture.md        # System Architecture & Clean Code Design
│   ├── 02-database-schema.md     # PostgreSQL Database Schema & Indexes
│   ├── 03-api-reference.md       # REST Endpoints & WebSocket Protocol
│   ├── 04-background-jobs-queue.md # Worker Pool & Job Lifecycle
│   ├── 05-troubleshooting-and-logging.md # Logging & Diagnostic Guide
│   ├── 06-deployment-guide.md    # Production Runbook & SSL Setup
│   └── 07-ui-ux-design-system.md # UI Design Tokens & Iconography Standards
├── Dockerfile.backend            # Multi-stage Go backend production container
├── Dockerfile.frontend           # Multi-stage Vue 3 + Nginx production container
├── docker-compose.yml            # Multi-container service definition
├── install.sh                    # Universal automated Linux installer
├── update-version.sh             # Zero-downtime repository & container updater
├── reset-password.sh             # Quick user password reset utility
├── uninstall.sh                  # Clean uninstallation and teardown script
├── Makefile                      # Developer build and automation tasks
└── README.md
```

---

## Documentation

Detailed technical documentation and operational runbooks are maintained in the [`docs/`](./docs) directory:

- **[Product Requirement Document (PRD)](./docs/PRD.md)**: Product scope, user personas, functional specifications, and technical metrics.
- **[Developer & UI/UX Engineering Guide](./docs/DEVELOPMENT_GUIDE.md)**: Engineering guidelines, design tokens, Lucide monochrome icon standards, zero-emoji policy, and feature recipes.
- **[01 - Architecture & System Design](./docs/01-architecture.md)**: Decoupled layered architecture, concurrency models, and security principles.
- **[02 - Database Schema & Data Models](./docs/02-database-schema.md)**: Complete 29-table PostgreSQL specification, indexing strategy, and encryption formats.
- **[03 - REST API & WebSocket Protocol Reference](./docs/03-api-reference.md)**: Endpoints, authentication handshakes, and WebSocket message schemas.
- **[04 - Background Jobs & Worker Pool](./docs/04-background-jobs-queue.md)**: Goroutine concurrency mechanics, job states, and progress reporting.
- **[05 - Logging & Troubleshooting Guide](./docs/05-troubleshooting-and-logging.md)**: Diagnostic strategies, file rotation, and error resolution.
- **[06 - Production Deployment Runbook](./docs/06-deployment-guide.md)**: Multi-container orchestration, SSL reverse proxies, and backup maintenance.
- **[07 - UI/UX Design System Standards](./docs/07-ui-ux-design-system.md)**: Component styling rules, theme colors, and layout guidelines.

---

## License & Copyright

This project is licensed under the **[MIT License](./LICENSE)**.  
Copyright (c) 2026 HONET Labs & Hephaestus Contributors.
