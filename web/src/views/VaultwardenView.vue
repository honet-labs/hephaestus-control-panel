<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import ThemeToggle from '../components/ThemeToggle.vue';
import {
  Shield,
  Key,
  RefreshCw,
  Settings,
  Search,
  Copy,
  Check,
  Eye,
  EyeOff,
  ExternalLink,
  Trash2,
  Folder,
  FileText,
  Lock,
  ArrowLeft,
  X,
  Clock,
  Layers,
  CheckCircle2,
  AlertCircle,
  Plus,
  RotateCcw
} from 'lucide-vue-next';

interface VaultCredentialItem {
  id: string;
  folderId?: string;
  folderName?: string;
  name: string;
  type: number;
  typeLabel: string;
  username: string;
  password?: string;
  notes?: string;
  uris?: string[];
  totp?: string;
  revisionDate: string;
}

interface VaultwardenConfig {
  id: string;
  name: string;
  serverUrl: string;
  email: string;
  isActive: boolean;
  lastSyncedAt?: string;
  cachedCiphers?: VaultCredentialItem[];
  createdAt: string;
  updatedAt: string;
}

// State
const loading = ref(true);
const syncing = ref(false);
const saving = ref(false);
const testing = ref(false);
const deleting = ref(false);

const isConfigured = ref(false);
const config = ref<VaultwardenConfig | null>(null);
const ciphers = ref<VaultCredentialItem[]>([]);

// Filter & Search
const searchKeyword = ref('');
const selectedFolder = ref('all');
const selectedType = ref('all');

// UI Controls
const showConfigModal = ref(false);
const showDeleteModal = ref(false);
const showDetailModal = ref(false);
const showAddModal = ref(false);
const showDeleteCipherModal = ref(false);
const selectedItem = ref<VaultCredentialItem | null>(null);
const cipherToDelete = ref<VaultCredentialItem | null>(null);
const deletingCipher = ref(false);

// Auto-sync state
const autoSyncInterval = ref<number>(300); // 300s = 5 minutes default
let autoSyncTimer: any = null;

// Add Credential Form State
const addForm = ref({
  type: 1, // 1: Login, 2: Secure Note
  name: '',
  username: '',
  password: '',
  showPassword: true,
  uri: '',
  notes: '',
});
const addingCredential = ref(false);

// Form State
const formServerUrl = ref('');
const formEmail = ref('');
const formMasterPassword = ref('');
const showMasterPassword = ref(false);
const testResult = ref<{ ok: boolean; message: string; count?: number } | null>(null);

// Visibility of item passwords map: { [id: string]: boolean }
const visiblePasswords = ref<Record<string, boolean>>({});
const copiedKey = ref<string | null>(null);

// Feedback message
const feedbackMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null);

const triggerToast = (text: string, type: 'success' | 'error' = 'success') => {
  feedbackMessage.value = { type, text };
  setTimeout(() => {
    if (feedbackMessage.value?.text === text) {
      feedbackMessage.value = null;
    }
  }, 3000);
};

// Copy to clipboard helper
const copyToClipboard = async (text: string, keyId: string, label: string) => {
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    copiedKey.value = keyId;
    triggerToast(`${label} copied to clipboard`);
    setTimeout(() => {
      if (copiedKey.value === keyId) {
        copiedKey.value = null;
      }
    }, 2000);
  } catch (_) {
    triggerToast('Failed to copy to clipboard', 'error');
  }
};

const togglePasswordVisibility = (id: string) => {
  visiblePasswords.value[id] = !visiblePasswords.value[id];
};

// Folders List (Unique extracted from ciphers)
const availableFolders = computed(() => {
  const folders = new Set<string>();
  ciphers.value.forEach((item) => {
    if (item.folderName) {
      folders.add(item.folderName);
    }
  });
  return Array.from(folders).sort();
});

// Filtered Credentials
const filteredCiphers = computed(() => {
  let list = ciphers.value;

  if (selectedFolder.value !== 'all') {
    list = list.filter((item) => item.folderName === selectedFolder.value);
  }

  if (selectedType.value !== 'all') {
    const typeNum = parseInt(selectedType.value, 10);
    list = list.filter((item) => item.type === typeNum);
  }

  const kw = searchKeyword.value.trim().toLowerCase();
  if (kw) {
    list = list.filter((item) => {
      const matchName = item.name?.toLowerCase().includes(kw);
      const matchUser = item.username?.toLowerCase().includes(kw);
      const matchNotes = item.notes?.toLowerCase().includes(kw);
      const matchUri = item.uris?.some((u) => u.toLowerCase().includes(kw));
      return matchName || matchUser || matchNotes || matchUri;
    });
  }

  return list;
});

// Stats
const totalLogins = computed(() => ciphers.value.filter((i) => i.type === 1).length);
const totalNotes = computed(() => ciphers.value.filter((i) => i.type === 2).length);

// Format date helper
const formatRelativeTime = (isoString?: string) => {
  if (!isoString) return 'Never';
  try {
    const date = new Date(isoString);
    return date.toLocaleString();
  } catch (_) {
    return 'Unknown';
  }
};

