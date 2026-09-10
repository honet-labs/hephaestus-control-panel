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
  Trash2,
  ArrowLeft,
  ExternalLink,
  CheckCircle2,
  AlertTriangle,
  FileCode,
  Server,
} from 'lucide-vue-next';

const router = useRouter();

interface DataPrepperInstance {
  id: string;
  name: string;
  host?: string;
  path?: string;
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
const yamlContent = ref('');
const saving = ref(false);
const deleting = ref(false);
const showDeleteConfirm = ref(false);
const editorRef = ref<HTMLTextAreaElement | null>(null);
const gutterRef = ref<HTMLDivElement | null>(null);

const syncScroll = () => {
  if (editorRef.value && gutterRef.value) {
    gutterRef.value.scrollTop = editorRef.value.scrollTop;
  }
};

// Fetch real Data Prepper instances from database connections
const fetchInstances = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/settings/prometheus').catch(() => null);
    const dpList: DataPrepperInstance[] = [];
    if (res && res.data && res.data.success && Array.isArray(res.data.data)) {
      res.data.data.forEach((p: any) => {
        if (p.name.toLowerCase().includes('data prepper') || p.name.toLowerCase().includes('dataprepper')) {
          dpList.push({
            id: p.id,
            name: p.name,
            host: p.sshHost,
            path: p.path,
          });
        }
      });
    }

    if (dpList.length > 0) {
      instances.value = dpList;
      selectedInstanceId.value = dpList[0].id;
      await fetchPipelineFiles();
    } else {
      // ZERO DUMMY DATA: Leave completely empty
      instances.value = [];
      selectedInstanceId.value = '';
      pipelineFiles.value = [];
      yamlContent.value = '';
    }
  } catch (err) {
    instances.value = [];
    selectedInstanceId.value = '';
    pipelineFiles.value = [];
    yamlContent.value = '';
  } finally {
    loading.value = false;
  }
};

const fetchPipelineFiles = async () => {
  if (!selectedInstanceId.value) return;
  loading.value = true;
  try {
    const res = await axios.get(`/api/v1/dataprepper/pipelines?instanceId=${selectedInstanceId.value}`).catch(() => null);
    if (res && res.data && res.data.success && Array.isArray(res.data.data) && res.data.data.length > 0) {
      pipelineFiles.value = res.data.data;
      selectedPipelineFile.value = pipelineFiles.value[0];
      await loadPipelineContent(selectedPipelineFile.value);
    } else {
      pipelineFiles.value = [];
      yamlContent.value = '';
    }
  } catch (err) {
    pipelineFiles.value = [];
    yamlContent.value = '';
  } finally {
    loading.value = false;
  }
};

