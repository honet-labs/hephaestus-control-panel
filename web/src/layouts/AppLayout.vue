<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import CommandPalette from '../components/CommandPalette.vue';
import ThemeToggle from '../components/ThemeToggle.vue';
import {
  LayoutDashboard,
  Server,
  Network,
  Sliders,
  Wrench,
  Activity,
  Settings,
  LogOut,
  ChevronRight,
  ChevronDown,
  Command,
} from 'lucide-vue-next';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

// Accordion states - collapsed by default, open only if current route belongs to submenu
const isServerOpen = ref(
  route.path.startsWith('/connections') ||
  route.path.startsWith('/remote-server') ||
  route.path.startsWith('/remote-host')
);
const isNetworkingOpen = ref(route.path.startsWith('/network-topology'));
const isRemoteConfigOpen = ref(
  route.path.startsWith('/dataprepper-config') ||
  route.path.startsWith('/prometheus-config')
);
const isToolsOpen = ref(
  route.path.startsWith('/snmp') ||
  route.path.startsWith('/grok-debugger') ||
  route.path.startsWith('/backup')
);
const isMonitoringOpen = ref(
  route.path.startsWith('/opensearch-cluster') ||
  route.path.startsWith('/slideshow')
);

// Active indicators for parent accordion headers
const isServerActive = computed(() =>
  route.path.startsWith('/connections') ||
  route.path.startsWith('/remote-server') ||
  route.path.startsWith('/remote-host')
);
const isNetworkingActive = computed(() => route.path.startsWith('/network-topology'));
const isRemoteConfigActive = computed(() =>
  route.path.startsWith('/dataprepper-config') ||
  route.path.startsWith('/prometheus-config')
);
const isToolsActive = computed(() =>
  route.path.startsWith('/snmp') ||
  route.path.startsWith('/grok-debugger') ||
  route.path.startsWith('/backup')
);
const isMonitoringActive = computed(() =>
  route.path.startsWith('/opensearch-cluster') ||
  route.path.startsWith('/slideshow')
);

watch(
  () => route.path,
  (newPath) => {
    if (newPath.startsWith('/connections') || newPath.startsWith('/remote-server') || newPath.startsWith('/remote-host')) {
      isServerOpen.value = true;
    }
    if (newPath.startsWith('/network-topology')) {
      isNetworkingOpen.value = true;
    }
    if (newPath.startsWith('/dataprepper-config') || newPath.startsWith('/prometheus-config')) {
      isRemoteConfigOpen.value = true;
    }
    if (newPath.startsWith('/snmp') || newPath.startsWith('/grok-debugger') || newPath.startsWith('/backup')) {
      isToolsOpen.value = true;
    }
    if (newPath.startsWith('/opensearch-cluster') || newPath.startsWith('/slideshow')) {
      isMonitoringOpen.value = true;
    }
  }
);

const handleLogout = async () => {
  await authStore.logout();
  router.push('/login');
};

watch(
  () => authStore.isAuthenticated,
  (isAuth) => {
    if (!isAuth) {
      router.push({ name: 'login', query: { redirect: route.fullPath } });
    }
  }
);

onMounted(() => {
  authStore.fetchUser();
});
</script>

