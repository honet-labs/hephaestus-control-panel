<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import axios from 'axios';
import {
  Plus,
  RefreshCw,
  ExternalLink,
  Copy,
  Check,
  Trash2,
  Edit3,
  Globe,
  Lock,
  Layers,
  Activity,
  AlertTriangle,
  Server,
  Network,
  Database,
  BarChart2,
  X,
  Clock,
  Sliders,
  ChevronDown,
  ChevronRight,
  Eye,
} from 'lucide-vue-next';

interface StatusPageGroup {
  id: string;
  pageId: string;
  name: string;
  sortOrder: number;
}

interface StatusPageItem {
  id: string;
  pageId: string;
  groupId?: string | null;
  name: string;
  sourceType: string; // "topology" | "opensearch" | "prometheus" | "grafana" | "remote_server"
  sourceId?: string | null;
  sourceConfig?: Record<string, any>;
  description?: string;
  sortOrder: number;
}

interface StatusPageIncident {
  id: string;
  pageId: string;
  title: string;
  status: string; // "investigating" | "identified" | "monitoring" | "resolved" | "maintenance"
  severity: string; // "info" | "minor" | "major" | "critical"
  message: string;
  isActive: boolean;
  createdAt?: string;
  updatedAt?: string;
}

interface StatusPage {
  id: string;
  title: string;
  slug: string;
  description: string;
  footerText: string;
  theme: string;
  refreshInterval: number;
  isPublic: boolean;
  isPublished: boolean;
  showTags: boolean;
  customCss?: string;
  groups?: StatusPageGroup[];
  items?: StatusPageItem[];
  incidents?: StatusPageIncident[];
  createdAt?: string;
  updatedAt?: string;
}

interface SourceOption {
  id: string;
  name: string;
  sourceType: string;
  detail?: string;
  ipOrHost?: string;
  status?: string;
}

// State
const pages = ref<StatusPage[]>([]);
const sourceOptions = ref<SourceOption[]>([]);
const loading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const toastMessage = ref<{ text: string; isError: boolean } | null>(null);
const copiedSlug = ref<string | null>(null);

// Modal states
const showCreateModal = ref(false);
const showEditDrawer = ref(false);
const showDeleteModal = ref(false);
const showItemModal = ref(false);
const showIncidentModal = ref(false);

const pageToDelete = ref<StatusPage | null>(null);
const editingPage = ref<StatusPage | null>(null);

// New Page Form
const newPageForm = ref({
  title: '',
  slug: '',
  isPublic: true,
});

// Group Form
const newGroupName = ref('');

// Item Form
const targetGroupId = ref<string | null>(null);
const itemForm = ref({
  id: '',
  name: '',
  sourceType: 'topology',
  sourceId: '',
  description: '',
});

// Incident Form
const incidentForm = ref({
  id: '',
  title: '',
  status: 'investigating',
  severity: 'minor',
  message: '',
  isActive: true,
});

const showToast = (text: string, isError = false) => {
  toastMessage.value = { text, isError };
  setTimeout(() => {
    toastMessage.value = null;
  }, 3000);
};

const fetchPages = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/status-pages');
    if (res.data?.success) {
      pages.value = res.data.data || [];
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to load status pages', true);
  } finally {
    loading.value = false;
  }
};

const fetchSourceOptions = async () => {
  try {
    const res = await axios.get('/api/v1/status-pages/sources');
    if (res.data?.success) {
      sourceOptions.value = res.data.data || [];
    }
  } catch (err) {
    console.error('Failed to load source options', err);
  }
};

const copyPageLink = (slug: string) => {
  const fullUrl = `${window.location.origin}/status/${slug}`;
  navigator.clipboard.writeText(fullUrl);
  copiedSlug.value = slug;
  showToast('Public link copied to clipboard!');
  setTimeout(() => {
    copiedSlug.value = null;
  }, 2500);
};

// Create New Page
const isSlugManuallyEdited = ref(false);

const generateSlug = (text: string) => {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/[\s_]+/g, '-')
    .replace(/^-+|-+$/g, '');
};

const handleTitleInput = () => {
  if (!isSlugManuallyEdited.value) {
    newPageForm.value.slug = generateSlug(newPageForm.value.title);
  }
};

const handleSlugManualInput = () => {
  isSlugManuallyEdited.value = true;
};

const openCreateModal = () => {
  isSlugManuallyEdited.value = false;
  newPageForm.value = {
    title: '',
    slug: '',
    isPublic: true,
  };
  showCreateModal.value = true;
};

