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
} from 'lucide-vue-next';

const router = useRouter();

// Active Navigation Tab
const activeTab = ref<'overview' | 'nodes' | 'indices' | 'shards' | 'connection' | 'logs'>('overview');

// Auto-Refresh state
const refreshIntervalSec = ref(30);
const countdown = ref(30);
const isRefreshing = ref(false);
const timer = ref<any>(null);

// Data states
const error = ref('');
const clusterHealth = ref<any | null>(null);
const nodesList = ref<any[]>([]);
const indicesList = ref<any[]>([]);
const shardsList = ref<any[]>([]);
const logsList = ref<any[]>([]);

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

// Hover Tooltip State for Shards
const hoveredShard = ref<any | null>(null);
const tooltipPosition = ref({ x: 0, y: 0 });

// Formatted Overview Computations
const totalIndicesCount = computed(() => indicesList.value.length);
const totalDocumentsCount = computed(() => {
  return indicesList.value.reduce((acc, idx) => acc + (parseInt(idx['docs.count'] || idx.docsCount || 0) || 0), 0);
});

const totalStoreSizeBytes = computed(() => {
  if (indicesList.value.length > 0) {
    const bytes = indicesList.value.reduce((acc, idx) => acc + (parseInt(idx['store.size'] || idx.storeSize || 0) || 0), 0);
    return formatBytes(bytes);
  }
  return '0 B';
});

const greenIndicesCount = computed(() => indicesList.value.filter(i => (i.health || '').toLowerCase() === 'green').length);
const yellowIndicesCount = computed(() => indicesList.value.filter(i => (i.health || '').toLowerCase() === 'yellow').length);
const redIndicesCount = computed(() => indicesList.value.filter(i => (i.health || '').toLowerCase() === 'red').length);

// Filtered indices
const filteredIndices = computed(() => {
  if (!indexSearch.value) return indicesList.value;
  const q = indexSearch.value.toLowerCase();
  return indicesList.value.filter(idx => (idx.index || idx.name || '').toLowerCase().includes(q));
});

// Grouped shards per node
const shardsByNode = computed(() => {
  const map: Record<string, any[]> = {};
  nodesList.value.forEach(n => {
    map[n.name] = [];
  });

  shardsList.value.forEach(shard => {
    const nodeName = shard.node || shard.nodeName || (nodesList.value[0]?.name || 'unknown');
    if (!map[nodeName]) map[nodeName] = [];
    map[nodeName].push(shard);
  });

  return map;
});

