<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Feedback Toast Notification (Auto-dismiss 3000ms) -->
    <transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="transform -translate-y-2 opacity-0"
      enter-to-class="transform translate-y-0 opacity-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="transform translate-y-0 opacity-100"
      leave-to-class="transform -translate-y-2 opacity-0"
    >
      <div
        v-if="toast.show"
        class="fixed top-5 right-5 z-50 flex items-center gap-3 px-4 py-3 rounded-xl border shadow-xl text-xs font-semibold backdrop-blur-md"
        :class="[
          toast.type === 'error'
            ? 'bg-rose-50/95 dark:bg-rose-950/90 text-rose-700 dark:text-rose-300 border-rose-200 dark:border-rose-900/60'
            : toast.type === 'warning'
            ? 'bg-amber-50/95 dark:bg-amber-950/90 text-amber-700 dark:text-amber-300 border-amber-200 dark:border-amber-900/60'
            : 'bg-emerald-50/95 dark:bg-emerald-950/90 text-emerald-700 dark:text-emerald-300 border-emerald-200 dark:border-emerald-900/60'
        ]"
      >
        <span>{{ toast.message }}</span>
      </div>
    </transition>

    <!-- Standard HCP Header (No icon in h1, clean text only) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Operational Events & Incidents
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Centralized event center tracking server instances, containers, cluster health, and backups.
        </p>
      </div>

      <!-- Header Action Toolbar -->
      <div class="flex items-center gap-2 shrink-0">
        <!-- Auto Refresh Toggle -->
        <button
          @click="toggleAutoRefresh"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-medium transition cursor-pointer"
          :class="[
            autoRefresh
              ? 'bg-slate-100 dark:bg-[#1a2337] border-slate-300 dark:border-[#26334d] text-slate-700 dark:text-slate-200'
              : 'bg-white dark:bg-[#111726] border-slate-200 dark:border-[#1f283d] text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-white'
          ]"
          :title="autoRefresh ? 'Auto refresh enabled (every 15s)' : 'Auto refresh paused'"
        >
          <span>Auto: {{ autoRefresh ? '15s' : 'Off' }}</span>
        </button>

        <!-- Refresh Button -->
        <button
          @click="fetchData"
          :disabled="loading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-white dark:bg-[#111726] hover:bg-slate-50 dark:hover:bg-[#182030] text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium transition cursor-pointer disabled:opacity-50"
        >
          <RotateCw class="w-3.5 h-3.5 text-slate-400" :class="{ 'animate-spin': loading }" />
          <span>Refresh</span>
        </button>

        <!-- Clear Resolved Button (Opens Delete Confirmation Modal) -->
        <button
          v-if="canManage"
          @click="showClearModal = true"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-white dark:bg-[#111726] hover:bg-rose-50 dark:hover:bg-rose-950/20 text-slate-600 dark:text-slate-300 hover:text-rose-600 dark:hover:text-rose-400 border border-slate-200 dark:border-[#1f283d] hover:border-rose-300 dark:hover:border-rose-900/50 rounded-lg text-xs font-medium transition cursor-pointer"
        >
          <Trash2 class="w-3.5 h-3.5 text-slate-400" />
          <span>Clear Resolved</span>
        </button>
      </div>
    </div>

    <!-- 4 High-Level Summary KPI Cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <!-- 1. Critical Issues -->
      <div
        @click="filterBySeverity('critical')"
        class="bg-white dark:bg-[#111726] border rounded-xl p-4 shadow-2xs space-y-1 transition cursor-pointer hover:border-slate-300 dark:hover:border-slate-600"
        :class="[
          selectedSeverity === 'critical'
            ? 'border-rose-500/70 ring-1 ring-rose-500/30'
            : 'border-slate-200 dark:border-[#1f293d]'
        ]"
      >
        <div class="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
          <span>Active Critical</span>
          <span v-if="(summary.statusCounts?.acknowledgedCritical || 0) > 0 || (summary.statusCounts?.resolvedCritical || 0) > 0" class="text-[10px] text-slate-400 font-mono">
            {{ (summary.statusCounts?.activeCritical || summary.criticalCount) + (summary.statusCounts?.acknowledgedCritical || 0) + (summary.statusCounts?.resolvedCritical || 0) }} total
          </span>
        </div>
        <div class="text-2xl font-bold" :class="summary.criticalCount > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-slate-800 dark:text-slate-200'">
          {{ summary.criticalCount }}
        </div>
        <div class="text-[11px] text-slate-400">
          <span v-if="summary.statusCounts?.acknowledgedCritical" class="text-amber-600 dark:text-amber-400 font-medium">
            {{ summary.statusCounts.acknowledgedCritical }} acknowledged, 
          </span>
          <span>{{ summary.instances?.critical || 0 }} servers, {{ summary.containers?.critical || 0 }} containers</span>
        </div>
      </div>

      <!-- 2. Warning Thresholds -->
      <div
        @click="filterBySeverity('warning')"
        class="bg-white dark:bg-[#111726] border rounded-xl p-4 shadow-2xs space-y-1 transition cursor-pointer hover:border-slate-300 dark:hover:border-slate-600"
        :class="[
          selectedSeverity === 'warning'
            ? 'border-amber-500/70 ring-1 ring-amber-500/30'
            : 'border-slate-200 dark:border-[#1f293d]'
        ]"
      >
        <div class="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
          <span>Active Warnings</span>
          <span v-if="(summary.statusCounts?.acknowledgedWarning || 0) > 0" class="text-[10px] text-slate-400 font-mono">
            {{ (summary.statusCounts?.activeWarning || summary.warningCount) + (summary.statusCounts?.acknowledgedWarning || 0) }} total
          </span>
        </div>
        <div class="text-2xl font-bold" :class="summary.warningCount > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-slate-800 dark:text-slate-200'">
          {{ summary.warningCount }}
        </div>
        <div class="text-[11px] text-slate-400">
          <span v-if="summary.statusCounts?.acknowledgedWarning" class="text-amber-600 dark:text-amber-400 font-medium">
            {{ summary.statusCounts.acknowledgedWarning }} acknowledged, 
          </span>
          <span>{{ summary.instances?.warning || 0 }} servers, {{ summary.containers?.warning || 0 }} containers</span>
        </div>
      </div>

      <!-- 3. OpenSearch Cluster Health -->
      <div
        @click="filterBySource('opensearch')"
        class="bg-white dark:bg-[#111726] border rounded-xl p-4 shadow-2xs space-y-1 transition cursor-pointer hover:border-slate-300 dark:hover:border-slate-600"
        :class="[
          selectedSource === 'opensearch'
            ? 'border-blue-500/70 ring-1 ring-blue-500/30'
            : 'border-slate-200 dark:border-[#1f293d]'
        ]"
      >
        <div class="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
          <span>OpenSearch Cluster</span>
          <span class="text-[10px] font-mono text-slate-400">{{ summary.openSearch?.clusterName || 'Default' }}</span>
        </div>
        <div class="flex items-center gap-2 pt-0.5">
          <span
            class="px-2 py-0.5 rounded text-xs font-bold uppercase tracking-wider border"
            :class="[
              summary.openSearch?.status === 'green'
                ? 'bg-emerald-50 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800/40'
                : summary.openSearch?.status === 'yellow'
                ? 'bg-amber-50 dark:bg-amber-950/60 text-amber-600 dark:text-amber-400 border-amber-200 dark:border-amber-800/40'
                : summary.openSearch?.status === 'red'
                ? 'bg-rose-50 dark:bg-rose-950/60 text-rose-600 dark:text-rose-400 border-rose-200 dark:border-rose-800/40'
                : 'bg-slate-100 dark:bg-[#1a2336] text-slate-500 dark:text-slate-400 border-slate-200 dark:border-[#222c42]'
            ]"
          >
            {{ summary.openSearch?.status || 'N/A' }}
          </span>
          <span class="text-xs text-slate-500 dark:text-slate-400 font-mono">
            {{ summary.openSearch?.activeNodes || 0 }} Nodes
          </span>
        </div>
        <div class="text-[11px] text-slate-400">
          {{ summary.openSearch?.unassignedShards || 0 }} unassigned shards
        </div>
      </div>

      <!-- 4. Backups (Last 24 Hours) -->
      <div
        @click="filterBySource('backup')"
        class="bg-white dark:bg-[#111726] border rounded-xl p-4 shadow-2xs space-y-1 transition cursor-pointer hover:border-slate-300 dark:hover:border-slate-600"
        :class="[
          selectedSource === 'backup'
            ? 'border-blue-500/70 ring-1 ring-blue-500/30'
            : 'border-slate-200 dark:border-[#1f293d]'
        ]"
      >
        <div class="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
          <span>Backups (24h)</span>
        </div>
        <div class="flex items-baseline gap-2">
          <div class="text-2xl font-bold text-slate-800 dark:text-slate-200">
            {{ summary.backups?.success24h || 0 }}
          </div>
          <span class="text-xs text-slate-400">successful</span>
          <span v-if="(summary.backups?.failed24h || 0) > 0" class="text-xs font-bold text-rose-600 dark:text-rose-400 ml-auto">
            {{ summary.backups?.failed24h }} failed
          </span>
        </div>
        <div class="text-[11px] text-slate-400">
          {{ summary.backups?.total24h || 0 }} total runs, {{ summary.backups?.running || 0 }} running
        </div>
      </div>
    </div>

    <!-- Filter & Search Toolbar -->
    <div class="bg-white dark:bg-[#111726] border border-slate-200 dark:border-[#1f293d] rounded-xl p-4 shadow-2xs space-y-3">
      <!-- Row 1: Source Category Tabs -->
      <div class="flex flex-wrap items-center gap-2 border-b border-slate-100 dark:border-[#1b2234] pb-3">
        <button
          v-for="tab in sourceTabs"
          :key="tab.value"
          @click="selectedSource = tab.value; page = 1; fetchEvents();"
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition cursor-pointer"
          :class="[
            selectedSource === tab.value
              ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-900'
              : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#182030]'
          ]"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Row 2: Severity, Status, and Keyword Search -->
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-2">
          <!-- Status Filter (Active / Acknowledged / Resolved / All) -->
          <div class="flex items-center bg-slate-100 dark:bg-[#161c2d] p-0.5 rounded-lg border border-slate-200 dark:border-[#222c42]">
            <button
              v-for="st in statusOptions"
              :key="st.value"
              @click="selectedStatus = st.value; page = 1; fetchEvents();"
              class="px-2.5 py-1 rounded-md text-[11px] font-medium transition cursor-pointer flex items-center gap-1.5"
              :class="[
                selectedStatus === st.value
                  ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-2xs font-semibold'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'
              ]"
            >
              <span>{{ st.label }}</span>
              <span
                class="px-1.5 py-0.2 rounded-full text-[9px] font-mono"
                :class="[
                  selectedStatus === st.value
                    ? 'bg-slate-100 dark:bg-[#121826] text-slate-800 dark:text-slate-200 font-bold'
                    : 'bg-slate-200/70 dark:bg-[#20293d] text-slate-600 dark:text-slate-400'
                ]"
              >
                {{ getStatusCount(st.value) }}
              </span>
            </button>
          </div>

          <!-- Severity Filter -->
          <select
            v-model="selectedSeverity"
            @change="page = 1; fetchEvents();"
            class="px-2.5 py-1 bg-white dark:bg-[#161c2d] border border-slate-200 dark:border-[#222c42] rounded-lg text-xs text-slate-700 dark:text-slate-200 outline-hidden"
          >
            <option value="all">All Severities</option>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
            <option value="info">Info</option>
          </select>
        </div>

        <!-- Search Bar -->
        <div class="relative w-full sm:w-64">
          <Search class="w-3.5 h-3.5 text-slate-400 absolute left-3 top-2.5" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search event source, title..."
            class="w-full pl-8 pr-3 py-1.5 bg-slate-50 dark:bg-[#161c2d] border border-slate-200 dark:border-[#222c42] rounded-lg text-xs text-slate-800 dark:text-slate-200 placeholder:text-slate-400 focus:outline-hidden focus:border-slate-400"
          />
        </div>
      </div>
    </div>

    <!-- Events Feed Table -->
    <div class="bg-white dark:bg-[#111726] border border-slate-200 dark:border-[#1f293d] rounded-xl overflow-hidden shadow-2xs">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-slate-50 dark:bg-[#0e1422] text-[11px] font-semibold text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-[#1b2234]">
            <tr>
              <th class="py-3 px-4">Severity</th>
              <th class="py-3 px-4">Event & Title</th>
              <th class="py-3 px-4">Source</th>
              <th class="py-3 px-4">Category</th>
              <th class="py-3 px-4">Started</th>
              <th class="py-3 px-4">Status</th>
              <th class="py-3 px-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-[#182133]">
            <tr
              v-for="e in filteredEvents"
              :key="e.id"
              class="hover:bg-slate-50/60 dark:hover:bg-[#161c2d]/40 transition"
              :class="[
                e.status === 'active' && e.severity === 'critical' ? 'bg-rose-50/10 dark:bg-rose-950/5' : '',
                e.status === 'active' && e.severity === 'warning' ? 'bg-amber-50/10 dark:bg-amber-950/5' : ''
              ]"
            >
              <!-- Severity Badge -->
              <td class="py-3 px-4 whitespace-nowrap">
                <span
                  class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider border shrink-0"
                  :class="[
                    e.severity === 'critical'
                      ? 'bg-rose-100 dark:bg-rose-950/80 text-rose-700 dark:text-rose-300 border-rose-300/50 dark:border-rose-800/50'
                      : e.severity === 'warning'
                      ? 'bg-amber-100 dark:bg-amber-950/80 text-amber-700 dark:text-amber-300 border-amber-300/50 dark:border-amber-800/50'
                      : 'bg-slate-100 dark:bg-[#1a2336] text-slate-600 dark:text-slate-400 border-slate-200 dark:border-[#222c42]'
                  ]"
                >
                  {{ e.severity }}
                </span>
              </td>

              <!-- Title & Message -->
              <td class="py-3 px-4">
                <div class="font-bold text-slate-900 dark:text-white">
                  {{ e.title }}
                </div>
                <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5 line-clamp-2 max-w-xl">
                  {{ formatMessage(e.message) }}
                </div>
              </td>

              <!-- Source -->
              <td class="py-3 px-4 whitespace-nowrap">
                <div class="font-medium text-slate-800 dark:text-slate-200">
                  {{ e.sourceName || e.sourceId }}
                </div>
                <div class="text-[10px] font-mono text-slate-400">
                  {{ e.sourceId }}
                </div>
              </td>

              <!-- Category -->
              <td class="py-3 px-4 whitespace-nowrap">
                <span class="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-100 dark:bg-[#192236] text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-[#222c42]">
                  {{ formatCategory(e.sourceType) }}
                </span>
              </td>

              <!-- Started At -->
              <td class="py-3 px-4 whitespace-nowrap">
                <div class="font-mono text-slate-700 dark:text-slate-300">
                  {{ formatTimestamp(e.startedAt) }}
                </div>
                <div class="text-[10px] text-slate-400">
                  {{ formatTimeAgo(e.startedAt) }}
                </div>
              </td>

              <!-- Status -->
              <td class="py-3 px-4 whitespace-nowrap">
                <span
                  class="px-2 py-0.5 rounded-full text-[10px] font-medium inline-flex items-center"
                  :class="[
                    e.status === 'active'
                      ? 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                      : e.status === 'acknowledged'
                      ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400'
                      : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                  ]"
                >
                  <span class="capitalize">{{ e.status }}</span>
                </span>
              </td>

              <!-- Actions -->
              <td class="py-3 px-4 text-right whitespace-nowrap">
                <div class="flex items-center justify-end gap-1.5">
                  <!-- Acknowledge -->
                  <button
                    v-if="canManage && e.status === 'active'"
                    @click="ackEvent(e.id)"
                    class="px-2 py-1 bg-slate-100 dark:bg-[#192236] hover:bg-slate-200 dark:hover:bg-[#222c42] text-slate-700 dark:text-slate-300 rounded text-[11px] font-medium transition cursor-pointer"
                    title="Acknowledge issue"
                  >
                    Ack
                  </button>

                  <!-- Resolve -->
                  <button
                    v-if="canManage && e.status !== 'resolved'"
                    @click="resolveEvent(e.id)"
                    class="px-2 py-1 bg-emerald-50 dark:bg-emerald-950/40 hover:bg-emerald-100 dark:hover:bg-emerald-950/80 text-emerald-700 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/40 rounded text-[11px] font-medium transition cursor-pointer"
                    title="Mark resolved"
                  >
                    Resolve
                  </button>

                  <!-- Direct Navigation Link to Module -->
                  <button
                    @click="navigateToSource(e)"
                    class="p-1 hover:bg-slate-100 dark:hover:bg-[#1a2336] text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 rounded transition cursor-pointer"
                    title="View related resource"
                  >
                    <ExternalLink class="w-3.5 h-3.5" />
                  </button>
                </div>
              </td>
            </tr>

            <!-- Empty State -->
            <tr v-if="!loading && filteredEvents.length === 0">
              <td colspan="7" class="py-10 text-center">
                <div class="text-sm font-semibold text-slate-700 dark:text-slate-300">
                  No events found
                </div>
                <p class="text-xs text-slate-400 mt-1">
                  All systems and monitored services are currently healthy.
                </p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination Footer -->
      <div v-if="totalEvents > 0" class="flex flex-col sm:flex-row items-center justify-between gap-3 px-4 py-3 border-t border-slate-100 dark:border-[#1b2234] text-xs text-slate-500 dark:text-slate-400">
        <!-- Left: Showing entries & Page Size selector -->
        <div class="flex items-center gap-3">
          <div>
            Showing <strong class="text-slate-800 dark:text-slate-200">{{ (page - 1) * pageSize + 1 }}</strong> to
            <strong class="text-slate-800 dark:text-slate-200">{{ Math.min(page * pageSize, totalEvents) }}</strong> of
            <strong class="text-slate-800 dark:text-slate-200">{{ totalEvents }}</strong> events
          </div>
          <div class="flex items-center gap-1.5 pl-3 border-l border-slate-200 dark:border-[#1f283d]">
            <span class="text-[11px] text-slate-400">Per page:</span>
            <select
              v-model="pageSize"
              @change="page = 1; fetchEvents();"
              class="px-2 py-0.5 bg-slate-50 dark:bg-[#161c2d] border border-slate-200 dark:border-[#222c42] rounded text-xs text-slate-700 dark:text-slate-300 outline-hidden cursor-pointer"
            >
              <option v-for="sz in pageSizeOptions" :key="sz" :value="sz">{{ sz }}</option>
            </select>
          </div>
        </div>

        <!-- Right: Prev, Numbered Pages, Next -->
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            @click="goToPage(page - 1)"
            :disabled="page <= 1"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
          >
            <ChevronLeft class="w-3.5 h-3.5 text-slate-400" />
            <span>Prev</span>
          </button>

          <div class="flex items-center gap-1">
            <template v-for="(p, idx) in getVisiblePages(page, totalPages)" :key="idx">
              <span v-if="p < 0" class="px-1 text-slate-400 select-none">...</span>
              <button
                v-else
                type="button"
                @click="goToPage(p)"
                :class="[
                  page === p
                    ? 'bg-blue-600 text-white font-bold border-blue-600 shadow-xs'
                    : 'bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-[#1f283d] hover:bg-slate-50 dark:hover:bg-[#1f283d]',
                  'min-w-7 h-7 px-2 rounded-lg text-xs flex items-center justify-center transition cursor-pointer font-medium'
                ]"
              >
                {{ p }}
              </button>
            </template>
          </div>

          <button
            type="button"
            @click="goToPage(page + 1)"
            :disabled="page >= totalPages"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
          >
            <span>Next</span>
            <ChevronRight class="w-3.5 h-3.5 text-slate-400" />
          </button>
        </div>
      </div>
    </div>

    <!-- Standard HCP Delete Confirmation Modal (Rule from AGENTS.md) -->
    <div
      v-if="showClearModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Red Circle Icon in Top Center -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Clear Resolved Events?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove all resolved event records older than 1 day? This action cannot be undone.
          </p>
        </div>

        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showClearModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeClearResolved"
            :disabled="clearing"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ clearing ? 'Clearing...' : 'Confirm Clear' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import {
  RotateCw,
  Search,
  Trash2,
  ExternalLink,
  ChevronLeft,
  ChevronRight
} from 'lucide-vue-next';

