<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useRouter } from 'vue-router';
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
} from 'lucide-vue-next';

const router = useRouter();
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
  activeView: 'terminal' | 'dashboard' | 'processes' | 'services' | 'network' | 'sftp';
  term?: Terminal;
  fitAddon?: FitAddon;
  ws?: WebSocket;
  connected: boolean;
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
const processSort = ref('cpu');
const serviceSearch = ref('');
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

// SFTP States
const sftpFiles = ref<any[]>([]);
const sftpCurrentPath = ref('/');
const sftpLoading = ref(false);

const activeSession = computed(() => {
  if (activeSessionIndex.value >= 0 && activeSessionIndex.value < openSessions.value.length) {
    return openSessions.value[activeSessionIndex.value];
  }
  return null;
});

// Groups computed
const groupedHosts = computed(() => {
  const groups: Record<string, RemoteHost[]> = {};
  hosts.value.forEach(h => {
    const g = h.groupName || 'Default';
    if (!groups[g]) groups[g] = [];
    groups[g].push(h);
  });
  return groups;
});

const existingGroupNames = computed(() => {
  const set = new Set<string>();
  hosts.value.forEach(h => {
    if (h.groupName) set.add(h.groupName);
  });
  if (set.size === 0) set.add('Default');
  return Array.from(set);
});

// Filtered hosts
const filteredHosts = computed(() => {
  let list = hosts.value;
  if (selectedGroupFilter.value) {
    list = list.filter(h => h.groupName === selectedGroupFilter.value);
  }
  if (!searchHostQuery.value) return list;
  const q = searchHostQuery.value.toLowerCase();
  return list.filter(
    h => h.name.toLowerCase().includes(q) || h.host.toLowerCase().includes(q) || h.username.toLowerCase().includes(q)
  );
});

// Filtered & Sorted Processes
const filteredProcesses = computed(() => {
  if (!activeSession.value?.processes) return [];
  let list = [...activeSession.value.processes];
  if (processSearch.value) {
    const q = processSearch.value.toLowerCase();
    list = list.filter(p => p.command.toLowerCase().includes(q) || p.user.toLowerCase().includes(q));
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
    s => s.name.toLowerCase().includes(q) || (s.description && s.description.toLowerCase().includes(q))
  );
});

