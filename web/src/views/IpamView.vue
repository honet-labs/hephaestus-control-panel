<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import {
  Search,
  Plus,
  RotateCw,
  Trash2,
  Edit2,
  ChevronLeft,
  Grid,
  List,
  History,
  Activity,
  CheckCircle2,
  AlertCircle,
  Clock,
  Sparkles,
  Radio,
  Server,
  Laptop,
  Monitor,
  Smartphone,
  Terminal,
  Cpu,
  X
} from 'lucide-vue-next';

const authStore = useAuthStore();
const canManage = computed(() => authStore.can('ipam', 'manage'));

// ==================== DATA INTERFACES ====================
interface IpamSubnet {
  id: string;
  name: string;
  cidr: string;
  ipVersion?: 'ipv4' | 'ipv6';
  gateway: string;
  vlanId: number;
  vrf: string;
  description: string;
  scanInterval: string;
  lastScannedAt: string | null;
  nextScanAt: string | null;
  totalIps: number;
  totalUsedIps: number;
  totalUnusedIps: number;
  createdAt: string;
  updatedAt: string;
}

interface IpamAddress {
  id: string;
  subnetId: string;
  ipAddress: string;
  ipVersion?: string;
  status: 'active' | 'reserved' | 'discovered' | 'offline';
  hostname: string;
  macAddress: string;
  macVendor?: string;
  osFamily?: string;
  deviceType: string;
  isOnline: boolean;
  responseTimeMs: number;
  lastSeenAt: string | null;
  notes: string;
  createdAt: string;
  updatedAt: string;
}

interface IpamScanLog {
  id: string;
  subnetId: string;
  startedAt: string;
  finishedAt: string;
  durationMs: number;
  scannedIps: number;
  foundActive: number;
  status: 'success' | 'failed';
  errorMessage: string;
}

interface IpamSummaryStats {
  totalSubnets: number;
  totalMonitored: number;
  totalUsedIps: number;
  totalUnusedIps: number;
  totalDiscovered: number;
  totalOnline: number;
}

// ==================== STATE ====================
const subnets = ref<IpamSubnet[]>([]);
const summaryStats = ref<IpamSummaryStats>({
  totalSubnets: 0,
  totalMonitored: 0,
  totalUsedIps: 0,
  totalUnusedIps: 0,
  totalDiscovered: 0,
  totalOnline: 0,
});
const loading = ref(false);
const scanningSubnetId = ref<string | null>(null);
const scanningAll = ref(false);

// Selected Subnet & Mode
const selectedSubnet = ref<IpamSubnet | null>(null);
const subnetAddresses = ref<IpamAddress[]>([]);
const loadingAddresses = ref(false);
const ipViewMode = ref<'grid' | 'table'>('grid');

// Search & Filters
const searchSubnetQuery = ref('');
const searchIpQuery = ref('');
const filterStatus = ref<string>('all');

// Modals
const showSubnetModal = ref(false);
const isEditingSubnet = ref(false);
const subnetForm = ref({
  id: '',
  name: '',
  cidr: '',
  ipVersion: 'ipv4' as 'ipv4' | 'ipv6',
  gateway: '',
  vlanId: 0,
  vrf: 'Default',
  description: '',
  scanInterval: '6h',
});

const showAddressModal = ref(false);
const isEditingAddress = ref(false);
const addressForm = ref({
  id: '',
  subnetId: '',
  ipAddress: '',
  ipVersion: 'ipv4',
  status: 'active' as 'active' | 'reserved' | 'discovered',
  hostname: '',
  macAddress: '',
  macVendor: '',
  osFamily: 'Unknown',
  deviceType: 'Server',
  notes: '',
});

const showLogsModal = ref(false);
const scanLogs = ref<IpamScanLog[]>([]);
const loadingLogs = ref(false);

// Standard Delete Confirmation Modals
const showDeleteSubnetModal = ref(false);
const subnetToDelete = ref<IpamSubnet | null>(null);
const deletingSubnet = ref(false);

const showDeleteAddressModal = ref(false);
const addressToDelete = ref<IpamAddress | null>(null);
const deletingAddress = ref(false);

// Feedback Notification
const notification = ref<{ type: 'success' | 'error' | 'warning'; message: string } | null>(null);
let notificationTimer: ReturnType<typeof setTimeout> | null = null;
const showNotification = (type: 'success' | 'error' | 'warning', message: string) => {
  if (notificationTimer) clearTimeout(notificationTimer);
  notification.value = { type, message };
  notificationTimer = setTimeout(() => {
    notification.value = null;
    notificationTimer = null;
  }, 3000);
};

// ==================== API ACTIONS ====================

const fetchSubnets = async () => {
  loading.value = true;
  try {
    const [subRes, statsRes] = await Promise.all([
      axios.get('/api/v1/ipam/subnets'),
      axios.get('/api/v1/ipam/stats')
    ]);

    if (subRes.data?.success && Array.isArray(subRes.data.data)) {
      subnets.value = subRes.data.data;
    }
    if (statsRes.data?.success && statsRes.data.data) {
      summaryStats.value = statsRes.data.data;
    }
  } catch (err: any) {
    showNotification('error', `Failed to load subnets: ${err.response?.data?.error || err.message}`);
  } finally {
    loading.value = false;
  }
};

const selectSubnet = async (sub: IpamSubnet) => {
  selectedSubnet.value = sub;
  loadingAddresses.value = true;
  try {
    const res = await axios.get(`/api/v1/ipam/subnets/${sub.id}`);
    if (res.data?.success) {
      selectedSubnet.value = res.data.data;
      subnetAddresses.value = res.data.addresses || [];
    }
  } catch (err: any) {
    showNotification('error', `Failed to load subnet details: ${err.response?.data?.error || err.message}`);
  } finally {
    loadingAddresses.value = false;
  }
};

const refreshCurrentSubnet = async () => {
  if (!selectedSubnet.value) return;
  await selectSubnet(selectedSubnet.value);
};

// Subnet CRUD
const openAddSubnetModal = () => {
  isEditingSubnet.value = false;
  subnetForm.value = {
    id: '',
    name: '',
    cidr: '',
    ipVersion: 'ipv4',
    gateway: '',
    vlanId: 0,
    vrf: 'Default',
    description: '',
    scanInterval: '6h',
  };
  showSubnetModal.value = true;
};

const openEditSubnetModal = (sub: IpamSubnet) => {
  isEditingSubnet.value = true;
  subnetForm.value = {
    id: sub.id,
    name: sub.name,
    cidr: sub.cidr,
    ipVersion: sub.ipVersion || (sub.cidr.includes(':') ? 'ipv6' : 'ipv4'),
    gateway: sub.gateway || '',
    vlanId: sub.vlanId || 0,
    vrf: sub.vrf || 'Default',
    description: sub.description || '',
    scanInterval: sub.scanInterval || '6h',
  };
  showSubnetModal.value = true;
};

