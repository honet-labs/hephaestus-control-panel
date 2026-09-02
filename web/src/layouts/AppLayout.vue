<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import CommandPalette from '../components/CommandPalette.vue';
import {
  LayoutDashboard,
  Link2,
  Sliders,
  Network,
  Search,
  Database,
  Wrench,
  Settings,
  LogOut,
  ExternalLink,
  ChevronDown,
  ChevronRight,
  Terminal,
  Radio,
  ListTree,
  FileCode,
  Layers,
  Activity,
  Command,
} from 'lucide-vue-next';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

// Accordion states
const isRemoteConfigOpen = ref(true);
const isToolsOpen = ref(true);

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
    <aside class="w-64 border-r border-slate-800/80 bg-[#0e1118] flex flex-col justify-between backdrop-blur-xl shrink-0">
      <div class="flex-1 flex flex-col min-h-0">
        <!-- App Brand Header -->
        <div class="h-16 flex items-center px-5 border-b border-slate-800/80 gap-3 shrink-0">
          <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-brand-600 to-emerald-400 flex items-center justify-center shadow-lg shadow-brand-500/20 font-mono font-black text-xs text-white tracking-tighter">
            HCP
          </div>
          <div>
            <h1 class="font-bold text-sm tracking-wide text-white leading-tight">HEPHAESTUS</h1>
            <span class="text-[10px] text-brand-400 font-mono font-semibold tracking-wider">CONTROL PANEL</span>
          </div>
        </div>

        <!-- Quick Jump / Search button -->
        <div class="px-4 py-3 shrink-0">
          <button
            @click="window?.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))"
            class="w-full flex items-center justify-between px-3 py-1.5 text-xs bg-[#141721] border border-slate-800 text-slate-400 rounded-lg hover:border-slate-700 transition"
          >
            <span class="flex items-center gap-1.5">
              <Command class="w-3.5 h-3.5" />
              Quick search...
            </span>
            <kbd class="text-[10px] font-mono bg-slate-800 px-1.5 py-0.5 rounded border border-slate-700">Ctrl+K</kbd>
          </button>
        </div>

        <!-- Section Label -->
        <div class="px-4 pt-1 pb-1">
          <p class="text-[10px] uppercase font-bold text-slate-500 tracking-wider">OPERATIONAL MODULES</p>
        </div>

        <!-- Navigation Links (With Sub-Menu Accordions) -->
        <nav class="px-3 space-y-1 overflow-y-auto flex-1 select-none pr-2">
          
          <!-- 1. Overview / Dashboard -->
          <router-link
            to="/"
            :class="[
              route.path === '/'
                ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-semibold'
                : 'text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <LayoutDashboard class="w-4 h-4 shrink-0" />
            <span>Overview</span>
          </router-link>

          <!-- 2. Connections -->
          <router-link
            to="/connections"
            :class="[
              route.path === '/connections'
                ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-semibold'
                : 'text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Link2 class="w-4 h-4 shrink-0" />
            <span>Connections</span>
          </router-link>

          <!-- 3. Remote Config (Accordion Parent) -->
          <div>
            <button
              @click="isRemoteConfigOpen = !isRemoteConfigOpen"
              class="w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-400 hover:bg-slate-900/60 hover:text-slate-200"
            >
              <div class="flex items-center gap-3">
                <Sliders class="w-4 h-4 shrink-0 text-slate-400" />
                <span>Remote Config</span>
              </div>
              <component :is="isRemoteConfigOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-500" />
            </button>

            <!-- Remote Config Sub-Menu Items -->
            <div v-show="isRemoteConfigOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-800/80 ml-5 my-0.5">
              <router-link
                to="/prometheus-config"
                :class="[
                  route.path === '/prometheus-config'
                    ? 'text-brand-400 font-semibold'
                    : 'text-slate-400 hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1 h-1 rounded-full" :class="route.path === '/prometheus-config' ? 'bg-brand-400' : 'bg-slate-600'"></span>
                <span>Prometheus Config</span>
              </router-link>

              <router-link
                to="/dataprepper-config"
                :class="[
                  route.path === '/dataprepper-config'
                    ? 'text-brand-400 font-semibold'
                    : 'text-slate-400 hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1 h-1 rounded-full" :class="route.path === '/dataprepper-config' ? 'bg-brand-400' : 'bg-slate-600'"></span>
                <span>Data Prepper Pipelines</span>
              </router-link>

              <a
                href="/remote-host"
                target="_blank"
                class="flex items-center justify-between py-1.5 px-2 rounded-md text-[11px] text-slate-400 hover:text-slate-200 transition group"
              >
                <div class="flex items-center gap-2">
                  <span class="w-1 h-1 rounded-full bg-slate-600 group-hover:bg-brand-400"></span>
                  <span>Remote Host</span>
                </div>
                <ExternalLink class="w-2.5 h-2.5 text-slate-600 group-hover:text-slate-400" />
              </a>
            </div>
          </div>

          <!-- 4. Network Topology -->
          <a
            href="/network-topology"
            target="_blank"
            class="flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 group cursor-pointer"
          >
            <div class="flex items-center gap-3">
              <Network class="w-4 h-4 shrink-0 text-slate-400 group-hover:text-brand-400 transition" />
              <span>Network Topology</span>
            </div>
            <ExternalLink class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
          </a>

          <!-- 5. OpenSearch Cluster -->
          <a
            href="/opensearch-cluster"
            target="_blank"
            class="flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 group cursor-pointer"
          >
            <div class="flex items-center gap-3">
              <Search class="w-4 h-4 shrink-0 text-slate-400 group-hover:text-brand-400 transition" />
              <span>OpenSearch Cluster</span>
            </div>
            <ExternalLink class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
          </a>

          <!-- 6. Backup Manager -->
          <router-link
            to="/backup"
            :class="[
              route.path === '/backup'
                ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-semibold'
                : 'text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Database class="w-4 h-4 shrink-0" />
            <span>Backup Manager</span>
          </router-link>

          <!-- 7. Tools (Accordion Parent) -->
          <div>
            <button
              @click="isToolsOpen = !isToolsOpen"
              class="w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-400 hover:bg-slate-900/60 hover:text-slate-200"
            >
              <div class="flex items-center gap-3">
                <Wrench class="w-4 h-4 shrink-0 text-slate-400" />
                <span>Tools</span>
              </div>
              <component :is="isToolsOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-500" />
            </button>

            <!-- Tools Sub-Menu Items -->
            <div v-show="isToolsOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-800/80 ml-5 my-0.5">
              <router-link
                to="/snmp"
                :class="[
                  route.path === '/snmp'
                    ? 'text-brand-400 font-semibold'
                    : 'text-slate-400 hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1 h-1 rounded-full" :class="route.path === '/snmp' ? 'bg-brand-400' : 'bg-slate-600'"></span>
                <span>SNMP Browser</span>
              </router-link>

              <router-link
                to="/grok-debugger"
                :class="[
                  route.path === '/grok-debugger'
                    ? 'text-brand-400 font-semibold'
                    : 'text-slate-400 hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1 h-1 rounded-full" :class="route.path === '/grok-debugger' ? 'bg-brand-400' : 'bg-slate-600'"></span>
                <span>Grok Debugger</span>
              </router-link>
            </div>
          </div>

          <!-- 8. System Settings -->
          <router-link
            to="/settings"
            :class="[
              route.path === '/settings'
                ? 'bg-brand-500/10 text-brand-400 border-brand-500/30 font-semibold'
                : 'text-slate-400 hover:bg-slate-900/60 hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Settings class="w-4 h-4 shrink-0" />
            <span>System Settings</span>
          </router-link>

        </nav>
      </div>

      <!-- User Profile & Logout -->
      <div class="p-3 border-t border-slate-800/80 bg-[#0a0d14] shrink-0">
        <div class="flex items-center justify-between px-2 py-1.5 rounded-lg">
          <div class="flex items-center gap-2.5 overflow-hidden">
            <div class="w-7 h-7 rounded-lg bg-slate-800 border border-slate-700 flex items-center justify-center font-bold text-xs text-brand-400 shrink-0">
              {{ authStore.user?.username?.charAt(0).toUpperCase() || 'A' }}
            </div>
            <div class="overflow-hidden">
              <p class="text-xs font-semibold text-white truncate">{{ authStore.user?.username || 'sysadministrator' }}</p>
              <p class="text-[10px] text-slate-500 uppercase tracking-wider font-mono">{{ authStore.user?.role || 'ADMIN' }}</p>
            </div>
          </div>

          <button
            @click="handleLogout"
            class="p-1.5 text-slate-500 hover:text-rose-400 rounded-lg hover:bg-rose-500/10 transition"
            title="Sign Out"
          >
            <LogOut class="w-4 h-4" />
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden bg-[#0e1118]">
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
  </div>
</template>
