<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { 
  Network, 
  Plus, 
  RefreshCw, 
  Scan, 
  Server, 
  Layers, 
  Radio, 
  ShieldCheck, 
  CheckCircle2, 
  XCircle 
} from 'lucide-vue-next';

interface Sheet {
  id: number;
  name: string;
}

interface Device {
  id: string;
  name: string;
  ipAddress: string;
  deviceType: string;
  status: string;
  sources: string[];
}

const sheets = ref<Sheet[]>([]);
const activeSheetId = ref<number | null>(null);
const devices = ref<Device[]>([]);
const edges = ref<any[]>([]);
const loading = ref(false);
const isAddDeviceModalOpen = ref(false);
const isScanSubnetModalOpen = ref(false);
const subnetCidr = ref('192.168.1.0/24');

const newDevice = ref({
  name: '',
  ipAddress: '',
  deviceType: 'server',
});

const fetchSheets = async () => {
  try {
    const res = await axios.get('/api/v1/topology/sheets');
    if (res.data.success && res.data.data.length > 0) {
      sheets.value = res.data.data;
      if (!activeSheetId.value) {
        activeSheetId.value = sheets.value[0].id;
      }
      fetchGraph();
    }
  } catch (err) {
    console.error('Failed to load sheets:', err);
  }
};

const fetchGraph = async () => {
  loading.value = true;
  try {
    const url = activeSheetId.value ? `/api/v1/topology?sheetId=${activeSheetId.value}` : '/api/v1/topology';
    const res = await axios.get(url);
    if (res.data.success) {
      devices.value = res.data.data.nodes || [];
      edges.value = res.data.data.edges || [];
    }
  } catch (err) {
    console.error('Failed to load graph:', err);
  } finally {
    loading.value = false;
  }
};

const discoverPrometheus = async () => {
  try {
    loading.value = true;
    const res = await axios.get('/api/v1/topology/discover/prometheus');
    if (res.data.success) {
      alert(`Discovered ${res.data.data.length} devices from Prometheus targets.`);
      fetchGraph();
    }
  } catch (err: any) {
    alert(`Discovery failed: ${err.response?.data?.error || err.message}`);
  } finally {
    loading.value = false;
  }
};

const scanSubnet = async () => {
  try {
    loading.value = true;
    const res = await axios.get(`/api/v1/topology/discover/subnet?cidr=${encodeURIComponent(subnetCidr.value)}`);
    if (res.data.success) {
      isScanSubnetModalOpen.value = false;
      alert(`Subnet scan found ${res.data.data.length} reachable devices.`);
      fetchGraph();
    }
  } catch (err: any) {
    alert(`Subnet scan failed: ${err.response?.data?.error || err.message}`);
  } finally {
    loading.value = false;
  }
};

const saveDevice = async () => {
  try {
    const res = await axios.post('/api/v1/topology/devices', {
      ...newDevice.value,
      sheetId: activeSheetId.value,
      status: 'unknown',
      sources: ['MANUAL'],
    });
    if (res.data.success) {
      isAddDeviceModalOpen.value = false;
      fetchGraph();
    }
  } catch (err) {
    console.error('Failed to save device:', err);
  }
};

