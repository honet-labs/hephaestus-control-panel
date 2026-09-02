<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import {
  Play,
  Pause,
  Plus,
  Trash2,
  Edit2,
  Maximize2,
  Minimize2,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Clock,
  RotateCw,
  X,
  Globe,
  Sliders,
  Monitor,
  Check,
  Power,
  Layers,
  Search,
} from 'lucide-vue-next';

interface EmbedItem {
  id: string;
  name: string;
  url: string;
  interval: number; // in seconds (e.g. 15)
  zoom: number; // in percentage (e.g. 100, 90, 80)
  isActive: boolean;
  createdAt?: string;
}

const activeTab = ref<'viewer' | 'manage'>('viewer');
const embedList = ref<EmbedItem[]>([]);
const loading = ref(false);

// Viewer / Player State
const currentActiveIndex = ref<number>(0);
const isAutoRotating = ref<boolean>(true);
const isFullscreen = ref<boolean>(false);
const progressPercent = ref<number>(0);
const iframeKey = ref<number>(0);

let progressTimer: any = null;

// Modal State (Add / Edit Embed URL)
const isModalOpen = ref(false);
const editingId = ref<string | null>(null);
const form = ref<{
  name: string;
  url: string;
  interval: number;
  zoom: number;
  isActive: boolean;
}>({
  name: '',
  url: '',
  interval: 15,
  zoom: 100,
  isActive: true,
});

// Active items participating in rotation
const activeEmbeds = computed(() => {
  return embedList.value.filter((item) => item.isActive && item.url);
});

const currentEmbed = computed<EmbedItem | null>(() => {
  if (activeEmbeds.value.length === 0) return null;
  const safeIdx = Math.max(0, Math.min(currentActiveIndex.value, activeEmbeds.value.length - 1));
  return activeEmbeds.value[safeIdx] || null;
});

// Load Embed URLs from backend
const fetchEmbeds = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/monitoring-views');
    if (res.data?.success && Array.isArray(res.data.data)) {
      embedList.value = res.data.data.map((item: any) => {
        let meta: any = {};
        if (typeof item.panels === 'object' && item.panels !== null) {
          meta = item.panels;
        }
        return {
          id: item.id,
          name: item.name,
          url: meta.url || item.description || '',
          interval: item.interval > 0 ? item.interval : 15,
          zoom: meta.zoom || 100,
          isActive: meta.isActive !== false,
          createdAt: item.createdAt,
        };
      });
    } else {
      embedList.value = [];
    }

    if (currentActiveIndex.value >= activeEmbeds.value.length) {
      currentActiveIndex.value = 0;
    }
  } catch (err) {
    console.error('Failed to load embed URLs:', err);
    embedList.value = [];
  } finally {
    loading.value = false;
  }
};

// Open Add Modal
const openAddModal = () => {
  editingId.value = null;
  form.value = {
    name: '',
    url: '',
    interval: 15,
    zoom: 100,
    isActive: true,
  };
  isModalOpen.value = true;
};

// Open Edit Modal
const openEditModal = (item: EmbedItem) => {
  editingId.value = item.id;
  form.value = {
    name: item.name,
    url: item.url,
    interval: item.interval,
    zoom: item.zoom || 100,
    isActive: item.isActive,
  };
  isModalOpen.value = true;
};

// Save Embed URL
const saveEmbedUrl = async () => {
  if (!form.value.name.trim() || !form.value.url.trim()) {
    alert('Please enter both a Name and a valid Embed URL.');
    return;
  }

  // Ensure protocol
  let cleanUrl = form.value.url.trim();
  if (!cleanUrl.startsWith('http://') && !cleanUrl.startsWith('https://') && !cleanUrl.startsWith('/')) {
    cleanUrl = 'https://' + cleanUrl;
  }

  const payload = {
    id: editingId.value || `embed-${Date.now()}`,
    name: form.value.name.trim(),
    description: cleanUrl,
    interval: form.value.interval > 0 ? form.value.interval : 15,
    mode: 'embed',
    panels: {
      url: cleanUrl,
      zoom: form.value.zoom || 100,
      isActive: form.value.isActive,
    },
  };

  try {
    const res = await axios.post('/api/v1/monitoring-views', payload);
    if (res.data?.success) {
      isModalOpen.value = false;
      await fetchEmbeds();
      if (activeEmbeds.value.length > 0 && activeTab.value === 'viewer') {
        resetTimer();
      }
    }
  } catch (err: any) {
    alert(`Failed to save Embed URL: ${err.response?.data?.error || err.message}`);
  }
};

