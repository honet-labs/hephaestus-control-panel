<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import {
  RotateCw,
  Check,
  Save,
  Plus,
  Trash2,
  ExternalLink,
  AlertTriangle,
  Server,
  History,
  X,
  Loader2,
  Edit3,
  ArrowRight,
  FileCode,
  Copy,
  CheckCircle2,
} from 'lucide-vue-next';

const router = useRouter();
const authStore = useAuthStore();
const canManage = computed(() => authStore.can('dataprepper_config', 'manage'));

interface DataPrepperInstance {
  id: string;
  name: string;
  host?: string;
  path?: string;
}

interface DataPrepperHistoryItem {
  id: string;
  instanceId: string;
  pipelineFile: string;
  content: string;
  createdBy: string;
  changeSummary: string;
  createdAt: string;
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
const validationErrorLine = ref<number | null>(null);
let validationTimer: ReturnType<typeof setTimeout> | null = null;

// History & Rollbacks
const showHistoryModal = ref(false);
const historyList = ref<DataPrepperHistoryItem[]>([]);
const loadingHistory = ref(false);
const previewHistoryItem = ref<DataPrepperHistoryItem | null>(null);
const configSummary = ref('');

const formatDate = (isoString?: string) => {
  if (!isoString) return '-';
  try {
    const d = new Date(isoString);
    return d.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  } catch {
    return isoString;
  }
};

const fetchHistory = async () => {
  if (!selectedInstanceId.value || !selectedPipelineFile.value) return;
  loadingHistory.value = true;
  previewHistoryItem.value = null;
  try {
    const res = await axios.get(
      `/api/v1/dataprepper/pipeline/history?instanceId=${selectedInstanceId.value}&file=${encodeURIComponent(selectedPipelineFile.value)}`
    );
    if (res.data?.success && Array.isArray(res.data.data)) {
      historyList.value = res.data.data;
      if (historyList.value.length > 0) {
        previewHistoryItem.value = historyList.value[0];
      }
    } else {
      historyList.value = [];
    }
  } catch {
    historyList.value = [];
  } finally {
    loadingHistory.value = false;
  }
};

const openHistoryModal = async () => {
  showHistoryModal.value = true;
  await fetchHistory();
};

const restoreHistoryVersion = (item: DataPrepperHistoryItem) => {
  yamlContent.value = item.content;
  showHistoryModal.value = false;
  validationMessage.value = `Snapshot from ${formatDate(item.createdAt)} restored into editor. Click "SAVE & RESTART" to apply.`;
  isValidationSuccess.value = true;
};

// Rename File
const showRenameModal = ref(false);
const renameTargetOld = ref('');
const renameTargetNew = ref('');
const renaming = ref(false);
const renameError = ref<string | null>(null);

const openRenameModal = () => {
  if (!selectedPipelineFile.value) return;
  renameTargetOld.value = selectedPipelineFile.value;
  renameTargetNew.value = selectedPipelineFile.value;
  renameError.value = null;
  showRenameModal.value = true;
};

const executeRename = async () => {
  if (!renameTargetNew.value.trim() || !selectedInstanceId.value) return;
  renaming.value = true;
  renameError.value = null;

  try {
    const res = await axios.post('/api/v1/dataprepper/pipeline/rename', {
      instanceId: selectedInstanceId.value,
      oldFile: renameTargetOld.value,
      newFile: renameTargetNew.value.trim(),
    });

    if (res.data?.success) {
      const newName = res.data.newFile || renameTargetNew.value.trim();
      showRenameModal.value = false;
      validationMessage.value = res.data.message || `Pipeline renamed to "${newName}" successfully.`;
      isValidationSuccess.value = true;
      await fetchPipelineFiles();
      selectedPipelineFile.value = newName;
      await loadPipelineContent(newName);
    } else {
      renameError.value = res.data?.error || 'Failed to rename file.';
    }
  } catch (err: any) {
    renameError.value = err.response?.data?.error || err.message || 'Failed to rename file.';
  } finally {
    renaming.value = false;
  }
};

// Auto-dismiss notification after 3 seconds only on success (errors remain visible)
watch(validationMessage, (newVal) => {
  if (validationTimer) {
    clearTimeout(validationTimer);
    validationTimer = null;
  }
  if (newVal && isValidationSuccess.value) {
    validationTimer = setTimeout(() => {
      validationMessage.value = null;
      restartResult.value = null;
      validationTimer = null;
    }, 3000);
  }
});

const yamlContent = ref('');
const initialLoadedContent = ref('');
const hasUnsavedChanges = computed(() => yamlContent.value !== initialLoadedContent.value);
const saving = ref(false);
const deleting = ref(false);
const showDeleteConfirm = ref(false);
const editorRef = ref<HTMLTextAreaElement | null>(null);
const gutterRef = ref<HTMLDivElement | null>(null);
const copied = ref(false);

const copyYamlToClipboard = async () => {
  if (!yamlContent.value) return;
  try {
    await navigator.clipboard.writeText(yamlContent.value);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch (e) {
    console.error('Failed to copy YAML:', e);
  }
};

// Reset error line when content changes
watch(yamlContent, () => {
  if (validationErrorLine.value !== null) {
    validationErrorLine.value = null;
  }
});

const extractLineFromError = (msg: string): number | null => {
  if (!msg) return null;
  const match = msg.match(/(?:line\s+(\d+)|\.ya?ml:(\d+))/i);
  if (match) {
    return parseInt(match[1] || match[2], 10);
  }
  return null;
};

const jumpToErrorLine = (line: number) => {
  if (!editorRef.value || line < 1) return;
  const lines = yamlContent.value.split('\n');
  let charIndex = 0;
  for (let i = 0; i < Math.min(line - 1, lines.length); i++) {
    charIndex += lines[i].length + 1;
  }
  const lineLength = lines[line - 1] ? lines[line - 1].length : 0;
  editorRef.value.focus();
  editorRef.value.setSelectionRange(charIndex, charIndex + lineLength);
  const approxLineHeight = 22;
  const targetScroll = Math.max(0, (line - 4) * approxLineHeight);
  editorRef.value.scrollTop = targetScroll;
  if (gutterRef.value) {
    gutterRef.value.scrollTop = targetScroll;
  }
};

const syncScroll = () => {
  if (editorRef.value && gutterRef.value) {
    gutterRef.value.scrollTop = editorRef.value.scrollTop;
  }
};

// Fetch real Data Prepper instances from database connections
const fetchInstances = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/settings/prometheus?type=dataprepper').catch(() => null);
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
      initialLoadedContent.value = res.data.data.content;
    } else {
      yamlContent.value = `# Data Prepper Pipeline: ${fileName}\nversion: "2"\n`;
      initialLoadedContent.value = yamlContent.value;
    }
  } catch (err) {
    yamlContent.value = `# Data Prepper Pipeline: ${fileName}\nversion: "2"\n`;
    initialLoadedContent.value = yamlContent.value;
  } finally {
    loading.value = false;
  }
};

