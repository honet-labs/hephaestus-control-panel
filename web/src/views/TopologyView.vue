<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import {
  Network,
  Plus,
  RefreshCw,
  Scan,
  Server,
  Layers,
  Radio,
  Wifi,
  HardDrive,
  ShieldCheck,
  CheckCircle2,
  XCircle,
  Search,
  ArrowLeft,
  X,
  Edit2,
  Trash2,
  Activity,
  Terminal,
  Grid,
  Maximize2,
  Type,
  Download,
  Upload,
  ChevronLeft,
  ChevronRight,
  Sliders,
  ExternalLink,
  Zap,
  Cable,
  Eye,
  Router,
  Database,
  Cloud,
  Boxes,
  Box,
  Flame,
  Monitor,
  Laptop,
  Cpu,
  Globe,
  ArrowUpDown,
} from 'lucide-vue-next';
import ThemeToggle from '../components/ThemeToggle.vue';

const router = useRouter();

interface Sheet {
  id: number;
  name: string;
  sortOrder: number;
}

interface Device {
  id: string;
  name: string;
  ipAddress: string;
  deviceType: string;
  status: string;
  sources: string[];
  sheetId?: number | null;
  x?: number;
  y?: number;
  labels?: Record<string, any>;
  interfaces?: any[];
}

interface Edge {
  id?: number;
  sourceId: string;
  targetId: string;
  label?: string;
  edgeType?: string;
  sheetId?: number | null;
}

// State
const sheets = ref<Sheet[]>([]);
const activeSheetId = ref<number | null>(null);
const allDevices = ref<Device[]>([]);
const activeNodes = ref<Device[]>([]);
const edges = ref<Edge[]>([]);
const loading = ref(false);

// UI Controls & Sidebars
const isSidebarCollapsed = ref(false);
const selectedNode = ref<Device | null>(null);
const selectedEdge = ref<any | null>(null);
const selectedDiscoveredIds = ref<string[]>([]);
const searchDiscoveredQuery = ref('');
const searchCanvasQuery = ref('');
const typeFilter = ref('ALL');
const tagFilter = ref('ALL');
const showLabels = ref(true);
const isFlowLayout = ref(false);

// Modals
const isScanModalOpen = ref(false);
const isDeviceModalOpen = ref(false);
const isLinkModalOpen = ref(false);
const isEditLink = ref(false);
const isSheetModalOpen = ref(false);
const isPingModalOpen = ref(false);
const isEditDeviceModal = ref(false);

// Forms
const scanCidr = ref('192.168.102.0/24');
const scanLoading = ref(false);
const newSheetName = ref('');

const deviceForm = ref<any>({
  id: '',
  name: '',
  ipAddress: '',
  deviceType: 'server',
  status: 'unknown',
  sources: ['MANUAL'],
});

const linkForm = ref<any>({
  id: undefined,
  sourceId: '',
  targetId: '',
  edgeType: 'Ethernet',
  label: 'Ethernet',
});

// Ping Modal State
const pingTarget = ref<{ name: string; ip: string } | null>(null);
const pingOutput = ref<string>('');
const pingLoading = ref(false);

// Context Menu State
const contextMenu = ref<{
  visible: boolean;
  x: number;
  y: number;
  node: Device | null;
}>({
  visible: false,
  x: 0,
  y: 0,
  node: null,
});

const edgeContextMenu = ref<{
  visible: boolean;
  x: number;
  y: number;
  edge: any | null;
}>({
  visible: false,
  x: 0,
  y: 0,
  edge: null,
});

// HCP Standard Delete Confirmation Modal State
const deleteModal = ref<{
  visible: boolean;
  type: 'edge' | 'device' | 'sheet';
  id?: any;
  name: string;
  subtitle?: string;
  deleting: boolean;
}>({
  visible: false,
  type: 'edge',
  name: '',
  subtitle: '',
  deleting: false,
});

// Canvas Pan & Zoom
const canvasTransform = ref({ x: 0, y: 0, scale: 1 });
const isPanning = ref(false);
const panStart = ref({ x: 0, y: 0 });

// Dragging Node
const draggingNodeId = ref<string | null>(null);
const dragStart = ref({ x: 0, y: 0, nodeX: 0, nodeY: 0 });
const nodePositionVersion = ref(0);

// Filtered Discovered Devices in Left Sidebar
const filteredDiscoveredDevices = computed(() => {
  let list = allDevices.value;
  if (typeFilter.value !== 'ALL') {
    list = list.filter(d => (d.deviceType || '').toLowerCase() === typeFilter.value.toLowerCase());
  }
  if (searchDiscoveredQuery.value) {
    const q = searchDiscoveredQuery.value.toLowerCase();
    list = list.filter(d => d.name.toLowerCase().includes(q) || d.ipAddress.toLowerCase().includes(q));
  }
  return list;
});

const standardDeviceTypes = [
  'SERVER',
  'ROUTER',
  'SWITCH',
  'ACCESS POINT',
  'DATABASE',
  'DOCKER',
  'CLOUD',
  'FIREWALL',
  'NAS',
  'WORKSTATION',
  'VM',
  'INTERNET',
];

// Discovered types list
const availableTypes = computed(() => {
  const set = new Set<string>(standardDeviceTypes);
  allDevices.value.forEach(d => {
    if (d.deviceType) set.add(d.deviceType.toUpperCase());
  });
  return Array.from(set);
});

// Check if a device is added to current sheet
const isDeviceOnCanvas = (devId: string) => {
  return activeNodes.value.some(n => n.id === devId);
};

// Connected Edges for Selected Node in Detail Drawer
const selectedNodeConnections = computed(() => {
  if (!selectedNode.value) return [];
  const node = selectedNode.value;
  return edges.value
    .filter(e => e.sourceId === node.id || e.targetId === node.id)
    .map(e => {
      const peerId = e.sourceId === node.id ? e.targetId : e.sourceId;
      const peer = activeNodes.value.find(n => n.id === peerId) || allDevices.value.find(n => n.id === peerId);
      return {
        edge: e,
        peerName: peer ? peer.name : peerId,
        peerIP: peer ? peer.ipAddress : '',
        edgeType: e.edgeType || e.label || 'Ethernet',
      };
    });
});

// Fetch Sheets
const fetchSheets = async () => {
  try {
    const res = await axios.get('/api/v1/topology/sheets');
    if (res.data.success && res.data.data && res.data.data.length > 0) {
      sheets.value = res.data.data;
      if (!activeSheetId.value) {
        activeSheetId.value = sheets.value[0].id;
      }
    } else {
      // Default Sheet
      const defaultSheet = await axios.post('/api/v1/topology/sheets', { name: 'Honet-labs Topology', sortOrder: 0 });
      if (defaultSheet.data.success) {
        sheets.value = [defaultSheet.data.data];
        activeSheetId.value = defaultSheet.data.data.id;
      }
    }
    await fetchGraph();
  } catch (err) {
    console.error('Failed to load topology sheets:', err);
  }
};

// Fetch Graph Nodes and Edges
const fetchGraph = async () => {
  loading.value = true;
  try {
    const [graphRes, allDevsRes] = await Promise.all([
      axios.get(activeSheetId.value ? `/api/v1/topology?sheetId=${activeSheetId.value}` : '/api/v1/topology'),
      axios.get('/api/v1/topology'),
    ]);

    if (graphRes.data.success) {
      const nodes: Device[] = graphRes.data.data.nodes || [];
      // Assign default positions if unset
      nodes.forEach((n, idx) => {
        if (n.x === undefined || n.x === null || n.x === 0) {
          n.x = 220 + (idx % 4) * 200 + (Math.floor(idx / 4) % 2) * 50;
        }
        if (n.y === undefined || n.y === null || n.y === 0) {
          n.y = 120 + Math.floor(idx / 4) * 160;
        }
      });
      activeNodes.value = nodes;
      edges.value = graphRes.data.data.edges || [];
    }

    if (allDevsRes.data.success) {
      allDevices.value = allDevsRes.data.data.nodes || [];
    }
  } catch (err) {
    console.error('Failed to fetch topology graph:', err);
  } finally {
    loading.value = false;
  }
};

// Select Sheet
const handleSelectSheet = (sheetId: number) => {
  activeSheetId.value = sheetId;
  selectedNode.value = null;
  selectedEdge.value = null;
  fetchGraph();
};

// Add Sheet
const handleCreateSheet = async () => {
  if (!newSheetName.value.trim()) return;
  try {
    const res = await axios.post('/api/v1/topology/sheets', {
      name: newSheetName.value.trim(),
      sortOrder: sheets.value.length,
    });
    if (res.data.success) {
      sheets.value.push(res.data.data);
      activeSheetId.value = res.data.data.id;
      isSheetModalOpen.value = false;
      newSheetName.value = '';
      fetchGraph();
    }
  } catch (err) {
    console.error('Failed to create sheet:', err);
  }
};

// Delete Sheet (using HCP Standard Modal)
const promptDeleteSheet = (sheetId: number, e: MouseEvent) => {
  e.stopPropagation();
  if (sheets.value.length <= 1) {
    alert('You must have at least one topology sheet.');
    return;
  }
  const s = sheets.value.find(item => item.id === sheetId);
  deleteModal.value = {
    visible: true,
    type: 'sheet',
    id: sheetId,
    name: s ? s.name : `Sheet #${sheetId}`,
    subtitle: `Are you sure you want to delete the sheet "${s ? s.name : sheetId}"? All device placements on this sheet will be cleared.`,
    deleting: false,
  };
};

// Toggle Device On Canvas
const handleAddDeviceToCanvas = async (dev: Device) => {
  try {
    const updated = {
      ...dev,
      sheetId: activeSheetId.value,
      x: 300 + (activeNodes.value.length % 3) * 180,
      y: 180 + Math.floor(activeNodes.value.length / 3) * 140,
    };
    await axios.post('/api/v1/topology/devices', updated);
    await fetchGraph();
  } catch (err) {
    console.error('Failed to add device to sheet:', err);
  }
};

// Add Selected Discovered Devices to Canvas
const handleAddSelectedToCanvas = async () => {
  for (const id of selectedDiscoveredIds.value) {
    const dev = allDevices.value.find(d => d.id === id);
    if (dev) {
      await handleAddDeviceToCanvas(dev);
    }
  }
  selectedDiscoveredIds.value = [];
};

// Select All Discovered Devices
const handleSelectAllDiscovered = () => {
  selectedDiscoveredIds.value = filteredDiscoveredDevices.value.map(d => d.id);
};

const handleClearSelectedDiscovered = () => {
  selectedDiscoveredIds.value = [];
};

// Open Add Device Modal
const handleOpenAddDevice = () => {
  deviceForm.value = {
    id: '',
    name: '',
    ipAddress: '',
    deviceType: 'server',
    status: 'unknown',
    sources: ['MANUAL'],
    sheetId: null,
    x: null,
    y: null,
  };
  isEditDeviceModal.value = false;
  isDeviceModalOpen.value = true;
};