const submitCreatePage = async () => {
  if (!newPageForm.value.title.trim() || !newPageForm.value.slug.trim()) {
    showToast('Name and slug are required', true);
    return;
  }
  saving.value = true;
  try {
    const res = await axios.post('/api/v1/status-pages', {
      title: newPageForm.value.title.trim(),
      slug: newPageForm.value.slug.trim().toLowerCase(),
      isPublic: newPageForm.value.isPublic,
      refreshInterval: 60,
      theme: 'auto',
      description: 'System Status and Live Incident Reports',
      footerText: 'Powered by Hephaestus Control Panel',
    });
    if (res.data?.success) {
      showToast('Status page created successfully!');
      showCreateModal.value = false;
      await fetchPages();
      // Open builder directly
      const created = pages.value.find((p) => p.slug === newPageForm.value.slug.trim().toLowerCase());
      if (created) {
        openEditDrawer(created);
      }
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to create status page', true);
  } finally {
    saving.value = false;
  }
};

// Edit Page / Builder
const openEditDrawer = async (page: StatusPage) => {
  try {
    const res = await axios.get(`/api/v1/status-pages/${page.id}`);
    if (res.data?.success) {
      editingPage.value = JSON.parse(JSON.stringify(res.data.data));
    } else {
      editingPage.value = JSON.parse(JSON.stringify(page));
    }
  } catch {
    editingPage.value = JSON.parse(JSON.stringify(page));
  }
  await fetchSourceOptions();
  showEditDrawer.value = true;
};

const savePageConfig = async () => {
  if (!editingPage.value) return;
  saving.value = true;
  try {
    const res = await axios.put(`/api/v1/status-pages/${editingPage.value.id}`, editingPage.value);
    if (res.data?.success) {
      showToast('Configuration saved successfully!');
      await fetchPages();
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to save configuration', true);
  } finally {
    saving.value = false;
  }
};

// Groups
const addGroup = async () => {
  if (!editingPage.value || !newGroupName.value.trim()) return;
  try {
    const res = await axios.post(`/api/v1/status-pages/${editingPage.value.id}/groups`, {
      name: newGroupName.value.trim(),
      sortOrder: (editingPage.value.groups?.length || 0) + 1,
    });
    if (res.data?.success) {
      if (!editingPage.value.groups) editingPage.value.groups = [];
      editingPage.value.groups.push(res.data.data);
      newGroupName.value = '';
      showToast('Group added successfully');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to add group', true);
  }
};

const deleteGroup = async (groupId: string) => {
  if (!editingPage.value) return;
  try {
    await axios.delete(`/api/v1/status-pages/groups/${groupId}`);
    editingPage.value.groups = editingPage.value.groups?.filter((g) => g.id !== groupId) || [];
    // Move items in this group to ungrouped
    if (editingPage.value.items) {
      editingPage.value.items.forEach((it) => {
        if (it.groupId === groupId) it.groupId = null;
      });
    }
    showToast('Group removed');
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to remove group', true);
  }
};

// Monitored Items
const openAddItemModal = async (groupId?: string | null) => {
  targetGroupId.value = groupId || null;
  itemForm.value = {
    id: '',
    name: '',
    sourceType: 'topology',
    sourceId: '',
    description: '',
  };
  if (sourceOptions.value.length === 0) {
    await fetchSourceOptions();
  }
  showItemModal.value = true;
};

const filteredSourceOptions = computed(() => {
  return sourceOptions.value.filter((opt) => opt.sourceType === itemForm.value.sourceType);
});

const onSourceSelect = () => {
  const selected = sourceOptions.value.find((opt) => opt.id === itemForm.value.sourceId);
  if (selected && !itemForm.value.name) {
    itemForm.value.name = selected.name;
  }
};

const submitSaveItem = async () => {
  if (!editingPage.value || !itemForm.value.name.trim()) {
    showToast('Monitor name is required', true);
    return;
  }
  try {
    const res = await axios.post(`/api/v1/status-pages/${editingPage.value.id}/items`, {
      id: itemForm.value.id || undefined,
      groupId: targetGroupId.value,
      name: itemForm.value.name.trim(),
      sourceType: itemForm.value.sourceType,
      sourceId: itemForm.value.sourceId || null,
      description: itemForm.value.description.trim(),
      sortOrder: (editingPage.value.items?.length || 0) + 1,
    });
    if (res.data?.success) {
      if (!editingPage.value.items) editingPage.value.items = [];
      const savedItem = res.data.data;
      const idx = editingPage.value.items.findIndex((it) => it.id === savedItem.id);
      if (idx >= 0) {
        editingPage.value.items[idx] = savedItem;
      } else {
        editingPage.value.items.push(savedItem);
      }
      showItemModal.value = false;
      showToast('Monitored item added');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to save item', true);
  }
};

const deleteItem = async (itemId: string) => {
  if (!editingPage.value) return;
  try {
    await axios.delete(`/api/v1/status-pages/items/${itemId}`);
    editingPage.value.items = editingPage.value.items?.filter((it) => it.id !== itemId) || [];
    showToast('Item removed');
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to remove item', true);
  }
};

// Incidents
const openAddIncidentModal = () => {
  incidentForm.value = {
    id: '',
    title: '',
    status: 'investigating',
    severity: 'minor',
    message: '',
    isActive: true,
  };
  showIncidentModal.value = true;
};

const submitSaveIncident = async () => {
  if (!editingPage.value || !incidentForm.value.title.trim() || !incidentForm.value.message.trim()) {
    showToast('Title and message are required', true);
    return;
  }
  try {
    const res = await axios.post(`/api/v1/status-pages/${editingPage.value.id}/incidents`, incidentForm.value);
    if (res.data?.success) {
      if (!editingPage.value.incidents) editingPage.value.incidents = [];
      const savedInc = res.data.data;
      const idx = editingPage.value.incidents.findIndex((i) => i.id === savedInc.id);
      if (idx >= 0) {
        editingPage.value.incidents[idx] = savedInc;
      } else {
        editingPage.value.incidents.unshift(savedInc);
      }
      showIncidentModal.value = false;
      showToast('Incident updated');
    }
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to save incident', true);
  }
};

const deleteIncident = async (incidentId: string) => {
  if (!editingPage.value) return;
  try {
    await axios.delete(`/api/v1/status-pages/incidents/${incidentId}`);
    editingPage.value.incidents = editingPage.value.incidents?.filter((i) => i.id !== incidentId) || [];
    showToast('Incident removed');
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to remove incident', true);
  }
};

// Delete Page
const confirmDeletePage = (page: StatusPage) => {
  pageToDelete.value = page;
  showDeleteModal.value = true;
};

const executeDelete = async () => {
  if (!pageToDelete.value) return;
  deleting.value = true;
  try {
    await axios.delete(`/api/v1/status-pages/${pageToDelete.value.id}`);
    showToast(`Status page "${pageToDelete.value.title}" deleted.`);
    showDeleteModal.value = false;
    if (editingPage.value?.id === pageToDelete.value.id) {
      showEditDrawer.value = false;
    }
    await fetchPages();
  } catch (err: any) {
    showToast(err.response?.data?.error || 'Failed to delete status page', true);
  } finally {
    deleting.value = false;
  }
};

const getItemsForGroup = (groupId: string) => {
  return editingPage.value?.items?.filter((it) => it.groupId === groupId) || [];
};

const getUngroupedItems = computed(() => {
  return editingPage.value?.items?.filter((it) => !it.groupId) || [];
});

onMounted(() => {
  fetchPages();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Feedback Toast -->
    <transition
      enter-active-class="transform transition ease-out duration-200"
      enter-from-class="translate-y-2 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition ease-in duration-150"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="toastMessage"
        class="fixed bottom-6 right-6 z-50 flex items-center gap-2.5 px-4 py-2.5 rounded-xl text-xs font-semibold shadow-xl border backdrop-blur-md"
        :class="toastMessage.isError ? 'bg-rose-900/90 text-rose-100 border-rose-700/60' : 'bg-slate-900/95 text-white border-emerald-500/50'"
      >
        <span class="w-2 h-2 rounded-full" :class="toastMessage.isError ? 'bg-rose-500' : 'bg-emerald-400'"></span>
        <span>{{ toastMessage.text }}</span>
      </div>
    </transition>

    <!-- Standard Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Status Pages
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Publish service availability and monitor real-time infrastructure health.
        </p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <button
          @click="fetchPages"
          :disabled="loading"
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-800 text-xs font-medium text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-50 dark:hover:bg-slate-800/60 transition cursor-pointer disabled:opacity-50"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          <span>Refresh</span>
        </button>
        <button
          @click="openCreateModal"
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold transition cursor-pointer shadow-xs"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>New Status Page</span>
        </button>
      </div>
    </div>

    <!-- Loading Skeleton Grid (Instant Visual Feedback) -->
    <div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5 animate-pulse">
      <div v-for="i in 3" :key="i" class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 space-y-4">
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-2 flex-1">
            <div class="h-4 bg-slate-200 dark:bg-slate-800 rounded-md w-3/4"></div>
            <div class="h-3 bg-slate-100 dark:bg-slate-800/60 rounded-md w-1/2"></div>
          </div>
          <div class="h-5 bg-slate-200 dark:bg-slate-800 rounded-full w-14"></div>
        </div>
        <div class="h-3 bg-slate-100 dark:bg-slate-800/60 rounded-md w-full"></div>
        <div class="flex items-center gap-3 pt-3 border-t border-slate-100 dark:border-slate-800/80">
          <div class="h-3 bg-slate-100 dark:bg-slate-800/60 rounded-md w-16"></div>
          <div class="h-3 bg-slate-100 dark:bg-slate-800/60 rounded-md w-20"></div>
        </div>
        <div class="flex items-center justify-between pt-3 border-t border-slate-100 dark:border-slate-800/80">
          <div class="h-7 bg-slate-200 dark:bg-slate-800 rounded-lg w-20"></div>
          <div class="h-7 bg-slate-100 dark:bg-slate-800/60 rounded-lg w-16"></div>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-else-if="pages.length === 0"
      class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-12 text-center space-y-4"
    >
      <div class="w-14 h-14 rounded-full bg-slate-100 dark:bg-slate-800/80 text-slate-400 flex items-center justify-center mx-auto">
        <Activity class="w-7 h-7" />
      </div>
      <div class="space-y-1">
        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-200">No status pages found</h3>
        <p class="text-xs text-slate-500 dark:text-slate-400 max-w-sm mx-auto">
          Create a public or private status page to showcase the availability of your services, clusters, and network.
        </p>
      </div>
      <button
        @click="openCreateModal"
        class="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer shadow-xs"
      >
        <Plus class="w-3.5 h-3.5" />
        <span>Create New Status Page</span>
      </button>
    </div>

    <!-- Status Pages Card Grid -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
      <div
        v-for="page in pages"
        :key="page.id"
        class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 shadow-xs hover:border-blue-500/40 transition flex flex-col justify-between space-y-4 group"
      >
        <div class="space-y-3">
          <!-- Card Header & Badges -->
          <div class="flex items-start justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white group-hover:text-blue-500 transition">
                {{ page.title }}
              </h3>
              <div class="flex items-center gap-1.5 mt-1 text-slate-500 text-xs">
                <span class="font-mono text-[11px] text-slate-600 dark:text-slate-400">/status/{{ page.slug }}</span>
                <button
                  @click="copyPageLink(page.slug)"
                  title="Copy Link"
                  class="p-1 hover:text-slate-900 dark:hover:text-white rounded cursor-pointer transition"
                >
                  <Check v-if="copiedSlug === page.slug" class="w-3 h-3 text-emerald-500" />
                  <Copy v-else class="w-3 h-3 text-slate-400" />
                </button>
              </div>
            </div>
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-bold tracking-wide shrink-0"
              :class="page.isPublic ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20' : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20'"
            >
              {{ page.isPublic ? 'PUBLIC' : 'PRIVATE' }}
            </span>
          </div>

          <!-- Description snippet -->
          <p class="text-xs text-slate-500 dark:text-slate-400 line-clamp-2">
            {{ page.description || 'No description provided' }}
          </p>

          <!-- Stats chips -->
          <div class="flex items-center gap-3 pt-2 text-[11px] text-slate-500 dark:text-slate-400 border-t border-slate-100 dark:border-slate-800/80">
            <span class="flex items-center gap-1">
              <Layers class="w-3.5 h-3.5 text-slate-400" />
              <span>{{ page.items?.length || 0 }} Services</span>
            </span>
            <span class="flex items-center gap-1">
              <Clock class="w-3.5 h-3.5 text-slate-400" />
              <span>{{ page.refreshInterval }}s Refresh</span>
            </span>
            <span v-if="page.incidents && page.incidents.length > 0" class="flex items-center gap-1 text-amber-500">
              <AlertTriangle class="w-3.5 h-3.5" />
              <span>{{ page.incidents.length }} Incidents</span>
            </span>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="flex items-center justify-between gap-2 pt-3 border-t border-slate-100 dark:border-slate-800/80">
          <div class="flex items-center gap-1.5">
            <button
              @click="openEditDrawer(page)"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 dark:bg-[#1a2133] hover:bg-slate-200 dark:hover:bg-[#252f48] text-slate-700 dark:text-slate-200 text-xs font-semibold transition cursor-pointer"
            >
              <Edit3 class="w-3.5 h-3.5 text-slate-400" />
              <span>Configure</span>
            </button>
            <a
              :href="`/status/${page.slug}`"
              target="_blank"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-50 dark:hover:bg-slate-800/50 text-xs font-medium transition cursor-pointer"
            >
              <ExternalLink class="w-3.5 h-3.5 text-slate-400" />
              <span>Open</span>
            </a>
          </div>
          <button
            @click="confirmDeletePage(page)"
            class="p-1.5 text-slate-400 hover:text-rose-500 hover:bg-rose-500/10 rounded-lg transition cursor-pointer"
            title="Delete Status Page"
          >
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL: Add New Status Page (Step 1)                                   -->
    <!-- ===================================================================== -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-6 space-y-5">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <h3 class="text-base font-bold text-slate-900 dark:text-white">Add New Status Page</h3>
          <button @click="showCreateModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="submitCreatePage" class="space-y-4">
          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Status Page Name</label>
            <input
              v-model="newPageForm.title"
              @input="handleTitleInput"
              type="text"
              required
              placeholder="e.g. Infrastructure Monitoring"
              class="w-full px-3.5 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden focus:border-blue-500 transition"
            />
          </div>

          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Slug URL</label>
            <div class="flex items-center rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] px-3 py-2 text-xs">
              <span class="text-slate-400 select-none font-mono">/status/</span>
              <input
                v-model="newPageForm.slug"
                @input="handleSlugManualInput"
                type="text"
                required
                placeholder="monitoring"
                class="w-full bg-transparent text-slate-900 dark:text-white focus:outline-hidden font-mono"
              />
            </div>
            <ul class="text-[11px] text-slate-500 dark:text-slate-400 mt-2 space-y-1 list-disc pl-4">
              <li>Supported characters: <code class="text-blue-500">a-z</code>, <code class="text-blue-500">0-9</code>, <code class="text-blue-500">-</code></li>
              <li>Special keyword <code class="text-emerald-500">default</code>: will be displayed directly at root route if configured.</li>
            </ul>
          </div>

          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Visibility Access</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                @click="newPageForm.isPublic = true"
                class="flex items-center justify-center gap-2 p-2.5 rounded-xl border text-xs font-semibold cursor-pointer transition"
                :class="newPageForm.isPublic ? 'bg-emerald-500/10 border-emerald-500/40 text-emerald-600 dark:text-emerald-400' : 'border-slate-200 dark:border-slate-800 text-slate-500'"
              >
                <Globe class="w-4 h-4" />
                <span>Public (No Login Required)</span>
              </button>
              <button
                type="button"
                @click="newPageForm.isPublic = false"
                class="flex items-center justify-center gap-2 p-2.5 rounded-xl border text-xs font-semibold cursor-pointer transition"
                :class="!newPageForm.isPublic ? 'bg-amber-500/10 border-amber-500/40 text-amber-600 dark:text-amber-400' : 'border-slate-200 dark:border-slate-800 text-slate-500'"
              >
                <Lock class="w-4 h-4" />
                <span>Private (Login Required)</span>
              </button>
            </div>
          </div>

          <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-[#1b2234]">
            <button
              type="button"
              @click="showCreateModal = false"
              class="px-4 py-2 text-xs font-medium text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="saving"
              class="px-5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-bold transition cursor-pointer disabled:opacity-50 shadow-xs"
            >
              {{ saving ? 'Saving...' : 'Next' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- DRAWER/MODAL: Status Page Editor & Builder                            -->
    <!-- ===================================================================== -->
    <div
      v-if="showEditDrawer && editingPage"
      class="fixed inset-0 z-50 flex items-center justify-center p-2 sm:p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#0f131f] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-5xl h-[92vh] flex flex-col shadow-2xl overflow-hidden">
        <!-- Top Toolbar -->
        <div class="px-5 py-3.5 border-b border-slate-200 dark:border-[#1f283d] flex items-center justify-between shrink-0 bg-slate-50/50 dark:bg-[#131826]">
          <div class="flex items-center gap-3">
            <h2 class="text-sm font-bold text-slate-900 dark:text-white">
              Edit Status Page: <span class="text-blue-500">{{ editingPage.title }}</span>
            </h2>
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-bold"
              :class="editingPage.isPublic ? 'bg-emerald-500/10 text-emerald-500' : 'bg-amber-500/10 text-amber-500'"
            >
              {{ editingPage.isPublic ? 'PUBLIC' : 'PRIVATE' }}
            </span>
          </div>
          <div class="flex items-center gap-2">
            <a
              :href="`/status/${editingPage.slug}`"
              target="_blank"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-800 text-xs text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition"
            >
              <ExternalLink class="w-3.5 h-3.5 text-slate-400" />
              <span>Open Public Page</span>
            </a>
            <button
              @click="savePageConfig"
              :disabled="saving"
              class="flex items-center gap-1.5 px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer shadow-xs disabled:opacity-50"
            >
              <span>{{ saving ? 'Saving...' : 'Save Changes' }}</span>
            </button>
            <button
              @click="showEditDrawer = false"
              class="p-1.5 text-slate-400 hover:text-slate-700 dark:hover:text-white rounded-lg cursor-pointer"
            >
              <X class="w-5 h-5" />
            </button>
          </div>
        </div>

        <!-- Builder Body (Split Left Settings / Right Monitors) -->
        <div class="flex-1 overflow-y-auto grid grid-cols-1 lg:grid-cols-12 divide-y lg:divide-y-0 lg:divide-x divide-slate-200 dark:divide-[#1f283d]">
          <!-- Left Column: Page Settings (4 Cols) -->
          <div class="lg:col-span-4 p-5 space-y-4 overflow-y-auto bg-slate-50/30 dark:bg-[#111624]">
            <h4 class="text-xs font-bold uppercase tracking-wider text-slate-400">Page Settings</h4>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Page Title</label>
              <input
                v-model="editingPage.title"
                type="text"
                class="w-full px-3 py-1.5 text-xs rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
              />
            </div>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Slug URL</label>
              <div class="flex items-center rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] px-2.5 py-1.5 text-xs font-mono">
                <span class="text-slate-400">/status/</span>
                <input
                  v-model="editingPage.slug"
                  type="text"
                  class="w-full bg-transparent text-slate-900 dark:text-white focus:outline-hidden"
                />
              </div>
            </div>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Description</label>
              <textarea
                v-model="editingPage.description"
                rows="3"
                placeholder="System status, service notes, or maintenance announcements..."
                class="w-full px-3 py-2 text-xs rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden resize-none"
              ></textarea>
            </div>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Footer Text</label>
              <input
                v-model="editingPage.footerText"
                type="text"
                placeholder="Powered by Hephaestus Control Panel"
                class="w-full px-3 py-1.5 text-xs rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
              />
            </div>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Refresh Interval</label>
              <select
                v-model="editingPage.refreshInterval"
                class="w-full px-3 py-1.5 text-xs rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
              >
                <option :value="30">30 Seconds</option>
                <option :value="60">60 Seconds (Recommended)</option>
                <option :value="120">120 Seconds (2 Minutes)</option>
                <option :value="300">300 Seconds (5 Minutes)</option>
              </select>
              <p class="text-[10px] text-slate-400 mt-1">Status page will automatically refresh live metrics periodically.</p>
            </div>

            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1">Access Visibility</label>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  @click="editingPage.isPublic = true"
                  class="flex-1 py-1.5 text-xs font-semibold rounded-lg border transition cursor-pointer"
                  :class="editingPage.isPublic ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/40' : 'border-slate-200 dark:border-slate-800 text-slate-400'"
                >
                  Public
                </button>
                <button
                  type="button"
                  @click="editingPage.isPublic = false"
                  class="flex-1 py-1.5 text-xs font-semibold rounded-lg border transition cursor-pointer"
                  :class="!editingPage.isPublic ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/40' : 'border-slate-200 dark:border-slate-800 text-slate-400'"
                >
                  Private (Auth)
                </button>
              </div>
            </div>

            <div class="pt-2">
              <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-700 dark:text-slate-300">
                <input type="checkbox" v-model="editingPage.showTags" class="rounded border-slate-700 text-blue-600" />
                <span>Show tags and latency metrics</span>
              </label>
            </div>
          </div>

          <!-- Right Column: Incidents & Monitors Groups (8 Cols) -->
          <div class="lg:col-span-8 p-5 space-y-6 overflow-y-auto">
            <!-- Incidents Section -->
            <div class="space-y-3">
              <div class="flex items-center justify-between">
                <div>
                  <h4 class="text-xs font-bold uppercase tracking-wider text-slate-400">Incidents & Maintenance</h4>
                  <p class="text-[11px] text-slate-500">Display active incident notices or scheduled maintenance banners.</p>
                </div>
                <button
                  @click="openAddIncidentModal"
                  class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-200 text-xs font-semibold hover:bg-slate-200 dark:hover:bg-slate-700 transition cursor-pointer"
                >
                  <Plus class="w-3 h-3" />
                  <span>New Incident</span>
                </button>
              </div>

              <!-- Incidents List -->
              <div v-if="editingPage.incidents && editingPage.incidents.length > 0" class="space-y-2">
                <div
                  v-for="inc in editingPage.incidents"
                  :key="inc.id"
                  class="p-3 rounded-xl border flex items-start justify-between gap-3"
                  :class="inc.isActive ? 'bg-amber-500/5 border-amber-500/30' : 'bg-slate-50 dark:bg-slate-900/40 border-slate-200 dark:border-slate-800 opacity-60'"
                >
                  <div class="space-y-1">
                    <div class="flex items-center gap-2">
                      <span
                        class="px-1.5 py-0.5 rounded text-[9px] font-bold uppercase"
                        :class="inc.severity === 'critical' ? 'bg-rose-500/20 text-rose-500' : 'bg-amber-500/20 text-amber-500'"
                      >
                        {{ inc.severity }}
                      </span>
                      <span class="text-xs font-bold text-slate-800 dark:text-slate-200">{{ inc.title }}</span>
                      <span class="text-[10px] text-slate-400 capitalize">({{ inc.status }})</span>
                    </div>
                    <p class="text-xs text-slate-600 dark:text-slate-400">{{ inc.message }}</p>
                  </div>
                  <button
                    @click="deleteIncident(inc.id)"
                    class="text-slate-400 hover:text-rose-500 p-1 cursor-pointer"
                    title="Delete Incident"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
              <div v-else class="text-center py-4 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl text-xs text-slate-400">
                No active incidents currently.
              </div>
            </div>

            <!-- Groups and Monitored Services -->
            <div class="space-y-4 pt-2 border-t border-slate-200 dark:border-[#1f283d]">
              <div class="flex items-center justify-between">
                <div>
                  <h4 class="text-xs font-bold uppercase tracking-wider text-slate-400">Monitored Services</h4>
                  <p class="text-[11px] text-slate-500">Group and connect live services from HCP data sources.</p>
                </div>
                <div class="flex items-center gap-2">
                  <input
                    v-model="newGroupName"
                    placeholder="New group name (e.g. Core Network)"
                    class="px-2.5 py-1 text-xs rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
                    @keyup.enter="addGroup"
                  />
                  <button
                    @click="addGroup"
                    class="px-3 py-1 bg-slate-200 dark:bg-slate-800 hover:bg-slate-300 dark:hover:bg-slate-700 text-xs font-bold rounded-lg transition cursor-pointer"
                  >
                    Add Group
                  </button>
                </div>
              </div>

              <!-- Group Cards -->
              <div class="space-y-4">
                <div
                  v-for="group in editingPage.groups"
                  :key="group.id"
                  class="border border-slate-200 dark:border-[#1f283d] rounded-xl p-4 bg-white dark:bg-[#111624] space-y-3"
                >
                  <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-2.5">
                    <div class="flex items-center gap-2">
                      <Layers class="w-4 h-4 text-blue-500" />
                      <h5 class="text-xs font-bold text-slate-900 dark:text-white">{{ group.name }}</h5>
                      <span class="text-[10px] text-slate-400">({{ getItemsForGroup(group.id).length }} monitors)</span>
                    </div>
                    <div class="flex items-center gap-2">
                      <button
                        @click="openAddItemModal(group.id)"
                        class="flex items-center gap-1 px-2 py-1 text-[11px] font-semibold bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-300 hover:bg-blue-100 rounded-md transition cursor-pointer"
                      >
                        <Plus class="w-3 h-3" />
                        <span>Add Monitor</span>
                      </button>
                      <button
                        @click="deleteGroup(group.id)"
                        class="text-slate-400 hover:text-rose-500 p-1 cursor-pointer"
                        title="Delete Group"
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  <!-- Items inside group -->
                  <div v-if="getItemsForGroup(group.id).length > 0" class="space-y-1.5">
                    <div
                      v-for="item in getItemsForGroup(group.id)"
                      :key="item.id"
                      class="flex items-center justify-between p-2 rounded-lg bg-slate-50 dark:bg-[#0c0f17] border border-slate-200/60 dark:border-slate-800/60 text-xs"
                    >
                      <div class="flex items-center gap-2.5">
                        <span class="w-2 h-2 rounded-full bg-emerald-500"></span>
                        <span class="font-semibold text-slate-800 dark:text-slate-200">{{ item.name }}</span>
                        <span class="px-1.5 py-0.5 rounded text-[9px] font-mono uppercase bg-slate-200 dark:bg-slate-800 text-slate-600 dark:text-slate-400">
                          {{ item.sourceType }}
                        </span>
                        <span v-if="item.description" class="text-[11px] text-slate-400">- {{ item.description }}</span>
                      </div>
                      <button
                        @click="deleteItem(item.id)"
                        class="text-slate-400 hover:text-rose-500 p-1 cursor-pointer"
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                  <div v-else class="text-center py-2 text-[11px] text-slate-400">
                    No monitors in this group yet.
                  </div>
                </div>

                <!-- Ungrouped Items -->
                <div class="border border-dashed border-slate-200 dark:border-[#1f283d] rounded-xl p-4 bg-slate-50/50 dark:bg-[#0c0f17]/50 space-y-3">
                  <div class="flex items-center justify-between">
                    <h5 class="text-xs font-semibold text-slate-600 dark:text-slate-400">Ungrouped Services</h5>
                    <button
                      @click="openAddItemModal(null)"
                      class="flex items-center gap-1 px-2.5 py-1 text-[11px] font-semibold bg-slate-200 dark:bg-slate-800 hover:bg-slate-300 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-md transition cursor-pointer"
                    >
                      <Plus class="w-3 h-3" />
                      <span>Add Monitor</span>
                    </button>
                  </div>
                  <div v-if="getUngroupedItems.length > 0" class="space-y-1.5">
                    <div
                      v-for="item in getUngroupedItems"
                      :key="item.id"
                      class="flex items-center justify-between p-2 rounded-lg bg-white dark:bg-[#111624] border border-slate-200 dark:border-slate-800 text-xs"
                    >
                      <div class="flex items-center gap-2.5">
                        <span class="w-2 h-2 rounded-full bg-emerald-500"></span>
                        <span class="font-semibold text-slate-800 dark:text-slate-200">{{ item.name }}</span>
                        <span class="px-1.5 py-0.5 rounded text-[9px] font-mono uppercase bg-slate-200 dark:bg-slate-800 text-slate-600 dark:text-slate-400">
                          {{ item.sourceType }}
                        </span>
                      </div>
                      <button
                        @click="deleteItem(item.id)"
                        class="text-slate-400 hover:text-rose-500 p-1 cursor-pointer"
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                  <div v-else class="text-center py-2 text-[11px] text-slate-400">
                    No ungrouped services.
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL: Add Monitored Service (Multi-Source Picker)                    -->
    <!-- ===================================================================== -->
    <div
      v-if="showItemModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Add Monitored Service</h3>
          <button @click="showItemModal = false" class="text-slate-400 hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="submitSaveItem" class="space-y-4">
          <!-- Source Type Selector -->
          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Select Data Source</label>
            <div class="grid grid-cols-3 gap-2">
              <button
                type="button"
                @click="itemForm.sourceType = 'topology'; itemForm.sourceId = ''"
                class="flex items-center gap-1.5 p-2 rounded-lg border text-[11px] font-semibold transition cursor-pointer"
                :class="itemForm.sourceType === 'topology' ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-600 dark:text-blue-300' : 'border-slate-200 dark:border-slate-800 text-slate-500'"
              >
                <Network class="w-3.5 h-3.5" />
                <span>Topology</span>
              </button>
              <button
                type="button"
                @click="itemForm.sourceType = 'opensearch'; itemForm.sourceId = ''"
                class="flex items-center gap-1.5 p-2 rounded-lg border text-[11px] font-semibold transition cursor-pointer"
                :class="itemForm.sourceType === 'opensearch' ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-600 dark:text-blue-300' : 'border-slate-200 dark:border-slate-800 text-slate-500'"
              >
                <Database class="w-3.5 h-3.5" />
                <span>OpenSearch</span>
              </button>
              <button
                type="button"
                @click="itemForm.sourceType = 'prometheus'; itemForm.sourceId = ''"
                class="flex items-center gap-1.5 p-2 rounded-lg border text-[11px] font-semibold transition cursor-pointer"
                :class="itemForm.sourceType === 'prometheus' ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-600 dark:text-blue-300' : 'border-slate-200 dark:border-slate-800 text-slate-500'"
              >
                <Activity class="w-3.5 h-3.5" />
                <span>Prometheus</span>
              </button>
              <button
                type="button"
                @click="itemForm.sourceType = 'grafana'; itemForm.sourceId = ''"
                class="flex items-center gap-1.5 p-2 rounded-lg border text-[11px] font-semibold transition cursor-pointer"
                :class="itemForm.sourceType === 'grafana' ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-600 dark:text-blue-300' : 'border-slate-200 dark:border-slate-800 text-slate-500'"
              >
                <BarChart2 class="w-3.5 h-3.5" />
                <span>Grafana</span>
              </button>
              <button
                type="button"
                @click="itemForm.sourceType = 'remote_server'; itemForm.sourceId = ''"
                class="flex items-center gap-1.5 p-2 rounded-lg border text-[11px] font-semibold transition cursor-pointer"
                :class="itemForm.sourceType === 'remote_server' ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-500 text-blue-600 dark:text-blue-300' : 'border-slate-200 dark:border-slate-800 text-slate-500'"
              >
                <Server class="w-3.5 h-3.5" />
                <span>SSH Server</span>
              </button>
            </div>
          </div>

          <!-- Target Selector -->
          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Target Device / Connection</label>
            <select
              v-model="itemForm.sourceId"
              @change="onSourceSelect"
              class="w-full px-3 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
            >
              <option value="">-- Select from registered targets --</option>
              <option v-for="opt in filteredSourceOptions" :key="opt.id" :value="opt.id">
                {{ opt.name }} [{{ opt.detail }}]
              </option>
            </select>
          </div>

          <!-- Display Name -->
          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Display Name on Status Page</label>
            <input
              v-model="itemForm.name"
              type="text"
              required
              placeholder="e.g. Core Switch Rack A"
              class="w-full px-3 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
            />
          </div>

          <!-- Description -->
          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Short Description (Optional)</label>
            <input
              v-model="itemForm.description"
              type="text"
              placeholder="e.g. Primary Gateway to ISP 1"
              class="w-full px-3 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
            />
          </div>

          <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-[#1b2234]">
            <button
              type="button"
              @click="showItemModal = false"
              class="px-3 py-1.5 text-xs text-slate-500 hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-bold transition cursor-pointer"
            >
              Add Service
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL: Create / Edit Incident                                         -->
    <!-- ===================================================================== -->
    <div
      v-if="showIncidentModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Create Incident / Maintenance</h3>
          <button @click="showIncidentModal = false" class="text-slate-400 hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="submitSaveIncident" class="space-y-4">
          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Incident Title</label>
            <input
              v-model="incidentForm.title"
              type="text"
              required
              placeholder="e.g. Core Switch Firmware Upgrade"
              class="w-full px-3 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Incident Status</label>
              <select
                v-model="incidentForm.status"
                class="w-full px-3 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
              >
                <option value="investigating">Investigating</option>
                <option value="identified">Identified</option>
                <option value="monitoring">Monitoring</option>
                <option value="maintenance">Maintenance</option>
                <option value="resolved">Resolved</option>
              </select>
            </div>
            <div>
              <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Impact Severity</label>
              <select
                v-model="incidentForm.severity"
                class="w-full px-3 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden"
              >
                <option value="info">Informational</option>
                <option value="minor">Minor Degradation</option>
                <option value="major">Major Outage</option>
                <option value="critical">Critical Downtime</option>
              </select>
            </div>
          </div>

          <div>
            <label class="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1.5">Incident Details Message</label>
            <textarea
              v-model="incidentForm.message"
              rows="3"
              required
              placeholder="Detailed explanation of the issue or scheduled maintenance window..."
              class="w-full px-3 py-2 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0c0f17] text-slate-900 dark:text-white focus:outline-hidden resize-none"
            ></textarea>
          </div>

          <div>
            <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-700 dark:text-slate-300">
              <input type="checkbox" v-model="incidentForm.isActive" class="rounded border-slate-700 text-blue-600" />
              <span>Incident is currently active (Show public banner)</span>
            </label>
          </div>

          <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-[#1b2234]">
            <button
              type="button"
              @click="showIncidentModal = false"
              class="px-3 py-1.5 text-xs text-slate-500 hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-bold transition cursor-pointer shadow-xs"
            >
              Save Incident
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- STANDARD DELETE CONFIRMATION MODAL (SESUAI AGENTS.MD)                -->
    <!-- ===================================================================== -->
    <div
      v-if="showDeleteModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Status Page?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ pageToDelete?.title }}</strong>? This action cannot be undone.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDelete"
            :disabled="deleting"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50 shadow-xs"
          >
            {{ deleting ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
