<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue';
import { useRouter } from 'vue-router';
import { 
  Terminal, 
  Network, 
  Database, 
  Search, 
  Radio, 
  Server, 
  Activity, 
  ListTree, 
  Settings, 
  FileText 
} from 'lucide-vue-next';

const isOpen = ref(false);
const searchQuery = ref('');
const router = useRouter();

const items = [
  { name: 'Dashboard', icon: Activity, route: '/' },
  { name: 'Remote Host Terminal', icon: Terminal, route: '/terminal' },
  { name: 'Network Topology Map', icon: Network, route: '/topology' },
  { name: 'Database Backup Manager', icon: Database, route: '/backup' },
  { name: 'SNMP Browser & MIBs', icon: Radio, route: '/snmp' },
  { name: 'OpenSearch Cluster Health', icon: Search, route: '/opensearch' },
  { name: 'Grok Regex Debugger', icon: ListTree, route: '/grok-debugger' },
  { name: 'VPS Telemetry & Services', icon: Server, route: '/vps-control' },
  { name: 'Live Backend Logs', icon: FileText, route: '/logs' },
  { name: 'Background Queue Tasks', icon: ListTree, route: '/queue' },
  { name: 'System Settings', icon: Settings, route: '/settings' },
];

const filteredItems = computed(() => {
  if (!searchQuery.value) return items;
  return items.filter(item => 
    item.name.toLowerCase().includes(searchQuery.value.toLowerCase())
  );
});

const navigate = (route: string) => {
  isOpen.value = false;
  searchQuery.value = '';
  router.push(route);
};

const handleKeyDown = (e: KeyboardEvent) => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault();
    isOpen.value = !isOpen.value;
  }
  if (e.key === 'Escape' && isOpen.value) {
    isOpen.value = false;
  }
};

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-start justify-center pt-24 bg-black/60 backdrop-blur-sm">
    <div class="w-full max-w-xl bg-slate-900 border border-slate-700 rounded-xl shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
      <!-- Search Input -->
      <div class="flex items-center px-4 py-3 border-b border-slate-800">
        <Search class="w-5 h-5 text-slate-400 mr-3" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search tools, servers, topologies... (Press ESC to close)"
          class="w-full bg-transparent text-slate-100 placeholder-slate-500 focus:outline-none text-sm"
          autofocus
        />
        <kbd class="px-2 py-0.5 text-xs bg-slate-800 text-slate-400 border border-slate-700 rounded">ESC</kbd>
      </div>

      <!-- Results List -->
      <div class="max-h-80 overflow-y-auto p-2">
        <button
          v-for="item in filteredItems"
          :key="item.name"
          @click="navigate(item.route)"
          class="w-full flex items-center px-3 py-2.5 rounded-lg text-sm text-slate-300 hover:bg-slate-800 hover:text-white transition group text-left"
        >
          <component :is="item.icon" class="w-4 h-4 mr-3 text-slate-400 group-hover:text-brand-500 transition" />
          <span>{{ item.name }}</span>
        </button>
        <div v-if="filteredItems.length === 0" class="py-8 text-center text-sm text-slate-500">
          No matching modules found
        </div>
      </div>
    </div>
  </div>
</template>
