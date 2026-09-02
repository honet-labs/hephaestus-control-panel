<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import axios from 'axios';
import {
  Clock,
  RotateCw,
  XCircle,
  CheckCircle2,
  AlertCircle,
  Server,
  Radio,
  Bell,
  Search,
  FileText,
  Zap,
  HardDrive,
  Network,
  Cpu,
  Settings,
  HelpCircle,
  LogOut,
  SlidersHorizontal,
  Trash2,
  Play,
  Filter,
  Wand2,
  Layers,
  Database,
  Eye,
  Activity,
} from 'lucide-vue-next';
import { useAuthStore } from '../stores/auth';
import { useRouter } from 'vue-router';

const authStore = useAuthStore();
const router = useRouter();

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

interface EngineServer {
  id: string;
  name: string;
  status: 'active' | 'warning' | 'stopped';
  type: string;
  icon: string;
  master: boolean;
  version: string;
  modules: string;
  lag: string;
  tq: string;
  updated: string;
  description: string;
}

const jobs = ref<Job[]>([]);
const loading = ref(false);
const filterQuery = ref('');
const activeFilterType = ref('all');
const timer = ref<any>(null);

// Pandora FMS Style Engine Servers List
const engineServers = ref<EngineServer[]>([
  {
    id: 'srv-icmp',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Network server (ICMP Sweep)',
    icon: 'network',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '309 of 309',
    lag: '- / 0',
    tq: '6 : 46',
    updated: '4 seconds',
    description: 'Continuous ICMP ping sweep & latency monitor across subnets',
  },
  {
    id: 'srv-opensearch',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Data server (OpenSearch Poller)',
    icon: 'search',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '2797 of 2797',
    lag: '20 seconds / 41',
    tq: '6 : 1',
    updated: '5 seconds',
    description: 'Real-time OpenSearch cluster health, nodes stats, and shard telemetry',
  },
  {
    id: 'srv-backup',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Backup server (PostgreSQL / MySQL)',
    icon: 'database',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '24 of 24',
    lag: '- / 0',
    tq: '2 : 0',
    updated: '18 seconds',
    description: 'Scheduled automated database dumps & AWS S3 cloud archiving',
  },
  {
    id: 'srv-snmp',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'SNMP trap server',
    icon: 'radio',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: 'N/A',
    lag: 'N/A',
    tq: '0 : 0',
    updated: '18 seconds',
    description: 'SNMP v2c/v3 traps ingestion, OID polling, and MIB dictionary compiler',
  },
  {
    id: 'srv-discovery',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Discovery server',
    icon: 'discovery',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '2 of 2',
    lag: '- / 0',
    tq: '6 : 0',
    updated: '4 seconds',
    description: 'Automated topology discovery, MAC address ARP tables, and subnet probing',
  },
  {
    id: 'srv-alert',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Alert server',
    icon: 'bell',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '0 of 0',
    lag: '- / 0',
    tq: '4 : 0',
    updated: '6 seconds',
    description: 'Immediate Telegram, Discord, and Webhook alert notification dispatcher',
  },
  {
    id: 'srv-log',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Log server',
    icon: 'log',
    master: false,
    version: '2.0.0 (Go 1.22)',
    modules: '0 of 0',
    lag: '- / 0',
    tq: '0 : 0',
    updated: '18 seconds',
    description: 'Centralized streaming log ingestion, Grok pattern parsing, and ring buffer',
  },
  {
    id: 'srv-heavy',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Heavy server (Cron Scheduler)',
    icon: 'heavy',
    master: false,
    version: '2.0.0 (Go 1.22)',
    modules: '386 of 386',
    lag: '17 seconds / 10',
    tq: '0 : 0',
    updated: '18 seconds',
    description: 'Robocron scheduler engine with precision millisecond execution timers',
  },
  {
    id: 'srv-worker',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'High performance server (Worker Pool)',
    icon: 'worker',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '603 of 603',
    lag: '4 minutes 47 seconds / 89',
    tq: '0 : 0',
    updated: '18 seconds',
    description: '5 Concurrent worker threads processing asynchronous queue tasks',
  },
]);

