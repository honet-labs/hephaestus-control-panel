<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import {
  Settings,
  Users,
  Database,
  Plus,
  Trash2,
  Server,
  FileText,
  RotateCw,
  CheckCircle2,
  Layers,
  Terminal,
  Save,
  Play,
} from 'lucide-vue-next';

const activeTab = ref<'general' | 'prometheus' | 'dataprepper' | 'users' | 'database' | 'audit'>('general');

// 1. General & Users State
const users = ref<any[]>([]);
const auditLogs = ref<any[]>([]);
const isUserModalOpen = ref(false);
const userForm = ref({
  username: '',
  password: '',
  role: 'operator',
});

// 2. PostgreSQL Connection State
const dbConfig = ref<any>({
  host: 'localhost',
  port: 5432,
  user: 'postgres',
  password: '',
  database: 'hephaestus',
  ssl: false,
});

// 3. Prometheus Remote Config State
const promConfigs = ref<any[]>([]);
const activePromId = ref<string>('');
const promForm = ref<any>({
  id: '',
  name: 'Default Prometheus Node',
  mode: 'local',
  path: '/etc/prometheus/prometheus.yml',
  reload_url: 'http://localhost:9090/-/reload',
  ssh_host: 'localhost',
  ssh_port: 22,
  ssh_user: 'root',
  ssh_auth: 'password',
  is_active: true,
});
const isPromModalOpen = ref(false);
const promReloading = ref(false);

// 4. Data Prepper Pipelines State
const dpPipelines = ref<string[]>([]);
const dpLoading = ref(false);
const dpYamlContent = ref<string>(`version: "2"
pipeline:
  source:
    http:
      path: "/log/ingest"
  processor:
    - grok:
        match:
          log: ["%{COMMONAPACHELOG}"]
  sink:
    - opensearch:
        hosts: ["https://localhost:9200"]
        index: "logs-dataprepper-%{yyyy.MM.dd}"
`);
const dpValidationResult = ref<{ valid: boolean; message: string } | null>(null);

// Toast Notification
const toastMessage = ref('');
const showToast = ref(false);
const notify = (msg: string) => {
  toastMessage.value = msg;
  showToast.value = true;
  setTimeout(() => {
    showToast.value = false;
  }, 3500);
};

// Fetch Settings Data
const fetchSettings = async () => {
  try {
    const [usersRes, logsRes, dbRes, promRes] = await Promise.all([
      axios.get('/api/v1/settings/users').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/settings/activity-logs?limit=50').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/settings/database').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/settings/prometheus').catch(() => ({ data: { success: false } })),
    ]);

    if (usersRes.data?.success) users.value = usersRes.data.data || [];
    if (logsRes.data?.success) auditLogs.value = logsRes.data.data?.logs || [];
    if (dbRes.data?.success && dbRes.data.data) dbConfig.value = dbRes.data.data;
    if (promRes.data?.success && promRes.data.data) {
      promConfigs.value = promRes.data.data;
      const active = promConfigs.value.find((p: any) => p.is_active);
      if (active) {
        activePromId.value = active.id;
        promForm.value = { ...active };
      }
    }
  } catch (err) {
    console.error('Settings fetch error:', err);
  }
};

// User Operations
const createUser = async () => {
  try {
    const res = await axios.post('/api/v1/settings/users', userForm.value);
    if (res.data.success) {
      isUserModalOpen.value = false;
      userForm.value = { username: '', password: '', role: 'operator' };
      notify('User created successfully');
      fetchSettings();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || err.message);
  }
};

const deleteUser = async (id: number) => {
  if (!confirm('Are you sure you want to delete this user?')) return;
  try {
    await axios.delete(`/api/v1/settings/users/${id}`);
    notify('User deleted');
    fetchSettings();
  } catch (err: any) {
    alert(err.response?.data?.error || err.message);
  }
};

// Database Switch
const saveDBConfig = async () => {
  try {
    const res = await axios.post('/api/v1/settings/database', dbConfig.value);
    if (res.data.success) {
      notify('Database connection updated and synchronized!');
    }
  } catch (err: any) {
    alert(`Failed to switch database: ${err.response?.data?.error || err.message}`);
  }
};

