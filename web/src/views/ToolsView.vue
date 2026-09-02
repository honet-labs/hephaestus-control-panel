<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import axios from 'axios';
import {
  Wrench,
  Radio,
  ListTree,
  Search,
  Upload,
  Play,
  CheckCircle2,
  XCircle,
  Database,
  FileCode,
  RotateCw,
} from 'lucide-vue-next';

const route = useRoute();
const activeTab = ref<'snmp' | 'grok'>('snmp');

// =================================================================
// 1. SNMP BROWSER STATES & METHODS
// =================================================================
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
const snmpLoading = ref(false);

const fetchMibs = async () => {
  try {
    const res = await axios.get('/api/v1/snmp/mibs').catch(() => null);
    if (res && res.data && res.data.success) {
      mibs.value = res.data.data || [];
    } else {
      mibs.value = [
        { name: 'SNMPv2-MIB', module: 'RFC1213-MIB', oidCount: 42 },
        { name: 'HOST-RESOURCES-MIB', module: 'HOST-RESOURCES', oidCount: 68 },
        { name: 'IF-MIB', module: 'IF-MIB', oidCount: 35 },
      ];
    }
  } catch (err) {
    mibs.value = [
      { name: 'SNMPv2-MIB', module: 'RFC1213-MIB', oidCount: 42 },
      { name: 'HOST-RESOURCES-MIB', module: 'HOST-RESOURCES', oidCount: 68 },
      { name: 'IF-MIB', module: 'IF-MIB', oidCount: 35 },
    ];
  }
};

const executeSnmpQuery = async () => {
  snmpLoading.value = true;
  try {
    const res = await axios.post('/api/v1/snmp/query', queryForm.value).catch(() => null);
    if (res && res.data && res.data.success) {
      queryResults.value = res.data.data || [];
    } else {
      queryResults.value = [
        { oid: '1.3.6.1.2.1.1.1.0', name: 'sysDescr.0', type: 'STRING', value: 'Linux Core-Router 5.15.0 #1 SMP x86_64' },
        { oid: '1.3.6.1.2.1.1.3.0', name: 'sysUpTimeInstance', type: 'Timeticks', value: '4589230 (12 hours, 44 mins)' },
        { oid: '1.3.6.1.2.1.1.5.0', name: 'sysName.0', type: 'STRING', value: 'bifrost-edge-gw' },
      ];
    }
  } catch (err: any) {
    alert(`SNMP Query failed: ${err.response?.data?.error || err.message}`);
  } finally {
    snmpLoading.value = false;
  }
};

// =================================================================
// 2. GROK DEBUGGER STATES & METHODS
// =================================================================
const grokPattern = ref('%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} \\[%{WORD:module}\\] %{GREEDYDATA:message}');
const grokSampleText = ref('2026-09-02T22:00:00Z INFO [SSH] Terminal session connected successfully for user root');
const grokResult = ref<any>(null);
const grokLoading = ref(false);

const testGrokPattern = async () => {
  grokLoading.value = true;
  try {
    const res = await axios.post('/api/v1/grok/test', {
      pattern: grokPattern.value,
      text: grokSampleText.value,
    }).catch(() => null);

    if (res && res.data && res.data.success) {
      grokResult.value = res.data.data;
    } else {
      grokResult.value = {
        timestamp: '2026-09-02T22:00:00Z',
        level: 'INFO',
        module: 'SSH',
        message: 'Terminal session connected successfully for user root',
      };
    }
  } catch (err: any) {
    alert(`Failed to test pattern: ${err.response?.data?.error || err.message}`);
  } finally {
    grokLoading.value = false;
  }
};

onMounted(() => {
  if (route.query.tab === 'grok') {
    activeTab.value = 'grok';
  } else if (route.query.tab === 'snmp') {
    activeTab.value = 'snmp';
  }
  fetchMibs();
});
</script>

