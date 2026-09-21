<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue';
import axios from 'axios';
import {
  Plus,
  Trash2,
  Edit3,
  Download,
  RefreshCw,
  Search,
  X,
  TrendingUp,
  Activity,
  Database,
  ChevronDown,
  ChevronUp,
  LineChart,
  PieChart,
  BarChart2,
  Server,
  HardDrive,
  Cpu,
  ArrowLeft,
  Calendar,
} from 'lucide-vue-next';

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------
interface ReportWidget {
  id: string;
  reportId: string;
  title: string;
  chartType: 'line' | 'area' | 'bar' | 'pie' | 'donut';
  sourceType: 'prometheus' | 'grafana' | 'opensearch';
  sourceConfig: {
    targetHost?: string;
    metricPreset?: string;
    query?: string;
    indexPattern?: string;
    module?: string;
    aggregation?: string;
    colorPalette?: string;
  };
  timeRange: string;
  theme: string;
  widthPercent: number; // 50 or 100
  sortOrder: number;
  createdAt: string;

  // Runtime telemetry cache
  loading?: boolean;
  points?: Array<{ timestamp: string; label: string; value: number }>;
  summary?: {
    min: number;
    max: number;
    avg: number;
    current: number;
    total: number;
    count: number;
    unit: string;
  };
  showSummary?: boolean;
}

interface RawReport {
  id: string;
  name: string;
  description: string;
  mode: string;
  widgets?: ReportWidget[];
  createdAt: string;
  updatedAt: string;
}

// -----------------------------------------------------------------------------
// State Management
// -----------------------------------------------------------------------------
const reports = ref<RawReport[]>([]);
const activeReport = ref<RawReport | null>(null);
const currentView = ref<'list' | 'detail'>('list');
const searchQuery = ref('');
const loading = ref(false);
const notification = ref<{ text: string; type: 'success' | 'error' } | null>(null);

// Discovered Metadata
const discoveredHosts = ref<Array<{ id: string; name: string; host: string }>>([]);
const discoveredIndices = ref<string[]>([]);

// Modals
const showCreateModal = ref(false);
const showWidgetModal = ref(false);
const showDeleteModal = ref(false);
const isSaving = ref(false);
const isDeleting = ref(false);

// Forms
const reportForm = ref({
  id: '',
  name: '',
  description: '',
});

const editingWidgetId = ref<string | null>(null);
const widgetForm = ref<{
  title: string;
  sourceType: 'prometheus' | 'grafana' | 'opensearch';
  targetHost: string;
  metricPreset: string;
  query: string;
  indexPattern: string;
  module: string;
  chartType: 'line' | 'area' | 'bar' | 'pie' | 'donut';
  timeRange: string;
  aggregation: string;
  widthPercent: number;
  colorPalette: string;
}>({
  title: 'CPU Usage',
  sourceType: 'prometheus',
  targetHost: 'all',
  metricPreset: 'cpu',
  query: '',
  indexPattern: '*',
  module: 'CPU Load',
  chartType: 'line',
  timeRange: '24h',
  aggregation: 'daily',
  widthPercent: 50,
  colorPalette: 'emerald',
});

// Delete target
const itemToDelete = ref<{
  type: 'report' | 'widget';
  id: string;
  name: string;
} | null>(null);

// -----------------------------------------------------------------------------
// ECharts Instance Management
// -----------------------------------------------------------------------------
const chartRefs = new Map<string, HTMLElement>();
const chartInstances = new Map<string, any>();

const setChartRef = (id: string, el: any) => {
  if (el) {
    chartRefs.set(id, el as HTMLElement);
  } else {
    chartRefs.delete(id);
    const existing = chartInstances.get(id);
    if (existing) {
      existing.dispose();
      chartInstances.delete(id);
    }
  }
};

const getECharts = async () => {
  if ((window as any).echarts) return (window as any).echarts;
  try {
    const mod = await import('echarts');
    return mod.default || mod;
  } catch (e) {
    return (window as any).echarts || null;
  }
};

// -----------------------------------------------------------------------------
// Notification Auto-Dismiss (3s)
// -----------------------------------------------------------------------------
const showNotice = (text: string, type: 'success' | 'error' = 'success') => {
  notification.value = { text, type };
  setTimeout(() => {
    notification.value = null;
  }, 3000);
};

// -----------------------------------------------------------------------------
// Load Metadata (Remote Hosts & OpenSearch Indices)
// -----------------------------------------------------------------------------
const loadMetadata = async () => {
  try {
    const [hostsRes, indicesRes] = await Promise.allSettled([
      axios.get('/api/v1/remote-host'),
      axios.get('/api/v1/opensearch/indices'),
    ]);

    if (hostsRes.status === 'fulfilled' && hostsRes.value.data?.success && Array.isArray(hostsRes.value.data.data)) {
      discoveredHosts.value = hostsRes.value.data.data.map((h: any) => ({
        id: h.id || h.host,
        name: h.name || h.hostname || h.host,
        host: h.host || h.ip,
      }));
    }

    if (indicesRes.status === 'fulfilled' && indicesRes.value.data?.success && Array.isArray(indicesRes.value.data.data)) {
      discoveredIndices.value = indicesRes.value.data.data.map((idx: any) => idx.index || idx.name).filter(Boolean);
    }
  } catch (e) {
    console.warn('Metadata discovery error:', e);
  }
};

