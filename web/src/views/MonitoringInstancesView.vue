<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import { useThemeStore } from '../stores/theme';
import {
  Server,
  Cpu,
  Layers,
  HardDrive,
  Network,
  Monitor,
  Bell,
  BellOff,
  MoreHorizontal,
  RefreshCw,
  Plus,
  Search,
  LayoutGrid,
  List,
  Trash2,
  Edit2,
  Share2,
  History,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  ArrowDown,
  ArrowUp,
  X,
  Check,
  AlertCircle,
  Users,
  Sliders,
  Box,
} from 'lucide-vue-next';

interface DockerContainerMetric {
  id: string;
  containerId: string;
  containerName: string;
  containerHostname: string;
  imageName: string;
  runtime: string;
  hostname: string;
  ipAddress: string;
  environment: string;
  isOnline: boolean;
  cpuPct: number | null;
  cpuTotalNs: number;
  cpuKernelNs: number;
  cpuUserNs: number;
  memPct: number | null;
  memUsageBytes: number;
  memLimitBytes: number;
  memCacheBytes: number;
  netRxBytes: number;
  netTxBytes: number;
  netRxRateMb: number;
  netTxRateMb: number;
  netRxDropped: number;
  netTxDropped: number;
  blockReadBytes: number;
  blockWriteBytes: number;
  blockReadRateMb: number;
  blockWriteRateMb: number;
  lastUpdated: string;
}

interface DiskMetric {
  mountpoint: string;
  device?: string;
  fsType?: string;
  usagePct: number;
  usedBytes: number;
  totalBytes: number;
  freeBytes: number;
  usageHuman: string;
}

interface LiveMetrics {
  isOnline: boolean;
  detectedHostname?: string;
  agentVersion: string;
  hasOtel: boolean;
  cpuPct: number | null;
  cpuCount: number;
  cpuPhysicalCount?: number;
  cpuLoad1m?: number | null;
  cpuLoad5m?: number | null;
  cpuLoad15m?: number | null;
  memPct: number | null;
  memUsedBytes: number;
  memFreeBytes: number;
  memTotalBytes: number;
  diskPct: number | null;
  diskUsedBytes?: number;
  diskFreeBytes?: number;
  diskTotalBytes?: number;
  disks: DiskMetric[];
  netDownloadMb: number;
  netUploadMb: number;
  netTotalMb: number;
  uptimeSeconds?: number | null;
  uptimeHuman?: string;
  osVersion?: string;
  temperature?: number | null;
  gpuUsagePct?: number | null;
  lastUpdated: string;
}

interface MonitoringInstance {
  id: string;
  name: string;
  host: string;
  ipAddress: string;
  hostname?: string;
  port: number;
  instanceType: string;
  groupName: string;
  tags: string[];
  prometheusTarget: string;
  remoteHostId?: string;
  userId?: number;
  ownerUsername?: string;
  visibility: string;
  alertEnabled: boolean;
  notes?: string;
  isOwner: boolean;
  userPermission?: string;
  sharesCount: number;
  createdAt: string;
  updatedAt: string;
  liveMetrics?: LiveMetrics;
}

interface MetricPoint {
  timestamp: number;
  value: number;
}

interface InstanceHistory {
  instanceId: string;
  timeRange: string;
  cpu: MetricPoint[];
  memory: MetricPoint[];
  disk: MetricPoint[];
  netIn: MetricPoint[];
  netOut: MetricPoint[];
}

interface RemoteHostItem {
  id: string;
  name: string;
  host: string;
  port: number;
  groupName?: string;
}

interface UserOption {
  id: number;
  username: string;
  role: string;
}

interface ShareItem {
  id: string;
  instanceId: string;
  userId: number;
  username: string;
  permission: string;
  sharedByUsername?: string;
  createdAt: string;
}

const authStore = useAuthStore();
const themeStore = useThemeStore();

// Top Navigation Tabs ('servers' | 'containers')
const activeTab = ref<'servers' | 'containers'>('servers');

// State
const instances = ref<MonitoringInstance[]>([]);
const groups = ref<string[]>([]);
const loading = ref(false);
const refreshing = ref(false);
const searchQuery = ref('');
const selectedGroup = ref('all');
const selectedTag = ref('all');
const viewMode = ref<'grid' | 'list'>('grid');
const autoRefreshInterval = ref<number>(30); // 30s default
const activeDropdownId = ref<string | null>(null);
const engineStatus = ref<{
  lastPolledAt: string;
  pollIntervalSeconds: number;
  isPolling: boolean;
  cachedInstances: number;
  cachedContainers?: number;
} | null>(null);

// Docker Containers State
const dockerContainers = ref<DockerContainerMetric[]>([]);
const loadingContainers = ref(false);
const containerSearch = ref('');
const containerHostFilter = ref('ALL');
const containerSortBy = ref<'name' | 'cpu' | 'mem' | 'host'>('cpu');
const containerSortDir = ref<'asc' | 'desc'>('desc');
const containerViewMode = ref<'grid' | 'table'>('grid');

// Notification
const notification = ref<{ text: string; type: 'success' | 'error' } | null>(null);
const showNotice = (text: string, type: 'success' | 'error' = 'success') => {
  notification.value = { text, type };
  setTimeout(() => {
    notification.value = null;
  }, 3000);
};

// Auto refresh timer
let refreshTimer: any = null;

const lastPolledHuman = computed(() => {
  if (engineStatus.value?.lastPolledAt) {
    const d = new Date(engineStatus.value.lastPolledAt);
    if (!isNaN(d.getTime()) && d.getFullYear() > 2000) {
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    }
  }
  return '';
});

const fetchEngineStatus = async () => {
  try {
    const res = await axios.get('/api/v1/monitoring/instances/engine/status');
    if (res.data?.success) {
      engineStatus.value = res.data.data;
    }
  } catch {
    // ignore
  }
};

const triggerPollNow = async () => {
  refreshing.value = true;
  try {
    await axios.post('/api/v1/monitoring/instances/poll-now');
    // Fetch cached instances & containers immediately (non-blocking)
    await Promise.all([
      fetchInstances(true),
      fetchDockerContainers(false),
      fetchEngineStatus(),
    ]);
    showNotice('Metrics poll queued and updated');
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to poll metrics', 'error');
  } finally {
    refreshing.value = false;
  }
};

// ECharts Dynamic Loader
const getECharts = async () => {
  try {
    const mod = await import('echarts');
    return mod.default || mod;
  } catch (e) {
    if ((window as any).echarts) return (window as any).echarts;
    return null;
  }
};

// -----------------------------------------------------------------------------
// Fetch Data
// -----------------------------------------------------------------------------
const fetchInstances = async (silent = false) => {
  if (!silent) loading.value = true;
  else refreshing.value = true;
  try {
    const res = await axios.get('/api/v1/monitoring/instances', {
      params: {
        group: selectedGroup.value !== 'all' ? selectedGroup.value : undefined,
        tag: selectedTag.value !== 'all' ? selectedTag.value : undefined,
        metrics: 'true',
      },
    });
    if (res.data?.success) {
      instances.value = res.data.data || [];
    }
  } catch (err: any) {
    if (!silent) {
      showNotice(err.response?.data?.error || 'Failed to load instances', 'error');
    }
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
};

const fetchGroups = async () => {
  try {
    const res = await axios.get('/api/v1/monitoring/instances/groups');
    if (res.data?.success) {
      groups.value = res.data.data || [];
    }
  } catch {
    // ignore
  }
};

// Distinct Tags from loaded instances
const availableTags = computed(() => {
  const set = new Set<string>();
  instances.value.forEach((inst) => {
    if (inst.tags) {
      inst.tags.forEach((t) => set.add(t));
    }
  });
  return Array.from(set).sort();
});

// Filtered instances
const filteredInstances = computed(() => {
  return instances.value.filter((inst) => {
    // Search
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase();
      const matchName = inst.name.toLowerCase().includes(q);
      const matchHost = inst.host.toLowerCase().includes(q);
      const matchIP = inst.ipAddress?.toLowerCase().includes(q);
      const matchGroup = inst.groupName?.toLowerCase().includes(q);
      const matchTag = inst.tags?.some((t) => t.toLowerCase().includes(q));
      if (!matchName && !matchHost && !matchIP && !matchGroup && !matchTag) {
        return false;
      }
    }
    // Group filter
    if (selectedGroup.value !== 'all' && inst.groupName !== selectedGroup.value) {
      return false;
    }
    // Tag filter
    if (selectedTag.value !== 'all' && (!inst.tags || !inst.tags.includes(selectedTag.value))) {
      return false;
    }
    return true;
  });
});

// -----------------------------------------------------------------------------
// Alert & Priority Thresholds Configuration (Warning default 80%, Critical default 90%)
// -----------------------------------------------------------------------------
const DEFAULT_WARNING_THRESHOLD = 80;
const DEFAULT_CRITICAL_THRESHOLD = 90;

const loadSavedThresholds = () => {
  try {
    const raw = localStorage.getItem('hcp_monitoring_thresholds');
    if (raw) {
      const parsed = JSON.parse(raw);
      const w = Number(parsed.warning);
      const c = Number(parsed.critical);
      if (!isNaN(w) && !isNaN(c) && w > 0 && c > w) {
        return { warning: w, critical: c };
      }
    }
  } catch {
    // ignore
  }
  return { warning: DEFAULT_WARNING_THRESHOLD, critical: DEFAULT_CRITICAL_THRESHOLD };
};

const savedThresholds = loadSavedThresholds();
const warningThreshold = ref<number>(savedThresholds.warning);
const criticalThreshold = ref<number>(savedThresholds.critical);
const showThresholdModal = ref(false);

const editWarningThreshold = ref<number>(warningThreshold.value);
const editCriticalThreshold = ref<number>(criticalThreshold.value);

const openThresholdModal = () => {
  editWarningThreshold.value = warningThreshold.value;
  editCriticalThreshold.value = criticalThreshold.value;
  showThresholdModal.value = true;
};

const saveThresholds = () => {
  const w = Number(editWarningThreshold.value);
  const c = Number(editCriticalThreshold.value);
  if (isNaN(w) || isNaN(c) || w <= 0 || c <= 0) {
    showNotice('Thresholds must be positive numbers', 'error');
    return;
  }
  if (w >= c) {
    showNotice('Warning threshold must be lower than critical threshold', 'error');
    return;
  }
  warningThreshold.value = w;
  criticalThreshold.value = c;
  try {
    localStorage.setItem(
      'hcp_monitoring_thresholds',
      JSON.stringify({ warning: w, critical: c })
    );
  } catch {
    // ignore
  }
  showNotice(`Thresholds saved: Warning ${w}%, Critical ${c}%`);
  showThresholdModal.value = false;
};

const resetThresholdsToDefault = () => {
  editWarningThreshold.value = DEFAULT_WARNING_THRESHOLD;
  editCriticalThreshold.value = DEFAULT_CRITICAL_THRESHOLD;
};

const applyPreset = (w: number, c: number) => {
  editWarningThreshold.value = w;
  editCriticalThreshold.value = c;
};

// -----------------------------------------------------------------------------
// Overall Host Storage Calculation
// -----------------------------------------------------------------------------
interface OverallDiskInfo {
  pct: number | null;
  usedBytes: number;
  totalBytes: number;
  freeBytes: number;
}

const getOverallDisk = (inst: MonitoringInstance): OverallDiskInfo => {
  if (inst.liveMetrics?.disks && inst.liveMetrics.disks.length > 0) {
    let sumUsed = 0;
    let sumTotal = 0;
    for (const d of inst.liveMetrics.disks) {
      sumUsed += d.usedBytes || 0;
      sumTotal += d.totalBytes || 0;
    }
    if (sumTotal > 0) {
      const pct = Math.round((sumUsed / sumTotal) * 1000) / 10;
      return {
        pct,
        usedBytes: sumUsed,
        totalBytes: sumTotal,
        freeBytes: Math.max(0, sumTotal - sumUsed),
      };
    }
  }

  const used = inst.liveMetrics?.diskUsedBytes || 0;
  const total = inst.liveMetrics?.diskTotalBytes || 0;
  const pct =
    inst.liveMetrics?.diskPct !== null && inst.liveMetrics?.diskPct !== undefined
      ? inst.liveMetrics.diskPct
      : null;

  return {
    pct,
    usedBytes: used,
    totalBytes: total,
    freeBytes: Math.max(0, total - used),
  };
};

// -----------------------------------------------------------------------------
// Resource Status & Smart Priority Sorting
// -----------------------------------------------------------------------------
interface InstanceStatus {
  maxUsage: number;
  criticalResource: 'cpu' | 'memory' | 'disk' | null;
  severity: 'critical' | 'warning' | 'normal' | 'offline';
  cpu: number;
  memory: number;
  disk: number;
}

const getInstanceStatus = (inst: MonitoringInstance): InstanceStatus => {
  if (!inst.liveMetrics || !inst.liveMetrics.isOnline) {
    return {
      maxUsage: -1,
      criticalResource: null,
      severity: 'offline',
      cpu: 0,
      memory: 0,
      disk: 0,
    };
  }

  const cpu = typeof inst.liveMetrics.cpuPct === 'number' ? inst.liveMetrics.cpuPct : 0;
  const memory = typeof inst.liveMetrics.memPct === 'number' ? inst.liveMetrics.memPct : 0;
  const overallDisk = getOverallDisk(inst);
  let disk = typeof overallDisk.pct === 'number' ? overallDisk.pct : 0;

  if (Array.isArray(inst.liveMetrics.disks) && inst.liveMetrics.disks.length > 0) {
    for (const d of inst.liveMetrics.disks) {
      if (typeof d.usagePct === 'number' && d.usagePct > disk) {
        disk = d.usagePct;
      }
    }
  }

  let maxUsage = cpu;
  let criticalResource: 'cpu' | 'memory' | 'disk' = 'cpu';

  if (memory > maxUsage) {
    maxUsage = memory;
    criticalResource = 'memory';
  }
  if (disk > maxUsage) {
    maxUsage = disk;
    criticalResource = 'disk';
  }

  let severity: 'critical' | 'warning' | 'normal' = 'normal';
  if (maxUsage >= criticalThreshold.value) {
    severity = 'critical';
  } else if (maxUsage >= warningThreshold.value) {
    severity = 'warning';
  }

  return {
    maxUsage,
    criticalResource: severity === 'normal' ? null : criticalResource,
    severity,
    cpu,
    memory,
    disk,
  };
};

// Sorted Instances: Critical first, then Warning, then Normal, then Offline
const sortedInstances = computed(() => {
  const list = [...filteredInstances.value];
  return list.sort((a, b) => {
    const statusA = getInstanceStatus(a);
    const statusB = getInstanceStatus(b);

    const tierMap = { critical: 0, warning: 1, normal: 2, offline: 3 };
    const tierA = tierMap[statusA.severity];
    const tierB = tierMap[statusB.severity];

    if (tierA !== tierB) {
      return tierA - tierB;
    }

    if (statusA.severity === 'critical' || statusA.severity === 'warning') {
      if (Math.abs(statusB.maxUsage - statusA.maxUsage) > 0.05) {
        return statusB.maxUsage - statusA.maxUsage;
      }
    }

    if (statusA.severity === 'normal') {
      if (Math.abs(statusB.maxUsage - statusA.maxUsage) > 0.05) {
        return statusB.maxUsage - statusA.maxUsage;
      }
    }

    return a.name.localeCompare(b.name);
  });
});

// Summary stats across filtered inventory
const inventoryStatusSummary = computed(() => {
  let criticalCount = 0;
  let warningCount = 0;
  let normalCount = 0;
  let offlineCount = 0;

  for (const inst of filteredInstances.value) {
    const s = getInstanceStatus(inst);
    if (s.severity === 'critical') criticalCount++;
    else if (s.severity === 'warning') warningCount++;
    else if (s.severity === 'normal') normalCount++;
    else offlineCount++;
  }

  return { criticalCount, warningCount, normalCount, offlineCount };
});

