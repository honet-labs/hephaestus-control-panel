<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue';
import { FileText, Pause, Play, Trash2, Filter } from 'lucide-vue-next';

interface LogEntry {
  timestamp: string;
  level: string;
  module: string;
  message: string;
  error?: string;
  fields?: Record<string, any>;
}

const logs = ref<LogEntry[]>([]);
const isPaused = ref(false);
const filterLevel = ref('ALL');
const filterModule = ref('');
const autoScroll = ref(true);
let ws: WebSocket | null = null;

const filteredLogs = computed(() => {
  return logs.value.filter(l => {
    if (filterLevel.value !== 'ALL' && l.level !== filterLevel.value) return false;
    if (filterModule.value && !l.module.toLowerCase().includes(filterModule.value.toLowerCase())) return false;
    return true;
  });
});

const connectWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/logs`;

  ws = new WebSocket(wsUrl);

  ws.onmessage = (event) => {
    if (isPaused.value) return;
    try {
      const entry: LogEntry = JSON.parse(event.data);
      logs.value.push(entry);
      if (logs.value.length > 500) {
        logs.value.shift();
      }
      if (autoScroll.value) {
        nextTick(() => {
          const container = document.getElementById('log-stream-container');
          if (container) {
            container.scrollTop = container.scrollHeight;
          }
        });
      }
    } catch (err) {
      console.error('Failed to parse log entry:', err);
    }
  };

  ws.onclose = () => {
    setTimeout(connectWebSocket, 3000);
  };
};

const clearLogs = () => {
  logs.value = [];
};

onMounted(() => {
  connectWebSocket();
});

onUnmounted(() => {
  if (ws) ws.close();
});
</script>

<template>
  <div class="h-full flex flex-col space-y-4">
    <!-- Header with controls -->
    <div class="flex items-center justify-between shrink-0">
      <div>
        <h2 class="text-xl font-bold text-white tracking-tight">Live Backend Logs</h2>
        <p class="text-xs text-slate-400">Real-time structured logging stream via WebSocket</p>
      </div>

      <!-- Controls -->
      <div class="flex items-center gap-2">
        <!-- Level Filter -->
        <select v-model="filterLevel" class="bg-slate-800 border border-slate-700 text-xs text-slate-200 rounded-lg px-2.5 py-1.5 focus:outline-none">
          <option value="ALL">All Levels</option>
          <option value="INFO">INFO</option>
          <option value="WARN">WARN</option>
          <option value="ERROR">ERROR</option>
          <option value="DEBUG">DEBUG</option>
        </select>

        <!-- Module Search Filter -->
        <input 
          v-model="filterModule" 
          placeholder="Filter module..." 
          class="bg-slate-800 border border-slate-700 text-xs text-slate-200 rounded-lg px-2.5 py-1.5 focus:outline-none placeholder-slate-500 w-32" 
        />

        <button
          @click="isPaused = !isPaused"
          :class="[
            isPaused ? 'bg-amber-500/10 text-amber-400 border-amber-500/30' : 'bg-slate-800 text-slate-300 hover:bg-slate-700',
            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-slate-700 transition'
          ]"
        >
          <component :is="isPaused ? Play : Pause" class="w-3.5 h-3.5" />
          {{ isPaused ? 'Resume' : 'Pause' }}
        </button>

        <button
          @click="clearLogs"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
        >
          <Trash2 class="w-3.5 h-3.5" />
          Clear
        </button>
      </div>
    </div>

    <!-- Live Stream Container -->
    <div id="log-stream-container" class="flex-1 bg-[#090d16] border border-slate-800 rounded-xl p-4 overflow-y-auto font-mono text-[12px] space-y-1 shadow-2xl">
      <div
        v-for="(log, idx) in filteredLogs"
        :key="idx"
        class="flex items-start gap-2 py-0.5 leading-relaxed hover:bg-slate-900/60 px-1 rounded transition"
      >
        <span class="text-slate-500 text-[11px] shrink-0">{{ new Date(log.timestamp).toLocaleTimeString() }}</span>
        <span :class="[
          log.level === 'ERROR' ? 'text-red-400 bg-red-500/10 border-red-500/20' :
          log.level === 'WARN' ? 'text-amber-400 bg-amber-500/10 border-amber-500/20' :
          log.level === 'DEBUG' ? 'text-blue-400 bg-blue-500/10 border-blue-500/20' :
          'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
          'px-1.5 py-0.2 rounded text-[10px] uppercase font-bold border shrink-0'
        ]">
          {{ log.level }}
        </span>
        <span class="text-slate-400 font-semibold shrink-0">[{{ log.module }}]</span>
        <span class="text-slate-200 break-all">{{ log.message }}</span>
        <span v-if="log.error" class="text-red-400 break-all">({{ log.error }})</span>
      </div>

      <div v-if="filteredLogs.length === 0" class="h-64 flex items-center justify-center text-slate-600 text-xs">
        Listening for log events...
      </div>
    </div>
  </div>
</template>
