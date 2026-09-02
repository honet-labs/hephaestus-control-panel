<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import {
  Database,
  Cloud,
  Clock,
  Play,
  Plus,
  Trash2,
  CheckCircle2,
  AlertCircle,
  Calendar,
  RotateCw,
  Server,
  Folder,
  HardDrive,
  Check,
  X,
} from 'lucide-vue-next';

const activeTab = ref<'databases' | 'destinations' | 'schedules' | 'history'>('databases');

const databases = ref<any[]>([]);
const destinations = ref<any[]>([]);
const schedules = ref<any[]>([]);
const history = ref<any[]>([]);
const loading = ref(false);

// Run Backup Modal
const isRunBackupModalOpen = ref(false);
const runForm = ref({
  dbConfigId: '',
  destinationId: '',
});

// Database Modal
const isDbModalOpen = ref(false);
const dbForm = ref({
  name: '',
  dbType: 'postgresql',
  host: 'localhost',
  port: 5432,
  username: 'postgres',
  password: '',
  databaseName: '',
  useSsh: false,
  sshHost: '',
  sshPort: 22,
  sshUser: 'root',
  sshPassword: '',
});

// Destination Modal
const isDestModalOpen = ref(false);
const destForm = ref({
  name: '',
  destType: 'nas',
  host: '',
  port: 22,
  username: 'administrator',
  authType: 'password',
  password: '',
  sshKey: '',
  path: '/opt/backups',
  bucket: '',
  endpoint: '',
  accessKeyId: '',
  secretAccessKey: '',
  accountId: '',
});

// Schedule Modal
const isScheduleModalOpen = ref(false);
const scheduleForm = ref({
  name: '',
  dbConfigId: '',
  destinationId: '',
  cronExpression: '0 2 * * *',
  isActive: true,
});

const fetchAll = async () => {
  loading.value = true;
  try {
    const [dbRes, destRes, schedRes, histRes] = await Promise.all([
      axios.get('/api/v1/backup/databases').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/backup/destinations').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/backup/schedules').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/backup/history').catch(() => ({ data: { success: false } })),
    ]);

    if (dbRes.data?.success) databases.value = dbRes.data.data || [];
    if (destRes.data?.success) destinations.value = destRes.data.data || [];
    if (schedRes.data?.success) schedules.value = schedRes.data.data || [];
    if (histRes.data?.success) history.value = histRes.data.data?.history || [];

    // Pre-select defaults for run modal
    if (databases.value.length > 0 && !runForm.value.dbConfigId) {
      runForm.value.dbConfigId = databases.value[0].id;
    }
    if (destinations.value.length > 0 && !runForm.value.destinationId) {
      runForm.value.destinationId = destinations.value[0].id;
    }
  } catch (err) {
    console.error('Failed to load backup data:', err);
  } finally {
    loading.value = false;
  }
};

const triggerBackup = async () => {
  if (!runForm.value.dbConfigId || !runForm.value.destinationId) {
    alert('Please select both a Database and Storage Destination.');
    return;
  }
  try {
    const res = await axios.post('/api/v1/backup/run', runForm.value);
    if (res.data.success) {
      isRunBackupModalOpen.value = false;
      alert(`Backup job enqueued successfully!`);
      activeTab.value = 'history';
      fetchAll();
    }
  } catch (err: any) {
    alert(`Failed to trigger backup: ${err.response?.data?.error || err.message}`);
  }
};

const saveDBConfig = async () => {
  if (!dbForm.value.name || !dbForm.value.databaseName) {
    alert('Name and Database Name are required.');
    return;
  }
  try {
    const payload: any = {
      name: dbForm.value.name,
      dbType: dbForm.value.dbType,
      host: dbForm.value.host,
      port: Number(dbForm.value.port),
      username: dbForm.value.username,
      password: dbForm.value.password,
      databaseName: dbForm.value.databaseName,
    };
    if (dbForm.value.useSsh) {
      payload.sshHost = dbForm.value.sshHost;
      payload.sshPort = Number(dbForm.value.sshPort);
      payload.sshUser = dbForm.value.sshUser;
      payload.sshPassword = dbForm.value.sshPassword;
      payload.sshAuth = 'password';
    }
    const res = await axios.post('/api/v1/backup/databases', payload);
    if (res.data.success) {
      isDbModalOpen.value = false;
      dbForm.value.name = '';
      dbForm.value.password = '';
      dbForm.value.databaseName = '';
      fetchAll();
    }
  } catch (err: any) {
    alert(`Failed to save database config: ${err.response?.data?.error || err.message}`);
  }
};

