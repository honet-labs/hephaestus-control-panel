# Hephaestus Control Panel (HCP)
# UI/UX Design System & Clean Code Engineering Standards

> **Document Version**: 1.0.0  
> **Status**: Mandatory Standard for All Contributors & Developers  
> **Target Audience**: Frontend Engineers, Backend Engineers, Fullstack Contributors  
> **Rule**: All new features, pull requests, and UI components **MUST** strictly adhere to this specification.

---

## 1. Design Philosophy: Enterprise NOC Aesthetic

Hephaestus Control Panel (HCP) is an **enterprise-grade Network Operations Center (NOC) and DevOps infrastructure platform**. It is designed for high-density information display, lightning-fast response times, and 24/7 mission-critical operations.

### Key Principles:
1. **Serious & Professional**: HCP is **NOT** a playful consumer web app. Avoid bright pastel tones, rainbow gradients, and playful animations.
2. **High-Contrast Dark Slate Default**: Optimized for dark NOC room environments with low eye strain. Light theme is supported with crisp, high-contrast slate surfaces.
3. **Information Density**: Clean, compact padding (`px-3 py-1.5`, `text-xs`) to display maximum actionable metrics without unnecessary white space.
4. **Monochrome by Default, Color for Status**: Colors are reserved **strictly** for semantic health, alerts, and operational states.
5. **Strict English Language Standard**: All UI text, tooltips, validation messages, backend error payloads, and code comments must be in **English**.

---

## 2. Color Palette & Design Tokens

### 2.1 Dark Mode Surface Tokens (Default)

| Token / Element | Hex Value | Tailwind Class | Purpose |
|---|---|---|---|
| **Canvas Background** | `#090d16` | `bg-[#090d16]` | Base background of the entire viewport and application canvas |
| **Primary Card Surface** | `#13161f` | `bg-[#13161f]` | Standard dashboard cards, metric panels, container boxes |
| **Elevated Surface / Modals** | `#171a23` | `bg-[#171a23]` | Modal dialogs, flyout drawers, dropdown popovers |
| **Header / Table Head** | `#1c202b` | `bg-[#1c202b]` | Sticky table headers, top control bars, toolbar wrappers |
| **Input / Inset Surface** | `#0f1219` | `bg-[#0f1219]` | Text inputs, code blocks, terminal panes, nested wells |
| **Hover / Active Item** | `#20242e` | `hover:bg-slate-800` or `bg-[#20242e]` | Button hover states, active segmented pills, selected rows |
| **Structural Borders** | `#1e2842` | `border-slate-800` or `border-[#1b2234]` | Separation borders between cards, rows, and sidebar |
| **Interactive Borders** | `#334155` | `border-slate-700` | Input borders, active modals, interactive toggle borders |

### 2.2 Light Mode Surface Tokens

| Token / Element | Hex Value | Tailwind Class | Purpose |
|---|---|---|---|
| **Canvas Background** | `#f1f5f9` | `bg-slate-100` / `bg-[#f1f5f9]` | Soft light slate background |
| **Card Surface** | `#ffffff` | `bg-white` | Content cards and containers |
| **Table Head / Headers** | `#f8fafc` | `bg-slate-50` | Table headers and toolbar background |
| **Hover State** | `#e2e8f0` | `hover:bg-slate-200` | Row hover and button hover |
| **Borders** | `#cbd5e1` | `border-slate-300` / `border-[#cbd5e1]` | Card and table borders with crisp contrast |

### 2.3 Text Contrast Hierarchy

| Text Level | Dark Mode Class | Light Mode Class | Purpose |
|---|---|---|---|
| **Primary Text** | `text-white` | `text-slate-900` | Page headings, modal titles, active selections, primary metrics |
| **Secondary Text** | `text-slate-200` / `text-slate-300` | `text-slate-700` | Body copy, table cells, form labels, descriptions |
| **Muted / Helper Text** | `text-slate-400` / `text-slate-500` | `text-slate-500` | Subtitles, input placeholders, timestamps, unit labels |
| **Code / Monospace** | `font-mono text-slate-300` | `font-mono text-slate-800` | IPs, OIDs, ports, hashes, JSON values |

---

## 3. Semantic Status Colors (Status ONLY!)

> [!IMPORTANT]
> **COLOR IS STRICTLY RESERVED FOR OPERATIONAL STATUS.**  
> Do not use arbitrary colors (such as purple, pink, orange, or sky blue) on ordinary buttons, navigation tabs, or card headers.

