# REST API & WebSocket Protocol Reference

Base URL: `http://localhost:5000/api/v1`

## 1. Authentication

### POST `/auth/login`
Authenticate user credentials and receive a session bearer token.
```json
// Request Body
{
  "username": "admin",
  "password": "your-secure-password"
}

// Response (200 OK)
{
  "success": true,
  "data": {
    "token": "d13e24...64charHex",
    "user": {
      "id": 1,
      "username": "admin",
      "role": "ADMIN",
      "forcePasswordChange": false
    }
  }
}
```

### POST `/auth/logout`
Invalidate current session token.

---

## 2. Remote Host & SFTP

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/remote-host` | List all configured SSH servers |
| `POST` | `/remote-host` | Save / Update server profile |
| `DELETE` | `/remote-host/:id` | Delete server profile |
| `POST` | `/remote-host/test` | Test SSH dial connection |
| `GET` | `/remote-host/:id/sftp/list?path=/` | List SFTP remote directory |
| `POST` | `/remote-host/:id/sftp/upload?path=/` | Multipart file upload |
| `GET` | `/remote-host/:id/sftp/download?path=/` | Stream remote file download |

---

## 3. WebSocket Terminal (`/ws/remote-host`)

### Handshake Protocol
Client connects to:
`ws://localhost:5000/ws/remote-host?cols=80&rows=24`

**Step 1: Client sends authentication frame within 10s**:
```json
{
  "type": "auth",
  "token": "d13e24...sessionToken",
  "hostConfigId": "rhc-12345"
}
```

**Step 2: Interactive session frames**:
- Client Input: `{"type": "input", "data": "ls -la\n"}`
- Terminal Resize: `{"type": "resize", "cols": 120, "rows": 35}`
- Server Output: `{"type": "data", "data": "drwxr-xr-x 2 root root..."}`
- Disconnect: `{"type": "disconnected"}`

---

## 4. Live Log WebSocket (`/ws/logs`)
Real-time JSON log event streaming:
```json
{
  "timestamp": "2026-09-01T22:30:00Z",
  "level": "INFO",
  "module": "SSH",
  "message": "Connected to host 192.168.1.100",
  "fields": { "requestId": "req-123" }
}
```

---

## 5. Network Topology & Discovery

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/topology?sheetId=1` | Get nodes, edges, and statuses |
| `POST` | `/topology/devices` | Add new device node |
| `PUT` | `/topology/devices/:id/position` | Update X/Y canvas coordinate |
| `GET` | `/topology/discover/prometheus` | Auto-discover targets from Prometheus |
| `GET` | `/topology/discover/subnet?cidr=192.168.1.0/24` | ICMP sweep discovery |

---

## 6. Database Backups

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/backup/databases` | List database configs |
| `POST` | `/backup/databases` | Create database profile |
| `GET` | `/backup/destinations` | List S3/R2/NAS destinations |
| `POST` | `/backup/run` | Enqueue background backup job |
| `GET` | `/backup/history` | List execution history and status |

---

## 7. Background Task Queue

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/queue/jobs` | List all background jobs in worker pool |
| `GET` | `/queue/jobs/:id` | Get progress and status of specific job |
| `POST` | `/queue/jobs/:id/cancel` | Cancel running or pending job |