const saveSubnet = async () => {
  if (!subnetForm.value.name.trim() || !subnetForm.value.cidr.trim()) {
    showNotification('warning', 'Please provide a valid subnet name and CIDR notation.');
    return;
  }

  try {
    if (isEditingSubnet.value) {
      await axios.put(`/api/v1/ipam/subnets/${subnetForm.value.id}`, {
        name: subnetForm.value.name,
        gateway: subnetForm.value.gateway,
        vlanId: Number(subnetForm.value.vlanId),
        vrf: subnetForm.value.vrf,
        description: subnetForm.value.description,
        scanInterval: subnetForm.value.scanInterval,
      });
      showNotification('success', 'Subnet updated successfully.');
    } else {
      await axios.post('/api/v1/ipam/subnets', {
        name: subnetForm.value.name,
        cidr: subnetForm.value.cidr,
        ipVersion: subnetForm.value.ipVersion,
        gateway: subnetForm.value.gateway,
        vlanId: Number(subnetForm.value.vlanId),
        vrf: subnetForm.value.vrf,
        description: subnetForm.value.description,
        scanInterval: subnetForm.value.scanInterval,
      });
      showNotification('success', 'Subnet created successfully.');
    }
    showSubnetModal.value = false;
    await fetchSubnets();
    if (selectedSubnet.value && isEditingSubnet.value) {
      await refreshCurrentSubnet();
    }
  } catch (err: any) {
    showNotification('error', `Failed to save subnet: ${err.response?.data?.error || err.message}`);
  }
};

const promptDeleteSubnet = (sub: IpamSubnet) => {
  subnetToDelete.value = sub;
  showDeleteSubnetModal.value = true;
};

const confirmDeleteSubnet = async () => {
  if (!subnetToDelete.value) return;
  deletingSubnet.value = true;
  try {
    await axios.delete(`/api/v1/ipam/subnets/${subnetToDelete.value.id}`);
    showNotification('success', `Subnet '${subnetToDelete.value.name}' deleted.`);
    showDeleteSubnetModal.value = false;
    if (selectedSubnet.value?.id === subnetToDelete.value.id) {
      selectedSubnet.value = null;
    }
    await fetchSubnets();
  } catch (err: any) {
    showNotification('error', `Failed to delete subnet: ${err.response?.data?.error || err.message}`);
  } finally {
    deletingSubnet.value = false;
  }
};

// Scan Subnet On-Demand
const runSubnetScan = async (subnetId: string) => {
  scanningSubnetId.value = subnetId;
  try {
    const res = await axios.post(`/api/v1/ipam/subnets/${subnetId}/scan`);
    if (res.data?.success) {
      const log: IpamScanLog = res.data.data;
      showNotification('success', `Scan complete in ${log.durationMs}ms: ${log.foundActive} active host(s) discovered.`);
      await fetchSubnets();
      if (selectedSubnet.value?.id === subnetId) {
        await refreshCurrentSubnet();
      }
    }
  } catch (err: any) {
    showNotification('error', `Scan failed: ${err.response?.data?.error || err.message}`);
  } finally {
    scanningSubnetId.value = null;
  }
};

// Scan All Subnets Action
const triggerScanAllSubnets = async () => {
  scanningAll.value = true;
  try {
    const res = await axios.post('/api/v1/ipam/subnets/scan-all');
    if (res.data?.success) {
      const d = res.data.data;
      showNotification('success', `Scan complete across ${d.scannedCount} subnet(s): ${d.totalActive} active host(s) found.`);
      await fetchSubnets();
      if (selectedSubnet.value) {
        await refreshCurrentSubnet();
      }
    }
  } catch (err: any) {
    showNotification('error', `Scan all subnets failed: ${err.response?.data?.error || err.message}`);
  } finally {
    scanningAll.value = false;
  }
};

// Next Available IP Action
const findNextAvailableIP = async () => {
  if (!selectedSubnet.value) return;
  try {
    const res = await axios.get(`/api/v1/ipam/subnets/${selectedSubnet.value.id}/next-available`);
    if (res.data?.success && res.data.available) {
      openAddAddressModal(res.data.available);
      showNotification('success', `Found next free IP: ${res.data.available}`);
    }
  } catch (err: any) {
    showNotification('warning', `No free IP found: ${err.response?.data?.error || err.message}`);
  }
};

// Address Management
const openAddAddressModal = (ipPreset?: string) => {
  if (!selectedSubnet.value) return;
  isEditingAddress.value = false;
  addressForm.value = {
    id: '',
    subnetId: selectedSubnet.value.id,
    ipAddress: ipPreset || '',
    ipVersion: selectedSubnet.value.ipVersion || (selectedSubnet.value.cidr.includes(':') ? 'ipv6' : 'ipv4'),
    status: 'active',
    hostname: '',
    macAddress: '',
    macVendor: '',
    osFamily: 'Unknown',
    deviceType: 'Server',
    notes: '',
  };
  showAddressModal.value = true;
};

const openEditAddressModal = (addr: IpamAddress) => {
  isEditingAddress.value = true;
  addressForm.value = {
    id: addr.id,
    subnetId: addr.subnetId,
    ipAddress: addr.ipAddress,
    ipVersion: addr.ipVersion || (addr.ipAddress.includes(':') ? 'ipv6' : 'ipv4'),
    status: addr.status === 'discovered' ? 'active' : addr.status,
    hostname: addr.hostname || '',
    macAddress: addr.macAddress || '',
    macVendor: addr.macVendor || '',
    osFamily: addr.osFamily || 'Unknown',
    deviceType: addr.deviceType || 'Server',
    notes: addr.notes || '',
  };
  showAddressModal.value = true;
};

const saveAddress = async () => {
  if (!addressForm.value.ipAddress.trim()) {
    showNotification('warning', 'Please provide a valid IP address.');
    return;
  }

  try {
    if (isEditingAddress.value) {
      await axios.put(`/api/v1/ipam/addresses/${addressForm.value.id}`, {
        status: addressForm.value.status,
        hostname: addressForm.value.hostname,
        macAddress: addressForm.value.macAddress,
        macVendor: addressForm.value.macVendor,
        osFamily: addressForm.value.osFamily,
        deviceType: addressForm.value.deviceType,
        notes: addressForm.value.notes,
      });
      showNotification('success', `IP ${addressForm.value.ipAddress} updated.`);
    } else {
      await axios.post('/api/v1/ipam/addresses', {
        subnetId: addressForm.value.subnetId,
        ipAddress: addressForm.value.ipAddress,
        ipVersion: addressForm.value.ipVersion,
        status: addressForm.value.status,
        hostname: addressForm.value.hostname,
        macAddress: addressForm.value.macAddress,
        macVendor: addressForm.value.macVendor,
        osFamily: addressForm.value.osFamily,
        deviceType: addressForm.value.deviceType,
        notes: addressForm.value.notes,
      });
      showNotification('success', `IP ${addressForm.value.ipAddress} allocated.`);
    }
    showAddressModal.value = false;
    await refreshCurrentSubnet();
    await fetchSubnets();
  } catch (err: any) {
    showNotification('error', `Failed to save IP: ${err.response?.data?.error || err.message}`);
  }
};

const getOSIcon = (os?: string) => {
  if (!os) return Monitor;
  const lower = os.toLowerCase();
  if (lower.includes('win')) return Monitor;
  if (lower.includes('linux') || lower.includes('ubuntu') || lower.includes('debian') || lower.includes('centos')) return Terminal;
  if (lower.includes('android')) return Smartphone;
  if (lower.includes('apple') || lower.includes('ios') || lower.includes('mac')) return Laptop;
  if (lower.includes('router') || lower.includes('cisco') || lower.includes('network') || lower.includes('switch')) return Cpu;
  return Monitor;
};

const promptDeleteAddress = (addr: IpamAddress) => {
  addressToDelete.value = addr;
  showDeleteAddressModal.value = true;
};

