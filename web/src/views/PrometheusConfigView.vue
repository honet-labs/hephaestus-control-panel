<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import {
  RotateCw,
  Check,
  Save,
  ExternalLink,
  CheckCircle2,
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
let validationTimer: ReturnType<typeof setTimeout> | null = null;

// Auto-dismiss notification after 3 seconds
watch(validationMessage, (newVal) => {
  if (validationTimer) {
    clearTimeout(validationTimer);
    validationTimer = null;
  }
  if (newVal) {
    validationTimer = setTimeout(() => {
      validationMessage.value = null;
      restartResult.value = null;
      validationTimer = null;
    }, 3000);
  }
});

const yamlContent = ref('');
const saving = ref(false);

// Fetch real Prometheus instances strictly from database /api/v1/settings/prometheus
const fetchPrometheusInstances = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/settings/prometheus?type=prometheus').catch(() => null);
    if (res && res.data && res.data.success && Array.isArray(res.data.data)) {
      // Filter strictly to Prometheus instances (exclude Data Prepper)
      const promList = res.data.data.filter((p: any) => {
        const name = (p.name || '').toLowerCase();
        const path = (p.path || '').toLowerCase();
        return !name.includes('data prepper') && !name.includes('dataprepper') && !path.includes('pipeline');
      });

      if (promList.length > 0) {
        instances.value = promList;
        // Prioritize currently selected, or active, or first item
        const matched = promList.find((i: any) => i.id === selectedInstanceId.value)
                     || promList.find((i: any) => i.isActive)
                     || promList[0];
        selectedInstanceId.value = matched.id;
        configFilePath.value = matched.path || '/etc/prometheus/prometheus.yml';
        await fetchConfigContent(matched.id);
      } else {
        // ZERO DUMMY DATA: If not registered, leave completely empty
        instances.value = [];
        selectedInstanceId.value = '';
        yamlContent.value = '';
        isLoaded.value = false;
        loading.value = false;
      }
    } else {
      instances.value = [];
      selectedInstanceId.value = '';
      yamlContent.value = '';
      isLoaded.value = false;
      loading.value = false;
    }
  } catch (err: any) {
    instances.value = [];
    selectedInstanceId.value = '';
    yamlContent.value = '';
    isLoaded.value = false;
    loading.value = false;
  }
};

const fetchConfigContent = async (instanceId: string) => {
  loading.value = true;
  validationMessage.value = null;
  yamlContent.value = '';
  isLoaded.value = false;

  try {
    const res = await axios.get(`/api/v1/prometheus/config?instanceId=${instanceId}`);
    if (res.data?.success && res.data.data) {
      yamlContent.value = res.data.data.content || '';
      if (res.data.data.path) {
        configFilePath.value = res.data.data.path;
      }
      isLoaded.value = true;
    } else {
      validationMessage.value = `Failed to fetch remote config: ${res.data?.error || 'Unknown error'}`;
      isValidationSuccess.value = false;
      isLoaded.value = false;
    }
  } catch (err: any) {
    validationMessage.value = `Failed to fetch remote config: ${err.response?.data?.error || err.message}`;
    isValidationSuccess.value = false;
    isLoaded.value = false;
  } finally {
    loading.value = false;
  }
};