// Save / Update Device
const handleSaveDevice = async () => {
  try {
    const isEdit = isEditDeviceModal.value;
    const currentSheet = activeSheetId.value ?? (sheets.value.length > 0 ? sheets.value[0].id : null);
    const startIdx = activeNodes.value.length;
    const defaultX = 220 + (startIdx % 4) * 200;
    const defaultY = 130 + Math.floor(startIdx / 4) * 160;

    const payload = {
      ...deviceForm.value,
      // Place newly added manual devices directly onto the active canvas sheet
      sheetId: isEdit ? (deviceForm.value.sheetId ?? currentSheet) : currentSheet,
      x: isEdit ? (deviceForm.value.x ?? defaultX) : defaultX,
      y: isEdit ? (deviceForm.value.y ?? defaultY) : defaultY,
    };
    const res = await axios.post('/api/v1/topology/devices', payload);
    if (res.data.success) {
      isDeviceModalOpen.value = false;
      isEditDeviceModal.value = false;
      deviceForm.value = {
        id: '',
        name: '',
        ipAddress: '',
        deviceType: 'server',
        status: 'unknown',
        sources: ['MANUAL'],
        sheetId: null,
        x: null,
        y: null,
      };
      await fetchGraph();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to save device');
  }
};

// Remove Device from Active Canvas Sheet (Keeps device in sidebar inventory!)
const handleRemoveDeviceFromCanvas = async (deviceId: string) => {
  try {
    await axios.post(`/api/v1/topology/devices/${deviceId}/remove-from-canvas`, {
      sheetId: activeSheetId.value,
    });
    if (selectedNode.value?.id === deviceId) {
      selectedNode.value = null;
    }
    await fetchGraph();
  } catch (err: any) {
    console.error('Failed to remove device from canvas:', err);
    alert(err.response?.data?.error || 'Failed to remove device from canvas');
  }
};

// Permanently Delete Device from Inventory (using HCP Standard Modal)
const promptDeleteDevice = (deviceId: string) => {
  const dev = allDevices.value.find(d => d.id === deviceId);
  const devName = dev ? dev.name : deviceId;
  deleteModal.value = {
    visible: true,
    type: 'device',
    id: deviceId,
    name: devName,
    subtitle: `Are you sure you want to permanently delete "${devName}" from the system inventory? This action cannot be undone.`,
    deleting: false,
  };
  contextMenu.value.visible = false;
};

// Open Edit Device Modal
const handleOpenEditDevice = (dev: Device) => {
  deviceForm.value = {
    id: dev.id,
    name: dev.name,
    ipAddress: dev.ipAddress,
    deviceType: dev.deviceType,
    status: dev.status,
    sources: dev.sources || ['MANUAL'],
    sheetId: dev.sheetId,
    x: dev.x,
    y: dev.y,
  };
  isEditDeviceModal.value = true;
};

// Open Add Link Modal
const handleOpenAddLink = () => {
  isEditLink.value = false;
  linkForm.value = {
    id: undefined,
    sourceId: '',
    targetId: '',
    edgeType: 'Ethernet',
    label: 'Ethernet',
  };
  isLinkModalOpen.value = true;
};

// Open Edit Link Modal
const handleOpenEditEdge = (edge: any) => {
  isEditLink.value = true;
  linkForm.value = {
    id: edge.id,
    sourceId: edge.sourceId,
    targetId: edge.targetId,
    edgeType: edge.edgeType || 'Ethernet',
    label: edge.label || edge.edgeType || 'Ethernet',
  };
  edgeContextMenu.value.visible = false;
  isLinkModalOpen.value = true;
};

const onLinkTypeChange = () => {
  if (!isEditLink.value || !linkForm.value.label || linkForm.value.label === 'Ethernet' || linkForm.value.label === 'VPN' || linkForm.value.label === 'Wireless' || linkForm.value.label === 'Fiber') {
    linkForm.value.label = linkForm.value.edgeType;
  }
};

// Save or Update Edge / Link
const handleSaveLink = async () => {
  if (!linkForm.value.sourceId || !linkForm.value.targetId) {
    alert('Please select both source and target devices');
    return;
  }
  try {
    const payload: any = {
      sourceId: linkForm.value.sourceId,
      targetId: linkForm.value.targetId,
      edgeType: linkForm.value.edgeType,
      label: linkForm.value.label,
      sheetId: activeSheetId.value,
    };
    if (isEditLink.value && linkForm.value.id) {
      payload.id = linkForm.value.id;
    }
    const res = await axios.post('/api/v1/topology/edges', payload);
    if (res.data.success) {
      isLinkModalOpen.value = false;
      isEditLink.value = false;
      linkForm.value = { id: undefined, sourceId: '', targetId: '', edgeType: 'Ethernet', label: 'Ethernet' };
      await fetchGraph();
      if (selectedEdge.value && payload.id && selectedEdge.value.id === payload.id) {
        const fresh = renderedEdges.value.find(e => e.id === payload.id);
        if (fresh) selectedEdge.value = fresh;
      }
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to save link');
  }
};

// Delete Edge Prompt (using HCP Standard Modal)
const promptDeleteEdge = (edge: any) => {
  const sName = edge.sourceName || (activeNodes.value.find(n => n.id === edge.sourceId)?.name) || edge.sourceId;
  const tName = edge.targetName || (activeNodes.value.find(n => n.id === edge.targetId)?.name) || edge.targetId;
  deleteModal.value = {
    visible: true,
    type: 'edge',
    id: edge.id,
    name: `${edge.edgeType || 'Connection'} (${sName} ↔ ${tName})`,
    subtitle: `Are you sure you want to remove the link between "${sName}" and "${tName}"? This action cannot be undone.`,
    deleting: false,
  };
  edgeContextMenu.value.visible = false;
};

// Generic Delete Confirmation Execution (HCP Standard)
const executeDelete = async () => {
  if (deleteModal.value.deleting) return;
  deleteModal.value.deleting = true;
  try {
    if (deleteModal.value.type === 'edge') {
      await axios.delete(`/api/v1/topology/edges/${deleteModal.value.id}`);
      if (selectedEdge.value?.id === deleteModal.value.id) {
        selectedEdge.value = null;
      }
    } else if (deleteModal.value.type === 'device') {
      await axios.delete(`/api/v1/topology/devices/${deleteModal.value.id}`);
      if (selectedNode.value?.id === deleteModal.value.id) {
        selectedNode.value = null;
      }
    } else if (deleteModal.value.type === 'sheet') {
      await axios.delete(`/api/v1/topology/sheets/${deleteModal.value.id}`);
      sheets.value = sheets.value.filter(s => s.id !== deleteModal.value.id);
      if (activeSheetId.value === deleteModal.value.id) {
        activeSheetId.value = sheets.value[0]?.id || null;
      }
    }
    deleteModal.value.visible = false;
    await fetchGraph();
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete');
  } finally {
    deleteModal.value.deleting = false;
  }
};

// Trigger ICMP Ping Modal & Live Execution
const handleTriggerPing = async (dev: Device) => {
  pingTarget.value = { name: dev.name, ip: dev.ipAddress };
  pingOutput.value = `PING ${dev.ipAddress} (${dev.ipAddress}): 56 data bytes...\n`;
  pingLoading.value = true;
  isPingModalOpen.value = true;

  try {
    const res = await axios.get(`/api/v1/topology/ping?ip=${encodeURIComponent(dev.ipAddress)}`);
    if (res.data.success && res.data.output) {
      pingOutput.value = res.data.output;
    } else {
      pingOutput.value = `Error executing ping for ${dev.ipAddress}`;
    }
  } catch (err: any) {
    pingOutput.value = `PING Error: ${err.response?.data?.error || err.message}`;
  } finally {
    pingLoading.value = false;
  }
};

// Scan Subnet Discovery
const handleRunSubnetScan = async () => {
  if (!scanCidr.value) return;
  scanLoading.value = true;
  try {
    const res = await axios.get(`/api/v1/topology/discover/subnet?cidr=${encodeURIComponent(scanCidr.value)}`);
    if (res.data.success) {
      isScanModalOpen.value = false;
      alert(`Subnet scan completed! Discovered ${res.data.data.length} reachable devices.`);
      await fetchGraph();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Subnet scan failed');
  } finally {
    scanLoading.value = false;
  }
};

// Sync Prometheus Discovery
const handleSyncPrometheus = async () => {
  try {
    loading.value = true;
    const res = await axios.get('/api/v1/topology/discover/prometheus');
    if (res.data.success) {
      alert(`Synced ${res.data.data.length} targets from Prometheus.`);
      await fetchGraph();
    }
  } catch (err: any) {
    alert(`Sync Prometheus failed: ${err.response?.data?.error || err.message}`);
  } finally {
    loading.value = false;
  }
};

// =================================================================
// REMOTE SERVER SYNC & INTEGRATION
// =================================================================
const isRemoteSyncModalOpen = ref(false);
const syncRemoteLoading = ref(false);
const remoteHosts = ref<any[]>([]);
const selectedRemoteHostIds = ref<string[]>([]);
const syncToastMessage = ref<string | null>(null);

const showSyncToast = (msg: string) => {
  syncToastMessage.value = msg;
  setTimeout(() => {
    syncToastMessage.value = null;
  }, 4500);
};

// Open Remote Server Sync Modal
const openRemoteSyncModal = async () => {
  isRemoteSyncModalOpen.value = true;
  syncRemoteLoading.value = true;
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data.success && Array.isArray(res.data.data)) {
      remoteHosts.value = res.data.data;
      selectedRemoteHostIds.value = remoteHosts.value.map((h: any) => h.id);
    }
  } catch (err: any) {
    console.error('Failed to load remote hosts:', err);
  } finally {
    syncRemoteLoading.value = false;
  }
};

// Check if a remote host is already in active sheet
const isRemoteHostInActiveSheet = (host: any) => {
  return activeNodes.value.some(
    n => n.id === `remote-${host.id}` || n.labels?.remoteHostId === host.id
  );
};

// Toggle all remote hosts selection
const toggleSelectAllRemoteHosts = () => {
  if (selectedRemoteHostIds.value.length === remoteHosts.value.length) {
    selectedRemoteHostIds.value = [];
  } else {
    selectedRemoteHostIds.value = remoteHosts.value.map(h => h.id);
  }
};

// Quick Sync All
const handleDirectSyncAllRemoteServers = async () => {
  syncRemoteLoading.value = true;
  try {
    const targetSheetId = activeSheetId.value ?? (sheets.value.length > 0 ? sheets.value[0].id : null);
    const sheetParam = targetSheetId ? `?sheetId=${targetSheetId}` : '';
    const res = await axios.post(`/api/v1/topology/sync/remote-server${sheetParam}`);
    if (res.data.success) {
      isRemoteSyncModalOpen.value = false;
      showSyncToast(res.data.message || `Synced ${res.data.data?.length || 0} remote servers!`);
      await fetchGraph();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to sync remote servers');
  } finally {
    syncRemoteLoading.value = false;
  }
};

// Sync Selected Remote Servers
const handleSyncSelectedRemoteServers = async () => {
  if (selectedRemoteHostIds.value.length === 0) return;
  syncRemoteLoading.value = true;
  try {
    const targets = remoteHosts.value.filter(h => selectedRemoteHostIds.value.includes(h.id));
    let count = 0;
    const baseX = 220;
    const baseY = 130;
    const startIdx = activeNodes.value.length;
    const targetSheetId = activeSheetId.value ?? (sheets.value.length > 0 ? sheets.value[0].id : null);

    for (let i = 0; i < targets.length; i++) {
      const h = targets[i];
      // Match existing remote server node only, never overwrite manual devices
      const existing = activeNodes.value.find(
        n => n.id === `remote-${h.id}` || n.labels?.remoteHostId === h.id
      );

      const x = existing?.x ?? (baseX + ((startIdx + i) % 4) * 200);
      const y = existing?.y ?? (baseY + Math.floor((startIdx + i) / 4) * 160);

      await axios.post('/api/v1/topology/devices', {
        id: existing?.id || `remote-${h.id}`,
        name: h.name,
        ipAddress: h.host,
        deviceType: 'server',
        status: 'online',
        sources: ['REMOTE', 'SSH'],
        sheetId: targetSheetId,
        x,
        y,
        labels: {
          remoteHostId: h.id,
          port: h.port,
          username: h.username,
          groupName: h.groupName,
          tags: h.tags,
          authType: h.authType,
        },
      });
      count++;
    }

    isRemoteSyncModalOpen.value = false;
    showSyncToast(`Successfully synced ${count} remote server(s) to active sheet!`);
    await fetchGraph();
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to sync selected remote servers');
  } finally {
    syncRemoteLoading.value = false;
  }
};

// Connect SSH Terminal directly from Topology Node
const handleConnectSSHFromTopology = (node: Device) => {
  const hostId = node.labels?.remoteHostId;
  if (hostId) {
    window.open(`/remote-server?hostId=${encodeURIComponent(hostId)}`, '_blank');
  } else {
    window.open(`/remote-server`, '_blank');
  }
};

// Auto Flow Layout (Hierarchical Arrangement)
const handleToggleFlowLayout = () => {
  isFlowLayout.value = !isFlowLayout.value;
  if (!isFlowLayout.value) return;

  const startX = 200;
  const startY = 140;
  const gapX = 220;
  const gapY = 160;

  activeNodes.value.forEach((node, i) => {
    node.x = startX + (i % 4) * gapX;
    node.y = startY + Math.floor(i / 4) * gapY;
    axios.put(`/api/v1/topology/devices/${node.id}/position`, { x: node.x, y: node.y }).catch(() => {});
  });
  nodePositionVersion.value++;
};

// Reset / Center Canvas
const handleFitCanvas = () => {
  canvasTransform.value = { x: 0, y: 0, scale: 1 };
};

// Canvas Mouse Interactions (Pan & Zoom)
const handleCanvasMouseDown = (e: MouseEvent) => {
  // If clicked on empty canvas, start panning and deselect
  if ((e.target as HTMLElement).tagName === 'svg' || (e.target as HTMLElement).id === 'topology-canvas-bg') {
    isPanning.value = true;
    panStart.value = { x: e.clientX - canvasTransform.value.x, y: e.clientY - canvasTransform.value.y };
    contextMenu.value.visible = false;
    edgeContextMenu.value.visible = false;
    selectedEdge.value = null;
  }
};

const handleCanvasMouseMove = (e: MouseEvent) => {
  if (isPanning.value) {
    canvasTransform.value.x = e.clientX - panStart.value.x;
    canvasTransform.value.y = e.clientY - panStart.value.y;
  }
};

const handleCanvasMouseUp = () => {
  isPanning.value = false;
};

const handleCanvasWheel = (e: WheelEvent) => {
  e.preventDefault();
  const zoomFactor = e.deltaY < 0 ? 1.1 : 0.9;
  const newScale = Math.min(Math.max(canvasTransform.value.scale * zoomFactor, 0.4), 2.5);
  canvasTransform.value.scale = newScale;
};

// Node Dragging Handlers with Global Window Event Listeners
const handleWindowMouseMove = (e: MouseEvent) => {
  if (draggingNodeId.value) {
    const node = activeNodes.value.find(n => String(n.id) === String(draggingNodeId.value));
    if (node) {
      const dx = (e.clientX - dragStart.value.x) / canvasTransform.value.scale;
      const dy = (e.clientY - dragStart.value.y) / canvasTransform.value.scale;
      node.x = Math.round(dragStart.value.nodeX + dx);
      node.y = Math.round(dragStart.value.nodeY + dy);
      nodePositionVersion.value++;
    }
  }
};

const handleWindowMouseUp = () => {
  if (draggingNodeId.value) {
    const node = activeNodes.value.find(n => String(n.id) === String(draggingNodeId.value));
    if (node && node.x !== undefined && node.y !== undefined) {
      axios.put(`/api/v1/topology/devices/${node.id}/position`, { x: node.x, y: node.y }).catch(() => {});
    }
    draggingNodeId.value = null;
    nodePositionVersion.value++;
  }
  window.removeEventListener('mousemove', handleWindowMouseMove);
  window.removeEventListener('mouseup', handleWindowMouseUp);
};

const handleNodeMouseDown = (dev: Device, e: MouseEvent) => {
  e.stopPropagation();
  contextMenu.value.visible = false;
  edgeContextMenu.value.visible = false;
  selectedEdge.value = null;
  if (e.button === 0) {
    // Left click
    draggingNodeId.value = dev.id;
    dragStart.value = {
      x: e.clientX,
      y: e.clientY,
      nodeX: Number(dev.x) || 0,
      nodeY: Number(dev.y) || 0,
    };
    selectedNode.value = dev;

    window.addEventListener('mousemove', handleWindowMouseMove);
    window.addEventListener('mouseup', handleWindowMouseUp);
  }
};

// Node Context Menu (Right Click)
const handleNodeContextMenu = (dev: Device, e: MouseEvent) => {
  e.preventDefault();
  e.stopPropagation();
  selectedNode.value = dev;
  selectedEdge.value = null;
  edgeContextMenu.value.visible = false;
  contextMenu.value = {
    visible: true,
    x: e.clientX,
    y: e.clientY,
    node: dev,
  };
};

// Edge / Link Left Click
const handleEdgeClick = (edge: any) => {
  selectedEdge.value = edge;
  selectedNode.value = null;
  edgeContextMenu.value.visible = false;
  contextMenu.value.visible = false;
};

// Edge / Link Right Click Context Menu
const handleEdgeContextMenu = (edge: any, e: MouseEvent) => {
  e.preventDefault();
  e.stopPropagation();
  selectedEdge.value = edge;
  selectedNode.value = null;
  contextMenu.value.visible = false;
  edgeContextMenu.value = {
    visible: true,
    x: e.clientX,
    y: e.clientY,
    edge,
  };
};

// Computed edges with multi-link separation and real-time reactive paths
const renderedEdges = computed(() => {
  // Read nodePositionVersion to guarantee re-computation on every drag movement
  nodePositionVersion.value;

  const nodeMap = new Map<string, Device>();
  for (const n of activeNodes.value) {
    if (n.id !== undefined && n.id !== null) {
      nodeMap.set(String(n.id), n);
    }
  }

  // Group edges by canonical pair key to detect multiple links between the same devices
  const pairGroups = new Map<string, Edge[]>();
  for (const edge of edges.value) {
    const sId = String(edge.sourceId);
    const tId = String(edge.targetId);
    const pairKey = [sId, tId].sort().join(':::');
    if (!pairGroups.has(pairKey)) {
      pairGroups.set(pairKey, []);
    }
    pairGroups.get(pairKey)!.push(edge);
  }

  return edges.value.map((edge, idx) => {
    const sId = String(edge.sourceId);
    const tId = String(edge.targetId);
    const source = nodeMap.get(sId);
    const target = nodeMap.get(tId);

    if (!source || !target || source.x === undefined || source.y === undefined || target.x === undefined || target.y === undefined) {
      return {
        ...edge,
        sourceNode: source,
        targetNode: target,
        sourceName: source?.name || sId,
        targetName: target?.name || tId,
        sourceIP: source?.ipAddress || '',
        targetIP: target?.ipAddress || '',
        renderKey: edge.id || `edge-${edge.sourceId}-${edge.targetId}-${idx}`,
        path: '',
        midX: 0,
        midY: 0,
        valid: false,
      };
    }

    const x1 = Number(source.x);
    const y1 = Number(source.y);
    const x2 = Number(target.x);
    const y2 = Number(target.y);

    const dx = x2 - x1;
    const dy = y2 - y1;
    const len = Math.sqrt(dx * dx + dy * dy) || 1;
    const nx = -dy / len;
    const ny = dx / len;

    // Calculate parallel link curvature offset
    const pairKey = [sId, tId].sort().join(':::');
    const group = pairGroups.get(pairKey) || [edge];
    const groupCount = group.length;
    const indexInGroup = group.indexOf(edge);

    let offset = 0;
    if (groupCount > 1) {
      // Fan out multiple links symmetrically: e.g. -24, +24 for 2 links; -36, 0, +36 for 3 links
      offset = (indexInGroup - (groupCount - 1) / 2) * 36;
      // Ensure canonical orientation consistency so A->B and B->A curve consistently
      const isCanonical = sId <= tId;
      if (!isCanonical) {
        offset = -offset;
      }
    }

    // Displace cubic bezier control points along normal vector
    const cx1 = x1 + dx * 0.25 + nx * offset;
    const cy1 = y1 + dy * 0.25 + ny * offset;
    const cx2 = x1 + dx * 0.75 + nx * offset;
    const cy2 = y1 + dy * 0.75 + ny * offset;

    const path = `M ${x1} ${y1} C ${cx1} ${cy1}, ${cx2} ${cy2}, ${x2} ${y2}`;
    // Precise midpoint of cubic bezier at t = 0.5:
    const midX = 0.125 * x1 + 0.375 * cx1 + 0.375 * cx2 + 0.125 * x2;
    const midY = 0.125 * y1 + 0.375 * cy1 + 0.375 * cy2 + 0.125 * y2;

    return {
      ...edge,
      sourceNode: source,
      targetNode: target,
      sourceName: source.name,
      targetName: target.name,
      sourceIP: source.ipAddress,
      targetIP: target.ipAddress,
      renderKey: edge.id || `edge-${edge.sourceId}-${edge.targetId}-${idx}`,
      path,
      midX,
      midY,
      valid: true,
    };
  });
});

// Return node icon component based on type
const getNodeIcon = (type?: string) => {
  const t = (type || '').toLowerCase();
  if (t.includes('router') || t.includes('rtr')) return Router;
  if (t.includes('switch') || t.includes('sw') || t.includes('hub')) return Network;
  if (t.includes('ap') || t.includes('wifi') || t.includes('access') || t.includes('wireless') || t.includes('radio') || t.includes('hotspot')) return Wifi;
  if (t.includes('database') || t.includes('db') || t.includes('sql') || t.includes('postgres') || t.includes('mysql') || t.includes('mongo') || t.includes('redis') || t.includes('opensearch') || t.includes('elasticsearch')) return Database;
  if (t.includes('docker') || t.includes('container') || t.includes('k8s') || t.includes('kubernetes') || t.includes('pod') || t.includes('swarm')) return Boxes;
  if (t.includes('cloud') || t.includes('aws') || t.includes('azure') || t.includes('gcp') || t.includes('cloudflare') || t.includes('saas') || t.includes('cdn')) return Cloud;
  if (t.includes('firewall') || t.includes('fw') || t.includes('shield') || t.includes('pfsense') || t.includes('forti') || t.includes('waf') || t.includes('security')) return ShieldCheck;
  if (t.includes('nas') || t.includes('storage') || t.includes('san') || t.includes('disk') || t.includes('backup') || t.includes('drive') || t.includes('hdd') || t.includes('ssd')) return HardDrive;
  if (t.includes('workstation') || t.includes('pc') || t.includes('desktop') || t.includes('laptop') || t.includes('client') || t.includes('computer') || t.includes('endpoint')) return Monitor;
  if (t.includes('vm') || t.includes('virtual') || t.includes('proxmox') || t.includes('esxi') || t.includes('vps') || t.includes('kvm')) return Cpu;
  if (t.includes('internet') || t.includes('wan') || t.includes('isp') || t.includes('gateway') || t.includes('web') || t.includes('public')) return Globe;
  return Server;
};

// Close all context menus on global click
const handleGlobalClick = () => {
  contextMenu.value.visible = false;
  edgeContextMenu.value.visible = false;
};

onMounted(() => {
  fetchSheets();
  window.addEventListener('click', handleGlobalClick);
});

onUnmounted(() => {
  window.removeEventListener('click', handleGlobalClick);
  window.removeEventListener('mousemove', handleWindowMouseMove);
  window.removeEventListener('mouseup', handleWindowMouseUp);
});
</script>

<template>
  <div class="h-screen w-screen bg-slate-100 dark:bg-[#111317] text-slate-800 dark:text-slate-200 font-sans flex flex-col overflow-hidden select-none">
    
    <!-- ================================================================= -->
    <!-- TOP TOOLBAR & HEADER -->
    <!-- ================================================================= -->
    <header class="h-12 bg-white dark:bg-[#171a21] border-b border-slate-200 dark:border-slate-800/90 px-4 flex items-center justify-between shrink-0 z-30">
      <!-- Left: Back & Title -->
      <div class="flex items-center gap-3">
        <button
          @click="router.push('/')"
          class="flex items-center gap-1.5 px-2.5 py-1 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] border border-slate-300 dark:border-slate-700/60 text-xs text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:border-slate-400 transition font-medium"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Portal</span>
        </button>
        <div class="flex items-center gap-2">
          <Network class="w-4 h-4 text-emerald-500" />
          <h1 class="text-xs font-semibold text-slate-900 dark:text-white tracking-wide">Network Topology</h1>
        </div>
      </div>

      <!-- Center Toolbar Actions -->
      <div class="flex items-center gap-2">
        <!-- Sync Remote Server Button -->
        <button
          @click="openRemoteSyncModal"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-blue-500/70 text-blue-600 dark:text-blue-400 hover:bg-blue-500/10 text-xs font-semibold tracking-wider transition"
          title="Sync registered Remote Server (SSH/SFTP) hosts into active Topology sheet"
        >
          <Terminal class="w-3.5 h-3.5" />
          <span>SYNC REMOTE SERVER</span>
        </button>

        <button
          @click="isScanModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-emerald-500/80 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-500/10 text-xs font-semibold tracking-wider transition"
        >
          <Scan class="w-3.5 h-3.5" />
          <span>+ SCAN</span>
        </button>

        <button
          @click="handleOpenAddDevice"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-[#282d3a] border border-slate-300 dark:border-slate-700 text-xs font-medium text-slate-700 dark:text-slate-200 hover:text-slate-900 dark:hover:text-white transition"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>+ DEVICE</span>
        </button>

        <button
          @click="handleOpenAddLink"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-[#282d3a] border border-slate-300 dark:border-slate-700 text-xs font-medium text-slate-700 dark:text-slate-200 hover:text-slate-900 dark:hover:text-white transition cursor-pointer"
        >
          <Cable class="w-3.5 h-3.5" />
          <span>+ LINK</span>
        </button>

        <!-- Canvas Search Input -->
        <div class="relative w-48 ml-2">
          <Search class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 absolute left-2.5 top-2.5" />
          <input
            v-model="searchCanvasQuery"
            placeholder="Search devices... (Ctrl+F)"
            class="w-full bg-slate-50 dark:bg-[#111317] border border-slate-300 dark:border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:border-brand-500 transition font-mono"
          />
        </div>
      </div>

      <!-- Right Controls & Status -->
      <div class="flex items-center gap-2">
        <button
          @click="handleToggleFlowLayout"
          :class="[
            'px-2.5 py-1.5 rounded-lg border text-xs font-mono transition flex items-center gap-1',
            isFlowLayout
              ? 'bg-blue-600 border-blue-500 text-white'
              : 'bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] border-slate-300 dark:border-slate-700 text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          ]"
          title="Auto Flow Layout"
        >
          <Network class="w-3.5 h-3.5" />
          <span>FLOW LAYOUT</span>
        </button>

        <button
          @click="handleFitCanvas"
          class="p-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] border border-slate-300 dark:border-slate-700 text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white transition"
          title="Fit / Center View"
        >
          <Maximize2 class="w-3.5 h-3.5" />
        </button>

        <button
          @click="showLabels = !showLabels"
          :class="[
            'p-1.5 rounded-lg border transition',
            showLabels ? 'bg-slate-200 dark:bg-slate-700 border-slate-300 dark:border-slate-600 text-slate-900 dark:text-white' : 'bg-slate-100 dark:bg-[#20242e] border-slate-300 dark:border-slate-700 text-slate-600 dark:text-slate-400'
          ]"
          title="Toggle Labels"
        >
          <Type class="w-3.5 h-3.5" />
        </button>

        <button
          @click="fetchGraph"
          class="p-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] border border-slate-300 dark:border-slate-700 text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white transition"
          title="Refresh Graph"
        >
          <RefreshCw :class="['w-3.5 h-3.5', loading ? 'animate-spin text-brand-400' : '']" />
        </button>

        <!-- Theme Toggle -->
        <ThemeToggle variant="compact" />

        <!-- Node & Edge Pill Badges -->
        <div class="flex items-center gap-1 text-[11px] font-mono ml-2">
          <span class="px-2 py-0.5 rounded bg-slate-200/80 dark:bg-slate-800/90 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700/60">
            Nodes: <strong class="text-slate-900 dark:text-white">{{ activeNodes.length }}</strong>
          </span>
          <span class="px-2 py-0.5 rounded bg-slate-200/80 dark:bg-slate-800/90 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700/60">
            Edges: <strong class="text-slate-900 dark:text-white">{{ edges.length }}</strong>
          </span>
        </div>
      </div>
    </header>

    <!-- ================================================================= -->
    <!-- SHEET TABS BAR -->
    <!-- ================================================================= -->
    <div class="bg-slate-50 dark:bg-[#171a21] border-b border-slate-200 dark:border-slate-800/80 px-4 flex items-center gap-1.5 text-xs shrink-0 py-1 overflow-x-auto z-20">
      <div
        v-for="s in sheets"
        :key="s.id"
        @click="handleSelectSheet(s.id)"
        :class="[
          'flex items-center gap-2 px-3 py-1.5 rounded-t-lg text-xs font-medium transition cursor-pointer border-t border-x',
          activeSheetId === s.id
            ? 'bg-white dark:bg-[#111317] border-slate-200 dark:border-slate-800 text-blue-600 dark:text-brand-400 font-bold shadow-sm'
            : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/30'
        ]"
      >
        <span>{{ s.name }}</span>
        <button
          v-if="sheets.length > 1"
          @click="promptDeleteSheet(s.id, $event)"
          class="p-0.5 hover:text-rose-500 rounded transition cursor-pointer"
          title="Delete Sheet"
        >
          <X class="w-3 h-3" />
        </button>
      </div>

      <!-- Add Sheet (+) Button -->
      <button
        @click="isSheetModalOpen = true"
        title="Add New Topology Sheet"
        class="p-1.5 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition"
      >
        <Plus class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- ================================================================= -->
    <!-- MAIN WORKSPACE: SIDEBAR + GRAPH CANVAS + RIGHT DRAWER -->
    <!-- ================================================================= -->
    <div class="flex-1 flex overflow-hidden relative">

      <!-- 1. LEFT SIDEBAR: DISCOVERED DEVICES -->
      <aside
        :class="[
          'bg-white dark:bg-[#171a21] border-r border-slate-200 dark:border-slate-800 flex flex-col transition-all duration-300 z-20 shrink-0 select-none',
          isSidebarCollapsed ? 'w-0 overflow-hidden' : 'w-72'
        ]"
      >
        <!-- Sidebar Header -->
        <div class="p-3 border-b border-slate-200 dark:border-slate-800/80 flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold text-slate-900 dark:text-white tracking-wide">Discovered Devices</span>
            <span class="px-1.5 py-0.2 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 text-[10px] font-mono">
              {{ allDevices.length }}
            </span>
          </div>
        </div>

        <!-- Multi-select Action Buttons -->
        <div class="p-2.5 bg-slate-50 dark:bg-[#14161b] border-b border-slate-200 dark:border-slate-800 flex items-center gap-2">
          <button
            @click="handleAddSelectedToCanvas"
            :disabled="selectedDiscoveredIds.length === 0"
            class="flex-1 py-1.5 rounded bg-blue-600 hover:bg-blue-500 disabled:bg-slate-200 disabled:text-slate-400 disabled:cursor-not-allowed dark:disabled:bg-slate-800 dark:disabled:text-slate-500 text-white font-bold text-[10px] tracking-wider uppercase transition shadow-sm cursor-pointer"
          >
            Add Selected
          </button>
          <button
            @click="handleSelectAllDiscovered"
            class="px-2.5 py-1.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 text-[10px] font-medium border border-slate-300 dark:border-slate-700 transition cursor-pointer"
          >
            Select All
          </button>
          <button
            @click="handleClearSelectedDiscovered"
            class="px-2.5 py-1.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 text-[10px] font-medium border border-slate-300 dark:border-slate-700 transition cursor-pointer"
          >
            Clear
          </button>
        </div>

        <!-- Filter Inputs -->
        <div class="p-2.5 space-y-2 border-b border-slate-800/80">
          <input
            v-model="searchDiscoveredQuery"
            placeholder="Filter devices..."
            class="w-full bg-[#111317] border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
          />

          <div class="grid grid-cols-2 gap-2">
            <select
              v-model="typeFilter"
              class="bg-[#111317] border border-slate-800 text-slate-300 text-[11px] rounded-lg px-2 py-1 focus:outline-none"
            >
              <option value="ALL">All Types</option>
              <option v-for="t in availableTypes" :key="t" :value="t">{{ t }}</option>
            </select>

            <select
              v-model="tagFilter"
              class="bg-[#111317] border border-slate-800 text-slate-300 text-[11px] rounded-lg px-2 py-1 focus:outline-none"
            >
              <option value="ALL">All Tags</option>
            </select>
          </div>
        </div>

        <!-- Device List -->
        <div class="flex-1 overflow-y-auto p-2 space-y-1.5 divide-y divide-slate-800/40 font-sans">
          <div
            v-for="dev in filteredDiscoveredDevices"
            :key="dev.id"
            class="pt-1.5 flex items-center justify-between group hover:bg-slate-100 dark:hover:bg-slate-800/30 p-1.5 rounded-lg transition"
          >
            <div class="flex items-center gap-2 overflow-hidden flex-1 mr-2">
              <input
                type="checkbox"
                :value="dev.id"
                v-model="selectedDiscoveredIds"
                class="rounded bg-slate-100 dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-blue-600 focus:ring-0 w-3.5 h-3.5"
              />
              <div class="w-6 h-6 rounded-md bg-slate-100 dark:bg-slate-800 flex items-center justify-center shrink-0 border border-slate-200 dark:border-slate-700/60">
                <component
                  :is="getNodeIcon(dev.deviceType)"
                  class="w-3.5 h-3.5"
                  :class="[
                    dev.status === 'online' ? 'text-emerald-600 dark:text-emerald-400' : dev.status === 'offline' ? 'text-rose-600 dark:text-red-400' : 'text-slate-500 dark:text-slate-400'
                  ]"
                />
              </div>
              <div class="overflow-hidden">
                <p class="text-xs font-medium text-slate-900 dark:text-slate-200 truncate">{{ dev.name }}</p>
                <div class="flex items-center gap-1.5 text-[10px] text-slate-500 font-mono">
                  <span>{{ dev.ipAddress }}</span>
                  <span class="px-1 py-0.2 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 uppercase text-[8px] font-bold">
                    {{ dev.deviceType || 'UNKNOWN' }}
                  </span>
                </div>
              </div>
            </div>

            <!-- Action Status -->
            <div class="shrink-0 flex items-center gap-1">
              <button
                v-if="isDeviceOnCanvas(dev.id)"
                @click="handleRemoveDeviceFromCanvas(dev.id)"
                class="flex items-center gap-1 px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 hover:bg-rose-500/20 text-slate-600 dark:text-slate-400 hover:text-rose-400 text-[10px] font-mono transition"
                title="Click to remove from canvas sheet (keeps in sidebar inventory)"
              >
                <span>Added</span>
                <X class="w-2.5 h-2.5" />
              </button>
              <button
                v-else
                @click="handleAddDeviceToCanvas(dev)"
                class="flex items-center gap-1 px-2 py-0.5 rounded border border-cyan-500/60 text-cyan-400 hover:bg-cyan-500/10 text-[10px] font-bold transition"
                title="Add to canvas sheet"
              >
                <Plus class="w-2.5 h-2.5" />
                <span>Add</span>
              </button>
              <button
                @click="handleOpenEditDevice(dev)"
                class="p-1 hover:text-white text-slate-500 transition"
                title="Edit device info"
              >
                <Edit2 class="w-3 h-3" />
              </button>
              <button
                @click="promptDeleteDevice(dev.id)"
                class="p-1 hover:text-rose-400 text-slate-500 transition cursor-pointer"
                title="Permanently delete device from inventory"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>
          </div>
        </div>
      </aside>

      <!-- Sidebar Toggle Collapse Button -->
      <button
        @click="isSidebarCollapsed = !isSidebarCollapsed"
        class="absolute left-0 top-1/2 -translate-y-1/2 z-30 p-1 rounded-r bg-slate-800 text-slate-400 hover:text-white border-y border-r border-slate-700 shadow-md transition"
        :style="{ left: isSidebarCollapsed ? '0px' : '288px' }"
      >
        <ChevronRight v-if="isSidebarCollapsed" class="w-3.5 h-3.5" />
        <ChevronLeft v-else class="w-3.5 h-3.5" />
      </button>

      <!-- 2. CANVAS VIEW (SVG GRAPH) -->
      <main
        class="flex-1 relative overflow-hidden bg-slate-50 dark:bg-[#111317] cursor-grab active:cursor-grabbing"
        @mousedown="handleCanvasMouseDown"
        @mousemove="handleCanvasMouseMove"
        @mouseup="handleCanvasMouseUp"
        @wheel="handleCanvasWheel"
      >
        <!-- Canvas Dotted Background -->
        <div
          id="topology-canvas-bg"
          class="absolute inset-0 pointer-events-none"
          style="background-image: radial-gradient(circle, #2d3748 1px, transparent 1px); background-size: 24px 24px;"
        ></div>

        <!-- Center Status Pill -->
        <div class="absolute top-4 left-1/2 -translate-x-1/2 z-10 pointer-events-none">
          <div class="px-3 py-1 rounded-full bg-white/95 dark:bg-[#171a21]/90 border border-slate-200 dark:border-slate-800 text-[11px] font-mono text-slate-700 dark:text-slate-400 flex items-center gap-2 shadow-sm backdrop-blur-sm">
            <span class="w-2 h-2 rounded-full bg-emerald-500"></span>
            <span>{{ activeNodes.length }} nodes, {{ edges.length }} edges</span>
          </div>
        </div>

        <!-- SVG Rendering Plane with Transform Matrix -->
        <svg
          class="w-full h-full absolute inset-0 overflow-visible"
          :style="{
            transform: `translate(${canvasTransform.x}px, ${canvasTransform.y}px) scale(${canvasTransform.scale})`,
            transformOrigin: '0 0'
          }"
        >
          <!-- 1. Render Links / Edges -->
          <g class="edges-layer">
            <g v-for="edge in renderedEdges" :key="edge.renderKey" class="edge-group">
              <!-- Invisible Hit Area Path for easy click/right-click -->
              <path
                v-if="edge.valid"
                :d="edge.path"
                fill="none"
                stroke="transparent"
                stroke-width="16"
                class="cursor-pointer"
                @click.stop="handleEdgeClick(edge)"
                @contextmenu.stop="handleEdgeContextMenu(edge, $event)"
              />

              <!-- Visible Edge Path -->
              <path
                v-if="edge.valid"
                :d="edge.path"
                :class="[
                  'transition-all duration-150 pointer-events-none',
                  selectedEdge?.id === edge.id
                    ? 'stroke-blue-500 dark:stroke-blue-400 stroke-[3.5]'
                    : (edge.edgeType || edge.label) === 'VPN'
                    ? 'stroke-emerald-500 stroke-2'
                    : (edge.edgeType || edge.label) === 'Wireless'
                    ? 'stroke-cyan-500 stroke-[1.5]'
                    : 'stroke-slate-400 dark:stroke-slate-600 stroke-2'
                ]"
                :stroke-dasharray="(edge.edgeType || edge.label) === 'VPN' ? '6,4' : (edge.edgeType || edge.label) === 'Wireless' ? '4,4' : 'none'"
                fill="none"
              />

              <!-- Edge Label Badge -->
              <g
                v-if="edge.valid && (edge.label || edge.edgeType)"
                :transform="`translate(${edge.midX}, ${edge.midY})`"
                class="cursor-pointer select-none"
                @click.stop="handleEdgeClick(edge)"
                @contextmenu.stop="handleEdgeContextMenu(edge, $event)"
              >
                <rect
                  x="-30"
                  y="-10"
                  width="60"
                  height="20"
                  rx="10"
                  :class="[
                    'transition-colors shadow-sm',
                    selectedEdge?.id === edge.id
                      ? 'fill-blue-600 stroke-blue-400 stroke-1'
                      : 'fill-white dark:fill-[#171a21] stroke-slate-300 dark:stroke-slate-700/80 stroke-1 hover:stroke-blue-400'
                  ]"
                />
                <text
                  x="0"
                  y="3.5"
                  text-anchor="middle"
                  :class="[
                    'text-[9px] font-mono font-medium pointer-events-none',
                    selectedEdge?.id === edge.id ? 'fill-white font-bold' : 'fill-slate-700 dark:fill-slate-300'
                  ]"
                >
                  {{ edge.label || edge.edgeType }}
                </text>
              </g>
            </g>
          </g>

          <!-- 2. Render Nodes -->
          <g class="nodes-layer">
            <g
              v-for="dev in activeNodes"
              :key="dev.id"
              :transform="`translate(${dev.x || 0}, ${dev.y || 0})`"
              class="node-group cursor-pointer"
              @mousedown="handleNodeMouseDown(dev, $event)"
              @contextmenu="handleNodeContextMenu(dev, $event)"
              @click="selectedNode = dev"
            >
              <!-- Solid Clean Node Circle (No Glow) -->
              <circle
                r="30"
                :class="[
                  'transition-all duration-200',
                  dev.status === 'online'
                    ? 'fill-emerald-50 dark:fill-[#121f18] stroke-emerald-500 dark:stroke-emerald-600 stroke-2'
                    : dev.status === 'offline'
                    ? 'fill-rose-50 dark:fill-[#261517] stroke-rose-500 dark:stroke-rose-600 stroke-2'
                    : 'fill-slate-100 dark:fill-[#171a21] stroke-slate-300 dark:stroke-slate-700 stroke-2'
                ]"
              />

              <!-- Center Icon -->
              <component
                :is="getNodeIcon(dev.deviceType)"
                :x="-12"
                :y="-12"
                width="24"
                height="24"
                :class="[
                  'pointer-events-none',
                  dev.status === 'online' ? 'text-emerald-600 dark:text-emerald-300' : dev.status === 'offline' ? 'text-rose-600 dark:text-red-400' : 'text-slate-500 dark:text-slate-400'
                ]"
              />

              <!-- Text Labels Below Node -->
              <g v-if="showLabels" transform="translate(0, 44)" class="pointer-events-none">
                <!-- Device Name -->
                <text
                  x="0"
                  y="0"
                  text-anchor="middle"
                  class="fill-slate-900 dark:fill-white font-sans text-xs font-bold tracking-wide"
                >
                  {{ dev.name }}
                </text>
                <!-- IP Address -->
                <text
                  x="0"
                  y="14"
                  text-anchor="middle"
                  class="fill-slate-500 dark:fill-slate-400 font-mono text-[10px]"
                >
                  {{ dev.ipAddress }}
                </text>
              </g>
            </g>
          </g>
        </svg>

        <!-- Bottom Status Legend -->
        <div class="absolute bottom-5 left-1/2 -translate-x-1/2 z-10">
          <div class="px-4 py-1.5 rounded-full bg-white/95 dark:bg-[#171a21]/90 border border-slate-200 dark:border-slate-800 flex items-center gap-4 text-xs font-mono shadow-sm backdrop-blur-sm">
            <div class="flex items-center gap-1.5">
              <span class="w-2.5 h-2.5 rounded-full bg-emerald-500"></span>
              <span class="text-slate-700 dark:text-slate-300">Online</span>
            </div>
            <div class="flex items-center gap-1.5">
              <span class="w-2.5 h-2.5 rounded-full bg-rose-500"></span>
              <span class="text-slate-700 dark:text-slate-300">Offline</span>
            </div>
            <div class="flex items-center gap-1.5">
              <span class="w-2.5 h-2.5 rounded-full bg-slate-400 dark:bg-slate-500"></span>
              <span class="text-slate-500 dark:text-slate-400">Unknown</span>
            </div>
          </div>
        </div>
      </main>

      <!-- 3. RIGHT DRAWER: SELECTED DEVICE DETAILS -->
      <aside
        v-if="selectedNode"
        class="w-80 bg-white dark:bg-[#171a21] border-l border-slate-200 dark:border-slate-800 flex flex-col z-20 shrink-0 shadow-xl transition-all"
      >
        <!-- Drawer Header -->
        <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <div class="flex items-center gap-2.5 overflow-hidden">
            <div class="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-500/10 border border-blue-200 dark:border-blue-500/30 flex items-center justify-center text-blue-600 dark:text-blue-400 shrink-0">
              <component :is="getNodeIcon(selectedNode.deviceType)" class="w-4 h-4" />
            </div>
            <div class="overflow-hidden">
              <h3 class="text-sm font-bold text-slate-900 dark:text-white tracking-wide truncate">{{ selectedNode.name }}</h3>
              <p class="text-xs font-mono text-slate-500 dark:text-slate-400">{{ selectedNode.ipAddress }}</p>
            </div>
          </div>
          <button @click="selectedNode = null" class="text-slate-400 hover:text-slate-900 dark:hover:text-white p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 transition">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Drawer Content -->
        <div class="flex-1 overflow-y-auto p-4 space-y-4 text-xs">
          <!-- Attributes Table -->
          <div class="space-y-2 font-mono">
            <div class="flex justify-between py-1 border-b border-slate-800/60">
              <span class="text-slate-500">Role / Type</span>
              <span class="font-bold text-slate-200 uppercase">{{ selectedNode.deviceType || 'UNKNOWN' }}</span>
            </div>
            <div class="flex justify-between py-1 border-b border-slate-800/60">
              <span class="text-slate-500">Status</span>
              <span :class="selectedNode.status === 'online' ? 'text-emerald-400 font-bold' : selectedNode.status === 'offline' ? 'text-red-400 font-bold' : 'text-slate-400'">
                {{ (selectedNode.status || 'unknown').toUpperCase() }}
              </span>
            </div>
            <div class="flex justify-between py-1 border-b border-slate-800/60">
              <span class="text-slate-500">Sources</span>
              <span class="px-2 py-0.2 rounded bg-blue-600/30 text-blue-400 text-[10px] font-bold">
                {{ (selectedNode.sources || ['MANUAL'])[0] }}
              </span>
            </div>
          </div>

          <!-- Connections Section -->
          <div class="space-y-2">
            <h4 class="text-[10px] font-bold uppercase tracking-wider text-slate-500">Connections</h4>
            <div v-if="selectedNodeConnections.length > 0" class="space-y-1.5">
              <div
                v-for="(conn, cIdx) in selectedNodeConnections"
                :key="cIdx"
                class="p-2 rounded-lg bg-slate-50 dark:bg-[#111317] border border-slate-200 dark:border-slate-800 flex items-center justify-between group hover:border-slate-300 dark:hover:border-slate-700 transition cursor-pointer"
                @click="handleEdgeClick(conn.edge)"
                title="Click to inspect this connection"
              >
                <div class="overflow-hidden mr-2">
                  <p class="text-xs font-medium text-slate-800 dark:text-slate-200 truncate">• {{ conn.peerName }}</p>
                  <div class="flex items-center gap-1 text-[10px] text-slate-500 font-mono">
                    <span>{{ conn.edgeType }}</span>
                    <span v-if="conn.edge.label && conn.edge.label !== conn.edgeType" class="text-slate-400 truncate">({{ conn.edge.label }})</span>
                  </div>
                </div>
                <div class="flex items-center gap-1 shrink-0" @click.stop>
                  <button
                    @click="handleOpenEditEdge(conn.edge)"
                    class="p-1 hover:text-slate-900 dark:hover:text-white text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 rounded transition cursor-pointer"
                    title="Edit Connection"
                  >
                    <Edit2 class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="promptDeleteEdge(conn.edge)"
                    class="p-1 hover:text-rose-500 text-slate-400 hover:bg-rose-500/10 rounded transition cursor-pointer"
                    title="Delete Connection"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            </div>
            <p v-else class="text-xs text-slate-500 italic">No direct connections attached.</p>
          </div>
        </div>

        <!-- Drawer Bottom Actions -->
        <div class="p-3 border-t border-slate-800 grid grid-cols-5 gap-1.5 bg-[#14161b]">
          <button
            @click="handleOpenEditDevice(selectedNode)"
            class="py-2 rounded bg-[#20242e] hover:bg-slate-700 text-slate-200 text-xs font-semibold tracking-wide transition flex items-center justify-center gap-1"
          >
            <span>EDIT</span>
          </button>

          <button
            @click="handleTriggerPing(selectedNode)"
            class="py-2 rounded bg-emerald-600/20 hover:bg-emerald-600/30 border border-emerald-500/40 text-emerald-400 text-xs font-bold tracking-wide transition flex items-center justify-center gap-1"
          >
            <span>PING</span>
          </button>

          <button
            @click="handleConnectSSHFromTopology(selectedNode)"
            class="py-2 rounded bg-blue-600/20 hover:bg-blue-600/30 border border-blue-500/40 text-blue-400 text-xs font-bold tracking-wide transition flex items-center justify-center gap-1 font-mono"
            title="Open Remote Server SSH Terminal"
          >
            <span>>_ SSH</span>
          </button>

          <button
            @click="router.push('/snmp')"
            class="py-2 rounded bg-[#20242e] hover:bg-slate-700 text-slate-200 text-xs font-semibold tracking-wide transition flex items-center justify-center gap-1 font-mono"
          >
            <span>SNMP</span>
          </button>

          <button
            @click="handleRemoveDeviceFromCanvas(selectedNode.id)"
            class="py-2 rounded bg-amber-600/20 hover:bg-amber-600/30 border border-amber-500/40 text-amber-300 text-xs font-bold tracking-wide transition flex items-center justify-center gap-1"
            title="Remove device from canvas sheet (device remains in sidebar inventory)"
          >
            <span>REMOVE</span>
          </button>
        </div>
      </aside>

      <!-- Selected Link / Connection Details Drawer -->
      <aside
        v-else-if="selectedEdge"
        class="w-80 bg-white dark:bg-[#171a21] border-l border-slate-200 dark:border-slate-800 flex flex-col z-20 shrink-0 shadow-xl transition-all"
      >
        <!-- Drawer Header -->
        <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <div class="flex items-center gap-2.5 overflow-hidden">
            <div class="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-500/10 border border-blue-200 dark:border-blue-500/30 flex items-center justify-center text-blue-600 dark:text-blue-400 shrink-0">
              <Cable class="w-4 h-4" />
            </div>
            <div class="overflow-hidden">
              <h3 class="text-sm font-bold text-slate-900 dark:text-white tracking-wide truncate">Connection Details</h3>
              <p class="text-xs font-mono text-slate-500 dark:text-slate-400">{{ selectedEdge.edgeType || 'Ethernet' }} Link</p>
            </div>
          </div>
          <button @click="selectedEdge = null" class="text-slate-400 hover:text-slate-900 dark:hover:text-white p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Drawer Content -->
        <div class="flex-1 overflow-y-auto p-4 space-y-4 text-xs">
          <!-- Endpoints Card -->
          <div class="space-y-2">
            <h4 class="text-[10px] font-bold uppercase tracking-wider text-slate-500">Connected Devices</h4>
            
            <!-- Source Node Box -->
            <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#111317] border border-slate-200 dark:border-slate-800 flex items-center justify-between">
              <div class="flex items-center gap-2 overflow-hidden">
                <component :is="getNodeIcon(selectedEdge.sourceNode?.deviceType)" class="w-4 h-4 text-slate-500 shrink-0" />
                <div class="overflow-hidden">
                  <p class="text-xs font-medium text-slate-900 dark:text-slate-200 truncate">{{ selectedEdge.sourceName || selectedEdge.sourceId }}</p>
                  <p class="text-[10px] text-slate-500 font-mono">{{ selectedEdge.sourceIP || '-' }}</p>
                </div>
              </div>
              <span class="text-[9px] px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 text-slate-600 dark:text-slate-400 uppercase font-mono font-semibold">Source</span>
            </div>

            <!-- Direction Indicator -->
            <div class="flex items-center justify-center text-slate-400">
              <ArrowUpDown class="w-3.5 h-3.5" />
            </div>

            <!-- Target Node Box -->
            <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#111317] border border-slate-200 dark:border-slate-800 flex items-center justify-between">
              <div class="flex items-center gap-2 overflow-hidden">
                <component :is="getNodeIcon(selectedEdge.targetNode?.deviceType)" class="w-4 h-4 text-slate-500 shrink-0" />
                <div class="overflow-hidden">
                  <p class="text-xs font-medium text-slate-900 dark:text-slate-200 truncate">{{ selectedEdge.targetName || selectedEdge.targetId }}</p>
                  <p class="text-[10px] text-slate-500 font-mono">{{ selectedEdge.targetIP || '-' }}</p>
                </div>
              </div>
              <span class="text-[9px] px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 text-slate-600 dark:text-slate-400 uppercase font-mono font-semibold">Target</span>
            </div>
          </div>

          <!-- Attributes Table -->
          <div class="space-y-2 font-mono">
            <div class="flex justify-between py-1 border-b border-slate-200 dark:border-slate-800/60">
              <span class="text-slate-500">Link Type</span>
              <span class="font-bold text-slate-900 dark:text-slate-200 uppercase">{{ selectedEdge.edgeType || 'Ethernet' }}</span>
            </div>
            <div class="flex justify-between py-1 border-b border-slate-200 dark:border-slate-800/60">
              <span class="text-slate-500">Label</span>
              <span class="text-slate-700 dark:text-slate-300">{{ selectedEdge.label || '-' }}</span>
            </div>
            <div v-if="selectedEdge.id" class="flex justify-between py-1 border-b border-slate-200 dark:border-slate-800/60">
              <span class="text-slate-500">Link ID</span>
              <span class="text-slate-500 font-mono">#{{ selectedEdge.id }}</span>
            </div>
          </div>
        </div>

        <!-- Drawer Bottom Actions -->
        <div class="p-3 border-t border-slate-200 dark:border-slate-800 grid grid-cols-2 gap-2 bg-slate-50 dark:bg-[#14161b]">
          <button
            @click="handleOpenEditEdge(selectedEdge)"
            class="py-2 rounded bg-slate-200 hover:bg-slate-300 dark:bg-[#20242e] dark:hover:bg-slate-700 text-slate-800 dark:text-slate-200 text-xs font-semibold tracking-wide transition flex items-center justify-center gap-1.5 cursor-pointer"
          >
            <Edit2 class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
            <span>EDIT LINK</span>
          </button>

          <button
            @click="promptDeleteEdge(selectedEdge)"
            class="py-2 rounded bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 text-rose-600 dark:text-rose-400 text-xs font-bold tracking-wide transition flex items-center justify-center gap-1.5 cursor-pointer"
          >
            <Trash2 class="w-3.5 h-3.5 text-rose-600 dark:text-rose-400" />
            <span>DELETE</span>
          </button>
        </div>
      </aside>

    </div>

    <!-- ================================================================= -->
    <!-- RIGHT CLICK CONTEXT MENU -->
    <!-- ================================================================= -->
    <!-- Node Right Click Context Menu -->
    <div
      v-if="contextMenu.visible && contextMenu.node"
      class="fixed z-50 bg-white dark:bg-[#171a21] border border-slate-200 dark:border-slate-700/80 rounded-xl py-1.5 w-48 shadow-2xl backdrop-blur-md text-xs font-sans"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
    >
      <button
        @click="selectedNode = contextMenu.node; selectedEdge = null; contextMenu.visible = false"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
      >
        <Eye class="w-3.5 h-3.5 text-slate-400" />
        <span>View Details</span>
      </button>

      <button
        @click="handleConnectSSHFromTopology(contextMenu.node); contextMenu.visible = false"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-blue-600 dark:text-blue-400 hover:text-blue-500 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
      >
        <Terminal class="w-3.5 h-3.5 text-blue-500" />
        <span>Open SSH Terminal</span>
      </button>

      <button
        @click="handleOpenEditDevice(contextMenu.node); contextMenu.visible = false"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
      >
        <Edit2 class="w-3.5 h-3.5 text-slate-400" />
        <span>Edit Device</span>
      </button>

      <button
        @click="handleTriggerPing(contextMenu.node); contextMenu.visible = false"
        class="w-full px-3 py-1.5 text-left flex items-center justify-between text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
      >
        <span class="flex items-center gap-2">
          <Activity class="w-3.5 h-3.5 text-emerald-500" />
          <span>Ping Device</span>
        </span>
        <span class="text-[10px] text-slate-500 font-mono">Ctrl+P</span>
      </button>

      <button
        @click="linkForm.sourceId = contextMenu.node.id; handleOpenAddLink(); contextMenu.visible = false"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition border-t border-slate-100 dark:border-slate-800 mt-1 pt-1.5 cursor-pointer"
      >
        <Cable class="w-3.5 h-3.5 text-cyan-500" />
        <span>Connect to...</span>
      </button>

      <button
        @click="handleRemoveDeviceFromCanvas(contextMenu.node.id); contextMenu.visible = false"
        class="w-full px-3 py-1.5 text-left flex items-center justify-between text-amber-600 dark:text-amber-400 hover:bg-amber-500/10 transition border-t border-slate-100 dark:border-slate-800 mt-1 pt-1.5 cursor-pointer"
        title="Remove node from active canvas sheet (device stays in sidebar inventory)"
      >
        <span class="flex items-center gap-2">
          <XCircle class="w-3.5 h-3.5" />
          <span>Remove from Canvas</span>
        </span>
        <span class="text-[10px] text-amber-600/80 dark:text-amber-400/80 font-mono">Unplace</span>
      </button>

      <button
        @click="promptDeleteDevice(contextMenu.node.id); contextMenu.visible = false"
        class="w-full px-3 py-1.5 text-left flex items-center justify-between text-rose-600 dark:text-rose-400 hover:bg-rose-500/10 transition cursor-pointer"
        title="Permanently delete device from inventory"
      >
        <span class="flex items-center gap-2">
          <Trash2 class="w-3.5 h-3.5" />
          <span>Delete Permanently</span>
        </span>
        <span class="text-[10px] text-rose-500/80 font-mono">Del</span>
      </button>
    </div>

    <!-- Edge Right Click Context Menu -->
    <div
      v-if="edgeContextMenu.visible && edgeContextMenu.edge"
      class="fixed z-50 bg-white dark:bg-[#171a21] border border-slate-200 dark:border-slate-700/80 rounded-xl py-1.5 w-52 shadow-2xl backdrop-blur-md text-xs font-sans"
      :style="{ left: `${edgeContextMenu.x}px`, top: `${edgeContextMenu.y}px` }"
    >
      <div class="px-3 py-1.5 border-b border-slate-100 dark:border-slate-800 text-[11px] font-mono text-slate-500 dark:text-slate-400 truncate">
        <span>{{ edgeContextMenu.edge.sourceName || 'Node' }} ↔ {{ edgeContextMenu.edge.targetName || 'Node' }}</span>
      </div>

      <button
        @click="handleEdgeClick(edgeContextMenu.edge)"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
      >
        <Eye class="w-3.5 h-3.5 text-slate-400" />
        <span>View Details</span>
      </button>

      <button
        @click="handleOpenEditEdge(edgeContextMenu.edge)"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
      >
        <Edit2 class="w-3.5 h-3.5 text-slate-400" />
        <span>Edit Link</span>
      </button>

      <button
        @click="promptDeleteEdge(edgeContextMenu.edge)"
        class="w-full px-3 py-1.5 text-left flex items-center justify-between text-rose-600 dark:text-rose-400 hover:bg-rose-500/10 transition border-t border-slate-100 dark:border-slate-800 mt-1 pt-1.5 cursor-pointer"
      >
        <span class="flex items-center gap-2">
          <Trash2 class="w-3.5 h-3.5 text-rose-600 dark:text-rose-400" />
          <span>Delete Link</span>
        </span>
        <span class="text-[10px] text-rose-500/80 font-mono">Del</span>
      </button>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: INTERACTIVE PING EXECUTION -->
    <!-- ================================================================= -->
    <div
      v-if="isPingModalOpen && pingTarget"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#171a21] border border-slate-800 rounded-2xl w-full max-w-xl p-5 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Activity class="w-4 h-4 text-emerald-400" />
            <h3 class="text-sm font-bold text-white">Ping {{ pingTarget.name }} ({{ pingTarget.ip }})</h3>
          </div>
          <button @click="isPingModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Terminal Output Box -->
        <div class="bg-[#0b0d11] border border-slate-800/90 rounded-xl p-4 font-mono text-xs text-emerald-400 leading-relaxed whitespace-pre-wrap min-h-[200px] overflow-y-auto">
          {{ pingOutput }}
        </div>

        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            @click="handleTriggerPing({ name: pingTarget.name, ipAddress: pingTarget.ip } as Device)"
            :disabled="pingLoading"
            class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold rounded-lg text-xs transition"
          >
            {{ pingLoading ? 'Pinging...' : 'Ping Again' }}
          </button>
          <button
            @click="isPingModalOpen = false"
            class="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-xs"
          >
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: SCAN DISCOVERY -->
    <!-- ================================================================= -->
    <div
      v-if="isScanModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#171a21] border border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Scan class="w-4 h-4 text-emerald-400" />
            <h3 class="text-sm font-bold text-white">Network Subnet Scanner</h3>
          </div>
          <button @click="isScanModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Target Subnet CIDR</label>
            <input
              v-model="scanCidr"
              class="w-full bg-[#111317] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
              placeholder="192.168.102.0/24 or 10.20.3.0/24"
            />
          </div>

          <div class="p-3 rounded-lg bg-slate-800/40 border border-slate-800 text-slate-400 space-y-1">
            <p class="font-semibold text-slate-300">Fast Parallel Discovery</p>
            <p class="text-[11px]">Automatically pings subnet range to register newly discovered network hosts.</p>
          </div>

          <div class="flex items-center justify-between pt-3 border-t border-slate-800">
            <button
              @click="handleSyncPrometheus"
              type="button"
              class="px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs"
            >
              Sync Prometheus Targets
            </button>

            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="isScanModalOpen = false"
                class="px-3 py-2 bg-slate-800 text-slate-300 rounded-lg text-xs"
              >
                Cancel
              </button>
              <button
                @click="handleRunSubnetScan"
                :disabled="scanLoading"
                class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold rounded-lg text-xs"
              >
                {{ scanLoading ? 'Scanning...' : 'Start Scan' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: ADD / EDIT DEVICE -->
    <!-- ================================================================= -->
    <div
      v-if="isDeviceModalOpen || isEditDeviceModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#171a21] border border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Server class="w-4 h-4 text-emerald-400" />
            <h3 class="text-sm font-bold text-white">{{ isEditDeviceModal ? 'Edit Device' : 'Add Topology Device' }}</h3>
          </div>
          <button @click="isDeviceModalOpen = false; isEditDeviceModal = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleSaveDevice" class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Device Name</label>
            <input
              v-model="deviceForm.name"
              required
              class="w-full bg-[#111317] border border-slate-700 rounded-lg px-3 py-2 text-white"
              placeholder="e.g. Server - Horus, AP-INDOOR-98"
            />
          </div>

          <div>
            <label class="block text-slate-400 mb-1">IP Address</label>
            <input
              v-model="deviceForm.ipAddress"
              required
              class="w-full bg-[#111317] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
              placeholder="10.20.3.4"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1">Device Type</label>
              <select
                v-model="deviceForm.deviceType"
                class="w-full bg-[#111317] border border-slate-700 rounded-lg px-3 py-2 text-white"
              >
                <option value="server">Server (VPS / Bare Metal)</option>
                <option value="router">Router / Gateway</option>
                <option value="switch">Switch / Core Switch</option>
                <option value="ap">Access Point / WiFi</option>
                <option value="database">Database (SQL / OpenSearch)</option>
                <option value="docker">Docker / Container / K8s</option>
                <option value="cloud">Cloud / SaaS (AWS / Cloudflare)</option>
                <option value="firewall">Firewall / Security (pfSense / WAF)</option>
                <option value="nas">NAS / Storage / SAN</option>
                <option value="workstation">Workstation / PC / Laptop</option>
                <option value="vm">Virtual Machine (Proxmox / ESXi)</option>
                <option value="internet">Internet / WAN Provider</option>
                <option value="unknown">Other / Unknown</option>
              </select>
            </div>
            <div>
              <label class="block text-slate-400 mb-1">Initial Status</label>
              <select
                v-model="deviceForm.status"
                class="w-full bg-[#111317] border border-slate-700 rounded-lg px-3 py-2 text-white"
              >
                <option value="online">Online</option>
                <option value="offline">Offline</option>
                <option value="unknown">Unknown</option>
              </select>
            </div>
          </div>

          <div class="flex items-center justify-between pt-3 border-t border-slate-800">
            <button
              v-if="isEditDeviceModal && deviceForm.id"
              type="button"
              @click="promptDeleteDevice(deviceForm.id); isEditDeviceModal = false"
              class="px-3 py-2 bg-rose-600/20 hover:bg-rose-600/30 text-rose-400 border border-rose-500/30 font-medium rounded-lg text-xs transition cursor-pointer"
            >
              Delete Permanently
            </button>
            <div v-else></div>

            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="isDeviceModalOpen = false; isEditDeviceModal = false"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs"
              >
                Cancel
              </button>
              <button
                type="submit"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg text-xs"
              >
                Save Device
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: ADD / EDIT LINK / CONNECTION -->
    <!-- ================================================================= -->
    <div
      v-if="isLinkModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-white dark:bg-[#171a21] border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Cable class="w-4 h-4 text-blue-600 dark:text-blue-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">
              {{ isEditLink ? 'Edit Network Connection' : 'Create Network Connection' }}
            </h3>
          </div>
          <button @click="isLinkModalOpen = false" class="text-slate-400 hover:text-slate-900 dark:hover:text-white p-1 rounded transition cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleSaveLink" class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-600 dark:text-slate-400 mb-1">Source Device</label>
            <select
              v-model="linkForm.sourceId"
              required
              class="w-full bg-slate-50 dark:bg-[#111317] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-medium focus:outline-none focus:border-brand-500"
            >
              <option value="" disabled>Select Source...</option>
              <option v-for="n in activeNodes" :key="n.id" :value="n.id">
                {{ n.name }} ({{ n.ipAddress }})
              </option>
            </select>
          </div>

          <div>
            <label class="block text-slate-600 dark:text-slate-400 mb-1">Target Device</label>
            <select
              v-model="linkForm.targetId"
              required
              class="w-full bg-slate-50 dark:bg-[#111317] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-medium focus:outline-none focus:border-brand-500"
            >
              <option value="" disabled>Select Target...</option>
              <option v-for="n in activeNodes" :key="n.id" :value="n.id">
                {{ n.name }} ({{ n.ipAddress }})
              </option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-600 dark:text-slate-400 mb-1">Link Type</label>
              <select
                v-model="linkForm.edgeType"
                @change="onLinkTypeChange"
                class="w-full bg-slate-50 dark:bg-[#111317] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-brand-500"
              >
                <option value="Ethernet">Ethernet</option>
                <option value="VPN">VPN</option>
                <option value="Wireless">Wireless</option>
                <option value="Fiber">Fiber</option>
              </select>
            </div>
            <div>
              <label class="block text-slate-600 dark:text-slate-400 mb-1">Label</label>
              <input
                v-model="linkForm.label"
                class="w-full bg-slate-50 dark:bg-[#111317] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-brand-500"
                placeholder="Ethernet / VPN / Fiber"
              />
            </div>
          </div>

          <div class="flex items-center justify-end gap-3 pt-3 border-t border-slate-200 dark:border-slate-800">
            <button
              type="button"
              @click="isLinkModalOpen = false"
              class="px-4 py-2 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-lg text-xs font-medium cursor-pointer transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg text-xs transition cursor-pointer shadow-sm"
            >
              {{ isEditLink ? 'Update Link' : 'Save Link' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: ADD SHEET -->
    <!-- ================================================================= -->
    <div
      v-if="isSheetModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#171a21] border border-slate-800 rounded-2xl w-full max-w-sm p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Layers class="w-4 h-4 text-emerald-400" />
            <h3 class="text-sm font-bold text-white">Create Topology Sheet</h3>
          </div>
          <button @click="isSheetModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Sheet Name</label>
            <input
              v-model="newSheetName"
              required
              class="w-full bg-[#111317] border border-slate-700 rounded-lg px-3 py-2 text-white"
              placeholder="e.g. Servers, DMZ, Branch Office"
            />
          </div>

          <div class="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              type="button"
              @click="isSheetModalOpen = false"
              class="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-xs"
            >
              Cancel
            </button>
            <button
              @click="handleCreateSheet"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg text-xs"
            >
              Create Sheet
            </button>
          </div>
        </div>
      </div>
    </div>
    <!-- ================================================================= -->
    <!-- MODAL: SYNC REMOTE SERVERS TO TOPOLOGY -->
    <!-- ================================================================= -->
    <div
      v-if="isRemoteSyncModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#171a21] border border-slate-800 rounded-2xl w-full max-w-2xl p-6 space-y-4 shadow-2xl font-sans">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-blue-600/20 border border-blue-500/40 flex items-center justify-center text-blue-400">
              <Terminal class="w-4 h-4" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-white tracking-wide">Sync Remote Server to Topology</h3>
              <p class="text-xs text-slate-400 mt-0.5">
                Import registered SSH / SFTP hosts into active sheet:
                <span class="text-blue-400 font-semibold font-mono">{{ sheets.find(s => s.id === activeSheetId)?.name || 'Default Sheet' }}</span>
              </p>
            </div>
          </div>
          <button @click="isRemoteSyncModalOpen = false" class="text-slate-400 hover:text-white transition">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Remote Hosts Selection List / Table -->
        <div class="space-y-2 max-h-72 overflow-y-auto pr-1">
          <div v-if="syncRemoteLoading" class="py-8 text-center text-slate-400 flex items-center justify-center gap-2">
            <RefreshCw class="w-4 h-4 animate-spin text-blue-400" />
            <span class="text-xs">Loading remote server hosts...</span>
          </div>

          <div v-else-if="remoteHosts.length === 0" class="py-8 text-center text-slate-500 text-xs">
            <Server class="w-6 h-6 mx-auto mb-2 text-slate-600" />
            <p>No remote hosts registered yet in Remote Server.</p>
            <router-link to="/remote-server" target="_blank" class="text-blue-400 hover:underline mt-1 inline-block">
              Register a Remote Host &rarr;
            </router-link>
          </div>

          <div v-else class="space-y-1.5">
            <!-- Header bar with Select All toggle -->
            <div class="flex items-center justify-between px-3 py-1.5 bg-[#111317] border border-slate-800/80 rounded-lg text-xs text-slate-400">
              <label class="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  :checked="selectedRemoteHostIds.length === remoteHosts.length && remoteHosts.length > 0"
                  @change="toggleSelectAllRemoteHosts"
                  class="rounded border-slate-700 bg-slate-900 text-blue-600 focus:ring-0"
                />
                <span class="font-medium text-slate-300">Select All ({{ selectedRemoteHostIds.length }}/{{ remoteHosts.length }})</span>
              </label>
              <span class="text-[11px] text-slate-500 font-mono">{{ remoteHosts.length }} Hosts Available</span>
            </div>

            <!-- Host Cards -->
            <div
              v-for="host in remoteHosts"
              :key="host.id"
              @click="selectedRemoteHostIds.includes(host.id)
                ? selectedRemoteHostIds = selectedRemoteHostIds.filter(id => id !== host.id)
                : selectedRemoteHostIds.push(host.id)"
              class="p-2.5 rounded-xl border transition cursor-pointer flex items-center justify-between gap-3 text-xs"
              :class="[
                selectedRemoteHostIds.includes(host.id)
                  ? 'bg-[#1b2234]/60 border-blue-500/60'
                  : 'bg-[#111317] border-slate-800/80 hover:border-slate-700'
              ]"
            >
              <div class="flex items-center gap-3">
                <input
                  type="checkbox"
                  :value="host.id"
                  v-model="selectedRemoteHostIds"
                  @click.stop
                  class="rounded border-slate-700 bg-slate-900 text-blue-600 focus:ring-0"
                />
                <div>
                  <div class="flex items-center gap-2">
                    <span class="font-bold text-white">{{ host.name }}</span>
                    <span v-if="host.groupName" class="px-1.5 py-0.2 rounded bg-slate-800 text-[10px] text-slate-400 border border-slate-700">
                      {{ host.groupName }}
                    </span>
                  </div>
                  <p class="text-[11px] text-slate-400 font-mono mt-0.5">
                    {{ host.username }}@{{ host.host }}:{{ host.port }}
                  </p>
                </div>
              </div>

              <div class="flex items-center gap-2">
                <span
                  v-if="isRemoteHostInActiveSheet(host)"
                  class="px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30"
                >
                  On Canvas
                </span>
                <span
                  v-else
                  class="px-2 py-0.5 rounded text-[10px] font-semibold bg-blue-500/10 text-blue-400 border border-blue-500/30"
                >
                  New
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer Actions -->
        <div class="flex items-center justify-between border-t border-slate-800 pt-3 text-xs">
          <button
            type="button"
            @click="isRemoteSyncModalOpen = false"
            class="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 hover:text-white transition"
          >
            Cancel
          </button>

          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="handleDirectSyncAllRemoteServers"
              :disabled="syncRemoteLoading || remoteHosts.length === 0"
              class="px-3.5 py-2 rounded-lg bg-[#20242e] border border-slate-700 text-slate-200 hover:text-white hover:border-slate-500 transition font-medium disabled:opacity-50"
            >
              Sync All ({{ remoteHosts.length }})
            </button>

            <button
              type="button"
              @click="handleSyncSelectedRemoteServers"
              :disabled="syncRemoteLoading || selectedRemoteHostIds.length === 0"
              class="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-bold transition flex items-center gap-1.5 disabled:opacity-50"
            >
              <RefreshCw v-if="syncRemoteLoading" class="w-3.5 h-3.5 animate-spin" />
              <span>Sync Selected ({{ selectedRemoteHostIds.length }})</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Toast Notification Banner -->
    <div
      v-if="syncToastMessage"
      class="fixed bottom-6 right-6 z-50 bg-[#171a21] border border-blue-500/80 text-white px-4 py-2.5 rounded-xl shadow-2xl flex items-center gap-2.5 text-xs"
    >
      <CheckCircle2 class="w-4 h-4 text-emerald-400" />
      <span>{{ syncToastMessage }}</span>
    </div>

    <!-- ================================================================= -->
    <!-- HCP STANDARD DELETE CONFIRMATION MODAL -->
    <!-- ================================================================= -->
    <div
      v-if="deleteModal.visible"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Circular Red Icon -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <!-- Title & Subtitle -->
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            Delete {{ deleteModal.type === 'edge' ? 'Connection' : deleteModal.type === 'device' ? 'Device' : 'Sheet' }}?
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            {{ deleteModal.subtitle || `Are you sure you want to remove ${deleteModal.name}? This action cannot be undone.` }}
          </p>
        </div>

        <!-- Action Buttons -->
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="deleteModal.visible = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer transition"
          >
            Cancel
          </button>
          <button
            @click="executeDelete"
            :disabled="deleteModal.deleting"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deleteModal.deleting ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>

  </div>
</template>
