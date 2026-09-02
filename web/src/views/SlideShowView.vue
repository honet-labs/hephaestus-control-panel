<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import { useRoute } from 'vue-router';
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
  Monitor,
  ExternalLink,
  Layers,
  Clock,
  RotateCw,
  X,
  ArrowUp,
  ArrowDown,
  Globe,
  Sliders,
  Sparkles,
  LayoutGrid,
} from 'lucide-vue-next';

interface SlideItem {
  id: string;
  title: string;
  type: 'url' | 'internal' | 'text';
  url?: string;
  routePath?: string;
  textTitle?: string;
  textContent?: string;
  textBgColor?: string;
  duration?: number; // duration in seconds
}

interface SlideShow {
  id: string;
  name: string;
  description: string;
  interval: number; // default interval in seconds
  mode: string;
  panels: SlideItem[];
  createdAt?: string;
}

const route = useRoute();
const slideShows = ref<SlideShow[]>([]);
const loading = ref(false);

// Active View Mode: 'list' | 'editor' | 'player'
const viewMode = ref<'list' | 'editor' | 'player'>('list');
const selectedShow = ref<SlideShow | null>(null);

// ==========================================
// Slide Show Create / Edit State
// ==========================================
const isModalOpen = ref(false);
const showForm = ref<SlideShow>({
  id: '',
  name: '',
  description: '',
  interval: 15,
  mode: 'slideshow',
  panels: [],
});

// Slide Item Add / Edit Modal
const isSlideModalOpen = ref(false);
const editingSlideIndex = ref<number>(-1);
const slideForm = ref<SlideItem>({
  id: '',
  title: '',
  type: 'url',
  url: '',
  routePath: '/',
  textTitle: '',
  textContent: '',
  textBgColor: '#0f172a',
  duration: 15,
});

// ==========================================
// Presentation Player State
// ==========================================
const currentSlideIndex = ref<number>(0);
const isPlaying = ref<boolean>(true);
const isFullscreen = ref<boolean>(false);
const showControls = ref<boolean>(true);
const progressPercent = ref<number>(0);

let progressTimer: any = null;
let slideTimeoutTimer: any = null;
let hideControlsTimer: any = null;

const currentSlide = computed(() => {
  if (!selectedShow.value || !selectedShow.value.panels || selectedShow.value.panels.length === 0) {
    return null;
  }
  return selectedShow.value.panels[currentSlideIndex.value] || null;
});

const currentSlideDuration = computed(() => {
  if (currentSlide.value?.duration && currentSlide.value.duration > 0) {
    return currentSlide.value.duration;
  }
  return selectedShow.value?.interval || 15;
});

// Fetch all slide shows
const fetchSlideShows = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/monitoring-views');
    if (res.data?.success) {
      slideShows.value = res.data.data || [];
    } else {
      slideShows.value = [];
    }
  } catch (err) {
    console.error('Failed to load slide shows:', err);
    slideShows.value = [];
  } finally {
    loading.value = false;
  }
};

// Create / Edit SlideShow Meta
const openCreateModal = () => {
  showForm.value = {
    id: '',
    name: '',
    description: '',
    interval: 15,
    mode: 'slideshow',
    panels: [],
  };
  isModalOpen.value = true;
};

const openEditMetaModal = (show: SlideShow) => {
  showForm.value = JSON.parse(JSON.stringify(show));
  isModalOpen.value = true;
};

const saveSlideShowMeta = async () => {
  if (!showForm.value.name) {
    alert('Please enter a name for the slide show.');
    return;
  }
  try {
    const res = await axios.post('/api/v1/monitoring-views', showForm.value);
    if (res.data?.success) {
      isModalOpen.value = false;
      await fetchSlideShows();
      if (selectedShow.value && selectedShow.value.id === showForm.value.id) {
        selectedShow.value = res.data.data;
      }
    }
  } catch (err: any) {
    alert(`Failed to save slide show: ${err.response?.data?.error || err.message}`);
  }
};

