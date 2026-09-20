<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import axios from 'axios';
import ThemeToggle from '../components/ThemeToggle.vue';
import {
  Boxes,
  Server,
  Layers,
  Play,
  Square,
  RotateCw,
  Pause,
  Trash2,
  Terminal,
  Activity,
  Plus,
  Search,
  RefreshCw,
  Copy,
  Check,
  ExternalLink,
  ArrowLeft,
  X,
  HardDrive,
  Cpu,
  DownloadCloud,
  CheckCircle2,
  AlertCircle,
  Clock,
  Radio,
  Sliders,
  ChevronDown,
  Network
} from 'lucide-vue-next';

interface DockerConnection {
  id: string;
  name: string;
  driver: 'socket' | 'ssh' | 'tcp';
  socketPath: string;
  tcpHost: string;
  tcpPort: number;
  tcpTls: boolean;
  sshHost: string;
  sshPort: number;
  sshUser: string;
  isDefault: boolean;
  isActive: boolean;
  status?: string;
}

interface DockerPort {
  ip?: string;
  privatePort: number;
  publicPort?: number;
  type: string;
}

interface DockerContainer {
  id: string;
  names: string[];
  name?: string;
  image: string;
  imageId: string;
  command: string;
  created: number;
  state: 'running' | 'exited' | 'paused' | 'restarting' | 'dead' | 'created';
  status: string;
  ports: DockerPort[];
  networks?: string[];
  ipAddress?: string;
  labels?: Record<string, string>;
}

interface DockerImage {
  id: string;
  parentId?: string;
  repoTags: string[];
  repoDigests?: string[];
  created: number;
  createdStr?: string;
  size: number;
  sizeMb?: number;
  virtualSize?: number;
  labels?: Record<string, string>;
  containers?: number;
}

interface DockerNetwork {
  id: string;
  name: string;
  driver: string;
  scope: string;
  subnet?: string;
  gateway?: string;
  internal: boolean;
  enableIPv6: boolean;
  containersCount: number;
  containers?: Record<string, string>;
  created?: string;
}

interface DockerStats {
  containerId: string;
  name: string;
  cpuPercent: number;
  memUsage: number;
  memLimit: number;
  memPercent: number;
  netRx: number;
  netTx: number;
  blockRead: number;
  blockWrite: number;
  pids: number;
  readAt: string;
}

interface DockerSystemInfo {
  serverVersion: string;
  operatingSystem: string;
  osType: string;
  architecture: string;
  ncpu: number;
  memTotal: number;
  containers: number;
  containersRunning: number;
  containersPaused: number;
  containersStopped: number;
  images: number;
  driver: string;
}

// State
const connections = ref<DockerConnection[]>([]);
const selectedConnectionId = ref<string>('');
const systemInfo = ref<DockerSystemInfo | null>(null);
const containers = ref<DockerContainer[]>([]);
const images = ref<DockerImage[]>([]);
const networks = ref<DockerNetwork[]>([]);

// Loading states
const loading = ref(true);
const refreshing = ref(false);
const actionLoading = ref<Record<string, boolean>>({});

// Tab Navigation
const activeTab = ref<'containers' | 'images' | 'networks' | 'deploy' | 'connections'>('containers');

// Filters
const searchKeyword = ref('');
const statusFilter = ref<'all' | 'running' | 'stopped' | 'paused'>('all');
const selectedContainers = ref<string[]>([]);

// Modals
const showLogsModal = ref(false);
const showStatsModal = ref(false);
const showDeleteModal = ref(false);
const showPullModal = ref(false);
const showAddConnModal = ref(false);
const showCreateNetModal = ref(false);

// Create Network Form State
const newNetForm = ref({
  name: '',
  driver: 'bridge',
  subnet: '',
  gateway: '',
  internal: false,
  enableIPv6: false,
});
const creatingNet = ref(false);

// Active Modal Data
const activeContainer = ref<DockerContainer | null>(null);
const containerLogs = ref<string>('');
const logsLoading = ref(false);
const logsTail = ref<number>(200);
const autoRefreshLogs = ref(false);
let logsInterval: any = null;
const logsTerminalRef = ref<HTMLElement | null>(null);

const scrollToBottom = () => {
  if (logsTerminalRef.value) {
    logsTerminalRef.value.scrollTop = logsTerminalRef.value.scrollHeight;
  }
};

// ANSI Terminal Formatter & Parser for Container Logs
const stripAnsi = (str: string): string => {
  if (!str) return '';
  return str
    .replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '')
    .replace(/\x1b\].*?(\x07|\x1b\\)/g, '')
    .replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '');
};

const formatAnsiToHtml = (raw: string): string => {
  if (!raw) return '';

  // 1. Sanitize HTML characters to prevent XSS injection
  let text = raw
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // 2. Parse ANSI SGR escape codes (\x1b[...m)
  const ansiRegex = /\x1b\[([0-9;]*)m/g;

  let hasAnsi = false;
  let openSpans = 0;

  text = text.replace(ansiRegex, (_, codes) => {
    hasAnsi = true;
    if (!codes || codes === '0' || codes === '00') {
      let close = '';
      while (openSpans > 0) {
        close += '</span>';
        openSpans--;
      }
      return close;
    }

    const codeList = codes.split(';');
    let style = '';
    for (const code of codeList) {
      switch (code) {
        case '1': // Bold
          style += 'font-weight: 700;';
          break;
        case '2': // Dim
          style += 'opacity: 0.85;';
          break;
        case '3': // Italic
          style += 'font-style: italic;';
          break;
        case '4': // Underline
          style += 'text-decoration: underline;';
          break;
        // High-contrast vibrant terminal colors
        case '30': // Black -> soft gray
        case '90': // Bright Black / Dark Gray -> soft slate-400 (never dark black)
          style += 'color: #94a3b8;';
          break;
        case '31': // Red
        case '91': // Bright Red
          style += 'color: #f87171;';
          break;
        case '32': // Green
        case '92': // Bright Green
          style += 'color: #34d399;';
          break;
        case '33': // Yellow
        case '93': // Bright Yellow
          style += 'color: #fbbf24;';
          break;
        case '34': // Blue
        case '94': // Bright Blue
          style += 'color: #60a5fa;';
          break;
        case '35': // Magenta
        case '95': // Bright Magenta
          style += 'color: #c084fc;';
          break;
        case '36': // Cyan
        case '96': // Bright Cyan
          style += 'color: #38bdf8;';
          break;
        case '37': // White
        case '97': // Bright White
          style += 'color: #f8fafc;';
          break;
        case '39': // Default text color
          style += 'color: #f1f5f9;';
          break;
      }
    }

    if (style) {
      openSpans++;
      return `<span style="${style}">`;
    }
    return '';
  });

  // Close any unclosed spans at the end
  while (openSpans > 0) {
    text += '</span>';
    openSpans--;
  }

  // Remove any remaining stray ANSI escape sequences
  text = text.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '');
  // Clean other unprintable characters (preserve newline, carriage return, tab)
  text = text.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '');

  // If the log stream had NO ANSI color codes, apply graceful syntax highlighting
  if (!hasAnsi) {
    text = text.replace(/(\b\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b)/g, '<span style="color: #94a3b8;">$1</span>');
    text = text.replace(/\b(INFO|info)\b/g, '<span style="color: #34d399; font-weight: 600;">$1</span>');
    text = text.replace(/\b(WARN|WARNING|warn|warning)\b/g, '<span style="color: #fbbf24; font-weight: 600;">$1</span>');
    text = text.replace(/\b(ERROR|FATAL|PANIC|error|fatal|panic)\b/g, '<span style="color: #f87171; font-weight: 700;">$1</span>');
    text = text.replace(/\b(DEBUG|debug|TRACE|trace)\b/g, '<span style="color: #38bdf8;">$1</span>');
  }

  return text;
};

const renderedLogs = computed(() => {
  return formatAnsiToHtml(containerLogs.value);
});

const liveStats = ref<DockerStats | null>(null);
const statsLoading = ref(false);
let statsInterval: any = null;

// Delete Target
const deleteTarget = ref<{
  type: 'container' | 'image' | 'connection' | 'network';
  id: string;
  name: string;
  force?: boolean;
} | null>(null);
const deleting = ref(false);
const forceDelete = ref(false);

// Pull Image Form
const pullImageName = ref('');
const pulling = ref(false);

// Deploy Container Form
const deployForm = ref({
  name: '',
  image: '',
  command: '',
  ports: [{ host: '', container: '', protocol: 'tcp' }],
  env: [{ key: '', value: '' }],
  volumes: [{ host: '', container: '', readonly: false }],
  restartPolicy: 'unless-stopped',
  networkMode: 'bridge',
  memoryLimitMb: 0,
  cpuLimit: 0,
  autoRemove: false,
});
const deploying = ref(false);

// Connection Form
const newConn = ref({
  name: '',
  driver: 'socket' as 'socket' | 'ssh' | 'tcp',
  socketPath: '/var/run/docker.sock',
  tcpHost: '',
  tcpPort: 2375,
  tcpTls: false,
  sshHost: '',
  sshPort: 22,
  sshUser: 'root',
  sshAuth: 'password' as 'password' | 'key',
  sshPassword: '',
  sshKey: '',
  isDefault: false,
});
const testingConn = ref(false);
const testConnResult = ref<{ success: boolean; message: string } | null>(null);
const savingConn = ref(false);

// Feedback Toast Notification (Auto-dismiss 3000ms)
const toast = ref<{ message: string; type: 'success' | 'error' } | null>(null);
let toastTimer: any = null;
const showToast = (message: string, type: 'success' | 'error' = 'success') => {
  if (toastTimer) clearTimeout(toastTimer);
  toast.value = { message, type };
  toastTimer = setTimeout(() => {
    toast.value = null;
  }, 3000);
};

