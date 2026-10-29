<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import { useThemeStore } from '../stores/theme';
import ThemeToggle from '../components/ThemeToggle.vue';
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
  Loader2,
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
  ChevronLeft,
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
  Minimize2,
  ChevronDown,
  Palette,
  GripVertical,
  Share2,
  Users,
  AlertCircle,
  ZoomIn,
  ZoomOut,
  ChevronUp,
  LogOut,
  ArrowUpDown,
  Unlink,
  Key,
} from 'lucide-vue-next';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const themeStore = useThemeStore();
const canManage = computed(() => authStore.can('remote_servers', 'manage'));

const darkTerminalTheme = {
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
};

const lightTerminalTheme = {
  background: '#f8fafc',
  foreground: '#1e293b',
  cursor: '#293681',
  selectionBackground: '#D0E7E6',
  black: '#0f172a',
  red: '#dc2626',
  green: '#16a34a',
  yellow: '#ca8a04',
  blue: '#2563eb',
  magenta: '#9333ea',
  cyan: '#0891b2',
  white: '#ffffff',
  brightBlack: '#94a3b8',
  brightRed: '#ef4444',
  brightGreen: '#22c55e',
  brightYellow: '#eab308',
  brightBlue: '#3b82f6',
  brightMagenta: '#a855f7',
  brightCyan: '#06b6d4',
  brightWhite: '#f1f5f9',
};

interface RemoteHost {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  authType: string;
  groupName: string;
  tags: string[];
  userId?: number;
  ownerUsername?: string;
  isOwner?: boolean;
  sharedAccess?: string;
  sharesCount?: number;
}

interface RemoteHostShare {
  id: string;
  hostId: string;
  userId: number;
  username: string;
  permission: string;
  sharedBy?: number;
  sharedByUsername?: string;
  createdAt: string;
}

interface TabGroup {
  id: string;
  name: string;
  color: string;
  collapsed: boolean;
}

const CHROME_COLORS = [
  { name: 'Grey', hex: '#64748b' },
  { name: 'Blue', hex: '#3b82f6' },
  { name: 'Red', hex: '#ef4444' },
  { name: 'Yellow', hex: '#eab308' },
  { name: 'Green', hex: '#10b981' },
  { name: 'Pink', hex: '#ec4899' },
  { name: 'Purple', hex: '#8b5cf6' },
  { name: 'Cyan', hex: '#06b6d4' },
  { name: 'Orange', hex: '#f97316' },
];

interface OpenSession {
  id: string;
  host: RemoteHost;
  displayName?: string;
  activeView: 'terminal' | 'dashboard' | 'processes' | 'services' | 'network' | 'sftp' | 'firewall';
  term?: Terminal;
  fitAddon?: FitAddon;
  resizeObserver?: ResizeObserver;
  ws?: WebSocket;
  connected: boolean;
  connecting?: boolean;
  isIdle?: boolean;
  lastActivity?: number;
  heartbeatTimer?: any;
  heartbeatWorker?: Worker;
  reconnectAttempts?: number;
  reconnectPendingOnVisible?: boolean;
  metrics?: any;
  processes?: any[];
  services?: any[];
  networkInfo?: {
    interfaces?: any[];
    listeningPorts?: any[];
    connections?: any[];
  };
  firewall?: {
    active: boolean;
    service: string;
    rawStatus: string;
    rules: any[];
  };
  groupId?: string;
}

const tabBarRef = ref<HTMLElement | null>(null);

const tabGroups = ref<TabGroup[]>([]);
const isSftpFullScreen = ref(true);

const hosts = ref<RemoteHost[]>([]);
const openSessions = ref<OpenSession[]>([]);
const activeSessionIndex = ref<number>(-1); // -1 means Server List view
const reconnectMenuSessionId = ref<string | null>(null);
const toggleReconnectMenu = (sessionId: string) => {
  if (reconnectMenuSessionId.value === sessionId) {
    reconnectMenuSessionId.value = null;
  } else {
    reconnectMenuSessionId.value = sessionId;
  }
};
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

// Notification state
const notification = ref<{ text: string; type: 'success' | 'error' } | null>(null);
const showNotice = (text: string, type: 'success' | 'error' = 'success') => {
  notification.value = { text, type };
  setTimeout(() => {
    notification.value = null;
  }, 3000);
};

// Delete Confirmation Modal State
const hostToDelete = ref<RemoteHost | null>(null);
const showDeleteHostModal = ref(false);
const deletingHost = ref(false);

// Host Ownership & Sharing State
const hostOwnershipFilter = ref<'all' | 'mine' | 'shared'>('all');
const isShareModalOpen = ref(false);
const selectedHostForShare = ref<RemoteHost | null>(null);
const hostShares = ref<RemoteHostShare[]>([]);
const availableUsers = ref<{ id: number; username: string; role: string }[]>([]);
const isShareLoading = ref(false);
const isShareSubmitting = ref(false);
const shareForm = ref<{ userId: number | ''; permission: 'read' | 'manage' }>({
  userId: '',
  permission: 'read',
});

// New Group Form & State
const newGroupName = ref('');
const groupCreationMode = ref<'existing' | 'new'>('existing');
const selectedHostIdsForGroup = ref<string[]>([]);
const groupSearchQuery = ref('');
const isSubmittingGroup = ref(false);

const selectableHostsForGroup = computed(() => {
  if (!groupSearchQuery.value) return hosts.value;
  const q = groupSearchQuery.value.toLowerCase();
  return hosts.value.filter(
    (h) =>
      h.name.toLowerCase().includes(q) ||
      h.host.toLowerCase().includes(q) ||
      (h.groupName && h.groupName.toLowerCase().includes(q))
  );
});

const toggleSelectAllHostsForGroup = () => {
  const currentSelectable = selectableHostsForGroup.value;
  if (currentSelectable.length === 0) return;
  const allSelected = currentSelectable.every((h) => selectedHostIdsForGroup.value.includes(h.id));
  if (allSelected) {
    selectedHostIdsForGroup.value = selectedHostIdsForGroup.value.filter(
      (id) => !currentSelectable.some((h) => h.id === id)
    );
  } else {
    const set = new Set([...selectedHostIdsForGroup.value, ...currentSelectable.map((h) => h.id)]);
    selectedHostIdsForGroup.value = Array.from(set);
  }
};

// Protocol Preset ('ssh' | 'rdp')
const selectedProtocolPreset = ref<'ssh' | 'rdp'>('ssh');

const isRdpHost = (h: RemoteHost) =>
  h.port === 3389 ||
  h.tags?.some((t: string) => t.toLowerCase() === 'rdp') ||
  (h as any).protocol === 'rdp';
const getHostConnectionLabel = (h: RemoteHost) => {
  if (isRdpHost(h)) return `rdp, ${h.username || 'Administrator'}, ${h.host}:${h.port || 3389}`;
  return `ssh, ${h.username || 'root'}, ${h.host}:${h.port || 22}`;
};

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

const selectProtocolPreset = (preset: 'ssh' | 'rdp') => {
  selectedProtocolPreset.value = preset;
  if (!hostForm.value.tags) hostForm.value.tags = [];
  if (preset === 'rdp') {
    if (hostForm.value.port === 22) {
      hostForm.value.port = 3389;
    }
    if (hostForm.value.username === 'root' || !hostForm.value.username) {
      hostForm.value.username = 'Administrator';
    }
    if (!hostForm.value.tags.includes('rdp')) {
      hostForm.value.tags = hostForm.value.tags.filter((t: string) => t !== 'ssh');
      hostForm.value.tags.push('rdp');
    }
    hostForm.value.authType = 'password';
  } else {
    if (hostForm.value.port === 3389) {
      hostForm.value.port = 22;
    }
    if (hostForm.value.username === 'Administrator' || !hostForm.value.username) {
      hostForm.value.username = 'root';
    }
    hostForm.value.tags = hostForm.value.tags.filter((t: string) => t !== 'rdp');
  }
};

const isTestingConnection = ref(false);
const testResult = ref<{ success: boolean; message: string } | null>(null);

const handleKeyFileUpload = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (!target.files || target.files.length === 0) return;
  const file = target.files[0];
  const reader = new FileReader();
  reader.onload = (e) => {
    const content = e.target?.result as string;
    if (content) {
      hostForm.value.sshKey = content.trim();
    }
  };
  reader.readAsText(file);
  target.value = '';
};

const handleTestConnection = async () => {
  if (!hostForm.value.host || !hostForm.value.username) {
    testResult.value = { success: false, message: 'Host/IP and Username are required to test connection.' };
    return;
  }
  isTestingConnection.value = true;
  testResult.value = null;
  try {
    const payload = {
      id: hostForm.value.id || '',
      name: hostForm.value.name || 'Test Host',
      host: hostForm.value.host,
      port: Number(hostForm.value.port) || (selectedProtocolPreset.value === 'rdp' ? 3389 : 22),
      username: hostForm.value.username,
      authType: hostForm.value.authType || 'password',
      password: hostForm.value.password || '',
      sshKey: hostForm.value.sshKey || '',
      tags: selectedProtocolPreset.value === 'rdp' ? ['rdp'] : (hostForm.value.tags || []),
    };
    const res = await axios.post('/api/v1/remote-host/test', payload);
    if (res.data) {
      testResult.value = {
        success: !!res.data.success,
        message: res.data.message || (res.data.success ? 'Connection successful!' : 'Connection failed'),
      };
    }
  } catch (err: any) {
    testResult.value = {
      success: false,
      message: err.response?.data?.error || err.response?.data?.message || err.message || 'Connection test failed',
    };
  } finally {
    isTestingConnection.value = false;
  }
};

const openAddHostModal = (preset: 'ssh' | 'rdp' = 'ssh') => {
  selectedProtocolPreset.value = preset;
  testResult.value = null;
  hostForm.value = {
    id: '',
    name: '',
    host: '',
    port: preset === 'rdp' ? 3389 : 22,
    username: preset === 'rdp' ? 'Administrator' : 'root',
    authType: 'password',
    password: '',
    sshKey: '',
    groupName: 'Default',
    tags: preset === 'rdp' ? ['rdp'] : [],
  };
  isHostModalOpen.value = true;
};

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

// Ownership counts
const myHostsCount = computed(() => hosts.value.filter((h) => h.isOwner).length);
const sharedHostsCount = computed(() => hosts.value.filter((h) => !h.isOwner).length);