const confirmDeleteAddress = async () => {
  if (!addressToDelete.value) return;
  deletingAddress.value = true;
  try {
    await axios.delete(`/api/v1/ipam/addresses/${addressToDelete.value.id}`);
    showNotification('success', `IP ${addressToDelete.value.ipAddress} released.`);
    showDeleteAddressModal.value = false;
    await refreshCurrentSubnet();
    await fetchSubnets();
  } catch (err: any) {
    showNotification('error', `Failed to release IP: ${err.response?.data?.error || err.message}`);
  } finally {
    deletingAddress.value = false;
  }
};

// Ping Test Single IP
const pingSingleIP = async (ip: string) => {
  try {
    showNotification('warning', `Pinging ${ip}...`);
    const res = await axios.post('/api/v1/ipam/addresses/ping', { ip });
    if (res.data?.success) {
      if (res.data.reachable) {
        showNotification('success', `Ping ${ip} OK: Latency ${res.data.latencyMs ?? 0}ms`);
      } else {
        showNotification('error', `Ping ${ip}: Host unreachable / no response`);
      }
      await refreshCurrentSubnet();
    }
  } catch (err: any) {
    showNotification('error', `Ping test failed: ${err.response?.data?.error || err.message}`);
  }
};

// Scan Logs History
const openScanLogsModal = async () => {
  if (!selectedSubnet.value) return;
  showLogsModal.value = true;
  loadingLogs.value = true;
  try {
    const res = await axios.get(`/api/v1/ipam/subnets/${selectedSubnet.value.id}/logs?limit=15`);
    if (res.data?.success && Array.isArray(res.data.data)) {
      scanLogs.value = res.data.data;
    }
  } catch (err: any) {
    showNotification('error', `Failed to load logs: ${err.response?.data?.error || err.message}`);
  } finally {
    loadingLogs.value = false;
  }
};

// ==================== COMPUTED HELPERS ====================

const filteredSubnets = computed(() => {
  const q = searchSubnetQuery.value.trim().toLowerCase();
  if (!q) return subnets.value;
  return subnets.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    s.cidr.toLowerCase().includes(q) ||
    s.gateway?.toLowerCase().includes(q) ||
    s.vrf?.toLowerCase().includes(q)
  );
});

// Map of assigned addresses indexed by IP for quick lookup in grid
const addressMap = computed(() => {
  const map = new Map<string, IpamAddress>();
  subnetAddresses.value.forEach(a => {
    map.set(a.ipAddress, a);
  });
  return map;
});

// Grid items generated from CIDR (e.g. for /24 -> 1 to 254)
interface GridIPItem {
  ip: string;
  hostNum: number;
  addr?: IpamAddress;
  status: 'active' | 'reserved' | 'discovered' | 'offline' | 'available';
}

const gridIPList = computed<GridIPItem[]>(() => {
  if (!selectedSubnet.value) return [];
  const cidr = selectedSubnet.value.cidr;
  const parts = cidr.split('/');
  if (parts.length !== 2) return [];

  const baseIP = parts[0];
  const prefix = parseInt(parts[1], 10);
  if (isNaN(prefix) || prefix < 20 || prefix > 30) {
    // For unusual or very large subnets, show recorded addresses only
    return subnetAddresses.value.map((a, idx) => ({
      ip: a.ipAddress,
      hostNum: idx + 1,
      addr: a,
      status: a.isOnline ? (a.status as any) : (a.status === 'discovered' ? 'offline' : (a.status as any))
    }));
  }

  // Calculate base IPv4 integer
  const octets = baseIP.split('.').map(Number);
  if (octets.length !== 4) return [];
  const baseInt = (octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3];
  const maskInt = prefix === 0 ? 0 : (~0 << (32 - prefix));
  const netInt = baseInt & maskInt;
  const bcastInt = netInt | (~maskInt >>> 0);

  const total = bcastInt - netInt + 1;
  const items: GridIPItem[] = [];

  // Generate hosts (net + 1 to bcast - 1)
  const start = total <= 2 ? netInt : netInt + 1;
  const end = total <= 2 ? bcastInt : bcastInt - 1;

  for (let u = start; u <= end; u++) {
    const o1 = (u >>> 24) & 255;
    const o2 = (u >>> 16) & 255;
    const o3 = (u >>> 8) & 255;
    const o4 = u & 255;
    const ipStr = `${o1}.${o2}.${o3}.${o4}`;
    const assigned = addressMap.value.get(ipStr);

    let status: GridIPItem['status'] = 'available';
    if (assigned) {
      if (assigned.status === 'reserved') {
        status = 'reserved';
      } else if (assigned.status === 'discovered') {
        status = assigned.isOnline ? 'discovered' : 'offline';
      } else {
        status = assigned.isOnline ? 'active' : 'offline';
      }
    }

    items.push({
      ip: ipStr,
      hostNum: o4,
      addr: assigned,
      status
    });
  }

  return items;
});

// Filtered addresses for Table view
const filteredTableAddresses = computed(() => {
  const q = searchIpQuery.value.trim().toLowerCase();
  const st = filterStatus.value;

  return subnetAddresses.value.filter(a => {
    const matchQuery = !q ||
      a.ipAddress.toLowerCase().includes(q) ||
      a.hostname?.toLowerCase().includes(q) ||
      a.macAddress?.toLowerCase().includes(q) ||
      a.deviceType?.toLowerCase().includes(q);

    const matchStatus = st === 'all' ||
      (st === 'online' && a.isOnline) ||
      (st === 'offline' && !a.isOnline) ||
      (st === 'active' && a.status === 'active') ||
      (st === 'reserved' && a.status === 'reserved') ||
      (st === 'discovered' && a.status === 'discovered');

    return matchQuery && matchStatus;
  });
});

const formatTimeAgo = (dateStr: string | null) => {
  if (!dateStr) return 'Never';
  const d = new Date(dateStr);
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) + ' ' +
         d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
};

const getScanIntervalLabel = (val: string) => {
  switch (val) {
    case '6h': return 'Every 6 Hours';
    case '12h': return 'Every 12 Hours';
    case '1d':
    case '24h': return 'Every 1 Day';
    case '3d':
    case '72h': return 'Every 3 Days';
    case 'manual': return 'Manual Only';
    default: return val || 'Manual';
  }
};

const summaryTotalUsed = computed(() => {
  return summaryStats.value.totalUsedIps ?? (summaryStats.value as any).totalUsedIPs ?? 0;
});

const summaryTotalUnused = computed(() => {
  return summaryStats.value.totalUnusedIps ?? (summaryStats.value as any).totalUnusedIPs ?? 0;
});

const calculatedTotalUsed = computed(() => {
  if (!selectedSubnet.value) return 0;
  if (subnetAddresses.value.length > 0) {
    return subnetAddresses.value.filter(a => a.status === 'active' || a.status === 'reserved' || a.status === 'discovered' || a.isOnline).length;
  }
  return selectedSubnet.value.totalUsedIps ?? (selectedSubnet.value as any).totalUsedIPs ?? 0;
});

const calculatedTotalUnused = computed(() => {
  if (!selectedSubnet.value) return 0;
  const total = selectedSubnet.value.totalIps || 0;
  return Math.max(0, total - calculatedTotalUsed.value);
});

const calculatedUtilization = computed(() => {
  if (!selectedSubnet.value || !selectedSubnet.value.totalIps) return 0;
  return Math.min(100, Math.round((calculatedTotalUsed.value / selectedSubnet.value.totalIps) * 100));
});

const getSubnetUsedCount = (sub: IpamSubnet) => {
  return sub.totalUsedIps ?? (sub as any).totalUsedIPs ?? 0;
};

