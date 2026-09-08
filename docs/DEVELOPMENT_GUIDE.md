# Hephaestus Control Panel (HCP)
# Developer & UI/UX Engineering Guide

> **Document Version**: 1.0.0  
> **Last Updated**: September 2026  
> **Target Audience**: Backend Engineers, Frontend Engineers, DevOps Developers  
> **Scope**: Architecture, UI/UX Design System, Feature Implementation Recipe, Clean Code Standards

---

## 1. Architectural Philosophy & Overview

Hephaestus Control Panel (HCP) is built to deliver low-latency, high-throughput DevOps and Network Operations infrastructure monitoring with a minimal memory footprint (< 50MB baseline).

### 1.1 Technology Stack

```
+--------------------------------------------------------------------------------+
| FRONTEND: Vue 3 (Composition API) + TypeScript + Vite + Tailwind CSS + Lucide   |
+--------------------------------------------------------------------------------+
                                        | Reverse Proxy (Port 80 / Nginx)
                                        v
+--------------------------------------------------------------------------------+
| BACKEND: Go 1.22+ Clean Architecture (Gin HTTP + WebSockets + Native Worker)   |
+--------------------------------------------------------------------------------+
                                        | pgxpool Connection Pool (Port 5432)
                                        v
+--------------------------------------------------------------------------------+
| DATABASE: PostgreSQL 16 ACID Storage (AES-256-GCM Encrypted Sensitive Secrets)  |
+--------------------------------------------------------------------------------+
```

### 1.2 Backend Layered Architecture (Clean Architecture)

```
[cmd/server/main.go]
         |
         v
[internal/routes] --------> [internal/middleware] (Auth, RBAC, RateLimit, Audit)
         |
         v
[internal/handlers] ------> Controller layer (HTTP parsing, DTO binding, Status codes)
         |
         v
[internal/services] ------> Business logic, AES encryption, SSH/SNMP/Cron execution
         |
         v
[internal/repository] ----> SQL queries, pgxpool transactions, Scan mapping
         |
         v
[internal/domain] --------> Pure entity structs, DTO contracts, Enums, Feature keys
```

---

## 2. UI/UX Design System & Aesthetic Standards

HCP is an **enterprise-grade NOC (Network Operations Center)** tool. It must look sleek, professional, modern, and serious. It is **NOT** a playful consumer app.

### 2.1 Theme & Color Palette

The interface uses a tailored dark slate palette. Avoid bright saturated backgrounds or unnecessary neon gradients.

| Element | Tailwind Class / Hex Value | Purpose |
|---|---|---|
| **Canvas Background** | `bg-[#090d16]` | Base background of the entire dashboard |
| **Card Surface** | `bg-[#13161f]` | Standard content cards and containers |
| **Elevated Surface** | `bg-[#171a23]` | Tables, modal dialogs, flyout drawers |
| **Header / Table Head** | `bg-[#1c202b]` or `bg-[#14161b]` | Sticky table headers, top control bars |
| **Active / Hover State** | `bg-slate-800` or `bg-[#20242e]` | Button hover, active segmented pills |
| **Borders** | `border-slate-800` (default), `border-slate-700` (inputs/modals) | Structural separation |
| **Primary Text** | `text-white` | Page headings, active items, modal titles |
| **Secondary Text** | `text-slate-200` / `text-slate-300` | Body copy, table cells, form labels |
| **Muted Text** | `text-slate-400` / `text-slate-500` | Helper text, timestamps, subtitles |

### 2.2 Icon System Rules (MANDATORY)

1. **Library**: Use **ONLY** `lucide-vue-next`. Do not install or mix other icon libraries (e.g. FontAwesome, Material Icons).
2. **Neutral Monochrome Styling**:
   - General toolbar icons, navigation buttons, and table action buttons must use **neutral monochrome styles**:
     - Inactive / default: `text-slate-400`
     - Hover / focused: `hover:text-white` or `hover:text-slate-200`
   - **DO NOT** use arbitrary bright colors on utility icons (e.g. avoid `text-sky-400`, `text-brand-400`, `text-purple-400` on refresh, edit, or copy buttons).
3. **Strict Icon Sizing**:
   - Micro badges & inline buttons: `w-3 h-3` or `w-3.5 h-3.5`
   - Standard button icons: `w-4 h-4`
   - Modal headers & section headers: `w-5 h-5`
   - Empty state placeholders: `w-8 h-8` (placed inside a `w-16 h-16` rounded container)

