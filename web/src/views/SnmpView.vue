<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import axios from 'axios';
import {
  Radio,
  Search,
  Upload,
  Trash2,
  Filter,
  Info,
  AlertTriangle,
  X,
  Copy,
  Check,
  Download,
  RefreshCw,
  Play,
  Layers,
  Activity,
  Cpu,
  HardDrive,
  Zap,
  Network,
  Globe,
  FileText,
  Sliders,
  CheckCircle2,
  XCircle,
  AlertCircle
} from 'lucide-vue-next';

// ==================== TABS & NAVIGATION ====================
type ActiveTab = 'discovery' | 'subnet' | 'walk' | 'mibs';
const activeTab = ref<ActiveTab>('discovery');

// Notification banner
const notification = ref<{ type: 'success' | 'error'; message: string } | null>(null);
const showNotification = (type: 'success' | 'error', message: string) => {
  notification.value = { type, message };
  setTimeout(() => {
    notification.value = null;
  }, 3000);
};

// Clipboard helper
const copiedOid = ref<string | null>(null);
const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text);
    copiedOid.value = text;
    setTimeout(() => {
      copiedOid.value = null;
    }, 2000);
  } catch (e) {
    console.error(e);
  }
};

// ==================== DISCOVERY SCAN STATE ====================
interface DiscoveryProfile {
  id: string;
  name: string;
  description: string;
  modules: string[];
}

const profiles = ref<DiscoveryProfile[]>([
  {
    id: 'provisioning',
    name: 'Provisioning (Fast)',
    description: 'Fast production scan for device identity, basic interface speeds, and optics.',
    modules: ['system', 'interfaces', 'optical_dom', 'environment']
  },
  {
    id: 'network',
    name: 'Network & Interfaces',
    description: 'Full network scan: interface states, 64-bit HC traffic counters, drops, and optical DOM.',
    modules: ['system', 'interfaces', 'interface_stats', 'optical_dom']
  },
  {
    id: 'optical_dom',
    name: 'Optical DOM / Transceivers',
    description: 'Targeted SFP/XFP/QSFP transceiver diagnostics: RX/TX Power, Temperature, Voltage, and Bias Current.',
    modules: ['optical_dom', 'gpon_optics']
  },
  {
    id: 'router_switch',
    name: 'Router & Switch',
    description: 'Comprehensive switch/router telemetry: Interfaces, Bandwidth usage, Optical DOM, CPU, Memory, and Fans.',
    modules: ['system', 'interfaces', 'interface_stats', 'optical_dom', 'cpu', 'memory', 'environment']
  },
  {
    id: 'system',
    name: 'System & Resources',
    description: 'Server and OS metrics: CPU cores, RAM, Swap, Storage/Disks, Processes, and Uptime.',
    modules: ['system', 'cpu', 'memory', 'storage']
  },
  {
    id: 'olt',
    name: 'OLT & Telecom GPON',
    description: 'GPON OLT and ONT telemetry: PON ports, ONT optical receive power, and OLT transceivers.',
    modules: ['system', 'optical_dom', 'gpon_optics']
  },
  {
    id: 'rectifier',
    name: 'Rectifier & Power Systems',
    description: 'DC power system scan: AC Phase Voltages, Battery Voltage/Current, System Power, and Load Current.',
    modules: ['system', 'rectifier', 'environment']
  },
  {
    id: 'printer',
    name: 'Printer & Consumables',
    description: 'Targeted printer scan: Supplies, Toner/Ink levels, Paper trays, and Page counters.',
    modules: ['system', 'printer']
  },
  {
    id: 'full',
    name: 'Full Comprehensive Scan',
    description: 'Runs all discovery modules across hardware, optics, interfaces, and environmental sensors.',
    modules: ['system', 'interfaces', 'interface_stats', 'optical_dom', 'cpu', 'memory', 'storage', 'environment', 'gpon_optics', 'rectifier']
  }
]);

const discoveryForm = ref({
  host: '192.168.1.1',
  port: 161,
  version: '2c',
  community: 'public',
  profile: 'provisioning',
  timeout: 6,
  retries: 2,
});

const discoveryLoading = ref(false);
const discoveryError = ref<string | null>(null);
const discoveryResult = ref<any | null>(null);
const sensorFilterSearch = ref('');
const selectedSensorClass = ref<string>('all');

const selectedProfileInfo = computed(() => {
  return profiles.value.find(p => p.id === discoveryForm.value.profile);
});

const runDiscovery = async () => {
  discoveryLoading.value = true;
  discoveryError.value = null;
  discoveryResult.value = null;
  sensorFilterSearch.value = '';
  selectedSensorClass.value = 'all';

  try {
    const res = await axios.post('/api/v1/snmp/discover', discoveryForm.value);
    if (res.data.success) {
      discoveryResult.value = res.data.data;
      showNotification('success', `Discovery completed in ${res.data.data.durationSec}s. Found ${res.data.data.sensorCount} sensors.`);
    }
  } catch (err: any) {
    discoveryError.value = err.response?.data?.error || err.message || 'SNMP Discovery failed';
  } finally {
    discoveryLoading.value = false;
  }
};

// Filtered sensors in discovery results
const sensorClasses = computed(() => {
  if (!discoveryResult.value || !discoveryResult.value.sensors) return [];
  const set = new Set<string>();
  discoveryResult.value.sensors.forEach((s: any) => {
    if (s.sensorClass) set.add(s.sensorClass);
  });
  return Array.from(set).sort();
});

const filteredSensors = computed(() => {
  if (!discoveryResult.value || !discoveryResult.value.sensors) return [];
  let list = discoveryResult.value.sensors;

  if (selectedSensorClass.value !== 'all') {
    list = list.filter((s: any) => s.sensorClass === selectedSensorClass.value);
  }

  const q = sensorFilterSearch.value.trim().toLowerCase();
  if (q) {
    list = list.filter((s: any) =>
      (s.sensorName && s.sensorName.toLowerCase().includes(q)) ||
      (s.oid && s.oid.toLowerCase().includes(q)) ||
      (s.oidName && s.oidName.toLowerCase().includes(q)) ||
      (s.interfaceName && s.interfaceName.toLowerCase().includes(q)) ||
      (s.sensorType && s.sensorType.toLowerCase().includes(q)) ||
      (s.rawValue && String(s.rawValue).toLowerCase().includes(q))
    );
  }

  return list;
});

