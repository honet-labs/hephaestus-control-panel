# Database Schema Documentation

Hephaestus utilizes **PostgreSQL 16+** as its primary ACID transactional storage. The schema consists of 24 tables organized by domain module.

## 1. Schema Diagram Overview

```
 [users] ────< [sessions]
    │
    └───< [activity_logs]

 [remote_hosts] ────< [remote_host_tags]

 [topology_sheets] ────< [topology_devices] ────< [topology_device_pings]
         │                          │
         └──────────< [topology_edges] >───┘

 [backup_db_configs] ────────┐
                             ├───< [backup_schedules] ────< [backup_history]
 [backup_destinations] ──────┘

 [imported_mibs] ────< [oid_registries]

 [grafana_configs]  [prometheus_configs]  [opensearch_configs]  [dataprepper_configs]
```

---

## 2. Table Specifications

### 2.1 Authentication & System Users
1. `users`
   - `id`: `SERIAL PRIMARY KEY`
   - `username`: `VARCHAR(50) UNIQUE NOT NULL`
   - `password_hash`: `VARCHAR(255) NOT NULL` (Bcrypt)
   - `role`: `VARCHAR(20) DEFAULT 'operator'` (`admin`, `operator`, `viewer`)
   - `force_password_change`: `BOOLEAN DEFAULT false`
   - `created_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

2. `sessions`
   - `id`: `SERIAL PRIMARY KEY`
   - `user_id`: `INT REFERENCES users(id) ON DELETE CASCADE`
   - `token`: `VARCHAR(128) UNIQUE NOT NULL`
   - `created_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`
   - `expires_at`: `TIMESTAMPTZ NOT NULL`

3. `app_config`
   - `key`: `VARCHAR(100) PRIMARY KEY`
   - `value`: `TEXT`
   - `updated_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

4. `activity_logs`
   - `id`: `SERIAL PRIMARY KEY`
   - `user_id`: `INT REFERENCES users(id) ON DELETE SET NULL`
   - `username`: `VARCHAR(50)`
   - `module`: `VARCHAR(50) NOT NULL`
   - `action`: `VARCHAR(100) NOT NULL`
   - `details`: `TEXT`
   - `status`: `VARCHAR(20) DEFAULT 'SUCCESS'`
   - `timestamp`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

---

### 2.2 Remote Host Management (SSH / SFTP)
5. `remote_hosts`
   - `id`: `VARCHAR(64) PRIMARY KEY`
   - `name`: `VARCHAR(100) NOT NULL`
   - `host`: `VARCHAR(255) NOT NULL`
   - `port`: `INT DEFAULT 22`
   - `username`: `VARCHAR(100) NOT NULL`
   - `auth_type`: `VARCHAR(20) DEFAULT 'password'` (`password` or `key`)
   - `password_encrypted`: `TEXT` (AES-256-GCM)
   - `ssh_key_encrypted`: `TEXT` (AES-256-GCM)
   - `group_name`: `VARCHAR(100) DEFAULT 'Default'`
   - `created_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

6. `remote_host_tags`
   - `host_id`: `VARCHAR(64) REFERENCES remote_hosts(id) ON DELETE CASCADE`
   - `tag`: `VARCHAR(50) NOT NULL`
   - `PRIMARY KEY (host_id, tag)`

---

### 2.3 Network Topology & ICMP Monitoring
7. `topology_sheets`
   - `id`: `SERIAL PRIMARY KEY`
   - `name`: `VARCHAR(100) NOT NULL`
   - `sort_order`: `INT DEFAULT 0`
   - `created_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

8. `topology_devices`
   - `id`: `VARCHAR(64) PRIMARY KEY`
   - `sheet_id`: `INT REFERENCES topology_sheets(id) ON DELETE SET NULL`
   - `name`: `VARCHAR(100) NOT NULL`
   - `ip_address`: `VARCHAR(45) NOT NULL`
   - `device_type`: `VARCHAR(50) DEFAULT 'server'` (`server`, `switch`, `router`, `firewall`, `nas`, `vm`)
   - `pos_x`: `DOUBLE PRECISION DEFAULT 0`
   - `pos_y`: `DOUBLE PRECISION DEFAULT 0`
   - `status`: `VARCHAR(20) DEFAULT 'unknown'` (`online`, `offline`, `unknown`)
   - `sources`: `TEXT[]` (e.g. `{"PROM", "SCAN", "MANUAL"}`)
   - `labels`: `JSONB DEFAULT '{}'`
   - `created_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

9. `topology_edges`
   - `id`: `SERIAL PRIMARY KEY`
   - `sheet_id`: `INT REFERENCES topology_sheets(id) ON DELETE CASCADE`
   - `source_id`: `VARCHAR(64) REFERENCES topology_devices(id) ON DELETE CASCADE`
   - `target_id`: `VARCHAR(64) REFERENCES topology_devices(id) ON DELETE CASCADE`
   - `label`: `VARCHAR(100)`
   - `edge_type`: `VARCHAR(50) DEFAULT 'smoothstep'`
   - `animated`: `BOOLEAN DEFAULT false`

10. `topology_device_pings`
    - `id`: `SERIAL PRIMARY KEY`
    - `device_id`: `VARCHAR(64) REFERENCES topology_devices(id) ON DELETE CASCADE`
    - `ip`: `VARCHAR(45) NOT NULL`
    - `reachable`: `BOOLEAN NOT NULL`
    - `latency_ms`: `DOUBLE PRECISION`
    - `checked_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

