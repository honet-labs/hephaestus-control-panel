<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import axios from 'axios';
import {
  Settings,
  Users,
  Database,
  Shield,
  Activity,
  Plus,
  Trash2,
  Server,
  Network,
  Radio,
  Zap,
  FileText,
  Bell,
  HardDrive,
  Cpu,
  Layers,
  RotateCw,
  Play,
  CheckCircle2,
  Clock,
  AlertTriangle,
  Search,
  Share2,
  Sliders,
  Terminal,
} from 'lucide-vue-next';

const activeTab = ref<'services' | 'queue' | 'general' | 'users' | 'database' | 'audit'>('services');

// Status Services List (Exact 11 Core Pandora FMS & Hephaestus Daemons from Screenshot)
interface ServiceStatusItem {
  id: string;
  name: string;
  status: 'running' | 'warning' | 'stopped';
  type: string;
  iconName: string;
  master: boolean;
  version: string;
  description?: string;
}

const serviceSearch = ref('');
const serviceStatusList = ref<ServiceStatusItem[]>([
  {
    id: 'srv-data',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Data server',
    iconName: 'database',
    master: true,
    version: '8.0NG.803 (P) 260703',
    description: 'Processes incoming asynchronous data packets, XML modules, and metrics',
  },
  {
    id: 'srv-network',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Network server',
    iconName: 'server',
    master: true,
    version: '8.0NG.803 (P) 260703',
    description: 'Performs network checks, ICMP sweeps, TCP latency checks, and port monitoring',
  },
  {
    id: 'srv-snmptrap',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'SNMP trap server',
    iconName: 'network',
    master: true,
    version: '8.0NG.803 (P) 260703',
    description: 'Daemon for receiving and processing SNMP v1/v2c/v3 traps in real time',
  },
  {
    id: 'srv-discovery',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Discovery server',
    iconName: 'radio',
    master: true,
    version: '8.0NG.803 (P) 260703',
    description: 'Network topology scanner, ARP lookup, and automatic asset discovery daemon',
  },
  {
    id: 'srv-event',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Event server',
    iconName: 'zap',
    master: false,
    version: '8.0NG.803 (P) 260703',
    description: 'Correlates system events, threshold alarms, and state changes',
  },
  {
    id: 'srv-syslog',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Syslog server',
    iconName: 'filetext',
    master: false,
    version: '8.0NG.803 (P) 260703',
    description: 'High-speed RFC 3164/5424 syslog collector and parser daemon',
  },
  {
    id: 'srv-alert',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Alert server',
    iconName: 'bell',
    master: true,
    version: '8.0NG.803 (P) 260703',
    description: 'Dispatches multi-channel alerts, webhook payloads, emails, and notifications',
  },
  {
    id: 'srv-netflow',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Netflow server',
    iconName: 'share',
    master: false,
    version: '8.0NG.803 (P) 260703',
    description: 'Collects and analyzes NetFlow v5/v9 and IPFIX network traffic streams',
  },
  {
    id: 'srv-log',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Log server',
    iconName: 'harddrive',
    master: false,
    version: '8.0NG.803 (P) 260703',
    description: 'Centralized log aggregation, storage poller, and OpenSearch pipeline worker',
  },
  {
    id: 'srv-highperf',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'High performance server',
    iconName: 'cpu',
    master: true,
    version: '8.0NG.803 (P) 260703',
    description: 'Concurrent multi-threaded poller for massive enterprise monitoring tasks',
  },
  {
    id: 'srv-heavy',
    name: 'labs-pfms-master',
    status: 'running',
    type: 'Heavy server',
    iconName: 'layers',
    master: false,
    version: '8.0NG.803 (P) 260703',
    description: 'Executes complex synthetic transactions, WMI, and large scheduled scripts',
  },
]);

const filteredServices = computed(() => {
  if (!serviceSearch.value) return serviceStatusList.value;
  const q = serviceSearch.value.toLowerCase();
  return serviceStatusList.value.filter(
    s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q) || s.version.toLowerCase().includes(q)
  );
});