onMounted(() => {
  fetchSheets();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <!-- Header with Tabs and Actions -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">Network Topology</h2>
        <p class="text-xs text-slate-400">Visual topology graph and automated device discovery</p>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="discoverPrometheus"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
        >
          <RefreshCw class="w-3.5 h-3.5 text-brand-400" />
          Sync Prometheus
        </button>
        <button
          @click="isScanSubnetModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
        >
          <Scan class="w-3.5 h-3.5 text-blue-400" />
          Scan Subnet
        </button>
        <button
          @click="isAddDeviceModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-brand-500 hover:bg-brand-600 text-white transition"
        >
          <Plus class="w-3.5 h-3.5" />
          Add Device
        </button>
      </div>
    </div>

    <!-- Sheet Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2">
      <button
        v-for="sheet in sheets"
        :key="sheet.id"
        @click="activeSheetId = sheet.id; fetchGraph()"
        :class="[
          activeSheetId === sheet.id ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400 hover:bg-slate-900 border-transparent',
          'px-3 py-1 text-xs font-medium rounded-lg border transition'
        ]"
      >
        {{ sheet.name }}
      </button>
    </div>

    <!-- Interactive Canvas & Devices Grid -->
    <div class="flex-1 bg-slate-950 border border-slate-800 rounded-xl p-4 overflow-y-auto">
      <div class="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-4 gap-3">
        <div
          v-for="dev in devices"
          :key="dev.id"
          class="p-4 rounded-xl bg-slate-900/80 border border-slate-800 hover:border-slate-700 transition space-y-2 relative overflow-hidden"
        >
          <!-- Status Indicator Dot -->
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span :class="[
                dev.status === 'online' ? 'bg-emerald-400 animate-pulse' : 'bg-red-400',
                'w-2 h-2 rounded-full'
              ]"></span>
              <span class="text-xs font-bold text-white truncate max-w-[140px]">{{ dev.name }}</span>
            </div>
            <span class="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
              {{ dev.deviceType }}
            </span>
          </div>

          <div class="text-[11px] font-mono text-slate-400">
            {{ dev.ipAddress }}
          </div>

          <div class="flex items-center justify-between text-[10px] text-slate-500 pt-1 border-t border-slate-800/60">
            <span>Sources: {{ dev.sources.join(', ') }}</span>
            <span :class="dev.status === 'online' ? 'text-emerald-400 font-medium' : 'text-red-400'">
              {{ dev.status }}
            </span>
          </div>
        </div>
      </div>

      <div v-if="devices.length === 0 && !loading" class="h-64 flex flex-col items-center justify-center text-slate-500 space-y-2">
        <Network class="w-8 h-8 text-slate-600" />
        <p class="text-xs">No devices added to this topology sheet.</p>
        <p class="text-[11px] text-slate-600">Use 'Scan Subnet' or 'Add Device' to start mapping.</p>
      </div>
    </div>

    <!-- Scan Subnet Modal -->
    <div v-if="isScanSubnetModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="w-full max-w-md bg-slate-900 border border-slate-700 rounded-xl p-6 shadow-2xl space-y-4">
        <h3 class="text-sm font-bold text-white">Scan Subnet (ICMP Sweep)</h3>
        <p class="text-xs text-slate-400">Concurrent ICMP sweep to discover active network devices</p>
        <div>
          <label class="block text-xs text-slate-400 mb-1">Subnet (CIDR)</label>
          <input v-model="subnetCidr" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-xs text-white font-mono focus:outline-none focus:border-brand-500" placeholder="192.168.1.0/24" />
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button @click="isScanSubnetModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">Cancel</button>
          <button @click="scanSubnet" class="px-4 py-1.5 text-xs bg-brand-500 hover:bg-brand-600 text-white font-medium rounded">Start Scan</button>
        </div>
      </div>
    </div>

    <!-- Add Device Modal -->
    <div v-if="isAddDeviceModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="w-full max-w-md bg-slate-900 border border-slate-700 rounded-xl p-6 shadow-2xl space-y-4">
        <h3 class="text-sm font-bold text-white">Add Device</h3>
        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Device Name</label>
            <input v-model="newDevice.name" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none" placeholder="e.g. Core Switch 01" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">IP Address</label>
            <input v-model="newDevice.ipAddress" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white font-mono focus:outline-none" placeholder="192.168.1.1" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Device Type</label>
            <select v-model="newDevice.deviceType" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none">
              <option value="server">Server</option>
              <option value="switch">Switch</option>
              <option value="router">Router</option>
              <option value="firewall">Firewall</option>
            </select>
          </div>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button @click="isAddDeviceModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">Cancel</button>
          <button @click="saveDevice" class="px-4 py-1.5 text-xs bg-brand-500 hover:bg-brand-600 text-white font-medium rounded">Save</button>
        </div>
      </div>
    </div>
  </div>
</template>
