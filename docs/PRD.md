# Product Requirement Document (PRD)
# Hephaestus Control Panel (HCP)

**Version**: 2.0.0  
**Status**: Approved & Implemented  
**Author**: HONET Labs Infrastructure Engineering Team  
**Target Release**: Production Ready (Q3 2026)  

---

## 1. Executive Summary & Problem Statement

### 1.1 Background
Infrastructure management in modern enterprise environments often suffers from severe toolchain fragmentation:
- Server administration requires standalone desktop SSH clients (PuTTY, Termius).
- Database backups rely on disparate crontab scripts with no centralized error tracking or automated multi-cloud uploaders.
- Network device mapping is performed manually or with disconnected network scanners.
- Observability and log streams are scattered across multiple dashboards with heavy resource requirements.

Legacy control panels (often built with dynamic interpreted runtimes) suffer from high RAM overhead, blocking event loops during long-running tasks, and lack of real-time multi-terminal orchestration.

### 1.2 The Solution: Hephaestus Control Panel (HCP)
**Hephaestus Control Panel (HCP)** is a high-performance, unified DevOps and Network Infrastructure Control Panel built on **Go 1.22+**, **PostgreSQL 16**, and **Vue 3 + Vite + Tailwind CSS**. It consolidates server administration, visual network topology mapping, automated multi-database backups, native in-memory task queues, SNMP telemetry, and live log auditing into a single, responsive web dashboard.

---

## 2. Product Goals & Objectives

### 2.1 Business Objectives
- **Centralized Control**: Single pane of glass for all server, database, and networking operations.
- **Zero Third-Party Queue Overhead**: Eliminate external queue broker dependencies (Redis/RabbitMQ) through native Go Goroutine Worker Pools.
- **Disaster Recovery Assurance**: Automated periodic database dumps with gzip compression and instant replication to Cloudflare R2 / AWS S3 / NAS.
- **Enterprise Security Compliance**: Zero plaintext credential storage using AES-256-GCM encryption and automated brute-force IP lockouts.

### 2.2 Technical Success Metrics
- **Memory Footprint**: Backend baseline RAM usage < 50MB.
- **Terminal Latency**: Interactive WebSocket SSH keystroke latency < 15ms.
- **Concurrent Execution**: Ability to handle 50+ concurrent background tasks without blocking HTTP API requests.
- **Deployment Time**: Fully automated single-command installation (`installer.sh`) < 3 minutes on any fresh Linux distribution.

---

## 3. User Personas & Core Use Cases

| Persona | Role | Primary Use Cases |
|---|---|---|
| **DevOps / SRE Engineer** | Infrastructure Management | Multi-tab SSH server access, split-screen broadcasting, Data Prepper pipeline editing, systemd service control. |
| **Database Administrator (DBA)** | Database Operations | Managing scheduled database dumps (Postgres/MySQL/SQL Server), monitoring backup health, verifying S3/R2 storage destinations. |
| **NOC Network Engineer** | Network Operations | Visual topology mapping, subnet ICMP sweeps, SNMP OID walks, Prometheus scrape target sync. |
| **Security Auditor** | Security & Compliance | Inspecting activity logs, reviewing user roles, auditing SSH sessions, verifying encrypted credential storage. |

---

## 4. Functional Requirements Specifications (FRS)

### 4.1 Module 1: Authentication, Access Control & Security
- **REQ-1.1 (Setup Wizard)**: If no administrator account exists, the application must automatically redirect to `/setup` for initial master account creation.
- **REQ-1.2 (Authentication)**: Secure cookie and Bearer token session authentication with configurable expiration (default 7 days).
- **REQ-1.3 (Role-Based Access Control)**: Enforce RBAC roles (`ADMIN`, `OPERATOR`, `VIEWER`) across all REST endpoints and WebSocket channels.
- **REQ-1.4 (Brute-Force Protection)**: Sliding-window rate limiter blocking IP addresses for 15 minutes after 5 consecutive failed login attempts.
- **REQ-1.5 (API Rate Limiting)**: General token-bucket rate limiter (120 req/min, burst 30) on all `/api/v1` routes.
- **REQ-1.6 (Data Encryption)**: All passwords, SSH private keys, and S3 credentials must be encrypted using AES-256-GCM with authentication tags.