// Background Jobs Queue State
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

const queueJobs = ref<Job[]>([]);
const queueLoading = ref(false);
const queueTimer = ref<any>(null);

const fetchQueueJobs = async () => {
  try {
    const res = await axios.get('/api/v1/queue/jobs');
    if (res.data.success && res.data.data) {
      queueJobs.value = res.data.data;
    }
  } catch (err) {
    console.error('Failed to fetch queue jobs:', err);
  }
};

const triggerJob = async (type: string) => {
  queueLoading.value = true;
  try {
    const res = await axios.post('/api/v1/queue/jobs/trigger', { type });
    if (res.data.success) {
      await fetchQueueJobs();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to trigger job');
  } finally {
    queueLoading.value = false;
  }
};

// Settings & Users & DB State
const users = ref<any[]>([]);
const auditLogs = ref<any[]>([]);
const dbConfig = ref<any>({
  host: 'localhost',
  port: 5432,
  user: 'postgres',
  password: '',
  database: 'hephaestus',
  ssl: false,
});

const isUserModalOpen = ref(false);
const userForm = ref({
  username: '',
  password: '',
  role: 'operator',
});

const fetchSettings = async () => {
  try {
    const [usersRes, logsRes, dbRes] = await Promise.all([
      axios.get('/api/v1/settings/users'),
      axios.get('/api/v1/settings/activity-logs?limit=50'),
      axios.get('/api/v1/settings/database'),
    ]);
    if (usersRes.data.success) users.value = usersRes.data.data || [];
    if (logsRes.data.success) auditLogs.value = logsRes.data.data.logs || [];
    if (dbRes.data.success) dbConfig.value = dbRes.data.data;
  } catch (err) {
    console.error(err);
  }
};

const createUser = async () => {
  try {
    const res = await axios.post('/api/v1/settings/users', userForm.value);
    if (res.data.success) {
      isUserModalOpen.value = false;
      userForm.value = { username: '', password: '', role: 'operator' };
      fetchSettings();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || err.message);
  }
};

const deleteUser = async (id: number) => {
  if (!confirm('Are you sure you want to delete this user?')) return;
  try {
    await axios.delete(`/api/v1/settings/users/${id}`);
    fetchSettings();
  } catch (err: any) {
    alert(err.response?.data?.error || err.message);
  }
};

const saveDBConfig = async () => {
  try {
    const res = await axios.post('/api/v1/settings/database', dbConfig.value);
    if (res.data.success) {
      alert('Database connection updated and synchronized successfully!');
    }
  } catch (err: any) {
    alert(`Failed to switch database: ${err.response?.data?.error || err.message}`);
  }
};

onMounted(() => {
  fetchSettings();
  fetchQueueJobs();
  queueTimer.value = setInterval(fetchQueueJobs, 5000);
});

onUnmounted(() => {
  if (queueTimer.value) clearInterval(queueTimer.value);
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">System Settings & Management</h2>
        <p class="text-xs text-slate-400">Status services, background daemons, worker pool, user accounts, and database switching</p>
      </div>
      <div v-if="activeTab === 'services'" class="flex items-center gap-2">
        <span class="flex items-center gap-1.5 px-2.5 py-1 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-xs font-mono font-medium">
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          11 Daemons Active
        </span>
      </div>
    </div>

    <!-- Tabs Navigation Bar -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2 overflow-x-auto text-xs">
      <button
        @click="activeTab = 'services'"
        :class="[
          activeTab === 'services'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Server class="w-3.5 h-3.5" />
        <span>Status Services ({{ serviceStatusList.length }})</span>
      </button>

      <button
        @click="activeTab = 'queue'"
        :class="[
          activeTab === 'queue'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Activity class="w-3.5 h-3.5" />
        <span>Background Jobs Queue ({{ queueJobs.length }})</span>
      </button>

      <button
        @click="activeTab = 'general'"
        :class="[
          activeTab === 'general'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200',
          'px-3.5 py-1.5 rounded-lg border transition'
        ]"
      >
        General Info
      </button>

      <button
        @click="activeTab = 'users'"
        :class="[
          activeTab === 'users'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200',
          'px-3.5 py-1.5 rounded-lg border transition'
        ]"
      >
        User Accounts ({{ users.length }})
      </button>

      <button
        @click="activeTab = 'database'"
        :class="[
          activeTab === 'database'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200',
          'px-3.5 py-1.5 rounded-lg border transition'
        ]"
      >
        PostgreSQL Connection
      </button>

      <button
        @click="activeTab = 'audit'"
        :class="[
          activeTab === 'audit'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200',
          'px-3.5 py-1.5 rounded-lg border transition'
        ]"
      >
        Activity Logs
      </button>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 1: STATUS SERVICES (MATCHING USER SCREENSHOT EXACTLY) -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'services'" class="space-y-4">
      <!-- Search & Refresh Toolbar -->
      <div class="flex items-center justify-between gap-3">
        <div class="relative w-72">
          <Search class="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500" />
          <input
            v-model="serviceSearch"
            placeholder="Search service name, type, or version..."
            class="w-full bg-[#1b1e26] border border-slate-800 rounded-lg pl-9 pr-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
          />
        </div>

        <div class="flex items-center gap-2 text-xs text-slate-400">
          <span class="text-slate-500 font-mono text-[11px]">Server: labs-pfms-master</span>
        </div>
      </div>

      <!-- Main Status Services Table (Exact Visual Layout from Screenshot) -->
      <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs border-collapse">
            <thead class="bg-[#20242e] text-slate-300 text-[11px] font-bold border-b border-slate-800 select-none">
              <tr>
                <th class="py-3 px-6 w-1/4">Name</th>
                <th class="py-3 px-4 w-20 text-center">Status</th>
                <th class="py-3 px-6 flex items-center gap-1 cursor-pointer hover:text-white">
                  <span class="text-emerald-400 text-[10px]">▲</span>
                  <span>Type</span>
                </th>
                <th class="py-3 px-6 w-28">Master</th>
                <th class="py-3 px-6 w-1/4">Version</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-sans text-xs">
              <tr
                v-for="s in filteredServices"
                :key="s.id"
                class="hover:bg-slate-800/30 transition text-slate-300"
              >
                <!-- 1. Name -->
                <td class="py-3.5 px-6 font-medium text-slate-200">
                  {{ s.name }}
                </td>

                <!-- 2. Status (Solid Vibrant Green Square Icon) -->
                <td class="py-3.5 px-4 text-center">
                  <div class="inline-flex items-center justify-center">
                    <span
                      :class="[
                        'w-3 h-3 rounded-[2px] shadow-sm',
                        s.status === 'running' ? 'bg-[#22c55e] shadow-emerald-500/50' : 'bg-[#ef4444]'
                      ]"
                      :title="s.status === 'running' ? 'Service is Running' : 'Service is Stopped'"
                    ></span>
                  </div>
                </td>

                <!-- 3. Type (Icon + Name) -->
                <td class="py-3.5 px-6">
                  <div class="flex items-center gap-2.5">
                    <!-- Dynamic Lucide Icons matching screenshot -->
                    <Database v-if="s.iconName === 'database'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Server v-else-if="s.iconName === 'server'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Network v-else-if="s.iconName === 'network'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Radio v-else-if="s.iconName === 'radio'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Zap v-else-if="s.iconName === 'zap'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <FileText v-else-if="s.iconName === 'filetext'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Bell v-else-if="s.iconName === 'bell'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Share2 v-else-if="s.iconName === 'share'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <HardDrive v-else-if="s.iconName === 'harddrive'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Cpu v-else-if="s.iconName === 'cpu'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Layers v-else-if="s.iconName === 'layers'" class="w-4 h-4 text-slate-300 shrink-0" />
                    <Activity v-else class="w-4 h-4 text-slate-300 shrink-0" />

                    <span class="font-medium text-slate-200">{{ s.type }}</span>
                  </div>
                </td>

                <!-- 4. Master -->
                <td class="py-3.5 px-6 font-medium text-slate-300">
                  {{ s.master ? 'Yes' : 'No' }}
                </td>

                <!-- 5. Version -->
                <td class="py-3.5 px-6 text-slate-300 font-mono text-[11px]">
                  {{ s.version }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Table Footer (Exact string from screenshot) -->
        <div class="p-3 px-6 border-t border-slate-800/80 bg-[#171a22] text-[11px] text-slate-400 font-sans">
          Showing 1 to {{ filteredServices.length }} of {{ serviceStatusList.length }} entries
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 2: BACKGROUND JOBS QUEUE -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'queue'" class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-xs text-slate-400">Background task workers, scheduled ICMP pollers, telemetry sync, and backup jobs</p>
        <button
          @click="fetchQueueJobs"
          :disabled="queueLoading"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#1b1e26] border border-slate-700/60 text-xs text-slate-300 hover:text-white transition font-medium"
        >
          <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': queueLoading }" />
          <span>Refresh</span>
        </button>
      </div>

      <!-- Quick Trigger Actions -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
        <button
          @click="triggerJob('icmp_sweep')"
          :disabled="queueLoading"
          class="p-3.5 bg-[#1b1e26] border border-slate-800 hover:border-brand-500/50 rounded-xl text-left transition space-y-1 shadow-lg group"
        >
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-white group-hover:text-brand-400 transition">ICMP Ping Sweep</span>
            <Play class="w-3.5 h-3.5 text-slate-500 group-hover:text-brand-400 transition" />
          </div>
          <p class="text-[11px] text-slate-400">Sweep active subnets for device latency</p>
        </button>

        <button
          @click="triggerJob('opensearch_sync')"
          :disabled="queueLoading"
          class="p-3.5 bg-[#1b1e26] border border-slate-800 hover:border-brand-500/50 rounded-xl text-left transition space-y-1 shadow-lg group"
        >
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-white group-hover:text-brand-400 transition">OpenSearch Sync</span>
            <Play class="w-3.5 h-3.5 text-slate-500 group-hover:text-brand-400 transition" />
          </div>
          <p class="text-[11px] text-slate-400">Poll cluster shard state & telemetry</p>
        </button>

        <button
          @click="triggerJob('database_backup')"
          :disabled="queueLoading"
          class="p-3.5 bg-[#1b1e26] border border-slate-800 hover:border-brand-500/50 rounded-xl text-left transition space-y-1 shadow-lg group"
        >
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-white group-hover:text-brand-400 transition">Database Backup</span>
            <Play class="w-3.5 h-3.5 text-slate-500 group-hover:text-brand-400 transition" />
          </div>
          <p class="text-[11px] text-slate-400">Execute PostgreSQL automated backup</p>
        </button>

        <button
          @click="triggerJob('subnet_discovery')"
          :disabled="queueLoading"
          class="p-3.5 bg-[#1b1e26] border border-slate-800 hover:border-brand-500/50 rounded-xl text-left transition space-y-1 shadow-lg group"
        >
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-white group-hover:text-brand-400 transition">Subnet Discovery</span>
            <Play class="w-3.5 h-3.5 text-slate-500 group-hover:text-brand-400 transition" />
          </div>
          <p class="text-[11px] text-slate-400">Scan subnet for ARP & topology nodes</p>
        </button>
      </div>

      <!-- Queue Jobs Table -->
      <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
        <div class="p-3 px-5 border-b border-slate-800 flex items-center justify-between">
          <h3 class="text-xs font-bold text-white tracking-wide uppercase">Active & Recent Jobs</h3>
          <span class="text-xs font-mono text-slate-400">{{ queueJobs.length }} jobs recorded</span>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-2.5 px-5">Job ID</th>
                <th class="py-2.5 px-4">Type</th>
                <th class="py-2.5 px-4">Status</th>
                <th class="py-2.5 px-4">Progress</th>
                <th class="py-2.5 px-4">Message</th>
                <th class="py-2.5 px-4">Created</th>
              </tr>
            </thead>
            <tbody v-if="queueJobs.length > 0" class="divide-y divide-slate-800/60 font-mono text-xs">
              <tr
                v-for="job in queueJobs"
                :key="job.id"
                class="hover:bg-slate-800/30 transition text-slate-300"
              >
                <td class="py-3 px-5 text-slate-200 font-bold font-sans">{{ job.id }}</td>
                <td class="py-3 px-4 font-sans text-brand-400 font-semibold">{{ job.type }}</td>
                <td class="py-3 px-4 font-sans">
                  <span
                    :class="[
                      'px-2 py-0.5 rounded text-[10px] font-bold uppercase',
                      job.status === 'completed'
                        ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                        : job.status === 'running'
                        ? 'bg-blue-500/10 text-blue-400 border border-blue-500/30 animate-pulse'
                        : job.status === 'failed'
                        ? 'bg-red-500/10 text-red-400 border border-red-500/30'
                        : 'bg-slate-800 text-slate-400'
                    ]"
                  >
                    {{ job.status }}
                  </span>
                </td>
                <td class="py-3 px-4">
                  <div class="flex items-center gap-2">
                    <div class="w-20 bg-slate-800 h-1.5 rounded-full overflow-hidden">
                      <div class="bg-brand-500 h-full rounded-full transition-all duration-300" :style="{ width: `${job.progress || 100}%` }"></div>
                    </div>
                    <span class="text-[10px] text-slate-400">{{ job.progress || 100 }}%</span>
                  </div>
                </td>
                <td class="py-3 px-4 text-slate-300 font-sans max-w-xs truncate">{{ job.message || '-' }}</td>
                <td class="py-3 px-4 text-slate-400 text-[11px]">{{ new Date(job.createdAt).toLocaleTimeString() }}</td>
              </tr>
            </tbody>
            <tbody v-else>
              <tr>
                <td colspan="6" class="py-8 text-center text-slate-500 text-xs font-sans">No background jobs registered in queue.</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 3: GENERAL INFO -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'general'" class="max-w-xl p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-4 shadow-xl">
      <h3 class="text-xs font-bold text-white uppercase tracking-wider">System Information</h3>
      <div class="space-y-2 text-xs text-slate-300 font-sans">
        <div class="flex justify-between py-2 border-b border-slate-800"><span class="text-slate-400">Service</span><span class="font-medium text-white">Hephaestus Control Panel (HCP)</span></div>
        <div class="flex justify-between py-2 border-b border-slate-800"><span class="text-slate-400">Version</span><span class="font-mono text-brand-400 font-bold">v2.0.0 (Go Edition)</span></div>
        <div class="flex justify-between py-2 border-b border-slate-800"><span class="text-slate-400">Backend Engine</span><span class="font-mono">Go 1.22 + Gin Framework</span></div>
        <div class="flex justify-between py-2 border-b border-slate-800"><span class="text-slate-400">Frontend Engine</span><span class="font-mono">Vue 3 + Vite + Tailwind CSS</span></div>
        <div class="flex justify-between py-2"><span class="text-slate-400">Architecture</span><span class="font-mono text-slate-200">Microservice Event-Driven Core</span></div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 4: USERS -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'users'" class="space-y-4">
      <div class="flex justify-end">
        <button @click="isUserModalOpen = true" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-brand-500 hover:bg-brand-600 text-white font-semibold shadow-lg shadow-brand-500/20 transition">
          <Plus class="w-3.5 h-3.5" /> Add User
        </button>
      </div>
      <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-x-auto shadow-xl">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-slate-400 bg-[#20242e] uppercase text-[10px] font-bold tracking-wider">
              <th class="p-3 px-5">Username</th>
              <th class="p-3 px-4">Role</th>
              <th class="p-3 px-4">Created</th>
              <th class="p-3 px-5 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 text-slate-300">
            <tr v-for="u in users" :key="u.id" class="hover:bg-slate-800/30 transition">
              <td class="p-3 px-5 font-bold text-white">{{ u.username }}</td>
              <td class="p-3 px-4 uppercase font-mono text-[10px] text-brand-400">{{ u.role }}</td>
              <td class="p-3 px-4 text-slate-500 font-mono text-[11px]">{{ new Date(u.createdAt).toLocaleDateString() }}</td>
              <td class="p-3 px-5 text-right">
                <button @click="deleteUser(u.id)" class="text-slate-400 hover:text-red-400 transition"><Trash2 class="w-4 h-4 inline" /></button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 5: DATABASE -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'database'" class="max-w-lg p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-4 shadow-xl">
      <h3 class="text-xs font-bold text-white uppercase tracking-wider">PostgreSQL Connection Settings</h3>
      <div class="space-y-3 text-xs">
        <div class="grid grid-cols-3 gap-2">
          <div class="col-span-2">
            <label class="block text-slate-400 mb-1">Host</label>
            <input v-model="dbConfig.host" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Port</label>
            <input v-model.number="dbConfig.port" type="number" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Database Name</label>
          <input v-model="dbConfig.database" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Username</label>
          <input v-model="dbConfig.user" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Password</label>
          <input v-model="dbConfig.password" type="password" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" placeholder="Leave empty to keep unchanged" />
        </div>
        <button @click="saveDBConfig" class="w-full py-2 bg-brand-500 hover:bg-brand-600 text-white font-semibold rounded-lg shadow-lg shadow-brand-500/20 transition">
          Test & Switch Database Connection
        </button>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 6: AUDIT LOGS -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'audit'" class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-x-auto shadow-xl">
      <table class="w-full text-left text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-slate-400 bg-[#20242e] uppercase text-[10px] font-bold tracking-wider">
            <th class="p-3 px-5">Time</th>
            <th class="p-3 px-4">Module</th>
            <th class="p-3 px-4">Action</th>
            <th class="p-3 px-4">User</th>
            <th class="p-3 px-4">Details</th>
            <th class="p-3 px-5">Status</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-slate-300">
          <tr v-for="l in auditLogs" :key="l.id" class="hover:bg-slate-800/30 transition">
            <td class="p-3 px-5 text-slate-500 font-mono text-[11px]">{{ new Date(l.timestamp).toLocaleString() }}</td>
            <td class="p-3 px-4 font-semibold text-slate-300">[{{ l.module }}]</td>
            <td class="p-3 px-4 text-white font-medium">{{ l.action }}</td>
            <td class="p-3 px-4 text-slate-400 font-mono text-[11px]">{{ l.username || 'System' }}</td>
            <td class="p-3 px-4 text-slate-400 truncate max-w-xs">{{ l.details }}</td>
            <td class="p-3 px-5">
              <span :class="l.status === 'SUCCESS' ? 'text-emerald-400' : 'text-red-400'" class="font-bold text-[10px]">
                {{ l.status }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add User Modal -->
    <div v-if="isUserModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="w-full max-w-sm bg-[#1b1e26] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4">
        <h3 class="text-sm font-bold text-white">Create User</h3>
        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Username</label>
            <input v-model="userForm.username" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white focus:outline-none focus:border-brand-500" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Password</label>
            <input v-model="userForm.password" type="password" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white focus:outline-none focus:border-brand-500" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Role</label>
            <select v-model="userForm.role" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white focus:outline-none focus:border-brand-500">
              <option value="operator">Operator</option>
              <option value="ADMIN">Administrator</option>
            </select>
          </div>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button @click="isUserModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">Cancel</button>
          <button @click="createUser" class="px-4 py-1.5 text-xs bg-brand-500 hover:bg-brand-600 text-white font-semibold rounded-lg shadow-lg shadow-brand-500/20">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>
