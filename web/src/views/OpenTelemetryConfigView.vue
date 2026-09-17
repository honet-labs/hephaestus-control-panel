<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import {
  Server,
  Plus,
  RotateCw,
  Play,
  Save,
  Trash2,
  Edit3,
  CheckCircle2,
  AlertTriangle,
  FileCode,
  History,
  Terminal,
  Search,
  X,
  Copy,
  ChevronDown,
  ChevronUp,
  Tag,
  Key,
  Lock,
  Eye,
  EyeOff,
  Check,
  ExternalLink,
  Shield,
  Layers,
  ArrowRight,
  Info,
} from 'lucide-vue-next';

interface OTelHost {
  id: string;
  name: string;
  tags?: string[];
  sshHost: string;
  sshPort: number;
  sshUser: string;
  sshAuth: string;
  sshPassword?: string;
  sshKey?: string;
  configPath: string;
  serviceName: string;
  reloadMode: string;
  lastStatus: string; // active, inactive, failed, unreachable, unknown
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

interface OTelPreset {
  id: string;
  name: string;
  description: string;
  category: string;
  content: string;
}

interface OTelHistoryItem {
  id: string;
  otelConfigId: string;
  content: string;
  createdBy?: string;
  changeSummary?: string;
  createdAt: string;
}

// Auth & RBAC
const authStore = useAuthStore();
const canManage = computed(() => authStore.can('opentelemetry_config', 'manage'));

// State
const hosts = ref<OTelHost[]>([]);
const selectedHostId = ref<string>('');
const selectedHost = computed(() => hosts.value.find((h) => h.id === selectedHostId.value) || null);
const loadingHosts = ref(false);
const searchQuery = ref('');
const statusFilter = ref<string>('all'); // all, active, failed, unreachable

// Editor state
const yamlContent = ref('');
const initialLoadedContent = ref('');
const configSummary = ref('');
const saving = ref(false);
const loadingConfig = ref(false);
const editorRef = ref<HTMLTextAreaElement | null>(null);
const gutterRef = ref<HTMLDivElement | null>(null);

// Status & diagnostics
const restartingService = ref(false);
const feedbackMsg = ref<{ type: 'success' | 'error' | 'info'; title: string; detail?: string } | null>(null);
let feedbackTimer: ReturnType<typeof setTimeout> | null = null;

// Auto-dismiss notification after 3 seconds
watch(feedbackMsg, (newVal) => {
  if (feedbackTimer) {
    clearTimeout(feedbackTimer);
    feedbackTimer = null;
  }
  if (newVal) {
    feedbackTimer = setTimeout(() => {
      feedbackMsg.value = null;
      feedbackTimer = null;
    }, 3000);
  }
});

// Presets & History
const presets = ref<OTelPreset[]>([]);
const showPresetModal = ref(false);
const showHistoryModal = ref(false);
const historyList = ref<OTelHistoryItem[]>([]);
const loadingHistory = ref(false);
const previewHistoryItem = ref<OTelHistoryItem | null>(null);

// Host Add/Edit Modal
const showHostModal = ref(false);
const isEditingHost = ref(false);
const showPassword = ref(false);
const testingInModal = ref(false);
const modalTestResult = ref<{ ok: boolean; message: string } | null>(null);
const savingHost = ref(false);

const hostForm = ref({
  id: '',
  name: '',
  tagsInput: '',
  sshHost: '',
  sshPort: 22,
  sshUser: 'root',
  sshAuth: 'password',
  sshPassword: '',
  sshKey: '',
  configPath: '/etc/otelcol-contrib/config.yaml',
  serviceName: 'otelcol-contrib',
  reloadMode: 'restart',
});

// Delete confirmation modal
const showDeleteModal = ref(false);
const hostToDelete = ref<OTelHost | null>(null);
const deletingHost = ref(false);

// Copy feedback
const copied = ref(false);

// Synchronize gutter scrolling
const syncScroll = () => {
  if (editorRef.value && gutterRef.value) {
    gutterRef.value.scrollTop = editorRef.value.scrollTop;
  }
};

// Tab key handling in textarea
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

// Line numbers generator
const lineNumbers = computed(() => {
  if (!yamlContent.value) return Array.from({ length: 25 }, (_, i) => i + 1);
  const count = yamlContent.value.split('\n').length;
  return Array.from({ length: Math.max(count, 25) }, (_, i) => i + 1);
});

// Real-time YAML Syntax Validation
const yamlValidation = computed(() => {
  if (!yamlContent.value || !yamlContent.value.trim()) {
    return { valid: true, error: null };
  }

  const lines = yamlContent.value.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    // Tab characters check
    if (line.includes('\t')) {
      return { valid: false, error: `Line ${i + 1}: Tab characters are forbidden in YAML. Use spaces.` };
    }
    // Basic colon spacing check
    const trimmed = line.trim();
    if (trimmed && !trimmed.startsWith('#') && !trimmed.startsWith('-')) {
      const colonIdx = trimmed.indexOf(':');
      if (colonIdx > 0 && colonIdx < trimmed.length - 1) {
        const afterColon = trimmed[colonIdx + 1];
        if (afterColon !== ' ' && afterColon !== '\n' && afterColon !== '\r') {
          // Could be url like http:// or port :8080, check if key:value
          if (!trimmed.includes('://')) {
            return { valid: false, error: `Line ${i + 1}: Missing space after colon in key-value mapping.` };
          }
        }
      }
    }
  }

  return { valid: true, error: null };
});

// Filtered host list
const filteredHosts = computed(() => {
  let list = hosts.value;

  if (statusFilter.value !== 'all') {
    list = list.filter((h) => h.lastStatus === statusFilter.value);
  }

  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim();
    list = list.filter((h) => {
      const matchName = (h.name || '').toLowerCase().includes(q);
      const matchHost = (h.sshHost || '').toLowerCase().includes(q);
      const matchService = (h.serviceName || '').toLowerCase().includes(q);
      const matchTags = Array.isArray(h.tags) && h.tags.some((t) => t.toLowerCase().includes(q));
      return matchName || matchHost || matchService || matchTags;
    });
  }

  return list;
});

// Status counts
const hostCounts = computed(() => {
  return {
    all: hosts.value.length,
    active: hosts.value.filter((h) => h.lastStatus === 'active').length,
    failed: hosts.value.filter((h) => h.lastStatus === 'failed').length,
    unreachable: hosts.value.filter((h) => h.lastStatus === 'unreachable').length,
  };
});

// Has unsaved edits
const hasUnsavedChanges = computed(() => {
  return yamlContent.value !== initialLoadedContent.value;
});

