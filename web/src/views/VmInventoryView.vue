<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import {
  Server,
  Cpu,
  HardDrive,
  Layers,
  Search,
  Plus,
  RefreshCw,
  Download,
  Upload,
  FileSpreadsheet,
  Trash2,
  Edit2,
  Eye,
  CheckCircle2,
  AlertCircle,
  X,
  ExternalLink,
  Copy,
  Check,
  Radio,
  FileText,
  Network,
} from 'lucide-vue-next';

interface ServerInventoryItem {
  id: string;
  remoteHostId?: string;
  remoteHostName?: string;
  serverName: string;
  ipAddress: string;
  osVersion: string;
  osType: string;
  architectureType: string;
  processorModel: string;
  totalCore: string;
  totalMemory: string;
  totalDimmMemory: string;
  totalStorageSize: string;
  totalDiskCount: string;
  totalNetworkInterfaces: string;
  gpuModel: string;
  gpuType: string;
  totalVram: string;
  status: string;
  notes: string;
  userId?: number;
  ownerUsername?: string;
  lastSyncedAt?: string;
  createdAt: string;
  updatedAt: string;
}

interface ServerInventoryStats {
  totalServers: number;
  activeServers: number;
  syncedFromRemote: number;
  manualServers: number;
  withGpuCount: number;
}

interface RemoteHostOption {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
}

const authStore = useAuthStore();

// State
const loading = ref(false);
const syncingAll = ref(false);
const syncingHostId = ref<string | null>(null);
const inventoryList = ref<ServerInventoryItem[]>([]);
const stats = ref<ServerInventoryStats>({
  totalServers: 0,
  activeServers: 0,
  syncedFromRemote: 0,
  manualServers: 0,
  withGpuCount: 0,
});
const remoteHosts = ref<RemoteHostOption[]>([]);

// Filters
const searchQuery = ref('');
const statusFilter = ref('all');
const osTypeFilter = ref('all');

// Modals
const showAddModal = ref(false);
const isEditing = ref(false);
const editingId = ref('');
const showSyncModal = ref(false);
const showImportModal = ref(false);
const showDetailModal = ref(false);
const selectedDetailItem = ref<ServerInventoryItem | null>(null);

// Delete Confirmation Modal (AGENTS.md Strict Standard)
const showDeleteModal = ref(false);
const deleting = ref(false);
const deleteTargetItem = ref<ServerInventoryItem | null>(null);

// Notification feedback
const notification = ref<{ type: 'success' | 'error'; message: string } | null>(null);
let notifTimeout: any = null;

const showNotification = (type: 'success' | 'error', message: string) => {
  if (notifTimeout) clearTimeout(notifTimeout);
  notification.value = { type, message };
  notifTimeout = setTimeout(() => {
    notification.value = null;
  }, 3000);
};

// Form State
const formData = ref({
  remoteHostId: '',
  serverName: '',
  ipAddress: '',
  osVersion: 'Ubuntu 22.04 LTS',
  osType: 'Linux',
  architectureType: 'x86_64',
  processorModel: '',
  totalCore: '',
  totalMemory: '',
  totalDimmMemory: '',
  totalStorageSize: '',
  totalDiskCount: '',
  totalNetworkInterfaces: '',
  gpuModel: '',
  gpuType: '',
  totalVram: '',
  status: 'active',
  notes: '',
});

// CSV Import State
const importFile = ref<File | null>(null);
const importing = ref(false);
const importResult = ref<{
  totalProcessed: number;
  createdCount: number;
  updatedCount: number;
  failedCount: number;
  errors: string[];
} | null>(null);

// Copy State
const copiedId = ref<string | null>(null);
const copyToClipboard = (text: string, id: string) => {
  if (!text || text === 'N/A') return;
  navigator.clipboard.writeText(text);
  copiedId.value = id;
  setTimeout(() => {
    copiedId.value = null;
  }, 1500);
};

// Load Data
const fetchData = async () => {
  loading.value = true;
  try {
    const params: Record<string, string> = {};
    if (searchQuery.value.trim()) params.search = searchQuery.value.trim();
    if (statusFilter.value !== 'all') params.status = statusFilter.value;
    if (osTypeFilter.value !== 'all') params.osType = osTypeFilter.value;

    const [itemsRes, statsRes] = await Promise.all([
      axios.get('/api/v1/inventory/servers', { params }),
      axios.get('/api/v1/inventory/servers/stats'),
    ]);

    inventoryList.value = itemsRes.data || [];
    if (statsRes.data) {
      stats.value = statsRes.data;
    }
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || 'Failed loading server inventory.');
  } finally {
    loading.value = false;
  }
};

const fetchRemoteHosts = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data?.success && Array.isArray(res.data?.data)) {
      remoteHosts.value = res.data.data;
    } else if (Array.isArray(res.data)) {
      remoteHosts.value = res.data;
    } else if (Array.isArray(res.data?.data)) {
      remoteHosts.value = res.data.data;
    } else {
      remoteHosts.value = [];
    }
  } catch (err) {
    remoteHosts.value = [];
  }
};

const openSyncModal = async () => {
  showSyncModal.value = true;
  await fetchRemoteHosts();
};

