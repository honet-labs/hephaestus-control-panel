<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { Search, RefreshCw, HardDrive, Server, Layers } from 'lucide-vue-next';

const health = ref<any>(null);
const nodes = ref<any>(null);
const loading = ref(false);

const fetchData = async () => {
  loading.value = true;
  try {
    const [healthRes, nodesRes] = await Promise.all([
      axios.get('/api/v1/opensearch/health'),
      axios.get('/api/v1/opensearch/nodes'),
    ]);
    if (healthRes.data.success) health.value = healthRes.data.data;
    if (nodesRes.data.success) nodes.value = nodesRes.data.data;
  } catch (err) {
    console.error('Failed to load OpenSearch metrics:', err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchData();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">OpenSearch Cluster</h2>
        <p class="text-xs text-slate-400">Cluster health, node JVM metrics, and shard allocation</p>
      </div>
      <button
        @click="fetchData"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
      >
        <RefreshCw class="w-3.5 h-3.5 text-brand-400" />
        Refresh
      </button>
    </div>

    <!-- Cluster Status Cards -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
      <div class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-1">
        <span class="text-xs text-slate-400">Cluster Status</span>
        <div class="flex items-center gap-2">
          <span :class="[
            health?.status === 'green' ? 'bg-emerald-400' : health?.status === 'yellow' ? 'bg-amber-400' : 'bg-red-400',
            'w-3 h-3 rounded-full'
          ]"></span>
          <span class="text-xl font-bold uppercase text-white">{{ health?.status || 'UNKNOWN' }}</span>
        </div>
      </div>

      <div class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-1">
        <span class="text-xs text-slate-400">Nodes Count</span>
        <p class="text-xl font-bold text-white">{{ health?.number_of_nodes || 0 }} <span class="text-xs font-normal text-slate-500">({{ health?.number_of_data_nodes || 0 }} data)</span></p>
      </div>

      <div class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-1">
        <span class="text-xs text-slate-400">Active Primary Shards</span>
        <p class="text-xl font-bold text-white">{{ health?.active_primary_shards || 0 }}</p>
      </div>

      <div class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-1">
        <span class="text-xs text-slate-400">Unassigned Shards</span>
        <p class="text-xl font-bold text-white" :class="health?.unassigned_shards > 0 ? 'text-amber-400' : 'text-slate-300'">{{ health?.unassigned_shards || 0 }}</p>
      </div>
    </div>

    <!-- Raw Nodes JSON Inspector -->
    <div class="flex-1 bg-slate-900/60 border border-slate-800 rounded-xl p-4 overflow-y-auto font-mono text-xs">
      <h3 class="text-xs font-bold text-slate-300 mb-2">Cluster Telemetry Data</h3>
      <pre class="text-slate-400 text-[11px] leading-relaxed">{{ JSON.stringify({ health, nodes }, null, 2) }}</pre>
    </div>
  </div>
</template>