### 2.3 Strict Ban on Emojis & Emoticons

> [!CAUTION]
> **NEVER USE UNICODE EMOJIS IN THE UI.**  
> Emojis (e.g. ⚡, 👁️, 💻, 🏢, 🔀, ➔, 🚀, 📌, ⚠️, ❌) render inconsistently across Windows, macOS, and Linux, and break the enterprise NOC aesthetic.

- Replace emoji arrows (`➔`) with `<ArrowRight class="w-3.5 h-3.5" />`
- Replace server emoji (`🏢`) with `<Server class="w-3.5 h-3.5" />`
- Replace laptop emoji (`💻`) with `<Monitor class="w-3.5 h-3.5" />`
- Replace swap emoji (`🔀`) with `<RotateCw class="w-3.5 h-3.5" />` or `<ArrowLeftRight class="w-3.5 h-3.5" />`
- Replace lightning emoji (`⚡`) with `<Zap class="w-3.5 h-3.5" />` or clean text

### 2.4 Semantic Status Colors (Status Only!)

Color is reserved **strictly** for semantic operational status indicators:

```html
<!-- Healthy / Connected / Active -->
<span class="w-2 h-2 rounded-full bg-emerald-500"></span>
<span class="text-xs text-emerald-400 font-semibold">Online</span>

<!-- Warning / Degraded / Paused -->
<span class="w-2 h-2 rounded-full bg-amber-500"></span>
<span class="text-xs text-amber-400 font-semibold">Degraded</span>

<!-- Critical / Offline / Error -->
<span class="w-2 h-2 rounded-full bg-rose-500"></span>
<span class="text-xs text-rose-400 font-semibold">Offline</span>
```

### 2.5 Component Design Patterns

#### Standard Button Hierarchy
```html
<!-- Primary Action (Call to Action / Submit) -->
<button class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white shadow-md transition">
  <Plus class="w-3.5 h-3.5" />
  <span>Add Host</span>
</button>

<!-- Secondary / Neutral Action -->
<button class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-[#090d16] hover:bg-slate-800 text-slate-200 border border-slate-700 transition">
  <RotateCw class="w-3.5 h-3.5 text-slate-400" />
  <span>Refresh</span>
</button>

<!-- Danger Action -->
<button class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-rose-600/10 hover:bg-rose-600/20 text-rose-400 border border-rose-500/30 transition">
  <Trash2 class="w-3.5 h-3.5 text-rose-400" />
  <span>Delete</span>
</button>
```

#### Modal Dialog Structure
```html
<div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 animate-in fade-in duration-150">
  <div class="w-full max-w-lg bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 text-white">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-slate-800 pb-3">
      <h3 class="text-sm font-bold flex items-center gap-2">
        <Server class="w-4 h-4 text-slate-400" />
        <span>Configure Service</span>
      </h3>
      <button @click="isOpen = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition">
        <X class="w-4 h-4" />
      </button>
    </div>
    
    <!-- Body -->
    <div class="space-y-3 text-xs">
      <!-- Input fields -->
    </div>

    <!-- Footer -->
    <div class="flex justify-end gap-2 pt-3 border-t border-slate-800">
      <button type="button" @click="isOpen = false" class="px-3 py-2 text-slate-400 hover:text-white transition">Cancel</button>
      <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow transition">Save</button>
    </div>
  </div>
</div>
```

---

## 3. Step-by-Step Recipe: Adding a New Configuration / Feature

When introducing a new feature or remote configuration module (e.g. a new telemetry agent, new cloud provider, or new observability module), follow this systematic 7-step blueprint.

---

### Step 1: Database Migration (SQL)

Location: `internal/database/migrations/`

> [!IMPORTANT]
> Always write **non-destructive, idempotent migrations**. Existing user databases must not fail when running `ALTER TABLE` or `CREATE TABLE`.

```sql
-- internal/database/migrations/000008_create_cloud_targets.sql

CREATE TABLE IF NOT EXISTS cloud_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    access_key VARCHAR(255) NOT NULL,
    secret_key_encrypted TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cloud_targets_provider ON cloud_targets(provider);
```

If altering an existing table:
```sql
ALTER TABLE system_roles ADD COLUMN IF NOT EXISTS permissions JSONB DEFAULT '{}'::jsonb;
```

---

### Step 2: Domain Entity Model (`internal/domain/`)