interface SystemEvent {
  id: string;
  sourceType: string;
  sourceId: string;
  sourceName: string;
  eventType: string;
  severity: 'critical' | 'warning' | 'info';
  title: string;
  message: string;
  metadata?: Record<string, any>;
  status: 'active' | 'acknowledged' | 'resolved';
  startedAt: string;
  resolvedAt?: string;
}

interface EventStatusCounts {
  total: number;
  active: number;
  acknowledged: number;
  resolved: number;
  activeCritical: number;
  acknowledgedCritical: number;
  resolvedCritical: number;
  activeWarning: number;
  acknowledgedWarning: number;
  resolvedWarning: number;
}

interface EventSummary {
  totalActive?: number;
  criticalCount: number;
  warningCount: number;
  infoCount: number;
  statusCounts?: EventStatusCounts;
  instances: { total: number; critical: number; warning: number; healthy: number; offline: number };
  containers: { total: number; critical: number; warning: number; healthy: number; stopped: number };
  openSearch: { status: string; clusterName: string; activeNodes: number; unassignedShards: number };
  backups: { total24h: number; failed24h: number; success24h: number; running: number };
}

const router = useRouter();
const authStore = useAuthStore();
const canManage = computed(() => authStore.can('monitoring_instances', 'manage') || authStore.can('events', 'manage'));