// Actions
const openAddModal = async () => {
  isEditing.value = false;
  editingId.value = '';
  formData.value = {
    remoteHostId: '',
    serverName: '',
    ipAddress: '',
    osVersion: 'Ubuntu 22.04 LTS',
    osType: 'Linux',
    architectureType: 'x86_64',
    processorModel: '',
    totalCore: '',
    totalMemory: '',
    totalDimmMemory: '',
    totalStorageSize: '',
    totalDiskCount: '',
    totalNetworkInterfaces: '',
    gpuModel: '',
    gpuType: '',
    totalVram: '',
    status: 'active',
    notes: '',
  };
  showAddModal.value = true;
  await fetchRemoteHosts();
};

const openEditModal = async (item: ServerInventoryItem) => {
  isEditing.value = true;
  editingId.value = item.id;
  formData.value = {
    remoteHostId: item.remoteHostId || '',
    serverName: item.serverName,
    ipAddress: item.ipAddress,
    osVersion: item.osVersion,
    osType: item.osType,
    architectureType: item.architectureType,
    processorModel: item.processorModel === 'N/A' ? '' : item.processorModel,
    totalCore: item.totalCore === 'N/A' ? '' : item.totalCore,
    totalMemory: item.totalMemory === 'N/A' ? '' : item.totalMemory,
    totalDimmMemory: item.totalDimmMemory === 'N/A' ? '' : item.totalDimmMemory,
    totalStorageSize: item.totalStorageSize === 'N/A' ? '' : item.totalStorageSize,
    totalDiskCount: item.totalDiskCount === 'N/A' ? '' : item.totalDiskCount,
    totalNetworkInterfaces: item.totalNetworkInterfaces === 'N/A' ? '' : item.totalNetworkInterfaces,
    gpuModel: item.gpuModel === 'N/A' ? '' : item.gpuModel,
    gpuType: item.gpuType === 'N/A' ? '' : item.gpuType,
    totalVram: item.totalVram === 'N/A' ? '' : item.totalVram,
    status: item.status,
    notes: item.notes || '',
  };
  showAddModal.value = true;
  await fetchRemoteHosts();
};

const saveServer = async () => {
  if (!formData.value.serverName.trim()) {
    showNotification('error', 'Server name is required.');
    return;
  }
  if (!formData.value.ipAddress.trim()) {
    showNotification('error', 'IP Address is required.');
    return;
  }

  try {
    const payload = {
      ...formData.value,
      remoteHostId: formData.value.remoteHostId ? formData.value.remoteHostId : null,
      processorModel: formData.value.processorModel.trim() || 'N/A',
      totalCore: formData.value.totalCore.trim() || 'N/A',
      totalMemory: formData.value.totalMemory.trim() || 'N/A',
      totalDimmMemory: formData.value.totalDimmMemory.trim() || 'N/A',
      totalStorageSize: formData.value.totalStorageSize.trim() || 'N/A',
      totalDiskCount: formData.value.totalDiskCount.trim() || 'N/A',
      totalNetworkInterfaces: formData.value.totalNetworkInterfaces.trim() || 'N/A',
      gpuModel: formData.value.gpuModel.trim() || 'N/A',
      gpuType: formData.value.gpuType.trim() || 'N/A',
      totalVram: formData.value.totalVram.trim() || 'N/A',
    };

    if (isEditing.value) {
      await axios.put(`/api/v1/inventory/servers/${editingId.value}`, payload);
      showNotification('success', `Server ${formData.value.serverName} updated successfully.`);
    } else {
      await axios.post('/api/v1/inventory/servers', payload);
      showNotification('success', `Server ${formData.value.serverName} created successfully.`);
    }

    showAddModal.value = false;
    await fetchData();
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || 'Failed saving server inventory.');
  }
};

// Sync Remote Host
const triggerSyncRemote = async (remoteHostId: string, hostName: string) => {
  syncingHostId.value = remoteHostId;
  try {
    const res = await axios.post(`/api/v1/inventory/servers/sync-remote/${remoteHostId}`);
    showNotification('success', res.data?.message || `Specifications for ${hostName} synchronized successfully.`);
    await fetchData();
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || `Failed probing specifications for ${hostName}.`);
  } finally {
    syncingHostId.value = null;
  }
};

const getHostInventoryStatus = (host: RemoteHostOption) => {
  return inventoryList.value.find(
    (i) => i.remoteHostId === host.id || i.ipAddress === host.host
  );
};

const triggerSyncAll = async () => {
  syncingAll.value = true;
  try {
    const res = await axios.post('/api/v1/inventory/servers/sync-all');
    const result = res.data;
    if (result.errors && result.errors.length > 0) {
      showNotification(
        'success',
        `Synchronized ${result.totalHosts} remote servers (${result.syncedSuccess} active, ${result.errors.length} unreachable/offline).`
      );
    } else {
      showNotification(
        'success',
        `All ${result.totalHosts} remote servers synchronized successfully.`
      );
    }
    await fetchData();
    showSyncModal.value = false;
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || 'Failed synchronizing all remote hosts.');
  } finally {
    syncingAll.value = false;
  }
};

// CSV Export & Template
const downloadTemplate = () => {
  window.open('/api/v1/inventory/servers/template', '_blank');
};

const exportCSV = () => {
  window.open('/api/v1/inventory/servers/export', '_blank');
};