const deleteDBConfig = async (id: string) => {
  if (!confirm('Are you sure you want to delete this database configuration?')) return;
  try {
    await axios.delete(`/api/v1/backup/databases/${id}`);
    fetchAll();
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete');
  }
};

const saveDestination = async () => {
  if (!destForm.value.name) {
    alert('Destination name is required.');
    return;
  }
  try {
    const configData: any = {};
    if (destForm.value.destType === 'nas' || destForm.value.destType === 'nfs') {
      configData.host = destForm.value.host;
      configData.port = Number(destForm.value.port) || 22;
      configData.username = destForm.value.username;
      configData.authType = destForm.value.authType;
      configData.password = destForm.value.password;
      configData.sshKey = destForm.value.sshKey;
      configData.path = destForm.value.path || '/opt/backups';
    } else if (destForm.value.destType === 'local') {
      configData.path = destForm.value.path || '/opt/backups';
    } else {
      configData.bucket = destForm.value.bucket;
      configData.endpoint = destForm.value.endpoint;
      configData.accessKeyId = destForm.value.accessKeyId;
      configData.secretAccessKey = destForm.value.secretAccessKey;
      configData.accountId = destForm.value.accountId;
    }
    const res = await axios.post('/api/v1/backup/destinations', {
      name: destForm.value.name,
      destType: destForm.value.destType,
      config: configData,
    });
    if (res.data.success) {
      isDestModalOpen.value = false;
      destForm.value.name = '';
      destForm.value.host = '';
      destForm.value.password = '';
      destForm.value.sshKey = '';
      destForm.value.path = '/opt/backups';
      fetchAll();
    }
  } catch (err: any) {
    alert(`Failed to save destination: ${err.response?.data?.error || err.message}`);
  }
};

const deleteDestination = async (id: string) => {
  if (!confirm('Are you sure you want to delete this storage destination?')) return;
  try {
    await axios.delete(`/api/v1/backup/destinations/${id}`);
    fetchAll();
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete destination');
  }
};

const saveSchedule = async () => {
  if (!scheduleForm.value.name || !scheduleForm.value.dbConfigId || !scheduleForm.value.destinationId) {
    alert('Name, Database, and Destination are required.');
    return;
  }
  try {
    const res = await axios.post('/api/v1/backup/schedules', scheduleForm.value);
    if (res.data.success) {
      isScheduleModalOpen.value = false;
      scheduleForm.value.name = '';
      fetchAll();
    }
  } catch (err: any) {
    alert(`Failed to save schedule: ${err.response?.data?.error || err.message}`);
  }
};

const deleteSchedule = async (id: string) => {
  if (!confirm('Are you sure you want to delete this cron schedule?')) return;
  try {
    await axios.delete(`/api/v1/backup/schedules/${id}`);
    fetchAll();
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete schedule');
  }
};

const deleteHistoryItem = async (id: string) => {
  if (!confirm('Delete this history record?')) return;
  try {
    await axios.delete(`/api/v1/backup/history/${id}`);
    fetchAll();
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete history');
  }
};

const handleRunSingle = (dbId: string) => {
  runForm.value.dbConfigId = dbId;
  isRunBackupModalOpen.value = true;
};