// API Calls
const loadConfig = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/vaultwarden/config');
    if (res.data.success && res.data.configured) {
      isConfigured.value = true;
      config.value = res.data.data;
      formServerUrl.value = config.value?.serverUrl || '';
      formEmail.value = config.value?.email || '';
      await loadCiphers();
    } else {
      isConfigured.value = false;
      config.value = null;
    }
  } catch (err: any) {
    triggerToast(err.response?.data?.message || 'Failed to connect to backend', 'error');
  } finally {
    loading.value = false;
  }
};

const loadCiphers = async () => {
  try {
    const res = await axios.get('/api/v1/vaultwarden/ciphers');
    if (res.data.success) {
      ciphers.value = res.data.data || [];
    }
  } catch (err: any) {
    triggerToast(err.response?.data?.message || 'Failed to load credentials', 'error');
  }
};

const syncNow = async (silent = false) => {
  if (syncing.value) return;
  syncing.value = true;
  try {
    const res = await axios.post('/api/v1/vaultwarden/sync');
    if (res.data.success) {
      if (!silent) {
        triggerToast(res.data.data?.message || 'Vault synchronized successfully');
      }
      if (res.data.data?.items) {
        ciphers.value = res.data.data.items;
      }
      if (config.value && res.data.data?.lastSyncedAt) {
        config.value.lastSyncedAt = res.data.data.lastSyncedAt;
      }
    } else if (!silent) {
      triggerToast(res.data.error || 'Failed to synchronize vault', 'error');
    }
  } catch (err: any) {
    if (!silent) {
      triggerToast(err.response?.data?.error || 'Sync request failed', 'error');
    }
  } finally {
    syncing.value = false;
  }
};

const startAutoSync = () => {
  if (autoSyncTimer) {
    clearInterval(autoSyncTimer);
    autoSyncTimer = null;
  }
  if (autoSyncInterval.value > 0) {
    autoSyncTimer = setInterval(() => {
      if (isConfigured.value) {
        syncNow(true);
      }
    }, autoSyncInterval.value * 1000);
  }
};

watch(autoSyncInterval, () => {
  startAutoSync();
});

const generatePassword = () => {
  const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  const lower = 'abcdefghijklmnopqrstuvwxyz';
  const digits = '0123456789';
  const symbols = '!@#$%^&*()-_=+';
  const all = upper + lower + digits + symbols;

  let pwd = '';
  const array = new Uint8Array(16);
  window.crypto.getRandomValues(array);
  
  pwd += upper[array[0] % upper.length];
  pwd += lower[array[1] % lower.length];
  pwd += digits[array[2] % digits.length];
  pwd += symbols[array[3] % symbols.length];

  for (let i = 4; i < 16; i++) {
    pwd += all[array[i] % all.length];
  }

  addForm.value.password = pwd.split('').sort(() => 0.5 - Math.random()).join('');
  addForm.value.showPassword = true;
};

const handleCreateCredential = async () => {
  if (!addForm.value.name.trim()) {
    triggerToast('Credential name is required', 'error');
    return;
  }

  addingCredential.value = true;
  try {
    const res = await axios.post('/api/v1/vaultwarden/ciphers', {
      type: addForm.value.type,
      name: addForm.value.name.trim(),
      username: addForm.value.username.trim(),
      password: addForm.value.password,
      uri: addForm.value.uri.trim(),
      notes: addForm.value.notes.trim(),
    });

    if (res.data.success) {
      triggerToast(`Credential "${addForm.value.name}" added to Vaultwarden successfully!`);
      showAddModal.value = false;
      addForm.value = {
        type: 1,
        name: '',
        username: '',
        password: '',
        showPassword: true,
        uri: '',
        notes: '',
      };
      await loadCiphers();
      if (config.value) {
        config.value.lastSyncedAt = new Date().toISOString();
      }
    } else {
      triggerToast(res.data.error || 'Failed to create credential', 'error');
    }
  } catch (err: any) {
    triggerToast(err.response?.data?.error || 'Failed to save credential to Vaultwarden', 'error');
  } finally {
    addingCredential.value = false;
  }
};

const confirmDeleteCipher = (item: VaultCredentialItem) => {
  cipherToDelete.value = item;
  showDeleteCipherModal.value = true;
};

const executeDeleteCipher = async () => {
  if (!cipherToDelete.value) return;
  deletingCipher.value = true;
  const target = cipherToDelete.value;
  try {
    const res = await axios.delete(`/api/v1/vaultwarden/ciphers/${target.id}`);
    if (res.data.success) {
      triggerToast(`Credential "${target.name}" removed from Vaultwarden`);
      showDeleteCipherModal.value = false;
      cipherToDelete.value = null;
      if (selectedItem.value?.id === target.id) {
        showDetailModal.value = false;
      }
      await loadCiphers();
      if (config.value) {
        config.value.lastSyncedAt = new Date().toISOString();
      }
    } else {
      triggerToast(res.data.error || 'Failed to delete credential', 'error');
    }
  } catch (err: any) {
    triggerToast(err.response?.data?.error || 'Failed to delete credential', 'error');
  } finally {
    deletingCipher.value = false;
  }
};

