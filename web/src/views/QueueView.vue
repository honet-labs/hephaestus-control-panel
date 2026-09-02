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
const timer = ref<any>(null);

// Engine Services List (9 Core Daemons)
const engineServers = ref<EngineServer[]>([
  {
    id: 'srv-icmp',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Network Server (ICMP Sweep)',
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
    type: 'Data Server (OpenSearch Poller)',
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
    type: 'Backup Server (PostgreSQL / MySQL)',
    icon: 'database',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '24 of 24',
    lag: '- / 0',
    tq: '2 : 0',
    updated: '18 seconds',
    description: 'Scheduled automated database dumps & cloud archiving',
  },
  {
    id: 'srv-snmp',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'SNMP Trap & Poller Server',
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
    type: 'Discovery Server (ARP / Subnet)',
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
    type: 'Alert & Notification Server',
    icon: 'bell',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '15 of 15',
    lag: '- / 0',
    tq: '2 : 0',
    updated: '7 seconds',
    description: 'Threshold evaluation, incident escalation, and webhook notifications',
  },
  {
    id: 'srv-prometheus',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Prometheus & PromQL Collector',
    icon: 'log',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '500 of 500',
    lag: '- / 0',
    tq: '4 : 0',
    updated: '12 seconds',
    description: 'High-frequency metric ingestion from node exporters and Prometheus daemons',
  },
  {
    id: 'srv-heavy',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Heavy Background Worker Pool',
    icon: 'heavy',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: 'N/A',
    lag: 'N/A',
    tq: '0 : 0',
    updated: '18 seconds',
    description: '5 Concurrent worker threads for async batch tasks, exports, and heavy jobs',
  },
  {
    id: 'srv-prediction',
    name: 'labs-hcp-master',
    status: 'active',
    type: 'Prediction & Log Parser (Grok Engine)',
    icon: 'prediction',
    master: true,
    version: '2.0.0 (Go 1.22)',
    modules: '10 of 10',
    lag: '- / 0',
    tq: '1 : 0',
    updated: '18 seconds',
    description: 'Pattern matching, log structure transformation, and telemetry forecasting',
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
  <div class="h-full flex flex-col space-y-5 overflow-y-auto pr-1 select-none">
    
    <!-- Header Title & Quick Filter Bar -->
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-base font-bold text-white tracking-wide">Status Services</h2>
        <p class="text-xs text-slate-400">Real-time status of backend daemons, telemetry pollers, and asynchronous queue workers</p>
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
    <!-- SERVICES TABLE (9 CORE DAEMONS) -->
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

              <!-- Status (Green Square Icon) -->
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

              <!-- Op. Actions -->
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
          <h3 class="text-xs font-bold text-white tracking-wide uppercase">Live Asynchronous Job Queue (Worker Threads)</h3>
        </div>

        <!-- Quick Trigger Actions -->
        <div class="flex items-center gap-2">
          <button
            @click="triggerJob('icmp_ping_cycle')"
            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-200 font-medium transition"
          >
            + Ping Sweep
          </button>
          <button
            @click="triggerJob('opensearch_poll')"
            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-200 font-medium transition"
          >
            + OpenSearch Poll
          </button>
        </div>
      </div>

      <!-- Jobs Table -->
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
            <tr>
              <th class="py-2.5 px-3">Job ID</th>
              <th class="py-2.5 px-3">Task Type</th>
              <th class="py-2.5 px-3">Status</th>
              <th class="py-2.5 px-3">Progress</th>
              <th class="py-2.5 px-3">Message</th>
              <th class="py-2.5 px-3">Created</th>
              <th class="py-2.5 px-3 text-right">Action</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono text-[11px]">
            <tr
              v-for="job in jobs"
              :key="job.id"
              class="hover:bg-slate-800/20 transition text-slate-300"
            >
              <td class="py-2.5 px-3 text-slate-400">{{ job.id }}</td>
              <td class="py-2.5 px-3 text-white font-sans">{{ job.type }}</td>
              <td class="py-2.5 px-3 font-sans">
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
              <td class="py-2.5 px-3">
                <div class="w-24 h-1.5 bg-slate-800 rounded-full overflow-hidden">
                  <div
                    class="h-full bg-emerald-400 rounded-full"
                    :style="{ width: `${job.progress}%` }"
                  ></div>
                </div>
              </td>
              <td class="py-2.5 px-3 max-w-xs truncate text-slate-400 font-sans">{{ job.message }}</td>
              <td class="py-2.5 px-3 text-slate-500">{{ job.createdAt ? new Date(job.createdAt).toLocaleTimeString() : '-' }}</td>
              <td class="py-2.5 px-3 text-right font-sans">
                <button
                  v-if="job.status === 'RUNNING' || job.status === 'PENDING'"
                  @click="cancelJob(job.id)"
                  class="px-2 py-0.5 rounded bg-red-600/80 hover:bg-red-500 text-white text-[10px] transition"
                >
                  Cancel
                </button>
                <span v-else class="text-slate-600 text-[10px]">-</span>
              </td>
            </tr>
            <tr v-if="jobs.length === 0">
              <td colspan="7" class="py-6 text-center text-slate-500 text-xs font-sans">
                No active jobs in queue.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

  </div>
</template>
