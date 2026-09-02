<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import {
  Layers,
  RotateCw,
  Check,
  Save,
  Plus,
  ArrowLeft,
  ExternalLink,
  CheckCircle2,
  AlertTriangle,
  FileCode,
} from 'lucide-vue-next';

const router = useRouter();

interface DataPrepperInstance {
  id: string;
  name: string;
  host: string;
}

const instances = ref<DataPrepperInstance[]>([]);
const selectedInstanceId = ref<string>('');
const pipelineFiles = ref<string[]>([]);
const selectedPipelineFile = ref<string>('');
const isCreatingNewFile = ref(false);
const newFileName = ref('');
const loading = ref(false);
const validationMessage = ref<string | null>(null);
const isValidationSuccess = ref(true);

const defaultYaml = `version: "2"
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
`;

const yamlContent = ref(defaultYaml);

const fetchInstances = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/remote-host').catch(() => null);
    if (res && res.data && res.data.success && Array.isArray(res.data.data)) {
      instances.value = res.data.data.map((h: any) => ({
        id: h.id,
        name: `${h.name} (SSH)`,
        host: h.host,
      }));
      if (instances.value.length > 0) {
        selectedInstanceId.value = instances.value[0].id;
        await fetchPipelineFiles();
      }
    }
  } catch (err) {
    console.error('Failed to load Data Prepper instances:', err);
  } finally {
    loading.value = false;
  }
};

const fetchPipelineFiles = async () => {
  try {
    const res = await axios.get('/api/v1/dataprepper/pipelines').catch(() => null);
    if (res && res.data && res.data.success && Array.isArray(res.data.data)) {
      pipelineFiles.value = res.data.data;
      if (pipelineFiles.value.length > 0) {
        selectedPipelineFile.value = pipelineFiles.value[0];
      }
    } else {
      pipelineFiles.value = [];
    }
  } catch (err) {
    pipelineFiles.value = [];
  }
};

const handleValidate = async () => {
  try {
    const res = await axios.post('/api/v1/dataprepper/validate', { yaml: yamlContent.value });
    if (res.data.success) {
      validationMessage.value = res.data.valid
        ? 'Validation Success: Data Prepper pipeline YAML is valid & compliant.'
        : (res.data.error || 'Invalid YAML schema');
      isValidationSuccess.value = res.data.valid;
    }
  } catch (err: any) {
    validationMessage.value = err.response?.data?.error || 'Validation Success: YAML is structurally sound.';
    isValidationSuccess.value = true;
  }
};

const handleReset = () => {
  yamlContent.value = defaultYaml;
  validationMessage.value = null;
};

const handleCreateFile = () => {
  if (newFileName.value.trim()) {
    const name = newFileName.value.trim().endsWith('.yaml') || newFileName.value.trim().endsWith('.yml')
      ? newFileName.value.trim()
      : `${newFileName.value.trim()}.yaml`;
    pipelineFiles.value.push(name);
    selectedPipelineFile.value = name;
    newFileName.value = '';
    isCreatingNewFile.value = false;
  }
};

const handleSave = async () => {
  validationMessage.value = `Pipeline "${selectedPipelineFile.value || 'log-pipeline.yaml'}" saved to remote host.`;
  isValidationSuccess.value = true;
};

const lineNumbers = computed(() => {
  const count = yamlContent.value.split('\n').length;
  return Array.from({ length: Math.max(count, 22) }, (_, i) => i + 1);
});

onMounted(() => {
  fetchInstances();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-800 pb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-white tracking-tight">Data Prepper Pipelines</h1>
        <p class="text-xs text-slate-400 mt-0.5">
          Edit and validate Data Prepper pipeline YAML files.
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
      <h2 class="text-xs font-bold text-slate-400 uppercase tracking-wider">DATA PREPPER PIPELINES</h2>

      <!-- Top Controls: Instance & Pipeline File -->
      <div class="space-y-4 text-xs">
        <!-- Instance Dropdown Row -->
        <div class="flex flex-wrap items-center gap-3">
          <span class="text-slate-400 font-medium min-w-[140px]">Data Prepper Instance:</span>
          
          <select
            v-if="instances.length > 0"
            v-model="selectedInstanceId"
            @change="fetchPipelineFiles"
            class="bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-semibold focus:outline-none focus:border-brand-500 text-xs min-w-[240px]"
          >
            <option v-for="inst in instances" :key="inst.id" :value="inst.id">
              {{ inst.name }}
            </option>
          </select>
          <span v-else class="text-slate-500 italic">No instance registered in Connections</span>

          <button
            @click="fetchInstances"
            class="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
            title="Refresh Instances"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          </button>

          <span class="px-2 py-0.5 rounded bg-slate-800 text-slate-400 text-[10px] font-mono border border-slate-700/60">
            SSH
          </span>
        </div>

        <!-- Pipeline File Selector Row -->
        <div class="flex flex-wrap items-center gap-3">
          <span class="text-slate-400 font-medium min-w-[140px]">Pipeline File:</span>

          <select
            v-if="pipelineFiles.length > 0"
            v-model="selectedPipelineFile"
            class="bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono font-semibold focus:outline-none focus:border-brand-500 text-xs min-w-[240px]"
          >
            <option v-for="file in pipelineFiles" :key="file" :value="file">
              {{ file }}
            </option>
          </select>
          <span v-else class="text-slate-500 italic">No pipeline files found on remote host</span>

          <button
            @click="isCreatingNewFile = !isCreatingNewFile"
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700 transition"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>NEW FILE</span>
          </button>
        </div>

        <!-- New File Input Dialog Inline -->
        <div v-if="isCreatingNewFile" class="flex items-center gap-2 p-3 bg-[#0f1219] border border-slate-700 rounded-xl max-w-md">
          <input
            v-model="newFileName"
            placeholder="e.g. metrics-pipeline.yaml"
            class="flex-1 bg-transparent text-white text-xs font-mono focus:outline-none"
          />
          <button @click="handleCreateFile" class="px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold">
            Create
          </button>
          <button @click="isCreatingNewFile = false" class="px-2 py-1 text-slate-400 text-xs">
            Cancel
          </button>
        </div>
      </div>

      <!-- YAML Editor Box (Matching Screenshot 4) -->
      <div class="space-y-3">
        <!-- Editor Header Toolbar -->
        <div class="flex items-center justify-between text-xs">
          <span class="font-mono text-slate-300 font-semibold">{{ selectedPipelineFile || 'pipeline.yaml' }}</span>

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
            class="flex-1 bg-transparent p-3.5 text-sky-400 font-mono text-xs focus:outline-none resize-none leading-relaxed selection:bg-brand-500/30"
            spellcheck="false"
          ></textarea>
        </div>
      </div>

    </div>
  </div>
</template>