```
+-----------------------------------------------------------------------------------+
| Emerald (Green)  -> Online, Healthy, Reachable, Connected, Success, Active         |
| Amber (Yellow)   -> Warning, Degraded, Paused, Capped, Syncing, Stale             |
| Rose (Red)       -> Offline, Critical, Error, Failed, Disconnected, Kill Process   |
| Slate (Neutral)  -> Unknown, Pending, Disabled, Inactive, Muted                   |
| Blue (#4274D9)   -> Primary Action (Submit, Create, Add, Save, Connect) ONLY      |
+-----------------------------------------------------------------------------------+
```

### Semantic Status Implementation Blueprint

```html
<!-- Online / Healthy Status -->
<span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
  <span>Online</span>
</span>

<!-- Warning / Degraded Status -->
<span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
  <span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
  <span>Degraded</span>
</span>

<!-- Critical / Offline Status -->
<span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
  <span class="w-1.5 h-1.5 rounded-full bg-rose-500"></span>
  <span>Offline</span>
</span>
```

---

## 4. Typography & Font Family Standards

HCP standardizes on two font families loaded via system fallbacks:

### 4.1 Interface Typography (Sans-Serif)
```css
font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, system-ui, sans-serif;
```
- Used for all UI chrome: headings, labels, navigation, buttons, dialogues, and table headers.

### 4.2 Technical & Data Typography (Monospace)
```css
font-family: 'JetBrains Mono', 'Fira Code', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
```
- **Mandatory** for:
  - IP addresses (`192.168.1.1`, `10.0.0.0/24`)
  - Ports (`22`, `5432`, `9200`)
  - SNMP OIDs (`1.3.6.1.2.1.1.1.0`)
  - MAC addresses (`00:1A:2B:3C:4D:5E`)
  - Terminal output and logs
  - JSON payloads and configuration files
  - Metrics and latencies (`12.4 ms`, `3.2 GB`)

### 4.3 Font Sizing Hierarchy
- `text-[10px]`: Micro badges, uppercase category pills, compact timestamps.
- `text-[11px]`: Secondary helper text, table code cells, toolbar sub-labels.
- `text-xs` (12px): **Standard UI body size** for form inputs, table rows, button labels.
- `text-sm` (14px): Card headers, modal titles, section dividers.
- `text-base` / `text-lg`: Major view titles (e.g. "Remote Host Terminal", "OpenSearch Cluster").

---

## 5. Icon System Rules (MANDATORY)

1. **Single Icon Library**: Use **ONLY** `lucide-vue-next`. Never import FontAwesome, Material Icons, or heroicons.
2. **Neutral Monochrome Styling**:
   - Navigation, toolbar buttons, and table action icons must be **monochrome slate**:
     ```html
     <!-- Standard Action Icon -->
     <RotateCw class="w-3.5 h-3.5 text-slate-400 hover:text-white transition" />
     ```
   - **Prohibited**: Do NOT use arbitrary colored icons like `text-sky-400`, `text-purple-400`, or `text-pink-400` on general action buttons.
3. **Strict Sizing**:
   - Micro inline indicators: `w-3 h-3` or `w-3.5 h-3.5`
   - Standard button icons: `w-3.5 h-3.5` or `w-4 h-4`
   - Modal headers: `w-5 h-5`
   - Empty state placeholders: `w-8 h-8` placed inside a `w-16 h-16` container.

### 5.1 Strict Ban on Unicode Emojis & Emoticons

> [!CAUTION]
> **NEVER USE UNICODE EMOJIS IN THE UI OR TEMPLATES.**  
> Emojis render inconsistently across operating systems and break the serious enterprise NOC aesthetic.

| Prohibited Emoji | Required Lucide Replacement |
|---|---|
| `➔` / `→` | `<ArrowRight class="w-3.5 h-3.5" />` |
| `←` | `<ArrowLeft class="w-3.5 h-3.5" />` |
| `🏢` / `🖥️` | `<Server class="w-3.5 h-3.5" />` |
| `💻` | `<Monitor class="w-3.5 h-3.5" />` or `<Laptop class="w-3.5 h-3.5" />` |
| `🔀` | `<RotateCw class="w-3.5 h-3.5" />` |
| `⚡` | `<Zap class="w-3.5 h-3.5" />` |
| `⚠️` / `⚠` | `<AlertTriangle class="w-3.5 h-3.5" />` |
| `❌` / `✕` | `<X class="w-3.5 h-3.5" />` |
| `✅` / `✓` | `<Check class="w-3.5 h-3.5" />` |
| `ℹ` | `<Info class="w-3.5 h-3.5" />` |
| `⭐` | Clean text or `<Star class="w-3.5 h-3.5" />` |
| `🔒` | `<Lock class="w-3.5 h-3.5" />` |
| `🗑️` | `<Trash2 class="w-3.5 h-3.5" />` |

