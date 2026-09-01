# Deployment & Production Runbook

## 1. Automated One-Command Installation (`install.sh`)

Hephaestus includes an automated universal installer for Linux servers.

### Supported Operating Systems
- **Ubuntu**: 20.04, 22.04, 24.04 LTS
- **Debian**: 10, 11, 12
- **Rocky Linux**: 8, 9
- **RHEL / AlmaLinux / CentOS Stream**: 8, 9

### Automated Actions Performed by `install.sh`:
1. Detects OS family (APT vs DNF/YUM).
2. Downloads and installs system dependencies (`curl`, `git`, `openssl`, `jq`).
3. Installs official Docker Engine & Docker Compose Plugin.
4. Generates a secure 64-character hexadecimal `APP_ENCRYPTION_KEY` and random PostgreSQL database password.
5. Generates `.env` file and opens HTTP ports in UFW / Firewalld.
6. Builds and deploys the **multi-container stack** (`hephaestus-frontend`, `hephaestus-backend`, `hephaestus-postgres`).

### Quick Installation Command
```bash
# Clone and run installer
git clone https://github.com/honet-labs/hephaestus.git
cd hephaestus
chmod +x install.sh
sudo ./install.sh
```

---

## 2. Multi-Container Architecture Overview

Each core service runs in its own dedicated, isolated Docker container:

```
                  [Client Browser]
                         │
                         ▼ (Port 80 / 443)
            ┌──────────────────────────────┐
            │      hephaestus-frontend     │
            │  (Vue 3 SPA on Nginx Alpine) │
            └──────────────┬───────────────┘
                           │
             ┌─────────────┴─────────────┐
             │ proxy_pass /api & /ws     │
             ▼                           │
   ┌──────────────────────┐              │
   │  hephaestus-backend  │              │
   │ (Go API & WebSocket) │              │
   └──────────┬───────────┘              │
              │                          │
              │ pgx connection pool      │
              ▼                          ▼
   ┌──────────────────────┐    ┌────────────────────┐
   │  hephaestus-postgres │    │   Docker Volumes   │
   │   (PostgreSQL 16)    │    │ (pg_data, app_data)│
   └──────────────────────┘    └────────────────────┘
```

### Container Breakdown

1. **`hephaestus-frontend`** (`Dockerfile.frontend`):
   - Builds Vue 3 + Vite + Tailwind CSS into static assets.
   - Served via Nginx Alpine with gzip compression and cache headers.
   - Handles reverse proxying for `/api/` and WebSocket `/ws/` to backend.
2. **`hephaestus-backend`** (`Dockerfile.backend`):
   - Go 1.22 REST API, Worker Pool queue, Cron scheduler, SSH/SFTP engine.
   - Healthcheck on `/health`.
3. **`hephaestus-postgres`** (`postgres:16-alpine`):
   - Relational persistence with 24 tables and automated schema migrations.

---

## 3. Manual Docker Compose Operations

```bash
# Start all containers in background
docker compose up -d

# View live container logs
docker compose logs -f

# View status and health
docker compose ps

# Stop all containers
docker compose down

# Rebuild specific service (e.g. backend)
docker compose up -d --build backend
```

---

## 4. Reverse Proxy with SSL (Nginx / Caddy / Cloudflare)

If running behind an external domain with HTTPS, point your domain to port 80:

```nginx
server {
    listen 443 ssl http2;
    server_name controlplane.example.com;

    ssl_certificate /etc/letsencrypt/live/controlplane.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/controlplane.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:80;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket timeouts for terminal
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
```