// -----------------------------------------------------------------------------
// Fetch & Manage Reports
// -----------------------------------------------------------------------------
const fetchReports = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/reports');
    if (res.data?.success && Array.isArray(res.data.data)) {
      reports.value = res.data.data;
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to fetch reports', 'error');
  } finally {
    loading.value = false;
  }
};

const openReport = async (report: RawReport) => {
  loading.value = true;
  try {
    const res = await axios.get(`/api/v1/reports/${report.id}`);
    if (res.data?.success && res.data.data) {
      activeReport.value = res.data.data;
      currentView.value = 'detail';
      await nextTick();
      await refreshAllPanels();
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to open report', 'error');
  } finally {
    loading.value = false;
  }
};

const openCreateReportModal = () => {
  reportForm.value = {
    id: '',
    name: '',
    description: '',
  };
  showCreateModal.value = true;
};

const saveReport = async () => {
  if (!reportForm.value.name.trim()) {
    showNotice('Report name is required', 'error');
    return;
  }

  isSaving.value = true;
  try {
    const payload = {
      name: reportForm.value.name.trim(),
      description: reportForm.value.description.trim(),
      mode: 'grid',
      headerConfig: {
        title: reportForm.value.name.trim(),
        subtitle: reportForm.value.description.trim(),
        showDate: true,
        logoText: 'HEPHAESTUS',
      },
    };

    const res = await axios.post('/api/v1/reports', payload);
    if (res.data?.success && res.data.data) {
      showNotice('Report created successfully', 'success');
      showCreateModal.value = false;
      await fetchReports();
      await openReport(res.data.data);
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to create report', 'error');
  } finally {
    isSaving.value = false;
  }
};

// -----------------------------------------------------------------------------
// Add / Edit Panel Widget
// -----------------------------------------------------------------------------
const openAddPanelModal = (presetMetric: string = 'cpu') => {
  editingWidgetId.value = null;
  widgetForm.value = {
    title: presetMetric === 'cpu' ? 'CPU Usage' : presetMetric === 'memory' ? 'Memory Used' : 'System Metrics',
    sourceType: 'prometheus',
    targetHost: discoveredHosts.value[0]?.host || 'all',
    metricPreset: presetMetric,
    query: '',
    indexPattern: discoveredIndices.value[0] || '*',
    module: 'CPU Load',
    chartType: 'line',
    timeRange: '24h',
    aggregation: 'daily',
    widthPercent: 50,
    colorPalette: 'emerald',
  };
  showWidgetModal.value = true;
};

const openEditPanelModal = (widget: ReportWidget) => {
  editingWidgetId.value = widget.id;
  widgetForm.value = {
    title: widget.title,
    sourceType: widget.sourceType || 'prometheus',
    targetHost: widget.sourceConfig?.targetHost || 'all',
    metricPreset: widget.sourceConfig?.metricPreset || 'cpu',
    query: widget.sourceConfig?.query || '',
    indexPattern: widget.sourceConfig?.indexPattern || '*',
    module: widget.sourceConfig?.module || 'CPU Load',
    chartType: widget.chartType || 'line',
    timeRange: widget.timeRange || '24h',
    aggregation: widget.sourceConfig?.aggregation || 'daily',
    widthPercent: widget.widthPercent || 50,
    colorPalette: widget.sourceConfig?.colorPalette || 'emerald',
  };
  showWidgetModal.value = true;
};

const savePanel = async () => {
  if (!activeReport.value) return;
  if (!widgetForm.value.title.trim()) {
    showNotice('Panel title is required', 'error');
    return;
  }

  isSaving.value = true;
  try {
    const payload = {
      reportId: activeReport.value.id,
      pageNumber: 1,
      title: widgetForm.value.title.trim(),
      chartType: widgetForm.value.chartType,
      sourceType: widgetForm.value.sourceType,
      sourceConfig: {
        targetHost: widgetForm.value.targetHost,
        metricPreset: widgetForm.value.metricPreset,
        query: widgetForm.value.query,
        indexPattern: widgetForm.value.indexPattern,
        module: widgetForm.value.module,
        aggregation: widgetForm.value.aggregation,
        colorPalette: widgetForm.value.colorPalette,
      },
      timeRange: widgetForm.value.timeRange,
      theme: 'default',
      widthPercent: widgetForm.value.widthPercent,
      sortOrder: (activeReport.value.widgets?.length || 0) + 1,
    };

    if (editingWidgetId.value) {
      await axios.put(`/api/v1/reports/widgets/${editingWidgetId.value}`, payload);
      showNotice('Panel updated successfully', 'success');
    } else {
      await axios.post(`/api/v1/reports/${activeReport.value.id}/widgets`, payload);
      showNotice('Panel added successfully', 'success');
    }

    showWidgetModal.value = false;
    await openReport(activeReport.value);
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to save panel', 'error');
  } finally {
    isSaving.value = false;
  }
};

// -----------------------------------------------------------------------------
// Delete Handlers (HCP Standard Confirmation Modal)
// -----------------------------------------------------------------------------
const confirmDeleteReport = (report: RawReport) => {
  itemToDelete.value = {
    type: 'report',
    id: report.id,
    name: report.name,
  };
  showDeleteModal.value = true;
};

const confirmDeleteWidget = (widget: ReportWidget) => {
  itemToDelete.value = {
    type: 'widget',
    id: widget.id,
    name: widget.title,
  };
  showDeleteModal.value = true;
};

const executeDelete = async () => {
  if (!itemToDelete.value) return;
  isDeleting.value = true;
  try {
    if (itemToDelete.value.type === 'report') {
      await axios.delete(`/api/v1/reports/${itemToDelete.value.id}`);
      showNotice('Report deleted successfully', 'success');
      showDeleteModal.value = false;
      if (activeReport.value?.id === itemToDelete.value.id) {
        currentView.value = 'list';
        activeReport.value = null;
      }
      await fetchReports();
    } else {
      await axios.delete(`/api/v1/reports/widgets/${itemToDelete.value.id}`);
      showNotice('Panel deleted successfully', 'success');
      showDeleteModal.value = false;
      if (activeReport.value) {
        await openReport(activeReport.value);
      }
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to delete item', 'error');
  } finally {
    isDeleting.value = false;
  }
};

// -----------------------------------------------------------------------------
// Query Panel Data & Render with Apache ECharts
// -----------------------------------------------------------------------------
const refreshAllPanels = async () => {
  if (!activeReport.value?.widgets || activeReport.value.widgets.length === 0) return;

  const echartsLib = await getECharts();

  const promises = activeReport.value.widgets.map(async (widget) => {
    widget.loading = true;
    try {
      const payload = {
        sourceType: widget.sourceType,
        sourceConfig: {
          query: widget.sourceConfig?.query || '',
          metric: widget.sourceConfig?.metricPreset || 'cpu',
          targetHost: widget.sourceConfig?.targetHost || 'all',
          indexPattern: widget.sourceConfig?.indexPattern || '*',
          module: widget.sourceConfig?.module || 'CPU Load',
          aggregation: widget.sourceConfig?.aggregation || 'daily',
        },
        timeRange: widget.timeRange || '24h',
        metricKey: widget.title,
      };

      const res = await axios.post('/api/v1/reports/query-data', payload);
      if (res.data?.success && res.data.data) {
        widget.points = res.data.data.points || [];
        widget.summary = res.data.data.summary || { min: 0, max: 0, avg: 0, current: 0, unit: '' };
      }
    } catch (err) {
      console.warn(`Query failed for widget ${widget.id}:`, err);
    } finally {
      widget.loading = false;
    }
  });

  await Promise.all(promises);
  await nextTick();

  // Render ECharts on each card
  if (echartsLib && activeReport.value?.widgets) {
    for (const widget of activeReport.value.widgets) {
      renderEChart(widget, echartsLib);
    }
  }
};

const formatPointLocalLabel = (p: { timestamp: string; label?: string }, timeRange?: string) => {
  if (!p) return '';
  let ts = (p.timestamp || '').trim();
  if (!ts) return p.label || '';

  if (/^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}$/.test(ts)) {
    ts = ts.replace(' ', 'T') + ':00Z';
  } else if (/^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}$/.test(ts)) {
    ts = ts.replace(' ', 'T') + 'Z';
  }

  const d = new Date(ts);
  if (isNaN(d.getTime())) {
    return p.label || p.timestamp;
  }

  if (timeRange === '7d' || timeRange === '30d') {
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  }

  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hour12: false });
};