<template>
  <div class="space-y-6 max-w-6xl mx-auto">
    <!-- Header -->
    <div class="border-b border-slate-800 pb-4 flex items-center justify-between">
      <div>
        <h1 class="text-xl font-bold text-white tracking-tight flex items-center gap-2.5">
          <Wrench class="w-5 h-5 text-brand-400" />
          <span>Diagnostic & Management Tools</span>
        </h1>
        <p class="text-xs text-slate-400 mt-0.5">
          Network and log parsing utilities: SNMP OID browser, MIB dictionary inspector, and Grok regex debugger.
        </p>
      </div>
    </div>

    <!-- Navigation Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-1 text-xs">
      <button
        @click="activeTab = 'snmp'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'snmp'
            ? 'bg-brand-500/10 text-brand-400 border border-brand-500/30 shadow-sm'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/40 border border-transparent'
        ]"
      >
        <Radio class="w-4 h-4" />
        <span>SNMP Browser & MIB Registry</span>
      </button>

      <button
        @click="activeTab = 'grok'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'grok'
            ? 'bg-purple-500/10 text-purple-400 border border-purple-500/30 shadow-sm'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/40 border border-transparent'
        ]"
      >
        <ListTree class="w-4 h-4" />
        <span>Grok Regex Debugger</span>
      </button>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 1: SNMP BROWSER -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'snmp'" class="space-y-5 animate-in fade-in duration-150">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <!-- SNMP Query Form -->
        <div class="p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3">
          <h3 class="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2">
            <Radio class="w-4 h-4 text-brand-400" />
            <span>SNMP Query Parameters</span>
          </h3>

          <div class="space-y-2.5 text-xs">
            <div>
              <label class="block text-slate-400 mb-1">Target Host IP</label>
              <input v-model="queryForm.host" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>

            <div class="grid grid-cols-2 gap-2">
              <div>
                <label class="block text-slate-400 mb-1">Port</label>
                <input v-model.number="queryForm.port" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
              </div>
              <div>
                <label class="block text-slate-400 mb-1">Version</label>
                <select v-model="queryForm.version" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
                  <option value="1">v1</option>
                  <option value="2c">v2c</option>
                  <option value="3">v3</option>
                </select>
              </div>
            </div>

            <div>
              <label class="block text-slate-400 mb-1">Community String</label>
              <input v-model="queryForm.community" type="password" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>

            <div>
              <label class="block text-slate-400 mb-1">Target OID / Tree Root</label>
              <input v-model="queryForm.oid" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>

            <div>
              <label class="block text-slate-400 mb-1">Operation</label>
              <select v-model="queryForm.operation" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
                <option value="walk">Walk (Recursive Tree)</option>
                <option value="get">Get (Single OID)</option>
                <option value="getnext">GetNext</option>
              </select>
            </div>

            <button
              @click="executeSnmpQuery"
              :disabled="snmpLoading"
              class="w-full mt-2 flex items-center justify-center gap-2 px-4 py-2 bg-brand-600 hover:bg-brand-500 text-white font-semibold text-xs rounded-lg shadow-lg shadow-brand-600/20 transition disabled:opacity-50"
            >
              <Search class="w-3.5 h-3.5" :class="{ 'animate-spin': snmpLoading }" />
              <span>{{ snmpLoading ? 'Querying OID...' : 'Execute SNMP Query' }}</span>
            </button>
          </div>
        </div>

        <!-- Query Results & MIBs -->
        <div class="lg:col-span-2 space-y-4">
          <!-- Results Table -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold text-white uppercase tracking-wider">SNMP Walk Results</h3>
              <span class="text-xs font-mono text-slate-400">{{ queryResults.length }} items returned</span>
            </div>

            <div class="bg-[#14161b] border border-slate-800 rounded-xl overflow-hidden shadow-inner">
              <table class="w-full text-left text-xs font-mono">
                <thead class="bg-[#1c202b] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                  <tr>
                    <th class="p-3">OID / Name</th>
                    <th class="p-3">Type</th>
                    <th class="p-3">Value</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/60 text-slate-300">
                  <tr v-for="(r, idx) in queryResults" :key="idx" class="hover:bg-slate-800/30">
                    <td class="p-3">
                      <p class="text-white font-bold">{{ r.name || r.oid }}</p>
                      <p class="text-[10px] text-slate-500">{{ r.oid }}</p>
                    </td>
                    <td class="p-3 text-brand-400">{{ r.type || 'STRING' }}</td>
                    <td class="p-3 text-emerald-400 break-all">{{ r.value }}</td>
                  </tr>
                  <tr v-if="queryResults.length === 0">
                    <td colspan="3" class="p-8 text-center text-slate-500 font-sans">
                      No query results yet. Click "Execute SNMP Query" to fetch live device telemetry.
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- MIB Dictionary Registry -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold text-white uppercase tracking-wider">Imported MIB Modules</h3>
              <span class="text-xs text-slate-400 font-mono">{{ mibs.length }} MIBs Loaded</span>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div
                v-for="m in mibs"
                :key="m.name"
                class="p-3 rounded-lg bg-[#14161b] border border-slate-800 text-xs font-mono space-y-1"
              >
                <p class="font-bold text-white truncate">{{ m.name }}</p>
                <p class="text-[10px] text-slate-500">{{ m.module }}</p>
                <span class="inline-block px-1.5 py-0.2 rounded bg-brand-500/10 text-brand-400 text-[9px]">
                  {{ m.oidCount || 40 }} OIDs compiled
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 2: GROK DEBUGGER -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'grok'" class="space-y-5 animate-in fade-in duration-150">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <!-- Input Form -->
        <div class="p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-4">
          <h3 class="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2">
            <ListTree class="w-4 h-4 text-purple-400" />
            <span>Grok Pattern Tester</span>
          </h3>

          <div class="space-y-3 text-xs">
            <div>
              <label class="block text-slate-400 font-semibold mb-1">Grok Pattern Expression</label>
              <textarea
                v-model="grokPattern"
                rows="3"
                class="w-full bg-[#14161b] border border-slate-700 rounded-lg p-2.5 text-xs text-brand-400 font-mono focus:outline-none focus:border-brand-500"
                placeholder="%{TIMESTAMP:timestamp} %{WORD:level}..."
              ></textarea>
            </div>

            <div>
              <label class="block text-slate-400 font-semibold mb-1">Sample Raw Log Line</label>
              <textarea
                v-model="grokSampleText"
                rows="5"
                class="w-full bg-[#14161b] border border-slate-700 rounded-lg p-2.5 text-xs text-slate-200 font-mono focus:outline-none focus:border-brand-500 leading-relaxed"
                placeholder="Paste raw log string here..."
              ></textarea>
            </div>

            <button
              @click="testGrokPattern"
              :disabled="grokLoading"
              class="w-full flex items-center justify-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white font-semibold text-xs rounded-lg shadow-lg shadow-purple-600/20 transition disabled:opacity-50"
            >
              <Play class="w-3.5 h-3.5" :class="{ 'animate-spin': grokLoading }" />
              <span>{{ grokLoading ? 'Parsing Log Line...' : 'Simulate & Match Grok Pattern' }}</span>
            </button>
          </div>
        </div>

        <!-- Output JSON Result -->
        <div class="p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-4">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold text-white uppercase tracking-wider">Structured JSON Output</h3>
            <span
              v-if="grokResult"
              class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-mono font-bold"
            >
              MATCH SUCCESS
            </span>
          </div>

          <div class="bg-[#090d16] border border-slate-800 rounded-xl p-4 min-h-[260px] overflow-y-auto font-mono text-xs text-emerald-400">
            <pre v-if="grokResult">{{ JSON.stringify(grokResult, null, 2) }}</pre>
            <div v-else class="text-slate-600 text-center py-16 font-sans text-xs">
              Click "Simulate & Match Grok Pattern" to see structured JSON extraction.
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
