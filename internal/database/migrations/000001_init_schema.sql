-- ==============================================================================
-- Hephaestus PostgreSQL Database Schema Initial Migration (v2.0 / Go Refactor)
-- Tables: 24 (Comprehensive configuration, credentials, topology, and logs)
-- ==============================================================================

-- 1. AppConfig - Key-value store for app settings (setup_completed, etc.)
CREATE TABLE IF NOT EXISTS app_config (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. SystemRoles - Available user roles with granular feature permissions
CREATE TABLE IF NOT EXISTS system_roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    description VARCHAR(255),
    permissions JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. Users - Local user accounts
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'operator',
    force_password_change BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 4. UserSessions - Active user login sessions
CREATE TABLE IF NOT EXISTS user_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 5. ActivityLogs - Audit trail of all system actions
CREATE TABLE IF NOT EXISTS activity_logs (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    module VARCHAR(100) NOT NULL,
    action VARCHAR(100) NOT NULL,
    details TEXT,
    status VARCHAR(50) DEFAULT 'SUCCESS',
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL
);

-- 6. GrafanaConfigs - Stores Grafana server connection profiles
CREATE TABLE IF NOT EXISTS grafana_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    token TEXT NOT NULL,
    datasource_uid VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 7. PrometheusConfigs - Stores Prometheus server connection profiles
CREATE TABLE IF NOT EXISTS prometheus_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mode VARCHAR(50) NOT NULL DEFAULT 'local',
    path VARCHAR(255) NOT NULL DEFAULT '/etc/prometheus/prometheus.yml',
    reload_url VARCHAR(255) NOT NULL DEFAULT 'http://localhost:9090/-/reload',
    ssh_host VARCHAR(255),
    ssh_port INTEGER DEFAULT 22,
    ssh_user VARCHAR(255),
    ssh_auth VARCHAR(50) DEFAULT 'password',
    ssh_password TEXT,
    ssh_key TEXT,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 8. MonitoringViews - Dashboard slideshow configurations
CREATE TABLE IF NOT EXISTS monitoring_views (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    interval INTEGER NOT NULL DEFAULT 10,
    mode VARCHAR(50) NOT NULL DEFAULT 'slideshow',
    panels JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 9. QueryPanels - Saved query configurations for data explorer
CREATE TABLE IF NOT EXISTS query_panels (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    datasource_type VARCHAR(50) NOT NULL,
    datasource_uid VARCHAR(100) NOT NULL,
    time_range_from VARCHAR(50) DEFAULT 'now-1h',
    time_range_to VARCHAR(50) DEFAULT 'now',
    step VARCHAR(50) DEFAULT '1m',
    columns JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 10. UptimeKumaConfigs - Uptime Kuma server connection profiles
CREATE TABLE IF NOT EXISTS uptime_kuma_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    password TEXT NOT NULL,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 11. DataPrepperConfigs - Data Prepper pipeline connection profiles
CREATE TABLE IF NOT EXISTS dataprepper_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mode VARCHAR(10) NOT NULL DEFAULT 'local',
    pipelines_dir TEXT NOT NULL DEFAULT '/opt/data-prepper/pipelines',
    reload_url TEXT,
    ssh_host VARCHAR(255),
    ssh_port INTEGER DEFAULT 22,
    ssh_user VARCHAR(255),
    ssh_auth VARCHAR(20) DEFAULT 'password',
    ssh_password TEXT,
    ssh_key TEXT,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 12. OpenSearchConfigs - OpenSearch cluster connection profiles
CREATE TABLE IF NOT EXISTS opensearch_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL DEFAULT 9200,
    username VARCHAR(255) NOT NULL,
    password TEXT NOT NULL DEFAULT '',
    use_ssl BOOLEAN DEFAULT false,
    verify_ssl BOOLEAN DEFAULT true,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 13. ImportedMibs - SNMP MIB modules imported into the system
CREATE TABLE IF NOT EXISTS imported_mibs (
    name VARCHAR(255) PRIMARY KEY,
    node_count INTEGER NOT NULL DEFAULT 0,
    imported_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 14. OidRegistry - OID definitions extracted from imported MIBs
CREATE TABLE IF NOT EXISTS oid_registry (
    oid VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mib_name VARCHAR(255) NOT NULL REFERENCES imported_mibs(name) ON DELETE CASCADE,
    syntax VARCHAR(255),
    access VARCHAR(255),
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 15. RemoteHostConfigs - SSH host configurations for terminal access
CREATE TABLE IF NOT EXISTS remote_host_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INTEGER DEFAULT 22,
    username VARCHAR(255) NOT NULL,
    auth_type VARCHAR(20) DEFAULT 'password',
    password TEXT,
    ssh_key TEXT,
    group_name VARCHAR(255) DEFAULT 'Default',
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 16. TopologySheets - Workspace sheets for organizing topology maps
CREATE TABLE IF NOT EXISTS topology_sheets (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 17. TopologyDevices - Manually added or auto-discovered network devices
CREATE TABLE IF NOT EXISTS topology_devices (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    device_type VARCHAR(50) DEFAULT 'unknown',
    status VARCHAR(20) DEFAULT 'unknown',
    sources TEXT[] DEFAULT '{}',
    labels JSONB DEFAULT '{}',
    interfaces JSONB DEFAULT '[]',
    sheet_id INTEGER REFERENCES topology_sheets(id) ON DELETE SET NULL,
    x DOUBLE PRECISION,
    y DOUBLE PRECISION,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 18. TopologyEdges - Connections between devices
CREATE TABLE IF NOT EXISTS topology_edges (
    id SERIAL PRIMARY KEY,
    source_id VARCHAR(50) NOT NULL REFERENCES topology_devices(id) ON DELETE CASCADE,
    target_id VARCHAR(50) NOT NULL REFERENCES topology_devices(id) ON DELETE CASCADE,
    label VARCHAR(100),
    source_label VARCHAR(100),
    target_label VARCHAR(100),
    edge_type VARCHAR(50) DEFAULT 'ethernet',
    sheet_id INTEGER REFERENCES topology_sheets(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT topology_edges_sheet_source_target_key UNIQUE (source_id, target_id, sheet_id)
);

-- 19. TopologyPending - Persist scan results before adding to map
CREATE TABLE IF NOT EXISTS topology_pending (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 20. DevicePingResults - ICMP ping results for topology devices
CREATE TABLE IF NOT EXISTS device_ping_results (
    device_id VARCHAR(50) PRIMARY KEY REFERENCES topology_devices(id) ON DELETE CASCADE,
    ip VARCHAR(45) NOT NULL,
    reachable BOOLEAN DEFAULT false,
    latency_ms DOUBLE PRECISION,
    checked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 21. BackupDatabaseConfigs - Database connections for backup
CREATE TABLE IF NOT EXISTS backup_database_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    db_type VARCHAR(50) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL,
    username VARCHAR(255) NOT NULL,
    password TEXT NOT NULL,
    database_name VARCHAR(255) NOT NULL,
    ssh_host VARCHAR(255),
    ssh_port INTEGER DEFAULT 22,
    ssh_user VARCHAR(255),
    ssh_auth VARCHAR(20) DEFAULT 'password',
    ssh_password TEXT,
    ssh_key TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 22. BackupDestinations - Storage destinations for backups
CREATE TABLE IF NOT EXISTS backup_destinations (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    dest_type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 23. BackupHistory - Backup execution history
CREATE TABLE IF NOT EXISTS backup_history (
    id VARCHAR(50) PRIMARY KEY,
    db_config_id VARCHAR(50) REFERENCES backup_database_configs(id) ON DELETE SET NULL,
    destination_id VARCHAR(50) REFERENCES backup_destinations(id) ON DELETE SET NULL,
    db_name VARCHAR(255) NOT NULL,
    db_type VARCHAR(50) NOT NULL,
    dest_type VARCHAR(50) NOT NULL,
    filename VARCHAR(500) NOT NULL,
    file_size BIGINT DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE
);

-- 24. BackupSchedules - Automated backup scheduling
CREATE TABLE IF NOT EXISTS backup_schedules (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    db_config_id VARCHAR(50) REFERENCES backup_database_configs(id) ON DELETE CASCADE,
    destination_id VARCHAR(50) REFERENCES backup_destinations(id) ON DELETE CASCADE,
    cron_expression VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    last_run TIMESTAMP WITH TIME ZONE,
    next_run TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ==============================================================================
-- INDEXES FOR PERFORMANCE
-- ==============================================================================
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(token);
CREATE INDEX IF NOT EXISTS idx_activity_logs_timestamp ON activity_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_activity_logs_user_id ON activity_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_grafana_configs_is_active ON grafana_configs(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_prometheus_configs_is_active ON prometheus_configs(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_uptime_kuma_configs_is_active ON uptime_kuma_configs(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_opensearch_configs_is_active ON opensearch_configs(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_dataprepper_configs_is_active ON dataprepper_configs(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_oid_registry_mib_name ON oid_registry(mib_name);
CREATE INDEX IF NOT EXISTS idx_oid_registry_name ON oid_registry(name);
CREATE INDEX IF NOT EXISTS idx_oid_registry_lower_name ON oid_registry(lower(name));
CREATE INDEX IF NOT EXISTS idx_oid_registry_lower_oid ON oid_registry(lower(oid));
CREATE INDEX IF NOT EXISTS idx_topology_devices_ip ON topology_devices(ip_address);
CREATE INDEX IF NOT EXISTS idx_topology_devices_sheet ON topology_devices(sheet_id);
CREATE INDEX IF NOT EXISTS idx_topology_edges_source ON topology_edges(source_id);
CREATE INDEX IF NOT EXISTS idx_topology_edges_target ON topology_edges(target_id);
CREATE INDEX IF NOT EXISTS idx_topology_edges_sheet ON topology_edges(sheet_id);
CREATE INDEX IF NOT EXISTS idx_topology_pending_user ON topology_pending(user_id);
CREATE INDEX IF NOT EXISTS idx_backup_schedules_is_active ON backup_schedules(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_backup_history_started_at ON backup_history(started_at DESC);

-- ==============================================================================
-- SEED DEFAULT DATA
-- ==============================================================================
INSERT INTO system_roles (name, description, is_default, permissions) VALUES 
    ('ADMIN', 'Full system administrator with unrestricted access', true, '{"*": "manage"}'::jsonb),
    ('OPERATOR', 'Operational user with read and manage access to monitoring, servers, and network', true, '{"dashboard": "manage", "remote_servers": "manage", "network_topology": "manage", "backup": "read", "connections": "read", "snmp": "manage", "opensearch": "read", "grok_debugger": "read", "dataprepper_config": "read", "prometheus_config": "read", "slideshow": "read", "settings": "read"}'::jsonb),
    ('VIEWER', 'Read-only observer access across all monitoring and telemetry views', true, '{"dashboard": "read", "remote_servers": "read", "network_topology": "read", "backup": "read", "connections": "read", "snmp": "read", "opensearch": "read", "grok_debugger": "read", "dataprepper_config": "read", "prometheus_config": "read", "slideshow": "read", "settings": "none"}'::jsonb)
ON CONFLICT (name) DO UPDATE SET 
    permissions = EXCLUDED.permissions,
    description = EXCLUDED.description;

INSERT INTO app_config (key, value) VALUES 
    ('setup_completed', 'false'),
    ('system_name', 'Hephaestus Control Panel (HCP)'),
    ('alert_telegram_enabled', 'false'),
    ('alert_discord_enabled', 'false')
ON CONFLICT (key) DO NOTHING;

INSERT INTO topology_sheets (name, sort_order) 
SELECT 'Tab 1', 0 
WHERE NOT EXISTS (SELECT 1 FROM topology_sheets);
