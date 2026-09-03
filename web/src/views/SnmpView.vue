<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { Radio, Search, Upload, Trash2 } from 'lucide-vue-next';

const mibs = ref<any[]>([]);
const queryForm = ref({
  host: '192.168.1.1',
  port: 161,
  version: '2c',
  community: 'public',
  oid: '1.3.6.1.2.1.1.1.0',
  operation: 'walk',
});

const queryResults = ref<any[]>([]);
const loading = ref(false);

const fetchMibs = async () => {
  try {
    const res = await axios.get('/api/v1/snmp/mibs');
    if (res.data.success) {
      mibs.value = res.data.data || [];
    }
  } catch (err) {
    console.error(err);
  }
};

const executeQuery = async () => {
  loading.value = true;
  try {
    const res = await axios.post('/api/v1/snmp/query', queryForm.value);
    if (res.data.success) {
      queryResults.value = res.data.data || [];
    }
  } catch (err: any) {
    alert(`SNMP Query failed: ${err.response?.data?.error || err.message}`);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchMibs();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">SNMP Browser & MIB Registry</h1>
      <p class="text-xs text-blue-700 dark:text-[#95CCDD]/80 mt-0.5">
        Query OIDs, execute SNMP walks, and inspect imported MIB modules in real-time.
      </p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <!-- Query Panel (Left) -->
      <div class="lg:col-span-4 p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-4 shadow-sm flex flex-col">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-1.5">
            <Radio class="w-3.5 h-3.5 text-blue-600 dark:text-[#95CCDD]" />
            <span>SNMP Query Form</span>
          </h2>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] mb-1 uppercase tracking-wider">Target Host / IP</label>
            <input
              v-model="queryForm.host"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
              placeholder="192.168.1.1"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] mb-1 uppercase tracking-wider">Port</label>
              <input
                v-model.number="queryForm.port"
                type="number"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
              />
            </div>
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] mb-1 uppercase tracking-wider">Version</label>
              <select
                v-model="queryForm.version"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
              >
                <option value="2c">v2c</option>
                <option value="v1">v1</option>
              </select>
            </div>
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] mb-1 uppercase tracking-wider">Community String</label>
            <input
              v-model="queryForm.community"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
              placeholder="public"
            />
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] mb-1 uppercase tracking-wider">OID / Subtree</label>
            <input
              v-model="queryForm.oid"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
              placeholder="1.3.6.1.2.1.1.1.0"
            />
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] mb-1 uppercase tracking-wider">Operation</label>
            <select
              v-model="queryForm.operation"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
            >
              <option value="walk">SNMP Walk</option>
              <option value="get">SNMP Get</option>
            </select>
          </div>
        </div>

        <button
          @click="executeQuery"
          :disabled="loading"
          class="mt-2 w-full py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold text-xs rounded-lg transition flex items-center justify-center gap-1.5 shadow-sm"
        >
          <Search class="w-3.5 h-3.5" />
          <span>{{ loading ? 'Querying...' : 'Execute Query' }}</span>
        </button>
      </div>

      <!-- Results Table (Right) -->
      <div class="lg:col-span-8 p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-4 shadow-sm flex flex-col min-w-0">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-2">
            <span>Results</span>
            <span class="px-2 py-0.5 rounded-full bg-blue-50 dark:bg-blue-500/10 text-blue-700 dark:text-blue-400 text-[10px] font-mono border border-blue-200 dark:border-blue-500/30 font-bold">
              {{ queryResults.length }} items
            </span>
          </h2>
        </div>

        <div class="flex-1 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-3 overflow-y-auto font-mono text-[11px] min-h-[420px]">
          <div
            v-for="(res, idx) in queryResults"
            :key="idx"
            class="p-3 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-lg shadow-sm space-y-1.5 mb-2 hover:border-blue-400 dark:hover:border-blue-500/50 transition"
          >
            <div class="flex items-center justify-between">
              <span class="text-blue-700 dark:text-brand-400 font-bold text-xs">{{ res.name || res.oid }}</span>
              <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{{ res.oid }} ({{ res.type }})</span>
            </div>
            <div class="text-slate-900 dark:text-slate-200 break-all bg-slate-50 dark:bg-[#121826] px-2.5 py-1.5 rounded border border-slate-200 dark:border-[#1b2234]/60 font-semibold">
              {{ res.value }}
            </div>
          </div>

          <div v-if="queryResults.length === 0 && !loading" class="h-64 flex flex-col items-center justify-center text-slate-500 dark:text-slate-400 text-xs gap-2">
            <Radio class="w-8 h-8 text-slate-300 dark:text-slate-600" />
            <span>Run an SNMP walk or get query to view values here</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
