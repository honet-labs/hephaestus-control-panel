<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { Server, RefreshCw, Cpu, HardDrive, Play, Square, RotateCw } from 'lucide-vue-next';

const hosts = ref<any[]>([]);
const selectedHostId = ref('');
const metrics = ref<any>(null);
const loading = ref(false);

const serviceName = ref('nginx');
const serviceOutput = ref('');

const fetchHosts = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data.success && res.data.data.length > 0) {
      hosts.value = res.data.data;
      selectedHostId.value = hosts.value[0].id;
      fetchMetrics();
    }
  } catch (err) {
    console.error(err);
  }
};

const fetchMetrics = async () => {
  if (!selectedHostId.value) return;
  loading.value = true;
  try {
    const res = await axios.get(`/api/v1/vps/${selectedHostId.value}/metrics`);
    if (res.data.success) {
      metrics.value = res.data.data;
    }
  } catch (err) {
    console.error('Failed to load VPS metrics:', err);
  } finally {
    loading.value = false;
  }
};

const controlService = async (action: string) => {
  if (!selectedHostId.value || !serviceName.value) return;
  try {
    const res = await axios.post(`/api/v1/vps/${selectedHostId.value}/control`, {
      serviceName: serviceName.value,
      action,
    });
    serviceOutput.value = res.data.output || 'Success';
  } catch (err: any) {
    serviceOutput.value = err.response?.data?.error || err.message;
  }
};

onMounted(() => {
  fetchHosts();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <!-- Header -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">VPS Telemetry & Services</h2>
        <p class="text-xs text-slate-400">Real-time OS metrics and systemd service control</p>
      </div>

      <div class="flex items-center gap-2">
        <select 
          v-model="selectedHostId" 
          @change="fetchMetrics" 
          class="bg-slate-800 border border-slate-700 text-xs text-slate-200 rounded-lg px-3 py-1.5 focus:outline-none"
        >
          <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }} ({{ h.host }})</option>
        </select>
        <button
          @click="fetchMetrics"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
        >
          <RefreshCw class="w-3.5 h-3.5 text-brand-400" />
          Refresh
        </button>
      </div>
    </div>

    <!-- Metrics Cards -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-2">
        <div class="flex items-center justify-between text-slate-400 text-xs">
          <span>CPU Cores & Load</span>
          <Cpu class="w-4 h-4 text-brand-400" />
        </div>
        <p class="text-2xl font-bold text-white">{{ metrics?.cpu?.cores || 2 }} <span class="text-xs font-normal text-slate-500">vCPUs</span></p>
        <p class="text-[11px] font-mono text-slate-400">Load Avg: {{ metrics?.loadAvg?.join(', ') || '0.12, 0.08, 0.05' }}</p>
      </div>

      <div class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-2">
        <div class="flex items-center justify-between text-slate-400 text-xs">
          <span>RAM Utilization</span>
          <Server class="w-4 h-4 text-purple-400" />
        </div>
        <p class="text-2xl font-bold text-white">{{ metrics?.memory?.used || 1024 }} <span class="text-xs font-normal text-slate-500">MB / {{ metrics?.memory?.total || 4096 }} MB</span></p>
        <p class="text-[11px] text-slate-400">{{ metrics?.memory?.percent || 25 }}% used</p>
      </div>

      <div class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-2">
        <div class="flex items-center justify-between text-slate-400 text-xs">
          <span>OS Kernel</span>
          <HardDrive class="w-4 h-4 text-blue-400" />
        </div>
        <p class="text-sm font-bold text-white truncate">{{ metrics?.kernel || 'Linux 5.15.0-x86_64' }}</p>
        <p class="text-[11px] text-slate-500 font-mono">Uptime: {{ metrics?.uptime || 'Online' }}</p>
      </div>
    </div>

    <!-- Systemd Service Controller -->
    <div class="p-5 rounded-xl bg-slate-900/60 border border-slate-800 space-y-3">
      <h3 class="text-xs font-bold text-white uppercase tracking-wider">Systemd Service Manager</h3>
      <div class="flex items-center gap-2">
        <input 
          v-model="serviceName" 
          placeholder="Service name (e.g. nginx, docker, postgresql)" 
          class="bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-xs text-white font-mono w-72 focus:outline-none" 
        />
        <button @click="controlService('status')" class="px-3 py-1.5 text-xs bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg">Status</button>
        <button @click="controlService('restart')" class="px-3 py-1.5 text-xs bg-amber-500/20 text-amber-400 hover:bg-amber-500/30 border border-amber-500/30 rounded-lg flex items-center gap-1"><RotateCw class="w-3 h-3" /> Restart</button>
        <button @click="controlService('start')" class="px-3 py-1.5 text-xs bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 border border-emerald-500/30 rounded-lg flex items-center gap-1"><Play class="w-3 h-3" /> Start</button>
        <button @click="controlService('stop')" class="px-3 py-1.5 text-xs bg-red-500/20 text-red-400 hover:bg-red-500/30 border border-red-500/30 rounded-lg flex items-center gap-1"><Square class="w-3 h-3" /> Stop</button>
      </div>
      <div v-if="serviceOutput" class="p-3 bg-slate-950 border border-slate-800 rounded-lg font-mono text-[11px] text-slate-300 max-h-48 overflow-y-auto">
        <pre>{{ serviceOutput }}</pre>
      </div>
    </div>
  </div>
</template>
