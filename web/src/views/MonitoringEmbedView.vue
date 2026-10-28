<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import { useRoute } from 'vue-router';
import axios from 'axios';
import {
  Server,
  Box,
  ArrowDown,
  ArrowUp,
  RefreshCw,
  Sun,
  Moon,
  Clock,
} from 'lucide-vue-next';
import { useThemeStore } from '../stores/theme';

interface DiskMetric {
  mountpoint: string;
  usagePct: number;
  usedBytes: number;
  totalBytes: number;
  freeBytes: number;
  usageHuman: string;
}

interface LiveMetrics {
  isOnline: boolean;
  detectedHostname?: string;
  agentVersion?: string;
  hasOtel?: boolean;
  cpuPct: number | null;
  cpuCount: number;
  memPct: number | null;
  memUsedBytes: number;
  memFreeBytes: number;
  memTotalBytes: number;
  diskPct: number | null;
  disks: DiskMetric[];
  netDownloadMb: number;
  netUploadMb: number;
  netTotalMb: number;
  uptimeHuman?: string;
  lastUpdated?: string;
}

interface MonitoringInstance {
  id: string;
  name: string;
  host: string;
  ipAddress: string;
  hostname?: string;
  groupName?: string;
  tags?: string[];
  connectionMethod?: 'ssh' | 'prometheus';
  liveMetrics?: LiveMetrics;
}

interface DockerContainerMetric {
  id: string;
  containerId: string;
  containerName: string;
  imageName: string;
  hostname: string;
  ipAddress: string;
  isOnline: boolean;
  cpuPct: number | null;
  memPct: number | null;
  memUsageBytes: number;
  memLimitBytes: number;
  memCacheBytes: number;
  netRxRateMb: number;
  netTxRateMb: number;
  blockReadRateMb: number;
  blockWriteRateMb: number;
  lastUpdated: string;
}

const route = useRoute();
const themeStore = useThemeStore();

// Theme state linked directly to global theme store
const isDarkMode = computed(() => themeStore.isDark);

const toggleTheme = () => {
  themeStore.toggleTheme();
};

// Query Parameters
const viewType = computed<'servers' | 'containers'>(() => {
  return (route.query.type as string) === 'containers' ? 'containers' : 'servers';
});

const filterIds = computed<string[]>(() => {
  const idsStr = (route.query.ids as string) || '';
  if (!idsStr.trim()) return [];
  return idsStr.split(',').map((s) => s.trim()).filter(Boolean);
});

const filterGroup = computed<string>(() => {
  return (route.query.group as string) || '';
});

const searchQuery = computed<string>(() => {
  return ((route.query.search as string) || '').trim().toLowerCase();
});

const customTitle = computed<string>(() => {
  return (route.query.title as string) || (viewType.value === 'servers' ? 'Infrastructure Telemetry' : 'Container Fleet Telemetry');
});

const refreshIntervalSec = computed<number>(() => {
  const r = parseInt(route.query.refresh as string, 10);
  return !isNaN(r) && r >= 3 ? r : 10;
});

// State
const instances = ref<MonitoringInstance[]>([]);
const containers = ref<DockerContainerMetric[]>([]);
const loading = ref(true);
const refreshing = ref(false);
const lastUpdatedTime = ref<string>('-');
let pollTimer: any = null;

// Sync theme with URL query or default
onMounted(() => {
  const themeParam = route.query.theme as string;
  if (themeParam === 'light') {
    themeStore.applyTheme('light');
  } else if (themeParam === 'dark') {
    themeStore.applyTheme('dark');
  } else {
    themeStore.initTheme();
  }
});

watch(
  () => route.query.theme,
  (newTheme) => {
    if (newTheme === 'light') {
      themeStore.applyTheme('light');
    } else if (newTheme === 'dark') {
      themeStore.applyTheme('dark');
    }
  }
);