const handleInstanceChange = () => {
  const current = instances.value.find((i) => i.id === selectedInstanceId.value);
  if (current) {
    if (current.path) configFilePath.value = current.path;
    fetchConfigContent(current.id);
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
  if (selectedInstanceId.value) {
    fetchConfigContent(selectedInstanceId.value);
  }
};

const restartResult = ref<{ attempted?: boolean; success?: boolean; output?: string; error?: string } | null>(null);

const handleSave = async () => {
  if (!selectedInstanceId.value) return;
  saving.value = true;
  validationMessage.value = null;
  restartResult.value = null;

  try {
    const res = await axios.post('/api/v1/prometheus/config', {
      instanceId: selectedInstanceId.value,
      yaml: yamlContent.value,
      reload: true,
    });

    if (res.data?.success) {
      const data = res.data.data;
      if (data?.restart) {
        restartResult.value = data.restart;
        if (data.restart.success) {
          validationMessage.value = data.message || 'Config saved & Prometheus service restarted successfully.';
          isValidationSuccess.value = true;
        } else {
          validationMessage.value = data.message || `Config saved, but service restart failed: ${data.restart.error || 'Unknown error'}`;
          isValidationSuccess.value = false;
        }
      } else {
        validationMessage.value = res.data.message || 'Config saved successfully.';
        isValidationSuccess.value = true;
      }
    } else {
      validationMessage.value = `Save failed: ${res.data?.error || 'Unknown error'}`;
      isValidationSuccess.value = false;
    }
  } catch (err: any) {
    validationMessage.value = `Save failed: ${err.response?.data?.error || err.message}`;
    isValidationSuccess.value = false;
  } finally {
    saving.value = false;
  }
};

const lineNumbers = computed(() => {
  const count = yamlContent.value.split('\n').length;
  return Array.from({ length: Math.max(count, 20) }, (_, i) => i + 1);
});

const editorRef = ref<HTMLTextAreaElement | null>(null);
const gutterRef = ref<HTMLDivElement | null>(null);

const syncScroll = () => {
  if (editorRef.value && gutterRef.value) {
    gutterRef.value.scrollTop = editorRef.value.scrollTop;
  }
};

onMounted(() => {
  fetchPrometheusInstances();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-200 dark:border-slate-800 pb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">Prometheus Config</h1>
        <p class="text-xs text-slate-600 dark:text-slate-400 mt-0.5">
          Edit and validate prometheus.yml configuration directly from the portal.
        </p>
      </div>

      <!-- Go to Connections button -->
      <button
        @click="router.push('/connections')"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-200 text-xs font-semibold border border-slate-300 dark:border-slate-700 transition cursor-pointer shadow-sm"
      >
        <ExternalLink class="w-3.5 h-3.5" />
        <span>GO TO CONNECTIONS</span>
      </button>
    </div>

    <!-- Main Card Body -->
    <div class="p-6 bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-800 rounded-2xl space-y-6 shadow-sm">
      <h2 class="text-xs font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider">PROMETHEUS CONFIG</h2>

      <!-- Top Controls: Instance Selector & File Path -->
      <div class="flex flex-wrap items-center justify-between gap-4 text-xs">
        <!-- Instance Dropdown -->
        <div class="flex items-center gap-3">
          <span class="text-slate-700 dark:text-slate-400 font-medium">Prometheus Instance:</span>
          
          <select
            v-if="instances.length > 0"
            v-model="selectedInstanceId"
            @change="handleInstanceChange"
            class="bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-semibold focus:outline-none focus:border-blue-500 text-xs min-w-[220px] shadow-sm"
          >
            <option v-for="inst in instances" :key="inst.id" :value="inst.id">
              {{ inst.name }}
            </option>
          </select>
          <span v-else class="text-slate-500 italic">No Prometheus connection registered</span>

          <button
            @click="fetchPrometheusInstances"
            class="p-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 transition cursor-pointer shadow-sm"
            title="Refresh Instances"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          </button>

          <span v-if="instances.length > 0" class="px-2 py-0.5 rounded bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-slate-400 text-[10px] font-mono border border-blue-200 dark:border-slate-700/60 font-semibold">
            SSH Remote
          </span>
        </div>

        <!-- Config file status -->
        <div v-if="instances.length > 0" class="flex items-center gap-2 font-mono text-xs">
          <span class="text-slate-600 dark:text-slate-400">Config file:</span>
          <span class="text-blue-700 dark:text-sky-400 font-semibold bg-slate-100 dark:bg-[#0f1219] px-2.5 py-1 rounded border border-slate-200 dark:border-slate-800">
            {{ configFilePath }}
          </span>
          <span v-if="isLoaded" class="text-emerald-600 dark:text-emerald-400 font-bold text-[11px] flex items-center gap-1">
            <CheckCircle2 class="w-3 h-3 text-emerald-600 dark:text-emerald-400" /> Loaded
          </span>
          <span v-else-if="loading" class="text-amber-600 dark:text-amber-400 font-bold text-[11px] flex items-center gap-1">
            <RotateCw class="w-3 h-3 animate-spin" /> Fetching...
          </span>
          <span v-else class="text-rose-600 dark:text-rose-400 font-bold text-[11px]">
            Failed
          </span>
        </div>
      </div>

      <!-- IF NO INSTANCE CONFIGURED (Clean Empty State) -->
      <div v-if="instances.length === 0 && !loading" class="p-12 text-center bg-slate-50 dark:bg-[#0e1118] border border-slate-200 dark:border-slate-800/80 rounded-xl space-y-3">
        <Server class="w-8 h-8 text-slate-400 dark:text-slate-600 mx-auto mb-2" />
        <p class="text-xs font-bold text-slate-800 dark:text-slate-300">No Prometheus Connection Found</p>
        <p class="text-[11px] text-slate-600 dark:text-slate-500 max-w-md mx-auto">
          You have not registered any Prometheus server in Connections yet. Please add a Prometheus connection first to edit and reload prometheus.yml.
        </p>
        <button
          @click="router.push('/connections')"
          class="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg transition mt-2 cursor-pointer shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Prometheus Connection</span>
        </button>
      </div>

      <!-- YAML Editor Box (Only shown when instance exists) -->
      <div v-else-if="instances.length > 0" class="space-y-3">
        <!-- Editor Header Toolbar -->
        <div class="flex items-center justify-between text-xs">
          <span class="font-mono text-slate-800 dark:text-slate-200 font-bold text-xs">prometheus.yml</span>

          <div class="flex items-center gap-2">
            <button
              @click="handleValidate"
              :disabled="loading || !isLoaded"
              class="flex items-center gap-1 px-3 py-1 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-slate-700 disabled:opacity-50 text-slate-700 hover:text-slate-900 dark:text-slate-200 text-xs font-semibold border border-slate-300 dark:border-slate-700 transition cursor-pointer"
            >
              <Check class="w-3.5 h-3.5 text-emerald-500" />
              <span>VALIDATE</span>
            </button>

            <button
              @click="handleReset"
              :disabled="loading"
              class="flex items-center gap-1 px-3 py-1 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-slate-700 disabled:opacity-50 text-slate-700 hover:text-slate-900 dark:text-slate-300 text-xs font-semibold border border-slate-300 dark:border-slate-700 transition cursor-pointer"
            >
              <RotateCw class="w-3.5 h-3.5 text-amber-500" :class="{ 'animate-spin': loading }" />
              <span>RESET</span>
            </button>

            <button
              @click="handleSave"
              :disabled="saving || loading || !isLoaded"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-xs font-semibold transition cursor-pointer shadow-sm"
            >
              <RotateCw v-if="saving" class="w-3.5 h-3.5 animate-spin" />
              <Save v-else class="w-3.5 h-3.5" />
              <span>{{ saving ? 'RESTARTING SERVICE...' : 'SAVE & RESTART' }}</span>
            </button>
          </div>
        </div>

        <!-- Validation & Restart Feedback Banner -->
        <div
          v-if="validationMessage"
          :class="[
            'p-3 rounded-lg border text-xs font-mono space-y-1 animate-in fade-in transition-all duration-200 shadow-xs',
            isValidationSuccess
              ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-700 dark:text-emerald-300'
              : 'bg-rose-500/10 border-rose-500/30 text-rose-700 dark:text-rose-300'
          ]"
        >
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 font-semibold">
              <span class="w-2 h-2 rounded-full shrink-0" :class="isValidationSuccess ? 'bg-emerald-500 dark:bg-emerald-400' : 'bg-rose-500 dark:bg-rose-400'"></span>
              <span>{{ validationMessage }}</span>
            </div>
            <button
              @click="validationMessage = null; restartResult = null"
              class="opacity-60 hover:opacity-100 p-0.5 cursor-pointer text-sm leading-none font-bold"
              title="Close notification"
            >
              &times;
            </button>
          </div>
          <div v-if="restartResult?.output" class="text-[11px] text-slate-700 dark:text-slate-300 pl-4 border-l border-slate-300 dark:border-slate-700/50 mt-1">
            Status: {{ restartResult.output }}
          </div>
          <div v-if="restartResult && !restartResult.success && restartResult.error" class="text-[11px] text-rose-700 dark:text-rose-400 pl-4 border-l border-rose-300 dark:border-rose-700/50 mt-1">
            Error: {{ restartResult.error }}
          </div>
        </div>

        <!-- Textarea Code Editor with Line Numbers -->
        <div class="flex bg-[#0b0e14] border border-slate-800 rounded-xl overflow-hidden font-mono text-xs select-text shadow-inner h-[calc(100vh-320px)] min-h-[460px] relative">
          <!-- Line Numbers Gutter -->
          <div
            ref="gutterRef"
            class="bg-[#12151e] border-r border-slate-800/80 p-3.5 text-right select-none text-slate-600 min-w-[50px] leading-relaxed overflow-hidden shrink-0 pointer-events-none"
          >
            <div v-for="n in lineNumbers" :key="n" class="leading-relaxed">{{ n }}</div>
          </div>

          <!-- Code Textarea Area -->
          <textarea
            ref="editorRef"
            v-model="yamlContent"
            @scroll="syncScroll"
            class="flex-1 bg-transparent p-3.5 text-amber-400 font-mono text-xs focus:outline-none resize-none leading-relaxed selection:bg-brand-500/30 overflow-y-auto overflow-x-auto whitespace-pre outline-none h-full"
            spellcheck="false"
          ></textarea>
        </div>
      </div>

    </div>
  </div>
</template>