// CSV Import
const onFileSelected = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    importFile.value = target.files[0];
  }
};

const submitImportCSV = async () => {
  if (!importFile.value) {
    showNotification('error', 'Please choose a CSV file to import.');
    return;
  }

  importing.value = true;
  importResult.value = null;
  const data = new FormData();
  data.append('file', importFile.value);

  try {
    const res = await axios.post('/api/v1/inventory/servers/import', data, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    importResult.value = res.data;
    showNotification(
      'success',
      `Import complete: ${res.data.createdCount} created, ${res.data.updatedCount} updated.`
    );
    await fetchData();
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || 'Failed importing CSV file.');
  } finally {
    importing.value = false;
  }
};

// Details Modal
const openDetail = (item: ServerInventoryItem) => {
  selectedDetailItem.value = item;
  showDetailModal.value = true;
};

// Delete Handler (AGENTS.md Strict Standard)
const promptDelete = (item: ServerInventoryItem) => {
  deleteTargetItem.value = item;
  showDeleteModal.value = true;
};

const executeDelete = async () => {
  if (!deleteTargetItem.value) return;
  deleting.value = true;
  try {
    await axios.delete(`/api/v1/inventory/servers/${deleteTargetItem.value.id}`);
    showNotification('success', `Server ${deleteTargetItem.value.serverName} removed successfully.`);
    showDeleteModal.value = false;
    deleteTargetItem.value = null;
    await fetchData();
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || 'Failed deleting server inventory.');
  } finally {
    deleting.value = false;
  }
};

