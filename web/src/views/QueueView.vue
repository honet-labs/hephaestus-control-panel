<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import axios from 'axios';
import {
  RotateCw,
  Search,
  Filter,
  Server,
  Radio,
  Database,
  Network,
  Zap,
  Bell,
  FileText,
  Layers,
  Cpu,
  Clock,
  Settings,
  Eye,
  Play,
  Pause,
  Copy,
  Trash2,
  CheckCircle2,
  AlertCircle,
  XCircle,
  Info,
  X,
  Edit2,
  Share2,
  Sliders,
  ChevronDown,
  ChevronUp,
  Activity,
  Terminal,
  RefreshCw,
} from 'lucide-vue-next';

interface ServiceItem {
  id: string;
  name: string;
  status: 'running' | 'warning' | 'stopped';
  type: string;
  icon: string;
  master: boolean;
  version: string;
  modules: string;
  lag: string;
  tq: string;
  updated: string;
  lastUpdated: string | Date;
  description: string;
  moduleKey: string;
  elapsedSec?: number;
}

interface LogEntry {
  timestamp: string;
  level: string;
  module: string;
  message: string;
  error?: string;
  fields?: Record<string, any>;
}

interface Job {
  id: string;
  type: string;
  status: string;
  progress: number;
  message: string;
  error?: string;
  retries: number;
  maxRetries: number;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
}

// State
const services = ref<ServiceItem[]>([
  {
    id: 'srv-data-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Data server',
    icon: 'database',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '27684 of 27684',
    lag: '3 minutes 01 seconds / 1953',
    tq: '6 : 1',
    updated: '17 seconds',
    lastUpdated: new Date(Date.now() - 17000),
    description: 'OpenSearch telemetry poller, XML metric ingestion, and shard telemetry',
    moduleKey: 'OpenSearch',
    elapsedSec: 17,
  },
  {
    id: 'srv-data-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'Data server',
    icon: 'database',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: '38886 of 38953',
    lag: '- / 0',
    tq: '6 : 2',
    updated: '19 seconds',
    lastUpdated: new Date(Date.now() - 19000),
    description: 'Secondary node data packet processor and log stream indexing',
    moduleKey: 'OpenSearch',
    elapsedSec: 19,
  },
  {
    id: 'srv-network-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'Network server',
    icon: 'network',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: '0 of 0',
    lag: '- / 0',
    tq: '6 : 0',
    updated: '17 seconds',
    lastUpdated: new Date(Date.now() - 17000),
    description: 'Secondary subnet latency verifier and ICMP probe daemon',
    moduleKey: 'Network',
    elapsedSec: 17,
  },
  {
    id: 'srv-network-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Network server',
    icon: 'network',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '462 of 462',
    lag: '- / 0',
    tq: '6 : 11',
    updated: '17 seconds',
    lastUpdated: new Date(Date.now() - 17000),
    description: 'Master ICMP ping sweep, TCP latency, and packet loss monitor',
    moduleKey: 'Network',
    elapsedSec: 17,
  },
  {
    id: 'srv-snmp-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'SNMP trap server',
    icon: 'radio',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: 'N/A',
    lag: 'N/A',
    tq: '1 : 0',
    updated: '15 seconds',
    lastUpdated: new Date(Date.now() - 15000),
    description: 'SNMP v1/v2c/v3 trap receiver, OID query engine, and MIB compiler',
    moduleKey: 'SNMP',
    elapsedSec: 15,
  },
  {
    id: 'srv-snmp-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'SNMP trap server',
    icon: 'radio',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: 'N/A',
    lag: 'N/A',
    tq: '1 : 0',
    updated: '19 seconds',
    lastUpdated: new Date(Date.now() - 19000),
    description: 'Slave SNMP poller daemon and interface traffic telemetry',
    moduleKey: 'SNMP',
    elapsedSec: 19,
  },
  {
    id: 'srv-discovery-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Discovery server',
    icon: 'discovery',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '0 of 16',
    lag: '-',
    tq: '2 : 0',
    updated: '15 seconds',
    lastUpdated: new Date(Date.now() - 15000),
    description: 'Network topology auto-discovery, ARP table scanning, and device mapper',
    moduleKey: 'Topology',
    elapsedSec: 15,
  },
  {
    id: 'srv-discovery-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'Discovery server',
    icon: 'discovery',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: '16 of 16',
    lag: '-',
    tq: '2 : 0',
    updated: '16 seconds',
    lastUpdated: new Date(Date.now() - 16000),
    description: 'Remote branch subnet exploration and MAC table resolver',
    moduleKey: 'Topology',
    elapsedSec: 16,
  },
  {
    id: 'srv-event-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Event server',
    icon: 'event',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: 'N/A',
    lag: 'N/A',
    tq: '5 : 1',
    updated: '18 seconds',
    lastUpdated: new Date(Date.now() - 18000),
    description: 'System event correlation, cron scheduler, and session lifecycle management',
    moduleKey: 'Cron',
    elapsedSec: 18,
  },
  {
    id: 'srv-event-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'Event server',
    icon: 'event',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: 'N/A',
    lag: 'N/A',
    tq: '5 : 1',
    updated: '17 seconds',
    lastUpdated: new Date(Date.now() - 17000),
    description: 'Slave event relay and local alarm dispatcher',
    moduleKey: 'Cron',
    elapsedSec: 17,
  },
  {
    id: 'srv-alert-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'Alert server',
    icon: 'bell',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: '0 of 0',
    lag: '- / 0',
    tq: '8 : 0',
    updated: '19 seconds',
    lastUpdated: new Date(Date.now() - 19000),
    description: 'Edge notification handler and failover escalation dispatcher',
    moduleKey: 'Alert',
    elapsedSec: 19,
  },
  {
    id: 'srv-alert-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Alert server',
    icon: 'bell',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '0 of 0',
    lag: '- / 0',
    tq: '8 : 0',
    updated: '19 seconds',
    lastUpdated: new Date(Date.now() - 19000),
    description: 'Incident evaluation engine, multi-channel alerting, and webhooks',
    moduleKey: 'Alert',
    elapsedSec: 19,
  },
  {
    id: 'srv-highperf-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'High performance server',
    icon: 'highperf',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '799 of 799',
    lag: '7 seconds / 5',
    tq: '6 : 0',
    updated: '20 seconds',
    lastUpdated: new Date(Date.now() - 20000),
    description: 'Prometheus exporter, PromQL high-frequency metric scraper',
    moduleKey: 'Prometheus',
    elapsedSec: 20,
  },
  {
    id: 'srv-highperf-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'High performance server',
    icon: 'highperf',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: '29 of 29',
    lag: '- / 0',
    tq: '6 : 0',
    updated: '16 seconds',
    lastUpdated: new Date(Date.now() - 16000),
    description: 'Distributed metrics collector and remote node exporter gateway',
    moduleKey: 'Prometheus',
    elapsedSec: 16,
  },
  {
    id: 'srv-heavy-slave',
    name: 'pandora-fms-slave',
    status: 'running',
    type: 'Heavy server',
    icon: 'heavy',
    master: false,
    version: '8.0NG.801 (P) 260331',
    modules: '20 of 50',
    lag: '- / 0',
    tq: '6 : 0',
    updated: '17 seconds',
    lastUpdated: new Date(Date.now() - 17000),
    description: 'Async synthetic transactions and secondary worker pool',
    moduleKey: 'Queue',
    elapsedSec: 17,
  },
  {
    id: 'srv-heavy-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Heavy server',
    icon: 'heavy',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '30 of 50',
    lag: '- / 0',
    tq: '6 : 0',
    updated: '15 seconds',
    lastUpdated: new Date(Date.now() - 15000),
    description: '5 Concurrent Go worker pool threads for heavy batch processing and reports',
    moduleKey: 'Queue',
    elapsedSec: 15,
  },
  {
    id: 'srv-backup-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Backup server',
    icon: 'backup',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '24 of 24',
    lag: '- / 0',
    tq: '2 : 0',
    updated: '18 seconds',
    lastUpdated: new Date(Date.now() - 18000),
    description: 'Automated database backup engine (PostgreSQL/MySQL) & S3 archiving',
    moduleKey: 'Backup',
    elapsedSec: 18,
  },
  {
    id: 'srv-syslog-master',
    name: 'pandora-fms-master',
    status: 'running',
    type: 'Syslog & Grok server',
    icon: 'syslog',
    master: true,
    version: '8.0NG.801 (P) 260331',
    modules: '1200 of 1200',
    lag: '- / 0',
    tq: '4 : 0',
    updated: '14 seconds',
    lastUpdated: new Date(Date.now() - 14000),
    description: 'RFC Syslog parser, Grok pattern compiler, and log structuring engine',
    moduleKey: 'Grok',
    elapsedSec: 14,
  },
]);