Location: `internal/domain/cloud_target.go`

```go
package domain

import (
	"time"
)

type CloudTarget struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Provider           string    `json:"provider"`
	Endpoint           string    `json:"endpoint"`
	AccessKey          string    `json:"accessKey"`
	SecretKeyEncrypted string    `json:"-"` // Never expose encrypted ciphertext to JSON directly
	IsActive           bool      `json:"isActive"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type CreateCloudTargetRequest struct {
	Name      string `json:"name" binding:"required"`
	Provider  string `json:"provider" binding:"required"`
	Endpoint  string `json:"endpoint" binding:"required"`
	AccessKey string `json:"accessKey" binding:"required"`
	SecretKey string `json:"secretKey" binding:"required"`
	IsActive  bool   `json:"isActive"`
}
```

---

### Step 3: Repository Layer (`internal/repository/`)

Location: `internal/repository/cloud_target_repository.go`

```go
package repository

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"go-hephaestus/internal/domain"
)

type CloudTargetRepository interface {
	GetAll(ctx context.Context) ([]domain.CloudTarget, error)
	GetByID(ctx context.Context, id string) (*domain.CloudTarget, error)
	Create(ctx context.Context, target *domain.CloudTarget) error
	Delete(ctx context.Context, id string) error
}

type cloudTargetRepo struct {
	db *pgxpool.Pool
}

func NewCloudTargetRepository(db *pgxpool.Pool) CloudTargetRepository {
	return &cloudTargetRepo{db: db}
}

