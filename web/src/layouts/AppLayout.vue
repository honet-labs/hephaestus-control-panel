<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import CommandPalette from '../components/CommandPalette.vue';
import {
  LayoutDashboard,
  Terminal,
  Network,
  Database,
  Radio,
  Search,
  ListTree,
  Server,
  FileText,
  Clock,
  Settings,
  LogOut,
  ShieldCheck,
  ExternalLink,
  Activity,
  Sliders,
} from 'lucide-vue-next';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

const navigation = [
  { name: 'Dashboard', icon: LayoutDashboard, route: '/' },
  { name: 'Remote Server', icon: Terminal, route: '/remote-host', newTab: true },
  { name: 'Network Topology', icon: Network, route: '/network-topology', newTab: true },
  { name: 'Remote Config', icon: Sliders, route: '/remote-config' },
  { name: 'Backup Manager', icon: Database, route: '/backup' },
  { name: 'SNMP Browser', icon: Radio, route: '/snmp' },
  { name: 'OpenSearch Cluster', icon: Search, route: '/opensearch-cluster', newTab: true },
  { name: 'Grok Debugger', icon: ListTree, route: '/grok-debugger' },
  { name: 'Live Logs', icon: FileText, route: '/logs' },
  { name: 'Status Services', icon: Activity, route: '/queue' },
  { name: 'Settings', icon: Settings, route: '/settings' },
];

const handleLogout = async () => {
  await authStore.logout();
  router.push('/login');
};

onMounted(() => {
  authStore.fetchUser();
});
</script>

<template>
  <div class="flex h-screen bg-[#090d16] text-slate-100 overflow-hidden font-sans">
    <!-- Command Palette (Ctrl+K) -->
    <CommandPalette />

    <!-- Sidebar -->
    <aside class="w-64 border-r border-slate-800/80 bg-slate-950/60 flex flex-col justify-between backdrop-blur-xl">
      <div>
        <!-- App Brand Header -->
        <div class="h-16 flex items-center px-5 border-b border-slate-800/80 gap-3">
          <div class="w-10 h-10 rounded-xl bg-gradient-to-tr from-brand-600 to-emerald-400 flex items-center justify-center shadow-lg shadow-brand-500/20 font-mono font-black text-xs text-white tracking-tighter">
            HCP
          </div>
          <div>
            <h1 class="font-bold text-sm tracking-wide text-white leading-tight">HEPHAESTUS</h1>
            <span class="text-[10px] text-brand-400 font-mono font-semibold tracking-wider">CONTROL PANEL</span>
          </div>
        </div>

        <!-- Quick Jump button -->
        <div class="px-4 py-3">
          <button
            @click="window?.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))"
            class="w-full flex items-center justify-between px-3 py-1.5 text-xs bg-slate-900 border border-slate-800 text-slate-400 rounded-lg hover:border-slate-700 transition"
          >
            <span class="flex items-center gap-1.5">
              <Command class="w-3.5 h-3.5" />
              Quick search...
            </span>
            <kbd class="text-[10px] font-mono bg-slate-800 px-1.5 py-0.5 rounded border border-slate-700">Ctrl+K</kbd>
          </button>
        </div>

        <!-- Navigation Links -->
        <nav class="px-3 space-y-1 overflow-y-auto max-h-[calc(100vh-210px)]">
          <template v-for="item in navigation" :key="item.name">
            <a
              v-if="item.newTab"
              :href="item.route"
              target="_blank"
              class="flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 group cursor-pointer"
            >
              <div class="flex items-center gap-3">
                <component :is="item.icon" class="w-4 h-4 shrink-0 text-slate-400 group-hover:text-brand-400 transition" />
                <span>{{ item.name }}</span>
              </div>
              <ExternalLink class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
            </a>

            <router-link
              v-else
              :to="item.route"
              :class="[
                route.path === item.route
                  ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-medium'
                  : 'text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 border-transparent',
                'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
              ]"
            >
              <component :is="item.icon" class="w-4 h-4 shrink-0" />
              <span>{{ item.name }}</span>
            </router-link>
          </template>
        </nav>
      </div>

      <!-- User Profile & Logout -->
      <div class="p-3 border-t border-slate-800/80 bg-slate-950/40">
        <div class="flex items-center justify-between px-2 py-1.5 rounded-lg">
          <div class="flex items-center gap-2.5 overflow-hidden">
            <div class="w-7 h-7 rounded-full bg-slate-800 border border-slate-700 flex items-center justify-center font-bold text-xs text-brand-400">
              {{ authStore.user?.username?.charAt(0).toUpperCase() || 'A' }}
            </div>
            <div class="truncate">
              <p class="text-xs font-medium text-slate-200 truncate">{{ authStore.user?.username || 'Admin' }}</p>
              <p class="text-[10px] text-slate-500 uppercase font-mono">{{ authStore.user?.role || 'OPERATOR' }}</p>
            </div>
          </div>
          <button
            @click="handleLogout"
            title="Log out"
            class="p-1.5 text-slate-400 hover:text-red-400 hover:bg-red-500/10 rounded-md transition"
          >
            <LogOut class="w-4 h-4" />
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <main class="flex-1 flex flex-col min-w-0 overflow-hidden bg-[#090d16]">
      <div class="flex-1 overflow-y-auto p-6">
        <router-view />
      </div>
    </main>
  </div>
</template>