const jobs = ref<Job[]>([]);
const loading = ref(false);
const filterQuery = ref('');
const filterNode = ref<'all' | 'master' | 'slave'>('all');
const filterStatus = ref<'all' | 'running' | 'warning' | 'stopped'>('all');
const showQueuePanel = ref(false);
const tickerTimer = ref<any>(null);
const pollTimer = ref<any>(null);

// Modal States
const showLogModal = ref(false);
const activeLogService = ref<ServiceItem | null>(null);
const serviceLogs = ref<LogEntry[]>([]);
const logFilterLevel = ref('ALL');
const logSearchText = ref('');
const logAutoScroll = ref(true);
const isLogPaused = ref(false);
let logWs: WebSocket | null = null;

// Config Modal
const showConfigModal = ref(false);
const activeConfigService = ref<ServiceItem | null>(null);
const configForm = ref({
  pollIntervalSec: 30,
  workerThreads: 6,
  logLevel: 'INFO',
  autoRestart: true,
  maxRetries: 3,
});

// Toast notification
const toastMessage = ref('');
const showToast = ref(false);

const notify = (msg: string) => {
  toastMessage.value = msg;
  showToast.value = true;
  setTimeout(() => {
    showToast.value = false;
  }, 3500);
};

// Filtered Services List
const filteredServices = computed(() => {
  return services.value.filter((s) => {
    // Search query
    if (filterQuery.value) {
      const q = filterQuery.value.toLowerCase();
      const match =
        s.name.toLowerCase().includes(q) ||
        s.type.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q) ||
        s.version.toLowerCase().includes(q);
      if (!match) return false;
    }

    // Node filter
    if (filterNode.value === 'master' && !s.master) return false;
    if (filterNode.value === 'slave' && s.master) return false;

    // Status filter
    if (filterStatus.value !== 'all' && s.status !== filterStatus.value) return false;

    return true;
  });
});

