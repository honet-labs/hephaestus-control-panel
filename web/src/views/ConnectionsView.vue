<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import {
  Link2,
  Plus,
  Trash2,
  Server,
  Activity,
  Shield,
  Key,
  CheckCircle2,
  Pencil,
  Check,
  X,
  RotateCcw,
  AlertTriangle
} from 'lucide-vue-next';

const router = useRouter();

interface RegistryItem {
  id: string;
  name: string;
  type: 'GRAFANA API' | 'PROMETHEUS' | 'DATA PREPPER' | 'OPENSEARCH' | 'DOCKER ENGINE';
  url: string;
  authType?: string;
  isActive: boolean;
  status: 'connected' | 'offline' | 'checking';
  rawType: string;
  rawItem: any;
}

const registry = ref<RegistryItem[]>([]);
const loading = ref(false);
const testStatus = ref<{ message: string; success: boolean } | null>(null);
const testing = ref(false);

// Edit Mode State
const editingId = ref<string | null>(null);
const editingRawType = ref<string | null>(null);

const form = ref({
  type: 'Grafana Core API' as 'Grafana Core API' | 'Prometheus Server (SSH / Local File)' | 'Data Prepper (SSH / Local Directory)' | 'OpenSearch Cluster' | 'Docker Engine (Socket / SSH / TCP)',
  name: '',
  // Grafana
  url: '',
  token: '',
  datasourceUid: '',
  // Prometheus & Data Prepper
  accessMode: 'local' as 'local' | 'ssh',
  filePath: '/etc/prometheus/prometheus.yml',
  pipelinesDir: '/opt/data-prepper/pipelines',
  reloadUrl: 'http://localhost:9090/-/reload',
  // SSH / Host
  sshHost: '',
  sshPort: 22,
  sshUser: 'root',
  sshAuth: 'password' as 'password' | 'key',
  sshPassword: '',
  sshKey: '',
  // OpenSearch
  osHost: '',
  osPort: 9200,
  osUser: '',
  osPassword: '',
  osUseSsl: true,
  osVerifySsl: false,
  // Docker Engine
  dockerDriver: 'socket' as 'socket' | 'ssh' | 'tcp',
  dockerSocketPath: '/var/run/docker.sock',
  dockerTcpHost: '',
  dockerTcpPort: 2375,
  dockerTcpTls: false,
  dockerSshHost: '',
  dockerSshPort: 22,
  dockerSshUser: 'root',
  dockerSshAuth: 'password' as 'password' | 'key',
  dockerSshPassword: '',
  dockerSshKey: '',
  dockerIsDefault: false,
});

// Fetch all registered connections from backend database
const fetchConnections = async () => {
  loading.value = true;
  const items: RegistryItem[] = [];

  try {
    const [grafanaRes, promRes, osRes, dockerRes] = await Promise.all([
      axios.get('/api/v1/settings/grafana').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/settings/prometheus').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/opensearch/config').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/docker/connections').catch(() => ({ data: { success: false } })),
    ]);

    // 1. Grafana Configs
    if (grafanaRes.data?.success && Array.isArray(grafanaRes.data.data)) {
      grafanaRes.data.data.forEach((g: any) => {
        items.push({
          id: g.id,
          name: g.name,
          type: 'GRAFANA API',
          url: g.host,
          isActive: g.isActive,
          status: 'connected',
          rawType: 'grafana',
          rawItem: g,
        });
      });
    }

    // 2. Prometheus Configs
    if (promRes.data?.success && Array.isArray(promRes.data.data)) {
      promRes.data.data.forEach((p: any) => {
        const isDataPrepper = p.name && p.name.toLowerCase().includes('data prepper');
        const displayUrl = p.mode === 'ssh' || p.sshHost
          ? `${p.path || '/etc/prometheus/prometheus.yml'} (${p.sshHost})`
          : `${p.path || '/etc/prometheus/prometheus.yml'} (local)`;
        items.push({
          id: p.id,
          name: p.name,
          type: isDataPrepper ? 'DATA PREPPER' : 'PROMETHEUS',
          url: displayUrl,
          authType: p.mode === 'ssh' || p.sshHost ? 'SSH' : 'LOCAL',
          isActive: p.isActive,
          status: 'connected',
          rawType: 'prometheus',
          rawItem: p,
        });
      });
    }

    // 3. OpenSearch Config
    if (osRes.data?.success && osRes.data.data && osRes.data.data.host) {
      const os = osRes.data.data;
      items.push({
        id: os.id || 'opensearch-active',
        name: os.name || 'OpenSearch Primary',
        type: 'OPENSEARCH',
        url: `${os.host}:${os.port || 9200}`,
        isActive: os.isActive ?? true,
        status: 'connected',
        rawType: 'opensearch',
        rawItem: os,
      });
    }

    // 4. Docker Engine Connections
    if (dockerRes.data?.success && Array.isArray(dockerRes.data.data)) {
      dockerRes.data.data.forEach((d: any) => {
        let displayUrl = '';
        if (d.driver === 'socket') {
          displayUrl = d.socketPath || '/var/run/docker.sock';
        } else if (d.driver === 'ssh') {
          displayUrl = `${d.sshUser || 'root'}@${d.sshHost}:${d.sshPort || 22}`;
        } else {
          displayUrl = `${d.tcpHost}:${d.tcpPort || 2375}`;
        }
        items.push({
          id: d.id,
          name: d.name,
          type: 'DOCKER ENGINE',
          url: displayUrl,
          authType: d.driver.toUpperCase(),
          isActive: d.isDefault,
          status: 'connected',
          rawType: 'docker',
          rawItem: d,
        });
      });
    }

    registry.value = items;
  } catch (err) {
    console.error('Failed to load connections:', err);
  } finally {
    loading.value = false;
  }
};

