<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import {
  RotateCw,
  Settings,
  ArrowLeft,
  Clock,
  Search,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Database,
  Server,
  Layers,
  Activity,
  HardDrive,
  Cpu,
  Eye,
  Sliders,
  Terminal,
  ExternalLink,
  AlertCircle,
  X,
  Zap,
} from 'lucide-vue-next';

const router = useRouter();

// Active Navigation Tab
const activeTab = ref<'overview' | 'nodes' | 'indices' | 'shards' | 'connection' | 'logs'>('overview');

// Auto-Refresh state
const refreshIntervalSec = ref(30);
const countdown = ref(30);
const isRefreshing = ref(false);
const timer = ref<any>(null);
const isSettingsModalOpen = ref(false);

// Data states
const error = ref('');
const clusterHealth = ref<any | null>(null);
const nodesList = ref<any[]>([]);
const indicesList = ref<any[]>([]);
const shardsList = ref<any[]>([]);
const logsList = ref<any[]>([]);

// Selected Index Detail Modal
const selectedIndexModal = ref<any | null>(null);

// Connection Config State
const configForm = ref({
  id: '',
  name: '',
  host: '',
  port: 9200,
  username: '',
  password: '',
  useSsl: false,
  verifySsl: false,
  isActive: true,
});
const testResult = ref<string | null>(null);
const testSuccess = ref(false);
const isSavingConfig = ref(false);

// Filter & Search states
const indexSearch = ref('');
const shardSearch = ref('');

// Hover Tooltip State for Shards (Matching Screenshot)
const hoveredShard = ref<any | null>(null);
const tooltipPosition = ref({ x: 0, y: 0 });

// Formatted Overview Computations
const totalIndicesCount = computed(() => indicesList.value.length);
const totalDocumentsCount = computed(() => {
  return indicesList.value.reduce((acc, idx) => acc + (parseInt(idx['docs.count'] || idx.docsCount || 0) || 0), 0);
});

function parseBytesString(val: any): number {
  if (typeof val === 'number') return val;
  if (!val || typeof val !== 'string') return 0;
  const s = val.trim().toLowerCase();
  const num = parseFloat(s);
  if (isNaN(num)) return 0;
  if (s.endsWith('tb') || s.endsWith('t')) return num * 1024 * 1024 * 1024 * 1024;
  if (s.endsWith('gb') || s.endsWith('g')) return num * 1024 * 1024 * 1024;
  if (s.endsWith('mb') || s.endsWith('m')) return num * 1024 * 1024;
  if (s.endsWith('kb') || s.endsWith('k')) return num * 1024;
  if (s.endsWith('b')) return num;
  return num;
}

const totalStoreSizeBytes = computed(() => {
  if (indicesList.value.length > 0) {
    const totalBytes = indicesList.value.reduce((acc, idx) => {
      const raw = idx['store.size'] || idx.storeSize || idx['pri.store.size'] || 0;
      return acc + parseBytesString(raw);
    }, 0);
    if (totalBytes > 0) {
      return formatBytes(totalBytes);
    }
  }
  if (nodesList.value.length > 0) {
    const fsUsed = nodesList.value.reduce((acc, n) => acc + (n.diskUsedBytes || n.fsUsedBytes || 0), 0);
    if (fsUsed > 0) return formatBytes(fsUsed);
  }
  return '0 B';
});

// All Distinct Node Names (from nodes stats or discovered shards)
const clusterNodes = computed(() => {
  const map: Record<string, { name: string; ip: string; primaryCount: number; replicaCount: number }> = {};
  
  nodesList.value.forEach(n => {
    map[n.name] = {
      name: n.name,
      ip: n.ip || '-',
      primaryCount: 0,
      replicaCount: 0,
    };
  });

  shardsList.value.forEach(s => {
    const nodeName = s.node || s.nodeName || 'unassigned';
    if (!map[nodeName]) {
      map[nodeName] = {
        name: nodeName,
        ip: s.ip || '-',
        primaryCount: 0,
        replicaCount: 0,
      };
    }
    if (s.prirep === 'p' || s.type === 'Primary') {
      map[nodeName].primaryCount++;
    } else if (s.prirep === 'r' || s.type === 'Replica') {
      map[nodeName].replicaCount++;
    }
  });

  return Object.values(map);
});

// Filtered indices
const filteredIndices = computed(() => {
  if (!indexSearch.value) return indicesList.value;
  const q = indexSearch.value.toLowerCase();
  return indicesList.value.filter(idx => (idx.index || idx.name || '').toLowerCase().includes(q));
});

// Filtered shards for All Shards table
const filteredShards = computed(() => {
  if (!shardSearch.value) return shardsList.value;
  const q = shardSearch.value.toLowerCase();
  return shardsList.value.filter(
    s => (s.index || '').toLowerCase().includes(q) || (s.node || '').toLowerCase().includes(q) || (s.ip || '').toLowerCase().includes(q)
  );
});

// Grouped shards per node
const shardsByNode = computed(() => {
  const map: Record<string, any[]> = {};
  clusterNodes.value.forEach(n => {
    map[n.name] = [];
  });
  if (clusterNodes.value.length === 0) {
    map['cluster-node'] = [];
  }

  shardsList.value.forEach(shard => {
    const nodeName = shard.node || shard.nodeName || (clusterNodes.value[0]?.name || 'cluster-node');
    if (!map[nodeName]) map[nodeName] = [];
    map[nodeName].push(shard);
  });

  return map;
});

// Shards belonging to the modal-selected index
const selectedIndexShards = computed(() => {
  if (!selectedIndexModal.value) return [];
  const idxName = selectedIndexModal.value.index || selectedIndexModal.value.name;
  return shardsList.value.filter(s => s.index === idxName);
});