// State
const loading = ref(false);
const clearing = ref(false);
const showClearModal = ref(false);
const autoRefresh = ref(true);
let autoRefreshTimer: any = null;

const summary = ref<EventSummary>({
  criticalCount: 0,
  warningCount: 0,
  infoCount: 0,
  instances: { total: 0, critical: 0, warning: 0, healthy: 0, offline: 0 },
  containers: { total: 0, critical: 0, warning: 0, healthy: 0, stopped: 0 },
  openSearch: { status: 'green', clusterName: '', activeNodes: 0, unassignedShards: 0 },
  backups: { total24h: 0, failed24h: 0, success24h: 0, running: 0 },
});

const events = ref<SystemEvent[]>([]);
const totalEvents = ref(0);
const page = ref(1);
const pageSize = ref(10);
const pageSizeOptions = [5, 10, 25, 50];

// Filters
const selectedSource = ref('all');
const selectedStatus = ref('active');
const selectedSeverity = ref('all');
const searchQuery = ref('');

const sourceTabs = [
  { label: 'All Sources', value: 'all' },
  { label: 'Server Instances', value: 'instance' },
  { label: 'Docker Containers', value: 'container' },
  { label: 'OpenSearch', value: 'opensearch' },
  { label: 'Backups', value: 'backup' },
];