const handleValidate = async () => {
  validationErrorLine.value = null;
  try {
    const res = await axios.post('/api/v1/dataprepper/validate', { yaml: yamlContent.value });
    if (res.data?.success) {
      isValidationSuccess.value = !!res.data.valid;
      if (res.data.valid) {
        validationMessage.value = 'Validation Success: Data Prepper pipeline YAML is valid & compliant.';
        validationErrorLine.value = null;
      } else {
        const errMsg = res.data.message || res.data.error || 'Invalid YAML schema';
        validationMessage.value = errMsg;
        let line = typeof res.data.line === 'number' && res.data.line > 0 ? res.data.line : null;
        if (!line) {
          line = extractLineFromError(errMsg);
        }
        validationErrorLine.value = line;
        if (line) {
          setTimeout(() => jumpToErrorLine(line), 60);
        }
      }
    } else {
      validationMessage.value = res.data?.error || res.data?.message || 'Invalid YAML schema';
      isValidationSuccess.value = false;
      const line = extractLineFromError(validationMessage.value || '');
      validationErrorLine.value = line;
      if (line) {
        setTimeout(() => jumpToErrorLine(line), 60);
      }
    }
  } catch (err: any) {
    const errMsg = err.response?.data?.error || err.response?.data?.message || err.message || 'Validation error.';
    validationMessage.value = errMsg;
    isValidationSuccess.value = false;
    const line = extractLineFromError(errMsg);
    validationErrorLine.value = line;
    if (line) {
      setTimeout(() => jumpToErrorLine(line), 60);
    }
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

const restartResult = ref<{ attempted?: boolean; success?: boolean; output?: string; error?: string } | null>(null);

const handleSave = async () => {
  if (!selectedPipelineFile.value || !selectedInstanceId.value) return;
  saving.value = true;
  validationMessage.value = null;
  restartResult.value = null;

  try {
    const res = await axios.post('/api/v1/dataprepper/pipeline', {
      instanceId: selectedInstanceId.value,
      file: selectedPipelineFile.value,
      content: yamlContent.value,
      changeSummary: configSummary.value.trim() || undefined,
    });
    if (res.data?.success) {
      initialLoadedContent.value = yamlContent.value;
      configSummary.value = '';
      const data = res.data.data;
      if (data?.restart) {
        restartResult.value = data.restart;
        if (data.restart.success) {
          validationMessage.value = data.message || `Pipeline "${selectedPipelineFile.value}" saved and Data Prepper restarted successfully.`;
          isValidationSuccess.value = true;
        } else {
          validationMessage.value = data.message || `Pipeline "${selectedPipelineFile.value}" saved, but service restart failed: ${data.restart.error}`;
          isValidationSuccess.value = false;
        }
      } else {
        validationMessage.value = res.data?.message || `Pipeline "${selectedPipelineFile.value}" saved and Data Prepper service restarted.`;
        isValidationSuccess.value = true;
      }
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

const handleKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Tab') {
    e.preventDefault();
    const textarea = editorRef.value;
    if (!textarea) return;

    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const spaces = '  ';

    yamlContent.value = yamlContent.value.substring(0, start) + spaces + yamlContent.value.substring(end);
    setTimeout(() => {
      textarea.selectionStart = textarea.selectionEnd = start + spaces.length;
    }, 0);
  }
};

const lineNumbers = computed(() => {
  if (!yamlContent.value) return Array.from({ length: 20 }, (_, i) => i + 1);
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
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Data Prepper Pipelines
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Edit, validate, and manage Data Prepper pipeline YAML configurations.
        </p>
      </div>

      <!-- Header Action Buttons -->
      <div class="flex items-center gap-2 shrink-0">
        <button
          @click="fetchInstances"
          :disabled="loading"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white dark:bg-[#121826] hover:bg-slate-50 dark:hover:bg-[#1a2336] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#1b2234] transition shadow-xs cursor-pointer disabled:opacity-50"
          title="Refresh Data Prepper instances and pipelines"
        >
          <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          <span class="hidden sm:inline">Refresh</span>
        </button>

        <button
          @click="router.push('/connections')"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-200 text-xs font-semibold border border-slate-300 dark:border-slate-700 transition cursor-pointer"
        >
          <ExternalLink class="w-3.5 h-3.5" />
          <span>Go to Connections</span>
        </button>
      </div>
    </div>

    <!-- Top Controls: Instance & Pipeline File Management Card -->
    <div class="bg-white dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-2xl p-4 shadow-sm space-y-4">
      <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-3">
        <!-- Left: Instance & Pipeline Dropdowns -->
        <div class="flex flex-wrap items-center gap-3">
          <!-- Instance Selector -->
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold text-slate-500 dark:text-slate-400">Instance:</span>
            <select
              v-if="instances.length > 0"
              v-model="selectedInstanceId"
              @change="fetchPipelineFiles"
              class="bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#222d42] rounded-lg px-3 py-1.5 text-xs text-slate-900 dark:text-white font-semibold focus:outline-none focus:border-blue-500 min-w-[200px]"
            >
              <option v-for="inst in instances" :key="inst.id" :value="inst.id">
                {{ inst.name }}
              </option>
            </select>
            <span v-else class="text-xs text-slate-400 italic">No connection registered</span>
            <span v-if="instances.length > 0" class="px-2 py-0.5 rounded bg-slate-100 dark:bg-[#192233] text-slate-600 dark:text-slate-400 text-[10px] font-mono border border-slate-200 dark:border-[#222d42]">
              SSH
            </span>
          </div>

          <!-- Divider dot -->
          <span v-if="instances.length > 0 && pipelineFiles.length > 0" class="text-slate-300 dark:text-slate-700 hidden sm:inline">•</span>

          <!-- Pipeline File Selector -->
          <div v-if="instances.length > 0" class="flex items-center gap-2">
            <span class="text-xs font-bold text-slate-500 dark:text-slate-400">Pipeline:</span>
            <select
              v-if="pipelineFiles.length > 0"
              v-model="selectedPipelineFile"
              @change="loadPipelineContent(selectedPipelineFile)"
              class="bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#222d42] rounded-lg px-3 py-1.5 text-xs text-slate-900 dark:text-white font-mono font-semibold focus:outline-none focus:border-blue-500 min-w-[200px]"
            >
              <option v-for="file in pipelineFiles" :key="file" :value="file">
                {{ file }}
              </option>
            </select>
            <span v-else class="text-xs text-slate-400 italic">No pipelines found</span>
          </div>
        </div>

        <!-- Right: Pipeline File Management Actions (Unified Single Place) -->
        <div v-if="instances.length > 0" class="flex items-center gap-1.5 shrink-0 flex-wrap">
          <!-- New Pipeline -->
          <button
            v-if="canManage"
            @click="isCreatingNewFile = !isCreatingNewFile"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold transition shadow-xs cursor-pointer"
            title="Create new pipeline YAML file"
          >
            <Plus class="w-3.5 h-3.5 stroke-[2.5]" />
            <span>New Pipeline</span>
          </button>

          <!-- Rename Pipeline -->
          <button
            v-if="canManage && pipelineFiles.length > 0 && selectedPipelineFile"
            @click="openRenameModal"
            class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#161d2c] dark:hover:bg-[#1f2a3f] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#243046] transition cursor-pointer"
            title="Rename selected pipeline file"
          >
            <Edit3 class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
            <span>Rename</span>
          </button>

          <!-- History / Rollback -->
          <button
            v-if="pipelineFiles.length > 0 && selectedPipelineFile"
            @click="openHistoryModal"
            class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#161d2c] dark:hover:bg-[#1f2a3f] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#243046] transition cursor-pointer"
            title="View configuration snapshot history & rollback"
          >
            <History class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
            <span>History</span>
          </button>

          <!-- Delete Pipeline -->
          <button
            v-if="canManage && pipelineFiles.length > 0 && selectedPipelineFile"
            @click="handleDeleteClick"
            :disabled="deleting || loading"
            class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-rose-50 hover:bg-rose-100 dark:bg-rose-950/40 dark:hover:bg-rose-900/60 text-rose-700 dark:text-rose-300 text-xs font-semibold border border-rose-200 dark:border-rose-900/60 transition cursor-pointer disabled:opacity-50"
            title="Delete selected pipeline file"
          >
            <Trash2 class="w-3.5 h-3.5 text-rose-600 dark:text-rose-400" />
            <span>Delete</span>
          </button>
        </div>
      </div>

      <!-- Inline New File Dialog -->
      <div v-if="isCreatingNewFile" class="flex items-center gap-2 p-3 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#222d42] rounded-xl max-w-md shadow-xs animate-in fade-in">
        <input
          v-model="newFileName"
          placeholder="e.g. metrics-pipeline.yaml"
          class="flex-1 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-slate-700 rounded-lg px-2.5 py-1 text-slate-900 dark:text-white placeholder-slate-400 text-xs font-mono focus:outline-none focus:border-blue-500"
          @keyup.enter="handleCreateFile"
        />
        <button @click="handleCreateFile" class="px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold cursor-pointer">
          Create
        </button>
        <button @click="isCreatingNewFile = false" class="px-2 py-1 text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white text-xs cursor-pointer">
          Cancel
        </button>
      </div>
    </div>

    <!-- Empty State: No Instances Found -->
    <div v-if="instances.length === 0 && !loading" class="p-12 text-center bg-white dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-2xl space-y-3 shadow-sm">
      <Server class="w-8 h-8 text-slate-400 dark:text-slate-600 mx-auto mb-2" />
      <p class="text-xs font-bold text-slate-900 dark:text-slate-300">No Data Prepper Connection Found</p>
      <p class="text-[11px] text-slate-500 max-w-md mx-auto">
        You have not registered any Data Prepper instance in Connections yet. Please add a Data Prepper connection first to manage pipelines.
      </p>
      <button
        @click="router.push('/connections')"
        class="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg transition mt-2 cursor-pointer shadow-sm shadow-blue-500/20"
      >
        <Plus class="w-3.5 h-3.5" />
        <span>Add Data Prepper Connection</span>
      </button>
    </div>

    <!-- Validation & Restart Feedback Banner -->
    <div
      v-if="validationMessage"
      :class="[
        'p-3.5 rounded-xl border text-xs font-mono space-y-1 animate-in fade-in transition-all duration-200 shadow-xs',
        isValidationSuccess
          ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-700 dark:text-emerald-300'
          : 'bg-rose-500/10 border-rose-500/30 text-rose-700 dark:text-rose-300'
      ]"
    >
      <div class="flex items-center justify-between gap-2 flex-wrap">
        <div class="flex items-center gap-2 font-semibold">
          <span class="w-2 h-2 rounded-full shrink-0" :class="isValidationSuccess ? 'bg-emerald-500 dark:bg-emerald-400' : 'bg-rose-500 dark:bg-rose-400'"></span>
          <span class="break-all">{{ validationMessage }}</span>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <button
            v-if="!isValidationSuccess && validationErrorLine"
            @click="jumpToErrorLine(validationErrorLine)"
            class="flex items-center gap-1 px-2.5 py-1 rounded bg-rose-500/20 hover:bg-rose-500/30 text-rose-800 dark:text-rose-200 font-bold text-xs transition cursor-pointer"
            title="Jump to error line in editor"
          >
            <span>Jump to Line {{ validationErrorLine }}</span>
            <ArrowRight class="w-3.5 h-3.5" />
          </button>
          <button
            @click="validationMessage = null; restartResult = null; validationErrorLine = null;"
            class="opacity-60 hover:opacity-100 p-0.5 cursor-pointer text-sm leading-none font-bold"
            title="Close notification"
          >
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>
      <div v-if="restartResult?.output" class="text-[11px] text-slate-700 dark:text-slate-300 pl-4 border-l border-slate-300 dark:border-slate-700/50 mt-1">
        Status: {{ restartResult.output }}
      </div>
      <div v-if="restartResult && !restartResult.success && restartResult.error" class="text-[11px] text-rose-700 dark:text-rose-400 pl-4 border-l border-rose-300 dark:border-rose-700/50 mt-1">
        Error: {{ restartResult.error }}
      </div>
    </div>

    <!-- YAML Code Editor Card (Shown whenever an instance and pipeline file is selected, even if empty) -->
    <div v-if="instances.length > 0 && selectedPipelineFile" class="bg-white dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-2xl overflow-hidden shadow-sm flex flex-col">
      <!-- Editor Top Sub-bar -->
      <div class="flex items-center justify-between px-4 py-2.5 border-b border-slate-200 dark:border-[#1b2234] bg-slate-50 dark:bg-[#121826] text-xs">
        <div class="flex items-center gap-2">
          <FileCode class="w-4 h-4 text-slate-400 dark:text-slate-500" />
          <span class="font-mono font-bold text-slate-800 dark:text-slate-200 text-[11px]">
            {{ selectedPipelineFile || 'pipeline.yaml' }}
          </span>
          <span v-if="hasUnsavedChanges" class="text-[10px] font-semibold px-2 py-0.2 rounded-full bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
            Unsaved Changes
          </span>
          <span v-if="!canManage" class="text-[10px] font-mono font-bold px-2 py-0.2 rounded bg-slate-200 dark:bg-[#1c2436] text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-[#28354f]">
            READ ONLY
          </span>
        </div>

        <!-- Status & Tool Actions -->
        <div class="flex items-center gap-3">
          <!-- YAML Validation Status Indicator -->
          <div v-if="!validationErrorLine && isValidationSuccess && validationMessage" class="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400 text-[11px] font-medium font-mono">
            <CheckCircle2 class="w-3.5 h-3.5" />
            <span>Valid YAML</span>
          </div>
          <button
            v-else-if="validationErrorLine"
            @click="jumpToErrorLine(validationErrorLine)"
            class="flex items-center gap-1.5 text-rose-600 dark:text-rose-400 hover:text-rose-700 dark:hover:text-rose-300 text-[11px] font-medium font-mono cursor-pointer bg-rose-500/10 hover:bg-rose-500/20 px-2 py-0.5 rounded transition"
            :title="'Click to jump to line ' + validationErrorLine"
          >
            <AlertTriangle class="w-3.5 h-3.5 shrink-0" />
            <span class="truncate max-w-[280px]">Error on Line {{ validationErrorLine }}</span>
            <ArrowRight class="w-3 h-3 shrink-0" />
          </button>

          <!-- Reset Button -->
          <button
            @click="handleReset"
            class="flex items-center gap-1 px-2 py-1 rounded bg-slate-200/60 dark:bg-[#1c2436] hover:bg-slate-300 dark:hover:bg-[#253047] text-slate-700 dark:text-slate-300 text-[11px] font-semibold transition cursor-pointer"
            title="Reset pipeline back to last saved version"
          >
            <RotateCw class="w-3 h-3 text-slate-500 dark:text-slate-400" />
            <span>Reset</span>
          </button>

          <!-- Copy Button -->
          <button
            @click="copyYamlToClipboard"
            class="flex items-center gap-1 px-2 py-1 rounded bg-slate-200/60 dark:bg-[#1c2436] hover:bg-slate-300 dark:hover:bg-[#253047] text-slate-700 dark:text-slate-300 text-[11px] font-semibold transition cursor-pointer"
            title="Copy YAML to clipboard"
          >
            <component :is="copied ? Check : Copy" class="w-3 h-3" />
            <span>{{ copied ? 'Copied' : 'Copy' }}</span>
          </button>
        </div>
      </div>

      <!-- Textarea Code Editor Area -->
      <div class="relative bg-white dark:bg-[#090d16] font-mono text-xs">
        <div class="flex min-h-[460px] max-h-[600px] h-[calc(100vh-380px)]">
          <!-- Line Numbers Gutter -->
          <div
            ref="gutterRef"
            class="w-12 py-3 bg-slate-100 dark:bg-[#070a10] border-r border-slate-200 dark:border-slate-800 text-slate-400 dark:text-slate-500 text-right pr-2.5 select-none overflow-hidden shrink-0 font-mono text-[11px] leading-relaxed pointer-events-none"
          >
            <div
              v-for="n in lineNumbers"
              :key="n"
              :class="[
                'leading-relaxed transition-colors px-1',
                validationErrorLine === n
                  ? 'bg-rose-500/25 text-rose-600 dark:text-rose-400 font-bold border-r-2 border-rose-500 -mr-2.5 pr-2'
                  : ''
              ]"
            >
              {{ n }}
            </div>
          </div>

          <!-- Code Textarea Area -->
          <textarea
            ref="editorRef"
            v-model="yamlContent"
            :readonly="!canManage"
            @scroll="syncScroll"
            @keydown="handleKeyDown"
            class="flex-1 p-3 bg-white dark:bg-transparent text-slate-800 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none resize-none font-mono text-[11px] leading-relaxed whitespace-pre overflow-y-auto selection:bg-blue-500/20 dark:selection:bg-blue-600/40 outline-none"
            style="border: none !important; box-shadow: none !important;"
            placeholder="# Paste or type Data Prepper pipeline YAML configuration..."
            spellcheck="false"
          ></textarea>
        </div>
      </div>

      <!-- Bottom Deployment Action Bar (Clean layout like OpenTelemetry) -->
      <div class="p-3 bg-slate-50 dark:bg-[#121826] border-t border-slate-200 dark:border-[#1b2234] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <!-- Left: Optional Change Note Input -->
        <div class="flex-1 max-w-md">
          <input
            v-if="canManage"
            v-model="configSummary"
            type="text"
            placeholder="Optional commit / change note (e.g. Added OpenSearch sink index)"
            class="w-full px-3 py-1.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs text-slate-800 dark:text-slate-200 placeholder-slate-400 focus:outline-none focus:border-blue-500 transition"
          />
        </div>

        <!-- Right: Action Buttons -->
        <div class="flex items-center gap-2 shrink-0">
          <button
            @click="handleValidate"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white dark:bg-[#161d2c] hover:bg-slate-100 dark:hover:bg-[#1f2a3f] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#243046] transition cursor-pointer shadow-xs"
            title="Validate YAML syntax"
          >
            <Check class="w-3.5 h-3.5 text-emerald-500" />
            <span>Validate YAML</span>
          </button>

          <button
            v-if="canManage"
            @click="handleSave"
            :disabled="saving || loading || !selectedPipelineFile"
            class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 active:scale-98 disabled:opacity-50 text-white text-xs font-bold transition cursor-pointer shadow-sm shadow-blue-500/20"
          >
            <Loader2 v-if="saving" class="w-3.5 h-3.5 animate-spin" />
            <Save v-else class="w-3.5 h-3.5" />
            <span>{{ saving ? 'Saving & Restarting...' : 'Save & Restart' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal (Standard HCP Modal) -->
    <div
      v-if="showDeleteConfirm"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Pipeline File?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200 font-mono">{{ selectedPipelineFile }}</strong>? This action cannot be undone.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteConfirm = false"
            :disabled="deleting"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="confirmDelete"
            :disabled="deleting"
            class="flex items-center gap-1.5 px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            <Loader2 v-if="deleting" class="w-3.5 h-3.5 animate-spin" />
            <Trash2 v-else class="w-3.5 h-3.5" />
            <span>{{ deleting ? 'Deleting...' : 'Confirm Delete' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- ==================== HISTORY / ROLLBACK MODAL ==================== -->
    <div
      v-if="showHistoryModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-5xl h-[85vh] min-h-[580px] shadow-2xl overflow-hidden flex flex-col">
        <!-- Modal Header -->
        <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-[#1b2234] bg-slate-50/50 dark:bg-[#161d2d]/50 shrink-0">
          <div class="flex items-center gap-2">
            <History class="w-4 h-4 text-slate-400 dark:text-slate-500" />
            <h2 class="text-sm font-bold text-slate-900 dark:text-white">Pipeline History & Rollbacks</h2>
            <span class="font-mono text-xs text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-950/40 px-2 py-0.5 rounded border border-blue-200 dark:border-blue-900/50">{{ selectedPipelineFile }}</span>
            <span v-if="historyList.length > 0" class="text-xs text-slate-400">({{ historyList.length }} snapshots)</span>
          </div>
          <button @click="showHistoryModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-12 flex-1 min-h-0 overflow-hidden">
          <!-- Left history list -->
          <div class="md:col-span-4 border-r border-slate-200 dark:border-[#1b2234] overflow-y-auto p-3 space-y-2 h-full bg-slate-50/30 dark:bg-[#0c1019]">
            <div v-if="loadingHistory" class="p-8 text-center text-xs text-slate-400 flex flex-col items-center gap-2">
              <Loader2 class="w-5 h-5 animate-spin text-slate-500" />
              <span>Loading version history...</span>
            </div>
            <div v-else-if="historyList.length === 0" class="p-8 text-center text-xs text-slate-500">
              No version history recorded yet. Snapshots are created automatically before every pipeline save & restart.
            </div>
            <div
              v-else
              v-for="item in historyList"
              :key="item.id"
              @click="previewHistoryItem = item"
              :class="[
                previewHistoryItem?.id === item.id 
                  ? 'bg-blue-50 dark:bg-blue-950/40 border-blue-300 dark:border-blue-800 shadow-xs' 
                  : 'bg-white dark:bg-[#121826] border-slate-200 dark:border-[#1b2234] hover:border-slate-300 dark:hover:border-slate-700',
                'p-3 rounded-xl border transition cursor-pointer text-left space-y-1'
              ]"
            >
              <div class="flex items-center justify-between text-[11px]">
                <span class="font-bold text-slate-800 dark:text-slate-200 font-mono">{{ formatDate(item.createdAt) }}</span>
              </div>
              <p class="text-xs text-slate-600 dark:text-slate-400 truncate">{{ item.changeSummary || 'Automatic backup before deploy' }}</p>
              <div class="flex items-center justify-between text-[10px] text-slate-400 pt-0.5">
                <span>By {{ item.createdBy || 'system' }}</span>
                <span>{{ (item.content?.length || 0).toLocaleString() }} bytes</span>
              </div>
            </div>
          </div>

          <!-- Right preview container -->
          <div class="md:col-span-8 p-4 flex flex-col bg-slate-50 dark:bg-[#0a0d15] text-slate-800 dark:text-slate-200 font-mono text-[11px] h-full min-h-0">
            <div v-if="previewHistoryItem" class="flex-1 min-h-0 flex flex-col space-y-3">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5 shrink-0">
                <div class="space-y-0.5">
                  <div class="flex items-center gap-2">
                    <span class="text-xs font-bold text-slate-800 dark:text-slate-200 font-sans">Snapshot Preview</span>
                    <span class="text-[10px] text-slate-500 font-mono">{{ (previewHistoryItem.content?.length || 0).toLocaleString() }} bytes</span>
                  </div>
                  <p class="text-[11px] text-slate-400 font-sans">{{ formatDate(previewHistoryItem.createdAt) }} — {{ previewHistoryItem.changeSummary || 'Automatic backup' }}</p>
                </div>
                <button
                  @click="restoreHistoryVersion(previewHistoryItem)"
                  class="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer shadow-xs font-sans"
                >
                  <RotateCw class="w-3.5 h-3.5" />
                  <span>Restore This Version</span>
                </button>
              </div>
              <textarea
                readonly
                :value="previewHistoryItem.content"
                class="flex-1 min-h-0 w-full bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] rounded-xl text-slate-800 dark:text-slate-200 resize-none p-3.5 focus:outline-none text-xs leading-relaxed overflow-y-auto whitespace-pre font-mono shadow-inner"
              ></textarea>
            </div>
            <div v-else class="flex-1 flex items-center justify-center text-slate-500 italic font-sans text-xs">
              Select a version snapshot to preview.
            </div>
          </div>
        </div>
      </div>
    </div>
    <!-- ==================== RENAME FILE MODAL ==================== -->
    <div
      v-if="showRenameModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <Edit3 class="w-4 h-4 text-slate-500 dark:text-slate-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">Rename Pipeline File</h3>
          </div>
          <button @click="showRenameModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3">
          <div class="space-y-1">
            <label class="text-xs text-slate-500 dark:text-slate-400">Current Filename</label>
            <div class="font-mono text-xs font-semibold text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-[#0f1219] px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-800 truncate">
              {{ renameTargetOld }}
            </div>
          </div>

          <div class="space-y-1">
            <label class="text-xs text-slate-700 dark:text-slate-300 font-medium">New Filename</label>
            <input
              v-model="renameTargetNew"
              @keydown.enter="executeRename"
              type="text"
              placeholder="e.g. otc-docker-logs.yaml"
              class="w-full font-mono text-xs px-3 py-2 bg-white dark:bg-[#0a0d15] border border-slate-300 dark:border-slate-700 rounded-lg text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              autofocus
            />
            <p class="text-[11px] text-slate-400">Must end with .yaml or .yml. Data Prepper service will restart automatically.</p>
          </div>

          <div v-if="renameError" class="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-500 text-xs font-medium">
            {{ renameError }}
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-[#1b2234]">
          <button
            @click="showRenameModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeRename"
            :disabled="renaming || !renameTargetNew.trim() || renameTargetNew.trim() === renameTargetOld"
            class="flex items-center gap-1.5 px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            <Loader2 v-if="renaming" class="w-3.5 h-3.5 animate-spin" />
            <Edit3 v-else class="w-3.5 h-3.5" />
            <span>{{ renaming ? 'Renaming...' : 'Rename File' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