---

### 2.4 Database Backups & Storage
11. `backup_db_configs`
    - `id`: `VARCHAR(64) PRIMARY KEY`
    - `name`: `VARCHAR(100) NOT NULL`
    - `db_type`: `VARCHAR(30) NOT NULL` (`postgresql`, `mysql`, `mariadb`, `sqlserver`)
    - `host`: `VARCHAR(255) NOT NULL`
    - `port`: `INT NOT NULL`
    - `username`: `VARCHAR(100) NOT NULL`
    - `password_encrypted`: `TEXT NOT NULL`
    - `database_name`: `VARCHAR(100) NOT NULL`
    - `ssh_host`: `VARCHAR(255)`
    - `ssh_port`: `INT`
    - `ssh_user`: `VARCHAR(100)`
    - `ssh_auth`: `VARCHAR(20)`
    - `ssh_password_encrypted`: `TEXT`
    - `ssh_key_encrypted`: `TEXT`

12. `backup_destinations`
    - `id`: `VARCHAR(64) PRIMARY KEY`
    - `name`: `VARCHAR(100) NOT NULL`
    - `dest_type`: `VARCHAR(30) NOT NULL` (`local`, `r2`, `s3`, `nas_sftp`)
    - `config`: `JSONB NOT NULL` (Path, Bucket, Endpoint, Access Keys)

13. `backup_schedules`
    - `id`: `VARCHAR(64) PRIMARY KEY`
    - `name`: `VARCHAR(100) NOT NULL`
    - `db_config_id`: `VARCHAR(64) REFERENCES backup_db_configs(id) ON DELETE CASCADE`
    - `destination_id`: `VARCHAR(64) REFERENCES backup_destinations(id) ON DELETE CASCADE`
    - `cron_expression`: `VARCHAR(100) NOT NULL`
    - `is_active`: `BOOLEAN DEFAULT true`
    - `last_run`: `TIMESTAMPTZ`

14. `backup_history`
    - `id`: `VARCHAR(64) PRIMARY KEY`
    - `db_config_id`: `VARCHAR(64) REFERENCES backup_db_configs(id) ON DELETE SET NULL`
    - `destination_id`: `VARCHAR(64) REFERENCES backup_destinations(id) ON DELETE SET NULL`
    - `db_name`: `VARCHAR(100) NOT NULL`
    - `db_type`: `VARCHAR(30) NOT NULL`
    - `dest_type`: `VARCHAR(30) NOT NULL`
    - `filename`: `VARCHAR(255) NOT NULL`
    - `file_size`: `BIGINT DEFAULT 0`
    - `status`: `VARCHAR(20) NOT NULL` (`running`, `success`, `failed`)
    - `error_message`: `TEXT`
    - `started_at`: `TIMESTAMPTZ NOT NULL`
    - `completed_at`: `TIMESTAMPTZ`

---

### 2.5 SNMP & MIB Registry
15. `imported_mibs`
    - `name`: `VARCHAR(100) PRIMARY KEY`
    - `node_count`: `INT DEFAULT 0`
    - `imported_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

16. `oid_registries`
    - `oid`: `VARCHAR(255) PRIMARY KEY`
    - `name`: `VARCHAR(255) NOT NULL`
    - `mib_name`: `VARCHAR(100) REFERENCES imported_mibs(name) ON DELETE CASCADE`
    - `description`: `TEXT`
    - `created_at`: `TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`

---

### 2.6 Observability & External Connectors
17. `grafana_configs`
18. `prometheus_configs`
19. `opensearch_configs`
20. `dataprepper_configs`
21. `monitoring_views`
22. `roles` & `role_permissions`
23. `backup_logs`
24. `schema_migrations`

---

## 3. Database Indexes
- `idx_sessions_token`: `ON sessions(token)`
- `idx_sessions_expires_at`: `ON sessions(expires_at)`
- `idx_topology_devices_sheet`: `ON topology_devices(sheet_id)`
- `idx_topology_devices_ip`: `ON topology_devices(ip_address)`
- `idx_topology_device_pings_time`: `ON topology_device_pings(checked_at DESC)`
- `idx_backup_history_started_at`: `ON backup_history(started_at DESC)`
- `idx_oid_registries_name`: `ON oid_registries(name)`