---

## 6. Standard Component Blueprints

### 6.1 Button Hierarchy

```html
<!-- 1. Primary Action Button (Add, Create, Submit, Connect) -->
<button class="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white font-bold text-xs rounded-lg shadow transition">
  <Plus class="w-3.5 h-3.5" />
  <span>Add Host</span>
</button>

<!-- 2. Secondary Neutral Button (Refresh, Filter, Cancel) -->
<button class="flex items-center gap-1.5 px-3 py-1.5 bg-[#090d16] hover:bg-slate-800 text-slate-300 hover:text-white border border-slate-700 text-xs font-medium rounded-lg transition">
  <RotateCw class="w-3.5 h-3.5 text-slate-400" />
  <span>Refresh</span>
</button>

<!-- 3. Danger Action Button (Delete, Kill Process, Revoke) -->
<button class="flex items-center gap-1.5 px-3 py-1.5 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/30 text-xs font-medium rounded-lg transition">
  <Trash2 class="w-3.5 h-3.5 text-rose-400" />
  <span>Delete</span>
</button>
```

### 6.2 Modal Dialog Blueprint

```html
<div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-150">
  <div class="w-full max-w-lg bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans text-white">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-slate-800 pb-3">
      <div class="flex items-center gap-2">
        <Server class="w-5 h-5 text-slate-400" />
        <h3 class="text-sm font-bold text-white">Configure Remote Host</h3>
      </div>
      <button @click="isOpen = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition">
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Form Content -->
    <div class="space-y-3 text-xs">
      <div>
        <label class="block text-slate-400 mb-1 font-semibold">Host Name</label>
        <input class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white text-xs font-mono focus:outline-none focus:border-blue-500" />
      </div>
    </div>

    <!-- Footer -->
    <div class="flex justify-end gap-2 pt-3 border-t border-slate-800">
      <button type="button" @click="isOpen = false" class="px-3 py-1.5 text-slate-400 hover:text-white text-xs transition">Cancel</button>
      <button type="submit" class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg shadow transition">Save Host</button>
    </div>
  </div>
</div>
```

### 6.3 Empty State Blueprint

```html
<div class="w-full h-64 flex flex-col items-center justify-center p-8 text-center space-y-3">
  <div class="w-16 h-16 rounded-2xl bg-[#171a23] border border-slate-800 flex items-center justify-center text-slate-500 shadow-lg">
    <Globe class="w-8 h-8 text-slate-400" />
  </div>
  <div class="space-y-1 max-w-sm">
    <h4 class="text-sm font-bold text-white">No Target Devices Found</h4>
    <p class="text-xs text-slate-400 leading-relaxed">
      Execute a network subnet sweep or sync with Prometheus scrape targets to populate this topology view.
    </p>
  </div>
</div>
```

---

## 7. Clean Code & Architectural Guidelines

### 7.1 Go Backend Principles
1. **Decoupled Layering**:
   - `handlers/` parses HTTP/WebSocket requests and returns JSON status codes. **Never** put SQL queries inside handlers.
   - `services/` contains core business logic, encryption, and concurrency dispatch.
   - `repository/` contains prepared PostgreSQL queries via `pgxpool`.
2. **Context Propagation**: Always pass `ctx context.Context` into database queries, network dials, and worker loops for cancellation.
3. **Goroutine Bounding**: Never spawn unmanaged `go func() { ... }()` in request handlers. Use the native `queue.WorkerPool`.
4. **Secret Safety**: Always mask or encrypt secrets. Never output raw passwords or private keys to loggers.

### 7.2 Vue 3 / TypeScript Principles
1. **`<script setup lang="ts">`**: Always use Vue 3 Composition API with explicit TypeScript interface models.
2. **Resource Teardown**: Always clean up WebSockets, timers, and event listeners in `onUnmounted`:
   ```ts
   onUnmounted(() => {
     if (timer) clearInterval(timer);
     if (ws) ws.close();
   });
   ```
3. **RBAC Guard**: Always wrap privileged action buttons with `authStore.can('feature', 'manage')`.

---

## 8. Language Standard: 100% English

To ensure universal maintainability for international DevOps and open-source contributors:
- **All code comments, UI labels, tooltips, validation errors, and API responses MUST be in English.**
- If you find any legacy non-English strings, replace them immediately according to this guide.

---
*HONET Labs & Hephaestus Control Panel Engineering Team*