// Clipboard copy helper
const copiedId = ref<string | null>(null);
const copyToClipboard = async (text: string, id: string) => {
  try {
    await navigator.clipboard.writeText(text);
    copiedId.value = id;
    setTimeout(() => {
      if (copiedId.value === id) copiedId.value = null;
    }, 2000);
  } catch {
    showToast('Failed to copy to clipboard', 'error');
  }
};

// Format utilities
const formatBytes = (bytes: number): string => {
  if (!bytes || isNaN(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  if (i < 0 || i >= units.length) return '0 B';
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
};

const formatTimestamp = (unix: number | string): string => {
  if (!unix || unix === 0 || unix === '0') return '-';
  if (typeof unix === 'number') {
    const d = new Date(unix * 1000);
    if (!isNaN(d.getTime())) return d.toLocaleString();
  }
  if (typeof unix === 'string') {
    const d = new Date(unix);
    if (!isNaN(d.getTime())) return d.toLocaleString();
    return unix;
  }
  return '-';
};

const getCleanContainerName = (c?: DockerContainer | null): string => {
  if (!c) return '-';
  if (c.name && c.name.trim()) return c.name.startsWith('/') ? c.name.substring(1) : c.name;
  if (Array.isArray(c.names) && c.names.length > 0 && c.names[0]) {
    const n = c.names[0];
    return n.startsWith('/') ? n.substring(1) : n;
  }
  return c.id ? c.id.substring(0, 12) : '-';
};

// Computed Active Connection
const activeConnection = computed(() => {
  return connections.value.find((c) => c.id === selectedConnectionId.value) || null;
});

// Filtered Containers
const filteredContainers = computed(() => {
  let list = containers.value;

  if (statusFilter.value === 'running') {
    list = list.filter((c) => c.state === 'running');
  } else if (statusFilter.value === 'stopped') {
    list = list.filter((c) => c.state === 'exited' || c.state === 'dead');
  } else if (statusFilter.value === 'paused') {
    list = list.filter((c) => c.state === 'paused');
  }

  if (searchKeyword.value.trim()) {
    const q = searchKeyword.value.toLowerCase().trim();
    list = list.filter((c) => {
      const name = getCleanContainerName(c).toLowerCase();
      const img = (c.image || '').toLowerCase();
      const id = (c.id || '').toLowerCase();
      return name.includes(q) || img.includes(q) || id.includes(q);
    });
  }

  return list;
});

// Load Connections List
const fetchConnections = async () => {
  try {
    const res = await axios.get('/api/v1/docker/connections');
    if (res.data?.success && Array.isArray(res.data.data)) {
      connections.value = res.data.data;
      if (!selectedConnectionId.value && connections.value.length > 0) {
        const def = connections.value.find((c) => c.isDefault);
        selectedConnectionId.value = def ? def.id : connections.value[0].id;
      }
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to fetch Docker environments', 'error');
  }
};

// Fetch System Info & Containers & Images & Networks
const fetchData = async () => {
  if (!selectedConnectionId.value) {
    loading.value = false;
    refreshing.value = false;
    return;
  }
  refreshing.value = true;
  try {
    const [infoRes, contRes, imgRes, netRes] = await Promise.all([
      axios.get('/api/v1/docker/system/info', { params: { connectionId: selectedConnectionId.value } }).catch(() => ({ data: null })),
      axios.get('/api/v1/docker/containers', { params: { connectionId: selectedConnectionId.value, all: true } }).catch(() => ({ data: null })),
      axios.get('/api/v1/docker/images', { params: { connectionId: selectedConnectionId.value } }).catch(() => ({ data: null })),
      axios.get('/api/v1/docker/networks', { params: { connectionId: selectedConnectionId.value } }).catch(() => ({ data: null })),
    ]);

    if (infoRes.data?.success) {
      systemInfo.value = infoRes.data.data;
    }
    if (contRes.data?.success && Array.isArray(contRes.data.data)) {
      containers.value = contRes.data.data;
    } else {
      containers.value = [];
    }
    if (imgRes.data?.success && Array.isArray(imgRes.data.data)) {
      images.value = imgRes.data.data;
    } else {
      images.value = [];
    }
    if (netRes.data?.success && Array.isArray(netRes.data.data)) {
      networks.value = netRes.data.data;
    } else {
      networks.value = [];
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Error refreshing Docker data', 'error');
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
};

// Container Lifecycle Actions
const performContainerAction = async (containerId: string, action: 'start' | 'stop' | 'restart' | 'pause' | 'unpause') => {
  actionLoading.value[containerId] = true;
  try {
    const res = await axios.post(`/api/v1/docker/containers/${containerId}/${action}`, null, {
      params: { connectionId: selectedConnectionId.value },
    });
    if (res.data?.success) {
      showToast(`Container ${action}ed successfully`);
      await fetchData();
    } else {
      showToast(res.data?.error || `Failed to ${action} container`, 'error');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || `Failed to ${action} container`, 'error');
  } finally {
    actionLoading.value[containerId] = false;
  }
};

// Open Logs Modal
const openLogsModal = async (container: DockerContainer) => {
  activeContainer.value = container;
  showLogsModal.value = true;
  await fetchLogs();
  await nextTick();
  scrollToBottom();
};

const fetchLogs = async () => {
  if (!activeContainer.value) return;
  logsLoading.value = true;
  try {
    const res = await axios.get(`/api/v1/docker/containers/${activeContainer.value.id}/logs`, {
      params: {
        connectionId: selectedConnectionId.value,
        tail: logsTail.value,
        timestamps: true,
      },
    });
    if (res.data?.success) {
      const output = res.data.logs !== undefined ? res.data.logs : (res.data.data?.logs !== undefined ? res.data.data.logs : res.data.data);
      containerLogs.value = output !== undefined && output !== null && output !== '' ? String(output) : '';
      await nextTick();
      scrollToBottom();
    } else {
      containerLogs.value = res.data?.error || 'Failed to fetch logs.';
    }
  } catch (err: any) {
    containerLogs.value = err.response?.data?.error || err.message || 'Error retrieving logs.';
  } finally {
    logsLoading.value = false;
  }
};

watch(autoRefreshLogs, (val) => {
  if (val) {
    logsInterval = setInterval(fetchLogs, 3000);
  } else if (logsInterval) {
    clearInterval(logsInterval);
    logsInterval = null;
  }
});

// Open Stats Modal
const openStatsModal = async (container: DockerContainer) => {
  activeContainer.value = container;
  showStatsModal.value = true;
  liveStats.value = null;
  await fetchStats();
  if (statsInterval) clearInterval(statsInterval);
  statsInterval = setInterval(fetchStats, 3000);
};

const fetchStats = async () => {
  if (!activeContainer.value) return;
  statsLoading.value = true;
  try {
    const res = await axios.get(`/api/v1/docker/containers/${activeContainer.value.id}/stats`, {
      params: { connectionId: selectedConnectionId.value },
    });
    if (res.data?.success && res.data.data) {
      const raw = res.data.data;
      const cpu = typeof raw.cpuPercent === 'number' && !isNaN(raw.cpuPercent) ? raw.cpuPercent : 0;
      const memPerc = typeof raw.memPercent === 'number' && !isNaN(raw.memPercent) ? raw.memPercent : (typeof raw.memoryPercent === 'number' && !isNaN(raw.memoryPercent) ? raw.memoryPercent : 0);
      const memUsage = typeof raw.memUsage === 'number' && !isNaN(raw.memUsage) ? raw.memUsage : (typeof raw.memoryUsageMb === 'number' && !isNaN(raw.memoryUsageMb) ? Math.round(raw.memoryUsageMb * 1024 * 1024) : 0);
      const memLimit = typeof raw.memLimit === 'number' && !isNaN(raw.memLimit) ? raw.memLimit : (typeof raw.memoryLimitMb === 'number' && !isNaN(raw.memoryLimitMb) ? Math.round(raw.memoryLimitMb * 1024 * 1024) : 0);
      const netRx = typeof raw.netRx === 'number' && !isNaN(raw.netRx) ? raw.netRx : (typeof raw.networkRxMb === 'number' && !isNaN(raw.networkRxMb) ? Math.round(raw.networkRxMb * 1024 * 1024) : 0);
      const netTx = typeof raw.netTx === 'number' && !isNaN(raw.netTx) ? raw.netTx : (typeof raw.networkTxMb === 'number' && !isNaN(raw.networkTxMb) ? Math.round(raw.networkTxMb * 1024 * 1024) : 0);

      liveStats.value = {
        containerId: raw.containerId || activeContainer.value.id,
        name: raw.name || getCleanContainerName(activeContainer.value),
        cpuPercent: cpu,
        memPercent: memPerc,
        memUsage: memUsage,
        memLimit: memLimit,
        netRx: netRx,
        netTx: netTx,
        blockRead: typeof raw.blockRead === 'number' && !isNaN(raw.blockRead) ? raw.blockRead : 0,
        blockWrite: typeof raw.blockWrite === 'number' && !isNaN(raw.blockWrite) ? raw.blockWrite : 0,
        pids: typeof raw.pids === 'number' && !isNaN(raw.pids) ? raw.pids : (raw.pids ? parseInt(raw.pids, 10) || 0 : 0),
        readAt: raw.readAt || new Date().toLocaleTimeString(),
      };
    }
  } catch {
    // Silent polling error
  } finally {
    statsLoading.value = false;
  }
};

// Pull Docker Image
const handlePullImage = async () => {
  if (!pullImageName.value.trim()) {
    showToast('Please provide an image name (e.g. nginx:alpine)', 'error');
    return;
  }
  pulling.value = true;
  try {
    const res = await axios.post('/api/v1/docker/images/pull', {
      image: pullImageName.value.trim(),
    }, {
      params: { connectionId: selectedConnectionId.value },
    });
    if (res.data?.success) {
      showToast(`Image ${pullImageName.value} pulled successfully`);
      showPullModal.value = false;
      pullImageName.value = '';
      await fetchData();
    } else {
      showToast(res.data?.error || 'Failed to pull image', 'error');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Image pull failed', 'error');
  } finally {
    pulling.value = false;
  }
};

// Deploy Container Wizard
const addPortRow = () => {
  deployForm.value.ports.push({ host: '', container: '', protocol: 'tcp' });
};
const removePortRow = (index: number) => {
  deployForm.value.ports.splice(index, 1);
};

const addEnvRow = () => {
  deployForm.value.env.push({ key: '', value: '' });
};
const removeEnvRow = (index: number) => {
  deployForm.value.env.splice(index, 1);
};

const addVolumeRow = () => {
  deployForm.value.volumes.push({ host: '', container: '', readonly: false });
};
const removeVolumeRow = (index: number) => {
  deployForm.value.volumes.splice(index, 1);
};

const handleDeploy = async () => {
  if (!deployForm.value.name.trim() || !deployForm.value.image.trim()) {
    showToast('Container Name and Image are required', 'error');
    return;
  }
  deploying.value = true;

  // Process ports
  const validPorts = deployForm.value.ports
    .filter((p) => p.host && p.container)
    .map((p) => `${p.host}:${p.container}/${p.protocol || 'tcp'}`);

  // Process env
  const validEnv = deployForm.value.env
    .filter((e) => e.key)
    .map((e) => `${e.key}=${e.value}`);

  // Process volumes
  const validVols = deployForm.value.volumes
    .filter((v) => v.host && v.container)
    .map((v) => `${v.host}:${v.container}${v.readonly ? ':ro' : ''}`);

  try {
    const res = await axios.post('/api/v1/docker/containers/deploy', {
      name: deployForm.value.name.trim(),
      image: deployForm.value.image.trim(),
      command: deployForm.value.command ? deployForm.value.command.split(' ') : [],
      ports: validPorts,
      environment: validEnv,
      volumes: validVols,
      restartPolicy: deployForm.value.restartPolicy,
      networkMode: deployForm.value.networkMode,
      memoryLimit: Number(deployForm.value.memoryLimitMb) * 1024 * 1024,
      cpuLimit: Number(deployForm.value.cpuLimit),
      autoRemove: deployForm.value.autoRemove,
    }, {
      params: { connectionId: selectedConnectionId.value },
    });

    if (res.data?.success) {
      showToast(`Container "${deployForm.value.name}" deployed successfully`);
      activeTab.value = 'containers';
      // Reset form
      deployForm.value = {
        name: '',
        image: '',
        command: '',
        ports: [{ host: '', container: '', protocol: 'tcp' }],
        env: [{ key: '', value: '' }],
        volumes: [{ host: '', container: '', readonly: false }],
        restartPolicy: 'unless-stopped',
        networkMode: 'bridge',
        memoryLimitMb: 0,
        cpuLimit: 0,
        autoRemove: false,
      };
      await fetchData();
    } else {
      showToast(res.data?.error || 'Deployment failed', 'error');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Deployment request failed', 'error');
  } finally {
    deploying.value = false;
  }
};

// Delete Confirmation Flow
const confirmDeleteContainer = (c: DockerContainer) => {
  deleteTarget.value = {
    type: 'container',
    id: c.id,
    name: getCleanContainerName(c),
    force: false,
  };
  forceDelete.value = false;
  showDeleteModal.value = true;
};

const confirmDeleteImage = (img: DockerImage) => {
  const tagName = img.repoTags && img.repoTags.length > 0 ? img.repoTags[0] : img.id.substring(7, 19);
  deleteTarget.value = {
    type: 'image',
    id: img.id,
    name: tagName,
    force: false,
  };
  forceDelete.value = false;
  showDeleteModal.value = true;
};

const confirmDeleteConnection = (conn: DockerConnection) => {
  deleteTarget.value = {
    type: 'connection',
    id: conn.id,
    name: conn.name,
  };
  showDeleteModal.value = true;
};

const confirmDeleteNetwork = (net: DockerNetwork) => {
  deleteTarget.value = {
    type: 'network',
    id: net.id,
    name: net.name,
  };
  showDeleteModal.value = true;
};

const handleCreateNetwork = async () => {
  if (!newNetForm.value.name.trim()) {
    showToast('Network name is required', 'error');
    return;
  }
  creatingNet.value = true;
  try {
    const res = await axios.post('/api/v1/docker/networks', {
      name: newNetForm.value.name.trim(),
      driver: newNetForm.value.driver,
      subnet: newNetForm.value.subnet.trim(),
      gateway: newNetForm.value.gateway.trim(),
      internal: newNetForm.value.internal,
      enableIPv6: newNetForm.value.enableIPv6,
    }, {
      params: { connectionId: selectedConnectionId.value },
    });
    if (res.data?.success) {
      showToast(`Network "${newNetForm.value.name}" created successfully`);
      showCreateNetModal.value = false;
      newNetForm.value = {
        name: '',
        driver: 'bridge',
        subnet: '',
        gateway: '',
        internal: false,
        enableIPv6: false,
      };
      await fetchData();
    } else {
      showToast(res.data?.error || 'Failed to create network', 'error');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to create network', 'error');
  } finally {
    creatingNet.value = false;
  }
};

const executeDelete = async () => {
  if (!deleteTarget.value) return;
  deleting.value = true;

  try {
    if (deleteTarget.value.type === 'container') {
      const res = await axios.delete(`/api/v1/docker/containers/${deleteTarget.value.id}`, {
        params: {
          connectionId: selectedConnectionId.value,
          force: forceDelete.value,
        },
      });
      if (res.data?.success) {
        showToast(`Container "${deleteTarget.value.name}" removed successfully`);
        showDeleteModal.value = false;
        await fetchData();
      } else {
        showToast(res.data?.error || 'Failed to remove container', 'error');
      }
    } else if (deleteTarget.value.type === 'image') {
      const res = await axios.delete(`/api/v1/docker/images/${deleteTarget.value.id}`, {
        params: {
          connectionId: selectedConnectionId.value,
          force: forceDelete.value,
        },
      });
      if (res.data?.success) {
        showToast(`Image "${deleteTarget.value.name}" removed successfully`);
        showDeleteModal.value = false;
        await fetchData();
      } else {
        showToast(res.data?.error || 'Failed to remove image', 'error');
      }
    } else if (deleteTarget.value.type === 'network') {
      const res = await axios.delete(`/api/v1/docker/networks/${deleteTarget.value.id}`, {
        params: { connectionId: selectedConnectionId.value },
      });
      if (res.data?.success) {
        showToast(`Network "${deleteTarget.value.name}" removed successfully`);
        showDeleteModal.value = false;
        await fetchData();
      } else {
        showToast(res.data?.error || 'Failed to remove network', 'error');
      }
    } else if (deleteTarget.value.type === 'connection') {
      const res = await axios.delete(`/api/v1/docker/connections/${deleteTarget.value.id}`);
      if (res.data?.success) {
        showToast(`Connection "${deleteTarget.value.name}" removed`);
        showDeleteModal.value = false;
        await fetchConnections();
        if (selectedConnectionId.value === deleteTarget.value.id) {
          selectedConnectionId.value = connections.value.length > 0 ? connections.value[0].id : '';
          await fetchData();
        }
      } else {
        showToast(res.data?.error || 'Failed to remove connection', 'error');
      }
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Deletion failed', 'error');
  } finally {
    deleting.value = false;
  }
};

// Add Connection Flow
const handleTestNewConn = async () => {
  testingConn.value = true;
  testConnResult.value = null;
  try {
    const res = await axios.post('/api/v1/docker/connections/test', {
      driver: newConn.value.driver,
      socketPath: newConn.value.socketPath,
      tcpHost: newConn.value.tcpHost,
      tcpPort: Number(newConn.value.tcpPort) || 2375,
      tcpTls: newConn.value.tcpTls,
      sshHost: newConn.value.sshHost,
      sshPort: Number(newConn.value.sshPort) || 22,
      sshUser: newConn.value.sshUser,
      sshAuth: newConn.value.sshAuth,
      sshPassword: newConn.value.sshPassword,
      sshKey: newConn.value.sshKey,
    });
    testConnResult.value = {
      success: res.data?.success || false,
      message: res.data?.success
        ? (res.data?.message || 'Docker connection verified successfully!')
        : (res.data?.error || 'Failed to connect to Docker engine.'),
    };
  } catch (err: any) {
    testConnResult.value = {
      success: false,
      message: err.response?.data?.error || 'Failed to test Docker connection',
    };
  } finally {
    testingConn.value = false;
  }
};

const handleSaveNewConn = async () => {
  if (!newConn.value.name.trim()) {
    showToast('Connection Name is required', 'error');
    return;
  }
  savingConn.value = true;
  try {
    const res = await axios.post('/api/v1/docker/connections', {
      name: newConn.value.name.trim(),
      driver: newConn.value.driver,
      socketPath: newConn.value.socketPath || '/var/run/docker.sock',
      tcpHost: newConn.value.tcpHost,
      tcpPort: Number(newConn.value.tcpPort) || 2375,
      tcpTls: newConn.value.tcpTls,
      sshHost: newConn.value.sshHost,
      sshPort: Number(newConn.value.sshPort) || 22,
      sshUser: newConn.value.sshUser,
      sshAuth: newConn.value.sshAuth,
      sshPassword: newConn.value.sshPassword,
      sshKey: newConn.value.sshKey,
      isDefault: newConn.value.isDefault,
    });
    if (res.data?.success) {
      showToast('Docker connection registered successfully');
      showAddConnModal.value = false;
      const createdId = res.data.data?.id;
      await fetchConnections();
      if (createdId) {
        selectedConnectionId.value = createdId;
      }
      await fetchData();
    } else {
      showToast(res.data?.error || 'Failed to save connection', 'error');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Save connection failed', 'error');
  } finally {
    savingConn.value = false;
  }
};

// Lifecycle Hooks
onMounted(async () => {
  await fetchConnections();
  await fetchData();
});

onUnmounted(() => {
  if (logsInterval) clearInterval(logsInterval);
  if (statsInterval) clearInterval(statsInterval);
});

// Watch connection change
watch(selectedConnectionId, () => {
  fetchData();
});
</script>

<template>
  <div class="min-h-screen bg-slate-100 dark:bg-[#090d16] text-slate-800 dark:text-slate-100 font-sans">
    
    <!-- Top Bar Navigation Header -->
    <header class="h-14 md:h-16 bg-white dark:bg-[#0c101a] border-b border-slate-200 dark:border-[#1b2234] px-4 sm:px-6 flex items-center justify-between sticky top-0 z-30 shadow-xs">
      <div class="flex items-center gap-3">
        <a
          href="/"
          class="flex items-center gap-2 text-xs font-semibold text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#121826] transition cursor-pointer"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span class="hidden sm:inline">Back to Dashboard</span>
        </a>

        <div class="h-4 w-px bg-slate-200 dark:bg-[#1b2234]"></div>

        <div class="flex items-center gap-2">
          <span class="font-bold text-xs tracking-wider text-slate-900 dark:text-white uppercase">DOCKER ENGINE</span>
          <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20 font-semibold">INFRASTRUCTURE</span>
        </div>
      </div>

      <!-- Environment Selector & Global Actions -->
      <div class="flex items-center gap-2.5">
        <!-- Environment Dropdown -->
        <div class="relative flex items-center">
          <div class="flex items-center gap-2 px-3 py-1.5 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg text-xs">
            <span
              :class="[
                'w-2 h-2 rounded-full shrink-0',
                systemInfo ? 'bg-emerald-500 animate-pulse' : 'bg-rose-500'
              ]"
            ></span>
            <select
              v-model="selectedConnectionId"
              class="bg-transparent text-slate-800 dark:text-slate-200 font-semibold focus:outline-none cursor-pointer text-xs pr-2"
            >
              <option
                v-for="c in connections"
                :key="c.id"
                :value="c.id"
                class="bg-white dark:bg-[#0c101a] text-slate-800 dark:text-slate-200"
              >
                {{ c.name }} ({{ c.driver.toUpperCase() }})
              </option>
            </select>
          </div>
        </div>

        <button
          @click="fetchData"
          :disabled="refreshing"
          class="p-2 text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white rounded-lg border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#121826] transition cursor-pointer disabled:opacity-50"
          title="Refresh Docker Data"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': refreshing }" />
        </button>

        <ThemeToggle />
      </div>
    </header>

    <!-- Feedback Toast Banner -->
    <Transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="transform -translate-y-4 opacity-0"
      enter-to-class="transform translate-y-0 opacity-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="transform translate-y-0 opacity-100"
      leave-to-class="transform -translate-y-4 opacity-0"
    >
      <div
        v-if="toast"
        class="fixed top-20 right-6 z-50 px-4 py-2.5 rounded-xl shadow-xl text-xs font-semibold flex items-center gap-2 border"
        :class="[
          toast.type === 'success'
            ? 'bg-emerald-50 dark:bg-emerald-950/90 text-emerald-700 dark:text-emerald-300 border-emerald-300 dark:border-emerald-800'
            : 'bg-rose-50 dark:bg-rose-950/90 text-rose-700 dark:text-rose-300 border-rose-300 dark:border-rose-800'
        ]"
      >
        <CheckCircle2 v-if="toast.type === 'success'" class="w-4 h-4 shrink-0 text-emerald-500" />
        <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-500" />
        <span>{{ toast.message }}</span>
      </div>
    </Transition>

    <!-- Main Content Container -->
    <main class="max-w-[1600px] mx-auto p-4 sm:p-6 space-y-6">

      <!-- Page Header (Strict AGENTS.md: pure clean text h1, no icons, no badges) -->
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
        <div>
          <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
            Management Containers
          </h1>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            Manage Docker containers, lifecycle states, logs, metrics, images, and deployments across local and remote environments.
          </p>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <button
            @click="showPullModal = true"
            class="px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-300 bg-white dark:bg-[#121826] border border-slate-300 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#1a2336] rounded-lg transition flex items-center gap-1.5 cursor-pointer"
          >
            <DownloadCloud class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
            <span>Pull Image</span>
          </button>
          <button
            @click="activeTab = 'deploy'"
            class="px-3.5 py-1.5 text-xs font-bold text-white bg-blue-600 hover:bg-blue-500 rounded-lg shadow-sm transition flex items-center gap-1.5 cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Deploy Container</span>
          </button>
        </div>
      </div>

      <!-- Quick Metrics Summary Cards -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <!-- Containers Card -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1 shadow-xs">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">Total Containers</span>
            <Boxes class="w-4 h-4 text-slate-400" />
          </div>
          <div class="text-2xl font-bold text-slate-900 dark:text-white font-mono">
            {{ systemInfo?.containers ?? containers.length }}
          </div>
          <div class="flex items-center gap-2 text-[10px] text-slate-500 dark:text-slate-400 pt-0.5">
            <span class="text-emerald-600 dark:text-emerald-400 font-semibold">{{ systemInfo?.containersRunning ?? 0 }} Running</span>
            <span>•</span>
            <span class="text-rose-600 dark:text-rose-400 font-semibold">{{ systemInfo?.containersStopped ?? 0 }} Stopped</span>
          </div>
        </div>

        <!-- Running Containers Card -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1 shadow-xs">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">Active Workloads</span>
            <Play class="w-4 h-4 text-slate-400" />
          </div>
          <div class="text-2xl font-bold text-emerald-600 dark:text-emerald-400 font-mono">
            {{ systemInfo?.containersRunning ?? containers.filter(c => c.state === 'running').length }}
          </div>
          <div class="text-[10px] text-slate-500 dark:text-slate-400 pt-0.5">
            {{ systemInfo?.containersPaused ?? 0 }} paused containers
          </div>
        </div>

        <!-- Images Card -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1 shadow-xs">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">Docker Images</span>
            <Layers class="w-4 h-4 text-slate-400" />
          </div>
          <div class="text-2xl font-bold text-slate-900 dark:text-white font-mono">
            {{ systemInfo?.images ?? images.length }}
          </div>
          <div class="text-[10px] text-slate-500 dark:text-slate-400 pt-0.5">
            Locally stored repository tags
          </div>
        </div>

        <!-- Engine Details Card -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-1 shadow-xs">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">Engine Version</span>
            <Server class="w-4 h-4 text-slate-400" />
          </div>
          <div class="text-base font-bold text-slate-900 dark:text-white font-mono truncate">
            {{ systemInfo?.serverVersion || 'Docker API' }}
          </div>
          <div class="text-[10px] text-slate-500 dark:text-slate-400 pt-0.5 truncate">
            {{ systemInfo?.operatingSystem || (activeConnection?.driver ? (activeConnection.driver.toUpperCase() + ' Driver') : 'Docker Host') }}
          </div>
        </div>
      </div>

      <!-- Tab Buttons Navigation -->
      <div class="flex items-center gap-1 border-b border-slate-200 dark:border-[#1b2234]">
        <button
          @click="activeTab = 'containers'"
          :class="[
            'px-4 py-2.5 text-xs font-bold border-b-2 transition flex items-center gap-2 cursor-pointer',
            activeTab === 'containers'
              ? 'border-blue-600 text-blue-700 dark:text-[#95CCDD]'
              : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          ]"
        >
          <Boxes class="w-3.5 h-3.5" />
          <span>Containers ({{ containers.length }})</span>
        </button>

        <button
          @click="activeTab = 'images'"
          :class="[
            'px-4 py-2.5 text-xs font-bold border-b-2 transition flex items-center gap-2 cursor-pointer',
            activeTab === 'images'
              ? 'border-blue-600 text-blue-700 dark:text-[#95CCDD]'
              : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          ]"
        >
          <Layers class="w-3.5 h-3.5" />
          <span>Images ({{ images.length }})</span>
        </button>

        <button
          @click="activeTab = 'networks'"
          :class="[
            'px-4 py-2.5 text-xs font-bold border-b-2 transition flex items-center gap-2 cursor-pointer',
            activeTab === 'networks'
              ? 'border-blue-600 text-blue-700 dark:text-[#95CCDD]'
              : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          ]"
        >
          <Network class="w-3.5 h-3.5" />
          <span>Networks ({{ networks.length }})</span>
        </button>

        <button
          @click="activeTab = 'deploy'"
          :class="[
            'px-4 py-2.5 text-xs font-bold border-b-2 transition flex items-center gap-2 cursor-pointer',
            activeTab === 'deploy'
              ? 'border-blue-600 text-blue-700 dark:text-[#95CCDD]'
              : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          ]"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Deploy Wizard</span>
        </button>

        <button
          @click="activeTab = 'connections'"
          :class="[
            'px-4 py-2.5 text-xs font-bold border-b-2 transition flex items-center gap-2 cursor-pointer',
            activeTab === 'connections'
              ? 'border-blue-600 text-blue-700 dark:text-[#95CCDD]'
              : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          ]"
        >
          <Server class="w-3.5 h-3.5" />
          <span>Connections ({{ connections.length }})</span>
        </button>
      </div>

      <!-- ============================================================= -->
      <!-- TAB 1: CONTAINERS VIEW -->
      <!-- ============================================================= -->
      <div v-if="activeTab === 'containers'" class="space-y-4">
        <!-- Filter Toolbar -->
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-white dark:bg-[#0e121c] p-3 border border-slate-200 dark:border-[#1b2234] rounded-xl">
          <!-- Search -->
          <div class="relative flex-1 max-w-md">
            <Search class="w-3.5 h-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              v-model="searchKeyword"
              placeholder="Search container by name, image, or ID..."
              class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg pl-9 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 font-mono"
            />
          </div>

          <!-- State Filter Buttons -->
          <div class="flex items-center gap-1 bg-slate-50 dark:bg-[#141824] p-1 rounded-lg border border-slate-200 dark:border-[#1b2234] self-start sm:self-auto">
            <button
              @click="statusFilter = 'all'"
              :class="[
                'px-2.5 py-1 text-[11px] font-semibold rounded-md transition cursor-pointer',
                statusFilter === 'all'
                  ? 'bg-white dark:bg-[#20283e] text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              All ({{ containers.length }})
            </button>
            <button
              @click="statusFilter = 'running'"
              :class="[
                'px-2.5 py-1 text-[11px] font-semibold rounded-md transition cursor-pointer',
                statusFilter === 'running'
                  ? 'bg-white dark:bg-[#20283e] text-emerald-600 dark:text-emerald-400 shadow-xs'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Running ({{ containers.filter(c => c.state === 'running').length }})
            </button>
            <button
              @click="statusFilter = 'stopped'"
              :class="[
                'px-2.5 py-1 text-[11px] font-semibold rounded-md transition cursor-pointer',
                statusFilter === 'stopped'
                  ? 'bg-white dark:bg-[#20283e] text-rose-600 dark:text-rose-400 shadow-xs'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Stopped ({{ containers.filter(c => c.state === 'exited' || c.state === 'dead').length }})
            </button>
            <button
              @click="statusFilter = 'paused'"
              :class="[
                'px-2.5 py-1 text-[11px] font-semibold rounded-md transition cursor-pointer',
                statusFilter === 'paused'
                  ? 'bg-white dark:bg-[#20283e] text-amber-600 dark:text-amber-400 shadow-xs'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Paused ({{ containers.filter(c => c.state === 'paused').length }})
            </button>
          </div>
        </div>

        <!-- Containers Table -->
        <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl overflow-hidden shadow-xs">
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-slate-50 dark:bg-[#121826] border-b border-slate-200 dark:border-[#1b2234] text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                <tr>
                  <th class="py-3 px-4">State</th>
                  <th class="py-3 px-4">Container Name</th>
                  <th class="py-3 px-4">Image</th>
                  <th class="py-3 px-4">Network & IP</th>
                  <th class="py-3 px-4">Ports</th>
                  <th class="py-3 px-4">Status</th>
                  <th class="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]">
                <tr v-if="loading" class="text-center">
                  <td colspan="7" class="py-12 text-slate-500">
                    <RefreshCw class="w-5 h-5 animate-spin mx-auto mb-2 text-slate-400" />
                    <span>Loading containers...</span>
                  </td>
                </tr>

                <tr v-else-if="filteredContainers.length === 0" class="text-center">
                  <td colspan="7" class="py-12 text-slate-500">
                    <Boxes class="w-8 h-8 text-slate-400 mx-auto mb-2 opacity-50" />
                    <p class="font-bold text-slate-700 dark:text-slate-300">No containers found</p>
                    <p class="text-xs text-slate-400 mt-1">Deploy a new container or change filter parameters.</p>
                  </td>
                </tr>

                <tr
                  v-for="c in filteredContainers"
                  :key="c.id"
                  class="hover:bg-slate-50/70 dark:hover:bg-[#141b2c]/50 transition group"
                >
                  <!-- State Indicator -->
                  <td class="py-3 px-4 whitespace-nowrap">
                    <div class="flex items-center gap-2">
                      <span
                        :class="[
                          'w-2.5 h-2.5 rounded-full shrink-0',
                          c.state === 'running' ? 'bg-emerald-500 animate-pulse' :
                          c.state === 'paused' ? 'bg-amber-500' :
                          'bg-rose-500'
                        ]"
                      ></span>
                      <span
                        :class="[
                          'px-2 py-0.5 rounded text-[10px] font-bold uppercase font-mono',
                          c.state === 'running' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20' :
                          c.state === 'paused' ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20' :
                          'bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20'
                        ]"
                      >
                        {{ c.state }}
                      </span>
                    </div>
                  </td>

                  <!-- Name & Short ID -->
                  <td class="py-3 px-4">
                    <div class="font-bold text-slate-900 dark:text-white">
                      {{ getCleanContainerName(c) }}
                    </div>
                    <div class="flex items-center gap-1.5 text-[10px] text-slate-400 font-mono mt-0.5">
                      <span>{{ c.id.substring(0, 12) }}</span>
                      <button
                        @click="copyToClipboard(c.id, c.id)"
                        class="hover:text-slate-600 dark:hover:text-slate-200 transition cursor-pointer"
                        title="Copy Container ID"
                      >
                        <Check v-if="copiedId === c.id" class="w-3 h-3 text-emerald-500" />
                        <Copy v-else class="w-3 h-3" />
                      </button>
                    </div>
                  </td>

                  <!-- Image -->
                  <td class="py-3 px-4 font-mono text-[11px] text-slate-700 dark:text-slate-300 max-w-[200px] truncate" :title="c.image">
                    {{ c.image }}
                  </td>

                  <!-- Network & IP -->
                  <td class="py-3 px-4 text-[11px] font-mono">
                    <div v-if="(c.networks && c.networks.length > 0) || c.ipAddress" class="flex flex-col gap-0.5">
                      <div v-if="c.networks && c.networks.length > 0" class="flex flex-wrap gap-1">
                        <span
                          v-for="net in c.networks"
                          :key="net"
                          class="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20"
                        >
                          {{ net }}
                        </span>
                      </div>
                      <div v-if="c.ipAddress" class="text-[10px] text-slate-500 dark:text-slate-400">
                        {{ c.ipAddress }}
                      </div>
                    </div>
                    <span v-else class="text-slate-400">-</span>
                  </td>

                  <!-- Ports -->
                  <td class="py-3 px-4 font-mono text-[11px]">
                    <div v-if="c.ports && c.ports.length > 0" class="flex flex-wrap gap-1">
                      <span
                        v-for="(p, pIdx) in c.ports"
                        :key="pIdx"
                        class="px-1.5 py-0.5 rounded bg-slate-100 dark:bg-[#141824] border border-slate-200 dark:border-[#1b2234] text-slate-700 dark:text-slate-300 text-[10px]"
                      >
                        <span v-if="p.publicPort">{{ p.publicPort }}:</span>{{ p.privatePort }}/{{ p.type }}
                      </span>
                    </div>
                    <span v-else class="text-slate-400">-</span>
                  </td>

                  <!-- Status -->
                  <td class="py-3 px-4 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                    {{ c.status }}
                  </td>

                  <!-- Actions -->
                  <td class="py-3 px-4 text-right whitespace-nowrap">
                    <div class="flex items-center justify-end gap-1">
                      <!-- Start button if stopped -->
                      <button
                        v-if="c.state !== 'running'"
                        @click="performContainerAction(c.id, 'start')"
                        :disabled="actionLoading[c.id]"
                        class="p-1.5 text-slate-500 hover:text-emerald-600 dark:hover:text-emerald-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="Start Container"
                      >
                        <Play class="w-3.5 h-3.5" />
                      </button>

                      <!-- Stop button if running -->
                      <button
                        v-if="c.state === 'running'"
                        @click="performContainerAction(c.id, 'stop')"
                        :disabled="actionLoading[c.id]"
                        class="p-1.5 text-slate-500 hover:text-rose-600 dark:hover:text-rose-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="Stop Container"
                      >
                        <Square class="w-3.5 h-3.5" />
                      </button>

                      <!-- Restart button -->
                      <button
                        @click="performContainerAction(c.id, 'restart')"
                        :disabled="actionLoading[c.id]"
                        class="p-1.5 text-slate-500 hover:text-blue-600 dark:hover:text-blue-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="Restart Container"
                      >
                        <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': actionLoading[c.id] }" />
                      </button>

                      <!-- Pause / Unpause -->
                      <button
                        v-if="c.state === 'running'"
                        @click="performContainerAction(c.id, 'pause')"
                        :disabled="actionLoading[c.id]"
                        class="p-1.5 text-slate-500 hover:text-amber-600 dark:hover:text-amber-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="Pause Container"
                      >
                        <Pause class="w-3.5 h-3.5" />
                      </button>
                      <button
                        v-if="c.state === 'paused'"
                        @click="performContainerAction(c.id, 'unpause')"
                        :disabled="actionLoading[c.id]"
                        class="p-1.5 text-slate-500 hover:text-emerald-600 dark:hover:text-emerald-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="Unpause Container"
                      >
                        <Play class="w-3.5 h-3.5" />
                      </button>

                      <!-- Logs button -->
                      <button
                        @click="openLogsModal(c)"
                        class="p-1.5 text-slate-500 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="View Logs"
                      >
                        <Terminal class="w-3.5 h-3.5" />
                      </button>

                      <!-- Stats button -->
                      <button
                        v-if="c.state === 'running'"
                        @click="openStatsModal(c)"
                        class="p-1.5 text-slate-500 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="View Resource Metrics"
                      >
                        <Activity class="w-3.5 h-3.5" />
                      </button>

                      <!-- Delete button -->
                      <button
                        @click="confirmDeleteContainer(c)"
                        class="p-1.5 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                        title="Delete Container"
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

      <!-- ============================================================= -->
      <!-- TAB 2: IMAGES VIEW -->
      <!-- ============================================================= -->
      <div v-if="activeTab === 'images'" class="space-y-4">
        <div class="flex items-center justify-between">
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Locally stored Docker images available on this environment.
          </p>
          <button
            @click="showPullModal = true"
            class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition flex items-center gap-1.5 cursor-pointer"
          >
            <DownloadCloud class="w-3.5 h-3.5" />
            <span>Pull New Image</span>
          </button>
        </div>

        <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl overflow-hidden shadow-xs">
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-slate-50 dark:bg-[#121826] border-b border-slate-200 dark:border-[#1b2234] text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                <tr>
                  <th class="py-3 px-4">Repository & Tag</th>
                  <th class="py-3 px-4">Image ID</th>
                  <th class="py-3 px-4">Virtual Size</th>
                  <th class="py-3 px-4">Created Date</th>
                  <th class="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]">
                <tr v-if="images.length === 0" class="text-center">
                  <td colspan="5" class="py-12 text-slate-500">
                    <Layers class="w-8 h-8 text-slate-400 mx-auto mb-2 opacity-50" />
                    <span>No Docker images stored on this host.</span>
                  </td>
                </tr>

                <tr
                  v-for="img in images"
                  :key="img.id"
                  class="hover:bg-slate-50/70 dark:hover:bg-[#141b2c]/50 transition"
                >
                  <!-- Repo Tag -->
                  <td class="py-3 px-4">
                    <div class="font-bold text-slate-900 dark:text-white font-mono">
                      {{ img.repoTags && img.repoTags.length > 0 ? img.repoTags[0] : '<none>:<none>' }}
                    </div>
                    <div v-if="img.repoTags && img.repoTags.length > 1" class="text-[10px] text-slate-400">
                      +{{ img.repoTags.length - 1 }} additional tags
                    </div>
                  </td>

                  <!-- Short ID -->
                  <td class="py-3 px-4 font-mono text-[11px] text-slate-500 dark:text-slate-400">
                    {{ img.id.replace('sha256:', '').substring(0, 12) }}
                  </td>

                  <!-- Size -->
                  <td class="py-3 px-4 font-mono text-[11px] text-slate-700 dark:text-slate-300">
                    {{ formatBytes(img.size || (img.sizeMb ? Math.round(img.sizeMb * 1024 * 1024) : 0)) }}
                  </td>

                  <!-- Created -->
                  <td class="py-3 px-4 text-slate-500 dark:text-slate-400 text-[11px]">
                    {{ formatTimestamp(img.created) !== '-' ? formatTimestamp(img.created) : (img.createdStr || '-') }}
                  </td>

                  <!-- Actions -->
                  <td class="py-3 px-4 text-right">
                    <button
                      @click="confirmDeleteImage(img)"
                      class="p-1.5 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                      title="Delete Image"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ============================================================= -->
      <!-- TAB: NETWORKS VIEW -->
      <!-- ============================================================= -->
      <div v-if="activeTab === 'networks'" class="space-y-4">
        <div class="flex items-center justify-between">
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Docker virtual network bridges, overlays, and host drivers configured on this environment.
          </p>
          <button
            @click="showCreateNetModal = true"
            class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition flex items-center gap-1.5 cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Create Network</span>
          </button>
        </div>

        <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl overflow-hidden shadow-xs">
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-slate-50 dark:bg-[#121826] border-b border-slate-200 dark:border-[#1b2234] text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                <tr>
                  <th class="py-3 px-4">Network Name</th>
                  <th class="py-3 px-4">Network ID</th>
                  <th class="py-3 px-4">Driver</th>
                  <th class="py-3 px-4">Scope</th>
                  <th class="py-3 px-4">Subnet / Gateway</th>
                  <th class="py-3 px-4">Connected</th>
                  <th class="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]">
                <tr v-if="networks.length === 0" class="text-center">
                  <td colspan="7" class="py-12 text-slate-500">
                    <Network class="w-8 h-8 text-slate-400 mx-auto mb-2 opacity-50" />
                    <span>No Docker networks found on this host.</span>
                  </td>
                </tr>

                <tr
                  v-for="net in networks"
                  :key="net.id"
                  class="hover:bg-slate-50/70 dark:hover:bg-[#141b2c]/50 transition"
                >
                  <!-- Network Name -->
                  <td class="py-3 px-4">
                    <div class="font-bold text-slate-900 dark:text-white font-mono flex items-center gap-1.5">
                      <span>{{ net.name }}</span>
                      <span
                        v-if="net.name === 'bridge' || net.name === 'host' || net.name === 'none'"
                        class="text-[9px] font-semibold px-1.5 py-0.5 rounded bg-slate-100 dark:bg-[#1f283d] text-slate-500"
                      >
                        BUILT-IN
                      </span>
                    </div>
                  </td>

                  <!-- Short ID -->
                  <td class="py-3 px-4 font-mono text-[11px] text-slate-500 dark:text-slate-400">
                    <div class="flex items-center gap-1.5">
                      <span>{{ net.id.substring(0, 12) }}</span>
                      <button
                        @click="copyToClipboard(net.id, net.id)"
                        class="hover:text-slate-600 dark:hover:text-slate-200 transition cursor-pointer"
                        title="Copy Network ID"
                      >
                        <Check v-if="copiedId === net.id" class="w-3 h-3 text-emerald-500" />
                        <Copy v-else class="w-3 h-3" />
                      </button>
                    </div>
                  </td>

                  <!-- Driver -->
                  <td class="py-3 px-4 font-mono text-[11px]">
                    <span
                      :class="[
                        'px-2 py-0.5 rounded text-[10px] font-bold uppercase font-mono',
                        net.driver === 'bridge' ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20' :
                        net.driver === 'host' ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20' :
                        net.driver === 'overlay' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20' :
                        'bg-slate-100 dark:bg-[#182136] text-slate-600 dark:text-slate-400'
                      ]"
                    >
                      {{ net.driver }}
                    </span>
                  </td>

                  <!-- Scope -->
                  <td class="py-3 px-4 font-mono text-[11px] text-slate-500 dark:text-slate-400">
                    {{ net.scope || 'local' }}
                  </td>

                  <!-- Subnet / Gateway -->
                  <td class="py-3 px-4 font-mono text-[11px] text-slate-700 dark:text-slate-300">
                    <div v-if="net.subnet">
                      <div>{{ net.subnet }}</div>
                      <div v-if="net.gateway" class="text-[10px] text-slate-400">GW: {{ net.gateway }}</div>
                    </div>
                    <span v-else class="text-slate-400">-</span>
                  </td>

                  <!-- Connected Containers -->
                  <td class="py-3 px-4 text-[11px] text-slate-600 dark:text-slate-400">
                    <div v-if="net.containersCount > 0" class="flex items-center gap-1.5 font-mono">
                      <span class="font-bold text-slate-900 dark:text-white">{{ net.containersCount }}</span>
                      <span>connected</span>
                    </div>
                    <span v-else class="text-slate-400">0 connected</span>
                  </td>

                  <!-- Actions -->
                  <td class="py-3 px-4 text-right">
                    <button
                      v-if="net.name !== 'bridge' && net.name !== 'host' && net.name !== 'none'"
                      @click="confirmDeleteNetwork(net)"
                      class="p-1.5 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 rounded hover:bg-slate-100 dark:hover:bg-[#182136] transition cursor-pointer"
                      title="Delete Network"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                    <span v-else class="text-[10px] text-slate-400 italic px-1.5">System</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ============================================================= -->
      <!-- TAB 4: DEPLOY CONTAINER WIZARD -->
      <!-- ============================================================= -->
      <div v-if="activeTab === 'deploy'" class="max-w-3xl mx-auto bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl p-5 sm:p-6 space-y-6 shadow-xs">
        <div class="border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-sm font-bold text-slate-900 dark:text-white uppercase tracking-wider">Deploy New Container</h2>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            Configure container parameters, port forwards, and mount paths on {{ activeConnection?.name }}.
          </p>
        </div>

        <form @submit.prevent="handleDeploy" class="space-y-4 text-xs">
          <!-- Name & Image -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Container Name *</label>
              <input
                v-model="deployForm.name"
                required
                placeholder="e.g. production-nginx, my-redis"
                class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Docker Image *</label>
              <input
                v-model="deployForm.image"
                required
                placeholder="e.g. nginx:alpine, redis:7-alpine"
                class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <!-- Port Mappings -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider">Port Bindings</label>
              <button
                type="button"
                @click="addPortRow"
                class="text-[11px] text-blue-600 dark:text-[#95CCDD] font-bold hover:underline flex items-center gap-1 cursor-pointer"
              >
                <Plus class="w-3 h-3" />
                <span>Add Port</span>
              </button>
            </div>
            <div
              v-for="(p, idx) in deployForm.ports"
              :key="idx"
              class="flex items-center gap-2"
            >
              <input
                v-model="p.host"
                placeholder="Host Port (e.g. 8080)"
                class="w-1/3 bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
              <span class="text-slate-400">:</span>
              <input
                v-model="p.container"
                placeholder="Container Port (e.g. 80)"
                class="w-1/3 bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
              <select
                v-model="p.protocol"
                class="bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-2 py-1.5 text-slate-800 dark:text-slate-200"
              >
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
              <button
                type="button"
                @click="removePortRow(idx)"
                class="p-1.5 text-slate-400 hover:text-rose-500 cursor-pointer"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Environment Variables -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider">Environment Variables</label>
              <button
                type="button"
                @click="addEnvRow"
                class="text-[11px] text-blue-600 dark:text-[#95CCDD] font-bold hover:underline flex items-center gap-1 cursor-pointer"
              >
                <Plus class="w-3 h-3" />
                <span>Add Variable</span>
              </button>
            </div>
            <div
              v-for="(e, idx) in deployForm.env"
              :key="idx"
              class="flex items-center gap-2"
            >
              <input
                v-model="e.key"
                placeholder="KEY (e.g. APP_ENV)"
                class="w-1/2 bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
              <span class="text-slate-400">=</span>
              <input
                v-model="e.value"
                placeholder="VALUE (e.g. production)"
                class="w-1/2 bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
              <button
                type="button"
                @click="removeEnvRow(idx)"
                class="p-1.5 text-slate-400 hover:text-rose-500 cursor-pointer"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Volume Mounts -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider">Volume Mounts</label>
              <button
                type="button"
                @click="addVolumeRow"
                class="text-[11px] text-blue-600 dark:text-[#95CCDD] font-bold hover:underline flex items-center gap-1 cursor-pointer"
              >
                <Plus class="w-3 h-3" />
                <span>Add Volume</span>
              </button>
            </div>
            <div
              v-for="(v, idx) in deployForm.volumes"
              :key="idx"
              class="flex items-center gap-2"
            >
              <input
                v-model="v.host"
                placeholder="Host Path (/var/data)"
                class="w-1/2 bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
              <span class="text-slate-400">:</span>
              <input
                v-model="v.container"
                placeholder="Container Path (/app/data)"
                class="w-1/2 bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
              <button
                type="button"
                @click="removeVolumeRow(idx)"
                class="p-1.5 text-slate-400 hover:text-rose-500 cursor-pointer"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Advanced: Restart Policy & Network -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
            <div>
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Restart Policy</label>
              <select
                v-model="deployForm.restartPolicy"
                class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-800 dark:text-slate-200"
              >
                <option value="unless-stopped">Unless Stopped</option>
                <option value="always">Always</option>
                <option value="on-failure">On Failure</option>
                <option value="no">Never (No)</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Network Mode</label>
              <select
                v-model="deployForm.networkMode"
                class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-800 dark:text-slate-200"
              >
                <option value="bridge">bridge (Default NAT network)</option>
                <option value="host">host (Use host network stack)</option>
                <option value="none">none (No networking)</option>
                <option
                  v-for="net in networks.filter(n => n.name !== 'bridge' && n.name !== 'host' && n.name !== 'none')"
                  :key="net.id"
                  :value="net.name"
                >
                  {{ net.name }} ({{ net.driver }})
                </option>
              </select>
            </div>
          </div>

          <div class="pt-4 border-t border-slate-200 dark:border-[#1b2234] flex items-center justify-end gap-3">
            <button
              type="button"
              @click="activeTab = 'containers'"
              class="px-4 py-2 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white text-xs font-semibold cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="deploying"
              class="px-5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition flex items-center gap-2 cursor-pointer disabled:opacity-50"
            >
              <RefreshCw v-if="deploying" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ deploying ? 'Deploying Container...' : 'Deploy Container' }}</span>
            </button>
          </div>
        </form>
      </div>

      <!-- ============================================================= -->
      <!-- TAB 4: CONNECTIONS VIEW -->
      <!-- ============================================================= -->
      <div v-if="activeTab === 'connections'" class="space-y-4">
        <div class="flex items-center justify-between">
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Registered Docker instances (local sockets, remote SSH servers, TCP sockets).
          </p>
          <button
            @click="showAddConnModal = true"
            class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition flex items-center gap-1.5 cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Connection</span>
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <div
            v-for="conn in connections"
            :key="conn.id"
            :class="[
              'p-4 bg-white dark:bg-[#0e121c] border rounded-xl space-y-3 shadow-xs transition',
              selectedConnectionId === conn.id ? 'border-blue-500 ring-1 ring-blue-500/30' : 'border-slate-200 dark:border-[#1b2234]'
            ]"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Server class="w-4 h-4 text-slate-400" />
                <span class="font-bold text-slate-900 dark:text-white text-xs">{{ conn.name }}</span>
              </div>
              <span
                :class="[
                  'px-2 py-0.5 rounded text-[10px] font-bold uppercase font-mono',
                  conn.driver === 'socket' ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20' :
                  conn.driver === 'ssh' ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20' :
                  'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20'
                ]"
              >
                {{ conn.driver }}
              </span>
            </div>

            <div class="text-[11px] font-mono text-slate-500 dark:text-slate-400 space-y-0.5">
              <div v-if="conn.driver === 'socket'" class="truncate">
                Socket: {{ conn.socketPath || '/var/run/docker.sock' }}
              </div>
              <div v-else-if="conn.driver === 'ssh'" class="truncate">
                SSH: {{ conn.sshUser }}@{{ conn.sshHost }}:{{ conn.sshPort || 22 }}
              </div>
              <div v-else class="truncate">
                TCP: {{ conn.tcpHost }}:{{ conn.tcpPort || 2375 }}
              </div>
            </div>

            <div class="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-[#1b2234]">
              <button
                @click="selectedConnectionId = conn.id"
                :class="[
                  'px-3 py-1 rounded text-[11px] font-bold transition cursor-pointer',
                  selectedConnectionId === conn.id
                    ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400'
                    : 'text-slate-500 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#1a2336]'
                ]"
              >
                {{ selectedConnectionId === conn.id ? 'ACTIVE ENVIRONMENT' : 'SELECT' }}
              </button>

              <button
                v-if="conn.id !== 'docker-local-default'"
                @click="confirmDeleteConnection(conn)"
                class="p-1 text-slate-400 hover:text-rose-500 rounded transition cursor-pointer"
                title="Remove Connection"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>

    </main>

    <!-- ============================================================= -->
    <!-- LOGS VIEWER MODAL -->
    <!-- ============================================================= -->
    <div
      v-if="showLogsModal && activeContainer"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-2xl w-full max-w-4xl shadow-2xl flex flex-col max-h-[85vh] overflow-hidden">
        <!-- Modal Header -->
        <div class="p-4 border-b border-slate-200 dark:border-[#1b2234] flex items-center justify-between shrink-0">
          <div class="flex items-center gap-2">
            <Terminal class="w-4 h-4 text-slate-400" />
            <div>
              <h3 class="text-xs font-bold text-slate-900 dark:text-white font-mono">
                {{ getCleanContainerName(activeContainer) }} - Logs
              </h3>
              <p class="text-[10px] text-slate-500 font-mono">{{ activeContainer?.id ? activeContainer.id.substring(0, 12) : '-' }}</p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <select
              v-model.number="logsTail"
              @change="fetchLogs"
              class="bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1b2234] text-[11px] rounded px-2 py-1 text-slate-800 dark:text-slate-200"
            >
              <option :value="100">Last 100 lines</option>
              <option :value="250">Last 250 lines</option>
              <option :value="500">Last 500 lines</option>
              <option :value="1000">Last 1000 lines</option>
            </select>

            <label class="flex items-center gap-1.5 text-[11px] text-slate-600 dark:text-slate-400 cursor-pointer">
              <input type="checkbox" v-model="autoRefreshLogs" class="rounded text-blue-600 focus:ring-0" />
              <span>Auto-refresh (3s)</span>
            </label>

            <button
              @click="copyToClipboard(stripAnsi(containerLogs), 'logs')"
              class="p-1.5 text-slate-400 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1b2234] transition cursor-pointer"
              title="Copy Clean Logs"
            >
              <Check v-if="copiedId === 'logs'" class="w-4 h-4 text-emerald-500" />
              <Copy v-else class="w-4 h-4" />
            </button>

            <button
              @click="showLogsModal = false; autoRefreshLogs = false;"
              class="p-1.5 text-slate-400 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1b2234] transition cursor-pointer"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Terminal Log Body -->
        <div
          ref="logsTerminalRef"
          data-terminal="true"
          class="terminal-log-viewer flex-1 p-4 font-mono text-[11px] overflow-y-auto leading-relaxed whitespace-pre-wrap select-text selection:bg-blue-600 selection:text-white"
          style="background-color: #080b12 !important; color: #f1f5f9 !important;"
        >
          <div v-if="logsLoading && !containerLogs" class="text-slate-400 flex items-center gap-2">
            <RefreshCw class="w-3.5 h-3.5 animate-spin text-blue-400" />
            <span>Fetching log stream...</span>
          </div>
          <div v-else-if="!containerLogs" class="text-slate-500 italic">
            No log output recorded.
          </div>
          <div v-else v-html="renderedLogs" class="terminal-content"></div>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- STATS MODAL -->
    <!-- ============================================================= -->
    <div
      v-if="showStatsModal && activeContainer"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-2xl w-full max-w-lg shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <Activity class="w-4 h-4 text-slate-400" />
            <div>
              <h3 class="text-xs font-bold text-slate-900 dark:text-white font-mono">
                {{ getCleanContainerName(activeContainer) }} - Metrics
              </h3>
              <p class="text-[10px] text-slate-500">Real-time resource utilization</p>
            </div>
          </div>
          <button
            @click="showStatsModal = false; if (statsInterval) clearInterval(statsInterval);"
            class="p-1.5 text-slate-400 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1b2234] transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div v-if="statsLoading && !liveStats" class="py-8 text-center text-slate-500 text-xs">
          <RefreshCw class="w-5 h-5 animate-spin mx-auto mb-2 text-slate-400" />
          <span>Polling container metrics...</span>
        </div>

        <div v-else-if="liveStats" class="grid grid-cols-2 gap-3 text-xs">
          <!-- CPU -->
          <div class="p-3 bg-slate-50 dark:bg-[#141824] rounded-xl border border-slate-200 dark:border-[#1b2234] space-y-1">
            <span class="text-[10px] font-bold uppercase text-slate-400">CPU Usage</span>
            <div class="text-xl font-bold font-mono text-slate-900 dark:text-white">
              {{ (liveStats.cpuPercent ?? 0).toFixed(2) }}%
            </div>
          </div>

          <!-- Memory -->
          <div class="p-3 bg-slate-50 dark:bg-[#141824] rounded-xl border border-slate-200 dark:border-[#1b2234] space-y-1">
            <span class="text-[10px] font-bold uppercase text-slate-400">Memory Usage</span>
            <div class="text-xl font-bold font-mono text-slate-900 dark:text-white">
              {{ formatBytes(liveStats.memUsage || 0) }}
            </div>
            <div class="text-[10px] text-slate-500">
              of {{ formatBytes(liveStats.memLimit || 0) }} ({{ (liveStats.memPercent ?? 0).toFixed(1) }}%)
            </div>
          </div>

          <!-- Network -->
          <div class="p-3 bg-slate-50 dark:bg-[#141824] rounded-xl border border-slate-200 dark:border-[#1b2234] space-y-1">
            <span class="text-[10px] font-bold uppercase text-slate-400">Network I/O</span>
            <div class="font-mono text-xs text-slate-800 dark:text-slate-200">
              Rx: {{ formatBytes(liveStats.netRx || 0) }}
            </div>
            <div class="font-mono text-xs text-slate-800 dark:text-slate-200">
              Tx: {{ formatBytes(liveStats.netTx || 0) }}
            </div>
          </div>

          <!-- Block I/O & PIDs -->
          <div class="p-3 bg-slate-50 dark:bg-[#141824] rounded-xl border border-slate-200 dark:border-[#1b2234] space-y-1">
            <span class="text-[10px] font-bold uppercase text-slate-400">PIDs / Processes</span>
            <div class="text-xl font-bold font-mono text-slate-900 dark:text-white">
              {{ liveStats.pids ?? '-' }}
            </div>
            <div class="text-[10px] text-slate-500 font-mono">
              IO: {{ formatBytes(liveStats.blockRead || 0) }} / {{ formatBytes(liveStats.blockWrite || 0) }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- PULL IMAGE MODAL -->
    <!-- ============================================================= -->
    <div
      v-if="showPullModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-2xl w-full max-w-md shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <DownloadCloud class="w-4 h-4 text-slate-400" />
            <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
              Pull Docker Image
            </h3>
          </div>
          <button
            @click="showPullModal = false"
            class="p-1 text-slate-400 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1b2234] transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handlePullImage" class="space-y-3 text-xs">
          <div>
            <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Image Name & Tag</label>
            <input
              v-model="pullImageName"
              required
              placeholder="e.g. nginx:alpine, redis:latest, postgres:16"
              class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500 text-xs"
            />
          </div>

          <div class="flex items-center justify-end gap-2 pt-2">
            <button
              type="button"
              @click="showPullModal = false"
              class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="pulling"
              class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
            >
              <RefreshCw v-if="pulling" class="w-3 h-3 animate-spin" />
              <span>{{ pulling ? 'Pulling Image...' : 'Pull Image' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- ADD CONNECTION MODAL -->
    <!-- ============================================================= -->
    <div
      v-if="showAddConnModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-2xl w-full max-w-lg shadow-2xl p-5 space-y-4 max-h-[90vh] overflow-y-auto">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <Server class="w-4 h-4 text-slate-400" />
            <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
              Add Docker Connection
            </h3>
          </div>
          <button
            @click="showAddConnModal = false"
            class="p-1 text-slate-400 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1b2234] transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleSaveNewConn" class="space-y-3.5 text-xs">
          <div>
            <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Connection Name *</label>
            <input
              v-model="newConn.name"
              required
              placeholder="e.g. Remote Host VM, Cloud Docker Swarm"
              class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Driver Mode</label>
            <div class="grid grid-cols-3 gap-2">
              <button
                type="button"
                @click="newConn.driver = 'socket'"
                :class="[
                  'py-1.5 px-2 text-xs font-semibold rounded-lg border transition text-center cursor-pointer',
                  newConn.driver === 'socket'
                    ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-[#1b2234] text-slate-600 dark:text-slate-400'
                ]"
              >
                Unix Socket
              </button>
              <button
                type="button"
                @click="newConn.driver = 'ssh'"
                :class="[
                  'py-1.5 px-2 text-xs font-semibold rounded-lg border transition text-center cursor-pointer',
                  newConn.driver === 'ssh'
                    ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-[#1b2234] text-slate-600 dark:text-slate-400'
                ]"
              >
                Remote SSH
              </button>
              <button
                type="button"
                @click="newConn.driver = 'tcp'"
                :class="[
                  'py-1.5 px-2 text-xs font-semibold rounded-lg border transition text-center cursor-pointer',
                  newConn.driver === 'tcp'
                    ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-[#1b2234] text-slate-600 dark:text-slate-400'
                ]"
              >
                TCP Socket
              </button>
            </div>
          </div>

          <!-- Socket Fields -->
          <div v-if="newConn.driver === 'socket'">
            <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Socket Path</label>
            <input
              v-model="newConn.socketPath"
              placeholder="/var/run/docker.sock"
              class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- SSH Fields -->
          <div v-if="newConn.driver === 'ssh'" class="space-y-2 p-3 bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1b2234] rounded-lg">
            <div class="grid grid-cols-3 gap-2">
              <div class="col-span-2">
                <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">SSH Host IP</label>
                <input v-model="newConn.sshHost" placeholder="10.20.3.10" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
              </div>
              <div>
                <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">Port</label>
                <input v-model.number="newConn.sshPort" type="number" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div>
                <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">SSH User</label>
                <input v-model="newConn.sshUser" placeholder="root" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
              </div>
              <div>
                <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">Password / Key</label>
                <input v-model="newConn.sshPassword" type="password" placeholder="••••••" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
              </div>
            </div>
          </div>

          <!-- TCP Fields -->
          <div v-if="newConn.driver === 'tcp'" class="space-y-2 p-3 bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1b2234] rounded-lg">
            <div class="grid grid-cols-3 gap-2">
              <div class="col-span-2">
                <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">TCP Host</label>
                <input v-model="newConn.tcpHost" placeholder="10.20.3.15" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
              </div>
              <div>
                <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">Port</label>
                <input v-model.number="newConn.tcpPort" type="number" placeholder="2375" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
              </div>
            </div>
          </div>

          <!-- Test Result Banner -->
          <div
            v-if="testConnResult"
            :class="[
              'p-2.5 rounded-lg border text-xs font-mono',
              testConnResult.success ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-600 dark:text-rose-400'
            ]"
          >
            {{ testConnResult.message }}
          </div>

          <!-- Actions -->
          <div class="grid grid-cols-2 gap-2 pt-2">
            <button
              type="button"
              @click="handleTestNewConn"
              :disabled="testingConn"
              class="px-3 py-2 bg-slate-100 hover:bg-slate-200 dark:bg-[#141824] dark:hover:bg-[#1b2234] text-slate-800 dark:text-slate-200 rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
            >
              {{ testingConn ? 'Testing...' : 'Test Connection' }}
            </button>
            <button
              type="submit"
              :disabled="savingConn"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
            >
              {{ savingConn ? 'Saving...' : 'Save Connection' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- CREATE NETWORK MODAL -->
    <!-- ============================================================= -->
    <div
      v-if="showCreateNetModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-2xl w-full max-w-md shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <div class="flex items-center gap-2">
            <Network class="w-4 h-4 text-slate-400" />
            <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
              Create Docker Network
            </h3>
          </div>
          <button
            @click="showCreateNetModal = false"
            class="p-1 text-slate-400 hover:text-slate-900 dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#1b2234] transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleCreateNetwork" class="space-y-3.5 text-xs">
          <div>
            <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Network Name *</label>
            <input
              v-model="newNetForm.name"
              required
              placeholder="e.g. app-network, custom-bridge"
              class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Driver</label>
            <select
              v-model="newNetForm.driver"
              class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-800 dark:text-slate-200 font-mono"
            >
              <option value="bridge">bridge (Standard virtual switch)</option>
              <option value="overlay">overlay (Multi-host swarm routing)</option>
              <option value="macvlan">macvlan (Direct physical MAC interface)</option>
              <option value="ipvlan">ipvlan (Direct physical IP interface)</option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Subnet (CIDR)</label>
              <input
                v-model="newNetForm.subnet"
                placeholder="172.28.0.0/16"
                class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Gateway</label>
              <input
                v-model="newNetForm.gateway"
                placeholder="172.28.0.1"
                class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div class="space-y-2 pt-1">
            <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-700 dark:text-slate-300">
              <input type="checkbox" v-model="newNetForm.internal" class="rounded text-blue-600 focus:ring-0 w-3.5 h-3.5 cursor-pointer" />
              <span>Internal Network (Restrict external internet access)</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-700 dark:text-slate-300">
              <input type="checkbox" v-model="newNetForm.enableIPv6" class="rounded text-blue-600 focus:ring-0 w-3.5 h-3.5 cursor-pointer" />
              <span>Enable IPv6 Networking</span>
            </label>
          </div>

          <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-200 dark:border-[#1b2234]">
            <button
              type="button"
              @click="showCreateNetModal = false"
              class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="creatingNet"
              class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <RefreshCw v-if="creatingNet" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ creatingNet ? 'Creating...' : 'Create Network' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- STANDARD DELETE CONFIRMATION MODAL (Strict AGENTS.md compliance) -->
    <!-- ============================================================= -->
    <div
      v-if="showDeleteModal && deleteTarget"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Red circle trash icon -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            Delete {{ deleteTarget.type === 'container' ? 'Container' : deleteTarget.type === 'image' ? 'Image' : deleteTarget.type === 'network' ? 'Network' : 'Connection' }}?
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ deleteTarget.name }}</strong>? This action cannot be undone.
          </p>
        </div>

        <!-- Optional Force Delete for containers/images -->
        <div v-if="deleteTarget.type === 'container' || deleteTarget.type === 'image'" class="text-left pt-1">
          <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-600 dark:text-slate-400">
            <input type="checkbox" v-model="forceDelete" class="rounded text-rose-600 focus:ring-0 w-3.5 h-3.5 cursor-pointer" />
            <span>Force remove (stop running container if active)</span>
          </label>
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

<style scoped>
.terminal-log-viewer,
.terminal-content {
  color: #f1f5f9 !important;
  background-color: #080b12 !important;
}
:deep(.terminal-content span) {
  display: inline;
}
</style>