const formatDate = (dateStr?: string) => {
  if (!dateStr) return 'Never';
  try {
    const d = new Date(dateStr);
    return d.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch (e) {
    return dateStr;
  }
};

onMounted(() => {
  fetchData();
  fetchRemoteHosts();
});
</script>

<template>
  <div class="space-y-6 w-full font-sans">
    <!-- Notification Banner (Auto-dismisses in 3000ms) -->
    <transition
      enter-active-class="transform ease-out duration-200 transition"
      enter-from-class="translate-y-2 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition ease-in duration-150"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="notification"
        :class="[
          'fixed bottom-6 right-6 z-50 flex items-center gap-3 px-4 py-3 rounded-xl shadow-xl text-xs font-medium border backdrop-blur-md',
          notification.type === 'success'
            ? 'bg-emerald-50/90 text-emerald-800 border-emerald-300 dark:bg-emerald-950/80 dark:text-emerald-300 dark:border-emerald-800'
            : 'bg-rose-50/90 text-rose-800 border-rose-300 dark:bg-rose-950/80 dark:text-rose-300 dark:border-rose-800',
        ]"
      >
        <CheckCircle2 v-if="notification.type === 'success'" class="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
        <AlertCircle v-else class="w-4 h-4 text-rose-600 dark:text-rose-400 shrink-0" />
        <span>{{ notification.message }}</span>
      </div>
    </transition>

    <!-- Standard Header (AGENTS.md: Pure text h1, no icons, no badges) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Server Inventory
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Comprehensive hardware, CPU architecture, memory DIMMs, storage disks, and GPU specifications.
        </p>
      </div>

      <!-- Action Buttons Toolbar -->
      <div class="flex flex-wrap items-center gap-2 shrink-0">
        <button
          @click="openSyncModal"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-[#141824] hover:bg-slate-200 dark:hover:bg-[#1a2336] border border-slate-300 dark:border-[#222c42] transition cursor-pointer"
        >
          <Radio class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
          <span>Sync Remote Hosts</span>
        </button>

        <button
          @click="showImportModal = true"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-[#141824] hover:bg-slate-200 dark:hover:bg-[#1a2336] border border-slate-300 dark:border-[#222c42] transition cursor-pointer"
        >
          <Upload class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
          <span>Bulk Import</span>
        </button>

        <button
          @click="exportCSV"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-[#141824] hover:bg-slate-200 dark:hover:bg-[#1a2336] border border-slate-300 dark:border-[#222c42] transition cursor-pointer"
          title="Export current inventory list as CSV"
        >
          <Download class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
          <span>Export CSV</span>
        </button>

        <button
          @click="openAddModal"
          class="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 dark:bg-[#293681] dark:hover:bg-[#3446a8] border border-blue-500/30 transition shadow-sm cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Server</span>
        </button>

        <button
          @click="fetchData"
          :disabled="loading"
          class="p-1.5 rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white bg-slate-100 dark:bg-[#141824] hover:bg-slate-200 dark:hover:bg-[#1a2336] border border-slate-300 dark:border-[#222c42] transition cursor-pointer disabled:opacity-50"
          title="Refresh Inventory"
        >
          <RefreshCw :class="['w-4 h-4 text-slate-500 dark:text-slate-400', loading ? 'animate-spin' : '']" />
        </button>
      </div>
    </div>

    <!-- Filters & Search Toolbar -->
    <div class="flex flex-col sm:flex-row gap-3 items-center justify-between bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl p-3 shadow-sm">
      <div class="relative w-full sm:w-80">
        <Search class="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
        <input
          v-model="searchQuery"
          @input="fetchData"
          type="text"
          placeholder="Search hostname, IP, CPU, GPU..."
          class="w-full pl-9 pr-3 py-1.5 text-xs rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500 transition"
        />
      </div>

      <div class="flex items-center gap-2 w-full sm:w-auto">
        <select
          v-model="statusFilter"
          @change="fetchData"
          class="px-2.5 py-1.5 text-xs rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-blue-500 cursor-pointer"
        >
          <option value="all">All Statuses</option>
          <option value="active">Active</option>
          <option value="maintenance">Maintenance</option>
          <option value="offline">Offline</option>
        </select>

        <select
          v-model="osTypeFilter"
          @change="fetchData"
          class="px-2.5 py-1.5 text-xs rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-blue-500 cursor-pointer"
        >
          <option value="all">All OS Types</option>
          <option value="Linux">Linux</option>
          <option value="Windows">Windows</option>
          <option value="FreeBSD">FreeBSD</option>
        </select>

        <button
          @click="downloadTemplate"
          class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#141824] transition cursor-pointer"
          title="Download CSV Template for Bulk Add"
        >
          <FileSpreadsheet class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
          <span class="hidden md:inline">CSV Template</span>
        </button>
      </div>
    </div>

    <!-- Inventory Table -->
    <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-slate-50 dark:bg-[#141824] text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-[#1b2234]">
            <tr>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[150px]">Server Name</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[130px]">IP Address</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[160px]">OS Version</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[90px]">OS Type</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[110px]">Architecture</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[220px]">Processor Model</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[90px]">Total Cores</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[110px]">Total Memory</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[120px]">DIMM Slots</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[110px]">Storage Size</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[160px]">Storage Disks</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[160px]">Network Interfaces</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[170px]">GPU Model</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[100px]">GPU Type</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[100px]">Total VRAM</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[100px]">Status</th>
              <th class="px-3.5 py-3 font-semibold whitespace-nowrap min-w-[130px]">Last Synced</th>
              <th class="px-3.5 py-3 font-semibold text-right whitespace-nowrap sticky right-0 bg-slate-50 dark:bg-[#141824] z-10 shadow-[-4px_0_6px_-2px_rgba(0,0,0,0.06)] min-w-[130px]">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-[#192132]">
            <tr v-if="loading && inventoryList.length === 0">
              <td colspan="18" class="px-4 py-12 text-center text-slate-400 dark:text-slate-500">
                <RefreshCw class="w-6 h-6 animate-spin mx-auto mb-2 text-slate-400" />
                <span>Loading server inventory...</span>
              </td>
            </tr>

            <tr v-else-if="inventoryList.length === 0">
              <td colspan="18" class="px-4 py-12 text-center text-slate-500 dark:text-slate-400">
                <Server class="w-8 h-8 mx-auto mb-2 text-slate-400 opacity-60" />
                <p class="font-medium text-slate-700 dark:text-slate-300">No servers registered in inventory</p>
                <p class="text-[11px] text-slate-400 mt-1">
                  Add servers manually, sync from your registered Remote Hosts, or import via CSV.
                </p>
                <div class="mt-4 flex items-center justify-center gap-2">
                  <button
                    @click="openAddModal"
                    class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-semibold cursor-pointer"
                  >
                    Add Server Manual
                  </button>
                  <button
                    @click="openSyncModal"
                    class="px-3 py-1.5 bg-slate-100 dark:bg-[#161d2d] text-slate-700 dark:text-slate-200 rounded-lg text-xs font-semibold border border-slate-300 dark:border-[#222c42] cursor-pointer"
                  >
                    Sync from Remote Host
                  </button>
                </div>
              </td>
            </tr>

            <tr
              v-for="item in inventoryList"
              :key="item.id"
              class="hover:bg-slate-50/80 dark:hover:bg-[#131926]/50 transition"
            >
              <!-- 1. Server Name -->
              <td class="px-3.5 py-3 font-semibold text-slate-900 dark:text-white break-words whitespace-normal min-w-[150px]">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span>{{ item.serverName }}</span>
                  <span
                    v-if="item.remoteHostId"
                    class="px-1.5 py-0.2 rounded text-[10px] bg-slate-100 dark:bg-[#161d2d] text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-[#222c42]"
                    title="Linked to Remote Host"
                  >
                    SSH
                  </span>
                </div>
              </td>

              <!-- 2. IP Address -->
              <td class="px-3.5 py-3 font-mono text-[11px] text-slate-600 dark:text-slate-300 whitespace-nowrap min-w-[130px]">
                <div class="flex items-center gap-1">
                  <span>{{ item.ipAddress }}</span>
                  <button
                    @click="copyToClipboard(item.ipAddress, item.id + '-ip')"
                    class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer"
                    title="Copy IP Address"
                  >
                    <Check v-if="copiedId === item.id + '-ip'" class="w-3 h-3 text-emerald-500" />
                    <Copy v-else class="w-3 h-3" />
                  </button>
                </div>
              </td>

              <!-- 3. OS Version -->
              <td class="px-3.5 py-3 text-slate-800 dark:text-slate-200 break-words whitespace-normal min-w-[160px]">
                {{ item.osVersion }}
              </td>

              <!-- 4. OS Type -->
              <td class="px-3.5 py-3 text-slate-600 dark:text-slate-400 whitespace-nowrap min-w-[90px]">
                {{ item.osType }}
              </td>

              <!-- 5. Architecture Type -->
              <td class="px-3.5 py-3 text-slate-600 dark:text-slate-400 font-mono text-[11px] whitespace-nowrap min-w-[110px]">
                {{ item.architectureType }}
              </td>

              <!-- 6. Processor Model -->
              <td class="px-3.5 py-3 text-slate-800 dark:text-slate-200 break-words whitespace-normal min-w-[220px]">
                {{ item.processorModel }}
              </td>

              <!-- 7. Total Cores -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 font-mono text-[11px] whitespace-nowrap min-w-[90px]">
                {{ item.totalCore }}
              </td>

              <!-- 8. Total Memory -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 font-mono text-[11px] whitespace-nowrap min-w-[110px]">
                {{ item.totalMemory }}
              </td>

              <!-- 9. DIMM Slots -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 break-words whitespace-normal min-w-[120px]">
                {{ item.totalDimmMemory }}
              </td>

              <!-- 10. Storage Size -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 font-mono text-[11px] whitespace-nowrap min-w-[110px]">
                {{ item.totalStorageSize }}
              </td>

              <!-- 11. Storage Disks -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 break-words whitespace-normal min-w-[160px]">
                {{ item.totalDiskCount }}
              </td>

              <!-- 12. Network Interfaces -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 font-mono text-[11px] break-words whitespace-normal min-w-[160px]">
                {{ item.totalNetworkInterfaces }}
              </td>

              <!-- 13. GPU Model -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 break-words whitespace-normal min-w-[170px]">
                {{ item.gpuModel }}
              </td>

              <!-- 14. GPU Type -->
              <td class="px-3.5 py-3 text-slate-600 dark:text-slate-400 whitespace-nowrap min-w-[100px]">
                {{ item.gpuType }}
              </td>

              <!-- 15. Total VRAM -->
              <td class="px-3.5 py-3 text-slate-700 dark:text-slate-300 whitespace-nowrap min-w-[100px]">
                {{ item.totalVram }}
              </td>

              <!-- 16. Status -->
              <td class="px-3.5 py-3 whitespace-nowrap min-w-[100px]">
                <span
                  :class="[
                    'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase',
                    item.status === 'active'
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : item.status === 'maintenance'
                      ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400'
                      : 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
                  ]"
                >
                  <span
                    :class="[
                      'w-1.5 h-1.5 rounded-full',
                      item.status === 'active' ? 'bg-emerald-500' : item.status === 'maintenance' ? 'bg-amber-500' : 'bg-rose-500',
                    ]"
                  ></span>
                  <span>{{ item.status }}</span>
                </span>
              </td>

              <!-- 17. Last Synced -->
              <td class="px-3.5 py-3 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap min-w-[130px]">
                {{ formatDate(item.lastSyncedAt) }}
              </td>

              <!-- 18. Actions (Sticky Right) -->
              <td class="px-3.5 py-3 text-right whitespace-nowrap sticky right-0 bg-white dark:bg-[#0e121c] z-10 shadow-[-4px_0_6px_-2px_rgba(0,0,0,0.06)] min-w-[130px] border-l border-slate-100 dark:border-[#192132]">
                <div class="inline-flex items-center gap-1">
                  <button
                    @click="openDetail(item)"
                    class="p-1 text-slate-500 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1a2336] transition cursor-pointer"
                    title="View Full Specifications"
                  >
                    <Eye class="w-4 h-4" />
                  </button>

                  <button
                    v-if="item.remoteHostId"
                    @click="triggerSyncRemote(item.remoteHostId, item.serverName)"
                    :disabled="syncingHostId === item.remoteHostId"
                    class="p-1 text-slate-500 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1a2336] transition cursor-pointer disabled:opacity-50"
                    title="Re-probe SSH specifications now"
                  >
                    <Radio :class="['w-4 h-4', syncingHostId === item.remoteHostId ? 'animate-pulse' : '']" />
                  </button>

                  <button
                    @click="openEditModal(item)"
                    class="p-1 text-slate-500 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1a2336] transition cursor-pointer"
                    title="Edit Specifications"
                  >
                    <Edit2 class="w-4 h-4" />
                  </button>

                  <button
                    @click="promptDelete(item)"
                    class="p-1 text-slate-500 hover:text-rose-600 rounded hover:bg-slate-100 dark:hover:bg-[#1a2336] transition cursor-pointer"
                    title="Delete Server"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- MODAL: ADD / EDIT SERVER MANUAL -->
    <div
      v-if="showAddModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-2xl shadow-2xl p-6 space-y-4 max-h-[90vh] overflow-y-auto">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-sm font-bold text-slate-900 dark:text-white">
            {{ isEditing ? 'Edit Server Specifications' : 'Add Server to Inventory' }}
          </h2>
          <button @click="showAddModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
          <!-- Server Name -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Server Name *</label>
            <input
              v-model="formData.serverName"
              type="text"
              placeholder="e.g. prod-db-01"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- IP Address -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">IP Address *</label>
            <input
              v-model="formData.ipAddress"
              type="text"
              placeholder="e.g. 192.168.1.10"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Linked Remote Host -->
          <div class="md:col-span-2">
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Link to Remote Host (Optional SSH Sync)</label>
            <select
              v-model="formData.remoteHostId"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-blue-500"
            >
              <option value="">None (Standalone Manual Server)</option>
              <option v-for="host in remoteHosts" :key="host.id" :value="host.id">
                {{ host.name }} ({{ host.host }}:{{ host.port }})
              </option>
            </select>
          </div>

          <!-- OS Version -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">OS Version</label>
            <input
              v-model="formData.osVersion"
              type="text"
              placeholder="e.g. Ubuntu 22.04.4 LTS"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- OS Type -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">OS Type</label>
            <select
              v-model="formData.osType"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-blue-500"
            >
              <option value="Linux">Linux</option>
              <option value="Windows">Windows</option>
              <option value="FreeBSD">FreeBSD</option>
              <option value="Other">Other</option>
            </select>
          </div>

          <!-- Architecture -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Architecture Type</label>
            <input
              v-model="formData.architectureType"
              type="text"
              placeholder="e.g. x86_64, aarch64"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Status -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Status</label>
            <select
              v-model="formData.status"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-blue-500"
            >
              <option value="active">Active</option>
              <option value="maintenance">Maintenance</option>
              <option value="offline">Offline</option>
            </select>
          </div>

          <!-- Processor Model -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Processor Model</label>
            <input
              v-model="formData.processorModel"
              type="text"
              placeholder="e.g. Intel Xeon Gold 6330 @ 2.00GHz"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Total Cores -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Total Core</label>
            <input
              v-model="formData.totalCore"
              type="text"
              placeholder="e.g. 16 Cores"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Total Memory -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Total Memory</label>
            <input
              v-model="formData.totalMemory"
              type="text"
              placeholder="e.g. 64.00 GB"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Total DIMM Memory -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Total DIMM Memory</label>
            <input
              v-model="formData.totalDimmMemory"
              type="text"
              placeholder="e.g. 4 DIMMs (or N/A)"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Total Storage Size -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Total Size Storage Disk</label>
            <input
              v-model="formData.totalStorageSize"
              type="text"
              placeholder="e.g. 2.00 TB"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Total Disk Count & List -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Total Disk Listing</label>
            <input
              v-model="formData.totalDiskCount"
              type="text"
              placeholder="e.g. 2 Disks [sda (1T), sdb (1T)]"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Network Interfaces -->
          <div class="md:col-span-2">
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Total Interface Network</label>
            <input
              v-model="formData.totalNetworkInterfaces"
              type="text"
              placeholder="e.g. 4 Interfaces [eth0, eth1, docker0, lo]"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- GPU Model -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">GPU Model</label>
            <input
              v-model="formData.gpuModel"
              type="text"
              placeholder="e.g. NVIDIA RTX 4090 (or N/A)"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- GPU Type -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">GPU Type</label>
            <input
              v-model="formData.gpuType"
              type="text"
              placeholder="e.g. Discrete (NVIDIA) (or N/A)"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Total VRAM -->
          <div>
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Total VRAM</label>
            <input
              v-model="formData.totalVram"
              type="text"
              placeholder="e.g. 24576 MiB (or N/A)"
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <!-- Notes -->
          <div class="md:col-span-2">
            <label class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">Notes / Description</label>
            <textarea
              v-model="formData.notes"
              rows="2"
              placeholder="Optional notes or rack location..."
              class="w-full px-3 py-2 rounded-lg bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1f283d] text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            ></textarea>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 border-t border-slate-200 dark:border-[#1b2234] pt-4">
          <button
            @click="showAddModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="saveServer"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-semibold cursor-pointer transition shadow-sm"
          >
            {{ isEditing ? 'Save Changes' : 'Create Server' }}
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: SYNC FROM REMOTE HOSTS -->
    <div
      v-if="showSyncModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-xl shadow-2xl p-6 space-y-4 max-h-[85vh] overflow-y-auto">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div>
            <h2 class="text-sm font-bold text-slate-900 dark:text-white">Sync from Remote Servers</h2>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">
              Discovers hardware specs, CPU cores, memory DIMMs, disks, and GPU via SSH.
            </p>
          </div>
          <button @click="showSyncModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="flex items-center justify-between bg-slate-50 dark:bg-[#141824] p-3 rounded-xl border border-slate-200 dark:border-[#1f283d]">
          <div>
            <p class="text-xs font-semibold text-slate-800 dark:text-slate-200">Sync All Available Hosts</p>
            <p class="text-[11px] text-slate-500">Run parallel discovery probe across all {{ remoteHosts.length }} registered SSH servers</p>
          </div>
          <button
            @click="triggerSyncAll"
            :disabled="syncingAll || remoteHosts.length === 0"
            class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-semibold cursor-pointer transition shadow-sm disabled:opacity-50"
          >
            {{ syncingAll ? 'Syncing All...' : `Sync All (${remoteHosts.length})` }}
          </button>
        </div>

        <div class="space-y-2">
          <p class="text-xs font-semibold text-slate-700 dark:text-slate-300">Registered Remote Hosts ({{ remoteHosts.length }})</p>
          <div v-if="remoteHosts.length === 0" class="text-center py-6 text-xs text-slate-500">
            No remote hosts configured yet. Please add connections in the Remote Host menu first.
          </div>
          <div
            v-for="host in remoteHosts"
            :key="host.id"
            class="flex items-center justify-between p-3 rounded-xl border border-slate-100 dark:border-[#182030] bg-slate-50/50 dark:bg-[#141824]/50"
          >
            <div>
              <div class="flex items-center gap-2">
                <p class="text-xs font-semibold text-slate-900 dark:text-white">{{ host.name }}</p>
                <span
                  v-if="getHostInventoryStatus(host)"
                  :class="[
                    'text-[9px] px-1.5 py-0.5 rounded font-semibold uppercase',
                    getHostInventoryStatus(host)?.status === 'active'
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                  ]"
                >
                  {{ getHostInventoryStatus(host)?.status }}
                </span>
                <span
                  v-else
                  class="text-[9px] px-1.5 py-0.5 rounded font-semibold uppercase bg-slate-200/60 dark:bg-slate-800 text-slate-500"
                >
                  Not In Inventory
                </span>
              </div>
              <p class="text-[11px] text-slate-500 font-mono mt-0.5">{{ host.username }}@{{ host.host }}:{{ host.port }}</p>
            </div>
            <button
              @click="triggerSyncRemote(host.id, host.name)"
              :disabled="syncingHostId === host.id"
              class="px-3 py-1 bg-slate-200 dark:bg-[#1a2336] hover:bg-slate-300 dark:hover:bg-[#222c42] text-slate-800 dark:text-slate-200 rounded-lg text-xs font-semibold transition cursor-pointer disabled:opacity-50"
            >
              {{ syncingHostId === host.id ? 'Probing...' : 'Probe Specs' }}
            </button>
          </div>
        </div>

        <div class="flex justify-end pt-2 border-t border-slate-200 dark:border-[#1b2234]">
          <button
            @click="showSyncModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: BULK CSV IMPORT -->
    <div
      v-if="showImportModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div>
            <h2 class="text-sm font-bold text-slate-900 dark:text-white">Bulk Import Servers (CSV)</h2>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">
              Upload a CSV file containing multiple server hardware profiles.
            </p>
          </div>
          <button @click="showImportModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="space-y-3">
          <div class="border-2 border-dashed border-slate-200 dark:border-[#222c42] rounded-xl p-6 text-center hover:border-blue-500 transition cursor-pointer">
            <FileSpreadsheet class="w-8 h-8 mx-auto text-slate-400 mb-2" />
            <p class="text-xs font-medium text-slate-800 dark:text-slate-200">
              Select or drop your CSV file here
            </p>
            <p class="text-[10px] text-slate-400 mt-0.5">Maximum file size: 10MB</p>
            <input
              type="file"
              accept=".csv,text/csv"
              @change="onFileSelected"
              class="mt-3 text-xs text-slate-500 file:mr-3 file:py-1 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-semibold file:bg-blue-50 file:text-blue-700 dark:file:bg-[#1a2336] dark:file:text-slate-200 hover:file:bg-blue-100 cursor-pointer"
            />
          </div>

          <div class="flex items-center justify-between text-xs text-slate-500">
            <span>Need the official template?</span>
            <button
              @click="downloadTemplate"
              class="text-blue-600 dark:text-blue-400 hover:underline font-semibold cursor-pointer inline-flex items-center gap-1"
            >
              <Download class="w-3 h-3" />
              <span>Download CSV Template</span>
            </button>
          </div>

          <!-- Result Feedback -->
          <div
            v-if="importResult"
            class="p-3 rounded-xl border bg-slate-50 dark:bg-[#141824] border-slate-200 dark:border-[#1f283d] text-xs space-y-1.5"
          >
            <p class="font-semibold text-slate-800 dark:text-slate-200">Import Summary:</p>
            <div class="grid grid-cols-3 gap-2 text-[11px] text-center">
              <div class="p-2 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 rounded-lg">
                <span class="block font-bold">{{ importResult.createdCount }}</span>
                <span>Created</span>
              </div>
              <div class="p-2 bg-blue-500/10 text-blue-600 dark:text-blue-400 rounded-lg">
                <span class="block font-bold">{{ importResult.updatedCount }}</span>
                <span>Updated</span>
              </div>
              <div class="p-2 bg-rose-500/10 text-rose-600 dark:text-rose-400 rounded-lg">
                <span class="block font-bold">{{ importResult.failedCount }}</span>
                <span>Failed</span>
              </div>
            </div>

            <div v-if="importResult.errors && importResult.errors.length > 0" class="mt-2 text-[11px] text-rose-500 max-h-24 overflow-y-auto space-y-1">
              <p v-for="(err, idx) in importResult.errors" :key="idx">• {{ err }}</p>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 border-t border-slate-200 dark:border-[#1b2234] pt-4">
          <button
            @click="showImportModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="submitImportCSV"
            :disabled="importing || !importFile"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-semibold cursor-pointer transition shadow-sm disabled:opacity-50"
          >
            {{ importing ? 'Importing...' : 'Upload & Import' }}
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: SERVER SPECIFICATIONS DETAIL DRAWER -->
    <div
      v-if="showDetailModal && selectedDetailItem"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-2xl shadow-2xl p-6 space-y-5 max-h-[90vh] overflow-y-auto">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-white">
              {{ selectedDetailItem.serverName }}
            </h2>
            <p class="text-xs text-slate-500 font-mono">{{ selectedDetailItem.ipAddress }}</p>
          </div>
          <button @click="showDetailModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
          <!-- OS & Platform -->
          <div class="bg-slate-50 dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200/60 dark:border-[#1f283d] space-y-2">
            <p class="font-semibold text-slate-700 dark:text-slate-300">Operating System</p>
            <div class="space-y-1 text-slate-600 dark:text-slate-400">
              <div class="flex justify-between">
                <span>Distribution:</span>
                <span class="font-medium text-slate-900 dark:text-white text-right">{{ selectedDetailItem.osVersion }}</span>
              </div>
              <div class="flex justify-between">
                <span>Kernel / Type:</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.osType }}</span>
              </div>
              <div class="flex justify-between">
                <span>Architecture:</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.architectureType }}</span>
              </div>
            </div>
          </div>

          <!-- Compute & Processor -->
          <div class="bg-slate-50 dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200/60 dark:border-[#1f283d] space-y-2">
            <p class="font-semibold text-slate-700 dark:text-slate-300">CPU & Compute</p>
            <div class="space-y-1 text-slate-600 dark:text-slate-400">
              <div class="flex justify-between">
                <span>Model:</span>
                <span class="font-medium text-slate-900 dark:text-white text-right max-w-[180px] truncate" :title="selectedDetailItem.processorModel">
                  {{ selectedDetailItem.processorModel }}
                </span>
              </div>
              <div class="flex justify-between">
                <span>Total Cores:</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.totalCore }}</span>
              </div>
            </div>
          </div>

          <!-- Memory & RAM -->
          <div class="bg-slate-50 dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200/60 dark:border-[#1f283d] space-y-2">
            <p class="font-semibold text-slate-700 dark:text-slate-300">Memory & DIMMs</p>
            <div class="space-y-1 text-slate-600 dark:text-slate-400">
              <div class="flex justify-between">
                <span>Total RAM:</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.totalMemory }}</span>
              </div>
              <div class="flex justify-between">
                <span>Total DIMM Slots:</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.totalDimmMemory }}</span>
              </div>
            </div>
          </div>

          <!-- Storage Disks -->
          <div class="bg-slate-50 dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200/60 dark:border-[#1f283d] space-y-2">
            <p class="font-semibold text-slate-700 dark:text-slate-300">Storage & Disks</p>
            <div class="space-y-1 text-slate-600 dark:text-slate-400">
              <div class="flex justify-between">
                <span>Total Storage Size:</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.totalStorageSize }}</span>
              </div>
              <div class="flex justify-between">
                <span>Disk Listing:</span>
                <span class="font-medium text-slate-900 dark:text-white text-right max-w-[180px] truncate" :title="selectedDetailItem.totalDiskCount">
                  {{ selectedDetailItem.totalDiskCount }}
                </span>
              </div>
            </div>
          </div>

          <!-- Network Interfaces -->
          <div class="md:col-span-2 bg-slate-50 dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200/60 dark:border-[#1f283d] space-y-2">
            <p class="font-semibold text-slate-700 dark:text-slate-300">Network Interfaces</p>
            <p class="text-slate-800 dark:text-slate-200 font-mono text-[11px] leading-relaxed">
              {{ selectedDetailItem.totalNetworkInterfaces }}
            </p>
          </div>

          <!-- GPU & Acceleration -->
          <div class="md:col-span-2 bg-slate-50 dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200/60 dark:border-[#1f283d] space-y-2">
            <p class="font-semibold text-slate-700 dark:text-slate-300">GPU & Acceleration</p>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 text-slate-600 dark:text-slate-400">
              <div>
                <span class="block text-[10px] uppercase text-slate-400">GPU Model</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.gpuModel }}</span>
              </div>
              <div>
                <span class="block text-[10px] uppercase text-slate-400">GPU Type</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.gpuType }}</span>
              </div>
              <div>
                <span class="block text-[10px] uppercase text-slate-400">Total VRAM</span>
                <span class="font-medium text-slate-900 dark:text-white">{{ selectedDetailItem.totalVram }}</span>
              </div>
            </div>
          </div>
        </div>

        <div v-if="selectedDetailItem.notes" class="text-xs text-slate-500 bg-slate-50 dark:bg-[#141824] p-3 rounded-xl border border-slate-200/60 dark:border-[#1f283d]">
          <span class="font-semibold text-slate-700 dark:text-slate-300">Notes:</span>
          <p class="mt-0.5">{{ selectedDetailItem.notes }}</p>
        </div>

        <div class="flex items-center justify-end gap-2 border-t border-slate-200 dark:border-[#1b2234] pt-4">
          <button
            @click="showDetailModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Close
          </button>
          <button
            @click="openEditModal(selectedDetailItem); showDetailModal = false"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-semibold cursor-pointer transition shadow-sm"
          >
            Edit Specifications
          </button>
        </div>
      </div>
    </div>

    <!-- STANDARD DELETE CONFIRMATION MODAL (AGENTS.md Strict Standard) -->
    <div
      v-if="showDeleteModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Icon Lingkaran Merah di Tengah Atas -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <!-- Judul & Teks Penjelasan -->
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Server Inventory?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ deleteTargetItem?.serverName }}</strong> ({{ deleteTargetItem?.ipAddress }})? This action cannot be undone.
          </p>
        </div>

        <!-- Tombol Aksi -->
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