const formatPointLocalTooltip = (p: { timestamp: string; label?: string }) => {
  if (!p) return '';
  let ts = (p.timestamp || '').trim();
  if (!ts) return p.label || '';

  if (/^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}$/.test(ts)) {
    ts = ts.replace(' ', 'T') + ':00Z';
  } else if (/^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}$/.test(ts)) {
    ts = ts.replace(' ', 'T') + 'Z';
  }

  const d = new Date(ts);
  if (isNaN(d.getTime())) {
    return p.label || p.timestamp;
  }

  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
};

const renderEChart = (widget: ReportWidget, echartsLib: any) => {
  const dom = chartRefs.get(widget.id);
  if (!dom || !echartsLib) return;

  let chart = chartInstances.get(widget.id);
  if (!chart) {
    chart = echartsLib.init(dom);
    chartInstances.set(widget.id, chart);
  }

  const isDark = document.documentElement.classList.contains('dark');
  const palette = widget.sourceConfig?.colorPalette || 'emerald';

  let primaryColor = '#10b981'; // emerald
  let gradientStop = 'rgba(16, 185, 129, 0.25)';

  if (palette === 'blue') {
    primaryColor = '#3b82f6';
    gradientStop = 'rgba(59, 130, 246, 0.25)';
  } else if (palette === 'amber') {
    primaryColor = '#f59e0b';
    gradientStop = 'rgba(245, 158, 11, 0.25)';
  } else if (palette === 'purple') {
    primaryColor = '#8b5cf6';
    gradientStop = 'rgba(139, 92, 246, 0.25)';
  }

  const textColor = isDark ? '#94a3b8' : '#64748b';
  const splitLineColor = isDark ? '#1e293b' : '#f1f5f9';

  const points = widget.points || [];
  const labels = points.map((p) => formatPointLocalLabel(p, widget.timeRange));
  const values = points.map((p) => p.value);

  let option: any = {};

  if (widget.chartType === 'pie' || widget.chartType === 'donut') {
    const pieData = points.slice(0, 6).map((p, idx) => ({
      name: formatPointLocalLabel(p, widget.timeRange) || `Point ${idx + 1}`,
      value: p.value,
    }));

    option = {
      tooltip: {
        trigger: 'item',
        formatter: `{b}: {c} ${widget.summary?.unit || ''} ({d}%)`,
      },
      legend: {
        bottom: '0%',
        left: 'center',
        textStyle: { color: textColor, fontSize: 11 },
      },
      series: [
        {
          name: widget.title,
          type: 'pie',
          radius: widget.chartType === 'donut' ? ['45%', '70%'] : '70%',
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 6,
            borderColor: isDark ? '#111624' : '#ffffff',
            borderWidth: 2,
          },
          label: {
            show: false,
            position: 'center',
          },
          emphasis: {
            label: {
              show: true,
              fontSize: 14,
              fontWeight: 'bold',
              color: isDark ? '#ffffff' : '#0f172a',
            },
          },
          data: pieData.length ? pieData : [{ name: 'No Data', value: 0 }],
        },
      ],
    };
  } else if (widget.chartType === 'bar') {
    option = {
      tooltip: {
        trigger: 'axis',
        backgroundColor: isDark ? '#0c101a' : '#ffffff',
        borderColor: isDark ? '#1f283d' : '#e2e8f0',
        textStyle: { color: isDark ? '#f8fafc' : '#0f172a', fontSize: 11 },
        formatter: (params: any) => {
          if (!params || !params.length) return '';
          const item = params[0];
          const pt = points[item.dataIndex];
          const timeLabel = pt ? formatPointLocalTooltip(pt) : item.name;
          const unit = widget.summary?.unit || '';
          return `<div style="font-family: inherit; font-size: 11px; line-height: 1.4;">
            <div style="opacity: 0.7; margin-bottom: 2px;">${timeLabel}</div>
            <div style="font-weight: 700; font-size: 13px;">${item.value} ${unit}</div>
          </div>`;
        },
      },
      grid: {
        left: 45,
        right: 15,
        top: 25,
        bottom: 30,
      },
      xAxis: {
        type: 'category',
        data: labels,
        axisLine: { lineStyle: { color: splitLineColor } },
        axisLabel: { color: textColor, fontSize: 10 },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: textColor, fontSize: 10 },
        splitLine: { lineStyle: { color: splitLineColor } },
      },
      series: [
        {
          name: widget.title,
          type: 'bar',
          barMaxWidth: 24,
          itemStyle: {
            color: primaryColor,
            borderRadius: [4, 4, 0, 0],
          },
          data: values,
        },
      ],
    };
  } else {
    // line or area
    const isArea = widget.chartType === 'area';
    option = {
      tooltip: {
        trigger: 'axis',
        backgroundColor: isDark ? '#0c101a' : '#ffffff',
        borderColor: isDark ? '#1f283d' : '#e2e8f0',
        textStyle: { color: isDark ? '#f8fafc' : '#0f172a', fontSize: 11 },
        formatter: (params: any) => {
          if (!params || !params.length) return '';
          const item = params[0];
          const pt = points[item.dataIndex];
          const timeLabel = pt ? formatPointLocalTooltip(pt) : item.name;
          const unit = widget.summary?.unit || '';
          return `<div style="font-family: inherit; font-size: 11px; line-height: 1.4;">
            <div style="opacity: 0.7; margin-bottom: 2px;">${timeLabel}</div>
            <div style="font-weight: 700; font-size: 13px;">${item.value} ${unit}</div>
          </div>`;
        },
      },
      grid: {
        left: 45,
        right: 15,
        top: 25,
        bottom: 30,
      },
      xAxis: {
        type: 'category',
        data: labels,
        axisLine: { lineStyle: { color: splitLineColor } },
        axisLabel: { color: textColor, fontSize: 10 },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: textColor, fontSize: 10 },
        splitLine: { lineStyle: { color: splitLineColor } },
      },
      series: [
        {
          name: widget.title,
          type: 'line',
          smooth: true,
          showSymbol: true,
          symbolSize: 6,
          lineStyle: {
            color: primaryColor,
            width: 2.5,
          },
          itemStyle: {
            color: primaryColor,
          },
          areaStyle: isArea
            ? {
                color: new echartsLib.graphic.LinearGradient(0, 0, 0, 1, [
                  { offset: 0, color: gradientStop },
                  { offset: 1, color: 'rgba(0, 0, 0, 0)' },
                ]),
              }
            : undefined,
          data: values,
        },
      ],
    };
  }

  chart.setOption(option, true);
};

