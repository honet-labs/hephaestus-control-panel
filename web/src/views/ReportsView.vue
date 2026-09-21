<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import axios from 'axios';
import {
  FileText,
  Plus,
  Trash2,
  Edit3,
  Download,
  Eye,
  LayoutGrid,
  ChevronDown,
  ChevronUp,
  RefreshCw,
  Search,
  Check,
  X,
  Printer,
  Calendar,
  Layers,
  Activity,
  Database,
  Sliders,
  MoveUp,
  MoveDown,
  Copy,
  BarChart2,
  TrendingUp,
  Maximize2,
  Table,
  Zap,
} from 'lucide-vue-next';

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------
interface HeaderConfig {
  title: string;
  subtitle: string;
  showDate: boolean;
  logoText: string;
  totalPages?: number;
}

interface ReportWidget {
  id: string;
  reportId: string;
  pageNumber: number;
  title: string;
  chartType: string; // line, bar, area, gauge, table
  sourceType: string; // opensearch, grafana
  sourceConfig: Record<string, any>;
  timeRange: string;
  theme: string;
  widthPercent: number; // 50 or 100
  sortOrder: number;
  createdAt: string;

  // Runtime query cache
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
  tableRows?: Array<Record<string, any>>;
  showSummary?: boolean;
}

interface VisualReport {
  id: string;
  name: string;
  description: string;
  mode: 'document' | 'grid';
  pageOrientation: 'portrait' | 'landscape';
  headerConfig: HeaderConfig;
  widgets?: ReportWidget[];
  createdAt: string;
  updatedAt: string;
}

// -----------------------------------------------------------------------------
// State Management
// -----------------------------------------------------------------------------
const reports = ref<VisualReport[]>([]);
const activeReport = ref<VisualReport | null>(null);
const currentViewMode = ref<'list' | 'designer' | 'viewer'>('list');
const activePage = ref<number>(1);
const reportPageCount = ref<number>(1);
const zoomLevel = ref<number>(100);
const searchQuery = ref<string>('');
const loading = ref<boolean>(false);
const notification = ref<{ text: string; type: 'success' | 'error' } | null>(null);

// Modal states
const showCreateModal = ref<boolean>(false);
const showWidgetModal = ref<boolean>(false);
const showDeleteModal = ref<boolean>(false);
const deletingItem = ref<{ type: 'report' | 'widget' | 'page'; id: string; name: string } | null>(null);
const isDeleting = ref<boolean>(false);
const isSaving = ref<boolean>(false);

// Create / Edit Report Form
const reportForm = ref<{
  id?: string;
  name: string;
  description: string;
  mode: 'document' | 'grid';
  pageOrientation: 'portrait' | 'landscape';
  headerTitle: string;
  headerSubtitle: string;
  logoText: string;
  showDate: boolean;
}>({
  name: '',
  description: '',
  mode: 'document',
  pageOrientation: 'portrait',
  headerTitle: '',
  headerSubtitle: '',
  logoText: 'HEPHAESTUS',
  showDate: true,
});

// Add / Edit Widget Form
const editingWidgetId = ref<string | null>(null);
const widgetForm = ref<{
  title: string;
  chartType: string;
  sourceType: 'opensearch' | 'prometheus' | 'grafana';
  queryMode: 'preset' | 'custom';
  presetKey: string;
  query: string;
  indexPattern: string;
  targetHost: string;
  timeRange: string;
  theme: string;
  colorPalette: string;
  widthPercent: number;
  pageNumber: number;
}>({
  title: 'Log Event Volume Trend',
  chartType: 'line',
  sourceType: 'opensearch',
  queryMode: 'preset',
  presetKey: 'log_volume',
  query: '*',
  indexPattern: '*',
  targetHost: 'all',
  timeRange: '24h',
  theme: 'default',
  colorPalette: 'Grafana Classic',
  widthPercent: 50,
  pageNumber: 1,
});

// OpenSearch DSL Mode & Templates
const widgetDslMode = ref<'dsl' | 'lucene'>('dsl');
const openSearchDslTemplates = [
  {
    name: 'Match All',
    code: JSON.stringify({
      query: { match_all: {} }
    }, null, 2),
  },
  {
    name: 'Status >= 500',
    code: JSON.stringify({
      query: {
        range: {
          status: { gte: 500 }
        }
      }
    }, null, 2),
  },
  {
    name: 'Error Logs',
    code: JSON.stringify({
      query: {
        match: {
          level: 'ERROR'
        }
      }
    }, null, 2),
  },
  {
    name: 'Search Message',
    code: JSON.stringify({
      query: {
        match_phrase: {
          message: 'error'
        }
      }
    }, null, 2),
  },
  {
    name: 'Term Filter',
    code: JSON.stringify({
      query: {
        term: {
          "host.keyword": "server-horus"
        }
      }
    }, null, 2),
  },
];

// Presets for OpenSearch, Prometheus, and Grafana
const openSearchPresets = [
  {
    key: 'log_volume',
    title: 'Log Event Volume Trend',
    chartType: 'line',
    query: '*',
    desc: 'Total events aggregated over time',
  },
  {
    key: 'error_logs',
    title: 'Application & System Error Rates',
    chartType: 'bar',
    query: 'status:>=500 OR level:ERROR OR level:FATAL',
    desc: 'Failures and HTTP 5xx responses',
  },
  {
    key: 'warn_logs',
    title: 'Warning & Incident Degradations',
    chartType: 'bar',
    query: 'level:WARN OR level:WARNING',
    desc: 'Degradation alerts and notices',
  },
  {
    key: 'incident_table',
    title: 'Recent Incident Logs Table',
    chartType: 'table',
    query: 'level:ERROR OR status:500',
    desc: 'Tabular log records with timestamps',
  },
];

const prometheusPresets = [
  {
    key: 'cpu_util',
    title: 'CPU Core Utilization (%)',
    chartType: 'line',
    query: '100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)',
    desc: 'Total system CPU load percentage',
  },
  {
    key: 'mem_util',
    title: 'Memory / RAM Usage (%)',
    chartType: 'area',
    query: '(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100',
    desc: 'Active RAM consumption versus total',
  },
  {
    key: 'disk_util',
    title: 'Disk Storage Space Used (%)',
    chartType: 'bar',
    query: '(1 - (node_filesystem_free_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100',
    desc: 'Root disk usage percentage',
  },
  {
    key: 'net_traffic',
    title: 'Network Traffic Throughput',
    chartType: 'line',
    query: 'sum(rate(node_network_receive_bytes_total[5m])) * 8',
    desc: 'Ingress bandwidth in bits per second',
  },
  {
    key: 'sys_load',
    title: 'System Load Average (1m)',
    chartType: 'line',
    query: 'node_load1',
    desc: 'Normalized 1-minute system load',
  },
];

const grafanaPresets = [
  {
    key: 'grafana_cpu',
    title: 'Grafana Monitored CPU Load',
    chartType: 'line',
    query: 'CPU Load',
    desc: 'Queries active Grafana datasource',
  },
  {
    key: 'grafana_mem',
    title: 'Grafana Memory Allocation',
    chartType: 'area',
    query: 'Memory Allocation',
    desc: 'Heap and resident memory metrics',
  },
];

// Discovered Indices & Remote Hosts
const discoveredIndices = ref<string[]>([]);
const discoveredHosts = ref<Array<{ id: string; name: string; host: string }>>([]);
const testingQuery = ref(false);
const testQueryResult = ref<{ success: boolean; message: string } | null>(null);

const loadClusterMetadata = async () => {
  try {
    const [indicesRes, hostsRes] = await Promise.allSettled([
      axios.get('/api/v1/opensearch/indices'),
      axios.get('/api/v1/remote-host'),
    ]);

    if (indicesRes.status === 'fulfilled' && indicesRes.value.data?.success && Array.isArray(indicesRes.value.data.data)) {
      discoveredIndices.value = indicesRes.value.data.data.map((idx: any) => idx.index || idx.name).filter(Boolean);
    }
    if (hostsRes.status === 'fulfilled' && hostsRes.value.data?.success && Array.isArray(hostsRes.value.data.data)) {
      discoveredHosts.value = hostsRes.value.data.data.map((h: any) => ({
        id: h.id || h.host,
        name: h.name || h.hostname || h.host,
        host: h.host || h.ip,
      }));
    }
  } catch (e) {
    console.warn('Metadata discovery error:', e);
  }
};