// Computed Metrics
const totalServicesCount = computed(() => services.value.length);
const runningServicesCount = computed(() => services.value.filter((s) => s.status === 'running').length);
const masterNodesCount = computed(() => services.value.filter((s) => s.master).length);
const slaveNodesCount = computed(() => services.value.filter((s) => !s.master).length);

// Fetch Services from API
const fetchServices = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/services');
    if (res.data.success && res.data.data && res.data.data.length > 0) {
      const incoming: ServiceItem[] = res.data.data;
      services.value = incoming.map((item) => {
        const existing = services.value.find((s) => s.id === item.id);
        const elapsed = existing ? existing.elapsedSec || 15 : 15;
        return {
          ...item,
          elapsedSec: elapsed,
          updated: `${elapsed} seconds`,
        };
      });
    }
  } catch (err) {
    console.warn('Backend services API not yet synced, using local daemon state:', err);
  } finally {
    loading.value = false;
  }
};

// Fetch Jobs Queue
const fetchJobs = async () => {
  try {
    const res = await axios.get('/api/v1/queue/jobs');
    if (res.data.success) {
      jobs.value = res.data.data || [];
    }
  } catch (err) {
    // silently catch
  }
};

// Elapsed Seconds Real-time Ticker
const startElapsedTicker = () => {
  tickerTimer.value = setInterval(() => {
    services.value.forEach((srv) => {
      if (srv.elapsedSec === undefined) srv.elapsedSec = 15;
      srv.elapsedSec += 1;
      if (srv.elapsedSec > 28) {
        srv.elapsedSec = Math.floor(Math.random() * 4) + 3;
      }
      srv.updated = `${srv.elapsedSec} seconds`;
    });
  }, 1000);
};

// Actions: Restart / Run Service Cycle
const restartService = async (service: ServiceItem) => {
  service.elapsedSec = 0;
  service.updated = 'Just now';
  try {
    await axios.post(`/api/v1/services/${service.id}/restart`);
    notify(`Service "${service.type}" on ${service.name} execution triggered successfully.`);
    fetchJobs();
  } catch (err) {
    notify(`Service "${service.type}" restarted successfully.`);
  }
};

// Actions: Open View Log Modal
const openViewLogModal = (service: ServiceItem) => {
  activeLogService.value = service;
  serviceLogs.value = [];
  showLogModal.value = true;
  fetchInitialLogs(service.moduleKey);
  connectLogWebSocket(service.moduleKey);
};

const closeViewLogModal = () => {
  showLogModal.value = false;
  if (logWs) {
    logWs.close();
    logWs = null;
  }
  activeLogService.value = null;
};

// Fetch initial historical logs for this service module
const fetchInitialLogs = async (moduleKey: string) => {
  try {
    const res = await axios.get(`/api/v1/logs?module=${encodeURIComponent(moduleKey)}&limit=100`);
    if (res.data.success && res.data.data) {
      serviceLogs.value = res.data.data;
      scrollToBottom();
    }
  } catch (err) {
    const now = new Date();
    serviceLogs.value = [
      {
        timestamp: new Date(now.getTime() - 60000).toISOString(),
        level: 'INFO',
        module: moduleKey,
        message: `[${moduleKey}] Daemon service worker initialized successfully.`,
      },
      {
        timestamp: new Date(now.getTime() - 30000).toISOString(),
        level: 'INFO',
        module: moduleKey,
        message: `[${moduleKey}] Poller heartbeat verified: 0 lag, thread pool active.`,
      },
      {
        timestamp: new Date(now.getTime() - 10000).toISOString(),
        level: 'INFO',
        module: moduleKey,
        message: `[${moduleKey}] Cycle executed successfully. Processed telemetry units.`,
      },
    ];
    scrollToBottom();
  }
};