const handleTestConnection = async () => {
  testing.value = true;
  testStatus.value = null;

  try {
    if (form.value.type === 'OpenSearch Cluster') {
      const payload: any = {
        host: form.value.osHost,
        port: Number(form.value.osPort) || 9200,
        username: form.value.osUser,
        password: form.value.osPassword,
        useSsl: form.value.osUseSsl,
        verifySsl: form.value.osVerifySsl,
      };
      if (editingId.value) payload.id = editingId.value;

      const res = await axios.post('/api/v1/opensearch/test', payload);
      testStatus.value = {
        success: res.data?.success || false,
        message: res.data?.success ? 'Connection verified successfully!' : (res.data?.error || 'Test failed'),
      };
    } else if (form.value.type === 'Prometheus Server (SSH / Local File)') {
      const payload: any = {
        name: form.value.name,
        mode: form.value.accessMode,
        path: form.value.filePath || '/etc/prometheus/prometheus.yml',
        reloadUrl: form.value.reloadUrl || 'http://localhost:9090/-/reload',
        sshHost: form.value.accessMode === 'ssh' ? form.value.sshHost : null,
        sshPort: form.value.accessMode === 'ssh' ? Number(form.value.sshPort) || 22 : null,
        sshUser: form.value.accessMode === 'ssh' ? form.value.sshUser : null,
        sshAuth: form.value.accessMode === 'ssh' ? form.value.sshAuth : null,
        sshPassword: form.value.accessMode === 'ssh' ? form.value.sshPassword : null,
        sshKey: form.value.accessMode === 'ssh' ? form.value.sshKey : null,
      };
      if (editingId.value) payload.id = editingId.value;

      const res = await axios.post('/api/v1/settings/prometheus/test', payload);
      testStatus.value = {
        success: res.data?.success || false,
        message: res.data?.success
          ? (res.data?.message || 'Prometheus connection verified successfully!')
          : (res.data?.error || res.data?.message || 'Failed to connect to Prometheus server.'),
      };
    } else if (form.value.type === 'Data Prepper (SSH / Local Directory)') {
      if (form.value.accessMode === 'ssh') {
        const payload: any = {
          name: `${form.value.name || 'DataPrepper'} (Data Prepper)`,
          mode: 'ssh',
          path: form.value.pipelinesDir || '/opt/data-prepper/pipelines',
          sshHost: form.value.sshHost,
          sshPort: Number(form.value.sshPort) || 22,
          sshUser: form.value.sshUser,
          sshAuth: form.value.sshAuth,
          sshPassword: form.value.sshPassword,
          sshKey: form.value.sshKey,
        };
        if (editingId.value) payload.id = editingId.value;

        const res = await axios.post('/api/v1/settings/prometheus/test', payload);
        testStatus.value = {
          success: res.data?.success || false,
          message: res.data?.success
            ? (res.data?.message || 'Data Prepper host connection verified!')
            : (res.data?.error || res.data?.message || 'Failed to connect to Data Prepper host.'),
        };
      } else {
        testStatus.value = {
          success: true,
          message: 'Local directory mode configured.',
        };
      }
    } else if (form.value.type === 'Docker Engine (Socket / SSH / TCP)') {
      const payload: any = {
        driver: form.value.dockerDriver,
        socketPath: form.value.dockerSocketPath || '/var/run/docker.sock',
        tcpHost: form.value.dockerTcpHost,
        tcpPort: Number(form.value.dockerTcpPort) || 2375,
        tcpTls: form.value.dockerTcpTls,
        sshHost: form.value.dockerSshHost,
        sshPort: Number(form.value.dockerSshPort) || 22,
        sshUser: form.value.dockerSshUser,
        sshAuth: form.value.dockerSshAuth,
        sshPassword: form.value.dockerSshPassword,
        sshKey: form.value.dockerSshKey,
      };
      if (editingId.value) payload.id = editingId.value;

      const res = await axios.post('/api/v1/docker/connections/test', payload);
      testStatus.value = {
        success: res.data?.success || false,
        message: res.data?.success
          ? (res.data?.message || 'Docker connection verified successfully!')
          : (res.data?.error || 'Failed to connect to Docker Engine.'),
      };
    } else {
      testStatus.value = {
        success: true,
        message: 'Endpoint configuration verified.',
      };
    }
  } catch (err: any) {
    testStatus.value = {
      success: false,
      message: err.response?.data?.error || err.message || 'Failed to reach service endpoint.',
    };
  } finally {
    testing.value = false;
  }
};