// Modal live preview counts based on pending slider values
const previewModalCounts = computed(() => {
  let critical = 0;
  let warning = 0;
  let normal = 0;
  const w = Number(editWarningThreshold.value) || 80;
  const c = Number(editCriticalThreshold.value) || 90;

  for (const inst of instances.value) {
    if (!inst.liveMetrics?.isOnline) continue;
    const cpu = inst.liveMetrics.cpuPct ?? 0;
    const mem = inst.liveMetrics.memPct ?? 0;
    const overallDisk = getOverallDisk(inst);
    let disk = overallDisk.pct ?? 0;
    if (inst.liveMetrics.disks) {
      for (const d of inst.liveMetrics.disks) {
        if (d.usagePct > disk) disk = d.usagePct;
      }
    }
    const maxVal = Math.max(cpu, mem, disk);
    if (maxVal >= c) critical++;
    else if (maxVal >= w) warning++;
    else normal++;
  }
  return { critical, warning, normal };
});

// -----------------------------------------------------------------------------
// Pagination Logic (Grid: max 9 items, Table: max 12 items)
// -----------------------------------------------------------------------------
const GRID_PAGE_SIZE = 9;
const gridPage = ref(1);
const gridTotalPages = computed(() => Math.max(1, Math.ceil(sortedInstances.value.length / GRID_PAGE_SIZE)));
const paginatedGridInstances = computed(() => {
  const start = (gridPage.value - 1) * GRID_PAGE_SIZE;
  return sortedInstances.value.slice(start, start + GRID_PAGE_SIZE);
});

const TABLE_PAGE_SIZE = 12;
const tablePage = ref(1);
const tableTotalPages = computed(() => Math.max(1, Math.ceil(sortedInstances.value.length / TABLE_PAGE_SIZE)));
const paginatedTableInstances = computed(() => {
  const start = (tablePage.value - 1) * TABLE_PAGE_SIZE;
  return sortedInstances.value.slice(start, start + TABLE_PAGE_SIZE);
});

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

// Reset page to 1 on filter or threshold changes
watch([searchQuery, selectedGroup, selectedTag, warningThreshold, criticalThreshold], () => {
  gridPage.value = 1;
  tablePage.value = 1;
});

// Clamp pages within valid bounds
watch(gridTotalPages, (newTotal) => {
  if (gridPage.value > newTotal) {
    gridPage.value = newTotal;
  }
});

watch(tableTotalPages, (newTotal) => {
  if (tablePage.value > newTotal) {
    tablePage.value = newTotal;
  }
});

// Toggle alert notification
const toggleAlert = async (inst: MonitoringInstance) => {
  try {
    const nextState = !inst.alertEnabled;
    inst.alertEnabled = nextState;
    await axios.put(`/api/v1/monitoring/instances/${inst.id}/alert`, {
      alertEnabled: nextState,
    });
    showNotice(`Alerts ${nextState ? 'enabled' : 'muted'} for ${inst.name}`);
  } catch (err: any) {
    inst.alertEnabled = !inst.alertEnabled;
    showNotice(err.response?.data?.error || 'Failed to update alert setting', 'error');
  }
};

// -----------------------------------------------------------------------------
// History Modal & ECharts Visualizer
// -----------------------------------------------------------------------------
const showHistoryModal = ref(false);
const historyInstance = ref<MonitoringInstance | null>(null);
const historyTimeRange = ref<'1h' | '6h' | '24h' | '7d'>('24h');
const historyActiveMetric = ref<'cpu' | 'memory' | 'disk' | 'network'>('cpu');
const historyData = ref<InstanceHistory | null>(null);
const loadingHistory = ref(false);
const chartDomRef = ref<HTMLDivElement | null>(null);
let historyChartInstance: any = null;

const openHistoryModal = async (inst: MonitoringInstance) => {
  activeDropdownId.value = null;
  historyInstance.value = inst;
  showHistoryModal.value = true;
  await fetchHistoryData();
};

const closeHistoryModal = () => {
  showHistoryModal.value = false;
  historyInstance.value = null;
  historyData.value = null;
  if (historyChartInstance) {
    historyChartInstance.dispose();
    historyChartInstance = null;
  }
};

const fetchHistoryData = async () => {
  if (!historyInstance.value) return;
  loadingHistory.value = true;
  try {
    const res = await axios.get(`/api/v1/monitoring/instances/${historyInstance.value.id}/history`, {
      params: { range: historyTimeRange.value },
    });
    if (res.data?.success) {
      historyData.value = res.data.data;
      nextTick(() => {
        renderHistoryChart();
      });
    }
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to load telemetry history', 'error');
  } finally {
    loadingHistory.value = false;
  }
};

const renderHistoryChart = async () => {
  if (!chartDomRef.value || !historyData.value) return;
  const echarts = await getECharts();
  if (!echarts) return;

  if (historyChartInstance) {
    historyChartInstance.dispose();
  }

  const isDark = themeStore.isDark;
  historyChartInstance = echarts.init(chartDomRef.value, isDark ? 'dark' : undefined);

  let seriesConfig: any[] = [];
  let yAxisName = '%';
  let yMax: number | undefined = 100;

  if (historyActiveMetric.value === 'cpu') {
    yAxisName = 'CPU (%)';
    const cpuPoints = historyData.value.cpu.map((p) => [p.timestamp * 1000, p.value]);
    seriesConfig = [
      {
        name: 'CPU Usage',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: cpuPoints,
        lineStyle: { width: 2, color: '#10b981' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(16, 185, 129, 0.35)' },
            { offset: 1, color: 'rgba(16, 185, 129, 0.02)' },
          ]),
        },
      },
    ];
  } else if (historyActiveMetric.value === 'memory') {
    yAxisName = 'Memory (%)';
    const memPoints = historyData.value.memory.map((p) => [p.timestamp * 1000, p.value]);
    seriesConfig = [
      {
        name: 'Memory Usage',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: memPoints,
        lineStyle: { width: 2, color: '#3b82f6' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(59, 130, 246, 0.35)' },
            { offset: 1, color: 'rgba(59, 130, 246, 0.02)' },
          ]),
        },
      },
    ];
  } else if (historyActiveMetric.value === 'disk') {
    yAxisName = 'Disk (%)';
    const diskPoints = historyData.value.disk.map((p) => [p.timestamp * 1000, p.value]);
    seriesConfig = [
      {
        name: 'Disk Usage',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: diskPoints,
        lineStyle: { width: 2, color: '#f59e0b' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245, 158, 11, 0.35)' },
            { offset: 1, color: 'rgba(245, 158, 11, 0.02)' },
          ]),
        },
      },
    ];
  } else if (historyActiveMetric.value === 'network') {
    yAxisName = 'MB/s';
    yMax = undefined;
    const inPoints = historyData.value.netIn.map((p) => [p.timestamp * 1000, p.value]);
    const outPoints = historyData.value.netOut.map((p) => [p.timestamp * 1000, p.value]);
    seriesConfig = [
      {
        name: 'Download (In)',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: inPoints,
        lineStyle: { width: 2, color: '#06b6d4' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(6, 182, 212, 0.3)' },
            { offset: 1, color: 'rgba(6, 182, 212, 0.02)' },
          ]),
        },
      },
      {
        name: 'Upload (Out)',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: outPoints,
        lineStyle: { width: 2, color: '#8b5cf6' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(139, 92, 246, 0.3)' },
            { offset: 1, color: 'rgba(139, 92, 246, 0.02)' },
          ]),
        },
      },
    ];
  }

  const option = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? '#111624' : '#ffffff',
      borderColor: isDark ? '#1f283d' : '#e2e8f0',
      textStyle: { color: isDark ? '#f1f5f9' : '#0f172a', fontSize: 12 },
      formatter: (params: any) => {
        if (!params || !params.length) return '';
        const d = new Date(params[0].value[0]);
        const timeStr = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        let html = `<div class="font-bold mb-1">${timeStr}</div>`;
        params.forEach((item: any) => {
          const val = item.value[1];
          const unit = historyActiveMetric.value === 'network' ? ' MB/s' : '%';
          html += `<div class="flex items-center gap-2 text-xs">
            <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${item.color};"></span>
            <span>${item.seriesName}: <strong>${val !== undefined ? val + unit : 'N/A'}</strong></span>
          </div>`;
        });
        return html;
      },
    },
    grid: {
      top: 30,
      left: 50,
      right: 20,
      bottom: 30,
    },
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: isDark ? '#334155' : '#cbd5e1' } },
      axisLabel: { color: isDark ? '#94a3b8' : '#64748b', fontSize: 10 },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      name: yAxisName,
      nameTextStyle: { color: isDark ? '#94a3b8' : '#64748b', fontSize: 10 },
      max: yMax,
      min: 0,
      axisLine: { show: false },
      axisLabel: { color: isDark ? '#94a3b8' : '#64748b', fontSize: 10 },
      splitLine: { lineStyle: { color: isDark ? '#1e293b' : '#f1f5f9' } },
    },
    series: seriesConfig,
  };

  historyChartInstance.setOption(option);
};

watch(historyTimeRange, () => {
  fetchHistoryData();
});

watch(historyActiveMetric, () => {
  renderHistoryChart();
});

// -----------------------------------------------------------------------------
// Sync Remote Hosts Modal
// -----------------------------------------------------------------------------
const showSyncModal = ref(false);
const remoteHostsList = ref<RemoteHostItem[]>([]);
const selectedSyncHosts = ref<string[]>([]);
const loadingRemoteHosts = ref(false);
const syncing = ref(false);

const openSyncModal = async () => {
  showSyncModal.value = true;
  loadingRemoteHosts.value = true;
  selectedSyncHosts.value = [];
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data?.data) {
      remoteHostsList.value = res.data.data;
      selectedSyncHosts.value = remoteHostsList.value.map((h) => h.id);
    }
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to fetch remote hosts', 'error');
  } finally {
    loadingRemoteHosts.value = false;
  }
};

const executeSync = async () => {
  syncing.value = true;
  try {
    const res = await axios.post('/api/v1/monitoring/instances/sync-remote-hosts', {
      hostIds: selectedSyncHosts.value,
    });
    if (res.data?.success) {
      showNotice(`Successfully synchronized ${res.data.count || 0} host(s)`);
      showSyncModal.value = false;
      await fetchInstances();
      await fetchGroups();
    }
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to sync remote hosts', 'error');
  } finally {
    syncing.value = false;
  }
};

// -----------------------------------------------------------------------------
// Add / Edit Manual Instance Modal
// -----------------------------------------------------------------------------
const showInstanceModal = ref(false);
const isEditing = ref(false);
const currentInstanceId = ref('');
const savingInstance = ref(false);
const instanceForm = ref({
  name: '',
  host: '',
  ipAddress: '',
  hostname: '',
  port: 8889,
  instanceType: 'server',
  groupName: 'Default',
  tags: '',
  prometheusTarget: '',
  visibility: 'private',
  alertEnabled: true,
  notes: '',
});

const openCreateModal = () => {
  isEditing.value = false;
  currentInstanceId.value = '';
  instanceForm.value = {
    name: '',
    host: '',
    ipAddress: '',
    hostname: '',
    port: 8889,
    instanceType: 'server',
    groupName: 'Default',
    tags: '',
    prometheusTarget: '',
    visibility: 'private',
    alertEnabled: true,
    notes: '',
  };
  showInstanceModal.value = true;
};

const openEditModal = (inst: MonitoringInstance) => {
  activeDropdownId.value = null;
  isEditing.value = true;
  currentInstanceId.value = inst.id;
  instanceForm.value = {
    name: inst.name,
    host: inst.host,
    ipAddress: inst.ipAddress || inst.host,
    hostname: inst.hostname || inst.liveMetrics?.detectedHostname || '',
    port: inst.port || 8889,
    instanceType: inst.instanceType || 'server',
    groupName: inst.groupName || 'Default',
    tags: inst.tags ? inst.tags.join(', ') : '',
    prometheusTarget: inst.prometheusTarget || '',
    visibility: inst.visibility || 'private',
    alertEnabled: inst.alertEnabled ?? true,
    notes: inst.notes || '',
  };
  showInstanceModal.value = true;
};

const saveInstance = async () => {
  if (!instanceForm.value.name.trim() || !instanceForm.value.host.trim()) {
    showNotice('Name and Host are required', 'error');
    return;
  }
  savingInstance.value = true;
  try {
    const payload = {
      name: instanceForm.value.name.trim(),
      host: instanceForm.value.host.trim(),
      ipAddress: instanceForm.value.ipAddress.trim() || instanceForm.value.host.trim(),
      hostname: instanceForm.value.hostname.trim(),
      port: Number(instanceForm.value.port) || 8889,
      instanceType: instanceForm.value.instanceType,
      groupName: instanceForm.value.groupName.trim() || 'Default',
      tags: instanceForm.value.tags
        ? instanceForm.value.tags.split(',').map((t) => t.trim()).filter(Boolean)
        : [],
      prometheusTarget: instanceForm.value.prometheusTarget.trim(),
      visibility: instanceForm.value.visibility,
      alertEnabled: instanceForm.value.alertEnabled,
      notes: instanceForm.value.notes.trim(),
    };

    if (isEditing.value) {
      await axios.put(`/api/v1/monitoring/instances/${currentInstanceId.value}`, payload);
      showNotice('Instance updated successfully');
    } else {
      await axios.post('/api/v1/monitoring/instances', payload);
      showNotice('Instance created successfully');
    }

    showInstanceModal.value = false;
    await fetchInstances();
    await fetchGroups();
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to save instance', 'error');
  } finally {
    savingInstance.value = false;
  }
};

// -----------------------------------------------------------------------------
// Granular Sharing Modal (RBAC)
// -----------------------------------------------------------------------------
const showShareModal = ref(false);
const sharingInstance = ref<MonitoringInstance | null>(null);
const instanceShares = ref<ShareItem[]>([]);
const availableUsers = ref<UserOption[]>([]);
const selectedShareUser = ref<number | ''>('');
const selectedSharePerm = ref<'read' | 'manage'>('read');
const loadingShares = ref(false);
const sharingActionLoading = ref(false);

const openShareModal = async (inst: MonitoringInstance) => {
  activeDropdownId.value = null;
  sharingInstance.value = inst;
  showShareModal.value = true;
  loadingShares.value = true;
  try {
    const [sharesRes, usersRes] = await Promise.all([
      axios.get(`/api/v1/monitoring/instances/${inst.id}/shares`),
      axios.get('/api/v1/settings/users'),
    ]);
    if (sharesRes.data?.success) {
      instanceShares.value = sharesRes.data.data || [];
    }
    if (usersRes.data?.data) {
      availableUsers.value = usersRes.data.data;
    }
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to load shares', 'error');
  } finally {
    loadingShares.value = false;
  }
};

const executeAddShare = async () => {
  if (!sharingInstance.value || !selectedShareUser.value) return;
  sharingActionLoading.value = true;
  try {
    await axios.post(`/api/v1/monitoring/instances/${sharingInstance.value.id}/shares`, {
      userId: selectedShareUser.value,
      permission: selectedSharePerm.value,
    });
    showNotice('Instance shared successfully');
    selectedShareUser.value = '';
    const res = await axios.get(`/api/v1/monitoring/instances/${sharingInstance.value.id}/shares`);
    if (res.data?.success) {
      instanceShares.value = res.data.data || [];
    }
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to add share', 'error');
  } finally {
    sharingActionLoading.value = false;
  }
};

const executeRevokeShare = async (userId: number) => {
  if (!sharingInstance.value) return;
  try {
    await axios.delete(`/api/v1/monitoring/instances/${sharingInstance.value.id}/shares/${userId}`);
    showNotice('Share revoked');
    instanceShares.value = instanceShares.value.filter((s) => s.userId !== userId);
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to revoke share', 'error');
  }
};

// -----------------------------------------------------------------------------
// HCP Standard Delete Modal
// -----------------------------------------------------------------------------
const showDeleteModal = ref(false);
const instanceToDelete = ref<MonitoringInstance | null>(null);
const deleting = ref(false);

const confirmDelete = (inst: MonitoringInstance) => {
  activeDropdownId.value = null;
  instanceToDelete.value = inst;
  showDeleteModal.value = true;
};

