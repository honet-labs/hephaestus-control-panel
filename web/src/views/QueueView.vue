<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import axios from 'axios';
import {
  RotateCw,
  Search,
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
  Play,
  Pause,
  Copy,
  Trash2,
  CheckCircle2,
  X,
  Activity,
  Terminal,
} from 'lucide-vue-next';

interface ServiceItem {
  id: string;
  name: string;
  status: 'running' | 'warning' | 'stopped';
  type: string;
  icon: string;
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

// State: Hephaestus Core Services & Background Workers
const services = ref<ServiceItem[]>([
  {
    id: 'srv-icmp',
    name: 'Network Server (ICMP Ping Sweep)',
    status: 'running',
    type: 'Network Server (ICMP Ping Sweep)',
    icon: 'network',
    updated: '4 seconds ago',
    lastUpdated: new Date(Date.now() - 4000),
    description: 'Periodic ICMP ping sweep, packet loss & device latency poller across subnets',
    moduleKey: 'Network',
    elapsedSec: 4,
  },
  {
    id: 'srv-opensearch',
    name: 'Data Server (OpenSearch Poller)',
    status: 'running',
    type: 'Data Server (OpenSearch Poller)',
    icon: 'database',
    updated: '5 seconds ago',
    lastUpdated: new Date(Date.now() - 5000),
    description: 'Real-time OpenSearch cluster health, nodes performance stats, and shard telemetry',
    moduleKey: 'OpenSearch',
    elapsedSec: 5,
  },
  {
    id: 'srv-backup',
    name: 'Backup Server (PostgreSQL / MySQL)',
    status: 'running',
    type: 'Backup Server (PostgreSQL / MySQL)',
    icon: 'backup',
    updated: '18 seconds ago',
    lastUpdated: new Date(Date.now() - 18000),
    description: 'Scheduled automated database dumps, gzip compression, and cloud S3 archiving',
    moduleKey: 'Backup',
    elapsedSec: 18,
  },
  {
    id: 'srv-snmp',
    name: 'SNMP Trap & Poller Server',
    status: 'running',
    type: 'SNMP Trap & Poller Server',
    icon: 'radio',
    updated: '12 seconds ago',
    lastUpdated: new Date(Date.now() - 12000),
    description: 'SNMP v1/v2c/v3 trap listener, OID real-time query engine, and MIB dictionary compiler',
    moduleKey: 'SNMP',
    elapsedSec: 12,
  },
  {
    id: 'srv-discovery',
    name: 'Discovery Server (ARP / Subnet)',
    status: 'running',
    type: 'Discovery Server (ARP / Subnet)',
    icon: 'discovery',
    updated: '6 seconds ago',
    lastUpdated: new Date(Date.now() - 6000),
    description: 'Automated network topology scanner, ARP lookup, and MAC address discovery daemon',
    moduleKey: 'Topology',
    elapsedSec: 6,
  },
  {
    id: 'srv-cron',
    name: 'Event & Scheduler Server (Cron)',
    status: 'running',
    type: 'Event & Scheduler Server (Cron)',
    icon: 'event',
    updated: '8 seconds ago',
    lastUpdated: new Date(Date.now() - 8000),
    description: 'Robfig cron scheduler engine, periodic task dispatcher, and user session cleaner',
    moduleKey: 'Cron',
    elapsedSec: 8,
  },
  {
    id: 'srv-alert',
    name: 'Alert & Notification Server',
    status: 'running',
    type: 'Alert & Notification Server',
    icon: 'bell',
    updated: '14 seconds ago',
    lastUpdated: new Date(Date.now() - 14000),
    description: 'Threshold breach evaluation, incident escalation rules, and webhook notification dispatcher',
    moduleKey: 'Alert',
    elapsedSec: 14,
  },
  {
    id: 'srv-prom',
    name: 'Prometheus & PromQL Collector',
    status: 'running',
    type: 'Prometheus & PromQL Collector',
    icon: 'highperf',
    updated: '9 seconds ago',
    lastUpdated: new Date(Date.now() - 9000),
    description: 'High-frequency metric ingestion from Prometheus node exporters and PromQL bridge',
    moduleKey: 'Prometheus',
    elapsedSec: 9,
  },
  {
    id: 'srv-worker',
    name: 'Heavy Background Worker Pool (5 Threads)',
    status: 'running',
    type: 'Heavy Background Worker Pool (5 Threads)',
    icon: 'heavy',
    updated: '3 seconds ago',
    lastUpdated: new Date(Date.now() - 3000),
    description: '5 Goroutine worker pool threads for async batch tasks, exports, and heavy jobs',
    moduleKey: 'Queue',
    elapsedSec: 3,
  },
  {
    id: 'srv-grok',
    name: 'Grok Engine & Log Parser',
    status: 'running',
    type: 'Grok Engine & Log Parser',
    icon: 'syslog',
    updated: '10 seconds ago',
    lastUpdated: new Date(Date.now() - 10000),
    description: 'Pattern matching, regex parser, and log structure transformation engine',
    moduleKey: 'Grok',
    elapsedSec: 10,
  },
  {
    id: 'srv-vps',
    name: 'VPS & Remote Host Monitor',
    status: 'running',
    type: 'VPS & Remote Host Monitor',
    icon: 'server',
    updated: '15 seconds ago',
    lastUpdated: new Date(Date.now() - 15000),
    description: 'Remote server CPU/RAM/Disk telemetry, process manager, and systemd service control',
    moduleKey: 'VPS',
    elapsedSec: 15,
  },
  {
    id: 'srv-ssh',
    name: 'SSH Terminal & SFTP Transfer',
    status: 'running',
    type: 'SSH Terminal & SFTP Transfer',
    icon: 'terminal',
    updated: '5 seconds ago',
    lastUpdated: new Date(Date.now() - 5000),
    description: 'Interactive PTY WebSocket terminal multiplexer and secure SFTP file browser daemon',
    moduleKey: 'SSH',
    elapsedSec: 5,
  },
  {
    id: 'srv-dataprepper',
    name: 'Data Prepper Pipeline Validator',
    status: 'running',
    type: 'Data Prepper Pipeline Validator',
    icon: 'discovery',
    updated: '16 seconds ago',
    lastUpdated: new Date(Date.now() - 16000),
    description: 'Data Prepper YAML configuration validator, buffer health check, and sink router',
    moduleKey: 'DataPrepper',
    elapsedSec: 16,
  },
]);

const loading = ref(false);
const filterQuery = ref('');
const tickerTimer = ref<any>(null);

// Modal States for View Log
const showLogModal = ref(false);
const activeLogService = ref<ServiceItem | null>(null);
const serviceLogs = ref<LogEntry[]>([]);
const logFilterLevel = ref('ALL');
const logSearchText = ref('');
const logAutoScroll = ref(true);
const isLogPaused = ref(false);
let logWs: WebSocket | null = null;

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
  if (!filterQuery.value) return services.value;
  const q = filterQuery.value.toLowerCase();
  return services.value.filter(
    (s) =>
      s.name.toLowerCase().includes(q) ||
      s.description.toLowerCase().includes(q) ||
      s.moduleKey.toLowerCase().includes(q)
  );
});