// ==================== API ACTIONS ====================

const fetchHosts = async (autoSelectFirst = false) => {
  loadingHosts.value = true;
  try {
    const res = await axios.get('/api/v1/otel/hosts');
    if (res.data?.success && Array.isArray(res.data.data)) {
      hosts.value = res.data.data;

      // Auto-select host
      if (hosts.value.length > 0) {
        if (!selectedHostId.value || !hosts.value.some((h) => h.id === selectedHostId.value)) {
          if (autoSelectFirst || !selectedHostId.value) {
            selectHost(hosts.value[0].id);
          }
        }
      } else {
        selectedHostId.value = '';
        yamlContent.value = '';
        initialLoadedContent.value = '';
      }
    }
  } catch (err: any) {
    feedbackMsg.value = {
      type: 'error',
      title: 'Failed to load hosts',
      detail: err.response?.data?.error || err.message,
    };
  } finally {
    loadingHosts.value = false;
  }
};

const selectHost = async (hostId: string) => {
  if (selectedHostId.value === hostId && yamlContent.value) return;
  selectedHostId.value = hostId;
  feedbackMsg.value = null;
  await loadHostConfig(hostId);
};

const loadHostConfig = async (hostId: string) => {
  loadingConfig.value = true;
  feedbackMsg.value = null;
  try {
    const res = await axios.get(`/api/v1/otel/hosts/${hostId}/config`);
    if (res.data?.success) {
      yamlContent.value = res.data.data || '';
      initialLoadedContent.value = yamlContent.value;
    } else {
      yamlContent.value = '';
      initialLoadedContent.value = '';
      feedbackMsg.value = {
        type: 'error',
        title: 'Failed to fetch remote config',
        detail: res.data?.error || 'Unknown error',
      };
    }
  } catch (err: any) {
    yamlContent.value = '';
    initialLoadedContent.value = '';
    feedbackMsg.value = {
      type: 'error',
      title: 'Failed to read config from host',
      detail: err.response?.data?.error || err.message,
    };
  } finally {
    loadingConfig.value = false;
  }
};

const saveConfig = async () => {
  if (!selectedHost.value) return;
  if (!yamlValidation.value.valid) {
    feedbackMsg.value = {
      type: 'error',
      title: 'Invalid YAML Syntax',
      detail: yamlValidation.value.error || 'Please fix syntax errors before deploying.',
    };
    return;
  }

  saving.value = true;
  feedbackMsg.value = null;

  try {
    const res = await axios.post(`/api/v1/otel/hosts/${selectedHost.value.id}/config`, {
      content: yamlContent.value,
      summary: configSummary.value || undefined,
      restartAfter: true,
      restart: true,
    });

    if (res.data?.success) {
      const data = res.data.data;
      initialLoadedContent.value = yamlContent.value;
      configSummary.value = '';

      if (data?.restart?.attempted) {
        if (data.restart.success) {
          feedbackMsg.value = {
            type: 'success',
            title: 'Configuration Deployed & Agent Running',
            detail: res.data.message || 'Config saved and OpenTelemetry Collector service restarted successfully.',
          };
          updateHostStatusInList(selectedHost.value.id, 'active');
        } else if (data.restart && !data.restart.success) {
          feedbackMsg.value = {
            type: 'error',
            title: 'Config Saved, but Service Restart Failed!',
            detail: data.restart.error || data.restart.output || 'Service could not be restarted.',
          };
          updateHostStatusInList(selectedHost.value.id, 'failed');
        }
      } else {
        feedbackMsg.value = {
          type: 'success',
          title: 'Configuration Saved to Remote Host',
          detail: res.data.message,
        };
      }
    }
  } catch (err: any) {
    feedbackMsg.value = {
      type: 'error',
      title: 'Failed to deploy configuration',
      detail: err.response?.data?.error || err.message,
    };
  } finally {
    saving.value = false;
  }
};


const fetchLiveStatus = async (hostId: string) => {
  try {
    const res = await axios.get(`/api/v1/otel/hosts/${hostId}/status`);
    if (res.data?.success && res.data.data) {
      const data = res.data.data;
      if (data.serviceStatus) {
        updateHostStatusInList(hostId, data.serviceStatus);
      }
    }
  } catch (err) {
    // Ignore silent error
  }
};

const restartCurrentService = async (mode: 'restart' | 'reload' = 'restart') => {
  if (!selectedHost.value) return;
  restartingService.value = true;
  feedbackMsg.value = null;

  try {
    const res = await axios.post(`/api/v1/otel/hosts/${selectedHost.value.id}/restart`, { mode });
    if (res.data?.success) {
      feedbackMsg.value = {
        type: 'success',
        title: `Service ${mode === 'reload' ? 'Reloaded' : 'Restarted'} Successfully`,
        detail: `OpenTelemetry agent on ${selectedHost.value.name} is healthy and active.`,
      };
      updateHostStatusInList(selectedHost.value.id, 'active');
    } else {
      feedbackMsg.value = {
        type: 'error',
        title: `Service ${mode} Failed`,
        detail: res.data.data?.error || res.data.data?.output || 'Failed to restart service on remote host.',
      };
      updateHostStatusInList(selectedHost.value.id, 'failed');
    }
  } catch (err: any) {
    feedbackMsg.value = {
      type: 'error',
      title: 'Restart Error',
      detail: err.response?.data?.error || err.message,
    };
  } finally {
    restartingService.value = false;
  }
};

const fetchPresets = async () => {
  try {
    const res = await axios.get('/api/v1/otel/presets');
    if (res.data?.success && Array.isArray(res.data.data)) {
      presets.value = res.data.data;
    }
  } catch (err) {
    // Presets fallback if backend not ready
  }
};

const applyPreset = (preset: OTelPreset) => {
  if (hasUnsavedChanges.value) {
    if (!confirm('You have unsaved changes. Overwrite current editor content with preset template?')) {
      return;
    }
  }
  yamlContent.value = preset.content;
  showPresetModal.value = false;
  feedbackMsg.value = {
    type: 'info',
    title: `Applied Preset: ${preset.name}`,
    detail: 'Review the pipeline configuration and click "Deploy & Restart" to push to remote agent.',
  };
};

const fetchHistory = async () => {
  if (!selectedHost.value) return;
  loadingHistory.value = true;
  previewHistoryItem.value = null;
  try {
    const res = await axios.get(`/api/v1/otel/hosts/${selectedHost.value.id}/history`);
    if (res.data?.success && Array.isArray(res.data.data)) {
      historyList.value = res.data.data;
      if (historyList.value.length > 0) {
        previewHistoryItem.value = historyList.value[0];
      }
    }
  } catch (err) {
    historyList.value = [];
  } finally {
    loadingHistory.value = false;
  }
};