const applyPreset = (preset: { key: string; title: string; chartType: string; query: string }) => {
  widgetForm.value.presetKey = preset.key;
  widgetForm.value.title = preset.title;
  widgetForm.value.chartType = preset.chartType;
  widgetForm.value.query = preset.query;
  testQueryResult.value = null;
};

const setQueryChip = (snippet: string) => {
  if (snippet === '*' || snippet === 'clear') {
    widgetForm.value.query = snippet === 'clear' ? '' : '*';
  } else {
    if (!widgetForm.value.query || widgetForm.value.query === '*') {
      widgetForm.value.query = snippet;
    } else {
      widgetForm.value.query += ` AND (${snippet})`;
    }
  }
  testQueryResult.value = null;
};

const testWidgetQuery = async () => {
  testingQuery.value = true;
  testQueryResult.value = null;
  try {
    const payload = {
      sourceType: widgetForm.value.sourceType,
      sourceConfig: {
        query: widgetForm.value.query,
        indexPattern: widgetForm.value.indexPattern,
        targetHost: widgetForm.value.targetHost,
      },
      timeRange: widgetForm.value.timeRange,
      metricKey: widgetForm.value.title,
    };
    const res = await axios.post('/api/v1/reports/query-data', payload);
    if (res.data?.success) {
      const pts = res.data.data?.points?.length || 0;
      const rows = res.data.data?.tableRows?.length || 0;
      testQueryResult.value = {
        success: true,
        message: `${res.data.data.message || 'Connected'} (${pts} points${rows > 0 ? `, ${rows} rows` : ''})`,
      };
    } else {
      testQueryResult.value = {
        success: false,
        message: res.data?.error || 'Query returned empty result',
      };
    }
  } catch (err: any) {
    testQueryResult.value = {
      success: false,
      message: err?.response?.data?.error || err?.message || 'Query test failed',
    };
  } finally {
    testingQuery.value = false;
  }
};

// -----------------------------------------------------------------------------
// Feedback Banner Auto-Dismiss
// -----------------------------------------------------------------------------
const showNotice = (text: string, type: 'success' | 'error' = 'success') => {
  notification.value = { text, type };
  setTimeout(() => {
    notification.value = null;
  }, 3000);
};

// -----------------------------------------------------------------------------
// Fetch & Load Reports
// -----------------------------------------------------------------------------
const fetchReports = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/reports');
    if (res.data?.success && Array.isArray(res.data.data)) {
      reports.value = res.data.data;
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to load reports', 'error');
  } finally {
    loading.value = false;
  }
};