onMounted(() => {
  fetchAll();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
      <div>
        <h1 class="text-xl font-bold text-white tracking-tight">Database Backup Manager</h1>
        <p class="text-xs text-slate-400 mt-0.5">
          Automated multi-database backups, cron scheduling, and S3 / Cloudflare R2 / Local disaster recovery.
        </p>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="fetchAll"
          class="p-2 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
          title="Refresh Backup Status"
        >
          <RotateCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </button>

        <button
          @click="isRunBackupModalOpen = true"
          class="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white transition"
        >
          <Play class="w-3.5 h-3.5 fill-current" />
          <span>RUN BACKUP NOW</span>
        </button>
      </div>
    </div>

    <!-- Navigation Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2 text-xs font-medium">
      <button
        @click="activeTab = 'databases'"
        :class="[
          activeTab === 'databases'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Database class="w-3.5 h-3.5" />
        <span>Databases ({{ databases.length }})</span>
      </button>

      <button
        @click="activeTab = 'destinations'"
        :class="[
          activeTab === 'destinations'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <HardDrive class="w-3.5 h-3.5" />
        <span>Storage Destinations ({{ destinations.length }})</span>
      </button>

      <button
        @click="activeTab = 'schedules'"
        :class="[
          activeTab === 'schedules'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Clock class="w-3.5 h-3.5" />
        <span>Cron Schedules ({{ schedules.length }})</span>
      </button>

      <button
        @click="activeTab = 'history'"
        :class="[
          activeTab === 'history'
            ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-bold'
            : 'text-slate-400 hover:text-slate-200 border-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5'
        ]"
      >
        <Calendar class="w-3.5 h-3.5" />
        <span>Execution History ({{ history.length }})</span>
      </button>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 1: DATABASES -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'databases'" class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-xs text-slate-400">Registered Database Targets for Automated & On-Demand Backup</p>
        <button
          @click="isDbModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Database</span>
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div
          v-for="db in databases"
          :key="db.id"
          class="p-4 rounded-xl bg-[#171a23] border border-slate-800 hover:border-slate-700 space-y-3 shadow-lg transition"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Database class="w-4 h-4 text-brand-400" />
              <h4 class="text-xs font-bold text-white">{{ db.name }}</h4>
            </div>
            <span class="text-[10px] uppercase font-mono px-2 py-0.5 rounded bg-slate-800 text-brand-400 border border-slate-700/60 font-semibold">
              {{ db.dbType }}
            </span>
          </div>

          <p class="text-[11px] font-mono text-slate-400 bg-[#0f1219] p-2 rounded border border-slate-800/80 truncate">
            {{ db.username }}@{{ db.host }}:{{ db.port }}/{{ db.databaseName }}
          </p>

          <div class="flex items-center justify-between pt-1 border-t border-slate-800/80">
            <button
              @click="handleRunSingle(db.id)"
              class="flex items-center gap-1 text-[11px] font-bold text-emerald-400 hover:text-emerald-300 transition"
            >
              <Play class="w-3 h-3 fill-current" />
              <span>Backup Now</span>
            </button>

            <button
              @click="deleteDBConfig(db.id)"
              class="p-1 text-slate-500 hover:text-rose-400 transition"
              title="Delete Database Config"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <!-- Empty state -->
        <div v-if="databases.length === 0 && !loading" class="col-span-3 p-12 text-center bg-[#171a23] border border-slate-800 rounded-xl space-y-2">
          <Database class="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <p class="text-xs font-bold text-slate-300">No Databases Configured</p>
          <p class="text-[11px] text-slate-500 max-w-sm mx-auto">
            Click "Add Database" above to configure a PostgreSQL, MySQL, or MariaDB target for automated backups.
          </p>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 2: STORAGE DESTINATIONS -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'destinations'" class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-xs text-slate-400">Storage Repositories (Local Filesystem, Cloudflare R2, AWS S3, MinIO)</p>
        <button
          @click="isDestModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Destination</span>
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div
          v-for="dest in destinations"
          :key="dest.id"
          class="p-4 rounded-xl bg-[#171a23] border border-slate-800 hover:border-slate-700 space-y-3 shadow-lg transition"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <HardDrive v-if="dest.destType === 'nas' || dest.destType === 'nfs'" class="w-4 h-4 text-emerald-400" />
              <Folder v-else-if="dest.destType === 'local'" class="w-4 h-4 text-sky-400" />
              <Cloud v-else class="w-4 h-4 text-purple-400" />
              <h4 class="text-xs font-bold text-white">{{ dest.name }}</h4>
            </div>
            <span class="text-[10px] uppercase font-mono px-2 py-0.5 rounded bg-slate-800 text-purple-400 border border-slate-700/60 font-semibold">
              {{ dest.destType === 'nas' ? 'NAS (SSH)' : dest.destType }}
            </span>
          </div>

          <p class="text-[11px] font-mono text-slate-400 bg-[#0f1219] p-2 rounded border border-slate-800/80 truncate">
            {{ (dest.destType === 'nas' || dest.destType === 'nfs') ? (dest.config?.host ? `${dest.config.host}:${dest.config.path || '/opt/backups'}` : (dest.config?.path || '/opt/backups')) : (dest.destType === 'local' ? (dest.config?.path || '/opt/backups') : (dest.config?.bucket || 'S3 Bucket')) }}
          </p>

          <div class="flex items-center justify-end pt-1 border-t border-slate-800/80">
            <button
              @click="deleteDestination(dest.id)"
              class="p-1 text-slate-500 hover:text-rose-400 transition"
              title="Delete Storage Destination"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <div v-if="destinations.length === 0 && !loading" class="col-span-3 p-12 text-center bg-[#171a23] border border-slate-800 rounded-xl space-y-2">
          <Cloud class="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <p class="text-xs font-bold text-slate-300">No Storage Destinations Configured</p>
          <p class="text-[11px] text-slate-500 max-w-sm mx-auto">
            Add a Local folder, Cloudflare R2, or AWS S3 destination to store backup archives.
          </p>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 3: CRON SCHEDULES -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'schedules'" class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-xs text-slate-400">Automated Background Cron Backup Jobs</p>
        <button
          @click="isScheduleModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Schedule</span>
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div
          v-for="sched in schedules"
          :key="sched.id"
          class="p-4 rounded-xl bg-[#171a23] border border-slate-800 hover:border-slate-700 space-y-3 shadow-lg transition"
        >
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white">{{ sched.name }}</h4>
            <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-amber-400 border border-slate-700/60 font-semibold">
              {{ sched.cronExpression }}
            </span>
          </div>

          <div class="text-[11px] text-slate-400 flex items-center justify-between">
            <span>Last Run: {{ sched.lastRun ? new Date(sched.lastRun).toLocaleString() : 'Never' }}</span>
            <span :class="sched.isActive ? 'text-emerald-400 font-semibold' : 'text-slate-500'">
              {{ sched.isActive ? '● Active' : '○ Paused' }}
            </span>
          </div>

          <div class="flex items-center justify-end pt-1 border-t border-slate-800/80">
            <button
              @click="deleteSchedule(sched.id)"
              class="p-1 text-slate-500 hover:text-rose-400 transition"
              title="Delete Schedule"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <div v-if="schedules.length === 0 && !loading" class="col-span-2 p-12 text-center bg-[#171a23] border border-slate-800 rounded-xl space-y-2">
          <Clock class="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <p class="text-xs font-bold text-slate-300">No Scheduled Backups</p>
          <p class="text-[11px] text-slate-500 max-w-sm mx-auto">
            Set up a cron schedule to automatically dump and upload databases at scheduled times.
          </p>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 4: EXECUTION HISTORY -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'history'" class="bg-[#171a23] border border-slate-800 rounded-xl overflow-x-auto shadow-xl">
      <table class="w-full text-left text-xs font-mono">
        <thead class="bg-[#1c202b] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
          <tr>
            <th class="p-3">File Archive</th>
            <th class="p-3">Database</th>
            <th class="p-3">Destination</th>
            <th class="p-3">Size</th>
            <th class="p-3">Status</th>
            <th class="p-3">Executed At</th>
            <th class="p-3 w-16 text-right">Action</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-slate-300 text-[11px]">
          <tr v-for="h in history" :key="h.id" class="hover:bg-slate-800/40 transition">
            <td class="p-3 font-semibold text-white">{{ h.filename }}</td>
            <td class="p-3">{{ h.dbName }} ({{ h.dbType }})</td>
            <td class="p-3 uppercase text-purple-400 font-semibold">{{ h.destType }}</td>
            <td class="p-3 text-slate-400">{{ (h.fileSize / 1024 / 1024).toFixed(2) }} MB</td>
            <td class="p-3">
              <span
                :class="[
                  h.status === 'success' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' :
                  h.status === 'running' ? 'bg-amber-500/10 text-amber-400 border-amber-500/30' :
                  'bg-rose-500/10 text-rose-400 border-rose-500/30',
                  'px-2 py-0.5 rounded text-[10px] font-bold uppercase border'
                ]"
              >
                {{ h.status }}
              </span>
            </td>
            <td class="p-3 text-slate-400">{{ new Date(h.startedAt).toLocaleString() }}</td>
            <td class="p-3 text-right">
              <button
                @click="deleteHistoryItem(h.id)"
                class="p-1 text-slate-500 hover:text-rose-400 transition"
                title="Delete Record"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </td>
          </tr>
          <tr v-if="history.length === 0">
            <td colspan="7" class="py-12 text-center text-slate-500 text-xs font-sans">
              No backup executions recorded yet.
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 1: RUN ON-DEMAND BACKUP -->
    <!-- ============================================================= -->
    <div v-if="isRunBackupModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-md bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Play class="w-4 h-4 text-emerald-400 fill-current" />
            <span>Run On-Demand Backup</span>
          </h3>
          <button @click="isRunBackupModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Select Database</label>
            <select v-model="runForm.dbConfigId" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500">
              <option v-for="db in databases" :key="db.id" :value="db.id">
                {{ db.name }} ({{ db.databaseName }}) - {{ db.dbType }}
              </option>
            </select>
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-bold">Select Storage Destination</label>
            <select v-model="runForm.destinationId" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500">
              <option v-for="d in destinations" :key="d.id" :value="d.id">
                {{ d.name }} ({{ d.destType }})
              </option>
            </select>
          </div>
        </div>

        <div class="flex justify-end gap-2 pt-2">
          <button @click="isRunBackupModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">
            Cancel
          </button>
          <button @click="triggerBackup" class="px-4 py-2 text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-lg">
            Start Backup
          </button>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 2: ADD DATABASE CONFIG -->
    <!-- ============================================================= -->
    <div v-if="isDbModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-lg bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Database class="w-4 h-4 text-brand-400" />
            <span>Add Database Configuration</span>
          </h3>
          <button @click="isDbModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveDBConfig" class="space-y-3 text-xs">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Config Label</label>
              <input v-model="dbForm.name" required placeholder="e.g. Primary Postgres" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Database Type</label>
              <select v-model="dbForm.dbType" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
                <option value="postgresql">PostgreSQL</option>
                <option value="mysql">MySQL</option>
                <option value="mariadb">MariaDB</option>
              </select>
            </div>
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div class="col-span-2">
              <label class="block text-slate-400 mb-1 font-bold">Host / IP</label>
              <input v-model="dbForm.host" required class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Port</label>
              <input v-model.number="dbForm.port" type="number" required class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Username</label>
              <input v-model="dbForm.username" required class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Password</label>
              <input v-model="dbForm.password" type="password" placeholder="••••••" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-bold">Database Name</label>
            <input v-model="dbForm.databaseName" required placeholder="e.g. hephaestus_db" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>

          <!-- SSH Tunnel Option -->
          <div class="pt-2 border-t border-slate-800">
            <label class="flex items-center gap-2 cursor-pointer text-slate-300">
              <input type="checkbox" v-model="dbForm.useSsh" class="rounded bg-slate-800 border-slate-700" />
              <span>Connect via Remote SSH Host</span>
            </label>

            <div v-if="dbForm.useSsh" class="mt-2 space-y-2 p-3 bg-[#0f1219] rounded-lg border border-slate-800">
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block text-slate-500 text-[10px]">SSH Host</label>
                  <input v-model="dbForm.sshHost" placeholder="10.20.3.1" class="w-full bg-[#171a23] border border-slate-700 rounded px-2 py-1 text-white text-xs font-mono" />
                </div>
                <div>
                  <label class="block text-slate-500 text-[10px]">SSH Port</label>
                  <input v-model.number="dbForm.sshPort" type="number" class="w-full bg-[#171a23] border border-slate-700 rounded px-2 py-1 text-white text-xs font-mono" />
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-slate-500 text-[10px]">SSH User</label>
                  <input v-model="dbForm.sshUser" placeholder="root" class="w-full bg-[#171a23] border border-slate-700 rounded px-2 py-1 text-white text-xs font-mono" />
                </div>
                <div>
                  <label class="block text-slate-500 text-[10px]">SSH Password</label>
                  <input v-model="dbForm.sshPassword" type="password" placeholder="••••••" class="w-full bg-[#171a23] border border-slate-700 rounded px-2 py-1 text-white text-xs font-mono" />
                </div>
              </div>
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="isDbModalOpen = false" class="px-3 py-1.5 text-slate-400 hover:text-white">Cancel</button>
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow">Save Database</button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 3: ADD STORAGE DESTINATION -->
    <!-- ============================================================= -->
    <div v-if="isDestModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-lg bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Cloud class="w-4 h-4 text-purple-400" />
            <span>Add Storage Destination</span>
          </h3>
          <button @click="isDestModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveDestination" class="space-y-3 text-xs">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Destination Name</label>
              <input v-model="destForm.name" required placeholder="e.g. Local Storage, Cloudflare R2" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Storage Type</label>
              <select v-model="destForm.destType" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
                <option value="nas">NAS (SMB/SSH)</option>
                <option value="local">Local Filesystem Folder</option>
                <option value="r2">Cloudflare R2 Object Storage</option>
                <option value="s3">AWS S3 / MinIO Storage</option>
              </select>
            </div>
          </div>

          <!-- ================= NAS (SMB/SSH) ================= -->
          <template v-if="destForm.destType === 'nas' || destForm.destType === 'nfs'">
            <div class="grid grid-cols-3 gap-3">
              <div class="col-span-2">
                <label class="block text-slate-400 mb-1 font-bold">NAS HOST</label>
                <input
                  v-model="destForm.host"
                  required
                  placeholder="10.3.16.184"
                  class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono"
                />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-bold">SSH PORT</label>
                <input
                  v-model.number="destForm.port"
                  type="number"
                  required
                  class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono"
                />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-slate-400 mb-1 font-bold">USERNAME</label>
                <input
                  v-model="destForm.username"
                  required
                  placeholder="administrator"
                  class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono"
                />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-bold">AUTH</label>
                <select
                  v-model="destForm.authType"
                  class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white"
                >
                  <option value="password">Password</option>
                  <option value="key">SSH Key</option>
                </select>
              </div>
            </div>

            <div v-if="destForm.authType === 'password'">
              <label class="block text-slate-400 mb-1 font-bold">PASSWORD</label>
              <input
                v-model="destForm.password"
                type="password"
                placeholder="••••••••"
                class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono"
              />
            </div>
            <div v-else>
              <label class="block text-slate-400 mb-1 font-bold">SSH PRIVATE KEY</label>
              <textarea
                v-model="destForm.sshKey"
                rows="3"
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono text-[11px]"
              ></textarea>
            </div>

            <div>
              <label class="block text-slate-400 mb-1 font-bold">BACKUP PATH</label>
              <input
                v-model="destForm.path"
                required
                placeholder="/opt/backups-wordpress"
                class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono"
              />
            </div>
          </template>

          <!-- ================= LOCAL FILESYSTEM ================= -->
          <template v-else-if="destForm.destType === 'local'">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Local Directory Path</label>
              <input v-model="destForm.path" required placeholder="/opt/backups" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>
          </template>

          <template v-else>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-slate-400 mb-1 font-bold">Bucket Name</label>
                <input v-model="destForm.bucket" required placeholder="hephaestus-backups" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-bold">Account ID / Region</label>
                <input v-model="destForm.accountId" placeholder="e.g. auto / acct-id" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
              </div>
            </div>

            <div>
              <label class="block text-slate-400 mb-1 font-bold">S3 / R2 Endpoint URL</label>
              <input v-model="destForm.endpoint" placeholder="https://<id>.r2.cloudflarestorage.com" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-slate-400 mb-1 font-bold">Access Key ID</label>
                <input v-model="destForm.accessKeyId" required placeholder="AKIAIOSFODNN7EXAMPLE" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-bold">Secret Access Key</label>
                <input v-model="destForm.secretAccessKey" type="password" required placeholder="••••••••••••••••" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
              </div>
            </div>
          </template>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="isDestModalOpen = false" class="px-3 py-1.5 text-slate-400 hover:text-white">Cancel</button>
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow">Save Destination</button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 4: ADD CRON SCHEDULE -->
    <!-- ============================================================= -->
    <div v-if="isScheduleModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-md bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Clock class="w-4 h-4 text-amber-400" />
            <span>Add Backup Schedule</span>
          </h3>
          <button @click="isScheduleModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveSchedule" class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Schedule Name</label>
            <input v-model="scheduleForm.name" required placeholder="Daily Midnight Backup" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white" />
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-bold">Select Database</label>
            <select v-model="scheduleForm.dbConfigId" required class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
              <option v-for="db in databases" :key="db.id" :value="db.id">
                {{ db.name }} ({{ db.databaseName }})
              </option>
            </select>
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-bold">Select Destination</label>
            <select v-model="scheduleForm.destinationId" required class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
              <option v-for="d in destinations" :key="d.id" :value="d.id">
                {{ d.name }} ({{ d.destType }})
              </option>
            </select>
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-bold">Cron Expression</label>
            <input v-model="scheduleForm.cronExpression" required placeholder="0 2 * * *" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            <span class="text-[10px] text-slate-500 mt-0.5 block">Format: min hour dom mon dow (e.g. 0 2 * * * for 2 AM daily)</span>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="isScheduleModalOpen = false" class="px-3 py-1.5 text-slate-400 hover:text-white">Cancel</button>
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow">Save Schedule</button>
          </div>
        </form>
      </div>
    </div>

  </div>
</template>
