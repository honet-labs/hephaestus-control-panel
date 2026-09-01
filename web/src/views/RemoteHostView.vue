<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue';
import axios from 'axios';
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
  RefreshCw, 
  Trash2, 
  Radio, 
  Maximize2,
  Grid2X2,
  Columns2,
  Rows2,
  Square
} from 'lucide-vue-next';

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

interface TerminalSession {
  id: string;
  hostId: string;
  title: string;
  term?: Terminal;
  fitAddon?: FitAddon;
  ws?: WebSocket;
}

type SplitLayout = 'single' | 'split-h' | 'split-v' | 'grid-4';

const hosts = ref<RemoteHost[]>([]);
const layout = ref<SplitLayout>('single');
const activePaneIndex = ref<number>(0);
const broadcastMode = ref<boolean>(false);

// Active terminal session per pane (max 4 panes)
const paneSessions = ref<(TerminalSession | null)[]>([null, null, null, null]);

const isHostModalOpen = ref(false);
const isSftpModalOpen = ref(false);
const selectedHostForSftp = ref<RemoteHost | null>(null);
const sftpFiles = ref<any[]>([]);
const sftpCurrentPath = ref('/');

// New Host Form
const hostForm = ref<any>({
  id: '',
  name: '',
  host: '',
  port: 22,
  username: 'root',
  authType: 'password',
  password: '',
  sshKey: '',
  groupName: 'Default',
});

const fetchHosts = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res.data.success) {
      hosts.value = res.data.data || [];
    }
  } catch (err) {
    console.error('Failed to load hosts:', err);
  }
};

const connectHostToPane = async (host: RemoteHost, paneIdx: number) => {
  // Close existing session on this pane if any
  closeSession(paneIdx);

  const sessionId = `session-${paneIdx}-${Date.now()}`;
  const session: TerminalSession = {
    id: sessionId,
    hostId: host.id,
    title: `${host.name} (${host.host})`,
  };

  paneSessions.value[paneIdx] = session;
  activePaneIndex.value = paneIdx;

  await nextTick();
  initXterm(session, host, paneIdx);
};

