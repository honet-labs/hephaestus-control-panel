<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import CommandPalette from '../components/CommandPalette.vue';
import ThemeToggle from '../components/ThemeToggle.vue';
import {
  LayoutDashboard,
  Link2,
  Network,
  Sliders,
  Wrench,
  Shield,
  Activity,
  Settings,
  LogOut,
  ChevronRight,
  ChevronDown,
  Command,
  Menu,
  X,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
  Boxes,
  Server,
  FileText,
} from 'lucide-vue-next';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

// Accordion states - collapsed by default, open only if current route belongs to submenu
const isInfrastructureOpen = ref(
  route.path.startsWith('/inventory-server') ||
  route.path.startsWith('/inventory') ||
  route.path.startsWith('/remote-server') ||
  route.path.startsWith('/remote-host') ||
  route.path.startsWith('/infrastructure')
);
const isServerSubOpen = ref(
  route.path.startsWith('/inventory-server') ||
  route.path.startsWith('/inventory') ||
  route.path.startsWith('/remote-server') ||
  route.path.startsWith('/remote-host')
);
const isNetworkingOpen = ref(route.path.startsWith('/network-topology'));
const isRemoteConfigOpen = ref(
  route.path.startsWith('/dataprepper-config') ||
  route.path.startsWith('/prometheus-config') ||
  route.path.startsWith('/opentelemetry-config')
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
const isSecurityOpen = ref(route.path.startsWith('/security'));
const isReportsOpen = ref(
  route.path.startsWith('/reports') ||
  route.path.startsWith('/report')
);

// Active indicators for parent accordion headers
const isInfrastructureActive = computed(() =>
  route.path.startsWith('/inventory-server') ||
  route.path.startsWith('/inventory') ||
  route.path.startsWith('/remote-server') ||
  route.path.startsWith('/remote-host') ||
  route.path.startsWith('/infrastructure')
);
const isServerSubActive = computed(() =>
  route.path.startsWith('/inventory-server') ||
  route.path.startsWith('/inventory') ||
  route.path.startsWith('/remote-server') ||
  route.path.startsWith('/remote-host')
);
const isNetworkingActive = computed(() => route.path.startsWith('/network-topology'));
const isRemoteConfigActive = computed(() =>
  route.path.startsWith('/dataprepper-config') ||
  route.path.startsWith('/prometheus-config') ||
  route.path.startsWith('/opentelemetry-config')
);
const isToolsActive = computed(() =>
  route.path.startsWith('/snmp') ||
  route.path.startsWith('/grok-debugger') ||
  route.path.startsWith('/backup')
);
const isSecurityActive = computed(() => route.path.startsWith('/security'));
const isMonitoringActive = computed(() =>
  route.path.startsWith('/opensearch-cluster') ||
  route.path.startsWith('/slideshow')
);
const isReportsActive = computed(() =>
  route.path.startsWith('/reports') ||
  route.path.startsWith('/report')
);

// Mobile & Desktop Sidebar Visibility State
const isMobileSidebarOpen = ref(false);
const isDesktopSidebarOpen = ref(true);

const toggleDesktopSidebar = () => {
  isDesktopSidebarOpen.value = !isDesktopSidebarOpen.value;
};

const triggerSearch = () => {
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }));
};

const currentRouteName = computed(() => {
  if (route.path === '/') return 'System Overview';
  if (route.path.startsWith('/connections')) return 'Add Connections';
  if (route.path.startsWith('/inventory-server') || route.path.startsWith('/inventory')) return 'Inventory Server';
  if (route.path.startsWith('/remote-server') || route.path.startsWith('/remote-host')) return 'Remote Server';
  if (route.path.startsWith('/network-topology')) return 'Network Topology';
  if (route.path.startsWith('/infrastructure')) return 'Management Containers';
  if (route.path.startsWith('/dataprepper-config')) return 'Data Prepper Pipelines';
  if (route.path.startsWith('/prometheus-config')) return 'Prometheus Config';
  if (route.path.startsWith('/opentelemetry-config')) return 'OpenTelemetry Config';
  if (route.path.startsWith('/snmp')) return 'SNMP Browser';
  if (route.path.startsWith('/grok-debugger')) return 'Grok Debugger';
  if (route.path.startsWith('/backup')) return 'Backup Manager';
  if (route.path.startsWith('/security/vaultwarden')) return 'Vaultwarden';
  if (route.path.startsWith('/opensearch-cluster')) return 'OpenSearch Cluster';
  if (route.path.startsWith('/slideshow')) return 'Slide Show';
  if (route.path === '/reports/raw') return 'Raw Data Report';
  if (route.path.startsWith('/reports') || route.path.startsWith('/report')) return 'Visual Reports';
  if (route.path.startsWith('/settings')) return 'System Settings';
  return 'Dashboard';
});

