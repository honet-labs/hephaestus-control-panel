<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { Clock, RefreshCw, XCircle, CheckCircle2, AlertCircle } from 'lucide-vue-next';

interface Job {
  id: string;
  type: string;
  status: string;
  progress: number;
  message: string;
  error?: string;
  retries: number;
  maxRetries: number;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
}

const jobs = ref<Job[]>([]);
const loading = ref(false);

const fetchJobs = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/queue/jobs');
    if (res.data.success) {
      jobs.value = res.data.data || [];
    }
  } catch (err) {
    console.error('Failed to fetch jobs:', err);
  } finally {
    loading.value = false;
  }
};

const cancelJob = async (id: string) => {
  try {
    const res = await axios.post(`/api/v1/queue/jobs/${id}/cancel`);
    if (res.data.success) {
      fetchJobs();
    }
  } catch (err) {
    console.error('Failed to cancel job:', err);
  }
};

onMounted(() => {
  fetchJobs();
  const interval = setInterval(fetchJobs, 3000);
  return () => clearInterval(interval);
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <!-- Header -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">Background Task Queue</h2>
        <p class="text-xs text-slate-400">Manage and monitor asynchronous worker jobs, backups, and discovery sweeps</p>
      </div>
      <button
        @click="fetchJobs"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
      >
        <RefreshCw class="w-3.5 h-3.5 text-brand-400" />
        Refresh
      </button>
    </div>

    <!-- Jobs Table -->
    <div class="flex-1 bg-slate-900/60 border border-slate-800 rounded-xl overflow-y-auto">
      <table class="w-full text-left text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-slate-400 bg-slate-950/40 sticky top-0">
            <th class="p-3">Job ID</th>
            <th class="p-3">Type</th>
            <th class="p-3">Status</th>
            <th class="p-3">Progress</th>
            <th class="p-3">Message</th>
            <th class="p-3">Retries</th>
            <th class="p-3">Created</th>
            <th class="p-3 text-right">Action</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-slate-300">
          <tr v-for="job in jobs" :key="job.id" class="hover:bg-slate-800/30">
            <td class="p-3 font-mono text-[11px] text-brand-400 font-medium">{{ job.id }}</td>
            <td class="p-3 font-mono text-[11px] text-white">{{ job.type }}</td>
            <td class="p-3">
              <span :class="[
                job.status === 'completed' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' :
                job.status === 'running' ? 'bg-blue-500/10 text-blue-400 border-blue-500/20 animate-pulse' :
                job.status === 'failed' ? 'bg-red-500/10 text-red-400 border-red-500/20' :
                job.status === 'cancelled' ? 'bg-amber-500/10 text-amber-400 border-amber-500/20' :
                'bg-slate-800 text-slate-400 border-slate-700',
                'px-2 py-0.5 rounded text-[10px] uppercase font-bold border'
              ]">
                {{ job.status }}
              </span>
            </td>
            <td class="p-3">
              <div class="w-24 bg-slate-800 h-1.5 rounded-full overflow-hidden">
                <div class="bg-brand-500 h-full rounded-full transition-all" :style="{ width: `${job.progress}%` }"></div>
              </div>
            </td>
            <td class="p-3 text-slate-400 truncate max-w-xs">{{ job.message }}</td>
            <td class="p-3 font-mono text-slate-400">{{ job.retries }}/{{ job.maxRetries }}</td>
            <td class="p-3 text-slate-500 font-mono text-[11px]">{{ new Date(job.createdAt).toLocaleTimeString() }}</td>
            <td class="p-3 text-right">
              <button
                v-if="job.status === 'running' || job.status === 'pending'"
                @click="cancelJob(job.id)"
                class="px-2 py-1 text-[11px] font-medium text-red-400 hover:bg-red-500/10 rounded transition"
              >
                Cancel
              </button>
            </td>
          </tr>
          <tr v-if="jobs.length === 0">
            <td colspan="8" class="py-8 text-center text-slate-500">No background jobs in queue</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