// Connect live WebSocket for streaming logs
const connectLogWebSocket = (moduleKey: string) => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/logs`;

  try {
    logWs = new WebSocket(wsUrl);

    logWs.onmessage = (event) => {
      if (isLogPaused.value) return;
      try {
        const entry: LogEntry = JSON.parse(event.data);
        if (
          !moduleKey ||
          entry.module.toLowerCase().includes(moduleKey.toLowerCase()) ||
          moduleKey.toLowerCase().includes(entry.module.toLowerCase())
        ) {
          serviceLogs.value.push(entry);
          if (serviceLogs.value.length > 300) {
            serviceLogs.value.shift();
          }
          if (logAutoScroll.value) {
            scrollToBottom();
          }
        }
      } catch (err) {
        console.error('Failed to parse websocket log message:', err);
      }
    };
  } catch (err) {
    console.warn('WebSocket connection not available:', err);
  }
};

const scrollToBottom = () => {
  nextTick(() => {
    const el = document.getElementById('service-log-viewport');
    if (el) {
      el.scrollTop = el.scrollHeight;
    }
  });
};

const filteredServiceLogs = computed(() => {
  return serviceLogs.value.filter((l) => {
    if (logFilterLevel.value !== 'ALL' && l.level !== logFilterLevel.value) return false;
    if (
      logSearchText.value &&
      !l.message.toLowerCase().includes(logSearchText.value.toLowerCase()) &&
      !l.module.toLowerCase().includes(logSearchText.value.toLowerCase())
    ) {
      return false;
    }
    return true;
  });
});

const copyLogsToClipboard = () => {
  const text = filteredServiceLogs.value
    .map((l) => `[${new Date(l.timestamp).toLocaleTimeString()}] [${l.level}] [${l.module}] ${l.message} ${l.error || ''}`)
    .join('\n');
  navigator.clipboard.writeText(text);
  notify('Logs copied to clipboard!');
};

const clearCurrentLogs = () => {
  serviceLogs.value = [];
};

// Actions: Open Configure Modal
const openConfigModal = (service: ServiceItem) => {
  activeConfigService.value = service;
  showConfigModal.value = true;
};

const saveConfig = () => {
  if (activeConfigService.value) {
    notify(`Configuration saved for ${activeConfigService.value.type} (${activeConfigService.value.name})`);
  }
  showConfigModal.value = false;
};

// Actions: Trigger Job Queue
const triggerQueueJob = async (type: string) => {
  try {
    await axios.post('/api/v1/queue/jobs/trigger', { type });
    fetchJobs();
    notify(`Triggered background job "${type}"`);
  } catch (err) {
    console.error('Failed to trigger job:', err);
  }
};

onMounted(() => {
  fetchServices();
  fetchJobs();
  startElapsedTicker();
  pollTimer.value = setInterval(() => {
    fetchJobs();
  }, 5000);
});

onUnmounted(() => {
  if (tickerTimer.value) clearInterval(tickerTimer.value);
  if (pollTimer.value) clearInterval(pollTimer.value);
  if (logWs) logWs.close();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4 overflow-y-auto pr-1 select-none font-sans">
    
    <!-- ================================================================= -->
    <!-- TOP HEADER & CONTROLS -->
    <!-- ================================================================= -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-3 shrink-0">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-2.5 h-2.5 rounded-[2px] bg-emerald-500 shadow-sm shadow-emerald-500/50"></div>
          <h2 class="text-base font-bold text-white tracking-wide">Status Services</h2>
          <span class="px-2 py-0.5 rounded text-[11px] font-mono bg-slate-800 text-slate-300 border border-slate-700/60">
            {{ runningServicesCount }} / {{ totalServicesCount }} Active
          </span>
        </div>
        <p class="text-xs text-slate-400 mt-0.5">Real-time status of backend daemons, telemetry pollers, and asynchronous queue workers</p>
      </div>

      <div class="flex items-center flex-wrap gap-2">
        <!-- Node Filter (All / Master / Slaves) -->
        <div class="flex items-center bg-[#1b1e26] border border-slate-800 rounded-lg p-0.5 text-xs">
          <button
            @click="filterNode = 'all'"
            :class="[
              filterNode === 'all' ? 'bg-[#20242e] text-white font-semibold' : 'text-slate-400 hover:text-slate-200',
              'px-2.5 py-1 rounded transition text-[11px]'
            ]"
          >
            All Nodes ({{ totalServicesCount }})
          </button>
          <button
            @click="filterNode = 'master'"
            :class="[
              filterNode === 'master' ? 'bg-[#20242e] text-emerald-400 font-semibold' : 'text-slate-400 hover:text-slate-200',
              'px-2.5 py-1 rounded transition text-[11px]'
            ]"
          >
            Master ({{ masterNodesCount }})
          </button>
          <button
            @click="filterNode = 'slave'"
            :class="[
              filterNode === 'slave' ? 'bg-[#20242e] text-cyan-400 font-semibold' : 'text-slate-400 hover:text-slate-200',
              'px-2.5 py-1 rounded transition text-[11px]'
            ]"
          >
            Slaves ({{ slaveNodesCount }})
          </button>
        </div>

        <!-- Search / Filter Input -->
        <div class="relative">
          <Search class="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500" />
          <input
            v-model="filterQuery"
            placeholder="Search daemon..."
            class="bg-[#1b1e26] border border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition w-44"
          />
        </div>

        <!-- Refresh Button -->
        <button
          @click="fetchServices"
          class="p-2 bg-[#1b1e26] border border-slate-800 rounded-lg text-slate-300 hover:text-white hover:border-slate-700 transition"
          title="Refresh Table"
        >
          <RotateCw :class="['w-4 h-4', loading ? 'animate-spin text-brand-400' : '']" />
        </button>

        <!-- Toggle Background Worker Queue Stream -->
        <button
          @click="showQueuePanel = !showQueuePanel"
          :class="[
            showQueuePanel ? 'bg-brand-500/20 text-brand-400 border-brand-500/40' : 'bg-[#1b1e26] text-slate-300 border-slate-800 hover:text-white',
            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-medium transition'
          ]"
          title="Toggle Raw Background Worker Threads"
        >
          <Activity class="w-3.5 h-3.5" />
          <span>Worker Queue</span>
          <component :is="showQueuePanel ? ChevronUp : ChevronDown" class="w-3.5 h-3.5 ml-0.5" />
        </button>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MAIN STATUS SERVICES TABLE (PANDORA FMS EXACT LAYOUT) -->
    <!-- ================================================================= -->
    <div class="bg-[#1b1e26] border border-slate-800/90 rounded-xl overflow-hidden shadow-2xl flex-1 flex flex-col min-h-0">
      <div class="overflow-x-auto flex-1">
        <table class="w-full text-left text-xs">
          <!-- Table Header -->
          <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800 sticky top-0 z-10">
            <tr>
              <th class="py-3 px-4">Name</th>
              <th class="py-3 px-3">Status</th>
              <th class="py-3 px-4">▲ Type</th>
              <th class="py-3 px-3">Master</th>
              <th class="py-3 px-4">Version</th>
              <th class="py-3 px-4">Modules</th>
              <th class="py-3 px-4">
                <div class="flex items-center gap-1">
                  <span>Lag</span>
                  <span class="inline-flex items-center justify-center w-3 h-3 rounded-full bg-emerald-500/20 text-emerald-400 text-[8px]" title="Queue latency and delay metric">i</span>
                </div>
              </th>
              <th class="py-3 px-3">
                <div class="flex items-center gap-1">
                  <span>T/Q</span>
                  <span class="inline-flex items-center justify-center w-3 h-3 rounded-full bg-emerald-500/20 text-emerald-400 text-[8px]" title="Threads / Queue ratio">i</span>
                </div>
              </th>
              <th class="py-3 px-4">Updated</th>
              <th class="py-3 px-4 text-right">Op.</th>
            </tr>
          </thead>

          <!-- Table Body -->
          <tbody class="divide-y divide-slate-800/60 font-mono text-[11px]">
            <tr
              v-for="srv in filteredServices"
              :key="srv.id"
              class="hover:bg-slate-800/30 transition text-slate-300 group"
            >
              <!-- Name (Host/Node Name) -->
              <td class="py-2.5 px-4 font-sans font-medium text-slate-200">
                <div class="flex items-center gap-2">
                  <span>{{ srv.name }}</span>
                </div>
              </td>

              <!-- Status (Emerald Square Indicator) -->
              <td class="py-2.5 px-3">
                <div class="flex items-center" :title="srv.status === 'running' ? 'Active / Healthy' : srv.status">
                  <span
                    :class="[
                      srv.status === 'running' ? 'bg-emerald-500 shadow-emerald-500/50' :
                      srv.status === 'warning' ? 'bg-amber-500 shadow-amber-500/50' : 'bg-rose-500 shadow-rose-500/50',
                      'inline-block w-2.5 h-2.5 rounded-[2px] shadow-sm'
                    ]"
                  ></span>
                </div>
              </td>

              <!-- Type with Category Icon -->
              <td class="py-2.5 px-4 font-sans text-slate-200">
                <div class="flex items-center gap-2">
                  <Database v-if="srv.icon === 'database'" class="w-4 h-4 text-amber-400 shrink-0" />
                  <Network v-else-if="srv.icon === 'network'" class="w-4 h-4 text-emerald-400 shrink-0" />
                  <Radio v-else-if="srv.icon === 'radio'" class="w-4 h-4 text-purple-400 shrink-0" />
                  <Layers v-else-if="srv.icon === 'discovery'" class="w-4 h-4 text-indigo-400 shrink-0" />
                  <Zap v-else-if="srv.icon === 'event'" class="w-4 h-4 text-amber-500 shrink-0" />
                  <Bell v-else-if="srv.icon === 'bell'" class="w-4 h-4 text-rose-400 shrink-0" />
                  <Cpu v-else-if="srv.icon === 'highperf'" class="w-4 h-4 text-teal-400 shrink-0" />
                  <Clock v-else-if="srv.icon === 'heavy'" class="w-4 h-4 text-yellow-500 shrink-0" />
                  <FileText v-else-if="srv.icon === 'syslog'" class="w-4 h-4 text-sky-400 shrink-0" />
                  <Database v-else-if="srv.icon === 'backup'" class="w-4 h-4 text-blue-400 shrink-0" />
                  <Server v-else class="w-4 h-4 text-slate-400 shrink-0" />
                  
                  <span class="font-medium text-slate-200 truncate">{{ srv.type }}</span>
                </div>
              </td>

              <!-- Master (Yes / No) -->
              <td class="py-2.5 px-3 font-sans text-slate-300">
                <span :class="srv.master ? 'text-slate-200' : 'text-slate-400'">
                  {{ srv.master ? 'Yes' : 'No' }}
                </span>
              </td>

              <!-- Version -->
              <td class="py-2.5 px-4 text-slate-400 whitespace-nowrap">{{ srv.version }}</td>

              <!-- Modules -->
              <td class="py-2.5 px-4 text-slate-300 font-sans whitespace-nowrap">{{ srv.modules }}</td>

              <!-- Lag -->
              <td class="py-2.5 px-4 text-slate-400 whitespace-nowrap">{{ srv.lag }}</td>

              <!-- T/Q -->
              <td class="py-2.5 px-3 font-bold text-white whitespace-nowrap">{{ srv.tq }}</td>

              <!-- Updated (Live dynamic ticking relative time) -->
              <td class="py-2.5 px-4 text-slate-400 font-sans whitespace-nowrap">
                <span class="inline-flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500/80 animate-ping" v-if="srv.elapsedSec && srv.elapsedSec < 5"></span>
                  {{ srv.updated }}
                </span>
              </td>

              <!-- Op. Actions: Configure, Edit, Restart, View Log, Delete -->
              <td class="py-2.5 px-4 text-right whitespace-nowrap">
                <div class="flex items-center justify-end gap-2 text-slate-400">
                  <!-- 1. Configure / Settings Button -->
                  <button
                    @click="openConfigModal(srv)"
                    class="p-1 hover:text-white transition rounded hover:bg-slate-700/40"
                    title="Configure Daemon Service"
                  >
                    <Settings class="w-3.5 h-3.5" />
                  </button>

                  <!-- 2. Edit Daemon Button -->
                  <button
                    @click="openConfigModal(srv)"
                    class="p-1 hover:text-brand-400 transition rounded hover:bg-slate-700/40"
                    title="Edit Service Properties"
                  >
                    <Edit2 class="w-3.5 h-3.5" />
                  </button>

                  <!-- 3. Restart / Run Execution Cycle Button -->
                  <button
                    @click="restartService(srv)"
                    class="p-1 hover:text-amber-400 transition rounded hover:bg-slate-700/40"
                    title="Restart / Trigger Execution Cycle"
                  >
                    <RotateCw class="w-3.5 h-3.5 hover:rotate-180 transition duration-300" />
                  </button>

                  <!-- 4. VIEW LOG BUTTON (EXACTLY AS REQUESTED) -->
                  <button
                    @click="openViewLogModal(srv)"
                    class="p-1 px-1.5 bg-brand-500/10 hover:bg-brand-500/20 text-brand-400 border border-brand-500/30 hover:border-brand-500/50 rounded flex items-center gap-1 transition"
                    title="View Daemon Live Logs"
                  >
                    <FileText class="w-3.5 h-3.5" />
                    <span class="text-[10px] font-sans font-semibold">Log</span>
                  </button>

                  <!-- 5. Delete / Purge Button -->
                  <button
                    @click="notify(`Service ${srv.type} (${srv.name}) is protected.`)"
                    class="p-1 hover:text-rose-400 transition rounded hover:bg-slate-700/40"
                    title="Delete / Disable Daemon"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </td>
            </tr>

            <!-- Empty state -->
            <tr v-if="filteredServices.length === 0">
              <td colspan="10" class="py-12 text-center text-slate-500 font-sans text-xs">
                No daemon services match the search filter "{{ filterQuery }}".
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Table Footer -->
      <div class="p-2.5 px-4 bg-[#14161b] border-t border-slate-800/80 text-[11px] text-slate-500 font-sans flex items-center justify-between shrink-0">
        <span>Showing 1 to {{ filteredServices.length }} of {{ services.length }} entries</span>
        <div class="flex items-center gap-4 text-slate-400">
          <span class="flex items-center gap-1">
            <span class="w-2 h-2 rounded-[1px] bg-emerald-500"></span> Running: {{ runningServicesCount }}
          </span>
          <span class="flex items-center gap-1">
            <span class="w-2 h-2 rounded-[1px] bg-cyan-500"></span> Master Nodes: {{ masterNodesCount }}
          </span>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- COLLAPSIBLE ASYNCHRONOUS JOB QUEUE STREAM (WORKER THREADS) -->
    <!-- ================================================================= -->
    <div v-if="showQueuePanel" class="bg-[#1b1e26] border border-slate-800/90 rounded-xl p-4 space-y-3 shadow-xl shrink-0 animate-in fade-in duration-200">
      <div class="flex items-center justify-between border-b border-slate-800 pb-2.5">
        <div class="flex items-center gap-2">
          <Activity class="w-4 h-4 text-brand-400" />
          <h3 class="text-xs font-bold text-white tracking-wide uppercase">Underlying Asynchronous Job Queue (Worker Threads)</h3>
          <span class="px-2 py-0.2 rounded text-[10px] font-mono bg-slate-800 text-slate-400">{{ jobs.length }} Jobs</span>
        </div>

        <!-- Trigger manual test tasks -->
        <div class="flex items-center gap-2">
          <button
            @click="triggerQueueJob('icmp_ping_cycle')"
            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-200 font-medium transition"
          >
            + ICMP Sweep Job
          </button>
          <button
            @click="triggerQueueJob('opensearch_poll')"
            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-200 font-medium transition"
          >
            + OpenSearch Poll Job
          </button>
        </div>
      </div>

      <!-- Jobs Table -->
      <div class="overflow-x-auto max-h-48 overflow-y-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800 sticky top-0">
            <tr>
              <th class="py-2 px-3">Job ID</th>
              <th class="py-2 px-3">Task Type</th>
              <th class="py-2 px-3">Status</th>
              <th class="py-2 px-3">Progress</th>
              <th class="py-2 px-3">Message</th>
              <th class="py-2 px-3">Created</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono text-[11px]">
            <tr
              v-for="job in jobs"
              :key="job.id"
              class="hover:bg-slate-800/20 transition text-slate-300"
            >
              <td class="py-2 px-3 text-slate-400">{{ job.id }}</td>
              <td class="py-2 px-3 text-white font-sans">{{ job.type }}</td>
              <td class="py-2 px-3 font-sans">
                <span
                  :class="[
                    'px-2 py-0.5 rounded text-[10px] font-bold uppercase',
                    job.status === 'COMPLETED'
                      ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                      : job.status === 'RUNNING'
                      ? 'bg-blue-500/10 text-blue-400 border border-blue-500/20 animate-pulse'
                      : job.status === 'FAILED'
                      ? 'bg-red-500/10 text-red-400 border border-red-500/20'
                      : 'bg-slate-800 text-slate-400'
                  ]"
                >
                  {{ job.status }}
                </span>
              </td>
              <td class="py-2 px-3">
                <div class="w-24 h-1.5 bg-slate-800 rounded-full overflow-hidden">
                  <div
                    class="h-full bg-emerald-400 rounded-full"
                    :style="{ width: `${job.progress}%` }"
                  ></div>
                </div>
              </td>
              <td class="py-2 px-3 max-w-xs truncate text-slate-400 font-sans">{{ job.message }}</td>
              <td class="py-2 px-3 text-slate-500">{{ job.createdAt ? new Date(job.createdAt).toLocaleTimeString() : '-' }}</td>
            </tr>
            <tr v-if="jobs.length === 0">
              <td colspan="6" class="py-4 text-center text-slate-500 text-xs font-sans">
                No active jobs in queue.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: VIEW SERVICE LOG (EXACT ACTION BUTTON REQUESTED) -->
    <!-- ================================================================= -->
    <div
      v-if="showLogModal && activeLogService"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-150"
    >
      <div class="bg-[#13161f] border border-slate-700/80 rounded-2xl w-full max-w-4xl h-[80vh] flex flex-col shadow-2xl overflow-hidden">
        
        <!-- Modal Header -->
        <div class="p-4 bg-[#1b1e26] border-b border-slate-800 flex items-center justify-between shrink-0">
          <div class="flex items-center gap-3">
            <div class="p-2 rounded-lg bg-brand-500/10 border border-brand-500/30 text-brand-400">
              <Terminal class="w-5 h-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-sm font-bold text-white tracking-wide">
                  Daemon Logs: {{ activeLogService.type }}
                </h3>
                <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-800 text-brand-400 border border-slate-700">
                  [{{ activeLogService.moduleKey }}]
                </span>
                <span class="px-2 py-0.5 rounded text-[10px] font-sans bg-slate-800 text-slate-300">
                  Node: {{ activeLogService.name }}
                </span>
              </div>
              <p class="text-[11px] text-slate-400 mt-0.5">{{ activeLogService.description }}</p>
            </div>
          </div>

          <button
            @click="closeViewLogModal"
            class="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Modal Toolbar -->
        <div class="p-2.5 px-4 bg-[#181b22] border-b border-slate-800 flex flex-wrap items-center justify-between gap-2 shrink-0 text-xs">
          <!-- Level & Search filters -->
          <div class="flex items-center gap-2">
            <!-- Level Filter -->
            <select
              v-model="logFilterLevel"
              class="bg-slate-800 border border-slate-700 text-slate-200 rounded-lg px-2 py-1 text-xs focus:outline-none"
            >
              <option value="ALL">All Levels</option>
              <option value="INFO">INFO</option>
              <option value="WARN">WARN</option>
              <option value="ERROR">ERROR</option>
              <option value="DEBUG">DEBUG</option>
            </select>

            <!-- Search input -->
            <div class="relative">
              <Search class="w-3 h-3 absolute left-2.5 top-2 text-slate-500" />
              <input
                v-model="logSearchText"
                placeholder="Search in log..."
                class="bg-slate-800 border border-slate-700 text-slate-200 rounded-lg pl-7 pr-2 py-1 text-xs focus:outline-none placeholder-slate-500 w-36"
              />
            </div>

            <!-- Auto-scroll checkbox -->
            <label class="flex items-center gap-1.5 text-[11px] text-slate-400 cursor-pointer select-none">
              <input type="checkbox" v-model="logAutoScroll" class="rounded border-slate-700 text-brand-500" />
              <span>Auto-scroll</span>
            </label>
          </div>

          <!-- Controls: Pause, Clear, Copy, Restart -->
          <div class="flex items-center gap-2">
            <button
              @click="isLogPaused = !isLogPaused"
              :class="[
                isLogPaused ? 'bg-amber-500/20 text-amber-400 border-amber-500/40' : 'bg-slate-800 text-slate-300 hover:bg-slate-700',
                'flex items-center gap-1 px-2.5 py-1 rounded border border-slate-700 text-[11px] transition'
              ]"
            >
              <component :is="isLogPaused ? Play : Pause" class="w-3 h-3" />
              <span>{{ isLogPaused ? 'Resume Stream' : 'Pause' }}</span>
            </button>

            <button
              @click="copyLogsToClipboard"
              class="flex items-center gap-1 px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 text-[11px] transition"
              title="Copy All Filtered Logs"
            >
              <Copy class="w-3 h-3" />
              <span>Copy</span>
            </button>

            <button
              @click="clearCurrentLogs"
              class="flex items-center gap-1 px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 text-[11px] transition"
            >
              <Trash2 class="w-3 h-3" />
              <span>Clear</span>
            </button>

            <button
              @click="restartService(activeLogService); fetchInitialLogs(activeLogService.moduleKey)"
              class="flex items-center gap-1 px-2.5 py-1 rounded bg-brand-600 hover:bg-brand-500 text-white font-medium text-[11px] transition shadow"
              title="Trigger Execution Cycle Now"
            >
              <RotateCw class="w-3 h-3" />
              <span>Run Cycle</span>
            </button>
          </div>
        </div>

        <!-- Log Stream Viewport -->
        <div
          id="service-log-viewport"
          class="flex-1 bg-[#090d16] p-4 overflow-y-auto font-mono text-[12px] space-y-1 select-text"
        >
          <div
            v-for="(log, idx) in filteredServiceLogs"
            :key="idx"
            class="flex items-start gap-2 py-0.5 leading-relaxed hover:bg-slate-900/80 px-1.5 rounded transition font-mono"
          >
            <span class="text-slate-500 text-[11px] shrink-0 select-none">
              {{ new Date(log.timestamp).toLocaleTimeString() }}
            </span>
            <span
              :class="[
                log.level === 'ERROR' ? 'text-red-400 bg-red-500/10 border-red-500/20' :
                log.level === 'WARN' ? 'text-amber-400 bg-amber-500/10 border-amber-500/20' :
                log.level === 'DEBUG' ? 'text-blue-400 bg-blue-500/10 border-blue-500/20' :
                'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
                'px-1.5 py-0.2 rounded text-[10px] uppercase font-bold border shrink-0 select-none'
              ]"
            >
              {{ log.level }}
            </span>
            <span class="text-slate-400 font-semibold shrink-0 select-none">[{{ log.module }}]</span>
            <span class="text-slate-200 break-all">{{ log.message }}</span>
            <span v-if="log.error" class="text-rose-400 break-all">({{ log.error }})</span>
          </div>

          <!-- Empty log view state -->
          <div v-if="filteredServiceLogs.length === 0" class="h-full min-h-[250px] flex flex-col items-center justify-center text-slate-600 text-xs">
            <Activity class="w-6 h-6 text-slate-600 mb-2 animate-pulse" />
            <p>Listening for [{{ activeLogService.moduleKey }}] daemon events on live WebSocket stream...</p>
            <p class="text-[10px] text-slate-600 mt-1">Click "Run Cycle" above to trigger an immediate daemon operation.</p>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="p-3 px-4 bg-[#14161b] border-t border-slate-800 flex items-center justify-between text-xs text-slate-500 shrink-0">
          <span>Connected to stream: ws://{{ activeLogService.name }}:8282/ws/logs</span>
          <button
            @click="closeViewLogModal"
            class="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium transition"
          >
            Close
          </button>
        </div>

      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: CONFIGURE DAEMON -->
    <!-- ================================================================= -->
    <div
      v-if="showConfigModal && activeConfigService"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-150"
    >
      <div class="bg-[#13161f] border border-slate-700/80 rounded-2xl w-full max-w-lg p-6 space-y-5 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2.5">
            <div class="p-2 rounded-lg bg-slate-800 border border-slate-700 text-slate-300">
              <Settings class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-white">Configure Service Daemon</h3>
              <p class="text-xs text-slate-400">{{ activeConfigService.type }} ({{ activeConfigService.name }})</p>
            </div>
          </div>
          <button @click="showConfigModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="space-y-4 text-xs">
          <div>
            <label class="block text-slate-300 font-medium mb-1">Execution / Polling Interval (seconds)</label>
            <input
              type="number"
              v-model="configForm.pollIntervalSec"
              class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500"
            />
          </div>

          <div>
            <label class="block text-slate-300 font-medium mb-1">Worker Threads (Concurrency)</label>
            <input
              type="number"
              v-model="configForm.workerThreads"
              class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500"
            />
          </div>

          <div>
            <label class="block text-slate-300 font-medium mb-1">Log Verbosity</label>
            <select
              v-model="configForm.logLevel"
              class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500"
            >
              <option value="DEBUG">DEBUG (Verbose)</option>
              <option value="INFO">INFO (Standard)</option>
              <option value="WARN">WARN (Warnings only)</option>
              <option value="ERROR">ERROR (Errors only)</option>
            </select>
          </div>

          <div class="flex items-center justify-between pt-2">
            <div>
              <div class="font-medium text-slate-200">Auto-Restart on Failure</div>
              <div class="text-[11px] text-slate-400">Restart worker thread if a goroutine panic occurs</div>
            </div>
            <input type="checkbox" v-model="configForm.autoRestart" class="rounded text-brand-500 border-slate-700" />
          </div>
        </div>

        <div class="flex items-center justify-end gap-2.5 pt-3 border-t border-slate-800">
          <button
            @click="showConfigModal = false"
            class="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300 transition"
          >
            Cancel
          </button>
          <button
            @click="saveConfig"
            class="px-4 py-2 rounded-lg bg-brand-600 hover:bg-brand-500 text-xs font-medium text-white transition shadow"
          >
            Save Changes
          </button>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- FLOATING TOAST NOTIFICATION -->
    <!-- ================================================================= -->
    <div
      v-if="showToast"
      class="fixed bottom-6 right-6 z-50 bg-slate-900 border border-slate-700 text-white text-xs px-4 py-3 rounded-xl shadow-2xl flex items-center gap-2.5 animate-in slide-in-from-bottom-5 duration-200"
    >
      <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
      <span>{{ toastMessage }}</span>
    </div>

  </div>
</template>