const initXterm = (session: TerminalSession, host: RemoteHost, paneIdx: number) => {
  const container = document.getElementById(`terminal-pane-${paneIdx}`);
  if (!container) return;

  container.innerHTML = ''; // Clear container

  const term = new Terminal({
    fontFamily: 'JetBrains Mono, monospace',
    fontSize: 12,
    theme: {
      background: '#090d16',
      foreground: '#e2e8f0',
      cursor: '#22c55e',
      selectionBackground: '#1e293b',
    },
    cursorBlink: true,
  });

  const fitAddon = new FitAddon();
  term.loadAddon(fitAddon);
  term.loadAddon(new WebLinksAddon());
  term.open(container);
  
  setTimeout(() => fitAddon.fit(), 100);

  session.term = term;
  session.fitAddon = fitAddon;

  // Connect WebSocket
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/remote-host?cols=${term.cols}&rows=${term.rows}`;
  const ws = new WebSocket(wsUrl);
  session.ws = ws;

  const token = localStorage.getItem('hephaestus_token') || '';

  ws.onopen = () => {
    ws.send(JSON.stringify({
      type: 'auth',
      token,
      hostConfigId: host.id,
    }));
  };

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === 'data') {
        term.write(msg.data);
      } else if (msg.type === 'connected') {
        term.writeln(`\r\n\x1b[32m[Connected to ${host.name}]\x1b[0m\r\n`);
      } else if (msg.type === 'error') {
        term.writeln(`\r\n\x1b[31m[Error: ${msg.message}]\x1b[0m\r\n`);
      }
    } catch {
      term.write(event.data);
    }
  };

  term.onData((data) => {
    if (broadcastMode.value) {
      // Send input to all open active pane WebSocket connections
      paneSessions.value.forEach(s => {
        if (s?.ws && s.ws.readyState === WebSocket.OPEN) {
          s.ws.send(JSON.stringify({ type: 'input', data }));
        }
      });
    } else {
      // Send only to this pane
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    }
  });

  term.onResize((size) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: size.cols, rows: size.rows }));
    }
  });
};

const closeSession = (paneIdx: number) => {
  const session = paneSessions.value[paneIdx];
  if (session) {
    session.ws?.close();
    session.term?.dispose();
    paneSessions.value[paneIdx] = null;
  }
};

const changeLayout = (newLayout: SplitLayout) => {
  layout.value = newLayout;
  nextTick(() => {
    // Re-fit all active terminals
    paneSessions.value.forEach(s => s?.fitAddon?.fit());
  });
};

const getVisiblePanesCount = () => {
  switch (layout.value) {
    case 'single': return 1;
    case 'split-h': return 2;
    case 'split-v': return 2;
    case 'grid-4': return 4;
  }
};

// SFTP Functions
const openSftp = async (host: RemoteHost) => {
  selectedHostForSftp.value = host;
  sftpCurrentPath.value = '/';
  isSftpModalOpen.value = true;
  await fetchSftpFiles();
};

const fetchSftpFiles = async () => {
  if (!selectedHostForSftp.value) return;
  try {
    const res = await axios.get(`/api/v1/remote-host/${selectedHostForSftp.value.id}/sftp/list?path=${encodeURIComponent(sftpCurrentPath.value)}`);
    if (res.data.success) {
      sftpFiles.value = res.data.data || [];
    }
  } catch (err) {
    console.error('SFTP fetch failed:', err);
  }
};

const saveHost = async () => {
  try {
    const res = await axios.post('/api/v1/remote-host', hostForm.value);
    if (res.data.success) {
      isHostModalOpen.value = false;
      fetchHosts();
    }
  } catch (err) {
    console.error('Failed to save host:', err);
  }
};

const deleteHost = async (id: string) => {
  if (!confirm('Are you sure you want to delete this host?')) return;
  try {
    await axios.delete(`/api/v1/remote-host/${id}`);
    fetchHosts();
  } catch (err) {
    console.error('Failed to delete host:', err);
  }
};

const handleWindowResize = () => {
  paneSessions.value.forEach(s => s?.fitAddon?.fit());
};

onMounted(() => {
  fetchHosts();
  window.addEventListener('resize', handleWindowResize);
});

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize);
  paneSessions.value.forEach((_, idx) => closeSession(idx));
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4 font-sans">
    <!-- Header with Split-Screen & Multi-Cast Controls -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">Remote Server Terminal</h2>
        <p class="text-xs text-slate-400">Interactive SSH terminal with Split-Screen multi-view and Broadcast mode</p>
      </div>

      <div class="flex items-center gap-3">
        <!-- Broadcast Mode Toggle -->
        <button
          @click="broadcastMode = !broadcastMode"
          :class="[
            broadcastMode ? 'bg-red-500/20 text-red-400 border-red-500/40 animate-pulse font-bold' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white',
            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs border transition'
          ]"
          title="Send keystrokes simultaneously to all open terminal panes"
        >
          <Radio class="w-3.5 h-3.5" />
          {{ broadcastMode ? 'Broadcast ON (All Panes)' : 'Broadcast OFF' }}
        </button>

        <!-- Split Layout Controls -->
        <div class="flex items-center bg-slate-900 border border-slate-800 rounded-lg p-0.5 gap-0.5">
          <button
            @click="changeLayout('single')"
            :class="[layout === 'single' ? 'bg-slate-800 text-white' : 'text-slate-500 hover:text-slate-300', 'p-1.5 rounded transition']"
            title="Single Pane (1x1)"
          >
            <Square class="w-3.5 h-3.5" />
          </button>
          <button
            @click="changeLayout('split-h')"
            :class="[layout === 'split-h' ? 'bg-slate-800 text-white' : 'text-slate-500 hover:text-slate-300', 'p-1.5 rounded transition']"
            title="2 Columns (Side-by-side)"
          >
            <Columns2 class="w-3.5 h-3.5" />
          </button>
          <button
            @click="changeLayout('split-v')"
            :class="[layout === 'split-v' ? 'bg-slate-800 text-white' : 'text-slate-500 hover:text-slate-300', 'p-1.5 rounded transition']"
            title="2 Rows (Top & Bottom)"
          >
            <Rows2 class="w-3.5 h-3.5" />
          </button>
          <button
            @click="changeLayout('grid-4')"
            :class="[layout === 'grid-4' ? 'bg-slate-800 text-white' : 'text-slate-500 hover:text-slate-300', 'p-1.5 rounded transition']"
            title="4 Grid (2x2)"
          >
            <Grid2X2 class="w-3.5 h-3.5" />
          </button>
        </div>

        <button
          @click="isHostModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-brand-500 hover:bg-brand-600 text-white shadow-lg shadow-brand-500/20 transition"
        >
          <Plus class="w-4 h-4" />
          Add Server
        </button>
      </div>
    </div>

    <!-- Main Workspace -->
    <div class="flex-1 flex gap-4 min-h-0">
      <!-- Hosts List Sidebar -->
      <div class="w-72 bg-slate-900/60 border border-slate-800/80 rounded-xl p-3 flex flex-col shrink-0">
        <div class="flex items-center justify-between px-2 mb-2">
          <h3 class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Saved Servers</h3>
          <span class="text-[10px] text-slate-500">Target Pane: #{{ activePaneIndex + 1 }}</span>
        </div>

        <div class="flex-1 overflow-y-auto space-y-1.5">
          <div
            v-for="host in hosts"
            :key="host.id"
            class="p-2.5 rounded-lg bg-slate-800/40 hover:bg-slate-800 border border-slate-700/40 transition group"
          >
            <div class="flex items-start justify-between">
              <div>
                <p class="text-xs font-semibold text-white">{{ host.name }}</p>
                <p class="text-[11px] font-mono text-slate-400">{{ host.username }}@{{ host.host }}:{{ host.port }}</p>
              </div>
              <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition">
                <button
                  @click="openSftp(host)"
                  title="Browse SFTP"
                  class="p-1 hover:text-brand-400 transition"
                >
                  <Folder class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="deleteHost(host.id)"
                  title="Delete"
                  class="p-1 hover:text-red-400 transition"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Connect to Active Pane Button -->
            <button
              @click="connectHostToPane(host, activePaneIndex)"
              class="mt-2 w-full flex items-center justify-center gap-1.5 py-1 px-2 text-[11px] font-medium bg-slate-700/60 hover:bg-brand-500 hover:text-white text-slate-300 rounded transition"
            >
              <Server class="w-3 h-3" />
              Connect to Pane #{{ activePaneIndex + 1 }}
            </button>
          </div>

          <div v-if="hosts.length === 0" class="text-center py-8 text-xs text-slate-500">
            No servers configured
          </div>
        </div>
      </div>

      <!-- Split-Screen Terminal Grid -->
      <div
        :class="[
          layout === 'single' ? 'grid-cols-1 grid-rows-1' :
          layout === 'split-h' ? 'grid-cols-2 grid-rows-1' :
          layout === 'split-v' ? 'grid-cols-1 grid-rows-2' :
          'grid-cols-2 grid-rows-2',
          'flex-1 grid gap-2 min-w-0 min-h-0'
        ]"
      >
        <div
          v-for="paneIdx in getVisiblePanesCount()"
          :key="paneIdx - 1"
          @click="activePaneIndex = paneIdx - 1"
          :class="[
            activePaneIndex === paneIdx - 1 ? 'border-brand-500/80 ring-1 ring-brand-500/30' : 'border-slate-800/80',
            'bg-slate-950 border rounded-xl flex flex-col min-h-0 overflow-hidden shadow-2xl transition-all'
          ]"
        >
          <!-- Pane Header Bar -->
          <div class="h-8 bg-slate-900/90 border-b border-slate-800 px-3 flex items-center justify-between shrink-0">
            <div class="flex items-center gap-2 overflow-hidden">
              <span class="w-2 h-2 rounded-full" :class="paneSessions[paneIdx - 1] ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600'"></span>
              <span class="text-xs font-medium text-slate-300 truncate">
                Pane #{{ paneIdx }}: {{ paneSessions[paneIdx - 1]?.title || 'Disconnected (Click server to connect)' }}
              </span>
            </div>

            <div class="flex items-center gap-2">
              <button
                v-if="paneSessions[paneIdx - 1]"
                @click.stop="closeSession(paneIdx - 1)"
                title="Disconnect"
                class="text-slate-500 hover:text-red-400 transition"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Terminal Container for this Pane -->
          <div class="flex-1 relative bg-[#090d16] p-1.5 min-h-0 overflow-hidden">
            <div :id="`terminal-pane-${paneIdx - 1}`" class="w-full h-full"></div>
            
            <!-- Empty Placeholder -->
            <div
              v-if="!paneSessions[paneIdx - 1]"
              class="absolute inset-0 flex flex-col items-center justify-center text-slate-600 text-xs space-y-1"
            >
              <Server class="w-6 h-6 text-slate-700" />
              <span>Select a server from the sidebar to connect Pane #{{ paneIdx }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Add Server Modal -->
    <div v-if="isHostModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="w-full max-w-md bg-slate-900 border border-slate-700 rounded-xl p-6 shadow-2xl space-y-4">
        <h3 class="text-sm font-bold text-white">Add Remote Server</h3>
        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Server Name</label>
            <input v-model="hostForm.name" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none focus:border-brand-500" placeholder="e.g. Production Web 01" />
          </div>
          <div class="grid grid-cols-3 gap-2">
            <div class="col-span-2">
              <label class="block text-slate-400 mb-1">Host / IP Address</label>
              <input v-model="hostForm.host" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none" placeholder="192.168.1.10" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1">SSH Port</label>
              <input v-model.number="hostForm.port" type="number" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none" />
            </div>
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Username</label>
            <input v-model="hostForm.username" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none" placeholder="root" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Authentication Type</label>
            <select v-model="hostForm.authType" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none">
              <option value="password">Password</option>
              <option value="key">SSH Private Key</option>
            </select>
          </div>
          <div v-if="hostForm.authType === 'password'">
            <label class="block text-slate-400 mb-1">Password</label>
            <input v-model="hostForm.password" type="password" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none" />
          </div>
          <div v-if="hostForm.authType === 'key'">
            <label class="block text-slate-400 mb-1">Private Key (PEM format)</label>
            <textarea v-model="hostForm.sshKey" rows="4" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white font-mono text-[11px] focus:outline-none"></textarea>
          </div>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button @click="isHostModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">Cancel</button>
          <button @click="saveHost" class="px-4 py-1.5 text-xs bg-brand-500 hover:bg-brand-600 text-white font-medium rounded">Save Server</button>
        </div>
      </div>
    </div>
  </div>
</template>
