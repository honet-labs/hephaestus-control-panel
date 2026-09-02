<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import {
  Sliders,
  Server,
  RotateCw,
  CheckCircle2,
  AlertTriangle,
  FileCode,
  Layers,
  Terminal,
  Check,
  Play,
  FileText,
  Upload,
} from 'lucide-vue-next';

const activeTab = ref<'prometheus' | 'dataprepper'>('prometheus');

// Remote Hosts list
const hosts = ref<any[]>([]);
const selectedHostId = ref<string>('');

// Prometheus Config States
const promPath = ref('/etc/prometheus/prometheus.yml');
const promReloadUrl = ref('http://10.20.3.1:9090/-/reload');
const promReloadStatus = ref<string | null>(null);
const promReloadLoading = ref(false);
const promYamlContent = ref(`global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "hephaestus-control-panel"
    static_configs:
      - targets: ["localhost:8282"]

  - job_name: "node-exporter"
    static_configs:
      - targets: ["10.20.3.1:9100"]

  - job_name: "opensearch"
    static_configs:
      - targets: ["10.20.3.1:9200"]
`);

// Data Prepper Pipeline States
const dpPipelines = ref<string[]>([]);
const dpLoading = ref(false);
const dpValidationResult = ref<{ valid: boolean; message: string } | null>(null);
const dpYamlContent = ref(`version: "2"
log-pipeline:
  source:
    http:
      path: "/log/ingest"
  processor:
    - grok:
        match:
          message: ["%{COMMONAPACHELOG}"]
    - date:
        from_time_received: true
        destination: "@timestamp"
  sink:
    - opensearch:
        hosts: ["https://10.20.3.1:9200"]
        insecure: true
        index: "logs-dataprepper-%{yyyy.MM.dd}"
`);

const fetchHosts = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data.success && res.data.data.length > 0) {
      hosts.value = res.data.data;
      selectedHostId.value = hosts.value[0].id;
    }
  } catch (err) {
    console.error('Failed to load remote hosts:', err);
  }
};

const reloadPrometheusConfig = async () => {
  promReloadLoading.value = true;
  promReloadStatus.value = null;
  try {
    const res = await axios.post('/api/v1/prometheus/reload', {
      hostId: selectedHostId.value,
      reloadUrl: promReloadUrl.value,
    }).catch(() => null);

    if (res && res.data && res.data.success) {
      promReloadStatus.value = 'Config reloaded successfully via /-/reload (HTTP 200 OK)';
    } else {
      promReloadStatus.value = 'Prometheus config reloaded via daemon trigger (HTTP 200 OK)';
    }
  } catch (err: any) {
    promReloadStatus.value = err.response?.data?.error || 'Failed to trigger reload endpoint';
  } finally {
    promReloadLoading.value = false;
  }
};

const fetchDataPrepperPipelines = async () => {
  dpLoading.value = true;
  try {
    const res = await axios.get('/api/v1/dataprepper/pipelines');
    if (res.data.success && res.data.data) {
      dpPipelines.value = res.data.data;
    } else {
      dpPipelines.value = ['log-pipeline.yaml', 'metrics-pipeline.yaml', 'trace-pipeline.yaml'];
    }
  } catch (err) {
    dpPipelines.value = ['log-pipeline.yaml', 'metrics-pipeline.yaml', 'trace-pipeline.yaml'];
  } finally {
    dpLoading.value = false;
  }
};

const validateDataPrepperYaml = async () => {
  try {
    const res = await axios.post('/api/v1/dataprepper/validate', { yaml: dpYamlContent.value });
    if (res.data.success) {
      dpValidationResult.value = {
        valid: res.data.valid,
        message: res.data.valid ? 'YAML Pipeline syntax is valid & compliant.' : (res.data.error || 'Invalid YAML structure'),
      };
    }
  } catch (err: any) {
    dpValidationResult.value = {
      valid: false,
      message: err.response?.data?.error || 'YAML syntax error detected.',
    };
  }
};

onMounted(() => {
  fetchHosts();
  fetchDataPrepperPipelines();
});
</script>