const executeDelete = async () => {
  if (!instanceToDelete.value) return;
  deleting.value = true;
  try {
    await axios.delete(`/api/v1/monitoring/instances/${instanceToDelete.value.id}`);
    showNotice(`Instance ${instanceToDelete.value.name} deleted`);
    showDeleteModal.value = false;
    instanceToDelete.value = null;
    await fetchInstances();
    await fetchGroups();
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to delete instance', 'error');
  } finally {
    deleting.value = false;
  }
};

// Auto refresh interval handler (30s, 1m, 5m, 0/pause)
const setAutoRefresh = async (sec: number) => {
  autoRefreshInterval.value = sec;
  clearInterval(refreshTimer);
  if (sec > 0) {
    refreshTimer = setInterval(() => {
      fetchInstances(true);
      fetchDockerContainers(false);
      fetchEngineStatus();
    }, sec * 1000);

    // Sync interval with backend queue engine
    try {
      await axios.post('/api/v1/monitoring/instances/engine/interval', {
        interval: `${sec}s`,
      });
    } catch {
      // ignore
    }
  }
};

// Progress bar color helper (dynamically based on custom thresholds)
const getBarColor = (val: number | null | undefined) => {
  if (val === null || val === undefined) return 'bg-slate-700 dark:bg-slate-800';
  if (val >= criticalThreshold.value) return 'bg-rose-500';
  if (val >= warningThreshold.value) return 'bg-amber-500';
  return 'bg-emerald-500';
};

// Expandable rows state & helpers
const expandedRows = ref<Record<string, boolean>>({});

const toggleRowExpand = (id: string) => {
  expandedRows.value[id] = !expandedRows.value[id];
};

const isAllExpanded = computed(() => {
  if (paginatedTableInstances.value.length === 0) return false;
  return paginatedTableInstances.value.every((i) => !!expandedRows.value[i.id]);
});

const toggleExpandAll = () => {
  const next = !isAllExpanded.value;
  paginatedTableInstances.value.forEach((i) => {
    expandedRows.value[i.id] = next;
  });
};

const formatBytesGB = (bytes: number | null | undefined): string => {
  if (!bytes || bytes <= 0) return '0 GB';
  const gb = bytes / 1073741824;
  if (gb >= 1000) {
    return `${(gb / 1024).toFixed(2)} TB`;
  }
  return `${gb.toFixed(2)} GB`;
};

const getMemFreePct = (memPct: number | null | undefined): string => {
  if (memPct === null || memPct === undefined) return 'N/A';
  return `${Math.max(0, Math.min(100, +(100 - memPct).toFixed(1)))}%`;
};

// -----------------------------------------------------------------------------
// Docker Containers Logic & Computeds
// -----------------------------------------------------------------------------
const fetchDockerContainers = async (showLoading = false) => {
  if (showLoading) loadingContainers.value = true;
  try {
    const res = await axios.get('/api/v1/monitoring/containers');
    if (res.data?.success) {
      dockerContainers.value = res.data.data || [];
    }
  } catch (err: any) {
    console.error('Failed to fetch docker containers:', err);
  } finally {
    loadingContainers.value = false;
  }
};

const containerHosts = computed(() => {
  const hosts = new Set<string>();
  for (const c of dockerContainers.value) {
    if (c.hostname) hosts.add(c.hostname);
  }
  return Array.from(hosts).sort();
});

const filteredContainers = computed(() => {
  let list = dockerContainers.value;

  if (containerHostFilter.value !== 'ALL') {
    list = list.filter(
      (c) =>
        c.hostname === containerHostFilter.value ||
        c.ipAddress === containerHostFilter.value
    );
  }

  const q = containerSearch.value.trim().toLowerCase();
  if (q) {
    list = list.filter(
      (c) =>
        c.containerName.toLowerCase().includes(q) ||
        c.imageName.toLowerCase().includes(q) ||
        c.hostname.toLowerCase().includes(q) ||
        c.ipAddress.toLowerCase().includes(q) ||
        c.containerId.toLowerCase().includes(q) ||
        (c.containerHostname && c.containerHostname.toLowerCase().includes(q))
    );
  }

  return [...list].sort((a, b) => {
    let diff = 0;
    if (containerSortBy.value === 'cpu') {
      diff = (a.cpuPct ?? 0) - (b.cpuPct ?? 0);
    } else if (containerSortBy.value === 'mem') {
      diff = (a.memPct ?? 0) - (b.memPct ?? 0);
    } else if (containerSortBy.value === 'host') {
      diff = a.hostname.localeCompare(b.hostname);
    } else {
      diff = a.containerName.localeCompare(b.containerName);
    }
    return containerSortDir.value === 'desc' ? -diff : diff;
  });
});

const containerStats = computed(() => {
  const total = dockerContainers.value.length;
  let maxCpu = 0;
  let maxMem = 0;
  let totalMemUsed = 0;
  const hosts = new Set<string>();

  for (const c of dockerContainers.value) {
    if (c.hostname) hosts.add(c.hostname);
    if ((c.cpuPct ?? 0) > maxCpu) maxCpu = c.cpuPct ?? 0;
    if ((c.memPct ?? 0) > maxMem) maxMem = c.memPct ?? 0;
    totalMemUsed += c.memUsageBytes;
  }

  return {
    total,
    hostsCount: hosts.size,
    maxCpu: maxCpu.toFixed(1),
    maxMem: maxMem.toFixed(1),
    totalMemHuman: formatBytesHuman(totalMemUsed),
  };
});

const CONTAINER_GRID_PAGE_SIZE = 9;
const containerGridPage = ref(1);
const containerGridTotalPages = computed(() =>
  Math.max(1, Math.ceil(filteredContainers.value.length / CONTAINER_GRID_PAGE_SIZE))
);
const paginatedGridContainers = computed(() => {
  const start = (containerGridPage.value - 1) * CONTAINER_GRID_PAGE_SIZE;
  return filteredContainers.value.slice(start, start + CONTAINER_GRID_PAGE_SIZE);
});

const CONTAINER_TABLE_PAGE_SIZE = 12;
const containerTablePage = ref(1);
const containerTableTotalPages = computed(() =>
  Math.max(1, Math.ceil(filteredContainers.value.length / CONTAINER_TABLE_PAGE_SIZE))
);
const paginatedTableContainers = computed(() => {
  const start = (containerTablePage.value - 1) * CONTAINER_TABLE_PAGE_SIZE;
  return filteredContainers.value.slice(start, start + CONTAINER_TABLE_PAGE_SIZE);
});

watch([containerSearch, containerHostFilter, containerSortBy, containerSortDir], () => {
  containerGridPage.value = 1;
  containerTablePage.value = 1;
});

const formatBytesHuman = (bytes: number | null | undefined): string => {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let b = bytes;
  let u = 0;
  while (b >= 1024 && u < units.length - 1) {
    b /= 1024;
    u++;
  }
  return `${b.toFixed(u === 0 ? 0 : u >= 2 ? 1 : 0)} ${units[u]}`;
};

const formatRateMB = (mbVal: number | null | undefined): string => {
  if (!mbVal || mbVal <= 0) return '0 B/s';
  const bytesSec = mbVal * 1024 * 1024;
  if (bytesSec < 1024) return `${bytesSec.toFixed(0)} B/s`;
  if (bytesSec < 1024 * 1024) return `${(bytesSec / 1024).toFixed(1)} KB/s`;
  return `${(bytesSec / (1024 * 1024)).toFixed(2)} MB/s`;
};

// Close dropdown on outside click
const handleClickOutside = (e: MouseEvent) => {
  const target = e.target as HTMLElement;
  if (!target.closest('.dropdown-container')) {
    activeDropdownId.value = null;
  }
};

onMounted(async () => {
  document.addEventListener('click', handleClickOutside);
  await Promise.all([
    fetchInstances(),
    fetchDockerContainers(true),
    fetchGroups(),
    fetchEngineStatus(),
  ]);
  setAutoRefresh(autoRefreshInterval.value);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
  if (refreshTimer) clearInterval(refreshTimer);
  if (historyChartInstance) historyChartInstance.dispose();
});
</script>