function formatBytes(bytes: number, decimals = 1) {
  if (!+bytes) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

function formatNumber(num: number) {
  return new Intl.NumberFormat('en-US').format(num);
}

// Fetch all cluster telemetry from actual backend
const fetchClusterData = async () => {
  isRefreshing.value = true;
  error.value = '';
  try {
    const [healthRes, nodesStatsRes, nodesInfoRes, indicesRes, shardsRes] = await Promise.allSettled([
      axios.get('/api/v1/opensearch/health'),
      axios.get('/api/v1/opensearch/nodes'),
      axios.get('/api/v1/opensearch/nodes/info'),
      axios.get('/api/v1/opensearch/indices'),
      axios.get('/api/v1/opensearch/shards'),
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
  } catch (err: any) {
    console.error('Error fetching OpenSearch data:', err);
  } finally {
    isRefreshing.value = false;
    countdown.value = refreshIntervalSec.value;
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

    // Count shards on this node
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

// Hover Tooltip Handlers
const showTooltip = (event: MouseEvent, shard: any) => {
  hoveredShard.value = shard;
  const target = event.currentTarget as HTMLElement;
  const rect = target.getBoundingClientRect();
  tooltipPosition.value = {
    x: Math.min(rect.left - 80, window.innerWidth - 320),
    y: rect.bottom + 8,
  };
};

const hideTooltip = () => {
  hoveredShard.value = null;
};

// Configuration Methods
const loadConfig = async () => {
  try {
    const res = await axios.get('/api/v1/opensearch/config');
    if (res.data.success && res.data.data) {
      configForm.value = { ...configForm.value, ...res.data.data };
    }
  } catch (err) {
    console.error('Failed to load OpenSearch config', err);
  }
};

const handleTestConnection = async () => {
  testResult.value = 'Testing connection...';
  testSuccess.value = false;
  try {
    const res = await axios.post('/api/v1/opensearch/test', {
      host: configForm.value.host,
      port: Number(configForm.value.port),
      username: configForm.value.username,
      password: configForm.value.password,
      useSsl: configForm.value.useSsl,
    });
    if (res.data.success) {
      testSuccess.value = true;
      testResult.value = `Connected successfully! Cluster: ${res.data.data?.cluster_name || 'OpenSearch'}`;
    }
  } catch (err: any) {
    testSuccess.value = false;
    testResult.value = err.response?.data?.error || 'Connection failed: Host unreachable or bad credentials.';
  }
};

const handleSaveConfig = async () => {
  isSavingConfig.value = true;
  try {
    const res = await axios.post('/api/v1/opensearch/config', configForm.value);
    if (res.data.success) {
      await fetchClusterData();
      activeTab.value = 'overview';
    }
  } catch (err: any) {
    error.value = err.response?.data?.error || 'Failed to save configuration';
  } finally {
    isSavingConfig.value = false;
  }
};

const handleBackToPortal = () => {
  if (window.opener) {
    window.close();
  } else {
    router.push('/');
  }
};

// Timer Tick
const startAutoRefresh = () => {
  timer.value = setInterval(() => {
    if (countdown.value > 1) {
      countdown.value--;
    } else {
      fetchClusterData();
    }
  }, 1000);
};

onMounted(() => {
  loadConfig();
  fetchClusterData();
  startAutoRefresh();
});

onUnmounted(() => {
  if (timer.value) clearInterval(timer.value);
});
</script>

<template>
  <div class="min-h-screen bg-[#14161b] text-slate-200 font-sans flex flex-col selection:bg-brand-500/30">
    <!-- Top Header Bar -->
    <header class="h-14 bg-[#1b1e26] border-b border-slate-800/80 px-6 flex items-center justify-between shrink-0">
      <!-- Title with OpenSearch Icon -->
      <div class="flex items-center gap-3">
        <div class="w-6 h-6 rounded flex items-center justify-center text-slate-300">
          <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
          </svg>
        </div>
        <h1 class="text-sm font-semibold text-white tracking-wide">OpenSearch Cluster Monitor</h1>
      </div>

      <!-- Right Header Actions -->
      <div class="flex items-center gap-3">
        <!-- Auto Refresh Indicator -->
        <div class="flex items-center gap-1.5 px-2.5 py-1 rounded bg-[#242833] border border-slate-700/60 text-xs text-slate-300 font-mono">
          <Clock class="w-3.5 h-3.5 text-slate-400" />
          <span>{{ countdown }} s</span>
        </div>

        <!-- Manual Refresh Button -->
        <button
          @click="fetchClusterData"
          :disabled="isRefreshing"
          title="Refresh Now"
          class="p-1.5 rounded bg-[#242833] border border-slate-700/60 text-slate-300 hover:text-white hover:border-slate-600 transition disabled:opacity-50"
        >
          <RotateCw :class="['w-4 h-4', isRefreshing ? 'animate-spin text-brand-400' : '']" />
        </button>

        <!-- Settings Modal Trigger -->
        <button
          @click="activeTab = 'connection'"
          title="Cluster Connection Settings"
          :class="[
            'p-1.5 rounded border transition',
            activeTab === 'connection'
              ? 'bg-brand-500/20 border-brand-500/40 text-brand-400'
              : 'bg-[#242833] border-slate-700/60 text-slate-300 hover:text-white hover:border-slate-600'
          ]"
        >
          <Settings class="w-4 h-4" />
        </button>

        <!-- Back to Portal Button -->
        <button
          @click="handleBackToPortal"
          class="flex items-center gap-1.5 px-3 py-1 rounded bg-[#242833] border border-slate-700/60 text-xs text-slate-300 hover:text-white hover:border-slate-500 transition ml-1 font-medium"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Back to Portal</span>
        </button>
      </div>
    </header>

    <!-- Sub-Header Navigation Tabs -->
    <div class="bg-[#1b1e26] border-b border-slate-800/80 px-6 flex items-center gap-8 text-xs font-medium shrink-0">
      <button
        v-for="tab in [
          { id: 'overview', label: 'Overview' },
          { id: 'nodes', label: 'Nodes' },
          { id: 'indices', label: 'Indices' },
          { id: 'shards', label: 'Shards' },
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

    <!-- Main Content Container -->
    <main class="flex-1 p-6 overflow-y-auto space-y-6 max-w-[1600px] w-full mx-auto">
      
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
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl relative overflow-hidden flex flex-col justify-between h-32">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Cluster Health</span>
            <div class="text-2xl font-black tracking-tight uppercase" :class="clusterHealth?.status === 'green' ? 'text-emerald-400' : clusterHealth?.status === 'yellow' ? 'text-amber-400' : clusterHealth?.status === 'red' ? 'text-red-400' : 'text-slate-500'">
              {{ clusterHealth?.status || 'NOT CONNECTED' }}
            </div>
            <!-- Bottom glowing indicator line -->
            <div
              class="w-full h-1 rounded-full shadow-sm"
              :class="clusterHealth?.status === 'green' ? 'bg-emerald-500 shadow-emerald-500/50' : clusterHealth?.status === 'yellow' ? 'bg-amber-500' : clusterHealth?.status === 'red' ? 'bg-red-500' : 'bg-slate-700'"
            ></div>
          </div>

          <!-- Card 2: Total Indices -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Total Indices</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ formatNumber(totalIndicesCount) }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Managed across cluster</span>
          </div>

          <!-- Card 3: Total Documents -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Total Documents</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ formatNumber(totalDocumentsCount) }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Indexed searchable docs</span>
          </div>

          <!-- Card 4: Store Size -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Store Size</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ totalStoreSizeBytes }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Total allocated storage</span>
          </div>
        </div>

        <!-- Middle Section: Index & Shards + Node Load (2 Cards) -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Left: Index & Shards -->
          <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6">
            <h3 class="text-xs font-bold text-white tracking-wide">Index & Shards</h3>
            <div class="grid grid-cols-2 gap-y-6 text-xs">
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Primary Shards</p>
                <p class="text-xl font-bold text-white">{{ clusterHealth?.active_primary_shards ?? 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Replica Shards</p>
                <p class="text-xl font-bold text-white">{{ clusterHealth ? (clusterHealth.active_shards - clusterHealth.active_primary_shards) : 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Total Shards</p>
                <p class="text-xl font-bold text-white">{{ clusterHealth?.active_shards ?? 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Unassigned</p>
                <p class="text-xl font-bold text-emerald-400">{{ clusterHealth?.unassigned_shards ?? 0 }}</p>
              </div>
            </div>
          </div>

          <!-- Right: Node Load -->
          <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6">
            <h3 class="text-xs font-bold text-white tracking-wide">Node Load</h3>
            <div class="grid grid-cols-2 gap-y-6 text-xs">
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Total Nodes</p>
                <p class="text-xl font-bold text-white">{{ nodesList.length }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Data Nodes</p>
                <p class="text-xl font-bold text-white">{{ clusterHealth?.number_of_data_nodes ?? nodesList.length }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Cluster Manager</p>
                <p class="text-xl font-bold text-white">{{ nodesList.length }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Ingest Nodes</p>
                <p class="text-xl font-bold text-white">{{ nodesList.length }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Bottom: Index Health Overview Table -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="p-4 px-6 border-b border-slate-800/80 flex items-center justify-between">
            <h3 class="text-xs font-bold text-white tracking-wide">Index Health Overview</h3>
            <div class="flex items-center gap-3 text-xs">
              <span class="px-2.5 py-0.5 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-[11px] font-medium flex items-center gap-1.5">
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                Green: {{ greenIndicesCount }}
              </span>
              <span class="px-2.5 py-0.5 rounded-full bg-amber-500/10 border border-amber-500/20 text-amber-400 text-[11px] font-medium flex items-center gap-1.5">
                <span class="w-1.5 h-1.5 rounded-full bg-amber-400"></span>
                Yellow: {{ yellowIndicesCount }}
              </span>
              <span class="px-2.5 py-0.5 rounded-full bg-red-500/10 border border-red-500/20 text-red-400 text-[11px] font-medium flex items-center gap-1.5">
                <span class="w-1.5 h-1.5 rounded-full bg-red-400"></span>
                Red: {{ redIndicesCount }}
              </span>
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                <tr>
                  <th class="py-3 px-6">Health</th>
                  <th class="py-3 px-4">Status</th>
                  <th class="py-3 px-4">Index</th>
                  <th class="py-3 px-4">Primary</th>
                  <th class="py-3 px-4">Replica</th>
                  <th class="py-3 px-4">Docs</th>
                  <th class="py-3 px-4">Store Size</th>
                  <th class="py-3 px-6">Primary Size</th>
                </tr>
              </thead>
              <tbody v-if="indicesList.length > 0" class="divide-y divide-slate-800/60 font-mono text-[11px]">
                <tr
                  v-for="idx in indicesList.slice(0, 20)"
                  :key="idx.index || idx.name"
                  class="hover:bg-slate-800/30 transition text-slate-300"
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
                  <td class="py-3 px-4 font-sans font-medium text-slate-200">{{ idx.index || idx.name }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ idx.pri || idx.primary || 1 }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ idx.rep || idx.replica || 0 }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ formatNumber(idx['docs.count'] || idx.docsCount || 0) }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ idx['store.size'] || idx.storeSize || '-' }}</td>
                  <td class="py-3 px-6 text-slate-300">{{ idx['pri.store.size'] || idx.priStoreSize || '-' }}</td>
                </tr>
              </tbody>
              <tbody v-else>
                <tr>
                  <td colspan="8" class="text-center py-8 text-slate-500 text-xs font-sans">
                    No indices data available. Connect an OpenSearch cluster to display indices.
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 2: NODES -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'nodes'" class="space-y-4">
        <div v-if="nodesList.length === 0" class="p-8 bg-[#1b1e26] border border-slate-800/80 rounded-xl text-center space-y-2">
          <p class="text-xs text-slate-400">No nodes discovered yet. Ensure OpenSearch cluster is reachable.</p>
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
              <span class="text-slate-600">v</span>
            </div>
          </div>

          <!-- Node Metrics Grid (8 metrics matching screenshot) -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-6 text-xs font-sans">
            <!-- CPU -->
            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">CPU</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.cpu }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Load (1m/5m/15m)</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.load }}</div>
            </div>

            <!-- Heap -->
            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">Heap</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.heapPercent }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Uptime</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.uptime }}</div>
            </div>

            <!-- RAM -->
            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">RAM</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.ram }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">JVM Heap</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.jvmHeap }}</div>
            </div>

            <!-- Disk -->
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
                  class="hover:bg-slate-800/30 transition text-slate-300"
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
                  <td class="py-3 px-4 font-sans font-medium text-slate-200">{{ idx.index || idx.name }}</td>
                  <td class="py-3 px-4 text-slate-500 truncate max-w-[140px]">{{ idx.uuid || '-' }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ idx.pri || idx.primary || 1 }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ idx.rep || idx.replica || 0 }}</td>
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
      <!-- TAB 4: SHARDS -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'shards'" class="space-y-6">
        <!-- Legend Bar (Matching Screenshot 4) -->
        <div class="flex items-center gap-6 text-xs bg-[#1b1e26] border border-slate-800/80 p-3 px-5 rounded-xl">
          <div class="flex items-center gap-2">
            <span class="w-3.5 h-3.5 rounded bg-emerald-500"></span>
            <span class="text-slate-300">Primary</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-3.5 h-3.5 rounded bg-sky-500"></span>
            <span class="text-slate-300">Replica</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-3.5 h-3.5 rounded bg-red-500"></span>
            <span class="text-slate-300">Unassigned</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-3.5 h-3.5 rounded bg-amber-500"></span>
            <span class="text-slate-300">Relocating</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-3.5 h-3.5 rounded bg-purple-500"></span>
            <span class="text-slate-300">Initializing</span>
          </div>
        </div>

        <!-- Shard Allocation by Node Card -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl p-6 space-y-6 shadow-xl">
          <h3 class="text-xs font-bold text-white tracking-wide">Shard Allocation by Node</h3>

          <div v-if="nodesList.length === 0" class="text-center py-6 text-slate-500 text-xs">
            No shard allocations discovered yet.
          </div>

          <!-- Node Grids -->
          <div v-for="node in nodesList" :key="node.name" class="space-y-2 border-b border-slate-800 pb-6 last:border-b-0 last:pb-0">
            <div class="flex items-center justify-between text-xs">
              <span class="font-bold text-slate-200">{{ node.name }}</span>
              <div class="flex items-center gap-4 text-slate-400 text-[11px] font-mono">
                <span class="text-emerald-400">Primary: {{ node.primaryShards }}</span>
                <span class="text-sky-400">Replica: {{ node.replicaShards }}</span>
              </div>
            </div>

            <!-- Visual Shard Blocks Matrix -->
            <div class="flex flex-wrap gap-1.5 p-3 bg-[#14161b] rounded-lg border border-slate-800/80">
              <div
                v-for="(shard, sIdx) in (shardsByNode[node.name] || [])"
                :key="sIdx"
                @mouseenter="showTooltip($event, shard)"
                @mouseleave="hideTooltip"
                :class="[
                  'w-5 h-5 rounded flex items-center justify-center text-[10px] font-bold cursor-pointer transition transform hover:scale-125 hover:z-20',
                  shard.prirep === 'p' || shard.type === 'Primary'
                    ? 'bg-emerald-600/90 text-white hover:bg-emerald-500 shadow-sm shadow-emerald-600/20'
                    : shard.prirep === 'r' || shard.type === 'Replica'
                    ? 'bg-sky-600/90 text-white hover:bg-sky-500 shadow-sm shadow-sky-600/20'
                    : 'bg-red-600 text-white'
                ]"
              >
                {{ shard.prirep === 'p' || shard.type === 'Primary' ? 'P' : 'R' }}
              </div>
            </div>
          </div>
        </div>

        <!-- Bottom: All Shards Table -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="p-4 px-6 border-b border-slate-800/80">
            <h3 class="text-xs font-bold text-white tracking-wide">All Shards</h3>
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
              <tbody v-if="shardsList.length > 0" class="divide-y divide-slate-800/60 font-mono text-[11px]">
                <tr
                  v-for="(shard, idx) in shardsList"
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

        <!-- Interactive Floating Tooltip (Matching Screenshot 5) -->
        <div
          v-if="hoveredShard"
          :style="{ left: `${tooltipPosition.x}px`, top: `${tooltipPosition.y}px` }"
          class="fixed z-50 w-72 p-4 bg-[#1b2234] border border-blue-500/40 rounded-xl shadow-2xl backdrop-blur-md pointer-events-none text-xs space-y-2 animate-in fade-in zoom-in-95 duration-100"
        >
          <div class="font-bold text-white text-xs border-b border-slate-700/60 pb-1.5 truncate">
            {{ hoveredShard.index }}
          </div>
          <div class="space-y-1 text-[11px] font-sans text-slate-300">
            <div class="flex justify-between"><span class="text-slate-400">Index:</span> <span class="font-mono text-slate-200 truncate max-w-[170px]">{{ hoveredShard.index }}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Size:</span> <span class="font-mono text-slate-200">{{ hoveredShard.store || '-' }}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Node:</span> <span class="text-slate-200">{{ hoveredShard.node || '-' }}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">IP:</span> <span class="font-mono text-slate-200">{{ hoveredShard.ip || '-' }}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Docs:</span> <span class="font-mono text-slate-200">{{ hoveredShard.docs || 0 }}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Shard:</span> <span class="font-mono text-slate-200">{{ hoveredShard.shard || 0 }}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Type:</span> <span :class="hoveredShard.prirep === 'p' || hoveredShard.type === 'Primary' ? 'text-emerald-400' : 'text-sky-400'">{{ hoveredShard.prirep === 'p' || hoveredShard.type === 'Primary' ? 'Primary' : 'Replica' }}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">State:</span> <span class="text-emerald-400 uppercase font-semibold">{{ hoveredShard.state || 'STARTED' }}</span></div>
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
              <p class="text-xs text-slate-400">Configure connection endpoints to your OpenSearch / Elasticsearch cluster</p>
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
                <input v-model="configForm.host" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="e.g. 192.168.1.50 or search.example.com" />
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
                <label class="block text-slate-400 mb-1 font-medium">Password (Optional)</label>
                <input v-model="configForm.password" type="password" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" />
              </div>
            </div>

            <div class="flex items-center gap-6 pt-2">
              <label class="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" v-model="configForm.useSsl" class="rounded border-slate-700 text-brand-500 focus:ring-0" />
                <span class="text-slate-300">Use HTTPS / SSL</span>
              </label>
              <label class="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" v-model="configForm.verifySsl" class="rounded border-slate-700 text-brand-500 focus:ring-0" />
                <span class="text-slate-300">Verify SSL Certificate</span>
              </label>
            </div>

            <div v-if="testResult" :class="['p-3 rounded-lg text-xs', testSuccess ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-400' : 'bg-red-500/10 border border-red-500/20 text-red-400']">
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
                class="px-5 py-2 bg-brand-500 hover:bg-brand-600 disabled:opacity-50 text-white font-medium rounded-lg shadow-lg shadow-brand-500/20 transition"
              >
                {{ isSavingConfig ? 'Saving...' : 'Save Configuration' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 6: CLUSTER LOGS -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'logs'" class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-4 shadow-xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-xs font-bold text-white tracking-wide flex items-center gap-2">
            <Terminal class="w-4 h-4 text-brand-400" />
            Cluster Event & Query Logs
          </h3>
          <span class="text-[11px] text-slate-500 font-mono">Real-time OpenSearch diagnostic stream</span>
        </div>

        <div v-if="logsList.length > 0" class="font-mono text-xs bg-[#14161b] p-4 rounded-lg border border-slate-800/80 space-y-2 text-slate-400">
          <div v-for="(log, lIdx) in logsList" :key="lIdx" class="flex items-start gap-3">
            <span class="text-slate-600">{{ log.time }}</span>
            <span :class="['px-1.5 py-0.2 rounded font-semibold text-[10px]', log.level === 'ERROR' ? 'bg-red-500/10 text-red-400' : log.level === 'WARN' ? 'bg-amber-500/10 text-amber-400' : 'bg-emerald-500/10 text-emerald-400']">{{ log.level }}</span>
            <span class="text-slate-300">{{ log.message }}</span>
          </div>
        </div>
        <div v-else class="text-center py-8 text-slate-500 text-xs">
          No logs available. Connect an OpenSearch cluster to stream logs.
        </div>
      </div>
    </main>
  </div>
</template>
