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
  Calendar 
} from 'lucide-vue-next';

const activeTab = ref<'databases' | 'destinations' | 'schedules' | 'history'>('databases');

const databases = ref<any[]>([]);
const destinations = ref<any[]>([]);
const schedules = ref<any[]>([]);
const history = ref<any[]>([]);
const loading = ref(false);

const isRunBackupModalOpen = ref(false);
const runForm = ref({
  dbConfigId: '',
  destinationId: '',
});

const isDbModalOpen = ref(false);
const dbForm = ref({
  name: '',
  dbType: 'postgresql',
  host: 'localhost',
  port: 5432,
  username: 'postgres',
  password: '',
  databaseName: '',
});

const isDestModalOpen = ref(false);
const destForm = ref({
  name: '',
  destType: 'local',
  config: {
    path: '/opt/backups',
    bucket: '',
    endpoint: '',
    accessKeyId: '',
    secretAccessKey: '',
  },
});

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
      axios.get('/api/v1/backup/databases'),
      axios.get('/api/v1/backup/destinations'),
      axios.get('/api/v1/backup/schedules'),
      axios.get('/api/v1/backup/history'),
    ]);

    if (dbRes.data.success) databases.value = dbRes.data.data || [];
    if (destRes.data.success) destinations.value = destRes.data.data || [];
    if (schedRes.data.success) schedules.value = schedRes.data.data || [];
    if (histRes.data.success) history.value = histRes.data.data.history || [];
  } catch (err) {
    console.error('Failed to load backup data:', err);
  } finally {
    loading.value = false;
  }
};

const triggerBackup = async () => {
  try {
    const res = await axios.post('/api/v1/backup/run', runForm.value);
    if (res.data.success) {
      isRunBackupModalOpen.value = false;
      alert(`Backup job #${res.data.data.jobId} enqueued!`);
      activeTab.value = 'history';
      fetchAll();
    }
  } catch (err: any) {
    alert(`Failed to trigger backup: ${err.response?.data?.error || err.message}`);
  }
};

const saveDBConfig = async () => {
  try {
    const res = await axios.post('/api/v1/backup/databases', dbForm.value);
    if (res.data.success) {
      isDbModalOpen.value = false;
      fetchAll();
    }
  } catch (err) {
    console.error(err);
  }
};

const saveDestination = async () => {
  try {
    const res = await axios.post('/api/v1/backup/destinations', destForm.value);
    if (res.data.success) {
      isDestModalOpen.value = false;
      fetchAll();
    }
  } catch (err) {
    console.error(err);
  }
};

const saveSchedule = async () => {
  try {
    const res = await axios.post('/api/v1/backup/schedules', scheduleForm.value);
    if (res.data.success) {
      isScheduleModalOpen.value = false;
      fetchAll();
    }
  } catch (err) {
    console.error(err);
  }
};

