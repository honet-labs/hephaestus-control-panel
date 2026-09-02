<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { 
  Server, 
  Database, 
  Network, 
  Activity, 
  Terminal, 
  Cpu, 
  HardDrive, 
  CheckCircle2, 
  AlertTriangle,
  Layers,
  Search,
  Globe
} from 'lucide-vue-next';

const stats = ref<any>(null);
const topologySummary = ref<any>({ total: 0, online: 0, offline: 0 });
const backupHistory = ref<any[]>([]);
const loading = ref(true);

const fetchDashboardData = async () => {
  try {
    const [statsRes, topoRes, backupRes] = await Promise.all([
      axios.get('/api/v1/settings/system'),
      axios.get('/api/v1/topology'),
      axios.get('/api/v1/backup/history?limit=5'),
    ]);

    if (statsRes.data.success) stats.value = statsRes.data.data;
    if (topoRes.data.success) {
      const nodes = topoRes.data.data.nodes || [];
      const online = nodes.filter((n: any) => n.status === 'online').length;
      topologySummary.value = {
        total: nodes.length,
        online,
        offline: nodes.length - online,
      };
    }
    if (backupRes.data.success) backupHistory.value = backupRes.data.data.history || [];
  } catch (err) {
    console.error('Failed to fetch dashboard data:', err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchDashboardData();
});
</script>

<template>
  <div class="space-y-6 font-sans">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">System Overview</h2>
        <p class="text-xs text-[#95CCDD]/80">Real-time status of your infrastructure and services</p>
      </div>
      <div class="flex items-center gap-2">
        <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-[#293681]/30 text-[#95CCDD] border border-[#4274D9]/40">
          <span class="w-2 h-2 rounded-full bg-[#4274D9] animate-pulse"></span>
          Hephaestus Control Panel Active
        </span>
      </div>
    </div>

    <!-- Metrics Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <!-- Card 1: Server Stats -->
      <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] backdrop-blur space-y-3 shadow-lg">
        <div class="flex items-center justify-between text-slate-400">
          <span class="text-xs font-semibold text-[#D0E7E6]">Memory Allocation</span>
          <Cpu class="w-4 h-4 text-[#95CCDD]" />
        </div>
        <div class="flex items-baseline gap-2">
          <span class="text-2xl font-bold text-white">{{ stats?.memoryAllocMb ? stats.memoryAllocMb.toFixed(1) : '0' }}</span>
          <span class="text-xs text-slate-500 font-mono">MB / {{ stats?.memoryTotalMb ? stats.memoryTotalMb.toFixed(0) : '0' }} MB</span>
        </div>
        <div class="w-full bg-[#161c2c] h-1.5 rounded-full overflow-hidden">
          <div 
            class="bg-gradient-to-r from-[#293681] to-[#4274D9] h-full rounded-full transition-all duration-500"
            :style="{ width: `${stats ? Math.min((stats.memoryAllocMb / stats.memoryTotalMb) * 100, 100) : 0}%` }"
          ></div>
        </div>
      </div>

      <!-- Card 2: Topology Devices -->
      <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] backdrop-blur space-y-3 shadow-lg">
        <div class="flex items-center justify-between text-slate-400">
          <span class="text-xs font-semibold text-[#D0E7E6]">Network Devices</span>
          <Network class="w-4 h-4 text-[#95CCDD]" />
        </div>
        <div class="flex items-baseline gap-2">
          <span class="text-2xl font-bold text-white">{{ topologySummary.total }}</span>
          <span class="text-xs text-emerald-400 font-mono">({{ topologySummary.online }} online)</span>
        </div>
        <div class="text-[11px] text-slate-500 flex items-center justify-between">
          <span>{{ topologySummary.offline }} offline</span>
          <a href="/network-topology" target="_blank" class="text-[#95CCDD] hover:text-white font-medium hover:underline">View Map &rarr;</a>
        </div>
      </div>

      <!-- Card 3: Database Status -->
      <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] backdrop-blur space-y-3 shadow-lg">
        <div class="flex items-center justify-between text-slate-400">
          <span class="text-xs font-semibold text-[#D0E7E6]">PostgreSQL Database</span>
          <Database class="w-4 h-4 text-[#4274D9]" />
        </div>
        <div class="flex items-baseline gap-2">
          <span class="text-2xl font-bold text-white">{{ stats?.databaseStatus || 'CONNECTED' }}</span>
        </div>
        <div class="text-[11px] text-slate-400 flex items-center gap-1.5">
          <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
          <span>Pool Healthy & Synchronized</span>
        </div>
      </div>

      <!-- Card 4: Goroutines / Uptime -->
      <div class="p-4 rounded-xl bg-[#0e121c] border border-[#1b2234] backdrop-blur space-y-3 shadow-lg">
        <div class="flex items-center justify-between text-slate-400">
          <span class="text-xs font-semibold text-[#D0E7E6]">Active Goroutines</span>
          <Activity class="w-4 h-4 text-[#95CCDD]" />
        </div>
        <div class="flex items-baseline gap-2">
          <span class="text-2xl font-bold text-white">{{ stats?.goroutineCount || '8' }}</span>
          <span class="text-xs text-slate-500 font-mono">concurrency workers</span>
        </div>
        <div class="text-[11px] text-slate-400">
          Uptime: <span class="font-mono text-[#D0E7E6]">{{ stats ? Math.floor(stats.uptimeSeconds / 60) : 0 }}m</span>
        </div>
      </div>
    </div>

    <!-- Quick Tools & Recent Backups Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Fast Actions Panel -->
      <div class="p-5 rounded-xl bg-[#0e121c] border border-[#1b2234] space-y-4 shadow-lg">
        <h3 class="text-sm font-bold text-white">Quick Actions</h3>
        <div class="grid grid-cols-2 gap-2.5">
          <a href="/remote-server" target="_blank" class="p-3 rounded-lg bg-[#131825] hover:bg-[#1a2133] border border-[#1b2234] hover:border-[#4274D9]/50 text-left transition group">
            <Terminal class="w-5 h-5 text-[#95CCDD] mb-2 group-hover:scale-110 transition" />
            <p class="text-xs font-bold text-white">Remote Server</p>
            <p class="text-[10px] text-slate-400">SSH & Telemetry</p>
          </a>

          <router-link to="/backup" class="p-3 rounded-lg bg-[#131825] hover:bg-[#1a2133] border border-[#1b2234] hover:border-[#4274D9]/50 text-left transition group">
            <Database class="w-5 h-5 text-[#4274D9] mb-2 group-hover:scale-110 transition" />
            <p class="text-xs font-bold text-white">Run Backup</p>
            <p class="text-[10px] text-slate-400">Dump to NFS/S3</p>
          </router-link>

          <a href="/network-topology" target="_blank" class="p-3 rounded-lg bg-[#131825] hover:bg-[#1a2133] border border-[#1b2234] hover:border-[#4274D9]/50 text-left transition group">
            <Network class="w-5 h-5 text-[#95CCDD] mb-2 group-hover:scale-110 transition" />
            <p class="text-xs font-bold text-white">Scan Subnet</p>
            <p class="text-[10px] text-slate-400">Discover IP devices</p>
          </a>

          <router-link to="/slideshow" class="p-3 rounded-lg bg-[#131825] hover:bg-[#1a2133] border border-[#1b2234] hover:border-[#4274D9]/50 text-left transition group">
            <Globe class="w-5 h-5 text-[#D0E7E6] mb-2 group-hover:scale-110 transition" />
            <p class="text-xs font-bold text-white">Slide Show</p>
            <p class="text-[10px] text-slate-400">NOC Wall Embed</p>
          </router-link>
        </div>
      </div>

      <!-- Recent Backup Executions -->
      <div class="lg:col-span-2 p-5 rounded-xl bg-[#0e121c] border border-[#1b2234] space-y-4 shadow-lg">
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-bold text-white">Recent Database Backups</h3>
          <router-link to="/backup" class="text-xs text-[#95CCDD] font-semibold hover:underline">View All &rarr;</router-link>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead>
              <tr class="border-b border-[#1b2234] text-[#95CCDD]/80 font-bold uppercase text-[10px] tracking-wider">
                <th class="pb-2">Database</th>
                <th class="pb-2">Destination</th>
                <th class="pb-2">Size</th>
                <th class="pb-2">Status</th>
                <th class="pb-2">Time</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-[#1b2234]/60 text-slate-300">
              <tr v-for="h in backupHistory" :key="h.id" class="hover:bg-[#131825] transition">
                <td class="py-2.5 font-bold text-white">{{ h.dbName }} ({{ h.dbType }})</td>
                <td class="py-2.5 uppercase font-mono text-[10px] text-[#95CCDD] font-semibold">{{ h.destType }}</td>
                <td class="py-2.5 font-mono text-slate-400">{{ (h.fileSize / 1024 / 1024).toFixed(2) }} MB</td>
                <td class="py-2.5">
                  <span :class="[
                    h.status === 'success' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20',
                    'px-2 py-0.5 rounded text-[10px] font-bold uppercase border'
                  ]">
                    {{ h.status }}
                  </span>
                </td>
                <td class="py-2.5 text-slate-400 font-mono text-[11px]">{{ new Date(h.startedAt).toLocaleString() }}</td>
              </tr>
              <tr v-if="backupHistory.length === 0">
                <td colspan="5" class="py-6 text-center text-slate-500">No backup records yet</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
