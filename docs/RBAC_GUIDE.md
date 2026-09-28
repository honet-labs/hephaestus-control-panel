# Panduan Arsitektur RBAC & Granular Sharing (Hephaestus Control Panel)

Dokumen ini menjelaskan implementasi **Role-Based Access Control (RBAC)** dan **Granular Resource Sharing** di Hephaestus Control Panel (HCP). Dokumen ini ditujukan sebagai referensi resmi bagi tim developer dan administrator sistem.

---

## 1. Konsep Dasar Keamanan & Multi-Tenancy

Hephaestus menerapkan model keamanan berlapis (*Defense-in-Depth*):
1. **Autentikasi**: JWT Token (*Bearer Token*) yang diverifikasi pada setiap panggilan API via `middleware.AuthMiddleware()`.
2. **Level 1 - Feature Permission (Menu/Modul Access)**: Dikontrol oleh peran pengguna (`system_roles`), menentukan apakah user boleh membuka menu tertentu dan melakukan aksi `read` atau `manage`.
3. **Level 2 - Resource-Level Ownership & Granular Sharing**: Dikontrol oleh kolom `user_id`, `visibility`, dan tabel *share* per modul (seperti `monitoring_instance_shares`, `remote_host_shares`, dsb.). Resource yang berstatus `private` **TIDAK AKAN BISA DILIHAT** oleh pengguna lain kecuali dibagikan (*shared*) secara eksplisit.

---

## 2. Peran Sistem Bawaan (*Default System Roles*)

Tabel `system_roles` menyimpan definisi hak akses modul dalam format JSONB:

| Role | Deskripsi | Hak Akses Utama |
| :--- | :--- | :--- |
| **`ADMIN`** | Administrator Utama Sistem | Full access (`{"*": "manage"}`). Kebal terhadap pembatasan kepemilikan data (bisa melihat & mengelola semua data). |
| **`OPERATOR`** | Staf Operasional & Teknisi | `manage` pada modul operasional (monitoring, remote host, topology, monitoring instances, backups, docker, dsb.). Tunduk pada *visibility/sharing* kepemilikan resource non-admin. |
| **`VIEWER`** | Observer / Pengamat | `read` pada monitoring dan telemetri. `none` pada modul berisiko tinggi (*backup*, *security*, *settings*). Hanya bisa melihat resource yang `public` atau dibagikan ke akunnya. |

---

## 3. Fitur Sistem & Permission Matrix

Setiap modul di HCP direpresentasikan oleh key unik dalam field `permissions` JSONB:

```json
{
  "dashboard": "read" | "manage" | "none",
  "remote_servers": "read" | "manage" | "none",
  "monitoring_instances": "read" | "manage" | "none",
  "infrastructure": "read" | "manage" | "none",
  "network_topology": "read" | "manage" | "none",
  "backup": "read" | "manage" | "none",
  "connections": "read" | "manage" | "none",
  "security": "read" | "manage" | "none",
  "status_pages": "read" | "manage" | "none",
  "reports": "read" | "manage" | "none",
  "snmp": "read" | "manage" | "none",
  "opensearch": "read" | "manage" | "none",
  "grok_debugger": "read" | "manage" | "none",
  "dataprepper_config": "read" | "manage" | "none",
  "prometheus_config": "read" | "manage" | "none",
  "opentelemetry_config": "read" | "manage" | "none",
  "slideshow": "read" | "manage" | "none",
  "settings": "read" | "manage" | "none"
}
```

---

## 4. Pola Implementasi Granular Resource Sharing

Setiap entitas yang mendukung multi-tenancy (seperti `monitoring_instances`, `remote_host_configs`, `docker_connections`, `vaultwarden_configs`, `opensearch_configs`) menerapkan standar arsitektur berikut:

### 4.1. Skema Database

```sql
-- 1. Tabel Utama Entitas
CREATE TABLE IF NOT EXISTS monitoring_instances (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    ...
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL, -- Pemilik entitas
    visibility VARCHAR(20) NOT NULL DEFAULT 'public',        -- 'public' atau 'private'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_monitoring_instances_user_id ON monitoring_instances(user_id);

-- 2. Tabel Granular Sharing (One-to-Many)
CREATE TABLE IF NOT EXISTS monitoring_instance_shares (
    id VARCHAR(50) PRIMARY KEY,
    instance_id VARCHAR(50) NOT NULL REFERENCES monitoring_instances(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- User penerima share
    permission VARCHAR(20) NOT NULL DEFAULT 'read',                  -- 'read' atau 'manage'
    shared_by INTEGER REFERENCES users(id) ON DELETE SET NULL,       -- Siapa yang membagikan
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(instance_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_monitoring_instance_shares_inst ON monitoring_instance_shares(instance_id);
CREATE INDEX IF NOT EXISTS idx_monitoring_instance_shares_user ON monitoring_instance_shares(user_id);
```

### 4.2. Logika Query SQL di Repository