const handleTestConnection = async () => {
  if (!formServerUrl.value || !formEmail.value) {
    triggerToast('Please provide server URL and email', 'error');
    return;
  }
  testing.value = true;
  testResult.value = null;
  try {
    const res = await axios.post('/api/v1/vaultwarden/test', {
      serverUrl: formServerUrl.value,
      email: formEmail.value,
      masterPassword: formMasterPassword.value,
    });
    if (res.data.success) {
      testResult.value = {
        ok: true,
        message: res.data.message || 'Connection successful!',
        count: res.data.totalItems,
      };
    } else {
      testResult.value = {
        ok: false,
        message: res.data.error || 'Connection failed',
      };
    }
  } catch (err: any) {
    testResult.value = {
      ok: false,
      message: err.response?.data?.error || 'Unable to connect to Vaultwarden server',
    };
  } finally {
    testing.value = false;
  }
};

const handleSaveConfig = async () => {
  if (!formServerUrl.value || !formEmail.value) {
    triggerToast('Server URL and Email are required', 'error');
    return;
  }
  if (!isConfigured.value && !formMasterPassword.value) {
    triggerToast('Master password is required for initial setup', 'error');
    return;
  }

  saving.value = true;
  try {
    const res = await axios.post('/api/v1/vaultwarden/config', {
      id: config.value?.id || '',
      serverUrl: formServerUrl.value,
      email: formEmail.value,
      masterPassword: formMasterPassword.value,
      autoSync: true,
    });

    if (res.data.success) {
      triggerToast('Vaultwarden configuration saved');
      showConfigModal.value = false;
      formMasterPassword.value = '';
      testResult.value = null;
      await loadConfig();
    } else {
      triggerToast(res.data.error || 'Failed to save configuration', 'error');
    }
  } catch (err: any) {
    triggerToast(err.response?.data?.error || 'Save failed', 'error');
  } finally {
    saving.value = false;
  }
};

const executeDelete = async () => {
  deleting.value = true;
  try {
    const res = await axios.delete('/api/v1/vaultwarden/config');
    if (res.data.success) {
      triggerToast('Vaultwarden disconnected successfully');
      showDeleteModal.value = false;
      showConfigModal.value = false;
      isConfigured.value = false;
      config.value = null;
      ciphers.value = [];
      formMasterPassword.value = '';
    } else {
      triggerToast(res.data.error || 'Failed to disconnect', 'error');
    }
  } catch (err: any) {
    triggerToast(err.response?.data?.error || 'Disconnect failed', 'error');
  } finally {
    deleting.value = false;
  }
};

const openDetail = (item: VaultCredentialItem) => {
  selectedItem.value = item;
  showDetailModal.value = true;
};

const onWindowFocus = () => {
  if (isConfigured.value && !syncing.value) {
    syncNow(true);
  }
};

onMounted(async () => {
  await loadConfig();
  if (isConfigured.value) {
    syncNow(true);
  }
  startAutoSync();
  window.addEventListener('focus', onWindowFocus);
});

onUnmounted(() => {
  if (autoSyncTimer) clearInterval(autoSyncTimer);
  window.removeEventListener('focus', onWindowFocus);
});
</script>