const openHistoryModal = async () => {
  showHistoryModal.value = true;
  await fetchHistory();
};

const restoreHistoryVersion = (item: OTelHistoryItem) => {
  if (confirm(`Restore configuration from ${formatDate(item.createdAt)}? This will replace your current editor content.`)) {
    yamlContent.value = item.content;
    showHistoryModal.value = false;
    feedbackMsg.value = {
      type: 'info',
      title: 'Previous Version Restored to Editor',
      detail: `Restored snapshot from ${formatDate(item.createdAt)}. Remember to click "Deploy & Restart" to apply.`,
    };
  }
};

// ==================== HOST MODAL (ADD / EDIT) ====================

const openAddHostModal = () => {
  isEditingHost.value = false;
  showPassword.value = false;
  modalTestResult.value = null;
  hostForm.value = {
    id: '',
    name: '',
    tagsInput: '',
    sshHost: '',
    sshPort: 22,
    sshUser: 'root',
    sshAuth: 'password',
    sshPassword: '',
    sshKey: '',
    configPath: '/etc/otelcol-contrib/config.yaml',
    serviceName: 'otelcol-contrib',
    reloadMode: 'restart',
  };
  showHostModal.value = true;
};

const openEditHostModal = (host: OTelHost) => {
  isEditingHost.value = true;
  showPassword.value = false;
  modalTestResult.value = null;
  hostForm.value = {
    id: host.id,
    name: host.name,
    tagsInput: Array.isArray(host.tags) ? host.tags.join(', ') : '',
    sshHost: host.sshHost,
    sshPort: host.sshPort || 22,
    sshUser: host.sshUser || 'root',
    sshAuth: host.sshAuth || 'password',
    sshPassword: '', // keep empty to leave unchanged
    sshKey: '',
    configPath: host.configPath || '/etc/otelcol-contrib/config.yaml',
    serviceName: host.serviceName || 'otelcol-contrib',
    reloadMode: host.reloadMode || 'restart',
  };
  showHostModal.value = true;
};

const testConnectionInModal = async () => {
  if (!hostForm.value.sshHost || !hostForm.value.name) {
    modalTestResult.value = { ok: false, message: 'Please specify Host Name and SSH IP before testing.' };
    return;
  }

  testingInModal.value = true;
  modalTestResult.value = null;

  try {
    const payload: any = {
      id: hostForm.value.id || undefined,
      name: hostForm.value.name,
      sshHost: hostForm.value.sshHost,
      sshPort: Number(hostForm.value.sshPort) || 22,
      sshUser: hostForm.value.sshUser || 'root',
      sshAuth: hostForm.value.sshAuth,
      serviceName: hostForm.value.serviceName,
      configPath: hostForm.value.configPath,
    };

    if (hostForm.value.sshAuth === 'password' && hostForm.value.sshPassword) {
      payload.sshPassword = hostForm.value.sshPassword;
    } else if (hostForm.value.sshAuth === 'key' && hostForm.value.sshKey) {
      payload.sshKey = hostForm.value.sshKey;
    }

    const res = await axios.post('/api/v1/otel/hosts/test', payload);
    if (res.data?.success) {
      modalTestResult.value = { ok: true, message: res.data.message };
    } else {
      modalTestResult.value = { ok: false, message: res.data.message || 'Connection test failed' };
    }
  } catch (err: any) {
    modalTestResult.value = {
      ok: false,
      message: err.response?.data?.message || err.response?.data?.error || err.message,
    };
  } finally {
    testingInModal.value = false;
  }
};

const saveHostForm = async () => {
  if (!hostForm.value.name.trim() || !hostForm.value.sshHost.trim()) {
    modalTestResult.value = { ok: false, message: 'Profile name and SSH IP are required.' };
    return;
  }

  savingHost.value = true;
  modalTestResult.value = null;

  const tags = hostForm.value.tagsInput
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);

  const payload: any = {
    id: hostForm.value.id || undefined,
    name: hostForm.value.name.trim(),
    tags: tags,
    sshHost: hostForm.value.sshHost.trim(),
    sshPort: Number(hostForm.value.sshPort) || 22,
    sshUser: hostForm.value.sshUser.trim() || 'root',
    sshAuth: hostForm.value.sshAuth,
    configPath: hostForm.value.configPath.trim() || '/etc/otelcol-contrib/config.yaml',
    serviceName: hostForm.value.serviceName.trim() || 'otelcol-contrib',
    reloadMode: hostForm.value.reloadMode,
    isActive: true,
  };

  if (hostForm.value.sshAuth === 'password' && hostForm.value.sshPassword) {
    payload.sshPassword = hostForm.value.sshPassword;
  }
  if (hostForm.value.sshAuth === 'key' && hostForm.value.sshKey) {
    payload.sshKey = hostForm.value.sshKey;
  }

  try {
    const res = await axios.post('/api/v1/otel/hosts', payload);
    if (res.data?.success) {
      showHostModal.value = false;
      await fetchHosts();
      if (res.data.data?.id) {
        selectHost(res.data.data.id);
      }
      feedbackMsg.value = {
        type: 'success',
        title: isEditingHost.value ? 'Host Profile Updated' : 'New OpenTelemetry Host Added',
        detail: `Host ${payload.name} (${payload.sshHost}) is ready for configuration management.`,
      };
    }
  } catch (err: any) {
    modalTestResult.value = {
      ok: false,
      message: err.response?.data?.error || err.message || 'Failed to save host configuration.',
    };
  } finally {
    savingHost.value = false;
  }
};

// ==================== DELETE HOST ====================

const confirmDeleteHost = (host: OTelHost) => {
  hostToDelete.value = host;
  showDeleteModal.value = true;
};

const executeDeleteHost = async () => {
  if (!hostToDelete.value) return;
  deletingHost.value = true;
  try {
    const res = await axios.delete(`/api/v1/otel/hosts/${hostToDelete.value.id}`);
    if (res.data?.success) {
      showDeleteModal.value = false;
      const deletedId = hostToDelete.value.id;
      hostToDelete.value = null;
      await fetchHosts(true);
      if (selectedHostId.value === deletedId) {
        selectedHostId.value = hosts.value.length > 0 ? hosts.value[0].id : '';
      }
      feedbackMsg.value = {
        type: 'info',
        title: 'Host Removed',
        detail: 'OpenTelemetry host profile deleted.',
      };
    }
  } catch (err: any) {
    feedbackMsg.value = {
      type: 'error',
      title: 'Failed to delete host',
      detail: err.response?.data?.error || err.message,
    };
  } finally {
    deletingHost.value = false;
  }
};