// Filtered hosts
const filteredHosts = computed(() => {
  let list = hosts.value;
  if (selectedGroupFilter.value) {
    list = list.filter((h) => h.groupName === selectedGroupFilter.value);
  }
  if (hostOwnershipFilter.value === 'mine') {
    list = list.filter((h) => h.isOwner);
  } else if (hostOwnershipFilter.value === 'shared') {
    list = list.filter((h) => !h.isOwner);
  }
  if (!searchHostQuery.value) return list;
  const q = searchHostQuery.value.toLowerCase();
  return list.filter(
    (h) =>
      h.name.toLowerCase().includes(q) ||
      h.host.toLowerCase().includes(q) ||
      h.username.toLowerCase().includes(q) ||
      (h.ownerUsername && h.ownerUsername.toLowerCase().includes(q))
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

const tcpPortsCount = computed(() => {
  if (!activeSession.value?.networkInfo?.listeningPorts) return 0;
  return activeSession.value.networkInfo.listeningPorts.filter(
    (p) => p.proto && p.proto.toUpperCase().includes('TCP')
  ).length;
});

const udpPortsCount = computed(() => {
  if (!activeSession.value?.networkInfo?.listeningPorts) return 0;
  return activeSession.value.networkInfo.listeningPorts.filter(
    (p) => p.proto && p.proto.toUpperCase().includes('UDP')
  ).length;
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
    if (openSessions.value.length === 0) {
      localStorage.removeItem('hcp_remote_sessions_state');
      return;
    }
    const state = {
      sessions: openSessions.value.map((s) => ({
        hostId: s.host.id,
        displayName: s.displayName,
        activeView: s.activeView || 'terminal',
        groupId: s.groupId,
      })),
      groups: tabGroups.value,
      activeSessionIndex: activeSessionIndex.value,
    };
    localStorage.setItem('hcp_remote_sessions_state', JSON.stringify(state));
  } catch (e) {}
};

const restorePersistedSessions = async () => {
  // Guard: Never re-run restoration if sessions are already loaded in memory
  if (openSessions.value.length > 0) return;

  const queryHostId = route.query.hostId as string;
  if (queryHostId) {
    // Clear query parameter immediately so refreshing the page won't force re-opening
    router.replace({ path: route.path, query: {} });
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
    if (!parsed) return;

    if (Array.isArray(parsed.groups)) {
      tabGroups.value = parsed.groups;
    }

    if (!Array.isArray(parsed.sessions) || parsed.sessions.length === 0) return;

    // Deduplicate sessions by hostId to clean up any past corrupted storage
    const seenHostIds = new Set<string>();
    for (const sInfo of parsed.sessions) {
      if (seenHostIds.has(sInfo.hostId)) continue;
      seenHostIds.add(sInfo.hostId);
      const host = hosts.value.find((h) => h.id === sInfo.hostId);
      if (host) {
        const resolvedView: any = sInfo.activeView || 'terminal';
        const session: OpenSession = reactive({
          id: `session-${Date.now()}-${Math.random().toString(36).substring(2, 7)}`,
          host,
          displayName: sInfo.displayName || host.name,
          activeView: resolvedView,
          connected: false,
          connecting: false,
          groupId: sInfo.groupId,
        });
        openSessions.value.push(session);
      }
    }

    if (openSessions.value.length > 0) {
      let targetIdx = -1;
      if (
        typeof parsed.activeSessionIndex === 'number' &&
        parsed.activeSessionIndex >= -1 &&
        parsed.activeSessionIndex < openSessions.value.length
      ) {
        targetIdx = parsed.activeSessionIndex;
      }
      activeSessionIndex.value = targetIdx;

      if (targetIdx >= 0 && openSessions.value[targetIdx]) {
        await nextTick();
        const activeS = openSessions.value[targetIdx];
        if (activeS.activeView === 'terminal') {
          await ensureTerminalReady(activeS);
          fetchHostTelemetry(activeS);
        } else {
          fetchHostTelemetry(activeS);
        }
      }
    }
    saveSessionsState();
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
const connectHost = async (host: RemoteHost, forceNew = false, defaultGroupId?: string) => {
  if (isRdpHost(host)) {
    downloadRdpFile(host);
    return;
  }

  if (!forceNew) {
    const existingIdx = openSessions.value.findIndex((s) => s.host.id === host.id);
    if (existingIdx >= 0) {
      activeSessionIndex.value = existingIdx;
      saveSessionsState();
      const existingSession = openSessions.value[existingIdx];
      await ensureTerminalReady(existingSession);
      return;
    }
  }

  // Calculate instance count for tab label
  const count = openSessions.value.filter((s) => s.host.id === host.id).length;
  const tabName = count > 0 ? `${host.name} #${count + 1}` : host.name;

  const session: OpenSession = reactive({
    id: `session-${Date.now()}-${Math.random().toString(36).substring(2, 7)}`,
    host,
    displayName: tabName,
    activeView: 'terminal',
    connected: false,
    connecting: true,
    groupId: defaultGroupId,
  });

  openSessions.value.push(session);
  activeSessionIndex.value = openSessions.value.length - 1;
  saveSessionsState();

  await ensureTerminalReady(session);
  fetchHostTelemetry(session);
};

const duplicateSession = async (session: OpenSession, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  await connectHost(session.host, true, session.groupId);
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
    if (session.term.options.fontSize !== terminalFontSize.value) {
      session.term.options.fontSize = terminalFontSize.value;
    }
    setTimeout(() => {
      try {
        session.fitAddon?.fit();
        session.term?.focus();
        if (
          session.ws &&
          session.ws.readyState === WebSocket.OPEN &&
          session.term &&
          session.term.cols > 0 &&
          session.term.rows > 0
        ) {
          session.ws.send(
            JSON.stringify({
              type: 'resize',
              cols: session.term.cols,
              rows: session.term.rows,
            })
          );
        }
      } catch (e) {}
    }, 60);
  }
};

// Close Session
const closeSession = (sessionOrIdx: OpenSession | number | string, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  let idx = -1;
  if (typeof sessionOrIdx === 'number') {
    idx = sessionOrIdx;
  } else if (typeof sessionOrIdx === 'string') {
    idx = openSessions.value.findIndex((s) => s.id === sessionOrIdx);
  } else if (sessionOrIdx && typeof sessionOrIdx === 'object') {
    idx = openSessions.value.findIndex((s) => s.id === sessionOrIdx.id);
  }
  if (idx < 0 || idx >= openSessions.value.length) return;

  const s = openSessions.value[idx];
  if (s) {
    if (s.heartbeatTimer) {
      clearInterval(s.heartbeatTimer);
      s.heartbeatTimer = undefined;
    }
    if (s.resizeObserver) {
      try {
        s.resizeObserver.disconnect();
      } catch (e) {}
      s.resizeObserver = undefined;
    }
    if (s.ws) {
      try {
        if (s.ws.readyState === WebSocket.OPEN) {
          s.ws.send(JSON.stringify({ type: 'disconnect' }));
        }
        s.ws.onopen = null;
        s.ws.onmessage = null;
        s.ws.onerror = null;
        s.ws.onclose = null;
        s.ws.close();
      } catch (e) {}
      s.ws = undefined;
    }
    if (s.term) {
      try {
        s.term.dispose();
      } catch (e) {}
      s.term = undefined;
    }
  }
  openSessions.value.splice(idx, 1);
  if (activeSessionIndex.value >= openSessions.value.length) {
    activeSessionIndex.value = openSessions.value.length - 1;
  }

  // Clear query parameter if route has hostId so refresh doesn't reopen it
  if (route.query.hostId) {
    router.replace({ path: route.path, query: {} });
  }

  saveSessionsState();
};

// Clear Terminal Buffer & Scrollback manually if requested by user
const clearTerminal = (session: OpenSession) => {
  if (session.term) {
    session.term.clear();
    session.term.focus();
  }
};

// Terminal Font Size & Zoom Controls
const DEFAULT_TERMINAL_FONT_SIZE = 15;
const MIN_TERMINAL_FONT_SIZE = 9;
const MAX_TERMINAL_FONT_SIZE = 28;

const loadTerminalFontSize = (): number => {
  try {
    const saved = localStorage.getItem('hcp_terminal_font_size');
    if (saved) {
      const parsed = parseInt(saved, 10);
      if (!isNaN(parsed) && parsed >= MIN_TERMINAL_FONT_SIZE && parsed <= MAX_TERMINAL_FONT_SIZE) {
        // Automatically migrate users from older default (13px or 14px) to the new 15px default
        if (parsed === 13 || parsed === 14) {
          localStorage.setItem('hcp_terminal_font_size', String(DEFAULT_TERMINAL_FONT_SIZE));
          return DEFAULT_TERMINAL_FONT_SIZE;
        }
        return parsed;
      }
    }
  } catch (e) {}
  return DEFAULT_TERMINAL_FONT_SIZE;
};

const terminalFontSize = ref<number>(loadTerminalFontSize());
const zoomHudMessage = ref<string>('');
let zoomHudTimer: any = null;

const showZoomHud = (size: number) => {
  zoomHudMessage.value = `Terminal Font: ${size}px`;
  if (zoomHudTimer) clearTimeout(zoomHudTimer);
  zoomHudTimer = setTimeout(() => {
    zoomHudMessage.value = '';
  }, 1200);
};

const setTerminalFontSize = (size: number) => {
  const clamped = Math.min(MAX_TERMINAL_FONT_SIZE, Math.max(MIN_TERMINAL_FONT_SIZE, size));
  terminalFontSize.value = clamped;
  try {
    localStorage.setItem('hcp_terminal_font_size', String(clamped));
  } catch (e) {}

  showZoomHud(clamped);

  openSessions.value.forEach((sess) => {
    if (sess.term) {
      sess.term.options.fontSize = clamped;
      if (sess.fitAddon && sess.activeView === 'terminal') {
        try {
          sess.fitAddon.fit();
          if (sess.ws && sess.ws.readyState === WebSocket.OPEN && sess.term.cols > 0 && sess.term.rows > 0) {
            sess.ws.send(
              JSON.stringify({
                type: 'resize',
                cols: sess.term.cols,
                rows: sess.term.rows,
              })
            );
          }
        } catch (e) {}
      }
    }
  });
};

const zoomInTerminal = () => {
  setTerminalFontSize(terminalFontSize.value + 1);
};

const zoomOutTerminal = () => {
  setTerminalFontSize(terminalFontSize.value - 1);
};

const resetZoomTerminal = () => {
  setTerminalFontSize(DEFAULT_TERMINAL_FONT_SIZE);
};

// MobaXterm-style Terminal Idle Detection
let idleCheckerInterval: any = null;
const startIdleDetection = () => {
  if (idleCheckerInterval) clearInterval(idleCheckerInterval);
  idleCheckerInterval = setInterval(() => {
    const now = Date.now();
    openSessions.value.forEach((sess) => {
      if (sess.connected && sess.lastActivity) {
        sess.isIdle = (now - sess.lastActivity) > 12000;
      }
    });
  }, 3000);
};

const stopIdleDetection = () => {
  if (idleCheckerInterval) {
    clearInterval(idleCheckerInterval);
    idleCheckerInterval = null;
  }
};

// Background-immune Heartbeat Management for Terminal Sessions
const startSessionHeartbeat = (session: OpenSession, ws: WebSocket) => {
  stopSessionHeartbeat(session);

  // Use Web Worker for heartbeat: browser NEVER throttles Web Worker timers when minimized or tab is in background
  try {
    let worker: Worker | null = null;
    try {
      worker = new Worker('/terminal-worker.js');
    } catch (_) {
      // Fallback to inline Blob worker if static file is unavailable
      const workerScript = `
        let timer = null;
        self.onmessage = function(e) {
          if (e.data === 'start') {
            if (timer) clearInterval(timer);
            timer = setInterval(function() {
              self.postMessage('ping');
            }, 5000);
          } else if (e.data === 'stop') {
            if (timer) clearInterval(timer);
            timer = null;
          }
        };
      `;
      const blob = new Blob([workerScript], { type: 'application/javascript' });
      worker = new Worker(URL.createObjectURL(blob));
    }

    if (worker) {
      worker.onmessage = (e) => {
        if (e.data === 'ping' && session.ws === ws && ws.readyState === WebSocket.OPEN) {
          try {
            ws.send(JSON.stringify({ type: 'ping' }));
          } catch (_) {}
        }
      };
      worker.postMessage('start');
      session.heartbeatWorker = worker;
      return;
    }
  } catch (_) {}

  // Fallback to standard setInterval if Web Worker is restricted
  session.heartbeatTimer = setInterval(() => {
    if (session.ws === ws && ws.readyState === WebSocket.OPEN) {
      try {
        ws.send(JSON.stringify({ type: 'ping' }));
      } catch (_) {}
    }
  }, 5000);
};

const stopSessionHeartbeat = (session: OpenSession) => {
  if (session.heartbeatWorker) {
    try {
      session.heartbeatWorker.postMessage('stop');
      session.heartbeatWorker.terminate();
    } catch (_) {}
    session.heartbeatWorker = undefined;
  }
  if (session.heartbeatTimer) {
    clearInterval(session.heartbeatTimer);
    session.heartbeatTimer = undefined;
  }
};

// Connect WebSocket Stream for Terminal Session
const connectWsTerminal = (session: OpenSession, isReconnect: boolean = false, forceNew: boolean = false) => {
  // Teardown previous heartbeat & ws if present
  stopSessionHeartbeat(session);
  if (session.ws) {
    try {
      session.ws.onopen = null;
      session.ws.onmessage = null;
      session.ws.onerror = null;
      session.ws.onclose = null;
      session.ws.close();
    } catch (e) {}
    session.ws = undefined;
  }

  session.connecting = true;
  session.connected = false;

  // Open WebSocket with token and detected terminal dimensions
  const token = authStore.token || sessionStorage.getItem('hcp_session') || '';
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const initialCols = session.term && session.term.cols > 0 ? session.term.cols : 80;
  const initialRows = session.term && session.term.rows > 0 ? session.term.rows : 24;
  const wsUrl = `${protocol}//${window.location.host}/ws/remote-host?token=${encodeURIComponent(token)}&hostId=${session.host.id}&sessionId=${encodeURIComponent(session.id)}&cols=${initialCols}&rows=${initialRows}${forceNew ? '&forceNew=true' : ''}`;
  const ws = new WebSocket(wsUrl);
  session.ws = ws;

  ws.onopen = () => {
    if (session.ws !== ws) return;
    session.connecting = false;
    session.connected = true;
    session.reconnectAttempts = 0;
    session.reconnectPendingOnVisible = false;
    session.lastActivity = Date.now();
    session.isIdle = false;

    // Display connection banner
    if (session.term) {
      if (isReconnect) {
        session.term.write('\r\n\x1b[32m[Connected to ' + session.host.name + ' (' + session.host.host + ')]\x1b[0m\r\n\r\n');
      } else if (!session.term.buffer.active.cursorY) {
        session.term.write('\r\n\x1b[32m[Connected to ' + session.host.name + ' (' + session.host.host + ')]\x1b[0m\r\n\r\n');
      }
      try {
        session.fitAddon?.fit();
        session.term.focus();
      } catch (e) {}
      setTimeout(() => {
        try {
          session.fitAddon?.fit();
          session.term?.focus();
        } catch (_) {}
      }, 50);
    }

    // Send auth handshake with sessionId for persistent session re-attach
    ws.send(
      JSON.stringify({
        type: 'auth',
        token: token,
        hostConfigId: session.host.id,
        sessionId: session.id,
        cols: session.term?.cols || initialCols,
        rows: session.term?.rows || initialRows,
        forceNew: forceNew,
      })
    );

    // Send explicit resize message to ensure PTY adopts dimensions immediately
    if (session.term && session.term.cols > 0 && session.term.rows > 0) {
      ws.send(
        JSON.stringify({
          type: 'resize',
          cols: session.term.cols,
          rows: session.term.rows,
        })
      );
    }

    startSessionHeartbeat(session, ws);
  };

  ws.onmessage = (ev) => {
    if (session.ws !== ws) return;
    session.lastActivity = Date.now();
    session.isIdle = false;
    try {
      const msg = JSON.parse(ev.data);
      if (msg.type === 'data' && msg.data) {
        if (!session.connected) {
          session.connecting = false;
          session.connected = true;
        }
        session.term?.write(msg.data);
      } else if (msg.type === 'connected') {
        session.connecting = false;
        session.connected = true;
        session.reconnectAttempts = 0;
        session.reconnectPendingOnVisible = false;
        // On connected ack, ensure remote PTY matches client dimensions and focus terminal
        if (session.term) {
          try {
            session.fitAddon?.fit();
            session.term.focus();
            if (session.term.cols > 0 && session.term.rows > 0 && ws.readyState === WebSocket.OPEN) {
              ws.send(JSON.stringify({ type: 'resize', cols: session.term.cols, rows: session.term.rows }));
            }
          } catch (e) {}
        }
      } else if (msg.type === 'error') {
        session.term?.write(`\r\n\x1b[31m[Error: ${msg.message}]\x1b[0m\r\n`);
      } else if (msg.type === 'ping') {
        // Echo back pong to server to keep Cloudflare connection alive
        if (ws.readyState === WebSocket.OPEN) {
          try {
            ws.send(JSON.stringify({ type: 'pong' }));
          } catch (_) {}
        }
      } else if (msg.type === 'disconnected') {
        session.connecting = false;
        session.connected = false;
        stopSessionHeartbeat(session);
        session.term?.write('\r\n\x1b[33m[Session closed - Click Reconnect to resume]\x1b[0m\r\n');
      }
    } catch (e) {
      if (!session.connected) {
        session.connecting = false;
        session.connected = true;
      }
      session.term?.write(ev.data);
    }
  };

  ws.onclose = (ev: CloseEvent) => {
    if (session.ws !== ws) return;
    session.connecting = false;
    session.connected = false;
    stopSessionHeartbeat(session);

    // Clean close or user logout (code 1000)
    if (ev.code === 1000) {
      session.reconnectAttempts = 0;
      session.reconnectPendingOnVisible = false;
      session.term?.write('\r\n\x1b[33m[Session closed - Click Reconnect to resume]\x1b[0m\r\n');
      return;
    }

    // If browser window is currently minimized / tab is hidden:
    // Mark as pending re-attach silently when window is restored
    if (document.hidden) {
      session.reconnectPendingOnVisible = true;
      return;
    }

    // Normal foreground reconnect with controlled attempts and backoff
    session.reconnectAttempts = (session.reconnectAttempts || 0) + 1;
    if (session.reconnectAttempts <= 5) {
      const delay = Math.min(1500 * Math.pow(1.5, session.reconnectAttempts - 1), 6000);
      setTimeout(() => {
        if (!session.connected && session.term && session.ws === ws && !document.hidden) {
          reconnectTerminal(session, false);
        }
      }, delay);
    } else {
      session.term?.write('\r\n\x1b[31m[Connection interrupted. Click Reconnect to resume]\x1b[0m\r\n');
    }
  };

  ws.onerror = () => {
    if (session.ws !== ws) return;
    session.connecting = false;
    session.connected = false;
    stopSessionHeartbeat(session);
  };
};

// Reconnect Terminal preserving output history & scrollback buffer
const reconnectTerminal = (session: OpenSession, forceNew: boolean = false) => {
  session.connecting = true;
  session.connected = false;
  reconnectMenuSessionId.value = null;

  if (session.term) {
    if (forceNew) {
      session.term.write('\r\n\x1b[33m[Resetting terminal shell and reconnecting...]\x1b[0m\r\n');
    } else {
      session.term.write('\r\n\x1b[33m[Reconnecting to terminal stream...]\x1b[0m\r\n');
    }
  }

  // If term already exists in container, reconnect WebSocket directly without wiping buffer or disposing term
  if (session.term && session.fitAddon) {
    connectWsTerminal(session, true, forceNew);
  } else {
    initXterm(session);
  }
};

// Initialize xterm.js Terminal with WebSocket & Heartbeats
const initXterm = (session: OpenSession) => {
  // If WebSocket is already open or connecting, do not re-create
  if (session.ws && (session.ws.readyState === WebSocket.OPEN || session.ws.readyState === WebSocket.CONNECTING)) {
    try {
      session.fitAddon?.fit();
      session.term?.focus();
    } catch (e) {}
    return;
  }

  // If terminal instance already exists, reconnect stream without wiping
  if (session.term && session.fitAddon) {
    connectWsTerminal(session, false);
    return;
  }

  const container = document.getElementById(`terminal-container-${session.id}`);
  if (!container) return;
  container.innerHTML = '';

  const term = new Terminal({
    cursorBlink: true,
    fontSize: terminalFontSize.value || DEFAULT_TERMINAL_FONT_SIZE,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: themeStore.isDark ? darkTerminalTheme : lightTerminalTheme,
    allowProposedApi: true,
    scrollback: 10000,
  });

  const fitAddon = new FitAddon();
  term.loadAddon(fitAddon);
  term.loadAddon(new WebLinksAddon());
  term.open(container);

  // Attach key event handler for zoom shortcuts inside xterm.js
  term.attachCustomKeyEventHandler((event: KeyboardEvent) => {
    if (event.ctrlKey || event.metaKey) {
      if (event.key === '=' || event.key === '+' || event.code === 'Equal' || event.code === 'NumpadAdd') {
        if (event.type === 'keydown') {
          zoomInTerminal();
        }
        return false;
      }
      if (event.key === '-' || event.key === '_' || event.code === 'Minus' || event.code === 'NumpadSubtract') {
        if (event.type === 'keydown') {
          zoomOutTerminal();
        }
        return false;
      }
      if (event.key === '0' || event.code === 'Digit0' || event.code === 'Numpad0') {
        if (event.type === 'keydown') {
          resetZoomTerminal();
        }
        return false;
      }
    }
    return true;
  });

  // Support Ctrl + MouseWheel zoom on terminal container
  container.addEventListener(
    'wheel',
    (e: WheelEvent) => {
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        if (e.deltaY < 0) {
          zoomInTerminal();
        } else if (e.deltaY > 0) {
          zoomOutTerminal();
        }
      }
    },
    { passive: false }
  );

  // Initial fit attempt
  try {
    fitAddon.fit();
    term.focus();
  } catch (e) {}

  session.term = term;
  session.fitAddon = fitAddon;

  // Set up listeners on terminal to send data and resize dynamically to active WebSocket
  term.onData((data) => {
    session.lastActivity = Date.now();
    session.isIdle = false;
    if (session.ws && session.ws.readyState === WebSocket.OPEN) {
      try {
        session.ws.send(JSON.stringify({ type: 'input', data }));
      } catch (err) {
        console.warn('Failed to send terminal input data:', err);
        session.connected = false;
      }
    } else if (session.connected) {
      session.connected = false;
    }
  });

  term.onResize(({ cols, rows }) => {
    if (session.ws && session.ws.readyState === WebSocket.OPEN) {
      session.ws.send(JSON.stringify({ type: 'resize', cols, rows }));
    }
  });

  // Attach ResizeObserver to container so any layout change automatically fits terminal
  if (session.resizeObserver) {
    try {
      session.resizeObserver.disconnect();
    } catch (e) {}
    session.resizeObserver = undefined;
  }

  const resizeObserver = new ResizeObserver(() => {
    if (session.term && session.fitAddon && session.activeView === 'terminal') {
      try {
        session.fitAddon.fit();
        if (session.ws && session.ws.readyState === WebSocket.OPEN && session.term.cols > 0 && session.term.rows > 0) {
          session.ws.send(
            JSON.stringify({
              type: 'resize',
              cols: session.term.cols,
              rows: session.term.rows,
            })
          );
        }
      } catch (e) {}
    }
  });
  resizeObserver.observe(container);
  session.resizeObserver = resizeObserver;

  // Establish initial WebSocket connection
  connectWsTerminal(session, false);
};

// Watch activeSession changes to ensure fitted terminal
watch(
  [activeSessionIndex, () => activeSession.value?.activeView],
  async () => {
    if (!activeSession.value) return;
    if (activeSession.value.activeView === 'terminal') {
      await ensureTerminalReady(activeSession.value);
    }
    saveSessionsState();
  }
);

// Watch theme changes to update xterm.js theme in real-time
watch(
  () => themeStore.currentTheme,
  (newTheme) => {
    const activeTheme = newTheme === 'light' ? lightTerminalTheme : darkTerminalTheme;
    openSessions.value.forEach((s) => {
      if (s.term) {
        s.term.options.theme = activeTheme;
      }
    });
  }
);

// Auto-recover disconnected terminal WebSockets on tab focus / visibilitychange
const handleVisibilityOrFocus = () => {
  if (document.visibilityState === 'visible') {
    openSessions.value.forEach((sess) => {
      if (sess.activeView === 'terminal' && sess.term) {
        // If connection is already open and healthy, refit dimensions and do NOT reconnect!
        if (sess.ws && sess.ws.readyState === WebSocket.OPEN) {
          sess.connected = true;
          sess.connecting = false;
          sess.reconnectPendingOnVisible = false;
          try {
            sess.fitAddon?.fit();
            if (sess.term.cols > 0 && sess.term.rows > 0) {
              sess.ws.send(JSON.stringify({ type: 'resize', cols: sess.term.cols, rows: sess.term.rows }));
            }
          } catch (_) {}
          return;
        }

        // If connection was disconnected while in background, seamlessly re-attach to backend persistent session
        if (sess.reconnectPendingOnVisible || (!sess.connected && !sess.connecting)) {
          sess.reconnectPendingOnVisible = false;
          sess.reconnectAttempts = 0;
          reconnectTerminal(sess);
        }
      }
    });
  }
};

// =================================================================
// CHROME-STYLE TAB GROUPS & DRAG-AND-DROP REORDERING
// =================================================================
const splitLayout = ref<'single' | 'vertical' | 'horizontal'>('single');

// Drag and drop states
const draggedGlobalIndex = ref<number | null>(null);
const dragOverGlobalIndex = ref<number | null>(null);
const dragOverGroupId = ref<string | null>(null);
const dragDropPosition = ref<'left' | 'right' | null>(null);

// Context menus state
const tabContextMenu = ref<{
  show: boolean;
  x: number;
  y: number;
  globalIndex: number;
  session: OpenSession | null;
}>({
  show: false,
  x: 0,
  y: 0,
  globalIndex: -1,
  session: null,
});

const groupMenu = ref<{
  show: boolean;
  x: number;
  y: number;
  group: TabGroup | null;
  editingName: string;
}>({
  show: false,
  x: 0,
  y: 0,
  group: null,
  editingName: '',
});

interface TabItemCluster {
  type: 'group';
  group: TabGroup;
  sessions: { session: OpenSession; globalIndex: number }[];
}

interface UngroupedCluster {
  type: 'ungrouped';
  session: OpenSession;
  globalIndex: number;
}

type TabCluster = TabItemCluster | UngroupedCluster;

// Clusters for rendering tabs seamlessly like Google Chrome
const tabClusters = computed<TabCluster[]>(() => {
  const result: TabCluster[] = [];
  const processedGroupIds = new Set<string>();

  openSessions.value.forEach((session, idx) => {
    if (session.groupId) {
      if (processedGroupIds.has(session.groupId)) return;
      processedGroupIds.add(session.groupId);
      const grp = tabGroups.value.find((g) => g.id === session.groupId);
      if (grp) {
        const memberSessions: { session: OpenSession; globalIndex: number }[] = [];
        openSessions.value.forEach((s, sIdx) => {
          if (s.groupId === grp.id) {
            memberSessions.push({ session: s, globalIndex: sIdx });
          }
        });
        result.push({
          type: 'group',
          group: grp,
          sessions: memberSessions,
        });
      } else {
        result.push({
          type: 'ungrouped',
          session,
          globalIndex: idx,
        });
      }
    } else {
      result.push({
        type: 'ungrouped',
        session,
        globalIndex: idx,
      });
    }
  });

  return result;
});

// Drag and drop handlers
const handleTabDragStart = (e: DragEvent, globalIndex: number) => {
  draggedGlobalIndex.value = globalIndex;
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', String(globalIndex));
  }
};

const handleTabDragOver = (e: DragEvent, targetGlobalIndex: number) => {
  e.preventDefault();
  if (draggedGlobalIndex.value === null || draggedGlobalIndex.value === targetGlobalIndex) return;
  dragOverGlobalIndex.value = targetGlobalIndex;
  dragOverGroupId.value = null;
  const targetEl = e.currentTarget as HTMLElement;
  const rect = targetEl.getBoundingClientRect();
  dragDropPosition.value = e.clientX < rect.left + rect.width / 2 ? 'left' : 'right';
};

const handleTabDrop = (e: DragEvent, targetGlobalIndex: number) => {
  e.preventDefault();
  if (draggedGlobalIndex.value === null) {
    resetDrag();
    return;
  }
  const srcIdx = draggedGlobalIndex.value;
  if (srcIdx === targetGlobalIndex) {
    resetDrag();
    return;
  }

  const dragged = openSessions.value[srcIdx];
  const target = openSessions.value[targetGlobalIndex];

  // If dropped on a grouped tab, inherit the target's group
  dragged.groupId = target.groupId;

  // Move in array
  openSessions.value.splice(srcIdx, 1);
  let insertIdx = targetGlobalIndex;
  if (dragDropPosition.value === 'right') {
    insertIdx = srcIdx < targetGlobalIndex ? targetGlobalIndex : targetGlobalIndex + 1;
  } else {
    insertIdx = srcIdx < targetGlobalIndex ? targetGlobalIndex - 1 : targetGlobalIndex;
  }
  if (insertIdx < 0) insertIdx = 0;
  if (insertIdx > openSessions.value.length) insertIdx = openSessions.value.length;

  openSessions.value.splice(insertIdx, 0, dragged);
  activeSessionIndex.value = insertIdx;

  saveSessionsState();
  resetDrag();
};

const handleGroupPillDragOver = (e: DragEvent, groupId: string) => {
  e.preventDefault();
  dragOverGroupId.value = groupId;
  dragOverGlobalIndex.value = null;
};

const handleGroupPillDrop = (e: DragEvent, group: TabGroup) => {
  e.preventDefault();
  if (draggedGlobalIndex.value === null) {
    resetDrag();
    return;
  }
  const dragged = openSessions.value[draggedGlobalIndex.value];
  if (dragged) {
    dragged.groupId = group.id;
    group.collapsed = false;
    saveSessionsState();
  }
  resetDrag();
};

const handleTabBarDrop = (e: DragEvent) => {
  if (draggedGlobalIndex.value === null) return;
  const dragged = openSessions.value[draggedGlobalIndex.value];
  if (dragged && dragged.groupId) {
    dragged.groupId = undefined;
    saveSessionsState();
  }
  resetDrag();
};

const resetDrag = () => {
  draggedGlobalIndex.value = null;
  dragOverGlobalIndex.value = null;
  dragOverGroupId.value = null;
  dragDropPosition.value = null;
};

// Group actions
const createGroupForSession = (session: OpenSession, name?: string) => {
  const color = CHROME_COLORS[(tabGroups.value.length) % CHROME_COLORS.length].hex;
  const groupName = name || session.host.name;
  const newGroup: TabGroup = {
    id: `tg-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
    name: groupName,
    color,
    collapsed: false,
  };
  tabGroups.value.push(newGroup);
  session.groupId = newGroup.id;
  saveSessionsState();
  return newGroup;
};

const addSessionToGroup = (session: OpenSession, groupId: string) => {
  session.groupId = groupId;
  const grp = tabGroups.value.find((g) => g.id === groupId);
  if (grp) grp.collapsed = false;
  saveSessionsState();
};

const removeSessionFromGroup = (session: OpenSession) => {
  const oldGroupId = session.groupId;
  session.groupId = undefined;
  if (oldGroupId) {
    const remaining = openSessions.value.filter((s) => s.groupId === oldGroupId);
    if (remaining.length === 0) {
      tabGroups.value = tabGroups.value.filter((g) => g.id !== oldGroupId);
    }
  }
  saveSessionsState();
};

const ungroupTabs = (groupId: string) => {
  openSessions.value.forEach((s) => {
    if (s.groupId === groupId) {
      s.groupId = undefined;
    }
  });
  tabGroups.value = tabGroups.value.filter((g) => g.id !== groupId);
  saveSessionsState();
};

const closeGroupTabs = (groupId: string) => {
  const toRemoveIndices: number[] = [];
  openSessions.value.forEach((s, idx) => {
    if (s.groupId === groupId) toRemoveIndices.push(idx);
  });
  for (let i = toRemoveIndices.length - 1; i >= 0; i--) {
    closeSession(toRemoveIndices[i]);
  }
  tabGroups.value = tabGroups.value.filter((g) => g.id !== groupId);
  saveSessionsState();
};

const autoGroupByHost = () => {
  let colorIdx = 0;
  const hostGroupMap = new Map<string, string>();
  tabGroups.value = [];

  openSessions.value.forEach((s) => {
    let gId = hostGroupMap.get(s.host.id);
    if (!gId) {
      const color = CHROME_COLORS[colorIdx % CHROME_COLORS.length].hex;
      colorIdx++;
      const newGroup: TabGroup = {
        id: `tg-${s.host.id}-${Date.now()}`,
        name: s.host.name,
        color,
        collapsed: false,
      };
      tabGroups.value.push(newGroup);
      gId = newGroup.id;
      hostGroupMap.set(s.host.id, gId);
    }
    s.groupId = gId;
  });
  saveSessionsState();
};

const toggleGroupCollapse = (groupId: string) => {
  const g = tabGroups.value.find((item) => item.id === groupId);
  if (g) {
    g.collapsed = !g.collapsed;
    saveSessionsState();
  }
};

const openTabContextMenu = (e: MouseEvent, session: OpenSession, globalIndex: number) => {
  e.preventDefault();
  closeAllContextMenus();
  tabContextMenu.value = {
    show: true,
    x: Math.min(e.clientX, window.innerWidth - 220),
    y: Math.min(e.clientY, window.innerHeight - 250),
    globalIndex,
    session,
  };
};

const openGroupContextMenu = (e: MouseEvent, group: TabGroup) => {
  e.preventDefault();
  e.stopPropagation();
  closeAllContextMenus();
  groupMenu.value = {
    show: true,
    x: Math.min(e.clientX, window.innerWidth - 240),
    y: Math.min(e.clientY, window.innerHeight - 250),
    group,
    editingName: group.name,
  };
};

const setGroupColor = (group: TabGroup, colorHex: string) => {
  group.color = colorHex;
  saveSessionsState();
};

const saveGroupName = () => {
  if (groupMenu.value.group && groupMenu.value.editingName.trim()) {
    groupMenu.value.group.name = groupMenu.value.editingName.trim();
    saveSessionsState();
  }
  groupMenu.value.show = false;
};

const closeOtherSessions = (keepGlobalIndex: number) => {
  const keepSession = openSessions.value[keepGlobalIndex];
  if (!keepSession) return;
  for (let i = openSessions.value.length - 1; i >= 0; i--) {
    if (i !== keepGlobalIndex) {
      closeSession(i);
    }
  }
  activeSessionIndex.value = 0;
  saveSessionsState();
};

const closeAllContextMenus = () => {
  tabContextMenu.value.show = false;
  groupMenu.value.show = false;
  reconnectMenuSessionId.value = null;
};

const handleGlobalKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') {
    closeAllContextMenus();
  }

  // Keyboard shortcut to reconnect active terminal (Ctrl/Cmd + Shift + R)
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'R' || e.key === 'r') && activeSession.value?.activeView === 'terminal') {
    e.preventDefault();
    reconnectTerminal(activeSession.value, true);
    return;
  }

  // Keyboard shortcuts for terminal zoom (Ctrl/Cmd + Plus, Minus, 0)
  if ((e.ctrlKey || e.metaKey) && activeSession.value?.activeView === 'terminal') {
    if (e.key === '=' || e.key === '+' || e.code === 'Equal' || e.code === 'NumpadAdd') {
      e.preventDefault();
      zoomInTerminal();
    } else if (e.key === '-' || e.key === '_' || e.code === 'Minus' || e.code === 'NumpadSubtract') {
      e.preventDefault();
      zoomOutTerminal();
    } else if (e.key === '0' || e.code === 'Digit0' || e.code === 'Numpad0') {
      e.preventDefault();
      resetZoomTerminal();
    }
  }
};

const switchActiveView = async (session: OpenSession, viewName: 'terminal' | 'dashboard' | 'processes' | 'services' | 'network' | 'sftp' | 'firewall') => {
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

const downloadRdpFile = (host?: RemoteHost) => {
  if (!host) return;
  const targetHost = host.host || '127.0.0.1';
  const targetPort = host.port && host.port !== 22 ? host.port : 3389;
  const username = host.username || '';

  // Generate standard Microsoft Remote Desktop (.rdp) configuration
  const rdpLines = [
    `full address:s:${targetHost}:${targetPort}`,
    username ? `username:s:${username}` : '',
    'screen mode id:i:2',
    'use multimon:i:0',
    'desktopwidth:i:1920',
    'desktopheight:i:1080',
    'session bpp:i:32',
    'winposstr:s:0,1,0,0,800,600',
    'compression:i:1',
    'keyboardhook:i:2',
    'audiocapturemode:i:0',
    'videoplaybackmode:i:1',
    'connection type:i:7',
    'networkautodetect:i:1',
    'bandwidthautodetect:i:1',
    'displayconnectionbar:i:1',
    'enableworkspacereconnect:i:0',
    'disable wallpaper:i:0',
    'allow font smoothing:i:1',
    'allow desktop composition:i:1',
    'disable full window drag:i:0',
    'disable menu anims:i:0',
    'disable themes:i:0',
    'disable cursor setting:i:0',
    'bitmapcachepersistenable:i:1',
    'audiomode:i:0',
    'redirectprinters:i:0',
    'redirectcomports:i:0',
    'redirectsmartcards:i:0',
    'redirectclipboard:i:1',
    'redirectposdevices:i:0',
    'autoreconnection enabled:i:1',
    'authentication level:i:2',
    'prompt for credentials:i:0',
    'negotiate security layer:i:1',
    'remoteapplicationmode:i:0',
    'alternate shell:s:',
    'shell working directory:s:',
    'gatewayhostname:s:',
    'gatewayusagemethod:i:4',
    'gatewaycredentialssource:i:4',
    'gatewayprofileusagemethod:i:0',
    'promptcredentialonce:i:0',
    'gatewaybrokeringtype:i:0',
    'use redirection server name:i:0',
    'rdgiskdcproxy:i:0',
    'kdcproxyname:s:',
  ];

  const content = rdpLines.filter(Boolean).join('\r\n') + '\r\n';
  const blob = new Blob([content], { type: 'application/x-rdp;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  const safeName = (host.name || host.host || 'remote-desktop').replace(/[^a-zA-Z0-9_-]/g, '_');
  a.href = url;
  a.download = `${safeName}.rdp`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
  showNotice(`.rdp file for "${host.name || host.host}" downloaded successfully. Please launch the file to connect via Remote Desktop.`);
};

// Fetch Host Telemetry
const fetchHostTelemetry = async (session: OpenSession) => {
  if (!session || !session.host?.id) return;
  isTelemetryLoading.value = true;
  try {
    const [metricsRes, procRes, srvRes, netRes, fwRes] = await Promise.all([
      axios.get(`/api/v1/remote-host/${session.host.id}/metrics`).catch(() => ({ data: { success: false } })),
      axios.get(`/api/v1/remote-host/${session.host.id}/processes`).catch(() => ({ data: { success: false } })),
      axios.get(`/api/v1/remote-host/${session.host.id}/services`).catch(() => ({ data: { success: false } })),
      axios.get(`/api/v1/remote-host/${session.host.id}/network`).catch(() => ({ data: { success: false } })),
      axios.get(`/api/v1/remote-host/${session.host.id}/firewall`).catch(() => ({ data: { success: false } })),
    ]);

    if (metricsRes.data?.success && metricsRes.data.data) session.metrics = metricsRes.data.data;
    if (procRes.data?.success && Array.isArray(procRes.data.data)) session.processes = procRes.data.data;
    if (srvRes.data?.success && Array.isArray(srvRes.data.data)) session.services = srvRes.data.data;
    if (netRes.data?.success && netRes.data.data) session.networkInfo = netRes.data.data;
    if (fwRes.data?.success && fwRes.data.data) session.firewall = fwRes.data.data;
  } catch (err) {
    console.warn('Telemetry poll error:', err);
  } finally {
    isTelemetryLoading.value = false;
  }
};

// =================================================================
// FIREWALL MANAGEMENT STATE & ACTIONS
// =================================================================
const isFirewallModalOpen = ref(false);
const isFirewallSubmitting = ref(false);
const isFirewallToggling = ref(false);
const firewallSearch = ref('');
const firewallAppType = ref('Custom');

const APPLICATION_TYPES: Record<string, { protocol: string; port: string; desc: string }> = {
  Custom: { protocol: 'TCP', port: '', desc: '' },
  SSH: { protocol: 'TCP', port: '22', desc: 'Allow SSH' },
  TELNET: { protocol: 'TCP', port: '23', desc: 'Allow Telnet' },
  MySQL: { protocol: 'TCP', port: '3306', desc: 'Allow MySQL' },
  'SQL Server': { protocol: 'TCP', port: '1433', desc: 'Allow SQL Server' },
  HTTP: { protocol: 'TCP', port: '80', desc: 'Allow HTTP' },
  HTTPS: { protocol: 'TCP', port: '443', desc: 'Allow HTTPS' },
  'All TCP': { protocol: 'TCP', port: 'ALL', desc: 'Allow All TCP' },
  'All UDP': { protocol: 'UDP', port: 'ALL', desc: 'Allow All UDP' },
  ALL: { protocol: 'ALL', port: 'ALL', desc: 'Allow All Request' },
};

const handleAppTypeChange = () => {
  const preset = APPLICATION_TYPES[firewallAppType.value];
  if (preset) {
    firewallForm.value.protocol = preset.protocol;
    firewallForm.value.portRange = preset.port;
    if (preset.desc) {
      firewallForm.value.description = preset.desc;
    }
  }
};

const firewallForm = ref({
  protocol: 'TCP',
  portRange: '',
  sourceIp: '0.0.0.0/0',
  action: 'ALLOW',
  description: '',
});

const openAddRuleModal = () => {
  firewallAppType.value = 'Custom';
  firewallForm.value = {
    protocol: 'TCP',
    portRange: '',
    sourceIp: '0.0.0.0/0',
    action: 'ALLOW',
    description: '',
  };
  isFirewallModalOpen.value = true;
};

const handleAddFirewallRule = async () => {
  if (!activeSession.value) return;
  if (!firewallForm.value.portRange && firewallForm.value.protocol !== 'ICMP') {
    alert('Please enter a port range (e.g., 22, 80,443, 8000:8500, or ALL)');
    return;
  }
  isFirewallSubmitting.value = true;
  try {
    const res = await axios.post(`/api/v1/remote-host/${activeSession.value.host.id}/firewall/rules`, firewallForm.value);
    if (res.data?.success) {
      isFirewallModalOpen.value = false;
      await fetchHostTelemetry(activeSession.value);
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to add firewall rule');
  } finally {
    isFirewallSubmitting.value = false;
  }
};

const handleDeleteFirewallRule = async (ruleId: string) => {
  if (!activeSession.value) return;
  if (!confirm('Are you sure you want to delete this firewall rule?')) return;
  try {
    await axios.delete(`/api/v1/remote-host/${activeSession.value.host.id}/firewall/rules/${ruleId}`);
    await fetchHostTelemetry(activeSession.value);
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete firewall rule');
  }
};

const handleToggleFirewall = async (enable: boolean) => {
  if (!activeSession.value) return;
  const actionText = enable ? 'enable' : 'disable';
  if (!confirm(`Are you sure you want to ${actionText} the firewall service on ${activeSession.value.host.name}?`)) return;
  isFirewallToggling.value = true;
  try {
    await axios.post(`/api/v1/remote-host/${activeSession.value.host.id}/firewall/toggle`, { enable });
    await fetchHostTelemetry(activeSession.value);
  } catch (err: any) {
    alert(err.response?.data?.error || `Failed to ${actionText} firewall`);
  } finally {
    isFirewallToggling.value = false;
  }
};

const filteredFirewallRules = computed(() => {
  if (!activeSession.value?.firewall?.rules) return [];
  let list = activeSession.value.firewall.rules;
  if (firewallSearch.value) {
    const q = firewallSearch.value.toLowerCase();
    list = list.filter((r: any) =>
      (r.protocol && r.protocol.toLowerCase().includes(q)) ||
      (r.portRange && r.portRange.toLowerCase().includes(q)) ||
      (r.sourceIp && r.sourceIp.toLowerCase().includes(q)) ||
      (r.description && r.description.toLowerCase().includes(q))
    );
  }
  return list;
});

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
const sftpMode = ref<'local-remote' | 'server-server'>('local-remote');
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

// Selection & Sorting states
const selectedLocalId = ref<string | null>(null);
const selectedRemoteFile = ref<any | null>(null);
const selectedSourceFile = ref<any | null>(null);

const destSortKey = ref<'name' | 'size' | 'modTime'>('name');
const destSortAsc = ref<boolean>(true);
const localSortKey = ref<'name' | 'size' | 'modified'>('name');
const localSortAsc = ref<boolean>(true);
const sourceSortKey = ref<'name' | 'size' | 'modTime'>('name');
const sourceSortAsc = ref<boolean>(true);

// Delete Confirmation Modal State
const showSftpDeleteModal = ref<boolean>(false);
const sftpItemToDelete = ref<{ name: string; path: string; isDir: boolean } | null>(null);
const deletingSftpItem = ref<boolean>(false);

// Create Folder Modal State
const showSftpMkdirModal = ref<boolean>(false);
const sftpNewFolderName = ref<string>('');
const creatingSftpFolder = ref<boolean>(false);

// Console logs visibility toggle
const showSftpLogs = ref<boolean>(false);

interface LocalStagedFile {
  id: string;
  file: File;
  name: string;
  size: number;
  type: string;
  lastModified: number;
  status: 'ready' | 'uploading' | 'completed' | 'failed';
}

const isDragOverLocal = ref(false);
const isDragOverRemote = ref(false);
const sftpFileInput = ref<HTMLInputElement | null>(null);
const localStagedFiles = ref<LocalStagedFile[]>([]);
const sftpCommandLogs = ref<{ time: string; type: 'status' | 'command' | 'response' | 'error'; text: string }[]>([]);
const sftpTransferQueue = ref<any[]>([]);
const sftpQueueTab = ref<'queued' | 'success' | 'failed'>('queued');

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

    sftpSourceHostId.value = 'local';
    sftpMode.value = 'local-remote';
  }

  sftpSourceCurrentPath.value = '';
  sftpSourceInputPath.value = '';
  sftpDestCurrentPath.value = '';
  sftpDestInputPath.value = '';
  selectedLocalId.value = null;
  selectedRemoteFile.value = null;
  selectedSourceFile.value = null;
  sftpCommandLogs.value = [];
  isSftpModalOpen.value = true;

  logSftp('status', 'SFTP Transfer Session initialized.');
  if (sftpSourceHostId.value !== 'local') fetchSourceSftpFiles('');
  if (sftpDestHostId.value && sftpDestHostId.value !== 'local') fetchDestSftpFiles('');
};

const handleDestHostChange = () => {
  if (sftpDestHostId.value) {
    selectedRemoteFile.value = null;
    sftpDestCurrentPath.value = '';
    sftpDestInputPath.value = '';
    fetchDestSftpFiles('');
  } else {
    sftpDestFiles.value = [];
    selectedRemoteFile.value = null;
  }
};

const disconnectDestHost = () => {
  sftpDestHostId.value = '';
  sftpDestFiles.value = [];
  selectedRemoteFile.value = null;
  logSftp('status', 'Disconnected from remote server.');
};

// Fetch files for Source pane
const fetchSourceSftpFiles = async (path?: string) => {
  if (sftpSourceHostId.value === 'local') return;
  const host = sourceHostObj.value;
  if (!host) return;

  sftpSourceLoading.value = true;
  const targetPath = path !== undefined ? path : sftpSourceCurrentPath.value;
  logSftp('command', `[Source: ${host.name}] CWD "${targetPath || '~'}"`);

  try {
    const res = await axios.get(`/api/v1/remote-host/${host.id}/sftp/list`, {
      params: { path: targetPath || '' },
    });
    if (res.data.success && res.data.data) {
      sftpSourceFiles.value = res.data.data;
      sftpSourceCurrentPath.value = res.data.currentPath || targetPath || '/';
      sftpSourceInputPath.value = sftpSourceCurrentPath.value;
      selectedSourceFile.value = null;
      logSftp('response', `[Source: ${host.name}] Directory listing of "${sftpSourceCurrentPath.value}" OK (${sftpSourceFiles.value.length} items).`);
    } else {
      sftpSourceFiles.value = [];
    }
  } catch (err: any) {
    const errMsg = err.response?.data?.error || err.message;
    logSftp('error', `[Source: ${host.name}] Failed: ${errMsg}`);
    showNotice(`Source error: ${errMsg}`, 'error');
    sftpSourceFiles.value = [];
  } finally {
    sftpSourceLoading.value = false;
  }
};

// Fetch files for Destination pane
const fetchDestSftpFiles = async (path?: string) => {
  if (sftpDestHostId.value === 'local') return;
  const host = destHostObj.value;
  if (!host) return;

  sftpDestLoading.value = true;
  const targetPath = path !== undefined ? path : sftpDestCurrentPath.value;
  logSftp('command', `[Destination: ${host.name}] CWD "${targetPath || '~'}"`);

  try {
    const res = await axios.get(`/api/v1/remote-host/${host.id}/sftp/list`, {
      params: { path: targetPath || '' },
    });
    if (res.data.success && res.data.data) {
      sftpDestFiles.value = res.data.data;
      sftpDestCurrentPath.value = res.data.currentPath || targetPath || '/';
      sftpDestInputPath.value = sftpDestCurrentPath.value;
      selectedRemoteFile.value = null;
      logSftp('response', `[Destination: ${host.name}] Directory listing of "${sftpDestCurrentPath.value}" OK (${sftpDestFiles.value.length} items).`);
    } else {
      sftpDestFiles.value = [];
    }
  } catch (err: any) {
    const errMsg = err.response?.data?.error || err.message;
    logSftp('error', `[Destination: ${host.name}] Failed: ${errMsg}`);
    showNotice(`Destination error: ${errMsg}`, 'error');
    sftpDestFiles.value = [];
  } finally {
    sftpDestLoading.value = false;
  }
};

const navigateDestParent = () => {
  const parts = sftpDestCurrentPath.value.split('/').filter(Boolean);
  parts.pop();
  const parentPath = parts.length === 0 ? '/' : '/' + parts.join('/');
  fetchDestSftpFiles(parentPath);
};

const navigateSourceParent = () => {
  const parts = sftpSourceCurrentPath.value.split('/').filter(Boolean);
  parts.pop();
  const parentPath = parts.length === 0 ? '/' : '/' + parts.join('/');
  fetchSourceSftpFiles(parentPath);
};

// Swap Source and Destination hosts & paths
const swapSourceAndDest = () => {
  const tempHostId = sftpSourceHostId.value;
  sftpSourceHostId.value = sftpDestHostId.value;
  sftpDestHostId.value = tempHostId;

  const tempPath = sftpSourceCurrentPath.value;
  sftpSourceCurrentPath.value = sftpDestCurrentPath.value;
  sftpDestCurrentPath.value = tempPath;

  selectedLocalId.value = null;
  selectedRemoteFile.value = null;
  selectedSourceFile.value = null;

  logSftp('status', 'Swapped Source and Destination endpoints.');
  if (sftpSourceHostId.value !== 'local') fetchSourceSftpFiles();
  if (sftpDestHostId.value !== 'local') fetchDestSftpFiles();
};

// Server-to-Server FXP or Remote-to-Local transfer execution
const transferSourceFileToDest = async (fileName: string) => {
  const srcHost = sourceHostObj.value;
  const dstHost = destHostObj.value;

  const srcBase = sftpSourceCurrentPath.value === '/' ? '' : sftpSourceCurrentPath.value.replace(/\/+$/, '');
  const srcPath = `${srcBase}/${fileName}`;

  const dstBase = sftpDestCurrentPath.value === '/' ? '' : sftpDestCurrentPath.value.replace(/\/+$/, '');
  const dstPath = `${dstBase}/${fileName}`;

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
      error: '',
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
        showNotice(`Transfer "${fileName}" to [${dstHost.name}] completed.`);
        await fetchDestSftpFiles();
      }
    } catch (err: any) {
      queueItem.status = 'failed';
      const msg = err.response?.data?.error || err.message || 'Server-to-Server transfer failed';
      queueItem.error = msg;
      logSftp('error', `Transfer failed: ${msg}`);
      showNotice(`Transfer failed: ${msg}`, 'error');
    }
  }
};

const transferDestFileToSource = async (fileName: string) => {
  const srcHost = destHostObj.value;
  const dstHost = sourceHostObj.value;
  if (!srcHost || !dstHost) return;

  const srcBase = sftpDestCurrentPath.value === '/' ? '' : sftpDestCurrentPath.value.replace(/\/+$/, '');
  const srcPath = `${srcBase}/${fileName}`;

  const dstBase = sftpSourceCurrentPath.value === '/' ? '' : sftpSourceCurrentPath.value.replace(/\/+$/, '');
  const dstPath = `${dstBase}/${fileName}`;

  logSftp('command', `FXP "${srcPath}" [${srcHost.name}] ➔ "${dstPath}" [${dstHost.name}]`);
  const queueItem = {
    name: `${fileName} ([${srcHost.name}] ➔ [${dstHost.name}])`,
    size: 'Transferring...',
    status: 'transferring' as const,
    progress: 50,
    error: '',
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
      showNotice(`Transfer "${fileName}" to [${dstHost.name}] completed.`);
      await fetchSourceSftpFiles();
    }
  } catch (err: any) {
    queueItem.status = 'failed';
    const msg = err.response?.data?.error || err.message || 'Transfer failed';
    queueItem.error = msg;
    logSftp('error', `Transfer failed: ${msg}`);
    showNotice(`Transfer failed: ${msg}`, 'error');
  }
};

// Download Remote File directly to Browser
const downloadRemoteFile = (file: any) => {
  const host = destHostObj.value;
  if (!host) return;
  const dstBase = sftpDestCurrentPath.value === '/' ? '' : sftpDestCurrentPath.value.replace(/\/+$/, '');
  const filePath = `${dstBase}/${file.name}`;
  logSftp('command', `RETR "${filePath}" from [${host.name}] (Download to Local Computer)`);
  const queueItem = {
    name: `${file.name} ([${host.name}] ➔ Local)`,
    size: formatFileSize(file.size),
    status: 'success' as const,
    progress: 100,
  };
  sftpTransferQueue.value.unshift(queueItem);
  const url = `/api/v1/remote-host/${host.id}/sftp/download?path=${encodeURIComponent(filePath)}`;
  window.open(url, '_blank');
};

// Local Staging Upload to Destination
const uploadStagedFile = async (staged: LocalStagedFile) => {
  const dstHost = destHostObj.value;
  if (!dstHost) {
    showNotice('Please select a remote server destination for uploads.', 'error');
    return;
  }

  const dstBase = sftpDestCurrentPath.value === '/' ? '' : sftpDestCurrentPath.value.replace(/\/+$/, '');
  const targetPath = `${dstBase}/${staged.name}`;

  staged.status = 'uploading';
  const queueItem = {
    name: `${staged.name} (Local ➔ [${dstHost.name}]:${targetPath})`,
    size: formatFileSize(staged.size),
    status: 'transferring' as const,
    progress: 0,
    error: '',
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
      showNotice(`Uploaded "${staged.name}" to [${dstHost.name}] successfully.`);
      await fetchDestSftpFiles();
    }
  } catch (err: any) {
    staged.status = 'failed';
    queueItem.status = 'failed';
    const msg = err.response?.data?.error || err.message || 'Upload failed';
    queueItem.error = msg;
    logSftp('error', `Upload failed: ${msg}`);
    showNotice(`Upload failed: ${msg}`, 'error');
  }
};

const uploadAllStagedFiles = async () => {
  const readyFiles = localStagedFiles.value.filter((f) => f.status === 'ready' || f.status === 'failed');
  for (const f of readyFiles) {
    await uploadStagedFile(f);
  }
};

// Central Transfer Buttons Logic
const canTransferLeftToRight = computed(() => {
  if (!destHostObj.value) return false;
  if (sftpMode.value === 'local-remote') {
    return localStagedFiles.value.some((f) => f.status === 'ready' || f.status === 'failed') || selectedLocalId.value !== null;
  } else {
    return !!sourceHostObj.value && !!selectedSourceFile.value && !selectedSourceFile.value.isDir;
  }
});

const canTransferRightToLeft = computed(() => {
  if (!destHostObj.value) return false;
  if (sftpMode.value === 'local-remote') {
    return !!selectedRemoteFile.value && !selectedRemoteFile.value.isDir;
  } else {
    return !!sourceHostObj.value && !!selectedRemoteFile.value && !selectedRemoteFile.value.isDir;
  }
});

const handleTransferLeftToRight = async () => {
  if (sftpMode.value === 'local-remote') {
    if (selectedLocalId.value) {
      const item = localStagedFiles.value.find((f) => f.id === selectedLocalId.value);
      if (item) {
        await uploadStagedFile(item);
        return;
      }
    }
    await uploadAllStagedFiles();
  } else {
    if (selectedSourceFile.value && !selectedSourceFile.value.isDir) {
      await transferSourceFileToDest(selectedSourceFile.value.name);
    }
  }
};

const handleTransferRightToLeft = async () => {
  if (selectedRemoteFile.value && !selectedRemoteFile.value.isDir) {
    if (sftpMode.value === 'local-remote') {
      downloadRemoteFile(selectedRemoteFile.value);
    } else {
      await transferDestFileToSource(selectedRemoteFile.value.name);
    }
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
      id: `local-${Date.now()}-${i}-${Math.random().toString(36).substring(2, 7)}`,
      file: f,
      name: f.name,
      size: f.size,
      type: f.type || getFileTypeLabel(f.name, false),
      lastModified: f.lastModified || Date.now(),
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
        id: `local-${Date.now()}-${i}-${Math.random().toString(36).substring(2, 7)}`,
        file: f,
        name: f.name,
        size: f.size,
        type: f.type || getFileTypeLabel(f.name, false),
        lastModified: f.lastModified || Date.now(),
        status: 'ready',
      });
    }
  }
};

const removeLocalStagedFile = (id: string) => {
  localStagedFiles.value = localStagedFiles.value.filter((f) => f.id !== id);
  if (selectedLocalId.value === id) selectedLocalId.value = null;
};

const clearLocalStagedFiles = () => {
  localStagedFiles.value = [];
  selectedLocalId.value = null;
};

// Remote File Actions: Delete & Mkdir
const confirmDeleteRemoteItem = (file: any) => {
  const host = destHostObj.value;
  if (!host) return;
  const filePath = sftpDestCurrentPath.value === '/' ? `/${file.name}` : `${sftpDestCurrentPath.value.replace(/\/+$/, '')}/${file.name}`;
  sftpItemToDelete.value = {
    name: file.name,
    path: filePath,
    isDir: file.isDir,
  };
  showSftpDeleteModal.value = true;
};

const executeDeleteRemoteItem = async () => {
  const host = destHostObj.value;
  if (!host || !sftpItemToDelete.value) return;
  deletingSftpItem.value = true;
  try {
    const res = await axios.delete(`/api/v1/remote-host/${host.id}/sftp/delete`, {
      params: { path: sftpItemToDelete.value.path },
    });
    if (res.data.success) {
      logSftp('response', `Deleted "${sftpItemToDelete.value.name}" successfully.`);
      showNotice(`"${sftpItemToDelete.value.name}" deleted successfully.`);
      showSftpDeleteModal.value = false;
      if (selectedRemoteFile.value?.name === sftpItemToDelete.value?.name) {
        selectedRemoteFile.value = null;
      }
      sftpItemToDelete.value = null;
      await fetchDestSftpFiles();
    }
  } catch (err: any) {
    logSftp('error', `Failed to delete: ${err.response?.data?.error || err.message}`);
    alert(err.response?.data?.error || 'Failed to delete item');
  } finally {
    deletingSftpItem.value = false;
  }
};

const openCreateFolderModal = () => {
  sftpNewFolderName.value = '';
  showSftpMkdirModal.value = true;
};

const executeCreateFolder = async () => {
  const host = destHostObj.value;
  if (!host || !sftpNewFolderName.value.trim()) return;
  creatingSftpFolder.value = true;
  const folderName = sftpNewFolderName.value.trim();
  const targetPath = sftpDestCurrentPath.value === '/' ? `/${folderName}` : `${sftpDestCurrentPath.value.replace(/\/+$/, '')}/${folderName}`;
  try {
    const res = await axios.post(`/api/v1/remote-host/${host.id}/sftp/mkdir`, {
      path: targetPath,
    });
    if (res.data.success) {
      logSftp('response', `Directory "${folderName}" created successfully.`);
      showNotice(`Directory "${folderName}" created successfully.`);
      showSftpMkdirModal.value = false;
      sftpNewFolderName.value = '';
      await fetchDestSftpFiles();
    }
  } catch (err: any) {
    logSftp('error', `Failed to create folder: ${err.response?.data?.error || err.message}`);
    alert(err.response?.data?.error || 'Failed to create folder');
  } finally {
    creatingSftpFolder.value = false;
  }
};

// Filtered & Sorted Lists
const toggleDestSort = (key: 'name' | 'size' | 'modTime') => {
  if (destSortKey.value === key) {
    destSortAsc.value = !destSortAsc.value;
  } else {
    destSortKey.value = key;
    destSortAsc.value = true;
  }
};

const toggleLocalSort = (key: 'name' | 'size' | 'modified') => {
  if (localSortKey.value === key) {
    localSortAsc.value = !localSortAsc.value;
  } else {
    localSortKey.value = key;
    localSortAsc.value = true;
  }
};

const toggleSourceSort = (key: 'name' | 'size' | 'modTime') => {
  if (sourceSortKey.value === key) {
    sourceSortAsc.value = !sourceSortAsc.value;
  } else {
    sourceSortKey.value = key;
    sourceSortAsc.value = true;
  }
};

const filteredDestFiles = computed(() => {
  if (!sftpDestFilter.value) return sftpDestFiles.value;
  const q = sftpDestFilter.value.toLowerCase();
  return sftpDestFiles.value.filter((f) => f.name.toLowerCase().includes(q));
});

const sortedDestFiles = computed(() => {
  const list = filteredDestFiles.value.slice();
  list.sort((a, b) => {
    if (a.isDir && !b.isDir) return -1;
    if (!a.isDir && b.isDir) return 1;

    let comp = 0;
    if (destSortKey.value === 'name') {
      comp = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    } else if (destSortKey.value === 'size') {
      comp = (a.size || 0) - (b.size || 0);
    } else if (destSortKey.value === 'modTime') {
      const timeA = new Date(a.modTime || 0).getTime();
      const timeB = new Date(b.modTime || 0).getTime();
      comp = timeA - timeB;
    }
    return destSortAsc.value ? comp : -comp;
  });
  return list;
});

const sortedLocalFiles = computed(() => {
  let list = localStagedFiles.value.slice();
  if (sftpSourceFilter.value) {
    const q = sftpSourceFilter.value.toLowerCase();
    list = list.filter((f) => f.name.toLowerCase().includes(q));
  }
  list.sort((a, b) => {
    let comp = 0;
    if (localSortKey.value === 'name') {
      comp = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    } else if (localSortKey.value === 'size') {
      comp = a.size - b.size;
    } else if (localSortKey.value === 'modified') {
      comp = (a.lastModified || 0) - (b.lastModified || 0);
    }
    return localSortAsc.value ? comp : -comp;
  });
  return list;
});

const filteredSourceFiles = computed(() => {
  if (!sftpSourceFilter.value) return sftpSourceFiles.value;
  const q = sftpSourceFilter.value.toLowerCase();
  return sftpSourceFiles.value.filter((f) => f.name.toLowerCase().includes(q));
});

const sortedSourceFiles = computed(() => {
  const list = filteredSourceFiles.value.slice();
  list.sort((a, b) => {
    if (a.isDir && !b.isDir) return -1;
    if (!a.isDir && b.isDir) return 1;

    let comp = 0;
    if (sourceSortKey.value === 'name') {
      comp = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    } else if (sourceSortKey.value === 'size') {
      comp = (a.size || 0) - (b.size || 0);
    } else if (sourceSortKey.value === 'modTime') {
      const timeA = new Date(a.modTime || 0).getTime();
      const timeB = new Date(b.modTime || 0).getTime();
      comp = timeA - timeB;
    }
    return sourceSortAsc.value ? comp : -comp;
  });
  return list;
});

function formatFileSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function formatSftpDate(dateVal: any): string {
  if (!dateVal) return '-';
  const d = new Date(dateVal);
  if (isNaN(d.getTime())) return '-';
  return d.toLocaleString('en-US', {
    month: 'short',
    day: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: true,
  });
}

function getFileTypeLabel(name: string, isDir: boolean): string {
  if (isDir) return 'Folder';
  const ext = name.split('.').pop()?.toLowerCase() || '';
  if (['yaml', 'yml'].includes(ext)) return 'YAML Config';
  if (['json'].includes(ext)) return 'JSON Doc';
  if (['log'].includes(ext)) return 'Log File';
  if (['tar', 'gz', 'zip', 'bz2', 'tgz', 'xz', '7z'].includes(ext)) return 'Archive';
  if (['sh', 'bash', 'zsh'].includes(ext)) return 'Shell Script';
  if (['conf', 'cfg', 'ini', 'env'].includes(ext)) return 'Config File';
  if (['txt', 'md'].includes(ext)) return 'Text Doc';
  if (['bin', 'iso', 'img'].includes(ext)) return 'Binary';
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
    const isNew = !hostForm.value.id;
    const res = await axios.post('/api/v1/remote-host', hostForm.value);
    if (res.data.success) {
      isHostModalOpen.value = false;
      const savedHost = res.data.data;
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
      selectedProtocolPreset.value = 'ssh';
      await fetchHosts();
      if (isNew && savedHost) {
        const targetHost = hosts.value.find((h) => h.id === savedHost.id) || savedHost;
        await connectHost(targetHost);
      } else if (!isNew && savedHost) {
        openSessions.value.forEach((s) => {
          if (s.host.id === savedHost.id) {
            s.host = { ...s.host, ...savedHost };
            s.displayName = savedHost.name;
          }
        });
        saveSessionsState();
      }
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to save host');
  }
};

// Share Access Handlers
const openShareModal = async (host: RemoteHost, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  selectedHostForShare.value = host;
  isShareModalOpen.value = true;
  shareForm.value = { userId: '', permission: 'read' };
  await Promise.all([fetchHostShares(host.id), fetchAvailableUsers()]);
};

const fetchHostShares = async (hostId: string) => {
  isShareLoading.value = true;
  try {
    const res = await axios.get(`/api/v1/remote-host/${hostId}/shares`);
    if (res.data.success) {
      hostShares.value = res.data.data || [];
    }
  } catch (err: any) {
    console.error('Failed to fetch shares:', err);
    hostShares.value = [];
  } finally {
    isShareLoading.value = false;
  }
};

const fetchAvailableUsers = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host/users');
    if (res.data.success) {
      availableUsers.value = res.data.data || [];
    }
  } catch (err: any) {
    console.error('Failed to fetch available users:', err);
  }
};

const handleGrantShare = async () => {
  if (!selectedHostForShare.value || !shareForm.value.userId) return;
  isShareSubmitting.value = true;
  try {
    const res = await axios.post(`/api/v1/remote-host/${selectedHostForShare.value.id}/shares`, {
      userId: Number(shareForm.value.userId),
      permission: shareForm.value.permission,
    });
    if (res.data.success) {
      shareForm.value.userId = '';
      await fetchHostShares(selectedHostForShare.value.id);
      await fetchHosts();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to grant share access');
  } finally {
    isShareSubmitting.value = false;
  }
};

const handleRevokeShare = async (targetUserId: number) => {
  if (!selectedHostForShare.value) return;
  if (!confirm('Are you sure you want to revoke access for this user?')) return;
  try {
    const res = await axios.delete(`/api/v1/remote-host/${selectedHostForShare.value.id}/shares/${targetUserId}`);
    if (res.data.success) {
      await fetchHostShares(selectedHostForShare.value.id);
      await fetchHosts();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to revoke access');
  }
};

const openEditHostModal = (host: RemoteHost, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  selectedProtocolPreset.value = isRdpHost(host) ? 'rdp' : 'ssh';
  testResult.value = null;
  hostForm.value = {
    id: host.id,
    name: host.name,
    host: host.host,
    port: host.port,
    username: host.username,
    authType: host.authType || 'password',
    password: '',
    sshKey: '',
    groupName: host.groupName || 'Default',
    tags: host.tags || [],
  };
  isHostModalOpen.value = true;
};

const openDeleteHostModal = (host: RemoteHost, event?: MouseEvent) => {
  if (event) event.stopPropagation();
  hostToDelete.value = host;
  showDeleteHostModal.value = true;
};

const executeDeleteHost = async () => {
  if (!hostToDelete.value) return;
  deletingHost.value = true;
  try {
    const res = await axios.delete(`/api/v1/remote-host/${hostToDelete.value.id}`);
    if (res.data.success) {
      for (let i = openSessions.value.length - 1; i >= 0; i--) {
        if (openSessions.value[i].host.id === hostToDelete.value.id) {
          closeSession(i);
        }
      }
      await fetchHosts();
      showNotice(`Server "${hostToDelete.value.name}" deleted successfully.`);
      showDeleteHostModal.value = false;
      hostToDelete.value = null;
    }
  } catch (err: any) {
    showNotice(err.response?.data?.error || 'Failed to delete host', 'error');
  } finally {
    deletingHost.value = false;
  }
};

const handleCreateGroup = async () => {
  const gName = newGroupName.value.trim();
  if (!gName) {
    alert('Please enter a group name.');
    return;
  }

  if (groupCreationMode.value === 'new') {
    isGroupModalOpen.value = false;
    selectedProtocolPreset.value = 'ssh';
    hostForm.value = {
      id: '',
      name: '',
      host: '',
      port: 22,
      username: '',
      authType: 'password',
      password: '',
      sshKey: '',
      groupName: gName,
      tags: [],
    };
    isHostModalOpen.value = true;
    newGroupName.value = '';
    selectedHostIdsForGroup.value = [];
  } else {
    if (selectedHostIdsForGroup.value.length === 0) {
      alert('Please select at least one server to include in this group.');
      return;
    }

    isSubmittingGroup.value = true;
    try {
      const res = await axios.post('/api/v1/remote-host/batch-group', {
        hostIds: selectedHostIdsForGroup.value,
        groupName: gName,
      });

      if (res.data?.success) {
        isGroupModalOpen.value = false;
        newGroupName.value = '';
        selectedHostIdsForGroup.value = [];
        groupSearchQuery.value = '';
        await fetchHosts();
      } else {
        alert(res.data?.error || 'Failed to update server group');
      }
    } catch (err: any) {
      alert(err.response?.data?.error || err.message || 'Failed to update server group');
    } finally {
      isSubmittingGroup.value = false;
    }
  }
};

const canScrollLeft = ref(false);
const canScrollRight = ref(false);

const updateTabScrollState = () => {
  if (!tabBarRef.value) return;
  const el = tabBarRef.value;
  canScrollLeft.value = el.scrollLeft > 6;
  canScrollRight.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 6;
};

const scrollTabBar = (direction: 'left' | 'right') => {
  if (!tabBarRef.value) return;
  const el = tabBarRef.value;
  const distance = Math.max(200, Math.floor(el.clientWidth * 0.45));
  el.scrollBy({
    left: direction === 'left' ? -distance : distance,
    behavior: 'smooth',
  });
  setTimeout(updateTabScrollState, 320);
};

const handleTabBarWheel = (e: WheelEvent) => {
  if (!tabBarRef.value) return;
  if (e.deltaY !== 0) {
    tabBarRef.value.scrollLeft += e.deltaY * 0.85;
    updateTabScrollState();
  }
};

watch([() => openSessions.value.length, () => tabClusters.value], async () => {
  await nextTick();
  updateTabScrollState();
});

const handleWindowResize = () => {
  updateTabScrollState();
  if (activeSession.value) {
    if (activeSession.value.activeView === 'terminal') {
      try {
        activeSession.value.fitAddon?.fit();
        if (
          activeSession.value.ws &&
          activeSession.value.ws.readyState === WebSocket.OPEN &&
          activeSession.value.term &&
          activeSession.value.term.cols > 0 &&
          activeSession.value.term.rows > 0
        ) {
          activeSession.value.ws.send(
            JSON.stringify({
              type: 'resize',
              cols: activeSession.value.term.cols,
              rows: activeSession.value.term.rows,
            })
          );
        }
      } catch (e) {}
    }
  }
};

watch(activeSessionIndex, async (newIdx) => {
  if (newIdx >= 0 && openSessions.value[newIdx]) {
    const s = openSessions.value[newIdx];
    await nextTick();
    if (s.activeView === 'terminal') {
      s.fitAddon?.fit();
    }
    if (tabBarRef.value) {
      const activeEl = tabBarRef.value.querySelector('.active-session-tab') as HTMLElement;
      if (activeEl) {
        activeEl.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' });
      }
      setTimeout(updateTabScrollState, 350);
    }
  }
});

onMounted(async () => {
  await fetchHosts();
  await restorePersistedSessions();
  startIdleDetection();
  window.addEventListener('click', closeAllContextMenus);
  window.addEventListener('keydown', handleGlobalKeydown);
  window.addEventListener('resize', handleWindowResize);
  document.addEventListener('visibilitychange', handleVisibilityOrFocus);
  window.addEventListener('focus', handleVisibilityOrFocus);
  setTimeout(updateTabScrollState, 250);
});

onUnmounted(() => {
  stopIdleDetection();
  window.removeEventListener('click', closeAllContextMenus);
  window.removeEventListener('keydown', handleGlobalKeydown);
  window.removeEventListener('resize', handleWindowResize);
  document.removeEventListener('visibilitychange', handleVisibilityOrFocus);
  window.removeEventListener('focus', handleVisibilityOrFocus);
  openSessions.value.forEach((s) => {
    stopSessionHeartbeat(s);
    if (s.resizeObserver) {
      try {
        s.resizeObserver.disconnect();
      } catch (e) {}
    }
    if (s.ws) s.ws.close();
    if (s.term) s.term.dispose();
  });
});
</script>

<template>
  <div class="h-screen w-screen bg-slate-100 dark:bg-[#14161b] text-slate-800 dark:text-slate-200 font-sans flex flex-col overflow-hidden selection:bg-brand-500/30">
    
    <!-- Top Header Bar -->
    <header class="h-12 bg-white dark:bg-[#1b1e26] border-b border-slate-200 dark:border-slate-800 px-4 flex items-center justify-between shrink-0 shadow-sm">
      <div class="flex items-center gap-2.5">
        <SquareTerminal class="w-4 h-4 text-blue-600 dark:text-brand-400" />
        <h1 class="text-xs font-semibold text-slate-900 dark:text-white tracking-wide">Remote Server (SSH & SFTP)</h1>
      </div>

      <div class="flex items-center gap-2">
        <ThemeToggle variant="button" />
        <button
          @click="handleBackToPortal"
          class="flex items-center gap-1.5 px-3 py-1 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#242833] border border-slate-300 dark:border-slate-700/60 text-xs text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white dark:hover:border-slate-500 transition font-medium shadow-sm"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Back to Portal</span>
        </button>
      </div>
    </header>

    <!-- Sub-Header Tabs & Quick Actions Bar -->
    <div class="h-10 bg-slate-50 dark:bg-[#1b1e26] border-b border-slate-200 dark:border-slate-800/80 px-2 sm:px-3 flex items-center gap-1.5 text-xs shrink-0 select-none overflow-hidden">
      <!-- Left Fixed Group: SERVERS, Add (+), SFTP Transfer -->
      <div class="flex items-center gap-1.5 shrink-0 pr-2 border-r border-slate-200 dark:border-slate-800/80">
        <!-- Servers Menu Button -->
        <button
          @click="activeSessionIndex = -1"
          :class="[
            'shrink-0 flex items-center gap-1.5 px-2.5 py-1 rounded-lg font-medium transition text-xs border whitespace-nowrap h-7 cursor-pointer',
            activeSessionIndex === -1
              ? 'bg-white text-blue-700 border-slate-300 dark:bg-slate-800 dark:text-white dark:border-slate-700 shadow-xs font-bold'
              : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/60 dark:hover:bg-slate-800/50'
          ]"
        >
          <Server class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400 shrink-0" />
          <span>SERVERS</span>
        </button>

        <!-- Quick Add Server (+) Button -->
        <button
          v-if="canManage"
          @click="openAddHostModal()"
          title="Add New Remote Server"
          class="shrink-0 p-1.5 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-200/60 dark:hover:bg-slate-800 transition h-7 flex items-center justify-center cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
        </button>

        <!-- Dual-Pane SFTP Transfer Button -->
        <button
          @click="openSftpModal()"
          class="shrink-0 flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] border border-slate-300 dark:border-slate-700/80 hover:border-slate-400 dark:hover:border-slate-600 transition font-medium text-xs shadow-xs whitespace-nowrap h-7 cursor-pointer"
          title="Open FileZilla Dual-Pane SFTP Transfer"
        >
          <Upload class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400 shrink-0" />
          <span class="hidden md:inline">SFTP TRANSFER</span>
          <span class="md:hidden">SFTP</span>
        </button>
      </div>

      <!-- Center Flexible Tabs Scroller Strip -->
      <div class="relative flex items-center flex-1 min-w-0 h-full overflow-hidden mx-1">
        
        <!-- Left Scroll Button (Visible when scrolled to the right) -->
        <button
          v-if="canScrollLeft"
          @click="scrollTabBar('left')"
          class="absolute left-0 z-20 h-7 w-6 flex items-center justify-center bg-white/95 dark:bg-[#1b1e26]/95 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded shadow-sm border border-slate-200 dark:border-slate-700 transition cursor-pointer backdrop-blur-xs"
          title="Scroll tabs left"
        >
          <ChevronLeft class="w-3.5 h-3.5" />
        </button>

        <!-- Left Overflow Fade Gradient -->
        <div
          v-if="canScrollLeft"
          class="absolute left-6 top-0 bottom-0 w-6 bg-gradient-to-r from-slate-50 dark:from-[#1b1e26] to-transparent pointer-events-none z-10"
        ></div>

        <!-- Open Session Tabs Track (Custom smooth scroll, completely hidden native scrollbar) -->
        <div
          ref="tabBarRef"
          class="flex items-center gap-1.5 px-1.5 flex-1 overflow-x-auto overflow-y-hidden select-none py-1 h-full scrollbar-none tab-bar-strip scroll-smooth"
          @dragover.prevent
          @drop.prevent="handleTabBarDrop"
          @wheel.passive="handleTabBarWheel"
          @scroll="updateTabScrollState"
        >
          <template v-for="cluster in tabClusters" :key="cluster.type === 'group' ? cluster.group.id : cluster.session.id">
            
            <!-- CHROME TAB GROUP -->
            <div
              v-if="cluster.type === 'group'"
              class="shrink-0 flex items-center gap-1 p-0.5 rounded-lg border transition h-7.5"
              :style="{
                borderColor: cluster.group.color + '55',
                backgroundColor: cluster.group.color + '10'
              }"
            >
              <!-- Group Header Pill -->
              <div
                @click="toggleGroupCollapse(cluster.group.id)"
                @contextmenu.prevent="openGroupContextMenu($event, cluster.group)"
                @dragover.prevent="handleGroupPillDragOver($event, cluster.group.id)"
                @drop.prevent="handleGroupPillDrop($event, cluster.group)"
                :class="[
                  'shrink-0 flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-bold font-mono transition cursor-pointer select-none group/grp whitespace-nowrap h-6.5',
                  dragOverGroupId === cluster.group.id ? 'ring-2 ring-white scale-105' : ''
                ]"
                :style="{
                  backgroundColor: cluster.group.color + '25',
                  color: cluster.group.color,
                  border: '1.5px solid ' + cluster.group.color + '70'
                }"
                :title="'Click to ' + (cluster.group.collapsed ? 'expand' : 'collapse') + ' | Right click to configure group'"
              >
                <span class="w-2 h-2 rounded-full shadow-sm shrink-0" :style="{ backgroundColor: cluster.group.color }"></span>
                <span class="truncate max-w-[120px]">{{ cluster.group.name }}</span>
                <span
                  v-if="cluster.group.collapsed"
                  class="px-1.5 py-0.2 rounded-full text-[10px] font-bold bg-slate-900/80 text-white shrink-0"
                >
                  {{ cluster.sessions.length }}
                </span>
                <ChevronDown
                  v-else
                  class="w-3 h-3 opacity-60 group-hover/grp:opacity-100 transition shrink-0"
                />
              </div>

              <!-- Member Tabs (Hidden when group is collapsed) -->
              <template v-if="!cluster.group.collapsed">
                <div
                  v-for="sItem in cluster.sessions"
                  :key="sItem.session.id"
                  draggable="true"
                  @dragstart="handleTabDragStart($event, sItem.globalIndex)"
                  @dragover.prevent="handleTabDragOver($event, sItem.globalIndex)"
                  @drop.prevent="handleTabDrop($event, sItem.globalIndex)"
                  @dragend="resetDrag"
                  @click="activeSessionIndex = sItem.globalIndex"
                  @contextmenu.prevent="openTabContextMenu($event, sItem.session, sItem.globalIndex)"
                  :class="[
                    'shrink-0 flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-mono transition cursor-pointer select-none relative group/tab border h-6.5 max-w-[200px]',
                    draggedGlobalIndex === sItem.globalIndex ? 'opacity-40 scale-95' : '',
                    dragOverGlobalIndex === sItem.globalIndex && dragDropPosition === 'left' ? 'border-l-2 border-l-white' : '',
                    dragOverGlobalIndex === sItem.globalIndex && dragDropPosition === 'right' ? 'border-r-2 border-r-white' : '',
                    activeSessionIndex === sItem.globalIndex
                      ? 'active-session-tab bg-white dark:bg-[#242833] border-slate-300 dark:border-slate-700 text-slate-900 dark:text-white shadow-xs font-semibold'
                      : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/50 dark:hover:bg-slate-800/40'
                  ]"
                  :style="{
                    borderLeftColor: activeSessionIndex === sItem.globalIndex ? cluster.group.color : undefined,
                    borderLeftWidth: activeSessionIndex === sItem.globalIndex ? '3px' : undefined
                  }"
                  :title="sItem.session.displayName || sItem.session.host.name"
                >
                  <GripVertical class="w-2.5 h-2.5 opacity-20 group-hover/tab:opacity-70 text-slate-400 cursor-grab shrink-0" />
                  <span
                    class="w-1.5 h-1.5 rounded-full transition-all shrink-0"
                    :class="sItem.session.connected ? (sItem.session.isIdle ? 'animate-pulse' : '') : (sItem.session.connecting ? 'animate-pulse' : 'ring-1 ring-red-500')"
                    :style="{ backgroundColor: sItem.session.connected ? cluster.group.color : (sItem.session.connecting ? cluster.group.color : '#ef4444') }"
                  ></span>
                  <span class="truncate whitespace-nowrap text-[11px]">{{ sItem.session.displayName || sItem.session.host.name }}</span>

                  <!-- Duplicate Tab Button -->
                  <button
                    @click.stop="duplicateSession(sItem.session)"
                    title="Duplicate tab in group"
                    class="p-0.5 hover:text-slate-900 dark:hover:text-white hover:bg-slate-200 dark:hover:bg-slate-700/50 rounded transition text-slate-400 shrink-0 cursor-pointer"
                  >
                    <Copy class="w-2.5 h-2.5" />
                  </button>

                  <!-- Close Tab Button -->
                  <button
                    @click.stop="closeSession(sItem.session)"
                    title="Close Tab"
                    class="p-0.5 hover:text-red-500 hover:bg-slate-200 dark:hover:bg-slate-700/50 rounded transition text-slate-400 shrink-0 cursor-pointer"
                  >
                    <X class="w-2.5 h-2.5" />
                  </button>
                </div>
              </template>
            </div>

            <!-- UNGROUPED TAB -->
            <div
              v-else
              draggable="true"
              @dragstart="handleTabDragStart($event, cluster.globalIndex)"
              @dragover.prevent="handleTabDragOver($event, cluster.globalIndex)"
              @drop.prevent="handleTabDrop($event, cluster.globalIndex)"
              @dragend="resetDrag"
              @click="activeSessionIndex = cluster.globalIndex"
              @contextmenu.prevent="openTabContextMenu($event, cluster.session, cluster.globalIndex)"
              :class="[
                'shrink-0 flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-mono transition cursor-pointer select-none relative group/tab border h-7.5 max-w-[210px]',
                draggedGlobalIndex === cluster.globalIndex ? 'opacity-40 scale-95' : '',
                dragOverGlobalIndex === cluster.globalIndex && dragDropPosition === 'left' ? 'border-l-2 border-l-blue-500' : '',
                dragOverGlobalIndex === cluster.globalIndex && dragDropPosition === 'right' ? 'border-r-2 border-r-blue-500' : '',
                activeSessionIndex === cluster.globalIndex
                  ? 'active-session-tab bg-white dark:bg-[#242833] border-slate-300 dark:border-slate-700 text-slate-900 dark:text-white shadow-xs font-semibold'
                  : 'border-transparent text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/50 dark:hover:bg-slate-800/40'
              ]"
              :title="cluster.session.displayName || cluster.session.host.name"
            >
              <GripVertical class="w-2.5 h-2.5 opacity-20 group-hover/tab:opacity-70 text-slate-400 cursor-grab shrink-0" />
              <span
                class="w-2 h-2 rounded-full transition-colors shrink-0"
                :class="cluster.session.connected ? (cluster.session.isIdle ? 'bg-emerald-500 animate-pulse' : 'bg-emerald-500') : (cluster.session.connecting ? 'bg-amber-500 animate-pulse' : 'bg-red-500')"
              ></span>
              <span class="truncate whitespace-nowrap text-[11px]">{{ cluster.session.displayName || cluster.session.host.name }}</span>

              <!-- Duplicate Tab Button -->
              <button
                @click.stop="duplicateSession(cluster.session)"
                title="Duplicate Terminal Tab"
                class="p-0.5 hover:text-slate-900 dark:hover:text-white hover:bg-slate-200 dark:hover:bg-slate-700/50 rounded transition text-slate-400 ml-0.5 shrink-0 cursor-pointer"
              >
                <Copy class="w-3 h-3" />
              </button>

              <!-- Close Tab Button -->
              <button
                @click.stop="closeSession(cluster.session)"
                title="Close Tab"
                class="p-0.5 hover:text-red-500 hover:bg-slate-200 dark:hover:bg-slate-700/50 rounded transition text-slate-400 shrink-0 cursor-pointer"
              >
                <X class="w-3 h-3" />
              </button>
            </div>

          </template>
        </div>

        <!-- Right Overflow Fade Gradient -->
        <div
          v-if="canScrollRight"
          class="absolute right-6 top-0 bottom-0 w-6 bg-gradient-to-l from-slate-50 dark:from-[#1b1e26] to-transparent pointer-events-none z-10"
        ></div>

        <!-- Right Scroll Button (Visible when scrolled to the left) -->
        <button
          v-if="canScrollRight"
          @click="scrollTabBar('right')"
          class="absolute right-0 z-20 h-7 w-6 flex items-center justify-center bg-white/95 dark:bg-[#1b1e26]/95 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded shadow-sm border border-slate-200 dark:border-slate-700 transition cursor-pointer backdrop-blur-xs"
          title="Scroll tabs right"
        >
          <ChevronRight class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- Right Fixed Global Actions: Auto Group & New Tab -->
      <div v-if="openSessions.length > 0" class="flex items-center gap-1.5 shrink-0 pl-2 border-l border-slate-200 dark:border-slate-800/80">
        <!-- Auto Group Helper Button (shown when 2+ sessions open) -->
        <button
          v-if="openSessions.length > 1"
          @click="autoGroupByHost"
          title="Auto-organize tabs into Chrome groups by Host"
          class="shrink-0 flex items-center gap-1 px-2.5 py-1 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 text-[11px] font-mono border border-slate-300/80 dark:border-slate-700/60 transition whitespace-nowrap h-7 cursor-pointer"
        >
          <Layers class="w-3 h-3 text-slate-500 dark:text-slate-400 shrink-0" />
          <span class="hidden lg:inline">Auto Group</span>
        </button>

        <!-- Global New Tab Button -->
        <button
          v-if="activeSession"
          @click="duplicateSession(activeSession)"
          title="Duplicate Current Server Tab"
          class="shrink-0 flex items-center gap-1 px-2.5 py-1 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 text-[11px] font-mono border border-slate-300/80 dark:border-slate-700/60 transition whitespace-nowrap h-7 cursor-pointer"
        >
          <Plus class="w-3 h-3 text-slate-500 dark:text-slate-400 shrink-0" />
          <span class="hidden sm:inline">New Tab</span>
        </button>
      </div>
    </div>

    <!-- Main Workspace Body -->
    <div class="flex-1 flex overflow-hidden min-h-0 w-full h-full">
      
      <!-- VIEW 1: SERVER LIST / DISCOVERY -->
      <div v-if="activeSessionIndex === -1" class="flex-1 px-4 sm:px-6 lg:px-8 xl:px-10 py-6 overflow-y-auto w-full max-w-[1720px] mx-auto space-y-6">
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div class="relative flex-1">
            <Search class="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2 pointer-events-none" />
            <input
              v-model="searchHostQuery"
              placeholder="Find a host or ssh user@hostname..."
              class="w-full bg-white dark:bg-[#1b1e26] border border-slate-300 dark:border-slate-800 rounded-lg pl-10 pr-4 py-2 text-xs text-slate-900 dark:text-white placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:border-blue-500 transition shadow-sm"
            />
          </div>

          <div class="flex items-center gap-2.5 shrink-0">
            <button
              v-if="canManage"
              @click="openAddHostModal()"
              class="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold rounded-lg transition shadow-sm cursor-pointer"
            >
              <Plus class="w-4 h-4" />
              <span>NEW HOST</span>
            </button>

            <button
              v-if="canManage"
              @click="isGroupModalOpen = true"
              class="flex items-center gap-2 px-4 py-2 bg-white hover:bg-slate-50 dark:bg-[#1b1e26] dark:hover:bg-[#242833] text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white text-xs font-semibold rounded-lg border border-slate-300 dark:border-slate-800 transition shadow-sm cursor-pointer"
            >
              <FolderPlus class="w-4 h-4 text-slate-500 dark:text-slate-400" />
              <span>NEW GROUP</span>
            </button>
          </div>
        </div>

        <div v-if="hosts.length === 0" class="p-12 bg-white dark:bg-[#1b1e26] border border-slate-200 dark:border-slate-800/80 rounded-xl text-center space-y-3 shadow-sm">
          <div class="w-12 h-12 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 flex items-center justify-center mx-auto">
            <Server class="w-6 h-6" />
          </div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">No Remote Servers Configured</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400 max-w-sm mx-auto">
            Add your first SSH server or VPS to manage interactive terminal sessions, telemetry metrics, and system services.
          </p>
        </div>

        <!-- Groups Section -->
        <div v-if="hosts.length > 0" class="space-y-2">
          <div class="flex items-center justify-between">
            <h3 class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">Groups</h3>
            <button
              v-if="selectedGroupFilter"
              @click="selectedGroupFilter = null"
              class="text-[11px] text-blue-600 dark:text-brand-400 hover:underline font-medium"
            >
              Clear Filter (Show All)
            </button>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3">
            <div
              v-for="(gHosts, gName) in groupedHosts"
              :key="gName"
              @click="selectedGroupFilter = selectedGroupFilter === gName ? null : gName"
              :class="[
                'p-3.5 border rounded-xl flex items-center justify-between gap-3 cursor-pointer transition shadow-xs select-none',
                selectedGroupFilter === gName
                  ? 'bg-blue-50/90 dark:bg-[#1a2337] border-blue-500 dark:border-blue-500 ring-2 ring-blue-500/25 dark:ring-blue-500/35 shadow-sm'
                  : 'bg-white dark:bg-[#111624] border-slate-200 dark:border-[#1f283d] hover:border-slate-300 dark:hover:border-slate-700 hover:bg-slate-50/60 dark:hover:bg-[#151c2d]/60'
              ]"
            >
              <div class="flex items-center gap-3 min-w-0">
                <div
                  :class="[
                    'p-2.5 rounded-lg transition shrink-0',
                    selectedGroupFilter === gName
                      ? 'bg-blue-600 text-white shadow-xs'
                      : 'bg-slate-100 dark:bg-[#151c2d] text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-[#1f283d]'
                  ]"
                >
                  <Server class="w-4 h-4" />
                </div>
                <div class="min-w-0">
                  <p
                    :class="[
                      'text-xs truncate transition',
                      selectedGroupFilter === gName
                        ? 'font-bold text-blue-700 dark:text-blue-300'
                        : 'font-bold text-slate-900 dark:text-white'
                    ]"
                  >
                    {{ gName }}
                  </p>
                  <p
                    :class="[
                      'text-[11px] transition',
                      selectedGroupFilter === gName
                        ? 'text-blue-600 dark:text-blue-400 font-semibold'
                        : 'text-slate-500 dark:text-slate-400'
                    ]"
                  >
                    {{ gHosts.length }} Host{{ gHosts.length > 1 ? 's' : '' }}
                  </p>
                </div>
              </div>

              <!-- Selected Checkmark Indicator -->
              <div
                v-if="selectedGroupFilter === gName"
                class="w-5 h-5 rounded-full bg-blue-600 text-white flex items-center justify-center shrink-0 shadow-xs"
                title="Active Filter"
              >
                <Check class="w-3 h-3 stroke-[3]" />
              </div>
            </div>
          </div>
        </div>

        <!-- Hosts Section -->
        <div v-if="hosts.length > 0" class="space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h3 class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
              Hosts ({{ filteredHosts.length }})
            </h3>

            <!-- Ownership Filter Tabs -->
            <div class="flex items-center p-0.5 rounded-lg bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-xs shadow-xs">
              <button
                @click="hostOwnershipFilter = 'all'"
                :class="hostOwnershipFilter === 'all' ? 'bg-white text-slate-800 dark:bg-slate-800 dark:text-white font-semibold shadow-xs border border-slate-200/80 dark:border-slate-700' : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'"
                class="px-2.5 py-1 rounded-md text-[11px] transition cursor-pointer"
              >
                ALL ({{ hosts.length }})
              </button>
              <button
                @click="hostOwnershipFilter = 'mine'"
                :class="hostOwnershipFilter === 'mine' ? 'bg-white text-slate-800 dark:bg-slate-800 dark:text-white font-semibold shadow-xs border border-slate-200/80 dark:border-slate-700' : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'"
                class="px-2.5 py-1 rounded-md text-[11px] transition cursor-pointer"
              >
                MY HOSTS ({{ myHostsCount }})
              </button>
              <button
                @click="hostOwnershipFilter = 'shared'"
                :class="hostOwnershipFilter === 'shared' ? 'bg-white text-slate-800 dark:bg-slate-800 dark:text-white font-semibold shadow-xs border border-slate-200/80 dark:border-slate-700' : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'"
                class="px-2.5 py-1 rounded-md text-[11px] transition cursor-pointer"
              >
                SHARED ({{ sharedHostsCount }})
              </button>
            </div>
          </div>

          <!-- Enterprise Server List Table -->
          <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl overflow-hidden shadow-xs">
            <div class="overflow-x-auto">
              <table class="w-full text-left text-xs border-collapse">
                <thead>
                  <tr class="border-b border-slate-200 dark:border-[#1f283d] bg-slate-50/80 dark:bg-[#151c2d]/80 text-slate-500 dark:text-slate-400 font-semibold text-[11px] uppercase tracking-wider">
                    <th class="py-3 px-4">Server Name</th>
                    <th class="py-3 px-4">IP Server</th>
                    <th class="py-3 px-4">Protocol</th>
                    <th class="py-3 px-4">Group</th>
                    <th class="py-3 px-4">Ownership</th>
                    <th class="py-3 px-4 text-right">Action</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-[#192132]">
                  <tr
                    v-for="host in filteredHosts"
                    :key="host.id"
                    @click="connectHost(host)"
                    class="hover:bg-slate-50/80 dark:hover:bg-[#151c2d]/50 transition cursor-pointer group"
                  >
                    <!-- Server Name -->
                    <td class="py-3 px-4 min-w-[180px]">
                      <div class="flex items-center gap-3">
                        <div class="w-8 h-8 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 flex items-center justify-center font-bold text-[11px] shrink-0 font-mono">
                          {{ host.name.substring(0, 2).toUpperCase() }}
                        </div>
                        <span class="font-bold text-slate-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-blue-400 transition truncate" :title="host.name">
                          {{ host.name }}
                        </span>
                      </div>
                    </td>

                    <!-- IP Server -->
                    <td class="py-3 px-4 whitespace-nowrap font-mono text-slate-700 dark:text-slate-300 font-medium">
                      {{ host.host }}
                    </td>

                    <!-- Protocol -->
                    <td class="py-3 px-4 whitespace-nowrap">
                      <span
                        v-if="isRdpHost(host)"
                        class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] font-semibold bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-950/40 dark:text-blue-300 dark:border-blue-800/50"
                      >
                        <Monitor class="w-3 h-3 text-blue-600 dark:text-blue-400" />
                        <span>Windows RDP</span>
                      </span>
                      <span
                        v-else
                        class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] font-semibold bg-slate-100 text-slate-700 border border-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700"
                      >
                        <SquareTerminal class="w-3 h-3 text-slate-500 dark:text-slate-400" />
                        <span>Linux SSH</span>
                        <span v-if="host.authType === 'key'" class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">· Key</span>
                      </span>
                    </td>

                    <!-- Group -->
                    <td class="py-3 px-4 whitespace-nowrap">
                      <span class="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 text-slate-600 dark:bg-slate-800/60 dark:text-slate-300 border border-slate-200 dark:border-slate-700/60">
                        {{ host.groupName || 'Default' }}
                      </span>
                    </td>

                    <!-- Ownership -->
                    <td class="py-3 px-4 whitespace-nowrap">
                      <div class="flex items-center gap-1.5">
                        <span
                          v-if="host.isOwner"
                          class="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300 border border-slate-200 dark:border-slate-700"
                        >
                          My Host
                        </span>
                        <span
                          v-else
                          class="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300 border border-slate-200 dark:border-slate-700 truncate max-w-[140px]"
                          :title="`Shared by @${host.ownerUsername || 'User'} (${host.sharedAccess === 'manage' ? 'Full Control' : 'Read Only'})`"
                        >
                          @{{ host.ownerUsername || 'User' }}
                        </span>
                        <span
                          v-if="host.sharesCount && host.sharesCount > 0 && (host.isOwner || authStore.user?.role === 'ADMIN')"
                          class="px-1.5 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 flex items-center gap-1 font-mono"
                          title="Shared users count"
                        >
                          <Users class="w-2.5 h-2.5 text-slate-400" />
                          {{ host.sharesCount }}
                        </span>
                      </div>
                    </td>

                    <!-- Action: Clear CONNECT button & tool buttons -->
                    <td class="py-3 px-4 text-right whitespace-nowrap">
                      <div class="flex items-center justify-end gap-1.5">
                        <!-- Connect / RDP Action -->
                        <button
                          v-if="isRdpHost(host)"
                          @click.stop="downloadRdpFile(host)"
                          title="Download .RDP configuration file for native Windows Remote Desktop client"
                          class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition cursor-pointer shadow-xs"
                        >
                          <Download class="w-3.5 h-3.5" />
                          <span>RDP File</span>
                        </button>
                        <button
                          v-else
                          @click.stop="connectHost(host)"
                          title="Open Interactive SSH Terminal session"
                          class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition cursor-pointer shadow-xs"
                        >
                          <SquareTerminal class="w-3.5 h-3.5" />
                          <span>Connect</span>
                        </button>

                        <!-- Share Button (Only Owner or Admin) -->
                        <button
                          v-if="canManage && (host.isOwner || authStore.user?.role === 'ADMIN')"
                          @click.stop="openShareModal(host, $event)"
                          title="Share access with other users"
                          class="p-1.5 rounded-lg bg-slate-50 hover:bg-slate-100 dark:bg-slate-800/80 dark:hover:bg-slate-700 text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 border border-slate-200 dark:border-slate-700/60 transition cursor-pointer"
                        >
                          <Share2 class="w-3.5 h-3.5" />
                        </button>

                        <!-- Edit Button (Only Owner or Admin) -->
                        <button
                          v-if="canManage && (host.isOwner || authStore.user?.role === 'ADMIN')"
                          @click.stop="openEditHostModal(host, $event)"
                          title="Edit Server Configuration"
                          class="p-1.5 rounded-lg bg-slate-50 hover:bg-slate-100 dark:bg-slate-800/80 dark:hover:bg-slate-700 text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 border border-slate-200 dark:border-slate-700/60 transition cursor-pointer"
                        >
                          <Settings class="w-3.5 h-3.5" />
                        </button>

                        <!-- Delete Button (Only Owner or Admin) -->
                        <button
                          v-if="canManage && (host.isOwner || authStore.user?.role === 'ADMIN')"
                          @click.stop="openDeleteHostModal(host, $event)"
                          title="Delete Server"
                          class="p-1.5 rounded-lg bg-slate-50 hover:bg-rose-50 dark:bg-slate-800/80 dark:hover:bg-rose-950/40 text-slate-500 hover:text-rose-600 dark:text-slate-400 dark:hover:text-rose-400 border border-slate-200 dark:border-slate-700/60 transition cursor-pointer"
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
      </div>

      <!-- VIEW 2: ACTIVE HOST WORKSPACES (PRESERVED IN DOM WITH v-show) -->
      <template v-for="(session, sIdx) in openSessions" :key="session.id">
        <div v-show="activeSessionIndex === sIdx" class="flex-1 flex overflow-hidden min-h-0 w-full h-full">
          <!-- Left Vertical Icon Nav Bar -->
          <aside class="w-12 bg-white dark:bg-[#1b1e26] border-r border-slate-200 dark:border-slate-800 flex flex-col items-center py-3 gap-2 shrink-0 shadow-sm">
            <button
              @click="switchActiveView(session, 'terminal')"
              title="Interactive Terminal"
              :class="[
                'p-2.5 rounded-lg transition cursor-pointer',
                session.activeView === 'terminal'
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
              ]"
            >
              <SquareTerminal class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'dashboard')"
              title="Utilization"
              :class="[
                'p-2.5 rounded-lg transition',
                session.activeView === 'dashboard'
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
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
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
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
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
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
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
              ]"
            >
              <Wifi class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'firewall')"
              title="Firewall Manager"
              :class="[
                'p-2.5 rounded-lg transition',
                session.activeView === 'firewall'
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
              ]"
            >
              <Shield class="w-4 h-4" />
            </button>

            <button
              @click="switchActiveView(session, 'sftp')"
              title="FileZilla Dual-Pane SFTP Transfer"
              class="p-2.5 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 transition"
            >
              <Folder class="w-4 h-4" />
            </button>
          </aside>

          <!-- Host Content Pane -->
          <div class="flex-1 flex flex-col overflow-hidden bg-slate-50 dark:bg-[#090d16] min-h-0 w-full h-full">
            
            <!-- 1. TERMINAL VIEW (Always mounted, no blank screens) -->
            <div v-show="session.activeView === 'terminal'" class="flex-1 flex flex-col relative w-full h-full min-h-0 min-w-0 overflow-hidden">
              <div class="absolute top-3 right-5 z-20 flex items-center gap-2">
                <!-- Terminal Font Zoom Controls -->
                <div
                  v-if="session.term"
                  class="flex items-center rounded border border-slate-300/80 dark:border-slate-700 bg-white/90 dark:bg-[#1b1e26]/90 text-slate-600 dark:text-slate-300 font-mono text-[11px] shadow-xs backdrop-blur-xs overflow-hidden"
                >
                  <button
                    @click="zoomOutTerminal"
                    title="Zoom Out (Ctrl + - or Ctrl + Scroll Down)"
                    class="px-1.5 py-0.5 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white transition cursor-pointer border-r border-slate-300/60 dark:border-slate-700 flex items-center justify-center"
                  >
                    <ZoomOut class="w-3 h-3" />
                  </button>
                  <button
                    @click="resetZoomTerminal"
                    title="Reset Zoom to 15px (Ctrl + 0)"
                    class="px-2 py-0.5 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white font-semibold transition cursor-pointer text-[10px]"
                  >
                    {{ terminalFontSize }}px
                  </button>
                  <button
                    @click="zoomInTerminal"
                    title="Zoom In (Ctrl + + or Ctrl + Scroll Up)"
                    class="px-1.5 py-0.5 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white transition cursor-pointer border-l border-slate-300/60 dark:border-slate-700 flex items-center justify-center"
                  >
                    <ZoomIn class="w-3 h-3" />
                  </button>
                </div>

                <!-- Clear Terminal Button -->
                <button
                  v-if="session.term"
                  @click="clearTerminal(session)"
                  title="Clear terminal screen & scrollback buffer"
                  class="flex items-center gap-1 px-2 py-0.5 rounded bg-white/90 hover:bg-slate-100 dark:bg-[#1b1e26]/90 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 text-[11px] font-mono border border-slate-300/80 dark:border-slate-700 transition cursor-pointer shadow-xs backdrop-blur-xs"
                >
                  <Trash2 class="w-3 h-3 text-slate-400 dark:text-slate-500" />
                  <span>Clear</span>
                </button>

                <!-- Reconnect Action Button (Always Available & Accessible) -->
                <div
                  class="relative inline-flex items-stretch rounded border font-mono text-[11px] shadow-xs backdrop-blur-xs transition"
                  :class="[
                    !session.connected && !session.connecting
                      ? 'border-blue-600 bg-blue-600 text-white'
                      : 'border-slate-300/80 dark:border-slate-700 bg-white/90 dark:bg-[#1b1e26]/90 text-slate-700 dark:text-slate-300'
                  ]"
                >
                  <button
                    @click="reconnectTerminal(session, true)"
                    :disabled="session.connecting"
                    title="Reconnect terminal session (Restores responsive shell & preserves scrollback history)"
                    :class="[
                      'flex items-center gap-1.5 px-2.5 py-0.5 rounded-l transition cursor-pointer disabled:opacity-50 text-[11px] font-mono leading-tight',
                      !session.connected && !session.connecting
                        ? 'hover:bg-blue-500 font-bold'
                        : 'hover:bg-slate-100 dark:hover:bg-slate-700'
                    ]"
                  >
                    <RotateCw :class="['w-3 h-3', !session.connected && !session.connecting ? 'text-white' : 'text-slate-400 dark:text-slate-500', session.connecting ? 'animate-spin' : '']" />
                    <span>{{ session.connecting ? 'Connecting...' : 'Reconnect' }}</span>
                  </button>
                  <button
                    @click.stop="toggleReconnectMenu(session.id)"
                    :title="'More reconnect options (Reset Shell)'"
                    :class="[
                      'flex items-center justify-center px-1.5 rounded-r transition cursor-pointer border-l text-[10px]',
                      !session.connected && !session.connecting
                        ? 'border-blue-500/60 hover:bg-blue-500 text-white'
                        : 'border-slate-300/60 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:hover:text-slate-200'
                    ]"
                  >
                    <ChevronDown class="w-3 h-3" />
                  </button>

                  <!-- Reconnect Options Dropdown Menu -->
                  <div
                    v-if="reconnectMenuSessionId === session.id"
                    class="absolute right-0 top-full mt-1.5 z-30 bg-white dark:bg-[#1a1d26] border border-slate-200 dark:border-slate-700 rounded-lg shadow-xl py-1 w-56 text-xs font-sans text-slate-700 dark:text-slate-200 animate-in fade-in zoom-in-95 duration-100"
                    @click.stop
                  >
                    <button
                      @click="reconnectTerminal(session, true)"
                      class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
                    >
                      <RotateCw class="w-3.5 h-3.5 text-slate-400 shrink-0" />
                      <div>
                        <div class="font-medium text-[11px]">Reset Shell & Reconnect</div>
                        <div class="text-[10px] text-slate-400">Spawns fresh shell, preserves scroll history</div>
                      </div>
                    </button>
                    <button
                      @click="reconnectTerminal(session, false)"
                      class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-600 dark:text-slate-300 transition cursor-pointer border-t border-slate-100 dark:border-slate-800"
                    >
                      <RefreshCw class="w-3.5 h-3.5 text-slate-400 shrink-0" />
                      <div>
                        <div class="font-medium text-[11px]">Re-attach Live Stream</div>
                        <div class="text-[10px] text-slate-400">Re-attaches to background running task</div>
                      </div>
                    </button>
                  </div>
                </div>

                <!-- Status Indicator Pill -->
                <div v-if="session.connected" class="flex items-center gap-1.5">
                  <span
                    v-if="session.isIdle"
                    class="px-2 py-0.5 rounded bg-emerald-50 text-emerald-700 border border-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-400 text-[10px] font-mono dark:border-emerald-500/30 flex items-center gap-1.5 shadow-sm transition-all"
                    title="Session is idle. Cloudflare Tunnel & SSH Keep-Alive packets are actively maintaining connection."
                  >
                    <span class="relative flex h-2 w-2">
                      <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                      <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                    </span>
                    <span>Idle &middot; Keep-Alive</span>
                  </span>
                  <span
                    v-else
                    class="px-2 py-0.5 rounded bg-emerald-50 text-emerald-700 border border-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-400 text-[10px] font-mono dark:border-emerald-500/30 flex items-center gap-1.5 shadow-sm transition-all"
                  >
                    <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                    <span>Connected</span>
                  </span>
                </div>
                <span
                  v-else-if="session.connecting"
                  class="px-2 py-0.5 rounded bg-amber-50 text-amber-700 border border-amber-200 dark:bg-amber-500/10 dark:text-amber-400 text-[10px] font-mono dark:border-amber-500/30 flex items-center gap-1.5 shadow-sm"
                >
                  <Loader2 class="w-3 h-3 animate-spin text-amber-500" />
                  Connecting...
                </span>
                <span
                  v-else
                  class="px-2 py-0.5 rounded bg-red-50 text-red-700 border border-red-200 dark:bg-red-500/10 dark:text-red-400 text-[10px] font-mono dark:border-red-500/30 flex items-center gap-1.5 shadow-sm"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-red-500"></span>
                  Disconnected
                </span>
              </div>
              <div
                :id="`terminal-container-${session.id}`"
                @click="session.term?.focus()"
                class="terminal-wrapper flex-1 w-full h-full min-h-0 min-w-0 overflow-hidden relative cursor-text"
              ></div>

              <!-- MobaXterm-style Idle & Keep-Alive Terminal HUD Animation -->
              <transition
                enter-active-class="transition duration-300 ease-out"
                enter-from-class="opacity-0 translate-y-2 scale-95"
                enter-to-class="opacity-100 translate-y-0 scale-100"
                leave-active-class="transition duration-200 ease-in"
                leave-from-class="opacity-100 translate-y-0 scale-100"
                leave-to-class="opacity-0 translate-y-2 scale-95"
              >
                <div
                  v-if="session.connected && session.isIdle"
                  class="absolute bottom-5 right-6 z-20 flex items-center gap-2.5 px-3 py-1.5 rounded-full bg-slate-900/85 dark:bg-[#0c101b]/90 text-slate-300 text-[11px] font-mono border border-slate-700/60 dark:border-slate-800/80 shadow-xl backdrop-blur-md select-none pointer-events-none"
                >
                  <!-- Beacon Radar Pulse -->
                  <span class="relative flex h-2 w-2">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                  </span>

                  <!-- Equalizer / Heartbeat wave bars (MobaXterm style) -->
                  <div class="flex items-end gap-0.5 h-3">
                    <span class="w-0.5 h-2 bg-emerald-400/70 rounded-full animate-pulse"></span>
                    <span class="w-0.5 h-3 bg-emerald-400 rounded-full animate-pulse [animation-delay:150ms]"></span>
                    <span class="w-0.5 h-1.5 bg-emerald-400/50 rounded-full animate-pulse [animation-delay:300ms]"></span>
                    <span class="w-0.5 h-2.5 bg-emerald-400/90 rounded-full animate-pulse [animation-delay:450ms]"></span>
                  </div>

                  <span class="text-[10px] text-slate-300 tracking-wider font-semibold uppercase">
                    Idle &middot; Keep-Alive Active
                  </span>
                </div>
              </transition>

              <!-- Floating Zoom HUD Indicator -->
              <transition
                enter-active-class="transition duration-150 ease-out"
                enter-from-class="opacity-0 translate-y-1"
                enter-to-class="opacity-100 translate-y-0"
                leave-active-class="transition duration-200 ease-in"
                leave-from-class="opacity-100 translate-y-0"
                leave-to-class="opacity-0 translate-y-1"
              >
                <div
                  v-if="zoomHudMessage"
                  class="absolute bottom-5 left-6 z-30 px-3 py-1.5 rounded-lg bg-slate-900/90 dark:bg-black/90 text-white text-xs font-mono border border-slate-700/60 shadow-xl backdrop-blur-md pointer-events-none"
                >
                  {{ zoomHudMessage }}
                </div>
              </transition>
            </div>

            <!-- 2. UTILIZATION VIEW -->
            <div v-if="session.activeView === 'dashboard'" class="flex-1 p-6 overflow-y-auto space-y-6">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
                <div>
                  <h2 class="text-sm font-bold text-slate-900 dark:text-white tracking-wide">Utilization</h2>
                  <span class="text-xs font-mono text-slate-500 dark:text-slate-400">{{ session.host.name }} ({{ session.host.host }})</span>
                </div>

                <button
                  @click="fetchHostTelemetry(session)"
                  :disabled="isTelemetryLoading"
                  class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#1b1e26] dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-200 text-xs font-semibold border border-slate-300 dark:border-slate-700 transition shadow-sm"
                >
                  <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTelemetryLoading }" />
                  <span>Refresh Stats</span>
                </button>
              </div>

              <!-- System Information Strip -->
              <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl p-4 grid grid-cols-2 sm:grid-cols-5 gap-3 shadow-lg text-xs font-mono">
                <div>
                  <p class="text-[10px] uppercase font-bold text-slate-500 tracking-wider">OS Distribution</p>
                  <p class="text-white font-semibold truncate mt-0.5" :title="session.metrics?.osName || 'Linux'">
                    {{ session.metrics?.osName || 'Linux' }}
                  </p>
                </div>
                <div>
                  <p class="text-[10px] uppercase font-bold text-slate-500 tracking-wider">Kernel Version</p>
                  <p class="text-slate-300 font-semibold truncate mt-0.5" :title="session.metrics?.kernel || '-'">
                    {{ session.metrics?.kernel || '-' }}
                  </p>
                </div>
                <div>
                  <p class="text-[10px] uppercase font-bold text-slate-500 tracking-wider">Architecture</p>
                  <p class="text-slate-300 font-semibold mt-0.5">{{ session.metrics?.arch || '-' }}</p>
                </div>
                <div>
                  <p class="text-[10px] uppercase font-bold text-slate-500 tracking-wider">Hostname</p>
                  <p class="text-slate-300 font-semibold truncate mt-0.5" :title="session.metrics?.hostname || session.host.name">
                    {{ session.metrics?.hostname || session.host.name }}
                  </p>
                </div>
                <div>
                  <p class="text-[10px] uppercase font-bold text-slate-500 tracking-wider">System Uptime</p>
                  <p class="text-emerald-400 font-semibold truncate mt-0.5" :title="session.metrics?.uptime || '-'">
                    {{ session.metrics?.uptime || '-' }}
                  </p>
                </div>
              </div>

              <!-- Utilization Cards -->
              <div class="grid grid-cols-1 sm:grid-cols-4 gap-4">
                <!-- CPU Usage -->
                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-2 shadow-lg">
                  <div class="flex justify-between items-center">
                    <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">CPU Usage</p>
                    <span class="text-[11px] font-mono text-slate-400">{{ session.metrics?.cpuCores || 1 }} cores</span>
                  </div>
                  <p class="text-2xl font-black text-white font-mono">{{ session.metrics?.cpuUsage || 0 }}%</p>
                  <div class="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
                    <div
                      class="h-full transition-all duration-300"
                      :class="(session.metrics?.cpuUsage || 0) > 85 ? 'bg-rose-500' : (session.metrics?.cpuUsage || 0) > 70 ? 'bg-amber-400' : 'bg-blue-500'"
                      :style="{ width: `${Math.min(session.metrics?.cpuUsage || 0, 100)}%` }"
                    ></div>
                  </div>
                </div>

                <!-- Memory Usage -->
                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-2 shadow-lg">
                  <div class="flex justify-between items-center">
                    <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Memory</p>
                    <span class="text-xs font-mono text-slate-300">{{ session.metrics?.memUsed || '0 B' }} / {{ session.metrics?.memTotal || '0 B' }}</span>
                  </div>
                  <p class="text-2xl font-black text-white font-mono">{{ Math.round(session.metrics?.memPercent || 0) }}%</p>
                  <div class="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
                    <div
                      class="h-full transition-all duration-300"
                      :class="(session.metrics?.memPercent || 0) > 90 ? 'bg-rose-500' : (session.metrics?.memPercent || 0) > 75 ? 'bg-amber-400' : 'bg-emerald-400'"
                      :style="{ width: `${Math.min(session.metrics?.memPercent || 0, 100)}%` }"
                    ></div>
                  </div>
                  <p class="text-[10px] font-mono text-slate-500">Avail: {{ session.metrics?.memAvailable || '-' }}</p>
                </div>

                <!-- Swap Usage -->
                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-2 shadow-lg">
                  <div class="flex justify-between items-center">
                    <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Swap Memory</p>
                    <span class="text-xs font-mono text-slate-300">{{ session.metrics?.swapUsed || '0 B' }} / {{ session.metrics?.swapTotal || '0 B' }}</span>
                  </div>
                  <p class="text-2xl font-black text-white font-mono">{{ Math.round(session.metrics?.swapPercent || 0) }}%</p>
                  <div class="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
                    <div
                      class="h-full bg-purple-500 transition-all duration-300"
                      :style="{ width: `${Math.min(session.metrics?.swapPercent || 0, 100)}%` }"
                    ></div>
                  </div>
                  <p class="text-[10px] font-mono text-slate-500">Total: {{ session.metrics?.swapTotal || '0 B' }}</p>
                </div>

                <!-- Load Average -->
                <div class="p-4 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-1 shadow-lg">
                  <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Load Average (1m, 5m, 15m)</p>
                  <p class="text-xl font-bold text-white font-mono mt-1">{{ session.metrics?.loadAverage || '0.00 / 0.00 / 0.00' }}</p>
                  <p class="text-[10px] font-mono text-slate-500 mt-2">Target &le; {{ session.metrics?.cpuCores || 1 }}.00</p>
                </div>
              </div>

              <!-- Storage & Mounted Disks Table -->
              <div class="space-y-3">
                <div class="flex items-center justify-between border-b border-slate-800 pb-2">
                  <h3 class="text-xs font-bold text-white uppercase tracking-wider">
                    Filesystem Disks & Mounts ({{ session.metrics?.disks?.length || 0 }})
                  </h3>
                </div>

                <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-x-auto shadow-xl">
                  <table class="w-full text-left text-xs font-mono">
                    <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                      <tr>
                        <th class="p-3">Mount Point</th>
                        <th class="p-3">Filesystem Device</th>
                        <th class="p-3 w-28">Total Size</th>
                        <th class="p-3 w-28">Used</th>
                        <th class="p-3 w-28">Available</th>
                        <th class="p-3 w-44">Usage %</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-800/60 text-slate-300">
                      <tr v-for="d in session.metrics?.disks || []" :key="d.mount" class="hover:bg-slate-800/30">
                        <td class="p-3 text-white font-bold">{{ d.mount }}</td>
                        <td class="p-3 text-slate-400">{{ d.filesystem || '-' }}</td>
                        <td class="p-3 font-semibold">{{ d.total }}</td>
                        <td class="p-3 text-slate-300">{{ d.used }}</td>
                        <td class="p-3 text-emerald-400 font-semibold">{{ d.avail }}</td>
                        <td class="p-3">
                          <div class="flex items-center gap-2">
                            <span class="w-10 text-right font-bold" :class="d.percent > 85 ? 'text-rose-400' : d.percent > 70 ? 'text-amber-400' : 'text-slate-300'">
                              {{ d.percent }}%
                            </span>
                            <div class="flex-1 h-1.5 bg-slate-800 rounded-full overflow-hidden">
                              <div
                                class="h-full rounded-full transition-all duration-300"
                                :class="d.percent > 85 ? 'bg-rose-500' : d.percent > 70 ? 'bg-amber-400' : 'bg-emerald-400'"
                                :style="{ width: `${Math.min(d.percent, 100)}%` }"
                              ></div>
                            </div>
                          </div>
                        </td>
                      </tr>
                      <tr v-if="!session.metrics?.disks || session.metrics.disks.length === 0">
                        <td colspan="6" class="p-8 text-center text-slate-500 text-xs">
                          {{ isTelemetryLoading ? 'Polling filesystem mounts...' : 'No mounted filesystems detected.' }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
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
                <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 pb-3">
                  <div class="flex flex-wrap items-center gap-3">
                    <h3 class="text-xs font-bold text-white uppercase tracking-wider">
                      Listening Ports ({{ filteredListeningPorts.length }})
                    </h3>
                    <!-- Protocol Filter Tabs -->
                    <div class="flex items-center p-0.5 rounded-lg bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-xs shadow-sm">
                      <button
                        @click="portProtoFilter = 'all'"
                        :class="portProtoFilter === 'all' ? 'bg-white text-blue-700 dark:bg-slate-700 dark:text-white font-bold shadow-sm border border-slate-200 dark:border-transparent' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'"
                        class="px-2.5 py-1 rounded-md text-[11px] transition cursor-pointer"
                      >
                        ALL ({{ activeSession?.networkInfo?.listeningPorts?.length || 0 }})
                      </button>
                      <button
                        @click="portProtoFilter = 'tcp'"
                        :class="portProtoFilter === 'tcp' ? 'bg-sky-50 text-sky-700 border border-sky-300 dark:bg-sky-600/30 dark:text-sky-300 dark:border-sky-500/40 font-bold shadow-sm' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'"
                        class="px-2.5 py-1 rounded-md text-[11px] transition flex items-center gap-1.5 cursor-pointer"
                      >
                        <span class="w-1.5 h-1.5 rounded-full bg-sky-500"></span>
                        TCP ({{ tcpPortsCount }})
                      </button>
                      <button
                        @click="portProtoFilter = 'udp'"
                        :class="portProtoFilter === 'udp' ? 'bg-amber-50 text-amber-700 border border-amber-300 dark:bg-amber-600/30 dark:text-amber-300 dark:border-amber-500/40 font-bold shadow-sm' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'"
                        class="px-2.5 py-1 rounded-md text-[11px] transition flex items-center gap-1.5 cursor-pointer"
                      >
                        <span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
                        UDP ({{ udpPortsCount }})
                      </button>
                    </div>
                  </div>

                  <div class="flex items-center gap-2">
                    <div class="relative w-48 sm:w-60">
                      <Search class="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-500" />
                      <input
                        v-model="portSearch"
                        type="text"
                        placeholder="Search port, process, PID..."
                        class="w-full bg-white dark:bg-[#161922] border border-slate-300 dark:border-slate-700/80 rounded-lg pl-8 pr-3 py-1 text-xs text-slate-900 dark:text-white placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:border-sky-500 transition font-sans shadow-sm"
                      />
                    </div>
                    <button
                      @click="fetchHostTelemetry(session)"
                      :disabled="isTelemetryLoading"
                      class="p-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 transition shadow-sm"
                      title="Refresh Listening Ports"
                    >
                      <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTelemetryLoading }" />
                    </button>
                  </div>
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
                      <tr v-for="p in filteredListeningPorts" :key="`${p.proto}-${p.port}-${p.pid}-${p.localAddr}`" class="hover:bg-slate-800/30 transition-colors">
                        <td class="p-3">
                          <span
                            v-if="p.proto && p.proto.toUpperCase().includes('TCP')"
                            class="px-2 py-0.5 rounded bg-sky-500/10 text-sky-400 border border-sky-500/20 font-bold text-[10px] uppercase"
                          >
                            {{ p.proto }}
                          </span>
                          <span
                            v-else-if="p.proto && p.proto.toUpperCase().includes('UDP')"
                            class="px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 font-bold text-[10px] uppercase"
                          >
                            {{ p.proto }}
                          </span>
                          <span v-else class="px-2 py-0.5 rounded bg-slate-800 text-slate-400 font-bold text-[10px] uppercase">
                            {{ p.proto }}
                          </span>
                        </td>
                        <td class="p-3 text-white font-bold">
                          {{ p.localAddr || '*' }}:{{ p.port }}
                          <span v-if="p.state === 'UNCONN'" class="ml-2 text-[10px] text-slate-400 font-normal font-sans">(UDP Listen)</span>
                        </td>
                        <td class="p-3">
                          <span v-if="p.process && p.process !== '-'" class="px-2 py-0.5 rounded bg-slate-800 text-sky-300 border border-slate-700 font-mono text-[11px] font-semibold">
                            {{ p.process }}
                          </span>
                          <span v-else class="text-slate-500 font-mono">-</span>
                        </td>
                        <td class="p-3 text-slate-400 font-mono">
                          <span v-if="p.pid && p.pid !== '-'" class="px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 text-[11px]">
                            {{ p.pid }}
                          </span>
                          <span v-else class="text-slate-600">-</span>
                        </td>
                      </tr>
                      <tr v-if="filteredListeningPorts.length === 0">
                        <td colspan="4" class="p-12 text-center text-slate-500 text-xs">
                          {{ isTelemetryLoading ? 'Polling open network ports...' : 'No listening ports detected matching current filters.' }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>

            <!-- 6. FIREWALL VIEW -->
            <div v-if="session.activeView === 'firewall'" class="flex-1 p-6 overflow-y-auto space-y-6">
              <!-- Header & Service Status Indicator -->
              <div class="flex flex-wrap items-center justify-between border-b border-slate-800 pb-4 gap-4">
                <div>
                  <div class="flex items-center gap-3">
                    <h2 class="text-sm font-bold text-white tracking-wide">Firewall & Security Rules</h2>
                    <!-- Status Service Indicator (Active / Inactive) -->
                    <span
                      v-if="session.firewall?.active"
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 text-[11px] font-semibold border border-emerald-500/20"
                    >
                      <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
                      ACTIVE ({{ session.firewall?.service?.toUpperCase() || 'RUNNING' }})
                    </span>
                    <span
                      v-else
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-rose-500/10 text-rose-400 text-[11px] font-semibold border border-rose-500/20"
                    >
                      <span class="w-2 h-2 rounded-full bg-rose-500"></span>
                      INACTIVE (STOPPED / DISABLED)
                    </span>
                  </div>
                  <p class="text-xs font-mono text-slate-400 mt-1">
                    {{ session.host.name }} ({{ session.host.host }}) &bull; {{ session.firewall?.rawStatus || 'Querying firewall daemon status...' }}
                  </p>
                </div>

                <div class="flex items-center gap-2">
                  <!-- Start / Stop Firewall Service -->
                  <button
                    v-if="session.firewall?.active"
                    @click="handleToggleFirewall(false)"
                    :disabled="isFirewallToggling"
                    class="px-3 py-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/30 text-xs font-semibold transition shadow flex items-center gap-1.5"
                    title="Stop / Disable Firewall Service on Remote Host"
                  >
                    <Loader2 v-if="isFirewallToggling" class="w-3.5 h-3.5 animate-spin" />
                    <StopCircle v-else class="w-3.5 h-3.5" />
                    <span>Disable Firewall</span>
                  </button>
                  <button
                    v-else
                    @click="handleToggleFirewall(true)"
                    :disabled="isFirewallToggling"
                    class="px-3 py-1.5 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 text-xs font-semibold transition shadow flex items-center gap-1.5"
                    title="Start / Enable Firewall Service on Remote Host"
                  >
                    <Loader2 v-if="isFirewallToggling" class="w-3.5 h-3.5 animate-spin" />
                    <PlayCircle v-else class="w-3.5 h-3.5" />
                    <span>Enable Firewall</span>
                  </button>

                  <!-- Refresh Stats -->
                  <button
                    @click="fetchHostTelemetry(session)"
                    :disabled="isTelemetryLoading"
                    class="p-2 rounded-lg bg-[#1b1e26] hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
                    title="Refresh Firewall Rules"
                  >
                    <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTelemetryLoading }" />
                  </button>

                  <!-- Add Rule Button -->
                  <button
                    @click="openAddRuleModal"
                    class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow transition"
                  >
                    <Plus class="w-3.5 h-3.5" />
                    <span>Add Rule</span>
                  </button>
                </div>
              </div>

              <!-- Filter Search -->
              <div class="flex items-center justify-between gap-3">
                <div class="relative flex-1 max-w-sm">
                  <Search class="w-3.5 h-3.5 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
                  <input
                    v-model="firewallSearch"
                    placeholder="Search by protocol, port, IP, description..."
                    class="w-full bg-[#1b1e26] border border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-white placeholder-slate-500"
                  />
                </div>
                <span class="text-xs text-slate-400 font-mono">{{ filteredFirewallRules.length }} rules defined</span>
              </div>

              <!-- Rules Table (PROTOCOL, PORT RANGE, SOURCE IP ADDRESS, ACTION, DESCRIPTION, ACTIONS) -->
              <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-x-auto shadow-xl">
                <table class="w-full text-left text-xs font-mono">
                  <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                    <tr>
                      <th class="p-3 w-28">Protocol</th>
                      <th class="p-3 w-36">Port Range</th>
                      <th class="p-3">Source IP Address</th>
                      <th class="p-3 w-28">Action</th>
                      <th class="p-3">Description</th>
                      <th class="p-3 w-24 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-800/60 text-slate-300">
                    <tr v-for="r in filteredFirewallRules" :key="r.id" class="hover:bg-slate-800/30">
                      <td class="p-3">
                        <span
                          class="px-2 py-0.5 rounded text-[10px] font-bold tracking-wider"
                          :class="{
                            'bg-blue-500/10 text-blue-400 border border-blue-500/30': r.protocol === 'TCP',
                            'bg-purple-500/10 text-purple-400 border border-purple-500/30': r.protocol === 'UDP',
                            'bg-amber-500/10 text-amber-400 border border-amber-500/30': r.protocol === 'ICMP',
                            'bg-slate-700/50 text-slate-300 border border-slate-600': r.protocol === 'ALL'
                          }"
                        >
                          {{ r.protocol }}
                        </span>
                      </td>
                      <td class="p-3 text-white font-bold">{{ r.portRange }}</td>
                      <td class="p-3 text-slate-300">{{ r.sourceIp }}</td>
                      <td class="p-3">
                        <span
                          class="px-2 py-0.5 rounded text-[10px] font-bold uppercase"
                          :class="r.action === 'ALLOW' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'"
                        >
                          {{ r.action }}
                        </span>
                      </td>
                      <td class="p-3 text-slate-400 truncate max-w-xs">{{ r.description || '-' }}</td>
                      <td class="p-3 text-right">
                        <button
                          @click="handleDeleteFirewallRule(r.id)"
                          class="p-1.5 rounded text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                          title="Delete Firewall Rule"
                        >
                          <Trash2 class="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                    <tr v-if="filteredFirewallRules.length === 0">
                      <td colspan="6" class="p-12 text-center text-slate-500 text-xs">
                        {{ isTelemetryLoading ? 'Querying firewall configuration...' : 'No firewall rules configured.' }}
                      </td>
                    </tr>
                  </tbody>
                </table>
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
      :class="[
        isSftpFullScreen
          ? 'fixed inset-0 z-50 flex flex-col bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm w-screen h-screen overflow-hidden font-sans'
          : 'fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm p-2 sm:p-4 font-sans'
      ]"
      class="animate-in fade-in duration-150"
    >
      <div
        :class="[
          isSftpFullScreen
            ? 'w-full h-full flex flex-col overflow-hidden bg-white dark:bg-[#0c1017]'
            : 'bg-white dark:bg-[#0c1017] border border-slate-200 dark:border-[#1c2333] rounded-2xl w-full max-w-7xl h-[94vh] flex flex-col shadow-2xl overflow-hidden'
        ]"
      >
        <!-- 1. TOP HEADER & REMOTE HOST SELECTOR BAR -->
        <div class="px-4 py-2.5 bg-slate-50 dark:bg-[#121622] border-b border-slate-200 dark:border-[#1c2333] flex flex-wrap items-center justify-between gap-3 shrink-0 text-xs">
          <div class="flex items-center gap-3">
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-sm font-bold text-slate-900 dark:text-white tracking-tight">SFTP Manager</h2>
                <span
                  :class="[
                    'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold font-mono',
                    destHostObj
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20'
                      : 'bg-slate-200/70 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border border-slate-300 dark:border-slate-700'
                  ]"
                >
                  <span
                    :class="[
                      'w-1.5 h-1.5 rounded-full',
                      destHostObj ? 'bg-emerald-500' : 'bg-slate-400'
                    ]"
                  ></span>
                  {{ destHostObj ? 'Connected' : 'Idle' }}
                </span>
              </div>
            </div>

            <!-- Mode Switcher -->
            <div class="hidden sm:flex items-center bg-slate-200/70 dark:bg-[#1a202c] rounded-lg p-0.5 border border-slate-200 dark:border-slate-700/80">
              <button
                type="button"
                @click="() => { sftpMode = 'local-remote'; sftpSourceHostId = 'local'; }"
                :class="[
                  'px-2.5 py-1 rounded-md text-[11px] font-semibold transition cursor-pointer',
                  sftpMode === 'local-remote'
                    ? 'bg-white dark:bg-[#252d3d] text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                Local ⇄ Remote
              </button>
              <button
                type="button"
                @click="() => { sftpMode = 'server-server'; if (sftpSourceHostId === 'local' && hosts.length > 0) sftpSourceHostId = hosts[0].id; }"
                :class="[
                  'px-2.5 py-1 rounded-md text-[11px] font-semibold transition cursor-pointer',
                  sftpMode === 'server-server'
                    ? 'bg-white dark:bg-[#252d3d] text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                Server ⇄ Server (FXP)
              </button>
            </div>
          </div>

          <!-- Top Controls & Host Selection -->
          <div class="flex items-center gap-2">
            <!-- Mode 1: Local-Remote Host Selector -->
            <template v-if="sftpMode === 'local-remote'">
              <div class="flex items-center gap-1.5">
                <span class="text-xs font-semibold text-slate-600 dark:text-slate-400 whitespace-nowrap">Remote Server:</span>
                <select
                  v-model="sftpDestHostId"
                  @change="handleDestHostChange"
                  class="bg-white dark:bg-[#0b0e14] border border-slate-200 dark:border-slate-700 rounded-lg px-2.5 py-1 text-xs text-slate-800 dark:text-slate-200 font-medium focus:outline-none focus:ring-1 focus:ring-blue-500 min-w-[210px] sm:min-w-[260px] cursor-pointer"
                >
                  <option value="">Select a host to connect...</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">
                    {{ h.name }} ({{ h.username }}@{{ h.host }})
                  </option>
                </select>
              </div>
              <button
                v-if="sftpDestHostId"
                @click="disconnectDestHost"
                class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-[#1a202c] dark:hover:bg-[#252d3d] text-slate-700 dark:text-slate-300 text-xs font-medium transition cursor-pointer"
                title="Disconnect from remote server"
              >
                <Unlink class="w-3.5 h-3.5 text-slate-400" />
                <span>Disconnect</span>
              </button>
            </template>

            <!-- Mode 2: Server-Server FXP Selectors -->
            <template v-else>
              <div class="flex items-center gap-1.5">
                <span class="text-xs font-medium text-slate-500">From:</span>
                <select
                  v-model="sftpSourceHostId"
                  @change="fetchSourceSftpFiles('/')"
                  class="bg-white dark:bg-[#0b0e14] border border-slate-200 dark:border-slate-700 rounded-lg px-2 py-1 text-xs text-slate-800 dark:text-slate-200 font-medium focus:outline-none min-w-[130px]"
                >
                  <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
                </select>
                <button
                  @click="swapSourceAndDest"
                  class="p-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 transition cursor-pointer"
                  title="Swap Source and Destination"
                >
                  <RotateCw class="w-3.5 h-3.5 text-slate-400" />
                </button>
                <span class="text-xs font-medium text-slate-500">To:</span>
                <select
                  v-model="sftpDestHostId"
                  @change="handleDestHostChange"
                  class="bg-white dark:bg-[#0b0e14] border border-slate-200 dark:border-slate-700 rounded-lg px-2 py-1 text-xs text-slate-800 dark:text-slate-200 font-medium focus:outline-none min-w-[130px]"
                >
                  <option value="">Select destination...</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
                </select>
              </div>
            </template>

            <!-- Fullscreen & Close -->
            <div class="flex items-center gap-1 pl-2 border-l border-slate-200 dark:border-slate-800">
              <button
                @click="isSftpFullScreen = !isSftpFullScreen"
                class="p-1.5 text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white rounded-lg hover:bg-slate-200/70 dark:hover:bg-slate-800 transition cursor-pointer"
                :title="isSftpFullScreen ? 'Restore Window' : 'Full Screen'"
              >
                <Minimize2 v-if="isSftpFullScreen" class="w-4 h-4" />
                <Maximize2 v-else class="w-4 h-4" />
              </button>
              <button
                @click="isSftpModalOpen = false"
                class="p-1.5 text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white rounded-lg hover:bg-slate-200/70 dark:hover:bg-slate-800 transition cursor-pointer"
                title="Close SFTP Manager"
              >
                <X class="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        <!-- 2. DUAL-PANE BODY WITH CENTRAL TRANSFER ARROWS -->
        <div class="flex-1 flex flex-col md:flex-row overflow-hidden min-h-0 divide-y md:divide-y-0 bg-white dark:bg-[#0c1017]">
          
          <!-- ============================================================= -->
          <!-- PANE 1: LOCAL MACHINE (OR SOURCE SERVER)                     -->
          <!-- ============================================================= -->
          <div
            class="flex-1 flex flex-col overflow-hidden min-h-0 bg-white dark:bg-[#0c1017] relative"
            @dragover.prevent="isDragOverLocal = true"
            @dragleave.prevent="isDragOverLocal = false"
            @drop.prevent="handleLocalDrop"
          >
            <!-- Pane 1 Header -->
            <div class="p-2.5 px-3.5 bg-slate-50/80 dark:bg-[#10141f] border-b border-slate-200 dark:border-slate-800 flex items-center justify-between gap-2 shrink-0">
              <div>
                <h3 class="text-xs font-bold text-slate-900 dark:text-white tracking-wide">
                  {{ sftpMode === 'local-remote' ? 'Local Machine' : (sourceHostObj ? sourceHostObj.name : 'Source Server') }}
                </h3>
                <p class="text-[11px] text-slate-500 dark:text-slate-400 font-mono truncate max-w-xs mt-0.5">
                  {{ sftpMode === 'local-remote' ? 'Browser Staging' : sftpSourceCurrentPath }}
                </p>
              </div>

              <div class="flex items-center gap-1.5">
                <!-- Search Filter -->
                <div class="relative">
                  <Search class="w-3 h-3 absolute left-2 top-2 text-slate-400" />
                  <input
                    v-model="sftpSourceFilter"
                    placeholder="Filter..."
                    class="pl-6 pr-2 py-1 text-xs bg-white dark:bg-[#0b0e14] border border-slate-200 dark:border-slate-700/80 rounded-lg text-slate-800 dark:text-slate-200 focus:outline-none w-28 sm:w-36 font-sans"
                  />
                </div>

                <!-- Local File Controls -->
                <template v-if="sftpMode === 'local-remote'">
                  <input type="file" ref="sftpFileInput" multiple @change="handleLocalFileSelection" class="hidden" />
                  <button
                    @click="triggerLocalFileBrowse"
                    class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-medium shadow-xs transition cursor-pointer"
                    title="Browse files from your computer"
                  >
                    <Plus class="w-3.5 h-3.5" />
                    <span>Browse</span>
                  </button>
                  <button
                    v-if="localStagedFiles.length > 0"
                    @click="clearLocalStagedFiles"
                    class="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-500 hover:text-rose-500 dark:text-slate-400 dark:hover:text-rose-400 transition cursor-pointer"
                    title="Clear staged files"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </template>

                <!-- Server Source Controls -->
                <template v-else>
                  <button
                    @click="navigateSourceParent"
                    :disabled="sftpSourceCurrentPath === '/' || !sftpSourceCurrentPath"
                    class="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 disabled:opacity-30 transition cursor-pointer"
                    title="Up one folder"
                  >
                    <CornerLeftUp class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="fetchSourceSftpFiles()"
                    class="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 transition cursor-pointer"
                    title="Refresh"
                  >
                    <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': sftpSourceLoading }" />
                  </button>
                </template>
              </div>
            </div>

            <!-- Drag Overlay on Drag Over -->
            <div
              v-if="isDragOverLocal && sftpMode === 'local-remote'"
              class="absolute inset-0 z-20 bg-blue-500/10 dark:bg-blue-500/20 border-2 border-dashed border-blue-500 flex items-center justify-center p-4 backdrop-blur-xs"
            >
              <div class="text-center space-y-1">
                <Upload class="w-8 h-8 text-blue-600 dark:text-blue-400 mx-auto animate-bounce" />
                <p class="text-xs font-bold text-blue-600 dark:text-blue-300">Drop files here to stage for upload</p>
              </div>
            </div>

            <!-- Pane 1 Table Body -->
            <div class="flex-1 overflow-y-auto select-text font-sans">
              <!-- Local Staging Table -->
              <table v-if="sftpMode === 'local-remote'" class="w-full text-left text-xs border-collapse">
                <thead class="bg-slate-50 dark:bg-[#0f131d] text-slate-500 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800 sticky top-0 z-10 select-none">
                  <tr>
                    <th @click="toggleLocalSort('name')" class="py-2 px-3.5 cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center gap-1">
                        <span>Name</span>
                        <ChevronUp v-if="localSortKey === 'name' && localSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="localSortKey === 'name' && !localSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th @click="toggleLocalSort('size')" class="py-2 px-3 w-20 text-right cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center justify-end gap-1">
                        <span>Size</span>
                        <ChevronUp v-if="localSortKey === 'size' && localSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="localSortKey === 'size' && !localSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th @click="toggleLocalSort('modified')" class="py-2 px-3 w-36 cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center gap-1">
                        <span>Modified</span>
                        <ChevronUp v-if="localSortKey === 'modified' && localSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="localSortKey === 'modified' && !localSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th class="py-2 px-3 w-20">Kind</th>
                    <th class="py-2 px-3 w-16 text-right">Action</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-[11px] text-slate-700 dark:text-slate-300">
                  <tr
                    v-for="staged in sortedLocalFiles"
                    :key="staged.id"
                    @click="selectedLocalId = staged.id"
                    @dblclick="uploadStagedFile(staged)"
                    :class="[
                      'cursor-pointer transition',
                      selectedLocalId === staged.id
                        ? 'bg-blue-50/80 dark:bg-blue-950/40 text-blue-900 dark:text-blue-200 font-medium'
                        : 'hover:bg-slate-50 dark:hover:bg-slate-800/40'
                    ]"
                  >
                    <td class="py-2 px-3.5 flex items-center gap-2 truncate max-w-[200px]">
                      <File class="w-4 h-4 text-slate-400 shrink-0" />
                      <span class="truncate">{{ staged.name }}</span>
                    </td>
                    <td class="py-2 px-3 text-right font-mono text-slate-500 dark:text-slate-400">
                      {{ formatFileSize(staged.size) }}
                    </td>
                    <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[10px] truncate">
                      {{ formatSftpDate(staged.lastModified) }}
                    </td>
                    <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[10px] truncate">
                      {{ staged.type || 'File' }}
                    </td>
                    <td class="py-2 px-3 text-right whitespace-nowrap">
                      <button
                        @click.stop="removeLocalStagedFile(staged.id)"
                        class="p-1 rounded text-slate-400 hover:text-rose-500 dark:hover:text-rose-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
                        title="Remove from staging"
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                  <!-- Empty Staging State -->
                  <tr v-if="sortedLocalFiles.length === 0">
                    <td colspan="5" class="py-16 text-center text-slate-400 dark:text-slate-500 text-xs">
                      <div class="w-12 h-12 rounded-2xl bg-slate-100 dark:bg-slate-800/60 flex items-center justify-center mx-auto mb-2 text-slate-400">
                        <Upload class="w-6 h-6" />
                      </div>
                      <p class="font-medium text-slate-700 dark:text-slate-300">Local Staging Area</p>
                      <p class="text-[11px] text-slate-400 dark:text-slate-500 mt-0.5">Click "Browse" or drag & drop files here to upload.</p>
                    </td>
                  </tr>
                </tbody>
              </table>

              <!-- Server Source Table (FXP Mode) -->
              <table v-else class="w-full text-left text-xs border-collapse">
                <thead class="bg-slate-50 dark:bg-[#0f131d] text-slate-500 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800 sticky top-0 z-10 select-none">
                  <tr>
                    <th @click="toggleSourceSort('name')" class="py-2 px-3.5 cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center gap-1">
                        <span>Name</span>
                        <ChevronUp v-if="sourceSortKey === 'name' && sourceSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="sourceSortKey === 'name' && !sourceSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th @click="toggleSourceSort('size')" class="py-2 px-3 w-20 text-right cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center justify-end gap-1">
                        <span>Size</span>
                        <ChevronUp v-if="sourceSortKey === 'size' && sourceSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="sourceSortKey === 'size' && !sourceSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th @click="toggleSourceSort('modTime')" class="py-2 px-3 w-36 cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center gap-1">
                        <span>Modified</span>
                        <ChevronUp v-if="sourceSortKey === 'modTime' && sourceSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="sourceSortKey === 'modTime' && !sourceSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th class="py-2 px-3 w-20">Kind</th>
                    <th class="py-2 px-3 w-16 text-right">Action</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-[11px] text-slate-700 dark:text-slate-300">
                  <!-- Parent Directory row -->
                  <tr
                    v-if="sftpSourceCurrentPath !== '/'"
                    @dblclick="navigateSourceParent"
                    class="hover:bg-slate-50 dark:hover:bg-slate-800/40 cursor-pointer"
                  >
                    <td colspan="5" class="py-2 px-3.5 flex items-center gap-2 font-semibold text-slate-600 dark:text-slate-400">
                      <Folder class="w-4 h-4 text-amber-500 shrink-0" />
                      <span>..</span>
                    </td>
                  </tr>
                  <tr
                    v-for="file in sortedSourceFiles"
                    :key="file.name"
                    @click="selectedSourceFile = file"
                    @dblclick="file.isDir ? fetchSourceSftpFiles(`${sftpSourceCurrentPath === '/' ? '' : sftpSourceCurrentPath}/${file.name}`) : transferSourceFileToDest(file.name)"
                    :class="[
                      'cursor-pointer transition',
                      selectedSourceFile?.name === file.name
                        ? 'bg-blue-50/80 dark:bg-blue-950/40 text-blue-900 dark:text-blue-200 font-medium'
                        : 'hover:bg-slate-50 dark:hover:bg-slate-800/40'
                    ]"
                  >
                    <td class="py-2 px-3.5 flex items-center gap-2 truncate max-w-[200px]">
                      <Folder v-if="file.isDir" class="w-4 h-4 text-amber-500 shrink-0" />
                      <File v-else class="w-4 h-4 text-slate-400 shrink-0" />
                      <span class="truncate" :class="file.isDir ? 'font-semibold text-slate-900 dark:text-white' : ''">{{ file.name }}</span>
                    </td>
                    <td class="py-2 px-3 text-right font-mono text-slate-500 dark:text-slate-400">
                      {{ file.isDir ? '-' : formatFileSize(file.size) }}
                    </td>
                    <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[10px] truncate">
                      {{ formatSftpDate(file.modTime) }}
                    </td>
                    <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[10px] truncate">
                      {{ getFileTypeLabel(file.name, file.isDir) }}
                    </td>
                    <td class="py-2 px-3 text-right whitespace-nowrap">
                      <button
                        v-if="!file.isDir"
                        @click.stop="transferSourceFileToDest(file.name)"
                        class="p-1 rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
                        title="Transfer to destination"
                      >
                        <ArrowRight class="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                  <tr v-if="sortedSourceFiles.length === 0 && !sftpSourceLoading">
                    <td colspan="5" class="py-16 text-center text-slate-400 dark:text-slate-500 text-xs">Directory is empty</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- ============================================================= -->
          <!-- CENTER TRANSFER CONTROLS (ARROWS)                            -->
          <!-- ============================================================= -->
          <div class="w-full md:w-12 h-10 md:h-auto bg-slate-50 dark:bg-[#0f1420] border-y md:border-y-0 md:border-x border-slate-200 dark:border-slate-800 flex flex-row md:flex-col items-center justify-center gap-2 p-1.5 shrink-0 select-none">
            <!-- Left to Right (Upload / Transfer to Remote) -->
            <button
              type="button"
              @click="handleTransferLeftToRight"
              :disabled="!canTransferLeftToRight"
              :class="[
                'w-8 h-8 rounded-lg flex items-center justify-center transition',
                canTransferLeftToRight
                  ? 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-xs cursor-pointer'
                  : 'bg-slate-200 dark:bg-slate-800/80 text-slate-400 dark:text-slate-600 cursor-not-allowed opacity-50'
              ]"
              title="Transfer to Remote Server (Upload)"
            >
              <ArrowRight class="w-4 h-4" />
            </button>

            <!-- Right to Left (Download / Transfer from Remote) -->
            <button
              type="button"
              @click="handleTransferRightToLeft"
              :disabled="!canTransferRightToLeft"
              :class="[
                'w-8 h-8 rounded-lg flex items-center justify-center transition',
                canTransferRightToLeft
                  ? 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-xs cursor-pointer'
                  : 'bg-slate-200 dark:bg-slate-800/80 text-slate-400 dark:text-slate-600 cursor-not-allowed opacity-50'
              ]"
              title="Download from Remote Server"
            >
              <ArrowLeft class="w-4 h-4" />
            </button>
          </div>

          <!-- ============================================================= -->
          <!-- PANE 2: REMOTE SERVER (DESTINATION)                          -->
          <!-- ============================================================= -->
          <div class="flex-1 flex flex-col overflow-hidden min-h-0 bg-white dark:bg-[#0c1017]">
            <!-- Pane 2 Header -->
            <div class="p-2.5 px-3.5 bg-slate-50/80 dark:bg-[#10141f] border-b border-slate-200 dark:border-slate-800 flex items-center justify-between gap-2 shrink-0">
              <div>
                <h3 class="text-xs font-bold text-slate-900 dark:text-white tracking-wide">
                  {{ destHostObj ? destHostObj.name : 'Remote Server' }}
                </h3>
                <p class="text-[11px] text-slate-500 dark:text-slate-400 font-mono truncate max-w-xs mt-0.5">
                  {{ destHostObj ? `${destHostObj.username}@${destHostObj.host}:${destHostObj.port}` : 'No connection' }}
                </p>
              </div>

              <!-- Remote Action Controls (When Connected) -->
              <div v-if="destHostObj" class="flex items-center gap-1.5">
                <!-- Search Filter -->
                <div class="relative">
                  <Search class="w-3 h-3 absolute left-2 top-2 text-slate-400" />
                  <input
                    v-model="sftpDestFilter"
                    placeholder="Filter..."
                    class="pl-6 pr-2 py-1 text-xs bg-white dark:bg-[#0b0e14] border border-slate-200 dark:border-slate-700/80 rounded-lg text-slate-800 dark:text-slate-200 focus:outline-none w-28 sm:w-36 font-sans"
                  />
                </div>

                <!-- Up one folder -->
                <button
                  @click="navigateDestParent"
                  :disabled="sftpDestCurrentPath === '/' || !sftpDestCurrentPath"
                  class="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 disabled:opacity-30 transition cursor-pointer"
                  title="Up one folder"
                >
                  <CornerLeftUp class="w-3.5 h-3.5" />
                </button>

                <!-- Create Folder -->
                <button
                  @click="openCreateFolderModal"
                  class="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 transition cursor-pointer"
                  title="Create New Folder"
                >
                  <FolderPlus class="w-3.5 h-3.5" />
                </button>

                <!-- Refresh -->
                <button
                  @click="fetchDestSftpFiles()"
                  class="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 transition cursor-pointer"
                  title="Refresh Remote Directory"
                >
                  <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': sftpDestLoading }" />
                </button>
              </div>
            </div>

            <!-- Path Bar (When Connected) -->
            <div v-if="destHostObj" class="p-1.5 px-3.5 bg-slate-100/60 dark:bg-[#121622] border-b border-slate-200 dark:border-slate-800/80 flex flex-wrap items-center justify-between gap-2 text-xs">
              <form @submit.prevent="fetchDestSftpFiles(sftpDestInputPath)" class="flex items-center gap-1.5 flex-1 min-w-[200px]">
                <span class="text-slate-400 font-mono text-[11px] select-none">Path:</span>
                <input
                  v-model="sftpDestInputPath"
                  class="w-full bg-white dark:bg-[#0b0e14] border border-slate-200 dark:border-slate-700/80 rounded px-2 py-0.5 text-xs text-slate-900 dark:text-white font-mono focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
                <button
                  type="submit"
                  class="px-2 py-0.5 rounded bg-slate-200 hover:bg-slate-300 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-white text-xs font-medium transition cursor-pointer"
                >
                  Go
                </button>
              </form>

              <!-- Quick Path Pills -->
              <div class="hidden sm:flex items-center gap-1 text-[11px]">
                <span class="text-slate-400 text-[10px] select-none">Quick:</span>
                <button
                  v-for="q in ['/', '/root', '/home', '/etc', '/var/log', '/tmp']"
                  :key="q"
                  @click="fetchDestSftpFiles(q)"
                  class="px-1.5 py-0.5 rounded bg-white hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-700 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 text-[10px] font-mono transition cursor-pointer"
                >
                  {{ q }}
                </button>
              </div>
            </div>

            <!-- Pane 2 Body -->
            <div class="flex-1 overflow-y-auto select-text font-sans">
              <!-- When NOT connected -->
              <div v-if="!destHostObj" class="h-full flex flex-col items-center justify-center p-8 text-center bg-slate-50/40 dark:bg-[#090d14]/40">
                <div class="w-14 h-14 rounded-2xl bg-slate-100 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700/60 flex items-center justify-center text-slate-400 mb-3">
                  <Server class="w-7 h-7" />
                </div>
                <h4 class="text-sm font-bold text-slate-800 dark:text-slate-200">Not connected</h4>
                <p class="text-xs text-slate-500 dark:text-slate-400 max-w-xs mt-1">
                  Select a host from the dropdown above to connect and explore files via SFTP.
                </p>
              </div>

              <!-- When Connected: Remote Files Table -->
              <table v-else class="w-full text-left text-xs border-collapse">
                <thead class="bg-slate-50 dark:bg-[#0f131d] text-slate-500 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800 sticky top-0 z-10 select-none">
                  <tr>
                    <th @click="toggleDestSort('name')" class="py-2 px-3.5 cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center gap-1">
                        <span>Name</span>
                        <ChevronUp v-if="destSortKey === 'name' && destSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="destSortKey === 'name' && !destSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th @click="toggleDestSort('size')" class="py-2 px-3 w-20 text-right cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center justify-end gap-1">
                        <span>Size</span>
                        <ChevronUp v-if="destSortKey === 'size' && destSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="destSortKey === 'size' && !destSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th @click="toggleDestSort('modTime')" class="py-2 px-3 w-36 cursor-pointer hover:text-slate-900 dark:hover:text-white transition">
                      <div class="flex items-center gap-1">
                        <span>Modified</span>
                        <ChevronUp v-if="destSortKey === 'modTime' && destSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                        <ChevronDown v-else-if="destSortKey === 'modTime' && !destSortAsc" class="w-3 h-3 text-slate-700 dark:text-slate-300" />
                      </div>
                    </th>
                    <th class="py-2 px-3 w-20">Kind</th>
                    <th class="py-2 px-3 w-24 font-mono">Perms</th>
                    <th class="py-2 px-3 w-20 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-[11px] text-slate-700 dark:text-slate-300">
                  <!-- Parent Directory row -->
                  <tr
                    v-if="sftpDestCurrentPath !== '/'"
                    @dblclick="navigateDestParent"
                    class="hover:bg-slate-50 dark:hover:bg-slate-800/40 cursor-pointer"
                  >
                    <td colspan="6" class="py-2 px-3.5 flex items-center gap-2 font-semibold text-slate-600 dark:text-slate-400">
                      <Folder class="w-4 h-4 text-amber-500 shrink-0" />
                      <span>..</span>
                    </td>
                  </tr>

                  <!-- Directory and File Rows -->
                  <tr
                    v-for="file in sortedDestFiles"
                    :key="file.name"
                    @click="selectedRemoteFile = file"
                    @dblclick="file.isDir ? fetchDestSftpFiles(`${sftpDestCurrentPath === '/' ? '' : sftpDestCurrentPath}/${file.name}`) : downloadRemoteFile(file)"
                    :class="[
                      'cursor-pointer transition',
                      selectedRemoteFile?.name === file.name
                        ? 'bg-blue-50/80 dark:bg-blue-950/40 text-blue-900 dark:text-blue-200 font-medium'
                        : 'hover:bg-slate-50 dark:hover:bg-slate-800/40'
                    ]"
                  >
                    <td class="py-2 px-3.5 flex items-center gap-2 truncate max-w-[200px]">
                      <Folder v-if="file.isDir" class="w-4 h-4 text-amber-500 shrink-0" />
                      <File v-else class="w-4 h-4 text-slate-400 shrink-0" />
                      <span class="truncate" :class="file.isDir ? 'font-semibold text-slate-900 dark:text-white' : ''">{{ file.name }}</span>
                    </td>
                    <td class="py-2 px-3 text-right font-mono text-slate-500 dark:text-slate-400">
                      {{ file.isDir ? '-' : formatFileSize(file.size) }}
                    </td>
                    <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[10px] truncate">
                      {{ formatSftpDate(file.modTime) }}
                    </td>
                    <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[10px] truncate">
                      {{ getFileTypeLabel(file.name, file.isDir) }}
                    </td>
                    <td class="py-2 px-3 font-mono text-[10px] text-slate-400">
                      {{ getFilePermissions(file) }}
                    </td>
                    <td class="py-2 px-3 text-right whitespace-nowrap">
                      <div class="flex items-center justify-end gap-1">
                        <button
                          v-if="!file.isDir"
                          @click.stop="downloadRemoteFile(file)"
                          class="p-1 rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
                          title="Download file"
                        >
                          <Download class="w-3.5 h-3.5" />
                        </button>
                        <button
                          @click.stop="confirmDeleteRemoteItem(file)"
                          class="p-1 rounded text-slate-400 hover:text-rose-500 dark:hover:text-rose-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
                          title="Delete item"
                        >
                          <Trash2 class="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>

                  <!-- Empty Directory State -->
                  <tr v-if="sortedDestFiles.length === 0 && !sftpDestLoading">
                    <td colspan="6" class="py-16 text-center text-slate-400 dark:text-slate-500 text-xs">
                      Directory is empty
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

        </div>

        <!-- 3. COLLAPSIBLE CONSOLE DRAWER (When Toggled) -->
        <div
          v-if="showSftpLogs"
          class="max-h-36 overflow-y-auto font-mono text-[11px] p-2.5 px-4 bg-slate-900 text-slate-200 border-t border-slate-800 space-y-0.5 select-text shrink-0"
        >
          <div v-for="(log, idx) in sftpCommandLogs" :key="idx" class="flex items-center gap-2">
            <span class="text-slate-500 text-[10px] select-none">{{ log.time }}</span>
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
          <div v-if="sftpCommandLogs.length === 0" class="text-slate-500 text-[10px]">
            Status: SFTP subsystem session ready. Connected to endpoints.
          </div>
        </div>

        <!-- 4. BOTTOM STATUS BAR -->
        <div class="px-4 py-2 bg-slate-50 dark:bg-[#121622] border-t border-slate-200 dark:border-slate-800 text-[11px] text-slate-500 dark:text-slate-400 flex flex-wrap items-center justify-between gap-3 shrink-0">
          <div class="flex items-center gap-3">
            <span class="font-medium text-slate-700 dark:text-slate-300">
              Transfer Queue: {{ sftpTransferQueue.length }}
            </span>
            <div v-if="sftpTransferQueue.length > 0" class="flex items-center gap-1.5 font-mono text-[10px]">
              <span class="text-emerald-600 dark:text-emerald-400 font-medium truncate max-w-xs">
                Active: {{ sftpTransferQueue[0].name }}
              </span>
              <span class="text-slate-400">({{ sftpTransferQueue[0].status }})</span>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <span class="font-mono text-[10px] hidden sm:inline">
              Local: {{ sortedLocalFiles.length }} items | Remote: {{ sortedDestFiles.length }} items
            </span>
            <button
              @click="showSftpLogs = !showSftpLogs"
              :class="[
                'px-2.5 py-1 rounded-lg text-xs font-medium border transition flex items-center gap-1.5 cursor-pointer',
                showSftpLogs
                  ? 'bg-blue-600 border-blue-500 text-white'
                  : 'bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white'
              ]"
              title="Toggle SFTP console commands log"
            >
              <SquareTerminal class="w-3.5 h-3.5" />
              <span>Console Log</span>
              <span v-if="sftpCommandLogs.length > 0" class="px-1.5 py-0.2 rounded-full bg-slate-200 dark:bg-slate-700 text-[10px]">
                {{ sftpCommandLogs.length }}
              </span>
            </button>
          </div>
        </div>

      </div>
    </div>

    <!-- STANDARD HCP DELETE CONFIRMATION MODAL FOR SFTP -->
    <div
      v-if="showSftpDeleteModal && sftpItemToDelete"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            Delete {{ sftpItemToDelete.isDir ? 'Folder' : 'File' }}?
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ sftpItemToDelete.name }}</strong>? This action cannot be undone.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showSftpDeleteModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer font-medium"
          >
            Cancel
          </button>
          <button
            @click="executeDeleteRemoteItem"
            :disabled="deletingSftpItem"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ deletingSftpItem ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: CREATE NEW SFTP FOLDER -->
    <div
      v-if="showSftpMkdirModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <FolderPlus class="w-4 h-4 text-slate-500 dark:text-slate-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">New Folder</h3>
          </div>
          <button @click="showSftpMkdirModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="executeCreateFolder" class="space-y-3.5 text-xs">
          <div>
            <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">Folder Name</label>
            <input
              v-model="sftpNewFolderName"
              required
              placeholder="e.g. backup, logs, data"
              autofocus
              class="w-full bg-slate-50 dark:bg-[#181d2a] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono text-xs"
            />
          </div>

          <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
            <button
              type="button"
              @click="showSftpMkdirModal = false"
              class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer font-medium"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="creatingSftpFolder || !sftpNewFolderName.trim()"
              class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
            >
              {{ creatingSftpFolder ? 'Creating...' : 'Create Folder' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Add New Remote Server -->
    <div
      v-if="isHostModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
    >
      <div class="bg-white dark:bg-[#1b1e26] border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-lg p-6 space-y-4 shadow-2xl max-h-[92vh] overflow-y-auto">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Server class="w-4 h-4 text-slate-500 dark:text-slate-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">{{ hostForm.id ? 'Edit Remote Server' : 'Add New Remote Server' }}</h3>
          </div>
          <button @click="isHostModalOpen = false" class="text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="handleSaveHost" class="space-y-3.5 text-xs">
          <!-- Protocol & Server Type Selector -->
          <div>
            <label class="block text-slate-600 dark:text-slate-400 mb-1.5 font-medium">Server Type & Protocol</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                @click="selectProtocolPreset('ssh')"
                :class="[
                  'flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg border text-xs font-semibold transition cursor-pointer',
                  selectedProtocolPreset === 'ssh'
                    ? 'bg-blue-600 border-blue-500 text-white shadow-xs'
                    : 'bg-slate-50 dark:bg-[#14161b] border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                <SquareTerminal class="w-3.5 h-3.5" />
                <span>Linux (SSH)</span>
              </button>
              <button
                type="button"
                @click="selectProtocolPreset('rdp')"
                :class="[
                  'flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg border text-xs font-semibold transition cursor-pointer',
                  selectedProtocolPreset === 'rdp'
                    ? 'bg-blue-600 border-blue-500 text-white shadow-xs'
                    : 'bg-slate-50 dark:bg-[#14161b] border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                <Monitor class="w-3.5 h-3.5" />
                <span>Windows (RDP)</span>
              </button>
            </div>
            <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
              <span v-if="selectedProtocolPreset === 'ssh'">Secure Shell CLI for Linux VPS, Ubuntu/Debian/CentOS, and networking equipment (Port 22).</span>
              <span v-else>Remote Desktop Protocol for Windows Server and Windows 10/11 Pro (Port 3389). Standard .rdp file is provided for high-performance native connection.</span>
            </p>
          </div>

          <div>
            <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">Server Name / Identifier</label>
            <input
              v-model="hostForm.name"
              required
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              :placeholder="selectedProtocolPreset === 'rdp' ? 'e.g. Windows Server 2022 / Workstation' : 'e.g. Server - Bifrost'"
            />
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div class="col-span-2">
              <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">Server IP / Domain</label>
              <input
                v-model="hostForm.host"
                required
                class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-mono"
                placeholder="10.20.3.1"
              />
            </div>
            <div>
              <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">Port</label>
              <input
                v-model.number="hostForm.port"
                type="number"
                required
                class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">Username</label>
              <input
                v-model="hostForm.username"
                required
                class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
                :placeholder="selectedProtocolPreset === 'rdp' ? 'Administrator' : 'root'"
              />
            </div>
            <div>
              <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">Group</label>
              <input
                v-model="hostForm.groupName"
                list="group-options"
                required
                class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
                placeholder="Production"
              />
              <datalist id="group-options">
                <option v-for="g in existingGroupNames" :key="g" :value="g" />
              </datalist>
            </div>
          </div>

          <!-- Authentication Method Toggle (SSH Only) -->
          <div v-if="selectedProtocolPreset === 'ssh'">
            <label class="block text-slate-600 dark:text-slate-400 mb-1.5 font-medium">Authentication Method</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                @click="hostForm.authType = 'password'"
                :class="[
                  'flex items-center justify-center gap-1.5 py-2 px-3 rounded-lg border text-xs font-semibold transition cursor-pointer',
                  hostForm.authType === 'password'
                    ? 'bg-blue-50 dark:bg-blue-950/40 border-blue-500 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/30'
                    : 'bg-slate-50 dark:bg-[#14161b] border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                <Lock class="w-3.5 h-3.5 text-slate-400" />
                <span>Password</span>
              </button>
              <button
                type="button"
                @click="hostForm.authType = 'key'"
                :class="[
                  'flex items-center justify-center gap-1.5 py-2 px-3 rounded-lg border text-xs font-semibold transition cursor-pointer',
                  hostForm.authType === 'key'
                    ? 'bg-blue-50 dark:bg-blue-950/40 border-blue-500 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/30'
                    : 'bg-slate-50 dark:bg-[#14161b] border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                <Key class="w-3.5 h-3.5 text-slate-400" />
                <span>SSH Key</span>
              </button>
            </div>
          </div>

          <!-- Password Field (RDP or SSH Password Mode) -->
          <div v-if="selectedProtocolPreset === 'rdp' || hostForm.authType === 'password'">
            <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">
              <span class="flex items-center gap-1.5">
                <Lock class="w-3.5 h-3.5 text-slate-400" />
                <span>Password (AES-256-GCM Encrypted)</span>
              </span>
            </label>
            <input
              v-model="hostForm.password"
              type="password"
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              placeholder="••••••••"
            />
            <p v-if="selectedProtocolPreset === 'rdp'" class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
              Password can be saved to facilitate Windows Remote Desktop (RDP) connections.
            </p>
            <p v-else-if="hostForm.id" class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
              Leave blank to keep existing saved password.
            </p>
          </div>

          <!-- SSH Key Section (SSH Key Mode) -->
          <div v-if="selectedProtocolPreset === 'ssh' && hostForm.authType === 'key'" class="space-y-3">
            <div>
              <div class="flex items-center justify-between mb-1">
                <label class="block text-slate-600 dark:text-slate-400 font-medium">
                  <span class="flex items-center gap-1.5">
                    <Key class="w-3.5 h-3.5 text-slate-400" />
                    <span>SSH Private Key (AES-256-GCM Encrypted)</span>
                  </span>
                </label>
                <label class="inline-flex items-center gap-1 text-[11px] text-blue-600 dark:text-blue-400 hover:underline cursor-pointer">
                  <Upload class="w-3 h-3" />
                  <span>Upload Key File</span>
                  <input
                    type="file"
                    class="hidden"
                    accept=".pem,.key,.id_rsa,.id_ed25519,.id_ecdsa,text/*"
                    @change="handleKeyFileUpload"
                  />
                </label>
              </div>
              <textarea
                v-model="hostForm.sshKey"
                rows="4"
                class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono text-[11px] focus:outline-none focus:border-blue-500 resize-y"
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
              ></textarea>
              <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                <span v-if="hostForm.id">Leave blank to keep existing SSH key. Supports RSA, Ed25519, ECDSA keys.</span>
                <span v-else>Paste private key in PEM/OpenSSH format or upload file. Encrypted before saving.</span>
              </p>
            </div>

            <div>
              <label class="block text-slate-600 dark:text-slate-400 mb-1 font-medium">
                <span class="flex items-center gap-1.5">
                  <Lock class="w-3.5 h-3.5 text-slate-400" />
                  <span>Passphrase / Sudo Password (Optional)</span>
                </span>
              </label>
              <input
                v-model="hostForm.password"
                type="password"
                class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
                placeholder="Leave blank if key has no passphrase"
              />
              <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                Optional: Passphrase if private key is encrypted, or password used for sudo commands.
              </p>
            </div>
          </div>

          <!-- Connection Test Feedback Banner -->
          <div
            v-if="testResult"
            :class="[
              'p-2.5 rounded-lg text-xs flex items-start gap-2',
              testResult.success
                ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border border-emerald-500/30'
                : 'bg-rose-500/10 text-rose-700 dark:text-rose-400 border border-rose-500/30'
            ]"
          >
            <CheckCircle2 v-if="testResult.success" class="w-4 h-4 shrink-0 mt-0.5 text-emerald-500" />
            <AlertCircle v-else class="w-4 h-4 shrink-0 mt-0.5 text-rose-500" />
            <span class="break-all">{{ testResult.message }}</span>
          </div>

          <div class="flex items-center justify-between pt-3 border-t border-slate-200 dark:border-slate-800">
            <button
              type="button"
              @click="handleTestConnection"
              :disabled="isTestingConnection || !hostForm.host || !hostForm.username"
              class="px-3 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white border border-slate-300 dark:border-transparent text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
            >
              <Loader2 v-if="isTestingConnection" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ isTestingConnection ? 'Testing...' : 'Test Connection' }}</span>
            </button>
            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="isHostModalOpen = false"
                class="px-4 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white border border-slate-300 dark:border-transparent text-xs font-semibold transition cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="submit"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg text-xs cursor-pointer shadow-xs transition"
              >
                Save Server
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Add New Group -->
    <div
      v-if="isGroupModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
    >
      <div class="bg-white dark:bg-[#1b1e26] border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <FolderPlus class="w-4 h-4 text-blue-600 dark:text-brand-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">Create New Group</h3>
          </div>
          <button @click="isGroupModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-4 text-xs">
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1">Group Name</label>
            <input
              v-model="newGroupName"
              required
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-medium"
              placeholder="e.g. Staging, Core Infra, DMZ, Production"
            />
          </div>

          <!-- Group Action Mode Selector -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-semibold mb-1.5">Action / Server Assignment</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                @click="groupCreationMode = 'existing'"
                :class="[
                  'flex items-center gap-2 p-2.5 rounded-lg border text-left transition cursor-pointer',
                  groupCreationMode === 'existing'
                    ? 'border-blue-600 bg-blue-50 dark:bg-blue-600/10 text-blue-700 dark:text-blue-300 font-semibold'
                    : 'border-slate-300 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800/60 text-slate-600 dark:text-slate-400'
                ]"
              >
                <Server class="w-4 h-4 shrink-0" />
                <div class="truncate">
                  <div class="text-[11px] leading-tight">Select Existing</div>
                  <div class="text-[10px] opacity-75">Assign servers</div>
                </div>
              </button>

              <button
                type="button"
                @click="groupCreationMode = 'new'"
                :class="[
                  'flex items-center gap-2 p-2.5 rounded-lg border text-left transition cursor-pointer',
                  groupCreationMode === 'new'
                    ? 'border-blue-600 bg-blue-50 dark:bg-blue-600/10 text-blue-700 dark:text-blue-300 font-semibold'
                    : 'border-slate-300 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800/60 text-slate-600 dark:text-slate-400'
                ]"
              >
                <Plus class="w-4 h-4 shrink-0" />
                <div class="truncate">
                  <div class="text-[11px] leading-tight">Add New Server</div>
                  <div class="text-[10px] opacity-75">Register to group</div>
                </div>
              </button>
            </div>
          </div>

          <!-- Option 1: Existing Servers Checklist -->
          <div v-if="groupCreationMode === 'existing'" class="space-y-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-slate-600 dark:text-slate-400 font-medium text-[11px]">
                Select servers to add to <strong class="text-blue-600 dark:text-blue-400">{{ newGroupName || 'this group' }}</strong>:
              </span>
              <button
                type="button"
                @click="toggleSelectAllHostsForGroup"
                class="text-[11px] font-semibold text-blue-600 dark:text-brand-400 hover:underline cursor-pointer shrink-0"
              >
                {{ selectedHostIdsForGroup.length === selectableHostsForGroup.length && selectableHostsForGroup.length > 0 ? 'Clear All' : 'Select All' }}
              </button>
            </div>

            <!-- Filter search input -->
            <div class="relative">
              <Search class="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                v-model="groupSearchQuery"
                class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-300 dark:border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
                placeholder="Search server name or IP..."
              />
            </div>

            <!-- Server list -->
            <div class="max-h-48 overflow-y-auto space-y-1 border border-slate-200 dark:border-slate-800 rounded-lg p-2 bg-slate-50/50 dark:bg-[#12151d]">
              <div v-if="hosts.length === 0" class="text-center py-4 text-slate-400 text-xs">
                No existing servers found. Choose "Add New Server".
              </div>
              <div v-else-if="selectableHostsForGroup.length === 0" class="text-center py-4 text-slate-400 text-xs">
                No servers match "{{ groupSearchQuery }}".
              </div>
              <label
                v-for="h in selectableHostsForGroup"
                :key="h.id"
                class="flex items-center gap-2.5 p-2 rounded-md hover:bg-white dark:hover:bg-slate-800/80 cursor-pointer transition border border-transparent hover:border-slate-200 dark:hover:border-slate-700/50"
              >
                <input
                  type="checkbox"
                  :value="h.id"
                  v-model="selectedHostIdsForGroup"
                  class="rounded text-blue-600 focus:ring-blue-500 cursor-pointer shrink-0"
                />
                <div class="flex-1 min-w-0">
                  <div class="flex items-center justify-between gap-1">
                    <span class="font-semibold text-slate-900 dark:text-white truncate">{{ h.name }}</span>
                    <span class="px-1.5 py-0.5 text-[9px] rounded bg-slate-200 dark:bg-slate-800 text-slate-600 dark:text-slate-400 shrink-0 font-medium">
                      {{ h.groupName || 'Default' }}
                    </span>
                  </div>
                  <div class="text-[11px] font-mono text-slate-500 dark:text-slate-400 truncate">
                    {{ h.username }}@{{ h.host }}:{{ h.port }}
                  </div>
                </div>
              </label>
            </div>
            <div class="text-[11px] text-slate-500 dark:text-slate-400">
              {{ selectedHostIdsForGroup.length }} of {{ hosts.length }} server(s) selected
            </div>
          </div>

          <!-- Option 2: Add New Server Notice -->
          <div v-else class="p-3 bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800/50 rounded-lg text-blue-800 dark:text-blue-300 text-xs space-y-1">
            <div class="font-semibold flex items-center gap-1.5">
              <Plus class="w-4 h-4" />
              <span>Create Server Directly in Group</span>
            </div>
            <p class="text-[11px] opacity-90">
              Clicking proceed will open the server connection form pre-configured with the group <strong>{{ newGroupName || '(Enter name above)' }}</strong>.
            </p>
          </div>

          <!-- Modal Action Buttons -->
          <div class="flex items-center justify-end gap-3 pt-3 border-t border-slate-200 dark:border-slate-800">
            <button
              type="button"
              @click="isGroupModalOpen = false"
              class="px-4 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white border border-slate-300 dark:border-transparent text-xs font-semibold transition cursor-pointer"
            >
              Cancel
            </button>
            <button
              @click="handleCreateGroup"
              :disabled="isSubmittingGroup || !newGroupName.trim()"
              class="flex items-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold rounded-lg text-xs transition cursor-pointer shadow-sm"
            >
              <RotateCw v-if="isSubmittingGroup" class="w-3.5 h-3.5 animate-spin" />
              <span>
                {{
                  groupCreationMode === 'new'
                    ? 'Proceed to Add Server'
                    : isSubmittingGroup
                      ? 'Saving...'
                      : `Save Group (${selectedHostIdsForGroup.length} Selected)`
                }}
              </span>
              <ArrowRight v-if="groupCreationMode === 'new' && !isSubmittingGroup" class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- CHROME-STYLE CONTEXT MENUS (TAB & GROUP) -->
    <!-- ================================================================= -->
    
    <!-- 1. TAB CONTEXT MENU -->
    <div
      v-if="tabContextMenu.show && tabContextMenu.session"
      :style="{ top: `${tabContextMenu.y}px`, left: `${tabContextMenu.x}px` }"
      class="fixed z-[100] bg-[#1a1d26] border border-slate-700/90 rounded-xl shadow-2xl py-1.5 min-w-[210px] text-xs font-sans text-slate-200 animate-in fade-in zoom-in-95 duration-100"
      @click.stop
    >
      <div class="px-3 py-1.5 text-[10px] uppercase font-bold text-slate-500 tracking-wider border-b border-slate-800/80 mb-1 flex items-center gap-1.5">
        <SquareTerminal class="w-3 h-3 text-brand-400" />
        <span class="truncate max-w-[170px]">{{ tabContextMenu.session.displayName || tabContextMenu.session.host.name }}</span>
      </div>

      <!-- Add to New Group -->
      <button
        @click="createGroupForSession(tabContextMenu.session); closeAllContextMenus()"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-700/60 hover:text-white transition"
      >
        <FolderPlus class="w-3.5 h-3.5 text-brand-400" />
        <span>Add tab to new group</span>
      </button>

      <!-- Add to Existing Group submenu / list if any -->
      <div v-if="tabGroups.length > 0" class="relative group/submenu">
        <div class="px-3 py-1.5 flex items-center justify-between text-slate-300 hover:bg-slate-700/60 hover:text-white transition cursor-pointer">
          <span class="flex items-center gap-2">
            <Folder class="w-3.5 h-3.5 text-amber-400" />
            <span>Add to group</span>
          </span>
          <ChevronRight class="w-3 h-3 text-slate-500" />
        </div>
        <!-- Submenu Flyout -->
        <div class="hidden group-hover/submenu:block absolute left-full top-0 ml-1 bg-[#1a1d26] border border-slate-700 rounded-xl shadow-2xl py-1 min-w-[160px]">
          <button
            v-for="grp in tabGroups"
            :key="grp.id"
            @click="addSessionToGroup(tabContextMenu.session, grp.id); closeAllContextMenus()"
            class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-700/60 hover:text-white transition text-xs"
          >
            <span class="w-2.5 h-2.5 rounded-full" :style="{ backgroundColor: grp.color }"></span>
            <span class="truncate">{{ grp.name }}</span>
          </button>
        </div>
      </div>

      <!-- Remove from group (if currently in a group) -->
      <button
        v-if="tabContextMenu.session.groupId"
        @click="removeSessionFromGroup(tabContextMenu.session); closeAllContextMenus()"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-700/60 text-amber-400 hover:text-amber-300 transition"
      >
        <X class="w-3.5 h-3.5" />
        <span>Remove from group</span>
      </button>

      <div class="my-1 border-t border-slate-800/80"></div>

      <!-- Reconnect Tab -->
      <button
        @click="reconnectTerminal(tabContextMenu.session, false); closeAllContextMenus()"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-700/60 hover:text-white transition text-xs"
      >
        <RotateCw class="w-3.5 h-3.5 text-slate-400" />
        <span>Reconnect tab</span>
      </button>

      <!-- Reset Shell (Force New) -->
      <button
        @click="reconnectTerminal(tabContextMenu.session, true); closeAllContextMenus()"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-700/60 text-amber-400 hover:text-amber-300 transition text-xs"
      >
        <RefreshCw class="w-3.5 h-3.5 text-amber-400" />
        <span>Reset shell (Force new)</span>
      </button>

      <!-- Duplicate Tab -->
      <button
        @click="duplicateSession(tabContextMenu.session); closeAllContextMenus()"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-700/60 hover:text-white transition"
      >
        <Copy class="w-3.5 h-3.5 text-slate-400" />
        <span>Duplicate tab</span>
      </button>

      <!-- Close Tab -->
      <button
        @click="closeSession(tabContextMenu.session || tabContextMenu.globalIndex); closeAllContextMenus()"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-rose-500/20 text-rose-400 hover:text-rose-300 transition"
      >
        <X class="w-3.5 h-3.5" />
        <span>Close tab</span>
      </button>

      <!-- Close Other Tabs -->
      <button
        v-if="openSessions.length > 1"
        @click="closeOtherSessions(tabContextMenu.globalIndex); closeAllContextMenus()"
        class="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-700/60 text-slate-400 hover:text-slate-200 transition"
      >
        <Square class="w-3.5 h-3.5" />
        <span>Close other tabs</span>
      </button>
    </div>

    <!-- 2. GROUP EDITOR MENU / POPOVER -->
    <div
      v-if="groupMenu.show && groupMenu.group"
      :style="{ top: `${groupMenu.y}px`, left: `${groupMenu.x}px` }"
      class="fixed z-[100] bg-[#1a1d26] border border-slate-700 rounded-xl shadow-2xl p-3 min-w-[240px] text-xs font-sans text-slate-200 animate-in fade-in zoom-in-95 duration-100 space-y-3"
      @click.stop
    >
      <!-- Group Name Input -->
      <div>
        <label class="block text-[10px] uppercase font-bold text-slate-400 tracking-wider mb-1">Group Name</label>
        <input
          v-model="groupMenu.editingName"
          @keydown.enter="saveGroupName"
          class="w-full bg-[#11141c] border border-slate-700 rounded-lg px-2.5 py-1.5 text-xs text-white focus:outline-none focus:border-brand-500 font-medium"
          placeholder="Name this group"
          autofocus
        />
      </div>

      <!-- Chrome Palette Preset Colors -->
      <div>
        <label class="block text-[10px] uppercase font-bold text-slate-400 tracking-wider mb-1.5">Color</label>
        <div class="flex items-center gap-1.5 flex-wrap">
          <button
            v-for="c in CHROME_COLORS"
            :key="c.name"
            @click="setGroupColor(groupMenu.group, c.hex)"
            class="w-5 h-5 rounded-full transition flex items-center justify-center relative hover:scale-110"
            :style="{ backgroundColor: c.hex }"
            :title="c.name"
          >
            <Check v-if="groupMenu.group.color === c.hex" class="w-3 h-3 text-white drop-shadow-md stroke-[3]" />
          </button>
        </div>
      </div>

      <div class="border-t border-slate-800/80 pt-2 space-y-1">
        <!-- New Tab in Group -->
        <button
          @click="() => {
            const firstSess = openSessions.find(s => s.groupId === groupMenu.group?.id);
            if (firstSess) duplicateSession(firstSess);
            closeAllContextMenus();
          }"
          class="w-full px-2 py-1.5 text-left rounded-lg flex items-center gap-2 hover:bg-slate-700/60 hover:text-white transition"
        >
          <Plus class="w-3.5 h-3.5 text-brand-400" />
          <span>New tab in group</span>
        </button>

        <!-- Ungroup -->
        <button
          @click="ungroupTabs(groupMenu.group.id); closeAllContextMenus()"
          class="w-full px-2 py-1.5 text-left rounded-lg flex items-center gap-2 hover:bg-slate-700/60 text-slate-300 hover:text-white transition"
        >
          <Layers class="w-3.5 h-3.5 text-slate-400" />
          <span>Ungroup</span>
        </button>

        <!-- Close Group -->
        <button
          @click="closeGroupTabs(groupMenu.group.id); closeAllContextMenus()"
          class="w-full px-2 py-1.5 text-left rounded-lg flex items-center gap-2 hover:bg-rose-500/20 text-rose-400 hover:text-rose-300 transition"
        >
          <Trash2 class="w-3.5 h-3.5" />
          <span>Close group</span>
        </button>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- ADD FIREWALL RULE MODAL -->
    <!-- ================================================================= -->
    <div
      v-if="isFirewallModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-150"
    >
      <div class="bg-[#1b1e26] border border-slate-700/80 rounded-2xl w-full max-w-md shadow-2xl overflow-hidden flex flex-col">
        <!-- Modal Header -->
        <div class="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-[#20242e]">
          <h3 class="text-sm font-bold text-white tracking-wide">Add Firewall Rule</h3>
          <button
            @click="isFirewallModalOpen = false"
            class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-700/50 transition"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Form Body -->
        <form @submit.prevent="handleAddFirewallRule" class="p-6 space-y-4 text-xs font-mono">
          <!-- Application Type -->
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1.5 font-sans">
              Application Type
            </label>
            <select
              v-model="firewallAppType"
              @change="handleAppTypeChange"
              class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-blue-500 font-sans text-xs"
            >
              <option value="Custom">Custom</option>
              <option value="SSH">SSH</option>
              <option value="TELNET">TELNET</option>
              <option value="MySQL">MySQL</option>
              <option value="SQL Server">SQL Server</option>
              <option value="HTTP">HTTP</option>
              <option value="HTTPS">HTTPS</option>
              <option value="All TCP">All TCP</option>
              <option value="All UDP">All UDP</option>
              <option value="ALL">ALL</option>
            </select>
          </div>

          <!-- Protocol & Port Range (Visible when Custom or preset overview) -->
          <div v-if="firewallAppType === 'Custom'" class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] font-medium text-slate-400 mb-1 font-sans">
                Protocol <span class="text-rose-400">*</span>
              </label>
              <select
                v-model="firewallForm.protocol"
                class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-blue-500 font-sans text-xs"
              >
                <option value="TCP">TCP</option>
                <option value="UDP">UDP</option>
                <option value="ALL">ALL</option>
                <option value="ICMP">ICMP</option>
              </select>
            </div>

            <div>
              <label class="block text-[11px] font-medium text-slate-400 mb-1 font-sans">
                Port Range <span class="text-rose-400">*</span>
              </label>
              <input
                v-model="firewallForm.portRange"
                type="text"
                placeholder="e.g. 22, 80,443, or ALL"
                class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 text-xs"
                :disabled="firewallForm.protocol === 'ICMP'"
              />
            </div>
          </div>

          <!-- Preset Details Indicator when not Custom -->
          <div v-else class="p-2.5 bg-[#13161f] border border-slate-800 rounded-lg flex items-center justify-between text-xs font-mono">
            <div class="flex items-center gap-2">
              <span class="text-slate-400">Protocol:</span>
              <span class="text-sky-400 font-bold">{{ firewallForm.protocol }}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-slate-400">Port Range:</span>
              <span class="text-emerald-400 font-bold">{{ firewallForm.portRange }}</span>
            </div>
          </div>

          <!-- Action & Source IP Address -->
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] font-medium text-slate-400 mb-1 font-sans">
                Action
              </label>
              <select
                v-model="firewallForm.action"
                class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-blue-500 font-sans text-xs"
              >
                <option value="ALLOW">ALLOW</option>
                <option value="DENY">DENY</option>
              </select>
            </div>

            <div>
              <label class="block text-[11px] font-medium text-slate-400 mb-1 font-sans">
                Source IP Address
              </label>
              <input
                v-model="firewallForm.sourceIp"
                type="text"
                placeholder="0.0.0.0/0"
                class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 text-xs"
              />
            </div>
          </div>

          <!-- Description (matching reference "Optional description") -->
          <div>
            <label class="block text-[11px] font-medium text-slate-400 mb-1 font-sans">
              Description
            </label>
            <input
              v-model="firewallForm.description"
              type="text"
              placeholder="Optional description"
              class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 font-sans text-xs"
            />
          </div>

          <!-- Modal Footer -->
          <div class="flex items-center justify-end gap-2 pt-4 border-t border-slate-200 dark:border-slate-800 font-sans">
            <button
              type="button"
              @click="isFirewallModalOpen = false"
              class="px-4 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white border border-slate-300 dark:border-transparent text-xs font-semibold transition cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="isFirewallSubmitting"
              class="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow transition disabled:opacity-50"
            >
              <RotateCw v-if="isFirewallSubmitting" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ isFirewallSubmitting ? 'Adding...' : 'Add Rule' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- SHARE ACCESS MODAL -->
    <!-- ================================================================= -->
    <div
      v-if="isShareModalOpen && selectedHostForShare"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-150"
    >
      <div class="bg-[#1b1e26] border border-slate-700/80 rounded-2xl w-full max-w-lg shadow-2xl overflow-hidden flex flex-col">
        <!-- Modal Header -->
        <div class="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-[#20242e]">
          <div class="flex items-center gap-3">
            <div class="p-2 rounded-xl bg-slate-800 text-slate-300 border border-slate-700">
              <Share2 class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-white tracking-wide">Share Server Access</h3>
              <p class="text-xs text-slate-400">
                {{ selectedHostForShare.name }} &bull; <span class="font-mono text-slate-300">{{ selectedHostForShare.host }}</span>
              </p>
            </div>
          </div>
          <button
            @click="isShareModalOpen = false"
            class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-700/50 transition"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-6 space-y-4">
          <!-- Server Owner Banner -->
          <div class="p-3 rounded-xl bg-slate-900/90 border border-slate-800 flex items-center justify-between text-xs font-sans">
            <span class="text-slate-400 font-medium">Server Owner</span>
            <span class="text-slate-300 font-semibold font-mono bg-slate-800 px-2.5 py-1 rounded-lg border border-slate-700">
              @{{ selectedHostForShare.ownerUsername || 'You' }} (Full Ownership)
            </span>
          </div>

          <!-- Grant Access Form -->
          <div class="p-4 bg-[#14161b] border border-slate-800/90 rounded-xl space-y-3">
            <h4 class="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2 font-sans">
              <Users class="w-3.5 h-3.5 text-slate-400" />
              <span>Grant Access to User</span>
            </h4>
            
            <div class="grid grid-cols-1 sm:grid-cols-12 gap-3">
              <div class="sm:col-span-6 space-y-1">
                <label class="block text-[11px] text-slate-400 font-medium">Select User</label>
                <select
                  v-model="shareForm.userId"
                  class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-blue-500 transition font-sans"
                >
                  <option value="" disabled>Choose user...</option>
                  <option
                    v-for="u in availableUsers.filter(u => u.id !== authStore.user?.id && (!selectedHostForShare.userId || u.id !== selectedHostForShare.userId))"
                    :key="u.id"
                    :value="u.id"
                  >
                    {{ u.username }} ({{ u.role }})
                  </option>
                </select>
              </div>

              <div class="sm:col-span-4 space-y-1">
                <label class="block text-[11px] text-slate-400 font-medium">Access Level</label>
                <select
                  v-model="shareForm.permission"
                  class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-blue-500 transition font-sans"
                >
                  <option value="read">Read Only (SSH & Telemetry)</option>
                  <option value="manage">Full Control (Manage & SFTP)</option>
                </select>
              </div>

              <div class="sm:col-span-2 flex items-end">
                <button
                  type="button"
                  @click="handleGrantShare"
                  :disabled="!shareForm.userId || isShareSubmitting"
                  class="w-full py-2 px-3 bg-blue-600 hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-lg transition shadow-sm flex items-center justify-center gap-1 cursor-pointer"
                >
                  <RotateCw v-if="isShareSubmitting" class="w-3.5 h-3.5 animate-spin" />
                  <span>{{ isShareSubmitting ? '...' : 'Grant' }}</span>
                </button>
              </div>
            </div>
          </div>

          <!-- Active Shares List -->
          <div class="space-y-2">
            <h4 class="text-xs font-bold text-slate-400 uppercase tracking-wider font-sans">
              Currently Shared Users ({{ hostShares.length }})
            </h4>

            <div v-if="isShareLoading" class="p-6 text-center text-xs text-slate-500">
              Loading access list...
            </div>
            <div v-else-if="hostShares.length === 0" class="p-6 text-center text-xs text-slate-500 bg-[#14161b] rounded-xl border border-slate-800">
              This server is private. Only you can view and connect to it.
            </div>
            <div v-else class="space-y-1.5 max-h-48 overflow-y-auto pr-1">
              <div
                v-for="s in hostShares"
                :key="s.id"
                class="p-3 bg-[#14161b] border border-slate-800/80 rounded-xl flex items-center justify-between gap-3 text-xs"
              >
                <div class="flex items-center gap-2.5">
                  <div class="w-7 h-7 rounded-full bg-slate-700 text-white font-bold text-xs flex items-center justify-center shrink-0">
                    {{ s.username.substring(0, 2).toUpperCase() }}
                  </div>
                  <div>
                    <p class="font-semibold text-white">@{{ s.username }}</p>
                    <p class="text-[10px] text-slate-500 font-mono">
                      Granted by @{{ s.sharedByUsername || 'Admin' }}
                    </p>
                  </div>
                </div>

                <div class="flex items-center gap-2">
                  <span
                    class="px-2 py-0.5 rounded text-[10px] font-semibold border bg-slate-800 text-slate-300 border-slate-700 uppercase"
                  >
                    {{ s.permission === 'manage' ? 'Full Control' : 'Read Only' }}
                  </span>
                  <button
                    @click="handleRevokeShare(s.userId)"
                    title="Revoke access"
                    class="p-1.5 text-slate-500 hover:text-rose-400 rounded-lg hover:bg-rose-500/10 transition cursor-pointer"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="flex items-center justify-end px-6 py-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-[#20242e] font-sans">
          <button
            type="button"
            @click="isShareModalOpen = false"
            class="px-4 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white border border-slate-300 dark:border-transparent text-xs font-semibold transition cursor-pointer"
          >
            Done
          </button>
        </div>
      </div>
    </div>

    <!-- Standard Delete Confirmation Modal -->
    <div
      v-if="showDeleteHostModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Remote Server?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ hostToDelete?.name }}</strong> ({{ hostToDelete?.host }})? This action cannot be undone.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteHostModal = false; hostToDelete = null"
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

    <!-- Notification Toast (Teleported to body) -->
    <Teleport to="body">
      <transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0 -translate-y-2 scale-95"
        enter-to-class="opacity-100 translate-y-0 scale-100"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100 translate-y-0 scale-100"
        leave-to-class="opacity-0 -translate-y-2 scale-95"
      >
        <div
          v-if="notification"
          class="fixed top-5 right-5 z-[9999] flex items-center gap-2.5 px-4 py-3 rounded-xl shadow-2xl text-xs font-semibold border pointer-events-auto"
          :class="
            notification.type === 'success'
              ? 'bg-emerald-50 dark:bg-[#0a2318] border-emerald-500/40 text-emerald-800 dark:text-emerald-300 shadow-emerald-950/10'
              : 'bg-rose-50 dark:bg-[#250d12] border-rose-500/40 text-rose-800 dark:text-rose-300 shadow-rose-950/10'
          "
        >
          <Check v-if="notification.type === 'success'" class="w-4 h-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
          <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-600 dark:text-rose-400" />
          <span>{{ notification.text }}</span>
        </div>
      </transition>
    </Teleport>

  </div>
</template>

<style>
.terminal-wrapper {
  width: 100%;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
  position: relative;
}

.terminal-wrapper .xterm {
  height: 100% !important;
  width: 100% !important;
  padding: 6px 10px;
}

.terminal-wrapper .xterm .xterm-viewport {
  overflow-y: auto !important;
  width: 100% !important;
}

.terminal-wrapper .xterm .xterm-screen {
  width: 100% !important;
}

/* Horizontal tab scroller styling - completely hide native scrollbar */
.tab-bar-strip {
  scrollbar-width: none !important;
  -ms-overflow-style: none !important;
}
.tab-bar-strip::-webkit-scrollbar {
  display: none !important;
  width: 0 !important;
  height: 0 !important;
}
</style>