const statusOptions = [
  { label: 'Active', value: 'active' },
  { label: 'Acknowledged', value: 'acknowledged' },
  { label: 'Resolved', value: 'resolved' },
  { label: 'All', value: 'all' },
];

const totalPages = computed(() => Math.max(1, Math.ceil(totalEvents.value / pageSize.value)));

const getVisiblePages = (current: number, total: number): number[] => {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }
  if (current <= 4) {
    return [1, 2, 3, 4, 5, -1, total];
  }
  if (current >= total - 3) {
    return [1, -1, total - 4, total - 3, total - 2, total - 1, total];
  }
  return [1, -1, current - 1, current, current + 1, -1, total];
};

const goToPage = (p: number) => {
  if (p < 1 || p > totalPages.value || p === page.value) return;
  page.value = p;
  fetchEvents();
};

const getStatusCount = (val: string): number => {
  if (!summary.value.statusCounts) {
    if (val === 'all') return totalEvents.value;
    if (val === 'active') return summary.value.criticalCount + summary.value.warningCount;
    return 0;
  }
  switch (val) {
    case 'active':
      return summary.value.statusCounts.active;
    case 'acknowledged':
      return summary.value.statusCounts.acknowledged;
    case 'resolved':
      return summary.value.statusCounts.resolved;
    case 'all':
      return summary.value.statusCounts.total;
    default:
      return 0;
  }
};

