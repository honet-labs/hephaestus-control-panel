<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import axios from 'axios';
import {
  RefreshCw,
  Download,
  FileText,
  Database,
  Activity,
  Search,
  ChevronLeft,
  ChevronRight,
  Layers,
  Image as ImageIcon,
} from 'lucide-vue-next';

// -----------------------------------------------------------------------------
// State & Filters
// -----------------------------------------------------------------------------
const sourceType = ref<'opensearch' | 'prometheus'>('opensearch');
const indexPattern = ref('*');
const targetHost = ref('all');
const queryString = ref('*');
const timeRange = ref('24h');
const chartType = ref<'line' | 'area' | 'bar'>('line');

const loading = ref(false);
const notification = ref<{ text: string; type: 'success' | 'error' } | null>(null);

// Data Results
const reportData = ref<{
  title: string;
  points: Array<{ timestamp: string; label: string; value: number }>;
  summary: { min: number; max: number; avg: number; current: number; unit: string };
  tableRows: Array<Record<string, any>>;
  message: string;
} | null>(null);

// Discovered Metadata
const discoveredIndices = ref<string[]>([]);
const discoveredHosts = ref<Array<{ id: string; name: string; host: string }>>([]);

// Table Search & Pagination
const tableSearch = ref('');
const currentPage = ref(1);
const itemsPerPage = ref(10);

// -----------------------------------------------------------------------------
// Auto-Dismiss Notification
// -----------------------------------------------------------------------------
const showNotice = (text: string, type: 'success' | 'error' = 'success') => {
  notification.value = { text, type };
  setTimeout(() => {
    notification.value = null;
  }, 3000);
};

// -----------------------------------------------------------------------------
// Fetch Cluster & Server Metadata
// -----------------------------------------------------------------------------
const loadMetadata = async () => {
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
  } catch (err) {
    console.warn('Failed to load discovery metadata:', err);
  }
};

// -----------------------------------------------------------------------------
// Generate & Query Report Data
// -----------------------------------------------------------------------------
const generateReport = async () => {
  loading.value = true;
  currentPage.value = 1;
  try {
    const payload = {
      sourceType: sourceType.value,
      sourceConfig: {
        query: queryString.value,
        indexPattern: indexPattern.value,
        targetHost: targetHost.value,
      },
      timeRange: timeRange.value,
      metricKey: sourceType.value === 'opensearch' ? 'OpenSearch Logs' : 'Prometheus Telemetry',
    };

    const res = await axios.post('/api/v1/reports/query-data', payload);
    if (res.data?.success && res.data.data) {
      reportData.value = {
        title: res.data.data.title || 'Raw Telemetry',
        points: res.data.data.points || [],
        summary: res.data.data.summary || { min: 0, max: 0, avg: 0, current: 0, unit: '' },
        tableRows: res.data.data.tableRows || [],
        message: res.data.data.message || '',
      };
      showNotice('Report generated successfully', 'success');
    } else {
      showNotice(res.data?.error || 'Failed to fetch report data', 'error');
    }
  } catch (err: any) {
    showNotice(err?.response?.data?.error || err?.message || 'Failed to fetch report data', 'error');
  } finally {
    loading.value = false;
  }
};

// -----------------------------------------------------------------------------
// Filter Chips
// -----------------------------------------------------------------------------
const setQueryChip = (snippet: string) => {
  if (snippet === '*' || snippet === 'clear') {
    queryString.value = snippet === 'clear' ? '' : '*';
  } else {
    if (!queryString.value || queryString.value === '*') {
      queryString.value = snippet;
    } else {
      queryString.value += ` AND (${snippet})`;
    }
  }
};

// -----------------------------------------------------------------------------
// Filtered Table Rows
// -----------------------------------------------------------------------------
const displayRows = computed(() => {
  if (!reportData.value) return [];
  // If tableRows exist (e.g. from OpenSearch)
  if (reportData.value.tableRows && reportData.value.tableRows.length > 0) {
    return reportData.value.tableRows;
  }
  // Otherwise map data points to rows
  return reportData.value.points.map((pt, idx) => ({
    id: idx + 1,
    timestamp: pt.timestamp || pt.label,
    metric: reportData.value?.title || 'Value',
    value: `${pt.value} ${reportData.value?.summary?.unit || ''}`.trim(),
    source: sourceType.value.toUpperCase(),
  }));
});