<template>
  <div class="flex h-screen bg-slate-100 dark:bg-[#090d16] text-slate-800 dark:text-slate-100 overflow-hidden font-sans">
    <!-- Command Palette (Ctrl+K) -->
    <CommandPalette />

    <!-- Sidebar -->
    <aside class="w-64 border-r border-slate-200 dark:border-[#1b2234] bg-white dark:bg-[#0c101a] flex flex-col justify-between shrink-0 shadow-sm">
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
            class="w-full flex items-center justify-between px-3 py-1.5 text-xs bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] text-slate-700 dark:text-slate-400 rounded-lg hover:border-blue-500/50 hover:text-slate-900 dark:hover:text-white transition"
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
          <p class="text-[10px] uppercase font-bold text-slate-500 dark:text-[#95CCDD]/70 tracking-wider">OPERATIONAL MODULES</p>
        </div>

        <!-- Navigation Links (With Sub-Menu Accordions) -->
        <nav class="px-3 space-y-1 overflow-y-auto flex-1 select-none pr-2">
          
          <!-- 1. Overview -->
          <router-link
            v-if="authStore.can('dashboard', 'read')"
            to="/"
            :class="[
              route.path === '/'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-bold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50 shadow-sm'
                : 'text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <LayoutDashboard class="w-4 h-4 shrink-0 transition" :class="route.path === '/' ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-500 dark:text-slate-400'" />
            <span>Overview</span>
          </router-link>

          <!-- 2. Server (Accordion) -->
          <div v-if="authStore.can('connections', 'read') || authStore.can('remote_servers', 'read')">
            <button
              @click="isServerOpen = !isServerOpen"
              :class="[
                isServerActive
                  ? 'text-slate-900 dark:text-white font-semibold'
                  : 'text-slate-700 dark:text-slate-400',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200'
              ]"
            >
              <div class="flex items-center gap-3">
                <Server class="w-4 h-4 shrink-0 transition" :class="isServerActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-500 dark:text-slate-400'" />
                <span>Server</span>
              </div>
              <component :is="isServerOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            </button>

            <!-- Server Sub-Menu Items -->
            <div v-show="isServerOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                v-if="authStore.can('connections', 'read')"
                to="/connections"
                :class="[
                  route.path === '/connections'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/connections' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Inventory Server</span>
              </router-link>

              <a
                v-if="authStore.can('remote_servers', 'read')"
                href="/remote-server"
                target="_blank"
                :class="[
                  (route.path === '/remote-server' || route.path === '/remote-host')
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="(route.path === '/remote-server' || route.path === '/remote-host') ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Remote Server</span>
              </a>
            </div>
          </div>

          <!-- 3. Networking (Accordion) -->
          <div v-if="authStore.can('network_topology', 'read')">
            <button
              @click="isNetworkingOpen = !isNetworkingOpen"
              :class="[
                isNetworkingActive
                  ? 'text-slate-900 dark:text-white font-semibold'
                  : 'text-slate-700 dark:text-slate-400',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200'
              ]"
            >
              <div class="flex items-center gap-3">
                <Network class="w-4 h-4 shrink-0 transition" :class="isNetworkingActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-500 dark:text-slate-400'" />
                <span>Networking</span>
              </div>
              <component :is="isNetworkingOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            </button>

            <!-- Networking Sub-Menu Items -->
            <div v-show="isNetworkingOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <a
                v-if="authStore.can('network_topology', 'read')"
                href="/network-topology"
                target="_blank"
                :class="[
                  route.path === '/network-topology'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/network-topology' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Network Topology</span>
              </a>
            </div>
          </div>

          <!-- 4. Remote Config (Accordion) -->
          <div v-if="authStore.can('dataprepper_config', 'read') || authStore.can('prometheus_config', 'read')">
            <button
              @click="isRemoteConfigOpen = !isRemoteConfigOpen"
              :class="[
                isRemoteConfigActive
                  ? 'text-slate-900 dark:text-white font-semibold'
                  : 'text-slate-700 dark:text-slate-400',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200'
              ]"
            >
              <div class="flex items-center gap-3">
                <Sliders class="w-4 h-4 shrink-0 transition" :class="isRemoteConfigActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-500 dark:text-slate-400'" />
                <span>Remote Config</span>
              </div>
              <component :is="isRemoteConfigOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            </button>

            <!-- Remote Config Sub-Menu Items -->
            <div v-show="isRemoteConfigOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                v-if="authStore.can('dataprepper_config', 'read')"
                to="/dataprepper-config"
                :class="[
                  route.path === '/dataprepper-config'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/dataprepper-config' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Data Prepper Pipelines</span>
              </router-link>

              <router-link
                v-if="authStore.can('prometheus_config', 'read')"
                to="/prometheus-config"
                :class="[
                  route.path === '/prometheus-config'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/prometheus-config' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Prometheus Config</span>
              </router-link>
            </div>
          </div>

          <!-- 5. Tools (Accordion) -->
          <div v-if="authStore.can('snmp', 'read') || authStore.can('grok_debugger', 'read') || authStore.can('backup', 'read')">
            <button
              @click="isToolsOpen = !isToolsOpen"
              :class="[
                isToolsActive
                  ? 'text-slate-900 dark:text-white font-semibold'
                  : 'text-slate-700 dark:text-slate-400',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200'
              ]"
            >
              <div class="flex items-center gap-3">
                <Wrench class="w-4 h-4 shrink-0 transition" :class="isToolsActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-500 dark:text-slate-400'" />
                <span>Tools</span>
              </div>
              <component :is="isToolsOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            </button>

            <!-- Tools Sub-Menu Items -->
            <div v-show="isToolsOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                v-if="authStore.can('snmp', 'read')"
                to="/snmp"
                :class="[
                  route.path === '/snmp'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/snmp' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>SNMP Browser</span>
              </router-link>

              <router-link
                v-if="authStore.can('grok_debugger', 'read')"
                to="/grok-debugger"
                :class="[
                  route.path === '/grok-debugger'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/grok-debugger' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Grok Debugger</span>
              </router-link>

              <router-link
                v-if="authStore.can('backup', 'read')"
                to="/backup"
                :class="[
                  route.path === '/backup'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/backup' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Backup Manager</span>
              </router-link>
            </div>
          </div>

          <!-- 6. Monitoring (Accordion) -->
          <div v-if="authStore.can('opensearch', 'read') || authStore.can('slideshow', 'read')">
            <button
              @click="isMonitoringOpen = !isMonitoringOpen"
              :class="[
                isMonitoringActive
                  ? 'text-slate-900 dark:text-white font-semibold'
                  : 'text-slate-700 dark:text-slate-400',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border border-transparent hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200'
              ]"
            >
              <div class="flex items-center gap-3">
                <Activity class="w-4 h-4 shrink-0 transition" :class="isMonitoringActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-500 dark:text-slate-400'" />
                <span>Monitoring</span>
              </div>
              <component :is="isMonitoringOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            </button>

            <!-- Monitoring Sub-Menu Items -->
            <div v-show="isMonitoringOpen" class="pl-7 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <a
                v-if="authStore.can('opensearch', 'read')"
                href="/opensearch-cluster"
                target="_blank"
                :class="[
                  route.path === '/opensearch-cluster'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/opensearch-cluster' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>OpenSearch Cluster</span>
              </a>

              <router-link
                v-if="authStore.can('slideshow', 'read')"
                to="/slideshow"
                :class="[
                  route.path === '/slideshow'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-bold bg-blue-50/70 dark:bg-[#293681]/30'
                    : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/60 dark:hover:bg-[#121826]/60',
                  'flex items-center gap-2 py-1.5 px-2 rounded-md text-[11px] transition'
                ]"
              >
                <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="route.path === '/slideshow' ? 'bg-blue-600 dark:bg-[#4274D9]' : 'bg-slate-400 dark:bg-slate-600'"></span>
                <span>Slide Show</span>
              </router-link>
            </div>
          </div>

          <!-- 7. System Settings -->
          <router-link
            v-if="authStore.can('settings', 'read') || authStore.user?.role?.toUpperCase() === 'ADMIN'"
            to="/settings"
            :class="[
              route.path === '/settings'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-bold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50 shadow-sm'
                : 'text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent',
              'flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Settings class="w-4 h-4 shrink-0 transition" :class="route.path === '/settings' ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-500 dark:text-slate-400'" />
            <span>System Settings</span>
          </router-link>

        </nav>
      </div>

      <!-- User Profile & Logout -->
      <div class="p-3 border-t border-slate-200 dark:border-[#1b2234] bg-slate-50 dark:bg-[#090d16] shrink-0">
        <div class="flex items-center justify-between px-2 py-1.5 rounded-lg">
          <div class="flex items-center gap-2.5 overflow-hidden" v-if="authStore.user">
            <div class="w-7 h-7 rounded-lg bg-blue-100 dark:bg-[#141b2d] border border-blue-200 dark:border-[#293681] flex items-center justify-center font-bold text-xs text-blue-800 dark:text-[#95CCDD] shrink-0">
              {{ authStore.user?.username?.charAt(0).toUpperCase() || 'U' }}
            </div>
            <div class="overflow-hidden">
              <p class="text-xs font-semibold text-slate-800 dark:text-[#D0E7E6] truncate">{{ authStore.user?.username }}</p>
              <p class="text-[10px] text-blue-700 dark:text-[#95CCDD]/70 uppercase tracking-wider font-mono">{{ authStore.user?.role }}</p>
            </div>
          </div>
          <div class="flex items-center gap-2.5 overflow-hidden text-xs text-slate-400" v-else>
            <span class="italic">Guest</span>
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
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden bg-slate-100 dark:bg-[#090d16]">
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
  </div>
</template>
