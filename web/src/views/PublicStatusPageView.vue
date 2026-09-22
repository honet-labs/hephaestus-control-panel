<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import axios from 'axios';
import {
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Clock,
  RefreshCw,
  Lock,
  Moon,
  Sun,
  Shield,
  Layers,
  Activity,
  Server,
  Network,
  Database,
  BarChart2,
  ArrowRight,
} from 'lucide-vue-next';

const route = useRoute();
const router = useRouter();

interface StatusItemLiveResult {
  itemId: string;
  name: string;
  groupId?: string | null;
  sourceType: string;
  sourceId?: string | null;
  status: string; // "operational" | "degraded" | "down" | "maintenance" | "unknown"
  latencyMs?: number | null;
  message?: string;
  description?: string;
  checkedAt: string;
}

interface StatusPageGroupReport {
  id: string;
  name: string;
  sortOrder: number;
  items: StatusItemLiveResult[];
}

interface StatusPageIncident {
  id: string;
  pageId: string;
  title: string;
  status: string;
  severity: string;
  message: string;
  isActive: boolean;
  createdAt: string;
}

interface StatusPageLiveReport {
  pageId: string;
  title: string;
  slug: string;
  description: string;
  footerText: string;
  theme: string;
  refreshInterval: number;
  isPublic: boolean;
  overallStatus: string; // "operational" | "partial_outage" | "major_outage" | "maintenance"
  overallMessage: string;
  activeIncidents: StatusPageIncident[];
  groups: StatusPageGroupReport[];
  ungroupedItems: StatusItemLiveResult[];
  lastChecked: string;
}

const report = ref<StatusPageLiveReport | null>(null);
const loading = ref(true);
const refreshing = ref(false);
const error = ref<string | null>(null);
const requiresAuth = ref(false);

// Auth login form state if private
const loginUsername = ref('');
const loginPassword = ref('');
const loginLoading = ref(false);
const loginError = ref<string | null>(null);

// Countdown timer
const countdown = ref(60);
let timerId: any = null;

// Dark/Light mode toggle
const isDark = ref(true);
const toggleTheme = () => {
  isDark.value = !isDark.value;
  if (isDark.value) {
    document.documentElement.classList.add('dark');
  } else {
    document.documentElement.classList.remove('dark');
  }
};