const filteredRows = computed(() => {
  if (!tableSearch.value.trim()) return displayRows.value;
  const q = tableSearch.value.toLowerCase();
  return displayRows.value.filter((row) =>
    Object.values(row).some((val) => String(val).toLowerCase().includes(q))
  );
});

const paginatedRows = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage.value;
  return filteredRows.value.slice(start, start + itemsPerPage.value);
});

const totalPages = computed(() => {
  return Math.ceil(filteredRows.value.length / itemsPerPage.value) || 1;
});

// -----------------------------------------------------------------------------
// EXPORT FORMATS: CSV, TXT, JSON, PNG
// -----------------------------------------------------------------------------

// 1. Download CSV
const exportCsv = () => {
  const data = filteredRows.value;
  if (!data || data.length === 0) {
    showNotice('No data available to export', 'error');
    return;
  }

  const headers = Object.keys(data[0]);
  const rows = [headers.join(',')];

  data.forEach((row) => {
    const values = headers.map((h) => {
      const val = row[h] !== undefined && row[h] !== null ? String(row[h]) : '';
      return `"${val.replace(/"/g, '""')}"`;
    });
    rows.push(values.join(','));
  });

  const blob = new Blob([rows.join('\n')], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `raw-report-${sourceType.value}-${Date.now()}.csv`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
  showNotice('CSV report downloaded', 'success');
};

// 2. Download TXT (Log / Metrics Dump)
const exportTxt = () => {
  const data = filteredRows.value;
  if (!data || data.length === 0) {
    showNotice('No data available to export', 'error');
    return;
  }

  const lines = [
    `HEPHAESTUS CONTROL PANEL - RAW DATA REPORT`,
    `Generated At: ${new Date().toISOString()}`,
    `Data Source: ${sourceType.value.toUpperCase()}`,
    `Query / Target: ${queryString.value}`,
    `Total Records: ${data.length}`,
    `------------------------------------------------------------------------------`,
  ];

  data.forEach((row, i) => {
    if (row.timestamp && row.message) {
      lines.push(`[${row.timestamp}] [${row.level || 'INFO'}] ${row.message}`);
    } else {
      lines.push(`[#${i + 1}] ${JSON.stringify(row)}`);
    }
  });

  const blob = new Blob([lines.join('\n')], { type: 'text/plain;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `raw-report-${sourceType.value}-${Date.now()}.txt`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
  showNotice('TXT report downloaded', 'success');
};

// 3. Download JSON
const exportJson = () => {
  const payload = {
    metadata: {
      generatedAt: new Date().toISOString(),
      sourceType: sourceType.value,
      timeRange: timeRange.value,
      query: queryString.value,
      summary: reportData.value?.summary,
      totalCount: filteredRows.value.length,
    },
    data: filteredRows.value,
  };

  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `raw-report-${sourceType.value}-${Date.now()}.json`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
  showNotice('JSON report downloaded', 'success');
};

// 4. Download Chart as PNG
const downloadChartPng = () => {
  const svgElement = document.getElementById('raw-report-chart-svg');
  if (!svgElement) {
    showNotice('Chart SVG not found', 'error');
    return;
  }

  const svgData = new XMLSerializer().serializeToString(svgElement);
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  const img = new Image();

  const svgBlob = new Blob([svgData], { type: 'image/svg+xml;charset=utf-8' });
  const url = URL.createObjectURL(svgBlob);

  img.onload = () => {
    const width = 1000;
    const height = 400;
    canvas.width = width;
    canvas.height = height;

    if (ctx) {
      // Dark/light background fill
      ctx.fillStyle = '#0f172a';
      ctx.fillRect(0, 0, width, height);

      // Draw title
      ctx.fillStyle = '#94a3b8';
      ctx.font = 'bold 14px sans-serif';
      ctx.fillText(
        `Hephaestus Control Panel - ${reportData.value?.title || 'Telemetry Chart'} (${sourceType.value.toUpperCase()})`,
        30,
        30
      );

      ctx.drawImage(img, 20, 40, width - 40, height - 60);

      const pngUrl = canvas.toDataURL('image/png');
      const downloadLink = document.createElement('a');
      downloadLink.href = pngUrl;
      downloadLink.download = `chart-${sourceType.value}-${Date.now()}.png`;
      document.body.appendChild(downloadLink);
      downloadLink.click();
      document.body.removeChild(downloadLink);
      showNotice('Chart PNG downloaded', 'success');
    }
    URL.revokeObjectURL(url);
  };

  img.src = url;
};

// -----------------------------------------------------------------------------
// SVG Path Helpers
// -----------------------------------------------------------------------------
const generateSvgPath = (points?: Array<{ value: number }>, width = 800, height = 220): string => {
  if (!points || points.length === 0) return '';
  const min = Math.min(...points.map((p) => p.value));
  const max = Math.max(...points.map((p) => p.value));
  const range = max - min || 1;
  const paddingY = 30;
  const usableH = height - paddingY * 2;

  return points
    .map((p, i) => {
      const x = (i / (points.length - 1 || 1)) * (width - 60) + 30;
      const normalized = (p.value - min) / range;
      const y = height - paddingY - normalized * usableH;
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
};

const generateAreaPath = (points?: Array<{ value: number }>, width = 800, height = 220): string => {
  const line = generateSvgPath(points, width, height);
  if (!line) return '';
  const paddingY = 30;
  return `${line} L ${width - 30},${height - paddingY} L 30,${height - paddingY} Z`;
};

onMounted(async () => {
  await loadMetadata();
  await generateReport();
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
        &times;
      </button>
    </div>

    <!-- Header (Strictly complies with AGENTS.md: No icon in H1) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Raw Data Report
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Generate, filter, and export raw logs and telemetry to CSV, TXT, JSON, and PNG charts.
        </p>
      </div>

      <div class="flex items-center gap-2 shrink-0 flex-wrap">
        <button
          @click="generateReport"
          :disabled="loading"
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#151c2e] dark:hover:bg-[#1d273e] border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium text-slate-700 dark:text-slate-300 transition flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
        >
          <RefreshCw :class="['w-3.5 h-3.5 text-slate-500', loading ? 'animate-spin' : '']" />
          <span>{{ loading ? 'Generating...' : 'Refresh' }}</span>
        </button>

        <button
          @click="exportCsv"
          :disabled="!displayRows.length"
          class="px-3 py-1.5 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] hover:border-slate-400 text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer shadow-xs disabled:opacity-50"
          title="Export CSV spreadsheet"
        >
          <Download class="w-3.5 h-3.5 text-slate-400" />
          <span>CSV</span>
        </button>

        <button
          @click="exportTxt"
          :disabled="!displayRows.length"
          class="px-3 py-1.5 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] hover:border-slate-400 text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer shadow-xs disabled:opacity-50"
          title="Export plain text logs"
        >
          <FileText class="w-3.5 h-3.5 text-slate-400" />
          <span>TXT</span>
        </button>

        <button
          @click="exportJson"
          :disabled="!displayRows.length"
          class="px-3 py-1.5 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] hover:border-slate-400 text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer shadow-xs disabled:opacity-50"
          title="Export structured JSON"
        >
          <Layers class="w-3.5 h-3.5 text-slate-400" />
          <span>JSON</span>
        </button>

        <button
          @click="downloadChartPng"
          :disabled="!reportData?.points?.length"
          class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer shadow-xs disabled:opacity-50"
          title="Download rendered chart as PNG image"
        >
          <ImageIcon class="w-3.5 h-3.5" />
          <span>Download PNG</span>
        </button>
      </div>
    </div>

    <!-- Query Configuration Card -->
    <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 space-y-4 shadow-sm">
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4 text-xs">
        <!-- Data Source -->
        <div class="space-y-1.5">
          <label class="font-semibold text-slate-700 dark:text-slate-300">Data Source</label>
          <div class="grid grid-cols-2 gap-2">
            <label
              :class="[
                sourceType === 'opensearch'
                  ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/30 text-blue-600 dark:text-[#95CCDD]'
                  : 'border-slate-200 dark:border-[#1f283d]',
                'flex items-center gap-2 p-2 rounded-xl border cursor-pointer font-medium transition'
              ]"
            >
              <input type="radio" value="opensearch" v-model="sourceType" class="hidden" />
              <Database class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400" />
              <span>OpenSearch</span>
            </label>

            <label
              :class="[
                sourceType === 'prometheus'
                  ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/30 text-blue-600 dark:text-[#95CCDD]'
                  : 'border-slate-200 dark:border-[#1f283d]',
                'flex items-center gap-2 p-2 rounded-xl border cursor-pointer font-medium transition'
              ]"
            >
              <input type="radio" value="prometheus" v-model="sourceType" class="hidden" />
              <Activity class="w-4 h-4 shrink-0 text-slate-500 dark:text-slate-400" />
              <span>Prometheus</span>
            </label>
          </div>
        </div>

        <!-- Target (Index or Host) -->
        <div class="space-y-1.5">
          <label class="font-semibold text-slate-700 dark:text-slate-300">
            {{ sourceType === 'opensearch' ? 'Index Pattern' : 'Target Host' }}
          </label>
          <template v-if="sourceType === 'opensearch'">
            <select
              v-if="discoveredIndices.length > 0"
              v-model="indexPattern"
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
            >
              <option value="*">* (All Indices)</option>
              <option v-for="idx in discoveredIndices" :key="idx" :value="idx">{{ idx }}</option>
            </select>
            <input
              v-else
              v-model="indexPattern"
              type="text"
              placeholder="e.g. * or logs-*"
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
            />
          </template>
          <template v-else>
            <select
              v-model="targetHost"
              class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
            >
              <option value="all">All Monitored Hosts</option>
              <option v-for="h in discoveredHosts" :key="h.id" :value="h.host">
                {{ h.name }} ({{ h.host }})
              </option>
            </select>
          </template>
        </div>

        <!-- Time Range -->
        <div class="space-y-1.5">
          <label class="font-semibold text-slate-700 dark:text-slate-300">Time Range</label>
          <select
            v-model="timeRange"
            class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
          >
            <option value="1h">Last 1 Hour</option>
            <option value="6h">Last 6 Hours</option>
            <option value="24h">Last 24 Hours</option>
            <option value="7d">Last 7 Days</option>
            <option value="30d">Last 30 Days</option>
          </select>
        </div>

        <!-- Chart Type -->
        <div class="space-y-1.5">
          <label class="font-semibold text-slate-700 dark:text-slate-300">Visualization Type</label>
          <select
            v-model="chartType"
            class="w-full px-3 py-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
          >
            <option value="line">Line Trend Chart</option>
            <option value="area">Filled Area Chart</option>
            <option value="bar">Bar / Histogram</option>
          </select>
        </div>
      </div>

      <!-- Query Filter & Helper Chips -->
      <div class="space-y-2 pt-2 border-t border-slate-100 dark:border-[#1b2234] text-xs">
        <div class="flex items-center justify-between">
          <label class="font-semibold text-slate-700 dark:text-slate-300">
            {{ sourceType === 'opensearch' ? 'Lucene Filter Query' : 'PromQL Query Expression' }}
          </label>
          <div class="flex items-center gap-1.5 flex-wrap">
            <template v-if="sourceType === 'opensearch'">
              <button
                type="button"
                @click="setQueryChip('*')"
                class="px-2 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-[#1a2336] hover:bg-slate-200 dark:hover:bg-[#25324d] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
              >
                All (*)
              </button>
              <button
                type="button"
                @click="setQueryChip('status:>=500')"
                class="px-2 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-[#1a2336] hover:bg-slate-200 dark:hover:bg-[#25324d] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
              >
                status:>=500
              </button>
              <button
                type="button"
                @click="setQueryChip('level:ERROR')"
                class="px-2 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-[#1a2336] hover:bg-slate-200 dark:hover:bg-[#25324d] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
              >
                level:ERROR
              </button>
            </template>
            <template v-else>
              <button
                type="button"
                @click="queryString = '100 - (avg(rate(node_cpu_seconds_total{mode=\'idle\'}[5m])) * 100)'"
                class="px-2 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-[#1a2336] hover:bg-slate-200 dark:hover:bg-[#25324d] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
              >
                CPU %
              </button>
              <button
                type="button"
                @click="queryString = '(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100'"
                class="px-2 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-[#1a2336] hover:bg-slate-200 dark:hover:bg-[#25324d] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
              >
                RAM %
              </button>
              <button
                type="button"
                @click="queryString = 'node_load1'"
                class="px-2 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-[#1a2336] hover:bg-slate-200 dark:hover:bg-[#25324d] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
              >
                Load
              </button>
            </template>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <input
            v-model="queryString"
            type="text"
            :placeholder="sourceType === 'opensearch' ? 'e.g. status:>=500 OR level:ERROR, service:nginx' : 'e.g. 100 - (avg(rate(node_cpu_seconds_total{mode=\'idle\'}[5m])) * 100)'"
            class="flex-1 px-3 py-2 font-mono text-[11px] bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 focus:ring-1 focus:ring-blue-500"
            @keyup.enter="generateReport"
          />
          <button
            @click="generateReport"
            :disabled="loading"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg transition cursor-pointer text-xs shrink-0 disabled:opacity-50"
          >
            Run Query
          </button>
        </div>
      </div>
    </div>

    <!-- Telemetry Summary Cards -->
    <div v-if="reportData" class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-4 space-y-1 shadow-xs">
        <span class="text-[11px] text-slate-400 font-medium">Total Data Points</span>
        <div class="text-xl font-bold text-slate-900 dark:text-white">
          {{ reportData.points?.length || 0 }}
        </div>
      </div>

      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-4 space-y-1 shadow-xs">
        <span class="text-[11px] text-slate-400 font-medium">Average Value</span>
        <div class="text-xl font-bold text-slate-900 dark:text-white">
          {{ reportData.summary?.avg || 0 }} <span class="text-xs font-normal text-slate-400">{{ reportData.summary?.unit }}</span>
        </div>
      </div>

      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-4 space-y-1 shadow-xs">
        <span class="text-[11px] text-slate-400 font-medium">Peak / Max</span>
        <div class="text-xl font-bold text-slate-900 dark:text-white">
          {{ reportData.summary?.max || 0 }} <span class="text-xs font-normal text-slate-400">{{ reportData.summary?.unit }}</span>
        </div>
      </div>

      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl p-4 space-y-1 shadow-xs">
        <span class="text-[11px] text-slate-400 font-medium">Minimum Value</span>
        <div class="text-xl font-bold text-slate-900 dark:text-white">
          {{ reportData.summary?.min || 0 }} <span class="text-xs font-normal text-slate-400">{{ reportData.summary?.unit }}</span>
        </div>
      </div>
    </div>

    <!-- Chart Visualization & Download PNG -->
    <div v-if="reportData && reportData.points?.length" class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 space-y-4 shadow-sm">
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
        <div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            {{ reportData.title }}
          </h3>
          <p class="text-[11px] text-slate-400 font-mono">
            {{ reportData.message }}
          </p>
        </div>

        <button
          @click="downloadChartPng"
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2336] dark:hover:bg-[#222e47] border border-slate-200 dark:border-[#1f283d] rounded-lg text-xs font-medium text-slate-700 dark:text-slate-300 transition flex items-center gap-1.5 cursor-pointer"
        >
          <ImageIcon class="w-3.5 h-3.5 text-slate-400" />
          <span>Download PNG</span>
        </button>
      </div>

      <!-- Rendered SVG Chart -->
      <div class="w-full bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl p-4 overflow-hidden">
        <svg
          id="raw-report-chart-svg"
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 800 220"
          class="w-full h-56"
        >
          <defs>
            <linearGradient id="rawGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.45" />
              <stop offset="100%" stop-color="#3b82f6" stop-opacity="0.0" />
            </linearGradient>
          </defs>

          <!-- Grid Lines -->
          <line x1="30" y1="30" x2="770" y2="30" stroke="#334155" stroke-dasharray="3,3" stroke-width="0.5" />
          <line x1="30" y1="95" x2="770" y2="95" stroke="#334155" stroke-dasharray="3,3" stroke-width="0.5" />
          <line x1="30" y1="160" x2="770" y2="160" stroke="#334155" stroke-dasharray="3,3" stroke-width="0.5" />
          <line x1="30" y1="190" x2="770" y2="190" stroke="#475569" stroke-width="1" />

          <!-- Filled Area -->
          <path
            v-if="chartType === 'area' || chartType === 'line'"
            :d="generateAreaPath(reportData.points, 800, 220)"
            fill="url(#rawGradient)"
          />

          <!-- Line Path -->
          <path
            v-if="chartType === 'line' || chartType === 'area'"
            :d="generateSvgPath(reportData.points, 800, 220)"
            fill="none"
            stroke="#3b82f6"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />

          <!-- Bar Columns (if Bar chart chosen) -->
          <template v-if="chartType === 'bar'">
            <rect
              v-for="(p, i) in reportData.points"
              :key="i"
              :x="(i / (reportData.points.length || 1)) * 740 + 35"
              :y="190 - ((p.value - (reportData.summary?.min || 0)) / ((reportData.summary?.max - reportData.summary?.min) || 1)) * 140"
              :width="Math.max(6, 700 / (reportData.points.length * 1.8))"
              :height="Math.max(4, ((p.value - (reportData.summary?.min || 0)) / ((reportData.summary?.max - reportData.summary?.min) || 1)) * 140)"
              rx="2"
              fill="#3b82f6"
              opacity="0.85"
            />
          </template>

          <!-- Data Points & Labels -->
          <template v-if="chartType !== 'bar'">
            <circle
              v-for="(p, i) in reportData.points"
              :key="i"
              :cx="(i / (reportData.points.length - 1 || 1)) * 740 + 30"
              :cy="190 - ((p.value - (reportData.summary?.min || 0)) / ((reportData.summary?.max - reportData.summary?.min) || 1)) * 140"
              r="3.5"
              fill="#ffffff"
              stroke="#2563eb"
              stroke-width="2"
            />
          </template>

          <!-- X-Axis Labels -->
          <text
            v-for="(p, i) in reportData.points.filter((_, idx) => idx % Math.max(1, Math.floor(reportData.points.length / 8)) === 0)"
            :key="'lbl-' + i"
            :x="(reportData.points.indexOf(p) / (reportData.points.length - 1 || 1)) * 740 + 30"
            y="208"
            font-size="9"
            fill="#94a3b8"
            text-anchor="middle"
          >
            {{ p.label }}
          </text>
        </svg>
      </div>
    </div>

    <!-- Raw Data Stream / Table -->
    <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-5 space-y-4 shadow-sm">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-3 border-b border-slate-100 dark:border-[#1b2234] pb-3">
        <div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            Tabular Records & Events ({{ filteredRows.length }})
          </h3>
          <p class="text-[11px] text-slate-400">
            Raw payload records available for export to CSV or text log files
          </p>
        </div>

        <div class="flex items-center gap-2">
          <!-- Search in Table -->
          <div class="relative">
            <Search class="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-400" />
            <input
              v-model="tableSearch"
              type="text"
              placeholder="Search table rows..."
              class="pl-8 pr-3 py-1.5 text-xs bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200 w-48 focus:w-60 transition-all"
            />
          </div>

          <!-- Items Per Page -->
          <select
            v-model="itemsPerPage"
            class="px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] rounded-lg text-slate-800 dark:text-slate-200"
          >
            <option :value="10">10 / page</option>
            <option :value="25">25 / page</option>
            <option :value="50">50 / page</option>
            <option :value="100">100 / page</option>
          </select>
        </div>
      </div>

      <!-- Table Content -->
      <div class="overflow-x-auto rounded-xl border border-slate-200 dark:border-[#1b2234]">
        <table class="w-full text-left text-xs border-collapse font-mono">
          <thead class="bg-slate-50 dark:bg-[#0c101a] border-b border-slate-200 dark:border-[#1b2234] text-slate-500 dark:text-slate-400 text-[11px] uppercase tracking-wider">
            <tr>
              <th class="px-4 py-2.5 font-semibold">#</th>
              <th class="px-4 py-2.5 font-semibold">Timestamp</th>
              <th class="px-4 py-2.5 font-semibold">Source / Host</th>
              <th class="px-4 py-2.5 font-semibold">Level / Status</th>
              <th class="px-4 py-2.5 font-semibold">Details / Value / Payload</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]">
            <tr
              v-for="(row, idx) in paginatedRows"
              :key="idx"
              class="hover:bg-slate-50/60 dark:hover:bg-[#151c2e]/60 transition"
            >
              <td class="px-4 py-2.5 text-slate-400 text-[11px]">
                {{ (currentPage - 1) * itemsPerPage + idx + 1 }}
              </td>
              <td class="px-4 py-2.5 text-slate-700 dark:text-slate-300 whitespace-nowrap text-[11px]">
                {{ row.timestamp || row['@timestamp'] || '-' }}
              </td>
              <td class="px-4 py-2.5 text-slate-600 dark:text-slate-400 whitespace-nowrap text-[11px]">
                {{ row.host || row.source || sourceType.toUpperCase() }}
              </td>
              <td class="px-4 py-2.5 whitespace-nowrap">
                <span
                  :class="[
                    (row.level === 'ERROR' || row.level === 'FATAL' || String(row.status).startsWith('5'))
                      ? 'bg-rose-50 dark:bg-rose-950/40 text-rose-600 dark:text-rose-400 border border-rose-200 dark:border-rose-900/60'
                      : (row.level === 'WARN' || row.level === 'WARNING')
                      ? 'bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400 border border-amber-200 dark:border-amber-900/60'
                      : 'bg-slate-100 dark:bg-[#1a2336] text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-[#222e47]',
                    'px-2 py-0.5 rounded text-[10px] font-bold inline-block'
                  ]"
                >
                  {{ row.level || row.status || 'DATA' }}
                </span>
              </td>
              <td class="px-4 py-2.5 text-slate-800 dark:text-slate-200 text-[11px] max-w-md truncate">
                {{ row.message || row.value || JSON.stringify(row) }}
              </td>
            </tr>
            <tr v-if="!paginatedRows.length">
              <td colspan="5" class="px-4 py-8 text-center text-slate-400 text-xs font-sans">
                No matching records found for this query.
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination Footer -->
      <div class="flex items-center justify-between text-xs text-slate-500 pt-2">
        <span>
          Showing {{ ((currentPage - 1) * itemsPerPage) + 1 }} to {{ Math.min(currentPage * itemsPerPage, filteredRows.length) }} of {{ filteredRows.length }} rows
        </span>

        <div class="flex items-center gap-1">
          <button
            @click="currentPage = Math.max(1, currentPage - 1)"
            :disabled="currentPage <= 1"
            class="p-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] hover:bg-slate-50 dark:hover:bg-[#151c2e] disabled:opacity-40 cursor-pointer"
          >
            <ChevronLeft class="w-3.5 h-3.5" />
          </button>
          <span class="px-2.5 py-1 text-slate-700 dark:text-slate-300 font-semibold">
            {{ currentPage }} / {{ totalPages }}
          </span>
          <button
            @click="currentPage = Math.min(totalPages, currentPage + 1)"
            :disabled="currentPage >= totalPages"
            class="p-1.5 rounded-lg border border-slate-200 dark:border-[#1f283d] hover:bg-slate-50 dark:hover:bg-[#151c2e] disabled:opacity-40 cursor-pointer"
          >
            <ChevronRight class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