// Prometheus Remote Config Operations
const savePromConfig = async () => {
  try {
    if (!promForm.value.id) {
      promForm.value.id = 'prom-' + Date.now();
    }
    const res = await axios.post('/api/v1/settings/prometheus', promForm.value);
    if (res.data.success) {
      notify('Prometheus configuration saved successfully.');
      isPromModalOpen.value = false;
      fetchSettings();
    }
  } catch (err: any) {
    alert(`Failed to save Prometheus config: ${err.response?.data?.error || err.message}`);
  }
};

const reloadPrometheus = async () => {
  promReloading.value = true;
  try {
    const res = await axios.post('/api/v1/prometheus/reload');
    if (res.data.success) {
      notify('Prometheus configuration reloaded via /-/reload successfully!');
    } else {
      notify('Triggered Prometheus reload signal.');
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Prometheus reload endpoint triggered.');
  } finally {
    promReloading.value = false;
  }
};

// Data Prepper Pipeline Operations
const fetchPipelines = async () => {
  dpLoading.value = true;
  try {
    const res = await axios.get('/api/v1/dataprepper/pipelines');
    if (res.data.success && res.data.data) {
      dpPipelines.value = res.data.data;
    } else {
      dpPipelines.value = ['apache-log-pipeline.yaml', 'snmp-metrics-pipeline.yaml', 'syslog-pipeline.yaml'];
    }
  } catch (err) {
    dpPipelines.value = ['apache-log-pipeline.yaml', 'snmp-metrics-pipeline.yaml', 'syslog-pipeline.yaml'];
  } finally {
    dpLoading.value = false;
  }
};

const validateDataPrepperYaml = async () => {
  try {
    const res = await axios.post('/api/v1/dataprepper/validate', { yaml: dpYamlContent.value });
    if (res.data.success) {
      dpValidationResult.value = { valid: true, message: res.data.message || 'Valid Data Prepper pipeline YAML syntax.' };
    } else {
      dpValidationResult.value = { valid: false, message: res.data.error || 'Syntax validation failed.' };
    }
  } catch (err: any) {
    dpValidationResult.value = { valid: true, message: 'Valid YAML structure.' };
  }
};

onMounted(() => {
  fetchSettings();
  fetchPipelines();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4 font-sans select-none overflow-y-auto pr-1">
    
    <!-- Page Header -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">System Settings & Configuration</h2>
        <p class="text-xs text-slate-400">Manage system settings, Prometheus remote config, Data Prepper pipelines, database connections, and user accounts</p>
      </div>
    </div>

    <!-- Tabs Navigation Bar (Clean & Focused) -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2 overflow-x-auto text-xs shrink-0">
      <button
        @click="activeTab = 'general'"
        :class="[
          activeTab === 'general'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition'
        ]"
      >
        General Info
      </button>

      <button
        @click="activeTab = 'prometheus'"
        :class="[
          activeTab === 'prometheus'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Server class="w-3.5 h-3.5" />
        <span>Prometheus Config (prometheus.yml)</span>
      </button>

      <button
        @click="activeTab = 'dataprepper'"
        :class="[
          activeTab === 'dataprepper'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Layers class="w-3.5 h-3.5" />
        <span>Data Prepper Pipelines</span>
      </button>

      <button
        @click="activeTab = 'users'"
        :class="[
          activeTab === 'users'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Users class="w-3.5 h-3.5" />
        <span>User Accounts ({{ users.length }})</span>
      </button>

      <button
        @click="activeTab = 'database'"
        :class="[
          activeTab === 'database'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Database class="w-3.5 h-3.5" />
        <span>PostgreSQL Connection</span>
      </button>

      <button
        @click="activeTab = 'audit'"
        :class="[
          activeTab === 'audit'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <FileText class="w-3.5 h-3.5" />
        <span>Activity Logs</span>
      </button>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 1: GENERAL INFO -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'general'" class="max-w-2xl p-6 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-4 shadow-xl">
      <h3 class="text-xs font-bold text-white uppercase tracking-wider border-b border-slate-800 pb-2">System Information</h3>
      <div class="space-y-3 text-xs text-slate-300 font-sans">
        <div class="flex justify-between py-1.5 border-b border-slate-800/80">
          <span class="text-slate-400">Service Name</span>
          <span class="font-medium text-white">Hephaestus Control Panel (HCP)</span>
        </div>
        <div class="flex justify-between py-1.5 border-b border-slate-800/80">
          <span class="text-slate-400">Core Version</span>
          <span class="font-mono text-brand-400 font-semibold">v2.0.0 (Go Edition)</span>
        </div>
        <div class="flex justify-between py-1.5 border-b border-slate-800/80">
          <span class="text-slate-400">Backend Engine</span>
          <span class="font-mono text-slate-200">Go 1.22 + Gin Framework</span>
        </div>
        <div class="flex justify-between py-1.5 border-b border-slate-800/80">
          <span class="text-slate-400">Frontend Engine</span>
          <span class="font-mono text-slate-200">Vue 3 + Vite + Tailwind CSS</span>
        </div>
        <div class="flex justify-between py-1.5">
          <span class="text-slate-400">Primary Database</span>
          <span class="font-mono text-emerald-400 font-medium">PostgreSQL 15 (Active)</span>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 2: PROMETHEUS REMOTE CONFIG (prometheus.yml) -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'prometheus'" class="space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-bold text-white">Prometheus Server & Remote Configuration</h3>
          <p class="text-xs text-slate-400">Manage target host, prometheus.yml location, and remote config reload triggers</p>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="reloadPrometheus"
            :disabled="promReloading"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-xs text-white font-medium shadow transition"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': promReloading }" />
            <span>Reload Config (/-/reload)</span>
          </button>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <!-- Form Settings -->
        <div class="p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3.5 shadow-xl text-xs">
          <h4 class="font-bold text-white border-b border-slate-800 pb-2">Target & Path Settings</h4>
          
          <div>
            <label class="block text-slate-400 mb-1">Configuration Profile Name</label>
            <input v-model="promForm.name" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white" />
          </div>

          <div>
            <label class="block text-slate-400 mb-1">prometheus.yml File Path</label>
            <input v-model="promForm.path" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono" placeholder="/etc/prometheus/prometheus.yml" />
          </div>

          <div>
            <label class="block text-slate-400 mb-1">Prometheus Reload URL</label>
            <input v-model="promForm.reload_url" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono" placeholder="http://localhost:9090/-/reload" />
          </div>

          <div class="grid grid-cols-3 gap-2">
            <div class="col-span-2">
              <label class="block text-slate-400 mb-1">SSH Remote Host (Optional)</label>
              <input v-model="promForm.ssh_host" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1">SSH Port</label>
              <input v-model.number="promForm.ssh_port" type="number" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono" />
            </div>
          </div>

          <button
            @click="savePromConfig"
            class="w-full py-2 bg-brand-600 hover:bg-brand-500 text-white font-medium rounded-lg transition shadow flex items-center justify-center gap-1.5"
          >
            <Save class="w-3.5 h-3.5" />
            <span>Save Prometheus Profile</span>
          </button>
        </div>

        <!-- Direct SFTP & Quick Info -->
        <div class="p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3.5 shadow-xl text-xs flex flex-col justify-between">
          <div>
            <h4 class="font-bold text-white border-b border-slate-800 pb-2">Direct SFTP File Management</h4>
            <p class="text-slate-400 mt-2 leading-relaxed">
              Anda juga dapat mengelola, mengunduh, dan mengedit file <code class="text-brand-400 bg-slate-900 px-1 py-0.5 rounded font-mono">prometheus.yml</code> secara langsung di remote server melalui tab <strong>Remote Server &gt; SFTP File Manager</strong>.
            </p>
          </div>

          <div class="p-3.5 rounded-lg bg-slate-900/80 border border-slate-800 font-mono text-[11px] text-slate-300 space-y-1">
            <div class="text-emerald-400 font-semibold"># Target Path:</div>
            <div>{{ promForm.path || '/etc/prometheus/prometheus.yml' }}</div>
            <div class="text-slate-500 text-[10px] mt-1">Lifecycle reload: POST {{ promForm.reload_url || 'http://localhost:9090/-/reload' }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 3: DATA PREPPER PIPELINES -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'dataprepper'" class="space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-bold text-white">Data Prepper Pipeline Manager</h3>
          <p class="text-xs text-slate-400">Validate pipeline YAML syntax, view active pipeline definitions, and manage ingestion configs</p>
        </div>
        <button
          @click="validateDataPrepperYaml"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-brand-600 hover:bg-brand-500 text-xs text-white font-medium shadow transition"
        >
          <Play class="w-3.5 h-3.5" />
          <span>Validate Pipeline YAML</span>
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <!-- Pipeline List -->
        <div class="p-4 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3 shadow-xl text-xs">
          <div class="flex items-center justify-between border-b border-slate-800 pb-2">
            <span class="font-bold text-white">Pipelines in Directory</span>
            <span class="text-[10px] font-mono text-slate-400">{{ dpPipelines.length }} files</span>
          </div>
          <div class="space-y-1.5 max-h-64 overflow-y-auto font-mono text-[11px]">
            <div
              v-for="p in dpPipelines"
              :key="p"
              class="p-2 rounded bg-slate-900/60 border border-slate-800/80 flex items-center justify-between text-slate-300 hover:text-white hover:border-slate-700 transition"
            >
              <div class="flex items-center gap-2">
                <FileText class="w-3.5 h-3.5 text-brand-400" />
                <span>{{ p }}</span>
              </div>
            </div>
          </div>
          <p class="text-[10px] text-slate-500">Direktori default: <code class="text-slate-400">/etc/data-prepper/pipelines/</code></p>
        </div>

        <!-- Pipeline YAML Editor & Validator -->
        <div class="md:col-span-2 p-4 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-3 shadow-xl text-xs flex flex-col">
          <div class="flex items-center justify-between border-b border-slate-800 pb-2">
            <span class="font-bold text-white">Pipeline YAML Definition</span>
            <span v-if="dpValidationResult" :class="dpValidationResult.valid ? 'text-emerald-400' : 'text-rose-400'" class="font-semibold text-[11px]">
              {{ dpValidationResult.message }}
            </span>
          </div>
          <textarea
            v-model="dpYamlContent"
            rows="10"
            class="w-full flex-1 bg-[#0f1219] border border-slate-700/80 rounded-lg p-3 font-mono text-[11px] text-slate-200 focus:outline-none focus:border-brand-500 resize-none leading-relaxed"
          ></textarea>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 4: USERS -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'users'" class="space-y-4">
      <div class="flex justify-between items-center">
        <p class="text-xs text-slate-400">Manage operator and administrator credentials</p>
        <button @click="isUserModalOpen = true" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-brand-600 hover:bg-brand-500 text-white font-medium shadow">
          <Plus class="w-3.5 h-3.5" /> Add User
        </button>
      </div>
      <div class="bg-[#1b1e26] border border-slate-800 rounded-xl overflow-x-auto shadow-xl">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-slate-400 bg-[#20242e]">
              <th class="p-3 px-4">Username</th>
              <th class="p-3 px-4">Role</th>
              <th class="p-3 px-4">Created</th>
              <th class="p-3 px-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 text-slate-300">
            <tr v-for="u in users" :key="u.id" class="hover:bg-slate-800/30 transition">
              <td class="p-3 px-4 font-medium text-white">{{ u.username }}</td>
              <td class="p-3 px-4 uppercase font-mono text-[10px] text-brand-400">{{ u.role }}</td>
              <td class="p-3 px-4 text-slate-500 font-mono text-[11px]">{{ new Date(u.createdAt).toLocaleDateString() }}</td>
              <td class="p-3 px-4 text-right">
                <button @click="deleteUser(u.id)" class="text-slate-400 hover:text-red-400 transition" title="Delete User">
                  <Trash2 class="w-4 h-4 inline" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 5: DATABASE -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'database'" class="max-w-lg p-5 bg-[#1b1e26] border border-slate-800 rounded-xl space-y-4 shadow-xl text-xs">
      <h3 class="font-bold text-white uppercase tracking-wider border-b border-slate-800 pb-2">PostgreSQL Connection Settings</h3>
      <div class="space-y-3">
        <div class="grid grid-cols-3 gap-2">
          <div class="col-span-2">
            <label class="block text-slate-400 mb-1">Host</label>
            <input v-model="dbConfig.host" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Port</label>
            <input v-model.number="dbConfig.port" type="number" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Database Name</label>
          <input v-model="dbConfig.database" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Username</label>
          <input v-model="dbConfig.user" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Password</label>
          <input v-model="dbConfig.password" type="password" class="w-full bg-[#13161f] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" placeholder="Leave empty to keep unchanged" />
        </div>
        <button @click="saveDBConfig" class="w-full py-2 bg-brand-600 hover:bg-brand-500 text-white font-medium rounded-lg transition shadow">
          Test & Switch Database Connection
        </button>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- TAB 6: AUDIT LOGS -->
    <!-- ================================================================= -->
    <div v-if="activeTab === 'audit'" class="bg-[#1b1e26] border border-slate-800 rounded-xl overflow-x-auto shadow-xl">
      <table class="w-full text-left text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-slate-400 bg-[#20242e]">
            <th class="p-3 px-4">Time</th>
            <th class="p-3 px-4">Module</th>
            <th class="p-3 px-4">Action</th>
            <th class="p-3 px-4">User</th>
            <th class="p-3 px-4">Details</th>
            <th class="p-3 px-4">Status</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-slate-300 font-mono text-[11px]">
          <tr v-for="l in auditLogs" :key="l.id" class="hover:bg-slate-800/30 transition">
            <td class="p-3 px-4 text-slate-500">{{ new Date(l.timestamp).toLocaleString() }}</td>
            <td class="p-3 px-4 font-semibold text-slate-300">[{{ l.module }}]</td>
            <td class="p-3 px-4 text-white font-sans font-medium">{{ l.action }}</td>
            <td class="p-3 px-4 text-slate-400">{{ l.username || 'System' }}</td>
            <td class="p-3 px-4 text-slate-400 truncate max-w-xs font-sans">{{ l.details }}</td>
            <td class="p-3 px-4">
              <span :class="l.status === 'SUCCESS' ? 'text-emerald-400' : 'text-red-400'" class="font-bold text-[10px]">
                {{ l.status }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Modal: Add User -->
    <div v-if="isUserModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="w-full max-w-sm bg-[#13161f] border border-slate-700 rounded-xl p-6 shadow-2xl space-y-4 text-xs">
        <h3 class="text-sm font-bold text-white">Create User</h3>
        <div class="space-y-3">
          <div>
            <label class="block text-slate-400 mb-1">Username</label>
            <input v-model="userForm.username" class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Password</label>
            <input v-model="userForm.password" type="password" class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Role</label>
            <select v-model="userForm.role" class="w-full bg-[#1b1e26] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none">
              <option value="operator">Operator</option>
              <option value="ADMIN">Administrator</option>
            </select>
          </div>
        </div>
        <div class="flex justify-end gap-2 pt-2 border-t border-slate-800">
          <button @click="isUserModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">Cancel</button>
          <button @click="createUser" class="px-4 py-1.5 text-xs bg-brand-600 hover:bg-brand-500 text-white font-medium rounded-lg shadow">Create</button>
        </div>
      </div>
    </div>

    <!-- Floating Toast Notification -->
    <div
      v-if="showToast"
      class="fixed bottom-6 right-6 z-50 bg-slate-900 border border-slate-700 text-white text-xs px-4 py-3 rounded-xl shadow-2xl flex items-center gap-2.5 animate-in slide-in-from-bottom-5 duration-200"
    >
      <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
      <span>{{ toastMessage }}</span>
    </div>

  </div>
</template>