<template>
  <div class="space-y-6 max-w-[1600px] w-full mx-auto font-sans">
    <!-- Notification Banner (Auto-dismiss 3s) -->
    <div
      v-if="notification"
      class="fixed top-5 right-5 z-50 flex items-center gap-2.5 px-4 py-3 rounded-xl shadow-xl text-xs font-semibold animate-in fade-in slide-in-from-top-2"
      :class="
        notification.type === 'success'
          ? 'bg-emerald-50 dark:bg-emerald-950/80 border border-emerald-500/30 text-emerald-800 dark:text-emerald-300'
          : 'bg-rose-50 dark:bg-rose-950/80 border border-rose-500/30 text-rose-800 dark:text-rose-300'
      "
    >
      <Check v-if="notification.type === 'success'" class="w-4 h-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
      <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-600 dark:text-rose-400" />
      <span>{{ notification.text }}</span>
    </div>

    <!-- Header (AGENTS.md compliant: No Icon on Title, Clean Pure Text) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Monitoring Instances
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Real-time resource metrics from OpenTelemetry & Prometheus for hosts and containers.
        </p>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <!-- Refresh Button (Instant queue trigger) -->
        <button
          @click="triggerPollNow"
          :disabled="loading || refreshing"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] hover:bg-slate-50 dark:hover:bg-[#161c2d] text-slate-700 dark:text-slate-300 text-xs font-medium rounded-lg transition cursor-pointer disabled:opacity-50"
          title="Queue Prometheus metric poll now"
        >
          <RefreshCw class="w-3.5 h-3.5 text-slate-400" :class="{ 'animate-spin': refreshing || loading }" />
          <span>{{ refreshing ? 'Polling...' : 'Refresh' }}</span>
        </button>

        <!-- Sync Remote Hosts -->
        <button
          v-if="activeTab === 'servers' && authStore.can('monitoring_instances', 'manage')"
          @click="openSyncModal"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] hover:bg-slate-50 dark:hover:bg-[#161c2d] text-slate-700 dark:text-slate-300 text-xs font-medium rounded-lg transition cursor-pointer"
        >
          <Server class="w-3.5 h-3.5 text-slate-400" />
          <span>Sync Remote Hosts</span>
        </button>

        <!-- Add Manual Host -->
        <button
          v-if="activeTab === 'servers' && authStore.can('monitoring_instances', 'manage')"
          @click="openCreateModal"
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold rounded-lg shadow-sm transition cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Host</span>
        </button>
      </div>
    </div>

    <!-- Navigation Tabs: Server vs Container Docker -->
    <div class="flex flex-wrap items-center gap-2.5 border-b border-slate-200 dark:border-[#1b2234] pb-3 text-xs" role="tablist">
      <button
        role="tab"
        :aria-selected="activeTab === 'servers'"
        @click="activeTab = 'servers'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2 cursor-pointer',
          activeTab === 'servers'
            ? 'bg-[#4274D9] text-white border border-[#4274D9] shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-[#1b2234] bg-white dark:bg-[#111624]'
        ]"
      >
        <Server class="w-3.5 h-3.5" />
        <span>Server ({{ instances.length }})</span>
      </button>

      <button
        role="tab"
        :aria-selected="activeTab === 'containers'"
        @click="activeTab = 'containers'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2 cursor-pointer',
          activeTab === 'containers'
            ? 'bg-[#4274D9] text-white border border-[#4274D9] shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-[#1b2234] bg-white dark:bg-[#111624]'
        ]"
      >
        <Layers class="w-3.5 h-3.5" />
        <span>Container Docker ({{ dockerContainers.length }})</span>
      </button>
    </div>

    <!-- ===================================================================== -->
    <!-- TAB 1: SERVER (HOSTS) MONITORING                                      -->
    <!-- ===================================================================== -->
    <div v-if="activeTab === 'servers'" class="space-y-6 mt-6 animate-in fade-in duration-150">
      <!-- Toolbar: Queue engine status, Instant Search, Filters & View Switcher -->
      <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-slate-50/80 dark:bg-[#0d121f] p-3 rounded-xl border border-slate-200/80 dark:border-[#1b2234]">
      <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400 font-medium">
        <div class="flex items-center gap-1.5">
          <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
          <span>Queue Engine: <strong>{{ autoRefreshInterval === 0 ? 'Paused' : autoRefreshInterval < 60 ? `${autoRefreshInterval}s` : `${autoRefreshInterval / 60}m` }}</strong></span>
        </div>
        <span class="text-slate-300 dark:text-slate-600">•</span>
        <span>Cached: <strong>{{ instances.length }} devices</strong></span>
        <template v-if="lastPolledHuman">
          <span class="text-slate-300 dark:text-slate-600">•</span>
          <span>Last polled: <strong>{{ lastPolledHuman }}</strong></span>
        </template>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <!-- Search filter -->
        <div class="relative min-w-[160px] sm:w-48">
          <Search class="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Filter..."
            class="w-full pl-8 pr-3 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 placeholder-slate-400 focus:outline-none focus:border-blue-500 transition"
          />
        </div>

        <!-- Group Filter -->
        <select
          v-model="selectedGroup"
          @change="fetchInstances(false)"
          class="px-2.5 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-700 dark:text-slate-300 focus:outline-none focus:border-blue-500 transition cursor-pointer"
        >
          <option value="all">All Groups</option>
          <option v-for="g in groups" :key="g" :value="g">{{ g }}</option>
        </select>

        <!-- Tag Filter -->
        <select
          v-if="availableTags.length > 0"
          v-model="selectedTag"
          @change="fetchInstances(false)"
          class="px-2.5 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-700 dark:text-slate-300 focus:outline-none focus:border-blue-500 transition cursor-pointer"
        >
          <option value="all">All Tags</option>
          <option v-for="t in availableTags" :key="t" :value="t">{{ t }}</option>
        </select>

        <!-- Alert Thresholds Config Button -->
        <button
          @click="openThresholdModal"
          class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-[#161c2d] transition cursor-pointer font-medium"
          :title="`Thresholds: Warning ${warningThreshold}%, Critical ${criticalThreshold}%. Click to customize.`"
        >
          <Sliders class="w-3.5 h-3.5 text-slate-400" />
          <span class="hidden sm:inline">Thresholds:</span>
          <span class="font-mono text-[11px] font-semibold text-amber-600 dark:text-amber-400">{{ warningThreshold }}%</span>
          <span class="text-slate-300 dark:text-slate-600">/</span>
          <span class="font-mono text-[11px] font-semibold text-rose-600 dark:text-rose-400">{{ criticalThreshold }}%</span>
        </button>

        <!-- Auto Refresh Selector (30s, 1m, 5m, Pause) -->
        <select
          :value="autoRefreshInterval"
          @change="setAutoRefresh(Number(($event.target as HTMLSelectElement).value))"
          class="px-2.5 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-700 dark:text-slate-300 focus:outline-none focus:border-blue-500 transition cursor-pointer font-medium"
          title="Auto Refresh Rate (Queue Engine)"
        >
          <option :value="30">Auto: 30s (Default)</option>
          <option :value="60">Auto: 1m</option>
          <option :value="300">Auto: 5m</option>
          <option :value="0">Auto: Pause</option>
        </select>

        <!-- Grid vs List View Toggle -->
        <div class="flex items-center bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg p-0.5">
          <button
            @click="viewMode = 'grid'"
            :class="viewMode === 'grid' ? 'bg-slate-100 dark:bg-[#1c2438] text-slate-900 dark:text-white font-semibold' : 'text-slate-400 hover:text-slate-600 dark:hover:text-slate-300'"
            class="p-1 rounded cursor-pointer transition"
            title="Grid View"
          >
            <LayoutGrid class="w-3.5 h-3.5" />
          </button>
          <button
            @click="viewMode = 'list'"
            :class="viewMode === 'list' ? 'bg-slate-100 dark:bg-[#1c2438] text-slate-900 dark:text-white font-semibold' : 'text-slate-400 hover:text-slate-600 dark:hover:text-slate-300'"
            class="p-1 rounded cursor-pointer transition"
            title="List View"
          >
            <List class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-if="!loading && filteredInstances.length === 0"
      class="text-center py-16 bg-white dark:bg-[#111624] border border-dashed border-slate-200 dark:border-[#1f283d] rounded-2xl"
    >
      <Server class="w-10 h-10 text-slate-400 mx-auto mb-3 opacity-60" />
      <h3 class="text-sm font-bold text-slate-800 dark:text-slate-200">No Monitored Instances Found</h3>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-1 max-w-sm mx-auto">
        Add devices manually or synchronize existing hosts from the Remote Server / Remote Host configs.
      </p>
      <div class="flex items-center justify-center gap-2 mt-4">
        <button
          v-if="authStore.can('monitoring_instances', 'manage')"
          @click="openSyncModal"
          class="px-3 py-1.5 text-xs font-semibold bg-white dark:bg-[#161c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-700 dark:text-slate-200 hover:bg-slate-50 cursor-pointer"
        >
          Sync Remote Hosts
        </button>
        <button
          v-if="authStore.can('monitoring_instances', 'manage')"
          @click="openCreateModal"
          class="px-3.5 py-1.5 text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white rounded-lg cursor-pointer"
        >
          Add Host
        </button>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- VIEW 1: GRID / CARD VIEW (Replicating User Screenshot 1)               -->
    <!-- ===================================================================== -->
    <div v-else-if="viewMode === 'grid'" class="space-y-4">
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="inst in paginatedGridInstances"
          :key="inst.id"
          class="bg-white dark:bg-[#111624] border rounded-xl p-4 shadow-sm hover:border-slate-300 dark:hover:border-slate-700 transition space-y-3.5 relative"
          :class="
            getInstanceStatus(inst).severity === 'critical'
              ? 'border-rose-400 dark:border-rose-700/80 ring-1 ring-rose-500/20 shadow-rose-500/5'
              : getInstanceStatus(inst).severity === 'warning'
              ? 'border-amber-400 dark:border-amber-700/80 ring-1 ring-amber-500/20 shadow-amber-500/5'
              : 'border-slate-200 dark:border-[#1f283d]'
          "
        >
          <!-- Card Header -->
          <div class="flex items-center justify-between gap-2 border-b border-slate-100 dark:border-[#1b2234] pb-2.5">
            <div class="flex items-center gap-2 min-w-0">
              <!-- Status Dot (Green for online with OTel, Amber if up without OTel, Slate/Red if offline) -->
              <span
                class="w-2.5 h-2.5 rounded-full shrink-0"
                :class="
                  inst.liveMetrics?.isOnline
                    ? 'bg-emerald-500 shadow-xs shadow-emerald-500/50'
                    : inst.liveMetrics?.hasOtel
                    ? 'bg-rose-500'
                    : 'bg-slate-400'
                "
                :title="inst.liveMetrics?.isOnline ? 'Online (OpenTelemetry Reporting)' : 'Offline / No Telemetry'"
              ></span>

              <span class="text-sm font-bold text-slate-900 dark:text-white truncate" :title="inst.name">
                {{ inst.name }}
              </span>

              <!-- Priority Severity Status Badge -->
              <span
                v-if="getInstanceStatus(inst).severity === 'critical'"
                class="px-1.5 py-0.5 rounded text-[10px] font-bold bg-rose-100 dark:bg-rose-950/80 text-rose-700 dark:text-rose-300 border border-rose-300/50 dark:border-rose-800/50 shrink-0"
                :title="`Critical threshold exceeded on ${getInstanceStatus(inst).criticalResource?.toUpperCase()} (${getInstanceStatus(inst).maxUsage.toFixed(1)}%)`"
              >
                CRITICAL
              </span>
              <span
                v-else-if="getInstanceStatus(inst).severity === 'warning'"
                class="px-1.5 py-0.5 rounded text-[10px] font-bold bg-amber-100 dark:bg-amber-950/80 text-amber-700 dark:text-amber-300 border border-amber-300/50 dark:border-amber-800/50 shrink-0"
                :title="`Warning threshold exceeded on ${getInstanceStatus(inst).criticalResource?.toUpperCase()} (${getInstanceStatus(inst).maxUsage.toFixed(1)}%)`"
              >
                WARNING
              </span>

              <span
                v-if="inst.groupName"
                class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-slate-100 dark:bg-[#192236] text-slate-600 dark:text-slate-400 shrink-0"
              >
                {{ inst.groupName }}
              </span>
            </div>

          <!-- Top Right Action Icons -->
          <div class="flex items-center gap-1 shrink-0 dropdown-container">
            <!-- Bell alert notification toggle -->
            <button
              @click="toggleAlert(inst)"
              class="p-1 rounded hover:bg-slate-100 dark:hover:bg-[#1a2337] transition cursor-pointer"
              :title="inst.alertEnabled ? 'Alerts active (click to mute)' : 'Alerts muted (click to enable)'"
            >
              <Bell
                v-if="inst.alertEnabled"
                class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-white"
              />
              <BellOff
                v-else
                class="w-3.5 h-3.5 text-slate-400 dark:text-slate-600 line-through"
              />
            </button>

            <!-- Three-dot Menu -->
            <div class="relative">
              <button
                @click="activeDropdownId = activeDropdownId === inst.id ? null : inst.id"
                class="p-1 rounded hover:bg-slate-100 dark:hover:bg-[#1a2337] transition cursor-pointer"
                title="Options"
              >
                <MoreHorizontal class="w-4 h-4 text-slate-400 hover:text-slate-700 dark:hover:text-white" />
              </button>

              <!-- Dropdown Menu -->
              <div
                v-if="activeDropdownId === inst.id"
                class="absolute right-0 top-6 z-30 w-44 bg-white dark:bg-[#161c2d] border border-slate-200 dark:border-[#222c42] rounded-xl shadow-xl py-1 text-xs text-slate-700 dark:text-slate-300 animate-in fade-in"
              >
                <!-- View History -->
                <button
                  @click="openHistoryModal(inst)"
                  class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-[#1f283d] text-left cursor-pointer"
                >
                  <History class="w-3.5 h-3.5 text-slate-400" />
                  <span>View History</span>
                </button>

                <!-- Manage Shares (Owner/Manager only) -->
                <button
                  v-if="inst.isOwner || inst.userPermission === 'manage' || authStore.user?.role?.toUpperCase() === 'ADMIN'"
                  @click="openShareModal(inst)"
                  class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-[#1f283d] text-left cursor-pointer"
                >
                  <Share2 class="w-3.5 h-3.5 text-slate-400" />
                  <span>Manage Shares ({{ inst.sharesCount }})</span>
                </button>

                <!-- Edit -->
                <button
                  v-if="inst.isOwner || inst.userPermission === 'manage' || authStore.user?.role?.toUpperCase() === 'ADMIN'"
                  @click="openEditModal(inst)"
                  class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-[#1f283d] text-left cursor-pointer"
                >
                  <Edit2 class="w-3.5 h-3.5 text-slate-400" />
                  <span>Edit Instance</span>
                </button>

                <!-- Delete -->
                <button
                  v-if="inst.isOwner || inst.userPermission === 'manage' || authStore.user?.role?.toUpperCase() === 'ADMIN'"
                  @click="confirmDelete(inst)"
                  class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-rose-50 dark:hover:bg-rose-950/40 text-rose-600 dark:text-rose-400 text-left cursor-pointer"
                >
                  <Trash2 class="w-3.5 h-3.5 text-rose-500" />
                  <span>Delete Instance</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Hostname & IP Info Subheader (Separated) -->
        <div class="flex flex-wrap items-center gap-2 text-[11px] pb-1 border-b border-slate-100 dark:border-[#161d2d]">
          <div class="flex items-center gap-1 font-mono">
            <span class="text-[10px] text-slate-400 uppercase tracking-wider">Host:</span>
            <span class="text-slate-800 dark:text-slate-200 font-semibold truncate max-w-[120px]" :title="inst.liveMetrics?.detectedHostname || inst.hostname || inst.host">
              {{ inst.liveMetrics?.detectedHostname || inst.hostname || (inst.host !== inst.ipAddress ? inst.host : '-') }}
            </span>
          </div>
          <span class="text-slate-300 dark:text-slate-700">•</span>
          <div class="flex items-center gap-1 font-mono">
            <span class="text-[10px] text-slate-400 uppercase tracking-wider">IP:</span>
            <span class="text-slate-600 dark:text-slate-300">
              {{ inst.ipAddress || inst.host }}
            </span>
          </div>
        </div>

        <!-- Metrics Rows (Replicating exact card rows from Screenshot 1) -->
        <div class="space-y-2 text-xs">
          <!-- CPU Row -->
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1.5 w-20 shrink-0 text-slate-500 dark:text-slate-400">
              <Cpu class="w-3.5 h-3.5 text-slate-400" />
              <span>CPU:</span>
            </div>
            <div class="w-16 font-semibold text-slate-800 dark:text-slate-200">
              {{ inst.liveMetrics?.cpuPct !== null && inst.liveMetrics?.cpuPct !== undefined ? `${inst.liveMetrics.cpuPct}%` : 'N/A' }}
            </div>
            <!-- Progress Bar -->
            <div class="flex-1 bg-slate-100 dark:bg-[#1a2133] h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="getBarColor(inst.liveMetrics?.cpuPct)"
                :style="{ width: `${Math.min(100, Math.max(0, inst.liveMetrics?.cpuPct || 0))}%` }"
              ></div>
            </div>
          </div>

          <!-- Memory Row -->
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1.5 w-20 shrink-0 text-slate-500 dark:text-slate-400">
              <Layers class="w-3.5 h-3.5 text-slate-400" />
              <span>Memory:</span>
            </div>
            <div class="w-16 font-semibold text-slate-800 dark:text-slate-200">
              {{ inst.liveMetrics?.memPct !== null && inst.liveMetrics?.memPct !== undefined ? `${inst.liveMetrics.memPct}%` : 'N/A' }}
            </div>
            <!-- Progress Bar -->
            <div class="flex-1 bg-slate-100 dark:bg-[#1a2133] h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="getBarColor(inst.liveMetrics?.memPct)"
                :style="{ width: `${Math.min(100, Math.max(0, inst.liveMetrics?.memPct || 0))}%` }"
              ></div>
            </div>
          </div>

          <!-- Disk Row (Overall Storage) -->
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1.5 w-20 shrink-0 text-slate-500 dark:text-slate-400">
              <HardDrive class="w-3.5 h-3.5 text-slate-400" />
              <span>Disk:</span>
            </div>
            <div class="w-16 font-semibold text-slate-800 dark:text-slate-200">
              {{ getOverallDisk(inst).pct !== null ? `${getOverallDisk(inst).pct}%` : 'N/A' }}
            </div>
            <!-- Progress Bar -->
            <div class="flex-1 bg-slate-100 dark:bg-[#1a2133] h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="getBarColor(getOverallDisk(inst).pct)"
                :style="{ width: `${Math.min(100, Math.max(0, getOverallDisk(inst).pct || 0))}%` }"
              ></div>
            </div>
          </div>

          <!-- GPU Row -->
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1.5 w-20 shrink-0 text-slate-500 dark:text-slate-400">
              <Monitor class="w-3.5 h-3.5 text-slate-400" />
              <span>GPU:</span>
            </div>
            <div class="w-16 font-semibold text-slate-800 dark:text-slate-200">
              {{ inst.liveMetrics?.gpuUsagePct !== null && inst.liveMetrics?.gpuUsagePct !== undefined ? `${inst.liveMetrics.gpuUsagePct}%` : '0.0%' }}
            </div>
            <!-- Progress Bar -->
            <div class="flex-1 bg-slate-100 dark:bg-[#1a2133] h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="getBarColor(inst.liveMetrics?.gpuUsagePct || 0)"
                :style="{ width: `${Math.min(100, Math.max(0, inst.liveMetrics?.gpuUsagePct || 0))}%` }"
              ></div>
            </div>
          </div>

          <!-- Net Download Row -->
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1.5 w-20 shrink-0 text-slate-500 dark:text-slate-400">
              <ArrowDown class="w-3.5 h-3.5 text-slate-400" />
              <span>Download:</span>
            </div>
            <div class="font-semibold font-mono text-xs text-slate-800 dark:text-slate-200">
              {{ (inst.liveMetrics?.netDownloadMb || 0).toFixed(2) }} MB/s
            </div>
          </div>

          <!-- Net Upload Row -->
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1.5 w-20 shrink-0 text-slate-500 dark:text-slate-400">
              <ArrowUp class="w-3.5 h-3.5 text-slate-400" />
              <span>Upload:</span>
            </div>
            <div class="font-semibold font-mono text-xs text-slate-800 dark:text-slate-200">
              {{ (inst.liveMetrics?.netUploadMb || 0).toFixed(2) }} MB/s
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- Grid View Pagination Bar (Only when instances exceed 9) -->
      <div
        v-if="sortedInstances.length > GRID_PAGE_SIZE"
        class="flex flex-col sm:flex-row items-center justify-between gap-3 p-3.5 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl shadow-xs text-xs text-slate-500 dark:text-slate-400"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span>
            Showing
            <strong class="text-slate-800 dark:text-slate-200">{{ (gridPage - 1) * GRID_PAGE_SIZE + 1 }}</strong>
            to
            <strong class="text-slate-800 dark:text-slate-200">{{ Math.min(gridPage * GRID_PAGE_SIZE, sortedInstances.length) }}</strong>
            of
            <strong class="text-slate-800 dark:text-slate-200">{{ sortedInstances.length }}</strong>
            instances
          </span>

          <span
            v-if="inventoryStatusSummary.criticalCount > 0"
            class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-rose-100 dark:bg-rose-950/60 text-rose-700 dark:text-rose-300"
          >
            {{ inventoryStatusSummary.criticalCount }} Critical
          </span>
          <span
            v-if="inventoryStatusSummary.warningCount > 0"
            class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-100 dark:bg-amber-950/60 text-amber-700 dark:text-amber-300"
          >
            {{ inventoryStatusSummary.warningCount }} Warning
          </span>
        </div>

        <!-- Pagination Controls -->
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            @click="gridPage--"
            :disabled="gridPage <= 1"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
          >
            <ChevronLeft class="w-3.5 h-3.5 text-slate-400" />
            <span>Prev</span>
          </button>

          <div class="flex items-center gap-1">
            <template v-for="(p, idx) in getVisiblePages(gridPage, gridTotalPages)" :key="idx">
              <span v-if="p < 0" class="px-1 text-slate-400 select-none">...</span>
              <button
                v-else
                type="button"
                @click="gridPage = p"
                :class="[
                  gridPage === p
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
            @click="gridPage++"
            :disabled="gridPage >= gridTotalPages"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
          >
            <span>Next</span>
            <ChevronRight class="w-3.5 h-3.5 text-slate-400" />
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <div v-else class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl shadow-sm">
      <div class="overflow-x-auto min-h-[340px]">
        <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300">
          <thead class="bg-slate-50/80 dark:bg-[#0e1422] border-b border-slate-200 dark:border-[#1f283d] text-[11px] font-semibold text-slate-500 dark:text-slate-400 whitespace-nowrap">
            <tr>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[160px]">
                <div class="flex items-center gap-1.5">
                  <button
                    @click="toggleExpandAll"
                    class="p-0.5 rounded text-slate-400 hover:text-slate-700 dark:hover:text-white transition cursor-pointer"
                    :title="isAllExpanded ? 'Collapse all rows' : 'Expand all rows'"
                  >
                    <ChevronRight
                      class="w-3.5 h-3.5 transition-transform duration-200"
                      :class="{ 'rotate-90': isAllExpanded }"
                    />
                  </button>
                  <span>System Name</span>
                </div>
              </th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[95px]">Group / Tags</th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[95px]">Hostname</th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[95px]">IP Address</th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[105px]">CPU</th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[115px]">Memory</th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[95px]">Disk</th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[105px]">Net Download</th>
              <th class="py-3 px-2.5 2xl:px-3.5 min-w-[105px]">Net Upload</th>
              <th class="py-3 px-2.5 2xl:px-3.5 text-right min-w-[65px]">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-[#1a2236]">
            <template
              v-for="(inst, index) in paginatedTableInstances"
              :key="inst.id"
            >
              <tr
                class="hover:bg-slate-50/60 dark:hover:bg-[#141b2c] transition"
                :class="[
                  expandedRows[inst.id] ? 'bg-slate-50/50 dark:bg-[#131929]' : '',
                  getInstanceStatus(inst).severity === 'critical' ? 'bg-rose-50/20 dark:bg-rose-950/10' : '',
                  getInstanceStatus(inst).severity === 'warning' ? 'bg-amber-50/20 dark:bg-amber-950/10' : '',
                ]"
              >
                <!-- System Name Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 whitespace-nowrap">
                  <div class="flex items-center gap-1.5">
                    <button
                      @click="toggleRowExpand(inst.id)"
                      class="p-0.5 rounded text-slate-400 hover:text-slate-700 dark:hover:text-white transition cursor-pointer"
                      :title="expandedRows[inst.id] ? 'Collapse row details' : 'Expand row details'"
                    >
                      <ChevronRight
                        class="w-3.5 h-3.5 transition-transform duration-200"
                        :class="{ 'rotate-90': expandedRows[inst.id] }"
                      />
                    </button>
                    <span
                      class="w-2 h-2 rounded-full shrink-0"
                      :class="inst.liveMetrics?.isOnline ? 'bg-emerald-500 shadow-xs shadow-emerald-500/50' : 'bg-slate-400'"
                      :title="inst.liveMetrics?.isOnline ? 'Online (Telemetry Active)' : 'Offline / Telemetry Inactive'"
                    ></span>
                    <span
                      class="font-bold text-slate-900 dark:text-white cursor-pointer select-none"
                      @click="toggleRowExpand(inst.id)"
                    >
                      {{ inst.name }}
                    </span>

                    <!-- Semantic Priority Badge -->
                    <span
                      v-if="getInstanceStatus(inst).severity === 'critical'"
                      class="px-1.5 py-0.5 rounded text-[9px] font-bold bg-rose-100 dark:bg-rose-950/80 text-rose-700 dark:text-rose-300 border border-rose-300/50 dark:border-rose-800/50 shrink-0"
                      :title="`Critical threshold exceeded on ${getInstanceStatus(inst).criticalResource?.toUpperCase()} (${getInstanceStatus(inst).maxUsage.toFixed(1)}%)`"
                    >
                      CRITICAL
                    </span>
                    <span
                      v-else-if="getInstanceStatus(inst).severity === 'warning'"
                      class="px-1.5 py-0.5 rounded text-[9px] font-bold bg-amber-100 dark:bg-amber-950/80 text-amber-700 dark:text-amber-300 border border-amber-300/50 dark:border-amber-800/50 shrink-0"
                      :title="`Warning threshold exceeded on ${getInstanceStatus(inst).criticalResource?.toUpperCase()} (${getInstanceStatus(inst).maxUsage.toFixed(1)}%)`"
                    >
                      WARNING
                    </span>
                  </div>
                </td>

                <!-- Group / Tags Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 whitespace-nowrap">
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
                    <span
                      v-if="!inst.groupName && (!inst.tags || inst.tags.length === 0)"
                      class="text-xs text-slate-400 font-mono"
                    >
                      N/A
                    </span>
                  </div>
                </td>

                <!-- Hostname Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 whitespace-nowrap">
                  <span
                    class="font-mono text-xs font-semibold text-slate-700 dark:text-slate-200"
                    :title="inst.liveMetrics?.detectedHostname || inst.hostname || inst.host"
                  >
                    {{ inst.liveMetrics?.detectedHostname || inst.hostname || (inst.host !== inst.ipAddress ? inst.host : '-') }}
                  </span>
                </td>

                <!-- IP Address Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 whitespace-nowrap">
                  <span class="font-mono text-xs text-slate-600 dark:text-slate-300">
                    {{ inst.ipAddress || inst.host }}
                  </span>
                </td>

                <!-- CPU Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 whitespace-nowrap">
                  <div class="flex items-center gap-1.5">
                    <span class="w-9 font-semibold">
                      {{ inst.liveMetrics?.cpuPct !== null && inst.liveMetrics?.cpuPct !== undefined ? `${inst.liveMetrics.cpuPct}%` : 'N/A' }}
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
                      title="Logical vCPU Cores"
                    >
                      {{ inst.liveMetrics.cpuCount }}c
                    </span>
                  </div>
                </td>

                <!-- Memory Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 whitespace-nowrap">
                  <div class="flex items-center gap-1.5">
                    <span class="w-9 font-semibold">
                      {{ inst.liveMetrics?.memPct !== null && inst.liveMetrics?.memPct !== undefined ? `${inst.liveMetrics.memPct}%` : 'N/A' }}
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
                    class="text-[10px] text-slate-400 dark:text-slate-500 mt-0.5 font-mono"
                  >
                    {{ (inst.liveMetrics.memUsedBytes / 1073741824).toFixed(1) }} / {{ (inst.liveMetrics.memTotalBytes / 1073741824).toFixed(1) }} GB
                  </div>
                </td>

                <!-- Disk Column (Overall Storage) -->
                <td class="py-3 px-2.5 2xl:px-3.5 whitespace-nowrap">
                  <div class="flex items-center gap-1.5">
                    <span class="w-9 font-semibold">
                      {{ getOverallDisk(inst).pct !== null ? `${getOverallDisk(inst).pct}%` : 'N/A' }}
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
                    class="text-[10px] text-slate-400 dark:text-slate-500 mt-0.5 font-mono"
                  >
                    {{ (getOverallDisk(inst).usedBytes / 1073741824).toFixed(1) }} / {{ (getOverallDisk(inst).totalBytes / 1073741824).toFixed(1) }} GB
                  </div>
                </td>

                <!-- Net Download Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 font-semibold whitespace-nowrap">
                  <div class="flex items-center gap-1 font-mono text-slate-800 dark:text-slate-200">
                    <ArrowDown class="w-3 h-3 text-slate-400 shrink-0" />
                    <span>{{ (inst.liveMetrics?.netDownloadMb || 0).toFixed(2) }} MB/s</span>
                  </div>
                </td>

                <!-- Net Upload Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 font-semibold whitespace-nowrap">
                  <div class="flex items-center gap-1 font-mono text-slate-800 dark:text-slate-200">
                    <ArrowUp class="w-3 h-3 text-slate-400 shrink-0" />
                    <span>{{ (inst.liveMetrics?.netUploadMb || 0).toFixed(2) }} MB/s</span>
                  </div>
                </td>

                <!-- Actions Column -->
                <td class="py-3 px-2.5 2xl:px-3.5 text-right whitespace-nowrap">
                  <div class="flex items-center justify-end gap-1 dropdown-container">
                    <button
                      @click="toggleAlert(inst)"
                      class="p-1 rounded hover:bg-slate-100 dark:hover:bg-[#1a2337] cursor-pointer"
                      :title="inst.alertEnabled ? 'Alerts active' : 'Alerts muted'"
                    >
                      <Bell
                        v-if="inst.alertEnabled"
                        class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-white"
                      />
                      <BellOff
                        v-else
                        class="w-3.5 h-3.5 text-slate-400 dark:text-slate-600"
                      />
                    </button>

                    <div class="relative" :class="{ 'z-40': activeDropdownId === inst.id }">
                      <button
                        @click="activeDropdownId = activeDropdownId === inst.id ? null : inst.id"
                        class="p-1 rounded hover:bg-slate-100 dark:hover:bg-[#1a2337] cursor-pointer"
                      >
                        <MoreHorizontal class="w-4 h-4 text-slate-400 hover:text-slate-700 dark:hover:text-white" />
                      </button>

                      <div
                        v-if="activeDropdownId === inst.id"
                        class="absolute right-0 z-50 w-44 bg-white dark:bg-[#161c2d] border border-slate-200 dark:border-[#222c42] rounded-xl shadow-2xl py-1 text-xs text-left animate-in fade-in"
                        :class="(paginatedTableInstances.length >= 4 && index >= Math.floor(paginatedTableInstances.length / 2)) ? 'bottom-full mb-1.5' : 'top-6'"
                      >
                        <button
                          @click="openHistoryModal(inst)"
                          class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-[#1f283d] cursor-pointer"
                        >
                          <History class="w-3.5 h-3.5 text-slate-400" />
                          <span>View History</span>
                        </button>
                        <button
                          v-if="inst.isOwner || inst.userPermission === 'manage' || authStore.user?.role?.toUpperCase() === 'ADMIN'"
                          @click="openShareModal(inst)"
                          class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-[#1f283d] cursor-pointer"
                        >
                          <Share2 class="w-3.5 h-3.5 text-slate-400" />
                          <span>Manage Shares ({{ inst.sharesCount }})</span>
                        </button>
                        <button
                          v-if="inst.isOwner || inst.userPermission === 'manage' || authStore.user?.role?.toUpperCase() === 'ADMIN'"
                          @click="openEditModal(inst)"
                          class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-[#1f283d] cursor-pointer"
                        >
                          <Edit2 class="w-3.5 h-3.5 text-slate-400" />
                          <span>Edit Instance</span>
                        </button>
                        <button
                          v-if="inst.isOwner || inst.userPermission === 'manage' || authStore.user?.role?.toUpperCase() === 'ADMIN'"
                          @click="confirmDeleteInstance(inst)"
                          class="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-rose-50 dark:hover:bg-rose-950/30 text-rose-600 dark:text-rose-400 cursor-pointer"
                        >
                          <Trash2 class="w-3.5 h-3.5 text-rose-500" />
                          <span>Delete Instance</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>

              <!-- ============================================================= -->
              <!-- EXPANDED DETAIL SUB-ROW                                        -->
              <!-- ============================================================= -->
              <tr
                v-if="expandedRows[inst.id]"
                class="bg-slate-50/80 dark:bg-[#0c101b] border-y border-slate-200/80 dark:border-[#1e273d]"
              >
                <td colspan="10" class="p-4 sm:p-5 space-y-4">
                  <!-- 1. Metadata Strip -->
                  <div class="flex flex-wrap items-center justify-between gap-3 pb-3 border-b border-slate-200 dark:border-[#1b2336] text-xs">
                    <div class="flex flex-wrap items-center gap-2 sm:gap-3">
                      <!-- Hostname Badge -->
                      <div class="flex items-center gap-1.5 bg-white dark:bg-[#141b2a] px-2.5 py-1 rounded-lg border border-slate-200 dark:border-[#222c42]">
                        <span class="text-slate-400 font-mono text-[11px]">Hostname:</span>
                        <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">
                          {{ inst.liveMetrics?.detectedHostname || inst.hostname || inst.host }}
                        </span>
                      </div>

                      <!-- IP Address Badge -->
                      <div class="flex items-center gap-1.5 bg-white dark:bg-[#141b2a] px-2.5 py-1 rounded-lg border border-slate-200 dark:border-[#222c42]">
                        <span class="text-slate-400 font-mono text-[11px]">IP:</span>
                        <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">
                          {{ inst.ipAddress || inst.host }}
                        </span>
                      </div>

                      <!-- OS Version -->
                      <div class="flex items-center gap-1.5 bg-white dark:bg-[#141b2a] px-2.5 py-1 rounded-lg border border-slate-200 dark:border-[#222c42]">
                        <span class="text-slate-400 font-mono text-[11px]">OS:</span>
                        <span class="font-semibold text-slate-800 dark:text-slate-200">
                          {{ inst.liveMetrics?.osVersion || 'Linux' }}
                        </span>
                      </div>

                      <!-- Uptime -->
                      <div class="flex items-center gap-1.5 bg-white dark:bg-[#141b2a] px-2.5 py-1 rounded-lg border border-slate-200 dark:border-[#222c42]">
                        <span class="text-slate-400 font-mono text-[11px]">Uptime:</span>
                        <span class="font-semibold font-mono text-slate-800 dark:text-slate-200">
                          {{ inst.liveMetrics?.uptimeHuman || 'N/A' }}
                        </span>
                      </div>

                      <!-- Target Exporter Port -->
                      <div class="flex items-center gap-1.5 bg-white dark:bg-[#141b2a] px-2.5 py-1 rounded-lg border border-slate-200 dark:border-[#222c42]">
                        <span class="text-slate-400 font-mono text-[11px]">Target:</span>
                        <span class="font-mono text-slate-700 dark:text-slate-300">
                          {{ inst.prometheusTarget || `${inst.ipAddress || inst.host}:${inst.port}` }}
                        </span>
                      </div>

                      <!-- Owner -->
                      <div class="flex items-center gap-1.5 bg-white dark:bg-[#141b2a] px-2.5 py-1 rounded-lg border border-slate-200 dark:border-[#222c42]">
                        <span class="text-slate-400 font-mono text-[11px]">Owner:</span>
                        <span class="font-semibold text-slate-800 dark:text-slate-200">
                          {{ inst.ownerUsername || 'Admin' }}
                        </span>
                      </div>

                      <!-- Visibility -->
                      <div class="flex items-center gap-1.5 bg-white dark:bg-[#141b2a] px-2.5 py-1 rounded-lg border border-slate-200 dark:border-[#222c42]">
                        <span class="text-slate-400 font-mono text-[11px]">Visibility:</span>
                        <span class="font-semibold" :class="inst.visibility === 'public' ? 'text-amber-500' : 'text-slate-600 dark:text-slate-400'">
                          {{ inst.visibility === 'public' ? 'Public' : 'Private' }}
                        </span>
                      </div>
                    </div>

                    <div class="flex items-center gap-2">
                      <button
                        @click="openHistoryModal(inst)"
                        class="flex items-center gap-1 px-2.5 py-1 bg-white dark:bg-[#141b2a] hover:bg-slate-100 dark:hover:bg-[#1d273d] text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-[#222c42] rounded-lg font-medium text-xs transition cursor-pointer"
                      >
                        <History class="w-3.5 h-3.5 text-slate-400" />
                        <span>View History</span>
                      </button>
                    </div>
                  </div>

                  <!-- 2. Metric Summary Cards (3 Columns) -->
                  <div class="grid grid-cols-1 md:grid-cols-3 gap-3.5">
                    <!-- Card 1: CPU & Load Averages -->
                    <div class="bg-white dark:bg-[#111726] border border-slate-200 dark:border-[#1f293d] rounded-xl p-3.5 space-y-2.5 shadow-2xs">
                      <div class="flex items-center justify-between">
                        <div class="flex items-center gap-2 text-slate-700 dark:text-slate-200 font-bold text-xs">
                          <Cpu class="w-4 h-4 text-slate-400" />
                          <span>CPU & System Load</span>
                        </div>
                        <span class="text-xs font-mono font-bold" :class="inst.liveMetrics?.cpuPct && inst.liveMetrics.cpuPct >= 80 ? 'text-rose-500' : 'text-slate-800 dark:text-slate-200'">
                          {{ inst.liveMetrics?.cpuPct !== null && inst.liveMetrics?.cpuPct !== undefined ? `${inst.liveMetrics.cpuPct}%` : 'N/A' }}
                        </span>
                      </div>

                      <!-- CPU Progress Bar -->
                      <div class="w-full bg-slate-100 dark:bg-[#192236] h-2 rounded-full overflow-hidden">
                        <div
                          class="h-full rounded-full transition-all duration-500"
                          :class="getBarColor(inst.liveMetrics?.cpuPct)"
                          :style="{ width: `${Math.min(100, Math.max(0, inst.liveMetrics?.cpuPct || 0))}%` }"
                        ></div>
                      </div>

                      <div class="space-y-1.5 pt-1 text-xs">
                        <div class="flex justify-between items-center text-slate-500 dark:text-slate-400">
                          <span>CPU Cores:</span>
                          <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">
                            {{ inst.liveMetrics?.cpuCount || 'N/A' }} Logical vCPU
                            <template v-if="inst.liveMetrics?.cpuPhysicalCount && inst.liveMetrics.cpuPhysicalCount !== inst.liveMetrics.cpuCount">
                              ({{ inst.liveMetrics.cpuPhysicalCount }} Physical)
                            </template>
                          </span>
                        </div>

                        <div class="flex justify-between items-center text-slate-500 dark:text-slate-400">
                          <span>Load Avg (1m, 5m, 15m):</span>
                          <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">
                            <template v-if="inst.liveMetrics?.cpuLoad1m !== undefined && inst.liveMetrics?.cpuLoad1m !== null">
                              {{ inst.liveMetrics.cpuLoad1m }} / {{ inst.liveMetrics.cpuLoad5m ?? '-' }} / {{ inst.liveMetrics.cpuLoad15m ?? '-' }}
                            </template>
                            <template v-else>
                              N/A
                            </template>
                          </span>
                        </div>
                      </div>
                    </div>

                    <!-- Card 2: Memory Breakdown (GB and %) -->
                    <div class="bg-white dark:bg-[#111726] border border-slate-200 dark:border-[#1f293d] rounded-xl p-3.5 space-y-2.5 shadow-2xs">
                      <div class="flex items-center justify-between">
                        <div class="flex items-center gap-2 text-slate-700 dark:text-slate-200 font-bold text-xs">
                          <Layers class="w-4 h-4 text-slate-400" />
                          <span>Memory Breakdown</span>
                        </div>
                        <span class="text-xs font-mono font-bold" :class="inst.liveMetrics?.memPct && inst.liveMetrics.memPct >= 85 ? 'text-rose-500' : 'text-slate-800 dark:text-slate-200'">
                          {{ inst.liveMetrics?.memPct !== null && inst.liveMetrics?.memPct !== undefined ? `${inst.liveMetrics.memPct}%` : 'N/A' }}
                        </span>
                      </div>

                      <!-- Mem Progress Bar -->
                      <div class="w-full bg-slate-100 dark:bg-[#192236] h-2 rounded-full overflow-hidden">
                        <div
                          class="h-full rounded-full transition-all duration-500"
                          :class="getBarColor(inst.liveMetrics?.memPct)"
                          :style="{ width: `${Math.min(100, Math.max(0, inst.liveMetrics?.memPct || 0))}%` }"
                        ></div>
                      </div>

                      <div class="space-y-1.5 pt-1 text-xs">
                        <div class="flex justify-between items-center text-slate-500 dark:text-slate-400">
                          <span>Total Memory:</span>
                          <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">
                            {{ formatBytesGB(inst.liveMetrics?.memTotalBytes) }} (100%)
                          </span>
                        </div>

                        <div class="flex justify-between items-center text-slate-500 dark:text-slate-400">
                          <span>Used Memory:</span>
                          <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">
                            {{ formatBytesGB(inst.liveMetrics?.memUsedBytes) }} ({{ inst.liveMetrics?.memPct !== null && inst.liveMetrics?.memPct !== undefined ? `${inst.liveMetrics.memPct}%` : 'N/A' }})
                          </span>
                        </div>

                        <div class="flex justify-between items-center text-slate-500 dark:text-slate-400">
                          <span>Free Memory:</span>
                          <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">
                            {{ formatBytesGB(inst.liveMetrics?.memFreeBytes) }} ({{ getMemFreePct(inst.liveMetrics?.memPct) }})
                          </span>
                        </div>
                      </div>
                    </div>

                    <!-- Card 3: Network Throughput -->
                    <div class="bg-white dark:bg-[#111726] border border-slate-200 dark:border-[#1f293d] rounded-xl p-3.5 space-y-2.5 shadow-2xs">
                      <div class="flex items-center justify-between">
                        <div class="flex items-center gap-2 text-slate-700 dark:text-slate-200 font-bold text-xs">
                          <Network class="w-4 h-4 text-slate-400" />
                          <span>Network Activity</span>
                        </div>
                        <span class="text-xs font-mono font-bold text-slate-800 dark:text-slate-200">
                          {{ ((inst.liveMetrics?.netDownloadMb || 0) + (inst.liveMetrics?.netUploadMb || 0)).toFixed(2) }} MB/s
                        </span>
                      </div>

                      <div class="space-y-2 pt-1 text-xs">
                        <div class="flex items-center justify-between p-2 rounded-lg bg-slate-50 dark:bg-[#141b2a] border border-slate-100 dark:border-[#1e273d]">
                          <div class="flex items-center gap-1.5 text-slate-600 dark:text-slate-300">
                            <ArrowDown class="w-3.5 h-3.5 text-slate-400" />
                            <span>Download (Rx):</span>
                          </div>
                          <div class="text-right">
                            <div class="font-mono font-semibold text-slate-900 dark:text-white">
                              {{ (inst.liveMetrics?.netDownloadMb || 0).toFixed(2) }} MB/s
                            </div>
                            <div class="text-[10px] font-mono text-slate-400">
                              {{ ((inst.liveMetrics?.netDownloadMb || 0) * 8).toFixed(2) }} Mbps
                            </div>
                          </div>
                        </div>

                        <div class="flex items-center justify-between p-2 rounded-lg bg-slate-50 dark:bg-[#141b2a] border border-slate-100 dark:border-[#1e273d]">
                          <div class="flex items-center gap-1.5 text-slate-600 dark:text-slate-300">
                            <ArrowUp class="w-3.5 h-3.5 text-slate-400" />
                            <span>Upload (Tx):</span>
                          </div>
                          <div class="text-right">
                            <div class="font-mono font-semibold text-slate-900 dark:text-white">
                              {{ (inst.liveMetrics?.netUploadMb || 0).toFixed(2) }} MB/s
                            </div>
                            <div class="text-[10px] font-mono text-slate-400">
                              {{ ((inst.liveMetrics?.netUploadMb || 0) * 8).toFixed(2) }} Mbps
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- 3. All Disk Mountpoints Table -->
                  <div class="bg-white dark:bg-[#111726] border border-slate-200 dark:border-[#1f293d] rounded-xl overflow-hidden shadow-2xs">
                    <div class="flex items-center justify-between px-4 py-2.5 bg-slate-50/80 dark:bg-[#0e1422] border-b border-slate-200 dark:border-[#1f283d]">
                      <div class="flex items-center gap-2 font-bold text-xs text-slate-800 dark:text-slate-200">
                        <HardDrive class="w-4 h-4 text-slate-400" />
                        <span>All Mounted Disks & Partitions ({{ inst.liveMetrics?.disks?.length || 0 }})</span>
                      </div>
                      <span class="text-[11px] text-slate-400 font-mono">
                        Root & Storage Mountpoints
                      </span>
                    </div>

                    <div class="overflow-x-auto">
                      <table class="w-full text-left text-xs">
                        <thead class="bg-slate-50/40 dark:bg-[#131928] text-[11px] font-semibold text-slate-500 dark:text-slate-400 border-b border-slate-100 dark:border-[#1a2336]">
                          <tr>
                            <th class="py-2.5 px-3.5">Mountpoint</th>
                            <th class="py-2.5 px-3.5">Device</th>
                            <th class="py-2.5 px-3.5">Filesystem</th>
                            <th class="py-2.5 px-3.5">Total</th>
                            <th class="py-2.5 px-3.5">Used</th>
                            <th class="py-2.5 px-3.5">Free</th>
                            <th class="py-2.5 px-3.5 min-w-[130px]">Usage (%)</th>
                          </tr>
                        </thead>
                        <tbody class="divide-y divide-slate-100 dark:divide-[#182133]">
                          <tr
                            v-for="d in (inst.liveMetrics?.disks || [])"
                            :key="d.mountpoint"
                            class="hover:bg-slate-50/50 dark:hover:bg-[#141c2e] transition"
                          >
                            <td class="py-2 px-3.5 font-mono font-bold text-slate-900 dark:text-white">
                              {{ d.mountpoint }}
                            </td>
                            <td class="py-2 px-3.5 font-mono text-slate-600 dark:text-slate-300">
                              {{ d.device || 'N/A' }}
                            </td>
                            <td class="py-2 px-3.5 font-mono text-slate-600 dark:text-slate-300">
                              <span class="px-1.5 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-[#192236] border border-slate-200 dark:border-[#222c42]">
                                {{ d.fsType || 'auto' }}
                              </span>
                            </td>
                            <td class="py-2 px-3.5 font-mono text-slate-700 dark:text-slate-300">
                              {{ formatBytesGB(d.totalBytes) }}
                            </td>
                            <td class="py-2 px-3.5 font-mono font-semibold text-slate-800 dark:text-slate-200">
                              {{ formatBytesGB(d.usedBytes) }}
                            </td>
                            <td class="py-2 px-3.5 font-mono text-slate-700 dark:text-slate-300">
                              {{ formatBytesGB(d.freeBytes) }}
                            </td>
                            <td class="py-2 px-3.5 whitespace-nowrap">
                              <div class="flex items-center gap-2">
                                <span class="w-10 font-mono font-semibold text-slate-800 dark:text-slate-200">
                                  {{ d.usagePct }}%
                                </span>
                                <div class="w-20 bg-slate-100 dark:bg-[#1a2133] h-2 rounded-full overflow-hidden">
                                  <div
                                    class="h-full rounded-full transition-all duration-500"
                                    :class="getBarColor(d.usagePct)"
                                    :style="{ width: `${Math.min(100, Math.max(0, d.usagePct || 0))}%` }"
                                  ></div>
                                </div>
                              </div>
                            </td>
                          </tr>
                          <tr v-if="!inst.liveMetrics?.disks || inst.liveMetrics.disks.length === 0">
                            <td colspan="7" class="py-4 text-center text-xs text-slate-400 font-sans">
                              No mounted filesystem metrics available for this instance.
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                  </div>
                </td>
              </tr>
            </template>

            <!-- Empty State -->
            <tr v-if="filteredInstances.length === 0">
              <td colspan="10" class="py-12 text-center text-slate-400 dark:text-slate-500">
                <Server class="w-8 h-8 mx-auto mb-2 opacity-30" />
                <p class="font-semibold text-xs text-slate-600 dark:text-slate-300">No monitoring instances found</p>
                <p class="text-[11px] text-slate-400 mt-0.5">Try adjusting your search or group filter.</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Table View Pagination Bar (Only when instances exceed 12) -->
      <div
        v-if="sortedInstances.length > TABLE_PAGE_SIZE"
        class="flex flex-col sm:flex-row items-center justify-between gap-3 px-4 py-3 border-t border-slate-200 dark:border-[#1f283d] bg-slate-50/50 dark:bg-[#0c101b] rounded-b-xl text-xs text-slate-500 dark:text-slate-400"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span>
            Showing
            <strong class="text-slate-800 dark:text-slate-200">{{ (tablePage - 1) * TABLE_PAGE_SIZE + 1 }}</strong>
            to
            <strong class="text-slate-800 dark:text-slate-200">{{ Math.min(tablePage * TABLE_PAGE_SIZE, sortedInstances.length) }}</strong>
            of
            <strong class="text-slate-800 dark:text-slate-200">{{ sortedInstances.length }}</strong>
            instances
          </span>

          <span
            v-if="inventoryStatusSummary.criticalCount > 0"
            class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-rose-100 dark:bg-rose-950/60 text-rose-700 dark:text-rose-300"
          >
            {{ inventoryStatusSummary.criticalCount }} Critical
          </span>
          <span
            v-if="inventoryStatusSummary.warningCount > 0"
            class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-100 dark:bg-amber-950/60 text-amber-700 dark:text-amber-300"
          >
            {{ inventoryStatusSummary.warningCount }} Warning
          </span>
        </div>

        <!-- Pagination Controls -->
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            @click="tablePage--"
            :disabled="tablePage <= 1"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
          >
            <ChevronLeft class="w-3.5 h-3.5 text-slate-400" />
            <span>Prev</span>
          </button>

          <div class="flex items-center gap-1">
            <template v-for="(p, idx) in getVisiblePages(tablePage, tableTotalPages)" :key="idx">
              <span v-if="p < 0" class="px-1 text-slate-400 select-none">...</span>
              <button
                v-else
                type="button"
                @click="tablePage = p"
                :class="[
                  tablePage === p
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
            @click="tablePage++"
            :disabled="tablePage >= tableTotalPages"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
          >
            <span>Next</span>
            <ChevronRight class="w-3.5 h-3.5 text-slate-400" />
          </button>
        </div>
      </div>
    </div>

    </div> <!-- Close Tab 1: Server Monitoring -->

    <!-- ===================================================================== -->
    <!-- TAB 2: CONTAINER DOCKER MONITORING                                   -->
    <!-- ===================================================================== -->
    <div v-if="activeTab === 'containers'" class="space-y-6 mt-6 animate-in fade-in duration-150">
      <!-- Container Toolbar: Queue engine status, Instant Search, Host Filter, Sort, View Switcher -->
      <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-slate-50/80 dark:bg-[#0d121f] p-3 rounded-xl border border-slate-200/80 dark:border-[#1b2234]">
        <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400 font-medium">
          <div class="flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
            <span>Queue Engine: <strong>{{ autoRefreshInterval === 0 ? 'Paused' : autoRefreshInterval < 60 ? `${autoRefreshInterval}s` : `${autoRefreshInterval / 60}m` }}</strong></span>
          </div>
          <span class="text-slate-300 dark:text-slate-600">•</span>
          <span>Containers: <strong>{{ dockerContainers.length }}</strong></span>
          <span class="text-slate-300 dark:text-slate-600">•</span>
          <span>Active Nodes: <strong>{{ containerStats.hostsCount }}</strong></span>
          <template v-if="lastPolledHuman">
            <span class="text-slate-300 dark:text-slate-600">•</span>
            <span>Last polled: <strong>{{ lastPolledHuman }}</strong></span>
          </template>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <!-- Container Search -->
          <div class="relative min-w-[180px] sm:w-56">
            <Search class="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <input
              v-model="containerSearch"
              type="text"
              placeholder="Search container, image, host..."
              class="w-full pl-8 pr-3 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 placeholder-slate-400 focus:outline-none focus:border-blue-500 transition"
            />
          </div>

          <!-- Host Node Filter -->
          <select
            v-model="containerHostFilter"
            class="px-2.5 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-700 dark:text-slate-300 focus:outline-none focus:border-blue-500 transition cursor-pointer"
          >
            <option value="ALL">All Nodes ({{ dockerContainers.length }})</option>
            <option v-for="h in containerHosts" :key="h" :value="h">
              Node: {{ h }} ({{ dockerContainers.filter(c => c.hostname === h).length }})
            </option>
          </select>

          <!-- Sort Selector -->
          <select
            v-model="containerSortBy"
            class="px-2.5 py-1.5 text-xs bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-700 dark:text-slate-300 focus:outline-none focus:border-blue-500 transition cursor-pointer"
          >
            <option value="cpu">Sort: CPU Usage</option>
            <option value="mem">Sort: Memory Usage</option>
            <option value="name">Sort: Container Name</option>
            <option value="host">Sort: Host Node</option>
          </select>

          <!-- Sort Direction Toggle -->
          <button
            @click="containerSortDir = containerSortDir === 'desc' ? 'asc' : 'desc'"
            class="p-1.5 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-300 rounded-lg hover:bg-slate-50 dark:hover:bg-[#161c2d] transition cursor-pointer"
            :title="containerSortDir === 'desc' ? 'Descending' : 'Ascending'"
          >
            <ArrowDown v-if="containerSortDir === 'desc'" class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
            <ArrowUp v-else class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
          </button>

          <!-- View Mode Switcher -->
          <div class="flex items-center bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg p-0.5">
            <button
              @click="containerViewMode = 'grid'"
              :class="containerViewMode === 'grid' ? 'bg-slate-100 dark:bg-[#1e273c] text-blue-600 dark:text-blue-400 shadow-xs' : 'text-slate-400 hover:text-slate-600 dark:hover:text-slate-200'"
              class="p-1 rounded-md transition cursor-pointer"
              title="Grid Card View"
            >
              <LayoutGrid class="w-3.5 h-3.5" />
            </button>
            <button
              @click="containerViewMode = 'table'"
              :class="containerViewMode === 'table' ? 'bg-slate-100 dark:bg-[#1e273c] text-blue-600 dark:text-blue-400 shadow-xs' : 'text-slate-400 hover:text-slate-600 dark:hover:text-slate-200'"
              class="p-1 rounded-md transition cursor-pointer"
              title="Table Details View"
            >
              <List class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>


      <!-- Loading State -->
      <div v-if="loadingContainers" class="p-12 text-center bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] rounded-2xl">
        <RefreshCw class="w-6 h-6 animate-spin mx-auto text-blue-500" />
        <p class="text-xs text-slate-400 mt-2 font-medium">Fetching Docker container metrics from Prometheus...</p>
      </div>

      <!-- Empty State -->
      <div v-else-if="filteredContainers.length === 0" class="p-12 text-center bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] rounded-2xl space-y-2">
        <Layers class="w-8 h-8 mx-auto text-slate-400" />
        <div class="text-sm font-bold text-slate-900 dark:text-white">No Docker Containers Found</div>
        <p class="text-xs text-slate-500 dark:text-slate-400 max-w-sm mx-auto">
          No active Docker containers match the filter criteria or telemetry has not reported yet from OpenTelemetry collector.
        </p>
      </div>

      <!-- Container Grid View (Cards) -->
      <div v-else-if="containerViewMode === 'grid'" class="space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5" style="gap: 1.25rem;">
          <div
            v-for="c in paginatedGridContainers"
            :key="c.id"
            class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-4 shadow-sm hover:border-slate-300 dark:hover:border-[#2a3652] transition flex flex-col justify-between"
            style="padding: 1.25rem;"
          >
            <!-- Card Header -->
            <div class="space-y-2 pb-3 border-b border-slate-100 dark:border-[#161c2d]">
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-center gap-2 min-w-0">
                  <span class="w-2.5 h-2.5 rounded-full bg-emerald-500 shrink-0 shadow-sm shadow-emerald-500/50"></span>
                  <div class="min-w-0">
                    <h3 class="text-sm font-bold text-slate-900 dark:text-white truncate" :title="c.containerName">
                      {{ c.containerName }}
                    </h3>
                  </div>
                </div>
                <span class="px-2 py-0.5 rounded text-[10px] font-semibold bg-slate-100 dark:bg-[#182032] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-[#1f283d] shrink-0 font-mono">
                  {{ c.hostname }}
                </span>
              </div>

              <!-- Image name badge -->
              <div class="flex items-center gap-1.5 text-[11px] font-mono text-slate-500 dark:text-slate-400 truncate">
                <span class="px-2.5 py-1 rounded-lg bg-slate-50 dark:bg-[#0c101a] border border-slate-200/60 dark:border-[#1b2234] truncate max-w-full text-slate-600 dark:text-slate-400">
                  {{ c.imageName }}
                </span>
              </div>
            </div>

            <!-- Resource Gauges (CPU & Memory) -->
            <div class="space-y-3.5">
              <!-- CPU Usage -->
              <div class="space-y-1.5">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-slate-500 dark:text-slate-400 font-medium flex items-center gap-1.5">
                    <Cpu class="w-3.5 h-3.5 text-slate-400" />
                    <span>CPU Usage</span>
                  </span>
                  <span class="font-bold text-slate-900 dark:text-white font-mono">
                    {{ c.cpuPct != null ? `${c.cpuPct.toFixed(1)}%` : '0%' }}
                  </span>
                </div>
                <div class="h-2 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all duration-300"
                    :class="getBarColor(c.cpuPct)"
                    :style="{ width: `${Math.min(100, Math.max(0, c.cpuPct ?? 0))}%` }"
                  ></div>
                </div>
              </div>

              <!-- Memory Usage -->
              <div class="space-y-1.5">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-slate-500 dark:text-slate-400 font-medium flex items-center gap-1.5">
                    <HardDrive class="w-3.5 h-3.5 text-slate-400" />
                    <span>Memory</span>
                  </span>
                  <span class="font-bold text-slate-900 dark:text-white font-mono">
                    {{ c.memPct != null ? `${c.memPct.toFixed(1)}%` : '0%' }}
                  </span>
                </div>
                <div class="h-2 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all duration-300"
                    :class="getBarColor(c.memPct)"
                    :style="{ width: `${Math.min(100, Math.max(0, c.memPct ?? 0))}%` }"
                  ></div>
                </div>
                <div class="flex items-center justify-between text-[11px] text-slate-400 pt-0.5">
                  <span>{{ formatBytesHuman(c.memUsageBytes) }} / {{ formatBytesHuman(c.memLimitBytes) }}</span>
                  <span v-if="c.memCacheBytes > 0">{{ formatBytesHuman(c.memCacheBytes) }} cache</span>
                </div>
              </div>
            </div>

            <!-- Network & Block I/O Stats (2 columns) -->
            <div class="grid grid-cols-2 gap-3 pt-3 border-t border-slate-100 dark:border-[#161c2d] text-[11px]">
              <!-- Network Column -->
              <div class="bg-slate-50/70 dark:bg-[#0c101a] p-3 rounded-xl border border-slate-200/60 dark:border-[#1b2234] space-y-1">
                <div class="text-[10px] font-semibold text-slate-400 flex items-center gap-1.5">
                  <Network class="w-3.5 h-3.5 text-slate-400" />
                  <span>Network</span>
                </div>
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">RX:</span> {{ formatRateMB(c.netRxRateMb) }}
                </div>
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">TX:</span> {{ formatRateMB(c.netTxRateMb) }}
                </div>
              </div>

              <!-- Block I/O Column -->
              <div class="bg-slate-50/70 dark:bg-[#0c101a] p-3 rounded-xl border border-slate-200/60 dark:border-[#1b2234] space-y-1">
                <div class="text-[10px] font-semibold text-slate-400 flex items-center gap-1.5">
                  <HardDrive class="w-3.5 h-3.5 text-slate-400" />
                  <span>Disk I/O</span>
                </div>
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">R:</span> {{ formatRateMB(c.blockReadRateMb) }}
                </div>
                <div class="text-slate-700 dark:text-slate-300">
                  <span class="text-slate-400 text-[10px]">W:</span> {{ formatRateMB(c.blockWriteRateMb) }}
                </div>
              </div>
            </div>

            <!-- Card Footer -->
            <div class="flex items-center justify-between pt-3 border-t border-slate-100 dark:border-[#161c2d] text-[11px] text-slate-400">
              <span class="font-mono truncate max-w-[150px]">ID: {{ c.containerHostname || (c.containerId ? c.containerId.substring(0, 12) : 'N/A') }}</span>
              <span class="font-mono text-slate-500 dark:text-slate-400">Node: {{ c.ipAddress }}</span>
            </div>
          </div>
        </div>

        <!-- Container Grid Pagination -->
        <div class="flex flex-col sm:flex-row items-center justify-between gap-3 p-4 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] rounded-xl text-xs">
          <div class="text-slate-500 dark:text-slate-400">
            Showing <strong class="text-slate-800 dark:text-slate-200">{{ (containerGridPage - 1) * CONTAINER_GRID_PAGE_SIZE + 1 }}</strong> to
            <strong class="text-slate-800 dark:text-slate-200">{{ Math.min(containerGridPage * CONTAINER_GRID_PAGE_SIZE, filteredContainers.length) }}</strong> of
            <strong class="text-slate-800 dark:text-slate-200">{{ filteredContainers.length }}</strong> containers
          </div>

          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="containerGridPage--"
              :disabled="containerGridPage <= 1"
              class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
            >
              <ChevronLeft class="w-3.5 h-3.5 text-slate-400" />
              <span>Prev</span>
            </button>

            <div class="flex items-center gap-1">
              <template v-for="(p, idx) in getVisiblePages(containerGridPage, containerGridTotalPages)" :key="idx">
                <span v-if="p < 0" class="px-1 text-slate-400 select-none">...</span>
                <button
                  v-else
                  type="button"
                  @click="containerGridPage = p"
                  :class="[
                    containerGridPage === p
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
              @click="containerGridPage++"
              :disabled="containerGridPage >= containerGridTotalPages"
              class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
            >
              <span>Next</span>
              <ChevronRight class="w-3.5 h-3.5 text-slate-400" />
            </button>
          </div>
        </div>
      </div>

      <!-- Container Table View (Details) -->
      <div v-else class="space-y-4">
        <div class="overflow-x-auto bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] rounded-2xl shadow-sm">
          <table class="w-full text-left text-xs">
            <thead class="bg-slate-50/80 dark:bg-[#0c101a] border-b border-slate-200 dark:border-[#1b2234] text-slate-500 dark:text-slate-400">
              <tr>
                <th class="py-3 px-4 font-semibold">Container</th>
                <th class="py-3 px-3 font-semibold">Node & IP</th>
                <th class="py-3 px-3 font-semibold">Status</th>
                <th class="py-3 px-3 font-semibold">CPU %</th>
                <th class="py-3 px-3 font-semibold">Memory Usage</th>
                <th class="py-3 px-3 font-semibold">Cache</th>
                <th class="py-3 px-3 font-semibold">Network I/O</th>
                <th class="py-3 px-3 font-semibold">Block I/O</th>
                <th class="py-3 px-4 font-semibold">Container ID</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-[#161c2d]">
              <tr
                v-for="c in paginatedTableContainers"
                :key="c.id"
                class="hover:bg-slate-50/60 dark:hover:bg-[#161c2d]/40 transition"
              >
                <!-- Container Name & Image -->
                <td class="py-3 px-4">
                  <div class="font-bold text-slate-900 dark:text-white truncate max-w-[200px]" :title="c.containerName">
                    {{ c.containerName }}
                  </div>
                  <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400 truncate max-w-[220px]">
                    {{ c.imageName }}
                  </div>
                </td>

                <!-- Node & IP -->
                <td class="py-3 px-3">
                  <div class="font-semibold text-slate-800 dark:text-slate-200">{{ c.hostname }}</div>
                  <div class="text-[10px] text-slate-400 font-mono">{{ c.ipAddress }}</div>
                </td>

                <!-- Status -->
                <td class="py-3 px-3">
                  <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-50 dark:bg-emerald-950/60 text-emerald-700 dark:text-emerald-400 border border-emerald-500/20">
                    <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                    <span>Online</span>
                  </span>
                </td>

                <!-- CPU % -->
                <td class="py-3 px-3 min-w-[120px]">
                  <div class="flex items-center justify-between text-[11px] mb-1">
                    <span class="font-bold text-slate-900 dark:text-white">{{ c.cpuPct != null ? `${c.cpuPct.toFixed(1)}%` : '0%' }}</span>
                  </div>
                  <div class="h-1.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-300"
                      :class="getBarColor(c.cpuPct)"
                      :style="{ width: `${Math.min(100, Math.max(0, c.cpuPct ?? 0))}%` }"
                    ></div>
                  </div>
                </td>

                <!-- Memory Usage -->
                <td class="py-3 px-3 min-w-[140px]">
                  <div class="flex items-center justify-between text-[11px] mb-1">
                    <span class="font-bold text-slate-900 dark:text-white">{{ c.memPct != null ? `${c.memPct.toFixed(1)}%` : '0%' }}</span>
                    <span class="text-[10px] text-slate-400">{{ formatBytesHuman(c.memUsageBytes) }}</span>
                  </div>
                  <div class="h-1.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-300"
                      :class="getBarColor(c.memPct)"
                      :style="{ width: `${Math.min(100, Math.max(0, c.memPct ?? 0))}%` }"
                    ></div>
                  </div>
                  <div class="text-[10px] text-slate-400 mt-0.5">Limit: {{ formatBytesHuman(c.memLimitBytes) }}</div>
                </td>

                <!-- Cache -->
                <td class="py-3 px-3 text-slate-600 dark:text-slate-300">
                  {{ formatBytesHuman(c.memCacheBytes) }}
                </td>

                <!-- Network I/O -->
                <td class="py-3 px-3">
                  <div class="text-slate-700 dark:text-slate-300 whitespace-nowrap">
                    <span class="text-slate-400 text-[10px]">RX:</span> {{ formatRateMB(c.netRxRateMb) }}
                  </div>
                  <div class="text-slate-700 dark:text-slate-300 whitespace-nowrap">
                    <span class="text-slate-400 text-[10px]">TX:</span> {{ formatRateMB(c.netTxRateMb) }}
                  </div>
                </td>

                <!-- Block I/O -->
                <td class="py-3 px-3">
                  <div class="text-slate-700 dark:text-slate-300 whitespace-nowrap">
                    <span class="text-slate-400 text-[10px]">R:</span> {{ formatRateMB(c.blockReadRateMb) }}
                  </div>
                  <div class="text-slate-700 dark:text-slate-300 whitespace-nowrap">
                    <span class="text-slate-400 text-[10px]">W:</span> {{ formatRateMB(c.blockWriteRateMb) }}
                  </div>
                </td>

                <!-- Container ID -->
                <td class="py-3 px-4 font-mono text-slate-500 dark:text-slate-400 text-[11px]">
                  {{ c.containerHostname || (c.containerId ? c.containerId.substring(0, 12) : 'N/A') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Container Table Pagination -->
        <div class="flex flex-col sm:flex-row items-center justify-between gap-3 p-4 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] rounded-xl text-xs">
          <div class="text-slate-500 dark:text-slate-400">
            Showing <strong class="text-slate-800 dark:text-slate-200">{{ (containerTablePage - 1) * CONTAINER_TABLE_PAGE_SIZE + 1 }}</strong> to
            <strong class="text-slate-800 dark:text-slate-200">{{ Math.min(containerTablePage * CONTAINER_TABLE_PAGE_SIZE, filteredContainers.length) }}</strong> of
            <strong class="text-slate-800 dark:text-slate-200">{{ filteredContainers.length }}</strong> containers
          </div>

          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="containerTablePage--"
              :disabled="containerTablePage <= 1"
              class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
            >
              <ChevronLeft class="w-3.5 h-3.5 text-slate-400" />
              <span>Prev</span>
            </button>

            <div class="flex items-center gap-1">
              <template v-for="(p, idx) in getVisiblePages(containerTablePage, containerTableTotalPages)" :key="idx">
                <span v-if="p < 0" class="px-1 text-slate-400 select-none">...</span>
                <button
                  v-else
                  type="button"
                  @click="containerTablePage = p"
                  :class="[
                    containerTablePage === p
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
              @click="containerTablePage++"
              :disabled="containerTablePage >= containerTableTotalPages"
              class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#161c2d] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-[#1f283d] transition cursor-pointer flex items-center gap-1 text-xs font-medium"
            >
              <span>Next</span>
              <ChevronRight class="w-3.5 h-3.5 text-slate-400" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 1: VIEW HISTORY (Interactive Line Charts with ECharts)         -->
    <!-- ===================================================================== -->
    <div
      v-if="showHistoryModal && historyInstance"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-4xl shadow-2xl p-6 space-y-4">
        <!-- Modal Header -->
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <div>
            <h3 class="text-base font-bold text-slate-900 dark:text-white">
              Telemetry History: {{ historyInstance.name }}
            </h3>
            <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              Host: {{ historyInstance.host }} | Group: {{ historyInstance.groupName }}
            </p>
          </div>
          <button
            @click="closeHistoryModal"
            class="p-1 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-[#192236] transition cursor-pointer"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Controls: Time Range & Metric Selector -->
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <!-- Metric Tabs -->
          <div class="flex items-center gap-1 bg-slate-100 dark:bg-[#161c2d] p-1 rounded-lg">
            <button
              @click="historyActiveMetric = 'cpu'"
              :class="historyActiveMetric === 'cpu' ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-xs font-semibold' : 'text-slate-500 dark:text-slate-400'"
              class="px-3 py-1 text-xs rounded-md transition cursor-pointer"
            >
              CPU Usage
            </button>
            <button
              @click="historyActiveMetric = 'memory'"
              :class="historyActiveMetric === 'memory' ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-xs font-semibold' : 'text-slate-500 dark:text-slate-400'"
              class="px-3 py-1 text-xs rounded-md transition cursor-pointer"
            >
              Memory Usage
            </button>
            <button
              @click="historyActiveMetric = 'disk'"
              :class="historyActiveMetric === 'disk' ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-xs font-semibold' : 'text-slate-500 dark:text-slate-400'"
              class="px-3 py-1 text-xs rounded-md transition cursor-pointer"
            >
              Disk Usage
            </button>
            <button
              @click="historyActiveMetric = 'network'"
              :class="historyActiveMetric === 'network' ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-xs font-semibold' : 'text-slate-500 dark:text-slate-400'"
              class="px-3 py-1 text-xs rounded-md transition cursor-pointer"
            >
              Network (MB/s)
            </button>
          </div>

          <!-- Time Range Selector -->
          <div class="flex items-center gap-1 bg-slate-100 dark:bg-[#161c2d] p-1 rounded-lg self-end sm:self-auto">
            <button
              v-for="r in (['1h', '6h', '24h', '7d'] as const)"
              :key="r"
              @click="historyTimeRange = r"
              :class="historyTimeRange === r ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-xs font-semibold' : 'text-slate-500 dark:text-slate-400'"
              class="px-2.5 py-1 text-xs rounded-md transition cursor-pointer"
            >
              {{ r }}
            </button>
          </div>
        </div>

        <!-- ECharts Canvas Container -->
        <div class="relative bg-slate-50 dark:bg-[#0a0f1d] border border-slate-200 dark:border-[#1b2234] rounded-xl p-3 min-h-[340px] flex items-center justify-center">
          <div v-show="loadingHistory" class="absolute inset-0 flex items-center justify-center bg-white/60 dark:bg-black/60 z-10 rounded-xl">
            <RefreshCw class="w-6 h-6 animate-spin text-blue-500" />
          </div>
          <div ref="chartDomRef" class="w-full h-[320px]"></div>
        </div>

        <div class="flex items-center justify-between text-[11px] text-slate-400">
          <span>Prometheus Target: <code>{{ historyInstance.prometheusTarget || historyInstance.host }}</code></span>
          <button
            @click="closeHistoryModal"
            class="px-4 py-1.5 bg-slate-200 dark:bg-[#1a2337] hover:bg-slate-300 dark:hover:bg-[#222d46] text-slate-800 dark:text-slate-200 rounded-lg text-xs font-semibold transition cursor-pointer"
          >
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 2: SYNC FROM REMOTE HOSTS                                       -->
    <!-- ===================================================================== -->
    <div
      v-if="showSyncModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">Sync Remote Hosts</h3>
            <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              Import configured servers from Remote Host / Remote Server into Monitoring Instances.
            </p>
          </div>
          <button @click="showSyncModal = false" class="text-slate-400 hover:text-slate-200 cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div v-if="loadingRemoteHosts" class="py-8 text-center">
          <RefreshCw class="w-5 h-5 animate-spin mx-auto text-blue-500" />
          <p class="text-xs text-slate-400 mt-2">Loading remote hosts...</p>
        </div>

        <div v-else-if="remoteHostsList.length === 0" class="py-6 text-center text-xs text-slate-400">
          No remote hosts found to sync.
        </div>

        <div v-else class="max-h-60 overflow-y-auto space-y-2 pr-1">
          <div class="flex items-center justify-between text-xs text-slate-500 pb-1">
            <span>Select Hosts to Sync:</span>
            <button
              @click="selectedSyncHosts = selectedSyncHosts.length === remoteHostsList.length ? [] : remoteHostsList.map(h => h.id)"
              class="text-blue-600 dark:text-[#95CCDD] hover:underline cursor-pointer"
            >
              {{ selectedSyncHosts.length === remoteHostsList.length ? 'Deselect All' : 'Select All' }}
            </button>
          </div>

          <label
            v-for="h in remoteHostsList"
            :key="h.id"
            class="flex items-center gap-3 p-2.5 rounded-lg border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#161c2d] cursor-pointer transition text-xs"
          >
            <input
              type="checkbox"
              :value="h.id"
              v-model="selectedSyncHosts"
              class="rounded text-blue-600 focus:ring-0 cursor-pointer"
            />
            <div class="flex-1 min-w-0">
              <div class="font-bold text-slate-900 dark:text-white truncate">{{ h.name }}</div>
              <div class="text-[11px] text-slate-400 font-mono">{{ h.host }}:{{ h.port }}</div>
            </div>
            <span v-if="h.groupName" class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 dark:bg-[#192236] text-slate-500">
              {{ h.groupName }}
            </span>
          </label>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showSyncModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeSync"
            :disabled="syncing || selectedSyncHosts.length === 0"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ syncing ? 'Syncing...' : `Sync ${selectedSyncHosts.length} Host(s)` }}
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 3: ADD / EDIT MANUAL HOST                                       -->
    <!-- ===================================================================== -->
    <div
      v-if="showInstanceModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            {{ isEditing ? 'Edit Monitoring Instance' : 'Add New Host' }}
          </h3>
          <button @click="showInstanceModal = false" class="text-slate-400 hover:text-slate-200 cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
          <!-- Name -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">Instance Name *</label>
            <input
              v-model="instanceForm.name"
              type="text"
              placeholder="e.g. debian-prod-01"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Type -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">Instance Type</label>
            <select
              v-model="instanceForm.instanceType"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            >
              <option value="server">Linux / Bare-metal Server</option>
              <option value="vm">Virtual Machine (VM)</option>
              <option value="docker">Docker Container</option>
            </select>
          </div>

          <!-- Hostname -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">Hostname</label>
            <input
              v-model="instanceForm.hostname"
              type="text"
              placeholder="e.g. yggdrasil, agent-node"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-mono text-xs"
            />
          </div>

          <!-- IP Address -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">IP Address *</label>
            <input
              v-model="instanceForm.ipAddress"
              type="text"
              placeholder="e.g. 10.20.3.29"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-mono text-xs"
            />
          </div>

          <!-- Port -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">OTel Exporter Port</label>
            <input
              v-model.number="instanceForm.port"
              type="number"
              placeholder="8889"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-mono text-xs"
            />
          </div>

          <!-- Group -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">Group</label>
            <input
              v-model="instanceForm.groupName"
              type="text"
              placeholder="Default"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Tags -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">Tags (comma-separated)</label>
            <input
              v-model="instanceForm.tags"
              type="text"
              placeholder="prod, node, dc-1"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Prometheus Target Override -->
          <div class="sm:col-span-2">
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">
              Prometheus Target Override (Optional)
            </label>
            <input
              v-model="instanceForm.prometheusTarget"
              type="text"
              placeholder="e.g. 10.20.3.36:8889 (leave blank for automatic matching)"
              class="w-full px-3 py-2 bg-white dark:bg-[#0c101c] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            />
            <span class="text-[10px] text-slate-400 mt-1 block">
              Leave blank to automatically match metrics by IP:Port or Hostname from Prometheus.
            </span>
          </div>

          <!-- Visibility & Alert -->
          <div class="flex items-center gap-4 sm:col-span-2 pt-1">
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                value="public"
                v-model="instanceForm.visibility"
                class="text-blue-600 focus:ring-0"
              />
              <span class="text-slate-800 dark:text-slate-200">Public (All Users)</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                value="private"
                v-model="instanceForm.visibility"
                class="text-blue-600 focus:ring-0"
              />
              <span class="text-slate-800 dark:text-slate-200">Private (Owner & Shared only)</span>
            </label>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showInstanceModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="saveInstance"
            :disabled="savingInstance"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ savingInstance ? 'Saving...' : isEditing ? 'Save Changes' : 'Create Instance' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 4: GRANULAR SHARING (RBAC)                                      -->
    <!-- ===================================================================== -->
    <div
      v-if="showShareModal && sharingInstance"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">
              Share Instance: {{ sharingInstance.name }}
            </h3>
            <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              Granular access control (Owner: {{ sharingInstance.ownerUsername || 'Admin' }})
            </p>
          </div>
          <button @click="showShareModal = false" class="text-slate-400 hover:text-slate-200 cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Add Share Row -->
        <div class="p-3 bg-slate-50 dark:bg-[#0c101c] rounded-xl border border-slate-200/80 dark:border-[#1b2234] space-y-2">
          <div class="text-xs font-semibold text-slate-800 dark:text-slate-200">Share with User</div>
          <div class="flex items-center gap-2">
            <select
              v-model="selectedShareUser"
              class="flex-1 px-2.5 py-1.5 text-xs bg-white dark:bg-[#161c2d] border border-slate-200 dark:border-[#222c42] rounded-lg text-slate-800 dark:text-slate-200 focus:outline-none"
            >
              <option value="" disabled>Select User...</option>
              <option
                v-for="u in availableUsers"
                :key="u.id"
                :value="u.id"
                :disabled="instanceShares.some(s => s.userId === u.id)"
              >
                {{ u.username }} ({{ u.role }})
              </option>
            </select>

            <select
              v-model="selectedSharePerm"
              class="w-24 px-2 py-1.5 text-xs bg-white dark:bg-[#161c2d] border border-slate-200 dark:border-[#222c42] rounded-lg text-slate-800 dark:text-slate-200 focus:outline-none"
            >
              <option value="read">Read</option>
              <option value="manage">Manage</option>
            </select>

            <button
              @click="executeAddShare"
              :disabled="!selectedShareUser || sharingActionLoading"
              class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold disabled:opacity-50 cursor-pointer"
            >
              Share
            </button>
          </div>
        </div>

        <!-- Active Shares List -->
        <div class="space-y-1.5 max-h-48 overflow-y-auto">
          <div class="text-xs font-semibold text-slate-500 mb-1">Active Shares</div>
          <div v-if="instanceShares.length === 0" class="text-xs text-slate-400 py-3 text-center">
            No specific user shares assigned.
          </div>
          <div
            v-for="s in instanceShares"
            :key="s.id"
            class="flex items-center justify-between p-2.5 rounded-lg border border-slate-100 dark:border-[#1b2234] text-xs bg-white dark:bg-[#111624]"
          >
            <div>
              <div class="font-bold text-slate-900 dark:text-white">{{ s.username }}</div>
              <div class="text-[10px] text-slate-400">Permission: <strong class="text-slate-600 dark:text-slate-300 uppercase">{{ s.permission }}</strong></div>
            </div>
            <button
              @click="executeRevokeShare(s.userId)"
              class="p-1 text-rose-500 hover:text-rose-600 dark:hover:text-rose-400 rounded cursor-pointer"
              title="Revoke Share"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <div class="flex justify-end pt-2 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showShareModal = false"
            class="px-4 py-1.5 bg-slate-200 dark:bg-[#1a2337] text-slate-800 dark:text-slate-200 rounded-lg text-xs font-semibold transition cursor-pointer"
          >
            Done
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 5: STANDARD HCP DELETE CONFIRMATION POPUP (AGENTS.md)          -->
    <!-- ===================================================================== -->
    <div
      v-if="showDeleteModal && instanceToDelete"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Red circle trash icon -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Monitoring Instance?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ instanceToDelete.name }}</strong>? This action cannot be undone.
          </p>
        </div>

        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDelete"
            :disabled="deleting"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deleting ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 6: ALERT & PRIORITY THRESHOLDS CONFIGURATION                     -->
    <!-- ===================================================================== -->
    <div
      v-if="showThresholdModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-6 space-y-4 font-sans">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">
              Alert & Priority Thresholds
            </h3>
            <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              Instances exceeding these levels automatically jump to the top.
            </p>
          </div>
          <button
            @click="showThresholdModal = false"
            class="p-1 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-[#192236] transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Form Cards -->
        <div class="space-y-3.5 text-xs">
          <!-- Warning Threshold Input -->
          <div class="p-3.5 rounded-xl bg-amber-50/50 dark:bg-amber-950/20 border border-amber-200 dark:border-amber-900/50 space-y-2.5">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="w-2.5 h-2.5 rounded-full bg-amber-500"></span>
                <label class="font-bold text-slate-800 dark:text-slate-200">
                  Warning Threshold
                </label>
              </div>
              <div class="flex items-center gap-1">
                <input
                  v-model.number="editWarningThreshold"
                  type="number"
                  min="30"
                  max="95"
                  class="w-16 px-2 py-1 text-xs text-right font-mono font-bold bg-white dark:bg-[#161c2d] border border-amber-300 dark:border-amber-800 rounded-lg text-amber-700 dark:text-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-500"
                />
                <span class="font-bold text-slate-500">%</span>
              </div>
            </div>
            <input
              v-model.number="editWarningThreshold"
              type="range"
              min="30"
              max="95"
              step="1"
              class="w-full accent-amber-500 cursor-pointer"
            />
            <p class="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
              Default 80%. When CPU, Memory, or any Disk exceeds this level, the instance is elevated ahead of normal hosts.
            </p>
          </div>

          <!-- Critical Threshold Input -->
          <div class="p-3.5 rounded-xl bg-rose-50/50 dark:bg-rose-950/20 border border-rose-200 dark:border-rose-900/50 space-y-2.5">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="w-2.5 h-2.5 rounded-full bg-rose-500"></span>
                <label class="font-bold text-slate-800 dark:text-slate-200">
                  Critical Threshold
                </label>
              </div>
              <div class="flex items-center gap-1">
                <input
                  v-model.number="editCriticalThreshold"
                  type="number"
                  min="40"
                  max="100"
                  class="w-16 px-2 py-1 text-xs text-right font-mono font-bold bg-white dark:bg-[#161c2d] border border-rose-300 dark:border-rose-800 rounded-lg text-rose-700 dark:text-rose-400 focus:outline-none focus:ring-1 focus:ring-rose-500"
                />
                <span class="font-bold text-slate-500">%</span>
              </div>
            </div>
            <input
              v-model.number="editCriticalThreshold"
              type="range"
              min="40"
              max="100"
              step="1"
              class="w-full accent-rose-500 cursor-pointer"
            />
            <p class="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
              Default 90%. When CPU, Memory, or any Disk exceeds this level, the instance gets highest priority and is pinned to the top of Page 1.
            </p>
          </div>

          <!-- Quick Presets -->
          <div class="flex items-center justify-between pt-1">
            <span class="text-[11px] font-medium text-slate-400">Quick Presets:</span>
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                @click="applyPreset(80, 90)"
                class="px-2 py-1 rounded text-[11px] font-medium bg-slate-100 hover:bg-slate-200 dark:bg-[#161c2d] dark:hover:bg-[#1f283d] text-slate-700 dark:text-slate-300 transition cursor-pointer"
              >
                Default (80/90)
              </button>
              <button
                type="button"
                @click="applyPreset(70, 85)"
                class="px-2 py-1 rounded text-[11px] font-medium bg-slate-100 hover:bg-slate-200 dark:bg-[#161c2d] dark:hover:bg-[#1f283d] text-slate-700 dark:text-slate-300 transition cursor-pointer"
              >
                Strict (70/85)
              </button>
              <button
                type="button"
                @click="applyPreset(85, 95)"
                class="px-2 py-1 rounded text-[11px] font-medium bg-slate-100 hover:bg-slate-200 dark:bg-[#161c2d] dark:hover:bg-[#1f283d] text-slate-700 dark:text-slate-300 transition cursor-pointer"
              >
                Relaxed (85/95)
              </button>
            </div>
          </div>

          <!-- Live Inventory Impact Preview -->
          <div class="p-3 bg-slate-50 dark:bg-[#0d121f] rounded-xl border border-slate-200 dark:border-[#1b2234]">
            <div class="font-medium text-slate-600 dark:text-slate-400 mb-2">Live Inventory Impact:</div>
            <div class="grid grid-cols-3 gap-2 text-center font-mono">
              <div class="p-2 rounded-lg bg-rose-50 dark:bg-rose-950/30 border border-rose-200 dark:border-rose-900/40">
                <div class="font-bold text-sm text-rose-600 dark:text-rose-400">{{ previewModalCounts.critical }}</div>
                <div class="text-[10px] text-slate-500 dark:text-slate-400 font-sans">Critical</div>
              </div>
              <div class="p-2 rounded-lg bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-900/40">
                <div class="font-bold text-sm text-amber-600 dark:text-amber-400">{{ previewModalCounts.warning }}</div>
                <div class="text-[10px] text-slate-500 dark:text-slate-400 font-sans">Warning</div>
              </div>
              <div class="p-2 rounded-lg bg-emerald-50 dark:bg-emerald-950/30 border border-emerald-200 dark:border-emerald-900/40">
                <div class="font-bold text-sm text-emerald-600 dark:text-emerald-400">{{ previewModalCounts.normal }}</div>
                <div class="text-[10px] text-slate-500 dark:text-slate-400 font-sans">Normal</div>
              </div>
            </div>
          </div>
        </div>

        <!-- Modal Actions -->
        <div class="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            type="button"
            @click="resetThresholdsToDefault"
            class="px-3 py-1.5 text-xs text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white transition cursor-pointer"
          >
            Reset Default
          </button>
          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="showThresholdModal = false"
              class="px-3.5 py-1.5 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-[#1a2337] rounded-lg transition cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="button"
              @click="saveThresholds"
              class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shadow-xs cursor-pointer"
            >
              Save & Apply
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
