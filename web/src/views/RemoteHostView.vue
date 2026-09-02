<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';
import {
  Server,
  Folder,
  Upload,
  Download,
  Plus,
  X,
  RotateCw,
  Trash2,
  Radio,
  Maximize2,
  ArrowLeft,
  Search,
  CheckCircle2,
  Activity,
  Sliders,
  Settings,
  LayoutGrid,
  SquareTerminal,
  Wifi,
  HardDrive,
  Cpu,
  Layers,
  Play,
  Square,
  AlertTriangle,
  PlayCircle,
  StopCircle,
  FolderPlus,
  Lock,
  Copy,
  ChevronRight,
  FileCode,
  FileText,
  FileArchive,
  File,
  CornerLeftUp,
  Terminal as TerminalIcon,
  Shield,
  ArrowUpRight,
  ArrowRight,
  Monitor,
  Cloud,
  Check,
  RefreshCw,
} from 'lucide-vue-next';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

interface RemoteHost {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  authType: string;
  groupName: string;
  tags: string[];
}

interface OpenSession {
  id: string;
  host: RemoteHost;
  displayName?: string;
  activeView: 'terminal' | 'dashboard' | 'processes' | 'services' | 'network' | 'sftp';
  term?: Terminal;
  fitAddon?: FitAddon;
  ws?: WebSocket;
  connected: boolean;
  heartbeatTimer?: any;
  metrics?: any;
  processes?: any[];
  services?: any[];
  networkInfo?: {
    interfaces?: any[];
    listeningPorts?: any[];
    connections?: any[];
  };
}

const hosts = ref<RemoteHost[]>([]);
const openSessions = ref<OpenSession[]>([]);
const activeSessionIndex = ref<number>(-1); // -1 means Server List view
const isHostModalOpen = ref(false);
const isGroupModalOpen = ref(false);
const isSftpModalOpen = ref(false);
const searchHostQuery = ref('');
const selectedGroupFilter = ref<string | null>(null);
const processSearch = ref('');
const processSort = ref<'cpu' | 'mem' | 'pid'>('cpu');
const serviceSearch = ref('');
const isTelemetryLoading = ref(false);
const portProtoFilter = ref<'all' | 'tcp' | 'udp'>('all');
const portSearch = ref('');
const interfaceSearch = ref('');

// New Group Form
const newGroupName = ref('');

// New Host Form
const hostForm = ref<any>({
  id: '',
  name: '',
  host: '',
  port: 22,
  username: '',
  authType: 'password',
  password: '',
  sshKey: '',
  groupName: 'Default',
  tags: [],
});

// =================================================================
// SFTP FILEZILLA-STYLE DUAL-PANE (SOURCE & DESTINATION) STATES
// =================================================================
interface LocalStagedFile {
  id: string;
  file: File;
  name: string;
  size: number;
  type: string;
  status: 'ready' | 'uploading' | 'completed' | 'failed';
}

const localStagedFiles = ref<LocalStagedFile[]>([]);
const sftpFiles = ref<any[]>([]);
const sftpCurrentPath = ref('/');
const sftpInputPath = ref('/');
const sftpLoading = ref(false);
const selectedSftpHostId = ref<string>('');
const sftpFileInput = ref<HTMLInputElement | null>(null);
const sftpUploadProgress = ref(false);
const sftpError = ref('');
const sftpFileFilter = ref('');
const sftpCommandLogs = ref<Array<{ time: string; type: 'status' | 'command' | 'response' | 'error'; text: string }>>([]);
const sftpTransferQueue = ref<Array<{ name: string; size: string; status: 'queued' | 'transferring' | 'success' | 'failed'; progress: number }>>([]);
const isDragOverLocal = ref(false);
const isDragOverRemote = ref(false);
const sftpQueueTab = ref<'queued' | 'success' | 'failed'>('queued');

const activeSession = computed(() => {
  if (activeSessionIndex.value >= 0 && activeSessionIndex.value < openSessions.value.length) {
    return openSessions.value[activeSessionIndex.value];
  }
  return null;
});

// Groups computed
const groupedHosts = computed(() => {
  const groups: Record<string, RemoteHost[]> = {};
  hosts.value.forEach((h) => {
    const g = h.groupName || 'Default';
    if (!groups[g]) groups[g] = [];
    groups[g].push(h);
  });
  return groups;
});

const existingGroupNames = computed(() => {
  const set = new Set<string>();
  hosts.value.forEach((h) => {
    if (h.groupName) set.add(h.groupName);
  });
  if (set.size === 0) set.add('Default');
  return Array.from(set);
});

// Filtered hosts
const filteredHosts = computed(() => {
  let list = hosts.value;
  if (selectedGroupFilter.value) {
    list = list.filter((h) => h.groupName === selectedGroupFilter.value);
  }
  if (!searchHostQuery.value) return list;
  const q = searchHostQuery.value.toLowerCase();
  return list.filter(
    (h) => h.name.toLowerCase().includes(q) || h.host.toLowerCase().includes(q) || h.username.toLowerCase().includes(q)
  );
});

// Filtered Processes
const filteredProcesses = computed(() => {
  if (!activeSession.value?.processes) return [];
  let list = [...activeSession.value.processes];
  if (processSearch.value) {
    const q = processSearch.value.toLowerCase();
    list = list.filter((p) => p.command.toLowerCase().includes(q) || p.user.toLowerCase().includes(q));
  }
  if (processSort.value === 'cpu') {
    list.sort((a, b) => b.cpu - a.cpu);
  } else if (processSort.value === 'mem') {
    list.sort((a, b) => b.mem - a.mem);
  } else if (processSort.value === 'pid') {
    list.sort((a, b) => a.pid - b.pid);
  }
  return list;
});

// Filtered Services
const filteredServices = computed(() => {
  if (!activeSession.value?.services) return [];
  if (!serviceSearch.value) return activeSession.value.services;
  const q = serviceSearch.value.toLowerCase();
  return activeSession.value.services.filter(
    (s) => s.name.toLowerCase().includes(q) || (s.description && s.description.toLowerCase().includes(q))
  );
});

// Filtered Listening Ports
const filteredListeningPorts = computed(() => {
  if (!activeSession.value?.networkInfo?.listeningPorts) return [];
  let list = [...activeSession.value.networkInfo.listeningPorts];
  if (portProtoFilter.value !== 'all') {
    const p = portProtoFilter.value.toUpperCase();
    list = list.filter((item) => item.proto && item.proto.toUpperCase().includes(p));
  }
  if (portSearch.value) {
    const q = portSearch.value.toLowerCase();
    list = list.filter(
      (item) =>
        (item.localAddr && item.localAddr.toLowerCase().includes(q)) ||
        (item.port && item.port.toString().toLowerCase().includes(q)) ||
        (item.process && item.process.toLowerCase().includes(q)) ||
        (item.pid && item.pid.toString().toLowerCase().includes(q))
    );
  }
  return list;
});

// Filtered Network Interfaces
const filteredInterfaces = computed(() => {
  if (!activeSession.value?.networkInfo?.interfaces) return [];
  if (!interfaceSearch.value) return activeSession.value.networkInfo.interfaces;
  const q = interfaceSearch.value.toLowerCase();
  return activeSession.value.networkInfo.interfaces.filter(
    (i: any) =>
      (i.name && i.name.toLowerCase().includes(q)) ||
      (i.ipv4 && i.ipv4.toLowerCase().includes(q)) ||
      (i.ipv6 && i.ipv6.toLowerCase().includes(q)) ||
      (i.mac && i.mac.toLowerCase().includes(q))
  );
});

// Session State Persistence Helpers
const saveSessionsState = () => {
  try {
    const state = {
      sessions: openSessions.value.map((s) => ({
        hostId: s.host.id,
        displayName: s.displayName,
        activeView: s.activeView || 'terminal',
      })),
      activeSessionIndex: activeSessionIndex.value,
    };
    localStorage.setItem('hcp_remote_sessions_state', JSON.stringify(state));
  } catch (e) {}
};

const restorePersistedSessions = async () => {
  const queryHostId = route.query.hostId as string;
  if (queryHostId) {
    const target = hosts.value.find((h) => h.id === queryHostId);
    if (target) {
      await connectHost(target);
      return;
    }
  }

  const saved = localStorage.getItem('hcp_remote_sessions_state');
  if (!saved) return;
  try {
    const parsed = JSON.parse(saved);
    if (!parsed || !Array.isArray(parsed.sessions) || parsed.sessions.length === 0) return;

    for (const sInfo of parsed.sessions) {
      const host = hosts.value.find((h) => h.id === sInfo.hostId);
      if (host) {
        const session: OpenSession = {
          id: `session-${Date.now()}-${Math.random().toString(36).substring(2, 7)}`,
          host,
          displayName: sInfo.displayName || host.name,
          activeView: sInfo.activeView || 'terminal',
          connected: false,
        };
        openSessions.value.push(session);
      }
    }

    if (openSessions.value.length > 0) {
      let targetIdx = 0;
      if (
        typeof parsed.activeSessionIndex === 'number' &&
        parsed.activeSessionIndex >= 0 &&
        parsed.activeSessionIndex < openSessions.value.length
      ) {
        targetIdx = parsed.activeSessionIndex;
      }
      activeSessionIndex.value = targetIdx;

      await nextTick();
      for (const session of openSessions.value) {
        initXterm(session);
        fetchHostTelemetry(session);
      }
    }
  } catch (e) {
    console.error('Failed to restore sessions:', e);
  }
};

// Fetch Hosts List
const fetchHosts = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data.success && res.data.data) {
      hosts.value = res.data.data;
      await restorePersistedSessions();
    } else {
      hosts.value = [];
    }
  } catch (err) {
    console.error('Failed to load remote hosts:', err);
    hosts.value = [];
  }
};