// Data Fetching
const fetchServerData = async (silent = false) => {
  if (!silent) loading.value = true;
  else refreshing.value = true;
  try {
    const res = await axios.get('/api/v1/monitoring/instances', {
      params: { metrics: 'true' },
    });
    if (res.data?.success) {
      instances.value = res.data.data || [];
      lastUpdatedTime.value = new Date().toLocaleTimeString();
    }
  } catch (err) {
    console.error('Failed to fetch server metrics:', err);
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
};

const fetchContainerData = async (silent = false) => {
  if (!silent) loading.value = true;
  else refreshing.value = true;
  try {
    const res = await axios.get('/api/v1/monitoring/containers');
    if (res.data?.success) {
      containers.value = res.data.data || [];
      lastUpdatedTime.value = new Date().toLocaleTimeString();
    }
  } catch (err) {
    console.error('Failed to fetch container metrics:', err);
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
};

const refreshData = async (silent = false) => {
  if (viewType.value === 'servers') {
    await fetchServerData(silent);
  } else {
    await fetchContainerData(silent);
  }
};

const warningThreshold = computed(() => {
  const w = Number(route.query.warning);
  return !isNaN(w) && w > 0 ? w : 80;
});

const criticalThreshold = computed(() => {
  const c = Number(route.query.critical);
  return !isNaN(c) && c > 0 ? c : 90;
});

const getServerStatus = (inst: MonitoringInstance) => {
  if (!inst.liveMetrics?.isOnline) {
    return { severity: 'offline' as const, maxUsage: -1, tier: 3 };
  }
  const cpu = typeof inst.liveMetrics.cpuPct === 'number' ? inst.liveMetrics.cpuPct : 0;
  const memory = typeof inst.liveMetrics.memPct === 'number' ? inst.liveMetrics.memPct : 0;
  const disk = getOverallDisk(inst).pct ?? 0;
  const maxUsage = Math.max(cpu, memory, disk);

  let severity: 'critical' | 'warning' | 'normal' = 'normal';
  let tier = 2;
  if (maxUsage >= criticalThreshold.value) {
    severity = 'critical';
    tier = 0;
  } else if (maxUsage >= warningThreshold.value) {
    severity = 'warning';
    tier = 1;
  }
  return { severity, maxUsage, tier };
};

// Filtered and Sorted Lists: High usage & critical/warning always placed first
const displayedServers = computed(() => {
  let list = instances.value;

  if (filterIds.value.length > 0) {
    list = list.filter((inst) => filterIds.value.includes(inst.id));
  }

  if (filterGroup.value && filterGroup.value !== 'all') {
    list = list.filter((inst) => inst.groupName === filterGroup.value);
  }

  if (searchQuery.value) {
    const q = searchQuery.value;
    list = list.filter(
      (inst) =>
        inst.name.toLowerCase().includes(q) ||
        inst.host.toLowerCase().includes(q) ||
        (inst.ipAddress && inst.ipAddress.toLowerCase().includes(q)) ||
        (inst.groupName && inst.groupName.toLowerCase().includes(q))
    );
  }

  return [...list].sort((a, b) => {
    const statusA = getServerStatus(a);
    const statusB = getServerStatus(b);

    if (statusA.tier !== statusB.tier) {
      return statusA.tier - statusB.tier;
    }
    if (Math.abs(statusB.maxUsage - statusA.maxUsage) > 0.05) {
      return statusB.maxUsage - statusA.maxUsage;
    }
    return a.name.localeCompare(b.name);
  });
});

const displayedContainers = computed(() => {
  let list = containers.value;

  if (filterIds.value.length > 0) {
    list = list.filter((c) => filterIds.value.includes(c.id) || filterIds.value.includes(c.containerId));
  }

  if (filterGroup.value && filterGroup.value !== 'all') {
    list = list.filter((c) => c.hostname === filterGroup.value);
  }

  if (searchQuery.value) {
    const q = searchQuery.value;
    list = list.filter(
      (c) =>
        c.containerName.toLowerCase().includes(q) ||
        c.imageName.toLowerCase().includes(q) ||
        c.hostname.toLowerCase().includes(q) ||
        c.ipAddress.toLowerCase().includes(q)
    );
  }

  return [...list].sort((a, b) => {
    const usageA = Math.max(a.cpuPct ?? 0, a.memPct ?? 0);
    const usageB = Math.max(b.cpuPct ?? 0, b.memPct ?? 0);
    const tierA = usageA >= criticalThreshold.value ? 0 : usageA >= warningThreshold.value ? 1 : 2;
    const tierB = usageB >= criticalThreshold.value ? 0 : usageB >= warningThreshold.value ? 1 : 2;

    if (tierA !== tierB) {
      return tierA - tierB;
    }
    if (Math.abs(usageB - usageA) > 0.05) {
      return usageB - usageA;
    }
    return a.containerName.localeCompare(b.containerName);
  });
});

// Helper to identify virtual, loop, snap, boot, and non-storage mounts
const isIgnoredMount = (mount?: string, device?: string, fsType?: string): boolean => {
  if (!mount) return true;
  const m = mount.toLowerCase().trim();
  const d = (device || '').toLowerCase().trim();
  const t = (fsType || '').toLowerCase().trim();

  // Snaps, loop devices, and ephemeral filesystems
  if (m.startsWith('/snap') || m.startsWith('/var/lib/snapd')) return true;
  if (d.startsWith('/dev/loop') || d.startsWith('loop')) return true;
  if (t === 'squashfs' || t === 'tmpfs' || t === 'devtmpfs' || t === 'overlay' || t === 'iso9660') return true;
  if (d === 'tmpfs' || d === 'devtmpfs' || d === 'udev' || d === 'none' || d === 'shm' || d === 'overlay') return true;
  if (m === '/boot' || m.startsWith('/boot/')) return true;
  if (m.startsWith('/dev') || m.startsWith('/run') || m.startsWith('/sys') || m.startsWith('/proc')) return true;
  if (m.includes('/docker/overlay2') || m.includes('/docker/containers') || m.includes('/var/lib/docker')) return true;

  return false;
};

// Overall Disk Helper
const getOverallDisk = (inst: MonitoringInstance) => {
  const disks = (inst.liveMetrics?.disks || []).filter(
    (d) => !isIgnoredMount(d.mountpoint, d.device, d.fsType)
  );
  if (disks.length === 0) {
    return {
      pct: inst.liveMetrics?.diskPct != null ? Math.round(inst.liveMetrics.diskPct * 10) / 10 : null,
      usedBytes: 0,
      totalBytes: 0,
    };
  }
  let total = 0;
  let used = 0;
  disks.forEach((d) => {
    total += d.totalBytes || 0;
    used += d.usedBytes || 0;
  });
  const pct = total > 0 ? Math.round((used / total) * 1000) / 10 : 0;
  return { pct, usedBytes: used, totalBytes: total };
};

// Progress Bar Color Helper
const getBarColor = (val: number | null | undefined) => {
  if (val === null || val === undefined) return 'bg-slate-300 dark:bg-slate-700';
  if (val >= 85) return 'bg-rose-500';
  if (val >= 70) return 'bg-amber-500';
  return 'bg-emerald-500';
};

const formatBytesHuman = (bytes: number | null | undefined): string => {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let val = bytes;
  let idx = 0;
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024;
    idx++;
  }
  return `${val.toFixed(1)} ${units[idx]}`;
};

const formatRateMB = (mbVal: number | null | undefined): string => {
  if (mbVal === null || mbVal === undefined || mbVal <= 0 || isNaN(mbVal)) return '0 B/s';
  const bytesSec = mbVal * 1024 * 1024;
  if (bytesSec < 1024) return `${bytesSec.toFixed(0)} B/s`;
  if (bytesSec < 1024 * 1024) return `${(bytesSec / 1024).toFixed(1)} KB/s`;
  if (bytesSec < 1024 * 1024 * 1024) return `${(bytesSec / (1024 * 1024)).toFixed(2)} MB/s`;
  return `${(bytesSec / (1024 * 1024 * 1024)).toFixed(2)} GB/s`;
};

const isSshInstance = (inst: MonitoringInstance): boolean => {
  if (inst.connectionMethod === 'ssh' || (inst as any).metricSource === 'ssh') return true;
  if (inst.connectionMethod === 'prometheus' || (inst as any).metricSource === 'prometheus') return false;
  return inst.liveMetrics?.agentVersion === 'SSH (Agentless)' || inst.liveMetrics?.agentVersion === 'SSH (Fallback)';
};

// Lifecycle
onMounted(async () => {
  await refreshData(false);
  pollTimer = setInterval(() => {
    refreshData(true);
  }, refreshIntervalSec.value * 1000);
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<template>
  <div class="min-h-screen bg-slate-50 dark:bg-[#0b0f19] text-slate-900 dark:text-slate-100 font-sans p-4 sm:p-6 transition-colors">
    <!-- Top Bar: Title, Live Status, Controls -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 mb-4 border-b border-slate-200 dark:border-[#1a2337]">
      <div class="flex items-center gap-3">
        <div>
          <h1 class="text-base sm:text-lg font-bold text-slate-900 dark:text-white tracking-tight flex items-center gap-2">
            <span>{{ customTitle }}</span>
            <span class="text-xs px-2 py-0.5 rounded-full font-mono bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">
              {{ viewType === 'servers' ? `${displayedServers.length} Nodes` : `${displayedContainers.length} Containers` }}
            </span>
          </h1>
          <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5 flex items-center gap-2">
            <span class="inline-flex items-center gap-1">
              <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
              <span>Live Auto-Refresh: {{ refreshIntervalSec }}s</span>
            </span>
            <span>•</span>
            <span class="flex items-center gap-1 font-mono">
              <Clock class="w-3 h-3 text-slate-400" />
              <span>Updated: {{ lastUpdatedTime }}</span>
            </span>
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <!-- Manual Refresh Button -->
        <button
          @click="refreshData(false)"
          :disabled="loading || refreshing"
          class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-white dark:bg-[#161c2d] hover:bg-slate-50 dark:hover:bg-[#1f283d] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-200 text-xs font-medium transition cursor-pointer disabled:opacity-50 shadow-2xs"
          title="Refresh telemetry now"
        >
          <RefreshCw class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" :class="{ 'animate-spin': loading || refreshing }" />
          <span class="hidden sm:inline">{{ refreshing ? 'Refreshing...' : 'Refresh' }}</span>
        </button>

        <!-- Theme Toggle Button -->
        <button
          @click="toggleTheme"
          class="p-1.5 rounded-lg bg-white dark:bg-[#161c2d] hover:bg-slate-50 dark:hover:bg-[#1f283d] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-200 transition cursor-pointer shadow-2xs"
          :title="isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'"
        >
          <Sun v-if="isDarkMode" class="w-3.5 h-3.5 text-amber-500" />
          <Moon v-else class="w-3.5 h-3.5 text-slate-600 dark:text-slate-300" />
        </button>
      </div>
    </div>

    <!-- Loading Skeleton -->
    <div v-if="loading && instances.length === 0 && containers.length === 0" class="py-20 text-center space-y-3">
      <RefreshCw class="w-8 h-8 animate-spin mx-auto text-slate-400" />
      <div class="text-sm font-semibold text-slate-500 dark:text-slate-400">Loading Telemetry Stream...</div>
    </div>

    <!-- ===================================================================== -->
    <!-- VIEW 1: SERVERS / INSTANCES TABLE VIEW (Exact User Screenshot Style)   -->
    <!-- ===================================================================== -->
    <div v-else-if="viewType === 'servers'" class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl shadow-xs overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-slate-50/75 dark:bg-[#0c101a] border-b border-slate-200 dark:border-[#1f283d] text-[11px] font-semibold text-slate-500 dark:text-slate-400 whitespace-nowrap">
            <tr>
              <th class="py-3 px-3.5 min-w-[170px]">System Name</th>
              <th class="py-3 px-3 min-w-[100px]">Group / Tags</th>
              <th class="py-3 px-3 min-w-[110px]">Hostname</th>
              <th class="py-3 px-3 min-w-[100px]">IP Address</th>
              <th class="py-3 px-3 min-w-[120px]">CPU</th>
              <th class="py-3 px-3 min-w-[130px]">Memory</th>
              <th class="py-3 px-3 min-w-[130px]">Disk</th>
              <th class="py-3 px-3 min-w-[105px]">RX</th>
              <th class="py-3 px-3 min-w-[105px]">TX</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-[#1a2236]">
            <tr
              v-for="inst in displayedServers"
              :key="inst.id"
              class="hover:bg-slate-50/80 dark:hover:bg-[#151c2d] transition"
              :class="[
                getServerStatus(inst).severity === 'critical' ? 'bg-rose-50/30 dark:bg-rose-950/20' : '',
                getServerStatus(inst).severity === 'warning' ? 'bg-amber-50/30 dark:bg-amber-950/20' : ''
              ]"
            >
              <!-- System Name Column -->
              <td class="py-3.5 px-3.5 whitespace-nowrap">
                <div class="flex items-center gap-2">
                  <span
                    class="w-2.5 h-2.5 rounded-full shrink-0 shadow-xs"
                    :class="inst.liveMetrics?.isOnline ? 'bg-emerald-500 shadow-emerald-500/50' : 'bg-slate-400'"
                    :title="inst.liveMetrics?.isOnline ? 'Online (Active)' : 'Offline'"
                  ></span>
                  <span class="font-bold text-slate-900 dark:text-white text-xs tracking-tight">
                    {{ inst.name }}
                  </span>

                  <!-- Connection Badge (SSH vs Prometheus) -->
                  <span
                    v-if="isSshInstance(inst)"
                    class="px-1.5 py-0.5 rounded text-[9px] font-semibold bg-sky-100 dark:bg-sky-950/80 text-sky-700 dark:text-sky-300 border border-sky-300/50 dark:border-sky-800/50 shrink-0"
                  >
                    SSH
                  </span>
                  <span
                    v-else
                    class="px-1.5 py-0.5 rounded text-[9px] font-semibold bg-emerald-100 dark:bg-emerald-950/80 text-emerald-700 dark:text-emerald-300 border border-emerald-300/50 dark:border-emerald-800/50 shrink-0"
                  >
                    Prometheus
                  </span>
                </div>
              </td>

              <!-- Group / Tags Column -->
              <td class="py-3.5 px-3 whitespace-nowrap">
                <div class="flex flex-wrap items-center gap-1.5">
                  <span
                    v-if="inst.groupName"
                    class="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-100 dark:bg-[#192236] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-[#222c42]"
                  >
                    {{ inst.groupName }}
                  </span>
                  <span
                    v-for="t in (inst.tags || [])"
                    :key="t"
                    class="px-1.5 py-0.5 rounded text-[9px] font-normal bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20"
                  >
                    {{ t }}
                  </span>
                  <span v-if="!inst.groupName && (!inst.tags || inst.tags.length === 0)" class="text-slate-400 text-[11px] font-mono">
                    -
                  </span>
                </div>
              </td>

              <!-- Hostname Column -->
              <td class="py-3.5 px-3 whitespace-nowrap">
                <span class="font-mono text-xs font-semibold text-slate-700 dark:text-slate-200">
                  {{ inst.liveMetrics?.detectedHostname || inst.hostname || (inst.host !== inst.ipAddress ? inst.host : '-') }}
                </span>
              </td>

              <!-- IP Address Column -->
              <td class="py-3.5 px-3 whitespace-nowrap">
                <span class="font-mono text-xs text-slate-600 dark:text-slate-400">
                  {{ inst.ipAddress || inst.host }}
                </span>
              </td>

              <!-- CPU Column -->
              <td class="py-3.5 px-3 whitespace-nowrap">
                <div class="flex items-center gap-2">
                  <span class="w-10 font-bold font-mono text-slate-900 dark:text-white text-xs">
                    {{ inst.liveMetrics?.cpuPct != null ? `${inst.liveMetrics.cpuPct.toFixed(1)}%` : 'N/A' }}
                  </span>
                  <div class="w-14 bg-slate-100 dark:bg-[#1a2133] h-1.5 rounded-full overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-500"
                      :class="getBarColor(inst.liveMetrics?.cpuPct)"
                      :style="{ width: `${Math.min(100, Math.max(0, inst.liveMetrics?.cpuPct || 0))}%` }"
                    ></div>
                  </div>
                  <span
                    v-if="inst.liveMetrics?.cpuCount"
                    class="text-[10px] text-slate-400 dark:text-slate-500 font-mono"
                    title="vCPU Cores"
                  >
                    {{ inst.liveMetrics.cpuCount }}c
                  </span>
                </div>
              </td>

              <!-- Memory Column -->
              <td class="py-3.5 px-3 whitespace-nowrap">
                <div class="flex items-center gap-2">
                  <span class="w-10 font-bold font-mono text-slate-900 dark:text-white text-xs">
                    {{ inst.liveMetrics?.memPct != null ? `${inst.liveMetrics.memPct.toFixed(1)}%` : 'N/A' }}
                  </span>
                  <div class="w-14 bg-slate-100 dark:bg-[#1a2133] h-1.5 rounded-full overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-500"
                      :class="getBarColor(inst.liveMetrics?.memPct)"
                      :style="{ width: `${Math.min(100, Math.max(0, inst.liveMetrics?.memPct || 0))}%` }"
                    ></div>
                  </div>
                </div>
                <div
                  v-if="inst.liveMetrics?.memTotalBytes"
                  class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5 font-mono"
                >
                  {{ (inst.liveMetrics.memUsedBytes / 1073741824).toFixed(1) }} / {{ (inst.liveMetrics.memTotalBytes / 1073741824).toFixed(1) }} GB
                </div>
              </td>

              <!-- Disk Column -->
              <td class="py-3.5 px-3 whitespace-nowrap">
                <div class="flex items-center gap-2">
                  <span class="w-10 font-bold font-mono text-slate-900 dark:text-white text-xs">
                    {{ getOverallDisk(inst).pct != null ? `${getOverallDisk(inst).pct.toFixed(1)}%` : 'N/A' }}
                  </span>
                  <div class="w-14 bg-slate-100 dark:bg-[#1a2133] h-1.5 rounded-full overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-500"
                      :class="getBarColor(getOverallDisk(inst).pct)"
                      :style="{ width: `${Math.min(100, Math.max(0, getOverallDisk(inst).pct || 0))}%` }"
                    ></div>
                  </div>
                </div>
                <div
                  v-if="getOverallDisk(inst).totalBytes > 0"
                  class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5 font-mono"
                >
                  {{ (getOverallDisk(inst).usedBytes / 1073741824).toFixed(1) }} / {{ (getOverallDisk(inst).totalBytes / 1073741824).toFixed(1) }} GB
                </div>
              </td>

              <!-- RX Column -->
              <td class="py-3.5 px-3 font-semibold whitespace-nowrap">
                <div class="flex items-center gap-1 font-mono text-slate-800 dark:text-slate-200">
                  <ArrowDown class="w-3.5 h-3.5 text-slate-400 shrink-0" />
                  <span>{{ formatRateMB(inst.liveMetrics?.netDownloadMb) }}</span>
                </div>
              </td>

              <!-- TX Column -->
              <td class="py-3.5 px-3 font-semibold whitespace-nowrap">
                <div class="flex items-center gap-1 font-mono text-slate-800 dark:text-slate-200">
                  <ArrowUp class="w-3.5 h-3.5 text-slate-400 shrink-0" />
                  <span>{{ formatRateMB(inst.liveMetrics?.netUploadMb) }}</span>
                </div>
              </td>
            </tr>

            <!-- Empty State -->
            <tr v-if="displayedServers.length === 0">
              <td colspan="9" class="py-12 text-center text-slate-400">
                <Server class="w-8 h-8 mx-auto text-slate-400 mb-2" />
                <div class="font-bold text-sm text-slate-700 dark:text-slate-300">No Monitored Servers Match Criteria</div>
                <p class="text-xs text-slate-500 mt-1">Check your filter parameters or ensure servers are registered.</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- VIEW 2: DOCKER CONTAINERS TABLE VIEW                                  -->
    <!-- ===================================================================== -->
    <div v-else class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl shadow-xs overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-slate-50/75 dark:bg-[#0c101a] border-b border-slate-200 dark:border-[#1f283d] text-[11px] font-semibold text-slate-500 dark:text-slate-400 whitespace-nowrap">
            <tr>
              <th class="py-3 px-3.5 min-w-[180px]">Container</th>
              <th class="py-3 px-3 min-w-[120px]">Node & IP</th>
              <th class="py-3 px-3 min-w-[80px]">Status</th>
              <th class="py-3 px-3 min-w-[120px]">CPU %</th>
              <th class="py-3 px-3 min-w-[140px]">Memory Usage</th>
              <th class="py-3 px-3 min-w-[110px]">Network RX / TX</th>
              <th class="py-3 px-3 min-w-[110px]">Block Read / Write</th>
              <th class="py-3 px-3 min-w-[100px]">Container ID</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-[#1a2236]">
            <tr
              v-for="c in displayedContainers"
              :key="c.id"
              class="hover:bg-slate-50/80 dark:hover:bg-[#151c2d] transition"
              :class="[
                Math.max(c.cpuPct ?? 0, c.memPct ?? 0) >= criticalThreshold ? 'bg-rose-50/30 dark:bg-rose-950/20' : '',
                Math.max(c.cpuPct ?? 0, c.memPct ?? 0) >= warningThreshold && Math.max(c.cpuPct ?? 0, c.memPct ?? 0) < criticalThreshold ? 'bg-amber-50/30 dark:bg-amber-950/20' : ''
              ]"
            >
              <!-- Container Name & Image -->
              <td class="py-3.5 px-3.5">
                <div class="font-bold text-slate-900 dark:text-white text-xs truncate max-w-[200px]" :title="c.containerName">
                  {{ c.containerName }}
                </div>
                <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400 truncate max-w-[220px]">
                  {{ c.imageName }}
                </div>
              </td>

              <!-- Node & IP -->
              <td class="py-3.5 px-3">
                <div class="font-semibold text-slate-800 dark:text-slate-200">{{ c.hostname }}</div>
                <div class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{{ c.ipAddress }}</div>
              </td>

              <!-- Status -->
              <td class="py-3.5 px-3">
                <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-100 dark:bg-emerald-950/70 text-emerald-700 dark:text-emerald-400 border border-emerald-300/50 dark:border-emerald-500/30">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                  <span>Online</span>
                </span>
              </td>

              <!-- CPU % -->
              <td class="py-3.5 px-3 min-w-[120px]">
                <div class="flex items-center gap-2">
                  <span class="w-10 font-bold font-mono text-slate-900 dark:text-white text-xs">
                    {{ c.cpuPct != null ? `${c.cpuPct.toFixed(1)}%` : '0%' }}
                  </span>
                  <div class="w-16 bg-slate-100 dark:bg-[#1a2133] h-1.5 rounded-full overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-300"
                      :class="getBarColor(c.cpuPct)"
                      :style="{ width: `${Math.min(100, Math.max(0, c.cpuPct ?? 0))}%` }"
                    ></div>
                  </div>
                </div>
              </td>

              <!-- Memory Usage -->
              <td class="py-3.5 px-3 min-w-[140px]">
                <div class="flex items-center gap-2">
                  <span class="w-10 font-bold font-mono text-slate-900 dark:text-white text-xs">
                    {{ c.memPct != null ? `${c.memPct.toFixed(1)}%` : '0%' }}
                  </span>
                  <div class="w-16 bg-slate-100 dark:bg-[#1a2133] h-1.5 rounded-full overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-300"
                      :class="getBarColor(c.memPct)"
                      :style="{ width: `${Math.min(100, Math.max(0, c.memPct ?? 0))}%` }"
                    ></div>
                  </div>
                </div>
                <div class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5 font-mono">
                  {{ formatBytesHuman(c.memUsageBytes) }} / {{ formatBytesHuman(c.memLimitBytes) }}
                </div>
              </td>

              <!-- Network RX / TX -->
              <td class="py-3.5 px-3 font-mono text-[11px] whitespace-nowrap">
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">RX:</span> {{ formatRateMB(c.netRxRateMb) }}
                </div>
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">TX:</span> {{ formatRateMB(c.netTxRateMb) }}
                </div>
              </td>

              <!-- Block Read / Write -->
              <td class="py-3.5 px-3 font-mono text-[11px] whitespace-nowrap">
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">R:</span> {{ formatRateMB(c.blockReadRateMb) }}
                </div>
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">W:</span> {{ formatRateMB(c.blockWriteRateMb) }}
                </div>
              </td>

              <!-- Container ID -->
              <td class="py-3.5 px-3 font-mono text-[10px] text-slate-500 dark:text-slate-400">
                {{ c.containerId.substring(0, 12) }}
              </td>
            </tr>

            <!-- Empty State -->
            <tr v-if="displayedContainers.length === 0">
              <td colspan="8" class="py-12 text-center text-slate-400">
                <Box class="w-8 h-8 mx-auto text-slate-400 mb-2" />
                <div class="font-bold text-sm text-slate-700 dark:text-slate-300">No Docker Containers Match Criteria</div>
                <p class="text-xs text-slate-500 mt-1">Ensure Docker containers are running and OpenTelemetry/Docker stats are active.</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