// Delete Embed URL
const deleteEmbed = async (id: string) => {
  if (!confirm('Are you sure you want to delete this Embed URL?')) return;
  try {
    await axios.delete(`/api/v1/monitoring-views/${id}`);
    await fetchEmbeds();
    resetTimer();
  } catch (err: any) {
    alert(`Failed to delete: ${err.response?.data?.error || err.message}`);
  }
};

// Toggle active status in rotation
const toggleItemActive = async (item: EmbedItem) => {
  item.isActive = !item.isActive;
  const payload = {
    id: item.id,
    name: item.name,
    description: item.url,
    interval: item.interval,
    mode: 'embed',
    panels: {
      url: item.url,
      zoom: item.zoom,
      isActive: item.isActive,
    },
  };
  try {
    await axios.post('/api/v1/monitoring-views', payload);
    await fetchEmbeds();
    resetTimer();
  } catch (err) {
    console.error('Failed to update active state:', err);
  }
};

// Rotation controls
const nextEmbed = () => {
  if (activeEmbeds.value.length <= 1) return;
  currentActiveIndex.value = (currentActiveIndex.value + 1) % activeEmbeds.value.length;
  resetTimer();
};

const prevEmbed = () => {
  if (activeEmbeds.value.length <= 1) return;
  currentActiveIndex.value = (currentActiveIndex.value - 1 + activeEmbeds.value.length) % activeEmbeds.value.length;
  resetTimer();
};

const selectEmbed = (idx: number) => {
  currentActiveIndex.value = idx;
  resetTimer();
};

const toggleAutoRotate = () => {
  isAutoRotating.value = !isAutoRotating.value;
  if (isAutoRotating.value) {
    startRotationTimer();
  } else {
    clearTimer();
  }
};

const refreshCurrentIframe = () => {
  iframeKey.value++;
  resetTimer();
};

const clearTimer = () => {
  if (progressTimer) clearInterval(progressTimer);
  progressPercent.value = 0;
};

const resetTimer = () => {
  clearTimer();
  if (isAutoRotating.value && activeEmbeds.value.length > 1) {
    startRotationTimer();
  }
};

const startRotationTimer = () => {
  clearTimer();
  if (!currentEmbed.value || activeEmbeds.value.length <= 1) return;

  const durationSec = currentEmbed.value.interval || 15;
  const totalMs = durationSec * 1000;
  const stepMs = 100;
  let elapsed = 0;

  progressTimer = setInterval(() => {
    elapsed += stepMs;
    progressPercent.value = Math.min(100, (elapsed / totalMs) * 100);
    if (elapsed >= totalMs) {
      clearInterval(progressTimer);
      nextEmbed();
    }
  }, stepMs);
};

const toggleFullscreen = () => {
  const el = document.getElementById('embed-slideshow-container');
  if (!el) return;
  if (!document.fullscreenElement) {
    el.requestFullscreen().then(() => {
      isFullscreen.value = true;
    }).catch(() => {});
  } else {
    document.exitFullscreen().then(() => {
      isFullscreen.value = false;
    }).catch(() => {});
  }
};

const openInNewTab = (url?: string) => {
  const target = url || currentEmbed.value?.url;
  if (target) {
    window.open(target, '_blank');
  }
};

// Keyboard navigation
const handleKeyDown = (e: KeyboardEvent) => {
  if (activeTab.value !== 'viewer') return;
  if (e.key === ' ' && e.target === document.body) {
    e.preventDefault();
    toggleAutoRotate();
  } else if (e.key === 'ArrowRight') {
    nextEmbed();
  } else if (e.key === 'ArrowLeft') {
    prevEmbed();
  }
};

watch(activeTab, (newTab) => {
  if (newTab === 'viewer') {
    resetTimer();
  } else {
    clearTimer();
  }
});

onMounted(async () => {
  await fetchEmbeds();
  if (activeEmbeds.value.length > 1) {
    startRotationTimer();
  }
  window.addEventListener('keydown', handleKeyDown);
});

