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
  oid: '1.3.6.1.2.1.1',
  operation: 'walk',
  timeout: 6,
  retries: 3,
});

const queryResults = ref<any[]>([]);
const loading = ref(false);
const errorMessage = ref<string | null>(null);
const showAdvanced = ref(false);

const presets = [
  { label: 'System Subtree (Walk)', oid: '1.3.6.1.2.1.1', op: 'walk' },
  { label: 'Interfaces Table (Walk)', oid: '1.3.6.1.2.1.2', op: 'walk' },
  { label: 'IP Address Table (Walk)', oid: '1.3.6.1.2.1.4', op: 'walk' },
  { label: 'sysDescr.0 (Get)', oid: '1.3.6.1.2.1.1.1.0', op: 'get' },
  { label: 'sysUpTime.0 (Get)', oid: '1.3.6.1.2.1.1.3.0', op: 'get' },
  { label: 'sysName.0 (Get)', oid: '1.3.6.1.2.1.1.5.0', op: 'get' },
];

const applyPreset = (preset: typeof presets[0]) => {
  queryForm.value.oid = preset.oid;
  queryForm.value.operation = preset.op;
};

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
  errorMessage.value = null;
  queryResults.value = [];
  try {
    const res = await axios.post('/api/v1/snmp/query', queryForm.value);
    if (res.data.success) {
      queryResults.value = res.data.data || [];
      if (queryResults.value.length === 0) {
        errorMessage.value = 'Query returned 0 OID records. The agent may not support this OID subtree or returned an empty table.';
      }
    }
  } catch (err: any) {
    const msg = err.response?.data?.error || err.message || 'SNMP Query failed';
    errorMessage.value = msg;
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
            <div class="flex items-center justify-between mb-1">
              <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] uppercase tracking-wider">OID / Subtree</label>
              <!-- Quick preset selector -->
              <div class="relative">
                <select
                  @change="(e: any) => { const p = presets.find(x => x.oid === e.target.value); if (p) applyPreset(p); e.target.value = ''; }"
                  class="text-[10px] bg-slate-100 dark:bg-[#1a2233] text-blue-600 dark:text-blue-400 border border-slate-300 dark:border-slate-700 rounded px-1.5 py-0.5 font-medium cursor-pointer"
                >
                  <option value="" disabled selected>Presets ▾</option>
                  <option v-for="p in presets" :key="p.oid" :value="p.oid">{{ p.label }}</option>
                </select>
              </div>
            </div>
            <input
              v-model="queryForm.oid"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
              placeholder="1.3.6.1.2.1.1"
            />
            <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-1">
              Tip: Use subtree OID (e.g. <code class="text-blue-600 dark:text-blue-400">1.3.6.1.2.1.1</code>) for Walk, or scalar OID (ending in <code class="text-amber-500">.0</code>) for Get.
            </p>
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] mb-1 uppercase tracking-wider">Operation</label>
            <select
              v-model="queryForm.operation"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
            >
              <option value="walk">SNMP Walk (Subtree / Bulk)</option>
              <option value="get">SNMP Get (Single OID)</option>
            </select>
          </div>

          <!-- Advanced Options Toggle -->
          <div class="pt-1 border-t border-slate-200 dark:border-[#1b2234]">
            <button
              type="button"
              @click="showAdvanced = !showAdvanced"
              class="text-[11px] font-semibold text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 cursor-pointer"
            >
              <span>{{ showAdvanced ? '▾ Hide Network Settings' : '▸ Advanced Network Settings (Timeout & Retries)' }}</span>
            </button>

            <div v-if="showAdvanced" class="grid grid-cols-2 gap-3 mt-2">
              <div>
                <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 mb-1">Timeout (Sec)</label>
                <input
                  v-model.number="queryForm.timeout"
                  type="number"
                  min="2"
                  max="30"
                  class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1 text-xs font-mono"
                />
              </div>
              <div>
                <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 mb-1">Retries</label>
                <input
                  v-model.number="queryForm.retries"
                  type="number"
                  min="1"
                  max="5"
                  class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1 text-xs font-mono"
                />
              </div>
            </div>
          </div>
        </div>

        <button
          @click="executeQuery"
          :disabled="loading"
          class="mt-2 w-full py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold text-xs rounded-lg transition flex items-center justify-center gap-1.5 shadow-sm cursor-pointer"
        >
          <Search class="w-3.5 h-3.5" />
          <span>{{ loading ? 'Querying SNMP...' : 'Execute Query' }}</span>
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
          <button
            v-if="queryResults.length > 0"
            @click="queryResults = []; errorMessage = null"
            class="text-[11px] text-slate-500 hover:text-slate-800 dark:hover:text-white font-medium cursor-pointer"
          >
            Clear Results
          </button>
        </div>

        <!-- Diagnostic Error Banner -->
        <div
          v-if="errorMessage"
          class="p-4 rounded-xl border border-rose-300 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/20 text-rose-800 dark:text-rose-300 text-xs space-y-2 font-sans"
        >
          <div class="font-bold flex items-center gap-2 text-rose-700 dark:text-rose-400 text-sm">
            <span>⚠ SNMP Query Unsuccessful</span>
          </div>
          <p class="font-mono text-[11px] leading-relaxed break-words bg-white/60 dark:bg-black/30 p-2.5 rounded-lg border border-rose-200 dark:border-rose-900/40">
            {{ errorMessage }}
          </p>
          <div class="text-[11px] space-y-1 text-slate-700 dark:text-slate-300 pt-1">
            <div class="font-semibold text-slate-900 dark:text-white">Troubleshooting Checklist:</div>
            <ul class="list-disc list-inside space-y-0.5 text-[10.5px] opacity-90">
              <li>Check Community String: Devices silently drop UDP packets if the community string (e.g. <code class="bg-slate-200 dark:bg-slate-800 px-1 py-0.5 rounded font-mono">{{ queryForm.community }}</code>) does not match.</li>
              <li>Check UDP Port 161: Ensure incoming UDP 161 is allowed in firewall/iptables on target <code class="bg-slate-200 dark:bg-slate-800 px-1 py-0.5 rounded font-mono">{{ queryForm.host }}</code>.</li>
              <li>For scalar OIDs (ending in <code class="font-mono">.0</code>), try operation <strong>SNMP Get</strong> instead of Walk.</li>
              <li>Increase Timeout to 10s and Retries to 3 in Advanced Network Settings.</li>
            </ul>
          </div>
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
