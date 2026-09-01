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
  Key, 
  Lock,
  ExternalLink
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

interface TerminalTab {
  id: string;
  hostId: string;
  title: string;
  term?: Terminal;
  fitAddon?: FitAddon;
  ws?: WebSocket;
}

const hosts = ref<RemoteHost[]>([]);
const tabs = ref<TerminalTab[]>([]);
const activeTabId = ref<string>('');
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

const openTerminal = async (host: RemoteHost) => {
  const tabId = `tab-${Date.now()}`;
  const newTab: TerminalTab = {
    id: tabId,
    hostId: host.id,
    title: `${host.name} (${host.host})`,
  };

  tabs.value.push(newTab);
  activeTabId.value = tabId;

  await nextTick();
  initXterm(newTab, host);
};

const closeTab = (tabId: string) => {
  const tab = tabs.value.find(t => t.id === tabId);
  if (tab) {
    tab.ws?.close();
    tab.term?.dispose();
  }
  tabs.value = tabs.value.filter(t => t.id !== tabId);
  if (activeTabId.value === tabId && tabs.value.length > 0) {
    activeTabId.value = tabs.value[tabs.value.length - 1].id;
  }
};

const initXterm = (tab: TerminalTab, host: RemoteHost) => {
  const container = document.getElementById(`terminal-container-${tab.id}`);
  if (!container) return;

  const term = new Terminal({
    fontFamily: 'JetBrains Mono, monospace',
    fontSize: 13,
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
  fitAddon.fit();

  tab.term = term;
  tab.fitAddon = fitAddon;

  // Connect WebSocket
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/remote-host?cols=${term.cols}&rows=${term.rows}`;
  const ws = new WebSocket(wsUrl);
  tab.ws = ws;

  const token = localStorage.getItem('hephaestus_token') || '';

  ws.onopen = () => {
    // Send auth message
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
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }));
    }
  });

  term.onResize((size) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: size.cols, rows: size.rows }));
    }
  });

  window.addEventListener('resize', () => fitAddon.fit());
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

onMounted(() => {
  fetchHosts();
});

onUnmounted(() => {
  tabs.value.forEach(t => {
    t.ws?.close();
    t.term?.dispose();
  });
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <!-- Header -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">Remote Server Terminal</h2>
        <p class="text-xs text-slate-400">Interactive SSH terminal and SFTP browser</p>
      </div>
      <button
        @click="isHostModalOpen = true"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-brand-500 hover:bg-brand-600 text-white shadow-lg shadow-brand-500/20 transition"
      >
        <Plus class="w-4 h-4" />
        Add Server
      </button>
    </div>

    <!-- Main Workspace -->
    <div class="flex-1 flex gap-4 min-h-0">
      <!-- Hosts List Sidebar -->
      <div class="w-72 bg-slate-900/60 border border-slate-800/80 rounded-xl p-3 flex flex-col shrink-0">
        <h3 class="text-xs font-semibold text-slate-400 uppercase tracking-wider px-2 mb-2">Saved Servers</h3>
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
            <button
              @click="openTerminal(host)"
              class="mt-2 w-full flex items-center justify-center gap-1.5 py-1 px-2 text-[11px] font-medium bg-slate-700/60 hover:bg-brand-500 hover:text-white text-slate-300 rounded transition"
            >
              <Terminal class="w-3 h-3" />
              Connect Terminal
            </button>
          </div>
          <div v-if="hosts.length === 0" class="text-center py-8 text-xs text-slate-500">
            No servers configured
          </div>
        </div>
      </div>

      <!-- Tabbed Terminal Area -->
      <div class="flex-1 bg-slate-950 border border-slate-800/80 rounded-xl flex flex-col min-w-0 overflow-hidden shadow-2xl">
        <!-- Terminal Tabs -->
        <div class="h-10 bg-slate-900 border-b border-slate-800 flex items-center px-2 gap-1 overflow-x-auto shrink-0">
          <div
            v-for="tab in tabs"
            :key="tab.id"
            :class="[
              activeTabId === tab.id ? 'bg-slate-950 text-white border-t-2 border-brand-500' : 'text-slate-400 hover:bg-slate-800/60',
              'flex items-center gap-2 px-3 py-1.5 text-xs rounded-t-lg transition cursor-pointer'
            ]"
            @click="activeTabId = tab.id"
          >
            <span class="truncate max-w-[150px]">{{ tab.title }}</span>
            <button @click.stop="closeTab(tab.id)" class="hover:text-red-400">
              <X class="w-3 h-3" />
            </button>
          </div>
          <div v-if="tabs.length === 0" class="text-xs text-slate-500 px-3 py-1.5">
            Select a server from the left to open a terminal session
          </div>
        </div>

        <!-- Terminal Containers -->
        <div class="flex-1 relative min-h-0 bg-[#090d16] p-2">
          <div
            v-for="tab in tabs"
            :key="tab.id"
            :id="`terminal-container-${tab.id}`"
            :class="[activeTabId === tab.id ? 'block' : 'hidden', 'w-full h-full']"
          ></div>
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