watch(
  () => route.path,
  (newPath) => {
    isMobileSidebarOpen.value = false;
    if (
      newPath.startsWith('/inventory-server') ||
      newPath.startsWith('/inventory') ||
      newPath.startsWith('/remote-server') ||
      newPath.startsWith('/remote-host')
    ) {
      isInfrastructureOpen.value = true;
      isServerSubOpen.value = true;
    } else if (newPath.startsWith('/infrastructure')) {
      isInfrastructureOpen.value = true;
    }
    if (newPath.startsWith('/network-topology')) {
      isNetworkingOpen.value = true;
    }
    if (newPath.startsWith('/dataprepper-config') || newPath.startsWith('/prometheus-config') || newPath.startsWith('/opentelemetry-config')) {
      isRemoteConfigOpen.value = true;
    }
    if (newPath.startsWith('/snmp') || newPath.startsWith('/grok-debugger') || newPath.startsWith('/backup')) {
      isToolsOpen.value = true;
    }
    if (newPath.startsWith('/security')) {
      isSecurityOpen.value = true;
    }
    if (newPath.startsWith('/opensearch-cluster') || newPath.startsWith('/slideshow')) {
      isMonitoringOpen.value = true;
    }
    if (newPath.startsWith('/reports') || newPath.startsWith('/report')) {
      isReportsOpen.value = true;
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

const handleKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isMobileSidebarOpen.value) {
    isMobileSidebarOpen.value = false;
  }
};

onMounted(() => {
  authStore.fetchUser();
  window.addEventListener('keydown', handleKeyDown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div class="flex h-screen bg-slate-100 dark:bg-[#090d16] text-slate-800 dark:text-slate-100 overflow-hidden font-sans relative">
    <!-- Command Palette (Ctrl+K) -->
    <CommandPalette />

    <!-- Mobile Backdrop Overlay -->
    <Transition
      enter-active-class="transition-opacity duration-300 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-200 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="isMobileSidebarOpen"
        @click="isMobileSidebarOpen = false"
        class="fixed inset-0 z-40 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs md:hidden"
        aria-hidden="true"
      ></div>
    </Transition>

    <!-- Sidebar (Off-canvas drawer on mobile, static collapsible on desktop) -->
    <aside
      :class="[
        'fixed inset-y-0 left-0 z-50 w-72 md:w-64 bg-white dark:bg-[#0c101a] border-r border-slate-200 dark:border-[#1b2234] flex flex-col justify-between shrink-0 shadow-2xl md:shadow-sm transition-all duration-300 ease-in-out md:static',
        isMobileSidebarOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
        !isDesktopSidebarOpen ? 'md:-ml-64' : ''
      ]"
    >
      <div class="flex-1 flex flex-col min-h-0">
        <!-- App Brand Header -->
        <div class="h-14 md:h-16 flex items-center justify-between px-5 border-b border-slate-200 dark:border-[#1b2234] shrink-0">
          <div class="flex items-center gap-3">
            <div class="w-8 h-8 md:w-9 md:h-9 rounded-xl bg-gradient-to-tr from-[#293681] to-[#4274D9] flex items-center justify-center font-mono font-black text-xs text-white tracking-tighter shadow-sm">
              HCP
            </div>
            <div>
              <h1 class="font-bold text-sm tracking-wide text-slate-900 dark:text-white leading-tight">HEPHAESTUS</h1>
              <span class="text-[10px] text-blue-700 dark:text-[#95CCDD] font-mono font-semibold tracking-wider">CONTROL PANEL</span>
            </div>
          </div>

          <!-- Mobile Close Button (X) -->
          <button
            @click="isMobileSidebarOpen = false"
            class="md:hidden p-1.5 text-slate-400 hover:text-slate-700 dark:hover:text-white rounded-lg hover:bg-slate-100 dark:hover:bg-[#1a2336] transition cursor-pointer"
            aria-label="Close Sidebar"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Quick Jump / Search button in sidebar -->
        <div class="px-4 py-3 shrink-0">
          <button
            @click="triggerSearch"
            class="w-full flex items-center justify-between px-3 py-1.5 text-xs bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] text-slate-700 dark:text-slate-400 rounded-lg hover:border-slate-400/50 hover:text-slate-900 dark:hover:text-white transition cursor-pointer"
          >
            <span class="flex items-center gap-1.5">
              <Command class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
              Quick search...
            </span>
            <kbd class="text-[10px] font-mono bg-white dark:bg-[#1a2336] text-slate-600 dark:text-slate-400 px-1.5 py-0.5 rounded border border-slate-300 dark:border-slate-700">Ctrl+K</kbd>
          </button>
        </div>

        <!-- Section Label -->
        <div class="px-4 pt-1 pb-1">
          <p class="text-[10px] uppercase font-bold text-slate-400 dark:text-slate-500 tracking-wider">OPERATIONAL MODULES</p>
        </div>

        <!-- Navigation Links (With Sub-Menu Accordions) -->
        <nav class="px-3 space-y-1 overflow-y-auto flex-1 select-none pr-2">
          
          <!-- 1. Overview -->
          <router-link
            v-if="authStore.can('dashboard', 'read')"
            to="/"
            :class="[
              route.path === '/'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-semibold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50 shadow-xs'
                : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
              'w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <LayoutDashboard class="w-4 h-4 shrink-0 transition" :class="route.path === '/' ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            <span>Overview</span>
          </router-link>

          <!-- 2. Connections -->
          <router-link
            v-if="authStore.can('connections', 'read')"
            to="/connections"
            :class="[
              route.path === '/connections'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-semibold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50 shadow-xs'
                : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
              'w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Link2 class="w-4 h-4 shrink-0 transition" :class="route.path === '/connections' ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            <span>Connections</span>
          </router-link>

          <!-- 3. Infrastructure (Accordion) -->
          <div v-if="authStore.can('infrastructure', 'read') || authStore.can('connections', 'read') || authStore.can('remote_servers', 'read')">
            <button
              @click="isInfrastructureOpen = !isInfrastructureOpen"
              :class="[
                isInfrastructureActive
                  ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/70 dark:bg-[#293681]/30 border-blue-200 dark:border-[#4274D9]/40 shadow-xs'
                  : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border cursor-pointer'
              ]"
            >
              <div class="flex items-center gap-3">
                <Boxes class="w-4 h-4 shrink-0 transition" :class="isInfrastructureActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                <span>Infrastructure</span>
              </div>
              <component :is="isInfrastructureOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 transition" :class="isInfrastructureActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            </button>

            <!-- Infrastructure Sub-Menu Items -->
            <div v-show="isInfrastructureOpen" class="pl-3 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <!-- Sub-group: Server (Collapsible Sub-menu) -->
              <div v-if="authStore.can('connections', 'read') || authStore.can('remote_servers', 'read')">
                <button
                  @click="isServerSubOpen = !isServerSubOpen"
                  :class="[
                    isServerSubActive
                      ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/50 dark:bg-[#293681]/20'
                      : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                    'w-full flex items-center justify-between py-1.5 px-2 rounded-md text-[11px] font-medium transition cursor-pointer'
                  ]"
                >
                  <div class="flex items-center gap-2">
                    <Server class="w-3.5 h-3.5 shrink-0 transition" :class="isServerSubActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                    <span>Server</span>
                  </div>
                  <component :is="isServerSubOpen ? ChevronDown : ChevronRight" class="w-3 h-3 transition" :class="isServerSubActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                </button>

                <!-- Sub-sub-menu items under Server -->
                <div v-show="isServerSubOpen" class="pl-3 py-0.5 space-y-0.5 border-l border-slate-200 dark:border-[#1b2234] ml-3.5 my-0.5">
                  <router-link
                    v-if="authStore.can('connections', 'read')"
                    to="/inventory-server"
                    :class="[
                      (route.path === '/inventory-server' || route.path === '/inventory')
                        ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                        : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                      'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                    ]"
                  >
                    <span>Inventory Server</span>
                  </router-link>

                  <a
                    v-if="authStore.can('remote_servers', 'read')"
                    href="/remote-server"
                    target="_blank"
                    @click="isMobileSidebarOpen = false"
                    :class="[
                      (route.path === '/remote-server' || route.path === '/remote-host')
                        ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                        : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                      'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                    ]"
                  >
                    <span>Remote Server</span>
                  </a>
                </div>
              </div>

              <!-- Management Containers -->
              <a
                v-if="authStore.can('infrastructure', 'read')"
                href="/infrastructure/containers"
                target="_blank"
                @click="isMobileSidebarOpen = false"
                :class="[
                  route.path.startsWith('/infrastructure/containers')
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition cursor-pointer'
                ]"
              >
                <span>Management Containers</span>
              </a>
            </div>
          </div>

          <!-- 4. Networking (Accordion) -->
          <div v-if="authStore.can('network_topology', 'read')">
            <button
              @click="isNetworkingOpen = !isNetworkingOpen"
              :class="[
                isNetworkingActive
                  ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/70 dark:bg-[#293681]/30 border-blue-200 dark:border-[#4274D9]/40 shadow-xs'
                  : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border'
              ]"
            >
              <div class="flex items-center gap-3">
                <Network class="w-4 h-4 shrink-0 transition" :class="isNetworkingActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                <span>Networking</span>
              </div>
              <component :is="isNetworkingOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 transition" :class="isNetworkingActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            </button>

            <!-- Networking Sub-Menu Items -->
            <div v-show="isNetworkingOpen" class="pl-4 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <a
                v-if="authStore.can('network_topology', 'read')"
                href="/network-topology"
                target="_blank"
                @click="isMobileSidebarOpen = false"
                :class="[
                  route.path === '/network-topology'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>Network Topology</span>
              </a>
            </div>
          </div>

          <!-- 6. Remote Config (Accordion) -->
          <div v-if="authStore.can('dataprepper_config', 'read') || authStore.can('prometheus_config', 'read') || authStore.can('opentelemetry_config', 'read')">
            <button
              @click="isRemoteConfigOpen = !isRemoteConfigOpen"
              :class="[
                isRemoteConfigActive
                  ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/70 dark:bg-[#293681]/30 border-blue-200 dark:border-[#4274D9]/40 shadow-xs'
                  : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border'
              ]"
            >
              <div class="flex items-center gap-3">
                <Sliders class="w-4 h-4 shrink-0 transition" :class="isRemoteConfigActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                <span>Remote Config</span>
              </div>
              <component :is="isRemoteConfigOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 transition" :class="isRemoteConfigActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            </button>

            <!-- Remote Config Sub-Menu Items -->
            <div v-show="isRemoteConfigOpen" class="pl-4 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                v-if="authStore.can('dataprepper_config', 'read')"
                to="/dataprepper-config"
                :class="[
                  route.path === '/dataprepper-config'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>Data Prepper Pipelines</span>
              </router-link>

              <router-link
                v-if="authStore.can('prometheus_config', 'read')"
                to="/prometheus-config"
                :class="[
                  route.path === '/prometheus-config'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>Prometheus Config</span>
              </router-link>

              <router-link
                v-if="authStore.can('opentelemetry_config', 'read')"
                to="/opentelemetry-config"
                :class="[
                  route.path === '/opentelemetry-config'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>OpenTelemetry Config</span>
              </router-link>
            </div>
          </div>

          <!-- 6. Tools (Accordion) -->
          <div v-if="authStore.can('snmp', 'read') || authStore.can('grok_debugger', 'read') || authStore.can('backup', 'read')">
            <button
              @click="isToolsOpen = !isToolsOpen"
              :class="[
                isToolsActive
                  ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/70 dark:bg-[#293681]/30 border-blue-200 dark:border-[#4274D9]/40 shadow-xs'
                  : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border'
              ]"
            >
              <div class="flex items-center gap-3">
                <Wrench class="w-4 h-4 shrink-0 transition" :class="isToolsActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                <span>Tools</span>
              </div>
              <component :is="isToolsOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 transition" :class="isToolsActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            </button>

            <!-- Tools Sub-Menu Items -->
            <div v-show="isToolsOpen" class="pl-4 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                v-if="authStore.can('snmp', 'read')"
                to="/snmp"
                :class="[
                  route.path === '/snmp'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>SNMP Browser</span>
              </router-link>

              <router-link
                v-if="authStore.can('grok_debugger', 'read')"
                to="/grok-debugger"
                :class="[
                  route.path === '/grok-debugger'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>Grok Debugger</span>
              </router-link>

              <router-link
                v-if="authStore.can('backup', 'read')"
                to="/backup"
                :class="[
                  route.path === '/backup'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>Backup Manager</span>
              </router-link>
            </div>
          </div>

          <!-- 7. Security (Accordion) -->
          <div v-if="authStore.can('security', 'read')">
            <button
              @click="isSecurityOpen = !isSecurityOpen"
              :class="[
                isSecurityActive
                  ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/70 dark:bg-[#293681]/30 border-blue-200 dark:border-[#4274D9]/40 shadow-xs'
                  : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border cursor-pointer'
              ]"
            >
              <div class="flex items-center gap-3">
                <Shield class="w-4 h-4 shrink-0 transition" :class="isSecurityActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                <span>Security</span>
              </div>
              <component :is="isSecurityOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 transition" :class="isSecurityActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            </button>

            <!-- Security Sub-Menu Items -->
            <div v-show="isSecurityOpen" class="pl-4 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <a
                v-if="authStore.can('security', 'read')"
                href="/security/vaultwarden"
                target="_blank"
                @click="isMobileSidebarOpen = false"
                :class="[
                  route.path.startsWith('/security/vaultwarden')
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition cursor-pointer'
                ]"
              >
                <span>Vaultwarden</span>
              </a>
            </div>
          </div>

          <!-- 8. Monitoring (Accordion) -->
          <div v-if="authStore.can('opensearch', 'read') || authStore.can('slideshow', 'read')">
            <button
              @click="isMonitoringOpen = !isMonitoringOpen"
              :class="[
                isMonitoringActive
                  ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/70 dark:bg-[#293681]/30 border-blue-200 dark:border-[#4274D9]/40 shadow-xs'
                  : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border'
              ]"
            >
              <div class="flex items-center gap-3">
                <Activity class="w-4 h-4 shrink-0 transition" :class="isMonitoringActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                <span>Monitoring</span>
              </div>
              <component :is="isMonitoringOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 transition" :class="isMonitoringActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            </button>

            <!-- Monitoring Sub-Menu Items -->
            <div v-show="isMonitoringOpen" class="pl-4 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <a
                v-if="authStore.can('opensearch', 'read')"
                href="/opensearch-cluster"
                target="_blank"
                @click="isMobileSidebarOpen = false"
                :class="[
                  route.path === '/opensearch-cluster'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>OpenSearch Cluster</span>
              </a>

              <router-link
                v-if="authStore.can('slideshow', 'read')"
                to="/slideshow"
                :class="[
                  route.path === '/slideshow'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition'
                ]"
              >
                <span>Slide Show</span>
              </router-link>
            </div>
          </div>

          <!-- 9. Report (Accordion) -->
          <div v-if="authStore.can('reports', 'read')">
            <button
              @click="isReportsOpen = !isReportsOpen"
              :class="[
                isReportsActive
                  ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50/70 dark:bg-[#293681]/30 border-blue-200 dark:border-[#4274D9]/40 shadow-xs'
                  : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
                'w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs tracking-wide transition border cursor-pointer'
              ]"
            >
              <div class="flex items-center gap-3">
                <FileText class="w-4 h-4 shrink-0 transition" :class="isReportsActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
                <span>Report</span>
              </div>
              <component :is="isReportsOpen ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 transition" :class="isReportsActive ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
            </button>

            <!-- Report Sub-Menu Items -->
            <div v-show="isReportsOpen" class="pl-4 pr-1 py-1 space-y-1 border-l border-slate-200 dark:border-[#1b2234] ml-5 my-0.5">
              <router-link
                to="/reports"
                :class="[
                  route.path === '/reports' || route.path === '/reports/visual'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition cursor-pointer'
                ]"
              >
                <span>Visual Report</span>
              </router-link>

              <router-link
                to="/reports/raw"
                :class="[
                  route.path === '/reports/raw'
                    ? 'text-blue-700 dark:text-[#95CCDD] font-semibold bg-blue-50 dark:bg-[#293681]/30'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100/70 dark:hover:bg-[#121826]/70',
                  'flex items-center py-1.5 px-2 rounded-md text-[11px] font-medium transition cursor-pointer'
                ]"
              >
                <span>Raw Report</span>
              </router-link>
            </div>
          </div>

          <!-- 10. System Settings -->
          <router-link
            v-if="authStore.can('settings', 'read') || authStore.user?.role?.toUpperCase() === 'ADMIN'"
            to="/settings"
            :class="[
              route.path === '/settings'
                ? 'bg-blue-50 text-blue-700 border-blue-200 font-semibold dark:bg-[#293681]/40 dark:text-[#95CCDD] dark:border-[#4274D9]/50 shadow-xs'
                : 'bg-transparent text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-[#121826] hover:text-slate-900 dark:hover:text-slate-200 border-transparent font-medium',
              'w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs tracking-wide transition border'
            ]"
          >
            <Settings class="w-4 h-4 shrink-0 transition" :class="route.path === '/settings' ? 'text-blue-600 dark:text-[#95CCDD]' : 'text-slate-400 dark:text-slate-500'" />
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
      <!-- Sticky / Top Navigation Header -->
      <header class="h-14 bg-white dark:bg-[#0c101a] border-b border-slate-200 dark:border-[#1b2234] flex items-center justify-between px-3 sm:px-5 shrink-0 z-30 shadow-xs">
        <!-- Left: Mobile Menu Trigger / Desktop Sidebar Collapse & Page Indicator -->
        <div class="flex items-center gap-2.5 sm:gap-3 min-w-0">
          <!-- Mobile Hamburger Toggle -->
          <button
            @click="isMobileSidebarOpen = true"
            class="md:hidden p-2 -ml-1 text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#151b2a] rounded-lg transition cursor-pointer"
            aria-label="Open Navigation Menu"
          >
            <Menu class="w-5 h-5" />
          </button>

          <!-- Desktop Sidebar Collapse/Expand Toggle -->
          <button
            @click="toggleDesktopSidebar"
            class="hidden md:flex items-center justify-center p-1.5 text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#151b2a] rounded-lg transition cursor-pointer"
            :title="isDesktopSidebarOpen ? 'Hide Sidebar' : 'Show Sidebar'"
          >
            <PanelLeftClose v-if="isDesktopSidebarOpen" class="w-4 h-4" />
            <PanelLeftOpen v-else class="w-4 h-4" />
          </button>

          <!-- Mobile Active Section Title -->
          <div class="flex md:hidden items-center gap-2 min-w-0">
            <span class="font-bold text-xs text-slate-900 dark:text-white truncate">
              {{ currentRouteName }}
            </span>
          </div>

          <!-- Desktop Breadcrumb / Active Section Title -->
          <div class="hidden md:flex items-center gap-2 text-xs font-semibold text-slate-700 dark:text-slate-200">
            <span>{{ currentRouteName }}</span>
          </div>
        </div>

        <!-- Right Header Actions -->
        <div class="flex items-center gap-1.5 sm:gap-2 shrink-0">
          <!-- Quick Search Button -->
          <button
            @click="triggerSearch"
            class="flex items-center gap-1.5 px-2 sm:px-2.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white bg-slate-50 dark:bg-[#121826] hover:bg-slate-100 dark:hover:bg-[#1a2336] rounded-lg border border-slate-200 dark:border-[#1b2234] transition cursor-pointer"
            title="Quick Search (Ctrl+K)"
          >
            <Search class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
            <span class="hidden sm:inline text-[11px]">Quick Search</span>
            <kbd class="hidden sm:inline text-[9px] font-mono bg-white dark:bg-[#1a2336] px-1 py-0.5 rounded border border-slate-300 dark:border-slate-700 text-slate-500 dark:text-slate-400">Ctrl+K</kbd>
          </button>

          <!-- Theme Toggle -->
          <ThemeToggle variant="compact" />

          <!-- Mobile Sign Out Button -->
          <button
            @click="handleLogout"
            class="md:hidden p-1.5 text-slate-400 hover:text-rose-600 rounded-lg hover:bg-rose-50 dark:hover:bg-rose-500/10 transition cursor-pointer"
            title="Sign Out"
          >
            <LogOut class="w-4 h-4" />
          </button>
        </div>
      </header>

      <!-- Main Content with responsive mobile padding -->
      <main class="flex-1 overflow-y-auto p-3.5 sm:p-5 md:p-6 w-full max-w-full">
        <router-view />
      </main>
    </div>
  </div>
</template>