// Sensor count helper for tabs
const getSensorCountByClass = (c: string) => {
  if (!discoveryResult.value || !discoveryResult.value.sensors) return 0;
  if (c === 'all') return discoveryResult.value.sensors.length;
  return discoveryResult.value.sensors.filter((s: any) => s.sensorClass === c).length;
};

// Export Discovery to CSV or JSON
const exportDiscovery = (format: 'json' | 'csv') => {
  if (!discoveryResult.value) return;
  let dataStr = '';
  let filename = `snmp_discovery_${discoveryForm.value.host}_${discoveryForm.value.profile}`;

  if (format === 'json') {
    dataStr = 'data:text/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(discoveryResult.value, null, 2));
    filename += '.json';
  } else {
    const headers = ['Sensor Class', 'Sensor Name', 'Sensor Type', 'Interface', 'Reading / Value', 'SNMP OID', 'OID Translate', 'Raw Value'];
    const rows = (discoveryResult.value.sensors || []).map((s: any) => {
      const valStr = s.metadata?.display || (s.normalizedValue !== undefined ? `${s.normalizedValue}${s.unit ? ' ' + s.unit : ''}` : `${s.rawValue || ''}${s.unit ? ' ' + s.unit : ''}`);
      return [
        `"${s.sensorClass || ''}"`,
        `"${s.sensorName || ''}"`,
        `"${s.sensorType || ''}"`,
        `"${s.interfaceName || ''}"`,
        `"${valStr}"`,
        `"${s.oid || ''}"`,
        `"${s.oidName || ''}"`,
        `"${s.rawValue || ''}"`
      ];
    });
    const csvContent = [headers.join(','), ...rows.map(r => r.join(','))].join('\n');
    dataStr = 'data:text/csv;charset=utf-8,' + encodeURIComponent(csvContent);
    filename += '.csv';
  }

  const downloadAnchor = document.createElement('a');
  downloadAnchor.setAttribute('href', dataStr);
  downloadAnchor.setAttribute('download', filename);
  document.body.appendChild(downloadAnchor);
  downloadAnchor.click();
  downloadAnchor.remove();
};

// ==================== SUBNET SCANNER STATE ====================
const subnetForm = ref({
  subnet: '192.168.1.0/24',
  community: 'public',
  version: '2c',
  port: 161,
  timeout: 2,
  retries: 1,
});

const subnetLoading = ref(false);
const subnetError = ref<string | null>(null);
const subnetResult = ref<any | null>(null);
const subnetFilterSearch = ref('');

const runSubnetScan = async () => {
  subnetLoading.value = true;
  subnetError.value = null;
  subnetResult.value = null;
  subnetFilterSearch.value = '';

  try {
    const res = await axios.post('/api/v1/snmp/subnet-scan', subnetForm.value);
    if (res.data.success) {
      subnetResult.value = res.data.data;
      showNotification('success', `Subnet scan completed. Found ${res.data.data.activeHosts} active SNMP devices.`);
    }
  } catch (err: any) {
    subnetError.value = err.response?.data?.error || err.message || 'Subnet scan failed';
  } finally {
    subnetLoading.value = false;
  }
};

const filteredSubnetHosts = computed(() => {
  if (!subnetResult.value || !subnetResult.value.hosts) return [];
  const q = subnetFilterSearch.value.trim().toLowerCase();
  if (!q) return subnetResult.value.hosts;
  return subnetResult.value.hosts.filter((h: any) =>
    (h.ipAddress && h.ipAddress.toLowerCase().includes(q)) ||
    (h.hostname && h.hostname.toLowerCase().includes(q)) ||
    (h.vendor && h.vendor.toLowerCase().includes(q)) ||
    (h.sysDescr && h.sysDescr.toLowerCase().includes(q))
  );
});

// Quick action from Subnet Scanner: load into Discovery Telemetry
const inspectHostFromSubnet = (host: string) => {
  discoveryForm.value.host = host;
  discoveryForm.value.community = subnetForm.value.community;
  discoveryForm.value.version = subnetForm.value.version;
  discoveryForm.value.port = subnetForm.value.port;
  activeTab.value = 'discovery';
  runDiscovery();
};

// ==================== RAW WALK & QUERY STATE ====================
const queryForm = ref({
  host: '192.168.1.1',
  port: 161,
  version: '2c',
  community: 'public',
  oid: '*',
  operation: 'walk',
  timeout: 6,
  retries: 3,
});

const presets = [
  { label: 'All OIDs / Entire Tree (*)', oid: '*', op: 'walk' },
  { label: 'System Subtree (Walk)', oid: '1.3.6.1.2.1.1', op: 'walk' },
  { label: 'Interfaces Table (Walk)', oid: '1.3.6.1.2.1.2', op: 'walk' },
  { label: 'IP Address Table (Walk)', oid: '1.3.6.1.2.1.4', op: 'walk' },
  { label: 'Enterprise Vendor Subtree (Walk)', oid: '1.3.6.1.4.1', op: 'walk' },
  { label: 'Host Resources Subtree (Walk)', oid: '1.3.6.1.2.1.25', op: 'walk' },
  { label: 'sysDescr.0 (Get)', oid: '1.3.6.1.2.1.1.1.0', op: 'get' },
  { label: 'sysUpTime.0 (Get)', oid: '1.3.6.1.2.1.1.3.0', op: 'get' },
  { label: 'sysName.0 (Get)', oid: '1.3.6.1.2.1.1.5.0', op: 'get' },
];

const queryResults = ref<any[]>([]);
const queryLoading = ref(false);
const queryError = ref<string | null>(null);
const querySearch = ref('');
const showQueryAdvanced = ref(false);

const applyPreset = (preset: typeof presets[0]) => {
  queryForm.value.oid = preset.oid;
  queryForm.value.operation = preset.op;
};