onMounted(() => {
  fetchAll();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <!-- Header -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">Database Backup Manager</h2>
        <p class="text-xs text-slate-400">Automated multi-database backups and S3/R2/NAS disaster recovery</p>
      </div>
      <button
        @click="isRunBackupModalOpen = true"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-brand-500 hover:bg-brand-600 text-white shadow-lg shadow-brand-500/20 transition"
      >
        <Play class="w-3.5 h-3.5 fill-current" />
        Run Backup Now
      </button>
    </div>

    <!-- Navigation Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2">
      <button
        @click="activeTab = 'databases'"
        :class="[activeTab === 'databases' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        Databases ({{ databases.length }})
      </button>
      <button
        @click="activeTab = 'destinations'"
        :class="[activeTab === 'destinations' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        Storage Destinations ({{ destinations.length }})
      </button>
      <button
        @click="activeTab = 'schedules'"
        :class="[activeTab === 'schedules' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        Cron Schedules ({{ schedules.length }})
      </button>
      <button
        @click="activeTab = 'history'"
        :class="[activeTab === 'history' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        Execution History ({{ history.length }})
      </button>
    </div>

    <!-- Tab 1: Databases -->
    <div v-if="activeTab === 'databases'" class="space-y-4">
      <div class="flex justify-end">
        <button @click="isDbModalOpen = true" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-white border border-slate-700">
          <Plus class="w-3.5 h-3.5" /> Add Database
        </button>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div v-for="db in databases" :key="db.id" class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-2">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white">{{ db.name }}</h4>
            <span class="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-slate-800 text-brand-400">{{ db.dbType }}</span>
          </div>
          <p class="text-[11px] font-mono text-slate-400">{{ db.username }}@{{ db.host }}:{{ db.port }}/{{ db.databaseName }}</p>
        </div>
      </div>
    </div>

    <!-- Tab 2: Destinations -->
    <div v-if="activeTab === 'destinations'" class="space-y-4">
      <div class="flex justify-end">
        <button @click="isDestModalOpen = true" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-white border border-slate-700">
          <Plus class="w-3.5 h-3.5" /> Add Destination
        </button>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div v-for="dest in destinations" :key="dest.id" class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-2">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white">{{ dest.name }}</h4>
            <span class="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-slate-800 text-purple-400">{{ dest.destType }}</span>
          </div>
          <p class="text-[11px] font-mono text-slate-400 truncate">{{ dest.destType === 'local' ? dest.config?.path : dest.config?.bucket }}</p>
        </div>
      </div>
    </div>

    <!-- Tab 3: Schedules -->
    <div v-if="activeTab === 'schedules'" class="space-y-4">
      <div class="flex justify-end">
        <button @click="isScheduleModalOpen = true" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-white border border-slate-700">
          <Plus class="w-3.5 h-3.5" /> Add Schedule
        </button>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div v-for="sched in schedules" :key="sched.id" class="p-4 rounded-xl bg-slate-900/60 border border-slate-800 space-y-2">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white">{{ sched.name }}</h4>
            <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-amber-400">{{ sched.cronExpression }}</span>
          </div>
          <div class="text-[11px] text-slate-400 flex items-center justify-between">
            <span>Last Run: {{ sched.lastRun ? new Date(sched.lastRun).toLocaleString() : 'Never' }}</span>
            <span :class="sched.isActive ? 'text-emerald-400' : 'text-slate-500'">{{ sched.isActive ? 'Active' : 'Paused' }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab 4: History -->
    <div v-if="activeTab === 'history'" class="bg-slate-900/60 border border-slate-800 rounded-xl overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-slate-400">
            <th class="p-3">File</th>
            <th class="p-3">Database</th>
            <th class="p-3">Destination</th>
            <th class="p-3">Size</th>
            <th class="p-3">Status</th>
            <th class="p-3">Time</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-slate-300">
          <tr v-for="h in history" :key="h.id">
            <td class="p-3 font-mono text-[11px] text-white">{{ h.filename }}</td>
            <td class="p-3">{{ h.dbName }} ({{ h.dbType }})</td>
            <td class="p-3 uppercase font-mono text-[10px]">{{ h.destType }}</td>
            <td class="p-3 font-mono">{{ (h.fileSize / 1024 / 1024).toFixed(2) }} MB</td>
            <td class="p-3">
              <span :class="[
                h.status === 'success' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-red-500/10 text-red-400 border-red-500/20',
                'px-2 py-0.5 rounded text-[10px] font-medium border'
              ]">
                {{ h.status }}
              </span>
            </td>
            <td class="p-3 text-slate-500 font-mono text-[11px]">{{ new Date(h.startedAt).toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Run Backup Modal -->
    <div v-if="isRunBackupModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="w-full max-w-md bg-slate-900 border border-slate-700 rounded-xl p-6 shadow-2xl space-y-4">
        <h3 class="text-sm font-bold text-white">Run On-Demand Backup</h3>
        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Select Database</label>
            <select v-model="runForm.dbConfigId" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none">
              <option v-for="db in databases" :key="db.id" :value="db.id">{{ db.name }} ({{ db.databaseName }})</option>
            </select>
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Select Destination</label>
            <select v-model="runForm.destinationId" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none">
              <option v-for="d in destinations" :key="d.id" :value="d.id">{{ d.name }} ({{ d.destType }})</option>
            </select>
          </div>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button @click="isRunBackupModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">Cancel</button>
          <button @click="triggerBackup" class="px-4 py-1.5 text-xs bg-brand-500 hover:bg-brand-600 text-white font-medium rounded">Start Backup</button>
        </div>
      </div>
    </div>
  </div>
</template>