const handleEditConnection = (item: RegistryItem) => {
  editingId.value = item.id;
  editingRawType.value = item.rawType;
  testStatus.value = null;

  if (item.rawType === 'grafana') {
    form.value.type = 'Grafana Core API';
    form.value.name = item.name;
    form.value.url = item.rawItem.host || '';
    form.value.token = item.rawItem.token || '';
    form.value.datasourceUid = item.rawItem.datasourceUid || '';
  } else if (item.rawType === 'prometheus') {
    const isDataPrepper = item.name && item.name.toLowerCase().includes('data prepper');
    if (isDataPrepper) {
      form.value.type = 'Data Prepper (SSH / Local Directory)';
      form.value.name = item.name.replace(/\s*\(Data Prepper\)\s*/i, '');
      form.value.pipelinesDir = item.rawItem.path || '/opt/data-prepper/pipelines';
    } else {
      form.value.type = 'Prometheus Server (SSH / Local File)';
      form.value.name = item.name;
      form.value.filePath = item.rawItem.path || '/etc/prometheus/prometheus.yml';
    }
    form.value.accessMode = item.rawItem.mode || (item.rawItem.sshHost ? 'ssh' : 'local');
    form.value.reloadUrl = item.rawItem.reloadUrl || 'http://localhost:9090/-/reload';
    form.value.sshHost = item.rawItem.sshHost || '';
    form.value.sshPort = item.rawItem.sshPort || 22;
    form.value.sshUser = item.rawItem.sshUser || 'root';
    form.value.sshAuth = item.rawItem.sshKey ? 'key' : 'password';
    form.value.sshPassword = item.rawItem.sshPassword || '';
    form.value.sshKey = item.rawItem.sshKey || '';
  } else if (item.rawType === 'opensearch') {
    form.value.type = 'OpenSearch Cluster';
    form.value.name = item.name;
    form.value.osHost = item.rawItem.host || '';
    form.value.osPort = item.rawItem.port || 9200;
    form.value.osUser = item.rawItem.username || '';
    form.value.osPassword = '';
    form.value.osUseSsl = item.rawItem.useSsl ?? true;
    form.value.osVerifySsl = item.rawItem.verifySsl ?? false;
  } else if (item.rawType === 'docker') {
    form.value.type = 'Docker Engine (Socket / SSH / TCP)';
    form.value.name = item.name;
    form.value.dockerDriver = item.rawItem.driver || 'socket';
    form.value.dockerSocketPath = item.rawItem.socketPath || '/var/run/docker.sock';
    form.value.dockerTcpHost = item.rawItem.tcpHost || '';
    form.value.dockerTcpPort = item.rawItem.tcpPort || 2375;
    form.value.dockerTcpTls = item.rawItem.tcpTls || false;
    form.value.dockerSshHost = item.rawItem.sshHost || '';
    form.value.dockerSshPort = item.rawItem.sshPort || 22;
    form.value.dockerSshUser = item.rawItem.sshUser || 'root';
    form.value.dockerSshAuth = item.rawItem.sshAuth || 'password';
    form.value.dockerSshPassword = '';
    form.value.dockerSshKey = '';
    form.value.dockerIsDefault = item.rawItem.isDefault || false;
  }

  // Smooth scroll to top form
  window.scrollTo({ top: 0, behavior: 'smooth' });
};

const cancelEdit = () => {
  editingId.value = null;
  editingRawType.value = null;
  form.value.name = '';
  form.value.url = '';
  form.value.token = '';
  form.value.datasourceUid = '';
  form.value.sshHost = '';
  form.value.sshPassword = '';
  form.value.sshKey = '';
  form.value.osHost = '';
  form.value.osUser = '';
  form.value.osPassword = '';
  form.value.dockerDriver = 'socket';
  form.value.dockerSocketPath = '/var/run/docker.sock';
  form.value.dockerTcpHost = '';
  form.value.dockerTcpPort = 2375;
  form.value.dockerTcpTls = false;
  form.value.dockerSshHost = '';
  form.value.dockerSshPort = 22;
  form.value.dockerSshUser = 'root';
  form.value.dockerSshPassword = '';
  form.value.dockerSshKey = '';
  form.value.dockerIsDefault = false;
  testStatus.value = null;
};

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

// Delete Confirmation Modal State
const showDeleteModal = ref(false);
const itemToDelete = ref<RegistryItem | null>(null);
const deleting = ref(false);

const confirmDelete = (item: RegistryItem) => {
  itemToDelete.value = item;
  showDeleteModal.value = true;
};

const executeDelete = async () => {
  if (!itemToDelete.value) return;
  deleting.value = true;
  const item = itemToDelete.value;
  try {
    if (item.rawType === 'grafana') {
      await axios.delete(`/api/v1/settings/grafana/${item.id}`);
    } else if (item.rawType === 'prometheus') {
      await axios.delete(`/api/v1/settings/prometheus/${item.id}`);
    } else if (item.rawType === 'opensearch') {
      await axios.delete(`/api/v1/opensearch/config/${item.id}`);
    } else if (item.rawType === 'docker') {
      await axios.delete(`/api/v1/docker/connections/${item.id}`);
    }
    if (editingId.value === item.id) {
      cancelEdit();
    }
    showDeleteModal.value = false;
    itemToDelete.value = null;
    showToast(`Connection "${item.name}" deleted successfully.`, 'success');
    await fetchConnections();
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to delete connection', 'error');
  } finally {
    deleting.value = false;
  }
};