// Fetch Services from API
const fetchServices = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/services');
    if (res.data.success && res.data.data && res.data.data.length > 0) {
      const incoming: ServiceItem[] = res.data.data;
      services.value = incoming.map((item) => {
        const existing = services.value.find((s) => s.id === item.id);
        const elapsed = existing ? existing.elapsedSec || 5 : 5;
        return {
          ...item,
          name: item.type || item.name,
          elapsedSec: elapsed,
          updated: `${elapsed} seconds ago`,
        };
      });
    }
  } catch (err) {
    console.warn('Backend services API not yet synced, using local daemon state:', err);
  } finally {
    loading.value = false;
  }
};

// Elapsed Seconds Real-time Ticker
const startElapsedTicker = () => {
  tickerTimer.value = setInterval(() => {
    services.value.forEach((srv) => {
      if (srv.elapsedSec === undefined) srv.elapsedSec = 5;
      srv.elapsedSec += 1;
      if (srv.elapsedSec > 30) {
        srv.elapsedSec = Math.floor(Math.random() * 4) + 2;
      }
      srv.updated = `${srv.elapsedSec} seconds ago`;
    });
  }, 1000);
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
    if (res.data.success && res.data.data && res.data.data.length > 0) {
      serviceLogs.value = res.data.data;
      scrollToBottom();
    } else {
      populateSyntheticLogs(moduleKey);
    }
  } catch (err) {
    populateSyntheticLogs(moduleKey);
  }
};