#### A. Untuk Admin (`domain.IsAdminRole(userRole)`):
Admin dapat melihat seluruh entitas baik `public` maupun `private`:
```sql
SELECT 
    m.*, 
    COALESCE(u.username, 'Admin') AS owner_username,
    (m.user_id = $1 OR m.user_id IS NULL) AS is_owner,
    'manage' AS user_permission,
    (SELECT COUNT(*) FROM monitoring_instance_shares s WHERE s.instance_id = m.id) AS shares_count
FROM monitoring_instances m
LEFT JOIN users u ON m.user_id = u.id
ORDER BY m.group_name ASC, m.name ASC
```

#### B. Untuk Non-Admin:
Data disaring ketat. Hanya menampilkan:
1. Resource milik user sendiri (`m.user_id = $1`).
2. Resource berstatus `public` (`m.visibility = 'public'`).
3. Resource yang dibagikan secara eksplisit kepada user di tabel share (`mis.user_id = $1`).

```sql
SELECT 
    m.*, 
    COALESCE(u.username, 'System') AS owner_username,
    (m.user_id = $1) AS is_owner,
    CASE 
        WHEN m.user_id = $1 THEN 'manage'
        WHEN mis.permission IS NOT NULL THEN mis.permission
        ELSE 'read'
    END AS user_permission,
    (SELECT COUNT(*) FROM monitoring_instance_shares s WHERE s.instance_id = m.id) AS shares_count
FROM monitoring_instances m
LEFT JOIN users u ON m.user_id = u.id
LEFT JOIN monitoring_instance_shares mis ON m.id = mis.instance_id AND mis.user_id = $1
WHERE (m.user_id = $1 OR m.visibility = 'public' OR mis.user_id = $1)
ORDER BY m.group_name ASC, m.name ASC
```

---

## 5. Proteksi API Backend

### 5.1. Route Middleware (`main.go`)
Gunakan `middleware.RequirePermission(feature, action)` pada router Gin:
```go
// Membaca daftar resource (read)
api.GET("/monitoring/instances", middleware.RequirePermission("monitoring_instances", "read"), handler.ListInstances)

// Menambah resource baru (manage)
api.POST("/monitoring/instances", middleware.RequirePermission("monitoring_instances", "manage"), handler.CreateInstance)

// Mengubah atau Menghapus resource (manage)
api.PUT("/monitoring/instances/:id", middleware.RequirePermission("monitoring_instances", "manage"), handler.UpdateInstance)
api.DELETE("/monitoring/instances/:id", middleware.RequirePermission("monitoring_instances", "manage"), handler.DeleteInstance)
```

### 5.2. Service-Level Access Verification
Pada operasi mutasi (`Update`, `Delete`, `Share`), panggil fungsi verifikasi akses:
```go
hasAccess, isOwner, perm, err := s.instRepo.CheckAccess(ctx, id, userID, userRole)
if err != nil {
    return err
}
if !hasAccess || (!isOwner && perm != "manage" && !domain.IsAdminRole(userRole)) {
    return fmt.Errorf("permission denied: manage access required")
}
```

---

## 6. Integrasi Frontend (Vue 3)

### 6.1. Pengecekan Izin Navigasi
Gunakan method bawaan `authStore.can(feature, action)` pada template atau router guard:
```html
<!-- Hanya tampil jika pengguna memiliki hak 'read' -->
<router-link
  v-if="authStore.can('monitoring_instances', 'read')"
  to="/monitoring/instances"
>
  Monitoring Instance
</router-link>

<!-- Tombol Tambah hanya tampil jika memiliki hak 'manage' -->
<button
  v-if="authStore.can('monitoring_instances', 'manage')"
  @click="openCreateModal"
>
  Add Host
</button>
```

### 6.2. Proteksi Aksi Item (Edit, Delete, Share)
Hanya tampilkan tombol aksi jika user adalah pemilik (*owner*), memiliki izin `manage`, atau user adalah `ADMIN`:
```html
<button
  v-if="inst.isOwner || inst.userPermission === 'manage' || authStore.user?.role?.toUpperCase() === 'ADMIN'"
  @click="openShareModal(inst)"
>
  Manage Shares
</button>
```

---

## 7. Checklist Pengembang saat Menambahkan Modul Baru

1. [ ] Tambahkan key permission pada migrasi `system_roles` (di `postgres.go` dan `init_schema.sql`).
2. [ ] Daftarkan nama key di `SYSTEM_FEATURES` pada `web/src/views/SettingsView.vue`.
3. [ ] Buat tabel utama dengan kolom `user_id` dan `visibility` (`public`/`private`).
4. [ ] Buat tabel sharing (`<modul>_shares`) dengan relasi `UNIQUE(resource_id, user_id)`.
5. [ ] Terapkan filtering `(user_id = $1 OR visibility = 'public' OR share.user_id = $1)` pada method `List` di repository.
6. [ ] Terapkan `middleware.RequirePermission` pada seluruh endpoint rute di `main.go`.
7. [ ] Bungkus tombol aksi di Vue template dengan `authStore.can(...)` atau `inst.isOwner`.
8. [ ] Tambahkan entri di `CommandPalette.vue` dan `AppLayout.vue`.