const filterBySeverity = (sev: 'critical' | 'warning') => {
  if (selectedSeverity.value === sev && selectedStatus.value === 'active') {
    selectedSeverity.value = 'all';
  } else {
    selectedSeverity.value = sev;
    selectedStatus.value = 'active';
  }
  page.value = 1;
  fetchEvents();
};

const filterBySource = (src: string) => {
  selectedSource.value = selectedSource.value === src ? 'all' : src;
  page.value = 1;
  fetchEvents();
};

const formatMessage = (msg: string) => {
  if (!msg) return '';
  return msg.replace(/%!s\(\*string=0x[0-9a-fA-F]+\)/g, 'operation encountered an error');
};

// Feedback Toast
const toast = ref<{ show: boolean; message: string; type: 'success' | 'warning' | 'error' }>({
  show: false,
  message: '',
  type: 'success',
});
let toastTimer: any = null;
const showNotice = (message: string, type: 'success' | 'warning' | 'error' = 'success') => {
  toast.value = { show: true, message, type };
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toast.value.show = false;
  }, 3000);
};

// Filtered Events
const filteredEvents = computed(() => {
  let list = events.value;
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase().trim();
    list = list.filter(
      e =>
        e.title.toLowerCase().includes(q) ||
        e.message.toLowerCase().includes(q) ||
        (e.sourceName && e.sourceName.toLowerCase().includes(q)) ||
        e.sourceId.toLowerCase().includes(q)
    );
  }
  return list;
});