// Filtered Listening Ports
const filteredListeningPorts = computed(() => {
  if (!activeSession.value?.networkInfo?.listeningPorts) return [];
  let list = [...activeSession.value.networkInfo.listeningPorts];
  if (portProtoFilter.value !== 'all') {
    const p = portProtoFilter.value.toUpperCase();
    list = list.filter(item => item.proto && item.proto.toUpperCase().includes(p));
  }
  if (portSearch.value) {
    const q = portSearch.value.toLowerCase();
    list = list.filter(
      item =>
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

// Fetch Hosts
const fetchHosts = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data.success && res.data.data) {
      hosts.value = res.data.data;
    } else {
      hosts.value = [];
    }
  } catch (err) {
    console.error('Failed to load remote hosts:', err);
    hosts.value = [];
  }
};

// Open a Host Session
const connectHost = async (host: RemoteHost) => {
  // Check if session already exists
  const existingIdx = openSessions.value.findIndex(s => s.host.id === host.id);
  if (existingIdx >= 0) {
    activeSessionIndex.value = existingIdx;
    return;
  }

  const session: OpenSession = {
    id: `session-${Date.now()}`,
    host,
    activeView: 'terminal',
    connected: false,
  };

  openSessions.value.push(session);
  activeSessionIndex.value = openSessions.value.length - 1;

  await nextTick();
  initXterm(session);
  fetchHostTelemetry(session);
};

// Close Session
const closeSession = (idx: number, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  const s = openSessions.value[idx];
  if (s) {
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
};

// Initialize xterm.js Terminal with Token Authentication
const initXterm = (session: OpenSession) => {
  const container = document.getElementById(`terminal-container-${session.id}`);
  if (!container) return;
  container.innerHTML = '';

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
  fitAddon.fit();

  session.term = term;
  session.fitAddon = fitAddon;

  // Open WebSocket with Auth Token query param
  const token = authStore.token || localStorage.getItem('hcp_token') || '';
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/remote-host?token=${encodeURIComponent(token)}&hostId=${session.host.id}&cols=${term.cols}&rows=${term.rows}`;
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    session.connected = true;
    term.write('\r\n\x1b[32m[Connected to ' + session.host.name + ' (' + session.host.host + ')]\x1b[0m\r\n\r\n');
    // Send initial auth handshake payload
    ws.send(JSON.stringify({
      type: 'auth',
      token: token,
      hostConfigId: session.host.id,
      cols: term.cols,
      rows: term.rows,
    }));
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
        term.write('\r\n\x1b[31m[Session closed]\x1b[0m\r\n');
      }
    } catch (e) {
      term.write(ev.data);
    }
  };

  ws.onclose = () => {
    session.connected = false;
    term.write('\r\n\x1b[31m[Session closed]\x1b[0m\r\n');
  };

  ws.onerror = () => {
    session.connected = false;
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

  window.addEventListener('resize', () => {
    try {
      fitAddon.fit();
    } catch (e) {}
  });

  session.ws = ws;
};

// Fetch Host Metrics, Processes, Services, and Network
const fetchHostTelemetry = async (session: OpenSession) => {
  try {
    const [metricsRes, procsRes, svcsRes, netRes] = await Promise.allSettled([
      axios.get(`/api/v1/vps/${session.host.id}/metrics`),
      axios.get(`/api/v1/vps/${session.host.id}/processes`),
      axios.get(`/api/v1/vps/${session.host.id}/services`),
      axios.get(`/api/v1/vps/${session.host.id}/network`),
    ]);

    if (metricsRes.status === 'fulfilled' && metricsRes.value.data.success) {
      session.metrics = metricsRes.value.data.data;
    }
    if (procsRes.status === 'fulfilled' && procsRes.value.data.success) {
      session.processes = procsRes.value.data.data;
    }
    if (svcsRes.status === 'fulfilled' && svcsRes.value.data.success) {
      session.services = svcsRes.value.data.data;
    }
    if (netRes.status === 'fulfilled' && netRes.value.data.success) {
      session.networkInfo = netRes.value.data.data;
    }
  } catch (err) {
    console.error('Failed to fetch telemetry:', err);
  }
};

// Kill Process
const handleKillProcess = async (pid: number) => {
  if (!activeSession.value) return;
  if (!confirm(`Are you sure you want to terminate process PID ${pid}?`)) return;
  try {
    await axios.post(`/api/v1/vps/${activeSession.value.host.id}/processes/${pid}/kill`);
    fetchHostTelemetry(activeSession.value);
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to kill process');
  }
};

// Control Service
const handleControlService = async (serviceName: string, action: string) => {
  if (!activeSession.value) return;
  try {
    await axios.post(`/api/v1/vps/${activeSession.value.host.id}/control`, {
      serviceName,
      action,
    });
    fetchHostTelemetry(activeSession.value);
  } catch (err: any) {
    alert(err.response?.data?.error || `Failed to ${action} service`);
  }
};

// Save Host
const handleSaveHost = async () => {
  try {
    const res = await axios.post('/api/v1/remote-host', hostForm.value);
    if (res.data.success) {
      isHostModalOpen.value = false;
      // Reset form
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

// Create Group
const handleCreateGroup = () => {
  if (!newGroupName.value.trim()) return;
  hostForm.value.groupName = newGroupName.value.trim();
  isGroupModalOpen.value = false;
  newGroupName.value = '';
};

const handleBackToPortal = () => {
  if (window.opener) {
    window.close();
  } else {
    router.push('/');
  }
};

onMounted(() => {
  fetchHosts();
});

onUnmounted(() => {
  openSessions.value.forEach(s => {
    if (s.ws) s.ws.close();
    if (s.term) s.term.dispose();
  });
});
</script>

<template>
  <div class="h-screen w-screen bg-[#14161b] text-slate-200 font-sans flex flex-col overflow-hidden selection:bg-brand-500/30">
    <!-- Top Header Bar -->
    <header class="h-12 bg-[#1b1e26] border-b border-slate-800 px-4 flex items-center justify-between shrink-0">
      <!-- Title -->
      <div class="flex items-center gap-2.5">
        <SquareTerminal class="w-4 h-4 text-brand-400" />
        <h1 class="text-xs font-semibold text-white tracking-wide">Remote Host</h1>
      </div>

      <!-- Right Action -->
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
            ? 'bg-slate-800 text-white border border-slate-700'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
        ]"
      >
        <Server class="w-3.5 h-3.5 text-slate-400" />
        <span>SERVERS</span>
      </button>

      <!-- Quick Add Host (+) Button -->
      <button
        @click="isHostModalOpen = true"
        title="Add New Remote Host"
        class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
      >
        <Plus class="w-3.5 h-3.5" />
      </button>

      <!-- Transfer Button -->
      <button
        @click="isSftpModalOpen = true"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800/50 transition font-medium text-xs"
      >
        <Upload class="w-3.5 h-3.5 text-slate-400" />
        <span>TRANSFER</span>
      </button>

      <!-- Open Session Tabs -->
      <div class="flex items-center gap-1.5 ml-2 border-l border-slate-800 pl-3">
        <div
          v-for="(session, idx) in openSessions"
          :key="session.id"
          @click="activeSessionIndex = idx"
          :class="[
            'flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-mono transition cursor-pointer border',
            activeSessionIndex === idx
              ? 'bg-[#242833] border-slate-700 text-white'
              : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          ]"
        >
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          <span>{{ session.host.name }}</span>
          <button
            @click="closeSession(idx, $event)"
            class="p-0.5 hover:text-red-400 rounded transition ml-1"
          >
            <X class="w-3 h-3" />
          </button>
        </div>
      </div>
    </div>

    <!-- Main Workspace Body -->
    <div class="flex-1 flex overflow-hidden">
      
      <!-- ================================================================= -->
      <!-- VIEW 1: SERVER LIST / DISCOVERY (When activeSessionIndex === -1) -->
      <!-- ================================================================= -->
      <div v-if="activeSessionIndex === -1" class="flex-1 p-6 overflow-y-auto max-w-5xl mx-auto w-full space-y-6">
        <!-- Search & New Host / New Group Bar -->
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

        <!-- Empty State -->
        <div v-if="hosts.length === 0" class="p-12 bg-[#1b1e26] border border-slate-800/80 rounded-xl text-center space-y-3">
          <div class="w-12 h-12 rounded-full bg-slate-800 text-slate-400 flex items-center justify-center mx-auto">
            <Server class="w-6 h-6" />
          </div>
          <h3 class="text-sm font-bold text-white">No Remote Servers Configured</h3>
          <p class="text-xs text-slate-400 max-w-sm mx-auto">
            Add your first SSH server or VPS to manage interactive terminal sessions, telemetry metrics, and system services.
          </p>
          <button
            @click="isHostModalOpen = true"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold rounded-lg shadow-lg shadow-blue-500/20 transition"
          >
            + Add New Server
          </button>
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
            <span v-if="selectedGroupFilter" class="text-slate-400 normal-case">in group "{{ selectedGroupFilter }}"</span>
          </h3>
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
            <div
              v-for="host in filteredHosts"
              :key="host.id"
              @click="connectHost(host)"
              class="p-4 bg-[#1b1e26] border border-emerald-500/30 rounded-xl flex items-center gap-3 cursor-pointer hover:border-emerald-500 hover:shadow-lg hover:shadow-emerald-500/10 transition group"
            >
              <!-- Initials Avatar -->
              <div class="w-10 h-10 rounded-full bg-blue-600/90 text-white flex items-center justify-center font-bold text-xs tracking-wider shrink-0 shadow-md">
                {{ host.name.substring(0, 2).toUpperCase() }}
              </div>
              <div class="overflow-hidden flex-1">
                <p class="text-xs font-bold text-white group-hover:text-emerald-400 transition truncate">{{ host.name }}</p>
                <p class="text-[10px] text-slate-400 font-mono truncate">ssh, {{ host.username }}, {{ host.host }}</p>
                <div class="flex items-center gap-1.5 mt-1.5">
                  <span class="px-2 py-0.2 rounded text-[9px] bg-slate-800 text-slate-400 font-medium">
                    {{ host.groupName || 'Default' }}
                  </span>
                  <span
                    v-for="tag in (host.tags || [])"
                    :key="tag"
                    class="px-2 py-0.2 rounded text-[9px] bg-slate-800 text-slate-400 font-medium"
                  >
                    {{ tag }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- VIEW 2: ACTIVE HOST WORKSPACE (When activeSessionIndex >= 0) -->
      <!-- ================================================================= -->
      <div v-else-if="activeSession" class="flex-1 flex overflow-hidden">
        <!-- Left Vertical Icon Nav Bar -->
        <aside class="w-12 bg-[#1b1e26] border-r border-slate-800 flex flex-col items-center py-3 gap-2 shrink-0">
          <button
            @click="activeSession.activeView = 'terminal'"
            :title="'Interactive Terminal'"
            :class="[
              'p-2.5 rounded-lg transition',
              activeSession.activeView === 'terminal'
                ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                : 'text-slate-400 hover:text-white hover:bg-slate-800'
            ]"
          >
            <SquareTerminal class="w-4 h-4" />
          </button>

          <button
            @click="activeSession.activeView = 'dashboard'"
            :title="'Host Dashboard & Metrics'"
            :class="[
              'p-2.5 rounded-lg transition',
              activeSession.activeView === 'dashboard'
                ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                : 'text-slate-400 hover:text-white hover:bg-slate-800'
            ]"
          >
            <LayoutGrid class="w-4 h-4" />
          </button>

          <button
            @click="activeSession.activeView = 'processes'"
            :title="'Process Manager'"
            :class="[
              'p-2.5 rounded-lg transition',
              activeSession.activeView === 'processes'
                ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                : 'text-slate-400 hover:text-white hover:bg-slate-800'
            ]"
          >
            <Activity class="w-4 h-4" />
          </button>

          <button
            @click="activeSession.activeView = 'services'"
            :title="'System Services'"
            :class="[
              'p-2.5 rounded-lg transition',
              activeSession.activeView === 'services'
                ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                : 'text-slate-400 hover:text-white hover:bg-slate-800'
            ]"
          >
            <Settings class="w-4 h-4" />
          </button>

          <button
            @click="activeSession.activeView = 'network'"
            :title="'Network & Ports'"
            :class="[
              'p-2.5 rounded-lg transition',
              activeSession.activeView === 'network'
                ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                : 'text-slate-400 hover:text-white hover:bg-slate-800'
            ]"
          >
            <Wifi class="w-4 h-4" />
          </button>
        </aside>

        <!-- Host Content Pane -->
        <div class="flex-1 flex flex-col overflow-hidden bg-[#090d16]">
          
          <!-- 1. TERMINAL VIEW -->
          <div v-show="activeSession.activeView === 'terminal'" class="flex-1 flex flex-col relative h-full">
            <div class="absolute top-3 right-5 z-20 flex items-center gap-2">
              <span class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 text-[10px] font-mono border border-emerald-500/30 flex items-center gap-1.5">
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                Connected
              </span>
            </div>
            <div :id="`terminal-container-${activeSession.id}`" class="flex-1 p-2 w-full h-full"></div>
          </div>

          <!-- 2. DASHBOARD / METRICS VIEW -->
          <div v-if="activeSession.activeView === 'dashboard'" class="flex-1 p-6 overflow-y-auto space-y-6">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
              <h2 class="text-sm font-bold text-white tracking-wide">Dashboard</h2>
              <span class="text-xs font-mono text-slate-400">{{ activeSession.host.name }} ({{ activeSession.host.host }})</span>
            </div>

            <!-- Metric Cards -->
            <div class="grid grid-cols-1 sm:grid-cols-4 gap-4">
              <!-- CPU -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">CPU Usage</p>
                <p class="text-2xl font-black text-white font-mono">{{ activeSession.metrics?.cpuUsage || 0 }}%</p>
                <p class="text-[10px] text-slate-500">{{ activeSession.metrics?.cpuCores || 1 }} cores</p>
              </div>

              <!-- Memory -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-2">
                <div class="flex justify-between items-center">
                  <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Memory</p>
                  <span class="text-xs font-mono text-slate-300">{{ activeSession.metrics?.memUsed || '0 B' }} / {{ activeSession.metrics?.memTotal || '0 B' }}</span>
                </div>
                <p class="text-2xl font-black text-white font-mono">{{ Math.round(activeSession.metrics?.memPercent || 0) }}%</p>
                <div class="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
                  <div class="h-full bg-emerald-400" :style="{ width: `${activeSession.metrics?.memPercent || 0}%` }"></div>
                </div>
              </div>

              <!-- Load Average -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Load Average</p>
                <p class="text-xl font-bold text-white font-mono mt-1">{{ activeSession.metrics?.loadAverage || '0.00 / 0.00 / 0.00' }}</p>
              </div>

              <!-- Disks -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Disks</p>
                <p class="text-2xl font-black text-white font-mono">{{ activeSession.metrics?.disksCount || 0 }}</p>
                <p class="text-[10px] text-slate-500">mounted</p>
              </div>
            </div>

            <!-- Disk Usage Table -->
            <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
              <div class="p-3 px-5 border-b border-slate-800 text-xs font-bold text-white">
                Disk Usage
              </div>
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="py-2.5 px-5">Mount</th>
                      <th class="py-2.5 px-4">Total</th>
                      <th class="py-2.5 px-4">Used</th>
                      <th class="py-2.5 px-4">Avail</th>
                      <th class="py-2.5 px-6">Usage</th>
                    </tr>
                  </thead>
                  <tbody v-if="activeSession.metrics?.disks && activeSession.metrics.disks.length > 0" class="divide-y divide-slate-800/60 font-mono text-xs">
                    <tr
                      v-for="(d, dIdx) in activeSession.metrics.disks"
                      :key="dIdx"
                      class="hover:bg-slate-800/30 transition text-slate-300"
                    >
                      <td class="py-3 px-5 text-slate-200">{{ d.mount }}</td>
                      <td class="py-3 px-4">{{ d.total }}</td>
                      <td class="py-3 px-4">{{ d.used }}</td>
                      <td class="py-3 px-4">{{ d.avail }}</td>
                      <td class="py-3 px-6">
                        <div class="flex items-center gap-3">
                          <div class="w-36 h-1.5 bg-slate-800 rounded-full overflow-hidden">
                            <div
                              class="h-full rounded-full"
                              :class="d.percent > 85 ? 'bg-red-500' : d.percent > 60 ? 'bg-amber-500' : 'bg-emerald-400'"
                              :style="{ width: `${d.percent}%` }"
                            ></div>
                          </div>
                          <span class="text-[11px]">{{ d.percent }}%</span>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                  <tbody v-else>
                    <tr>
                      <td colspan="5" class="py-6 text-center text-slate-500 text-xs">No disks mounted or telemetry unavailable.</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- 3. PROCESSES VIEW -->
          <div v-if="activeSession.activeView === 'processes'" class="flex-1 p-6 overflow-y-auto space-y-4">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
              <h2 class="text-sm font-bold text-white tracking-wide">Processes</h2>
              <span class="text-xs font-mono text-slate-400">{{ activeSession.host.name }} ({{ activeSession.host.host }})</span>
            </div>

            <!-- Search & Sort Controls -->
            <div class="flex items-center justify-between gap-4">
              <div class="relative flex-1 max-w-xs">
                <input
                  v-model="processSearch"
                  placeholder="Search processes..."
                  class="w-full bg-[#1b1e26] border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
                />
              </div>

              <div>
                <select
                  v-model="processSort"
                  class="bg-[#1b1e26] border border-slate-800 text-slate-300 text-xs rounded-lg px-3 py-1.5 focus:outline-none focus:border-brand-500"
                >
                  <option value="cpu">Sort by CPU</option>
                  <option value="mem">Sort by Memory</option>
                  <option value="pid">Sort by PID</option>
                </select>
              </div>
            </div>

            <!-- Processes Table -->
            <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="py-2.5 px-4">PID</th>
                      <th class="py-2.5 px-4">User</th>
                      <th class="py-2.5 px-4">CPU%</th>
                      <th class="py-2.5 px-4">MEM%</th>
                      <th class="py-2.5 px-4">RSS</th>
                      <th class="py-2.5 px-4">Command</th>
                      <th class="py-2.5 px-4 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody v-if="filteredProcesses.length > 0" class="divide-y divide-slate-800/60 font-mono text-xs">
                    <tr
                      v-for="p in filteredProcesses"
                      :key="p.pid"
                      class="hover:bg-slate-800/30 transition text-slate-300"
                    >
                      <td class="py-2.5 px-4 text-slate-400">{{ p.pid }}</td>
                      <td class="py-2.5 px-4 font-sans text-slate-300">{{ p.user }}</td>
                      <td class="py-2.5 px-4 font-bold text-white">{{ p.cpu }}</td>
                      <td class="py-2.5 px-4">{{ p.mem }}</td>
                      <td class="py-2.5 px-4">{{ p.rss }}</td>
                      <td class="py-2.5 px-4 max-w-md truncate text-slate-300">{{ p.command }}</td>
                      <td class="py-2.5 px-4 text-right">
                        <button
                          @click="handleKillProcess(p.pid)"
                          class="px-2.5 py-0.5 rounded bg-red-600/90 hover:bg-red-500 text-white font-sans text-[10px] font-bold tracking-wider uppercase transition shadow-sm"
                        >
                          Kill
                        </button>
                      </td>
                    </tr>
                  </tbody>
                  <tbody v-else>
                    <tr>
                      <td colspan="7" class="py-6 text-center text-slate-500 text-xs">No active processes discovered.</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- 4. SERVICES VIEW -->
          <div v-if="activeSession.activeView === 'services'" class="flex-1 p-6 overflow-y-auto space-y-4">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
              <h2 class="text-sm font-bold text-white tracking-wide">Services</h2>
              <span class="text-xs font-mono text-slate-400">{{ activeSession.host.name }} ({{ activeSession.host.host }})</span>
            </div>

            <!-- Search Service Input -->
            <div class="max-w-xs">
              <input
                v-model="serviceSearch"
                placeholder="Search services..."
                class="w-full bg-[#1b1e26] border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
              />
            </div>

            <!-- Services Table -->
            <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="py-2.5 px-4">Name</th>
                      <th class="py-2.5 px-4">Description</th>
                      <th class="py-2.5 px-4">Status</th>
                      <th class="py-2.5 px-4 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody v-if="filteredServices.length > 0" class="divide-y divide-slate-800/60 font-mono text-xs">
                    <tr
                      v-for="s in filteredServices"
                      :key="s.name"
                      class="hover:bg-slate-800/30 transition text-slate-300"
                    >
                      <td class="py-2.5 px-4 font-bold text-white">{{ s.name }}</td>
                      <td class="py-2.5 px-4 text-slate-400 font-sans">{{ s.description }}</td>
                      <td class="py-2.5 px-4 font-sans">
                        <span
                          :class="[
                            'px-2 py-0.5 rounded text-[10px] font-bold tracking-wider uppercase',
                            s.status === 'ACTIVE'
                              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                              : s.status === 'FAILED'
                              ? 'bg-red-500/10 text-red-400 border border-red-500/20'
                              : 'bg-slate-800 text-slate-400'
                          ]"
                        >
                          {{ s.status }}
                        </span>
                      </td>
                      <td class="py-2.5 px-4 text-right">
                        <div class="flex items-center justify-end gap-1.5 font-sans">
                          <button
                            v-if="s.status !== 'ACTIVE'"
                            @click="handleControlService(s.name, 'start')"
                            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-[11px] font-medium transition"
                          >
                            Start
                          </button>
                          <button
                            v-if="s.status === 'ACTIVE'"
                            @click="handleControlService(s.name, 'stop')"
                            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-[11px] font-medium transition"
                          >
                            Stop
                          </button>
                          <button
                            @click="handleControlService(s.name, 'restart')"
                            class="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-[11px] font-medium transition"
                          >
                            Restart
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                  <tbody v-else>
                    <tr>
                      <td colspan="4" class="py-6 text-center text-slate-500 text-xs">No services found.</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- 5. NETWORK & LISTENING PORTS VIEW -->
          <div v-if="activeSession.activeView === 'network'" class="flex-1 p-6 overflow-y-auto space-y-6">
            <!-- Header Bar -->
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
              <div class="flex items-center gap-2">
                <Wifi class="w-4 h-4 text-brand-400" />
                <h2 class="text-sm font-bold text-white tracking-wide">Network & Listening Ports</h2>
              </div>
              <div class="flex items-center gap-3">
                <span class="text-xs font-mono text-slate-400">{{ activeSession.host.name }} ({{ activeSession.host.host }})</span>
                <button
                  @click="fetchHostTelemetry(activeSession)"
                  class="flex items-center gap-1.5 px-3 py-1 rounded bg-[#242833] border border-slate-700/60 text-xs text-slate-300 hover:text-white hover:border-slate-500 transition font-medium"
                >
                  <RotateCw class="w-3.5 h-3.5" />
                  <span>Refresh</span>
                </button>
              </div>
            </div>

            <!-- Top Metric Cards Row (4 cards) -->
            <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
              <!-- Total Interfaces -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Network Interfaces</p>
                <p class="text-2xl font-black text-white font-mono">{{ activeSession.networkInfo?.interfaces?.length || 0 }}</p>
                <p class="text-[10px] text-emerald-400">{{ (activeSession.networkInfo?.interfaces || []).filter((i: any) => i.state === 'UP').length }} UP</p>
              </div>

              <!-- TCP Listening Ports -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">TCP Listening Ports</p>
                <p class="text-2xl font-black text-sky-400 font-mono">{{ (activeSession.networkInfo?.listeningPorts || []).filter((p: any) => p.proto?.startsWith('TCP')).length }}</p>
                <p class="text-[10px] text-slate-500">Active TCP Sockets</p>
              </div>

              <!-- UDP Listening Ports -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">UDP Ports</p>
                <p class="text-2xl font-black text-purple-400 font-mono">{{ (activeSession.networkInfo?.listeningPorts || []).filter((p: any) => p.proto?.startsWith('UDP')).length }}</p>
                <p class="text-[10px] text-slate-500">Unconnected UDP</p>
              </div>

              <!-- Established Connections -->
              <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Active Connections</p>
                <p class="text-2xl font-black text-emerald-400 font-mono">{{ activeSession.networkInfo?.connections?.length || 0 }}</p>
                <p class="text-[10px] text-slate-500">Sockets in ESTAB state</p>
              </div>
            </div>

            <!-- SECTION 1: NETWORK INTERFACES TABLE -->
            <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl space-y-0">
              <div class="p-3 px-5 border-b border-slate-800 flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <Activity class="w-4 h-4 text-emerald-400" />
                  <h3 class="text-xs font-bold text-white tracking-wide uppercase">Network Interfaces</h3>
                </div>
                <div class="relative w-48">
                  <input
                    v-model="interfaceSearch"
                    placeholder="Filter interface..."
                    class="w-full bg-[#14161b] border border-slate-800 rounded-lg px-2.5 py-1 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
                  />
                </div>
              </div>

              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="py-2.5 px-5">Interface</th>
                      <th class="py-2.5 px-4">State</th>
                      <th class="py-2.5 px-4">IPv4 Address & CIDR</th>
                      <th class="py-2.5 px-4">IPv6 Address</th>
                      <th class="py-2.5 px-4">MAC Address</th>
                      <th class="py-2.5 px-4">MTU</th>
                    </tr>
                  </thead>
                  <tbody v-if="filteredInterfaces.length > 0" class="divide-y divide-slate-800/60 font-mono text-xs">
                    <tr
                      v-for="(iface, iIdx) in filteredInterfaces"
                      :key="iIdx"
                      class="hover:bg-slate-800/30 transition text-slate-300"
                    >
                      <td class="py-3 px-5 font-bold text-white font-sans flex items-center gap-2">
                        <span class="w-2 h-2 rounded-full" :class="iface.state === 'UP' ? 'bg-emerald-400' : 'bg-red-400'"></span>
                        <span>{{ iface.name }}</span>
                      </td>
                      <td class="py-3 px-4">
                        <span
                          :class="[
                            'px-2 py-0.5 rounded text-[10px] font-bold font-sans tracking-wider uppercase',
                            iface.state === 'UP' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-red-500/10 text-red-400 border border-red-500/30'
                          ]"
                        >
                          {{ iface.state }}
                        </span>
                      </td>
                      <td class="py-3 px-4 font-bold text-slate-200">{{ iface.ipv4 }}</td>
                      <td class="py-3 px-4 text-slate-400 truncate max-w-[200px]" :title="iface.ipv6">{{ iface.ipv6 }}</td>
                      <td class="py-3 px-4 text-slate-400">{{ iface.mac }}</td>
                      <td class="py-3 px-4 text-slate-300">{{ iface.mtu }}</td>
                    </tr>
                  </tbody>
                  <tbody v-else>
                    <tr>
                      <td colspan="6" class="py-6 text-center text-slate-500 text-xs font-sans">No network interfaces discovered.</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- SECTION 2: ACTIVE LISTENING PORTS (TCP & UDP) TABLE -->
            <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl space-y-0">
              <div class="p-3 px-5 border-b border-slate-800 flex items-center justify-between">
                <div class="flex items-center gap-4">
                  <div class="flex items-center gap-2">
                    <Radio class="w-4 h-4 text-sky-400" />
                    <h3 class="text-xs font-bold text-white tracking-wide uppercase">Active Listening Ports</h3>
                  </div>

                  <!-- Protocol Filter Tabs -->
                  <div class="flex items-center bg-[#14161b] border border-slate-800 rounded-lg p-0.5 text-xs">
                    <button
                      v-for="pOpt in ['all', 'tcp', 'udp']"
                      :key="pOpt"
                      @click="portProtoFilter = pOpt as any"
                      :class="[
                        'px-2.5 py-0.5 rounded font-medium uppercase text-[10px] transition',
                        portProtoFilter === pOpt ? 'bg-brand-500 text-white shadow-sm' : 'text-slate-400 hover:text-white'
                      ]"
                    >
                      {{ pOpt }}
                    </button>
                  </div>
                </div>

                <div class="relative w-48">
                  <input
                    v-model="portSearch"
                    placeholder="Search port / process..."
                    class="w-full bg-[#14161b] border border-slate-800 rounded-lg px-2.5 py-1 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
                  />
                </div>
              </div>

              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="py-2.5 px-5">Protocol</th>
                      <th class="py-2.5 px-4">Local Address</th>
                      <th class="py-2.5 px-4">Port</th>
                      <th class="py-2.5 px-4">State</th>
                      <th class="py-2.5 px-4">Process / Daemon</th>
                      <th class="py-2.5 px-4">PID</th>
                    </tr>
                  </thead>
                  <tbody v-if="filteredListeningPorts.length > 0" class="divide-y divide-slate-800/60 font-mono text-xs">
                    <tr
                      v-for="(p, pIdx) in filteredListeningPorts"
                      :key="pIdx"
                      class="hover:bg-slate-800/30 transition text-slate-300"
                    >
                      <td class="py-3 px-5 font-bold font-sans">
                        <span
                          :class="[
                            'px-2 py-0.5 rounded text-[10px] font-bold',
                            p.proto?.startsWith('TCP') ? 'bg-sky-500/10 border border-sky-500/30 text-sky-400' : 'bg-purple-500/10 border border-purple-500/30 text-purple-400'
                          ]"
                        >
                          {{ p.proto }}
                        </span>
                      </td>
                      <td class="py-3 px-4 text-slate-200">{{ p.localAddr }}</td>
                      <td class="py-3 px-4 font-bold text-brand-400">{{ p.port }}</td>
                      <td class="py-3 px-4 font-sans">
                        <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                          {{ p.state || 'LISTEN' }}
                        </span>
                      </td>
                      <td class="py-3 px-4 font-sans text-slate-200 font-medium">{{ p.process }}</td>
                      <td class="py-3 px-4 text-slate-400">{{ p.pid }}</td>
                    </tr>
                  </tbody>
                  <tbody v-else>
                    <tr>
                      <td colspan="6" class="py-6 text-center text-slate-500 text-xs font-sans">No listening ports matched.</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- SECTION 3: ACTIVE SOCKET CONNECTIONS TABLE -->
            <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl space-y-0">
              <div class="p-3 px-5 border-b border-slate-800 flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <Activity class="w-4 h-4 text-purple-400" />
                  <h3 class="text-xs font-bold text-white tracking-wide uppercase">Active Established Sockets</h3>
                </div>
                <span class="text-xs font-mono text-slate-400">{{ (activeSession.networkInfo?.connections || []).length }} sockets</span>
              </div>

              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="py-2.5 px-5">Proto</th>
                      <th class="py-2.5 px-4">Local Address:Port</th>
                      <th class="py-2.5 px-4">Foreign / Remote Socket</th>
                      <th class="py-2.5 px-4">State</th>
                      <th class="py-2.5 px-4">Process / PID</th>
                    </tr>
                  </thead>
                  <tbody v-if="(activeSession.networkInfo?.connections || []).length > 0" class="divide-y divide-slate-800/60 font-mono text-xs">
                    <tr
                      v-for="(c, cIdx) in (activeSession.networkInfo?.connections || [])"
                      :key="cIdx"
                      class="hover:bg-slate-800/30 transition text-slate-300"
                    >
                      <td class="py-3 px-5 font-sans font-bold text-sky-400">{{ c.proto }}</td>
                      <td class="py-3 px-4 text-slate-200">{{ c.localAddr }}</td>
                      <td class="py-3 px-4 text-slate-300">{{ c.remoteAddr }}</td>
                      <td class="py-3 px-4 font-sans">
                        <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                          {{ c.state }}
                        </span>
                      </td>
                      <td class="py-3 px-4 text-slate-300 font-sans">{{ c.process }} (PID {{ c.pid }})</td>
                    </tr>
                  </tbody>
                  <tbody v-else>
                    <tr>
                      <td colspan="5" class="py-6 text-center text-slate-500 text-xs font-sans">No active established external connections.</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

          </div>

        </div>
      </div>
    </div>

    <!-- Modal: Add New Host -->
    <div
      v-if="isHostModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Server class="w-4 h-4 text-brand-400" />
            <h3 class="text-sm font-bold text-white">Add New Remote Host</h3>
          </div>
          <button @click="isHostModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleSaveHost" class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Host Name / Identifier</label>
            <input v-model="hostForm.name" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white" placeholder="e.g. Bifrost Server" />
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div class="col-span-2">
              <label class="block text-slate-400 mb-1">Host IP / Domain</label>
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
              Save Host
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