const deleteSlideShow = async (id: string) => {
  if (!confirm('Are you sure you want to delete this slide show?')) return;
  try {
    await axios.delete(`/api/v1/monitoring-views/${id}`);
    await fetchSlideShows();
    if (selectedShow.value?.id === id) {
      viewMode.value = 'list';
      selectedShow.value = null;
    }
  } catch (err: any) {
    alert(`Failed to delete slide show: ${err.response?.data?.error || err.message}`);
  }
};

// ==========================================
// Slide Editor
// ==========================================
const openEditor = (show: SlideShow) => {
  selectedShow.value = JSON.parse(JSON.stringify(show));
  if (!selectedShow.value.panels) selectedShow.value.panels = [];
  viewMode.value = 'editor';
};

const openAddSlideModal = () => {
  editingSlideIndex.value = -1;
  slideForm.value = {
    id: `slide-${Date.now()}`,
    title: `Slide ${(selectedShow.value?.panels?.length || 0) + 1}`,
    type: 'url',
    url: '',
    routePath: '/',
    textTitle: '',
    textContent: '',
    textBgColor: '#0f172a',
    duration: selectedShow.value?.interval || 15,
  };
  isSlideModalOpen.value = true;
};

const openEditSlideModal = (idx: number) => {
  editingSlideIndex.value = idx;
  const item = selectedShow.value!.panels[idx];
  slideForm.value = JSON.parse(JSON.stringify(item));
  isSlideModalOpen.value = true;
};

const saveSlideItem = async () => {
  if (!slideForm.value.title) {
    alert('Slide title is required.');
    return;
  }
  if (!selectedShow.value) return;

  if (editingSlideIndex.value >= 0) {
    selectedShow.value.panels[editingSlideIndex.value] = { ...slideForm.value };
  } else {
    selectedShow.value.panels.push({ ...slideForm.value });
  }

  isSlideModalOpen.value = false;
  await saveCurrentShowPanels();
};

const removeSlide = async (idx: number) => {
  if (!confirm('Remove this slide?')) return;
  selectedShow.value?.panels.splice(idx, 1);
  await saveCurrentShowPanels();
};

const moveSlide = async (idx: number, dir: 'up' | 'down') => {
  if (!selectedShow.value) return;
  const list = selectedShow.value.panels;
  const targetIdx = dir === 'up' ? idx - 1 : idx + 1;
  if (targetIdx < 0 || targetIdx >= list.length) return;
  const temp = list[idx];
  list[idx] = list[targetIdx];
  list[targetIdx] = temp;
  await saveCurrentShowPanels();
};

const saveCurrentShowPanels = async () => {
  if (!selectedShow.value) return;
  try {
    await axios.post('/api/v1/monitoring-views', selectedShow.value);
    await fetchSlideShows();
  } catch (err: any) {
    console.error('Failed to sync slide show:', err);
  }
};

// ==========================================
// Presentation / Player Mode
// ==========================================
const startPresentation = (show: SlideShow) => {
  selectedShow.value = show;
  if (!selectedShow.value.panels || selectedShow.value.panels.length === 0) {
    alert('This slide show has no slides yet. Please add slides first.');
    openEditor(show);
    return;
  }
  currentSlideIndex.value = 0;
  isPlaying.value = true;
  viewMode.value = 'player';
  startSlideTimer();
};

const exitPresentation = () => {
  clearTimers();
  if (document.fullscreenElement) {
    document.exitFullscreen().catch(() => {});
  }
  viewMode.value = 'list';
};

const togglePlayPause = () => {
  isPlaying.value = !isPlaying.value;
  if (isPlaying.value) {
    startSlideTimer();
  } else {
    clearTimers();
  }
};

const nextSlide = () => {
  if (!selectedShow.value || selectedShow.value.panels.length === 0) return;
  currentSlideIndex.value = (currentSlideIndex.value + 1) % selectedShow.value.panels.length;
  resetSlideTimer();
};