// Fetch Data
const fetchData = async () => {
  loading.value = true;
  try {
    const [resSummary, resEvents] = await Promise.all([
      axios.get('/api/v1/events/summary'),
      axios.get('/api/v1/events', {
        params: {
          status: selectedStatus.value,
          severity: selectedSeverity.value,
          sourceType: selectedSource.value,
          limit: pageSize.value,
          offset: (page.value - 1) * pageSize.value,
        },
      }),
    ]);

    if (resSummary.data && resSummary.data.success) {
      summary.value = resSummary.data.data;
    }
    if (resEvents.data && resEvents.data.success) {
      events.value = resEvents.data.data || [];
      totalEvents.value = resEvents.data.total || 0;
    }
  } catch (err: any) {
    showNotice(err.response?.data?.message || 'Failed to fetch events data', 'error');
  } finally {
    loading.value = false;
  }
};

const fetchEvents = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/events', {
      params: {
        status: selectedStatus.value,
        severity: selectedSeverity.value,
        sourceType: selectedSource.value,
        limit: pageSize.value,
        offset: (page.value - 1) * pageSize.value,
      },
    });
    if (res.data && res.data.success) {
      events.value = res.data.data || [];
      totalEvents.value = res.data.total || 0;
    }
  } catch (err: any) {
    showNotice('Failed to filter events', 'error');
  } finally {
    loading.value = false;
  }
};

