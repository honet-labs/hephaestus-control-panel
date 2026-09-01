# Hephaestus (Go + Vue 3 Edition)

> **Next-Generation DevOps & Network Infrastructure Control Plane**  
> High-performance Go backend, PostgreSQL ACID persistence, native in-memory worker pool, and sleek modern Vue 3 + Tailwind CSS web interface.

---

## 🚀 Key Features

- **💻 Multi-Tab SSH Terminal & SFTP Browser**: Seamless interactive browser terminal (`xterm.js` over WebSockets) with native SFTP file upload/download/management.
- **🌐 Network Topology & Auto-Discovery**: Interactive visual network graph with Prometheus scrape target synchronization and concurrent ICMP subnet sweeps.
- **🗄️ Multi-Database Automated Backups**: Periodic and on-demand dumps for PostgreSQL, MySQL, MariaDB, and SQL Server with gzip compression and Cloudflare R2 / AWS S3 / NAS uploaders.
- **⚡ In-Memory Background Job & Worker Pool**: Built-in Go concurrency queue with live progress percentage updates, retry mechanisms, and cancellation.
- **📡 SNMP & MIB Registry**: OID browser, automated MIB file parser, and live SNMP walk/get queries.
- **🔍 Observability Hub**: OpenSearch cluster telemetry, Prometheus PromQL querying, and Data Prepper pipeline manager.
- **📋 In-App Live Log Viewer**: Structured logging (Zerolog + Lumberjack rotation) streamed live via WebSockets to the web interface.
- **🛡️ Enterprise Security**: AES-256-GCM encrypted credentials, Bcrypt user authentication, session guards, and role-based access control.

---

## 🏗️ Multi-Container Architecture

Each core component runs in its own dedicated, isolated Docker container:

| Service | Container Name | Image / Base | Role |
|---|---|---|---|
| **Frontend** | `hephaestus-frontend` | `nginx:1.27-alpine` | Serves Vue 3 SPA + Gzip + Reverse Proxy (`/api`, `/ws`) |
| **Backend** | `hephaestus-backend` | `alpine:3.20` + Go 1.22 binary | REST API, Worker Pool, Scheduler, SSH/SFTP engine |
| **Database** | `hephaestus-postgres` | `postgres:16-alpine` | PostgreSQL 16 transactional database (24 tables) |

---

## ⚡ Quick Start: Automated Linux Installer (`install.sh`)

Compatible with **Ubuntu, Debian, Rocky Linux, RHEL, and AlmaLinux**:

```bash
# 1. Clone repository
git clone https://github.com/honet-labs/hephaestus.git
cd hephaestus

# 2. Make installer executable and run as root
chmod +x install.sh
sudo ./install.sh
```

The script will automatically:
1. Detect your Linux distribution.
2. Install dependencies (`curl`, `git`, `openssl`, `jq`, etc.) and official Docker Engine.
3. Generate secure cryptographic keys (`APP_ENCRYPTION_KEY`, DB passwords) in `.env`.
4. Configure firewall ports (UFW / Firewalld).
5. Build and launch all 3 containers (`frontend`, `backend`, `postgres`).

Access the Web UI at **`http://<SERVER_IP>`** or **`http://localhost`**.

---

## 📁 Project Structure

```
go-hephaestus/
├── cmd/
│   └── server/main.go            # Application entry point & router setup
├── internal/
│   ├── config/                   # Config loader, crypto (AES-256-GCM), env
│   ├── core/domain/              # Domain entities and struct models
│   ├── database/                 # PostgreSQL pool (pgxpool) & migrations
│   ├── handlers/                 # Gin HTTP REST & WebSocket handlers
│   ├── logger/                   # Zerolog + Lumberjack rotation + Pub/Sub
│   ├── middleware/               # Auth, RBAC, Request Logger, Rate Limiter
│   ├── queue/                    # Background Worker Pool engine
│   ├── repository/               # PostgreSQL data access layer (24 tables)
│   ├── scheduler/                # Cron engine (Backups, Ping, Cleanup)
│   └── services/                 # SSH, SFTP, Backup, SNMP, Topology, PromQL
├── web/                          # Modern Vue 3 + Vite + Tailwind CSS frontend
│   ├── nginx.conf                # Nginx SPA & reverse proxy configuration
│   └── src/                      # Pinia stores, router, layouts, views
├── docs/                         # Comprehensive architectural and API docs
│   ├── 01-architecture.md
│   ├── 02-database-schema.md
│   ├── 03-api-reference.md
│   ├── 04-background-jobs-queue.md
│   ├── 05-troubleshooting-and-logging.md
│   └── 06-deployment-guide.md
├── Dockerfile.backend            # Go backend production container
├── Dockerfile.frontend           # Vue 3 + Nginx production container
├── docker-compose.yml            # Multi-container stack definition
├── install.sh                    # Universal Linux automated installer
├── Makefile                      # Developer and build automation tasks
└── README.md
```

---

## 📖 Documentation

Detailed technical documentation is available in [`docs/`](./docs):
- [01 - Architecture & System Design](./docs/01-architecture.md)
- [02 - Database Schema (24 Tables)](./docs/02-database-schema.md)
- [03 - REST API & WebSocket Protocol Reference](./docs/03-api-reference.md)
- [04 - Background Jobs & Worker Pool](./docs/04-background-jobs-queue.md)
- [05 - Logging & Troubleshooting Guide](./docs/05-troubleshooting-and-logging.md)
- [06 - Production Deployment Runbook](./docs/06-deployment-guide.md)

---

## 📜 License
MIT License. Developed for HONET Infrastructure Operations.