const prevSlide = () => {
  if (!selectedShow.value || selectedShow.value.panels.length === 0) return;
  currentSlideIndex.value = (currentSlideIndex.value - 1 + selectedShow.value.panels.length) % selectedShow.value.panels.length;
  resetSlideTimer();
};

const jumpToSlide = (idx: number) => {
  currentSlideIndex.value = idx;
  resetSlideTimer();
};

const clearTimers = () => {
  if (progressTimer) clearInterval(progressTimer);
  if (slideTimeoutTimer) clearTimeout(slideTimeoutTimer);
  progressPercent.value = 0;
};

const resetSlideTimer = () => {
  clearTimers();
  if (isPlaying.value) {
    startSlideTimer();
  }
};

const startSlideTimer = () => {
  clearTimers();
  const durationSec = currentSlideDuration.value;
  const totalMs = durationSec * 1000;
  const intervalMs = 100;
  let elapsedMs = 0;

  progressTimer = setInterval(() => {
    elapsedMs += intervalMs;
    progressPercent.value = Math.min(100, (elapsedMs / totalMs) * 100);
    if (elapsedMs >= totalMs) {
      clearInterval(progressTimer);
      nextSlide();
    }
  }, intervalMs);
};

const toggleFullscreen = () => {
  const elem = document.documentElement;
  if (!document.fullscreenElement) {
    elem.requestFullscreen().then(() => {
      isFullscreen.value = true;
    }).catch(() => {});
  } else {
    document.exitFullscreen().then(() => {
      isFullscreen.value = false;
    }).catch(() => {});
  }
};

const handleMouseMove = () => {
  showControls.value = true;
  if (hideControlsTimer) clearTimeout(hideControlsTimer);
  hideControlsTimer = setTimeout(() => {
    if (viewMode.value === 'player' && isPlaying.value) {
      showControls.value = false;
    }
  }, 3500);
};

// Keyboard navigation
const handleKeyDown = (e: KeyboardEvent) => {
  if (viewMode.value !== 'player') return;
  if (e.key === ' ' || e.code === 'Space') {
    e.preventDefault();
    togglePlayPause();
  } else if (e.key === 'ArrowRight') {
    e.preventDefault();
    nextSlide();
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault();
    prevSlide();
  } else if (e.key === 'f' || e.key === 'F') {
    e.preventDefault();
    toggleFullscreen();
  } else if (e.key === 'Escape') {
    exitPresentation();
  }
};

onMounted(async () => {
  await fetchSlideShows();
  window.addEventListener('keydown', handleKeyDown);
  window.addEventListener('mousemove', handleMouseMove);

  // If launched via /kiosk/:id route parameter
  if (route.params.id) {
    const target = slideShows.value.find((s) => s.id === route.params.id);
    if (target) {
      startPresentation(target);
    }
  }
});

onUnmounted(() => {
  clearTimers();
  window.removeEventListener('keydown', handleKeyDown);
  window.removeEventListener('mousemove', handleMouseMove);
});
</script>