<template>
  <div class="min-h-screen bg-slate-100 dark:bg-[#090d16] text-slate-800 dark:text-slate-100 font-sans">
    <!-- Standalone Top Bar Header -->
    <header class="h-14 md:h-16 bg-white dark:bg-[#0c101a] border-b border-slate-200 dark:border-[#1b2234] px-4 sm:px-6 flex items-center justify-between sticky top-0 z-30">
      <div class="flex items-center gap-3">
        <a
          href="/"
          class="flex items-center gap-2 text-xs font-semibold text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#121826] transition cursor-pointer"
        >
          <ArrowLeft class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
          <span>Back to Dashboard</span>
        </a>
        <div class="h-4 w-px bg-slate-200 dark:bg-[#1b2234]"></div>
        <div class="flex items-center gap-2">
          <div class="w-7 h-7 rounded-lg bg-blue-50 dark:bg-[#293681]/30 border border-blue-200 dark:border-[#4274D9]/40 flex items-center justify-center">
            <Key class="w-4 h-4 text-blue-600 dark:text-[#95CCDD]" />
          </div>
          <div>
            <span class="text-xs font-bold text-slate-900 dark:text-white tracking-tight">VAULTWARDEN</span>
            <span class="text-[10px] text-slate-400 dark:text-slate-500 font-mono ml-1.5 hidden sm:inline">E2EE VAULT</span>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <!-- Connected status -->
        <div v-if="isConfigured" class="hidden md:flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-slate-50 dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] text-[11px] text-slate-500 dark:text-slate-400">
          <span class="w-2 h-2 rounded-full bg-emerald-500"></span>
          <span>Connected</span>
        </div>

        <!-- Auto-sync selector -->
        <div v-if="isConfigured" class="hidden lg:flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-slate-50 dark:bg-[#111624] border border-slate-200 dark:border-[#1b2234] text-[11px] text-slate-600 dark:text-slate-400">
          <Clock class="w-3 h-3 text-slate-400" />
          <span>Auto-sync:</span>
          <select
            v-model.number="autoSyncInterval"
            class="bg-transparent text-slate-800 dark:text-slate-200 font-semibold focus:outline-none cursor-pointer"
          >
            <option :value="60" class="bg-white dark:bg-[#0c101a]">1m</option>
            <option :value="300" class="bg-white dark:bg-[#0c101a]">5m</option>
            <option :value="900" class="bg-white dark:bg-[#0c101a]">15m</option>
            <option :value="0" class="bg-white dark:bg-[#0c101a]">Off</option>
          </select>
        </div>

        <button
          v-if="isConfigured"
          @click="syncNow(false)"
          :disabled="syncing"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#1a2336] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-medium transition cursor-pointer disabled:opacity-50"
        >
          <RefreshCw class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" :class="{ 'animate-spin': syncing }" />
          <span>{{ syncing ? 'Syncing...' : 'Sync Now' }}</span>
        </button>

        <button
          @click="showConfigModal = true"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-medium transition cursor-pointer shadow-xs"
        >
          <Settings class="w-3.5 h-3.5" />
          <span>{{ isConfigured ? 'Settings' : 'Connect Vault' }}</span>
        </button>

        <ThemeToggle />
      </div>
    </header>

    <!-- Toast Notification Banner (Auto dismiss 3s) -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="transform -translate-y-2 opacity-0"
      enter-to-class="transform translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="transform translate-y-0 opacity-100"
      leave-to-class="transform -translate-y-2 opacity-0"
    >
      <div
        v-if="feedbackMessage"
        class="fixed top-20 right-6 z-50 flex items-center gap-2.5 px-4 py-2.5 rounded-xl shadow-xl text-xs font-medium border"
        :class="feedbackMessage.type === 'success' ? 'bg-white dark:bg-[#111624] border-emerald-500 text-slate-900 dark:text-white' : 'bg-white dark:bg-[#111624] border-rose-500 text-slate-900 dark:text-white'"
      >
        <CheckCircle2 v-if="feedbackMessage.type === 'success'" class="w-4 h-4 text-emerald-500 shrink-0" />
        <AlertCircle v-else class="w-4 h-4 text-rose-500 shrink-0" />
        <span>{{ feedbackMessage.text }}</span>
      </div>
    </Transition>

    <!-- Main Content Container -->
    <main class="max-w-7xl mx-auto p-4 sm:p-6 space-y-6">
      <!-- Standard Header (Clean text without icon, matching AGENTS.md) -->
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
        <div>
          <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
            Vaultwarden Credentials
          </h1>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            End-to-end encrypted credential vault synchronized from your Vaultwarden instance.
          </p>
        </div>

        <div v-if="isConfigured" class="flex items-center gap-3 shrink-0">
          <span class="text-xs text-slate-500 dark:text-slate-400">
            Last sync: <strong class="text-slate-800 dark:text-slate-200">{{ formatRelativeTime(config?.lastSyncedAt) }}</strong>
          </span>
          <button
            @click="showAddModal = true"
            class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-bold transition flex items-center gap-1.5 cursor-pointer shadow-xs"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Credential</span>
          </button>
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="flex flex-col items-center justify-center py-20 text-slate-400">
        <RefreshCw class="w-8 h-8 animate-spin mb-3 text-slate-400 dark:text-slate-500" />
        <p class="text-xs font-medium">Loading Vaultwarden integration...</p>
      </div>

      <!-- Unconfigured Empty State -->
      <div v-else-if="!isConfigured" class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-2xl p-8 sm:p-12 text-center max-w-2xl mx-auto space-y-5 shadow-xs">
        <div class="w-16 h-16 rounded-2xl bg-blue-50 dark:bg-[#141b2d] border border-blue-200 dark:border-[#293681] flex items-center justify-center mx-auto text-blue-600 dark:text-[#95CCDD]">
          <Shield class="w-8 h-8" />
        </div>
        <div class="space-y-2">
          <h2 class="text-base font-bold text-slate-900 dark:text-white">Connect Your Vaultwarden Service</h2>
          <p class="text-xs text-slate-500 dark:text-slate-400 max-w-md mx-auto leading-relaxed">
            Link your self-hosted Vaultwarden server to access, search, and manage credentials securely within Hephaestus Control Panel without opening separate dashboards.
          </p>
        </div>
        <div class="pt-2">
          <button
            @click="showConfigModal = true"
            class="px-5 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-xl text-xs font-semibold transition cursor-pointer shadow-sm inline-flex items-center gap-2"
          >
            <Key class="w-4 h-4" />
            <span>Setup Vaultwarden Connection</span>
          </button>
        </div>
      </div>

      <!-- Active Content -->
      <div v-else class="space-y-6">
        <!-- Metric Overview Cards -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 sm:gap-4">
          <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-4 shadow-xs">
            <div class="flex items-center justify-between text-slate-400 mb-2">
              <span class="text-xs font-medium">Total Items</span>
              <Layers class="w-4 h-4 text-slate-400 dark:text-slate-500" />
            </div>
            <div class="text-2xl font-bold text-slate-900 dark:text-white font-mono">{{ ciphers.length }}</div>
            <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">Stored in vault</div>
          </div>

          <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-4 shadow-xs">
            <div class="flex items-center justify-between text-slate-400 mb-2">
              <span class="text-xs font-medium">Logins & Accounts</span>
              <Key class="w-4 h-4 text-slate-400 dark:text-slate-500" />
            </div>
            <div class="text-2xl font-bold text-slate-900 dark:text-white font-mono">{{ totalLogins }}</div>
            <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">Credentials available</div>
          </div>

          <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-4 shadow-xs">
            <div class="flex items-center justify-between text-slate-400 mb-2">
              <span class="text-xs font-medium">Secure Notes</span>
              <FileText class="w-4 h-4 text-slate-400 dark:text-slate-500" />
            </div>
            <div class="text-2xl font-bold text-slate-900 dark:text-white font-mono">{{ totalNotes }}</div>
            <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">Encrypted notes</div>
          </div>

          <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-4 shadow-xs">
            <div class="flex items-center justify-between text-slate-400 mb-2">
              <span class="text-xs font-medium">Vault Server</span>
              <Clock class="w-4 h-4 text-slate-400 dark:text-slate-500" />
            </div>
            <div class="text-xs font-semibold text-slate-800 dark:text-slate-200 truncate" :title="config?.serverUrl">
              {{ config?.serverUrl?.replace(/^https?:\/\//, '') }}
            </div>
            <div class="text-[11px] text-emerald-600 dark:text-emerald-400 mt-1 flex items-center gap-1">
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
              <span>Encrypted sync</span>
            </div>
          </div>
        </div>

        <!-- Filter & Search Toolbar -->
        <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-3 sm:p-4 flex flex-col sm:flex-row items-center justify-between gap-3 shadow-xs">
          <div class="relative w-full sm:w-80">
            <Search class="w-4 h-4 text-slate-400 dark:text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              v-model="searchKeyword"
              type="text"
              placeholder="Search by name, username, URL..."
              class="w-full pl-9 pr-3 py-1.5 text-xs bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div class="flex items-center gap-2 w-full sm:w-auto">
            <!-- Filter by Type -->
            <select
              v-model="selectedType"
              class="px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-slate-700 dark:text-slate-300 focus:outline-none cursor-pointer"
            >
              <option value="all">All Types</option>
              <option value="1">Logins Only</option>
              <option value="2">Secure Notes</option>
            </select>

            <!-- Filter by Folder -->
            <select
              v-if="availableFolders.length > 0"
              v-model="selectedFolder"
              class="px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-slate-700 dark:text-slate-300 focus:outline-none cursor-pointer"
            >
              <option value="all">All Folders</option>
              <option v-for="f in availableFolders" :key="f" :value="f">{{ f }}</option>
            </select>

            <span class="text-xs text-slate-400 dark:text-slate-500 font-mono ml-auto sm:ml-2">
              {{ filteredCiphers.length }} items
            </span>
          </div>
        </div>

        <!-- Empty Filter Result -->
        <div v-if="filteredCiphers.length === 0" class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-8 text-center text-slate-400">
          <p class="text-xs">No credentials found matching the search criteria.</p>
        </div>

        <!-- Credentials Grid View -->
        <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5 sm:gap-4">
          <div
            v-for="item in filteredCiphers"
            :key="item.id"
            class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-4 flex flex-col justify-between hover:border-slate-300 dark:hover:border-[#293681] transition shadow-xs space-y-3"
          >
            <!-- Card Header -->
            <div class="space-y-1">
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-center gap-2 overflow-hidden">
                  <div class="w-6 h-6 rounded-md bg-slate-100 dark:bg-[#141b2d] border border-slate-200 dark:border-[#1f283d] flex items-center justify-center shrink-0">
                    <Key v-if="item.type === 1" class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
                    <FileText v-else class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
                  </div>
                  <h3 class="text-xs font-bold text-slate-900 dark:text-white truncate" :title="item.name">
                    {{ item.name }}
                  </h3>
                </div>

                <div class="flex items-center gap-1 shrink-0">
                  <span v-if="item.folderName" class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 dark:bg-[#151c2d] text-slate-600 dark:text-slate-400 font-mono">
                    {{ item.folderName }}
                  </span>
                  <span class="text-[10px] px-1.5 py-0.5 rounded border border-slate-200 dark:border-[#1b2234] text-slate-500 font-mono">
                    {{ item.typeLabel }}
                  </span>
                </div>
              </div>

              <!-- URI / Target URL -->
              <div v-if="item.uris && item.uris.length > 0" class="flex items-center gap-1.5 text-[11px] text-blue-600 dark:text-[#95CCDD] truncate pt-0.5">
                <ExternalLink class="w-3 h-3 shrink-0" />
                <a :href="item.uris[0]" target="_blank" class="hover:underline truncate" :title="item.uris[0]">
                  {{ item.uris[0].replace(/^https?:\/\//, '') }}
                </a>
              </div>
            </div>

            <!-- Credentials Fields -->
            <div class="space-y-2 pt-1 border-t border-slate-100 dark:border-[#161d2d]">
              <!-- Username Row -->
              <div v-if="item.username" class="flex items-center justify-between bg-slate-50 dark:bg-[#111624] px-2.5 py-1.5 rounded-lg text-xs">
                <span class="text-slate-500 dark:text-slate-400 text-[11px] select-none">Username</span>
                <div class="flex items-center gap-1.5 overflow-hidden pl-2">
                  <span class="text-slate-800 dark:text-slate-200 font-mono text-[11px] truncate max-w-[150px]" :title="item.username">
                    {{ item.username }}
                  </span>
                  <button
                    @click="copyToClipboard(item.username, item.id + '-user', 'Username')"
                    class="p-1 hover:text-blue-600 dark:hover:text-[#95CCDD] text-slate-400 cursor-pointer rounded transition"
                    title="Copy Username"
                  >
                    <Check v-if="copiedKey === item.id + '-user'" class="w-3.5 h-3.5 text-emerald-500" />
                    <Copy v-else class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              <!-- Password Row -->
              <div v-if="item.password" class="flex items-center justify-between bg-slate-50 dark:bg-[#111624] px-2.5 py-1.5 rounded-lg text-xs">
                <span class="text-slate-500 dark:text-slate-400 text-[11px] select-none">Password</span>
                <div class="flex items-center gap-1.5">
                  <span class="font-mono text-[11px] tracking-wider text-slate-800 dark:text-slate-200">
                    {{ visiblePasswords[item.id] ? item.password : '••••••••••••' }}
                  </span>
                  <button
                    @click="togglePasswordVisibility(item.id)"
                    class="p-1 hover:text-slate-700 dark:hover:text-white text-slate-400 cursor-pointer rounded transition"
                    :title="visiblePasswords[item.id] ? 'Hide password' : 'Show password'"
                  >
                    <EyeOff v-if="visiblePasswords[item.id]" class="w-3.5 h-3.5" />
                    <Eye v-else class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="copyToClipboard(item.password, item.id + '-pass', 'Password')"
                    class="p-1 hover:text-blue-600 dark:hover:text-[#95CCDD] text-slate-400 cursor-pointer rounded transition"
                    title="Copy Password"
                  >
                    <Check v-if="copiedKey === item.id + '-pass'" class="w-3.5 h-3.5 text-emerald-500" />
                    <Copy v-else class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              <!-- Notes Snippet -->
              <div v-if="item.notes" class="text-[11px] text-slate-500 dark:text-slate-400 line-clamp-2 pt-0.5">
                {{ item.notes }}
              </div>
            </div>

            <!-- Card Actions -->
            <div class="pt-1 flex items-center justify-between text-[10px] text-slate-400 border-t border-slate-100 dark:border-[#161d2d]">
              <span>Modified: {{ formatRelativeTime(item.revisionDate) }}</span>
              <div class="flex items-center gap-2">
                <button
                  @click="openDetail(item)"
                  class="hover:text-slate-900 dark:hover:text-white text-blue-600 dark:text-[#95CCDD] font-semibold cursor-pointer"
                >
                  Details
                </button>
                <button
                  @click="confirmDeleteCipher(item)"
                  class="text-slate-400 hover:text-rose-500 transition cursor-pointer p-0.5"
                  title="Delete Credential"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Modal: Connection Configuration -->
    <div
      v-if="showConfigModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-6 space-y-5">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-blue-50 dark:bg-[#141b2d] border border-blue-200 dark:border-[#293681] flex items-center justify-center text-blue-600 dark:text-[#95CCDD]">
              <Key class="w-4 h-4" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white">Vaultwarden Connection</h3>
              <p class="text-[11px] text-slate-500 dark:text-slate-400">Configure self-hosted Bitwarden/Vaultwarden credentials</p>
            </div>
          </div>
          <button @click="showConfigModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="space-y-4 text-xs">
          <!-- Server URL -->
          <div class="space-y-1.5">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Vaultwarden Server URL</label>
            <input
              v-model="formServerUrl"
              type="url"
              placeholder="https://vault.yourdomain.com"
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
            <p class="text-[10px] text-slate-400">Base URL where your Vaultwarden instance is hosted (HTTP or HTTPS).</p>
          </div>

          <!-- Email -->
          <div class="space-y-1.5">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Account Email</label>
            <input
              v-model="formEmail"
              type="email"
              placeholder="admin@yourdomain.com"
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Master Password -->
          <div class="space-y-1.5">
            <label class="font-semibold text-slate-700 dark:text-slate-300">
              Master Password
              <span v-if="isConfigured" class="text-[10px] font-normal text-slate-400">(leave blank to keep current saved password)</span>
            </label>
            <div class="relative">
              <input
                v-model="formMasterPassword"
                :type="showMasterPassword ? 'text' : 'password'"
                placeholder="••••••••••••••••"
                class="w-full px-3 py-2 pr-9 bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 font-mono"
              />
              <button
                type="button"
                @click="showMasterPassword = !showMasterPassword"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer"
              >
                <EyeOff v-if="showMasterPassword" class="w-4 h-4" />
                <Eye v-else class="w-4 h-4" />
              </button>
            </div>
            <p class="text-[10px] text-slate-400">
              Stored securely in local PostgreSQL with AES-256-GCM encryption for zero-knowledge vault decryption.
            </p>
          </div>

          <!-- Test Result Banner -->
          <div
            v-if="testResult"
            class="p-3 rounded-lg border text-xs flex items-start gap-2"
            :class="testResult.ok ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-600 dark:text-rose-400'"
          >
            <CheckCircle2 v-if="testResult.ok" class="w-4 h-4 shrink-0 mt-0.5" />
            <AlertCircle v-else class="w-4 h-4 shrink-0 mt-0.5" />
            <div class="space-y-0.5">
              <div class="font-semibold">{{ testResult.ok ? 'Connection Verified' : 'Connection Failed' }}</div>
              <div class="text-[11px]">{{ testResult.message }}</div>
            </div>
          </div>
        </div>

        <!-- Modal Actions -->
        <div class="flex items-center justify-between pt-2 border-t border-slate-200 dark:border-[#1b2234]">
          <button
            v-if="isConfigured"
            type="button"
            @click="showDeleteModal = true"
            class="px-3 py-1.5 text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/30 rounded-lg text-xs font-semibold transition cursor-pointer"
          >
            Disconnect
          </button>
          <div v-else></div>

          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="handleTestConnection"
              :disabled="testing"
              class="px-3 py-1.5 bg-slate-100 dark:bg-[#151c2d] hover:bg-slate-200 dark:hover:bg-[#1b2339] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition cursor-pointer disabled:opacity-50"
            >
              {{ testing ? 'Testing...' : 'Test Connection' }}
            </button>
            <button
              type="button"
              @click="handleSaveConfig"
              :disabled="saving"
              class="px-4 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
            >
              {{ saving ? 'Saving...' : 'Save & Sync' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Item Detail & Notes -->
    <div
      v-if="showDetailModal && selectedItem"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <Key v-if="selectedItem.type === 1" class="w-4 h-4 text-blue-600 dark:text-[#95CCDD]" />
            <FileText v-else class="w-4 h-4 text-blue-600 dark:text-[#95CCDD]" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white truncate">{{ selectedItem.name }}</h3>
          </div>
          <button @click="showDetailModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div v-if="selectedItem.username" class="space-y-1">
            <span class="text-[11px] text-slate-400 font-medium">Username</span>
            <div class="flex items-center justify-between p-2 bg-slate-50 dark:bg-[#151c2d] rounded-lg font-mono text-[11px]">
              <span class="truncate">{{ selectedItem.username }}</span>
              <button @click="copyToClipboard(selectedItem.username, 'modal-user', 'Username')" class="text-slate-400 hover:text-white">
                <Copy class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div v-if="selectedItem.password" class="space-y-1">
            <span class="text-[11px] text-slate-400 font-medium">Password</span>
            <div class="flex items-center justify-between p-2 bg-slate-50 dark:bg-[#151c2d] rounded-lg font-mono text-[11px]">
              <span class="truncate">{{ selectedItem.password }}</span>
              <button @click="copyToClipboard(selectedItem.password, 'modal-pass', 'Password')" class="text-slate-400 hover:text-white">
                <Copy class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div v-if="selectedItem.uris && selectedItem.uris.length > 0" class="space-y-1">
            <span class="text-[11px] text-slate-400 font-medium">URIs / Target Host</span>
            <div v-for="u in selectedItem.uris" :key="u" class="p-2 bg-slate-50 dark:bg-[#151c2d] rounded-lg text-[11px] truncate">
              <a :href="u" target="_blank" class="text-blue-600 dark:text-[#95CCDD] hover:underline flex items-center gap-1.5">
                <ExternalLink class="w-3 h-3 shrink-0" />
                <span class="truncate">{{ u }}</span>
              </a>
            </div>
          </div>

          <div v-if="selectedItem.notes" class="space-y-1">
            <span class="text-[11px] text-slate-400 font-medium">Secure Notes</span>
            <div class="p-2.5 bg-slate-50 dark:bg-[#151c2d] rounded-lg text-[11px] text-slate-700 dark:text-slate-300 whitespace-pre-wrap font-mono max-h-40 overflow-y-auto">
              {{ selectedItem.notes }}
            </div>
          </div>
        </div>

        <div class="pt-2 flex items-center justify-between border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showDetailModal = false; confirmDeleteCipher(selectedItem)"
            class="px-3 py-1.5 text-xs text-rose-600 hover:text-rose-700 font-semibold flex items-center gap-1.5 cursor-pointer"
          >
            <Trash2 class="w-3.5 h-3.5" />
            <span>Delete Credential</span>
          </button>
          <button
            @click="showDetailModal = false"
            class="px-4 py-1.5 bg-slate-100 dark:bg-[#1b2339] hover:bg-slate-200 dark:hover:bg-[#252f4c] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold cursor-pointer"
          >
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Add New Credential -->
    <div
      v-if="showAddModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-5 sm:p-6 space-y-4 max-h-[90vh] overflow-y-auto">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <Key class="w-4 h-4 text-blue-600 dark:text-[#95CCDD]" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">Add Credential to Vaultwarden</h3>
          </div>
          <button @click="showAddModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleCreateCredential" class="space-y-3.5 text-xs">
          <!-- Type selector -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Item Type</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                @click="addForm.type = 1"
                :class="[
                  'py-1.5 px-3 rounded-lg border text-xs font-medium transition cursor-pointer text-center',
                  addForm.type === 1
                    ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'bg-slate-50 dark:bg-[#151c2d] border-slate-200 dark:border-[#1f283d] text-slate-600 dark:text-slate-400'
                ]"
              >
                Login Account
              </button>
              <button
                type="button"
                @click="addForm.type = 2"
                :class="[
                  'py-1.5 px-3 rounded-lg border text-xs font-medium transition cursor-pointer text-center',
                  addForm.type === 2
                    ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'bg-slate-50 dark:bg-[#151c2d] border-slate-200 dark:border-[#1f283d] text-slate-600 dark:text-slate-400'
                ]"
              >
                Secure Note
              </button>
            </div>
          </div>

          <!-- Name -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Name *</label>
            <input
              v-model="addForm.name"
              required
              placeholder="e.g. Proxmox VE, Production Database, Router Mikrotik"
              class="w-full bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg px-3 py-2 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 text-xs"
            />
          </div>

          <!-- Login Specific Fields -->
          <template v-if="addForm.type === 1">
            <div>
              <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Username / Email</label>
              <input
                v-model="addForm.username"
                placeholder="e.g. administrator, root, user@example.com"
                class="w-full bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg px-3 py-2 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 text-xs font-mono"
              />
            </div>

            <div>
              <div class="flex items-center justify-between mb-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">Password</label>
                <button
                  type="button"
                  @click="generatePassword"
                  class="text-[11px] text-blue-600 dark:text-[#95CCDD] hover:underline font-semibold flex items-center gap-1 cursor-pointer"
                >
                  <RotateCcw class="w-3 h-3" />
                  <span>Generate Password</span>
                </button>
              </div>
              <div class="relative">
                <input
                  v-model="addForm.password"
                  :type="addForm.showPassword ? 'text' : 'password'"
                  placeholder="••••••••••••••••"
                  class="w-full bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg pl-3 pr-16 py-2 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 text-xs font-mono"
                />
                <button
                  type="button"
                  @click="addForm.showPassword = !addForm.showPassword"
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-white p-1 cursor-pointer"
                >
                  <EyeOff v-if="addForm.showPassword" class="w-3.5 h-3.5" />
                  <Eye v-else class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div>
              <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Target Web URL / URI</label>
              <input
                v-model="addForm.uri"
                placeholder="https://10.20.3.1:8006/"
                class="w-full bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg px-3 py-2 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 text-xs font-mono"
              />
            </div>
          </template>

          <!-- Notes -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Notes / Description</label>
            <textarea
              v-model="addForm.notes"
              rows="3"
              placeholder="Additional notes, recovery codes, server details..."
              class="w-full bg-slate-50 dark:bg-[#151c2d] border border-slate-200 dark:border-[#1f283d] rounded-lg p-3 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 text-xs font-mono"
            ></textarea>
          </div>

          <div class="pt-3 border-t border-slate-200 dark:border-[#1b2234] flex items-center justify-end gap-2">
            <button
              type="button"
              @click="showAddModal = false"
              class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="addingCredential"
              class="px-4 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
            >
              <RefreshCw v-if="addingCredential" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ addingCredential ? 'Encrypting & Saving...' : 'Save Credential' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Standard Delete Confirmation Modal for Credential (Conforming strictly to AGENTS.md) -->
    <div
      v-if="showDeleteCipherModal && cipherToDelete"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Credential?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ cipherToDelete.name }}</strong> from Vaultwarden? This action cannot be undone.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteCipherModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDeleteCipher"
            :disabled="deletingCipher"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deletingCipher ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Standard Delete Confirmation Modal for Disconnect (Conforming strictly to AGENTS.md) -->
    <div
      v-if="showDeleteModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Disconnect Vaultwarden?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ config?.name || 'Vaultwarden' }}</strong> integration? Stored master password and cached credentials will be removed.
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
            @click="executeDelete"
            :disabled="deleting"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deleting ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