const getSubnetUnusedCount = (sub: IpamSubnet) => {
  const used = getSubnetUsedCount(sub);
  return sub.totalUnusedIps ?? (sub as any).totalUnusedIPs ?? Math.max(0, (sub.totalIps || 0) - used);
};

const getUtilizationRate = (sub: IpamSubnet) => {
  const total = sub.totalIps || 0;
  if (total <= 0) return 0;
  const used = getSubnetUsedCount(sub);
  return Math.min(100, Math.round((used / total) * 100));
};

onMounted(() => {
  fetchSubnets();
});
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto font-sans">
    <!-- Header (Strictly adhering to AGENTS.md: pure text title, no icon, no emoji) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          IP Address Management (IPAM)
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Manage subnets, track IP address allocations, and run periodic automated network scans.
        </p>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <button
          v-if="selectedSubnet"
          @click="selectedSubnet = null"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 text-xs font-semibold transition cursor-pointer"
        >
          <ChevronLeft class="w-4 h-4 text-slate-400" />
          <span>All Subnets</span>
        </button>

        <button
          @click="selectedSubnet ? refreshCurrentSubnet() : fetchSubnets()"
          :disabled="loading || loadingAddresses"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 text-xs font-semibold transition cursor-pointer disabled:opacity-50"
        >
          <RotateCw class="w-3.5 h-3.5 text-slate-400" :class="{ 'animate-spin': loading || loadingAddresses }" />
          <span>Refresh</span>
        </button>

        <button
          v-if="!selectedSubnet && canManage"
          @click="triggerScanAllSubnets"
          :disabled="scanningAll || loading"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 text-xs font-semibold transition cursor-pointer disabled:opacity-50 border border-slate-200 dark:border-slate-800"
          title="Scan all registered subnets in parallel"
        >
          <Radio class="w-3.5 h-3.5 text-slate-400" :class="{ 'animate-pulse text-blue-500': scanningAll }" />
          <span>{{ scanningAll ? 'Scanning All...' : 'Scan All Subnets' }}</span>
        </button>

        <button
          v-if="!selectedSubnet && canManage"
          @click="openAddSubnetModal"
          class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold transition shadow-sm cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Subnet</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner (Auto-dismissing within 3s) -->
    <div
      v-if="notification"
      :class="[
        'p-3 rounded-lg text-xs font-medium flex items-center gap-2.5 transition-all shadow-sm border',
        notification.type === 'success' ? 'bg-emerald-50 text-emerald-800 border-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-400 dark:border-emerald-500/30' :
        notification.type === 'error' ? 'bg-rose-50 text-rose-800 border-rose-200 dark:bg-rose-500/10 dark:text-rose-400 dark:border-rose-500/30' :
        'bg-amber-50 text-amber-800 border-amber-200 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/30'
      ]"
    >
      <CheckCircle2 v-if="notification.type === 'success'" class="w-4 h-4 text-emerald-500 shrink-0" />
      <AlertCircle v-else class="w-4 h-4 text-rose-500 shrink-0" />
      <span>{{ notification.message }}</span>
    </div>

    <!-- ==================== VIEW 1: SUBNETS DASHBOARD ==================== -->
    <div v-if="!selectedSubnet" class="space-y-6">
      <!-- 4 Top Stat Cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <!-- Stat 1: Subnets -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-1">
          <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
            Total Subnets
          </div>
          <div class="text-2xl font-bold text-slate-900 dark:text-white">
            {{ summaryStats.totalSubnets }}
          </div>
          <div class="text-[11px] text-slate-400">
            Monitored network segments
          </div>
        </div>

        <!-- Stat 2: Monitored Hosts -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-1">
          <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
            Total Capacity
          </div>
          <div class="text-2xl font-bold text-slate-900 dark:text-white">
            {{ summaryStats.totalMonitored }}
          </div>
          <div class="text-[11px] text-slate-400">
            Total addressable host IPs
          </div>
        </div>

        <!-- Stat 3: Used IPs -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-1">
          <div class="text-[11px] font-bold text-emerald-600 dark:text-emerald-400 uppercase tracking-wider">
            Total IP Used
          </div>
          <div class="text-2xl font-bold text-emerald-600 dark:text-emerald-400 flex items-center gap-2">
            <span>{{ summaryTotalUsed }}</span>
            <span class="text-xs font-semibold px-2 py-0.5 rounded-full bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-500/30">
              {{ summaryStats.totalMonitored > 0 ? Math.round((summaryTotalUsed / summaryStats.totalMonitored) * 100) : 0 }}%
            </span>
          </div>
          <div class="text-[11px] text-slate-400">
            Allocated or online devices
          </div>
        </div>

        <!-- Stat 4: Unused IPs -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-1">
          <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
            Total IP Unused
          </div>
          <div class="text-2xl font-bold text-slate-900 dark:text-white">
            {{ summaryTotalUnused }}
          </div>
          <div class="text-[11px] text-slate-400">
            Available free addresses
          </div>
        </div>
      </div>

      <!-- Subnet Filter & Search Bar -->
      <div class="flex flex-col sm:flex-row items-center justify-between gap-3 p-3 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm">
        <div class="relative w-full sm:w-80">
          <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
          <input
            v-model="searchSubnetQuery"
            placeholder="Search subnets by name, CIDR, or gateway..."
            class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg pl-9 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div class="text-xs text-slate-500 dark:text-slate-400">
          Showing <span class="font-bold text-slate-900 dark:text-white">{{ filteredSubnets.length }}</span> subnet(s)
        </div>
      </div>

      <!-- Subnets Grid List -->
      <div v-if="filteredSubnets.length > 0" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <div
          v-for="sub in filteredSubnets"
          :key="sub.id"
          class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl p-5 shadow-sm hover:border-blue-400 dark:hover:border-blue-500/40 transition flex flex-col justify-between space-y-4"
        >
          <!-- Subnet Card Header -->
          <div class="space-y-1.5">
            <div class="flex items-start justify-between gap-2">
              <div>
                <h3 class="text-sm font-bold text-slate-900 dark:text-white tracking-tight">
                  {{ sub.name }}
                </h3>
                <div class="flex items-center gap-2 mt-1">
                  <span class="font-mono text-xs font-semibold text-blue-600 dark:text-blue-400">
                    {{ sub.cidr }}
                  </span>
                  <span class="text-[9px] font-bold uppercase px-1.5 py-0.5 rounded bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-800">
                    {{ sub.ipVersion || (sub.cidr.includes(':') ? 'ipv6' : 'ipv4') }}
                  </span>
                  <span v-if="sub.vlanId > 0" class="text-[10px] font-bold px-1.5 py-0.5 rounded bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-800">
                    VLAN {{ sub.vlanId }}
                  </span>
                  <span class="text-[10px] font-bold px-1.5 py-0.5 rounded bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-800">
                    {{ sub.vrf || 'Default' }}
                  </span>
                </div>
              </div>

              <!-- Scan Interval Pill -->
              <span class="text-[10px] font-bold px-2 py-0.5 rounded-full bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-800 shrink-0 flex items-center gap-1">
                <Clock class="w-2.5 h-2.5 text-slate-400" />
                <span>{{ getScanIntervalLabel(sub.scanInterval) }}</span>
              </span>
            </div>

            <p v-if="sub.description" class="text-xs text-slate-500 dark:text-slate-400 line-clamp-2">
              {{ sub.description }}
            </p>
            <div v-if="sub.gateway" class="text-[11px] text-slate-400 flex items-center gap-1">
              <span>Gateway:</span>
              <span class="font-mono text-slate-600 dark:text-slate-300">{{ sub.gateway }}</span>
            </div>
          </div>

          <!-- Utilization Progress Bar -->
          <div class="space-y-1.5 pt-2 border-t border-slate-100 dark:border-[#1b2234]">
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-500 dark:text-slate-400 font-medium">Utilization</span>
              <span class="font-bold text-slate-900 dark:text-white">{{ getUtilizationRate(sub) }}%</span>
            </div>

            <!-- Progress Bar -->
            <div class="w-full h-2 rounded-full bg-slate-100 dark:bg-[#1b2234] overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-300"
                :class="[
                  getUtilizationRate(sub) > 85 ? 'bg-rose-500' :
                  getUtilizationRate(sub) > 70 ? 'bg-amber-500' :
                  'bg-blue-500'
                ]"
                :style="{ width: `${getUtilizationRate(sub)}%` }"
              ></div>
            </div>

            <!-- Counters: Used vs Unused -->
            <div class="flex items-center justify-between text-[11px] pt-1">
              <div class="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400 font-semibold">
                <span class="w-2 h-2 rounded-full bg-emerald-500"></span>
                <span>{{ getSubnetUsedCount(sub) }} Used</span>
              </div>
              <div class="flex items-center gap-1.5 text-slate-500 dark:text-slate-400">
                <span class="w-2 h-2 rounded-full bg-slate-400"></span>
                <span>{{ getSubnetUnusedCount(sub) }} Unused</span>
              </div>
              <div class="text-slate-400">
                Total: {{ sub.totalIps }}
              </div>
            </div>
          </div>

          <!-- Last Scan Meta -->
          <div class="text-[10px] text-slate-400 flex items-center justify-between pt-1">
            <span>Last scan: {{ formatTimeAgo(sub.lastScannedAt) }}</span>
            <span v-if="sub.scanInterval !== 'manual' && sub.nextScanAt" class="text-slate-500">
              Next: {{ formatTimeAgo(sub.nextScanAt) }}
            </span>
          </div>

          <!-- Card Actions -->
          <div class="flex items-center justify-between gap-2 pt-2 border-t border-slate-100 dark:border-[#1b2234]">
            <button
              @click="selectSubnet(sub)"
              class="px-3 py-1.5 bg-blue-50 dark:bg-blue-500/10 hover:bg-blue-100 dark:hover:bg-blue-500/20 text-blue-600 dark:text-blue-400 rounded-lg text-xs font-bold transition flex items-center gap-1.5 cursor-pointer"
            >
              <span>Manage IPs</span>
            </button>

            <div class="flex items-center gap-1">
              <button
                @click="runSubnetScan(sub.id)"
                :disabled="scanningSubnetId === sub.id"
                title="Scan Subnet Now"
                class="p-1.5 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-[#1a2233] rounded-lg transition cursor-pointer disabled:opacity-50"
              >
                <Radio class="w-4 h-4" :class="{ 'animate-pulse text-blue-500': scanningSubnetId === sub.id }" />
              </button>

              <button
                v-if="canManage"
                @click="openEditSubnetModal(sub)"
                title="Edit Subnet"
                class="p-1.5 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-[#1a2233] rounded-lg transition cursor-pointer"
              >
                <Edit2 class="w-3.5 h-3.5" />
              </button>

              <button
                v-if="canManage"
                @click="promptDeleteSubnet(sub)"
                title="Delete Subnet"
                class="p-1.5 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-500/10 rounded-lg transition cursor-pointer"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div
        v-else-if="!loading"
        class="text-center py-16 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl p-8 space-y-3"
      >
        <Server class="w-10 h-10 text-slate-400 mx-auto" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">
          No subnets registered yet
        </h3>
        <p class="text-xs text-slate-500 dark:text-slate-400 max-w-sm mx-auto">
          Add your first network subnet (e.g. 192.168.10.0/24) to start tracking IP allocations and running automated scans.
        </p>
        <button
          v-if="canManage"
          @click="openAddSubnetModal"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shadow-sm cursor-pointer"
        >
          Add First Subnet
        </button>
      </div>
    </div>

    <!-- ==================== VIEW 2: SUBNET DETAIL & IP MATRIX ==================== -->
    <div v-else class="space-y-6">
      <!-- Subnet Detail Header Banner -->
      <div class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
        <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
          <div class="space-y-1">
            <div class="flex items-center gap-3">
              <h2 class="text-lg font-bold text-slate-900 dark:text-white tracking-tight">
                {{ selectedSubnet.name }}
              </h2>
              <span class="font-mono text-sm font-semibold text-blue-600 dark:text-blue-400 px-2 py-0.5 rounded bg-blue-50 dark:bg-blue-500/10">
                {{ selectedSubnet.cidr }}
              </span>
              <span v-if="selectedSubnet.vlanId > 0" class="text-xs font-bold px-2 py-0.5 rounded bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300">
                VLAN {{ selectedSubnet.vlanId }}
              </span>
            </div>
            <div class="flex flex-wrap items-center gap-4 text-xs text-slate-500 dark:text-slate-400">
              <span v-if="selectedSubnet.gateway">Gateway: <strong class="text-slate-700 dark:text-slate-300 font-mono">{{ selectedSubnet.gateway }}</strong></span>
              <span>VRF: <strong class="text-slate-700 dark:text-slate-300">{{ selectedSubnet.vrf || 'Default' }}</strong></span>
              <span>Scan Schedule: <strong class="text-slate-700 dark:text-slate-300">{{ getScanIntervalLabel(selectedSubnet.scanInterval) }}</strong></span>
              <span>Last scanned: <strong class="text-slate-700 dark:text-slate-300">{{ formatTimeAgo(selectedSubnet.lastScannedAt) }}</strong></span>
            </div>
          </div>

          <!-- Actions Toolbar -->
          <div class="flex flex-wrap items-center gap-2 shrink-0">
            <button
              @click="runSubnetScan(selectedSubnet.id)"
              :disabled="scanningSubnetId === selectedSubnet.id"
              class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition flex items-center gap-1.5 shadow-sm cursor-pointer disabled:opacity-50"
            >
              <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': scanningSubnetId === selectedSubnet.id }" />
              <span>{{ scanningSubnetId === selectedSubnet.id ? 'Scanning...' : 'Scan Subnet Now' }}</span>
            </button>

            <button
              v-if="canManage"
              @click="findNextAvailableIP"
              class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer border border-slate-200 dark:border-slate-800"
            >
              <Sparkles class="w-3.5 h-3.5 text-amber-500" />
              <span>Next Available IP</span>
            </button>

            <button
              v-if="canManage"
              @click="openAddAddressModal()"
              class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer border border-slate-200 dark:border-slate-800"
            >
              <Plus class="w-3.5 h-3.5 text-slate-400" />
              <span>Assign IP</span>
            </button>

            <button
              @click="openScanLogsModal"
              class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer border border-slate-200 dark:border-slate-800"
            >
              <History class="w-3.5 h-3.5 text-slate-400" />
              <span>Scan Logs</span>
            </button>
          </div>
        </div>

        <!-- Subnet Quick Metrics -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div class="p-3 bg-slate-50 dark:bg-[#121826] rounded-lg">
            <span class="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">Total Host IPs</span>
            <span class="text-xl font-bold text-slate-900 dark:text-white font-mono">{{ selectedSubnet.totalIps }}</span>
          </div>

          <div class="p-3 bg-slate-50 dark:bg-[#121826] rounded-lg">
            <span class="text-[10px] font-bold text-emerald-600 dark:text-emerald-400 uppercase tracking-wider block">Total IP Used</span>
            <span class="text-xl font-bold text-emerald-600 dark:text-emerald-400 font-mono">{{ calculatedTotalUsed }}</span>
          </div>

          <div class="p-3 bg-slate-50 dark:bg-[#121826] rounded-lg">
            <span class="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">Total IP Unused</span>
            <span class="text-xl font-bold text-slate-900 dark:text-white font-mono">{{ calculatedTotalUnused }}</span>
          </div>

          <div class="p-3 bg-slate-50 dark:bg-[#121826] rounded-lg">
            <span class="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">Utilization</span>
            <span class="text-xl font-bold text-slate-900 dark:text-white font-mono">{{ calculatedUtilization }}%</span>
          </div>
        </div>
      </div>

      <!-- Controls: Search, View Switcher & Legend -->
      <div class="flex flex-col sm:flex-row items-center justify-between gap-3 p-3 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm">
        <!-- Search & Filter -->
        <div class="flex flex-wrap items-center gap-2 w-full sm:w-auto">
          <div class="relative w-full sm:w-64">
            <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
            <input
              v-model="searchIpQuery"
              placeholder="Filter by IP, hostname, MAC..."
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg pl-9 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>

          <select
            v-model="filterStatus"
            class="bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 text-xs text-slate-700 dark:text-slate-300 font-medium focus:outline-none focus:border-blue-500"
          >
            <option value="all">All Status</option>
            <option value="online">Online Only</option>
            <option value="offline">Offline Only</option>
            <option value="active">Active (Assigned)</option>
            <option value="reserved">Reserved</option>
            <option value="discovered">Discovered</option>
          </select>
        </div>

        <!-- Right Side: View Mode Switcher -->
        <div class="flex items-center gap-2 shrink-0">
          <div class="flex items-center bg-slate-100 dark:bg-[#121826] p-0.5 rounded-lg border border-slate-200 dark:border-[#1b2234]">
            <button
              @click="ipViewMode = 'grid'"
              :class="[
                'px-2.5 py-1 text-xs font-semibold rounded-md transition cursor-pointer flex items-center gap-1.5',
                ipViewMode === 'grid'
                  ? 'bg-white dark:bg-[#1a2233] text-blue-600 dark:text-blue-400 shadow-sm'
                  : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              <Grid class="w-3.5 h-3.5" />
              <span>IP Matrix</span>
            </button>
            <button
              @click="ipViewMode = 'table'"
              :class="[
                'px-2.5 py-1 text-xs font-semibold rounded-md transition cursor-pointer flex items-center gap-1.5',
                ipViewMode === 'table'
                  ? 'bg-white dark:bg-[#1a2233] text-blue-600 dark:text-blue-400 shadow-sm'
                  : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              <List class="w-3.5 h-3.5" />
              <span>List Table</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Color Legend -->
      <div class="flex flex-wrap items-center gap-4 px-3 py-2 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl text-xs">
        <span class="font-bold text-slate-700 dark:text-slate-300">Legend:</span>
        <div class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-emerald-500"></span>
          <span class="text-slate-600 dark:text-slate-400">Active (Assigned)</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-blue-500"></span>
          <span class="text-slate-600 dark:text-slate-400">Discovered (Online)</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-amber-500"></span>
          <span class="text-slate-600 dark:text-slate-400">Reserved / Gateway</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-rose-500"></span>
          <span class="text-slate-600 dark:text-slate-400">Offline / No Reply</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-slate-200 dark:bg-[#1b2234]"></span>
          <span class="text-slate-600 dark:text-slate-400">Available (Free)</span>
        </div>
      </div>

      <!-- MODE 1: VISUAL IP MATRIX GRID -->
      <div v-if="ipViewMode === 'grid'" class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm">
        <div class="grid grid-cols-2 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 xl:grid-cols-10 gap-2.5">
          <div
            v-for="item in gridIPList"
            :key="item.ip"
            @click="item.addr ? openEditAddressModal(item.addr) : (canManage ? openAddAddressModal(item.ip) : null)"
            :class="[
              'group relative p-2.5 rounded-xl border text-center transition cursor-pointer select-none',
              item.status === 'active' ? 'bg-emerald-50 dark:bg-emerald-500/10 border-emerald-300 dark:border-emerald-500/30 text-emerald-800 dark:text-emerald-300 hover:scale-105 shadow-xs' :
              item.status === 'reserved' ? 'bg-amber-50 dark:bg-amber-500/10 border-amber-300 dark:border-amber-500/30 text-amber-800 dark:text-amber-300 hover:scale-105 shadow-xs' :
              item.status === 'discovered' ? 'bg-blue-50 dark:bg-blue-500/10 border-blue-300 dark:border-blue-500/30 text-blue-800 dark:text-blue-300 hover:scale-105 shadow-xs' :
              item.status === 'offline' ? 'bg-rose-50 dark:bg-rose-500/10 border-rose-300 dark:border-rose-500/30 text-rose-800 dark:text-rose-300 hover:scale-105 shadow-xs' :
              'bg-slate-50 dark:bg-[#121826] border-slate-200 dark:border-[#1b2234] text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#1a2233]'
            ]"
          >
            <!-- Full IP Address in grid -->
            <div class="font-mono text-[11px] font-bold leading-tight tracking-tight truncate px-0.5" :title="item.ip">
              {{ item.ip }}
            </div>

            <!-- Hostname or OS if available -->
            <div
              v-if="item.addr?.hostname || (item.addr?.osFamily && item.addr.osFamily !== 'Unknown')"
              class="text-[9px] truncate text-slate-500 dark:text-slate-400 mt-0.5 px-0.5"
            >
              {{ item.addr?.hostname || item.addr?.osFamily }}
            </div>

            <!-- Status Dot & Latency -->
            <div class="mt-1 flex items-center justify-center gap-1">
              <span
                class="w-1.5 h-1.5 rounded-full"
                :class="[
                  item.status === 'active' ? 'bg-emerald-500' :
                  item.status === 'reserved' ? 'bg-amber-500' :
                  item.status === 'discovered' ? 'bg-blue-500' :
                  item.status === 'offline' ? 'bg-rose-500' :
                  'bg-slate-300 dark:bg-slate-700'
                ]"
              ></span>
              <span v-if="item.addr?.isOnline" class="font-mono text-[9px] text-slate-400">
                {{ item.addr.responseTimeMs }}ms
              </span>
            </div>

            <!-- Floating Tooltip on Hover -->
            <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 w-56 p-2.5 bg-slate-900 text-white dark:bg-black dark:text-slate-100 text-[11px] rounded-lg shadow-xl opacity-0 pointer-events-none group-hover:opacity-100 transition duration-150 z-30 font-sans text-left space-y-1">
              <div class="font-mono font-bold text-white border-b border-slate-800 pb-1 flex items-center justify-between">
                <span>{{ item.ip }}</span>
                <span class="uppercase text-[9px] px-1 py-0.2 rounded font-bold"
                  :class="item.status === 'active' ? 'bg-emerald-500/20 text-emerald-400' : item.status === 'discovered' ? 'bg-blue-500/20 text-blue-400' : item.status === 'available' ? 'bg-slate-700 text-slate-300' : 'bg-amber-500/20 text-amber-400'"
                >
                  {{ item.status }}
                </span>
              </div>
              <div v-if="item.addr?.hostname" class="truncate">
                <span class="text-slate-400">Host:</span> {{ item.addr.hostname }}
              </div>
              <div v-if="item.addr" class="truncate">
                <span class="text-slate-400">OS:</span>
                <span class="text-slate-200 font-medium ml-1">
                  {{ (item.addr.osFamily && item.addr.osFamily !== 'Unknown') ? item.addr.osFamily : (item.addr.isOnline ? 'Linux / Unix (Probable)' : 'Unknown') }}
                </span>
              </div>
              <div v-if="item.addr?.deviceType" class="text-slate-400">
                <span>Type:</span> <span class="text-slate-200">{{ item.addr.deviceType }}</span>
              </div>
              <div v-if="item.addr?.macAddress" class="font-mono text-[10px] text-slate-300 truncate">
                MAC: {{ item.addr.macAddress }}
                <span v-if="item.addr?.macVendor" class="block font-sans text-slate-400">({{ item.addr.macVendor }})</span>
              </div>
              <div v-if="item.addr?.isOnline" class="text-emerald-400 flex items-center gap-1">
                <Activity class="w-3 h-3 text-emerald-400" />
                <span>Online ({{ item.addr.responseTimeMs }}ms)</span>
              </div>
              <div v-else-if="item.addr" class="text-rose-400">
                Offline
              </div>
              <div v-else class="text-slate-400 italic">
                Unallocated IP (Click to assign)
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- MODE 2: TABLE LIST VIEW -->
      <div v-else class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="bg-slate-50 dark:bg-[#121826] border-b border-slate-200 dark:border-[#1b2234] text-slate-500 dark:text-slate-400 font-bold uppercase tracking-wider text-[10px]">
              <tr>
                <th class="py-3 px-4">Status</th>
                <th class="py-3 px-4">IP Address</th>
                <th class="py-3 px-4">Hostname</th>
                <th class="py-3 px-4">OS / System</th>
                <th class="py-3 px-4">Device Type</th>
                <th class="py-3 px-4">MAC Address</th>
                <th class="py-3 px-4">Latency</th>
                <th class="py-3 px-4">Last Seen</th>
                <th class="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]">
              <tr
                v-for="addr in filteredTableAddresses"
                :key="addr.id"
                class="hover:bg-slate-50/50 dark:hover:bg-[#161d2d]/50 transition"
              >
                <!-- Status Badge -->
                <td class="py-3 px-4 shrink-0">
                  <div class="flex items-center gap-1.5">
                    <span
                      class="w-2 h-2 rounded-full"
                      :class="addr.isOnline ? 'bg-emerald-500 animate-pulse' : 'bg-rose-500'"
                    ></span>
                    <span
                      class="text-[10px] font-bold px-1.5 py-0.5 rounded capitalize"
                      :class="[
                        addr.status === 'active' ? 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-500/30' :
                        addr.status === 'reserved' ? 'bg-amber-50 dark:bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-200 dark:border-amber-500/30' :
                        'bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-500/30'
                      ]"
                    >
                      {{ addr.status }}
                    </span>
                  </div>
                </td>

                <!-- IP Address -->
                <td class="py-3 px-4 font-mono font-bold text-slate-900 dark:text-white">
                  <span>{{ addr.ipAddress }}</span>
                  <span v-if="addr.ipVersion === 'ipv6'" class="ml-1.5 text-[9px] font-bold uppercase px-1 py-0.2 rounded bg-slate-100 dark:bg-[#1a2233] text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-800">
                    IPv6
                  </span>
                </td>

                <!-- Hostname -->
                <td class="py-3 px-4 text-slate-700 dark:text-slate-300 font-medium">
                  {{ addr.hostname || '—' }}
                </td>

                <!-- OS / System -->
                <td class="py-3 px-4">
                  <span
                    v-if="addr.osFamily && addr.osFamily !== 'Unknown'"
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 dark:bg-[#161d2d] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700"
                  >
                    <component :is="getOSIcon(addr.osFamily)" class="w-3 h-3 text-slate-400 shrink-0" />
                    <span>{{ addr.osFamily }}</span>
                  </span>
                  <span
                    v-else-if="addr.isOnline"
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 dark:bg-[#161d2d] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700"
                  >
                    <component :is="getOSIcon('Linux')" class="w-3 h-3 text-slate-400 shrink-0" />
                    <span>Linux / Unix (Probable)</span>
                  </span>
                  <span v-else class="text-slate-400 text-xs">—</span>
                </td>

                <!-- Device Type -->
                <td class="py-3 px-4 text-slate-600 dark:text-slate-400">
                  {{ addr.deviceType || 'Server' }}
                </td>

                <!-- MAC Address -->
                <td class="py-3 px-4 font-mono text-[11px] text-slate-600 dark:text-slate-300">
                  <div v-if="addr.macAddress">
                    <span class="font-bold">{{ addr.macAddress }}</span>
                    <span v-if="addr.macVendor" class="block font-sans text-[10px] text-slate-400 mt-0.5">
                      {{ addr.macVendor }}
                    </span>
                  </div>
                  <span v-else class="text-slate-400">—</span>
                </td>

                <!-- Latency -->
                <td class="py-3 px-4 font-mono text-[11px]">
                  <span v-if="addr.isOnline" class="text-emerald-600 dark:text-emerald-400 font-semibold">
                    {{ addr.responseTimeMs }}ms
                  </span>
                  <span v-else class="text-slate-400">
                    —
                  </span>
                </td>

                <!-- Last Seen -->
                <td class="py-3 px-4 text-slate-400 text-[11px]">
                  {{ formatTimeAgo(addr.lastSeenAt) }}
                </td>

                <!-- Row Actions -->
                <td class="py-3 px-4 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <button
                      @click="pingSingleIP(addr.ipAddress)"
                      title="Test Ping"
                      class="p-1.5 text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-500/10 rounded-lg transition cursor-pointer"
                    >
                      <Radio class="w-3.5 h-3.5" />
                    </button>

                    <button
                      v-if="canManage"
                      @click="openEditAddressModal(addr)"
                      title="Edit Allocation"
                      class="p-1.5 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-[#1a2233] rounded-lg transition cursor-pointer"
                    >
                      <Edit2 class="w-3.5 h-3.5" />
                    </button>

                    <button
                      v-if="canManage"
                      @click="promptDeleteAddress(addr)"
                      title="Release IP"
                      class="p-1.5 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-500/10 rounded-lg transition cursor-pointer"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- ==================== MODALS ==================== -->

    <!-- Modal 1: Add / Edit Subnet -->
    <div
      v-if="showSubnetModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            {{ isEditingSubnet ? 'Edit Subnet Configuration' : 'Add New Network Subnet' }}
          </h3>
          <button @click="showSubnetModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3.5 text-xs">
          <!-- IP Protocol Version Selector -->
          <div>
            <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">IP Protocol Version</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                :disabled="isEditingSubnet"
                @click="subnetForm.ipVersion = 'ipv4'"
                :class="subnetForm.ipVersion === 'ipv4' ? 'bg-blue-600 text-white font-bold border-blue-600' : 'bg-slate-50 dark:bg-[#161d2d] text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-blue-400'"
                class="px-3 py-2 rounded-lg border text-xs transition cursor-pointer text-center disabled:opacity-50"
              >
                IPv4 Network
              </button>
              <button
                type="button"
                :disabled="isEditingSubnet"
                @click="subnetForm.ipVersion = 'ipv6'"
                :class="subnetForm.ipVersion === 'ipv6' ? 'bg-blue-600 text-white font-bold border-blue-600' : 'bg-slate-50 dark:bg-[#161d2d] text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-blue-400'"
                class="px-3 py-2 rounded-lg border text-xs transition cursor-pointer text-center disabled:opacity-50"
              >
                IPv6 Network
              </button>
            </div>
          </div>

          <div>
            <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Subnet Name *</label>
            <input
              v-model="subnetForm.name"
              placeholder="e.g. Production Web Tier, DMZ Servers"
              class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">CIDR Notation *</label>
            <input
              v-model="subnetForm.cidr"
              :disabled="isEditingSubnet"
              :placeholder="subnetForm.ipVersion === 'ipv6' ? 'e.g. 2001:db8:10::/64' : 'e.g. 192.168.10.0/24, 10.0.0.0/22'"
              class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 disabled:opacity-50"
            />
            <span class="text-[10px] text-slate-400 mt-0.5 block">CIDR determines the total host capacity.</span>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Default Gateway</label>
              <input
                v-model="subnetForm.gateway"
                :placeholder="subnetForm.ipVersion === 'ipv6' ? 'e.g. 2001:db8:10::1' : 'e.g. 192.168.10.1'"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">VLAN ID</label>
              <input
                v-model.number="subnetForm.vlanId"
                type="number"
                placeholder="e.g. 100"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">VRF / Scope</label>
              <input
                v-model="subnetForm.vrf"
                placeholder="Default"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>

            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Automated Scan Interval</label>
              <select
                v-model="subnetForm.scanInterval"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-medium"
              >
                <option value="6h">Every 6 Hours</option>
                <option value="12h">Every 12 Hours</option>
                <option value="1d">Every 1 Day</option>
                <option value="3d">Every 3 Days</option>
                <option value="manual">Manual Only</option>
              </select>
            </div>
          </div>

          <div>
            <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Description / Notes</label>
            <textarea
              v-model="subnetForm.description"
              rows="2"
              placeholder="Optional notes or operational purposes..."
              class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            ></textarea>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-200 dark:border-[#1f283d]">
          <button
            @click="showSubnetModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="saveSubnet"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shadow-sm cursor-pointer"
          >
            {{ isEditingSubnet ? 'Update Subnet' : 'Create Subnet' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Modal 2: Assign / Edit IP Address -->
    <div
      v-if="showAddressModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            {{ isEditingAddress ? 'Edit IP Allocation' : 'Assign IP Address' }}
          </h3>
          <button @click="showAddressModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3.5 text-xs">
          <div>
            <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">IP Address *</label>
            <input
              v-model="addressForm.ipAddress"
              :disabled="isEditingAddress"
              placeholder="e.g. 192.168.10.45"
              class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 disabled:opacity-50"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Status</label>
              <select
                v-model="addressForm.status"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-medium"
              >
                <option value="active">Active (Assigned)</option>
                <option value="reserved">Reserved (Gateway/VIP)</option>
              </select>
            </div>

            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Device Type</label>
              <select
                v-model="addressForm.deviceType"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-medium"
              >
                <option value="Server">Server</option>
                <option value="VM">Virtual Machine (VM)</option>
                <option value="Router">Router</option>
                <option value="Switch">Switch</option>
                <option value="Gateway">Gateway</option>
                <option value="Printer">Printer</option>
                <option value="Unknown">Other / Unknown</option>
              </select>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Operating System</label>
              <select
                v-model="addressForm.osFamily"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-medium"
              >
                <option value="Unknown">Auto-Detect / Unknown</option>
                <option value="Windows">Windows</option>
                <option value="Linux">Linux</option>
                <option value="Android">Android</option>
                <option value="Apple">Apple (iOS / macOS)</option>
                <option value="Network Device">Network / Router</option>
                <option value="IoT">IoT / Embedded</option>
              </select>
            </div>

            <div>
              <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Hostname / Label</label>
              <input
                v-model="addressForm.hostname"
                placeholder="e.g. web-app-prod-01"
                class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div>
            <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">MAC Address</label>
            <input
              v-model="addressForm.macAddress"
              placeholder="e.g. 52:54:00:12:34:56"
              class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label class="font-bold text-slate-700 dark:text-slate-300 block mb-1">Notes</label>
            <textarea
              v-model="addressForm.notes"
              rows="2"
              placeholder="Operational notes, owner, or peruntukan..."
              class="w-full bg-slate-50 dark:bg-[#161d2d] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            ></textarea>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-200 dark:border-[#1f283d]">
          <button
            @click="showAddressModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="saveAddress"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shadow-sm cursor-pointer"
          >
            {{ isEditingAddress ? 'Update Allocation' : 'Assign IP' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Modal 3: Scan Logs History -->
    <div
      v-if="showLogsModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-2xl shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
          <div class="flex items-center gap-2">
            <History class="w-4 h-4 text-slate-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">
              Scan Logs History — {{ selectedSubnet?.name }}
            </h3>
          </div>
          <button @click="showLogsModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="max-h-96 overflow-y-auto space-y-2 pr-1">
          <div
            v-for="log in scanLogs"
            :key="log.id"
            class="p-3 bg-slate-50 dark:bg-[#161d2d] rounded-lg border border-slate-200 dark:border-slate-800 text-xs flex items-center justify-between"
          >
            <div class="space-y-1">
              <div class="flex items-center gap-2">
                <span
                  class="w-2 h-2 rounded-full"
                  :class="log.status === 'success' ? 'bg-emerald-500' : 'bg-rose-500'"
                ></span>
                <span class="font-bold text-slate-900 dark:text-white">
                  {{ log.foundActive }} Active Host(s) Discovered
                </span>
                <span class="text-slate-400 font-mono text-[11px]">
                  ({{ log.scannedIPs }} scanned in {{ log.durationMs }}ms)
                </span>
              </div>
              <div class="text-[11px] text-slate-400">
                Started: {{ formatTimeAgo(log.startedAt) }}
              </div>
            </div>

            <span class="text-[10px] font-bold px-2 py-0.5 rounded uppercase"
              :class="log.status === 'success' ? 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-rose-50 dark:bg-rose-500/10 text-rose-600 dark:text-rose-400'"
            >
              {{ log.status }}
            </span>
          </div>

          <div v-if="scanLogs.length === 0 && !loadingLogs" class="text-center py-8 text-xs text-slate-400">
            No scan logs recorded yet. Run a scan to see audit history.
          </div>
        </div>

        <div class="flex items-center justify-end pt-3 border-t border-slate-200 dark:border-[#1f283d]">
          <button
            @click="showLogsModal = false"
            class="px-4 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold cursor-pointer"
          >
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- Modal 4: Standard Delete Subnet Confirmation Modal (AGENTS.md compliant) -->
    <div
      v-if="showDeleteSubnetModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Red circle trash icon -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Subnet?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ subnetToDelete?.name }}</strong> ({{ subnetToDelete?.cidr }})? All IP records and scan history will be permanently deleted.
          </p>
        </div>

        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteSubnetModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer font-medium"
          >
            Cancel
          </button>
          <button
            @click="confirmDeleteSubnet"
            :disabled="deletingSubnet"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deletingSubnet ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Modal 5: Standard Release IP Confirmation Modal (AGENTS.md compliant) -->
    <div
      v-if="showDeleteAddressModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Release IP Address?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to release <strong class="text-slate-800 dark:text-slate-200 font-mono">{{ addressToDelete?.ipAddress }}</strong>? This IP address will be marked as available for allocation.
          </p>
        </div>

        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteAddressModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer font-medium"
          >
            Cancel
          </button>
          <button
            @click="confirmDeleteAddress"
            :disabled="deletingAddress"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deletingAddress ? 'Releasing...' : 'Confirm Release' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