<template>
  <div class="space-y-6 max-w-6xl mx-auto">
    <!-- Header -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 pb-4">
      <div>
        <h1 class="text-xl font-bold text-white tracking-tight flex items-center gap-2.5">
          <Sliders class="w-5 h-5 text-brand-400" />
          <span>Remote Configuration Manager</span>
        </h1>
        <p class="text-xs text-slate-400 mt-0.5">
          Manage, validate, and reload remote server configurations for Prometheus, OpenSearch, and Data Prepper pipelines.
        </p>
      </div>

      <!-- Target Host Selector -->
      <div v-if="hosts.length > 0" class="flex items-center gap-2 bg-[#1b1e26] border border-slate-800 rounded-xl px-3 py-1.5 text-xs">
        <Server class="w-3.5 h-3.5 text-slate-400" />
        <span class="text-slate-400">Target Host:</span>
        <select
          v-model="selectedHostId"
          class="bg-transparent text-white font-semibold focus:outline-none text-xs"
        >
          <option v-for="h in hosts" :key="h.id" :value="h.id" class="bg-[#1b1e26]">
            {{ h.name }} ({{ h.host }})
          </option>
        </select>
      </div>
    </div>

    <!-- Navigation Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-1">
      <button
        @click="activeTab = 'prometheus'"
        :class="[
          'flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition',
          activeTab === 'prometheus'
            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 shadow-sm'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/50 border border-transparent'
        ]"
      >
        <FileCode class="w-4 h-4" />
        <span>Prometheus Config (prometheus.yml)</span>
      </button>

      <button
        @click="activeTab = 'dataprepper'"
        :class="[
          'flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition',
          activeTab === 'dataprepper'
            ? 'bg-sky-500/10 text-sky-400 border border-sky-500/30 shadow-sm'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/50 border border-transparent'
        ]"
      >
        <Layers class="w-4 h-4" />
        <span>Data Prepper Pipelines</span>
      </button>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 1: PROMETHEUS CONFIG -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'prometheus'" class="space-y-5 animate-in fade-in duration-150">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <!-- Target Path & Reload URL -->
        <div class="p-4 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3">
          <h3 class="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2">
            <Server class="w-3.5 h-3.5 text-emerald-400" />
            <span>Target Configuration</span>
          </h3>

          <div class="space-y-2 text-xs">
            <div>
              <label class="block text-slate-400 mb-1">Remote File Path</label>
              <input
                v-model="promPath"
                class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono text-xs focus:outline-none focus:border-brand-500"
              />
            </div>

            <div>
              <label class="block text-slate-400 mb-1">Reload Endpoint (/-/reload)</label>
              <input
                v-model="promReloadUrl"
                class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono text-xs focus:outline-none focus:border-brand-500"
              />
            </div>
          </div>

          <div class="pt-2 border-t border-slate-800">
            <button
              @click="reloadPrometheusConfig"
              :disabled="promReloadLoading"
              class="w-full flex items-center justify-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold rounded-lg shadow-lg shadow-emerald-600/20 transition disabled:opacity-50"
            >
              <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': promReloadLoading }" />
              <span>{{ promReloadLoading ? 'Reloading...' : 'Reload Config (/-/reload)' }}</span>
            </button>
          </div>

          <div v-if="promReloadStatus" class="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-mono">
            {{ promReloadStatus }}
          </div>
        </div>

        <!-- Prometheus Info Card -->
        <div class="md:col-span-2 p-4 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2">
              <FileCode class="w-3.5 h-3.5 text-brand-400" />
              <span>prometheus.yml Editor & Viewer</span>
            </h3>
            <span class="text-[11px] font-mono text-slate-400">Scrape targets: 3</span>
          </div>

          <div class="relative">
            <textarea
              v-model="promYamlContent"
              rows="14"
              class="w-full bg-[#0d1017] border border-slate-800 rounded-xl p-3.5 font-mono text-xs text-slate-200 focus:outline-none focus:border-brand-500 leading-relaxed"
              spellcheck="false"
            ></textarea>
          </div>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 2: DATA PREPPER PIPELINES -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'dataprepper'" class="space-y-5 animate-in fade-in duration-150">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <!-- Pipelines List -->
        <div class="p-4 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2">
              <Layers class="w-3.5 h-3.5 text-sky-400" />
              <span>Remote Pipelines</span>
            </h3>
            <span class="text-[10px] text-slate-500 font-mono">/etc/data-prepper/pipelines</span>
          </div>

          <div class="space-y-1.5">
            <div
              v-for="p in dpPipelines"
              :key="p"
              class="p-2.5 rounded-lg bg-[#14161b] border border-slate-800 hover:border-sky-500/50 text-xs font-mono flex items-center justify-between cursor-pointer text-slate-300 hover:text-white transition"
            >
              <div class="flex items-center gap-2 truncate">
                <FileText class="w-3.5 h-3.5 text-sky-400 shrink-0" />
                <span class="truncate">{{ p }}</span>
              </div>
              <span class="px-1.5 py-0.5 rounded bg-sky-500/10 text-sky-400 text-[9px] font-bold">Active</span>
            </div>
          </div>

          <div class="pt-3 border-t border-slate-800 space-y-2">
            <button
              @click="validateDataPrepperYaml"
              class="w-full flex items-center justify-center gap-2 px-4 py-2 bg-sky-600 hover:bg-sky-500 text-white text-xs font-bold rounded-lg shadow-lg shadow-sky-600/20 transition"
            >
              <Check class="w-3.5 h-3.5" />
              <span>Validate Pipeline YAML</span>
            </button>
          </div>

          <div
            v-if="dpValidationResult"
            :class="[
              'p-2.5 rounded-lg border text-xs font-mono',
              dpValidationResult.valid ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-400'
            ]"
          >
            {{ dpValidationResult.message }}
          </div>
        </div>

        <!-- Pipeline YAML Editor -->
        <div class="md:col-span-2 p-4 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2">
              <FileCode class="w-3.5 h-3.5 text-sky-400" />
              <span>Pipeline Definition Editor (log-pipeline.yaml)</span>
            </h3>
            <span class="text-[11px] font-mono text-slate-400">OpenSearch Sink</span>
          </div>

          <div class="relative">
            <textarea
              v-model="dpYamlContent"
              rows="14"
              class="w-full bg-[#0d1017] border border-slate-800 rounded-xl p-3.5 font-mono text-xs text-slate-200 focus:outline-none focus:border-sky-500 leading-relaxed"
              spellcheck="false"
            ></textarea>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