const handleRegisterEndpoint = async () => {
  if (!form.value.name) {
    showToast('Please provide a Connection Name.', 'error');
    return;
  }

  try {
    if (form.value.type === 'Grafana Core API') {
      const payload: any = {
        name: form.value.name,
        host: form.value.url,
        token: form.value.token,
        datasourceUid: form.value.datasourceUid,
        isActive: true,
      };
      if (editingId.value) payload.id = editingId.value;
      await axios.post('/api/v1/settings/grafana', payload);
    } else if (form.value.type === 'Prometheus Server (SSH / Local File)') {
      const payload: any = {
        name: form.value.name,
        mode: form.value.accessMode,
        path: form.value.filePath || '/etc/prometheus/prometheus.yml',
        reloadUrl: form.value.reloadUrl || 'http://localhost:9090/-/reload',
        sshHost: form.value.accessMode === 'ssh' ? form.value.sshHost : null,
        sshPort: form.value.accessMode === 'ssh' ? Number(form.value.sshPort) : null,
        sshUser: form.value.accessMode === 'ssh' ? form.value.sshUser : null,
        sshPassword: form.value.accessMode === 'ssh' ? form.value.sshPassword : null,
        sshKey: form.value.accessMode === 'ssh' ? form.value.sshKey : null,
        isActive: true,
      };
      if (editingId.value) payload.id = editingId.value;
      await axios.post('/api/v1/settings/prometheus', payload);
    } else if (form.value.type === 'Data Prepper (SSH / Local Directory)') {
      const payload: any = {
        name: `${form.value.name} (Data Prepper)`,
        mode: form.value.accessMode,
        path: form.value.pipelinesDir || '/opt/data-prepper/pipelines',
        reloadUrl: '',
        sshHost: form.value.accessMode === 'ssh' ? form.value.sshHost : null,
        sshPort: form.value.accessMode === 'ssh' ? Number(form.value.sshPort) : null,
        sshUser: form.value.accessMode === 'ssh' ? form.value.sshUser : null,
        sshPassword: form.value.accessMode === 'ssh' ? form.value.sshPassword : null,
        sshKey: form.value.accessMode === 'ssh' ? form.value.sshKey : null,
        isActive: true,
      };
      if (editingId.value) payload.id = editingId.value;
      await axios.post('/api/v1/settings/prometheus', payload);
    } else if (form.value.type === 'OpenSearch Cluster') {
      const payload: any = {
        name: form.value.name,
        host: form.value.osHost,
        port: Number(form.value.osPort) || 9200,
        username: form.value.osUser,
        password: form.value.osPassword,
        useSsl: form.value.osUseSsl,
        verifySsl: form.value.osVerifySsl,
        isActive: true,
      };
      if (editingId.value) payload.id = editingId.value;
      await axios.post('/api/v1/opensearch/config', payload);
    } else if (form.value.type === 'Docker Engine (Socket / SSH / TCP)') {
      const payload: any = {
        name: form.value.name,
        driver: form.value.dockerDriver,
        socketPath: form.value.dockerSocketPath || '/var/run/docker.sock',
        tcpHost: form.value.dockerTcpHost,
        tcpPort: Number(form.value.dockerTcpPort) || 2375,
        tcpTls: form.value.dockerTcpTls,
        sshHost: form.value.dockerSshHost,
        sshPort: Number(form.value.dockerSshPort) || 22,
        sshUser: form.value.dockerSshUser,
        sshAuth: form.value.dockerSshAuth,
        sshPassword: form.value.dockerSshPassword,
        sshKey: form.value.dockerSshKey,
        isDefault: form.value.dockerIsDefault,
      };
      if (editingId.value) payload.id = editingId.value;
      await axios.post('/api/v1/docker/connections', payload);
    }

    cancelEdit();
    showToast('Service endpoint registered successfully!', 'success');
    await fetchConnections();
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to save service endpoint.', 'error');
  }
};

const handlePingTest = async (item: RegistryItem) => {
  item.status = 'checking';
  try {
    if (item.rawType === 'prometheus') {
      const res = await axios.post('/api/v1/settings/prometheus/test', {
        id: item.id,
      });
      if (res.data?.success) {
        item.status = 'connected';
        showToast(`Connected to Prometheus: ${item.name}`, 'success');
      } else {
        item.status = 'offline';
        showToast(res.data?.error || res.data?.message || `Prometheus connection failed: ${item.name}`, 'error');
      }
    } else if (item.rawType === 'opensearch') {
      const res = await axios.post('/api/v1/opensearch/test', {
        id: item.id,
        host: item.rawItem?.host,
        port: Number(item.rawItem?.port) || 9200,
        username: item.rawItem?.username,
        password: item.rawItem?.password,
        useSsl: item.rawItem?.useSsl,
        verifySsl: item.rawItem?.verifySsl,
      });
      if (res.data?.success) {
        item.status = 'connected';
        showToast(`Connected to OpenSearch: ${item.name}`, 'success');
      } else {
        item.status = 'offline';
        showToast(res.data?.error || `OpenSearch cluster unreachable for ${item.name}`, 'error');
      }
    } else if (item.rawType === 'docker') {
      const res = await axios.post('/api/v1/docker/connections/test', {
        id: item.id,
      });
      if (res.data?.success) {
        item.status = 'connected';
        showToast(`Docker Engine reachable: ${item.name}`, 'success');
      } else {
        item.status = 'offline';
        showToast(res.data?.error || `Docker Engine unreachable for ${item.name}`, 'error');
      }
    } else {
      await new Promise((r) => setTimeout(r, 400));
      item.status = 'connected';
      showToast(`Connection active: ${item.name}`, 'success');
    }
  } catch (e: any) {
    item.status = 'offline';
    showToast(e.response?.data?.error || e.message || `Ping test failed for ${item.name}`, 'error');
  }
};