// Copy config helper
const copyConfigToClipboard = () => {
  if (!yamlContent.value) return;
  navigator.clipboard.writeText(yamlContent.value).then(() => {
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  });
};

const updateHostStatusInList = (hostId: string, status: string) => {
  const h = hosts.value.find((item) => item.id === hostId);
  if (h) {
    h.lastStatus = status;
  }
};

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-';
  try {
    const d = new Date(dateStr);
    return d.toLocaleString();
  } catch {
    return dateStr;
  }
};

onMounted(async () => {
  await fetchHosts(true);
  await fetchPresets();
});
</script>

<template>
  <div class="p-4 sm:p-6 space-y-5 max-w-[1600px] mx-auto min-h-screen">
    <!-- Header Section -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          OpenTelemetry Remote Config
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Manage, validate, and hot-reload OpenTelemetry Collector agents across your multi-host infrastructure.
        </p>
      </div>

      <!-- Header Action Buttons -->
      <div class="flex items-center gap-2 shrink-0">
        <button
          @click="fetchHosts(false)"
          :disabled="loadingHosts"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white dark:bg-[#121826] hover:bg-slate-50 dark:hover:bg-[#1a2336] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#1b2234] transition shadow-xs cursor-pointer disabled:opacity-50"
          title="Refresh Host Fleet"
        >
          <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loadingHosts }" />
          <span class="hidden sm:inline">Refresh</span>
        </button>

        <button
          v-if="canManage"
          @click="openAddHostModal"
          class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 active:scale-98 text-white text-xs font-bold transition shadow-sm shadow-blue-500/20 cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5 stroke-[2.5]" />
          <span>Add OTel Host</span>
        </button>
      </div>
    </div>

    <!-- Alert / Feedback Notification Banner -->
    <div
      v-if="feedbackMsg"
      :class="[
        feedbackMsg.type === 'success' ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400' : '',
        feedbackMsg.type === 'error' ? 'bg-rose-500/10 border-rose-500/30 text-rose-600 dark:text-rose-400' : '',
        feedbackMsg.type === 'info' ? 'bg-blue-500/10 border-blue-500/30 text-blue-600 dark:text-blue-400' : '',
        'flex items-start justify-between p-3.5 rounded-xl border text-xs shadow-xs transition animate-in fade-in'
      ]"
    >
      <div class="flex items-start gap-2.5">
        <component
          :is="feedbackMsg.type === 'success' ? CheckCircle2 : feedbackMsg.type === 'error' ? AlertTriangle : Info"
          class="w-4 h-4 shrink-0 mt-0.5"
        />
        <div>
          <p class="font-bold">{{ feedbackMsg.title }}</p>
          <p v-if="feedbackMsg.detail" class="mt-0.5 opacity-90 leading-relaxed font-mono text-[11px] whitespace-pre-wrap">{{ feedbackMsg.detail }}</p>
        </div>
      </div>
      <button @click="feedbackMsg = null" class="opacity-60 hover:opacity-100 p-1 cursor-pointer">
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Master-Detail 2-Column Layout -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
      <!-- LEFT COLUMN: Host List & Search (4 cols) -->
      <div class="lg:col-span-4 xl:col-span-3 space-y-3">
        <!-- Host List Panel Card -->
        <div class="bg-white dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-2xl shadow-sm overflow-hidden flex flex-col">
          <!-- List Card Header with Counts -->
          <div class="p-3.5 border-b border-slate-200 dark:border-[#1b2234] space-y-2.5 bg-slate-50/50 dark:bg-[#121826]/40">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-bold tracking-wider uppercase text-slate-500 dark:text-slate-400 flex items-center gap-1.5">
                <Server class="w-3.5 h-3.5 text-blue-500" />
                <span>Collector Fleet</span>
              </span>
              <span class="text-[10px] font-mono px-2 py-0.5 rounded-full bg-slate-200 dark:bg-[#1b2234] text-slate-700 dark:text-slate-300 font-bold">
                {{ hosts.length }} Hosts
              </span>
            </div>

            <!-- Search input -->
            <div class="relative">
              <Search class="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                v-model="searchQuery"
                type="text"
                placeholder="Filter host, IP, or tag..."
                class="w-full pl-8 pr-3 py-1.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs text-slate-800 dark:text-slate-200 placeholder-slate-400 focus:outline-none focus:border-blue-500 transition"
              />
              <button
                v-if="searchQuery"
                @click="searchQuery = ''"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-white"
              >
                <X class="w-3 h-3" />
              </button>
            </div>

            <!-- Status Filter Chips -->
            <div class="flex items-center gap-1.5 text-[10px] overflow-x-auto pb-0.5 font-medium scrollbar-none">
              <button
                @click="statusFilter = 'all'"
                :class="[
                  statusFilter === 'all'
                    ? 'bg-blue-600 text-white font-bold'
                    : 'bg-slate-200/70 dark:bg-[#192132] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                  'px-2 py-0.5 rounded-md transition cursor-pointer shrink-0'
                ]"
              >
                All ({{ hostCounts.all }})
              </button>
              <button
                @click="statusFilter = 'active'"
                :class="[
                  statusFilter === 'active'
                    ? 'bg-emerald-600 text-white font-bold'
                    : 'bg-slate-200/70 dark:bg-[#192132] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                  'px-2 py-0.5 rounded-md transition cursor-pointer shrink-0'
                ]"
              >
                Active ({{ hostCounts.active }})
              </button>
              <button
                @click="statusFilter = 'failed'"
                :class="[
                  statusFilter === 'failed'
                    ? 'bg-rose-600 text-white font-bold'
                    : 'bg-slate-200/70 dark:bg-[#192132] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                  'px-2 py-0.5 rounded-md transition cursor-pointer shrink-0'
                ]"
              >
                Failed ({{ hostCounts.failed }})
              </button>
              <button
                @click="statusFilter = 'unreachable'"
                :class="[
                  statusFilter === 'unreachable'
                    ? 'bg-slate-600 text-white font-bold'
                    : 'bg-slate-200/70 dark:bg-[#192132] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                  'px-2 py-0.5 rounded-md transition cursor-pointer shrink-0'
                ]"
              >
                Offline ({{ hostCounts.unreachable }})
              </button>
            </div>
          </div>

          <!-- Host Card List Container -->
          <div class="max-h-[640px] overflow-y-auto divide-y divide-slate-100 dark:divide-[#171d2b] p-1.5 space-y-1">
            <!-- Empty search result -->
            <div v-if="filteredHosts.length === 0 && !loadingHosts" class="p-6 text-center space-y-2 text-xs text-slate-400">
              <Server class="w-6 h-6 mx-auto text-slate-300 dark:text-slate-600" />
              <p class="font-medium text-slate-700 dark:text-slate-300">No hosts found</p>
              <p class="text-[11px] text-slate-500">
                {{ hosts.length === 0 ? 'Click "+ Add OTel Host" to register your first OpenTelemetry agent.' : 'Try adjusting your search filter.' }}
              </p>
              <button
                v-if="hosts.length === 0 && canManage"
                @click="openAddHostModal"
                class="mt-2 inline-flex items-center gap-1 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg transition"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>Add First Host</span>
              </button>
            </div>

            <!-- Host Cards -->
            <div
              v-for="host in filteredHosts"
              :key="host.id"
              @click="selectHost(host.id)"
              :class="[
                selectedHostId === host.id
                  ? 'bg-blue-50 dark:bg-[#19243d] border-blue-300 dark:border-blue-500/50 shadow-xs'
                  : 'bg-transparent hover:bg-slate-50 dark:hover:bg-[#121826]/70 border-transparent',
                'p-2.5 rounded-xl border transition cursor-pointer text-left space-y-1.5 group'
              ]"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-center gap-2 min-w-0">
                  <!-- Status pulsing dot -->
                  <span class="relative flex h-2.5 w-2.5 shrink-0">
                    <span
                      v-if="host.lastStatus === 'active'"
                      class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"
                    ></span>
                    <span
                      :class="[
                        host.lastStatus === 'active' ? 'bg-emerald-500' : '',
                        host.lastStatus === 'failed' ? 'bg-rose-500' : '',
                        host.lastStatus === 'inactive' ? 'bg-amber-500' : '',
                        host.lastStatus === 'unreachable' ? 'bg-slate-400' : '',
                        host.lastStatus === 'unknown' ? 'bg-slate-500' : '',
                        'relative inline-flex rounded-full h-2.5 w-2.5'
                      ]"
                    ></span>
                  </span>

                  <h3 class="font-bold text-xs text-slate-900 dark:text-white truncate">
                    {{ host.name }}
                  </h3>
                </div>

                <!-- Status pill -->
                <span
                  :class="[
                    host.lastStatus === 'active' ? 'text-emerald-700 dark:text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : '',
                    host.lastStatus === 'failed' ? 'text-rose-700 dark:text-rose-400 bg-rose-500/10 border-rose-500/20' : '',
                    host.lastStatus === 'inactive' ? 'text-amber-700 dark:text-amber-400 bg-amber-500/10 border-amber-500/20' : '',
                    host.lastStatus === 'unreachable' ? 'text-slate-600 dark:text-slate-400 bg-slate-500/10 border-slate-500/20' : '',
                    host.lastStatus === 'unknown' ? 'text-slate-600 dark:text-slate-400 bg-slate-500/10 border-slate-500/20' : '',
                    'text-[9px] font-mono font-bold uppercase px-1.5 py-0.5 rounded border shrink-0'
                  ]"
                >
                  {{ host.lastStatus }}
                </span>
              </div>

              <!-- Host Address & Service Details -->
              <div class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                <span class="truncate">{{ host.sshUser }}@{{ host.sshHost }}:{{ host.sshPort }}</span>
                <span class="text-[10px] text-slate-400 font-sans shrink-0">{{ host.serviceName }}</span>
              </div>

              <!-- Tags list if any -->
              <div v-if="host.tags && host.tags.length > 0" class="flex flex-wrap gap-1 pt-0.5">
                <span
                  v-for="tag in host.tags"
                  :key="tag"
                  class="text-[9px] px-1.5 py-0.2 rounded bg-slate-100 dark:bg-[#1e2738] text-slate-600 dark:text-slate-300 font-medium"
                >
                  #{{ tag }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- RIGHT COLUMN: YAML Config Editor & Host Controls (8 cols) -->
      <div class="lg:col-span-8 xl:col-span-9 space-y-4">
        <!-- IF NO HOST SELECTED OR NO HOSTS REGISTERED -->
        <div
          v-if="!selectedHost"
          class="p-12 text-center bg-white dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-2xl space-y-4 shadow-sm"
        >
          <div class="w-14 h-14 rounded-2xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-500 mx-auto">
            <Radio class="w-7 h-7" />
          </div>
          <div class="space-y-1">
            <h2 class="text-base font-bold text-slate-900 dark:text-white">No OpenTelemetry Host Selected</h2>
            <p class="text-xs text-slate-500 max-w-md mx-auto">
              Select an OpenTelemetry Collector host from the left fleet panel or register a new host to manage its pipeline configuration remotely.
            </p>
          </div>
          <button
            v-if="canManage"
            @click="openAddHostModal"
            class="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg transition cursor-pointer shadow-sm shadow-blue-500/20"
          >
            <Plus class="w-4 h-4 stroke-[2.5]" />
            <span>Add OpenTelemetry Host</span>
          </button>
        </div>

        <!-- ACTIVE SELECTED HOST PANEL -->
        <div v-else class="space-y-4">
          <!-- Host Header & Toolbar Card -->
          <div class="bg-white dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-2xl p-4 shadow-sm space-y-3">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <!-- Left side: Host identity and path -->
              <div class="space-y-1">
                <div class="flex items-center gap-2 flex-wrap">
                  <h2 class="text-base font-black text-slate-900 dark:text-white">
                    {{ selectedHost.name }}
                  </h2>
                  <span class="text-xs font-mono px-2 py-0.5 rounded-md bg-slate-100 dark:bg-[#192233] text-slate-700 dark:text-slate-300 font-semibold border border-slate-200 dark:border-[#1b2234]">
                    {{ selectedHost.sshUser }}@{{ selectedHost.sshHost }}:{{ selectedHost.sshPort }}
                  </span>
                  <span
                    :class="[
                      selectedHost.lastStatus === 'active' ? 'text-emerald-700 dark:text-emerald-400 bg-emerald-500/10 border-emerald-500/30' : '',
                      selectedHost.lastStatus === 'failed' ? 'text-rose-700 dark:text-rose-400 bg-rose-500/10 border-rose-500/30' : '',
                      selectedHost.lastStatus === 'inactive' ? 'text-amber-700 dark:text-amber-400 bg-amber-500/10 border-amber-500/30' : '',
                      selectedHost.lastStatus === 'unreachable' ? 'text-slate-600 dark:text-slate-400 bg-slate-500/10 border-slate-500/30' : '',
                      selectedHost.lastStatus === 'unknown' ? 'text-slate-600 dark:text-slate-400 bg-slate-500/10 border-slate-500/30' : '',
                      'text-[10px] font-mono font-bold uppercase px-2 py-0.5 rounded-md border flex items-center gap-1.5'
                    ]"
                  >
                    <span
                      :class="[
                        selectedHost.lastStatus === 'active' ? 'bg-emerald-500' : 'bg-slate-400',
                        'w-1.5 h-1.5 rounded-full'
                      ]"
                    ></span>
                    <span>{{ selectedHost.lastStatus }}</span>
                  </span>
                </div>

                <div class="flex items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
                  <span class="font-mono text-[11px] text-blue-700 dark:text-blue-300">{{ selectedHost.configPath }}</span>
                  <span>•</span>
                  <span>Service: <strong class="text-slate-800 dark:text-slate-200">{{ selectedHost.serviceName }}</strong></span>
                  <span>•</span>
                  <span>Mode: <strong class="text-slate-800 dark:text-slate-200 uppercase">{{ selectedHost.reloadMode }}</strong></span>
                </div>
              </div>

              <!-- Right side: Host Quick Action Buttons -->
              <div class="flex items-center gap-1.5 flex-wrap">

                <!-- Restart Service Button -->
                <button
                  v-if="canManage"
                  @click="restartCurrentService('restart')"
                  :disabled="restartingService"
                  class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#161d2c] dark:hover:bg-[#1f2a3f] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#243046] transition cursor-pointer disabled:opacity-50"
                  title="Restart systemd service immediately"
                >
                  <Play class="w-3.5 h-3.5 text-amber-500 fill-amber-500/20" :class="{ 'animate-spin': restartingService }" />
                  <span>Restart Agent</span>
                </button>

                <!-- Presets dropdown button -->
                <button
                  v-if="canManage"
                  @click="showPresetModal = true"
                  class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#161d2c] dark:hover:bg-[#1f2a3f] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#243046] transition cursor-pointer"
                  title="Choose from pre-configured pipeline presets"
                >
                  <Layers class="w-3.5 h-3.5 text-blue-500" />
                  <span>Presets</span>
                </button>

                <!-- History / Backups Button -->
                <button
                  @click="openHistoryModal"
                  class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#161d2c] dark:hover:bg-[#1f2a3f] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#243046] transition cursor-pointer"
                  title="View previous version backups and rollback"
                >
                  <History class="w-3.5 h-3.5 text-purple-500" />
                  <span>History</span>
                </button>

                <!-- Edit Host Profile Button -->
                <button
                  v-if="canManage"
                  @click="openEditHostModal(selectedHost)"
                  class="p-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#161d2c] dark:hover:bg-[#1f2a3f] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-[#243046] transition cursor-pointer"
                  title="Edit host SSH settings & paths"
                >
                  <Edit3 class="w-3.5 h-3.5" />
                </button>

                <!-- Delete Host Profile Button -->
                <button
                  v-if="canManage"
                  @click="confirmDeleteHost(selectedHost)"
                  class="p-1.5 rounded-lg bg-rose-50 hover:bg-rose-100 dark:bg-rose-950/40 dark:hover:bg-rose-900/60 text-rose-600 dark:text-rose-400 border border-rose-200 dark:border-rose-900/60 transition cursor-pointer"
                  title="Delete host profile"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>

          <!-- Code Editor Card -->
          <div class="bg-white dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-2xl overflow-hidden shadow-sm flex flex-col">
            <!-- Editor Top Sub-bar: Status and tools -->
            <div class="flex items-center justify-between px-4 py-2.5 border-b border-slate-200 dark:border-[#1b2234] bg-slate-50 dark:bg-[#121826] text-xs">
              <div class="flex items-center gap-2">
                <FileCode class="w-4 h-4 text-blue-500" />
                <span class="font-mono font-bold text-slate-800 dark:text-slate-200 text-[11px]">
                  {{ selectedHost.configPath }}
                </span>
                <span v-if="hasUnsavedChanges" class="text-[10px] font-semibold px-2 py-0.2 rounded-full bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
                  Unsaved Changes
                </span>
                <span v-if="!canManage" class="text-[10px] font-mono font-bold px-2 py-0.2 rounded bg-slate-200 dark:bg-[#1c2436] text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-[#28354f]">
                  READ ONLY
                </span>
              </div>

              <!-- YAML Validation Status Indicator -->
              <div class="flex items-center gap-3">
                <div v-if="yamlValidation.valid" class="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400 text-[11px] font-medium font-mono">
                  <CheckCircle2 class="w-3.5 h-3.5" />
                  <span>Valid YAML</span>
                </div>
                <div v-else class="flex items-center gap-1.5 text-rose-600 dark:text-rose-400 text-[11px] font-medium font-mono">
                  <AlertTriangle class="w-3.5 h-3.5 shrink-0" />
                  <span class="truncate max-w-[280px]" :title="yamlValidation.error || ''">{{ yamlValidation.error }}</span>
                </div>

                <!-- Copy button -->
                <button
                  @click="copyConfigToClipboard"
                  class="flex items-center gap-1 px-2 py-1 rounded bg-slate-200/60 dark:bg-[#1c2436] hover:bg-slate-300 dark:hover:bg-[#253047] text-slate-700 dark:text-slate-300 text-[11px] font-semibold transition cursor-pointer"
                  title="Copy YAML to clipboard"
                >
                  <component :is="copied ? Check : Copy" class="w-3 h-3" />
                  <span>{{ copied ? 'Copied' : 'Copy' }}</span>
                </button>
              </div>
            </div>

            <!-- Loading overlay or Editor body -->
            <div class="relative bg-white dark:bg-[#090d16] font-mono text-xs">
              <div v-if="loadingConfig" class="absolute inset-0 bg-white/80 dark:bg-slate-900/80 backdrop-blur-xs z-10 flex flex-col items-center justify-center gap-2 text-slate-600 dark:text-slate-400">
                <RotateCw class="w-6 h-6 animate-spin text-blue-500" />
                <span class="text-xs font-semibold">Reading configuration from {{ selectedHost.sshHost }}...</span>
              </div>

              <div class="flex min-h-[460px] max-h-[580px]">
                <!-- Line numbers gutter -->
                <div
                  ref="gutterRef"
                  class="w-12 py-3 bg-slate-100 dark:bg-[#070a10] border-r border-slate-200 dark:border-slate-800 text-slate-400 dark:text-slate-500 text-right pr-2.5 select-none overflow-hidden shrink-0 font-mono text-[11px] leading-5"
                >
                  <div v-for="n in lineNumbers" :key="n">{{ n }}</div>
                </div>

                <!-- Textarea Code Editor -->
                <textarea
                  ref="editorRef"
                  v-model="yamlContent"
                  @scroll="syncScroll"
                  @keydown="handleKeyDown"
                  :readonly="!canManage"
                  spellcheck="false"
                  placeholder="# OpenTelemetry Collector configuration YAML"
                  class="flex-1 p-3 bg-white dark:bg-transparent text-slate-800 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none resize-none font-mono text-[11px] leading-5 whitespace-pre tab-2 overflow-y-auto selection:bg-blue-500/20 dark:selection:bg-blue-600/40 outline-none"
                  style="border: none !important; box-shadow: none !important;"
                ></textarea>
              </div>
            </div>

            <!-- Bottom Deployment Action Bar -->
            <div class="p-3.5 border-t border-slate-200 dark:border-[#1b2234] bg-slate-50 dark:bg-[#121826] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <!-- If User can manage: show deployment inputs -->
              <template v-if="canManage">
                <div class="flex-1 flex items-center gap-2">
                  <input
                    v-model="configSummary"
                    type="text"
                    placeholder="Optional commit / change note (e.g. Added Prometheus scrape target)"
                    class="w-full sm:max-w-md px-3 py-1.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs text-slate-800 dark:text-slate-200 placeholder-slate-400 focus:outline-none focus:border-blue-500 transition"
                  />
                </div>

                <div class="flex items-center gap-2 shrink-0">
                  <button
                    @click="saveConfig"
                    :disabled="saving || !yamlValidation.valid"
                    class="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 active:scale-98 text-white text-xs font-bold transition shadow-sm shadow-blue-500/20 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <Save class="w-4 h-4" :class="{ 'animate-spin': saving }" />
                    <span>{{ saving ? 'Deploying & Restarting...' : 'Deploy & Restart Agent' }}</span>
                  </button>
                </div>
              </template>

              <!-- If User only has Read permission: display observer notice -->
              <div v-else class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400 py-1">
                <Shield class="w-4 h-4 text-blue-500 shrink-0" />
                <span>Read-Only mode: You have observer access to OpenTelemetry configurations. Contact an administrator to deploy changes or restart agents.</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== HOST ADD / EDIT MODAL ==================== -->
    <div
      v-if="showHostModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        <!-- Modal Header -->
        <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-[#1b2234] bg-slate-50/50 dark:bg-[#161d2d]/50">
          <div class="flex items-center gap-2.5">
            <div class="p-2 rounded-xl bg-blue-500/10 text-blue-500">
              <Server class="w-4 h-4" />
            </div>
            <div>
              <h2 class="text-sm font-bold text-slate-900 dark:text-white">
                {{ isEditingHost ? 'Edit OpenTelemetry Host' : 'Add OpenTelemetry Host Profile' }}
              </h2>
              <p class="text-[11px] text-slate-500">
                Self-contained SSH connection profile for remote agent configuration.
              </p>
            </div>
          </div>
          <button @click="showHostModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Modal Body Form -->
        <div class="p-5 space-y-4 overflow-y-auto flex-1 text-xs">
          <!-- Test result banner in modal -->
          <div
            v-if="modalTestResult"
            :class="[
              modalTestResult.ok ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-600 dark:text-rose-400',
              'p-2.5 rounded-lg border text-[11px] font-mono leading-relaxed'
            ]"
          >
            {{ modalTestResult.message }}
          </div>

          <!-- Basic Info -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Profile Name *</label>
              <input
                v-model="hostForm.name"
                placeholder="e.g. k8s-worker-01 or prod-db"
                class="w-full px-3 py-1.5 bg-slate-50 dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs focus:outline-none focus:border-blue-500"
              />
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Tags (comma separated)</label>
              <input
                v-model="hostForm.tagsInput"
                placeholder="e.g. prod, kubernetes, gateway"
                class="w-full px-3 py-1.5 bg-slate-50 dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <!-- SSH Connection Settings -->
          <div class="p-3 bg-slate-50 dark:bg-[#0e121d] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-3">
            <h3 class="text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 flex items-center gap-1.5">
              <Key class="w-3.5 h-3.5 text-blue-500" />
              <span>SSH Connection Credentials</span>
            </h3>

            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div class="sm:col-span-2 space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">SSH Host / IP *</label>
                <input
                  v-model="hostForm.sshHost"
                  placeholder="e.g. 192.168.1.100 or node1.internal"
                  class="w-full px-3 py-1.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs font-mono focus:outline-none focus:border-blue-500"
                />
              </div>

              <div class="space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">SSH Port</label>
                <input
                  v-model.number="hostForm.sshPort"
                  type="number"
                  placeholder="22"
                  class="w-full px-3 py-1.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs font-mono focus:outline-none focus:border-blue-500"
                />
              </div>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">SSH Username</label>
                <input
                  v-model="hostForm.sshUser"
                  placeholder="root"
                  class="w-full px-3 py-1.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs font-mono focus:outline-none focus:border-blue-500"
                />
              </div>

              <div class="space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">Auth Method</label>
                <select
                  v-model="hostForm.sshAuth"
                  class="w-full px-3 py-1.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs focus:outline-none focus:border-blue-500"
                >
                  <option value="password">Password</option>
                  <option value="key">SSH Private Key</option>
                </select>
              </div>
            </div>

            <!-- Password Auth Input -->
            <div v-if="hostForm.sshAuth === 'password'" class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300 flex items-center justify-between">
                <span>SSH Password {{ isEditingHost ? '(Leave empty to keep existing)' : '' }}</span>
              </label>
              <div class="relative">
                <input
                  v-model="hostForm.sshPassword"
                  :type="showPassword ? 'text' : 'password'"
                  placeholder="••••••••"
                  class="w-full px-3 py-1.5 pr-9 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs font-mono focus:outline-none focus:border-blue-500"
                />
                <button
                  type="button"
                  @click="showPassword = !showPassword"
                  class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-white"
                >
                  <component :is="showPassword ? EyeOff : Eye" class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Private Key Input -->
            <div v-else class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">
                SSH Private Key {{ isEditingHost ? '(Leave empty to keep existing)' : '' }}
              </label>
              <textarea
                v-model="hostForm.sshKey"
                rows="3"
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;..."
                class="w-full p-2.5 bg-white dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-[11px] font-mono focus:outline-none focus:border-blue-500 leading-normal"
              ></textarea>
            </div>
          </div>

          <!-- Service & Config Paths -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Config File Path</label>
              <input
                v-model="hostForm.configPath"
                placeholder="/etc/otelcol-contrib/config.yaml"
                class="w-full px-3 py-1.5 bg-slate-50 dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs font-mono focus:outline-none focus:border-blue-500"
              />
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Systemd Service Name</label>
              <input
                v-model="hostForm.serviceName"
                placeholder="otelcol-contrib"
                class="w-full px-3 py-1.5 bg-slate-50 dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div class="space-y-1">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Reload Mode</label>
            <select
              v-model="hostForm.reloadMode"
              class="w-full px-3 py-1.5 bg-slate-50 dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs focus:outline-none focus:border-blue-500"
            >
              <option value="restart">systemctl restart (Recommended - full agent restart)</option>
              <option value="reload">systemctl reload (SIGHUP hot-reload if supported)</option>
            </select>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="flex items-center justify-between p-4 border-t border-slate-200 dark:border-[#1b2234] bg-slate-50/50 dark:bg-[#161d2d]/50">
          <button
            type="button"
            @click="testConnectionInModal"
            :disabled="testingInModal"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white dark:bg-[#121826] hover:bg-slate-100 dark:hover:bg-[#1c2436] text-slate-700 dark:text-slate-200 text-xs font-semibold border border-slate-200 dark:border-[#1b2234] transition cursor-pointer disabled:opacity-50"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': testingInModal }" />
            <span>{{ testingInModal ? 'Testing...' : 'Test Connection' }}</span>
          </button>

          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="showHostModal = false"
              class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="button"
              @click="saveHostForm"
              :disabled="savingHost"
              class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shadow-sm cursor-pointer disabled:opacity-50"
            >
              {{ savingHost ? 'Saving...' : 'Save Host Profile' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== PRESETS MODAL ==================== -->
    <div
      v-if="showPresetModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh]">
        <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-[#1b2234] bg-slate-50/50 dark:bg-[#161d2d]/50">
          <div class="flex items-center gap-2">
            <Layers class="w-4 h-4 text-blue-500" />
            <h2 class="text-sm font-bold text-slate-900 dark:text-white">OpenTelemetry Pipeline Presets</h2>
          </div>
          <button @click="showPresetModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-4 space-y-3 overflow-y-auto flex-1">
          <div
            v-for="preset in presets"
            :key="preset.id"
            class="p-3.5 bg-slate-50 dark:bg-[#0a0d15] border border-slate-200 dark:border-[#1b2234] rounded-xl hover:border-blue-500/50 transition space-y-2"
          >
            <div class="flex items-start justify-between gap-3">
              <div>
                <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-blue-500/10 text-blue-600 dark:text-blue-400 font-semibold">
                  {{ preset.category }}
                </span>
                <h3 class="font-bold text-xs text-slate-900 dark:text-white mt-1">{{ preset.name }}</h3>
                <p class="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">{{ preset.description }}</p>
              </div>
              <button
                @click="applyPreset(preset)"
                class="flex items-center gap-1 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shrink-0 cursor-pointer shadow-xs"
              >
                <span>Load Template</span>
                <ArrowRight class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== HISTORY / ROLLBACK MODAL ==================== -->
    <div
      v-if="showHistoryModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-3xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh]">
        <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-[#1b2234] bg-slate-50/50 dark:bg-[#161d2d]/50">
          <div class="flex items-center gap-2">
            <History class="w-4 h-4 text-purple-500" />
            <h2 class="text-sm font-bold text-slate-900 dark:text-white">Configuration History & Rollbacks</h2>
          </div>
          <button @click="showHistoryModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-12 flex-1 overflow-hidden">
          <!-- Left history list -->
          <div class="md:col-span-5 border-r border-slate-200 dark:border-[#1b2234] overflow-y-auto p-2 space-y-1 max-h-[500px]">
            <div v-if="historyList.length === 0 && !loadingHistory" class="p-6 text-center text-xs text-slate-500">
              No version history recorded yet. Snapshots are created automatically upon every config save.
            </div>
            <div
              v-for="item in historyList"
              :key="item.id"
              @click="previewHistoryItem = item"
              :class="[
                previewHistoryItem?.id === item.id ? 'bg-purple-50 dark:bg-purple-950/40 border-purple-300 dark:border-purple-800' : 'bg-transparent border-transparent hover:bg-slate-50 dark:hover:bg-[#121826]',
                'p-2.5 rounded-xl border transition cursor-pointer text-left space-y-1'
              ]"
            >
              <div class="flex items-center justify-between text-[11px]">
                <span class="font-bold text-slate-800 dark:text-slate-200 font-mono">{{ formatDate(item.createdAt) }}</span>
              </div>
              <p class="text-[11px] text-slate-500 truncate">{{ item.changeSummary || 'Automatic backup before deploy' }}</p>
              <span class="text-[9px] text-slate-400">By {{ item.createdBy || 'system' }}</span>
            </div>
          </div>

          <!-- Right preview container -->
          <div class="md:col-span-7 p-3 flex flex-col bg-slate-50 dark:bg-[#0a0d15] text-slate-800 dark:text-slate-200 font-mono text-[11px] max-h-[500px]">
            <div v-if="previewHistoryItem" class="flex-1 flex flex-col space-y-2 overflow-hidden">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2">
                <span class="text-slate-500 dark:text-slate-400 text-[10px]">Snapshot Preview ({{ previewHistoryItem.content.length }} bytes)</span>
                <button
                  @click="restoreHistoryVersion(previewHistoryItem)"
                  class="px-2.5 py-1 bg-purple-600 hover:bg-purple-500 text-white rounded text-xs font-bold transition cursor-pointer"
                >
                  Restore This Version
                </button>
              </div>
              <textarea
                readonly
                :value="previewHistoryItem.content"
                class="flex-1 w-full bg-white dark:bg-transparent text-slate-800 dark:text-slate-200 resize-none p-2 focus:outline-none text-[11px] leading-relaxed overflow-y-auto whitespace-pre font-mono"
                style="border: none !important; box-shadow: none !important;"
              ></textarea>
            </div>
            <div v-else class="flex-1 flex items-center justify-center text-slate-500 italic">
              Select a version snapshot to preview.
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== DELETE CONFIRMATION MODAL ==================== -->
    <div
      v-if="showDeleteModal && hostToDelete"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Host Profile?</h3>
          <p class="text-xs text-slate-500">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ hostToDelete.name }}</strong> ({{ hostToDelete.sshHost }})? This will not uninstall the agent on the host itself.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDeleteHost"
            :disabled="deletingHost"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deletingHost ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tab-2 {
  tab-size: 2;
}
</style>