// Actions
const ackEvent = async (id: string) => {
  try {
    await axios.post(`/api/v1/events/${id}/ack`);
    showNotice('Event acknowledged', 'success');
    fetchData();
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to acknowledge event', 'error');
  }
};

const resolveEvent = async (id: string) => {
  try {
    await axios.post(`/api/v1/events/${id}/resolve`);
    showNotice('Event marked as resolved', 'success');
    fetchData();
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to resolve event', 'error');
  }
};

const executeClearResolved = async () => {
  clearing.value = true;
  try {
    const res = await axios.delete('/api/v1/events/resolved?olderThanDays=1');
    showNotice(`Cleared ${res.data?.deleted || 0} resolved events`, 'success');
    showClearModal.value = false;
    fetchData();
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to clear events', 'error');
  } finally {
    clearing.value = false;
  }
};

const navigateToSource = (e: SystemEvent) => {
  switch (e.sourceType) {
    case 'instance':
      router.push({ name: 'monitoring-instance', query: { q: e.sourceName || e.sourceId } });
      break;
    case 'container':
      router.push({ name: 'container-management', query: { q: e.sourceName } });
      break;
    case 'opensearch':
      router.push({ name: 'opensearch-cluster' });
      break;
    case 'backup':
      router.push({ name: 'backup' });
      break;
    default:
      break;
  }
};

const toggleAutoRefresh = () => {
  autoRefresh.value = !autoRefresh.value;
  if (autoRefresh.value) {
    startAutoRefresh();
  } else if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
  }
};

const startAutoRefresh = () => {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer);
  autoRefreshTimer = setInterval(() => {
    fetchData();
  }, 15000);
};

// Formatting Helpers
const formatCategory = (sourceType: string) => {
  switch (sourceType) {
    case 'instance':
      return 'Server';
    case 'container':
      return 'Container';
    case 'opensearch':
      return 'OpenSearch';
    case 'backup':
      return 'Backup';
    default:
      return sourceType;
  }
};

const formatTimestamp = (ts: string) => {
  if (!ts) return 'N/A';
  const d = new Date(ts);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
};

const formatTimeAgo = (ts: string) => {
  if (!ts) return '';
  const diffSec = Math.floor((Date.now() - new Date(ts).getTime()) / 1000);
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  return `${Math.floor(diffHr / 24)}d ago`;
};

onMounted(() => {
  fetchData();
  if (autoRefresh.value) {
    startAutoRefresh();
  }
});

onUnmounted(() => {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer);
  if (toastTimer) clearTimeout(toastTimer);
});
</script>