const executeQuery = async () => {
  queryLoading.value = true;
  queryError.value = null;
  queryResults.value = [];
  querySearch.value = '';

  try {
    const res = await axios.post('/api/v1/snmp/query', queryForm.value);
    if (res.data.success) {
      queryResults.value = res.data.data || [];
      if (queryResults.value.length === 0) {
        queryError.value = 'Query returned 0 OID records. The agent may not support this OID subtree or returned an empty table.';
      }
    }
  } catch (err: any) {
    queryError.value = err.response?.data?.error || err.message || 'SNMP Query failed';
  } finally {
    queryLoading.value = false;
  }
};

const filteredQueryResults = computed(() => {
  const q = querySearch.value.trim().toLowerCase();
  if (!q) return queryResults.value;
  return queryResults.value.filter(r =>
    (r.name && r.name.toLowerCase().includes(q)) ||
    (r.oid && r.oid.toLowerCase().includes(q)) ||
    (r.value && String(r.value).toLowerCase().includes(q))
  );
});

// ==================== MIB REGISTRY & TRANSLATOR STATE ====================
const mibs = ref<any[]>([]);
const mibsLoading = ref(false);
const uploadModalOpen = ref(false);
const uploadName = ref('');
const uploadContent = ref('');
const uploadingMib = ref(false);

// Delete confirmation modal state
const deleteModalOpen = ref(false);
const mibToDelete = ref<string | null>(null);
const deletingMib = ref(false);

const fetchMibs = async () => {
  mibsLoading.value = true;
  try {
    const res = await axios.get('/api/v1/snmp/mibs');
    if (res.data.success) {
      mibs.value = res.data.data || [];
    }
  } catch (err) {
    console.error(err);
  } finally {
    mibsLoading.value = false;
  }
};

const confirmDeleteMib = (name: string) => {
  mibToDelete.value = name;
  deleteModalOpen.value = true;
};

const executeDeleteMib = async () => {
  if (!mibToDelete.value) return;
  deletingMib.value = true;
  try {
    const res = await axios.delete(`/api/v1/snmp/mibs/${mibToDelete.value}`);
    if (res.data.success) {
      showNotification('success', `MIB module '${mibToDelete.value}' deleted successfully.`);
      deleteModalOpen.value = false;
      mibToDelete.value = null;
      fetchMibs();
    }
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || 'Failed to delete MIB');
  } finally {
    deletingMib.value = false;
  }
};

const handleMibFileUpload = (event: any) => {
  const file = event.target.files?.[0];
  if (!file) return;
  uploadName.value = file.name.replace(/\.[^/.]+$/, '');
  const reader = new FileReader();
  reader.onload = (e) => {
    uploadContent.value = String(e.target?.result || '');
  };
  reader.readAsText(file);
};

const submitMibUpload = async () => {
  if (!uploadName.value.trim() || !uploadContent.value.trim()) {
    showNotification('error', 'Please provide MIB name and content.');
    return;
  }
  uploadingMib.value = true;
  try {
    const res = await axios.post('/api/v1/snmp/mibs', {
      name: uploadName.value.trim(),
      content: uploadContent.value
    });
    if (res.data.success) {
      showNotification('success', `MIB '${uploadName.value}' uploaded and compiled successfully.`);
      uploadModalOpen.value = false;
      uploadName.value = '';
      uploadContent.value = '';
      fetchMibs();
    }
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || 'Failed to upload MIB');
  } finally {
    uploadingMib.value = false;
  }
};

// OID Translator State
const translateOidInput = ref('1.3.6.1.2.1.1.1.0');
const translateResult = ref<any | null>(null);
const translateLoading = ref(false);

const runTranslate = async () => {
  if (!translateOidInput.value.trim()) return;
  translateLoading.value = true;
  try {
    const res = await axios.get(`/api/v1/snmp/translate?oid=${encodeURIComponent(translateOidInput.value.trim())}`);
    if (res.data.success) {
      translateResult.value = res.data.data;
    }
  } catch (err: any) {
    console.error(err);
  } finally {
    translateLoading.value = false;
  }
};