<template>
  <div class="h-full flex flex-col font-sans select-none" :class="{ 'bg-black': viewMode === 'player' }">

    <!-- ================================================================= -->
    <!-- VIEW 1: SLIDE SHOWS LIST -->
    <!-- ================================================================= -->
    <div v-if="viewMode === 'list'" class="space-y-6 max-w-7xl mx-auto w-full p-2">
      <!-- Header -->
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
        <div>
          <h1 class="text-xl font-bold text-white tracking-tight flex items-center gap-2">
            <Monitor class="w-5 h-5 text-brand-400" />
            <span>Slide Show & NOC Wall Display</span>
          </h1>
          <p class="text-xs text-slate-400 mt-0.5">
            Create automated rotating dashboards, NOC Wall playlists, and fullscreen kiosk presentations.
          </p>
        </div>

        <div class="flex items-center gap-2">
          <button
            @click="fetchSlideShows"
            class="p-2 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
            title="Refresh"
          >
            <RotateCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
          </button>

          <button
            @click="openCreateModal"
            class="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white shadow-lg shadow-blue-600/20 transition"
          >
            <Plus class="w-4 h-4" />
            <span>NEW SLIDE SHOW</span>
          </button>
        </div>
      </div>

      <!-- Slide Shows Grid -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-5">
        <div
          v-for="show in slideShows"
          :key="show.id"
          class="rounded-2xl bg-[#171a23] border border-slate-800 hover:border-slate-700 p-5 space-y-4 shadow-xl transition flex flex-col justify-between"
        >
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <h3 class="text-sm font-bold text-white truncate max-w-[200px]">{{ show.name }}</h3>
              <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-amber-400 border border-slate-700/60 font-semibold flex items-center gap-1">
                <Clock class="w-3 h-3" />
                {{ show.interval }}s / slide
              </span>
            </div>

            <p class="text-xs text-slate-400 line-clamp-2">
              {{ show.description || 'No description provided.' }}
            </p>

            <div class="flex items-center gap-2 pt-2 text-[11px] text-slate-400">
              <span class="px-2 py-0.5 rounded bg-[#0f1219] border border-slate-800 font-mono text-brand-400 font-bold">
                {{ show.panels?.length || 0 }} Slides
              </span>
              <span class="uppercase text-[10px] tracking-wider text-slate-500 font-semibold">{{ show.mode }}</span>
            </div>
          </div>

          <div class="flex items-center justify-between pt-4 border-t border-slate-800">
            <button
              @click="startPresentation(show)"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold shadow-md shadow-emerald-600/20 transition"
            >
              <Play class="w-3.5 h-3.5 fill-current" />
              <span>Present</span>
            </button>

            <div class="flex items-center gap-1">
              <button
                @click="openEditor(show)"
                class="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
                title="Edit Slides"
              >
                <Edit2 class="w-3.5 h-3.5" />
              </button>
              <button
                @click="deleteSlideShow(show.id)"
                class="p-1.5 rounded-lg bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 border border-slate-700 transition"
                title="Delete Slide Show"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>

        <!-- Empty State -->
        <div v-if="slideShows.length === 0 && !loading" class="col-span-3 p-12 text-center bg-[#171a23] border border-slate-800 rounded-2xl space-y-3">
          <Monitor class="w-10 h-10 text-slate-600 mx-auto" />
          <h3 class="text-sm font-bold text-white">No Slide Shows Configured</h3>
          <p class="text-xs text-slate-500 max-w-md mx-auto">
            Create a slide show to display rotating network topology, Prometheus metrics, Grafana dashboards, or NOC wall views.
          </p>
          <button
            @click="openCreateModal"
            class="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold inline-flex items-center gap-1.5 shadow-lg"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Create First Slide Show</span>
          </button>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- VIEW 2: SLIDE SHOW EDITOR -->
    <!-- ================================================================= -->
    <div v-if="viewMode === 'editor' && selectedShow" class="space-y-6 max-w-7xl mx-auto w-full p-2">
      <!-- Editor Top Bar -->
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
        <div class="flex items-center gap-3">
          <button
            @click="viewMode = 'list'"
            class="p-2 rounded-lg bg-[#20242e] hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
            title="Back to Slide Shows"
          >
            <ChevronLeft class="w-4 h-4" />
          </button>
          <div>
            <h2 class="text-lg font-bold text-white flex items-center gap-2">
              <span>{{ selectedShow.name }}</span>
              <button @click="openEditMetaModal(selectedShow)" class="text-slate-500 hover:text-white">
                <Edit2 class="w-3.5 h-3.5" />
              </button>
            </h2>
            <p class="text-xs text-slate-400">
              {{ selectedShow.panels?.length || 0 }} slides • Default {{ selectedShow.interval }}s per slide
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <button
            @click="startPresentation(selectedShow)"
            class="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-bold bg-emerald-600 hover:bg-emerald-500 text-white shadow-lg transition"
          >
            <Play class="w-3.5 h-3.5 fill-current" />
            <span>Start Presentation</span>
          </button>

          <button
            @click="openAddSlideModal"
            class="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white shadow-lg transition"
          >
            <Plus class="w-4 h-4" />
            <span>Add Slide</span>
          </button>
        </div>
      </div>

      <!-- Slides List -->
      <div class="space-y-3">
        <div
          v-for="(slide, idx) in selectedShow.panels"
          :key="slide.id"
          class="p-4 rounded-xl bg-[#171a23] border border-slate-800 hover:border-slate-700 flex items-center justify-between gap-4 shadow-lg transition"
        >
          <div class="flex items-center gap-4">
            <span class="w-6 h-6 rounded-full bg-[#0f1219] text-brand-400 border border-slate-800 flex items-center justify-center text-xs font-bold font-mono">
              {{ idx + 1 }}
            </span>

            <div class="space-y-1">
              <div class="flex items-center gap-2">
                <h4 class="text-xs font-bold text-white">{{ slide.title }}</h4>
                <span class="text-[10px] uppercase font-mono px-1.5 py-0.2 rounded bg-slate-800 text-sky-400 border border-slate-700/60 font-semibold">
                  {{ slide.type }}
                </span>
                <span class="text-[10px] text-slate-400 font-mono">
                  ⏱ {{ slide.duration || selectedShow.interval }}s
                </span>
              </div>
              <p class="text-[11px] font-mono text-slate-400 truncate max-w-xl">
                {{ slide.type === 'url' ? slide.url : slide.type === 'internal' ? `Internal Route: ${slide.routePath}` : slide.textTitle }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1">
            <button
              @click="moveSlide(idx, 'up')"
              :disabled="idx === 0"
              class="p-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white disabled:opacity-30 transition"
              title="Move Up"
            >
              <ArrowUp class="w-3.5 h-3.5" />
            </button>
            <button
              @click="moveSlide(idx, 'down')"
              :disabled="idx === selectedShow.panels.length - 1"
              class="p-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white disabled:opacity-30 transition"
              title="Move Down"
            >
              <ArrowDown class="w-3.5 h-3.5" />
            </button>
            <button
              @click="openEditSlideModal(idx)"
              class="p-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white transition"
              title="Edit Slide"
            >
              <Edit2 class="w-3.5 h-3.5" />
            </button>
            <button
              @click="removeSlide(idx)"
              class="p-1.5 rounded bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 transition"
              title="Delete Slide"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <div v-if="selectedShow.panels.length === 0" class="p-12 text-center bg-[#171a23] border border-slate-800 rounded-xl space-y-2">
          <Layers class="w-8 h-8 text-slate-600 mx-auto" />
          <p class="text-xs font-bold text-slate-300">No slides added to this show yet</p>
          <button @click="openAddSlideModal" class="px-3 py-1.5 rounded-lg bg-blue-600 text-white text-xs font-bold">
            + Add First Slide
          </button>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- VIEW 3: FULLSCREEN PRESENTATION / KIOSK PLAYER -->
    <!-- ================================================================= -->
    <div v-if="viewMode === 'player' && selectedShow && currentSlide" class="relative w-full h-screen bg-black overflow-hidden flex flex-col">
      
      <!-- Top Progress Bar -->
      <div class="absolute top-0 left-0 right-0 h-1 bg-slate-900 z-50">
        <div
          class="h-full bg-gradient-to-r from-blue-500 via-brand-400 to-emerald-400 transition-all duration-100 ease-linear"
          :style="{ width: `${progressPercent}%` }"
        ></div>
      </div>

      <!-- Slide Viewport Content -->
      <div class="flex-1 w-full h-full relative">
        
        <!-- Type 1: External URL / Iframe / Grafana Dashboard -->
        <iframe
          v-if="currentSlide.type === 'url' && currentSlide.url"
          :src="currentSlide.url"
          class="w-full h-full border-0 bg-[#090d16]"
          allow="fullscreen; clipboard-read; clipboard-write"
        ></iframe>

        <!-- Type 2: Internal Route (Overview, Topology, etc.) -->
        <iframe
          v-else-if="currentSlide.type === 'internal'"
          :src="currentSlide.routePath || '/'"
          class="w-full h-full border-0 bg-[#090d16]"
        ></iframe>

        <!-- Type 3: Custom Announcement Banner -->
        <div
          v-else-if="currentSlide.type === 'text'"
          class="w-full h-full flex flex-col items-center justify-center p-12 text-center"
          :style="{ backgroundColor: currentSlide.textBgColor || '#0f172a' }"
        >
          <h1 class="text-4xl md:text-6xl font-black text-white tracking-tight mb-4 max-w-4xl drop-shadow-lg">
            {{ currentSlide.textTitle || currentSlide.title }}
          </h1>
          <p class="text-lg md:text-2xl text-slate-300 max-w-3xl leading-relaxed whitespace-pre-line font-light">
            {{ currentSlide.textContent }}
          </p>
        </div>

        <div v-else class="w-full h-full flex items-center justify-center text-slate-500 text-sm">
          No preview available for this slide.
        </div>
      </div>

      <!-- Floating Presentation Controller HUD -->
      <div
        v-show="showControls"
        class="absolute bottom-6 left-1/2 -translate-x-1/2 z-50 bg-[#13161f]/90 border border-slate-700/80 rounded-2xl px-5 py-2.5 shadow-2xl backdrop-blur-md flex items-center gap-4 text-xs font-sans text-white transition-opacity duration-300"
      >
        <button
          @click="prevSlide"
          class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
          title="Previous (Left Arrow)"
        >
          <ChevronLeft class="w-4 h-4" />
        </button>

        <button
          @click="togglePlayPause"
          class="p-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow"
          :title="isPlaying ? 'Pause (Space)' : 'Play (Space)'"
        >
          <Pause v-if="isPlaying" class="w-4 h-4 fill-current" />
          <Play v-else class="w-4 h-4 fill-current" />
        </button>

        <button
          @click="nextSlide"
          class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
          title="Next (Right Arrow)"
        >
          <ChevronRight class="w-4 h-4" />
        </button>

        <div class="h-4 w-[1px] bg-slate-700"></div>

        <div class="flex items-center gap-2">
          <span class="font-bold text-white tracking-wide">
            {{ currentSlideIndex + 1 }} / {{ selectedShow.panels.length }}
          </span>
          <span class="text-slate-400 truncate max-w-[150px] font-mono text-[11px]">
            {{ currentSlide.title }}
          </span>
        </div>

        <div class="h-4 w-[1px] bg-slate-700"></div>

        <button
          @click="toggleFullscreen"
          class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
          title="Toggle Fullscreen (F)"
        >
          <Maximize2 v-if="!isFullscreen" class="w-4 h-4" />
          <Minimize2 v-else class="w-4 h-4" />
        </button>

        <button
          @click="exitPresentation"
          class="p-2 rounded-lg bg-rose-600/20 hover:bg-rose-600 text-rose-300 hover:text-white border border-rose-500/40 transition"
          title="Exit Presentation (Esc)"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

    </div>

    <!-- ================================================================= -->
    <!-- MODAL: CREATE / EDIT SLIDESHOW META -->
    <!-- ================================================================= -->
    <div v-if="isModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-md bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Monitor class="w-4 h-4 text-brand-400" />
            <span>{{ showForm.id ? 'Edit Slide Show' : 'New Slide Show' }}</span>
          </h3>
          <button @click="isModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveSlideShowMeta" class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Slide Show Name</label>
            <input v-model="showForm.name" required placeholder="e.g. NOC Main Wall Display" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white" />
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-bold">Description</label>
            <textarea v-model="showForm.description" rows="2" placeholder="Summary or purpose..." class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white"></textarea>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Default Interval (seconds)</label>
              <input v-model.number="showForm.interval" type="number" min="3" max="3600" required class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white font-mono" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Display Mode</label>
              <select v-model="showForm.mode" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-2 text-white">
                <option value="slideshow">Auto-Rotating Slide Show</option>
                <option value="kiosk">Kiosk Mode</option>
              </select>
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="isModalOpen = false" class="px-3 py-1.5 text-slate-400 hover:text-white">Cancel</button>
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow">Save Slide Show</button>
          </div>
        </form>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: ADD / EDIT SLIDE ITEM -->
    <!-- ================================================================= -->
    <div v-if="isSlideModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-lg bg-[#171a23] border border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Layers class="w-4 h-4 text-brand-400" />
            <span>{{ editingSlideIndex >= 0 ? 'Edit Slide' : 'Add Slide' }}</span>
          </h3>
          <button @click="isSlideModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveSlideItem" class="space-y-3 text-xs">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Slide Title</label>
              <input v-model="slideForm.title" required placeholder="e.g. Backbone Network Topology" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Slide Type</label>
              <select v-model="slideForm.type" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
                <option value="url">External URL / Dashboard Iframe</option>
                <option value="internal">Internal Hephaestus View</option>
                <option value="text">Announcement / Text Slide</option>
              </select>
            </div>
          </div>

          <!-- Type 1: URL -->
          <div v-if="slideForm.type === 'url'">
            <label class="block text-slate-400 mb-1 font-bold">Dashboard or Web URL</label>
            <input v-model="slideForm.url" placeholder="https://grafana.example.com/d/... or http://..." class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            <span class="text-[10px] text-slate-500 mt-0.5 block">Grafana, OpenSearch, Prometheus, or any external monitoring web page.</span>
          </div>

          <!-- Type 2: Internal -->
          <div v-if="slideForm.type === 'internal'">
            <label class="block text-slate-400 mb-1 font-bold">Select Internal View</label>
            <select v-model="slideForm.routePath" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white">
              <option value="/">Overview Dashboard (/)</option>
              <option value="/network-topology">Network Topology (/network-topology)</option>
              <option value="/opensearch-cluster">OpenSearch Cluster (/opensearch-cluster)</option>
              <option value="/backup">Backup Manager (/backup)</option>
              <option value="/connections">Connections (/connections)</option>
              <option value="/prometheus-config">Prometheus Config (/prometheus-config)</option>
              <option value="/dataprepper-config">Data Prepper Pipelines (/dataprepper-config)</option>
              <option value="/snmp">SNMP MIB Explorer (/snmp)</option>
              <option value="/grok-debugger">Grok Pattern Studio (/grok-debugger)</option>
            </select>
          </div>

          <!-- Type 3: Text Announcement -->
          <template v-if="slideForm.type === 'text'">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Heading Title</label>
              <input v-model="slideForm.textTitle" placeholder="MAINTENANCE NOTICE" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white" />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Content Message</label>
              <textarea v-model="slideForm.textContent" rows="3" placeholder="Server maintenance scheduled from 02:00 to 04:00 UTC." class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white"></textarea>
            </div>
          </template>

          <div class="grid grid-cols-2 gap-3 pt-1">
            <div>
              <label class="block text-slate-400 mb-1 font-bold">Slide Duration (seconds)</label>
              <input v-model.number="slideForm.duration" type="number" min="3" max="3600" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
            </div>
            <div v-if="slideForm.type === 'text'">
              <label class="block text-slate-400 mb-1 font-bold">Background Color</label>
              <input v-model="slideForm.textBgColor" type="color" class="w-full h-8 bg-[#0f1219] border border-slate-700 rounded-lg p-0.5 cursor-pointer" />
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="isSlideModalOpen = false" class="px-3 py-1.5 text-slate-400 hover:text-white">Cancel</button>
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow">Save Slide</button>
          </div>
        </form>
      </div>
    </div>

  </div>
</template>
