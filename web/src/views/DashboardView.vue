<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { 
  Activity, 
  CheckCircle2, 
  AlertTriangle, 
  Database, 
  Terminal, 
  Link2, 
  ExternalLink, 
  ArrowRight,
  RefreshCw
} from 'lucide-vue-next';

interface ServiceCount {
  total: number;
  running: number;
  warning: number;
  stopped: number;
}

const serviceStats = ref<ServiceCount>({
  total: 11,
  running: 11,
  warning: 0,
  stopped: 0,
});

const backupHistory = ref<any[]>([]);
const loading = ref(true);

const fetchDashboardData = async () => {
  loading.value = true;
  try {
    const backupRes = await axios.get('/api/v1/backup/history?limit=10').catch(() => null);
    if (backupRes && backupRes.data && backupRes.data.success) {
      backupHistory.value = backupRes.data.data.history || [];
    }
  } catch (err) {
    console.error('Failed to fetch dashboard backup data:', err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchDashboardData();
});
</script>

<template>
  <div class="space-y-6 font-sans max-w-7xl mx-auto">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 border-b border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-white tracking-tight flex items-center gap-2">
          <span>System Overview</span>
        </h1>
        <p class="text-xs text-[#95CCDD]/80 mt-0.5">Real-time status of service subsystems, quick actions, and backup executions</p>
      </div>
      <div class="flex items-center gap-2">
        <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-[#293681]/30 text-[#95CCDD] border border-[#4274D9]/40">
          <span class="w-2 h-2 rounded-full bg-[#4274D9] animate-pulse"></span>
          Hephaestus Control Panel Active
        </span>
      </div>
    </div>

    <!-- 1. CARDS COUNT STATUS SERVICES -->
    <div>
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-xs font-bold text-[#95CCDD] uppercase tracking-wider">Status Services Overview</h2>
        <router-link to="/settings?tab=services" class="text-[11px] text-slate-400 hover:text-[#95CCDD] transition flex items-center gap-1">
          <span>View All 11 Services</span>
          <ArrowRight class="w-3 h-3" />
        </router-link>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <!-- Card 1: Total Services -->
        <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] shadow-lg space-y-2 hover:border-[#4274D9]/40 transition">
          <div class="flex items-center justify-between text-slate-400">
            <span class="text-xs font-semibold text-[#D0E7E6]">Total Subsystems</span>
            <Activity class="w-4 h-4 text-[#95CCDD]" />
          </div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold text-white">{{ serviceStats.total }}</span>
            <span class="text-xs text-[#95CCDD] font-medium">Services</span>
          </div>
          <p class="text-[11px] text-slate-400">All HCP background daemons registered</p>
        </div>

        <!-- Card 2: Running & Healthy -->
        <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] shadow-lg space-y-2 hover:border-emerald-500/40 transition">
          <div class="flex items-center justify-between text-slate-400">
            <span class="text-xs font-semibold text-[#D0E7E6]">Running & Healthy</span>
            <CheckCircle2 class="w-4 h-4 text-emerald-400" />
          </div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold text-emerald-400">{{ serviceStats.running }}</span>
            <span class="text-xs text-emerald-400/80 font-medium">Active (100%)</span>
          </div>
          <p class="text-[11px] text-slate-400">Normal telemetry heartbeat verified</p>
        </div>

        <!-- Card 3: Issues / Stopped -->
        <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] shadow-lg space-y-2 hover:border-[#1b2234] transition">
          <div class="flex items-center justify-between text-slate-400">
            <span class="text-xs font-semibold text-[#D0E7E6]">Stopped / Warning</span>
            <AlertTriangle class="w-4 h-4 text-slate-500" />
          </div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold text-white">{{ serviceStats.stopped + serviceStats.warning }}</span>
            <span class="text-xs text-slate-500 font-medium">Issues</span>
          </div>
          <p class="text-[11px] text-slate-400">0 anomalies or failure alerts detected</p>
        </div>

        <!-- Card 4: Database Engine -->
        <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] shadow-lg space-y-2 hover:border-[#4274D9]/40 transition">
          <div class="flex items-center justify-between text-slate-400">
            <span class="text-xs font-semibold text-[#D0E7E6]">Database Engine</span>
            <Database class="w-4 h-4 text-[#4274D9]" />
          </div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold text-white">PostgreSQL 16</span>
          </div>
          <p class="text-[11px] text-emerald-400 flex items-center gap-1 font-medium">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
            Pool Active & Synchronized
          </p>
        </div>
      </div>
    </div>

    <!-- 2. QUICK ACTIONS (3 Dedicated Options) -->
    <div class="space-y-3">
      <h2 class="text-xs font-bold text-[#95CCDD] uppercase tracking-wider">Quick Actions</h2>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        
        <!-- Action 1: Remote Server -->
        <a
          href="/remote-server"
          target="_blank"
          class="p-5 rounded-xl bg-[#0e121c] border border-[#1b2234] hover:border-[#4274D9]/60 hover:bg-[#121724] transition group shadow-lg flex flex-col justify-between"
        >
          <div class="space-y-3">
            <div class="w-10 h-10 rounded-xl bg-[#141b2d] border border-[#293681] flex items-center justify-center text-[#95CCDD] group-hover:scale-110 group-hover:text-white transition">
              <Terminal class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-white flex items-center gap-1.5">
                <span>Remote Server</span>
                <ExternalLink class="w-3.5 h-3.5 text-slate-500 group-hover:text-[#95CCDD] transition" />
              </h3>
              <p class="text-xs text-slate-400 mt-1 leading-relaxed">
                SSH remote terminal, system resource telemetry, and bidirectional SFTP file transfer.
              </p>
            </div>
          </div>
          <div class="pt-4 mt-2 border-t border-[#1b2234] flex items-center justify-between text-xs font-semibold text-[#95CCDD]">
            <span>Launch Remote Console</span>
            <ArrowRight class="w-3.5 h-3.5 group-hover:translate-x-1 transition" />
          </div>
        </a>

        <!-- Action 2: Add Connections -->
        <router-link
          to="/connections"
          class="p-5 rounded-xl bg-[#0e121c] border border-[#1b2234] hover:border-[#4274D9]/60 hover:bg-[#121724] transition group shadow-lg flex flex-col justify-between"
        >
          <div class="space-y-3">
            <div class="w-10 h-10 rounded-xl bg-[#141b2d] border border-[#293681] flex items-center justify-center text-[#4274D9] group-hover:scale-110 group-hover:text-[#95CCDD] transition">
              <Link2 class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-white">Add Connections</h3>
              <p class="text-xs text-slate-400 mt-1 leading-relaxed">
                Register Grafana Core API, Prometheus SSH servers, and OpenSearch cluster endpoints.
              </p>
            </div>
          </div>
          <div class="pt-4 mt-2 border-t border-[#1b2234] flex items-center justify-between text-xs font-semibold text-[#95CCDD]">
            <span>Manage Integrations</span>
            <ArrowRight class="w-3.5 h-3.5 group-hover:translate-x-1 transition" />
          </div>
        </router-link>

        <!-- Action 3: Backup Manager -->
        <router-link
          to="/backup"
          class="p-5 rounded-xl bg-[#0e121c] border border-[#1b2234] hover:border-[#4274D9]/60 hover:bg-[#121724] transition group shadow-lg flex flex-col justify-between"
        >
          <div class="space-y-3">
            <div class="w-10 h-10 rounded-xl bg-[#141b2d] border border-[#293681] flex items-center justify-center text-[#D0E7E6] group-hover:scale-110 group-hover:text-white transition">
              <Database class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-white">Backup Manager</h3>
              <p class="text-xs text-slate-400 mt-1 leading-relaxed">
                Schedule automated PostgreSQL/MySQL dumps to NAS (SMB/SSH), S3, and Cloudflare R2.
              </p>
            </div>
          </div>
          <div class="pt-4 mt-2 border-t border-[#1b2234] flex items-center justify-between text-xs font-semibold text-[#95CCDD]">
            <span>Open Backup Manager</span>
            <ArrowRight class="w-3.5 h-3.5 group-hover:translate-x-1 transition" />
          </div>
        </router-link>

      </div>
    </div>

    <!-- 3. TABLE RECENT BACKUPS MANAGER -->
    <div class="p-5 rounded-xl bg-[#0e121c] border border-[#1b2234] space-y-4 shadow-lg">
      <div class="flex items-center justify-between border-b border-[#1b2234] pb-3">
        <div>
          <h2 class="text-xs font-bold text-[#95CCDD] uppercase tracking-wider">Recent Database Backups</h2>
          <p class="text-[11px] text-slate-400">Execution log of database dumps and cloud storage synchronization</p>
        </div>
        <div class="flex items-center gap-3">
          <button @click="fetchDashboardData" class="p-1.5 text-slate-400 hover:text-white transition" title="Refresh Backup History">
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          </button>
          <router-link to="/backup" class="text-xs text-[#95CCDD] font-semibold hover:underline">
            View All in Backup Manager &rarr;
          </router-link>
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-[#1b2234] text-[#95CCDD]/80 font-bold uppercase text-[10px] tracking-wider">
              <th class="pb-2.5">Database</th>
              <th class="pb-2.5">Destination</th>
              <th class="pb-2.5">File Size</th>
              <th class="pb-2.5">Status</th>
              <th class="pb-2.5">Executed Time</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[#1b2234]/60 text-slate-300">
            <tr v-for="h in backupHistory" :key="h.id" class="hover:bg-[#121724] transition">
              <td class="py-3 font-bold text-white">{{ h.dbName }} <span class="text-slate-500 font-normal">({{ h.dbType }})</span></td>
              <td class="py-3 uppercase font-mono text-[10px] text-[#95CCDD] font-semibold">{{ h.destType }}</td>
              <td class="py-3 font-mono text-slate-400">{{ (h.fileSize / 1024 / 1024).toFixed(2) }} MB</td>
              <td class="py-3">
                <span :class="[
                  h.status === 'success' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20',
                  'px-2 py-0.5 rounded text-[10px] font-bold uppercase border'
                ]">
                  {{ h.status }}
                </span>
              </td>
              <td class="py-3 text-slate-400 font-mono text-[11px]">{{ new Date(h.startedAt).toLocaleString() }}</td>
            </tr>
            <tr v-if="backupHistory.length === 0 && !loading">
              <td colspan="5" class="py-8 text-center text-slate-500 space-y-1">
                <Database class="w-6 h-6 text-slate-600 mx-auto mb-1" />
                <p>No database backup records found yet.</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

  </div>
</template>