onMounted(() => {
  fetchMibs();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header (Strictly adhering to AGENTS.md: pure text title, no icon, no emoji) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">SNMP Browser</h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Multi-vendor SNMP telemetry discovery, optical DOM transceiver analytics, interface counters, and MIB registry.
        </p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <button
          v-if="activeTab === 'discovery' && discoveryResult"
          @click="exportDiscovery('csv')"
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer"
        >
          <Download class="w-3.5 h-3.5 text-slate-500" />
          <span>Export CSV</span>
        </button>
        <button
          v-if="activeTab === 'discovery' && discoveryResult"
          @click="exportDiscovery('json')"
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer"
        >
          <Download class="w-3.5 h-3.5 text-slate-500" />
          <span>Export JSON</span>
        </button>
        <button
          v-if="activeTab === 'mibs'"
          @click="uploadModalOpen = true"
          class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition flex items-center gap-1.5 shadow-sm cursor-pointer"
        >
          <Upload class="w-3.5 h-3.5" />
          <span>Import MIB</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div
      v-if="notification"
      :class="[
        'p-3 rounded-lg text-xs font-medium flex items-center gap-2 transition animate-in fade-in',
        notification.type === 'success'
          ? 'bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/50 text-emerald-800 dark:text-emerald-300'
          : 'bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-800/50 text-rose-800 dark:text-rose-300'
      ]"
    >
      <CheckCircle2 v-if="notification.type === 'success'" class="w-4 h-4 text-emerald-500 shrink-0" />
      <AlertCircle v-else class="w-4 h-4 text-rose-500 shrink-0" />
      <span>{{ notification.message }}</span>
    </div>

    <!-- Main Navigation Tabs -->
    <div class="flex items-center gap-1 border-b border-slate-200 dark:border-[#1b2234] pb-px overflow-x-auto text-xs font-semibold">
      <button
        @click="activeTab = 'discovery'"
        :class="[
          'px-4 py-2 border-b-2 transition flex items-center gap-2 cursor-pointer shrink-0',
          activeTab === 'discovery'
            ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
            : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
        ]"
      >
        <Activity class="w-4 h-4 text-slate-500" />
        <span>Device Discovery & Telemetry</span>
      </button>

      <button
        @click="activeTab = 'subnet'"
        :class="[
          'px-4 py-2 border-b-2 transition flex items-center gap-2 cursor-pointer shrink-0',
          activeTab === 'subnet'
            ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
            : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
        ]"
      >
        <Globe class="w-4 h-4 text-slate-500" />
        <span>Subnet IP Scanner</span>
      </button>

      <button
        @click="activeTab = 'walk'"
        :class="[
          'px-4 py-2 border-b-2 transition flex items-center gap-2 cursor-pointer shrink-0',
          activeTab === 'walk'
            ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
            : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
        ]"
      >
        <Radio class="w-4 h-4 text-slate-500" />
        <span>Raw OID Walk & Query</span>
      </button>

      <button
        @click="activeTab = 'mibs'"
        :class="[
          'px-4 py-2 border-b-2 transition flex items-center gap-2 cursor-pointer shrink-0',
          activeTab === 'mibs'
            ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
            : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
        ]"
      >
        <FileText class="w-4 h-4 text-slate-500" />
        <span>MIB Module Registry</span>
        <span class="ml-1 px-1.5 py-0.2 rounded-full bg-slate-100 dark:bg-[#1a2233] text-[10px] text-slate-600 dark:text-slate-300 font-mono">
          {{ mibs.length }}
        </span>
      </button>
    </div>

    <!-- ==================== TAB 1: DEVICE DISCOVERY & TELEMETRY ==================== -->
    <div v-if="activeTab === 'discovery'" class="space-y-6">
      <!-- Target Configuration Card -->
      <div class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
        <!-- Row 1: Host & SNMP Connection Credentials -->
        <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-12 gap-3.5">
          <!-- Target Host -->
          <div class="md:col-span-5">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              Target IP / Hostname
            </label>
            <input
              v-model="discoveryForm.host"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 focus:outline-none focus:border-blue-500"
              placeholder="192.168.1.1"
            />
          </div>

          <!-- Port -->
          <div class="md:col-span-2">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              Port
            </label>
            <input
              v-model.number="discoveryForm.port"
              type="number"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- SNMP Version -->
          <div class="md:col-span-2">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              SNMP Version
            </label>
            <select
              v-model="discoveryForm.version"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
            >
              <option value="2c">SNMP v2c</option>
              <option value="v1">SNMP v1</option>
            </select>
          </div>

          <!-- Community String -->
          <div class="md:col-span-3">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              Community String
            </label>
            <input
              v-model="discoveryForm.community"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 focus:outline-none focus:border-blue-500"
              placeholder="public"
            />
          </div>
        </div>

        <!-- Row 2: Profile, Timeout, Retries & Action Button -->
        <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-12 gap-3.5 pt-1">
          <!-- Discovery Profile -->
          <div class="md:col-span-6">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              Discovery Profile
            </label>
            <select
              v-model="discoveryForm.profile"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
            >
              <option v-for="p in profiles" :key="p.id" :value="p.id">
                {{ p.name }}
              </option>
            </select>
          </div>

          <!-- Timeout -->
          <div class="md:col-span-2">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              Timeout (Sec)
            </label>
            <input
              v-model.number="discoveryForm.timeout"
              type="number"
              min="2"
              max="30"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Retries -->
          <div class="md:col-span-1">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              Retries
            </label>
            <input
              v-model.number="discoveryForm.retries"
              type="number"
              min="1"
              max="5"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Execute Button -->
          <div class="md:col-span-3 flex items-end">
            <button
              @click="runDiscovery"
              :disabled="discoveryLoading"
              class="w-full py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold text-xs rounded-lg transition flex items-center justify-center gap-1.5 shadow-sm cursor-pointer"
            >
              <Play class="w-3.5 h-3.5" />
              <span>{{ discoveryLoading ? 'Scanning Telemetry...' : 'Run Discovery Scan' }}</span>
            </button>
          </div>
        </div>

        <!-- Profile Scope Note -->
        <div class="pt-2 border-t border-slate-100 dark:border-[#1b2234]/80 text-xs">
          <p class="text-[11px] text-slate-500 dark:text-slate-400">
            <span class="font-bold text-slate-700 dark:text-slate-300">Profile Scope:</span>
            {{ selectedProfileInfo?.description }}
          </p>
        </div>
      </div>

      <!-- Error Banner -->
      <div
        v-if="discoveryError"
        class="p-4 rounded-xl border border-rose-300 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/20 text-rose-800 dark:text-rose-300 text-xs space-y-2"
      >
        <div class="font-bold flex items-center gap-2 text-rose-700 dark:text-rose-400 text-sm">
          <AlertTriangle class="w-4 h-4 text-rose-500 shrink-0" />
          <span>Device Discovery Failed</span>
        </div>
        <p class="font-mono text-[11px] p-2 bg-white/60 dark:bg-black/30 rounded border border-rose-200 dark:border-rose-900/40">
          {{ discoveryError }}
        </p>
      </div>

      <!-- Discovered Device Summary KPI Banner -->
      <div v-if="discoveryResult" class="space-y-4">
        <div class="grid grid-cols-2 md:grid-cols-6 gap-3">
          <!-- Hostname & IP -->
          <div class="p-3.5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1">
            <span class="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">Device Host</span>
            <div class="text-xs font-bold text-slate-900 dark:text-white truncate">
              {{ discoveryResult.device?.hostname }}
            </div>
            <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400">
              {{ discoveryResult.device?.ipAddress }}
            </div>
          </div>

          <!-- Vendor Adapter -->
          <div class="p-3.5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1">
            <span class="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">Vendor Profile</span>
            <div class="text-xs font-bold text-slate-900 dark:text-white">
              {{ discoveryResult.vendor }}
            </div>
            <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400 truncate" :title="discoveryResult.device?.sysObjectId">
              {{ discoveryResult.device?.sysObjectId || 'Generic' }}
            </div>
          </div>

          <!-- System Uptime -->
          <div class="p-3.5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1">
            <span class="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">System Uptime</span>
            <div class="text-xs font-bold text-slate-900 dark:text-white truncate">
              {{ discoveryResult.device?.sysUptime || 'Unknown' }}
            </div>
            <div class="text-[10px] text-emerald-600 dark:text-emerald-400 font-semibold">
              Host Alive
            </div>
          </div>

          <!-- Total Sensors Discovered -->
          <div class="p-3.5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1">
            <span class="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">Total Sensors</span>
            <div class="text-lg font-bold text-slate-900 dark:text-white font-mono">
              {{ discoveryResult.sensorCount }}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">
              Across all categories
            </div>
          </div>

          <!-- Discovery Profile -->
          <div class="p-3.5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1">
            <span class="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">Scan Profile</span>
            <div class="text-xs font-bold text-slate-900 dark:text-white capitalize truncate">
              {{ discoveryResult.profile }}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">
              Duration: {{ discoveryResult.durationSec }}s
            </div>
          </div>

          <!-- System Description Preview -->
          <div class="p-3.5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1">
            <span class="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">Hardware / OS</span>
            <div class="text-[11px] text-slate-700 dark:text-slate-300 font-mono line-clamp-2" :title="discoveryResult.device?.sysDescr">
              {{ discoveryResult.device?.sysDescr || 'N/A' }}
            </div>
          </div>
        </div>

        <!-- Sensor Inventory Table & Category Filter -->
        <div class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
          <!-- Subcategory Filter Pills & Search Input -->
          <div class="flex flex-col md:flex-row md:items-center justify-between gap-3 border-b border-slate-200 dark:border-[#1b2234] pb-3">
            <div class="flex items-center gap-1.5 overflow-x-auto text-xs font-semibold">
              <button
                @click="selectedSensorClass = 'all'"
                :class="[
                  'px-3 py-1 rounded-lg text-xs font-semibold transition cursor-pointer shrink-0',
                  selectedSensorClass === 'all'
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                All ({{ getSensorCountByClass('all') }})
              </button>

              <button
                v-for="c in sensorClasses"
                :key="c"
                @click="selectedSensorClass = c"
                :class="[
                  'px-3 py-1 rounded-lg text-xs font-semibold capitalize transition cursor-pointer shrink-0',
                  selectedSensorClass === c
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                {{ c.replace('_', ' ') }} ({{ getSensorCountByClass(c) }})
              </button>
            </div>

            <!-- Search Filter Input -->
            <div class="relative w-full md:w-72">
              <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
              <input
                v-model="sensorFilterSearch"
                placeholder="Filter sensors by name, OID, type..."
                class="w-full pl-8 pr-7 py-1.5 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
              />
              <button
                v-if="sensorFilterSearch"
                @click="sensorFilterSearch = ''"
                class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Sensors Table -->
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-slate-50 dark:bg-[#121826] border-y border-slate-200 dark:border-[#1b2234] text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                <tr>
                  <th class="py-2.5 px-3">Class</th>
                  <th class="py-2.5 px-3">Sensor Component / Metric</th>
                  <th class="py-2.5 px-3">Reading / Value</th>
                  <th class="py-2.5 px-3">SNMP OID</th>
                  <th class="py-2.5 px-3">OID Translate</th>
                  <th class="py-2.5 px-3 text-right">Raw Value</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]/60">
                <tr
                  v-for="(sensor, idx) in filteredSensors"
                  :key="idx"
                  class="hover:bg-slate-50/60 dark:hover:bg-[#151c2d] transition"
                >
                  <!-- Class Badge -->
                  <td class="py-2.5 px-3 whitespace-nowrap">
                    <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-slate-100 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-800">
                      {{ sensor.sensorClass }}
                    </span>
                  </td>

                  <!-- Sensor Name & Interface -->
                  <td class="py-2.5 px-3">
                    <div class="font-bold text-slate-900 dark:text-white">
                      {{ sensor.sensorName }}
                    </div>
                    <div v-if="sensor.interfaceName" class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">
                      Port: {{ sensor.interfaceName }}
                    </div>
                  </td>

                  <!-- Calibrated Value / State -->
                  <td class="py-2.5 px-3 whitespace-nowrap">
                    <!-- Case 1: Oper status UP / DOWN badge -->
                    <span
                      v-if="sensor.sensorType === 'oper_status' || (sensor.metadata && sensor.metadata.oper_status)"
                      :class="[
                        'px-2.5 py-0.5 rounded-full text-[10px] font-bold border',
                        (sensor.rawValue === '1' || sensor.metadata?.oper_status === 'up')
                          ? 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-200 dark:border-emerald-500/30'
                          : (sensor.rawValue === '2' || sensor.metadata?.oper_status === 'down')
                            ? 'bg-rose-50 dark:bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-200 dark:border-rose-500/30'
                            : 'bg-amber-50 dark:bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-200 dark:border-amber-500/30'
                      ]"
                    >
                      {{ sensor.metadata?.display || (sensor.rawValue === '1' ? 'UP' : sensor.rawValue === '2' ? 'DOWN' : sensor.rawValue) }}
                    </span>

                    <!-- Case 2: Display string if available (e.g. speed formatted, memory formatted) -->
                    <div v-else-if="sensor.metadata && sensor.metadata.display" class="font-mono font-bold text-slate-900 dark:text-white">
                      {{ sensor.metadata.display }}
                    </div>

                    <!-- Case 3: Standard reading with unit -->
                    <div v-else class="font-mono font-bold text-slate-900 dark:text-white">
                      {{ sensor.normalizedValue !== undefined ? sensor.normalizedValue : sensor.rawValue }}
                      <span v-if="sensor.unit" class="text-[10px] font-medium text-slate-500 dark:text-slate-400 ml-0.5">{{ sensor.unit }}</span>
                    </div>
                  </td>

                  <!-- OID with Quick Copy -->
                  <td class="py-2.5 px-3 font-mono text-[11px] text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    <div class="flex items-center gap-1.5">
                      <span>{{ sensor.oid }}</span>
                      <button
                        @click="copyToClipboard(sensor.oid)"
                        class="p-1 hover:bg-slate-100 dark:hover:bg-[#1a2233] rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
                        title="Copy OID"
                      >
                        <Check v-if="copiedOid === sensor.oid" class="w-3 h-3 text-emerald-500" />
                        <Copy v-else class="w-3 h-3" />
                      </button>
                    </div>
                  </td>

                  <!-- OID Translate with Quick Copy -->
                  <td class="py-2.5 px-3 font-mono text-[11px] whitespace-nowrap">
                    <div class="flex items-center gap-1.5">
                      <span class="font-semibold text-blue-600 dark:text-blue-400">{{ sensor.oidName || '-' }}</span>
                      <button
                        v-if="sensor.oidName"
                        @click="copyToClipboard(sensor.oidName)"
                        class="p-1 hover:bg-slate-100 dark:hover:bg-[#1a2233] rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
                        title="Copy Translated OID"
                      >
                        <Check v-if="copiedOid === sensor.oidName" class="w-3 h-3 text-emerald-500" />
                        <Copy v-else class="w-3 h-3" />
                      </button>
                    </div>
                  </td>

                  <!-- Raw Value -->
                  <td class="py-2.5 px-3 font-mono text-[11px] text-slate-500 dark:text-slate-400 text-right whitespace-nowrap">
                    {{ sensor.rawValue }}
                  </td>
                </tr>
              </tbody>
            </table>

            <div v-if="filteredSensors.length === 0" class="py-12 text-center text-slate-500 dark:text-slate-400 text-xs">
              No sensors found matching filter "{{ sensorFilterSearch }}".
            </div>
          </div>
        </div>
      </div>

      <!-- Blank State for Discovery -->
      <div
        v-if="!discoveryResult && !discoveryLoading && !discoveryError"
        class="p-12 text-center bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-2"
      >
        <Activity class="w-10 h-10 text-slate-300 dark:text-slate-600 mx-auto" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">Run SNMP Telemetry Discovery</h3>
        <p class="text-xs text-slate-500 dark:text-slate-400 max-w-md mx-auto">
          Specify a target host and profile above, then click <strong>Run Discovery</strong> to automatically discover transceivers (DOM), traffic statistics, interface states, and hardware metrics.
        </p>
      </div>
    </div>

    <!-- ==================== TAB 2: SUBNET IP SCANNER ==================== -->
    <div v-if="activeTab === 'subnet'" class="space-y-6">
      <div class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-12 gap-3.5">
          <!-- CIDR Input -->
          <div class="md:col-span-4">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              CIDR Subnet or IP Range
            </label>
            <input
              v-model="subnetForm.subnet"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 focus:outline-none focus:border-blue-500"
              placeholder="192.168.1.0/24 or 10.0.0.1-10.0.0.30"
            />
            <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-1">
              Supports CIDR (e.g. <code class="font-mono">192.168.1.0/24</code>), Range (<code class="font-mono">10.0.0.1-10.0.0.50</code>), or comma-separated IPs. Capped at 512 IPs per scan.
            </p>
          </div>

          <!-- Port -->
          <div class="md:col-span-2">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              UDP Port
            </label>
            <input
              v-model.number="subnetForm.port"
              type="number"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- SNMP Version -->
          <div class="md:col-span-2">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              SNMP Version
            </label>
            <select
              v-model="subnetForm.version"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
            >
              <option value="2c">SNMP v2c</option>
              <option value="v1">SNMP v1</option>
            </select>
          </div>

          <!-- Community -->
          <div class="md:col-span-2">
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
              Community String
            </label>
            <input
              v-model="subnetForm.community"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 focus:outline-none focus:border-blue-500"
              placeholder="public"
            />
          </div>

          <!-- Scan Button -->
          <div class="md:col-span-2 flex items-start pt-6">
            <button
              @click="runSubnetScan"
              :disabled="subnetLoading"
              class="w-full py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold text-xs rounded-lg transition flex items-center justify-center gap-1.5 shadow-sm cursor-pointer"
            >
              <Search class="w-3.5 h-3.5" />
              <span>{{ subnetLoading ? 'Scanning...' : 'Scan Network' }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Subnet Error -->
      <div
        v-if="subnetError"
        class="p-4 rounded-xl border border-rose-300 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/20 text-rose-800 dark:text-rose-300 text-xs"
      >
        <div class="font-bold flex items-center gap-2 mb-1">
          <AlertTriangle class="w-4 h-4 text-rose-500" />
          <span>Subnet Scan Error</span>
        </div>
        <p class="font-mono text-[11px]">{{ subnetError }}</p>
      </div>

      <!-- Subnet Results -->
      <div v-if="subnetResult" class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-3">
            <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
              Discovered Devices
            </h2>
            <span class="px-2 py-0.5 rounded-full bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-[10px] font-bold border border-emerald-200 dark:border-emerald-500/30">
              {{ subnetResult.activeHosts }} Active of {{ subnetResult.scannedIps }} IPs ({{ subnetResult.durationSec }}s)
            </span>
          </div>

          <div class="relative w-full sm:w-64">
            <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
            <input
              v-model="subnetFilterSearch"
              placeholder="Filter active hosts..."
              class="w-full pl-8 pr-3 py-1.5 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="bg-slate-50 dark:bg-[#121826] border-y border-slate-200 dark:border-[#1b2234] text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
              <tr>
                <th class="py-2.5 px-3">Status</th>
                <th class="py-2.5 px-3">IP Address</th>
                <th class="py-2.5 px-3">Device Hostname</th>
                <th class="py-2.5 px-3">Vendor</th>
                <th class="py-2.5 px-3">System Description</th>
                <th class="py-2.5 px-3">Uptime</th>
                <th class="py-2.5 px-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]/60">
              <tr
                v-for="(host, idx) in filteredSubnetHosts"
                :key="idx"
                class="hover:bg-slate-50/60 dark:hover:bg-[#151c2d] transition"
              >
                <!-- Status -->
                <td class="py-2.5 px-3 whitespace-nowrap">
                  <span
                    v-if="host.reachable"
                    class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-500/30"
                  >
                    ONLINE ({{ host.latencyMs }}ms)
                  </span>
                  <span
                    v-else
                    class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-100 dark:bg-slate-800 text-slate-500"
                  >
                    NO SNMP
                  </span>
                </td>

                <!-- IP Address -->
                <td class="py-2.5 px-3 font-mono font-bold text-slate-900 dark:text-white">
                  {{ host.ipAddress }}
                </td>

                <!-- Hostname -->
                <td class="py-2.5 px-3 font-medium text-slate-800 dark:text-slate-200">
                  {{ host.hostname || '-' }}
                </td>

                <!-- Vendor -->
                <td class="py-2.5 px-3 whitespace-nowrap">
                  <span v-if="host.vendor && host.vendor !== 'Generic MIB-2'" class="font-semibold text-blue-600 dark:text-blue-400">
                    {{ host.vendor }}
                  </span>
                  <span v-else class="text-slate-400">
                    {{ host.vendor || '-' }}
                  </span>
                </td>

                <!-- SysDescr -->
                <td class="py-2.5 px-3 font-mono text-[10px] text-slate-600 dark:text-slate-400 max-w-xs truncate" :title="host.sysDescr">
                  {{ host.sysDescr || '-' }}
                </td>

                <!-- Uptime -->
                <td class="py-2.5 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
                  {{ host.sysUptime || '-' }}
                </td>

                <!-- Action: Inspect -->
                <td class="py-2.5 px-3 text-right whitespace-nowrap">
                  <button
                    v-if="host.reachable"
                    @click="inspectHostFromSubnet(host.ipAddress)"
                    class="px-2.5 py-1 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded text-[11px] font-semibold transition cursor-pointer"
                  >
                    Inspect Telemetry
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- ==================== TAB 3: RAW OID WALK & QUERY ==================== -->
    <div v-if="activeTab === 'walk'" class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <!-- Query Panel (Left) -->
      <div class="lg:col-span-4 p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-4 shadow-sm flex flex-col">
        <div class="border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-1.5">
            <Radio class="w-3.5 h-3.5 text-slate-500" />
            <span>Raw Query Parameters</span>
          </h2>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1 uppercase tracking-wider">Target Host / IP</label>
            <input
              v-model="queryForm.host"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 focus:outline-none focus:border-blue-500"
              placeholder="192.168.1.1"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1 uppercase tracking-wider">Port</label>
              <input
                v-model.number="queryForm.port"
                type="number"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1 uppercase tracking-wider">Version</label>
              <select
                v-model="queryForm.version"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
              >
                <option value="2c">v2c</option>
                <option value="v1">v1</option>
              </select>
            </div>
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1 uppercase tracking-wider">Community String</label>
            <input
              v-model="queryForm.community"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 focus:outline-none focus:border-blue-500"
              placeholder="public"
            />
          </div>

          <div>
            <div class="flex items-center justify-between mb-1">
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider">OID / Subtree</label>
              <select
                @change="(e: any) => { const p = presets.find(x => x.oid === e.target.value); if (p) applyPreset(p); e.target.value = ''; }"
                class="text-[10px] bg-slate-100 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 rounded px-1.5 py-0.5 font-medium cursor-pointer"
              >
                <option value="" disabled selected>Presets ▾</option>
                <option v-for="p in presets" :key="p.oid" :value="p.oid">{{ p.label }}</option>
              </select>
            </div>
            <input
              v-model="queryForm.oid"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 focus:outline-none focus:border-blue-500"
              placeholder="* (Scan All) or 1.3.6.1.2.1.1"
            />
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1 uppercase tracking-wider">Operation</label>
            <select
              v-model="queryForm.operation"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
            >
              <option value="walk">SNMP Walk (Subtree / Bulk / All OIDs)</option>
              <option value="get">SNMP Get (Single OID)</option>
            </select>
          </div>

          <!-- Advanced Toggle -->
          <div class="pt-1 border-t border-slate-200 dark:border-[#1b2234]">
            <button
              type="button"
              @click="showQueryAdvanced = !showQueryAdvanced"
              class="text-[11px] font-semibold text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white flex items-center gap-1 cursor-pointer"
            >
              <span>{{ showQueryAdvanced ? '▾ Hide Advanced Settings' : '▸ Advanced Settings (Timeout & Retries)' }}</span>
            </button>

            <div v-if="showQueryAdvanced" class="grid grid-cols-2 gap-3 mt-2">
              <div>
                <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 mb-1">Timeout (Sec)</label>
                <input
                  v-model.number="queryForm.timeout"
                  type="number"
                  min="2"
                  max="30"
                  class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1 text-xs font-mono"
                />
              </div>
              <div>
                <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 mb-1">Retries</label>
                <input
                  v-model.number="queryForm.retries"
                  type="number"
                  min="1"
                  max="5"
                  class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1 text-xs font-mono"
                />
              </div>
            </div>
          </div>
        </div>

        <button
          @click="executeQuery"
          :disabled="queryLoading"
          class="mt-2 w-full py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold text-xs rounded-lg transition flex items-center justify-center gap-1.5 shadow-sm cursor-pointer"
        >
          <Search class="w-3.5 h-3.5" />
          <span>{{ queryLoading ? 'Querying SNMP...' : 'Execute Query' }}</span>
        </button>
      </div>

      <!-- Results Table (Right) -->
      <div class="lg:col-span-8 p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-4 shadow-sm flex flex-col min-w-0">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
              Walk Results
            </h2>
            <span class="px-2 py-0.5 rounded-full bg-slate-100 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 text-[10px] font-mono border border-slate-200 dark:border-slate-800 font-bold">
              {{ querySearch ? `${filteredQueryResults.length} of ${queryResults.length} items` : `${queryResults.length} items` }}
            </span>
          </div>
          <button
            v-if="queryResults.length > 0"
            @click="queryResults = []; queryError = null; querySearch = ''"
            class="text-[11px] text-slate-500 hover:text-slate-800 dark:hover:text-white font-medium cursor-pointer"
          >
            Clear Results
          </button>
        </div>

        <!-- Filter Search Bar -->
        <div v-if="queryResults.length > 0" class="relative">
          <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
          <input
            v-model="querySearch"
            placeholder="Filter results by OID, MIB name, or value..."
            class="w-full pl-8 pr-8 py-1.5 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
          />
          <button
            v-if="querySearch"
            @click="querySearch = ''"
            class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        </div>

        <!-- Diagnostic Error Banner -->
        <div
          v-if="queryError"
          class="p-4 rounded-xl border border-rose-300 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/20 text-rose-800 dark:text-rose-300 text-xs space-y-2"
        >
          <div class="font-bold flex items-center gap-2 text-rose-700 dark:text-rose-400 text-sm">
            <AlertTriangle class="w-4 h-4 text-rose-500 shrink-0" />
            <span>SNMP Query Unsuccessful</span>
          </div>
          <p class="font-mono text-[11px] p-2 bg-white/60 dark:bg-black/30 rounded border border-rose-200 dark:border-rose-900/40">
            {{ queryError }}
          </p>
        </div>

        <!-- Result List Cards -->
        <div class="flex-1 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-3 overflow-y-auto font-mono text-[11px] min-h-[420px]">
          <div
            v-for="(res, idx) in filteredQueryResults"
            :key="idx"
            class="p-3 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-lg shadow-sm space-y-1.5 mb-2 hover:border-slate-400 dark:hover:border-slate-600 transition"
          >
            <div class="flex items-center justify-between">
              <span class="text-slate-900 dark:text-white font-bold text-xs">{{ res.name || res.oid }}</span>
              <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{{ res.oid }} ({{ res.type }})</span>
            </div>
            <div class="text-slate-900 dark:text-slate-200 break-all bg-slate-50 dark:bg-[#121826] px-2.5 py-1.5 rounded border border-slate-200 dark:border-[#1b2234]/60 font-semibold">
              {{ res.value }}
            </div>
          </div>

          <div v-if="queryResults.length === 0 && !queryLoading" class="h-64 flex flex-col items-center justify-center text-slate-500 dark:text-slate-400 text-xs gap-2">
            <Radio class="w-8 h-8 text-slate-300 dark:text-slate-600" />
            <span>Run an SNMP walk or get query to view raw records here</span>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== TAB 4: MIB MODULE REGISTRY ==================== -->
    <div v-if="activeTab === 'mibs'" class="space-y-6">
      <!-- OID Translator Tool Card -->
      <div class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-3">
        <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-1.5">
          <Search class="w-3.5 h-3.5 text-slate-500" />
          <span>OID Name Translator</span>
        </h2>
        <div class="flex flex-col sm:flex-row items-center gap-2">
          <input
            v-model="translateOidInput"
            placeholder="Enter numeric OID (e.g. 1.3.6.1.2.1.1.1.0)"
            class="flex-1 w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
          />
          <button
            @click="runTranslate"
            :disabled="translateLoading"
            class="w-full sm:w-auto px-4 py-2 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition cursor-pointer"
          >
            Translate OID
          </button>
        </div>

        <div v-if="translateResult" class="p-3 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs font-mono space-y-1">
          <div><span class="text-slate-500">Symbolic Name:</span> <strong class="text-slate-900 dark:text-white">{{ translateResult.name || 'Not mapped in local MIBs' }}</strong></div>
          <div v-if="translateResult.info"><span class="text-slate-500">MIB Info:</span> {{ translateResult.info }}</div>
        </div>
      </div>

      <!-- MIB Modules Table -->
      <div class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
            Imported MIB Modules
          </h2>
          <span class="text-xs text-slate-500 dark:text-slate-400">
            {{ mibs.length }} definitions loaded
          </span>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="bg-slate-50 dark:bg-[#121826] border-y border-slate-200 dark:border-[#1b2234] text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
              <tr>
                <th class="py-2.5 px-3">MIB Module Name</th>
                <th class="py-2.5 px-3">Indexed Nodes</th>
                <th class="py-2.5 px-3">Imported Date</th>
                <th class="py-2.5 px-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]/60">
              <tr
                v-for="m in mibs"
                :key="m.name"
                class="hover:bg-slate-50/60 dark:hover:bg-[#151c2d] transition"
              >
                <td class="py-2.5 px-3 font-bold font-mono text-slate-900 dark:text-white">
                  {{ m.name }}
                </td>
                <td class="py-2.5 px-3 font-mono text-slate-600 dark:text-slate-300">
                  {{ m.nodeCount }} nodes
                </td>
                <td class="py-2.5 px-3 text-slate-500 dark:text-slate-400">
                  {{ m.importedAt ? new Date(m.importedAt).toLocaleDateString() : '-' }}
                </td>
                <td class="py-2.5 px-3 text-right">
                  <button
                    @click="confirmDeleteMib(m.name)"
                    class="p-1 hover:bg-rose-50 dark:hover:bg-rose-950/30 text-rose-500 rounded transition cursor-pointer"
                    title="Delete MIB"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>

          <div v-if="mibs.length === 0" class="py-10 text-center text-slate-500 dark:text-slate-400 text-xs">
            No custom MIB modules imported yet. Click "Import MIB" to upload enterprise definitions (e.g. Cisco, Huawei, MikroTik).
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== MODAL: UPLOAD MIB ==================== -->
    <div
      v-if="uploadModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Import MIB Definition</h3>
          <button @click="uploadModalOpen = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase mb-1">Upload File (.mib / .txt)</label>
            <input
              type="file"
              accept=".mib,.txt,.my"
              @change="handleMibFileUpload"
              class="w-full text-xs text-slate-500 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-semibold file:bg-slate-100 file:text-slate-700 dark:file:bg-[#1a2233] dark:file:text-slate-300 cursor-pointer"
            />
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase mb-1">MIB Module Name</label>
            <input
              v-model="uploadName"
              placeholder="e.g. HUAWEI-ENTITY-EXTENT-MIB"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 uppercase mb-1">MIB Content</label>
            <textarea
              v-model="uploadContent"
              rows="8"
              placeholder="Paste ASN.1 MIB syntax here..."
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-2.5 text-xs font-mono text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            ></textarea>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-[#1f283d]">
          <button
            @click="uploadModalOpen = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="submitMibUpload"
            :disabled="uploadingMib"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ uploadingMib ? 'Compiling...' : 'Save & Import' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ==================== MODAL: DELETE CONFIRMATION (STRICT HCP SPEC) ==================== -->
    <div
      v-if="deleteModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Red circle icon center top -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <!-- Title & explanation -->
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete MIB Definition?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ mibToDelete }}</strong>? This action cannot be undone.
          </p>
        </div>

        <!-- Action buttons -->
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="deleteModalOpen = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDeleteMib"
            :disabled="deletingMib"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deletingMib ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