### 4.2 Module 2: Remote Host & Terminal Management
- **REQ-2.1 (Host Profile Management)**: CRUD operations for SSH hosts with hostname, port, username, password, private key (PEM format), groups, and tags.
- **REQ-2.2 (Interactive Web Terminal)**: Full-duplex WebSocket interactive terminal powered by `@xterm/xterm` connected to Go pty allocating `xterm-256color`.
- **REQ-2.3 (Split-Screen Layouts)**: Support dynamic grid layouts: `1x1` (Single), `1x2` (Dual Column), `2x1` (Dual Row), and `2x2` (Quad Screen).
- **REQ-2.4 (Multi-Cast Broadcast Mode)**: Toggle allowing simultaneous broadcasting of keyboard input from the focused terminal to all active open split panes.
- **REQ-2.5 (Built-in SFTP Explorer)**: Modal-based remote filesystem explorer supporting directory navigation, file uploads (multipart), downloads (streaming), and deletions.
- **REQ-2.6 (Legacy Cipher Compatibility)**: Support legacy SSH ciphers (`3des-cbc`, `aes128-cbc`, `diffie-hellman-group1-sha1`) for Cisco, MikroTik, and older hardware.

### 4.3 Module 3: Network Topology & Auto-Discovery
- **REQ-3.1 (Topology Canvas)**: Interactive node-and-edge visual graph with sheet tabs and real-time node health pulse indicators.
- **REQ-3.2 (Subnet ICMP Sweeper)**: Concurrent ICMP discovery sweep across CIDR blocks (e.g. `192.168.1.0/24`) to auto-populate reachable devices.
- **REQ-3.3 (Prometheus Target Sync)**: Ingest active targets directly from Prometheus `/api/v1/targets` and map them into topology nodes.
- **REQ-3.4 (ICMP Ping Monitor)**: Background 60-second worker cycle pinging all registered devices, calculating latency, and updating database statuses.

### 4.4 Module 4: Database Disaster Recovery & Automated Backups
- **REQ-4.1 (Database Engines)**: Native dump support for PostgreSQL (`pg_dump`), MySQL / MariaDB (`mysqldump`), and SQL Server (`sqlcmd`).
- **REQ-4.2 (Execution Modes)**: Support both direct local execution and remote SSH dump execution for remote databases.
- **REQ-4.3 (Compression)**: Automated gzip compression stream for all database dump archives.
- **REQ-4.4 (Storage Destinations)**: Multi-destination uploaders for Local Filesystem, Cloudflare R2, AWS S3, and NAS (SFTP/SCP).
- **REQ-4.5 (Cron Scheduling)**: Standard cron expression parser (`robfig/cron/v3`) triggering automated recurring backups.
- **REQ-4.6 (History & Audit)**: Maintain complete execution history with file sizes, execution durations, timestamps, and error logs.

### 4.5 Module 5: Native In-Memory Background Job Queue
- **REQ-5.1 (Worker Pool Engine)**: Goroutine channel-based task queue with bounded worker concurrency (default: 5 concurrent workers).
- **REQ-5.2 (Job State Registry)**: Thread-safe job state tracking (`pending`, `running`, `completed`, `failed`, `cancelled`) with real-time percentage progress updates.
- **REQ-5.3 (Cancellation & Retries)**: Support `context.WithCancel` propagation for on-demand task cancellation and configurable retry attempts.
- **REQ-5.4 (Queue Dashboard)**: Dedicated Web UI view (`/queue`) displaying active jobs, progress bars, and cancellation triggers.

### 4.6 Module 6: SNMP Telemetry & MIB Registry
- **REQ-6.1 (GoSNMP Client)**: Native SNMP v1/v2c/v3 client executing `GET` and `WALK` operations without external OS binaries.
- **REQ-6.2 (MIB Parser)**: Syntax parser extracting `OBJECT-TYPE`, syntax definitions, and descriptions from `.mib` files.
- **REQ-6.3 (OID Resolution)**: Fast PostgreSQL prefix search indexing resolving raw OIDs to human-readable names.