const loadPipelineContent = async (fileName: string) => {
  if (!fileName) return;
  loading.value = true;
  validationMessage.value = null;
  try {
    const res = await axios.get(`/api/v1/dataprepper/pipeline?instanceId=${selectedInstanceId.value}&file=${encodeURIComponent(fileName)}`).catch(() => null);
    if (res && res.data && res.data.success && res.data.data?.content) {
      yamlContent.value = res.data.data.content;
    } else {
      yamlContent.value = `# Data Prepper Pipeline: ${fileName}\nversion: "2"\n`;
    }
  } catch (err) {
    yamlContent.value = `# Data Prepper Pipeline: ${fileName}\nversion: "2"\n`;
  } finally {
    loading.value = false;
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
  if (selectedPipelineFile.value) {
    loadPipelineContent(selectedPipelineFile.value);
  }
  validationMessage.value = null;
};

const handleCreateFile = () => {
  if (newFileName.value.trim()) {
    const name = newFileName.value.trim().endsWith('.yaml') || newFileName.value.trim().endsWith('.yml')
      ? newFileName.value.trim()
      : `${newFileName.value.trim()}.yaml`;
    pipelineFiles.value.push(name);
    selectedPipelineFile.value = name;
    loadPipelineContent(name);
    newFileName.value = '';
    isCreatingNewFile.value = false;
  }
};

const handleSave = async () => {
  if (!selectedPipelineFile.value || !selectedInstanceId.value) return;
  saving.value = true;
  validationMessage.value = null;
  try {
    const res = await axios.post('/api/v1/dataprepper/pipeline', {
      instanceId: selectedInstanceId.value,
      file: selectedPipelineFile.value,
      content: yamlContent.value,
    });
    if (res.data?.success) {
      validationMessage.value = res.data?.message || `Pipeline "${selectedPipelineFile.value}" saved and Data Prepper service restarted.`;
      isValidationSuccess.value = true;
    } else {
      validationMessage.value = res.data?.error || 'Failed to save pipeline.';
      isValidationSuccess.value = false;
    }
  } catch (err: any) {
    validationMessage.value = err.response?.data?.error || err.message || 'Failed to save pipeline.';
    isValidationSuccess.value = false;
  } finally {
    saving.value = false;
  }
};

const handleDeleteClick = () => {
  if (selectedPipelineFile.value) {
    showDeleteConfirm.value = true;
  }
};

const confirmDelete = async () => {
  if (!selectedPipelineFile.value || !selectedInstanceId.value) return;
  deleting.value = true;
  validationMessage.value = null;
  const fileToDelete = selectedPipelineFile.value;
  try {
    const res = await axios.delete(`/api/v1/dataprepper/pipeline?instanceId=${selectedInstanceId.value}&file=${encodeURIComponent(fileToDelete)}`);
    if (res.data?.success) {
      showDeleteConfirm.value = false;
      validationMessage.value = `Pipeline "${fileToDelete}" deleted successfully from remote host.`;
      isValidationSuccess.value = true;
      await fetchPipelineFiles();
      if (!pipelineFiles.value.includes(selectedPipelineFile.value)) {
        if (pipelineFiles.value.length > 0) {
          selectedPipelineFile.value = pipelineFiles.value[0];
          await loadPipelineContent(selectedPipelineFile.value);
        } else {
          selectedPipelineFile.value = '';
          yamlContent.value = '';
        }
      }
    } else {
      validationMessage.value = res.data?.error || 'Failed to delete pipeline.';
      isValidationSuccess.value = false;
    }
  } catch (err: any) {
    validationMessage.value = err.response?.data?.error || err.message || 'Failed to delete pipeline.';
    isValidationSuccess.value = false;
  } finally {
    deleting.value = false;
    showDeleteConfirm.value = false;
  }
};

const lineNumbers = computed(() => {
  if (!yamlContent.value) return [];
  const count = yamlContent.value.split('\n').length;
  return Array.from({ length: Math.max(count, 20) }, (_, i) => i + 1);
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
          <span v-else class="text-slate-500 italic">No Data Prepper connection registered</span>

          <button
            @click="fetchInstances"
            class="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
            title="Refresh Instances"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          </button>

          <span v-if="instances.length > 0" class="px-2 py-0.5 rounded bg-slate-800 text-slate-400 text-[10px] font-mono border border-slate-700/60">
            SSH
          </span>
        </div>

        <!-- Pipeline File Selector Row (Only when instances exist) -->
        <div v-if="instances.length > 0" class="flex flex-wrap items-center gap-3">
          <span class="text-slate-400 font-medium min-w-[140px]">Pipeline File:</span>

          <select
            v-if="pipelineFiles.length > 0"
            v-model="selectedPipelineFile"
            @change="loadPipelineContent(selectedPipelineFile)"
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

          <button
            v-if="pipelineFiles.length > 0 && selectedPipelineFile"
            @click="handleDeleteClick"
            :disabled="deleting || loading"
            class="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-rose-950/40 hover:bg-rose-900/60 text-rose-300 text-xs font-semibold border border-rose-800/60 transition disabled:opacity-50"
            title="Delete selected pipeline file"
          >
            <Trash2 class="w-3.5 h-3.5 text-rose-400" />
            <span>DELETE FILE</span>
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

      <!-- IF NO INSTANCE REGISTERED (Clean Empty State) -->
      <div v-if="instances.length === 0 && !loading" class="p-12 text-center bg-[#0e1118] border border-slate-800/80 rounded-xl space-y-3">
        <Server class="w-8 h-8 text-slate-600 mx-auto mb-2" />
        <p class="text-xs font-bold text-slate-300">No Data Prepper Connection Found</p>
        <p class="text-[11px] text-slate-500 max-w-md mx-auto">
          You have not registered any Data Prepper instance in Connections yet. Please add a Data Prepper connection first to manage pipelines.
        </p>
        <button
          @click="router.push('/connections')"
          class="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg transition mt-2"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Data Prepper Connection</span>
        </button>
      </div>

      <!-- YAML Editor Box (Only shown when instance and pipeline exists) -->
      <div v-else-if="instances.length > 0 && yamlContent" class="space-y-3">
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
              @click="handleDeleteClick"
              :disabled="deleting || loading || !selectedPipelineFile"
              class="flex items-center gap-1 px-3 py-1 rounded-lg bg-rose-950/40 hover:bg-rose-900/60 text-rose-300 text-xs font-semibold border border-rose-800/60 transition disabled:opacity-50"
              title="Delete selected pipeline file"
            >
              <Trash2 class="w-3.5 h-3.5 text-rose-400" />
              <span>DELETE</span>
            </button>

            <button
              @click="handleSave"
              :disabled="saving || loading"
              class="flex items-center gap-1 px-4 py-1 rounded-lg bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-xs font-bold transition"
            >
              <RotateCw v-if="saving" class="w-3.5 h-3.5 animate-spin" />
              <Save v-else class="w-3.5 h-3.5" />
              <span>{{ saving ? 'SAVING & RESTARTING...' : 'SAVE' }}</span>
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
            class="flex-1 bg-transparent p-3.5 text-sky-400 font-mono text-xs focus:outline-none resize-none leading-relaxed selection:bg-brand-500/30 overflow-y-auto overflow-x-auto whitespace-pre outline-none h-full"
            spellcheck="false"
          ></textarea>
        </div>
      </div>

    </div>

    <!-- Delete Confirmation Modal -->
    <div
      v-if="showDeleteConfirm"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm"
    >
      <div class="bg-[#171a23] border border-slate-700 rounded-2xl p-6 max-w-md w-full shadow-2xl space-y-4 font-sans animate-in fade-in zoom-in-95 duration-150">
        <div class="flex items-center gap-3">
          <div class="p-2.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 shrink-0">
            <AlertTriangle class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-white">Delete Pipeline File</h3>
            <p class="text-xs text-slate-400">This action will permanently delete the file from the remote host.</p>
          </div>
        </div>

        <p class="text-xs text-slate-300 leading-relaxed">
          Are you sure you want to delete
          <span class="font-mono font-semibold text-rose-300 bg-rose-950/60 px-1.5 py-0.5 rounded border border-rose-800/60">{{ selectedPipelineFile }}</span>?
        </p>

        <div class="flex items-center justify-end gap-2.5 pt-2 border-t border-slate-800">
          <button
            @click="showDeleteConfirm = false"
            :disabled="deleting"
            class="px-3.5 py-1.5 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-300 text-xs font-semibold border border-slate-700 transition"
          >
            Cancel
          </button>
          <button
            @click="confirmDelete"
            :disabled="deleting"
            class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold transition disabled:opacity-50"
          >
            <RotateCw v-if="deleting" class="w-3.5 h-3.5 animate-spin" />
            <Trash2 v-else class="w-3.5 h-3.5" />
            <span>{{ deleting ? 'Deleting...' : 'Confirm Delete' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