onUnmounted(() => {
  clearTimer();
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div id="embed-slideshow-container" class="h-full flex flex-col font-sans select-none bg-[#090d16] text-white">
    
    <!-- Top Bar -->
    <div class="px-4 py-3 bg-[#13161f] border-b border-slate-800 flex flex-wrap items-center justify-between gap-3 shrink-0">
      
      <!-- Title & Tab Switcher -->
      <div class="flex items-center gap-4">
        <div class="flex items-center gap-2">
          <Monitor class="w-5 h-5 text-brand-400" />
          <h1 class="text-sm font-bold text-white tracking-wide">Slide Show (Embed URL)</h1>
        </div>

        <div class="flex items-center bg-[#090d16] p-0.5 rounded-lg border border-slate-800 text-xs font-semibold">
          <button
            @click="activeTab = 'viewer'"
            :class="[
              activeTab === 'viewer'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-white',
              'px-3 py-1 rounded-md transition flex items-center gap-1.5'
            ]"
          >
            <Globe class="w-3.5 h-3.5" />
            <span>Embed Viewer ({{ activeEmbeds.length }})</span>
          </button>

          <button
            @click="activeTab = 'manage'"
            :class="[
              activeTab === 'manage'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-white',
              'px-3 py-1 rounded-md transition flex items-center gap-1.5'
            ]"
          >
            <Sliders class="w-3.5 h-3.5" />
            <span>Manage URLs ({{ embedList.length }})</span>
          </button>
        </div>
      </div>

      <!-- Viewer Controls (When on Viewer tab and has embeds) -->
      <div v-if="activeTab === 'viewer' && activeEmbeds.length > 0" class="flex items-center gap-2 text-xs">
        
        <!-- URL Selector Dropdown -->
        <select
          :value="currentActiveIndex"
          @change="selectEmbed(Number(($event.target as HTMLSelectElement).value))"
          class="bg-[#090d16] border border-slate-700 rounded-lg px-2.5 py-1.5 text-xs text-white font-medium focus:outline-none focus:border-brand-500 max-w-[220px] truncate"
        >
          <option v-for="(item, idx) in activeEmbeds" :key="item.id" :value="idx">
            {{ idx + 1 }}. {{ item.name }} ({{ item.interval }}s)
          </option>
        </select>

        <!-- Previous / Next -->
        <div class="flex items-center bg-[#090d16] border border-slate-700 rounded-lg p-0.5">
          <button
            @click="prevEmbed"
            :disabled="activeEmbeds.length <= 1"
            class="p-1 rounded text-slate-300 hover:text-white hover:bg-slate-800 disabled:opacity-30 transition"
            title="Previous URL (Left Arrow)"
          >
            <ChevronLeft class="w-4 h-4" />
          </button>

          <button
            @click="toggleAutoRotate"
            :disabled="activeEmbeds.length <= 1"
            :class="[
              isAutoRotating ? 'text-emerald-400 bg-emerald-500/10' : 'text-slate-400 hover:text-white',
              'px-2 py-1 rounded text-[11px] font-bold flex items-center gap-1 transition'
            ]"
            :title="isAutoRotating ? 'Pause Rotation (Space)' : 'Play Auto-Rotation (Space)'"
          >
            <Pause v-if="isAutoRotating" class="w-3.5 h-3.5 fill-current" />
            <Play v-else class="w-3.5 h-3.5 fill-current" />
            <span>{{ isAutoRotating ? 'Auto' : 'Paused' }}</span>
          </button>

          <button
            @click="nextEmbed"
            :disabled="activeEmbeds.length <= 1"
            class="p-1 rounded text-slate-300 hover:text-white hover:bg-slate-800 disabled:opacity-30 transition"
            title="Next URL (Right Arrow)"
          >
            <ChevronRight class="w-4 h-4" />
          </button>
        </div>

        <!-- Reload Current Iframe -->
        <button
          @click="refreshCurrentIframe"
          class="p-1.5 rounded-lg bg-[#090d16] hover:bg-slate-800 border border-slate-700 text-slate-300 transition"
          title="Reload Iframe"
        >
          <RotateCw class="w-3.5 h-3.5" />
        </button>

        <!-- Open in New Tab -->
        <button
          @click="openInNewTab()"
          class="p-1.5 rounded-lg bg-[#090d16] hover:bg-slate-800 border border-slate-700 text-slate-300 transition"
          title="Open in New Tab"
        >
          <ExternalLink class="w-3.5 h-3.5" />
        </button>

        <!-- Fullscreen / Kiosk -->
        <button
          @click="toggleFullscreen"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow"
          title="Toggle NOC Fullscreen (F11)"
        >
          <Maximize2 v-if="!isFullscreen" class="w-3.5 h-3.5" />
          <Minimize2 v-else class="w-3.5 h-3.5" />
          <span>{{ isFullscreen ? 'Exit' : 'Kiosk' }}</span>
        </button>
      </div>

      <!-- Actions on Manage Tab -->
      <div v-if="activeTab === 'manage'" class="flex items-center gap-2">
        <button
          @click="fetchEmbeds"
          class="p-2 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
          title="Refresh List"
        >
          <RotateCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </button>

        <button
          @click="openAddModal"
          class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white shadow-lg transition"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>ADD EMBED URL</span>
        </button>
      </div>

    </div>

    <!-- Top Countdown Progress Bar (When Auto-Rotating) -->
    <div v-if="activeTab === 'viewer' && isAutoRotating && activeEmbeds.length > 1" class="w-full h-1 bg-slate-900 shrink-0">
      <div
        class="h-full bg-gradient-to-r from-blue-500 via-sky-400 to-emerald-400 transition-all duration-100 ease-linear"
        :style="{ width: `${progressPercent}%` }"
      ></div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 1: EMBED VIEWER (LIVE ROTATING IFRAME) -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'viewer'" class="flex-1 relative w-full h-full overflow-hidden bg-[#05070d]">
      
      <!-- When URL is available -->
      <template v-if="currentEmbed && currentEmbed.url">
        <iframe
          :key="`${currentEmbed.id}-${iframeKey}`"
          :src="currentEmbed.url"
          class="w-full h-full border-0 bg-[#090d16]"
          allow="fullscreen; clipboard-read; clipboard-write; camera; microphone"
          :style="{
            transform: currentEmbed.zoom && currentEmbed.zoom !== 100 ? `scale(${currentEmbed.zoom / 100})` : 'none',
            transformOrigin: 'top left',
            width: currentEmbed.zoom && currentEmbed.zoom !== 100 ? `${(100 / currentEmbed.zoom) * 100}%` : '100%',
            height: currentEmbed.zoom && currentEmbed.zoom !== 100 ? `${(100 / currentEmbed.zoom) * 100}%` : '100%',
          }"
        ></iframe>

        <!-- Bottom Float Indicator Bar -->
        <div class="absolute bottom-3 left-4 z-30 bg-[#13161f]/85 backdrop-blur-md border border-slate-700/80 rounded-xl px-3 py-1.5 text-xs text-slate-300 flex items-center gap-2.5 shadow-xl pointer-events-none">
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          <span class="font-bold text-white">{{ currentEmbed.name }}</span>
          <span class="text-[11px] text-slate-400 font-mono truncate max-w-xs">{{ currentEmbed.url }}</span>
          <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-amber-400 font-semibold border border-slate-700/60">
            {{ currentActiveIndex + 1 }} / {{ activeEmbeds.length }}
          </span>
        </div>
      </template>

      <!-- Empty State When No Embed URLs Added -->
      <div v-else class="w-full h-full flex flex-col items-center justify-center p-8 text-center space-y-4">
        <div class="w-16 h-16 rounded-2xl bg-[#171a23] border border-slate-800 flex items-center justify-center text-slate-600 shadow-xl">
          <Globe class="w-8 h-8 text-brand-400" />
        </div>
        <div class="space-y-1 max-w-md">
          <h3 class="text-base font-bold text-white">No Embed URLs Configured</h3>
          <p class="text-xs text-slate-400 leading-relaxed">
            Add your Grafana dashboards, OpenSearch Dashboards, Prometheus targets, or any monitoring web pages to view them live and auto-rotate in Slide Show.
          </p>
        </div>
        <button
          @click="openAddModal"
          class="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold inline-flex items-center gap-1.5 shadow-lg shadow-blue-600/20"
        >
          <Plus class="w-4 h-4" />
          <span>Add Your First Embed URL</span>
        </button>
      </div>

    </div>

    <!-- ============================================================= -->
    <!-- TAB 2: MANAGE EMBED URLS TABLE -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'manage'" class="flex-1 p-6 overflow-y-auto max-w-6xl mx-auto w-full space-y-4">
      
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-sm font-bold text-white">Configured Embed URLs</h2>
          <p class="text-xs text-slate-400">List of web pages and dashboards for Slide Show rotation</p>
        </div>

        <button
          @click="openAddModal"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white shadow transition"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Embed URL</span>
        </button>
      </div>

      <div class="bg-[#171a23] border border-slate-800 rounded-xl overflow-x-auto shadow-xl">
        <table class="w-full text-left text-xs font-sans">
          <thead class="bg-[#1c202b] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
            <tr>
              <th class="p-3.5 w-12 text-center">Active</th>
              <th class="p-3.5">Name / Title</th>
              <th class="p-3.5">Embed URL Target</th>
              <th class="p-3.5 w-28">Rotation Interval</th>
              <th class="p-3.5 w-24">Zoom Scale</th>
              <th class="p-3.5 w-32 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 text-slate-300 text-xs">
            <tr v-for="item in embedList" :key="item.id" class="hover:bg-slate-800/30 transition">
              <td class="p-3.5 text-center">
                <button
                  @click="toggleItemActive(item)"
                  :class="[
                    item.isActive ? 'bg-emerald-500 text-white' : 'bg-slate-800 text-slate-500 border border-slate-700',
                    'w-5 h-5 rounded-md inline-flex items-center justify-center transition'
                  ]"
                  :title="item.isActive ? 'Active in Slide Show' : 'Paused'"
                >
                  <Check v-if="item.isActive" class="w-3.5 h-3.5" />
                  <X v-else class="w-3 h-3" />
                </button>
              </td>

              <td class="p-3.5 font-bold text-white flex items-center gap-2">
                <Globe class="w-4 h-4 text-brand-400 shrink-0" />
                <span>{{ item.name }}</span>
              </td>

              <td class="p-3.5 font-mono text-[11px] text-slate-400">
                <a :href="item.url" target="_blank" class="hover:text-brand-400 hover:underline truncate block max-w-md">
                  {{ item.url }}
                </a>
              </td>

              <td class="p-3.5 font-mono text-amber-400 font-bold">
                {{ item.interval }}s
              </td>

              <td class="p-3.5 font-mono text-slate-400">
                {{ item.zoom || 100 }}%
              </td>

              <td class="p-3.5 text-right space-x-1 whitespace-nowrap">
                <button
                  @click="openInNewTab(item.url)"
                  class="p-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                  title="Open in New Tab"
                >
                  <ExternalLink class="w-3.5 h-3.5" />
                </button>

                <button
                  @click="openEditModal(item)"
                  class="p-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                  title="Edit URL"
                >
                  <Edit2 class="w-3.5 h-3.5" />
                </button>

                <button
                  @click="deleteEmbed(item.id)"
                  class="p-1.5 rounded bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 transition"
                  title="Delete"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </td>
            </tr>

            <tr v-if="embedList.length === 0">
              <td colspan="6" class="p-12 text-center text-slate-500">
                No Embed URLs registered yet. Click "Add Embed URL" to add a dashboard link.
              </td>
            </tr>
          </tbody>
        </table>
      </div>

    </div>

    <!-- ============================================================= -->
    <!-- MODAL: ADD / EDIT EMBED URL -->
    <!-- ============================================================= -->
    <div v-if="isModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 animate-in fade-in duration-150">
      <div class="w-full max-w-lg bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans text-white">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-sm font-bold flex items-center gap-2">
            <Globe class="w-4 h-4 text-brand-400" />
            <span>{{ editingId ? 'Edit Embed URL' : 'Add New Embed URL' }}</span>
          </h3>
          <button @click="isModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveEmbedUrl" class="space-y-4 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Dashboard / URL Title</label>
            <input
              v-model="form.name"
              required
              placeholder="e.g. Grafana NOC Dashboard, OpenSearch Telemetry"
              class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500"
            />
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-bold">Embed URL Link</label>
            <input
              v-model="form.url"
              required
              placeholder="http://10.20.30.40:3000/d/... or https://grafana.mycorp.com/..."
              class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-brand-500"
            />
            <span class="text-[10px] text-slate-500 mt-1 block">
              Supports any iframe embeddable URL (Grafana, Kibana, OpenSearch, Prometheus, Uptime Kuma, Weathermap, etc.).
            </span>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Rotation Interval (seconds)</label>
              <input
                v-model.number="form.interval"
                type="number"
                min="3"
                max="3600"
                required
                class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-brand-500"
              />
              <span class="text-[10px] text-slate-500 mt-0.5 block">Time to display before rotating to next URL.</span>
            </div>

            <div>
              <label class="block text-slate-400 mb-1 font-bold">Zoom Scaling (%)</label>
              <select
                v-model.number="form.zoom"
                class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500"
              >
                <option :value="100">100% (Default)</option>
                <option :value="90">90%</option>
                <option :value="80">80%</option>
                <option :value="75">75% (Dense NOC Grid)</option>
                <option :value="67">67%</option>
                <option :value="50">50%</option>
              </select>
            </div>
          </div>

          <div class="pt-2 border-t border-slate-800 flex items-center justify-between">
            <label class="flex items-center gap-2 cursor-pointer text-slate-300 text-xs">
              <input type="checkbox" v-model="form.isActive" class="rounded bg-slate-800 border-slate-700 text-blue-600 focus:ring-0" />
              <span>Include in Slide Show Auto-Rotation</span>
            </label>
          </div>

          <div class="flex justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" @click="isModalOpen = false" class="px-3 py-2 text-slate-400 hover:text-white">Cancel</button>
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow-lg">
              Save Embed URL
            </button>
          </div>
        </form>
      </div>
    </div>

  </div>
</template>