### 4.7 Module 7: Observability & Utility Tools
- **REQ-7.1 (OpenSearch Hub)**: Real-time cluster health, node JVM metrics, and shard allocation stats.
- **REQ-7.2 (Prometheus Proxy)**: Interactive PromQL query execution and configuration reload trigger (`/-/reload`).
- **REQ-7.3 (VPS Telemetry & Systemd Control)**: Real-time OS CPU/RAM/Disk metrics via SSH and interactive systemd service management (`start`, `stop`, `restart`, `status`).
- **REQ-7.4 (Grok Debugger)**: Interactive regular expression pattern tester with preset patterns for log parsing.
- **REQ-7.5 (Data Prepper Manager)**: Pipeline YAML editor with real-time syntax validation.

### 4.8 Module 8: Structured Logging & Real-time Stream
- **REQ-8.1 (File Rotation)**: Structured JSON logging with Lumberjack rotation (`logs/app.log`, `logs/error.log` - 50MB limit, 10 backups, 30 days retention).
- **REQ-8.2 (Live Web Stream)**: In-memory Pub/Sub channel broadcasting log entries via WebSocket `/ws/logs`.
- **REQ-8.3 (Live Viewer UI)**: Web UI view with live pause/resume, log level filtering (`INFO`, `WARN`, `ERROR`, `DEBUG`), and module search.

---

## 5. Non-Functional Requirements (NFR)

### 5.1 Performance & Scalability
- **Sub-10ms API Response**: REST API endpoints must respond in < 10ms under standard operational load.
- **Connection Pooling**: PostgreSQL connections managed via `pgxpool.Pool` with idle connection reaping.
- **Frontend Bundle Size**: Compiled SPA bundle size < 1.5MB gzip.

### 5.2 Security Standards
- **OWASP Compliance**: Parameterized SQL queries to prevent SQL injection; XSS prevention through Vue 3 reactive DOM rendering.
- **Header Security**: CORS origin whitelisting, HTTP-only SameSite session cookies.
- **Audit Trails**: All administrative actions logged to `activity_logs` table with user IDs, timestamps, and IP addresses.

### 5.3 Reliability & Fault Tolerance
- **Graceful Shutdown**: Intercept `SIGINT` / `SIGTERM` with 10-second timeout draining active workers and HTTP connections.
- **Zero-Downtime DB Reconnect**: Dynamic database connection switching without requiring application restart.

### 5.4 Compatibility & Deployment
- **Linux Distro Support**: Ubuntu (20.04+), Debian (10+), Rocky Linux (8+), RHEL (8+), AlmaLinux (8+).
- **Multi-Container Separation**: Independent containers for `frontend` (Nginx), `backend` (Go), and `postgres` (PostgreSQL 16).

---

## 6. Architecture & Data Flow

```
+-------------------------------------------------------------------------------+
|                            Client Browser (Vue 3 SPA)                         |
|  [Dashboard]  [Split-Terminal]  [Topology Canvas]  [Backup Manager]  [Logs]   |
+---------------------------------------+---------------------------------------+
                                        | (HTTP / WebSockets on Port 80)
                                        v
+-------------------------------------------------------------------------------+
|                        hephaestus-frontend (Nginx Alpine)                     |
|  - Static Assets Cache & Gzip                                                 |
|  - Reverse Proxy: /api/* -> backend:5000 | /ws/* -> backend:5000              |
+---------------------------------------+---------------------------------------+
                                        |
                                        v
+-------------------------------------------------------------------------------+
|                        hephaestus-backend (Go 1.22 Runtime)                   |
|  - Gin HTTP Router & Middlewares (Auth, RateLimiter, Logger)                  |
|  - WebSocket Terminal & Live Log Broadcaster                                  |
|  - Background Worker Pool (Goroutines) & Cron Scheduler                       |
|  - Services: SSH/SFTP, Backup Dump Engine, GoSNMP, Topology Discovery         |
+---------------------------------------+---------------------------------------+
                                        |
                                        v
+-------------------------------------------------------------------------------+
|                     hephaestus-postgres (PostgreSQL 16 DB)                    |
|  - 24 Relational Tables | JSONB Configurations | B-Tree & Prefix Indexes      |
+-------------------------------------------------------------------------------+
```

---

## 7. Release & Deployment Strategy

- **Automated Installer**: Single-line installation via `curl -sSL .../installer.sh | sudo bash`.
- **CI/CD Automation**: GitHub Actions workflow (`.github/workflows/ci.yml`) compiling multi-arch images (`linux/amd64`, `linux/arm64`) on every push to `main`.
- **License**: MIT License (HONET Labs & Hephaestus Contributors).