const openReport = async (rep: VisualReport, mode: 'designer' | 'viewer' = 'viewer', targetPage?: number) => {
  loading.value = true;
  try {
    const res = await axios.get(`/api/v1/reports/${rep.id}`);
    if (res.data?.success && res.data.data) {
      activeReport.value = res.data.data;
      currentViewMode.value = mode;

      const widgetPages = (activeReport.value?.widgets || []).map((w) => w.pageNumber || 1);
      const savedPages = activeReport.value?.headerConfig?.totalPages || 1;
      const computedMax = Math.max(1, savedPages, ...widgetPages);
      reportPageCount.value = computedMax;

      if (targetPage !== undefined) {
        activePage.value = targetPage;
      } else if (activePage.value > computedMax || activePage.value < 1) {
        activePage.value = 1;
      }

      // Load live/demonstration data for each widget
      await refreshAllWidgetData();
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to open report', 'error');
  } finally {
    loading.value = false;
  }
};

const refreshAllWidgetData = async () => {
  if (!activeReport.value || !activeReport.value.widgets) return;

  const promises = activeReport.value.widgets.map(async (widget) => {
    widget.loading = true;
    try {
      const payload = {
        sourceType: widget.sourceType,
        sourceConfig: widget.sourceConfig,
        timeRange: widget.timeRange,
        metricKey: widget.title,
      };
      const res = await axios.post('/api/v1/reports/query-data', payload);
      if (res.data?.success && res.data.data) {
        widget.points = res.data.data.points || [];
        widget.summary = res.data.data.summary;
        widget.tableRows = res.data.data.tableRows || [];
      }
    } catch (_) {
      // Fallback handled in service
    } finally {
      widget.loading = false;
    }
  });

  await Promise.all(promises);
};

// -----------------------------------------------------------------------------
// Create / Edit Report
// -----------------------------------------------------------------------------
const openCreateReportModal = () => {
  reportForm.value = {
    name: '',
    description: '',
    mode: 'document',
    pageOrientation: 'portrait',
    headerTitle: '',
    headerSubtitle: 'Generated by Hephaestus Control Panel',
    logoText: 'HEPHAESTUS',
    showDate: true,
  };
  showCreateModal.value = true;
};

const submitReportForm = async () => {
  if (!reportForm.value.name.trim()) {
    showNotice('Report title is required', 'error');
    return;
  }

  isSaving.value = true;
  try {
    const payload = {
      name: reportForm.value.name.trim(),
      description: reportForm.value.description.trim(),
      mode: reportForm.value.mode,
      pageOrientation: reportForm.value.pageOrientation,
      headerConfig: {
        title: reportForm.value.headerTitle || reportForm.value.name.trim(),
        subtitle: reportForm.value.headerSubtitle,
        logoText: reportForm.value.logoText || 'HEPHAESTUS',
        showDate: reportForm.value.showDate,
      },
    };

    const res = await axios.post('/api/v1/reports', payload);
    if (res.data?.success && res.data.data) {
      showNotice('Report created successfully', 'success');
      showCreateModal.value = false;
      await fetchReports();
      // Auto open designer
      openReport(res.data.data, 'designer');
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to create report', 'error');
  } finally {
    isSaving.value = false;
  }
};

// -----------------------------------------------------------------------------
// Add / Edit Widget Modal
// -----------------------------------------------------------------------------
const openAddWidgetModal = (chartType: string = 'line') => {
  editingWidgetId.value = null;
  testQueryResult.value = null;
  widgetForm.value = {
    title: chartType === 'table' ? 'Recent Incident Logs Table' : 'Log Event Volume Trend',
    chartType,
    sourceType: 'opensearch',
    queryMode: 'preset',
    presetKey: chartType === 'table' ? 'incident_table' : 'log_volume',
    query: chartType === 'table' ? 'level:ERROR OR status:500' : '*',
    indexPattern: '*',
    targetHost: 'all',
    timeRange: '24h',
    theme: 'default',
    colorPalette: 'Grafana Classic',
    widthPercent: chartType === 'table' ? 100 : 50,
    pageNumber: activePage.value,
  };
  showWidgetModal.value = true;
};

const openEditWidgetModal = (w: ReportWidget) => {
  editingWidgetId.value = w.id;
  testQueryResult.value = null;
  widgetForm.value = {
    title: w.title,
    chartType: w.chartType,
    sourceType: (w.sourceType as any) || 'opensearch',
    queryMode: (w.sourceConfig?.queryMode as any) || 'custom',
    presetKey: w.sourceConfig?.presetKey || '',
    query: w.sourceConfig?.query || w.sourceConfig?.queryKeyword || '*',
    indexPattern: w.sourceConfig?.indexPattern || '*',
    targetHost: w.sourceConfig?.targetHost || 'all',
    timeRange: w.timeRange || '24h',
    theme: w.theme || 'default',
    colorPalette: w.sourceConfig?.colorPalette || 'Grafana Classic',
    widthPercent: w.widthPercent || 50,
    pageNumber: w.pageNumber || 1,
  };
  showWidgetModal.value = true;
};

const saveWidget = async () => {
  if (!activeReport.value) return;
  if (!widgetForm.value.title.trim()) {
    showNotice('Widget title is required', 'error');
    return;
  }

  isSaving.value = true;
  try {
    const payload = {
      reportId: activeReport.value.id,
      pageNumber: widgetForm.value.pageNumber || activePage.value,
      title: widgetForm.value.title.trim(),
      chartType: widgetForm.value.chartType,
      sourceType: widgetForm.value.sourceType,
      sourceConfig: {
        query: widgetForm.value.query,
        queryMode: widgetForm.value.queryMode,
        presetKey: widgetForm.value.presetKey,
        indexPattern: widgetForm.value.indexPattern,
        targetHost: widgetForm.value.targetHost,
        colorPalette: widgetForm.value.colorPalette,
      },
      timeRange: widgetForm.value.timeRange,
      theme: widgetForm.value.theme,
      widthPercent: widgetForm.value.widthPercent,
      sortOrder: (activeReport.value.widgets?.length || 0) + 1,
    };

    if (editingWidgetId.value) {
      await axios.put(`/api/v1/reports/widgets/${editingWidgetId.value}`, payload);
      showNotice('Widget updated successfully', 'success');
    } else {
      await axios.post(`/api/v1/reports/${activeReport.value.id}/widgets`, payload);
      showNotice('Widget added successfully', 'success');
    }

    showWidgetModal.value = false;
    await openReport(activeReport.value, currentViewMode.value, widgetForm.value.pageNumber || activePage.value);
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to save widget', 'error');
  } finally {
    isSaving.value = false;
  }
};

// -----------------------------------------------------------------------------
// Delete Handlers (HCP Standard Delete Confirmation Modal)
// -----------------------------------------------------------------------------
const confirmDeleteReport = (rep: VisualReport) => {
  deletingItem.value = { type: 'report', id: rep.id, name: rep.name };
  showDeleteModal.value = true;
};

const confirmDeleteWidget = (w: ReportWidget) => {
  deletingItem.value = { type: 'widget', id: w.id, name: w.title };
  showDeleteModal.value = true;
};

const confirmDeletePage = (p: number) => {
  const widgetsOnPage = (activeReport.value?.widgets || []).filter((w) => (w.pageNumber || 1) === p);
  deletingItem.value = {
    type: 'page',
    id: String(p),
    name: `Page ${p}${widgetsOnPage.length > 0 ? ` (${widgetsOnPage.length} widget${widgetsOnPage.length > 1 ? 's' : ''})` : ''}`,
  };
  showDeleteModal.value = true;
};

const executeDelete = async () => {
  if (!deletingItem.value) return;
  isDeleting.value = true;
  try {
    if (deletingItem.value.type === 'report') {
      await axios.delete(`/api/v1/reports/${deletingItem.value.id}`);
      showNotice('Report deleted successfully', 'success');
      showDeleteModal.value = false;
      if (activeReport.value?.id === deletingItem.value.id) {
        activeReport.value = null;
        currentViewMode.value = 'list';
      }
      await fetchReports();
    } else if (deletingItem.value.type === 'page') {
      const pageNum = parseInt(deletingItem.value.id, 10);
      // Remove widgets on this page
      const widgetsOnPage = (activeReport.value?.widgets || []).filter((w) => (w.pageNumber || 1) === pageNum);
      for (const w of widgetsOnPage) {
        try {
          await axios.delete(`/api/v1/reports/widgets/${w.id}`);
        } catch (e) {
          console.warn('Failed to delete widget on page:', e);
        }
      }

      // Re-number widgets on higher pages
      const higherWidgets = (activeReport.value?.widgets || []).filter((w) => (w.pageNumber || 1) > pageNum);
      for (const w of higherWidgets) {
        try {
          await axios.put(`/api/v1/reports/widgets/${w.id}`, {
            ...w,
            pageNumber: (w.pageNumber || 1) - 1,
          });
        } catch (e) {
          console.warn('Failed to reindex widget page:', e);
        }
      }

      reportPageCount.value = Math.max(1, reportPageCount.value - 1);
      const targetPage = activePage.value >= pageNum ? Math.max(1, activePage.value - 1) : activePage.value;
      activePage.value = targetPage;

      if (activeReport.value) {
        if (!activeReport.value.headerConfig) {
          activeReport.value.headerConfig = {
            title: activeReport.value.name,
            subtitle: '',
            showDate: true,
            logoText: 'HEPHAESTUS',
            totalPages: reportPageCount.value,
          };
        } else {
          activeReport.value.headerConfig.totalPages = reportPageCount.value;
        }
        await axios.put(`/api/v1/reports/${activeReport.value.id}`, activeReport.value);
        showDeleteModal.value = false;
        showNotice(`Page ${pageNum} removed successfully`, 'success');
        await openReport(activeReport.value, currentViewMode.value, targetPage);
      }
    } else {
      await axios.delete(`/api/v1/reports/widgets/${deletingItem.value.id}`);
      showNotice('Widget removed successfully', 'success');
      showDeleteModal.value = false;
      if (activeReport.value) {
        await openReport(activeReport.value, currentViewMode.value, activePage.value);
      }
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.message || 'Failed to delete item', 'error');
  } finally {
    isDeleting.value = false;
  }
};

// -----------------------------------------------------------------------------
// Multi-Page Management in Document Mode
// -----------------------------------------------------------------------------
const maxPages = computed(() => {
  const widgetPages = (activeReport.value?.widgets || []).map((w) => w.pageNumber || 1);
  const savedPages = activeReport.value?.headerConfig?.totalPages || 1;
  return Math.max(1, savedPages, reportPageCount.value, ...widgetPages);
});

const pagesList = computed(() => {
  const list: number[] = [];
  const count = Math.max(1, maxPages.value);
  for (let i = 1; i <= count; i++) {
    list.push(i);
  }
  return list;
});

const currentPageWidgets = computed(() => {
  if (!activeReport.value?.widgets) return [];
  return activeReport.value.widgets.filter((w) => (w.pageNumber || 1) === activePage.value);
});

const addPage = async () => {
  const newPageNum = maxPages.value + 1;
  reportPageCount.value = newPageNum;
  activePage.value = newPageNum;

  // Persist totalPages to activeReport
  if (activeReport.value) {
    if (!activeReport.value.headerConfig) {
      activeReport.value.headerConfig = {
        title: activeReport.value.name,
        subtitle: '',
        showDate: true,
        logoText: 'HEPHAESTUS',
        totalPages: newPageNum,
      };
    } else {
      activeReport.value.headerConfig.totalPages = newPageNum;
    }
    try {
      await axios.put(`/api/v1/reports/${activeReport.value.id}`, activeReport.value);
      showNotice(`Page ${newPageNum} added`, 'success');
    } catch (e) {
      console.warn('Failed to persist page count:', e);
    }
  }
};

// -----------------------------------------------------------------------------
// Chart SVG Path Generator
// -----------------------------------------------------------------------------
const generateSvgPath = (points?: Array<{ value: number }>, width = 500, height = 180): string => {
  if (!points || points.length === 0) return '';
  const min = Math.min(...points.map((p) => p.value));
  const max = Math.max(...points.map((p) => p.value));
  const range = max - min || 1;
  const paddingY = 25;
  const usableH = height - paddingY * 2;

  return points
    .map((p, i) => {
      const x = (i / (points.length - 1 || 1)) * (width - 40) + 20;
      const y = height - paddingY - ((p.value - min) / range) * usableH;
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
};

const generateAreaPath = (points?: Array<{ value: number }>, width = 500, height = 180): string => {
  const line = generateSvgPath(points, width, height);
  if (!line || !points) return '';
  const paddingY = 25;
  return `${line} L ${width - 20},${height - paddingY} L 20,${height - paddingY} Z`;
};

// -----------------------------------------------------------------------------
// Download Individual Widget as PNG (Pandora Style)
// -----------------------------------------------------------------------------
const downloadSingleWidgetPng = (widget: ReportWidget) => {
  const svgElement = document.getElementById(`widget-svg-${widget.id}`);
  if (!svgElement) {
    showNotice('Widget SVG element not found', 'error');
    return;
  }

  const svgData = new XMLSerializer().serializeToString(svgElement);
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  const img = new Image();

  const svgBlob = new Blob([svgData], { type: 'image/svg+xml;charset=utf-8' });
  const url = URL.createObjectURL(svgBlob);

  img.onload = () => {
    const width = 800;
    const height = 360;
    canvas.width = width;
    canvas.height = height;

    if (ctx) {
      ctx.fillStyle = '#0f172a';
      ctx.fillRect(0, 0, width, height);

      ctx.fillStyle = '#94a3b8';
      ctx.font = 'bold 14px sans-serif';
      ctx.fillText(
        `HCP - ${widget.title} (${(widget.sourceType || 'METRICS').toUpperCase()})`,
        24,
        28
      );

      ctx.drawImage(img, 15, 40, width - 30, height - 60);

      const pngUrl = canvas.toDataURL('image/png');
      const downloadLink = document.createElement('a');
      downloadLink.href = pngUrl;
      downloadLink.download = `${widget.title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Date.now()}.png`;
      document.body.appendChild(downloadLink);
      downloadLink.click();
      document.body.removeChild(downloadLink);
      showNotice('Widget chart PNG downloaded', 'success');
    }
    URL.revokeObjectURL(url);
  };

  img.src = url;
};

// -----------------------------------------------------------------------------
// Print / Export PDF Handler
// -----------------------------------------------------------------------------
const triggerPrint = () => {
  window.print();
};

onMounted(async () => {
  await fetchReports();
  await loadClusterMetadata();
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
        'p-3 rounded-xl border text-xs font-medium flex items-center justify-between shadow-sm animate-in fade-in'
      ]"
    >
      <span>{{ notification.text }}</span>
      <button @click="notification = null" class="cursor-pointer hover:opacity-75">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- Standard HCP Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4 print:hidden">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Visual Reports
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Generate, design, and export executive reporting documents with OpenSearch and Grafana data.
        </p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <!-- Back button if in detail view -->
        <button
          v-if="currentViewMode !== 'list'"
          @click="currentViewMode = 'list'"
          class="px-3 py-1.5 border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-[#151c2e] transition cursor-pointer"
        >
          All Reports
        </button>

        <!-- Mode Toggle (Designer Canvas vs Metrics Grid) -->
        <div v-if="currentViewMode !== 'list'" class="flex items-center p-0.5 bg-slate-100 dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs">
          <button
            @click="currentViewMode = 'designer'"
            :class="[
              currentViewMode === 'designer'
                ? 'bg-white dark:bg-[#1a2336] text-blue-600 dark:text-[#95CCDD] shadow-xs font-semibold'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
              'px-2.5 py-1 rounded-md transition cursor-pointer flex items-center gap-1.5'
            ]"
          >
            <FileText class="w-3.5 h-3.5" />
            <span>Document Canvas</span>
          </button>
          <button
            @click="currentViewMode = 'viewer'"
            :class="[
              currentViewMode === 'viewer'
                ? 'bg-white dark:bg-[#1a2336] text-blue-600 dark:text-[#95CCDD] shadow-xs font-semibold'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
              'px-2.5 py-1 rounded-md transition cursor-pointer flex items-center gap-1.5'
            ]"
          >
            <LayoutGrid class="w-3.5 h-3.5" />
            <span>Metrics Grid</span>
          </button>
        </div>

        <!-- Add Widget Button (when inside a report) -->
        <button
          v-if="currentViewMode !== 'list'"
          @click="openAddWidgetModal('line')"
          class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-sm transition flex items-center gap-1.5 cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Widget</span>
        </button>

        <!-- Print / Export PDF -->
        <button
          v-if="currentViewMode !== 'list'"
          @click="triggerPrint"
          class="px-3 py-1.5 border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-[#151c2e] transition flex items-center gap-1.5 cursor-pointer"
          title="Print or Save to PDF"
        >
          <Printer class="w-3.5 h-3.5 text-slate-500" />
          <span>Export PDF</span>
        </button>

        <!-- Create Report Button (when in catalog list) -->
        <button
          v-if="currentViewMode === 'list'"
          @click="openCreateReportModal"
          class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-sm transition flex items-center gap-1.5 cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Create Report</span>
        </button>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- VIEW 1: CATALOG LIST VIEW (PandoraFMS Image 4)                         -->
    <!-- ===================================================================== -->
    <div v-if="currentViewMode === 'list'" class="space-y-4 print:hidden">
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

      <!-- Report Cards Grid (Pandora Card Style) -->
      <div v-if="reports.length > 0" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="rep in reports"
          :key="rep.id"
          class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 hover:border-blue-300 dark:hover:border-blue-900/60 transition shadow-sm hover:shadow-md flex flex-col justify-between group"
        >
          <div class="space-y-3">
            <div class="flex items-center justify-between gap-2">
              <div class="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-950/60 text-blue-600 dark:text-[#95CCDD] flex items-center justify-center">
                <TrendingUp class="w-4 h-4" />
              </div>
              <div class="flex items-center gap-1.5">
                <span class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-50 dark:bg-emerald-950/50 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800">
                  {{ rep.widgets?.length || 0 }} Widget(s)
                </span>
                <span class="px-2 py-0.5 rounded-full text-[10px] font-medium bg-slate-100 dark:bg-[#1a2336] text-slate-600 dark:text-slate-400">
                  {{ rep.mode === 'document' ? 'Document' : 'Grid' }}
                </span>
              </div>
            </div>

            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-[#95CCDD] transition">
                {{ rep.name }}
              </h3>
              <p class="text-xs text-slate-500 dark:text-slate-400 mt-1 line-clamp-2">
                {{ rep.description || 'Custom visual reporting setup for infrastructure telemetry.' }}
              </p>
            </div>
          </div>

          <div class="pt-4 border-t border-slate-100 dark:border-[#1b2234] mt-4 flex items-center justify-between text-xs text-slate-400">
            <span class="text-[11px]">
              Created: {{ new Date(rep.createdAt).toLocaleDateString() }}
            </span>
            <div class="flex items-center gap-2">
              <button
                @click="openReport(rep, 'designer')"
                class="px-2.5 py-1 text-xs font-semibold text-blue-600 dark:text-[#95CCDD] hover:bg-blue-50 dark:hover:bg-blue-950/40 rounded-md transition cursor-pointer"
              >
                Designer &rarr;
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
        <FileText class="w-10 h-10 mx-auto text-slate-300 dark:text-slate-600" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">No Visual Reports Yet</h3>
        <p class="text-xs text-slate-500 max-w-sm mx-auto">
          Start by creating your first reporting layout to visualize OpenSearch cluster logs and Grafana metrics together.
        </p>
        <button
          @click="openCreateReportModal"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition cursor-pointer inline-flex items-center gap-1.5"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Create First Report</span>
        </button>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- VIEW 2: DOCUMENT CANVAS DESIGNER (PandoraFMS Image 1)                  -->
    <!-- ===================================================================== -->
    <div v-else-if="currentViewMode === 'designer'" class="grid grid-cols-12 gap-4 items-start">
      <!-- Left Sidebar: Multi-Page Thumbnails -->
      <div class="col-span-12 md:col-span-2 space-y-3 print:hidden">
        <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-3 space-y-3">
          <div class="flex items-center justify-between text-xs font-bold text-slate-700 dark:text-slate-300 pb-2 border-b border-slate-100 dark:border-[#1b2234]">
            <span>Pages</span>
            <button @click="addPage" class="p-1 hover:bg-slate-100 dark:hover:bg-[#151c2e] rounded text-blue-600 dark:text-[#95CCDD] cursor-pointer" title="Add Page">
              <Plus class="w-3.5 h-3.5" />
            </button>
          </div>

          <div class="space-y-2">
            <div
              v-for="p in pagesList"
              :key="p"
              class="relative group"
            >
              <button
                @click="activePage = p"
                :class="[
                  activePage === p
                    ? 'border-blue-500 ring-2 ring-blue-500/20 bg-blue-50/40 dark:bg-blue-950/20'
                    : 'border-slate-200 dark:border-[#1f283d] hover:border-slate-300',
                  'w-full text-left p-2 rounded-lg border transition cursor-pointer relative'
                ]"
              >
                <!-- Mini Page Preview Box -->
                <div class="aspect-3/4 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded flex flex-col justify-between p-1.5 shadow-xs">
                  <div class="h-1 bg-slate-200 dark:bg-[#1f283d] rounded w-2/3"></div>
                  <div class="space-y-0.5">
                    <div class="h-0.5 bg-slate-200 dark:bg-[#1f283d] rounded w-full"></div>
                    <div class="h-0.5 bg-slate-200 dark:bg-[#1f283d] rounded w-4/5"></div>
                  </div>
                </div>
                <span class="text-[11px] font-semibold text-slate-700 dark:text-slate-300 mt-1 block text-center">
                  Page {{ p }}
                </span>
              </button>

              <!-- Delete Page Button (if pagesList.length > 1) -->
              <button
                v-if="pagesList.length > 1"
                @click.stop="confirmDeletePage(p)"
                class="absolute top-1.5 right-1.5 p-1 bg-white dark:bg-[#111624] hover:bg-rose-50 dark:hover:bg-rose-950/50 text-slate-400 hover:text-rose-500 rounded border border-slate-200 dark:border-[#1f283d] opacity-0 group-hover:opacity-100 transition shadow-xs cursor-pointer"
                :title="'Delete Page ' + p"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Center Canvas: Printable A4 Document Sheet with Pandora-Style Grid -->
      <div class="col-span-12 md:col-span-8 flex flex-col items-center">
        <!-- Canvas Toolbar -->
        <div class="w-full max-w-[820px] mb-3 flex items-center justify-between text-xs text-slate-500 print:hidden px-1">
          <div class="flex items-center gap-2">
            <span class="font-medium">Page {{ activePage }} of {{ pagesList.length }}</span>
            <span class="text-slate-300 dark:text-slate-700">|</span>
            <span>Orientation: {{ activeReport?.pageOrientation || 'Portrait' }}</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="text-[11px]">Zoom: {{ zoomLevel }}%</span>
          </div>
        </div>

        <!-- Sheet Paper (A4 Style) -->
        <div
          class="w-full max-w-[820px] bg-white dark:bg-[#0d1117] border border-slate-300 dark:border-[#21262d] rounded-xl shadow-xl min-h-[960px] p-8 space-y-6 relative overflow-hidden transition"
          :style="{
            backgroundImage: 'radial-gradient(#cbd5e1 0.75px, transparent 0.75px)',
            backgroundSize: '16px 16px',
          }"
        >
          <!-- Pandora Style Report Header Bar -->
          <div class="border-b-2 border-slate-800 dark:border-slate-200 pb-3 flex items-end justify-between">
            <div class="flex items-center gap-2">
              <span class="text-xs font-black tracking-widest text-slate-900 dark:text-white uppercase">
                {{ activeReport?.headerConfig?.logoText || 'HEPHAESTUS' }}
              </span>
            </div>
            <div class="text-right">
              <h2 class="text-sm font-bold text-slate-900 dark:text-white">
                {{ activeReport?.headerConfig?.title || activeReport?.name }}
              </h2>
              <p v-if="activeReport?.headerConfig?.showDate" class="text-[10px] text-slate-500 dark:text-slate-400">
                {{ new Date().toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' }) }}
              </p>
            </div>
          </div>

          <!-- Page Widgets Container (50% or 100% Half/Full Width) -->
          <div v-if="currentPageWidgets.length > 0" class="grid grid-cols-2 gap-4">
            <div
              v-for="widget in currentPageWidgets"
              :key="widget.id"
              :class="[
                widget.widthPercent === 100 ? 'col-span-2' : 'col-span-1',
                'bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-4 shadow-xs hover:border-blue-400 transition group relative'
              ]"
            >
              <!-- Widget Card Header -->
              <div class="flex items-center justify-between pb-2 border-b border-slate-100 dark:border-[#1b2234] mb-3">
                <div>
                  <h4 class="text-xs font-bold text-slate-900 dark:text-white">{{ widget.title }}</h4>
                  <span class="text-[10px] text-slate-400 uppercase tracking-wider">{{ widget.sourceType }} &bull; {{ widget.timeRange }}</span>
                </div>
                <!-- Action Controls (Hidden on Print) -->
                <div class="flex items-center gap-1.5 opacity-0 group-hover:opacity-100 transition print:hidden">
                  <button @click="openEditWidgetModal(widget)" class="p-1 text-slate-400 hover:text-blue-500 rounded cursor-pointer" title="Edit">
                    <Edit3 class="w-3 h-3" />
                  </button>
                  <button @click="confirmDeleteWidget(widget)" class="p-1 text-slate-400 hover:text-rose-500 rounded cursor-pointer" title="Delete">
                    <Trash2 class="w-3 h-3" />
                  </button>
                </div>
              </div>

              <!-- Widget Chart Body -->
              <div v-if="widget.chartType !== 'table'" class="relative h-44 w-full">
                <svg class="w-full h-full" viewBox="0 0 500 180" preserveAspectRatio="none">
                  <!-- Gradient Area -->
                  <defs>
                    <linearGradient :id="'grad-' + widget.id" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.35" />
                      <stop offset="100%" stop-color="#3b82f6" stop-opacity="0.0" />
                    </linearGradient>
                  </defs>
                  <path :d="generateAreaPath(widget.points)" :fill="'url(#grad-' + widget.id + ')'" />
                  <path :d="generateSvgPath(widget.points)" fill="none" stroke="#2563eb" stroke-width="2.2" stroke-linecap="round" />
                  <!-- Data point dots -->
                  <circle
                    v-for="(pt, idx) in widget.points || []"
                    :key="idx"
                    :cx="(idx / ((widget.points?.length || 1) - 1 || 1)) * (500 - 40) + 20"
                    :cy="180 - 25 - ((pt.value - (widget.summary?.min || 0)) / ((widget.summary?.max || 1) - (widget.summary?.min || 0) || 1)) * 130"
                    r="3.5"
                    class="fill-white dark:fill-[#0c101a] stroke-blue-600"
                    stroke-width="2"
                  />
                </svg>

                <!-- Metric Summary Badge -->
                <div class="flex items-center justify-between text-[10px] text-slate-400 mt-2 px-1">
                  <span>Avg: <strong class="text-slate-700 dark:text-slate-200">{{ widget.summary?.avg || 0 }} {{ widget.summary?.unit }}</strong></span>
                  <span>Max: <strong class="text-slate-700 dark:text-slate-200">{{ widget.summary?.max || 0 }} {{ widget.summary?.unit }}</strong></span>
                </div>
              </div>

              <!-- Widget Table Body (for OpenSearch events) -->
              <div v-else class="overflow-x-auto text-[11px]">
                <table class="w-full text-left">
                  <thead>
                    <tr class="border-b border-slate-100 dark:border-[#1b2234] text-slate-400 text-[10px]">
                      <th class="py-1 px-1">Time</th>
                      <th class="py-1 px-1">Level</th>
                      <th class="py-1 px-1">Message</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, ridx) in widget.tableRows || []" :key="ridx" class="border-b border-slate-50 dark:border-[#151c2e]">
                      <td class="py-1 px-1 text-slate-400">{{ row.timestamp }}</td>
                      <td class="py-1 px-1">
                        <span :class="row.level === 'ERROR' ? 'text-rose-500 font-bold' : 'text-slate-500'">{{ row.level }}</span>
                      </td>
                      <td class="py-1 px-1 text-slate-600 dark:text-slate-300 truncate max-w-xs">{{ row.message }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- Empty Page Placeholder -->
          <div v-else class="py-24 text-center border border-dashed border-slate-300 dark:border-[#21262d] rounded-xl print:hidden">
            <p class="text-xs text-slate-400">Page {{ activePage }} is currently empty.</p>
            <p class="text-[11px] text-slate-400 mt-1">Select a widget from the palette on the right to place it on this sheet.</p>
          </div>
        </div>
      </div>

      <!-- Right Sidebar: Widget Library Palette (PandoraFMS Image 1) -->
      <div class="col-span-12 md:col-span-2 space-y-3 print:hidden">
        <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-3 space-y-3">
          <div class="pb-2 border-b border-slate-100 dark:border-[#1b2234]">
            <span class="text-xs font-bold text-slate-700 dark:text-slate-300">Widget Palette</span>
            <p class="text-[10px] text-slate-400 mt-0.5">Click to place widget</p>
          </div>

          <!-- Palette Buttons -->
          <div class="space-y-1.5">
            <button
              @click="openAddWidgetModal('line')"
              class="w-full flex items-center gap-2.5 p-2 rounded-lg border border-slate-200 dark:border-[#1f283d] hover:border-blue-400 hover:bg-slate-50 dark:hover:bg-[#151c2e] text-left transition cursor-pointer"
            >
              <TrendingUp class="w-4 h-4 text-blue-500 shrink-0" />
              <div>
                <p class="text-xs font-bold text-slate-800 dark:text-slate-200">Line Chart</p>
                <p class="text-[10px] text-slate-400">Time-series trend</p>
              </div>
            </button>

            <button
              @click="openAddWidgetModal('area')"
              class="w-full flex items-center gap-2.5 p-2 rounded-lg border border-slate-200 dark:border-[#1f283d] hover:border-blue-400 hover:bg-slate-50 dark:hover:bg-[#151c2e] text-left transition cursor-pointer"
            >
              <Activity class="w-4 h-4 text-emerald-500 shrink-0" />
              <div>
                <p class="text-xs font-bold text-slate-800 dark:text-slate-200">Area Chart</p>
                <p class="text-[10px] text-slate-400">Resource utilization</p>
              </div>
            </button>

            <button
              @click="openAddWidgetModal('bar')"
              class="w-full flex items-center gap-2.5 p-2 rounded-lg border border-slate-200 dark:border-[#1f283d] hover:border-blue-400 hover:bg-slate-50 dark:hover:bg-[#151c2e] text-left transition cursor-pointer"
            >
              <BarChart2 class="w-4 h-4 text-purple-500 shrink-0" />
              <div>
                <p class="text-xs font-bold text-slate-800 dark:text-slate-200">Histogram</p>
                <p class="text-[10px] text-slate-400">Distribution / events</p>
              </div>
            </button>

            <button
              @click="openAddWidgetModal('table')"
              class="w-full flex items-center gap-2.5 p-2 rounded-lg border border-slate-200 dark:border-[#1f283d] hover:border-blue-400 hover:bg-slate-50 dark:hover:bg-[#151c2e] text-left transition cursor-pointer"
            >
              <Table class="w-4 h-4 text-amber-500 shrink-0" />
              <div>
                <p class="text-xs font-bold text-slate-800 dark:text-slate-200">Event Table</p>
                <p class="text-[10px] text-slate-400">OpenSearch log rows</p>
              </div>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- VIEW 3: INTERACTIVE METRICS GRID VIEWER (PandoraFMS Image 2)          -->
    <!-- ===================================================================== -->
    <div v-else-if="currentViewMode === 'viewer'" class="space-y-4">
      <!-- Viewer Header Bar -->
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 shadow-xs">
        <div>
          <h2 class="text-base font-bold text-slate-900 dark:text-white">{{ activeReport?.name }}</h2>
          <p class="text-xs text-slate-500 mt-0.5">Created: {{ new Date(activeReport?.createdAt || '').toLocaleDateString() }} &bull; {{ activeReport?.widgets?.length || 0 }} Metrics Panels Active</p>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="openAddWidgetModal('line')"
            class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-xs transition flex items-center gap-1.5 cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Panel</span>
          </button>
          <button
            @click="refreshAllWidgetData"
            class="px-3 py-1.5 border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-[#151c2e] transition cursor-pointer flex items-center gap-1.5"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
            <span>Refresh Data</span>
          </button>
        </div>
      </div>

      <!-- Grid Cards (Pandora Chart Style) -->
      <div v-if="activeReport?.widgets && activeReport.widgets.length > 0" class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div
          v-for="widget in activeReport.widgets"
          :key="widget.id"
          class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 shadow-xs space-y-4"
        >
          <!-- Card Header (Pandora Style) -->
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">{{ widget.title }}</h3>
            <div class="flex items-center gap-2 text-slate-400">
              <button
                v-if="widget.chartType !== 'table'"
                @click="downloadSingleWidgetPng(widget)"
                class="hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
                title="Download Chart PNG"
              >
                <Download class="w-3.5 h-3.5" />
              </button>
              <button @click="openEditWidgetModal(widget)" class="hover:text-blue-500 cursor-pointer" title="Edit Widget">
                <Edit3 class="w-3.5 h-3.5" />
              </button>
              <button @click="confirmDeleteWidget(widget)" class="hover:text-rose-500 cursor-pointer" title="Delete Widget">
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Chart Subtitle & Time Range -->
          <div class="text-center space-y-0.5">
            <h4 class="text-xs font-semibold text-slate-800 dark:text-slate-200">{{ widget.title }}</h4>
            <p class="text-[10px] text-slate-400">Time Range: Last {{ widget.timeRange }}</p>
          </div>

          <!-- Chart SVG Visualization -->
          <div v-if="widget.chartType !== 'table'" class="h-48 w-full relative">
            <svg :id="'widget-svg-' + widget.id" class="w-full h-full" viewBox="0 0 500 180" preserveAspectRatio="none">
              <path :d="generateSvgPath(widget.points)" fill="none" stroke="#10b981" stroke-width="2.5" stroke-linecap="round" />
              <circle
                v-for="(pt, idx) in widget.points || []"
                :key="idx"
                :cx="(idx / ((widget.points?.length || 1) - 1 || 1)) * (500 - 40) + 20"
                :cy="180 - 25 - ((pt.value - (widget.summary?.min || 0)) / ((widget.summary?.max || 1) - (widget.summary?.min || 0) || 1)) * 130"
                r="3"
                class="fill-white dark:fill-[#111624] stroke-emerald-500"
                stroke-width="2"
              />
            </svg>

            <!-- Legend Indicator (Pandora Style) -->
            <div class="flex items-center justify-center gap-2 text-[11px] text-slate-500 mt-2">
              <span class="w-2.5 h-2.5 rounded-full bg-emerald-500"></span>
              <span>{{ widget.sourceConfig?.targetHost && widget.sourceConfig.targetHost !== 'all' ? widget.sourceConfig.targetHost + ' - ' : '' }}{{ widget.title }}</span>
            </div>
          </div>

          <!-- Table View for Logs -->
          <div v-else class="overflow-x-auto text-[11px] border border-slate-100 dark:border-[#1b2234] rounded-lg p-2">
            <table class="w-full text-left">
              <thead>
                <tr class="text-[10px] text-slate-400 border-b border-slate-100 dark:border-[#1b2234]">
                  <th class="py-1">Time</th>
                  <th class="py-1">Status</th>
                  <th class="py-1">Message</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in widget.tableRows || []" :key="i" class="border-b border-slate-50 dark:border-[#151c2e]">
                  <td class="py-1 text-slate-400">{{ r.timestamp }}</td>
                  <td class="py-1"><span class="font-bold text-emerald-500">{{ r.level || 'INFO' }}</span></td>
                  <td class="py-1 text-slate-600 dark:text-slate-300 truncate max-w-xs">{{ r.message }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Pandora Style "View Widget Summary" Accordion Toggle -->
          <div class="pt-2 border-t border-slate-100 dark:border-[#1b2234]">
            <button
              @click="widget.showSummary = !widget.showSummary"
              class="text-xs font-semibold text-emerald-600 dark:text-emerald-400 hover:text-emerald-500 flex items-center gap-1 cursor-pointer"
            >
              <component :is="widget.showSummary ? ChevronUp : ChevronDown" class="w-3.5 h-3.5" />
              <span>View Widget Summary</span>
            </button>

            <!-- Expanded Summary Stats Table -->
            <div v-if="widget.showSummary" class="mt-3 p-3 bg-slate-50 dark:bg-[#0c101a] rounded-xl border border-slate-200 dark:border-[#1f283d] grid grid-cols-4 gap-2 text-center text-xs">
              <div>
                <span class="text-[10px] text-slate-400 block">Min</span>
                <strong class="text-slate-800 dark:text-slate-200">{{ widget.summary?.min || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
              <div>
                <span class="text-[10px] text-slate-400 block">Average</span>
                <strong class="text-slate-800 dark:text-slate-200">{{ widget.summary?.avg || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
              <div>
                <span class="text-[10px] text-slate-400 block">Max</span>
                <strong class="text-slate-800 dark:text-slate-200">{{ widget.summary?.max || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
              <div>
                <span class="text-[10px] text-slate-400 block">Current</span>
                <strong class="text-slate-800 dark:text-slate-200">{{ widget.summary?.current || 0 }} {{ widget.summary?.unit }}</strong>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 1: ADD / EDIT WIDGET PANEL                                      -->
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
            <p class="text-[11px] text-slate-500">
              Configure data source, query metrics, and visual parameters
            </p>
          </div>
          <button @click="showWidgetModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-4 text-xs">
          <!-- Row 1: Widget Title & Chart Type -->
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Panel Widget Title</label>
              <input
                v-model="widgetForm.title"
                type="text"
                placeholder="e.g. Metrics Trend Chart"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Chart Type</label>
              <select
                v-model="widgetForm.chartType"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="line">Line Chart</option>
                <option value="area">Area Chart</option>
                <option value="bar">Bar / Histogram</option>
                <option value="table">Data Table</option>
              </select>
            </div>
          </div>

          <!-- Row 2: Data Source Provider (OpenSearch vs Prometheus vs Grafana) -->
          <div class="space-y-1.5">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Data Source Provider</label>
            <div class="grid grid-cols-3 gap-2">
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
                  <div class="text-[10px] text-slate-400 font-normal">Logs & Events</div>
                </div>
              </label>

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
                  <div class="text-[10px] text-slate-400 font-normal">Server Telemetry</div>
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
                  <div class="text-[10px] text-slate-400 font-normal">Optional Dashboards</div>
                </div>
              </label>
            </div>
          </div>

          <!-- Row 3: Mode Toggle (Quick Presets vs Custom Query) -->
          <div class="space-y-2 pt-1 border-t border-slate-100 dark:border-[#1b2234]">
            <div class="flex items-center justify-between">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Data Query Method</label>
              <div class="flex items-center rounded-lg bg-slate-100 dark:bg-[#0c101a] p-0.5 text-[11px] font-medium border border-slate-200 dark:border-[#1f283d]">
                <button
                  type="button"
                  @click="widgetForm.queryMode = 'preset'"
                  :class="[
                    widgetForm.queryMode === 'preset'
                      ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-xs font-semibold'
                      : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300',
                    'px-2.5 py-1 rounded-md transition cursor-pointer'
                  ]"
                >
                  Quick Presets
                </button>
                <button
                  type="button"
                  @click="widgetForm.queryMode = 'custom'"
                  :class="[
                    widgetForm.queryMode === 'custom'
                      ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white shadow-xs font-semibold'
                      : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300',
                    'px-2.5 py-1 rounded-md transition cursor-pointer'
                  ]"
                >
                  Custom Query
                </button>
              </div>
            </div>

            <!-- MODE A: PRESETS CARDS -->
            <div v-if="widgetForm.queryMode === 'preset'" class="space-y-2">
              <div class="text-[11px] text-slate-500">
                Choose a ready-to-use telemetry or log analysis preset:
              </div>
              <div class="grid grid-cols-2 gap-2">
                <template v-if="widgetForm.sourceType === 'opensearch'">
                  <div
                    v-for="p in openSearchPresets"
                    :key="p.key"
                    @click="applyPreset(p)"
                    :class="[
                      widgetForm.presetKey === p.key
                        ? 'border-blue-500 ring-1 ring-blue-500 bg-blue-50/40 dark:bg-blue-950/30'
                        : 'border-slate-200 dark:border-[#1f283d] hover:border-slate-300 dark:hover:border-[#2a3754]',
                      'p-2.5 rounded-xl border transition cursor-pointer flex flex-col justify-between'
                    ]"
                  >
                    <div>
                      <div class="font-bold text-slate-800 dark:text-slate-200 text-xs flex items-center justify-between">
                        <span>{{ p.title }}</span>
                        <span class="text-[10px] font-mono text-slate-400 bg-slate-100 dark:bg-[#0c101a] px-1.5 py-0.5 rounded">{{ p.chartType }}</span>
                      </div>
                      <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-1 leading-tight">{{ p.desc }}</p>
                    </div>
                    <div class="mt-2 pt-1 border-t border-slate-100 dark:border-[#1a2336] text-[10px] font-mono text-slate-400 truncate">
                      {{ p.query }}
                    </div>
                  </div>
                </template>

                <template v-else-if="widgetForm.sourceType === 'prometheus'">
                  <div
                    v-for="p in prometheusPresets"
                    :key="p.key"
                    @click="applyPreset(p)"
                    :class="[
                      widgetForm.presetKey === p.key
                        ? 'border-blue-500 ring-1 ring-blue-500 bg-blue-50/40 dark:bg-blue-950/30'
                        : 'border-slate-200 dark:border-[#1f283d] hover:border-slate-300 dark:hover:border-[#2a3754]',
                      'p-2.5 rounded-xl border transition cursor-pointer flex flex-col justify-between'
                    ]"
                  >
                    <div>
                      <div class="font-bold text-slate-800 dark:text-slate-200 text-xs flex items-center justify-between">
                        <span>{{ p.title }}</span>
                        <span class="text-[10px] font-mono text-slate-400 bg-slate-100 dark:bg-[#0c101a] px-1.5 py-0.5 rounded">{{ p.chartType }}</span>
                      </div>
                      <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-1 leading-tight">{{ p.desc }}</p>
                    </div>
                    <div class="mt-2 pt-1 border-t border-slate-100 dark:border-[#1a2336] text-[10px] font-mono text-slate-400 truncate">
                      {{ p.query }}
                    </div>
                  </div>
                </template>

                <template v-else>
                  <div
                    v-for="p in grafanaPresets"
                    :key="p.key"
                    @click="applyPreset(p)"
                    :class="[
                      widgetForm.presetKey === p.key
                        ? 'border-blue-500 ring-1 ring-blue-500 bg-blue-50/40 dark:bg-blue-950/30'
                        : 'border-slate-200 dark:border-[#1f283d] hover:border-slate-300 dark:hover:border-[#2a3754]',
                      'p-2.5 rounded-xl border transition cursor-pointer flex flex-col justify-between'
                    ]"
                  >
                    <div>
                      <div class="font-bold text-slate-800 dark:text-slate-200 text-xs flex items-center justify-between">
                        <span>{{ p.title }}</span>
                        <span class="text-[10px] font-mono text-slate-400 bg-slate-100 dark:bg-[#0c101a] px-1.5 py-0.5 rounded">{{ p.chartType }}</span>
                      </div>
                      <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-1 leading-tight">{{ p.desc }}</p>
                    </div>
                  </div>
                </template>
              </div>
            </div>

            <!-- MODE B: CUSTOM QUERY -->
            <div v-else class="space-y-3 p-3.5 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-xl">
              <!-- OpenSearch Index & Query -->
              <template v-if="widgetForm.sourceType === 'opensearch'">
                <div class="grid grid-cols-2 gap-3">
                  <div class="space-y-1">
                    <label class="font-semibold text-slate-700 dark:text-slate-300">Target Index Pattern</label>
                    <div class="relative">
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
                        placeholder="e.g. * or logs-*"
                        class="w-full px-3 py-2 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
                      />
                    </div>
                  </div>

                  <div class="space-y-1">
                    <label class="font-semibold text-slate-700 dark:text-slate-300">Quick Filter Preset</label>
                    <div class="flex items-center gap-1.5 flex-wrap pt-1">
                      <button
                        type="button"
                        @click="setQueryChip('*')"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        All (*)
                      </button>
                      <button
                        type="button"
                        @click="setQueryChip('status:>=500')"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        status:>=500
                      </button>
                      <button
                        type="button"
                        @click="setQueryChip('level:ERROR')"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        level:ERROR
                      </button>
                      <button
                        type="button"
                        @click="setQueryChip('level:WARN')"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        level:WARN
                      </button>
                    </div>
                  </div>
                </div>

                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <label class="font-semibold text-slate-700 dark:text-slate-300">
                      {{ widgetDslMode === 'dsl' ? 'OpenSearch Query DSL (JSON)' : 'Lucene Query Expression' }}
                    </label>
                    <div class="flex items-center p-0.5 bg-slate-100 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-[10px]">
                      <button
                        type="button"
                        @click="widgetDslMode = 'dsl'; if (!widgetForm.query.startsWith('{')) widgetForm.query = openSearchDslTemplates[0].code;"
                        :class="[
                          widgetDslMode === 'dsl'
                            ? 'bg-white dark:bg-[#1a2336] text-blue-600 dark:text-[#95CCDD] font-semibold shadow-xs'
                            : 'text-slate-500 hover:text-slate-800 dark:hover:text-slate-300',
                          'px-2 py-0.5 rounded cursor-pointer transition'
                        ]"
                      >
                        Query DSL (JSON)
                      </button>
                      <button
                        type="button"
                        @click="widgetDslMode = 'lucene'; if (widgetForm.query.startsWith('{')) widgetForm.query = '*';"
                        :class="[
                          widgetDslMode === 'lucene'
                            ? 'bg-white dark:bg-[#1a2336] text-blue-600 dark:text-[#95CCDD] font-semibold shadow-xs'
                            : 'text-slate-500 hover:text-slate-800 dark:hover:text-slate-300',
                          'px-2 py-0.5 rounded cursor-pointer transition'
                        ]"
                      >
                        Lucene String
                      </button>
                    </div>
                  </div>

                  <!-- Quick Templates for DSL -->
                  <div v-if="widgetDslMode === 'dsl'" class="flex items-center gap-1.5 flex-wrap">
                    <span class="text-[10px] text-slate-400">DSL Templates:</span>
                    <button
                      v-for="t in openSearchDslTemplates"
                      :key="t.name"
                      type="button"
                      @click="widgetForm.query = t.code"
                      class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-medium cursor-pointer"
                    >
                      {{ t.name }}
                    </button>
                  </div>

                  <textarea
                    v-model="widgetForm.query"
                    :rows="widgetDslMode === 'dsl' ? 6 : 2"
                    :placeholder="widgetDslMode === 'dsl' ? '{ \"query\": { \"match_all\": {} } }' : 'e.g. status:>=500 OR level:ERROR, service:nginx, *'"
                    class="w-full px-3 py-2 font-mono text-[11px] bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 focus:ring-1 focus:ring-blue-500 leading-relaxed"
                  ></textarea>
                </div>
              </template>

              <!-- Prometheus Host & PromQL Query -->
              <template v-else>
                <div class="grid grid-cols-2 gap-3">
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
                    <label class="font-semibold text-slate-700 dark:text-slate-300">Quick Metric Formula</label>
                    <div class="flex items-center gap-1.5 flex-wrap pt-1">
                      <button
                        type="button"
                        @click="widgetForm.query = '100 - (avg(rate(node_cpu_seconds_total{mode=\'idle\'}[5m])) * 100)'"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        CPU %
                      </button>
                      <button
                        type="button"
                        @click="widgetForm.query = '(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100'"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        RAM %
                      </button>
                      <button
                        type="button"
                        @click="widgetForm.query = '(1 - (node_filesystem_free_bytes{mountpoint=\'/\'} / node_filesystem_size_bytes{mountpoint=\'/\'})) * 100'"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        Disk %
                      </button>
                      <button
                        type="button"
                        @click="widgetForm.query = 'node_load1'"
                        class="px-2 py-0.5 rounded text-[10px] bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 text-slate-700 dark:text-slate-300 font-mono cursor-pointer"
                      >
                        Load Avg
                      </button>
                    </div>
                  </div>
                </div>

                <div class="space-y-1">
                  <label class="font-semibold text-slate-700 dark:text-slate-300">PromQL / Metric Expression</label>
                  <textarea
                    v-model="widgetForm.query"
                    rows="2"
                    placeholder="e.g. 100 - (avg(rate(node_cpu_seconds_total{mode='idle'}[5m])) * 100)"
                    class="w-full px-3 py-2 font-mono text-[11px] bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 focus:ring-1 focus:ring-blue-500"
                  ></textarea>
                </div>
              </template>
            </div>
          </div>

          <!-- Live Query Test & Feedback Row -->
          <div class="flex items-center justify-between p-2.5 rounded-xl bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d]">
            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="testWidgetQuery"
                :disabled="testingQuery"
                class="px-3 py-1 bg-slate-200 dark:bg-[#1f283d] hover:bg-slate-300 dark:hover:bg-[#283550] text-slate-800 dark:text-slate-200 rounded-lg text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
              >
                <RefreshCw :class="['w-3.5 h-3.5 text-slate-500', testingQuery ? 'animate-spin' : '']" />
                <span>{{ testingQuery ? 'Testing...' : 'Test Query' }}</span>
              </button>
              <span v-if="testQueryResult" :class="[testQueryResult.success ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-500', 'text-[11px] font-medium']">
                {{ testQueryResult.message }}
              </span>
              <span v-else class="text-[11px] text-slate-400">
                Click to preview live response before saving
              </span>
            </div>
          </div>

          <!-- Row 4: Time Range & Panel Size -->
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Time Range</label>
              <select
                v-model="widgetForm.timeRange"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="1h">Last 1 Hour</option>
                <option value="6h">Last 6 Hours</option>
                <option value="24h">Last 24 Hours</option>
                <option value="7d">Last 7 Days</option>
                <option value="30d">Last 30 Days</option>
              </select>
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Panel Size (Width)</label>
              <select
                v-model="widgetForm.widthPercent"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option :value="50">Half Width (50%)</option>
                <option :value="100">Full Width (100%)</option>
              </select>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showWidgetModal = false"
            class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="saveWidget"
            :disabled="isSaving"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ isSaving ? 'Saving...' : 'Save Widget' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 2: CREATE REPORT MODAL                                          -->
    <!-- ===================================================================== -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-6 space-y-4">
        <div class="flex items-center justify-between pb-3 border-b border-slate-100 dark:border-[#1b2234]">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Create New Visual Report</h3>
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
              placeholder="e.g. Linux Host & OpenSearch Cluster Report"
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
            />
          </div>

          <div class="space-y-1">
            <label class="font-semibold text-slate-700 dark:text-slate-300">Description</label>
            <textarea
              v-model="reportForm.description"
              rows="2"
              placeholder="Brief summary of report purpose..."
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
            ></textarea>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Default Mode</label>
              <select
                v-model="reportForm.mode"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="document">A4 Document Canvas</option>
                <option value="grid">Responsive Grid</option>
              </select>
            </div>

            <div class="space-y-1">
              <label class="font-semibold text-slate-700 dark:text-slate-300">Page Orientation</label>
              <select
                v-model="reportForm.pageOrientation"
                class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
              >
                <option value="portrait">Portrait</option>
                <option value="landscape">Landscape</option>
              </select>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-[#1b2234]">
          <button @click="showCreateModal = false" class="px-3.5 py-1.5 text-xs text-slate-600 dark:text-slate-400 cursor-pointer">
            Cancel
          </button>
          <button
            @click="submitReportForm"
            :disabled="isSaving"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
          >
            {{ isSaving ? 'Creating...' : 'Create & Open Designer' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ===================================================================== -->
    <!-- MODAL 3: HCP STANDARD DELETE CONFIRMATION MODAL                      -->
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
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            Delete {{ deletingItem?.type === 'report' ? 'Report' : deletingItem?.type === 'page' ? 'Page' : 'Widget' }}?
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ deletingItem?.name }}</strong>? This action cannot be undone.
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

<style scoped>
@media print {
  body {
    background: #ffffff !important;
    color: #000000 !important;
  }
}
</style>
