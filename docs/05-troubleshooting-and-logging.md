# Logging & Troubleshooting Guide

## 1. Structured Logging Architecture

Hephaestus uses **Zerolog** with **Lumberjack** automated file rotation.

### Log Files
- **`logs/app.log`**: All application events (`DEBUG`, `INFO`, `WARN`, `ERROR`).
- **`logs/error.log`**: Exclusively error and fatal events.

### Rotation Policy
- **Max File Size**: 50 MB
- **Max Backups**: 10 files
- **Max Age**: 30 days
- **Compression**: Enabled (`.gz`)

---

## 2. In-App Real-time Log Stream
Logs are published to an in-memory Pub/Sub channel and broadcast via WebSocket to the UI at `/ws/logs`. Operators can view real-time events on the `/logs` page with live pause, module filtering, and log level filtering.

---

## 3. Common Issues & Troubleshooting

### Issue 1: "PostgreSQL Connection Refused"
- **Cause**: Target PostgreSQL server is unreachable, credentials in `db_config.json` are incorrect, or database service is down.
- **Resolution**:
  1. Check PostgreSQL container: `docker compose ps`
  2. Verify credentials using `psql -h <host> -U <user> -d <db>`
  3. Use the Setup Wizard (`/setup`) or Settings Page (`/settings`) to update the connection.

### Issue 2: "SSH Dial Handshake Timeout"
- **Cause**: Port 22 blocked by network firewall, or legacy crypto ciphers required.
- **Resolution**:
  1. Check connectivity: `ping <host>`
  2. Verify SSH port open: `nc -zv <host> <port>`
  3. Hephaestus already includes compatibility ciphers (`3des-cbc`, `aes128-cbc`, `diffie-hellman-group1-sha1`) for legacy MikroTik/Cisco hardware.

### Issue 3: "Database Dump Tool Not Found"
- **Cause**: `pg_dump` or `mysqldump` not in system `$PATH` for direct local dumps.
- **Resolution**:
  - In Docker deployment, `pg_dump` and `mysqldump` are pre-installed in the Alpine runtime container.
  - Alternatively, use **SSH Remote Dump** mode in Database Config to run the dump directly on the remote database server over SSH.