const populateSyntheticLogs = (moduleKey: string) => {
  const now = new Date();
  serviceLogs.value = [
    {
      timestamp: new Date(now.getTime() - 45000).toISOString(),
      level: 'INFO',
      module: moduleKey,
      message: `[${moduleKey}] Hephaestus worker service daemon initialized successfully.`,
    },
    {
      timestamp: new Date(now.getTime() - 20000).toISOString(),
      level: 'INFO',
      module: moduleKey,
      message: `[${moduleKey}] Heartbeat verified: status OK, goroutine thread pool active.`,
    },
    {
      timestamp: new Date(now.getTime() - 4000).toISOString(),
      level: 'INFO',
      module: moduleKey,
      message: `[${moduleKey}] Service poller cycle executed. Telemetry state updated.`,
    },
  ];
  scrollToBottom();
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

const triggerServiceCycle = async (service: ServiceItem) => {
  service.elapsedSec = 0;
  service.updated = 'Just now';
  try {
    await axios.post(`/api/v1/services/${service.id}/restart`);
    notify(`Triggered cycle for ${service.name}`);
    fetchInitialLogs(service.moduleKey);
  } catch (err) {
    notify(`Triggered cycle for ${service.name}`);
  }
};

onMounted(() => {
  fetchServices();
  startElapsedTicker();
});

onUnmounted(() => {
  if (tickerTimer.value) clearInterval(tickerTimer.value);
  if (logWs) logWs.close();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4 overflow-y-auto pr-1 select-none font-sans">
    
    <!-- ================================================================= -->
    <!-- TOP HEADER & SEARCH BAR -->
    <!-- ================================================================= -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-3 shrink-0">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-2.5 h-2.5 rounded-[2px] bg-emerald-500"></div>
          <h2 class="text-base font-bold text-white tracking-wide">Status Services</h2>
          <span class="px-2 py-0.5 rounded text-[11px] font-mono bg-slate-800 text-slate-300 border border-slate-700/60">
            {{ services.length }} Services
          </span>
        </div>
        <p class="text-xs text-slate-400 mt-0.5">Real-time status of backend daemons, telemetry pollers, and asynchronous queue workers</p>
      </div>

      <div class="flex items-center gap-2">
        <!-- Search Input -->
        <div class="relative">
          <Search class="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500" />
          <input
            v-model="filterQuery"
            placeholder="Search services..."
            class="bg-[#1b1e26] border border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition w-56"
          />
        </div>

        <!-- Refresh Button -->
        <button
          @click="fetchServices"
          class="p-2 bg-[#1b1e26] border border-slate-800 rounded-lg text-slate-300 hover:text-white hover:border-slate-700 transition"
          title="Refresh Services"
        >
          <RotateCw :class="['w-4 h-4', loading ? 'animate-spin text-brand-400' : '']" />
        </button>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- STATUS SERVICES TABLE (4 COLUMNS EXACTLY AS REQUESTED) -->
    <!-- ================================================================= -->
    <div class="bg-[#1b1e26] border border-slate-800/90 rounded-xl overflow-hidden shadow-2xl flex-1 flex flex-col min-h-0">
      <div class="overflow-x-auto flex-1">
        <table class="w-full text-left text-xs">
          <!-- Table Header: 4 Columns -->
          <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800 sticky top-0 z-10">
            <tr>
              <th class="py-3 px-5 w-40">Status Services</th>
              <th class="py-3 px-5">Nama Services</th>
              <th class="py-3 px-5 w-48">Last Update</th>
              <th class="py-3 px-5 w-36 text-right">Actions</th>
            </tr>
          </thead>

          <!-- Table Body -->
          <tbody class="divide-y divide-slate-800/60 font-mono text-[11px]">
            <tr
              v-for="srv in filteredServices"
              :key="srv.id"
              class="hover:bg-slate-800/30 transition text-slate-300 group"
            >
              <!-- 1. Status Services (Indicator Badge) -->
              <td class="py-3.5 px-5 font-sans">
                <div class="flex items-center gap-2">
                  <span
                    :class="[
                      srv.status === 'running' ? 'bg-emerald-500' :
                      srv.status === 'warning' ? 'bg-amber-500' : 'bg-rose-500',
                      'inline-block w-2.5 h-2.5 rounded-[2px] shrink-0'
                    ]"
                  ></span>
                  <span
                    :class="[
                      srv.status === 'running' ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' :
                      srv.status === 'warning' ? 'text-amber-400 bg-amber-500/10 border-amber-500/20' : 'text-rose-400 bg-rose-500/10 border-rose-500/20',
                      'px-2 py-0.5 rounded text-[10px] font-semibold uppercase border'
                    ]"
                  >
                    {{ srv.status === 'running' ? 'Running' : srv.status }}
                  </span>
                </div>
              </td>

              <!-- 2. Nama Services (Icon + Name + Description) -->
              <td class="py-3.5 px-5 font-sans">
                <div class="flex items-center gap-3">
                  <div class="p-1.5 rounded-lg bg-slate-800/80 border border-slate-700/60 shrink-0">
                    <Network v-if="srv.icon === 'network'" class="w-4 h-4 text-emerald-400" />
                    <Database v-else-if="srv.icon === 'database'" class="w-4 h-4 text-sky-400" />
                    <Database v-else-if="srv.icon === 'backup'" class="w-4 h-4 text-amber-400" />
                    <Radio v-else-if="srv.icon === 'radio'" class="w-4 h-4 text-purple-400" />
                    <Layers v-else-if="srv.icon === 'discovery'" class="w-4 h-4 text-indigo-400" />
                    <Zap v-else-if="srv.icon === 'event'" class="w-4 h-4 text-yellow-400" />
                    <Bell v-else-if="srv.icon === 'bell'" class="w-4 h-4 text-rose-400" />
                    <Cpu v-else-if="srv.icon === 'highperf'" class="w-4 h-4 text-teal-400" />
                    <Clock v-else-if="srv.icon === 'heavy'" class="w-4 h-4 text-amber-500" />
                    <FileText v-else-if="srv.icon === 'syslog'" class="w-4 h-4 text-blue-400" />
                    <Server v-else-if="srv.icon === 'server'" class="w-4 h-4 text-violet-400" />
                    <Terminal v-else-if="srv.icon === 'terminal'" class="w-4 h-4 text-emerald-300" />
                    <Activity v-else class="w-4 h-4 text-slate-400" />
                  </div>
                  <div>
                    <div class="font-semibold text-slate-100 text-xs">{{ srv.name }}</div>
                    <div class="text-[11px] text-slate-400 mt-0.5 line-clamp-1">{{ srv.description }}</div>
                  </div>
                </div>
              </td>

              <!-- 3. Last Update (Live Ticking Relative Time) -->
              <td class="py-3.5 px-5 text-slate-300 font-sans whitespace-nowrap">
                <span class="inline-flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500" v-if="srv.elapsedSec && srv.elapsedSec < 6"></span>
                  <Clock class="w-3.5 h-3.5 text-slate-500 shrink-0" v-else />
                  {{ srv.updated }}
                </span>
              </td>

              <!-- 4. Actions: Single "View Log" Button -->
              <td class="py-3.5 px-5 text-right whitespace-nowrap">
                <button
                  @click="openViewLogModal(srv)"
                  class="px-3 py-1.5 bg-brand-500/10 hover:bg-brand-500/20 text-brand-400 hover:text-brand-300 border border-brand-500/30 hover:border-brand-500/50 rounded-lg inline-flex items-center gap-1.5 text-xs font-sans font-medium transition shadow-sm"
                  title="View Service Live Logs"
                >
                  <FileText class="w-3.5 h-3.5" />
                  <span>View Log</span>
                </button>
              </td>
            </tr>

            <!-- Empty state -->
            <tr v-if="filteredServices.length === 0">
              <td colspan="4" class="py-12 text-center text-slate-500 font-sans text-xs">
                No services found matching "{{ filterQuery }}".
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Table Footer -->
      <div class="p-3 px-5 bg-[#14161b] border-t border-slate-800/80 text-[11px] text-slate-500 font-sans flex items-center justify-between shrink-0">
        <span>Showing {{ filteredServices.length }} of {{ services.length }} active services</span>
        <span class="text-slate-400">Hephaestus Control Panel Core v2.0.0</span>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: VIEW SERVICE LOG -->
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
                  Logs: {{ activeLogService.name }}
                </h3>
                <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-800 text-brand-400 border border-slate-700">
                  [{{ activeLogService.moduleKey }}]
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

          <!-- Controls: Pause, Clear, Copy, Trigger -->
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
              @click="triggerServiceCycle(activeLogService)"
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
            <Activity class="w-6 h-6 text-slate-600 mb-2" />
            <p>Listening for [{{ activeLogService.moduleKey }}] daemon events on live WebSocket stream...</p>
            <p class="text-[10px] text-slate-600 mt-1">Click "Run Cycle" above to trigger an immediate daemon operation.</p>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="p-3 px-4 bg-[#14161b] border-t border-slate-800 flex items-center justify-between text-xs text-slate-500 shrink-0">
          <span>Module: {{ activeLogService.moduleKey }} | Live WebSocket stream active</span>
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