const fetchStatus = async (isManual = false) => {
  if (isManual) refreshing.value = true;
  const slug = route.params.slug || 'default';

  try {
    const res = await axios.get(`/api/v1/status-pages/public/${slug}`);
    if (res.data?.success) {
      report.value = res.data.data;
      requiresAuth.value = false;
      error.value = null;
      countdown.value = report.value?.refreshInterval || 60;
    }
  } catch (err: any) {
    if (err.response?.status === 401 && err.response?.data?.requiresAuth) {
      requiresAuth.value = true;
      report.value = {
        title: err.response.data.title || 'Private Status Page',
        slug: err.response.data.slug || (slug as string),
      } as any;
    } else {
      error.value = err.response?.data?.message || err.response?.data?.error || 'Failed to load status page';
    }
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
};

const handleLogin = async () => {
  if (!loginUsername.value || !loginPassword.value) {
    loginError.value = 'Username and password are required';
    return;
  }
  loginLoading.value = true;
  loginError.value = null;
  try {
    const res = await axios.post('/api/v1/auth/login', {
      username: loginUsername.value,
      password: loginPassword.value,
    });
    if (res.data?.success) {
      axios.defaults.headers.common['Authorization'] = `Bearer ${res.data.data.token}`;
      requiresAuth.value = false;
      await fetchStatus();
    }
  } catch (err: any) {
    loginError.value = err.response?.data?.error || 'Login failed. Please check your username and password.';
  } finally {
    loginLoading.value = false;
  }
};

// Overall Status Banner Styles & Icons
const overallBannerClass = computed(() => {
  if (!report.value) return '';
  switch (report.value.overallStatus) {
    case 'operational':
      return 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400';
    case 'partial_outage':
      return 'bg-amber-500/10 border-amber-500/30 text-amber-600 dark:text-amber-400';
    case 'major_outage':
      return 'bg-rose-500/10 border-rose-500/30 text-rose-600 dark:text-rose-400';
    case 'maintenance':
      return 'bg-blue-500/10 border-blue-500/30 text-blue-600 dark:text-blue-400';
    default:
      return 'bg-slate-500/10 border-slate-500/30 text-slate-500';
  }
});

const getStatusColor = (status: string) => {
  switch (status.toLowerCase()) {
    case 'operational':
      return 'bg-emerald-500';
    case 'degraded':
      return 'bg-amber-500';
    case 'down':
      return 'bg-rose-500';
    case 'maintenance':
      return 'bg-blue-500';
    default:
      return 'bg-slate-400';
  }
};

const getStatusBadge = (status: string) => {
  switch (status.toLowerCase()) {
    case 'operational':
      return { text: 'Operational', class: 'text-emerald-600 dark:text-emerald-400 bg-emerald-500/10' };
    case 'degraded':
      return { text: 'Degraded', class: 'text-amber-600 dark:text-amber-400 bg-amber-500/10' };
    case 'down':
      return { text: 'Down', class: 'text-rose-600 dark:text-rose-400 bg-rose-500/10' };
    case 'maintenance':
      return { text: 'Maintenance', class: 'text-blue-600 dark:text-blue-400 bg-blue-500/10' };
    default:
      return { text: 'Unknown', class: 'text-slate-500 bg-slate-500/10' };
  }
};

const totalOperationalCount = computed(() => {
  if (!report.value) return { op: 0, total: 0 };
  let total = 0;
  let op = 0;
  report.value.groups?.forEach((g) => {
    g.items?.forEach((it) => {
      total++;
      if (it.status === 'operational') op++;
    });
  });
  report.value.ungroupedItems?.forEach((it) => {
    total++;
    if (it.status === 'operational') op++;
  });
  return { op, total };
});

const startCountdownTimer = () => {
  timerId = setInterval(() => {
    if (countdown.value > 1) {
      countdown.value--;
    } else {
      countdown.value = report.value?.refreshInterval || 60;
      fetchStatus();
    }
  }, 1000);
};

onMounted(() => {
  // Sync initial dark theme
  isDark.value = document.documentElement.classList.contains('dark');
  fetchStatus();
  startCountdownTimer();
});

onUnmounted(() => {
  if (timerId) clearInterval(timerId);
});
</script>

<template>
  <div class="min-h-screen bg-slate-50 dark:bg-[#090d16] text-slate-900 dark:text-slate-100 font-sans transition-colors duration-300">
    <!-- Top Minimal Navigation -->
    <header class="border-b border-slate-200 dark:border-[#1a2133] bg-white/80 dark:bg-[#0e1322]/80 backdrop-blur-md sticky top-0 z-30">
      <div class="max-w-4xl mx-auto px-4 sm:px-6 py-3.5 flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-xl bg-slate-900 dark:bg-slate-800 text-white flex items-center justify-center shadow-xs">
            <Activity class="w-4 h-4" />
          </div>
          <div>
            <span class="text-xs font-bold tracking-wider text-slate-500 dark:text-slate-400 uppercase">HEPHAESTUS</span>
            <h1 class="text-sm font-bold text-slate-900 dark:text-white leading-tight">
              {{ report?.title || 'System Status' }}
            </h1>
          </div>
        </div>

        <!-- Controls: Refresh + Theme Toggle -->
        <div class="flex items-center gap-2">
          <button
            @click="fetchStatus(true)"
            :disabled="refreshing"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-800 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer disabled:opacity-50"
            title="Refresh data now"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': refreshing }" />
            <span class="hidden sm:inline">Refresh</span>
          </button>
          <button
            @click="toggleTheme"
            class="p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
            title="Toggle Dark / Light Mode"
          >
            <Sun v-if="isDark" class="w-4 h-4" />
            <Moon v-else class="w-4 h-4" />
          </button>
        </div>
      </div>
    </header>

    <!-- Main Content Container -->
    <main class="max-w-4xl mx-auto px-4 sm:px-6 py-8 space-y-8">
      <!-- Loading State -->
      <div v-if="loading" class="py-20 text-center space-y-3">
        <RefreshCw class="w-8 h-8 animate-spin text-blue-500 mx-auto" />
        <p class="text-xs text-slate-500">Checking infrastructure availability status...</p>
      </div>

      <!-- Private Status Page Auth Guard -->
      <div v-else-if="requiresAuth" class="max-w-md mx-auto py-12">
        <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-6 sm:p-8 shadow-xl space-y-6 text-center">
          <div class="w-14 h-14 rounded-full bg-amber-500/10 text-amber-500 flex items-center justify-center mx-auto">
            <Lock class="w-6 h-6" />
          </div>
          <div class="space-y-1">
            <h2 class="text-base font-bold text-slate-900 dark:text-white">{{ report?.title || 'Private Status Page' }}</h2>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              This status page is private. Please sign in with your account to view service performance.
            </p>
          </div>

          <form @submit.prevent="handleLogin" class="space-y-4 text-left">
            <div v-if="loginError" class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-xs text-rose-500">
              {{ loginError }}
            </div>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Username</label>
              <input
                v-model="loginUsername"
                type="text"
                required
                placeholder="Account username"
                class="w-full px-3.5 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden focus:border-blue-500"
              />
            </div>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Password</label>
              <input
                v-model="loginPassword"
                type="password"
                required
                placeholder="Password"
                class="w-full px-3.5 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden focus:border-blue-500"
              />
            </div>

            <button
              type="submit"
              :disabled="loginLoading"
              class="w-full flex items-center justify-center gap-2 py-2.5 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-bold transition cursor-pointer shadow-xs disabled:opacity-50"
            >
              <span v-if="loginLoading">Authenticating...</span>
              <span v-else>Sign In & View Status</span>
              <ArrowRight v-if="!loginLoading" class="w-3.5 h-3.5" />
            </button>
          </form>
        </div>
      </div>

      <!-- Error State -->
      <div v-else-if="error" class="py-16 text-center space-y-4">
        <AlertTriangle class="w-10 h-10 text-amber-500 mx-auto" />
        <h2 class="text-base font-bold text-slate-800 dark:text-slate-200">{{ error }}</h2>
        <p class="text-xs text-slate-500">Please make sure the status page slug URL is correct.</p>
        <button
          @click="fetchStatus(true)"
          class="px-4 py-2 bg-blue-600 text-white rounded-lg text-xs font-bold cursor-pointer"
        >
          Try Again
        </button>
      </div>

      <!-- Normal Status Page View -->
      <template v-else-if="report">
        <!-- Overall Status Banner -->
        <div
          class="p-5 sm:p-6 rounded-2xl border flex flex-col sm:flex-row sm:items-center justify-between gap-4 transition-all shadow-xs"
          :class="overallBannerClass"
        >
          <div class="flex items-center gap-3.5">
            <CheckCircle2 v-if="report.overallStatus === 'operational'" class="w-8 h-8 shrink-0" />
            <AlertTriangle v-else-if="report.overallStatus === 'partial_outage'" class="w-8 h-8 shrink-0" />
            <XCircle v-else-if="report.overallStatus === 'major_outage'" class="w-8 h-8 shrink-0" />
            <Sliders v-else class="w-8 h-8 shrink-0" />

            <div>
              <h2 class="text-lg font-bold leading-tight">
                {{ report.overallMessage }}
              </h2>
              <p class="text-xs opacity-80 mt-0.5">
                {{ totalOperationalCount.op }} of {{ totalOperationalCount.total }} services operating normally
              </p>
            </div>
          </div>

          <!-- Countdown Timer Chip -->
          <div class="flex items-center gap-2 self-start sm:self-auto text-xs opacity-75 shrink-0 bg-white/10 dark:bg-black/20 px-3 py-1.5 rounded-xl">
            <Clock class="w-3.5 h-3.5" />
            <span>Refreshes in {{ countdown }}s</span>
          </div>
        </div>

        <!-- Description Note -->
        <div v-if="report.description" class="p-4 rounded-xl bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
          {{ report.description }}
        </div>

        <!-- Active Incidents Banner -->
        <div v-if="report.activeIncidents && report.activeIncidents.length > 0" class="space-y-3">
          <h3 class="text-xs font-bold uppercase tracking-wider text-slate-400 flex items-center gap-2">
            <AlertTriangle class="w-3.5 h-3.5 text-amber-500" />
            <span>Active Incident Notice</span>
          </h3>

          <div
            v-for="inc in report.activeIncidents"
            :key="inc.id"
            class="p-4 rounded-2xl border border-amber-500/40 bg-amber-500/5 space-y-2"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <span class="px-2 py-0.5 rounded-full text-[10px] font-bold uppercase bg-amber-500/20 text-amber-600 dark:text-amber-400">
                  {{ inc.severity }}
                </span>
                <h4 class="text-sm font-bold text-slate-900 dark:text-white">{{ inc.title }}</h4>
              </div>
              <span class="text-[11px] text-slate-400 capitalize">{{ inc.status }}</span>
            </div>
            <p class="text-xs text-slate-700 dark:text-slate-300 leading-relaxed">{{ inc.message }}</p>
          </div>
        </div>

        <!-- Grouped Services -->
        <div class="space-y-6">
          <div
            v-for="group in report.groups"
            :key="group.id"
            class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl overflow-hidden shadow-xs"
          >
            <!-- Group Header -->
            <div class="px-5 py-3.5 bg-slate-50/70 dark:bg-[#151b2c] border-b border-slate-200 dark:border-[#1f283d] flex items-center justify-between">
              <div class="flex items-center gap-2.5">
                <Layers class="w-4 h-4 text-slate-400" />
                <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">{{ group.name }}</h3>
              </div>
              <span class="text-[11px] text-slate-500 font-medium">
                {{ group.items.filter(i => i.status === 'operational').length }}/{{ group.items.length }} Operational
              </span>
            </div>

            <!-- Service Items List -->
            <div class="divide-y divide-slate-100 dark:divide-[#1a2133]">
              <div
                v-for="item in group.items"
                :key="item.itemId"
                class="px-5 py-3.5 flex items-center justify-between gap-4 hover:bg-slate-50/50 dark:hover:bg-slate-800/20 transition"
              >
                <div class="flex items-center gap-3">
                  <span class="w-2.5 h-2.5 rounded-full shrink-0" :class="getStatusColor(item.status)"></span>
                  <div>
                    <h4 class="text-xs font-bold text-slate-800 dark:text-slate-200">{{ item.name }}</h4>
                    <p v-if="item.description" class="text-[11px] text-slate-400 mt-0.5">{{ item.description }}</p>
                  </div>
                </div>

                <div class="flex items-center gap-3 shrink-0">
                  <span
                    v-if="item.latencyMs !== undefined && item.latencyMs !== null"
                    class="text-[11px] text-slate-400 font-mono"
                  >
                    {{ item.latencyMs.toFixed(0) }}ms
                  </span>
                  <span
                    class="px-2.5 py-1 rounded-full text-[10px] font-bold"
                    :class="getStatusBadge(item.status).class"
                  >
                    {{ getStatusBadge(item.status).text }}
                  </span>
                </div>
              </div>

              <div v-if="group.items.length === 0" class="p-4 text-center text-xs text-slate-400">
                No services in this group.
              </div>
            </div>
          </div>

          <!-- Ungrouped Services -->
          <div
            v-if="report.ungroupedItems && report.ungroupedItems.length > 0"
            class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl overflow-hidden shadow-xs"
          >
            <div class="px-5 py-3 bg-slate-50/70 dark:bg-[#151b2c] border-b border-slate-200 dark:border-[#1f283d] flex items-center justify-between">
              <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">General Services</h3>
            </div>
            <div class="divide-y divide-slate-100 dark:divide-[#1a2133]">
              <div
                v-for="item in report.ungroupedItems"
                :key="item.itemId"
                class="px-5 py-3.5 flex items-center justify-between gap-4 hover:bg-slate-50/50 dark:hover:bg-slate-800/20 transition"
              >
                <div class="flex items-center gap-3">
                  <span class="w-2.5 h-2.5 rounded-full shrink-0" :class="getStatusColor(item.status)"></span>
                  <div>
                    <h4 class="text-xs font-bold text-slate-800 dark:text-slate-200">{{ item.name }}</h4>
                    <p v-if="item.description" class="text-[11px] text-slate-400 mt-0.5">{{ item.description }}</p>
                  </div>
                </div>

                <div class="flex items-center gap-3 shrink-0">
                  <span
                    v-if="item.latencyMs !== undefined && item.latencyMs !== null"
                    class="text-[11px] text-slate-400 font-mono"
                  >
                    {{ item.latencyMs.toFixed(0) }}ms
                  </span>
                  <span
                    class="px-2.5 py-1 rounded-full text-[10px] font-bold"
                    :class="getStatusBadge(item.status).class"
                  >
                    {{ getStatusBadge(item.status).text }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer Notes -->
        <footer class="text-center pt-8 pb-12 border-t border-slate-200 dark:border-[#1a2133] space-y-2">
          <p class="text-xs text-slate-500 dark:text-slate-400">
            {{ report.footerText || 'Powered by Hephaestus Control Panel (HCP)' }}
          </p>
          <p class="text-[10px] text-slate-400">
            Last updated: {{ new Date(report.lastChecked).toLocaleString() }}
          </p>
        </footer>
      </template>
    </main>
  </div>
</template>