// -----------------------------------------------------------------------------
// Download Panel Chart as PNG
// -----------------------------------------------------------------------------
const downloadPanelPng = (widget: ReportWidget) => {
  const chart = chartInstances.get(widget.id);
  if (!chart) {
    showNotice('Chart instance not ready for export', 'error');
    return;
  }

  const isDark = document.documentElement.classList.contains('dark');
  const dataUrl = chart.getDataURL({
    type: 'png',
    pixelRatio: 2,
    backgroundColor: isDark ? '#0f172a' : '#ffffff',
  });

  const link = document.createElement('a');
  link.href = dataUrl;
  link.download = `${widget.title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Date.now()}.png`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  showNotice('Chart PNG downloaded successfully', 'success');
};

// -----------------------------------------------------------------------------
// Filtered Reports Search
// -----------------------------------------------------------------------------
const filteredReports = computed(() => {
  if (!searchQuery.value.trim()) return reports.value;
  const q = searchQuery.value.toLowerCase();
  return reports.value.filter(
    (r) => r.name.toLowerCase().includes(q) || (r.description && r.description.toLowerCase().includes(q))
  );
});

// Resize handler
const onWindowResize = () => {
  chartInstances.forEach((chart) => {
    chart.resize();
  });
};

onMounted(async () => {
  window.addEventListener('resize', onWindowResize);
  await loadMetadata();
  await fetchReports();
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', onWindowResize);
  chartInstances.forEach((chart) => {
    chart.dispose();
  });
  chartInstances.clear();
});
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans text-slate-800 dark:text-slate-200">
    <!-- Notice Banner -->
    <div
      v-if="notification"
      :class="[
        notification.type === 'success'
          ? 'bg-emerald-50 dark:bg-emerald-950/40 border-emerald-300 dark:border-emerald-800 text-emerald-800 dark:text-emerald-300'
          : 'bg-rose-50 dark:bg-rose-950/40 border-rose-300 dark:border-rose-800 text-rose-800 dark:text-rose-300',
        'p-3 rounded-xl border text-xs font-medium flex items-center justify-between shadow-xs animate-in fade-in'
      ]"
    >
      <span>{{ notification.text }}</span>
      <button @click="notification = null" class="cursor-pointer hover:opacity-75">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- ===================================================================== -->
    <!-- VIEW 1: CATALOG LIST VIEW                                             -->
    <!-- ===================================================================== -->
    <template v-if="currentView === 'list'">
      <!-- Standard HCP Header (Strictly complies with AGENTS.md: No icon in H1) -->
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
        <div>
          <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
            Raw Data Report
          </h1>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            Create, manage, and monitor operational telemetry reports with Prometheus, Grafana, and OpenSearch.
          </p>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <button
            @click="openCreateReportModal"
            class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-xs transition flex items-center gap-1.5 cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Create Report</span>
          </button>
        </div>
      </div>

      <!-- Search and Filter Bar -->
      <div class="flex items-center justify-between gap-4 bg-white dark:bg-[#111624] p-3 rounded-xl border border-slate-200 dark:border-[#1f283d]">
        <div class="relative flex-1 max-w-sm">
          <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search report titles..."
            class="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-slate-800 dark:text-slate-200"
          />
        </div>
        <button
          @click="fetchReports"
          :disabled="loading"
          class="p-1.5 text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 border border-slate-200 dark:border-[#1f283d] rounded-lg hover:bg-slate-50 dark:hover:bg-[#151c2e] transition cursor-pointer"
          title="Refresh"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
        </button>
      </div>

      <!-- Report Cards Grid -->
      <div v-if="filteredReports.length > 0" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="rep in filteredReports"
          :key="rep.id"
          class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 hover:border-blue-300 dark:hover:border-blue-900/60 transition shadow-xs hover:shadow-sm flex flex-col justify-between group"
        >
          <div class="space-y-3">
            <div class="flex items-center justify-between gap-2">
              <div class="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-950/60 text-blue-600 dark:text-[#95CCDD] flex items-center justify-center">
                <TrendingUp class="w-4 h-4 text-slate-600 dark:text-slate-300" />
              </div>
              <span class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-50 dark:bg-emerald-950/50 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800">
                {{ rep.widgets?.length || 0 }} Panel(s)
              </span>
            </div>

            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-[#95CCDD] transition">
                {{ rep.name }}
              </h3>
              <p class="text-xs text-slate-500 dark:text-slate-400 mt-1 line-clamp-2">
                {{ rep.description || 'Operational telemetry report monitoring.' }}
              </p>
            </div>
          </div>

          <div class="pt-4 border-t border-slate-100 dark:border-[#1b2234] mt-4 flex items-center justify-between text-xs text-slate-400">
            <span class="text-[11px]">
              Created: {{ new Date(rep.createdAt).toLocaleDateString() }}
            </span>
            <div class="flex items-center gap-2">
              <button
                @click="openReport(rep)"
                class="px-2.5 py-1 text-xs font-semibold text-blue-600 dark:text-[#95CCDD] hover:bg-blue-50 dark:hover:bg-blue-950/40 rounded-md transition cursor-pointer"
              >
                Open Report &rarr;
              </button>
              <button
                @click="confirmDeleteReport(rep)"
                class="p-1 text-slate-400 hover:text-rose-500 transition cursor-pointer"
                title="Delete Report"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="text-center py-16 bg-white dark:bg-[#111624] border border-dashed border-slate-300 dark:border-[#1f283d] rounded-2xl space-y-3">
        <Activity class="w-10 h-10 mx-auto text-slate-300 dark:text-slate-600" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">No Reports Created Yet</h3>
        <p class="text-xs text-slate-500 max-w-sm mx-auto">
          Get started by creating your first report, then add Prometheus or Grafana telemetry panels.
        </p>
        <button
          @click="openCreateReportModal"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition cursor-pointer inline-flex items-center gap-1.5"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Create First Report</span>
        </button>
      </div>
    </template>

    <!-- ===================================================================== -->
    <!-- VIEW 2: REPORT DETAIL & PANELS GRID (Interactive Apache ECharts)       -->
    <!-- ===================================================================== -->
    <template v-else-if="currentView === 'detail' && activeReport">
      <!-- Report Header -->
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
        <div>
          <div class="flex items-center gap-2">
            <button
              @click="currentView = 'list'"
              class="p-1 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-[#151c2e] transition cursor-pointer"
              title="Back to All Reports"
            >
              <ArrowLeft class="w-4 h-4" />
            </button>
            <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
              {{ activeReport.name }}
            </h1>
          </div>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5 pl-6">
            Created: {{ new Date(activeReport.createdAt).toLocaleDateString() }} &bull; {{ activeReport.widgets?.length || 0 }} Metrics Panels Active
          </p>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <button
            @click="currentView = 'list'"
            class="px-3 py-1.5 border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-[#151c2e] transition cursor-pointer"
          >
            All Reports
          </button>
          <button
            @click="openAddPanelModal('cpu')"
            class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-xs transition flex items-center gap-1.5 cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Panel</span>
          </button>
          <button
            @click="refreshAllPanels"
            :disabled="loading"
            class="px-3 py-1.5 border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-[#151c2e] transition cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
            <span>Refresh Data</span>
          </button>
          <button
            @click="confirmDeleteReport(activeReport)"
            class="p-1.5 text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-950/40 rounded-lg border border-rose-200 dark:border-rose-900/50 transition cursor-pointer"
            title="Delete Report"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <!-- Panels Grid (50% Half Width / 100% Full Width) -->
      <div v-if="activeReport.widgets && activeReport.widgets.length > 0" class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div
          v-for="widget in activeReport.widgets"
          :key="widget.id"
          :class="[
            widget.widthPercent === 100 ? 'col-span-1 md:col-span-2' : 'col-span-1',
            'bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 shadow-xs space-y-4'
          ]"
        >
          <!-- Card Header (Reference Image Style) -->
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">{{ widget.title }}</h3>
            <div class="flex items-center gap-2 text-slate-400">
              <button
                @click="downloadPanelPng(widget)"
                class="hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
                title="Download Chart PNG"
              >
                <Download class="w-3.5 h-3.5" />
              </button>
              <button
                @click="openEditPanelModal(widget)"
                class="hover:text-blue-500 cursor-pointer"
                title="Edit Panel"
              >
                <Edit3 class="w-3.5 h-3.5" />
              </button>
              <button
                @click="confirmDeleteWidget(widget)"
                class="hover:text-rose-500 cursor-pointer"
                title="Delete Panel"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Chart Subtitle & Time Range -->
          <div class="text-center space-y-0.5">
            <h4 class="text-xs font-semibold text-slate-800 dark:text-slate-200">{{ widget.title }}</h4>
            <p class="text-[10px] text-slate-400">
              Time Range: Last {{ widget.timeRange }} &bull; {{ widget.sourceConfig?.aggregation || 'Daily' }} &bull; {{ widget.sourceType.toUpperCase() }}
            </p>
          </div>

          <!-- Apache ECharts Container -->
          <div :ref="(el) => setChartRef(widget.id, el)" class="w-full h-56 relative"></div>

          <!-- Legend Indicator -->
          <div class="flex items-center justify-center gap-2 text-xs text-slate-600 dark:text-slate-400 pt-1">
            <span
              :class="[
                widget.sourceConfig?.colorPalette === 'blue'
                  ? 'bg-blue-500'
                  : widget.sourceConfig?.colorPalette === 'amber'
                  ? 'bg-amber-500'
                  : widget.sourceConfig?.colorPalette === 'purple'
                  ? 'bg-purple-500'
                  : 'bg-emerald-500',
                'w-2.5 h-2.5 rounded-full shrink-0'
              ]"
            ></span>
            <span class="font-medium text-[11px]">
              {{ widget.sourceConfig?.targetHost && widget.sourceConfig.targetHost !== 'all' ? widget.sourceConfig.targetHost + ' - ' : '' }}{{ widget.title }}
            </span>
          </div>

          <!-- Collapsible "View Widget Summary" Accordion -->
          <div class="pt-3 border-t border-slate-100 dark:border-[#1b2234]">
            <button
              type="button"
              @click="widget.showSummary = !widget.showSummary"
              class="text-xs font-semibold text-emerald-600 dark:text-emerald-400 hover:text-emerald-500 flex items-center gap-1 cursor-pointer"
            >
              <component :is="widget.showSummary ? ChevronUp : ChevronDown" class="w-3.5 h-3.5" />
              <span>View Widget Summary</span>
            </button>

            <div
              v-if="widget.showSummary"
              class="mt-3 p-3 bg-slate-50 dark:bg-[#0c101a] rounded-xl border border-slate-200 dark:border-[#1f283d] grid grid-cols-2 sm:grid-cols-4 gap-2 text-center text-xs"
            >
              <div class="p-2 bg-white dark:bg-[#111624] rounded-lg border border-slate-200 dark:border-[#1f283d]">
                <span class="text-[10px] text-slate-400 block font-medium">Minimum</span>
                <strong class="text-slate-800 dark:text-slate-200 text-xs">{{ widget.summary?.min || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
              <div class="p-2 bg-white dark:bg-[#111624] rounded-lg border border-slate-200 dark:border-[#1f283d]">
                <span class="text-[10px] text-slate-400 block font-medium">Average</span>
                <strong class="text-slate-800 dark:text-slate-200 text-xs">{{ widget.summary?.avg || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
              <div class="p-2 bg-white dark:bg-[#111624] rounded-lg border border-slate-200 dark:border-[#1f283d]">
                <span class="text-[10px] text-slate-400 block font-medium">Peak / Maximum</span>
                <strong class="text-slate-800 dark:text-slate-200 text-xs">{{ widget.summary?.max || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
              <div class="p-2 bg-white dark:bg-[#111624] rounded-lg border border-slate-200 dark:border-[#1f283d]">
                <span class="text-[10px] text-slate-400 block font-medium">Current</span>
                <strong class="text-slate-800 dark:text-slate-200 text-xs">{{ widget.summary?.current || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty State (No Panels inside Report) -->
      <div v-else class="text-center py-16 bg-white dark:bg-[#111624] border border-dashed border-slate-300 dark:border-[#1f283d] rounded-2xl space-y-3">
        <BarChart2 class="w-10 h-10 mx-auto text-slate-300 dark:text-slate-600" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">No Panels Added to this Report</h3>
        <p class="text-xs text-slate-500 max-w-sm mx-auto">
          Add metric panels for CPU, Memory, Storage, or Logs to start visualizing data with Apache ECharts.
        </p>
        <button
          @click="openAddPanelModal('cpu')"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition cursor-pointer inline-flex items-center gap-1.5"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Panel</span>
        </button>
      </div>
    </template>

    <!-- ===================================================================== -->
    <!-- MODAL 1: CREATE REPORT                                                -->
    <!-- ===================================================================== -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between pb-3 border-b border-slate-100 dark:border-[#1b2234]">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Create New Report</h3>
          <button @click="showCreateModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div class="space-y-1">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Report Name</label>
            <input
              v-model="reportForm.name"
              type="text"
              placeholder="e.g. Production Server Health, Horus-Master"
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 focus:ring-1 focus:ring-blue-500"
              @keyup.enter="saveReport"
            />
          </div>

          <div class="space-y-1">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Description (Optional)</label>
            <textarea
              v-model="reportForm.description"
              rows="3"
              placeholder="Brief description of the monitored cluster or host telemetry..."
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 focus:ring-1 focus:ring-blue-500 leading-relaxed"
            ></textarea>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showCreateModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="saveReport"
            :disabled="isSaving"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ isSaving ? 'Creating...' : 'Create Report' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 2: ADD / EDIT PANEL WIDGET                                      -->
    <!-- ===================================================================== -->
    <div
      v-if="showWidgetModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-2xl shadow-2xl p-6 space-y-4 max-h-[90vh] overflow-y-auto">
        <div class="flex items-center justify-between pb-3 border-b border-slate-100 dark:border-[#1b2234]">
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">
              {{ editingWidgetId ? 'Edit Widget Panel' : 'Add Widget Panel' }}
            </h3>
            <p class="text-[11px] text-slate-500 mt-0.5">
              Configure telemetry provider, query target, and Apache ECharts visualization.
            </p>
          </div>
          <button @click="showWidgetModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-4 text-xs">
          <!-- Row 1: Widget Title & Chart Type -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Panel Widget Title</label>
              <input
                v-model="widgetForm.title"
                type="text"
                placeholder="e.g. CPU Usage, Memory Used"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Chart Type (Apache ECharts)</label>
              <select
                v-model="widgetForm.chartType"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="line">Line Chart</option>
                <option value="area">Area Chart</option>
                <option value="bar">Bar Chart</option>
                <option value="pie">Pie Chart</option>
                <option value="donut">Donut Chart</option>
              </select>
            </div>
          </div>

          <!-- Row 2: Datasource Selection (Prometheus vs Grafana vs OpenSearch) -->
          <div class="space-y-1.5">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Datasource Provider</label>
            <div class="grid grid-cols-3 gap-2">
              <label
                :class="[
                  widgetForm.sourceType === 'prometheus'
                    ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/30 text-blue-600 dark:text-[#95CCDD]'
                    : 'border-slate-200 dark:border-[#1f283d]',
                  'flex items-center gap-2 p-2.5 rounded-xl border cursor-pointer font-medium transition'
                ]"
              >
                <input type="radio" value="prometheus" v-model="widgetForm.sourceType" class="hidden" />
                <Activity class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400" />
                <div class="truncate">
                  <div class="text-xs font-semibold leading-tight">Prometheus</div>
                  <div class="text-[10px] text-slate-400 font-normal">Host Telemetry</div>
                </div>
              </label>

              <label
                :class="[
                  widgetForm.sourceType === 'grafana'
                    ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/30 text-blue-600 dark:text-[#95CCDD]'
                    : 'border-slate-200 dark:border-[#1f283d]',
                  'flex items-center gap-2 p-2.5 rounded-xl border cursor-pointer font-medium transition'
                ]"
              >
                <input type="radio" value="grafana" v-model="widgetForm.sourceType" class="hidden" />
                <TrendingUp class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400" />
                <div class="truncate">
                  <div class="text-xs font-semibold leading-tight">Grafana API</div>
                  <div class="text-[10px] text-slate-400 font-normal">Modules & Dashboards</div>
                </div>
              </label>

              <label
                :class="[
                  widgetForm.sourceType === 'opensearch'
                    ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/30 text-blue-600 dark:text-[#95CCDD]'
                    : 'border-slate-200 dark:border-[#1f283d]',
                  'flex items-center gap-2 p-2.5 rounded-xl border cursor-pointer font-medium transition'
                ]"
              >
                <input type="radio" value="opensearch" v-model="widgetForm.sourceType" class="hidden" />
                <Database class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400" />
                <div class="truncate">
                  <div class="text-xs font-semibold leading-tight">OpenSearch</div>
                  <div class="text-[10px] text-slate-400 font-normal">Log Aggregation</div>
                </div>
              </label>
            </div>
          </div>

          <!-- Row 3: Provider Specific Target Configuration -->
          <!-- Prometheus Config -->
          <div v-if="widgetForm.sourceType === 'prometheus'" class="p-3 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-xl space-y-3">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">Target Server / Host</label>
                <select
                  v-model="widgetForm.targetHost"
                  class="w-full px-3 py-2 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
                >
                  <option value="all">All Monitored Hosts</option>
                  <option v-for="h in discoveredHosts" :key="h.id" :value="h.host">
                    {{ h.name }} ({{ h.host }})
                  </option>
                </select>
              </div>

              <div class="space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">Metric Telemetry</label>
                <select
                  v-model="widgetForm.metricPreset"
                  @change="if (widgetForm.metricPreset === 'cpu') widgetForm.title = 'CPU Usage'; else if (widgetForm.metricPreset === 'memory') widgetForm.title = 'Memory Used'; else if (widgetForm.metricPreset === 'disk') widgetForm.title = 'Disk Storage'; else if (widgetForm.metricPreset === 'network') widgetForm.title = 'Network Traffic';"
                  class="w-full px-3 py-2 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
                >
                  <option value="cpu">CPU Usage (%)</option>
                  <option value="memory">Memory Used (%)</option>
                  <option value="disk">Disk Storage (%)</option>
                  <option value="network">Network Traffic (bps)</option>
                  <option value="load">System Load (1m avg)</option>
                  <option value="custom">Custom PromQL Expression</option>
                </select>
              </div>
            </div>

            <div v-if="widgetForm.metricPreset === 'custom'" class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Custom PromQL</label>
              <input
                v-model="widgetForm.query"
                type="text"
                placeholder="e.g. 100 - (avg(rate(node_cpu_seconds_total{mode='idle'}[5m])) * 100)"
                class="w-full px-3 py-2 font-mono text-[11px] bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              />
            </div>
          </div>

          <!-- Grafana Config -->
          <div v-else-if="widgetForm.sourceType === 'grafana'" class="p-3 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-xl space-y-3">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Target Module / Metric</label>
              <select
                v-model="widgetForm.module"
                @change="widgetForm.title = widgetForm.module"
                class="w-full px-3 py-2 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="CPU Load">CPU Load</option>
                <option value="Memory Used">Memory Used</option>
                <option value="Disk Storage">Disk Storage</option>
                <option value="Network Throughput">Network Throughput</option>
              </select>
            </div>
          </div>

          <!-- OpenSearch Config -->
          <div v-else class="p-3 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-xl space-y-3">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">Index Pattern</label>
                <select
                  v-if="discoveredIndices.length > 0"
                  v-model="widgetForm.indexPattern"
                  class="w-full px-3 py-2 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
                >
                  <option value="*">* (All Indices)</option>
                  <option v-for="idx in discoveredIndices" :key="idx" :value="idx">{{ idx }}</option>
                </select>
                <input
                  v-else
                  v-model="widgetForm.indexPattern"
                  type="text"
                  placeholder="*"
                  class="w-full px-3 py-2 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
                />
              </div>

              <div class="space-y-1">
                <label class="font-semibold text-slate-700 dark:text-slate-300">Filter Query</label>
                <input
                  v-model="widgetForm.query"
                  type="text"
                  placeholder="e.g. status:>=500 OR level:ERROR"
                  class="w-full px-3 py-2 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
                />
              </div>
            </div>
          </div>

          <!-- Row 4: Time Range, Interval / Aggregation, Panel Size & Color Palette -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Time Range</label>
              <select
                v-model="widgetForm.timeRange"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="1h">Last 1 Hour</option>
                <option value="24h">Last 24 Hours</option>
                <option value="7d">Last 7 Days</option>
                <option value="30d">Last 30 Days</option>
              </select>
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Data Interval</label>
              <select
                v-model="widgetForm.aggregation"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="actual">Actual (Raw Data)</option>
                <option value="daily">Average (Daily)</option>
                <option value="weekly">Weekly</option>
                <option value="monthly">Monthly</option>
              </select>
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Panel Width</label>
              <select
                v-model="widgetForm.widthPercent"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option :value="50">Half Width (50%)</option>
                <option :value="100">Full Width (100%)</option>
              </select>
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Color Palette</label>
              <select
                v-model="widgetForm.colorPalette"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="emerald">Emerald Green</option>
                <option value="blue">Ocean Blue</option>
                <option value="amber">Sunset Amber</option>
                <option value="purple">Purple Violet</option>
              </select>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showWidgetModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="savePanel"
            :disabled="isSaving"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ isSaving ? 'Saving...' : 'Save Panel' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 3: STANDARD DELETE CONFIRMATION MODAL (Strictly AGENTS.md)     -->
    <!-- ===================================================================== -->
    <div
      v-if="showDeleteModal && itemToDelete"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Icon Lingkaran Merah di Tengah Atas -->
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>

        <!-- Judul & Teks Penjelasan -->
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            Delete {{ itemToDelete.type === 'report' ? 'Report' : 'Panel' }}?
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ itemToDelete.name }}</strong>? This action cannot be undone.
          </p>
        </div>

        <!-- Tombol Aksi -->
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDelete"
            :disabled="isDeleting"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ isDeleting ? 'Deleting...' : 'Confirm Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
