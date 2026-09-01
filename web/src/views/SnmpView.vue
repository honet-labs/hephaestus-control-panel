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
  <div class="h-full flex flex-col space-y-4">
    <div>
      <h2 class="text-xl font-bold text-white tracking-tight">SNMP Browser & MIB Registry</h2>
      <p class="text-xs text-slate-400">Query OIDs, execute SNMP walks, and inspect imported MIB modules</p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 flex-1 min-h-0">
      <!-- Query Panel -->
      <div class="p-4 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3 flex flex-col shrink-0">
        <h3 class="text-xs font-bold text-white uppercase tracking-wider">SNMP Query Form</h3>
        <div class="space-y-2.5 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Target Host / IP</label>
            <input v-model="queryForm.host" class="w-full bg-slate-800 border border-slate-700 rounded px-2.5 py-1.5 text-white font-mono" />
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div>
              <label class="block text-slate-400 mb-1">Port</label>
              <input v-model.number="queryForm.port" type="number" class="w-full bg-slate-800 border border-slate-700 rounded px-2.5 py-1.5 text-white font-mono" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1">Version</label>
              <select v-model="queryForm.version" class="w-full bg-slate-800 border border-slate-700 rounded px-2.5 py-1.5 text-white">
                <option value="2c">v2c</option>
                <option value="v1">v1</option>
              </select>
            </div>
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Community String</label>
            <input v-model="queryForm.community" class="w-full bg-slate-800 border border-slate-700 rounded px-2.5 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">OID / Subtree</label>
            <input v-model="queryForm.oid" class="w-full bg-slate-800 border border-slate-700 rounded px-2.5 py-1.5 text-white font-mono" placeholder="1.3.6.1.2.1.1" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Operation</label>
            <select v-model="queryForm.operation" class="w-full bg-slate-800 border border-slate-700 rounded px-2.5 py-1.5 text-white">
              <option value="walk">SNMP Walk</option>
              <option value="get">SNMP Get</option>
            </select>
          </div>
        </div>

        <button
          @click="executeQuery"
          :disabled="loading"
          class="mt-2 w-full py-2 bg-brand-500 hover:bg-brand-600 disabled:opacity-50 text-white font-medium text-xs rounded transition flex items-center justify-center gap-1.5"
        >
          <Search class="w-3.5 h-3.5" />
          {{ loading ? 'Querying...' : 'Execute Query' }}
        </button>
      </div>

      <!-- Results Table -->
      <div class="lg:col-span-2 bg-slate-900/60 border border-slate-800 rounded-xl flex flex-col min-w-0 overflow-hidden">
        <div class="p-3 border-b border-slate-800 bg-slate-950/40 text-xs font-semibold text-white">
          Results ({{ queryResults.length }})
        </div>
        <div class="flex-1 overflow-y-auto p-2 font-mono text-[11px]">
          <div v-for="(res, idx) in queryResults" :key="idx" class="p-2 border-b border-slate-800/60 hover:bg-slate-800/40 rounded space-y-1">
            <div class="flex items-center justify-between text-slate-400">
              <span class="text-brand-400 font-bold">{{ res.name || res.oid }}</span>
              <span class="text-[10px] text-slate-500">{{ res.oid }} ({{ res.type }})</span>
            </div>
            <div class="text-slate-200 break-all">{{ res.value }}</div>
          </div>
          <div v-if="queryResults.length === 0 && !loading" class="h-64 flex items-center justify-center text-slate-500 text-xs">
            Run an SNMP walk or get to view values here
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