// =================================================================
// MULTI-SESSION & DUPLICATE TABS SUPPORT
// =================================================================
const connectHost = async (host: RemoteHost, forceNew = false) => {
  if (!forceNew) {
    const existingIdx = openSessions.value.findIndex((s) => s.host.id === host.id);
    if (existingIdx >= 0) {
      activeSessionIndex.value = existingIdx;
      saveSessionsState();
      await ensureTerminalReady(openSessions.value[existingIdx]);
      return;
    }
  }

  // Calculate instance count for tab label
  const count = openSessions.value.filter((s) => s.host.id === host.id).length;
  const tabName = count > 0 ? `${host.name} #${count + 1}` : host.name;

  const session: OpenSession = {
    id: `session-${Date.now()}-${Math.random().toString(36).substring(2, 7)}`,
    host,
    displayName: tabName,
    activeView: 'terminal',
    connected: false,
  };

  openSessions.value.push(session);
  activeSessionIndex.value = openSessions.value.length - 1;
  saveSessionsState();

  await ensureTerminalReady(session);
  fetchHostTelemetry(session);
};

const duplicateSession = async (session: OpenSession, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  await connectHost(session.host, true);
};

const ensureTerminalReady = async (session: OpenSession) => {
  await nextTick();
  let container = document.getElementById(`terminal-container-${session.id}`);
  if (!container) {
    for (let i = 0; i < 6; i++) {
      await new Promise((r) => setTimeout(r, 60));
      container = document.getElementById(`terminal-container-${session.id}`);
      if (container) break;
    }
  }

  if (!container) return;

  if (!session.term) {
    initXterm(session);
  } else {
    setTimeout(() => {
      try {
        session.fitAddon?.fit();
        session.term?.focus();
      } catch (e) {}
    }, 50);
  }
};

// Close Session
const closeSession = (idx: number, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  const s = openSessions.value[idx];
  if (s) {
    if (s.heartbeatTimer) clearInterval(s.heartbeatTimer);
    if (s.ws) {
      try {
        s.ws.close();
      } catch (e) {}
    }
    if (s.term) {
      try {
        s.term.dispose();
      } catch (e) {}
    }
  }
  openSessions.value.splice(idx, 1);
  if (activeSessionIndex.value >= openSessions.value.length) {
    activeSessionIndex.value = openSessions.value.length - 1;
  }
  saveSessionsState();
};

// Reconnect Terminal
const reconnectTerminal = (session: OpenSession) => {
  if (session.heartbeatTimer) clearInterval(session.heartbeatTimer);
  if (session.ws) {
    try {
      session.ws.close();
    } catch (e) {}
  }
  initXterm(session);
};

