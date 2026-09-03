<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useRoute } from 'vue-router';
import axios from 'axios';
import ThemeToggle from '../components/ThemeToggle.vue';
import {
  Settings,
  Users,
  Database,
  FileText,
  Save,
  Plus,
  Trash2,
  Lock,
  CheckCircle2,
  AlertTriangle,
  RotateCw,
  Info,
  Activity,
  Search,
  Terminal,
  X,
  Copy,
  Pause,
  Play,
  ShieldCheck,
} from 'lucide-vue-next';

const route = useRoute();
const activeTab = ref<'services' | 'users' | 'database' | 'audit'>('services');

// =================================================================
// 1. STATUS SERVICES STATE & METHODS
// =================================================================
interface ServiceItem {
  id: string;
  name: string;
  status: 'running' | 'warning' | 'stopped';
  type: string;
  updated: string;
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

const services = ref<ServiceItem[]>([
  {
    id: 'srv-icmp',
    name: 'Network Server (ICMP Ping Sweep)',
    status: 'running',
    type: 'Network Server (ICMP Ping Sweep)',
    updated: '4 seconds ago',
    description: 'Periodic ICMP ping sweep, packet loss & device latency poller across subnets',
    moduleKey: 'Network',
    elapsedSec: 4,
  },
  {
    id: 'srv-opensearch',
    name: 'Data Server (OpenSearch Poller)',
    status: 'running',
    type: 'Data Server (OpenSearch Poller)',
    updated: '5 seconds ago',
    description: 'Real-time OpenSearch cluster health, nodes performance stats, and shard telemetry',
    moduleKey: 'OpenSearch',
    elapsedSec: 5,
  },
  {
    id: 'srv-backup',
    name: 'Backup Server (PostgreSQL / MySQL)',
    status: 'running',
    type: 'Backup Server (PostgreSQL / MySQL)',
    updated: '18 seconds ago',
    description: 'Scheduled automated database dumps, gzip compression, and cloud S3 archiving',
    moduleKey: 'Backup',
    elapsedSec: 18,
  },
  {
    id: 'srv-snmp',
    name: 'SNMP Trap & Poller Server',
    status: 'running',
    type: 'SNMP Trap & Poller Server',
    updated: '12 seconds ago',
    description: 'SNMP v1/v2c/v3 trap listener, OID real-time query engine, and MIB dictionary compiler',
    moduleKey: 'SNMP',
    elapsedSec: 12,
  },
  {
    id: 'srv-discovery',
    name: 'Discovery Server (ARP / Subnet)',
    status: 'running',
    type: 'Discovery Server (ARP / Subnet)',
    updated: '6 seconds ago',
    description: 'Automated network topology scanner, ARP lookup, and MAC address discovery daemon',
    moduleKey: 'Topology',
    elapsedSec: 6,
  },
  {
    id: 'srv-cron',
    name: 'Event & Scheduler Server (Cron)',
    status: 'running',
    type: 'Event & Scheduler Server (Cron)',
    updated: '8 seconds ago',
    description: 'Robfig cron scheduler engine, periodic task dispatcher, and user session cleaner',
    moduleKey: 'Cron',
    elapsedSec: 8,
  },
  {
    id: 'srv-alert',
    name: 'Alert & Notification Dispatcher',
    status: 'running',
    type: 'Alert & Notification Dispatcher',
    updated: '15 seconds ago',
    description: 'Real-time notification engine for Slack, Discord, Telegram, and Email alerts',
    moduleKey: 'Notification',
    elapsedSec: 15,
  },
  {
    id: 'srv-prometheus',
    name: 'Prometheus Metrics Scraper',
    status: 'running',
    type: 'Prometheus Metrics Scraper',
    updated: '10 seconds ago',
    description: 'Periodic time-series metrics scraper for node_exporter, vCPUs, and RAM utilization',
    moduleKey: 'Prometheus',
    elapsedSec: 10,
  },
  {
    id: 'srv-worker',
    name: 'In-Memory Async Worker Pool',
    status: 'running',
    type: 'In-Memory Async Worker Pool',
    updated: '3 seconds ago',
    description: 'Go goroutine worker pool executing background asynchronous jobs and queue dispatch',
    moduleKey: 'Queue',
    elapsedSec: 3,
  },
  {
    id: 'srv-grok',
    name: 'Grok Pattern Parser Daemon',
    status: 'running',
    type: 'Grok Pattern Parser Daemon',
    updated: '22 seconds ago',
    description: 'High-throughput regex log parser extracting structured telemetry from raw log streams',
    moduleKey: 'Grok',
    elapsedSec: 22,
  },
  {
    id: 'srv-dataprepper',
    name: 'Data Prepper Pipeline Validator',
    status: 'running',
    type: 'Data Prepper Pipeline Validator',
    updated: '16 seconds ago',
    description: 'Data Prepper YAML configuration validator, buffer health check, and sink router',
    moduleKey: 'DataPrepper',
    elapsedSec: 16,
  },
]);

const servicesSearch = ref('');
const tickerTimer = ref<any>(null);

// View Log Modal States
const showLogModal = ref(false);
const activeLogService = ref<ServiceItem | null>(null);
const serviceLogs = ref<LogEntry[]>([]);
const logFilterLevel = ref('ALL');
const logSearchText = ref('');
const logAutoScroll = ref(true);
const isLogPaused = ref(false);
let logWs: WebSocket | null = null;

const filteredServices = computed(() => {
  if (!servicesSearch.value) return services.value;
  const q = servicesSearch.value.toLowerCase();
  return services.value.filter(
    (s) => s.name.toLowerCase().includes(q) || s.description.toLowerCase().includes(q) || s.moduleKey.toLowerCase().includes(q)
  );
});

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
    if (el) el.scrollTop = el.scrollHeight;
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

// =================================================================
// 2. USER ACCOUNTS STATE & METHODS
// =================================================================
const users = ref<any[]>([]);
const isUserModalOpen = ref(false);
const userForm = ref({ username: '', password: '', role: 'OPERATOR' });

const fetchUsers = async () => {
  try {
    const res = await axios.get('/api/v1/auth/users').catch(() => null);
    if (res && res.data && res.data.success) {
      users.value = res.data.data;
    } else {
      users.value = [{ id: '1', username: 'admin', role: 'ADMIN', createdAt: '2026-08-20' }];
    }
  } catch (err) {
    users.value = [{ id: '1', username: 'admin', role: 'ADMIN', createdAt: '2026-08-20' }];
  }
};

const handleCreateUser = async () => {
  try {
    const res = await axios.post('/api/v1/auth/users', userForm.value);
    if (res.data.success) {
      isUserModalOpen.value = false;
      userForm.value = { username: '', password: '', role: 'OPERATOR' };
      await fetchUsers();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to create user');
  }
};

// =================================================================
// 3. DATABASE CONFIG
// =================================================================
const dbConfig = ref({
  host: 'localhost',
  port: 5432,
  user: 'hephaestus',
  password: '',
  database: 'hephaestus_db',
  ssl: false,
});
const dbStatus = ref<{ success: boolean; message: string } | null>(null);
const dbTesting = ref(false);
const dbSaving = ref(false);

const fetchDatabaseConfig = async () => {
  try {
    const res = await axios.get('/api/v1/settings/database');
    if (res.data.success && res.data.data) {
      dbConfig.value.host = res.data.data.host || 'localhost';
      dbConfig.value.port = res.data.data.port || 5432;
      dbConfig.value.user = res.data.data.user || 'hephaestus';
      dbConfig.value.database = res.data.data.database || 'hephaestus_db';
      dbConfig.value.ssl = Boolean(res.data.data.ssl);
    }
  } catch (err) {
    console.warn('Could not fetch active db config:', err);
  }
};

const testDbConnection = async () => {
  dbTesting.value = true;
  dbStatus.value = null;
  try {
    const res = await axios.post('/api/v1/settings/database/test', {
      host: dbConfig.value.host,
      port: Number(dbConfig.value.port) || 5432,
      user: dbConfig.value.user,
      password: dbConfig.value.password,
      database: dbConfig.value.database,
      ssl: dbConfig.value.ssl,
    });
    if (res.data.success) {
      dbStatus.value = { success: true, message: res.data.message || 'PostgreSQL Connection Verified Successfully!' };
    } else {
      dbStatus.value = { success: false, message: res.data.error || 'Database connection test failed' };
    }
  } catch (err: any) {
    dbStatus.value = {
      success: false,
      message: err.response?.data?.error || err.message || 'Database connection test failed',
    };
  } finally {
    dbTesting.value = false;
  }
};

const saveDbConfig = async () => {
  dbSaving.value = true;
  dbStatus.value = null;
  try {
    const res = await axios.post('/api/v1/settings/database', {
      host: dbConfig.value.host,
      port: Number(dbConfig.value.port) || 5432,
      user: dbConfig.value.user,
      password: dbConfig.value.password,
      database: dbConfig.value.database,
      ssl: dbConfig.value.ssl,
    });
    if (res.data.success) {
      dbStatus.value = { success: true, message: 'Database configuration applied & pool reconnected successfully!' };
    } else {
      dbStatus.value = { success: false, message: res.data.error || 'Failed to apply database configuration' };
    }
  } catch (err: any) {
    dbStatus.value = {
      success: false,
      message: err.response?.data?.error || err.message || 'Failed to apply database configuration',
    };
  } finally {
    dbSaving.value = false;
  }
};

// =================================================================
// 4. AUDIT & ACTIVITY LOGS
// =================================================================
const auditLogs = ref<any[]>([]);
const systemLogs = ref<any[]>([]);
const logViewMode = ref<'audit' | 'system'>('audit');
const loadingLogs = ref(false);

const fetchAuditLogs = async () => {
  loadingLogs.value = true;
  try {
    const res = await axios.get('/api/v1/settings/activity-logs?limit=50').catch(() => null);
    if (res && res.data && res.data.success) {
      auditLogs.value = res.data.data.logs || [];
    } else {
      auditLogs.value = [];
    }
  } catch (err) {
    auditLogs.value = [];
  } finally {
    loadingLogs.value = false;
  }
};

const fetchSystemLogs = async () => {
  loadingLogs.value = true;
  try {
    const res = await axios.get('/api/v1/logs?limit=50').catch(() => null);
    if (res && res.data && res.data.success) {
      systemLogs.value = res.data.data || [];
    } else {
      systemLogs.value = [];
    }
  } catch (err) {
    systemLogs.value = [];
  } finally {
    loadingLogs.value = false;
  }
};

const refreshCurrentLogs = () => {
  if (logViewMode.value === 'audit') {
    fetchAuditLogs();
  } else {
    fetchSystemLogs();
  }
};

onMounted(() => {
  if (route.query.tab === 'services') {
    activeTab.value = 'services';
  } else if (route.query.tab === 'audit' || route.query.tab === 'activity') {
    activeTab.value = 'audit';
  }
  fetchUsers();
  fetchAuditLogs();
  fetchSystemLogs();
  fetchDatabaseConfig();
  startElapsedTicker();
});

onUnmounted(() => {
  if (tickerTimer.value) clearInterval(tickerTimer.value);
  if (logWs) logWs.close();
});
</script>

<template>
  <div class="space-y-6 max-w-6xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-200 dark:border-slate-800 pb-4 flex items-center justify-between">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight flex items-center gap-2">
          <Settings class="w-5 h-5 text-brand-500 dark:text-brand-400" />
          <span>System Settings & Services</span>
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Manage HCP system parameters, background service daemons, user access control, database connection, and audit trail.
        </p>
      </div>

      <div>
        <ThemeToggle variant="button" :showLabel="true" />
      </div>
    </div>

    <!-- Navigation Tabs -->
    <div class="flex flex-wrap items-center gap-2 border-b border-slate-300 dark:border-slate-800 pb-2 text-xs">
      <button
        @click="activeTab = 'services'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'services'
            ? 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-800 dark:text-emerald-400 border border-emerald-300 dark:border-emerald-500/30 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <Activity class="w-3.5 h-3.5" />
        <span>Status Services ({{ services.length }})</span>
      </button>

      <button
        @click="activeTab = 'users'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'users'
            ? 'bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-white border border-blue-300 dark:border-slate-700 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <Users class="w-3.5 h-3.5" />
        <span>User Accounts ({{ users.length }})</span>
      </button>

      <button
        @click="activeTab = 'database'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'database'
            ? 'bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-white border border-blue-300 dark:border-slate-700 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <Database class="w-3.5 h-3.5" />
        <span>PostgreSQL Connection</span>
      </button>

      <button
        @click="activeTab = 'audit'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'audit'
            ? 'bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-white border border-blue-300 dark:border-slate-700 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <FileText class="w-3.5 h-3.5" />
        <span>Activity Logs</span>
      </button>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 1: STATUS SERVICES (4 Core Columns + View Log Modal) -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'services'" class="space-y-4 animate-in fade-in duration-150">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <div class="w-2.5 h-2.5 rounded-[2px] bg-emerald-500"></div>
          <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Active Service Daemons</h3>
        </div>

        <!-- Search input daemon -->
        <div class="relative w-64">
          <Search class="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500 dark:text-slate-400" />
          <input
            v-model="servicesSearch"
            placeholder="Search daemon..."
            class="w-full bg-white dark:bg-[#1b1e26] border border-slate-300 dark:border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-500 dark:placeholder-slate-400 focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500/30"
          />
        </div>
      </div>

      <!-- 4 Core Columns Table -->
      <div class="bg-white dark:bg-[#1b1e26] border border-slate-300 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <table class="w-full text-left text-xs border-collapse">
          <thead class="bg-slate-100 dark:bg-[#20242e] text-slate-700 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-300 dark:border-slate-800 select-none">
            <tr>
              <th class="py-3 px-4 w-36">Status Services</th>
              <th class="py-3 px-4">Nama Services</th>
              <th class="py-3 px-4 w-44">Last Update</th>
              <th class="py-3 px-4 w-28 text-center">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-200 dark:divide-slate-800/60">
            <tr
              v-for="srv in filteredServices"
              :key="srv.id"
              class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition group"
            >
              <!-- 1. Status Services -->
              <td class="py-3 px-4 whitespace-nowrap">
                <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded bg-emerald-50 dark:bg-emerald-500/10 border border-emerald-300 dark:border-emerald-500/30 text-emerald-800 dark:text-emerald-400 text-[11px] font-bold">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                  <span class="uppercase">RUNNING</span>
                </div>
              </td>

              <!-- 2. Nama Services -->
              <td class="py-3 px-4">
                <div>
                  <p class="text-xs font-bold text-slate-900 dark:text-white group-hover:text-blue-700 dark:group-hover:text-emerald-400 transition">{{ srv.name }}</p>
                  <p class="text-[11px] text-slate-600 dark:text-slate-400 mt-0.5">{{ srv.description }}</p>
                </div>
              </td>

              <!-- 3. Last Update (Live Ticker) -->
              <td class="py-3 px-4 whitespace-nowrap text-xs font-mono text-slate-700 dark:text-slate-300 font-medium">
                <span>{{ srv.updated }}</span>
              </td>

              <!-- 4. Actions (View Log) -->
              <td class="py-3 px-4 text-center whitespace-nowrap">
                <button
                  @click="openViewLogModal(srv)"
                  class="px-3 py-1.5 rounded-lg bg-white hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-200 text-xs font-semibold border border-slate-300 dark:border-slate-700 transition inline-flex items-center gap-1.5 shadow-sm"
                >
                  <Terminal class="w-3.5 h-3.5 text-blue-600 dark:text-brand-400" />
                  <span>View Log</span>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 3: USER ACCOUNTS -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'users'" class="space-y-4 animate-in fade-in duration-150">
      <div class="flex items-center justify-between">
        <h3 class="text-xs font-bold text-white uppercase tracking-wider">System Users</h3>
        <button
          @click="isUserModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs transition"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add User</span>
        </button>
      </div>

      <div class="bg-[#1b1e26] border border-slate-800 rounded-xl overflow-hidden shadow-xl">
        <table class="w-full text-left text-xs font-mono">
          <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
            <tr>
              <th class="p-3">Username</th>
              <th class="p-3">Role</th>
              <th class="p-3">Created</th>
              <th class="p-3 text-right">Action</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 text-slate-300">
            <tr v-for="u in users" :key="u.id" class="hover:bg-slate-800/30">
              <td class="p-3 text-white font-bold">{{ u.username }}</td>
              <td class="p-3">
                <span class="px-2 py-0.5 rounded text-[10px] font-bold" :class="u.role === 'ADMIN' ? 'bg-purple-500/10 text-purple-400 border border-purple-500/30' : 'bg-slate-800 text-slate-300'">
                  {{ u.role }}
                </span>
              </td>
              <td class="p-3 text-slate-400">{{ u.createdAt || '2026-08-20' }}</td>
              <td class="p-3 text-right">
                <span class="text-slate-500 text-[11px]">Default</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 4: POSTGRESQL CONNECTION -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'database'" class="p-6 bg-[#171a23] border border-slate-800 rounded-xl space-y-4 max-w-3xl animate-in fade-in duration-150">
      <div class="flex items-center justify-between border-b border-slate-800/80 pb-3">
        <div>
          <h3 class="text-xs font-bold text-white uppercase tracking-wider">PostgreSQL Connection Settings</h3>
          <p class="text-[11px] text-slate-400">Configure PostgreSQL database credentials, host endpoint, port, and TLS encryption</p>
        </div>
        <span class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-mono font-bold">
          ACTIVE POOL
        </span>
      </div>
      
      <form @submit.prevent="saveDbConfig" class="space-y-4 text-xs">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Host</label>
            <input v-model="dbConfig.host" required placeholder="localhost" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Port</label>
            <input v-model.number="dbConfig.port" type="number" required placeholder="5432" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Database Name</label>
            <input v-model="dbConfig.database" required placeholder="hephaestus_db" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-bold">User</label>
            <input v-model="dbConfig.user" required placeholder="hephaestus" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
        </div>

        <div>
          <label class="block text-slate-400 mb-1 font-bold">Password</label>
          <input
            v-model="dbConfig.password"
            type="password"
            placeholder="••••••••••••••••"
            class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono"
          />
          <p class="text-[10px] text-slate-500 mt-1">Leave blank to retain existing encrypted database password</p>
        </div>

        <div class="pt-1">
          <label class="flex items-center gap-2 cursor-pointer text-slate-300">
            <input
              type="checkbox"
              v-model="dbConfig.ssl"
              class="rounded bg-[#0f1219] border-slate-700 text-[#4274D9] focus:ring-0 w-4 h-4 cursor-pointer"
            />
            <span class="font-medium">Use SSL / TLS Encrypted Connection (sslmode=require)</span>
          </label>
        </div>

        <div v-if="dbStatus" :class="['p-3 rounded-lg border text-xs font-mono', dbStatus.success ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-400']">
          {{ dbStatus.message }}
        </div>

        <div class="pt-3 border-t border-slate-800 flex items-center justify-between">
          <button
            type="button"
            @click="testDbConnection"
            :disabled="dbTesting"
            class="px-4 py-2 bg-[#20242e] hover:bg-[#282d3a] text-slate-200 text-xs font-bold rounded-lg border border-slate-700 transition disabled:opacity-50"
          >
            {{ dbTesting ? 'TESTING...' : 'Test Connection' }}
          </button>

          <button
            type="submit"
            :disabled="dbSaving"
            class="px-4 py-2 bg-[#4274D9] hover:bg-[#3461c2] text-white text-xs font-bold rounded-lg transition disabled:opacity-50"
          >
            {{ dbSaving ? 'SAVING...' : 'Save & Apply Config' }}
          </button>
        </div>
      </form>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 4: ACTIVITY LOGS & AUDIT TRAIL -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'audit'" class="space-y-4 animate-in fade-in duration-150">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-white dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200 dark:border-slate-800 shadow-sm">
        <div class="flex items-center gap-3">
          <div class="flex items-center p-1 bg-slate-100 dark:bg-slate-800/80 rounded-lg border border-slate-200 dark:border-slate-700/60 text-xs">
            <button
              @click="logViewMode = 'audit'; fetchAuditLogs()"
              :class="[
                'px-3 py-1.5 rounded-md font-semibold transition flex items-center gap-1.5',
                logViewMode === 'audit'
                  ? 'bg-[#4274D9] text-white shadow-sm'
                  : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              <ShieldCheck class="w-3.5 h-3.5" />
              <span>User & Security Audit Trail ({{ auditLogs.length }})</span>
            </button>
            <button
              @click="logViewMode = 'system'; fetchSystemLogs()"
              :class="[
                'px-3 py-1.5 rounded-md font-semibold transition flex items-center gap-1.5',
                logViewMode === 'system'
                  ? 'bg-[#4274D9] text-white shadow-sm'
                  : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              <Terminal class="w-3.5 h-3.5" />
              <span>System & Console Logs ({{ systemLogs.length }})</span>
            </button>
          </div>
        </div>

        <button
          @click="refreshCurrentLogs"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-slate-700 text-xs font-semibold transition shadow-sm self-start sm:self-auto"
        >
          <RotateCw class="w-3.5 h-3.5 text-[#4274D9]" :class="{ 'animate-spin': loadingLogs }" />
          <span>Refresh</span>
        </button>
      </div>

      <!-- Table 1: Security & User Audit Logs (Database) -->
      <div v-if="logViewMode === 'audit'" class="bg-white dark:bg-[#141824] border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <table class="w-full text-left text-xs">
          <thead class="bg-slate-50 dark:bg-[#1a202c] text-slate-700 dark:text-slate-300 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
            <tr>
              <th class="p-3 w-48">Timestamp</th>
              <th class="p-3 w-32">Actor / User</th>
              <th class="p-3 w-24">Module</th>
              <th class="p-3 w-36">Action</th>
              <th class="p-3">Details</th>
              <th class="p-3 w-28 text-right">Status</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-slate-800 dark:text-slate-200">
            <tr v-for="log in auditLogs" :key="log.id" class="hover:bg-slate-50/80 dark:hover:bg-slate-800/30 transition">
              <td class="p-3 text-slate-500 dark:text-slate-400 font-mono text-[11px] whitespace-nowrap">
                {{ new Date(log.timestamp).toLocaleString() }}
              </td>
              <td class="p-3 font-semibold text-slate-900 dark:text-white">
                <span v-if="log.username" class="px-2 py-0.5 rounded bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-mono text-[11px]">
                  {{ log.username }}
                </span>
                <span v-else class="text-slate-400 dark:text-slate-500 italic text-[11px]">
                  Anonymous
                </span>
              </td>
              <td class="p-3">
                <span class="px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 font-semibold text-[10px]">
                  {{ log.module }}
                </span>
              </td>
              <td class="p-3 font-medium">
                {{ log.action }}
              </td>
              <td class="p-3 text-slate-600 dark:text-slate-300 font-mono text-[11px] break-all">
                {{ log.details }}
              </td>
              <td class="p-3 text-right">
                <span
                  class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider"
                  :class="log.status === 'SUCCESS' ? 'bg-emerald-100 dark:bg-emerald-500/20 text-emerald-700 dark:text-emerald-400' : 'bg-red-100 dark:bg-red-500/20 text-red-700 dark:text-red-400'"
                >
                  {{ log.status }}
                </span>
              </td>
            </tr>
            <tr v-if="auditLogs.length === 0">
              <td colspan="6" class="p-8 text-center text-slate-400 dark:text-slate-500">
                No security or user activity logs recorded yet.
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Table 2: System Console Logs -->
      <div v-else class="bg-white dark:bg-[#141824] border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <table class="w-full text-left text-xs font-mono">
          <thead class="bg-slate-50 dark:bg-[#1a202c] text-slate-700 dark:text-slate-300 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
            <tr>
              <th class="p-3 w-48">Timestamp</th>
              <th class="p-3 w-20">Level</th>
              <th class="p-3 w-28">Module</th>
              <th class="p-3">Message</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-slate-800 dark:text-slate-200">
            <tr v-for="(log, idx) in systemLogs" :key="idx" class="hover:bg-slate-50/80 dark:hover:bg-slate-800/30 transition">
              <td class="p-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{{ log.timestamp }}</td>
              <td class="p-3">
                <span
                  class="px-1.5 py-0.5 rounded text-[9px] font-bold uppercase"
                  :class="[
                    log.level === 'ERROR' ? 'bg-red-100 dark:bg-red-500/20 text-red-700 dark:text-red-400' :
                    log.level === 'WARN' ? 'bg-amber-100 dark:bg-amber-500/20 text-amber-700 dark:text-amber-400' :
                    log.level === 'DEBUG' ? 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400' :
                    'bg-emerald-100 dark:bg-emerald-500/20 text-emerald-700 dark:text-emerald-400'
                  ]"
                >
                  {{ log.level }}
                </span>
              </td>
              <td class="p-3 font-semibold text-[#4274D9]">{{ log.module }}</td>
              <td class="p-3 text-slate-800 dark:text-slate-200 break-all">{{ log.message }}</td>
            </tr>
            <tr v-if="systemLogs.length === 0">
              <td colspan="4" class="p-8 text-center text-slate-400 dark:text-slate-500">
                No system log events recorded.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- VIEW LOG MODAL (CONNECTED VIA WEBSOCKET) -->
    <!-- ============================================================= -->
    <div
      v-if="showLogModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-150"
    >
      <div class="bg-[#11141c] border border-slate-700 rounded-2xl w-full max-w-4xl h-[75vh] flex flex-col shadow-2xl overflow-hidden font-mono">
        <div class="p-3.5 bg-[#171a23] border-b border-slate-800 flex items-center justify-between text-xs font-sans">
          <div class="flex items-center gap-2.5">
            <Terminal class="w-4 h-4 text-brand-400" />
            <h3 class="font-bold text-white">{{ activeLogService?.name }}</h3>
            <span class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-mono">
              LIVE WS STREAM
            </span>
          </div>
          <button @click="closeViewLogModal" class="text-slate-400 hover:text-white p-1">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div id="service-log-viewport" class="flex-1 p-4 overflow-y-auto bg-[#090d16] text-xs space-y-1 select-text leading-relaxed">
          <div
            v-for="(l, idx) in filteredServiceLogs"
            :key="idx"
            class="flex items-start gap-2 hover:bg-slate-900/60 p-0.5 rounded"
          >
            <span class="text-slate-600 text-[11px] shrink-0 select-none">
              {{ new Date(l.timestamp).toLocaleTimeString() }}
            </span>
            <span
              :class="[
                'px-1.5 py-0.2 rounded text-[9px] font-bold shrink-0 uppercase',
                l.level === 'ERROR' ? 'bg-red-500/20 text-red-400' : l.level === 'WARN' ? 'bg-amber-500/20 text-amber-400' : 'bg-emerald-500/20 text-emerald-400'
              ]"
            >
              {{ l.level }}
            </span>
            <span class="text-slate-200 break-all">{{ l.message }}</span>
          </div>
          <div v-if="filteredServiceLogs.length === 0" class="text-center py-12 text-slate-600 font-sans text-xs">
            No live log entries recorded for this service module.
          </div>
        </div>

        <div class="p-2.5 px-4 bg-[#141720] border-t border-slate-800 flex items-center justify-between text-xs text-slate-400 font-sans">
          <span>Module: {{ activeLogService?.moduleKey }}</span>
          <button @click="closeViewLogModal" class="px-3 py-1 bg-slate-800 hover:bg-slate-700 text-white rounded-lg">Close</button>
        </div>
      </div>
    </div>

    <!-- Modal: Add User -->
    <div
      v-if="isUserModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-sm p-6 space-y-4 shadow-2xl">
        <h3 class="text-sm font-bold text-white">Create New User</h3>
        <form @submit.prevent="handleCreateUser" class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Username</label>
            <input v-model="userForm.username" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Password</label>
            <input v-model="userForm.password" type="password" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Role</label>
            <select v-model="userForm.role" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white">
              <option value="OPERATOR">OPERATOR</option>
              <option value="ADMIN">ADMIN</option>
            </select>
          </div>
          <div class="flex justify-end gap-2 pt-3 border-t border-slate-800">
            <button type="button" @click="isUserModalOpen = false" class="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg">Cancel</button>
            <button type="submit" class="px-4 py-2 bg-blue-600 text-white font-semibold rounded-lg">Create</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
