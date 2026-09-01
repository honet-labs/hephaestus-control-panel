<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { Settings, Users, Database, Shield, Activity, Plus, Trash2 } from 'lucide-vue-next';

const activeTab = ref<'general' | 'users' | 'database' | 'audit'>('general');

const users = ref<any[]>([]);
const auditLogs = ref<any[]>([]);
const dbConfig = ref<any>({
  host: 'localhost',
  port: 5432,
  user: 'postgres',
  password: '',
  database: 'hephaestus',
  ssl: false,
});

const isUserModalOpen = ref(false);
const userForm = ref({
  username: '',
  password: '',
  role: 'operator',
});

const fetchSettings = async () => {
  try {
    const [usersRes, logsRes, dbRes] = await Promise.all([
      axios.get('/api/v1/settings/users'),
      axios.get('/api/v1/settings/activity-logs?limit=50'),
      axios.get('/api/v1/settings/database'),
    ]);
    if (usersRes.data.success) users.value = usersRes.data.data || [];
    if (logsRes.data.success) auditLogs.value = logsRes.data.data.logs || [];
    if (dbRes.data.success) dbConfig.value = dbRes.data.data;
  } catch (err) {
    console.error(err);
  }
};

const createUser = async () => {
  try {
    const res = await axios.post('/api/v1/settings/users', userForm.value);
    if (res.data.success) {
      isUserModalOpen.value = false;
      userForm.value = { username: '', password: '', role: 'operator' };
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
    fetchSettings();
  } catch (err: any) {
    alert(err.response?.data?.error || err.message);
  }
};

const saveDBConfig = async () => {
  try {
    const res = await axios.post('/api/v1/settings/database', dbConfig.value);
    if (res.data.success) {
      alert('Database connection updated and synchronized successfully!');
    }
  } catch (err: any) {
    alert(`Failed to switch database: ${err.response?.data?.error || err.message}`);
  }
};

onMounted(() => {
  fetchSettings();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <div>
      <h2 class="text-xl font-bold text-white tracking-tight">System Settings & Management</h2>
      <p class="text-xs text-slate-400">User accounts, role permissions, activity audit logs, and database switching</p>
    </div>

    <!-- Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2">
      <button
        @click="activeTab = 'general'"
        :class="[activeTab === 'general' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        General
      </button>
      <button
        @click="activeTab = 'users'"
        :class="[activeTab === 'users' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        User Accounts ({{ users.length }})
      </button>
      <button
        @click="activeTab = 'database'"
        :class="[activeTab === 'database' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        PostgreSQL Connection
      </button>
      <button
        @click="activeTab = 'audit'"
        :class="[activeTab === 'audit' ? 'bg-brand-500/10 text-brand-400 border-brand-500/30' : 'text-slate-400', 'px-3 py-1 text-xs font-medium rounded-lg border transition']"
      >
        Activity Logs
      </button>
    </div>

    <!-- Tab 1: General Info -->
    <div v-if="activeTab === 'general'" class="max-w-xl p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-4">
      <h3 class="text-xs font-bold text-white uppercase tracking-wider">System Information</h3>
      <div class="space-y-2 text-xs text-slate-300">
        <div class="flex justify-between py-1.5 border-b border-slate-800"><span class="text-slate-500">Service</span><span class="font-medium text-white">Hephaestus Control Panel (HCP)</span></div>
        <div class="flex justify-between py-1.5 border-b border-slate-800"><span class="text-slate-500">Version</span><span class="font-mono text-brand-400">v2.0.0 (Go Edition)</span></div>
        <div class="flex justify-between py-1.5 border-b border-slate-800"><span class="text-slate-500">Backend Engine</span><span class="font-mono">Go 1.22 + Gin</span></div>
        <div class="flex justify-between py-1.5 border-b border-slate-800"><span class="text-slate-500">Frontend Engine</span><span class="font-mono">Vue 3 + Vite + Tailwind CSS</span></div>
      </div>
    </div>

    <!-- Tab 2: Users -->
    <div v-if="activeTab === 'users'" class="space-y-4">
      <div class="flex justify-end">
        <button @click="isUserModalOpen = true" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-brand-500 hover:bg-brand-600 text-white font-medium">
          <Plus class="w-3.5 h-3.5" /> Add User
        </button>
      </div>
      <div class="bg-slate-900/60 border border-slate-800 rounded-xl overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-slate-400 bg-slate-950/40">
              <th class="p-3">Username</th>
              <th class="p-3">Role</th>
              <th class="p-3">Created</th>
              <th class="p-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 text-slate-300">
            <tr v-for="u in users" :key="u.id">
              <td class="p-3 font-medium text-white">{{ u.username }}</td>
              <td class="p-3 uppercase font-mono text-[10px] text-brand-400">{{ u.role }}</td>
              <td class="p-3 text-slate-500 font-mono text-[11px]">{{ new Date(u.createdAt).toLocaleDateString() }}</td>
              <td class="p-3 text-right">
                <button @click="deleteUser(u.id)" class="text-slate-400 hover:text-red-400 transition"><Trash2 class="w-4 h-4 inline" /></button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Tab 3: Database -->
    <div v-if="activeTab === 'database'" class="max-w-lg p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-4">
      <h3 class="text-xs font-bold text-white uppercase tracking-wider">PostgreSQL Connection Settings</h3>
      <div class="space-y-3 text-xs">
        <div class="grid grid-cols-3 gap-2">
          <div class="col-span-2">
            <label class="block text-slate-400 mb-1">Host</label>
            <input v-model="dbConfig.host" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Port</label>
            <input v-model.number="dbConfig.port" type="number" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white font-mono" />
          </div>
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Database Name</label>
          <input v-model="dbConfig.database" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white font-mono" />
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Username</label>
          <input v-model="dbConfig.user" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white font-mono" />
        </div>
        <div>
          <label class="block text-slate-400 mb-1">Password</label>
          <input v-model="dbConfig.password" type="password" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white font-mono" placeholder="Leave empty to keep unchanged" />
        </div>
        <button @click="saveDBConfig" class="w-full py-2 bg-brand-500 hover:bg-brand-600 text-white font-medium rounded transition">
          Test & Switch Database Connection
        </button>
      </div>
    </div>

    <!-- Tab 4: Audit Logs -->
    <div v-if="activeTab === 'audit'" class="bg-slate-900/60 border border-slate-800 rounded-xl overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-slate-400 bg-slate-950/40">
            <th class="p-3">Time</th>
            <th class="p-3">Module</th>
            <th class="p-3">Action</th>
            <th class="p-3">User</th>
            <th class="p-3">Details</th>
            <th class="p-3">Status</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-slate-300">
          <tr v-for="l in auditLogs" :key="l.id">
            <td class="p-3 text-slate-500 font-mono text-[11px]">{{ new Date(l.timestamp).toLocaleString() }}</td>
            <td class="p-3 font-semibold text-slate-300">[{{ l.module }}]</td>
            <td class="p-3 text-white font-medium">{{ l.action }}</td>
            <td class="p-3 text-slate-400 font-mono text-[11px]">{{ l.username || 'System' }}</td>
            <td class="p-3 text-slate-400 truncate max-w-xs">{{ l.details }}</td>
            <td class="p-3">
              <span :class="l.status === 'SUCCESS' ? 'text-emerald-400' : 'text-red-400'" class="font-bold text-[10px]">
                {{ l.status }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add User Modal -->
    <div v-if="isUserModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="w-full max-w-sm bg-slate-900 border border-slate-700 rounded-xl p-6 shadow-2xl space-y-4">
        <h3 class="text-sm font-bold text-white">Create User</h3>
        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1">Username</label>
            <input v-model="userForm.username" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Password</label>
            <input v-model="userForm.password" type="password" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1">Role</label>
            <select v-model="userForm.role" class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-white focus:outline-none">
              <option value="operator">Operator</option>
              <option value="ADMIN">Administrator</option>
            </select>
          </div>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button @click="isUserModalOpen = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">Cancel</button>
          <button @click="createUser" class="px-4 py-1.5 text-xs bg-brand-500 hover:bg-brand-600 text-white font-medium rounded">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>