// Initialize xterm.js Terminal with WebSocket & Heartbeats
const initXterm = (session: OpenSession) => {
  const container = document.getElementById(`terminal-container-${session.id}`);
  if (!container) return;
  container.innerHTML = '';

  if (session.heartbeatTimer) clearInterval(session.heartbeatTimer);

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 13,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: {
      background: '#090d16',
      foreground: '#e2e8f0',
      cursor: '#38bdf8',
      selectionBackground: '#1e293b',
      black: '#000000',
      red: '#ef4444',
      green: '#22c55e',
      yellow: '#eab308',
      blue: '#3b82f6',
      magenta: '#a855f7',
      cyan: '#06b6d4',
      white: '#ffffff',
      brightBlack: '#64748b',
      brightRed: '#f87171',
      brightGreen: '#4ade80',
      brightYellow: '#fde047',
      brightBlue: '#60a5fa',
      brightMagenta: '#c084fc',
      brightCyan: '#22d3ee',
      brightWhite: '#ffffff',
    },
  });

  const fitAddon = new FitAddon();
  term.loadAddon(fitAddon);
  term.loadAddon(new WebLinksAddon());
  term.open(container);

  setTimeout(() => {
    try {
      fitAddon.fit();
      term.focus();
    } catch (e) {}
  }, 50);

  session.term = term;
  session.fitAddon = fitAddon;

  // Open WebSocket
  const token = authStore.token || localStorage.getItem('hcp_token') || '';
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/remote-host?token=${encodeURIComponent(token)}&hostId=${session.host.id}&cols=${term.cols || 80}&rows=${term.rows || 24}`;
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    session.connected = true;
    term.write('\r\n\x1b[32m[Connected to ' + session.host.name + ' (' + session.host.host + ')]\x1b[0m\r\n\r\n');
    ws.send(
      JSON.stringify({
        type: 'auth',
        token: token,
        hostConfigId: session.host.id,
        cols: term.cols,
        rows: term.rows,
      })
    );

    session.heartbeatTimer = setInterval(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, 15000);
  };

  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      if (msg.type === 'data' && msg.data) {
        term.write(msg.data);
      } else if (msg.type === 'connected') {
        session.connected = true;
      } else if (msg.type === 'error') {
        term.write(`\r\n\x1b[31m[Error: ${msg.message}]\x1b[0m\r\n`);
      } else if (msg.type === 'disconnected') {
        session.connected = false;
        if (session.heartbeatTimer) clearInterval(session.heartbeatTimer);
        term.write('\r\n\x1b[31m[Session closed]\x1b[0m\r\n');
      }
    } catch (e) {
      term.write(ev.data);
    }
  };

  ws.onclose = () => {
    session.connected = false;
    if (session.heartbeatTimer) clearInterval(session.heartbeatTimer);
    term.write('\r\n\x1b[31m[Session closed]\x1b[0m\r\n');
  };

  ws.onerror = () => {
    session.connected = false;
    if (session.heartbeatTimer) clearInterval(session.heartbeatTimer);
    term.write('\r\n\x1b[31m[WebSocket connection error]\x1b[0m\r\n');
  };

  term.onData((data) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }));
    }
  });

  term.onResize(({ cols, rows }) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols, rows }));
    }
  });

  session.ws = ws;
};

// Watch activeSession changes to ensure fitted terminal
watch(
  [activeSessionIndex, () => activeSession.value?.activeView],
  async () => {
    if (activeSession.value && activeSession.value.activeView === 'terminal') {
      await ensureTerminalReady(activeSession.value);
    }
  }
);

// Tab Groups: Optional user toggle (disabled by default)
const isGroupTabsEnabled = ref(false);
const splitLayout = ref<'single' | 'vertical' | 'horizontal'>('single');

const groupedSessions = computed(() => {
  const groups: {
    hostId: string;
    hostName: string;
    sessions: { session: OpenSession; globalIndex: number }[];
  }[] = [];

  openSessions.value.forEach((session, idx) => {
    let grp = groups.find((g) => g.hostId === session.host.id);
    if (!grp) {
      grp = {
        hostId: session.host.id,
        hostName: session.host.name,
        sessions: [],
      };
      groups.push(grp);
    }
    grp.sessions.push({ session, globalIndex: idx });
  });

  return groups;
});

const switchActiveView = async (session: OpenSession, viewName: 'terminal' | 'dashboard' | 'processes' | 'services' | 'network' | 'sftp') => {
  if (viewName === 'sftp') {
    openSftpModal(session.host.id);
    return;
  }
  session.activeView = viewName;
  if (viewName === 'terminal') {
    await ensureTerminalReady(session);
  } else {
    await fetchHostTelemetry(session);
  }
};

// Fetch Host Telemetry
const fetchHostTelemetry = async (session: OpenSession) => {
  if (!session || !session.host?.id) return;
  isTelemetryLoading.value = true;
  try {
    const [metricsRes, procRes, srvRes, netRes] = await Promise.all([
      axios.get(`/api/v1/remote-host/${session.host.id}/metrics`).catch(() => ({ data: { success: false } })),
      axios.get(`/api/v1/remote-host/${session.host.id}/processes`).catch(() => ({ data: { success: false } })),
      axios.get(`/api/v1/remote-host/${session.host.id}/services`).catch(() => ({ data: { success: false } })),
      axios.get(`/api/v1/remote-host/${session.host.id}/network`).catch(() => ({ data: { success: false } })),
    ]);

    if (metricsRes.data?.success && metricsRes.data.data) session.metrics = metricsRes.data.data;
    if (procRes.data?.success && Array.isArray(procRes.data.data)) session.processes = procRes.data.data;
    if (srvRes.data?.success && Array.isArray(srvRes.data.data)) session.services = srvRes.data.data;
    if (netRes.data?.success && netRes.data.data) session.networkInfo = netRes.data.data;
  } catch (err) {
    console.warn('Telemetry poll error:', err);
  } finally {
    isTelemetryLoading.value = false;
  }
};

const handleKillProcess = async (pid: number) => {
  if (!activeSession.value) return;
  if (!confirm(`Are you sure you want to terminate process PID ${pid}?`)) return;
  try {
    await axios.delete(`/api/v1/remote-host/${activeSession.value.host.id}/processes/${pid}`);
    await fetchHostTelemetry(activeSession.value);
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to kill process');
  }
};

const handleControlService = async (serviceName: string, action: string) => {
  if (!activeSession.value) return;
  try {
    await axios.post(`/api/v1/remote-host/${activeSession.value.host.id}/services/control`, {
      serviceName,
      action,
    });
    await fetchHostTelemetry(activeSession.value);
  } catch (err: any) {
    alert(err.response?.data?.error || `Failed to ${action} service`);
  }
};

// =================================================================
// SFTP DUAL-PANE SERVER-TO-SERVER & SERVER-TO-LOCAL TRANSFER ENGINE
// =================================================================
const sftpSourceHostId = ref<string>('local');
const sftpDestHostId = ref<string>('');

const sftpSourceCurrentPath = ref<string>('/');
const sftpSourceInputPath = ref<string>('/');
const sftpSourceFiles = ref<any[]>([]);
const sftpSourceLoading = ref<boolean>(false);
const sftpSourceFilter = ref<string>('');

const sftpDestCurrentPath = ref<string>('/');
const sftpDestInputPath = ref<string>('/');
const sftpDestFiles = ref<any[]>([]);
const sftpDestLoading = ref<boolean>(false);
const sftpDestFilter = ref<string>('');

const isDragOverLocal = ref(false);
const sftpFileInput = ref<HTMLInputElement | null>(null);
const localStagedFiles = ref<LocalStagedFile[]>([]);
const sftpCommandLogs = ref<{ time: string; type: 'status' | 'command' | 'response' | 'error'; text: string }[]>([]);
const sftpTransferQueue = ref<any[]>([]);

const sourceHostObj = computed(() => {
  if (sftpSourceHostId.value === 'local') return null;
  return hosts.value.find((h) => h.id === sftpSourceHostId.value) || null;
});

const destHostObj = computed(() => {
  if (sftpDestHostId.value === 'local') return null;
  return hosts.value.find((h) => h.id === sftpDestHostId.value) || null;
});

const logSftp = (type: 'status' | 'command' | 'response' | 'error', text: string) => {
  const time = new Date().toLocaleTimeString();
  sftpCommandLogs.value.unshift({ time, type, text });
  if (sftpCommandLogs.value.length > 50) sftpCommandLogs.value.pop();
};

const openSftpModal = (hostId?: string) => {
  if (hosts.value.length > 0) {
    if (hostId) {
      sftpDestHostId.value = hostId;
    } else if (activeSession.value) {
      sftpDestHostId.value = activeSession.value.host.id;
    } else {
      sftpDestHostId.value = hosts.value[0].id;
    }

    // If source is not local and matches destination, make source another VM or local
    if (hosts.value.length > 1 && sftpSourceHostId.value === sftpDestHostId.value) {
      const otherHost = hosts.value.find((h) => h.id !== sftpDestHostId.value);
      if (otherHost) sftpSourceHostId.value = otherHost.id;
    }
  }

  sftpSourceCurrentPath.value = '/';
  sftpSourceInputPath.value = '/';
  sftpDestCurrentPath.value = '/';
  sftpDestInputPath.value = '/';
  sftpCommandLogs.value = [];
  isSftpModalOpen.value = true;

  logSftp('status', 'SFTP Transfer Session initialized.');
  if (sftpSourceHostId.value !== 'local') fetchSourceSftpFiles('/');
  if (sftpDestHostId.value && sftpDestHostId.value !== 'local') fetchDestSftpFiles('/');
};

// Fetch files for Source pane
const fetchSourceSftpFiles = async (path = sftpSourceCurrentPath.value) => {
  if (sftpSourceHostId.value === 'local') return;
  const host = sourceHostObj.value;
  if (!host) return;

  sftpSourceLoading.value = true;
  logSftp('command', `[Source: ${host.name}] CWD "${path || '/'}"`);

  try {
    const res = await axios.get(`/api/v1/remote-host/${host.id}/sftp/list`, {
      params: { path: path || '/' },
    });
    if (res.data.success && res.data.data) {
      sftpSourceFiles.value = res.data.data;
      sftpSourceCurrentPath.value = path || '/';
      sftpSourceInputPath.value = sftpSourceCurrentPath.value;
      logSftp('response', `[Source: ${host.name}] Directory listing of "${sftpSourceCurrentPath.value}" OK (${sftpSourceFiles.value.length} items).`);
    } else {
      sftpSourceFiles.value = [];
    }
  } catch (err: any) {
    logSftp('error', `[Source: ${host.name}] Failed: ${err.response?.data?.error || err.message}`);
    sftpSourceFiles.value = [];
  } finally {
    sftpSourceLoading.value = false;
  }
};

// Fetch files for Destination pane
const fetchDestSftpFiles = async (path = sftpDestCurrentPath.value) => {
  if (sftpDestHostId.value === 'local') return;
  const host = destHostObj.value;
  if (!host) return;

  sftpDestLoading.value = true;
  logSftp('command', `[Destination: ${host.name}] CWD "${path || '/'}"`);

  try {
    const res = await axios.get(`/api/v1/remote-host/${host.id}/sftp/list`, {
      params: { path: path || '/' },
    });
    if (res.data.success && res.data.data) {
      sftpDestFiles.value = res.data.data;
      sftpDestCurrentPath.value = path || '/';
      sftpDestInputPath.value = sftpDestCurrentPath.value;
      logSftp('response', `[Destination: ${host.name}] Directory listing of "${sftpDestCurrentPath.value}" OK (${sftpDestFiles.value.length} items).`);
    } else {
      sftpDestFiles.value = [];
    }
  } catch (err: any) {
    logSftp('error', `[Destination: ${host.name}] Failed: ${err.response?.data?.error || err.message}`);
    sftpDestFiles.value = [];
  } finally {
    sftpDestLoading.value = false;
  }
};

// Swap Source and Destination hosts & paths
const swapSourceAndDest = () => {
  const tempHostId = sftpSourceHostId.value;
  sftpSourceHostId.value = sftpDestHostId.value;
  sftpDestHostId.value = tempHostId;

  const tempPath = sftpSourceCurrentPath.value;
  sftpSourceCurrentPath.value = sftpDestCurrentPath.value;
  sftpDestCurrentPath.value = tempPath;

  logSftp('status', 'Swapped Source and Destination endpoints.');
  if (sftpSourceHostId.value !== 'local') fetchSourceSftpFiles();
  if (sftpDestHostId.value !== 'local') fetchDestSftpFiles();
};

// Server-to-Server or Remote-to-Local transfer execution
const transferSourceFileToDest = async (fileName: string) => {
  const srcHost = sourceHostObj.value;
  const dstHost = destHostObj.value;

  const srcPath = sftpSourceCurrentPath.value === '/' ? `/${fileName}` : `${sftpSourceCurrentPath.value.replace(/\/+$/, '')}/${fileName}`;
  const dstPath = sftpDestCurrentPath.value === '/' ? `/${fileName}` : `${sftpDestCurrentPath.value.replace(/\/+$/, '')}/${fileName}`;

  // Case A: Source is Remote VM and Destination is Local Computer (Download)
  if (srcHost && sftpDestHostId.value === 'local') {
    logSftp('command', `RETR "${srcPath}" from [${srcHost.name}] (Download to Local Computer)`);
    const queueItem = {
      name: `${fileName} ([${srcHost.name}] ➔ Local)`,
      size: '-',
      status: 'success' as const,
      progress: 100,
    };
    sftpTransferQueue.value.unshift(queueItem);
    const url = `/api/v1/remote-host/${srcHost.id}/sftp/download?path=${encodeURIComponent(srcPath)}`;
    window.open(url, '_blank');
    return;
  }

  // Case B: Source is Remote VM (VM A) and Destination is Remote VM (VM B) (Server-to-Server FXP Transfer!)
  if (srcHost && dstHost) {
    logSftp('command', `FXP "${srcPath}" [${srcHost.name}] ➔ "${dstPath}" [${dstHost.name}]`);
    const queueItem = {
      name: `${fileName} ([${srcHost.name}] ➔ [${dstHost.name}])`,
      size: 'Transferring...',
      status: 'transferring' as const,
      progress: 50,
    };
    sftpTransferQueue.value.unshift(queueItem);

    try {
      const res = await axios.post('/api/v1/remote-host/sftp/transfer-remote', {
        srcHostId: srcHost.id,
        srcPath: srcPath,
        dstHostId: dstHost.id,
        dstPath: dstPath,
      });

      if (res.data.success) {
        queueItem.status = 'success';
        queueItem.progress = 100;
        queueItem.size = 'Completed';
        logSftp('response', `FXP Transfer of "${fileName}" from [${srcHost.name}] to [${dstHost.name}] completed successfully.`);
        await fetchDestSftpFiles();
      }
    } catch (err: any) {
      queueItem.status = 'failed';
      const msg = err.response?.data?.error || 'Server-to-Server transfer failed';
      logSftp('error', `Transfer failed: ${msg}`);
    }
  }
};

// Local Staging Upload to Destination
const uploadStagedFile = async (staged: LocalStagedFile) => {
  const dstHost = destHostObj.value;
  if (!dstHost) {
    alert('Please select a remote server destination for uploads.');
    return;
  }

  let targetPath = sftpDestCurrentPath.value.replace(/\/+$/, '');
  if (!targetPath) targetPath = '/';
  if (targetPath === '/') {
    targetPath = '/' + staged.name;
  } else {
    targetPath = `${targetPath}/${staged.name}`;
  }

  staged.status = 'uploading';
  const queueItem = {
    name: `${staged.name} (Local ➔ [${dstHost.name}]:${targetPath})`,
    size: formatFileSize(staged.size),
    status: 'transferring' as const,
    progress: 0,
  };
  sftpTransferQueue.value.unshift(queueItem);

  const formData = new FormData();
  formData.append('file', staged.file);

  logSftp('command', `STOR "${staged.name}" -> [${dstHost.name}] "${targetPath}" (${formatFileSize(staged.size)})`);

  try {
    const res = await axios.post(`/api/v1/remote-host/${dstHost.id}/sftp/upload`, formData, {
      params: { path: targetPath },
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    if (res.data.success) {
      staged.status = 'completed';
      queueItem.status = 'success';
      queueItem.progress = 100;
      logSftp('response', `Upload of "${staged.name}" to [${dstHost.name}] completed successfully.`);
      await fetchDestSftpFiles();
    }
  } catch (err: any) {
    staged.status = 'failed';
    queueItem.status = 'failed';
    const msg = err.response?.data?.error || 'Upload failed';
    logSftp('error', `Upload failed: ${msg}`);
  }
};

const uploadAllStagedFiles = async () => {
  const readyFiles = localStagedFiles.value.filter((f) => f.status === 'ready' || f.status === 'failed');
  for (const f of readyFiles) {
    await uploadStagedFile(f);
  }
};

// Local File Staging Handlers
const triggerLocalFileBrowse = () => {
  if (sftpFileInput.value) sftpFileInput.value.click();
};

const handleLocalFileSelection = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (!target.files || target.files.length === 0) return;
  for (let i = 0; i < target.files.length; i++) {
    const f = target.files[i];
    localStagedFiles.value.unshift({
      id: `local-${Date.now()}-${i}`,
      file: f,
      name: f.name,
      size: f.size,
      type: f.type || getFileTypeLabel(f.name, false),
      status: 'ready',
    });
  }
  target.value = '';
};

const handleLocalDrop = (e: DragEvent) => {
  isDragOverLocal.value = false;
  if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
    for (let i = 0; i < e.dataTransfer.files.length; i++) {
      const f = e.dataTransfer.files[i];
      localStagedFiles.value.unshift({
        id: `local-${Date.now()}-${i}`,
        file: f,
        name: f.name,
        size: f.size,
        type: f.type || getFileTypeLabel(f.name, false),
        status: 'ready',
      });
    }
  }
};

const removeLocalStagedFile = (id: string) => {
  localStagedFiles.value = localStagedFiles.value.filter((f) => f.id !== id);
};

// Filtered Lists
const filteredSourceFiles = computed(() => {
  if (!sftpSourceFilter.value) return sftpSourceFiles.value;
  const q = sftpSourceFilter.value.toLowerCase();
  return sftpSourceFiles.value.filter((f) => f.name.toLowerCase().includes(q));
});

const filteredDestFiles = computed(() => {
  if (!sftpDestFilter.value) return sftpDestFiles.value;
  const q = sftpDestFilter.value.toLowerCase();
  return sftpDestFiles.value.filter((f) => f.name.toLowerCase().includes(q));
});

function formatFileSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function getFileTypeLabel(name: string, isDir: boolean): string {
  if (isDir) return 'File folder';
  const ext = name.split('.').pop()?.toLowerCase() || '';
  if (['yaml', 'yml'].includes(ext)) return 'YAML Configuration';
  if (['json'].includes(ext)) return 'JSON Document';
  if (['log'].includes(ext)) return 'System Log File';
  if (['tar', 'gz', 'zip', 'bz2'].includes(ext)) return 'Compressed Archive';
  if (['sh', 'bash'].includes(ext)) return 'Shell Script';
  if (['conf', 'cfg', 'ini'].includes(ext)) return 'Config File';
  if (['txt', 'md'].includes(ext)) return 'Text Document';
  return 'File';
}

function getFilePermissions(file: any): string {
  if (file.permissions) return file.permissions;
  return file.isDir ? 'drwxr-xr-x' : '-rw-r--r--';
}

const handleBackToPortal = () => {
  if (window.opener) {
    window.close();
  } else {
    router.push('/');
  }
};

const handleSaveHost = async () => {
  try {
    const res = await axios.post('/api/v1/remote-host', hostForm.value);
    if (res.data.success) {
      isHostModalOpen.value = false;
      hostForm.value = {
        id: '',
        name: '',
        host: '',
        port: 22,
        username: '',
        authType: 'password',
        password: '',
        sshKey: '',
        groupName: 'Default',
        tags: [],
      };
      await fetchHosts();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to save host');
  }
};

const handleCreateGroup = () => {
  if (newGroupName.value.trim()) {
    isGroupModalOpen.value = false;
    hostForm.value.groupName = newGroupName.value.trim();
    isHostModalOpen.value = true;
    newGroupName.value = '';
  }
};

onMounted(() => {
  fetchHosts();
});

onUnmounted(() => {
  openSessions.value.forEach((s) => {
    if (s.ws) s.ws.close();
    if (s.term) s.term.dispose();
  });
});
</script>

<template>
  <div class="h-screen w-screen bg-[#14161b] text-slate-200 font-sans flex flex-col overflow-hidden selection:bg-brand-500/30">
    
    <!-- Top Header Bar -->
    <header class="h-12 bg-[#1b1e26] border-b border-slate-800 px-4 flex items-center justify-between shrink-0">
      <div class="flex items-center gap-2.5">
        <SquareTerminal class="w-4 h-4 text-brand-400" />
        <h1 class="text-xs font-semibold text-white tracking-wide">Remote Server (SSH & SFTP)</h1>
      </div>

      <div>
        <button
          @click="handleBackToPortal"
          class="flex items-center gap-1.5 px-3 py-1 rounded bg-[#242833] border border-slate-700/60 text-xs text-slate-300 hover:text-white hover:border-slate-500 transition font-medium"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Back to Portal</span>
        </button>
      </div>
    </header>

    <!-- Sub-Header Tabs & Quick Actions Bar -->
    <div class="bg-[#1b1e26] border-b border-slate-800/80 px-4 flex items-center gap-2 text-xs shrink-0 py-1.5 overflow-x-auto">
      <!-- Servers Menu Button -->
      <button
        @click="activeSessionIndex = -1"
        :class="[
          'flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-medium transition text-xs',
          activeSessionIndex === -1
            ? 'bg-slate-800 text-white border border-slate-700 shadow-sm'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
        ]"
      >
        <Server class="w-3.5 h-3.5 text-slate-400" />
        <span>SERVERS</span>
      </button>

      <!-- Quick Add Server (+) Button -->
      <button
        @click="isHostModalOpen = true"
        title="Add New Remote Server"
        class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
      >
        <Plus class="w-3.5 h-3.5" />
      </button>

      <!-- Dual-Pane SFTP Transfer Button -->
      <button
        @click="openSftpModal()"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-slate-300 hover:text-white bg-[#20242e] border border-slate-700/80 hover:border-brand-500/50 transition font-medium text-xs shadow-sm"
        title="Open FileZilla Dual-Pane SFTP Transfer"
      >
        <Upload class="w-3.5 h-3.5 text-brand-400" />
        <span>SFTP TRANSFER</span>
      </button>

      <!-- Open Session Tabs (Default Flat Tabs vs Optional Group Tabs) -->
      <div class="flex items-center gap-1.5 ml-2 border-l border-slate-800 pl-3">
        
        <!-- Toggle Grouping Button (Only shown when 2+ sessions open) -->
        <button
          v-if="openSessions.length > 1"
          @click="isGroupTabsEnabled = !isGroupTabsEnabled"
          :class="[
            'flex items-center gap-1 px-2 py-1 rounded text-[11px] font-mono border transition mr-1',
            isGroupTabsEnabled
              ? 'bg-brand-500/20 text-brand-400 border-brand-500/40 font-bold'
              : 'bg-[#20242e] text-slate-400 border-slate-700/60 hover:text-slate-200'
          ]"
          :title="isGroupTabsEnabled ? 'Ungroup tabs (Flat view)' : 'Group tabs by Host'"
        >
          <Layers class="w-3 h-3" />
          <span>{{ isGroupTabsEnabled ? 'Grouped' : 'Group Tabs' }}</span>
        </button>

        <!-- MODE 1: OPTIONAL GROUP TABS (When User Enables It) -->
        <template v-if="isGroupTabsEnabled">
          <div
            v-for="grp in groupedSessions"
            :key="grp.hostId"
            class="flex items-center gap-1 p-0.5 rounded-xl bg-[#171a23] border border-slate-800/80 shadow-sm"
          >
            <!-- Group Tag / Host Label -->
            <div class="flex items-center gap-1.5 px-2 py-1 text-[11px] font-bold text-slate-400 font-mono">
              <Server class="w-3 h-3 text-brand-400" />
              <span>{{ grp.hostName }}</span>
              <span v-if="grp.sessions.length > 1" class="px-1.5 py-0.2 rounded bg-slate-800 text-brand-400 text-[10px] font-bold">
                {{ grp.sessions.length }} tabs
              </span>
            </div>

            <!-- Child Session Tabs within this Group -->
            <div
              v-for="sItem in grp.sessions"
              :key="sItem.session.id"
              @click="activeSessionIndex = sItem.globalIndex"
              :class="[
                'flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-mono transition cursor-pointer border',
                activeSessionIndex === sItem.globalIndex
                  ? 'bg-[#242833] border-slate-700 text-white shadow-sm font-semibold'
                  : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
              ]"
            >
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-400" :class="{ 'animate-pulse': activeSessionIndex === sItem.globalIndex }"></span>
              <span>#{{ sItem.session.instanceNumber || 1 }}</span>

              <!-- Duplicate Button -->
              <button
                @click.stop="duplicateSession(sItem.session)"
                title="Duplicate tab in this group"
                class="p-0.5 hover:text-brand-400 hover:bg-slate-700/50 rounded transition text-slate-500"
              >
                <Copy class="w-2.5 h-2.5" />
              </button>

              <!-- Close Tab Button -->
              <button
                @click.stop="closeSession(sItem.globalIndex)"
                title="Close Tab"
                class="p-0.5 hover:text-red-400 hover:bg-slate-700/50 rounded transition text-slate-500"
              >
                <X class="w-2.5 h-2.5" />
              </button>
            </div>
          </div>
        </template>

        <!-- MODE 2: DEFAULT FLAT TABS (Normal Individual Tabs) -->
        <template v-else>
          <div
            v-for="(session, idx) in openSessions"
            :key="session.id"
            @click="activeSessionIndex = idx"
            :class="[
              'flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-mono transition cursor-pointer border group',
              activeSessionIndex === idx
                ? 'bg-[#242833] border-slate-700 text-white shadow-sm'
                : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            ]"
          >
            <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
            <span>{{ session.displayName || session.host.name }}</span>

            <!-- DUPLICATE TAB BUTTON (+) -->
            <button
              @click.stop="duplicateSession(session)"
              title="Duplicate Terminal Tab"
              class="p-0.5 hover:text-brand-400 hover:bg-slate-700/50 rounded transition text-slate-400 ml-1"
            >
              <Copy class="w-3 h-3" />
            </button>

            <!-- Close Tab Button -->
            <button
              @click.stop="closeSession(idx)"
              title="Close Tab"
              class="p-0.5 hover:text-red-400 hover:bg-slate-700/50 rounded transition text-slate-400"
            >
              <X class="w-3 h-3" />
            </button>
          </div>
        </template>

        <!-- Global + Duplicate Active Tab -->
        <button
          v-if="activeSession"
          @click="duplicateSession(activeSession)"
          title="Duplicate Current Server Tab"
          class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-300 text-[11px] font-mono border border-slate-700/60 transition"
        >
          <Plus class="w-3 h-3 text-brand-400" />
          <span>New Tab</span>
        </button>
      </div>
    </div>

    <!-- Main Workspace Body -->
    <div class="flex-1 flex overflow-hidden">
      
      <!-- VIEW 1: SERVER LIST / DISCOVERY -->
      <div v-if="activeSessionIndex === -1" class="flex-1 p-6 overflow-y-auto max-w-5xl mx-auto w-full space-y-6">
        <div class="space-y-3">
          <div class="relative">
            <input
              v-model="searchHostQuery"
              placeholder="Find a host or ssh user@hostname..."
              class="w-full bg-[#1b1e26] border border-slate-800 rounded-lg px-4 py-2.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
            />
          </div>

          <div class="flex items-center gap-3">
            <button
              @click="isHostModalOpen = true"
              class="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold rounded-lg shadow-lg shadow-blue-500/20 transition"
            >
              <Plus class="w-4 h-4" />
              <span>NEW HOST</span>
            </button>

            <button
              @click="isGroupModalOpen = true"
              class="flex items-center gap-2 px-4 py-2 bg-[#1b1e26] hover:bg-[#242833] text-slate-300 hover:text-white text-xs font-semibold rounded-lg border border-slate-800 transition"
            >
              <FolderPlus class="w-4 h-4 text-brand-400" />
              <span>NEW GROUP</span>
            </button>
          </div>
        </div>

        <div v-if="hosts.length === 0" class="p-12 bg-[#1b1e26] border border-slate-800/80 rounded-xl text-center space-y-3">
          <div class="w-12 h-12 rounded-full bg-slate-800 text-slate-400 flex items-center justify-center mx-auto">
            <Server class="w-6 h-6" />
          </div>
          <h3 class="text-sm font-bold text-white">No Remote Servers Configured</h3>
          <p class="text-xs text-slate-400 max-w-sm mx-auto">
            Add your first SSH server or VPS to manage interactive terminal sessions, telemetry metrics, and system services.
          </p>
        </div>

        <!-- Groups Section -->
        <div v-if="hosts.length > 0" class="space-y-2">
          <div class="flex items-center justify-between">
            <h3 class="text-[11px] font-bold text-slate-500 uppercase tracking-wider">Groups</h3>
            <button
              v-if="selectedGroupFilter"
              @click="selectedGroupFilter = null"
              class="text-[11px] text-brand-400 hover:underline"
            >
              Clear Filter (Show All)
            </button>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div
              v-for="(gHosts, gName) in groupedHosts"
              :key="gName"
              @click="selectedGroupFilter = selectedGroupFilter === gName ? null : gName"
              :class="[
                'p-4 bg-[#1b1e26] border rounded-xl flex items-center gap-3 cursor-pointer transition',
                selectedGroupFilter === gName
                  ? 'border-blue-500 shadow-lg shadow-blue-500/10'
                  : 'border-slate-800/80 hover:border-slate-700'
              ]"
            >
              <div class="p-2.5 rounded-lg bg-slate-800 text-slate-400">
                <Server class="w-5 h-5" />
              </div>
              <div>
                <p class="text-xs font-bold text-white">{{ gName }}</p>
                <p class="text-[11px] text-slate-500">{{ gHosts.length }} Host{{ gHosts.length > 1 ? 's' : '' }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Hosts Section -->
        <div v-if="hosts.length > 0" class="space-y-2">
          <h3 class="text-[11px] font-bold text-slate-500 uppercase tracking-wider">
            Hosts ({{ filteredHosts.length }})
          </h3>
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
            <div
              v-for="host in filteredHosts"
              :key="host.id"
              @click="connectHost(host)"
              class="p-4 bg-[#1b1e26] border border-slate-800 hover:border-emerald-500/80 rounded-xl flex items-center justify-between gap-3 cursor-pointer hover:shadow-lg hover:shadow-emerald-500/10 transition group"
            >
              <div class="flex items-center gap-3 overflow-hidden">
                <div class="w-10 h-10 rounded-full bg-blue-600/90 text-white flex items-center justify-center font-bold text-xs tracking-wider shrink-0 shadow-md">
                  {{ host.name.substring(0, 2).toUpperCase() }}
                </div>
                <div class="overflow-hidden">
                  <p class="text-xs font-bold text-white group-hover:text-emerald-400 transition truncate">{{ host.name }}</p>
                  <p class="text-[10px] text-slate-400 font-mono truncate">ssh, {{ host.username }}, {{ host.host }}</p>
                  <div class="flex items-center gap-1.5 mt-1.5">
                    <span class="px-2 py-0.2 rounded text-[9px] bg-slate-800 text-slate-400 font-medium">
                      {{ host.groupName || 'Default' }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- Quick Duplicate / New Tab -->
              <button
                @click.stop="connectHost(host, true)"
                title="Open New Terminal Tab"
                class="p-2 rounded-lg bg-slate-800/80 text-slate-400 hover:text-white hover:bg-brand-600 transition shrink-0"
              >
                <Plus class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- VIEW 2: ACTIVE HOST WORKSPACES (PRESERVED IN DOM WITH v-show) -->
      <template v-for="(session, sIdx) in openSessions" :key="session.id">
        <div v-show="activeSessionIndex === sIdx" class="flex-1 flex overflow-hidden">
          <!-- Left Vertical Icon Nav Bar -->
          <aside class="w-12 bg-[#1b1e26] border-r border-slate-800 flex flex-col items-center py-3 gap-2 shrink-0">
            <button
              @click="session.activeView = 'terminal'"
              title="Interactive Terminal"
              :class="[
                'p-2.5 rounded-lg transition',
                session.activeView === 'terminal'
                  ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              ]"
            >
              <SquareTerminal class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'dashboard')"
              title="Host Dashboard & Metrics"
              :class="[
                'p-2.5 rounded-lg transition',
                session.activeView === 'dashboard'
                  ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              ]"
            >
              <LayoutGrid class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'processes')"
              title="Process Manager"
              :class="[
                'p-2.5 rounded-lg transition',
                session.activeView === 'processes'
                  ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              ]"
            >
              <Activity class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'services')"
              title="System Services"
              :class="[
                'p-2.5 rounded-lg transition',
                session.activeView === 'services'
                  ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              ]"
            >
              <Settings class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'network')"
              title="Network & Ports"
              :class="[
                'p-2.5 rounded-lg transition',
                session.activeView === 'network'
                  ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              ]"
            >
              <Wifi class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'sftp')"
              title="FileZilla Dual-Pane SFTP Transfer"
              class="p-2.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
            >
              <Folder class="w-4 h-4" />
            </button>
          </aside>

          <!-- Host Content Pane -->
          <div class="flex-1 flex flex-col overflow-hidden bg-[#090d16]">
            
            <!-- 1. TERMINAL VIEW (Always mounted, no blank screens) -->
            <div v-show="session.activeView === 'terminal'" class="flex-1 flex flex-col relative h-full">
              <div class="absolute top-3 right-5 z-20 flex items-center gap-2">
                <span
                  v-if="session.connected"
                  class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 text-[10px] font-mono border border-emerald-500/30 flex items-center gap-1.5 shadow-sm"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                  Connected
                </span>
                <div v-else class="flex items-center gap-2">
                  <span class="px-2 py-0.5 rounded bg-red-500/10 text-red-400 text-[10px] font-mono border border-red-500/30 flex items-center gap-1.5">
                    <span class="w-1.5 h-1.5 rounded-full bg-red-400"></span>
                    Disconnected
                  </span>
                  <button
                    @click="reconnectTerminal(session)"
                    class="flex items-center gap-1 px-2.5 py-0.5 rounded bg-brand-500 hover:bg-brand-600 text-white text-[11px] font-medium shadow-md transition"
                  >
                    <RotateCw class="w-3 h-3" />
                    <span>Reconnect</span>
                  </button>
                </div>
              </div>
              <div :id="`terminal-container-${session.id}`" class="flex-1 p-2 w-full h-full"></div>
            </div>

            <!-- 2. DASHBOARD VIEW -->
            <div v-if="session.activeView === 'dashboard'" class="flex-1 p-6 overflow-y-auto space-y-6">
              <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                <div>
                  <h2 class="text-sm font-bold text-white tracking-wide">Dashboard & System Telemetry</h2>
                  <span class="text-xs font-mono text-slate-400">{{ session.host.name }} ({{ session.host.host }})</span>
                </div>

                <button
                  @click="fetchHostTelemetry(session)"
                  :disabled="isTelemetryLoading"
                  class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#1b1e26] hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700 transition shadow"
                >
                  <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTelemetryLoading }" />
                  <span>Refresh Stats</span>
                </button>
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-4 gap-4">
                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                  <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">CPU Usage</p>
                  <p class="text-2xl font-black text-white font-mono">{{ session.metrics?.cpuUsage || 0 }}%</p>
                  <p class="text-[10px] text-slate-500">{{ session.metrics?.cpuCores || 1 }} cores</p>
                </div>

                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-2 shadow-lg">
                  <div class="flex justify-between items-center">
                    <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Memory</p>
                    <span class="text-xs font-mono text-slate-300">{{ session.metrics?.memUsed || '0 B' }} / {{ session.metrics?.memTotal || '0 B' }}</span>
                  </div>
                  <p class="text-2xl font-black text-white font-mono">{{ Math.round(session.metrics?.memPercent || 0) }}%</p>
                  <div class="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
                    <div class="h-full bg-emerald-400" :style="{ width: `${session.metrics?.memPercent || 0}%` }"></div>
                  </div>
                </div>

                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                  <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Load Average</p>
                  <p class="text-xl font-bold text-white font-mono mt-1">{{ session.metrics?.loadAverage || '0.00 / 0.00 / 0.00' }}</p>
                </div>

                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                  <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Disks</p>
                  <p class="text-2xl font-black text-white font-mono">{{ session.metrics?.disksCount || 0 }}</p>
                  <p class="text-[10px] text-slate-500">mounted</p>
                </div>
              </div>
            </div>

            <!-- 3. PROCESSES VIEW -->
            <div v-if="session.activeView === 'processes'" class="flex-1 p-6 overflow-y-auto space-y-4">
              <div class="flex flex-wrap items-center justify-between border-b border-slate-800 pb-3 gap-3">
                <div class="flex items-center gap-3">
                  <h2 class="text-sm font-bold text-white tracking-wide">Processes ({{ filteredProcesses.length }})</h2>
                  <button
                    @click="fetchHostTelemetry(session)"
                    :disabled="isTelemetryLoading"
                    class="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
                    title="Refresh Processes"
                  >
                    <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTelemetryLoading }" />
                  </button>
                </div>

                <div class="flex items-center gap-2">
                  <input
                    v-model="processSearch"
                    placeholder="Search process..."
                    class="bg-[#1b1e26] border border-slate-800 rounded-lg px-3 py-1 text-xs text-white placeholder-slate-500"
                  />
                  <select v-model="processSort" class="bg-[#1b1e26] border border-slate-800 rounded-lg px-2 py-1 text-xs text-slate-300">
                    <option value="cpu">Sort by CPU</option>
                    <option value="mem">Sort by Memory</option>
                    <option value="pid">Sort by PID</option>
                  </select>
                </div>
              </div>

              <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-x-auto shadow-xl">
                <table class="w-full text-left text-xs font-mono">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="p-3 w-20">PID</th>
                      <th class="p-3 w-28">User</th>
                      <th class="p-3 w-20 text-right">CPU %</th>
                      <th class="p-3 w-20 text-right">MEM %</th>
                      <th class="p-3">Command</th>
                      <th class="p-3 w-24 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-800/60 text-slate-300">
                    <tr v-for="p in filteredProcesses" :key="p.pid" class="hover:bg-slate-800/30">
                      <td class="p-3 text-slate-400 font-bold">{{ p.pid }}</td>
                      <td class="p-3 text-slate-300">{{ p.user }}</td>
                      <td class="p-3 text-emerald-400 font-bold text-right">{{ p.cpu }}%</td>
                      <td class="p-3 text-sky-400 text-right">{{ p.mem }}%</td>
                      <td class="p-3 text-white truncate max-w-md">{{ p.command }}</td>
                      <td class="p-3 text-right">
                        <button
                          @click="handleKillProcess(p.pid)"
                          class="px-2 py-0.5 rounded bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/30 text-[10px] font-semibold transition"
                          title="Terminate Process"
                        >
                          Kill
                        </button>
                      </td>
                    </tr>
                    <tr v-if="filteredProcesses.length === 0">
                      <td colspan="6" class="p-12 text-center text-slate-500 text-xs">
                        {{ isTelemetryLoading ? 'Polling active processes...' : 'No processes found.' }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- 4. SERVICES VIEW -->
            <div v-if="session.activeView === 'services'" class="flex-1 p-6 overflow-y-auto space-y-4">
              <div class="flex flex-wrap items-center justify-between border-b border-slate-800 pb-3 gap-3">
                <div class="flex items-center gap-3">
                  <h2 class="text-sm font-bold text-white tracking-wide">System Services ({{ filteredServices.length }})</h2>
                  <button
                    @click="fetchHostTelemetry(session)"
                    :disabled="isTelemetryLoading"
                    class="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
                    title="Refresh Services"
                  >
                    <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTelemetryLoading }" />
                  </button>
                </div>

                <input
                  v-model="serviceSearch"
                  placeholder="Search systemd service..."
                  class="bg-[#1b1e26] border border-slate-800 rounded-lg px-3 py-1 text-xs text-white placeholder-slate-500"
                />
              </div>

              <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-x-auto shadow-xl">
                <table class="w-full text-left text-xs font-mono">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="p-3">Service Name</th>
                      <th class="p-3 w-28">State</th>
                      <th class="p-3 w-28">Status</th>
                      <th class="p-3">Description</th>
                      <th class="p-3 w-32 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-800/60 text-slate-300">
                    <tr v-for="s in filteredServices" :key="s.name" class="hover:bg-slate-800/30">
                      <td class="p-3 text-white font-bold">{{ s.name }}</td>
                      <td class="p-3 font-semibold" :class="s.activeState === 'active' ? 'text-emerald-400' : 'text-slate-500'">{{ s.activeState }}</td>
                      <td class="p-3 text-slate-400">{{ s.subState }}</td>
                      <td class="p-3 text-slate-400 truncate max-w-md">{{ s.description }}</td>
                      <td class="p-3 text-right space-x-1 whitespace-nowrap">
                        <button
                          @click="handleControlService(s.name, 'restart')"
                          class="px-2 py-0.5 rounded bg-blue-600/20 hover:bg-blue-600/40 text-sky-400 border border-sky-500/30 text-[10px] font-semibold transition"
                          title="Restart Service"
                        >
                          Restart
                        </button>
                        <button
                          v-if="s.activeState === 'active'"
                          @click="handleControlService(s.name, 'stop')"
                          class="px-2 py-0.5 rounded bg-amber-500/20 hover:bg-amber-500/40 text-amber-400 border border-amber-500/30 text-[10px] font-semibold transition"
                          title="Stop Service"
                        >
                          Stop
                        </button>
                      </td>
                    </tr>
                    <tr v-if="filteredServices.length === 0">
                      <td colspan="5" class="p-12 text-center text-slate-500 text-xs">
                        {{ isTelemetryLoading ? 'Polling system services...' : 'No services found.' }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- 5. NETWORK & PORTS VIEW -->
            <div v-if="session.activeView === 'network'" class="flex-1 p-6 overflow-y-auto space-y-6">
              <div class="space-y-3">
                <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                  <h3 class="text-xs font-bold text-white uppercase tracking-wider">Listening Ports ({{ filteredListeningPorts.length }})</h3>
                  <button
                    @click="fetchHostTelemetry(session)"
                    :disabled="isTelemetryLoading"
                    class="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
                    title="Refresh Listening Ports"
                  >
                    <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTelemetryLoading }" />
                  </button>
                </div>

                <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-x-auto shadow-xl">
                  <table class="w-full text-left text-xs font-mono">
                    <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                      <tr>
                        <th class="p-3 w-24">Proto</th>
                        <th class="p-3">Local Address : Port</th>
                        <th class="p-3">Process</th>
                        <th class="p-3 w-28">PID</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-800/60 text-slate-300">
                      <tr v-for="p in filteredListeningPorts" :key="`${p.proto}-${p.port}-${p.pid}`" class="hover:bg-slate-800/30">
                        <td class="p-3 text-emerald-400 font-bold uppercase">{{ p.proto }}</td>
                        <td class="p-3 text-white font-bold">{{ p.localAddr || '*' }}:{{ p.port }}</td>
                        <td class="p-3 text-brand-400">{{ p.process || '-' }}</td>
                        <td class="p-3 text-slate-400">{{ p.pid || '-' }}</td>
                      </tr>
                      <tr v-if="filteredListeningPorts.length === 0">
                        <td colspan="4" class="p-12 text-center text-slate-500 text-xs">
                          {{ isTelemetryLoading ? 'Polling open network ports...' : 'No listening ports detected.' }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>

          </div>
        </div>
      </template>

    </div>

    <!-- ================================================================= -->
    <!-- TRUE FILEZILLA DUAL-PANE SFTP TRANSFER CLIENT (SERVER-TO-SERVER & SERVER-TO-LOCAL) -->
    <!-- ================================================================= -->
    <div
      v-if="isSftpModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/85 backdrop-blur-md p-3 animate-in fade-in duration-150"
    >
      <div class="bg-[#13161f] border border-slate-700 rounded-2xl w-full max-w-7xl h-[92vh] flex flex-col shadow-2xl overflow-hidden font-sans">
        
        <!-- 1. TOP QUICKCONNECT & SWAP BAR -->
        <div class="p-3 bg-[#1b1e26] border-b border-slate-800 flex flex-wrap items-center justify-between gap-3 shrink-0 text-xs">
          <div class="flex items-center gap-3">
            <div class="flex items-center gap-2 font-bold text-white tracking-wide">
              <Folder class="w-4 h-4 text-brand-400" />
              <span>SFTP Transfer (Server-to-Server & Server-to-Local)</span>
            </div>

            <span class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 text-[10px] font-mono border border-emerald-500/30 flex items-center gap-1">
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
              SFTP Ready
            </span>
          </div>

          <!-- Swap Source & Destination Button -->
          <div class="flex items-center gap-2">
            <button
              @click="swapSourceAndDest"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#20242e] hover:bg-slate-700 text-brand-400 font-bold text-xs border border-brand-500/30 transition shadow-sm"
              title="Swap Source and Destination Host"
            >
              <RotateCw class="w-3.5 h-3.5" />
              <span>🔀 SWAP SOURCE & DESTINATION</span>
            </button>

            <button @click="isSftpModalOpen = false" class="p-1 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition">
              <X class="w-5 h-5" />
            </button>
          </div>
        </div>

        <!-- 2. FILEZILLA STATUS & COMMAND CONSOLE LOG -->
        <div class="bg-[#090d16] border-b border-slate-800/80 p-2 px-4 max-h-20 overflow-y-auto font-mono text-[11px] space-y-0.5 shrink-0 select-text">
          <div v-for="(log, idx) in sftpCommandLogs" :key="idx" class="flex items-center gap-2">
            <span class="text-slate-600 text-[10px] select-none">{{ log.time }}</span>
            <span
              :class="[
                log.type === 'command' ? 'text-sky-400 font-semibold' :
                log.type === 'response' ? 'text-emerald-400' :
                log.type === 'error' ? 'text-rose-400 font-bold' : 'text-slate-400',
                'break-all'
              ]"
            >
              {{ log.type === 'command' ? 'Command: ' : log.type === 'response' ? 'Response: ' : log.type === 'error' ? 'Error: ' : 'Status: ' }}
              {{ log.text }}
            </span>
          </div>
          <div v-if="sftpCommandLogs.length === 0" class="text-slate-600 text-[10px]">
            Status: SFTP subsystem session ready. Connected to endpoints.
          </div>
        </div>

        <!-- 3. DUAL-PANE BODY (SOURCE ON LEFT, DESTINATION ON RIGHT) -->
        <div class="flex-1 grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-slate-800 overflow-hidden min-h-0 bg-[#0e1118]">
          
          <!-- ============================================================= -->
          <!-- PANE 1: SOURCE SITE (LOCAL OR REMOTE VM A) -->
          <!-- ============================================================= -->
          <div
            class="flex flex-col overflow-hidden relative bg-[#0b0e14]"
            @dragover.prevent="isDragOverLocal = true"
            @dragleave.prevent="isDragOverLocal = false"
            @drop.prevent="handleLocalDrop"
          >
            <!-- Source Header: Host Selector & Path -->
            <div class="p-2.5 px-3.5 bg-[#171a23] border-b border-slate-800 flex flex-wrap items-center justify-between gap-2 shrink-0 text-xs">
              <div class="flex items-center gap-2">
                <span class="font-bold text-emerald-400 uppercase text-[11px] tracking-wider flex items-center gap-1">
                  <Monitor v-if="sftpSourceHostId === 'local'" class="w-3.5 h-3.5" />
                  <Server v-else class="w-3.5 h-3.5" />
                  <span>SOURCE:</span>
                </span>

                <!-- Source Host Selector -->
                <select
                  v-model="sftpSourceHostId"
                  @change="fetchSourceSftpFiles('/')"
                  class="bg-[#0f1219] border border-slate-700 rounded-lg px-2 py-1 text-white font-semibold focus:outline-none focus:border-brand-500 text-xs"
                >
                  <option value="local">💻 Local Computer (Upload)</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">
                    🏢 {{ h.name }} ({{ h.host }})
                  </option>
                </select>
              </div>

              <!-- If Source is Local Computer -->
              <div v-if="sftpSourceHostId === 'local'" class="flex items-center gap-2">
                <input type="file" ref="sftpFileInput" multiple @change="handleLocalFileSelection" class="hidden" />
                <button
                  @click="triggerLocalFileBrowse"
                  class="flex items-center gap-1 px-2.5 py-1 rounded bg-blue-600 hover:bg-blue-500 text-white font-medium text-xs shadow transition"
                >
                  <Plus class="w-3.5 h-3.5" />
                  <span>Browse Local</span>
                </button>
                <button
                  v-if="localStagedFiles.length > 0"
                  @click="uploadAllStagedFiles"
                  class="flex items-center gap-1 px-2.5 py-1 rounded bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow transition"
                >
                  <ArrowRight class="w-3.5 h-3.5" />
                  <span>Upload All ➔</span>
                </button>
              </div>

              <!-- If Source is Remote VM (VM A) Path Bar -->
              <div v-else class="flex items-center gap-1.5 flex-1 max-w-xs">
                <button
                  @click="() => {
                    const parts = sftpSourceCurrentPath.split('/').filter(Boolean);
                    parts.pop();
                    fetchSourceSftpFiles(parts.length === 0 ? '/' : '/' + parts.join('/'));
                  }"
                  :disabled="sftpSourceCurrentPath === '/' || !sftpSourceCurrentPath"
                  class="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-30 text-[11px] font-mono border border-slate-700"
                >
                  <CornerLeftUp class="w-3.5 h-3.5" />
                </button>
                <form @submit.prevent="fetchSourceSftpFiles(sftpSourceInputPath)" class="flex items-center gap-1 flex-1">
                  <input
                    v-model="sftpSourceInputPath"
                    class="w-full bg-[#0f1219] border border-slate-700 rounded px-2 py-0.5 text-xs text-white font-mono"
                  />
                  <button type="submit" class="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-white text-xs">Go</button>
                </form>
                <button @click="fetchSourceSftpFiles()" class="p-1 rounded bg-slate-800 text-slate-300">
                  <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': sftpSourceLoading }" />
                </button>
              </div>
            </div>

            <!-- Source Sub-Bar (Bookmarks or Staging Info) -->
            <div class="p-1.5 px-3.5 bg-[#12151e] border-b border-slate-800/80 flex items-center justify-between text-[11px] text-slate-400 shrink-0 font-mono">
              <template v-if="sftpSourceHostId === 'local'">
                <span>Path: Local Computer\Staging</span>
                <span>{{ localStagedFiles.length }} file(s) staged</span>
              </template>
              <template v-else>
                <div class="flex items-center gap-1">
                  <span class="text-slate-500 text-[10px]">Quick:</span>
                  <button @click="fetchSourceSftpFiles('/')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/</button>
                  <button @click="fetchSourceSftpFiles('/root')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/root</button>
                  <button @click="fetchSourceSftpFiles('/etc')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/etc</button>
                  <button @click="fetchSourceSftpFiles('/var/log')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/var/log</button>
                  <button @click="fetchSourceSftpFiles('/home')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/home</button>
                </div>
                <input
                  v-model="sftpSourceFilter"
                  placeholder="Filter source files..."
                  class="bg-[#090d16] border border-slate-800 rounded px-2 py-0.5 text-[10px] text-white w-32 focus:outline-none"
                />
              </template>
            </div>

            <!-- Source Body: Local Staging List OR Remote VM A File Table -->
            <div class="flex-1 overflow-y-auto select-text">
              <!-- Mode 1: Local Staged Table -->
              <table v-if="sftpSourceHostId === 'local'" class="w-full text-left text-xs font-mono border-collapse">
                <thead class="bg-[#171a23] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800 sticky top-0 select-none">
                  <tr>
                    <th class="py-2.5 px-3.5">Filename</th>
                    <th class="py-2.5 px-3 w-24 text-right">Size</th>
                    <th class="py-2.5 px-3.5 w-32 text-right">Action</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/50 text-[11px] text-slate-300">
                  <tr v-for="staged in localStagedFiles" :key="staged.id" class="hover:bg-slate-800/40">
                    <td class="py-2 px-3.5 flex items-center gap-2 truncate max-w-[180px]">
                      <File class="w-4 h-4 text-slate-400 shrink-0" />
                      <span>{{ staged.name }}</span>
                    </td>
                    <td class="py-2 px-3 text-right text-slate-400">{{ formatFileSize(staged.size) }}</td>
                    <td class="py-2 px-3.5 text-right whitespace-nowrap">
                      <button
                        @click="uploadStagedFile(staged)"
                        :disabled="staged.status === 'uploading'"
                        class="px-2 py-0.5 rounded bg-emerald-600/20 hover:bg-emerald-600/40 text-emerald-400 border border-emerald-500/30 text-[10px] font-semibold transition"
                      >
                        {{ staged.status === 'uploading' ? '...' : 'Transfer ➔' }}
                      </button>
                      <button @click="removeLocalStagedFile(staged.id)" class="p-1 text-slate-500 hover:text-red-400 ml-1">
                        <Trash2 class="w-3 h-3" />
                      </button>
                    </td>
                  </tr>
                  <tr v-if="localStagedFiles.length === 0">
                    <td colspan="3" class="py-14 text-center text-slate-500 text-xs">
                      <Monitor class="w-8 h-8 text-slate-600 mx-auto mb-1" />
                      <p class="font-medium text-slate-400">Local Staging Empty</p>
                      <p class="text-[11px] text-slate-600">Browse or drag files to upload to Destination.</p>
                    </td>
                  </tr>
                </tbody>
              </table>

              <!-- Mode 2: Remote VM (VM A) File Table -->
              <table v-else class="w-full text-left text-xs font-mono border-collapse">
                <thead class="bg-[#171a23] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800 sticky top-0 select-none">
                  <tr>
                    <th class="py-2.5 px-3.5">Filename</th>
                    <th class="py-2.5 px-3 w-20 text-right">Size</th>
                    <th class="py-2.5 px-3.5 w-32 text-right">Transfer</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/50 text-[11px] text-slate-300">
                  <tr
                    v-for="file in filteredSourceFiles"
                    :key="file.name"
                    @dblclick="file.isDir ? fetchSourceSftpFiles(`${sftpSourceCurrentPath === '/' ? '' : sftpSourceCurrentPath}/${file.name}`) : transferSourceFileToDest(file.name)"
                    class="hover:bg-slate-800/40 cursor-pointer group"
                  >
                    <td class="py-2 px-3.5 flex items-center gap-2 truncate max-w-[180px]">
                      <Folder v-if="file.isDir" class="w-4 h-4 text-amber-400 shrink-0" />
                      <File v-else class="w-4 h-4 text-slate-400 shrink-0" />
                      <span :class="file.isDir ? 'font-bold text-white' : 'text-slate-200'">{{ file.name }}</span>
                    </td>
                    <td class="py-2 px-3 text-right font-mono text-slate-400">
                      {{ file.isDir ? 'DIR' : formatFileSize(file.size) }}
                    </td>
                    <td class="py-2 px-3.5 text-right whitespace-nowrap">
                      <button
                        v-if="!file.isDir"
                        @click.stop="transferSourceFileToDest(file.name)"
                        class="px-2 py-0.5 rounded bg-blue-600/20 hover:bg-blue-600/40 text-sky-400 border border-sky-500/30 text-[10px] font-semibold transition flex items-center gap-1 ml-auto"
                        title="Transfer to active folder on Destination Host"
                      >
                        <ArrowRight class="w-3 h-3" />
                        <span>Transfer ➔</span>
                      </button>
                      <button
                        v-else
                        @click.stop="fetchSourceSftpFiles(`${sftpSourceCurrentPath === '/' ? '' : sftpSourceCurrentPath}/${file.name}`)"
                        class="px-2 py-0.5 rounded bg-slate-800 text-slate-300 text-[10px]"
                      >
                        Open
                      </button>
                    </td>
                  </tr>
                  <tr v-if="filteredSourceFiles.length === 0 && !sftpSourceLoading">
                    <td colspan="3" class="py-12 text-center text-slate-500 text-xs">Directory is empty</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- ============================================================= -->
          <!-- PANE 2: DESTINATION SITE (REMOTE VM B OR LOCAL) -->
          <!-- ============================================================= -->
          <div class="flex flex-col overflow-hidden bg-[#0e1118]">
            <!-- Destination Header: Host Selector & Path -->
            <div class="p-2.5 px-3.5 bg-[#171a23] border-b border-slate-800 flex flex-wrap items-center justify-between gap-2 shrink-0 text-xs">
              <div class="flex items-center gap-2">
                <span class="font-bold text-sky-400 uppercase text-[11px] tracking-wider flex items-center gap-1">
                  <Server v-if="sftpDestHostId !== 'local'" class="w-3.5 h-3.5" />
                  <Monitor v-else class="w-3.5 h-3.5" />
                  <span>DESTINATION:</span>
                </span>

                <!-- Destination Host Selector -->
                <select
                  v-model="sftpDestHostId"
                  @change="fetchDestSftpFiles('/')"
                  class="bg-[#0f1219] border border-slate-700 rounded-lg px-2 py-1 text-white font-semibold focus:outline-none focus:border-brand-500 text-xs"
                >
                  <option v-for="h in hosts" :key="h.id" :value="h.id">
                    🏢 {{ h.name }} ({{ h.host }})
                  </option>
                  <option value="local">💻 Local Computer (Direct Download)</option>
                </select>
              </div>

              <!-- Destination Path Bar (If Remote Host VM B) -->
              <div v-if="sftpDestHostId !== 'local'" class="flex items-center gap-1.5 flex-1 max-w-xs">
                <button
                  @click="() => {
                    const parts = sftpDestCurrentPath.split('/').filter(Boolean);
                    parts.pop();
                    fetchDestSftpFiles(parts.length === 0 ? '/' : '/' + parts.join('/'));
                  }"
                  :disabled="sftpDestCurrentPath === '/' || !sftpDestCurrentPath"
                  class="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-30 text-[11px] font-mono border border-slate-700"
                >
                  <CornerLeftUp class="w-3.5 h-3.5" />
                </button>
                <form @submit.prevent="fetchDestSftpFiles(sftpDestInputPath)" class="flex items-center gap-1 flex-1">
                  <input
                    v-model="sftpDestInputPath"
                    class="w-full bg-[#0f1219] border border-slate-700 rounded px-2 py-0.5 text-xs text-white font-mono"
                  />
                  <button type="submit" class="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-white text-xs">Go</button>
                </form>
                <button @click="fetchDestSftpFiles()" class="p-1 rounded bg-slate-800 text-slate-300">
                  <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': sftpDestLoading }" />
                </button>
              </div>
            </div>

            <!-- Destination Sub-Bar (Bookmarks & Filter) -->
            <div class="p-1.5 px-3.5 bg-[#12151e] border-b border-slate-800/80 flex items-center justify-between text-[11px] text-slate-400 shrink-0 font-mono">
              <template v-if="sftpDestHostId === 'local'">
                <span>Path: Local Computer\Downloads</span>
                <span>Transfers will download directly to your browser.</span>
              </template>
              <template v-else>
                <div class="flex items-center gap-1">
                  <span class="text-slate-500 text-[10px]">Quick:</span>
                  <button @click="fetchDestSftpFiles('/')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/</button>
                  <button @click="fetchDestSftpFiles('/root')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/root</button>
                  <button @click="fetchDestSftpFiles('/etc')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/etc</button>
                  <button @click="fetchDestSftpFiles('/var/log')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/var/log</button>
                  <button @click="fetchDestSftpFiles('/home')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/home</button>
                  <button @click="fetchDestSftpFiles('/tmp')" class="px-1.5 py-0.2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300">/tmp</button>
                </div>
                <input
                  v-model="sftpDestFilter"
                  placeholder="Filter destination files..."
                  class="bg-[#090d16] border border-slate-800 rounded px-2 py-0.5 text-[10px] text-white w-36 focus:outline-none"
                />
              </template>
            </div>

            <!-- Destination Body: Destination VM B File Table -->
            <div class="flex-1 overflow-y-auto select-text">
              <table v-if="sftpDestHostId !== 'local'" class="w-full text-left text-xs font-mono border-collapse">
                <thead class="bg-[#171a23] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800 sticky top-0 select-none">
                  <tr>
                    <th class="py-2.5 px-3.5">Filename</th>
                    <th class="py-2.5 px-3 w-20 text-right">Size</th>
                    <th class="py-2.5 px-3 w-24">Type</th>
                    <th class="py-2.5 px-3 w-24 font-mono">Perms</th>
                    <th class="py-2.5 px-3.5 w-20 text-right">Action</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/50 text-[11px] text-slate-300">
                  <tr
                    v-for="file in filteredDestFiles"
                    :key="file.name"
                    @dblclick="file.isDir ? fetchDestSftpFiles(`${sftpDestCurrentPath === '/' ? '' : sftpDestCurrentPath}/${file.name}`) : null"
                    class="hover:bg-slate-800/50 cursor-pointer group"
                  >
                    <td class="py-2 px-3.5 flex items-center gap-2 truncate max-w-[170px]">
                      <Folder v-if="file.isDir" class="w-4 h-4 text-amber-400 shrink-0" />
                      <File v-else class="w-4 h-4 text-slate-400 shrink-0" />
                      <span :class="file.isDir ? 'font-bold text-white' : 'text-slate-200'">{{ file.name }}</span>
                    </td>
                    <td class="py-2 px-3 text-right font-mono" :class="file.isDir ? 'text-slate-600' : 'text-slate-300'">
                      {{ file.isDir ? 'DIR' : formatFileSize(file.size) }}
                    </td>
                    <td class="py-2 px-3 text-slate-400 truncate">{{ getFileTypeLabel(file.name, file.isDir) }}</td>
                    <td class="py-2 px-3 font-mono text-[10px] text-slate-400">{{ getFilePermissions(file) }}</td>
                    <td class="py-2 px-3.5 text-right whitespace-nowrap">
                      <button
                        v-if="file.isDir"
                        @click="fetchDestSftpFiles(`${sftpDestCurrentPath === '/' ? '' : sftpDestCurrentPath}/${file.name}`)"
                        class="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-[10px]"
                      >
                        Open
                      </button>
                      <span v-else class="text-slate-600 text-[10px]">Target</span>
                    </td>
                  </tr>
                  <tr v-if="filteredDestFiles.length === 0 && !sftpDestLoading">
                    <td colspan="5" class="py-12 text-center text-slate-500 text-xs">Destination directory is empty</td>
                  </tr>
                </tbody>
              </table>

              <!-- Destination is Local Mode -->
              <div v-else class="p-14 text-center text-slate-500 text-xs">
                <Monitor class="w-8 h-8 text-sky-500 mx-auto mb-2" />
                <p class="font-bold text-white text-sm">Local Computer (Browser Download Target)</p>
                <p class="text-slate-400 mt-1 max-w-sm mx-auto">
                  Clicking "Transfer ➔" on any file from the Source VM will download the file directly to your local computer downloads folder.
                </p>
              </div>
            </div>
          </div>

        </div>

        <!-- 4. BOTTOM TRANSFER QUEUE & STATUS BAR -->
        <div class="p-2.5 px-4 bg-[#14161b] border-t border-slate-800 text-[11px] text-slate-400 flex flex-wrap items-center justify-between gap-3 shrink-0">
          <div class="flex items-center gap-4">
            <span class="text-slate-300 font-semibold">Transfer Queue ({{ sftpTransferQueue.length }})</span>
            <div v-if="sftpTransferQueue.length > 0" class="flex items-center gap-2 font-mono text-[10px]">
              <span class="text-emerald-400 font-semibold">Active: {{ sftpTransferQueue[0].name }}</span>
              <span class="text-slate-500">({{ sftpTransferQueue[0].status }})</span>
            </div>
          </div>

          <div class="flex items-center gap-3 text-slate-400 font-mono text-[10px]">
            <span>[Source: {{ sourceHostObj ? sourceHostObj.name : 'Local Computer' }}] ➔ [Destination: {{ destHostObj ? destHostObj.name : 'Local Computer' }}]</span>
            <span class="text-slate-600">|</span>
            <span>SFTP Server-to-Server / FXP Engine</span>
          </div>
        </div>

      </div>
    </div>

    <!-- Modal: Add New Remote Server -->
    <div
      v-if="isHostModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Server class="w-4 h-4 text-brand-400" />
            <h3 class="text-sm font-bold text-white">Add New Remote Server</h3>
          </div>
          <button @click="isHostModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleSaveHost" class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Server Name / Identifier</label>
            <input v-model="hostForm.name" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white" placeholder="e.g. Server - Bifrost" />
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div class="col-span-2">
              <label class="block text-slate-400 mb-1">Server IP / Domain</label>
              <input v-model="hostForm.host" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white" placeholder="10.20.3.1" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1">Port</label>
              <input v-model.number="hostForm.port" type="number" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1">Username</label>
              <input v-model="hostForm.username" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white" placeholder="root" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1">Group</label>
              <input
                v-model="hostForm.groupName"
                list="group-options"
                required
                class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white"
                placeholder="Production"
              />
              <datalist id="group-options">
                <option v-for="g in existingGroupNames" :key="g" :value="g" />
              </datalist>
            </div>
          </div>

          <div>
            <label class="block text-slate-400 mb-1">
              <span class="flex items-center gap-1.5">
                <Lock class="w-3.5 h-3.5 text-amber-400" />
                Password (AES-256-GCM Encrypted)
              </span>
            </label>
            <input
              v-model="hostForm.password"
              type="password"
              class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
              placeholder="••••••••"
            />
          </div>

          <div class="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              type="button"
              @click="isHostModalOpen = false"
              class="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg shadow-lg shadow-blue-500/20"
            >
              Save Server
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Add New Group -->
    <div
      v-if="isGroupModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-sm p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <FolderPlus class="w-4 h-4 text-brand-400" />
            <h3 class="text-sm font-bold text-white">Create New Group</h3>
          </div>
          <button @click="isGroupModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Group Name</label>
            <input
              v-model="newGroupName"
              required
              class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500"
              placeholder="e.g. Staging, Core Infra, DMZ"
            />
          </div>

          <div class="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              type="button"
              @click="isGroupModalOpen = false"
              class="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg"
            >
              Cancel
            </button>
            <button
              @click="handleCreateGroup"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg shadow-lg shadow-blue-500/20"
            >
              Set Group
            </button>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>