func (r *cloudTargetRepo) GetAll(ctx context.Context) ([]domain.CloudTarget, error) {
	query := `SELECT id, name, provider, endpoint, access_key, secret_key_encrypted, is_active, created_at, updated_at 
	          FROM cloud_targets ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query cloud targets: %w", err)
	}
	defer rows.Close()

	var list []domain.CloudTarget
	for rows.Next() {
		var item domain.CloudTarget
		if err := rows.Scan(&item.ID, &item.Name, &item.Provider, &item.Endpoint, &item.AccessKey, &item.SecretKeyEncrypted, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		list = append(list, item)
	}
	return list, nil
}
```

---

### Step 4: Service Layer & Encryption (`internal/services/`)

Location: `internal/services/cloud_target_service.go`

> [!IMPORTANT]
> All credentials (passwords, tokens, private keys) **MUST** be encrypted before persisting to the database using `crypto.EncryptAES256GCM(plainText, masterKey)`.

```go
package services

import (
	"context"
	"fmt"
	"go-hephaestus/internal/crypto"
	"go-hephaestus/internal/domain"
	"go-hephaestus/internal/repository"
)

type CloudTargetService interface {
	CreateTarget(ctx context.Context, req domain.CreateCloudTargetRequest) (*domain.CloudTarget, error)
	ListTargets(ctx context.Context) ([]domain.CloudTarget, error)
}

type cloudTargetService struct {
	repo          repository.CloudTargetRepository
	encryptionKey []byte
}

func NewCloudTargetService(repo repository.CloudTargetRepository, encKey []byte) CloudTargetService {
	return &cloudTargetService{repo: repo, encryptionKey: encKey}
}

func (s *cloudTargetService) CreateTarget(ctx context.Context, req domain.CreateCloudTargetRequest) (*domain.CloudTarget, error) {
	// 1. Encrypt secret key
	encryptedSecret, err := crypto.EncryptAES256(req.SecretKey, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret key: %w", err)
	}

	entity := &domain.CloudTarget{
		Name:               req.Name,
		Provider:           req.Provider,
		Endpoint:           req.Endpoint,
		AccessKey:          req.AccessKey,
		SecretKeyEncrypted: encryptedSecret,
		IsActive:           req.IsActive,
	}

	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}
```

---

### Step 5: Handler & Granular RBAC Permissions

Location: `internal/handlers/cloud_target_handler.go` and `internal/domain/roles.go`

1. **Register Feature Key** in `internal/domain/roles.go`:
```go
// Add new feature key to system feature registry
FeatureCloudTargets = "cloud_targets"
```

2. **Write Handler**:
```go
package handlers

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"go-hephaestus/internal/domain"
	"go-hephaestus/internal/services"
)

type CloudTargetHandler struct {
	service services.CloudTargetService
}

func NewCloudTargetHandler(service services.CloudTargetService) *CloudTargetHandler {
	return &CloudTargetHandler{service: service}
}

func (h *CloudTargetHandler) List(c *gin.Context) {
	targets, err := h.service.ListTargets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": targets})
}

func (h *CloudTargetHandler) Create(c *gin.Context) {
	var req domain.CreateCloudTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	target, err := h.service.CreateTarget(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": target})
}
```

---

### Step 6: Route Registration & Middleware Guard

Location: `internal/routes/routes.go`

```go
cloudGroup := apiV1.Group("/cloud-targets")
cloudGroup.Use(middleware.AuthRequired())
{
    // Read operations require "read" permission
    cloudGroup.GET("", middleware.RequireFeaturePermission(domain.FeatureCloudTargets, "read"), cloudHandler.List)
    
    // Write / Modify operations require "manage" permission
    cloudGroup.POST("", middleware.RequireFeaturePermission(domain.FeatureCloudTargets, "manage"), cloudHandler.Create)
}
```

---

### Step 7: Frontend View & Navigation (`web/src/`)

1. **Create Vue Component** (`web/src/views/CloudTargetsView.vue`):
   - Use `<script setup lang="ts">`.
   - Adhere strictly to the UI/UX design tokens (`bg-[#090d16]`, `border-slate-800`, Lucide icons).
   - Verify user permissions before rendering write buttons:
     ```ts
     const canManage = computed(() => authStore.hasPermission('cloud_targets', 'manage'));
     ```
2. **Register Route** (`web/src/router/index.ts`):
   ```ts
   {
     path: '/cloud-targets',
     name: 'cloud-targets',
     component: () => import('../views/CloudTargetsView.vue'),
     meta: { requiresAuth: true, feature: 'cloud_targets' }
   }
   ```
3. **Add Sidebar Navigation Item** (`web/src/components/Sidebar.vue` or `App.vue`).

---

## 4. Clean Code & Engineering Standards

### 4.1 Go Backend Standards

1. **Error Handling & Context**:
   - Always propagate `context.Context` to all database, network, and SSH calls.
   - Wrap errors with informative context:
     ```go
     // GOOD:
     if err != nil {
         return fmt.Errorf("failed to ping remote host %s:%d: %w", host.IP, host.Port, err)
     }
     
     // BAD:
     if err != nil {
         return err
     }
     ```
2. **Safe Concurrency & Goroutines**:
   - Never launch unbounded goroutines (`go func() { ... }()`) inside HTTP handlers without lifecycle tracking.
   - Use worker pools, bounded worker channels, or `sync.WaitGroup` with timeout contexts.
   - Protect concurrent memory access with `sync.RWMutex`.
3. **Secret Masking**:
   - Never print raw passwords, secret keys, or private SSH keys to logs:
     ```go
     logger.Info().Str("host", h.Name).Str("user", h.Username).Msg("Initiating SSH connection")
     ```

### 4.2 Frontend Vue 3 / TypeScript Standards

1. **Composition API with `<script setup lang="ts">`**:
   - Always declare strict interfaces for reactive state and props.
   - Do not use `any` unless parsing dynamic raw telemetry JSON.
2. **Lifecycle Cleanup**:
   - Always clear intervals, timeouts, and WebSocket connections in `onUnmounted`:
     ```ts
     onUnmounted(() => {
       if (timer) clearInterval(timer);
       if (ws) ws.close();
     });
     ```
3. **Tailwind Class Order**:
   - Layout / Positioning (`flex items-center justify-between absolute top-0`)
   - Sizing / Spacing (`w-full px-4 py-2 gap-2`)
   - Background / Border (`bg-[#171a23] border border-slate-800 rounded-xl`)
   - Typography (`text-xs font-semibold text-white`)
   - Interactive / Transitions (`hover:bg-slate-800 transition-all duration-300`)

---

## 5. Developer Checklist Before Pull Request

- [ ] Database migration is non-destructive (`IF NOT EXISTS` guards verified).
- [ ] Sensitive fields are encrypted using `crypto.EncryptAES256`.
- [ ] Endpoint is protected with authentication and RBAC permissions.
- [ ] No raw emoticons / emojis in template strings or buttons.
- [ ] Icons use `lucide-vue-next` with monochrome slate styling.
- [ ] Status indicators use semantic colors (`emerald` = OK, `amber` = warning, `rose` = error).
- [ ] All goroutines and timers have proper cancellation and cleanup.
- [ ] Code builds without compiler or linter errors (`go vet ./...`, `npm run build`).