const filteredServers = computed(() => {
  if (!filterQuery.value) return engineServers.value;
  const q = filterQuery.value.toLowerCase();
  return engineServers.value.filter(
    s => s.name.toLowerCase().includes(q) || s.type.toLowerCase().includes(q) || s.description.toLowerCase().includes(q)
  );
});

const fetchJobs = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/queue/jobs');
    if (res.data.success) {
      jobs.value = res.data.data || [];
    }
  } catch (err) {
    console.error('Failed to fetch jobs:', err);
  } finally {
    loading.value = false;
  }
};

const triggerJob = async (jobType: string) => {
  try {
    await axios.post('/api/v1/queue/jobs/trigger', { type: jobType });
    fetchJobs();
  } catch (err) {
    console.error('Failed to trigger job:', err);
  }
};

const cancelJob = async (id: string) => {
  try {
    const res = await axios.post(`/api/v1/queue/jobs/${id}/cancel`);
    if (res.data.success) {
      fetchJobs();
    }
  } catch (err) {
    console.error('Failed to cancel job:', err);
  }
};

const handleLogout = async () => {
  await authStore.logout();
  router.push('/login');
};

onMounted(() => {
  fetchJobs();
  timer.value = setInterval(fetchJobs, 3000);
});

onUnmounted(() => {
  if (timer.value) clearInterval(timer.value);
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4 overflow-y-auto pr-1">
    
    <!-- Top Action & Status Bar (Matching Pandora FMS Screenshot Image 3) -->
    <div class="bg-[#1b1e26] border border-slate-800 rounded-xl p-3 px-5 flex items-center justify-between shadow-lg">
      <!-- Breadcrumb -->
      <div class="flex items-center gap-2 text-xs">
        <span class="text-slate-400">Servers</span>
        <span class="text-slate-600">/</span>
        <span class="text-white font-semibold">Manage Servers</span>
      </div>

      <!-- Center & Right Status Icons (Image 3) -->
      <div class="flex items-center gap-4 text-slate-300">
        <!-- History / Uptime Icon -->
        <button title="System Uptime: 100% OK" class="p-1.5 text-slate-400 hover:text-white transition">
          <Clock class="w-4 h-4" />
        </button>

        <!-- Red Alert Count Pill (Image 3) -->
        <div title="Active Alerts" class="px-2 py-0.5 rounded-full bg-red-600 text-white font-black text-[11px] shadow-sm shadow-red-600/30 flex items-center justify-center min-w-[22px]">
          0
        </div>

        <!-- Wand / Magic Cleaner -->
        <button @click="fetchJobs" title="Purge Completed Jobs" class="p-1.5 text-slate-400 hover:text-amber-400 transition">
          <Wand2 class="w-4 h-4" />
        </button>

        <!-- Database / Storage Orange Icon (Image 3) -->
        <div title="Database Engine: Online" class="p-1.5 text-amber-500 hover:text-amber-400 transition cursor-pointer">
          <Database class="w-4 h-4" />
        </div>

        <!-- Help Icon -->
        <button title="Documentation & Help" class="p-1.5 text-slate-400 hover:text-white transition">
          <HelpCircle class="w-4 h-4" />
        </button>

        <!-- Settings Gear Icon -->
        <button @click="router.push('/settings')" title="System Settings" class="p-1.5 text-slate-400 hover:text-white transition">
          <Settings class="w-4 h-4" />
        </button>

        <!-- User Badge [admin] -->
        <div class="flex items-center gap-1.5 px-3 py-1 rounded-full bg-slate-800/80 border border-slate-700 text-xs font-mono text-slate-200">
          <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
          <span>[{{ authStore.user?.username || 'admin' }}]</span>
        </div>

        <!-- Logout Action -->
        <button @click="handleLogout" title="Logout" class="p-1.5 text-slate-400 hover:text-red-400 transition">
          <LogOut class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Sub-Header Title & Quick Filter Bar -->
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-base font-bold text-white tracking-wide">Pandora FMS & Hephaestus Engine Servers</h2>
        <p class="text-xs text-slate-400">Active background daemons, queue workers, telemetry pollers, and schedulers</p>
      </div>

      <div class="flex items-center gap-2">
        <!-- Refresh Button -->
        <button
          @click="fetchJobs"
          class="p-2 bg-[#1b1e26] border border-slate-800 rounded-lg text-slate-300 hover:text-white hover:border-slate-700 transition"
          title="Refresh Table"
        >
          <RotateCw :class="['w-4 h-4', loading ? 'animate-spin text-brand-400' : '']" />
        </button>

        <!-- Search / Filter Input -->
        <div class="relative">
          <Search class="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500" />
          <input
            v-model="filterQuery"
            placeholder="Search daemon..."
            class="bg-[#1b1e26] border border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition w-44"
          />
        </div>

        <!-- Filter Button -->
        <button class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-700/80 bg-[#1b1e26] text-xs font-medium text-slate-200 hover:text-white transition">
          <Filter class="w-3.5 h-3.5 text-slate-400" />
          <span>Filters</span>
        </button>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- PANDORA FMS STYLE ENGINE SERVERS TABLE (Matching Screenshot Image 2) -->
    <!-- ================================================================= -->
    <div class="bg-[#1b1e26] border border-slate-800/90 rounded-xl overflow-hidden shadow-2xl">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
            <tr>
              <th class="py-3 px-4">Name</th>
              <th class="py-3 px-3">Status</th>
              <th class="py-3 px-4">Type</th>
              <th class="py-3 px-3">Master</th>
              <th class="py-3 px-4">Version</th>
              <th class="py-3 px-4">Modules</th>
              <th class="py-3 px-4">Lag</th>
              <th class="py-3 px-3">T/Q</th>
              <th class="py-3 px-4">Updated</th>
              <th class="py-3 px-4 text-right">Op.</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono text-[11px]">
            <tr
              v-for="srv in filteredServers"
              :key="srv.id"
              class="hover:bg-slate-800/30 transition text-slate-300"
            >
              <!-- Name -->
              <td class="py-3 px-4 font-sans font-medium text-slate-200">{{ srv.name }}</td>

              <!-- Status (Green Square Icon matching Image 2) -->
              <td class="py-3 px-3">
                <span class="inline-block w-2.5 h-2.5 rounded-[2px] bg-emerald-500 shadow-sm shadow-emerald-500/50"></span>
              </td>

              <!-- Type with Custom Daemon Icon -->
              <td class="py-3 px-4 font-sans text-slate-200">
                <div class="flex items-center gap-2">
                  <Network v-if="srv.icon === 'network'" class="w-4 h-4 text-emerald-400 shrink-0" />
                  <Search v-else-if="srv.icon === 'search'" class="w-4 h-4 text-sky-400 shrink-0" />
                  <Database v-else-if="srv.icon === 'database'" class="w-4 h-4 text-amber-400 shrink-0" />
                  <Radio v-else-if="srv.icon === 'radio'" class="w-4 h-4 text-purple-400 shrink-0" />
                  <Layers v-else-if="srv.icon === 'discovery'" class="w-4 h-4 text-indigo-400 shrink-0" />
                  <Bell v-else-if="srv.icon === 'bell'" class="w-4 h-4 text-rose-400 shrink-0" />
                  <FileText v-else-if="srv.icon === 'log'" class="w-4 h-4 text-blue-400 shrink-0" />
                  <Clock v-else-if="srv.icon === 'heavy'" class="w-4 h-4 text-amber-500 shrink-0" />
                  <Cpu v-else class="w-4 h-4 text-teal-400 shrink-0" />
                  <span class="truncate">{{ srv.type }}</span>
                </div>
              </td>

              <!-- Master -->
              <td class="py-3 px-3 font-sans text-slate-300">{{ srv.master ? 'Yes' : 'No' }}</td>

              <!-- Version -->
              <td class="py-3 px-4 text-slate-400">{{ srv.version }}</td>

              <!-- Modules -->
              <td class="py-3 px-4 text-slate-300 font-sans">{{ srv.modules }}</td>

              <!-- Lag -->
              <td class="py-3 px-4 text-slate-400">{{ srv.lag }}</td>

              <!-- T/Q -->
              <td class="py-3 px-3 font-bold text-white">{{ srv.tq }}</td>

              <!-- Updated -->
              <td class="py-3 px-4 text-slate-400 font-sans">{{ srv.updated }}</td>

              <!-- Op. Actions (Icons matching Image 2) -->
              <td class="py-3 px-4 text-right">
                <div class="flex items-center justify-end gap-2 text-slate-400">
                  <button title="Configure Daemon" class="hover:text-white transition">
                    <Settings class="w-3.5 h-3.5" />
                  </button>
                  <button title="Inspect Status" class="hover:text-brand-400 transition">
                    <Eye class="w-3.5 h-3.5" />
                  </button>
                  <button title="Restart Daemon Service" class="hover:text-amber-400 transition">
                    <RotateCw class="w-3.5 h-3.5" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="p-3 px-4 bg-[#14161b] border-t border-slate-800/80 text-[11px] text-slate-500 font-sans">
        Showing 1 to {{ filteredServers.length }} of {{ filteredServers.length }} entries
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- REAL-TIME ASYNCHRONOUS JOB QUEUE STREAM -->
    <!-- ================================================================= -->
    <div class="bg-[#1b1e26] border border-slate-800/90 rounded-xl p-5 space-y-4 shadow-xl">
      <div class="flex items-center justify-between border-b border-slate-800 pb-3">
        <div class="flex items-center gap-2">
          <Activity class="w-4 h-4 text-brand-400" />
          <h3 class="text-xs font-bold text-white uppercase tracking-wider">Live Asynchronous Job Queue (Worker Threads)</h3>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="triggerJob('icmp_ping_cycle')"
            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-medium transition"
          >
            + Ping Sweep
          </button>
          <button
            @click="triggerJob('opensearch_poll')"
            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-medium transition"
          >
            + OpenSearch Poll
          </button>
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="text-slate-400 text-[10px] uppercase font-bold border-b border-slate-800/60">
              <th class="py-2.5 px-3">Job ID</th>
              <th class="py-2.5 px-3">Task Type</th>
              <th class="py-2.5 px-3">Status</th>
              <th class="py-2.5 px-3">Progress</th>
              <th class="py-2.5 px-3">Message</th>
              <th class="py-2.5 px-3">Created</th>
              <th class="py-2.5 px-3 text-right">Action</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/50 font-mono text-[11px] text-slate-300">
            <tr v-for="job in jobs" :key="job.id" class="hover:bg-slate-800/20 transition">
              <td class="py-2.5 px-3 text-brand-400">{{ job.id }}</td>
              <td class="py-2.5 px-3 text-white font-sans">{{ job.type }}</td>
              <td class="py-2.5 px-3 font-sans">
                <span :class="[
                  job.status === 'completed' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' :
                  job.status === 'running' ? 'bg-blue-500/10 text-blue-400 border border-blue-500/20 animate-pulse' :
                  job.status === 'failed' ? 'bg-red-500/10 text-red-400 border border-red-500/20' :
                  'bg-slate-800 text-slate-400',
                  'px-2 py-0.5 rounded text-[10px] uppercase font-bold'
                ]">
                  {{ job.status }}
                </span>
              </td>
              <td class="py-2.5 px-3">
                <div class="w-24 bg-slate-800 h-1.5 rounded-full overflow-hidden">
                  <div class="bg-brand-500 h-full rounded-full transition-all" :style="{ width: `${job.progress}%` }"></div>
                </div>
              </td>
              <td class="py-2.5 px-3 text-slate-400 truncate max-w-xs font-sans">{{ job.message }}</td>
              <td class="py-2.5 px-3 text-slate-500 text-[10px]">{{ new Date(job.createdAt).toLocaleTimeString() }}</td>
              <td class="py-2.5 px-3 text-right font-sans">
                <button
                  v-if="job.status === 'running' || job.status === 'pending'"
                  @click="cancelJob(job.id)"
                  class="px-2 py-0.5 text-[10px] font-bold text-red-400 hover:bg-red-500/10 rounded transition"
                >
                  Cancel
                </button>
              </td>
            </tr>
            <tr v-if="jobs.length === 0">
              <td colspan="7" class="py-6 text-center text-slate-500 font-sans text-xs">
                No active background tasks in execution queue.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