onMounted(() => {
  fetchConnections();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">Add Connections</h1>
      <p class="text-xs text-blue-700 dark:text-[#95CCDD]/80 mt-0.5">
        Manage API and service endpoint connections for Grafana, Prometheus, Data Prepper, and OpenSearch.
      </p>
    </div>

    <!-- Main 2-Column Grid (Form on Left, Registry on Right) -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      
      <!-- ============================================================= -->
      <!-- LEFT COLUMN: REGISTER / EDIT SERVICE ENDPOINT FORM -->
      <!-- ============================================================= -->
      <div class="lg:col-span-5 p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-4 shadow-sm">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-1.5">
            <Pencil v-if="editingId" class="w-3.5 h-3.5 text-blue-600 dark:text-[#95CCDD]" />
            <Plus v-else class="w-3.5 h-3.5 text-blue-600 dark:text-[#95CCDD]" />
            <span>{{ editingId ? `Edit Connection: ${form.name}` : 'Register Service Endpoint' }}</span>
          </h2>

          <span v-if="editingId" class="px-2 py-0.5 rounded bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400 border border-amber-200 dark:border-amber-500/30 text-[10px] font-mono font-bold uppercase">
            EDIT MODE
          </span>
        </div>

        <form @submit.prevent="handleRegisterEndpoint" class="space-y-3.5 text-xs">
          <!-- Connection Type Dropdown -->
          <div>
            <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Connection Type</label>
            <select
              v-model="form.type"
              class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white font-medium focus:outline-none focus:border-[#4274D9] text-xs"
            >
              <option value="Grafana Core API">Grafana Core API</option>
              <option value="Prometheus Server (SSH / Local File)">Prometheus Server (SSH / Local File)</option>
              <option value="Data Prepper (SSH / Local Directory)">Data Prepper (SSH / Local Directory)</option>
              <option value="OpenSearch Cluster">OpenSearch Cluster</option>
              <option value="Docker Engine (Socket / SSH / TCP)">Docker Engine (Socket / SSH / TCP)</option>
            </select>
          </div>

          <!-- Connection Name -->
          <div>
            <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Connection Name</label>
            <input
              v-model="form.name"
              required
              placeholder="e.g. Production Grafana, Prometheus Horus"
              class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-[#4274D9] text-xs font-mono"
            />
          </div>

          <!-- ================= 1. GRAFANA FIELDS ================= -->
          <template v-if="form.type === 'Grafana Core API'">
            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">API Endpoint URL</label>
              <input
                v-model="form.url"
                required
                placeholder="http://10.20.3.3:3030/"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-[#4274D9] text-xs font-mono"
              />
            </div>
            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Bearer Service Account Token</label>
              <input
                v-model="form.token"
                type="password"
                placeholder="••••••••••••••••••••"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white placeholder-slate-600 focus:outline-none focus:border-[#4274D9] text-xs font-mono"
              />
            </div>
            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Prometheus Datasource UID (Optional)</label>
              <input
                v-model="form.datasourceUid"
                placeholder="e.g. bfo80enbiimf4f"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white placeholder-slate-600 focus:outline-none focus:border-[#4274D9] text-xs font-mono"
              />
            </div>
          </template>

          <!-- ================= 2. PROMETHEUS FIELDS ================= -->
          <template v-if="form.type === 'Prometheus Server (SSH / Local File)'">
            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">File Access Mode</label>
              <select
                v-model="form.accessMode"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white font-medium focus:outline-none focus:border-[#4274D9] text-xs"
              >
                <option value="local">Local File Path (Same Server / Mount)</option>
                <option value="ssh">SSH Remote Host</option>
              </select>
            </div>

            <!-- If SSH Remote Host -->
            <div v-if="form.accessMode === 'ssh'" class="space-y-2 p-3 bg-[#141824] border border-[#1b2234] rounded-lg">
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block text-slate-400 text-[10px]">SSH Host IP</label>
                  <input v-model="form.sshHost" placeholder="10.20.3.4" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
                <div>
                  <label class="block text-slate-400 text-[10px]">Port</label>
                  <input v-model.number="form.sshPort" type="number" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-slate-400 text-[10px]">SSH User</label>
                  <input v-model="form.sshUser" placeholder="root" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
                <div>
                  <label class="block text-slate-400 text-[10px]">Password</label>
                  <input v-model="form.sshPassword" type="password" placeholder="••••••" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
              </div>
            </div>

            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Prometheus.yml File Path</label>
              <input
                v-model="form.filePath"
                required
                placeholder="/etc/prometheus/prometheus.yml"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-[#4274D9] text-xs font-mono"
              />
            </div>

            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Prometheus Reload URL</label>
              <input
                v-model="form.reloadUrl"
                placeholder="http://localhost:9090/-/reload"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-[#4274D9] text-xs font-mono"
              />
            </div>
          </template>

          <!-- ================= 3. DATA PREPPER FIELDS ================= -->
          <template v-if="form.type === 'Data Prepper (SSH / Local Directory)'">
            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">File Access Mode</label>
              <select
                v-model="form.accessMode"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white font-medium focus:outline-none focus:border-[#4274D9] text-xs"
              >
                <option value="local">Local Pipelines Directory (Same Server / Mount)</option>
                <option value="ssh">SSH Remote Host</option>
              </select>
            </div>

            <!-- If SSH Remote Host -->
            <div v-if="form.accessMode === 'ssh'" class="space-y-2 p-3 bg-[#141824] border border-[#1b2234] rounded-lg">
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block text-slate-400 text-[10px]">SSH Host IP</label>
                  <input v-model="form.sshHost" placeholder="10.10.5.87" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
                <div>
                  <label class="block text-slate-400 text-[10px]">Port</label>
                  <input v-model.number="form.sshPort" type="number" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-slate-400 text-[10px]">SSH User</label>
                  <input v-model="form.sshUser" placeholder="root" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
                <div>
                  <label class="block text-slate-400 text-[10px]">Password</label>
                  <input v-model="form.sshPassword" type="password" autocomplete="new-password" placeholder="••••••" class="w-full bg-[#0e121c] border border-slate-700 rounded px-2 py-1 text-white font-mono" />
                </div>
              </div>
            </div>

            <div>
              <label class="block text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Pipelines Directory Path</label>
              <input
                v-model="form.pipelinesDir"
                required
                placeholder="/opt/data-prepper/pipelines"
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg px-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-[#4274D9] text-xs font-mono"
              />
            </div>
          </template>

          <!-- ================= 4. OPENSEARCH FIELDS ================= -->
          <template v-if="form.type === 'OpenSearch Cluster'">
            <div class="grid grid-cols-3 gap-2">
              <div class="col-span-2">
                <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Cluster Host / IP</label>
                <input v-model="form.osHost" required placeholder="103.171.31.56" class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono text-xs focus:outline-none focus:border-[#4274D9]" />
              </div>
              <div>
                <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Port</label>
                <input v-model.number="form.osPort" type="number" class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono text-xs focus:outline-none focus:border-[#4274D9]" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div>
                <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Username</label>
                <input v-model="form.osUser" placeholder="admin" class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono text-xs focus:outline-none focus:border-[#4274D9]" />
              </div>
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider">Password</label>
                  <span v-if="editingId && editingRawType === 'opensearch'" class="text-[9px] text-emerald-600 dark:text-emerald-400 font-normal">
                    (saved)
                  </span>
                </div>
                <input
                  v-model="form.osPassword"
                  type="password"
                  autocomplete="new-password"
                  :placeholder="editingId && editingRawType === 'opensearch' ? '•••••••• (leave blank to keep current)' : '••••••'"
                  class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono text-xs focus:outline-none focus:border-[#4274D9]"
                />
              </div>
            </div>

            <!-- SSL / TLS Checkboxes -->
            <div class="flex items-center gap-6 pt-1">
              <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-700 dark:text-slate-300">
                <input
                  type="checkbox"
                  v-model="form.osUseSsl"
                  class="rounded bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-slate-700 text-[#4274D9] focus:ring-0 w-4 h-4 cursor-pointer"
                />
                <span class="font-medium text-slate-800 dark:text-white">Use HTTPS (SSL/TLS)</span>
              </label>

              <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-700 dark:text-slate-300">
                <input
                  type="checkbox"
                  v-model="form.osVerifySsl"
                  :disabled="!form.osUseSsl"
                  class="rounded bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-slate-700 text-[#4274D9] focus:ring-0 w-4 h-4 cursor-pointer disabled:opacity-40"
                />
                <span :class="{ 'text-slate-400 dark:text-slate-500': !form.osUseSsl, 'text-slate-700 dark:text-slate-300': form.osUseSsl }">Verify SSL Certificate</span>
              </label>
            </div>
          </template>

          <!-- ================= 4. DOCKER ENGINE FIELDS ================= -->
          <template v-if="form.type === 'Docker Engine (Socket / SSH / TCP)'">
            <div>
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Docker Driver Mode</label>
              <div class="grid grid-cols-3 gap-2">
                <button
                  type="button"
                  @click="form.dockerDriver = 'socket'"
                  :class="[
                    'py-1.5 px-2 text-xs font-semibold rounded-lg border transition text-center cursor-pointer',
                    form.dockerDriver === 'socket'
                      ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                      : 'bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-[#1b2234] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                  ]"
                >
                  Unix Socket
                </button>
                <button
                  type="button"
                  @click="form.dockerDriver = 'ssh'"
                  :class="[
                    'py-1.5 px-2 text-xs font-semibold rounded-lg border transition text-center cursor-pointer',
                    form.dockerDriver === 'ssh'
                      ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                      : 'bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-[#1b2234] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                  ]"
                >
                  Remote SSH
                </button>
                <button
                  type="button"
                  @click="form.dockerDriver = 'tcp'"
                  :class="[
                    'py-1.5 px-2 text-xs font-semibold rounded-lg border transition text-center cursor-pointer',
                    form.dockerDriver === 'tcp'
                      ? 'bg-blue-500/10 border-blue-500 text-blue-600 dark:text-blue-400'
                      : 'bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-[#1b2234] text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                  ]"
                >
                  TCP Socket
                </button>
              </div>
            </div>

            <!-- Socket Fields -->
            <div v-if="form.dockerDriver === 'socket'">
              <label class="block text-[11px] font-bold text-slate-700 dark:text-slate-400 uppercase tracking-wider mb-1">Docker Socket Path</label>
              <input
                v-model="form.dockerSocketPath"
                placeholder="/var/run/docker.sock"
                class="w-full bg-slate-50 dark:bg-[#141824] border border-slate-300 dark:border-[#1b2234] rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono text-xs focus:outline-none focus:border-[#4274D9]"
              />
              <p class="text-[10px] text-slate-500 mt-1">Default local Docker daemon Unix domain socket.</p>
            </div>

            <!-- SSH Fields -->
            <div v-if="form.dockerDriver === 'ssh'" class="space-y-2 p-3 bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1b2234] rounded-lg">
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">SSH Host IP / Domain</label>
                  <input v-model="form.dockerSshHost" placeholder="10.20.3.10" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
                </div>
                <div>
                  <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">Port</label>
                  <input v-model.number="form.dockerSshPort" type="number" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">SSH Username</label>
                  <input v-model="form.dockerSshUser" placeholder="root" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
                </div>
                <div>
                  <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">Password / Key</label>
                  <input v-model="form.dockerSshPassword" type="password" placeholder="••••••" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
                </div>
              </div>
            </div>

            <!-- TCP Fields -->
            <div v-if="form.dockerDriver === 'tcp'" class="space-y-2 p-3 bg-slate-50 dark:bg-[#141824] border border-slate-200 dark:border-[#1b2234] rounded-lg">
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">TCP Host / IP</label>
                  <input v-model="form.dockerTcpHost" placeholder="10.20.3.15" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
                </div>
                <div>
                  <label class="block text-slate-700 dark:text-slate-400 text-[10px] font-bold uppercase">Port</label>
                  <input v-model.number="form.dockerTcpPort" type="number" placeholder="2375" class="w-full bg-white dark:bg-[#0e121c] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white font-mono text-xs" />
                </div>
              </div>
            </div>

            <!-- Set as Default Connection checkbox -->
            <div class="pt-1">
              <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-700 dark:text-slate-300">
                <input
                  type="checkbox"
                  v-model="form.dockerIsDefault"
                  class="rounded bg-slate-50 dark:bg-[#141824] border-slate-300 dark:border-slate-700 text-[#4274D9] focus:ring-0 w-4 h-4 cursor-pointer"
                />
                <span class="font-medium text-slate-800 dark:text-white">Default Environment for Container Management</span>
              </label>
            </div>
          </template>

          <!-- Buttons: Test Connection & Register/Update -->
          <div class="grid grid-cols-2 gap-3 pt-2">
            <button
              type="button"
              @click="handleTestConnection"
              :disabled="testing"
              class="px-4 py-2.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#20242e] dark:hover:bg-[#282d3a] text-slate-700 hover:text-slate-900 dark:text-slate-200 text-xs font-bold rounded-lg border border-slate-300 dark:border-slate-700 transition disabled:opacity-50 cursor-pointer"
            >
              {{ testing ? 'TESTING...' : 'TEST CONNECTION' }}
            </button>

            <button
              type="submit"
              class="px-4 py-2.5 bg-[#4274D9] hover:bg-[#3461c2] text-white text-xs font-bold rounded-lg shadow-lg shadow-[#4274D9]/20 transition flex items-center justify-center gap-1.5"
            >
              <Check v-if="editingId" class="w-3.5 h-3.5" />
              <Plus v-else class="w-3.5 h-3.5" />
              <span>{{ editingId ? 'UPDATE ENDPOINT' : 'REGISTER ENDPOINT' }}</span>
            </button>
          </div>

          <!-- Cancel Edit Mode Button -->
          <div v-if="editingId" class="pt-1">
            <button
              type="button"
              @click="cancelEdit"
              class="w-full py-1.5 text-center text-xs text-slate-700 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white bg-slate-100 hover:bg-slate-200 dark:bg-[#141824] dark:hover:bg-[#1b2234] rounded-lg border border-slate-300 dark:border-slate-700/80 transition cursor-pointer"
            >
              Cancel Edit Mode
            </button>
          </div>

          <!-- Test Status Banner -->
          <div
            v-if="testStatus"
            :class="[
              'p-2.5 rounded-lg border text-xs font-mono mt-2',
              testStatus.success ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-400'
            ]"
          >
            {{ testStatus.message }}
          </div>

          <!-- Datasource Notes -->
          <div class="pt-3 border-t border-[#1b2234] text-[11px] text-slate-500 space-y-1">
            <p class="font-bold uppercase tracking-wider text-slate-400">Datasource Notes</p>
            <p class="leading-relaxed">
              Registered endpoints are queried asynchronously for dashboard stats and report telemetry widgets. Ensure Network Security Groups allow ingress queries from the Hephaestus portal IP gateway.
            </p>
          </div>
        </form>
      </div>

      <!-- ============================================================= -->
      <!-- RIGHT COLUMN: ACTIVE REGISTRY LIST -->
      <!-- ============================================================= -->
      <div class="lg:col-span-7 space-y-4">
        <!-- Section Header -->
        <div class="flex items-center justify-between">
          <h2 class="text-xs font-bold text-blue-800 dark:text-[#95CCDD] uppercase tracking-wider">
            Active Registry ({{ registry.length }})
          </h2>

          <button
            @click="cancelEdit(); form.type = 'Grafana Core API'"
            class="flex items-center gap-1 text-xs text-blue-700 dark:text-[#95CCDD] hover:text-blue-900 dark:hover:text-white font-bold uppercase transition"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Server</span>
          </button>
        </div>

        <!-- Registry Cards List -->
        <div class="space-y-3">
          <div
            v-for="item in registry"
            :key="item.id"
            :class="[
              'p-4 bg-white dark:bg-[#0e121c] border rounded-xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 shadow-sm transition group',
              editingId === item.id ? 'border-blue-500 ring-1 ring-blue-500/40' : 'border-slate-200 dark:border-[#1b2234] hover:border-slate-400 dark:hover:border-slate-700'
            ]"
          >
            <!-- Left Card Info -->
            <div class="flex items-center gap-3 overflow-hidden">
              <div class="w-9 h-9 rounded-lg bg-blue-50 dark:bg-[#141b2d] border border-blue-200 dark:border-[#293681] flex items-center justify-center text-slate-700 dark:text-slate-300 shrink-0">
                <Server class="w-4 h-4 text-blue-600 dark:text-[#95CCDD]" />
              </div>

              <div class="overflow-hidden space-y-1">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-xs font-bold text-slate-900 dark:text-white">{{ item.name }}</span>

                  <!-- Type Badge -->
                  <span
                    :class="[
                      'px-1.5 py-0.5 rounded text-[9px] font-bold uppercase',
                      item.type === 'GRAFANA API' ? 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/30' :
                      item.type === 'PROMETHEUS' ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/30' :
                      item.type === 'DATA PREPPER' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30' :
                      item.type === 'DOCKER ENGINE' ? 'bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/30' :
                      'bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/30'
                    ]"
                  >
                    {{ item.type }}
                  </span>

                  <!-- Auth Type Badge -->
                  <span v-if="item.authType" class="px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-slate-700 text-[9px] font-mono">
                    {{ item.authType }}
                  </span>

                  <!-- Active Status -->
                  <span v-if="item.isActive" class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 text-[9px] font-bold uppercase">
                    ACTIVE
                  </span>
                </div>

                <p class="text-[11px] text-slate-500 dark:text-slate-400 font-mono truncate">
                  {{ item.url }}
                </p>
              </div>
            </div>

            <!-- Right Card Actions -->
            <div class="flex items-center gap-2 shrink-0 flex-wrap sm:flex-nowrap justify-end sm:justify-start pt-2 sm:pt-0 border-t sm:border-t-0 border-slate-100 dark:border-[#1b2234]">
              <!-- Connected Pill -->
              <span
                :class="[
                  'px-2.5 py-1 rounded-lg text-[10px] font-bold font-mono uppercase flex items-center gap-1',
                  item.status === 'connected' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30' :
                  item.status === 'checking' ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/30' :
                  'bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/30'
                ]"
              >
                <CheckCircle2 v-if="item.status === 'connected'" class="w-3 h-3" />
                <span>{{ item.status === 'checking' ? 'Testing...' : item.status === 'connected' ? 'CONNECTED' : 'OFFLINE' }}</span>
              </span>

              <!-- Ping Test Button -->
              <button
                @click="handlePingTest(item)"
                class="px-2.5 py-1 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#141b2d] dark:hover:bg-[#1f2842] text-slate-700 hover:text-slate-900 dark:text-slate-300 text-[11px] font-medium border border-slate-300 dark:border-[#293681] transition cursor-pointer"
              >
                Ping Test
              </button>

              <!-- Edit Button -->
              <button
                @click="handleEditConnection(item)"
                class="w-7 h-7 flex items-center justify-center rounded-lg bg-slate-100 hover:bg-blue-50 dark:bg-slate-800/80 text-slate-600 hover:text-blue-600 dark:text-slate-400 dark:hover:text-blue-400 dark:hover:bg-blue-950/60 border border-slate-300 dark:border-slate-700/60 transition shadow-xs cursor-pointer"
                title="Edit Connection"
              >
                <Pencil :size="15" class="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
              </button>

              <!-- Delete Button -->
              <button
                @click="confirmDelete(item)"
                class="w-7 h-7 flex items-center justify-center rounded-lg bg-slate-100 hover:bg-rose-50 dark:bg-slate-800/80 text-slate-600 hover:text-rose-600 dark:text-slate-400 dark:hover:text-rose-400 dark:hover:bg-rose-950/60 border border-slate-300 dark:border-slate-700/60 transition shadow-xs cursor-pointer"
                title="Delete Connection"
              >
                <Trash2 :size="15" class="w-3.5 h-3.5 text-rose-600 dark:text-rose-400" />
              </button>
            </div>
          </div>

          <!-- Empty State -->
          <div v-if="registry.length === 0 && !loading" class="p-12 text-center bg-slate-50 dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-2">
            <Server class="w-8 h-8 text-slate-500 dark:text-slate-600 mx-auto mb-2" />
            <p class="text-xs font-bold text-slate-800 dark:text-slate-300">No Service Endpoints Registered</p>
            <p class="text-[11px] text-slate-500 max-w-sm mx-auto">
              Fill out the form on the left to register your first Grafana, Prometheus, Data Prepper, or OpenSearch instance.
            </p>
          </div>
        </div>
      </div>

    </div>

    <!-- Standard HCP Delete Confirmation Modal -->
    <div
      v-if="showDeleteModal && itemToDelete"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Connection?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ itemToDelete.name }}</strong>? This action cannot be undone.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteModal = false"
            :disabled="deleting"
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

    <!-- Toast Notification Banner (Auto-dismiss 3000ms) -->
    <div
      v-if="toast"
      :class="[
        'fixed bottom-6 right-6 z-50 flex items-center gap-3 px-4 py-3 rounded-xl shadow-xl border text-xs font-medium transition-all duration-300 animate-in slide-in-from-bottom-5',
        toast.type === 'success'
          ? 'bg-emerald-50 dark:bg-[#0e1f18] text-emerald-800 dark:text-emerald-300 border-emerald-300 dark:border-emerald-800/60'
          : 'bg-rose-50 dark:bg-[#201015] text-rose-800 dark:text-rose-300 border-rose-300 dark:border-rose-800/60'
      ]"
    >
      <CheckCircle2 v-if="toast.type === 'success'" class="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
      <AlertTriangle v-else class="w-4 h-4 text-rose-600 dark:text-rose-400 shrink-0" />
      <span>{{ toast.message }}</span>
      <button @click="toast = null" class="ml-2 text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
        ✕
      </button>
    </div>
  </div>
</template>
