<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import CommandPalette from '../components/CommandPalette.vue';
import ThemeToggle from '../components/ThemeToggle.vue';
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
  ChevronRight,
  ChevronDown,
  Terminal,
  Command,
} from 'lucide-vue-next';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

// Accordion states - collapsed by default, open only if current route belongs to submenu
const isRemoteConfigOpen = ref(['/prometheus-config', '/dataprepper-config'].includes(route.path));
const isToolsOpen = ref(['/snmp', '/grok-debugger', '/slideshow'].includes(route.path));

const handleLogout = async () => {
  await authStore.logout();
  router.push('/login');
};

onMounted(() => {
  authStore.fetchUser();
});
</script>

<template>
  <div class="flex h-screen bg-slate-50 dark:bg-[#090d16] text-slate-800 dark:text-slate-100 overflow-hidden font-sans">
    <!-- Command Palette (Ctrl+K) -->
    <CommandPalette />

    <!-- Sidebar -->
    <aside class="w-64 border-r border-slate-200 dark:border-[#1b2234] bg-white dark:bg-[#0c101a] flex flex-col justify-between shrink-0">
      <div class="flex-1 flex flex-col min-h-0">
        <!-- App Brand Header -->
        <div class="h-16 flex items-center px-5 border-b border-slate-200 dark:border-[#1b2234] gap-3 shrink-0">
          <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-[#293681] to-[#4274D9] flex items-center justify-center font-mono font-black text-xs text-white tracking-tighter shadow-sm">
            HCP
          </div>
          <div>
            <h1 class="font-bold text-sm tracking-wide text-slate-900 dark:text-white leading-tight">HEPHAESTUS</h1>
            <span class="text-[10px] text-blue-700 dark:text-[#95CCDD] font-mono font-semibold tracking-wider">CONTROL PANEL</span>
          </div>
        </div>

        <!-- Quick Jump / Search button -->
        <div class="px-4 py-3 shrink-0">
          <button
            @click="window?.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))"
            class="w-full flex items-center justify-between px-3 py-1.5 text-xs bg-slate-100 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] text-slate-600 dark:text-slate-400 rounded-lg hover:border-blue-500/50 hover:text-slate-900 dark:hover:text-white transition"
          >
            <span class="flex items-center gap-1.5">
              <Command class="w-3.5 h-3.5 text-blue-600 dark:text-[#95CCDD]" />
              Quick search...
            </span>
            <kbd class="text-[10px] font-mono bg-white dark:bg-[#1a2336] text-slate-700 dark:text-[#D0E7E6] px-1.5 py-0.5 rounded border border-slate-300 dark:border-[#293681]">Ctrl+K</kbd>
          </button>
        </div>

        <!-- Section Label -->
        <div class="px-4 pt-1 pb-1">
          <p class="text-[10px] uppercase font-bold text-blue-800 dark:text-[#95CCDD]/70 tracking-wider">OPERATIONAL MODULES</p>
        </div>

        <!-- Navigation Links (With Sub-Menu Accordions) -->
        <nav class="px-3 space-y-1 overflow-y-auto flex-1 select-none pr-2">
          
          <!-- 1. Overview / Dashboard -->
          <router-link
            to="/"
            :class="[
              route.path === '/'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-bold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50'
                : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent',
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
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-bold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50'
                : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Link2 class="w-4 h-4 shrink-0" />
            <span>Connections</span>
          </router-link>

          <!-- 3. Remote Server (Dedicated Top-Level Menu) -->
          <a
            href="/remote-server"
            target="_blank"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 group cursor-pointer"
          >
            <Terminal class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400 group-hover:text-blue-600 dark:group-hover:text-[#95CCDD] transition" />
            <span>Remote Server</span>
          </a>

          <!-- 4. Remote Config (Accordion Parent) -->
          <div>
            <button
              @click="isRemoteConfigOpen = !isRemoteConfigOpen"
              class="w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200"
            >
              <div class="flex items-center gap-3">
                <Sliders class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400" />
                <span>Remote Config</span>
              </div>
              <component :is="isRemoteConfigOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            </button>

            <!-- Remote Config Sub-Menu Items -->
            <div v-show="isRemoteConfigOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                to="/prometheus-config"
                :class="[
                  route.path === '/prometheus-config'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full" :class="route.path === '/prometheus-config' ? 'bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Prometheus Config</span>
              </router-link>

              <router-link
                to="/dataprepper-config"
                :class="[
                  route.path === '/dataprepper-config'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full" :class="route.path === '/dataprepper-config' ? 'bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Data Prepper Pipelines</span>
              </router-link>
            </div>
          </div>

          <!-- 4. Network Topology -->
          <a
            href="/network-topology"
            target="_blank"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 group cursor-pointer"
          >
            <Network class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400 group-hover:text-blue-600 dark:group-hover:text-[#95CCDD] transition" />
            <span>Network Topology</span>
          </a>

          <!-- 5. OpenSearch Cluster -->
          <a
            href="/opensearch-cluster"
            target="_blank"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 group cursor-pointer"
          >
            <Search class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400 group-hover:text-blue-600 dark:group-hover:text-[#95CCDD] transition" />
            <span>OpenSearch Cluster</span>
          </a>

          <!-- 6. Backup Manager -->
          <router-link
            to="/backup"
            :class="[
              route.path === '/backup'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-bold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50'
                : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent',
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
              class="w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200"
            >
              <div class="flex items-center gap-3">
                <Wrench class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400" />
                <span>Tools</span>
              </div>
              <component :is="isToolsOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            </button>

            <!-- Tools Sub-Menu Items -->
            <div v-show="isToolsOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                to="/snmp"
                :class="[
                  route.path === '/snmp'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full" :class="route.path === '/snmp' ? 'bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>SNMP Browser</span>
              </router-link>

              <router-link
                to="/grok-debugger"
                :class="[
                  route.path === '/grok-debugger'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full" :class="route.path === '/grok-debugger' ? 'bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Grok Debugger</span>
              </router-link>

              <router-link
                to="/slideshow"
                :class="[
                  route.path === '/slideshow'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full" :class="route.path === '/slideshow' ? 'bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Slide Show</span>
              </router-link>
            </div>
          </div>

          <!-- 8. System Settings -->
          <router-link
            to="/settings"
            :class="[
              route.path === '/settings'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-bold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50'
                : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Settings class="w-4 h-4 shrink-0" />
            <span>System Settings</span>
          </router-link>

        </nav>
      </div>

      <!-- User Profile & Logout -->
      <div class="p-3 border-t border-slate-200 dark:border-[#1b2234] bg-slate-50 dark:bg-[#090d16] shrink-0">
        <div class="flex items-center justify-between px-2 py-1.5 rounded-lg">
          <div class="flex items-center gap-2.5 overflow-hidden">
            <div class="w-7 h-7 rounded-lg bg-blue-100 dark:bg-[#141b2d] border border-blue-200 dark:border-[#293681] flex items-center justify-center font-bold text-xs text-blue-800 dark:text-[#95CCDD] shrink-0">
              {{ authStore.user?.username?.charAt(0).toUpperCase() || 'A' }}
            </div>
            <div class="overflow-hidden">
              <p class="text-xs font-semibold text-slate-800 dark:text-[#D0E7E6] truncate">{{ authStore.user?.username || 'sysadministrator' }}</p>
              <p class="text-[10px] text-blue-700 dark:text-[#95CCDD]/70 uppercase tracking-wider font-mono">{{ authStore.user?.role || 'ADMIN' }}</p>
            </div>
          </div>

          <div class="flex items-center gap-1">
            <ThemeToggle variant="compact" />
            <button
              @click="handleLogout"
              class="p-1.5 text-slate-400 hover:text-rose-600 rounded-lg hover:bg-rose-50 dark:hover:bg-rose-500/10 transition"
              title="Sign Out"
            >
              <LogOut class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden bg-slate-50 dark:bg-[#090d16]">
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
  </div>
</template>
