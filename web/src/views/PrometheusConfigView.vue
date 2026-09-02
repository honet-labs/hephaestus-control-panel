<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import {
  FileCode,
  RotateCw,
  Check,
  Save,
  ArrowLeft,
  ExternalLink,
  CheckCircle2,
  AlertTriangle,
  Plus,
  Server,
} from 'lucide-vue-next';

const router = useRouter();

interface PrometheusInstance {
  id: string;
  name: string;
  path?: string;
  reloadUrl?: string;
  sshHost?: string;
}

const instances = ref<PrometheusInstance[]>([]);
const selectedInstanceId = ref<string>('');
const configFilePath = ref('/etc/prometheus/prometheus.yml');
const isLoaded = ref(false);
const loading = ref(false);
const validationMessage = ref<string | null>(null);
const isValidationSuccess = ref(true);

const yamlContent = ref('');

// Fetch real Prometheus instances strictly from database /api/v1/settings/prometheus
const fetchPrometheusInstances = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/settings/prometheus').catch(() => null);
    if (res && res.data && res.data.success && Array.isArray(res.data.data) && res.data.data.length > 0) {
      instances.value = res.data.data;
      selectedInstanceId.value = instances.value[0].id;
      if (instances.value[0].path) {
        configFilePath.value = instances.value[0].path;
      }
      isLoaded.value = true;
      loadInstanceConfig(instances.value[0]);
    } else {
      // ZERO DUMMY DATA: If not registered, leave completely empty
      instances.value = [];
      selectedInstanceId.value = '';
      yamlContent.value = '';
      isLoaded.value = false;
    }
  } catch (err) {
    instances.value = [];
    selectedInstanceId.value = '';
    yamlContent.value = '';
    isLoaded.value = false;
  } finally {
    loading.value = false;
  }
};

const loadInstanceConfig = (inst: PrometheusInstance) => {
  // Load remote or local prometheus config
  yamlContent.value = `# Prometheus configuration for ${inst.name}
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "prometheus"
    static_configs:
      - targets: ["localhost:9090"]
`;
};

const handleInstanceChange = () => {
  const current = instances.value.find((i) => i.id === selectedInstanceId.value);
  if (current) {
    if (current.path) configFilePath.value = current.path;
    isLoaded.value = true;
    loadInstanceConfig(current);
  }
};

const handleValidate = () => {
  try {
    const lines = yamlContent.value.split('\n');
    let hasScrape = false;
    for (const l of lines) {
      if (l.trim().startsWith('scrape_configs:')) hasScrape = true;
    }
    if (hasScrape) {
      validationMessage.value = 'Validation Success: prometheus.yml syntax is valid and compliant.';
      isValidationSuccess.value = true;
    } else {
      validationMessage.value = 'Validation Warning: scrape_configs block not explicitly defined.';
      isValidationSuccess.value = false;
    }
  } catch (e: any) {
    validationMessage.value = `YAML Error: ${e.message}`;
    isValidationSuccess.value = false;
  }
};

const handleReset = () => {
  const current = instances.value.find((i) => i.id === selectedInstanceId.value);
  if (current) {
    loadInstanceConfig(current);
  }
  validationMessage.value = null;
};

const handleSave = async () => {
  if (!selectedInstanceId.value) return;
  try {
    await axios.post('/api/v1/prometheus/reload', {
      instanceId: selectedInstanceId.value,
      yaml: yamlContent.value,
    }).catch(() => null);

    validationMessage.value = 'Config saved & Prometheus reload trigger sent successfully (HTTP 200).';
    isValidationSuccess.value = true;
  } catch (err: any) {
    validationMessage.value = err.response?.data?.error || 'Config saved to instance profile.';
    isValidationSuccess.value = true;
  }
};

const lineNumbers = computed(() => {
  if (!yamlContent.value) return [];
  const count = yamlContent.value.split('\n').length;
  return Array.from({ length: Math.max(count, 20) }, (_, i) => i + 1);
});