function formatBytes(bytes: number, decimals = 1) {
  if (!+bytes) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

function formatNumber(num: any) {
  const n = parseInt(num) || 0;
  return new Intl.NumberFormat('en-US').format(n);
}

// Fetch all cluster telemetry from actual backend
const fetchClusterData = async () => {
  isRefreshing.value = true;
  error.value = '';
  try {
    const [healthRes, nodesStatsRes, nodesInfoRes, indicesRes, shardsRes, logsRes] = await Promise.allSettled([
      axios.get('/api/v1/opensearch/health'),
      axios.get('/api/v1/opensearch/nodes'),
      axios.get('/api/v1/opensearch/nodes/info'),
      axios.get('/api/v1/opensearch/indices'),
      axios.get('/api/v1/opensearch/shards'),
      axios.get('/api/v1/logs'),
    ]);

    if (healthRes.status === 'fulfilled' && healthRes.value.data.success) {
      clusterHealth.value = healthRes.value.data.data;
    } else {
      clusterHealth.value = null;
    }

    if (indicesRes.status === 'fulfilled' && indicesRes.value.data.success && Array.isArray(indicesRes.value.data.data)) {
      indicesList.value = indicesRes.value.data.data;
    } else {
      indicesList.value = [];
    }

    if (shardsRes.status === 'fulfilled' && shardsRes.value.data.success && Array.isArray(shardsRes.value.data.data)) {
      shardsList.value = shardsRes.value.data.data;
    } else {
      shardsList.value = [];
    }

    if (nodesStatsRes.status === 'fulfilled' && nodesStatsRes.value.data.success) {
      parseNodesData(nodesStatsRes.value.data.data, nodesInfoRes.status === 'fulfilled' ? nodesInfoRes.value.data.data : null);
    } else {
      nodesList.value = [];
    }

    if (logsRes.status === 'fulfilled' && logsRes.value.data.success && Array.isArray(logsRes.value.data.data)) {
      logsList.value = logsRes.value.data.data.filter((l: any) => l.source === 'OpenSearch' || l.category === 'OpenSearch' || (l.message && l.message.includes('OpenSearch')));
    }
  } catch (err: any) {
    console.error('Error fetching OpenSearch data:', err);
  } finally {
    isRefreshing.value = false;
    countdown.value = refreshIntervalSec.value > 0 ? refreshIntervalSec.value : 0;
  }
};

const parseNodesData = (stats: any, info: any) => {
  if (!stats?.nodes) {
    nodesList.value = [];
    return;
  }
  const nodesArr: any[] = [];
  const nodesMap = stats.nodes;
  const infoMap = info?.nodes || {};

  Object.keys(nodesMap).forEach(nodeId => {
    const n = nodesMap[nodeId];
    const inf = infoMap[nodeId] || {};
    const os = n.os || {};
    const jvm = n.jvm || {};
    const fs = n.fs?.total || {};

    const jvmHeapUsed = jvm.mem?.heap_used_in_bytes || 0;
    const jvmHeapMax = jvm.mem?.heap_max_in_bytes || 1;
    const heapPct = Math.round((jvmHeapUsed / jvmHeapMax) * 100);

    const diskTotal = fs.total_in_bytes || 1;
    const diskAvailable = fs.available_in_bytes || 0;
    const diskUsed = diskTotal - diskAvailable;
    const diskPct = ((diskUsed / diskTotal) * 100).toFixed(2);

    const load1 = os.cpu?.load_average?.['1m'] ?? '0.00';
    const load5 = os.cpu?.load_average?.['5m'] ?? '0.00';
    const load15 = os.cpu?.load_average?.['15m'] ?? '0.00';

    const nodeShards = shardsList.value.filter(s => (s.node === n.name || s.nodeName === n.name));
    const priCount = nodeShards.filter(s => s.prirep === 'p' || s.type === 'Primary').length;
    const repCount = nodeShards.filter(s => s.prirep === 'r' || s.type === 'Replica').length;

    nodesArr.push({
      name: n.name || nodeId,
      ip: n.ip || inf.ip || '-',
      cpu: `${os.cpu?.percent ?? 0}%`,
      load: `${load1} / ${load5} / ${load15}`,
      heapPercent: `${heapPct}%`,
      uptime: formatUptime(jvm.uptime_in_millis || 0),
      ram: `${os.mem?.used_percent ?? 0}%`,
      jvmHeap: `${formatBytes(jvmHeapUsed)} / ${formatBytes(jvmHeapMax)}`,
      diskPercent: `${diskPct}%`,
      diskUsage: `${formatBytes(diskUsed)} / ${formatBytes(diskTotal)}`,
      primaryShards: priCount,
      replicaShards: repCount,
    });
  });

  nodesList.value = nodesArr;
};

function formatUptime(ms: number) {
  if (!ms || ms <= 0) return '0h';
  const days = Math.floor(ms / (1000 * 60 * 60 * 24));
  const hours = Math.floor((ms % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  return `${days}d ${hours}h`;
}

// Hover Tooltip Handlers for Shard Blocks (Matching Screenshot)
const showTooltip = (event: MouseEvent, shard: any) => {
  hoveredShard.value = shard;
  const target = event.currentTarget as HTMLElement;
  const rect = target.getBoundingClientRect();
  tooltipPosition.value = {
    x: Math.min(Math.max(rect.left - 100, 10), window.innerWidth - 340),
    y: rect.bottom + 10,
  };
};

const hideTooltip = () => {
  hoveredShard.value = null;
};

// Check if a shard belongs to the currently hovered index (Auto-detect Primary & Replica pairing)
const isShardHighlighted = (shard: any) => {
  if (!hoveredShard.value) return false;
  const targetIndex = hoveredShard.value.index || hoveredShard.value.name;
  const currentIndex = shard.index || shard.name;
  return Boolean(targetIndex && currentIndex && targetIndex === currentIndex);
};

// Check if a shard should be dimmed because a different index is hovered
const isShardDimmed = (shard: any) => {
  if (!hoveredShard.value) return false;
  const targetIndex = hoveredShard.value.index || hoveredShard.value.name;
  const currentIndex = shard.index || shard.name;
  return Boolean(targetIndex && currentIndex && targetIndex !== currentIndex);
};

// Configuration Methods
const fetchActiveConfig = async () => {
  try {
    const res = await axios.get('/api/v1/opensearch/config');
    if (res.data.success && res.data.data) {
      const c = res.data.data;
      configForm.value = {
        id: c.id || '',
        name: c.name || '',
        host: c.host || '',
        port: c.port || 9200,
        username: c.username || '',
        password: '',
        useSsl: c.useSsl || false,
        verifySsl: c.verifySsl || false,
        isActive: c.isActive !== false,
      };
    }
  } catch (err) {
    console.error('Failed to load OpenSearch config:', err);
  }
};

const handleTestConnection = async () => {
  testResult.value = 'Testing connection...';
  testSuccess.value = false;
  try {
    const res = await axios.post('/api/v1/opensearch/test', {
      host: configForm.value.host,
      port: configForm.value.port,
      username: configForm.value.username,
      password: configForm.value.password,
      useSsl: configForm.value.useSsl,
    });
    if (res.data.success) {
      testSuccess.value = true;
      testResult.value = `Success: Cluster "${res.data.data?.cluster_name || 'OpenSearch'}" status is ${res.data.data?.status || 'green'}.`;
    }
  } catch (err: any) {
    testSuccess.value = false;
    testResult.value = `Failed: ${err.response?.data?.error || err.message}`;
  }
};

const handleSaveConfig = async () => {
  isSavingConfig.value = true;
  try {
    const res = await axios.post('/api/v1/opensearch/config', configForm.value);
    if (res.data.success) {
      alert('OpenSearch configuration saved successfully!');
      activeTab.value = 'overview';
      fetchClusterData();
    }
  } catch (err: any) {
    alert(`Failed to save config: ${err.response?.data?.error || err.message}`);
  } finally {
    isSavingConfig.value = false;
  }
};

// Timer Management
const startTimer = () => {
  if (timer.value) clearInterval(timer.value);
  if (refreshIntervalSec.value <= 0) return;

  timer.value = setInterval(() => {
    if (countdown.value > 1) {
      countdown.value--;
    } else {
      fetchClusterData();
    }
  }, 1000);
};

const setRefreshInterval = (sec: number) => {
  refreshIntervalSec.value = sec;
  countdown.value = sec;
  localStorage.setItem('opensearch_refresh_sec', sec.toString());
  isSettingsModalOpen.value = false;
  startTimer();
};

const handleBackToPortal = () => {
  if (window.opener) {
    window.close();
  } else {
    router.push('/');
  }
};

onMounted(() => {
  const savedSec = localStorage.getItem('opensearch_refresh_sec');
  if (savedSec !== null) {
    refreshIntervalSec.value = parseInt(savedSec);
    countdown.value = refreshIntervalSec.value;
  }

  fetchActiveConfig();
  fetchClusterData();
  startTimer();
});

onUnmounted(() => {
  if (timer.value) clearInterval(timer.value);
});
</script>

<template>
  <div class="min-h-screen w-full bg-[#14161b] text-slate-200 font-sans flex flex-col selection:bg-brand-500/30">
    <!-- Top Header Bar (Sticky at top) -->
    <header class="h-12 bg-[#1b1e26] border-b border-slate-800 px-4 flex items-center justify-between shrink-0 sticky top-0 z-20 backdrop-blur-md">
      <!-- Left: Title and Status -->
      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2">
          <Database class="w-4 h-4 text-brand-400" />
          <h1 class="text-xs font-semibold text-white tracking-wide">OpenSearch Cluster Monitor</h1>
        </div>

        <!-- Connection Status Pill -->
        <div
          v-if="clusterHealth"
          class="flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-mono border"
          :class="clusterHealth.status === 'green' ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : clusterHealth.status === 'yellow' ? 'bg-amber-500/10 border-amber-500/30 text-amber-400' : 'bg-red-500/10 border-red-500/30 text-red-400'"
        >
          <span class="w-1.5 h-1.5 rounded-full" :class="clusterHealth.status === 'green' ? 'bg-emerald-400 animate-pulse' : clusterHealth.status === 'yellow' ? 'bg-amber-400' : 'bg-red-400'"></span>
          <span class="uppercase font-bold">{{ clusterHealth.status }}</span>
        </div>
      </div>

      <!-- Right Actions: Refresh, Countdown, Gear, Back to Portal -->
      <div class="flex items-center gap-3">
        <!-- Countdown indicator -->
        <div v-if="refreshIntervalSec > 0" class="flex items-center gap-1.5 text-xs text-slate-400 font-mono">
          <Clock class="w-3.5 h-3.5 text-slate-500" />
          <span>{{ countdown }}s</span>
        </div>

        <!-- Refresh Button -->
        <button
          @click="fetchClusterData"
          class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
          title="Refresh Data Now"
        >
          <RotateCw :class="['w-4 h-4', isRefreshing ? 'animate-spin text-brand-400' : '']" />
        </button>

        <!-- Settings Gear Modal Button -->
        <button
          @click="isSettingsModalOpen = true"
          class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
          title="Auto-Refresh Settings"
        >
          <Settings class="w-4 h-4" />
        </button>

        <!-- Back to Portal Button -->
        <button
          @click="handleBackToPortal"
          class="flex items-center gap-1.5 px-3 py-1 rounded bg-[#242833] border border-slate-700/60 text-xs text-slate-300 hover:text-white hover:border-slate-500 transition font-medium"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Back to Portal</span>
        </button>
      </div>
    </header>

    <!-- Navigation Tabs Bar (Sticky below header) -->
    <div class="bg-[#1b1e26]/95 border-b border-slate-800/80 px-6 flex items-center gap-8 text-xs shrink-0 sticky top-12 z-10 backdrop-blur-md">
      <button
        v-for="tab in [
          { id: 'overview', label: 'Overview' },
          { id: 'nodes', label: 'Nodes' },
          { id: 'indices', label: 'Indices' },
          { id: 'shards', label: 'Shards & Allocation' },
          { id: 'connection', label: 'Connection' },
          { id: 'logs', label: 'Logs' }
        ]"
        :key="tab.id"
        @click="activeTab = tab.id as any"
        :class="[
          'py-3 transition border-b-2 -mb-[1px]',
          activeTab === tab.id
            ? 'border-brand-500 text-brand-400 font-semibold'
            : 'border-transparent text-slate-400 hover:text-slate-200'
        ]"
      >
        {{ tab.label }}
      </button>
    </div>

    <!-- Main Content Container (Uses natural browser window scrolling) -->
    <main class="flex-1 p-6 space-y-6 max-w-[1600px] w-full mx-auto">
      
      <!-- ================================================================= -->
      <!-- TAB 1: OVERVIEW -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'overview'" class="space-y-6">
        <!-- Not Connected Notice -->
        <div v-if="!clusterHealth" class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl text-center space-y-3">
          <div class="w-10 h-10 rounded-full bg-amber-500/10 border border-amber-500/20 text-amber-400 flex items-center justify-center mx-auto">
            <AlertCircle class="w-5 h-5" />
          </div>
          <h3 class="text-sm font-bold text-white">No OpenSearch Cluster Connected</h3>
          <p class="text-xs text-slate-400 max-w-md mx-auto">
            Configure your OpenSearch endpoint host and credentials in the Connection tab to start monitoring cluster telemetry.
          </p>
          <button
            @click="activeTab = 'connection'"
            class="px-4 py-2 bg-brand-500 hover:bg-brand-600 text-white text-xs font-medium rounded-lg shadow-lg shadow-brand-500/20 transition"
          >
            Configure Connection
          </button>
        </div>

        <!-- Metric Cards Row (4 cards) -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
          <!-- Card 1: Cluster Health -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl relative overflow-hidden flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Cluster Health</span>
            <div class="text-2xl font-black tracking-tight uppercase" :class="clusterHealth?.status === 'green' ? 'text-emerald-400' : clusterHealth?.status === 'yellow' ? 'text-amber-400' : clusterHealth?.status === 'red' ? 'text-red-400' : 'text-slate-500'">
              {{ clusterHealth?.status || 'NOT CONNECTED' }}
            </div>
            <div
              class="w-full h-1 rounded-full shadow-sm"
              :class="clusterHealth?.status === 'green' ? 'bg-emerald-500 shadow-emerald-500/50' : clusterHealth?.status === 'yellow' ? 'bg-amber-500' : clusterHealth?.status === 'red' ? 'bg-red-500' : 'bg-slate-700'"
            ></div>
          </div>

          <!-- Card 2: Total Indices -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Total Indices</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ formatNumber(totalIndicesCount) }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Managed across cluster</span>
          </div>

          <!-- Card 3: Total Documents -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Total Documents</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ formatNumber(totalDocumentsCount) }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Indexed searchable docs</span>
          </div>

          <!-- Card 4: Store Size -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Store Size</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ totalStoreSizeBytes }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Total allocated storage</span>
          </div>
        </div>

        <!-- Middle Section: Index & Shards + Node Stats -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Left: Index & Shards Breakdown -->
          <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6 shadow-xl">
            <h3 class="text-xs font-bold text-white tracking-wide">Index & Shards Summary</h3>
            <div class="grid grid-cols-2 gap-y-6 text-xs font-sans">
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Primary Shards</p>
                <p class="text-xl font-bold text-emerald-400 font-mono">{{ clusterHealth?.active_primary_shards ?? 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Replica Shards</p>
                <p class="text-xl font-bold text-sky-400 font-mono">{{ clusterHealth ? (clusterHealth.active_shards - clusterHealth.active_primary_shards) : 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Total Shards</p>
                <p class="text-xl font-bold text-white font-mono">{{ clusterHealth?.active_shards ?? 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Unassigned Shards</p>
                <p class="text-xl font-bold font-mono" :class="clusterHealth?.unassigned_shards > 0 ? 'text-amber-400' : 'text-slate-300'">
                  {{ clusterHealth?.unassigned_shards ?? 0 }}
                </p>
              </div>
            </div>
          </div>

          <!-- Right: Cluster Nodes Status -->
          <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-4 shadow-xl">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold text-white tracking-wide">Cluster Nodes ({{ clusterNodes.length }})</h3>
              <span class="text-xs font-mono text-slate-400">{{ clusterHealth?.cluster_name || 'OpenSearch' }}</span>
            </div>

            <div v-if="clusterNodes.length > 0" class="space-y-2">
              <div
                v-for="node in clusterNodes"
                :key="node.name"
                class="p-3 bg-[#14161b] border border-slate-800 rounded-lg flex items-center justify-between text-xs"
              >
                <div class="flex items-center gap-2.5">
                  <Server class="w-4 h-4 text-brand-400" />
                  <div>
                    <p class="font-bold text-slate-200">{{ node.name }}</p>
                    <p class="text-[10px] text-slate-500 font-mono">{{ node.ip }}</p>
                  </div>
                </div>
                <div class="flex items-center gap-3 font-mono text-[11px]">
                  <span class="text-emerald-400 font-semibold">P: {{ node.primaryCount }}</span>
                  <span class="text-sky-400 font-semibold">R: {{ node.replicaCount }}</span>
                </div>
              </div>
            </div>
            <p v-else class="text-xs text-slate-500 italic py-4 text-center">No nodes registered.</p>
          </div>
        </div>

        <!-- ============================================================= -->
        <!-- SHARD ALLOCATION BY NODE (MATCHING USER SCREENSHOT) -->
        <!-- ============================================================= -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl p-6 space-y-6 shadow-xl">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold text-white tracking-wide uppercase">Shard Allocation by Node</h3>
            <!-- Mini Legend -->
            <div class="flex items-center gap-4 text-xs font-mono">
              <div class="flex items-center gap-1.5">
                <span class="w-3 h-3 rounded bg-[#0f3d28] border border-[#1c6b47] flex items-center justify-center text-[8px] font-bold text-[#4ade80]">P</span>
                <span class="text-slate-400">Primary</span>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="w-3 h-3 rounded bg-[#132c4a] border border-[#1e4976] flex items-center justify-center text-[8px] font-bold text-[#60a5fa]">R</span>
                <span class="text-slate-400">Replica</span>
              </div>
            </div>
          </div>

          <div v-if="clusterNodes.length === 0" class="text-center py-6 text-slate-500 text-xs">
            No shard allocations discovered yet.
          </div>

          <!-- Node Rows with Matrix of Shard Tiles -->
          <div v-for="node in clusterNodes" :key="node.name" class="space-y-2 border-b border-slate-800/80 pb-6 last:border-b-0 last:pb-0">
            <div class="flex items-center justify-between text-xs">
              <span class="font-bold text-white text-xs tracking-wide">{{ node.name }}</span>
              <div class="flex items-center gap-4 text-slate-400 text-[11px] font-mono">
                <span class="text-emerald-400 font-medium">Primary: {{ node.primaryCount }}</span>
                <span class="text-sky-400 font-medium">Replica: {{ node.replicaCount }}</span>
              </div>
            </div>

            <!-- Visual Shard Blocks Matrix -->
            <div class="flex flex-wrap gap-1.5 p-3.5 bg-[#13161c] rounded-xl border border-slate-800/80 min-h-[50px] items-center">
              <div
                v-for="(shard, sIdx) in (shardsByNode[node.name] || [])"
                :key="sIdx"
                @mouseenter="showTooltip($event, shard)"
                @mouseleave="hideTooltip"
                :class="[
                  'w-6 h-6 rounded flex items-center justify-center font-bold text-[10px] cursor-pointer transition-all duration-150 transform select-none',
                  // 1. Highlighted state: ALL primary and replica shards of the hovered index are highlighted with glowing white border
                  isShardHighlighted(shard)
                    ? (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#10b981] text-white border-2 border-white ring-4 ring-emerald-500/40 scale-125 z-30 shadow-2xl font-black'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#3b82f6] text-white border-2 border-white ring-4 ring-blue-500/40 scale-125 z-30 shadow-2xl font-black'
                        : 'bg-red-500 text-white border-2 border-white ring-4 ring-red-500/40 scale-125 z-30 shadow-2xl font-black')
                    // 2. Dimmed state: other unrelated indices are dimmed
                    : isShardDimmed(shard)
                    ? (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#0f3d28]/25 border border-[#1c6b47]/20 text-[#4ade80]/30 opacity-20 scale-90'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#132c4a]/25 border border-[#1e4976]/20 text-[#60a5fa]/30 opacity-20 scale-90'
                        : 'bg-[#3b1219]/25 border border-[#822735]/20 text-[#f87171]/30 opacity-20 scale-90')
                    // 3. Normal idle state
                    : (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#0f3d28] border border-[#1c6b47] text-[#4ade80] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#132c4a] border border-[#1e4976] text-[#60a5fa] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md'
                        : 'bg-[#3b1219] border border-[#822735] text-[#f87171] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md')
                ]"
              >
                {{ shard.prirep === 'p' || shard.type === 'Primary' ? 'P' : shard.prirep === 'r' || shard.type === 'Replica' ? 'R' : 'U' }}
              </div>
              <span v-if="(shardsByNode[node.name] || []).length === 0" class="text-[11px] text-slate-600 italic">
                No active shards on this node.
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 2: NODES -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'nodes'" class="space-y-4">
        <div v-if="nodesList.length === 0" class="p-8 bg-[#1b1e26] border border-slate-800/80 rounded-xl text-center space-y-2">
          <p class="text-xs text-slate-400">No node telemetry discovered. Ensure OpenSearch cluster is reachable.</p>
        </div>

        <div
          v-for="node in nodesList"
          :key="node.name"
          class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6 shadow-xl"
        >
          <!-- Node Header -->
          <div class="flex items-center justify-between border-b border-slate-800 pb-4">
            <div class="flex items-center gap-3">
              <Server class="w-5 h-5 text-brand-400" />
              <h2 class="text-sm font-bold text-white tracking-wide">{{ node.name }}</h2>
            </div>
            <div class="flex items-center gap-2 text-xs font-mono text-slate-400">
              <span>{{ node.ip }}</span>
            </div>
          </div>

          <!-- Node Metrics Grid -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-6 text-xs font-sans">
            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">CPU</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.cpu }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Load (1m/5m/15m)</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.load }}</div>
            </div>

            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">Heap</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.heapPercent }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Uptime</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.uptime }}</div>
            </div>

            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">RAM</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.ram }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">JVM Heap</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.jvmHeap }}</div>
            </div>

            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">Disk</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.diskPercent }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Disk Usage</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.diskUsage }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 3: INDICES -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'indices'" class="space-y-4">
        <!-- Search bar -->
        <div class="flex items-center justify-between gap-4">
          <div class="relative flex-1 max-w-md">
            <Search class="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
            <input
              v-model="indexSearch"
              placeholder="Search indices by name..."
              class="w-full bg-[#1b1e26] border border-slate-800 rounded-lg pl-9 pr-4 py-2 text-xs text-white focus:outline-none focus:border-brand-500 transition"
            />
          </div>
          <span class="text-xs text-slate-400 font-mono">Showing {{ filteredIndices.length }} indices</span>
        </div>

        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                <tr>
                  <th class="py-3 px-6">Health</th>
                  <th class="py-3 px-4">Status</th>
                  <th class="py-3 px-4">Index</th>
                  <th class="py-3 px-4">UUID</th>
                  <th class="py-3 px-4">Primary</th>
                  <th class="py-3 px-4">Replica</th>
                  <th class="py-3 px-4">Docs Count</th>
                  <th class="py-3 px-4">Docs Deleted</th>
                  <th class="py-3 px-4">Store Size</th>
                  <th class="py-3 px-6">Primary Size</th>
                </tr>
              </thead>
              <tbody v-if="filteredIndices.length > 0" class="divide-y divide-slate-800/60 font-mono text-[11px]">
                <tr
                  v-for="idx in filteredIndices"
                  :key="idx.index || idx.name"
                  @click="selectedIndexModal = idx"
                  class="hover:bg-slate-800/40 transition text-slate-300 cursor-pointer group"
                >
                  <td class="py-3 px-6 font-sans">
                    <span
                      :class="[
                        'px-2 py-0.5 rounded text-[10px] font-semibold lowercase',
                        idx.health === 'green' ? 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-400' : idx.health === 'yellow' ? 'bg-amber-500/10 border border-amber-500/30 text-amber-400' : 'bg-red-500/10 text-red-400'
                      ]"
                    >
                      {{ idx.health || 'green' }}
                    </span>
                  </td>
                  <td class="py-3 px-4 text-slate-400 lowercase font-sans">{{ idx.status || 'open' }}</td>
                  <td class="py-3 px-4 font-sans font-medium text-slate-200 group-hover:text-brand-400 transition">{{ idx.index || idx.name }}</td>
                  <td class="py-3 px-4 text-slate-500 truncate max-w-[140px]">{{ idx.uuid || '-' }}</td>
                  <td class="py-3 px-4 text-emerald-400 font-bold">{{ idx.pri || idx.primary || 1 }}</td>
                  <td class="py-3 px-4 text-sky-400 font-bold">{{ idx.rep || idx.replica || 0 }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ formatNumber(idx['docs.count'] || idx.docsCount || 0) }}</td>
                  <td class="py-3 px-4 text-slate-500">{{ formatNumber(idx['docs.deleted'] || idx.docsDeleted || 0) }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ idx['store.size'] || idx.storeSize || '-' }}</td>
                  <td class="py-3 px-6 text-slate-300">{{ idx['pri.store.size'] || idx.priStoreSize || '-' }}</td>
                </tr>
              </tbody>
              <tbody v-else>
                <tr>
                  <td colspan="10" class="text-center py-8 text-slate-500 text-xs font-sans">
                    No indices found.
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 4: SHARDS & ALLOCATION (MATCHING SCREENSHOT) -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'shards'" class="space-y-6">
        <!-- Legend Bar -->
        <div class="flex items-center gap-6 text-xs bg-[#1b1e26] border border-slate-800/80 p-3 px-5 rounded-xl shadow-lg">
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#0f3d28] border border-[#1c6b47] flex items-center justify-center text-[9px] font-bold text-[#4ade80]">P</span>
            <span class="text-slate-300 font-medium">Primary Shard</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#132c4a] border border-[#1e4976] flex items-center justify-center text-[9px] font-bold text-[#60a5fa]">R</span>
            <span class="text-slate-300 font-medium">Replica Shard</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#3b1219] border border-[#822735] flex items-center justify-center text-[9px] font-bold text-[#f87171]">U</span>
            <span class="text-slate-300 font-medium">Unassigned</span>
          </div>
        </div>

        <!-- Shard Allocation by Node Card (Exact visual from screenshot) -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl p-6 space-y-6 shadow-xl">
          <h3 class="text-xs font-bold text-white tracking-wide uppercase">Shard Allocation by Node</h3>

          <div v-if="clusterNodes.length === 0" class="text-center py-6 text-slate-500 text-xs">
            No shard allocations discovered yet.
          </div>

          <!-- Node Rows with Matrix of Shard Tiles -->
          <div v-for="node in clusterNodes" :key="node.name" class="space-y-2 border-b border-slate-800/80 pb-6 last:border-b-0 last:pb-0">
            <div class="flex items-center justify-between text-xs">
              <span class="font-bold text-white text-xs tracking-wide">{{ node.name }}</span>
              <div class="flex items-center gap-4 text-slate-400 text-[11px] font-mono">
                <span class="text-emerald-400 font-medium">Primary: {{ node.primaryCount }}</span>
                <span class="text-sky-400 font-medium">Replica: {{ node.replicaCount }}</span>
              </div>
            </div>

            <!-- Visual Shard Blocks Matrix -->
            <div class="flex flex-wrap gap-1.5 p-3.5 bg-[#13161c] rounded-xl border border-slate-800/80 min-h-[50px] items-center">
              <div
                v-for="(shard, sIdx) in (shardsByNode[node.name] || [])"
                :key="sIdx"
                @mouseenter="showTooltip($event, shard)"
                @mouseleave="hideTooltip"
                :class="[
                  'w-6 h-6 rounded flex items-center justify-center font-bold text-[10px] cursor-pointer transition-all duration-150 transform select-none',
                  // 1. Highlighted state: ALL primary and replica shards of the hovered index are highlighted with glowing white border
                  isShardHighlighted(shard)
                    ? (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#10b981] text-white border-2 border-white ring-4 ring-emerald-500/40 scale-125 z-30 shadow-2xl font-black'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#3b82f6] text-white border-2 border-white ring-4 ring-blue-500/40 scale-125 z-30 shadow-2xl font-black'
                        : 'bg-red-500 text-white border-2 border-white ring-4 ring-red-500/40 scale-125 z-30 shadow-2xl font-black')
                    // 2. Dimmed state: other unrelated indices are dimmed
                    : isShardDimmed(shard)
                    ? (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#0f3d28]/25 border border-[#1c6b47]/20 text-[#4ade80]/30 opacity-20 scale-90'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#132c4a]/25 border border-[#1e4976]/20 text-[#60a5fa]/30 opacity-20 scale-90'
                        : 'bg-[#3b1219]/25 border border-[#822735]/20 text-[#f87171]/30 opacity-20 scale-90')
                    // 3. Normal idle state
                    : (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#0f3d28] border border-[#1c6b47] text-[#4ade80] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#132c4a] border border-[#1e4976] text-[#60a5fa] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md'
                        : 'bg-[#3b1219] border border-[#822735] text-[#f87171] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md')
                ]"
              >
                {{ shard.prirep === 'p' || shard.type === 'Primary' ? 'P' : shard.prirep === 'r' || shard.type === 'Replica' ? 'R' : 'U' }}
              </div>
              <span v-if="(shardsByNode[node.name] || []).length === 0" class="text-[11px] text-slate-600 italic">
                No active shards on this node.
              </span>
            </div>
          </div>
        </div>

        <!-- Bottom: All Shards Table -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="p-4 px-6 border-b border-slate-800/80 flex items-center justify-between">
            <h3 class="text-xs font-bold text-white tracking-wide">All Shards ({{ filteredShards.length }})</h3>
            <div class="relative w-64">
              <Search class="w-3.5 h-3.5 absolute left-2.5 top-2 text-slate-500" />
              <input
                v-model="shardSearch"
                placeholder="Filter shards..."
                class="w-full bg-[#14161b] border border-slate-800 rounded-lg pl-8 pr-3 py-1 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
              />
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                <tr>
                  <th class="py-3 px-6">Index</th>
                  <th class="py-3 px-4">Shard</th>
                  <th class="py-3 px-4">Type</th>
                  <th class="py-3 px-4">State</th>
                  <th class="py-3 px-4">Docs</th>
                  <th class="py-3 px-4">Store</th>
                  <th class="py-3 px-4">IP</th>
                  <th class="py-3 px-6">Node</th>
                </tr>
              </thead>
              <tbody v-if="filteredShards.length > 0" class="divide-y divide-slate-800/60 font-mono text-[11px]">
                <tr
                  v-for="(shard, idx) in filteredShards"
                  :key="idx"
                  class="hover:bg-slate-800/30 transition text-slate-300"
                >
                  <td class="py-3 px-6 font-sans font-medium text-slate-200">{{ shard.index }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ shard.shard || '0' }}</td>
                  <td class="py-3 px-4 font-sans">
                    <span
                      :class="[
                        'px-2 py-0.5 rounded text-[10px] font-semibold',
                        shard.prirep === 'p' || shard.type === 'Primary'
                          ? 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-400'
                          : 'bg-sky-500/10 border border-sky-500/30 text-sky-400'
                      ]"
                    >
                      {{ shard.prirep === 'p' || shard.type === 'Primary' ? 'Primary' : 'Replica' }}
                    </span>
                  </td>
                  <td class="py-3 px-4 font-sans text-emerald-400 uppercase font-semibold text-[10px]">{{ shard.state || 'STARTED' }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ formatNumber(shard.docs || 0) }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ shard.store || '-' }}</td>
                  <td class="py-3 px-4 text-slate-400">{{ shard.ip || '-' }}</td>
                  <td class="py-3 px-6 text-slate-300 font-sans">{{ shard.node || '-' }}</td>
                </tr>
              </tbody>
              <tbody v-else>
                <tr>
                  <td colspan="8" class="text-center py-8 text-slate-500 text-xs font-sans">
                    No shard telemetry available.
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 5: CONNECTION CONFIGURATION -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'connection'" class="max-w-2xl mx-auto space-y-6">
        <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6 shadow-xl">
          <div class="flex items-center gap-3 border-b border-slate-800 pb-4">
            <Sliders class="w-5 h-5 text-brand-400" />
            <div>
              <h3 class="text-sm font-bold text-white">Cluster Connection Settings</h3>
              <p class="text-xs text-slate-400">Configure connection credentials to your OpenSearch cluster</p>
            </div>
          </div>

          <form @submit.prevent="handleSaveConfig" class="space-y-4 text-xs">
            <div>
              <label class="block text-slate-400 mb-1 font-medium">Cluster Name</label>
              <input v-model="configForm.name" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="e.g. Production Cluster" />
            </div>

            <div class="grid grid-cols-3 gap-4">
              <div class="col-span-2">
                <label class="block text-slate-400 mb-1 font-medium">OpenSearch Host / IP</label>
                <input v-model="configForm.host" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="e.g. 103.171.31.56 or search.example.com" />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-medium">HTTP Port</label>
                <input v-model.number="configForm.port" type="number" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-slate-400 mb-1 font-medium">Username (Optional)</label>
                <input v-model="configForm.username" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="admin" />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-medium">Password (Encrypted AES-256)</label>
                <input v-model="configForm.password" type="password" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500 font-mono" placeholder="••••••••" />
              </div>
            </div>

            <div class="flex items-center gap-6 pt-2">
              <label class="flex items-center gap-2 cursor-pointer">
                <input v-model="configForm.useSsl" type="checkbox" class="rounded bg-slate-800 border-slate-700 text-brand-500 focus:ring-0" />
                <span class="text-slate-300">Use HTTPS (SSL/TLS)</span>
              </label>
              <label class="flex items-center gap-2 cursor-pointer">
                <input v-model="configForm.verifySsl" type="checkbox" class="rounded bg-slate-800 border-slate-700 text-brand-500 focus:ring-0" />
                <span class="text-slate-300">Verify SSL Certificate</span>
              </label>
            </div>

            <div v-if="testResult" class="p-3 rounded-lg text-xs" :class="testSuccess ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-400' : 'bg-red-500/10 border border-red-500/20 text-red-400'">
              {{ testResult }}
            </div>

            <div class="flex items-center justify-between pt-4 border-t border-slate-800">
              <button
                type="button"
                @click="handleTestConnection"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg transition"
              >
                Test Connection
              </button>

              <button
                type="submit"
                :disabled="isSavingConfig"
                class="px-5 py-2 bg-brand-500 hover:bg-brand-600 text-white font-medium rounded-lg shadow-lg shadow-brand-500/20 transition disabled:opacity-50"
              >
                {{ isSavingConfig ? 'Saving...' : 'Save Configuration' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 6: LOGS -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'logs'" class="space-y-4">
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="p-4 px-6 border-b border-slate-800 flex items-center justify-between">
            <h3 class="text-xs font-bold text-white tracking-wide">OpenSearch Background Job Logs</h3>
            <span class="text-xs font-mono text-slate-400">{{ logsList.length }} entries</span>
          </div>

          <div class="p-4 max-h-[600px] overflow-y-auto font-mono text-xs space-y-2 bg-[#14161b]">
            <div
              v-for="(log, lIdx) in logsList"
              :key="lIdx"
              class="flex items-start gap-3 p-2 rounded hover:bg-slate-800/30 transition text-slate-300"
            >
              <span class="text-slate-500 shrink-0">{{ log.timestamp ? new Date(log.timestamp).toLocaleTimeString() : '-' }}</span>
              <span
                :class="[
                  'px-1.5 py-0.2 rounded text-[10px] font-bold shrink-0',
                  log.level === 'ERROR' ? 'bg-red-500/20 text-red-400' : log.level === 'WARN' ? 'bg-amber-500/20 text-amber-400' : 'bg-emerald-500/20 text-emerald-400'
                ]"
              >
                {{ log.level || 'INFO' }}
              </span>
              <span class="flex-1 text-slate-300">{{ log.message }}</span>
            </div>
            <p v-if="logsList.length === 0" class="text-center py-8 text-slate-500 italic">No OpenSearch logs recorded.</p>
          </div>
        </div>
      </div>

    </main>

    <!-- ================================================================= -->
    <!-- FLOATING SHARD TOOLTIP (MATCHING USER SCREENSHOT EXACTLY) -->
    <!-- ================================================================= -->
    <div
      v-if="hoveredShard"
      :style="{ left: `${tooltipPosition.x}px`, top: `${tooltipPosition.y}px` }"
      class="fixed z-50 w-80 p-4 bg-[#141b2d] border border-blue-500/50 rounded-xl shadow-2xl backdrop-blur-md pointer-events-none text-xs space-y-2.5 animate-in fade-in zoom-in-95 duration-100"
    >
      <div class="font-bold text-[#60a5fa] text-xs pb-1.5 border-b border-slate-700/60 truncate">
        {{ hoveredShard.index }}
      </div>
      <div class="space-y-1.5 text-[11px] font-sans text-slate-300">
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Index:</span> <span class="font-mono text-slate-200 truncate max-w-[180px]">{{ hoveredShard.index }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Size:</span> <span class="font-mono text-slate-200">{{ hoveredShard.store || hoveredShard['store.size'] || '-' }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Node:</span> <span class="text-slate-200 font-mono">{{ hoveredShard.node || '-' }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">IP:</span> <span class="font-mono text-slate-200">{{ hoveredShard.ip || '-' }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Docs:</span> <span class="font-mono text-slate-200">{{ formatNumber(hoveredShard.docs || 0) }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Shard:</span> <span class="font-mono text-slate-200">{{ hoveredShard.shard || 0 }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Type:</span> <span :class="hoveredShard.prirep === 'p' || hoveredShard.type === 'Primary' ? 'text-emerald-400 font-bold' : 'text-sky-400 font-bold'">{{ hoveredShard.prirep === 'p' || hoveredShard.type === 'Primary' ? 'Primary' : 'Replica' }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">State:</span> <span class="text-emerald-400 uppercase font-bold tracking-wider">{{ hoveredShard.state || 'STARTED' }}</span></div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: INDEX DETAILS & SHARDS BREAKDOWN -->
    <!-- ================================================================= -->
    <div
      v-if="selectedIndexModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-2xl p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Database class="w-4 h-4 text-brand-400" />
            <h3 class="text-sm font-bold text-white">{{ selectedIndexModal.index || selectedIndexModal.name }}</h3>
          </div>
          <button @click="selectedIndexModal = null" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="grid grid-cols-3 gap-3 text-xs font-sans">
          <div class="p-3 rounded-lg bg-[#14161b] border border-slate-800">
            <p class="text-[10px] text-slate-500 uppercase font-bold">Health</p>
            <p class="text-sm font-bold text-emerald-400 uppercase">{{ selectedIndexModal.health || 'GREEN' }}</p>
          </div>
          <div class="p-3 rounded-lg bg-[#14161b] border border-slate-800">
            <p class="text-[10px] text-slate-500 uppercase font-bold">Primary Shards</p>
            <p class="text-sm font-bold text-white font-mono">{{ selectedIndexModal.pri || 1 }}</p>
          </div>
          <div class="p-3 rounded-lg bg-[#14161b] border border-slate-800">
            <p class="text-[10px] text-slate-500 uppercase font-bold">Replica Shards</p>
            <p class="text-sm font-bold text-sky-400 font-mono">{{ selectedIndexModal.rep || 0 }}</p>
          </div>
        </div>

        <!-- Allocated Shards for this Index -->
        <div class="space-y-2">
          <h4 class="text-xs font-bold text-slate-400 uppercase tracking-wider">Allocated Shards</h4>
          <div class="overflow-x-auto rounded-lg border border-slate-800">
            <table class="w-full text-left text-xs font-mono">
              <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase">
                <tr>
                  <th class="p-2.5 px-4">Shard #</th>
                  <th class="p-2.5 px-4">Type</th>
                  <th class="p-2.5 px-4">Node</th>
                  <th class="p-2.5 px-4">IP</th>
                  <th class="p-2.5 px-4">Docs</th>
                  <th class="p-2.5 px-4">Size</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800/60 text-slate-300">
                <tr v-for="(s, sIdx) in selectedIndexShards" :key="sIdx" class="hover:bg-slate-800/30">
                  <td class="p-2.5 px-4 text-white">{{ s.shard }}</td>
                  <td class="p-2.5 px-4 font-sans">
                    <span :class="s.prirep === 'p' || s.type === 'Primary' ? 'text-emerald-400 font-bold' : 'text-sky-400 font-bold'">
                      {{ s.prirep === 'p' || s.type === 'Primary' ? 'Primary' : 'Replica' }}
                    </span>
                  </td>
                  <td class="p-2.5 px-4 text-slate-200">{{ s.node }}</td>
                  <td class="p-2.5 px-4 text-slate-400">{{ s.ip }}</td>
                  <td class="p-2.5 px-4">{{ formatNumber(s.docs || 0) }}</td>
                  <td class="p-2.5 px-4">{{ s.store || '-' }}</td>
                </tr>
                <tr v-if="selectedIndexShards.length === 0">
                  <td colspan="6" class="p-4 text-center text-slate-500 font-sans">No shard allocations discovered.</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div class="flex justify-end pt-2">
          <button @click="selectedIndexModal = null" class="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-xs">
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: AUTO-REFRESH SETTINGS (GEAR ICON) -->
    <!-- ================================================================= -->
    <div
      v-if="isSettingsModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-sm p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Settings class="w-4 h-4 text-brand-400" />
            <h3 class="text-sm font-bold text-white">Auto-Refresh Interval</h3>
          </div>
          <button @click="isSettingsModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-2">
          <button
            v-for="opt in [
              { label: 'Manual Refresh Only', sec: 0 },
              { label: 'Every 5 Seconds', sec: 5 },
              { label: 'Every 10 Seconds', sec: 10 },
              { label: 'Every 15 Seconds', sec: 15 },
              { label: 'Every 30 Seconds (Default)', sec: 30 },
              { label: 'Every 60 Seconds (1 Min)', sec: 60 },
              { label: 'Every 300 Seconds (5 Min)', sec: 300 }
            ]"
            :key="opt.sec"
            @click="setRefreshInterval(opt.sec)"
            :class="[
              'w-full p-2.5 rounded-lg text-xs text-left transition flex items-center justify-between border',
              refreshIntervalSec === opt.sec
                ? 'bg-brand-500/10 border-brand-500/40 text-brand-400 font-bold'
                : 'bg-[#14161b] border-slate-800 text-slate-300 hover:border-slate-700'
            ]"
          >
            <span>{{ opt.label }}</span>
            <CheckCircle2 v-if="refreshIntervalSec === opt.sec" class="w-3.5 h-3.5 text-brand-400" />
          </button>
        </div>
      </div>
    </div>

  </div>
</template>
