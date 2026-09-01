# Hephaestus Control Panel (HCP) - Architecture & System Design

## 1. Overview
**Hephaestus Control Panel (HCP)** is a modern, unified DevOps, Database, and Network Infrastructure Control Panel. It provides centralized management for remote server access (SSH/SFTP), network topology visualization, automated multi-database backups (S3/R2/NAS), SNMP telemetry, and observability integration (Prometheus, OpenSearch, Data Prepper).

### Core Stack
- **Backend**: Go 1.22+ (Gin Web Framework, `pgx/v5` PostgreSQL driver, `gorilla/websocket`, `gosnmp`, `robfig/cron/v3`, `zerolog`, `lumberjack`)
- **Database**: PostgreSQL 16+ (ACID transactional storage, JSONB configuration columns, B-tree and prefix search indexes)
- **Frontend**: Vue 3 (Composition API) + Vite + Tailwind CSS + Pinia + `@xterm/xterm` + Lucide Icons
- **Background Jobs**: Native In-Memory Go Worker Pool with concurrency limiting, progress tracking, and retries.

---

## 2. Layered Clean Architecture

The Go backend follows a decoupled, maintainable layered design pattern:

```
                  ┌─────────────────────────────────────┐
                  │            HTTP / WebSockets        │
                  │ (Gin Routers, Middlewares, Handlers)│
                  └──────────────────┬──────────────────┘
                                     │
                  ┌──────────────────▼──────────────────┐
                  │            Services Layer           │
                  │ (Business Logic, SSH/SFTP, Backups) │
                  └──────────┬───────────────────┬──────┘
                             │                   │
        ┌────────────────────▼─────┐       ┌─────▼────────────────────┐
        │     Repository Layer     │       │  Background Worker Pool  │
        │ (PostgreSQL pgx Queries) │       │   (Task Queue & Cron)    │
        └────────────────────┬─────┘       └──────────────────────────┘
                             │
                  ┌──────────▼──────────┐
                  │    Domain Models    │
                  │   & PostgreSQL DB   │
                  └─────────────────────┘
```

### Directory Breakdown
- `cmd/server/`: Main application entry point (`main.go`), router initialization, graceful shutdown lifecycle.
- `internal/core/domain/`: Pure domain data structs and models across all 11 modules.
- `internal/config/`: Configuration loader, environment defaults, AES-256-GCM crypto engine, dynamic `db_config.json` switching.
- `internal/database/`: PostgreSQL connection pool management (`pgxpool.Pool`) and schema migration execution.
- `internal/repository/`: Data persistence layer with prepared statements and parameter binding.
- `internal/services/`: Core business logic (SSH dialing, SFTP operations, Dump execution, SNMP walk, PromQL proxy, etc.).
- `internal/queue/`: In-memory worker pool with bounded job channels, task progress dispatch, and cancellation.
- `internal/scheduler/`: Cron engine for recurring jobs (Backups, ICMP ping sweep, Session garbage collection).
- `internal/middleware/`: Authentication token verification, RBAC, structured request logging, and panic recovery.
- `internal/handlers/`: Gin HTTP REST controllers and WebSocket handlers.
- `internal/logger/`: Zerolog structured logging, file rotation with Lumberjack, and live Pub/Sub log streaming.

---

## 3. Concurrency & Background Processing
Unlike external worker dependencies (e.g. Celery / BullMQ requiring Redis), Hephaestus uses Go's native CSP (Communicating Sequential Processes):
1. **Worker Pool (`internal/queue/worker_pool.go`)**:
   - Spawns fixed worker goroutines reading from a synchronized channel `chan *domain.Job`.
   - Thread-safe job state registry (`sync.RWMutex`) maintaining real-time execution status and progress percentages.
   - Context cancellation propagation via `context.WithCancel`.
2. **Cron Scheduler (`internal/scheduler/cron_scheduler.go`)**:
   - Automatically executes database backups per cron schedule expressions.
   - Triggers 60-second ICMP device health sweeps.
   - Cleans expired session tokens every 6 hours.

---

## 4. Security & Cryptography
- **AES-256-GCM Encryption**: All sensitive credentials (SSH passwords, private keys, database passwords, S3 secret keys) are stored encrypted.
- **Legacy Cipher Compatibility**: Replicates the `${iv}:${authTag}:${ciphertext}` hexadecimal format for seamless legacy database migrations.
- **Bcrypt Password Hashing**: User authentication passwords are hashed using bcrypt with cost factor 12.
- **Session Tokens**: Cryptographically secure 64-character hex tokens with idle expiration.