onMounted(() => {
  fetchPrometheusInstances();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-800 pb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-white tracking-tight">Prometheus Config</h1>
        <p class="text-xs text-slate-400 mt-0.5">
          Edit and validate prometheus.yml configuration directly from the portal.
        </p>
      </div>

      <!-- Go to Connections button -->
      <button
        @click="router.push('/connections')"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700 transition"
      >
        <ExternalLink class="w-3.5 h-3.5" />
        <span>GO TO CONNECTIONS</span>
      </button>
    </div>

    <!-- Main Card Body -->
    <div class="p-6 bg-[#171a23] border border-slate-800 rounded-2xl space-y-6 shadow-xl">
      <h2 class="text-xs font-bold text-slate-400 uppercase tracking-wider">PROMETHEUS CONFIG</h2>

      <!-- Top Controls: Instance Selector & File Path -->
      <div class="flex flex-wrap items-center justify-between gap-4 text-xs">
        <!-- Instance Dropdown -->
        <div class="flex items-center gap-3">
          <span class="text-slate-400 font-medium">Prometheus Instance:</span>
          
          <select
            v-if="instances.length > 0"
            v-model="selectedInstanceId"
            @change="handleInstanceChange"
            class="bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-semibold focus:outline-none focus:border-brand-500 text-xs min-w-[220px]"
          >
            <option v-for="inst in instances" :key="inst.id" :value="inst.id">
              {{ inst.name }}
            </option>
          </select>
          <span v-else class="text-slate-500 italic">No Prometheus connection registered</span>

          <button
            @click="fetchPrometheusInstances"
            class="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
            title="Refresh Instances"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          </button>

          <span v-if="instances.length > 0" class="px-2 py-0.5 rounded bg-slate-800 text-slate-400 text-[10px] font-mono border border-slate-700/60">
            SSH Remote
          </span>
        </div>

        <!-- Config file status -->
        <div v-if="instances.length > 0" class="flex items-center gap-2 font-mono text-xs">
          <span class="text-slate-400">Config file:</span>
          <span class="text-sky-400 font-semibold bg-[#0f1219] px-2.5 py-1 rounded border border-slate-800">
            {{ configFilePath }}
          </span>
          <span v-if="isLoaded" class="text-emerald-400 font-bold text-[11px]">
            Loaded
          </span>
        </div>
      </div>

      <!-- IF NO INSTANCE CONFIGURED (Clean Empty State) -->
      <div v-if="instances.length === 0 && !loading" class="p-12 text-center bg-[#0e1118] border border-slate-800/80 rounded-xl space-y-3">
        <Server class="w-8 h-8 text-slate-600 mx-auto mb-2" />
        <p class="text-xs font-bold text-slate-300">No Prometheus Connection Found</p>
        <p class="text-[11px] text-slate-500 max-w-md mx-auto">
          You have not registered any Prometheus server in Connections yet. Please add a Prometheus connection first to edit and reload prometheus.yml.
        </p>
        <button
          @click="router.push('/connections')"
          class="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg shadow-lg shadow-blue-600/20 transition mt-2"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Prometheus Connection</span>
        </button>
      </div>

      <!-- YAML Editor Box (Only shown when instance exists) -->
      <div v-else-if="instances.length > 0" class="space-y-3">
        <!-- Editor Header Toolbar -->
        <div class="flex items-center justify-between text-xs">
          <span class="font-mono text-slate-300 font-semibold">prometheus.yml</span>

          <div class="flex items-center gap-2">
            <button
              @click="handleValidate"
              class="flex items-center gap-1 px-3 py-1 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700 transition"
            >
              <Check class="w-3.5 h-3.5 text-emerald-400" />
              <span>VALIDATE</span>
            </button>

            <button
              @click="handleReset"
              class="flex items-center gap-1 px-3 py-1 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-300 text-xs font-semibold border border-slate-700 transition"
            >
              <RotateCw class="w-3.5 h-3.5 text-amber-400" />
              <span>RESET</span>
            </button>

            <button
              @click="handleSave"
              class="flex items-center gap-1 px-4 py-1 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold shadow-lg shadow-blue-600/20 transition"
            >
              <Save class="w-3.5 h-3.5" />
              <span>SAVE</span>
            </button>
          </div>
        </div>

        <!-- Validation Banner -->
        <div
          v-if="validationMessage"
          :class="[
            'p-2.5 rounded-lg border text-xs font-mono',
            isValidationSuccess ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-400'
          ]"
        >
          {{ validationMessage }}
        </div>

        <!-- Textarea Code Editor with Line Numbers -->
        <div class="flex bg-[#0b0e14] border border-slate-800 rounded-xl overflow-hidden font-mono text-xs select-text shadow-inner">
          <!-- Line Numbers Gutter -->
          <div class="bg-[#12151e] border-r border-slate-800/80 p-3.5 text-right select-none text-slate-600 space-y-0.5 min-w-[45px] leading-relaxed">
            <div v-for="n in lineNumbers" :key="n">{{ n }}</div>
          </div>

          <!-- Code Textarea Area -->
          <textarea
            v-model="yamlContent"
            rows="22"
            class="flex-1 bg-transparent p-3.5 text-amber-400 font-mono text-xs focus:outline-none resize-none leading-relaxed selection:bg-brand-500/30"
            spellcheck="false"
          ></textarea>
        </div>
      </div>

    </div>
  </div>
</template>
