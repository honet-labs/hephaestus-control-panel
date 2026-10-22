<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import axios from 'axios';
import {
  RotateCw,
  Settings,
  ArrowLeft,
  Clock,
  Search,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Database,
  Server,
  Layers,
  Activity,
  HardDrive,
  Cpu,
  Eye,
  Sliders,
  Terminal,
  ExternalLink,
  AlertCircle,
  X,
  Zap,
  Share2,
  Copy,
  Check,
  Code2,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  Download,
  Play,
  Send,
  Braces,
  Sparkles,
  Trash2,
  History,
  Wrench,
  HelpCircle,
  FileText,
  Upload,
} from 'lucide-vue-next';
import ThemeToggle from '../components/ThemeToggle.vue';
import { useAuthStore } from '../stores/auth';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const canManage = computed(() => authStore.can('opensearch', 'manage'));

// Embed / Standalone Mode Detection
const isEmbedMode = computed(() => {
  return route.query.embed === 'true' || route.query.kiosk === 'true' || route.query.shared === 'true';
});
const showHeader = computed(() => {
  return route.query.header !== 'false';
});

// Share / Embed Modal State
const isShareModalOpen = ref(false);
const shareIncludeToken = ref(true);
const shareEmbedMode = ref(true);
const shareHideHeader = ref(false);
const shareTheme = ref<'auto' | 'dark' | 'light'>('auto');
const shareRefreshSec = ref(30);
const copiedUrl = ref(false);
const copiedIframe = ref(false);

const generatedShareUrl = computed(() => {
  const origin = window.location.origin;
  const params = new URLSearchParams();
  params.set('tab', 'overview');
  if (shareEmbedMode.value) {
    params.set('embed', 'true');
  }
  if (shareHideHeader.value) {
    params.set('header', 'false');
  }
  if (shareTheme.value !== 'auto') {
    params.set('theme', shareTheme.value);
  }
  if (shareRefreshSec.value !== 30 && shareRefreshSec.value > 0) {
    params.set('refresh', shareRefreshSec.value.toString());
  }
  if (shareIncludeToken.value) {
    const token = authStore.token || sessionStorage.getItem('hcp_session');
    if (token) {
      params.set('token', token);
    }
  }
  return `${origin}/opensearch-cluster?${params.toString()}`;
});

const generatedIframeCode = computed(() => {
  return `<iframe src="${generatedShareUrl.value}" width="100%" height="850" frameborder="0" allowfullscreen style="border: 0; border-radius: 8px;"></iframe>`;
});

const copyShareUrl = async () => {
  try {
    await navigator.clipboard.writeText(generatedShareUrl.value);
    copiedUrl.value = true;
    setTimeout(() => { copiedUrl.value = false; }, 2500);
  } catch (_) {}
};

const copyIframeCode = async () => {
  try {
    await navigator.clipboard.writeText(generatedIframeCode.value);
    copiedIframe.value = true;
    setTimeout(() => { copiedIframe.value = false; }, 2500);
  } catch (_) {}
};

const openEmbedPreview = () => {
  window.open(generatedShareUrl.value, '_blank');
};

// Active Navigation Tab
const activeTab = ref<'overview' | 'nodes' | 'indices' | 'shards' | 'query' | 'connection' | 'logs'>('overview');

// Auto-Refresh state
const refreshIntervalSec = ref(30);
const countdown = ref(30);
const isRefreshing = ref(false);
const timer = ref<any>(null);
const isSettingsModalOpen = ref(false);

// Data states
const error = ref('');
const clusterHealth = ref<any | null>(null);
const nodesList = ref<any[]>([]);
const indicesList = ref<any[]>([]);
const shardsList = ref<any[]>([]);
const logsList = ref<any[]>([]);
const recoveryList = ref<any[]>([]);

// Selected Index Detail Modal
const selectedIndexModal = ref<any | null>(null);

// Connection Config State
const configForm = ref({
  id: '',
  name: '',
  host: '',
  port: 9200,
  username: '',
  password: '',
  useSsl: false,
  verifySsl: false,
  isActive: true,
});
const testResult = ref<string | null>(null);
const testSuccess = ref(false);
const isTestingConnection = ref(false);
const isSavingConfig = ref(false);

// Filter & Search states
const indexSearch = ref('');
const shardSearch = ref('');
const indexHealthFilter = ref<'all' | 'unhealthy' | 'green'>('all');
const shardStateFilter = ref<'all' | 'unassigned' | 'inflight' | 'started'>('all');

// Hover Tooltip State for Shards (Matching Screenshot)
const hoveredShard = ref<any | null>(null);
const tooltipPosition = ref({ x: 0, y: 0 });

// Formatted Overview Computations
const totalIndicesCount = computed(() => indicesList.value.length);
const totalDocumentsCount = computed(() => {
  return indicesList.value.reduce((acc, idx) => acc + (parseInt(idx['docs.count'] || idx.docsCount || 0) || 0), 0);
});

// Shard State Categorization (Unassigned & In-Flight Replications)
const unassignedShards = computed(() => {
  return shardsList.value.filter(s => {
    const state = (s.state || '').toUpperCase();
    const node = s.node || s.nodeName;
    return state === 'UNASSIGNED' || !node || node === 'UNASSIGNED';
  });
});

const inFlightShards = computed(() => {
  return shardsList.value.filter(s => {
    const state = (s.state || '').toUpperCase();
    return state === 'INITIALIZING' || state === 'RELOCATING';
  });
});

const startedShards = computed(() => {
  return shardsList.value.filter(s => {
    const state = (s.state || '').toUpperCase();
    const node = s.node || s.nodeName;
    return state === 'STARTED' && node && node !== 'UNASSIGNED';
  });
});

// Filtered In-Flight, Recovery & Unassigned Shards (synced with shardSearch)
const filteredRecoveryList = computed(() => {
  if (!shardSearch.value) return recoveryList.value;
  const q = shardSearch.value.toLowerCase();
  return recoveryList.value.filter((rec: any) =>
    (rec.index || '').toLowerCase().includes(q) ||
    (rec.source_node || '').toLowerCase().includes(q) ||
    (rec.target_node || '').toLowerCase().includes(q)
  );
});

const filteredInFlightShards = computed(() => {
  let list = inFlightShards.value;
  if (!shardSearch.value) return list;
  const q = shardSearch.value.toLowerCase();
  return list.filter(
    s => (s.index || s.name || '').toLowerCase().includes(q) ||
         (s.node || s.nodeName || '').toLowerCase().includes(q) ||
         (s.ip || '').toLowerCase().includes(q)
  );
});

const filteredUnassignedShards = computed(() => {
  let list = unassignedShards.value;
  if (!shardSearch.value) return list;
  const q = shardSearch.value.toLowerCase();
  return list.filter(
    s => (s.index || s.name || '').toLowerCase().includes(q) ||
         (s.node || s.nodeName || '').toLowerCase().includes(q) ||
         (s.ip || '').toLowerCase().includes(q)
  );
});

const unhealthyIndices = computed(() => {
  return indicesList.value.filter(idx => {
    const health = (idx.health || '').toLowerCase();
    return health === 'yellow' || health === 'red';
  });
});

const greenIndices = computed(() => {
  return indicesList.value.filter(idx => {
    const health = (idx.health || '').toLowerCase();
    return health === 'green';
  });
});

const getShardBadgeLabel = (shard: any) => {
  const state = (shard.state || '').toUpperCase();
  const num = shard.shard !== undefined && shard.shard !== null ? shard.shard : '';
  if (state === 'INITIALIZING') return `I${num}`;
  if (state === 'RELOCATING') return `M${num}`;
  if (state === 'UNASSIGNED') return `U${num}`;
  if (shard.prirep === 'p' || shard.type === 'Primary') return `P${num}`;
  if (shard.prirep === 'r' || shard.type === 'Replica') return `R${num}`;
  return `S${num}`;
};

const getShardColorClass = (shard: any, isHighlighted = false, isDimmed = false) => {
  const state = (shard.state || '').toUpperCase();
  const isPrimary = shard.prirep === 'p' || shard.type === 'Primary';

  if (state === 'INITIALIZING') {
    if (isHighlighted) return 'bg-amber-500 text-slate-950 ring-amber-400/50';
    if (isDimmed) return 'bg-[#3b2a12]/25 border border-[#784e1b]/20 text-amber-300/30';
    return 'bg-[#3b2a12] border border-[#784e1b] text-amber-300 animate-pulse';
  }
  if (state === 'RELOCATING') {
    if (isHighlighted) return 'bg-purple-500 text-white ring-purple-400/50';
    if (isDimmed) return 'bg-[#2e153b]/25 border border-[#6b258a]/20 text-purple-300/30';
    return 'bg-[#2e153b] border border-[#6b258a] text-purple-300 animate-pulse';
  }
  if (state === 'UNASSIGNED') {
    if (isHighlighted) return 'bg-red-500 text-white ring-red-400/50';
    if (isDimmed) return 'bg-[#3b1219]/25 border border-[#822735]/20 text-[#f87171]/30';
    return 'bg-[#3b1219] border border-[#822735] text-[#f87171]';
  }
  if (isPrimary) {
    if (isHighlighted) return 'bg-[#10b981] text-white ring-emerald-500/40';
    if (isDimmed) return 'bg-[#0f3d28]/25 border border-[#1c6b47]/20 text-[#4ade80]/30';
    return 'bg-[#0f3d28] border border-[#1c6b47] text-[#4ade80]';
  }
  // Replica
  if (isHighlighted) return 'bg-[#3b82f6] text-white ring-blue-500/40';
  if (isDimmed) return 'bg-[#132c4a]/25 border border-[#1e4976]/20 text-[#60a5fa]/30';
  return 'bg-[#132c4a] border border-[#1e4976] text-[#60a5fa]';
};

function parseBytesString(val: any): number {
  if (typeof val === 'number') return val;
  if (!val || typeof val !== 'string') return 0;
  const s = val.trim().toLowerCase();
  const num = parseFloat(s);
  if (isNaN(num)) return 0;
  if (s.endsWith('tb') || s.endsWith('t')) return num * 1024 * 1024 * 1024 * 1024;
  if (s.endsWith('gb') || s.endsWith('g')) return num * 1024 * 1024 * 1024;
  if (s.endsWith('mb') || s.endsWith('m')) return num * 1024 * 1024;
  if (s.endsWith('kb') || s.endsWith('k')) return num * 1024;
  if (s.endsWith('b')) return num;
  return num;
}

const totalStoreSizeBytes = computed(() => {
  if (indicesList.value.length > 0) {
    const totalBytes = indicesList.value.reduce((acc, idx) => {
      const raw = idx['store.size'] || idx.storeSize || idx['pri.store.size'] || 0;
      return acc + parseBytesString(raw);
    }, 0);
    if (totalBytes > 0) {
      return formatBytes(totalBytes);
    }
  }
  if (nodesList.value.length > 0) {
    const fsUsed = nodesList.value.reduce((acc, n) => acc + (n.diskUsedBytes || n.fsUsedBytes || 0), 0);
    if (fsUsed > 0) return formatBytes(fsUsed);
  }
  return '0 B';
});

// All Distinct Actual Node Names (excluding unassigned)
const clusterNodes = computed(() => {
  const map: Record<string, { name: string; ip: string; primaryCount: number; replicaCount: number }> = {};
  
  nodesList.value.forEach(n => {
    if (n.name && n.name.toLowerCase() !== 'unassigned') {
      map[n.name] = {
        name: n.name,
        ip: n.ip || '-',
        primaryCount: 0,
        replicaCount: 0,
      };
    }
  });

  shardsList.value.forEach(s => {
    const nodeName = s.node || s.nodeName;
    if (!nodeName || nodeName.toLowerCase() === 'unassigned' || s.state === 'UNASSIGNED') return;
    if (!map[nodeName]) {
      map[nodeName] = {
        name: nodeName,
        ip: s.ip || '-',
        primaryCount: 0,
        replicaCount: 0,
      };
    }
    if (s.prirep === 'p' || s.type === 'Primary') {
      map[nodeName].primaryCount++;
    } else if (s.prirep === 'r' || s.type === 'Replica') {
      map[nodeName].replicaCount++;
    }
  });

  return Object.values(map);
});

// Indices Sorting State
const indexSortKey = ref<'default' | 'storeSize' | 'priStoreSize' | 'docs' | 'name'>('default');
const indexSortOrder = ref<'asc' | 'desc'>('desc');

const toggleSort = (key: 'storeSize' | 'priStoreSize' | 'docs' | 'name') => {
  if (indexSortKey.value === key) {
    if (indexSortOrder.value === 'desc') {
      indexSortOrder.value = 'asc';
    } else {
      indexSortKey.value = 'default';
      indexSortOrder.value = 'desc';
    }
  } else {
    indexSortKey.value = key;
    indexSortOrder.value = 'desc';
  }
};

const sortSelectValue = computed({
  get() {
    if (indexSortKey.value === 'default') return 'default';
    return `${indexSortKey.value}_${indexSortOrder.value}`;
  },
  set(val: string) {
    if (val === 'default') {
      indexSortKey.value = 'default';
      indexSortOrder.value = 'desc';
    } else {
      const [key, order] = val.split('_') as ['storeSize' | 'priStoreSize' | 'docs' | 'name', 'asc' | 'desc'];
      indexSortKey.value = key;
      indexSortOrder.value = order;
    }
  }
});

// Filtered and sorted indices (with Health Filter tab & Size/Docs/Name Sort support)
const filteredIndices = computed(() => {
  let list = indicesList.value.slice();
  if (indexHealthFilter.value === 'unhealthy') {
    list = list.filter(idx => (idx.health || '').toLowerCase() === 'yellow' || (idx.health || '').toLowerCase() === 'red');
  } else if (indexHealthFilter.value === 'green') {
    list = list.filter(idx => (idx.health || '').toLowerCase() === 'green');
  }

  if (indexSearch.value) {
    const q = indexSearch.value.toLowerCase();
    list = list.filter(idx => (idx.index || idx.name || '').toLowerCase().includes(q));
  }

  if (indexSortKey.value === 'default') {
    return list;
  }

  return list.sort((a, b) => {
    let valA = 0;
    let valB = 0;

    if (indexSortKey.value === 'storeSize') {
      valA = parseBytesString(a['store.size'] || a.storeSize || 0);
      valB = parseBytesString(b['store.size'] || b.storeSize || 0);
    } else if (indexSortKey.value === 'priStoreSize') {
      valA = parseBytesString(a['pri.store.size'] || a.priStoreSize || 0);
      valB = parseBytesString(b['pri.store.size'] || b.priStoreSize || 0);
    } else if (indexSortKey.value === 'docs') {
      valA = Number(a['docs.count'] || a.docsCount || 0);
      valB = Number(b['docs.count'] || b.docsCount || 0);
    } else if (indexSortKey.value === 'name') {
      const nameA = (a.index || a.name || '').toLowerCase();
      const nameB = (b.index || b.name || '').toLowerCase();
      return indexSortOrder.value === 'asc' ? nameA.localeCompare(nameB) : nameB.localeCompare(nameA);
    }

    if (indexSortOrder.value === 'asc') {
      return valA - valB;
    } else {
      return valB - valA;
    }
  });
});

// Export filtered & sorted indices to CSV
const exportIndicesToCsv = () => {
  if (filteredIndices.value.length === 0) return;

  const headers = [
    'Health',
    'Status',
    'Index',
    'UUID',
    'Primary',
    'Replica',
    'Docs Count',
    'Docs Deleted',
    'Store Size',
    'Primary Size',
  ];

  const escapeCsv = (val: any): string => {
    if (val === null || val === undefined) return '""';
    const s = String(val).replace(/"/g, '""');
    return `"${s}"`;
  };

  const rows = filteredIndices.value.map(idx => [
    escapeCsv(idx.health || 'green'),
    escapeCsv(idx.status || 'open'),
    escapeCsv(idx.index || idx.name || ''),
    escapeCsv(idx.uuid || ''),
    escapeCsv(idx.pri || idx.primary || 1),
    escapeCsv(idx.rep || idx.replica || 0),
    escapeCsv(idx['docs.count'] || idx.docsCount || 0),
    escapeCsv(idx['docs.deleted'] || idx.docsDeleted || 0),
    escapeCsv(idx['store.size'] || idx.storeSize || '0b'),
    escapeCsv(idx['pri.store.size'] || idx.priStoreSize || '0b'),
  ]);

  const csvContent = [headers.join(','), ...rows.map(r => r.join(','))].join('\r\n');
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');

  const timestamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-');
  const clusterName = (clusterHealth.value?.cluster_name || 'opensearch').toLowerCase().replace(/\s+/g, '-');
  link.setAttribute('href', url);
  link.setAttribute('download', `${clusterName}-indices-${timestamp}.csv`);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

// =================================================================
// TAB: DEV TOOLS / OPENSEARCH CONSOLE WORKBENCH
// =================================================================
const activeDevToolsTab = ref<'console' | 'profiler'>('console');

// Default console input matching the user's screenshot
const consoleInput = ref(
`POST _bulk
{ "create": { "_index": "students", "_id": "2" } }
{ "name": "Jonathan Powers", "gpa": 3.85, "grad_year": 2025 }
{ "create": { "_index": "students", "_id": "3" } }
{ "name": "Jane Doe", "gpa": 3.52, "grad_year": 2024 }`
);

const queryMethod = ref<'GET' | 'POST' | 'PUT' | 'DELETE' | 'HEAD'>('POST');
const queryEndpoint = ref('/_bulk');
const queryBody = ref('');
const queryLoading = ref(false);
const queryError = ref('');

// Pre-populated with the exact response from the OpenSearch Dev Tools screenshot
const queryResult = ref<{
  statusCode: number;
  statusText: string;
  latencyMs: number;
  data: any;
  raw?: string;
} | null>({
  statusCode: 200,
  statusText: 'OK',
  latencyMs: 651,
  data: {
    took: 71,
    errors: false,
    items: [
      {
        create: {
          _index: "students",
          _id: "2",
          _version: 1,
          result: "created",
          _shards: {
            total: 2,
            successful: 1,
            failed: 0
          },
          _seq_no: 0,
          _primary_term: 1,
          status: 201
        }
      },
      {
        create: {
          _index: "students",
          _id: "3",
          _version: 1,
          result: "created",
          _shards: {
            total: 2,
            successful: 1,
            failed: 0
          },
          _seq_no: 1,
          _primary_term: 1,
          status: 201
        }
      }
    ]
  }
});

const copiedQueryResult = ref(false);
const copiedCurl = ref(false);
const queryHistory = ref<Array<{ method: string; endpoint: string; body: string; timestamp: string; status: number }>>([
  {
    method: 'POST',
    endpoint: '/_bulk',
    body: `POST _bulk\n{ "create": { "_index": "students", "_id": "2" } }\n{ "name": "Jonathan Powers", "gpa": 3.85, "grad_year": 2025 }\n{ "create": { "_index": "students", "_id": "3" } }\n{ "name": "Jane Doe", "gpa": 3.52, "grad_year": 2024 }`,
    timestamp: 'Recent',
    status: 200
  }
]);

// Modals & Menu State
const showHistoryModal = ref(false);
const showSettingsModal = ref(false);
const showHelpModal = ref(false);
const showImportModal = ref(false);
const showWrenchMenu = ref(false);
const importInputText = ref('');
const selectedPresetKey = ref('');

// Settings
const consoleFontSize = ref<'11' | '12' | '13' | '14'>('12');
const wrapResponse = ref(true);

// Resizing state
const editorWidthPercent = ref(50);
const isResizing = ref(false);
const splitContainerRef = ref<HTMLElement | null>(null);
const editorTextareaRef = ref<HTMLTextAreaElement | null>(null);
const editorGutterRef = ref<HTMLElement | null>(null);
const responseContainerRef = ref<HTMLElement | null>(null);
const responseGutterRef = ref<HTMLElement | null>(null);

// Profiler tab state
const profilerQueryInput = ref(
`POST _search
{
  "profile": true,
  "query": {
    "match_all": {}
  }
}`
);
const profilerLoading = ref(false);
const profilerResult = ref<any>(null);

interface QueryPreset {
  id: string;
  name: string;
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'HEAD';
  endpoint: string;
  body: string;
  description: string;
}

const QUERY_PRESETS: QueryPreset[] = [
  {
    id: 'bulk_students',
    name: 'Bulk Insert Sample Students',
    method: 'POST',
    endpoint: '/_bulk',
    body: '{\n  "query": {\n    "match_all": {}\n  }\n}',
    description: 'Bulk insert sample student records into students index',
  },
  {
    id: 'search_all',
    name: 'Search All Documents (Match All)',
    method: 'GET',
    endpoint: '/_search',
    body: '{\n  "query": {\n    "match_all": {}\n  },\n  "size": 20\n}',
    description: 'Retrieve the first 20 indexed documents across all indices',
  },
  {
    id: 'search_filtered',
    name: 'Search with Bool Filter & Sort',
    method: 'POST',
    endpoint: '/_search',
    body: '{\n  "size": 25,\n  "query": {\n    "bool": {\n      "must": [\n        { "match_all": {} }\n      ]\n    }\n  },\n  "sort": [\n    { "@timestamp": { "order": "desc", "unmapped_type": "boolean" } }\n  ]\n}',
    description: 'Full query DSL structure with boolean conditions and timestamp sort',
  },
  {
    id: 'count_docs',
    name: 'Count Documents (Cluster-wide)',
    method: 'GET',
    endpoint: '/_count',
    body: '{\n  "query": {\n    "match_all": {}\n  }\n}',
    description: 'Get exact count of all matched documents',
  },
  {
    id: 'cluster_health',
    name: 'Cluster Health (Pretty JSON)',
    method: 'GET',
    endpoint: '/_cluster/health?pretty',
    body: '',
    description: 'Cluster status, shard totals, and unassigned state',
  },
  {
    id: 'cluster_stats',
    name: 'Cluster Statistics & Node Hardware',
    method: 'GET',
    endpoint: '/_cluster/stats?pretty',
    body: '',
    description: 'Node memory, CPU, filesystem usage, and plugin counts',
  },
  {
    id: 'cat_indices',
    name: 'Cat Indices (Sorted by Size)',
    method: 'GET',
    endpoint: '/_cat/indices?v&s=store.size:desc',
    body: '',
    description: 'Tabular view of all indices sorted by store size descending',
  },
  {
    id: 'cat_shards',
    name: 'Cat Shards Allocation',
    method: 'GET',
    endpoint: '/_cat/shards?v',
    body: '',
    description: 'Inspect exact shard placements across cluster nodes',
  },
  {
    id: 'cat_nodes',
    name: 'Cat Nodes Hardware Summary',
    method: 'GET',
    endpoint: '/_cat/nodes?v&h=name,ip,disk.used_percent,cpu,load_1m,node.role',
    body: '',
    description: 'View real-time node resource pressure and roles',
  },
  {
    id: 'aggs_terms',
    name: 'Terms Aggregation (Top Indices Breakdown)',
    method: 'POST',
    endpoint: '/_search',
    body: '{\n  "size": 0,\n  "aggs": {\n    "top_indices": {\n      "terms": {\n        "field": "_index",\n        "size": 15\n      }\n    }\n  }\n}',
    description: 'Aggregate document distribution across top indices',
  },
  {
    id: 'cluster_settings',
    name: 'Cluster Settings (Persistent / Transient)',
    method: 'GET',
    endpoint: '/_cluster/settings?include_defaults=false',
    body: '',
    description: 'View active custom cluster configurations',
  },
];

// Computed editor line numbers
const editorLines = computed(() => {
  const lines = consoleInput.value.split('\n');
  return lines.map((_, idx) => idx + 1);
});

// Computed response string and folding lines
const formattedResponseStr = computed(() => {
  if (!queryResult.value) return '';
  if (typeof queryResult.value.data === 'string') {
    return queryResult.value.data;
  }
  try {
    return JSON.stringify(queryResult.value.data, null, 2);
  } catch {
    return String(queryResult.value.data);
  }
});

const responseLineItems = computed(() => {
  if (!formattedResponseStr.value) return [];
  const lines = formattedResponseStr.value.split('\n');
  return lines.map((line, idx) => {
    const trimmed = line.trim();
    // OpenSearch Console displays ▾ on lines opening an object or array
    const hasFold = trimmed.endsWith('{') || trimmed.endsWith('[') || trimmed === '{' || trimmed === '[';
    return {
      lineNum: idx + 1,
      hasFold,
    };
  });
});

function escapeHtml(str: string) {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function syntaxHighlightJson(jsonObj: any): string {
  if (jsonObj === undefined || jsonObj === null) return 'null';
  const str = JSON.stringify(jsonObj, null, 2);
  const escaped = escapeHtml(str);

  return escaped.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    (match) => {
      let cls = 'text-[#005a9e] dark:text-[#818cf8]'; // numbers
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          // Object key
          cls = 'text-[#006bb4] dark:text-[#38bdf8] font-semibold';
          return `<span class="${cls}">${match.slice(0, -1)}</span>:`;
        } else {
          // String value
          cls = 'text-[#00805a] dark:text-[#34d399]';
        }
      } else if (/true|false/.test(match)) {
        // Boolean
        cls = 'text-[#6f42c1] dark:text-[#c084fc] font-semibold';
      } else if (/null/.test(match)) {
        // Null
        cls = 'text-slate-400 dark:text-slate-500 italic';
      }
      return `<span class="${cls}">${match}</span>`;
    }
  );
}

const highlightedResponseHtml = computed(() => {
  if (!queryResult.value) return '';
  if (typeof queryResult.value.data === 'string') {
    return escapeHtml(queryResult.value.data);
  }
  return syntaxHighlightJson(queryResult.value.data);
});

// Robust Dev Tools Request Parser
function parseConsoleQuery(rawText: string) {
  const lines = rawText.split('\n');
  let method = 'GET';
  let endpoint = '/_search';
  let bodyStartIndex = -1;

  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (!trimmed || trimmed.startsWith('#') || trimmed.startsWith('//')) {
      continue;
    }
    const match = trimmed.match(/^(GET|POST|PUT|DELETE|HEAD)\s+(.+)$/i);
    if (match) {
      method = match[1].toUpperCase();
      endpoint = match[2].trim();
      bodyStartIndex = i + 1;
      break;
    } else if (trimmed.startsWith('/') || trimmed.startsWith('_')) {
      endpoint = trimmed;
      bodyStartIndex = i + 1;
      break;
    }
  }

  if (!endpoint.startsWith('/')) {
    endpoint = '/' + endpoint;
  }

  let body: any = null;
  let bodyText = '';
  if (bodyStartIndex !== -1 && bodyStartIndex < lines.length) {
    bodyText = lines.slice(bodyStartIndex).join('\n').trim();
  }

  if (bodyText) {
    if (endpoint.includes('_bulk')) {
      body = bodyText.endsWith('\n') ? bodyText : bodyText + '\n';
    } else {
      try {
        body = JSON.parse(bodyText);
      } catch (_) {
        body = bodyText;
      }
    }
  }

  return { method, endpoint, body, bodyText };
}

// Execute OpenSearch Dev Tools Query
const executeOpenSearchQuery = async () => {
  queryLoading.value = true;
  queryError.value = '';

  const { method, endpoint, body, bodyText } = parseConsoleQuery(consoleInput.value);
  queryMethod.value = method as any;
  queryEndpoint.value = endpoint;
  queryBody.value = bodyText;

  const startTime = Date.now();
  try {
    const res = await axios.post('/api/v1/opensearch/query', {
      method,
      endpoint,
      body,
    });

    const elapsed = Date.now() - startTime;
    if (res.data && res.data.success && res.data.data) {
      queryResult.value = {
        statusCode: res.data.data.statusCode || 200,
        statusText: res.data.data.statusText || 'OK',
        latencyMs: res.data.data.latencyMs ?? elapsed,
        data: res.data.data.data,
        raw: res.data.data.raw,
      };

      // Add to query history (max 20)
      queryHistory.value.unshift({
        method,
        endpoint,
        body: consoleInput.value,
        timestamp: new Date().toLocaleTimeString(),
        status: res.data.data.statusCode || 200,
      });
      if (queryHistory.value.length > 20) {
        queryHistory.value.pop();
      }
    } else {
      const errMsg = res.data?.error || 'Failed to execute query';
      queryError.value = errMsg;
      queryResult.value = {
        statusCode: 500,
        statusText: 'ERR',
        latencyMs: elapsed,
        data: { error: errMsg },
      };
    }
  } catch (err: any) {
    const elapsed = Date.now() - startTime;
    const errMsg = err.response?.data?.error || err.message || 'Error connecting to OpenSearch API';
    queryError.value = errMsg;
    queryResult.value = {
      statusCode: err.response?.status || 502,
      statusText: err.response?.statusText || 'ERR',
      latencyMs: elapsed,
      data: { error: errMsg, details: err.response?.data },
    };
  } finally {
    queryLoading.value = false;
  }
};

// Beautify / Auto-indent JSON in Console
const beautifyConsoleJson = () => {
  const { method, endpoint, bodyText } = parseConsoleQuery(consoleInput.value);
  if (!bodyText) return;

  if (endpoint.includes('_bulk')) {
    const lines = bodyText.split('\n');
    const formatted = lines
      .map(line => {
        const t = line.trim();
        if (!t) return '';
        try {
          return JSON.stringify(JSON.parse(t));
        } catch {
          return t;
        }
      })
      .filter(Boolean)
      .join('\n');
    consoleInput.value = `${method} ${endpoint}\n${formatted}\n`;
  } else {
    try {
      const parsed = JSON.parse(bodyText);
      consoleInput.value = `${method} ${endpoint}\n${JSON.stringify(parsed, null, 2)}`;
    } catch (e: any) {
      queryError.value = `Unable to format JSON: ${e.message}`;
      setTimeout(() => { queryError.value = ''; }, 3000);
    }
  }
  showWrenchMenu.value = false;
};

// Copy as cURL command
const copyAsCurl = async () => {
  const { method, endpoint, bodyText } = parseConsoleQuery(consoleInput.value);
  const host = activeConfig.value?.host || 'localhost';
  const port = activeConfig.value?.port || 9200;
  const proto = activeConfig.value?.useSSL ? 'https' : 'http';
  let curl = `curl -X ${method} "${proto}://${host}:${port}${endpoint}" -H 'Content-Type: application/json'`;
  if (bodyText) {
    curl += ` -d '${bodyText.replace(/'/g, `'\\''`)}'`;
  }
  try {
    await navigator.clipboard.writeText(curl);
    copiedCurl.value = true;
    showWrenchMenu.value = false;
    setTimeout(() => { copiedCurl.value = false; }, 2000);
  } catch (_) {}
};

// Copy Response JSON
const copyResponseJson = async () => {
  if (!formattedResponseStr.value) return;
  try {
    await navigator.clipboard.writeText(formattedResponseStr.value);
    copiedQueryResult.value = true;
    setTimeout(() => { copiedQueryResult.value = false; }, 2000);
  } catch (_) {}
};

// Export console session
const exportConsoleSession = () => {
  const payload = {
    exportedAt: new Date().toISOString(),
    query: consoleInput.value,
    result: queryResult.value,
  };
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `opensearch-devtools-${Date.now()}.json`;
  a.click();
  URL.revokeObjectURL(url);
};

// Import custom query text
const importQueryText = () => {
  if (!importInputText.value.trim()) return;
  consoleInput.value = importInputText.value.trim();
  showImportModal.value = false;
  importInputText.value = '';
};

// Load Preset to Console
const loadPresetToConsole = (preset: QueryPreset) => {
  if (preset.id === 'bulk_students') {
    consoleInput.value = `POST _bulk\n{ "create": { "_index": "students", "_id": "2" } }\n{ "name": "Jonathan Powers", "gpa": 3.85, "grad_year": 2025 }\n{ "create": { "_index": "students", "_id": "3" } }\n{ "name": "Jane Doe", "gpa": 3.52, "grad_year": 2024 }`;
  } else if (preset.body) {
    consoleInput.value = `${preset.method} ${preset.endpoint}\n${preset.body}`;
  } else {
    consoleInput.value = `${preset.method} ${preset.endpoint}`;
  }
  showWrenchMenu.value = false;
  showSettingsModal.value = false;
};

// Clear Response
const clearResponse = () => {
  queryResult.value = null;
  queryError.value = '';
};

// Resizer interaction
const startResize = (e: MouseEvent) => {
  isResizing.value = true;
  document.body.style.userSelect = 'none';
  document.body.style.cursor = 'col-resize';

  const onMouseMove = (moveEvent: MouseEvent) => {
    if (!splitContainerRef.value) return;
    const rect = splitContainerRef.value.getBoundingClientRect();
    const offsetX = moveEvent.clientX - rect.left;
    const newPercent = Math.max(25, Math.min(75, (offsetX / rect.width) * 100));
    editorWidthPercent.value = Math.round(newPercent);
  };

  const onMouseUp = () => {
    isResizing.value = false;
    document.body.style.userSelect = '';
    document.body.style.cursor = '';
    window.removeEventListener('mousemove', onMouseMove);
    window.removeEventListener('mouseup', onMouseUp);
  };

  window.addEventListener('mousemove', onMouseMove);
  window.addEventListener('mouseup', onMouseUp);
};

// Synchronized scrolling
const onEditorScroll = () => {
  if (editorTextareaRef.value && editorGutterRef.value) {
    editorGutterRef.value.scrollTop = editorTextareaRef.value.scrollTop;
  }
};

const onResponseScroll = () => {
  if (responseContainerRef.value && responseGutterRef.value) {
    responseGutterRef.value.scrollTop = responseContainerRef.value.scrollTop;
  }
};

// Jump from Index list to Query Workbench directly
const navigateToQueryIndex = (indexName: string) => {
  activeTab.value = 'query';
  activeDevToolsTab.value = 'console';
  consoleInput.value = `POST /${indexName}/_search\n{\n  "size": 25,\n  "query": {\n    "match_all": {}\n  },\n  "sort": [\n    { "@timestamp": { "order": "desc", "unmapped_type": "boolean" } }\n  ]\n}`;
  executeOpenSearchQuery();
};

// Query Profiler Runner
const executeProfilerQuery = async () => {
  profilerLoading.value = true;
  const { method, endpoint, body } = parseConsoleQuery(profilerQueryInput.value);
  let finalBody = body;
  if (typeof finalBody === 'object' && finalBody !== null) {
    finalBody = { ...finalBody, profile: true };
  }
  try {
    const res = await axios.post('/api/v1/opensearch/query', {
      method,
      endpoint,
      body: finalBody,
    });
    if (res.data && res.data.success && res.data.data) {
      profilerResult.value = res.data.data.data?.profile || res.data.data.data;
    }
  } catch (err: any) {
    profilerResult.value = { error: err.message };
  } finally {
    profilerLoading.value = false;
  }
};

// Filtered shards for All Shards table (with State Filter support)
const filteredShards = computed(() => {
  let list = shardsList.value;
  if (shardStateFilter.value === 'unassigned') {
    list = unassignedShards.value;
  } else if (shardStateFilter.value === 'inflight') {
    list = inFlightShards.value;
  } else if (shardStateFilter.value === 'started') {
    list = startedShards.value;
  }

  if (!shardSearch.value) return list;
  const q = shardSearch.value.toLowerCase();
  return list.filter(
    s => (s.index || s.name || '').toLowerCase().includes(q) ||
         (s.node || s.nodeName || '').toLowerCase().includes(q) ||
         (s.ip || '').toLowerCase().includes(q) ||
         (s.state || '').toLowerCase().includes(q)
  );
});

// Grouped shards per node (only assigned active shards)
const shardsByNode = computed(() => {
  const map: Record<string, any[]> = {};
  clusterNodes.value.forEach(n => {
    map[n.name] = [];
  });

  shardsList.value.forEach(shard => {
    const nodeName = shard.node || shard.nodeName;
    if (nodeName && map[nodeName] && shard.state !== 'UNASSIGNED' && nodeName !== 'UNASSIGNED') {
      map[nodeName].push(shard);
    }
  });

  return map;
});

// Grouped filtered shards per node (only assigned active shards, filtered by search & state filter)
const filteredShardsByNode = computed(() => {
  const map: Record<string, any[]> = {};
  clusterNodes.value.forEach(n => {
    map[n.name] = [];
  });

  filteredShards.value.forEach(shard => {
    const nodeName = shard.node || shard.nodeName;
    if (nodeName && map[nodeName] && shard.state !== 'UNASSIGNED' && nodeName !== 'UNASSIGNED') {
      map[nodeName].push(shard);
    }
  });

  return map;
});

// Node shard statistics respecting active search and state filter
const getNodeShardCounts = (nodeName: string) => {
  const shards = filteredShardsByNode.value[nodeName] || [];
  let primary = 0;
  let replica = 0;
  for (const s of shards) {
    if (s.prirep === 'p' || s.type === 'Primary') {
      primary++;
    } else if (s.prirep === 'r' || s.type === 'Replica') {
      replica++;
    }
  }
  return { primary, replica, total: shards.length };
};

// Open index detail modal from clicking a shard block
const openShardIndexDetails = (shard: any) => {
  const idxName = shard.index || shard.name;
  if (!idxName) return;
  const found = indicesList.value.find(i => (i.index || i.name) === idxName);
  if (found) {
    selectedIndexModal.value = found;
  } else {
    selectedIndexModal.value = {
      index: idxName,
      health: shard.health || 'green',
      pri: shard.pri || 1,
      rep: shard.rep || 0,
    };
  }
};

// Shards belonging to the modal-selected index
const selectedIndexShards = computed(() => {
  if (!selectedIndexModal.value) return [];
  const idxName = selectedIndexModal.value.index || selectedIndexModal.value.name;
  return shardsList.value.filter(s => s.index === idxName);
});

function formatBytes(bytes: number, decimals = 1) {
  if (!+bytes) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

function formatNumber(num: any) {
  const n = parseInt(num) || 0;
  return new Intl.NumberFormat('en-US').format(n);
}

// Fetch all cluster telemetry from actual backend
const fetchClusterData = async () => {
  isRefreshing.value = true;
  error.value = '';
  try {
    const [healthRes, nodesStatsRes, nodesInfoRes, indicesRes, shardsRes, logsRes, recoveryRes] = await Promise.allSettled([
      axios.get('/api/v1/opensearch/health'),
      axios.get('/api/v1/opensearch/nodes'),
      axios.get('/api/v1/opensearch/nodes/info'),
      axios.get('/api/v1/opensearch/indices'),
      axios.get('/api/v1/opensearch/shards'),
      axios.get('/api/v1/logs'),
      axios.get('/api/v1/opensearch/recovery'),
    ]);

    if (healthRes.status === 'fulfilled' && healthRes.value.data.success) {
      clusterHealth.value = healthRes.value.data.data;
    } else {
      clusterHealth.value = null;
    }

    if (indicesRes.status === 'fulfilled' && indicesRes.value.data.success && Array.isArray(indicesRes.value.data.data)) {
      indicesList.value = indicesRes.value.data.data;
    } else {
      indicesList.value = [];
    }

    if (shardsRes.status === 'fulfilled' && shardsRes.value.data.success && Array.isArray(shardsRes.value.data.data)) {
      shardsList.value = shardsRes.value.data.data;
    } else {
      shardsList.value = [];
    }

    if (recoveryRes.status === 'fulfilled' && recoveryRes.value.data.success && Array.isArray(recoveryRes.value.data.data)) {
      recoveryList.value = recoveryRes.value.data.data;
    } else {
      recoveryList.value = [];
    }

    if (nodesStatsRes.status === 'fulfilled' && nodesStatsRes.value.data.success) {
      parseNodesData(nodesStatsRes.value.data.data, nodesInfoRes.status === 'fulfilled' ? nodesInfoRes.value.data.data : null);
    } else {
      nodesList.value = [];
    }

    if (logsRes.status === 'fulfilled' && logsRes.value.data.success && Array.isArray(logsRes.value.data.data)) {
      logsList.value = logsRes.value.data.data.filter((l: any) => l.source === 'OpenSearch' || l.category === 'OpenSearch' || (l.message && l.message.includes('OpenSearch')));
    }
  } catch (err: any) {
    console.error('Error fetching OpenSearch data:', err);
  } finally {
    isRefreshing.value = false;
    countdown.value = refreshIntervalSec.value > 0 ? refreshIntervalSec.value : 0;
  }
};

const parseNodesData = (stats: any, info: any) => {
  if (!stats?.nodes) {
    nodesList.value = [];
    return;
  }
  const nodesArr: any[] = [];
  const nodesMap = stats.nodes;
  const infoMap = info?.nodes || {};

  Object.keys(nodesMap).forEach(nodeId => {
    const n = nodesMap[nodeId];
    const inf = infoMap[nodeId] || {};
    const os = n.os || {};
    const jvm = n.jvm || {};
    const fs = n.fs?.total || {};

    const jvmHeapUsed = jvm.mem?.heap_used_in_bytes || 0;
    const jvmHeapMax = jvm.mem?.heap_max_in_bytes || 1;
    const heapPct = Math.round((jvmHeapUsed / jvmHeapMax) * 100);

    const diskTotal = fs.total_in_bytes || 1;
    const diskAvailable = fs.available_in_bytes || 0;
    const diskUsed = diskTotal - diskAvailable;
    const diskPct = ((diskUsed / diskTotal) * 100).toFixed(2);

    const load1 = os.cpu?.load_average?.['1m'] ?? '0.00';
    const load5 = os.cpu?.load_average?.['5m'] ?? '0.00';
    const load15 = os.cpu?.load_average?.['15m'] ?? '0.00';

    const nodeShards = shardsList.value.filter(s => (s.node === n.name || s.nodeName === n.name));
    const priCount = nodeShards.filter(s => s.prirep === 'p' || s.type === 'Primary').length;
    const repCount = nodeShards.filter(s => s.prirep === 'r' || s.type === 'Replica').length;

    nodesArr.push({
      name: n.name || nodeId,
      ip: n.ip || inf.ip || '-',
      cpu: `${os.cpu?.percent ?? 0}%`,
      load: `${load1} / ${load5} / ${load15}`,
      heapPercent: `${heapPct}%`,
      uptime: formatUptime(jvm.uptime_in_millis || 0),
      ram: `${os.mem?.used_percent ?? 0}%`,
      jvmHeap: `${formatBytes(jvmHeapUsed)} / ${formatBytes(jvmHeapMax)}`,
      diskPercent: `${diskPct}%`,
      diskUsage: `${formatBytes(diskUsed)} / ${formatBytes(diskTotal)}`,
      primaryShards: priCount,
      replicaShards: repCount,
    });
  });

  nodesList.value = nodesArr;
};

function formatUptime(ms: number) {
  if (!ms || ms <= 0) return '0h';
  const days = Math.floor(ms / (1000 * 60 * 60 * 24));
  const hours = Math.floor((ms % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  return `${days}d ${hours}h`;
}

// Hover Tooltip Handlers for Shard Blocks (Matching Screenshot)
const showTooltip = (event: MouseEvent, shard: any) => {
  hoveredShard.value = shard;
  const target = event.currentTarget as HTMLElement;
  const rect = target.getBoundingClientRect();
  tooltipPosition.value = {
    x: Math.min(Math.max(rect.left - 100, 10), window.innerWidth - 340),
    y: rect.bottom + 10,
  };
};

const hideTooltip = () => {
  hoveredShard.value = null;
};

// Check if a shard belongs to the currently hovered index (Auto-detect Primary & Replica pairing)
const isShardHighlighted = (shard: any) => {
  if (!hoveredShard.value) return false;
  const targetIndex = hoveredShard.value.index || hoveredShard.value.name;
  const currentIndex = shard.index || shard.name;
  return Boolean(targetIndex && currentIndex && targetIndex === currentIndex);
};

// Check if a shard should be dimmed because a different index is hovered
const isShardDimmed = (shard: any) => {
  if (!hoveredShard.value) return false;
  const targetIndex = hoveredShard.value.index || hoveredShard.value.name;
  const currentIndex = shard.index || shard.name;
  return Boolean(targetIndex && currentIndex && targetIndex !== currentIndex);
};

// Configuration Methods
const fetchActiveConfig = async () => {
  try {
    const res = await axios.get('/api/v1/opensearch/config');
    if (res.data.success && res.data.data) {
      const c = res.data.data;
      configForm.value = {
        id: c.id || '',
        name: c.name || '',
        host: c.host || '',
        port: c.port || 9200,
        username: c.username || '',
        password: '',
        useSsl: c.useSsl || false,
        verifySsl: c.verifySsl || false,
        isActive: c.isActive !== false,
      };
    }
  } catch (err) {
    console.error('Failed to load OpenSearch config:', err);
  }
};

const handleTestConnection = async () => {
  isTestingConnection.value = true;
  testResult.value = 'Testing connection...';
  testSuccess.value = false;
  try {
    const res = await axios.post('/api/v1/opensearch/test', {
      id: configForm.value.id || undefined,
      host: configForm.value.host,
      port: configForm.value.port,
      username: configForm.value.username,
      password: configForm.value.password,
      useSsl: configForm.value.useSsl,
      verifySsl: configForm.value.verifySsl,
    });
    if (res.data.success) {
      testSuccess.value = true;
      testResult.value = `Success: Cluster "${res.data.data?.cluster_name || 'OpenSearch'}" status is ${res.data.data?.status || 'green'}.`;
    }
  } catch (err: any) {
    testSuccess.value = false;
    testResult.value = `Failed: ${err.response?.data?.error || err.message}`;
  } finally {
    isTestingConnection.value = false;
  }
};

const handleSaveConfig = async () => {
  isSavingConfig.value = true;
  try {
    const res = await axios.post('/api/v1/opensearch/config', configForm.value);
    if (res.data.success) {
      alert('OpenSearch configuration saved successfully!');
      activeTab.value = 'overview';
      fetchClusterData();
    }
  } catch (err: any) {
    alert(`Failed to save config: ${err.response?.data?.error || err.message}`);
  } finally {
    isSavingConfig.value = false;
  }
};

// Timer Management
const startTimer = () => {
  if (timer.value) clearInterval(timer.value);
  if (refreshIntervalSec.value <= 0) return;

  timer.value = setInterval(() => {
    if (countdown.value > 1) {
      countdown.value--;
    } else {
      fetchClusterData();
    }
  }, 1000);
};

const setRefreshInterval = (sec: number) => {
  refreshIntervalSec.value = sec;
  countdown.value = sec;
  localStorage.setItem('opensearch_refresh_sec', sec.toString());
  isSettingsModalOpen.value = false;
  startTimer();
};

const handleBackToPortal = () => {
  if (window.opener) {
    window.close();
  } else {
    router.push('/');
  }
};

onMounted(() => {
  if (route.query.tab && typeof route.query.tab === 'string') {
    activeTab.value = route.query.tab as any;
  }
  if (route.query.refresh) {
    const r = parseInt(route.query.refresh as string);
    if (!isNaN(r) && r > 0) {
      refreshIntervalSec.value = r;
      countdown.value = r;
    }
  }
  if (route.query.theme === 'dark') {
    document.documentElement.classList.add('dark');
  } else if (route.query.theme === 'light') {
    document.documentElement.classList.remove('dark');
  }

  const savedSec = localStorage.getItem('opensearch_refresh_sec');
  if (savedSec !== null && !route.query.refresh) {
    refreshIntervalSec.value = parseInt(savedSec);
    countdown.value = refreshIntervalSec.value;
  }

  fetchActiveConfig();
  fetchClusterData();
  startTimer();
});

onUnmounted(() => {
  if (timer.value) clearInterval(timer.value);
});
</script>

<template>
  <div class="min-h-screen w-full bg-slate-50 dark:bg-[#14161b] text-slate-800 dark:text-slate-200 font-sans flex flex-col selection:bg-brand-500/30">
    <!-- Top Header Bar (Sticky at top) -->
    <header
      v-if="showHeader"
      class="h-12 bg-white dark:bg-[#1b1e26] border-b border-slate-200 dark:border-slate-800 px-4 flex items-center justify-between shrink-0 sticky top-0 z-20 backdrop-blur-md"
    >
      <!-- Left: Title and Status -->
      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2">
          <Database class="w-4 h-4 text-slate-400" />
          <h1 class="text-xs font-semibold text-slate-900 dark:text-white tracking-wide">
            OpenSearch Cluster Monitor
            <span v-if="isEmbedMode && configForm.name" class="text-slate-400 font-normal"> · {{ configForm.name }}</span>
          </h1>
        </div>

        <!-- Connection Status Pill -->
        <div
          v-if="clusterHealth"
          class="flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-mono border"
          :class="clusterHealth.status === 'green' ? 'bg-emerald-50 dark:bg-emerald-500/10 border-emerald-300 dark:border-emerald-500/30 text-emerald-700 dark:text-emerald-400' : clusterHealth.status === 'yellow' ? 'bg-amber-50 dark:bg-amber-500/10 border-amber-300 dark:border-amber-500/30 text-amber-700 dark:text-amber-400' : 'bg-red-50 dark:bg-red-500/10 border-red-300 dark:border-red-500/30 text-red-700 dark:text-red-400'"
        >
          <span class="w-1.5 h-1.5 rounded-full" :class="clusterHealth.status === 'green' ? 'bg-emerald-500 dark:bg-emerald-400' : clusterHealth.status === 'yellow' ? 'bg-amber-500 dark:bg-amber-400' : 'bg-red-500 dark:bg-red-400'"></span>
          <span class="uppercase font-bold">{{ clusterHealth.status }}</span>
        </div>
      </div>

      <!-- Right Actions: Refresh, Countdown, Gear, ThemeToggle, Share, Back to Portal -->
      <div class="flex items-center gap-2.5">
        <!-- Countdown indicator -->
        <div v-if="refreshIntervalSec > 0" class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400 font-mono">
          <Clock class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
          <span>{{ countdown }}s</span>
        </div>

        <!-- Refresh Button -->
        <button
          @click="fetchClusterData"
          class="p-1.5 rounded-lg text-slate-500 hover:text-slate-900 hover:bg-slate-100 dark:text-slate-400 dark:hover:text-white dark:hover:bg-slate-800 transition"
          title="Refresh Data Now"
        >
          <RotateCw :class="['w-4 h-4', isRefreshing ? 'animate-spin text-slate-300' : '']" />
        </button>

        <!-- Settings Gear Modal Button (Only if not embed) -->
        <button
          v-if="!isEmbedMode"
          @click="isSettingsModalOpen = true"
          class="p-1.5 rounded-lg text-slate-500 hover:text-slate-900 hover:bg-slate-100 dark:text-slate-400 dark:hover:text-white dark:hover:bg-slate-800 transition"
          title="Auto-Refresh Settings"
        >
          <Settings class="w-4 h-4" />
        </button>

        <!-- Theme Toggle Button -->
        <ThemeToggle variant="compact" />

        <!-- Share / Embed Button (Top Header) -->
        <button
          v-if="!isEmbedMode"
          @click="isShareModalOpen = true"
          class="flex items-center gap-1.5 px-3 py-1 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#242833] border border-slate-300 dark:border-slate-700/60 text-xs text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white transition font-medium"
          title="Share & Embed Overview URL"
        >
          <Share2 class="w-3.5 h-3.5 text-slate-400" />
          <span class="hidden sm:inline">Share</span>
        </button>

        <!-- Back to Portal Button (or Open Full Monitor if in embed) -->
        <button
          v-if="!isEmbedMode"
          @click="handleBackToPortal"
          class="flex items-center gap-1.5 px-3 py-1 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#242833] border border-slate-300 dark:border-slate-700/60 text-xs text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white transition font-medium"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Back to Portal</span>
        </button>
        <a
          v-else
          href="/opensearch-cluster"
          target="_blank"
          class="p-1.5 rounded-lg text-slate-500 hover:text-slate-900 hover:bg-slate-100 dark:text-slate-400 dark:hover:text-white dark:hover:bg-slate-800 transition flex items-center gap-1 text-xs"
          title="Open Full Cluster Monitor in Portal"
        >
          <ExternalLink class="w-3.5 h-3.5" />
        </a>
      </div>
    </header>

    <!-- Navigation Tabs Bar (Sticky below header, hidden in embed mode) -->
    <nav
      v-if="!isEmbedMode"
      class="bg-white/95 dark:bg-[#1b1e26]/95 border-b border-slate-200 dark:border-slate-800/80 px-4 sm:px-6 flex items-center gap-6 sm:gap-8 text-xs shrink-0 sticky top-12 z-10 backdrop-blur-md overflow-x-auto overflow-y-hidden scrollbar-none whitespace-nowrap"
    >
      <button
        v-for="tab in [
          { id: 'overview', label: 'Overview' },
          { id: 'nodes', label: 'Nodes' },
          { id: 'indices', label: 'Indices' },
          { id: 'shards', label: 'Shards & Allocation' },
          { id: 'query', label: 'Dev Tools / Query' },
          { id: 'connection', label: 'Connection' },
          { id: 'logs', label: 'Logs' }
        ]"
        :key="tab.id"
        type="button"
        @click="activeTab = tab.id as any"
        :class="[
          'py-3 transition border-b-2 bg-transparent outline-none focus:outline-none cursor-pointer shrink-0',
          activeTab === tab.id
            ? 'border-blue-600 text-blue-600 dark:border-brand-500 dark:text-brand-400 font-bold'
            : 'border-transparent text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 font-medium'
        ]"
      >
        <span>{{ tab.label }}</span>
      </button>
    </nav>

    <!-- Main Content Container (Uses natural browser window scrolling) -->
    <main
      class="flex-1 p-3.5 sm:p-5 md:p-6 space-y-6 w-full"
      :class="['query', 'shards', 'indices', 'logs'].includes(activeTab) ? 'max-w-none' : 'max-w-[1600px] mx-auto'"
    >
      
      <!-- ================================================================= -->
      <!-- TAB 1: OVERVIEW -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'overview'" class="space-y-6">
        <!-- Not Connected Notice -->
        <div v-if="!clusterHealth" class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl text-center space-y-3">
          <div class="w-10 h-10 rounded-full bg-amber-500/10 border border-amber-500/20 text-amber-400 flex items-center justify-center mx-auto">
            <AlertCircle class="w-5 h-5" />
          </div>
          <h3 class="text-sm font-bold text-white">No OpenSearch Cluster Connected</h3>
          <p class="text-xs text-slate-400 max-w-md mx-auto">
            Configure your OpenSearch endpoint host and credentials in the Connection tab to start monitoring cluster telemetry.
          </p>
          <button
            @click="activeTab = 'connection'"
            class="px-4 py-2 bg-brand-500 hover:bg-brand-600 text-white text-xs font-medium rounded-lg transition"
          >
            Configure Connection
          </button>
        </div>

        <!-- Metric Cards Row (4 cards) -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
          <!-- Card 1: Cluster Health -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl relative overflow-hidden flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Cluster Health</span>
            <div class="text-2xl font-black tracking-tight uppercase" :class="clusterHealth?.status === 'green' ? 'text-emerald-400' : clusterHealth?.status === 'yellow' ? 'text-amber-400' : clusterHealth?.status === 'red' ? 'text-red-400' : 'text-slate-500'">
              {{ clusterHealth?.status || 'NOT CONNECTED' }}
            </div>
            <div
              class="w-full h-1 rounded-full shadow-sm"
              :class="clusterHealth?.status === 'green' ? 'bg-emerald-500' : clusterHealth?.status === 'yellow' ? 'bg-amber-500' : clusterHealth?.status === 'red' ? 'bg-red-500' : 'bg-slate-700'"
            ></div>
          </div>

          <!-- Card 2: Total Indices -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Total Indices</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ formatNumber(totalIndicesCount) }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Managed across cluster</span>
          </div>

          <!-- Card 3: Total Documents -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Total Documents</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ formatNumber(totalDocumentsCount) }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Indexed searchable docs</span>
          </div>

          <!-- Card 4: Store Size -->
          <div class="p-5 bg-[#1b1e26] border border-slate-800/80 rounded-xl flex flex-col justify-between h-32 shadow-lg">
            <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Store Size</span>
            <div class="text-3xl font-bold text-white tracking-tight">
              {{ totalStoreSizeBytes }}
            </div>
            <span class="text-[11px] text-slate-500 font-mono">Total allocated storage</span>
          </div>
        </div>

        <!-- Cluster Health & Activity Alert Banner -->
        <div
          v-if="clusterHealth?.status !== 'green' || unassignedShards.length > 0 || inFlightShards.length > 0"
          class="p-4 rounded-xl border flex flex-col sm:flex-row sm:items-center justify-between gap-3 shadow-md"
          :class="clusterHealth?.status === 'red' ? 'bg-red-950/20 border-red-500/30 text-red-300' : 'bg-amber-950/20 border-amber-500/30 text-amber-300'"
        >
          <div class="flex items-start sm:items-center gap-3">
            <div class="p-2 rounded-lg" :class="clusterHealth?.status === 'red' ? 'bg-red-500/10 text-red-400' : 'bg-amber-500/10 text-amber-400'">
              <AlertTriangle class="w-5 h-5" />
            </div>
            <div>
              <div class="font-bold text-xs uppercase tracking-wide flex items-center gap-2">
                <span>Cluster Health Attention: {{ clusterHealth?.status?.toUpperCase() || 'YELLOW' }}</span>
                <span v-if="inFlightShards.length > 0 || clusterHealth?.initializing_shards > 0" class="px-2 py-0.5 rounded-full bg-blue-500/20 text-blue-400 text-[10px] font-mono animate-pulse">
                  Shard Replication / Relocation In Progress
                </span>
              </div>
              <p class="text-[11px] opacity-90 mt-0.5">
                <span v-if="unassignedShards.length > 0 || clusterHealth?.unassigned_shards > 0">
                  There are <strong>{{ unassignedShards.length || clusterHealth?.unassigned_shards }} unassigned shards</strong> not allocated to any node.
                </span>
                <span v-if="inFlightShards.length > 0 || clusterHealth?.initializing_shards > 0">
                  Currently replicating / synchronizing <strong>{{ inFlightShards.length || clusterHealth?.initializing_shards }} shards</strong> across nodes.
                </span>
                <span v-if="unhealthyIndices.length > 0">
                  ({{ unhealthyIndices.length }} indices affected).
                </span>
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2 self-end sm:self-auto shrink-0">
            <button
              v-if="unassignedShards.length > 0 || clusterHealth?.unassigned_shards > 0"
              @click="activeTab = 'shards'; shardStateFilter = 'unassigned'"
              class="px-3 py-1.5 rounded-lg bg-amber-500 hover:bg-amber-600 text-slate-950 font-bold text-xs transition cursor-pointer shadow-sm"
            >
              View Unassigned Shards ({{ unassignedShards.length || clusterHealth?.unassigned_shards }})
            </button>
            <button
              v-if="unhealthyIndices.length > 0"
              @click="activeTab = 'indices'; indexHealthFilter = 'unhealthy'"
              class="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-white font-semibold text-xs border border-slate-700 transition cursor-pointer"
            >
              View Problem Indices ({{ unhealthyIndices.length }})
            </button>
          </div>
        </div>

        <!-- Middle Section: Index & Shards + Node Stats -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Left: Index & Shards Breakdown -->
          <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6 shadow-xl">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold text-white tracking-wide">Indices Allocation Summary</h3>
            </div>
            <div class="grid grid-cols-2 sm:grid-cols-3 gap-y-6 text-xs font-sans">
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Primary Indices</p>
                <p class="text-xl font-bold text-emerald-400 font-mono">{{ clusterHealth?.active_primary_shards ?? 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Replica Indices</p>
                <p class="text-xl font-bold text-sky-400 font-mono">{{ clusterHealth ? (clusterHealth.active_shards - clusterHealth.active_primary_shards) : 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Total Indices</p>
                <p class="text-xl font-bold text-white font-mono">{{ clusterHealth?.active_shards ?? 0 }}</p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Unassigned Indices</p>
                <p class="text-xl font-bold font-mono" :class="(clusterHealth?.unassigned_shards > 0 || unassignedShards.length > 0) ? 'text-amber-400 font-black' : 'text-slate-400'">
                  {{ clusterHealth?.unassigned_shards ?? unassignedShards.length }}
                </p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Initializing (Sync)</p>
                <p class="text-xl font-bold font-mono flex items-center gap-1.5" :class="(clusterHealth?.initializing_shards > 0 || inFlightShards.some(s => s.state === 'INITIALIZING')) ? 'text-amber-400 animate-pulse' : 'text-slate-400'">
                  <span>{{ clusterHealth?.initializing_shards ?? inFlightShards.filter(s => s.state === 'INITIALIZING').length }}</span>
                  <RotateCw v-if="clusterHealth?.initializing_shards > 0 || inFlightShards.some(s => s.state === 'INITIALIZING')" class="w-3.5 h-3.5 animate-spin text-amber-400 inline" />
                </p>
              </div>
              <div>
                <p class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Relocating (Moving)</p>
                <p class="text-xl font-bold font-mono flex items-center gap-1.5" :class="(clusterHealth?.relocating_shards > 0 || inFlightShards.some(s => s.state === 'RELOCATING')) ? 'text-purple-400 animate-pulse' : 'text-slate-400'">
                  <span>{{ clusterHealth?.relocating_shards ?? inFlightShards.filter(s => s.state === 'RELOCATING').length }}</span>
                  <Zap v-if="clusterHealth?.relocating_shards > 0 || inFlightShards.some(s => s.state === 'RELOCATING')" class="w-3.5 h-3.5 text-purple-400 inline" />
                </p>
              </div>
            </div>
          </div>

          <!-- Right: Cluster Nodes Status -->
          <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-4 shadow-xl">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold text-white tracking-wide">Cluster Nodes ({{ clusterNodes.length }})</h3>
              <span class="text-xs font-mono text-slate-400">{{ clusterHealth?.cluster_name || 'OpenSearch' }}</span>
            </div>

            <div v-if="clusterNodes.length > 0" class="space-y-2">
              <div
                v-for="node in clusterNodes"
                :key="node.name"
                class="p-3 bg-[#14161b] border border-slate-800 rounded-lg flex items-center justify-between text-xs"
              >
                <div class="flex items-center gap-2.5">
                  <Server class="w-4 h-4 text-brand-400" />
                  <div>
                    <p class="font-bold text-slate-200">{{ node.name }}</p>
                    <p class="text-[10px] text-slate-500 font-mono">{{ node.ip }}</p>
                  </div>
                </div>
                <div class="flex items-center gap-3 font-mono text-[11px]">
                  <span class="text-emerald-400 font-semibold">P: {{ node.primaryCount }}</span>
                  <span class="text-sky-400 font-semibold">R: {{ node.replicaCount }}</span>
                </div>
              </div>
            </div>
            <p v-else class="text-xs text-slate-500 italic py-4 text-center">No nodes registered.</p>
          </div>
        </div>

        <!-- ============================================================= -->
        <!-- SHARD ALLOCATION BY NODE (MATCHING USER SCREENSHOT) -->
        <!-- ============================================================= -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl p-6 space-y-6 shadow-xl">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold text-white tracking-wide uppercase">Shard Allocation by Node</h3>
            <!-- Mini Legend -->
            <div class="flex items-center gap-4 text-xs font-mono">
              <div class="flex items-center gap-1.5">
                <span class="px-1.5 py-0.5 rounded bg-[#0f3d28] border border-[#1c6b47] flex items-center justify-center text-[9px] font-bold text-[#4ade80]">P#</span>
                <span class="text-slate-400">Primary (e.g. P0, P1)</span>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="px-1.5 py-0.5 rounded bg-[#132c4a] border border-[#1e4976] flex items-center justify-center text-[9px] font-bold text-[#60a5fa]">R#</span>
                <span class="text-slate-400">Replica (e.g. R0, R1)</span>
              </div>
            </div>
          </div>

          <div v-if="clusterNodes.length === 0" class="text-center py-6 text-slate-500 text-xs">
            No shard allocations discovered yet.
          </div>

          <!-- Node Rows with Matrix of Shard Tiles -->
          <div v-for="node in clusterNodes" :key="node.name" class="space-y-2 border-b border-slate-800/80 pb-6 last:border-b-0 last:pb-0">
            <div class="flex items-center justify-between text-xs">
              <span class="font-bold text-white text-xs tracking-wide">{{ node.name }}</span>
              <div class="flex items-center gap-4 text-slate-400 text-[11px] font-mono">
                <span class="text-emerald-400 font-medium">Primary: {{ node.primaryCount }}</span>
                <span class="text-sky-400 font-medium">Replica: {{ node.replicaCount }}</span>
              </div>
            </div>

            <!-- Visual Shard Blocks Matrix -->
            <div class="flex flex-wrap gap-1.5 p-3.5 bg-[#13161c] rounded-xl border border-slate-800/80 min-h-[50px] items-center">
              <div
                v-for="(shard, sIdx) in (shardsByNode[node.name] || [])"
                :key="sIdx"
                @mouseenter="showTooltip($event, shard)"
                @mouseleave="hideTooltip"
                :title="`${shard.index} [Shard #${shard.shard}] - ${shard.prirep === 'p' || shard.type === 'Primary' ? 'Primary' : 'Replica'} (${shard.store || shard['store.size'] || 'N/A'})`"
                :class="[
                  'min-w-[26px] h-6 px-1 rounded flex items-center justify-center font-bold text-[10px] font-mono cursor-pointer transition-all duration-150 transform select-none',
                  // 1. Highlighted state: ALL primary and replica shards of the hovered index are highlighted with glowing white border
                  isShardHighlighted(shard)
                    ? (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#10b981] text-white border-2 border-white ring-4 ring-emerald-500/40 scale-125 z-30 shadow-2xl font-black'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#3b82f6] text-white border-2 border-white ring-4 ring-blue-500/40 scale-125 z-30 shadow-2xl font-black'
                        : 'bg-red-500 text-white border-2 border-white ring-4 ring-red-500/40 scale-125 z-30 shadow-2xl font-black')
                    // 2. Dimmed state: other unrelated indices are dimmed
                    : isShardDimmed(shard)
                    ? (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#0f3d28]/25 border border-[#1c6b47]/20 text-[#4ade80]/30 opacity-20 scale-90'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#132c4a]/25 border border-[#1e4976]/20 text-[#60a5fa]/30 opacity-20 scale-90'
                        : 'bg-[#3b1219]/25 border border-[#822735]/20 text-[#f87171]/30 opacity-20 scale-90')
                    // 3. Normal idle state
                    : (shard.prirep === 'p' || shard.type === 'Primary'
                        ? 'bg-[#0f3d28] border border-[#1c6b47] text-[#4ade80] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md'
                        : shard.prirep === 'r' || shard.type === 'Replica'
                        ? 'bg-[#132c4a] border border-[#1e4976] text-[#60a5fa] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md'
                        : 'bg-[#3b1219] border border-[#822735] text-[#f87171] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md')
                ]"
              >
                {{ getShardBadgeLabel(shard) }}
              </div>
              <span v-if="(shardsByNode[node.name] || []).length === 0" class="text-[11px] text-slate-600 italic">
                No active shards on this node.
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 2: NODES -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'nodes'" class="space-y-4">
        <div v-if="nodesList.length === 0" class="p-8 bg-[#1b1e26] border border-slate-800/80 rounded-xl text-center space-y-2">
          <p class="text-xs text-slate-400">No node telemetry discovered. Ensure OpenSearch cluster is reachable.</p>
        </div>

        <div
          v-for="node in nodesList"
          :key="node.name"
          class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6 shadow-xl"
        >
          <!-- Node Header -->
          <div class="flex items-center justify-between border-b border-slate-800 pb-4">
            <div class="flex items-center gap-3">
              <Server class="w-5 h-5 text-brand-400" />
              <h2 class="text-sm font-bold text-white tracking-wide">{{ node.name }}</h2>
            </div>
            <div class="flex items-center gap-2 text-xs font-mono text-slate-400">
              <span>{{ node.ip }}</span>
            </div>
          </div>

          <!-- Node Metrics Grid -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-6 text-xs font-sans">
            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">CPU</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.cpu }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Load (1m/5m/15m)</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.load }}</div>
            </div>

            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">Heap</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.heapPercent }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Uptime</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.uptime }}</div>
            </div>

            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">RAM</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.ram }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">JVM Heap</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.jvmHeap }}</div>
            </div>

            <div>
              <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">Disk</p>
              <p class="text-sm font-bold text-white font-mono">{{ node.diskPercent }}</p>
              <div class="mt-2 text-[10px] font-bold text-slate-400 uppercase tracking-wider">Disk Usage</div>
              <div class="text-xs font-mono text-slate-300 mt-0.5">{{ node.diskUsage }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 3: INDICES -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'indices'" class="space-y-4">
        <!-- Search bar, Health Filter, Sort & Export to CSV -->
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div class="flex flex-wrap items-center gap-3">
            <div class="relative w-64 sm:w-72">
              <Search class="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
              <input
                v-model="indexSearch"
                placeholder="Search indices by name..."
                class="w-full bg-[#1b1e26] border border-slate-800 rounded-lg pl-9 pr-4 py-2 text-xs text-white focus:outline-none focus:border-brand-500 transition"
              />
            </div>

            <!-- Health Quick Filter Buttons -->
            <div class="flex items-center bg-[#14161b] p-0.5 rounded-lg border border-slate-800 text-xs font-semibold">
              <button
                @click="indexHealthFilter = 'all'"
                :class="indexHealthFilter === 'all' ? 'bg-[#4274D9] text-white shadow-sm' : 'text-slate-400 hover:text-white'"
                class="px-3 py-1 rounded-md transition cursor-pointer text-[11px]"
              >
                All ({{ indicesList.length }})
              </button>
              <button
                @click="indexHealthFilter = 'unhealthy'"
                :class="indexHealthFilter === 'unhealthy' ? 'bg-amber-500 text-slate-950 font-bold shadow-sm' : 'text-amber-400 hover:text-amber-300'"
                class="px-3 py-1 rounded-md transition cursor-pointer text-[11px] flex items-center gap-1.5"
              >
                <span>Issues / Unassigned</span>
                <span v-if="unhealthyIndices.length > 0" class="px-1.5 py-0.2 rounded-full bg-amber-950 text-amber-300 font-mono text-[9px]">{{ unhealthyIndices.length }}</span>
              </button>
              <button
                @click="indexHealthFilter = 'green'"
                :class="indexHealthFilter === 'green' ? 'bg-emerald-600 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
                class="px-3 py-1 rounded-md transition cursor-pointer text-[11px]"
              >
                Healthy ({{ greenIndices.length }})
              </button>
            </div>

            <!-- Sort By Selector -->
            <div class="flex items-center gap-1.5 bg-[#14161b] px-2.5 py-1 rounded-lg border border-slate-800 text-xs">
              <ArrowUpDown class="w-3.5 h-3.5 text-slate-400 shrink-0" />
              <select
                v-model="sortSelectValue"
                class="bg-transparent text-slate-300 focus:outline-none cursor-pointer font-sans text-[11px]"
              >
                <option value="default" class="bg-[#1b1e26] text-slate-300">Sort: Default</option>
                <option value="storeSize_desc" class="bg-[#1b1e26] text-slate-300">Store Size: Highest First</option>
                <option value="storeSize_asc" class="bg-[#1b1e26] text-slate-300">Store Size: Lowest First</option>
                <option value="priStoreSize_desc" class="bg-[#1b1e26] text-slate-300">Primary Size: Highest First</option>
                <option value="priStoreSize_asc" class="bg-[#1b1e26] text-slate-300">Primary Size: Lowest First</option>
                <option value="docs_desc" class="bg-[#1b1e26] text-slate-300">Docs Count: Highest First</option>
                <option value="docs_asc" class="bg-[#1b1e26] text-slate-300">Docs Count: Lowest First</option>
                <option value="name_asc" class="bg-[#1b1e26] text-slate-300">Index Name: A - Z</option>
                <option value="name_desc" class="bg-[#1b1e26] text-slate-300">Index Name: Z - A</option>
              </select>
            </div>
          </div>

          <!-- Right side: Export to CSV and Indices Count -->
          <div class="flex items-center gap-3 self-end md:self-auto shrink-0">
            <button
              @click="exportIndicesToCsv"
              :disabled="filteredIndices.length === 0"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#14161b] hover:bg-slate-800 text-slate-300 hover:text-white border border-slate-800 text-[11px] font-semibold transition shadow-sm disabled:opacity-50 cursor-pointer"
              title="Export filtered & sorted indices to CSV"
            >
              <Download class="w-3.5 h-3.5 text-slate-400" />
              <span>Export CSV</span>
            </button>
            <span class="text-xs text-slate-400 font-mono">Showing {{ filteredIndices.length }} of {{ indicesList.length }} indices</span>
          </div>
        </div>

        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                <tr>
                  <th class="py-3 px-6">Health</th>
                  <th class="py-3 px-4">Status</th>
                  <th class="py-3 px-4 cursor-pointer hover:text-white transition select-none group" @click="toggleSort('name')">
                    <div class="flex items-center gap-1">
                      <span>Index</span>
                      <ArrowUp v-if="indexSortKey === 'name' && indexSortOrder === 'asc'" class="w-3 h-3 text-brand-400" />
                      <ArrowDown v-else-if="indexSortKey === 'name' && indexSortOrder === 'desc'" class="w-3 h-3 text-brand-400" />
                      <ArrowUpDown v-else class="w-3 h-3 text-slate-600 group-hover:text-slate-400 opacity-60" />
                    </div>
                  </th>
                  <th class="py-3 px-4">UUID</th>
                  <th class="py-3 px-4">Primary</th>
                  <th class="py-3 px-4">Replica</th>
                  <th class="py-3 px-4 cursor-pointer hover:text-white transition select-none group" @click="toggleSort('docs')">
                    <div class="flex items-center gap-1">
                      <span>Docs Count</span>
                      <ArrowUp v-if="indexSortKey === 'docs' && indexSortOrder === 'asc'" class="w-3 h-3 text-brand-400" />
                      <ArrowDown v-else-if="indexSortKey === 'docs' && indexSortOrder === 'desc'" class="w-3 h-3 text-brand-400" />
                      <ArrowUpDown v-else class="w-3 h-3 text-slate-600 group-hover:text-slate-400 opacity-60" />
                    </div>
                  </th>
                  <th class="py-3 px-4">Docs Deleted</th>
                  <th class="py-3 px-4 cursor-pointer hover:text-white transition select-none group" @click="toggleSort('storeSize')">
                    <div class="flex items-center gap-1">
                      <span>Store Size</span>
                      <ArrowUp v-if="indexSortKey === 'storeSize' && indexSortOrder === 'asc'" class="w-3 h-3 text-brand-400" />
                      <ArrowDown v-else-if="indexSortKey === 'storeSize' && indexSortOrder === 'desc'" class="w-3 h-3 text-brand-400" />
                      <ArrowUpDown v-else class="w-3 h-3 text-slate-600 group-hover:text-slate-400 opacity-60" />
                    </div>
                  </th>
                  <th class="py-3 px-6 cursor-pointer hover:text-white transition select-none group" @click="toggleSort('priStoreSize')">
                    <div class="flex items-center gap-1">
                      <span>Primary Size</span>
                      <ArrowUp v-if="indexSortKey === 'priStoreSize' && indexSortOrder === 'asc'" class="w-3 h-3 text-brand-400" />
                      <ArrowDown v-else-if="indexSortKey === 'priStoreSize' && indexSortOrder === 'desc'" class="w-3 h-3 text-brand-400" />
                      <ArrowUpDown v-else class="w-3 h-3 text-slate-600 group-hover:text-slate-400 opacity-60" />
                    </div>
                  </th>
                  <th class="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody v-if="filteredIndices.length > 0" class="divide-y divide-slate-800/60 font-mono text-[11px]">
                <tr
                  v-for="idx in filteredIndices"
                  :key="idx.index || idx.name"
                  @click="selectedIndexModal = idx"
                  class="hover:bg-slate-800/40 transition text-slate-300 cursor-pointer group"
                >
                  <td class="py-3 px-6 font-sans">
                    <span
                      :class="[
                        'px-2 py-0.5 rounded text-[10px] font-semibold lowercase',
                        idx.health === 'green' ? 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-400' : idx.health === 'yellow' ? 'bg-amber-500/10 border border-amber-500/30 text-amber-400' : 'bg-red-500/10 text-red-400'
                      ]"
                    >
                      {{ idx.health || 'green' }}
                    </span>
                  </td>
                  <td class="py-3 px-4 text-slate-400 lowercase font-sans">{{ idx.status || 'open' }}</td>
                  <td class="py-3 px-4 font-sans font-medium text-slate-200 group-hover:text-brand-400 transition">{{ idx.index || idx.name }}</td>
                  <td class="py-3 px-4 text-slate-500 truncate max-w-[140px]">{{ idx.uuid || '-' }}</td>
                  <td class="py-3 px-4 text-emerald-400 font-bold">{{ idx.pri || idx.primary || 1 }}</td>
                  <td class="py-3 px-4 text-sky-400 font-bold">{{ idx.rep || idx.replica || 0 }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ formatNumber(idx['docs.count'] || idx.docsCount || 0) }}</td>
                  <td class="py-3 px-4 text-slate-500">{{ formatNumber(idx['docs.deleted'] || idx.docsDeleted || 0) }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ idx['store.size'] || idx.storeSize || '-' }}</td>
                  <td class="py-3 px-6 text-slate-300">{{ idx['pri.store.size'] || idx.priStoreSize || '-' }}</td>
                  <td class="py-3 px-4 text-right font-sans" @click.stop>
                    <button
                      @click="navigateToQueryIndex(idx.index || idx.name)"
                      class="px-2.5 py-1 rounded bg-[#14161b] hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700 text-[10px] font-semibold transition cursor-pointer inline-flex items-center gap-1"
                      title="Open in Query Workbench"
                    >
                      <Terminal class="w-3 h-3 text-slate-400" />
                      <span>Query</span>
                    </button>
                  </td>
                </tr>
              </tbody>
              <tbody v-else>
                <tr>
                  <td colspan="11" class="text-center py-8 text-slate-500 text-xs font-sans">
                    No indices found.
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 4: SHARDS & ALLOCATION (MATCHING SCREENSHOT) -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'shards'" class="space-y-6">
        <!-- Legend Bar -->
        <div class="flex flex-wrap items-center gap-4 sm:gap-6 text-xs bg-[#1b1e26] border border-slate-800/80 p-3 px-5 rounded-xl shadow-lg">
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#0f3d28] border border-[#1c6b47] flex items-center justify-center text-[9px] font-bold text-[#4ade80]">P</span>
            <span class="text-slate-300 font-medium">Primary Shard</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#132c4a] border border-[#1e4976] flex items-center justify-center text-[9px] font-bold text-[#60a5fa]">R</span>
            <span class="text-slate-300 font-medium">Replica Shard</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#3b2a12] border border-[#784e1b] flex items-center justify-center text-[9px] font-bold text-amber-300 animate-pulse">I</span>
            <span class="text-slate-300 font-medium">Initializing / Syncing</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#2e153b] border border-[#6b258a] flex items-center justify-center text-[9px] font-bold text-purple-300 animate-pulse">M</span>
            <span class="text-slate-300 font-medium">Relocating / Moving</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-4 h-4 rounded bg-[#3b1219] border border-[#822735] flex items-center justify-center text-[9px] font-bold text-[#f87171]">U</span>
            <span class="text-slate-300 font-medium">Unassigned Shard</span>
          </div>
        </div>

        <!-- Live Shard Replication & Recovery Progress Panel (In-Progress Shard Replication & Relocation) -->
        <div
          v-if="(recoveryList.length > 0 || inFlightShards.length > 0) && (filteredRecoveryList.length > 0 || filteredInFlightShards.length > 0 || !shardSearch)"
          class="bg-[#1b1e26] border border-blue-500/40 rounded-xl p-5 space-y-4 shadow-xl animate-in fade-in duration-200"
        >
          <div class="flex items-center justify-between border-b border-slate-800/80 pb-3">
            <div class="flex items-center gap-2">
              <RotateCw class="w-4 h-4 text-blue-400 animate-spin" />
              <h3 class="text-xs font-bold text-white tracking-wide uppercase flex items-center gap-2">
                <span>In-Progress Shard Replication & Relocation</span>
                <span class="px-2 py-0.5 rounded-full bg-blue-500/20 text-blue-400 text-[10px] font-mono font-bold">
                  {{ filteredRecoveryList.length || filteredInFlightShards.length }} active
                </span>
              </h3>
            </div>
            <span class="text-[11px] text-blue-400 font-mono font-semibold">Live Real-time Sync</span>
          </div>

          <div class="space-y-3">
            <!-- If we have detailed recovery stats from /_cat/recovery -->
            <div
              v-for="(rec, rIdx) in filteredRecoveryList"
              :key="'rec-' + rIdx"
              class="p-3.5 bg-[#13161c] border border-slate-800 rounded-lg space-y-2.5 text-xs font-mono"
            >
              <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-1.5 text-[11px]">
                <div class="flex items-center gap-2">
                  <span class="w-2 h-2 rounded-full bg-blue-400 animate-ping"></span>
                  <span class="font-bold text-white">{{ rec.index }}</span>
                  <span class="text-slate-400">[Shard #{{ rec.shard }}]</span>
                  <span class="px-1.5 py-0.2 rounded bg-blue-500/10 text-blue-400 border border-blue-500/30 text-[10px] uppercase font-bold">{{ rec.stage || rec.type || 'sync' }}</span>
                </div>
                <div class="text-slate-300 flex items-center gap-2 text-xs">
                  <span>{{ rec.source_node || rec.source_host || 'source' }}</span>
                  <span class="text-blue-400 font-bold">➔</span>
                  <span class="font-bold text-white">{{ rec.target_node || rec.target_host || 'target' }}</span>
                </div>
              </div>

              <!-- Progress bar -->
              <div class="space-y-1">
                <div class="flex justify-between text-[10.5px] text-slate-400">
                  <span>Bytes: {{ rec.bytes_recovered || rec['bytes.recovered'] || '-' }} / {{ rec.bytes_total || rec['bytes.total'] || '-' }} ({{ rec.bytes_percent || rec['bytes.percent'] || '0%' }})</span>
                  <span>Files: {{ rec.files_recovered || rec['files.recovered'] || '-' }} / {{ rec.files_total || rec['files.total'] || '-' }} ({{ rec.files_percent || rec['files.percent'] || '0%' }})</span>
                </div>
                <div class="w-full bg-slate-800 rounded-full h-2 overflow-hidden">
                  <div
                    class="bg-blue-500 h-2 rounded-full transition-all duration-300"
                    :style="{ width: rec.bytes_percent || rec['bytes.percent'] || rec.files_percent || '50%' }"
                  ></div>
                </div>
              </div>
            </div>

            <!-- Fallback if recoveryList is empty but inFlightShards has records -->
            <div
              v-if="filteredRecoveryList.length === 0"
              v-for="(shard, fIdx) in filteredInFlightShards"
              :key="'flight-' + fIdx"
              class="p-3 bg-[#13161c] border border-slate-800 rounded-lg flex items-center justify-between text-xs font-mono"
            >
              <div class="flex items-center gap-2">
                <span class="w-2 h-2 rounded-full bg-amber-400 animate-ping"></span>
                <span class="font-bold text-white">{{ shard.index }}</span>
                <span class="text-slate-400">[Shard #{{ shard.shard }}]</span>
                <span
                  class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase"
                  :class="shard.state === 'INITIALIZING' ? 'bg-amber-500/10 text-amber-400 border border-amber-500/30' : 'bg-purple-500/10 text-purple-400 border border-purple-500/30'"
                >
                  {{ shard.state }}
                </span>
              </div>
              <div class="text-slate-300 text-[11px]">
                Target Node: <strong class="text-white">{{ shard.node || 'Allocating...' }}</strong>
              </div>
            </div>
          </div>
        </div>

        <!-- Unassigned Shards Card -->
        <div
          v-if="unassignedShards.length > 0 && (filteredUnassignedShards.length > 0 || !shardSearch)"
          class="bg-[#1b1e26] border border-rose-500/40 rounded-xl p-6 space-y-4 shadow-xl"
        >
          <div class="flex items-center justify-between border-b border-slate-800/80 pb-3">
            <div class="flex items-center gap-2">
              <AlertTriangle class="w-4 h-4 text-rose-400" />
              <h3 class="text-xs font-bold text-rose-400 tracking-wide uppercase">
                Unassigned Shards ({{ filteredUnassignedShards.length }}<span v-if="filteredUnassignedShards.length !== unassignedShards.length" class="text-slate-400 font-normal"> of {{ unassignedShards.length }}</span>)
              </h3>
            </div>
            <span class="text-[11px] text-slate-400 font-mono">Not allocated to any node</span>
          </div>

          <p class="text-xs text-slate-300 font-sans">
            The following shards have not been allocated to an active node (unassigned). Hover over the <code class="bg-rose-950 px-1 py-0.5 rounded text-rose-300 font-bold">U</code> block to see the index and unassigned reason. Click to inspect index.
          </p>

          <!-- Visual Unassigned Shards Matrix -->
          <div class="flex flex-wrap gap-1.5 p-3.5 bg-[#13161c] rounded-xl border border-rose-900/30 min-h-[50px] items-center">
            <div
              v-for="(shard, uIdx) in filteredUnassignedShards"
              :key="'unassigned-' + uIdx"
              @mouseenter="showTooltip($event, shard)"
              @mouseleave="hideTooltip"
              @click="openShardIndexDetails(shard)"
              class="w-6 h-6 rounded flex items-center justify-center font-bold text-[10px] cursor-pointer transition-all duration-150 transform select-none bg-[#3b1219] border border-[#822735] text-[#f87171] hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md"
            >
              U
            </div>
            <span v-if="filteredUnassignedShards.length === 0" class="text-[11px] text-slate-500 italic py-1 px-1">
              No matching unassigned shards found for "{{ shardSearch }}".
            </span>
          </div>
        </div>

        <!-- Shard Allocation by Node Card (Exact visual from screenshot) -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl p-6 space-y-6 shadow-xl">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold text-white tracking-wide uppercase">Shard Allocation by Node</h3>
            <div class="flex items-center gap-4 text-xs font-mono">
              <div class="flex items-center gap-1.5">
                <span class="px-1.5 py-0.5 rounded bg-[#0f3d28] border border-[#1c6b47] flex items-center justify-center text-[9px] font-bold text-[#4ade80]">P#</span>
                <span class="text-slate-400">Primary (e.g. P0, P1)</span>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="px-1.5 py-0.5 rounded bg-[#132c4a] border border-[#1e4976] flex items-center justify-center text-[9px] font-bold text-[#60a5fa]">R#</span>
                <span class="text-slate-400">Replica (e.g. R0, R1)</span>
              </div>
              <span class="text-xs text-slate-400 font-mono">{{ clusterNodes.length }} active nodes</span>
            </div>
          </div>

          <div v-if="clusterNodes.length === 0" class="text-center py-6 text-slate-500 text-xs">
            No shard allocations discovered yet.
          </div>

          <!-- Node Rows with Matrix of Shard Tiles -->
          <div v-for="node in clusterNodes" :key="node.name" class="space-y-2 border-b border-slate-800/80 pb-6 last:border-b-0 last:pb-0">
            <div class="flex items-center justify-between text-xs">
              <span class="font-bold text-white text-xs tracking-wide">{{ node.name }}</span>
              <div class="flex items-center gap-4 text-slate-400 text-[11px] font-mono">
                <span class="text-emerald-400 font-medium">Primary: {{ getNodeShardCounts(node.name).primary }}</span>
                <span class="text-sky-400 font-medium">Replica: {{ getNodeShardCounts(node.name).replica }}</span>
                <span v-if="shardSearch || shardStateFilter !== 'all'" class="text-slate-500 text-[10px] font-sans">
                  ({{ getNodeShardCounts(node.name).total }} shown)
                </span>
              </div>
            </div>

            <!-- Visual Shard Blocks Matrix -->
            <div class="flex flex-wrap gap-1.5 p-3.5 bg-[#13161c] rounded-xl border border-slate-800/80 min-h-[50px] items-center">
              <div
                v-for="(shard, sIdx) in (filteredShardsByNode[node.name] || [])"
                :key="sIdx"
                @mouseenter="showTooltip($event, shard)"
                @mouseleave="hideTooltip"
                @click="openShardIndexDetails(shard)"
                :title="`${shard.index} [Shard #${shard.shard}] - ${shard.prirep === 'p' || shard.type === 'Primary' ? 'Primary' : 'Replica'} (${shard.store || shard['store.size'] || 'N/A'})`"
                :class="[
                  'min-w-[26px] h-6 px-1 rounded flex items-center justify-center font-bold text-[10px] font-mono cursor-pointer transition-all duration-150 transform select-none',
                  // 1. Highlighted state: ALL primary and replica shards of the hovered index are highlighted with glowing white border
                  isShardHighlighted(shard)
                    ? 'border-2 border-white ring-4 scale-125 z-30 shadow-2xl font-black ' + getShardColorClass(shard, true)
                    // 2. Dimmed state: other unrelated indices are dimmed
                    : isShardDimmed(shard)
                    ? 'opacity-20 scale-90 ' + getShardColorClass(shard, false, true)
                    // 3. Normal idle state
                    : 'hover:scale-125 hover:border-2 hover:border-white hover:z-30 shadow-md ' + getShardColorClass(shard)
                ]"
              >
                {{ getShardBadgeLabel(shard) }}
              </div>
              <span v-if="(filteredShardsByNode[node.name] || []).length === 0" class="text-[11px] text-slate-500 italic py-1 px-1">
                {{ shardSearch ? 'No matching shards on this node for "' + shardSearch + '"' : 'No active shards on this node.' }}
              </span>
            </div>
          </div>
        </div>

        <!-- Bottom: All Shards Table -->
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="p-4 px-6 border-b border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div class="flex flex-wrap items-center gap-3">
              <h3 class="text-xs font-bold text-white tracking-wide">All Shards ({{ filteredShards.length }})</h3>

              <!-- State Quick Filters -->
              <div class="flex items-center bg-[#14161b] p-0.5 rounded-lg border border-slate-800 text-[11px] font-semibold">
                <button
                  @click="shardStateFilter = 'all'"
                  :class="shardStateFilter === 'all' ? 'bg-[#4274D9] text-white shadow-sm' : 'text-slate-400 hover:text-white'"
                  class="px-2.5 py-0.5 rounded transition cursor-pointer"
                >
                  All ({{ shardsList.length }})
                </button>
                <button
                  @click="shardStateFilter = 'unassigned'"
                  :class="shardStateFilter === 'unassigned' ? 'bg-rose-600 text-white shadow-sm' : 'text-rose-400 hover:text-rose-300'"
                  class="px-2.5 py-0.5 rounded transition cursor-pointer flex items-center gap-1"
                >
                  <span>Unassigned</span>
                  <span v-if="unassignedShards.length > 0" class="px-1.5 py-0.2 rounded-full bg-rose-950 text-rose-300 text-[9px] font-mono">{{ unassignedShards.length }}</span>
                </button>
                <button
                  @click="shardStateFilter = 'inflight'"
                  :class="shardStateFilter === 'inflight' ? 'bg-amber-500 text-slate-950 font-bold shadow-sm' : 'text-amber-400 hover:text-amber-300'"
                  class="px-2.5 py-0.5 rounded transition cursor-pointer flex items-center gap-1"
                >
                  <span>Replicating / Sync</span>
                  <span v-if="inFlightShards.length > 0" class="px-1.5 py-0.2 rounded-full bg-amber-950 text-amber-300 text-[9px] font-mono">{{ inFlightShards.length }}</span>
                </button>
                <button
                  @click="shardStateFilter = 'started'"
                  :class="shardStateFilter === 'started' ? 'bg-emerald-600 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
                  class="px-2.5 py-0.5 rounded transition cursor-pointer"
                >
                  Started
                </button>
              </div>
            </div>

            <div class="relative w-64">
              <Search class="w-3.5 h-3.5 absolute left-2.5 top-2.5 text-slate-500" />
              <input
                v-model="shardSearch"
                placeholder="Filter shards..."
                class="w-full bg-[#14161b] border border-slate-800 rounded-lg pl-8 pr-8 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition font-mono"
              />
              <button
                v-if="shardSearch"
                @click="shardSearch = ''"
                title="Clear filter"
                class="absolute right-2.5 top-2 text-slate-400 hover:text-white transition cursor-pointer p-0.5"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-800">
                <tr>
                  <th class="py-3 px-6">Index</th>
                  <th class="py-3 px-4">Shard</th>
                  <th class="py-3 px-4">Type</th>
                  <th class="py-3 px-4">State</th>
                  <th class="py-3 px-4">Docs</th>
                  <th class="py-3 px-4">Store</th>
                  <th class="py-3 px-4">IP</th>
                  <th class="py-3 px-6">Node</th>
                </tr>
              </thead>
              <tbody v-if="filteredShards.length > 0" class="divide-y divide-slate-800/60 font-mono text-[11px]">
                <tr
                  v-for="(shard, idx) in filteredShards"
                  :key="idx"
                  class="hover:bg-slate-800/30 transition text-slate-300"
                >
                  <td class="py-3 px-6 font-sans font-medium text-slate-200">{{ shard.index }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ shard.shard || '0' }}</td>
                  <td class="py-3 px-4 font-sans">
                    <span
                      :class="[
                        'px-2 py-0.5 rounded text-[10px] font-semibold',
                        shard.prirep === 'p' || shard.type === 'Primary'
                          ? 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-400'
                          : 'bg-sky-500/10 border border-sky-500/30 text-sky-400'
                      ]"
                    >
                      {{ shard.prirep === 'p' || shard.type === 'Primary' ? 'Primary' : 'Replica' }}
                    </span>
                  </td>
                  <td class="py-3 px-4 font-sans">
                    <span
                      :class="[
                        'px-2 py-0.5 rounded text-[10px] font-semibold uppercase',
                        shard.state === 'INITIALIZING'
                          ? 'bg-amber-500/10 border border-amber-500/30 text-amber-400 animate-pulse'
                          : shard.state === 'RELOCATING'
                          ? 'bg-purple-500/10 border border-purple-500/30 text-purple-400 animate-pulse'
                          : shard.state === 'UNASSIGNED'
                          ? 'bg-rose-500/10 border border-rose-500/30 text-rose-400 font-bold'
                          : 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-400'
                      ]"
                    >
                      {{ shard.state || 'STARTED' }}
                    </span>
                  </td>
                  <td class="py-3 px-4 text-slate-300">{{ formatNumber(shard.docs || 0) }}</td>
                  <td class="py-3 px-4 text-slate-300">{{ shard.store || '-' }}</td>
                  <td class="py-3 px-4 text-slate-400">{{ shard.ip || '-' }}</td>
                  <td class="py-3 px-6 text-slate-300 font-sans">
                    <span v-if="shard.node && shard.node !== 'UNASSIGNED'">{{ shard.node }}</span>
                    <span v-else class="px-1.5 py-0.5 rounded bg-rose-500/10 text-rose-400 border border-rose-500/30 text-[10px] font-bold">UNASSIGNED</span>
                  </td>
                </tr>
              </tbody>
              <tbody v-else>
                <tr>
                  <td colspan="8" class="text-center py-8 text-slate-500 text-xs font-sans">
                    No shard telemetry available for selected filter.
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB: DEV TOOLS / OPENSEARCH CONSOLE WORKBENCH -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'query'" class="space-y-4">
        <!-- Main Console Outer Card -->
        <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl overflow-hidden shadow-lg">
          <!-- Top Navigation Tabs (Console & Query Profiler) -->
          <div class="flex items-center gap-6 px-4 pt-3 border-b border-slate-200 dark:border-[#1f283d] bg-white dark:bg-[#111624]">
            <button
              type="button"
              @click="activeDevToolsTab = 'console'"
              class="pb-2.5 text-sm font-semibold tracking-tight border-b-2 transition cursor-pointer flex items-center gap-1.5"
              :class="activeDevToolsTab === 'console' ? 'border-[#00805a] text-[#00805a] dark:border-[#00a699] dark:text-[#00a699]' : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'"
            >
              Console
            </button>
            <button
              type="button"
              @click="activeDevToolsTab = 'profiler'"
              class="pb-2.5 text-sm font-semibold tracking-tight border-b-2 transition cursor-pointer flex items-center gap-1.5"
              :class="activeDevToolsTab === 'profiler' ? 'border-[#00805a] text-[#00805a] dark:border-[#00a699] dark:text-[#00a699]' : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'"
            >
              Query Profiler
            </button>
          </div>

          <!-- Sub-header Toolbar: History, Settings, Help, Export, Import + Status Badges -->
          <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-2 bg-[#f8fafc] dark:bg-[#131826] border-b border-slate-200 dark:border-[#1f283d] select-none text-xs">
            <!-- Left Toolbar Action Links -->
            <div class="flex items-center gap-4 text-slate-600 dark:text-slate-300">
              <button
                type="button"
                @click="showHistoryModal = true"
                class="hover:text-teal-600 dark:hover:text-teal-400 font-medium cursor-pointer transition flex items-center gap-1"
                title="View recent queries history"
              >
                <span>History</span>
              </button>

              <button
                type="button"
                @click="showSettingsModal = true"
                class="hover:text-teal-600 dark:hover:text-teal-400 font-medium cursor-pointer transition flex items-center gap-1"
                title="Console settings and query presets"
              >
                <span>Settings</span>
              </button>

              <button
                type="button"
                @click="showHelpModal = true"
                class="hover:text-teal-600 dark:hover:text-teal-400 font-medium cursor-pointer transition flex items-center gap-1"
                title="OpenSearch Dev Tools documentation and shortcuts"
              >
                <span>Help</span>
              </button>

              <button
                type="button"
                @click="exportConsoleSession"
                class="hover:text-teal-600 dark:hover:text-teal-400 font-medium cursor-pointer transition flex items-center gap-1"
                title="Export current console session to JSON"
              >
                <span>Export</span>
              </button>

              <button
                type="button"
                @click="showImportModal = true"
                class="hover:text-teal-600 dark:hover:text-teal-400 font-medium cursor-pointer transition flex items-center gap-1"
                title="Import query into console"
              >
                <span>Import</span>
              </button>
            </div>

            <!-- Right Status Badges (matches screenshot: 200 - OK & 651 ms) -->
            <div class="flex items-center gap-2">
              <div v-if="queryLoading" class="flex items-center gap-1.5 px-2.5 py-0.5 rounded bg-teal-500/10 text-teal-600 dark:text-teal-400 border border-teal-500/20 text-xs font-mono font-medium">
                <RotateCw class="w-3.5 h-3.5 animate-spin" />
                <span>Sending request...</span>
              </div>

              <template v-else-if="queryResult">
                <!-- Status code badge -->
                <span
                  class="px-2.5 py-0.5 rounded text-xs font-semibold tracking-wide font-mono shadow-xs transition"
                  :class="queryResult.statusCode >= 200 && queryResult.statusCode < 300 ? 'bg-[#007f5f] text-white' : 'bg-rose-600 text-white'"
                >
                  {{ queryResult.statusCode }} - {{ queryResult.statusText || 'OK' }}
                </span>

                <!-- Latency badge -->
                <span class="px-2.5 py-0.5 rounded bg-slate-200 dark:bg-slate-700/80 text-slate-800 dark:text-slate-200 text-xs font-mono font-medium tracking-tight shadow-xs">
                  {{ queryResult.latencyMs }} ms
                </span>
              </template>
            </div>
          </div>

          <!-- DEV TOOLS CONSOLE TAB -->
          <div v-if="activeDevToolsTab === 'console'">
            <!-- Split Dual Pane Console View -->
            <div
              ref="splitContainerRef"
              class="flex flex-col lg:flex-row relative bg-white dark:bg-[#111624] min-h-[580px] lg:min-h-[620px]"
            >
              <!-- LEFT PANE: Request Console Editor -->
              <div
                class="relative flex flex-row overflow-hidden bg-white dark:bg-[#111624] min-h-[300px] lg:min-h-[620px]"
                :style="{ width: editorWidthPercent + '%' }"
              >
                <!-- Editor Line Numbers Gutter -->
                <div
                  ref="editorGutterRef"
                  class="w-10 sm:w-12 shrink-0 py-3 select-none text-right pr-2 text-xs font-mono leading-[22px] overflow-hidden bg-[#f0f2f5] dark:bg-[#0d1017] border-r border-slate-200 dark:border-[#1e2536] text-slate-400 dark:text-slate-600 space-y-0"
                >
                  <div
                    v-for="lineNum in editorLines"
                    :key="'ed-line-' + lineNum"
                    class="h-[22px] leading-[22px]"
                  >
                    {{ lineNum }}
                  </div>
                </div>

                <!-- Editor Textarea Area -->
                <div class="relative flex-1 flex flex-col h-full overflow-hidden">
                  <textarea
                    ref="editorTextareaRef"
                    v-model="consoleInput"
                    @scroll="onEditorScroll"
                    @keydown.ctrl.enter.prevent="executeOpenSearchQuery"
                    @keydown.meta.enter.prevent="executeOpenSearchQuery"
                    spellcheck="false"
                    class="w-full h-full flex-1 py-3 px-3.5 bg-white dark:bg-[#111624] text-slate-900 dark:text-slate-100 font-mono text-xs leading-[22px] border-none outline-none resize-none selection:bg-teal-500/25 overflow-y-auto whitespace-pre tab-4"
                    placeholder="METHOD endpoint&#10;{&#10;  &quot;query&quot;: { &quot;match_all&quot;: {} }&#10;}"
                  ></textarea>

                  <!-- Floating Action Button (Matches Screenshot: Teal Box with Play ▶ and Wrench 🔧) -->
                  <div class="absolute top-2.5 right-4 z-20 flex items-center gap-1 bg-white/95 dark:bg-[#151b2b]/95 border border-teal-500/40 rounded-md p-0.5 shadow-md backdrop-blur-xs">
                    <!-- Play Button with "Send request" Tooltip -->
                    <div class="relative group">
                      <button
                        type="button"
                        @click="executeOpenSearchQuery"
                        :disabled="queryLoading"
                        class="p-1 hover:bg-teal-50 dark:hover:bg-teal-950/60 text-[#00805a] dark:text-[#00a699] rounded transition cursor-pointer flex items-center justify-center disabled:opacity-50"
                        title="Send request (Ctrl+Enter)"
                      >
                        <RotateCw v-if="queryLoading" class="w-3.5 h-3.5 animate-spin" />
                        <Play v-else class="w-3.5 h-3.5 fill-current" />
                      </button>

                      <!-- Dark Tooltip matching Kibana UI -->
                      <div class="absolute bottom-full mb-2 left-1/2 -translate-x-1/2 hidden group-hover:flex flex-col items-center pointer-events-none z-30">
                        <div class="px-2.5 py-1 bg-slate-900 text-white text-[11px] font-sans font-medium rounded shadow-xl whitespace-nowrap">
                          Send request
                        </div>
                        <div class="w-2 h-1 border-x-4 border-x-transparent border-t-4 border-t-slate-900"></div>
                      </div>
                    </div>

                    <!-- Divider -->
                    <div class="w-px h-3.5 bg-slate-200 dark:bg-slate-700"></div>

                    <!-- Wrench Dropdown Button -->
                    <div class="relative">
                      <button
                        type="button"
                        @click="showWrenchMenu = !showWrenchMenu"
                        class="p-1 hover:bg-slate-100 dark:hover:bg-slate-800 text-[#00805a] dark:text-[#00a699] rounded transition cursor-pointer flex items-center justify-center"
                        title="Action menu"
                      >
                        <Wrench class="w-3.5 h-3.5" />
                      </button>

                      <!-- Wrench Dropdown Popover -->
                      <div
                        v-if="showWrenchMenu"
                        class="absolute right-0 top-full mt-1.5 w-48 bg-white dark:bg-[#151b2b] border border-slate-200 dark:border-[#222c42] rounded-lg shadow-2xl py-1 text-xs z-30 animate-in fade-in zoom-in-95"
                      >
                        <button
                          type="button"
                          @click="beautifyConsoleJson"
                          class="w-full text-left px-3 py-1.5 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/80 flex items-center gap-2 cursor-pointer"
                        >
                          <Sparkles class="w-3.5 h-3.5 text-slate-400" />
                          <span>Auto-indent (Beautify)</span>
                        </button>
                        <button
                          type="button"
                          @click="copyAsCurl"
                          class="w-full text-left px-3 py-1.5 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/80 flex items-center gap-2 cursor-pointer"
                        >
                          <Copy class="w-3.5 h-3.5 text-slate-400" />
                          <span>{{ copiedCurl ? 'cURL Copied!' : 'Copy as cURL' }}</span>
                        </button>
                        <button
                          type="button"
                          @click="showSettingsModal = true; showWrenchMenu = false"
                          class="w-full text-left px-3 py-1.5 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/80 flex items-center gap-2 cursor-pointer"
                        >
                          <FileText class="w-3.5 h-3.5 text-slate-400" />
                          <span>Load Preset Query...</span>
                        </button>
                        <div class="h-px bg-slate-200 dark:bg-slate-800 my-1"></div>
                        <button
                          type="button"
                          @click="consoleInput = ''; showWrenchMenu = false"
                          class="w-full text-left px-3 py-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/30 flex items-center gap-2 cursor-pointer"
                        >
                          <Trash2 class="w-3.5 h-3.5 text-rose-500" />
                          <span>Clear Console</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- CENTER RESIZER SPLITTER (|| Handle) -->
              <div
                @mousedown="startResize"
                @dblclick="editorWidthPercent = 50"
                class="hidden lg:flex w-2 hover:w-3 bg-slate-100 dark:bg-[#0c0f17] hover:bg-teal-500/20 dark:hover:bg-teal-500/30 border-x border-slate-200 dark:border-[#1e2536] cursor-col-resize items-center justify-center select-none text-slate-400 dark:text-slate-600 text-[10px] tracking-tighter transition-all group shrink-0"
                title="Drag to resize panes (Double-click to reset 50/50)"
              >
                <span class="font-bold text-slate-400 dark:text-slate-600 group-hover:text-teal-600 dark:group-hover:text-teal-400">||</span>
              </div>

              <!-- RIGHT PANE: Response Inspector -->
              <div class="flex-1 relative flex flex-row overflow-hidden bg-[#f8fafc] dark:bg-[#0f1420] min-h-[300px] lg:min-h-[620px]">
                <!-- Response Line Numbers Gutter with Fold Arrows ▾ -->
                <div
                  ref="responseGutterRef"
                  class="w-12 sm:w-14 shrink-0 py-3 select-none text-right pr-2 text-xs font-mono leading-[22px] overflow-hidden bg-[#eef1f6] dark:bg-[#0c1018] border-r border-slate-200 dark:border-[#1e2536] text-slate-400 dark:text-slate-500 space-y-0"
                >
                  <div
                    v-for="item in responseLineItems"
                    :key="'resp-line-' + item.lineNum"
                    class="h-[22px] leading-[22px] flex items-center justify-end gap-1"
                  >
                    <span class="text-[11px]">{{ item.lineNum }}</span>
                    <span
                      v-if="item.hasFold"
                      class="text-[9px] text-slate-400 dark:text-slate-500 select-none font-bold"
                    >▾</span>
                    <span v-else class="w-2"></span>
                  </div>
                </div>

                <!-- Formatted Response Viewer Area -->
                <div
                  ref="responseContainerRef"
                  @scroll="onResponseScroll"
                  class="flex-1 py-3 px-3.5 overflow-auto font-mono text-xs leading-[22px] select-text"
                >
                  <!-- Loading State -->
                  <div v-if="queryLoading" class="h-full flex flex-col items-center justify-center p-8 space-y-3 text-slate-400">
                    <RotateCw class="w-6 h-6 animate-spin text-teal-600 dark:text-teal-400" />
                    <span class="text-xs font-mono">Executing query against OpenSearch cluster...</span>
                  </div>

                  <!-- Syntax Highlighted Response matching Kibana/OpenSearch Dev Tools -->
                  <pre
                    v-else-if="queryResult"
                    class="whitespace-pre font-mono leading-[22px] focus:outline-none select-text"
                    v-html="highlightedResponseHtml"
                  ></pre>

                  <!-- Empty State -->
                  <div
                    v-else
                    class="h-full flex flex-col items-center justify-center p-8 text-center space-y-2 text-slate-400"
                  >
                    <Terminal class="w-8 h-8 text-slate-300 dark:text-slate-600" />
                    <p class="text-xs font-semibold text-slate-700 dark:text-slate-300">No Query Executed Yet</p>
                    <p class="text-[11px] text-slate-500 dark:text-slate-400 max-w-sm">
                      Write your REST command on the left and click the Play button ▶ (or press <strong class="text-slate-700 dark:text-slate-300">Ctrl+Enter</strong>) to run.
                    </p>
                  </div>
                </div>

                <!-- Quick Copy Button on Response Corner -->
                <button
                  v-if="queryResult && !queryLoading"
                  type="button"
                  @click="copyResponseJson"
                  class="absolute top-2.5 right-4 px-2.5 py-1 bg-white/90 hover:bg-white dark:bg-[#151b2b]/90 dark:hover:bg-[#151b2b] text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-slate-700 rounded-md text-[11px] font-medium shadow-xs transition cursor-pointer flex items-center gap-1.5 z-20 backdrop-blur-xs"
                  title="Copy full response to clipboard"
                >
                  <Check v-if="copiedQueryResult" class="w-3.5 h-3.5 text-emerald-500" />
                  <Copy v-else class="w-3.5 h-3.5 text-slate-400" />
                  <span>{{ copiedQueryResult ? 'Copied' : 'Copy' }}</span>
                </button>
              </div>
            </div>
          </div>

          <!-- QUERY PROFILER TAB -->
          <div v-else-if="activeDevToolsTab === 'profiler'" class="p-6 space-y-6">
            <div class="border-b border-slate-200 dark:border-[#1f283d] pb-4">
              <h3 class="text-sm font-bold text-slate-900 dark:text-white">OpenSearch Query Profiler</h3>
              <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                Inspect detailed timing breakdown for each component of your search queries across all shards.
              </p>
            </div>

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <!-- Profiler Query Input -->
              <div class="space-y-3">
                <div class="flex items-center justify-between">
                  <label class="text-xs font-semibold text-slate-800 dark:text-slate-200">Query DSL (with profile: true)</label>
                  <button
                    type="button"
                    @click="executeProfilerQuery"
                    :disabled="profilerLoading"
                    class="px-3.5 py-1.5 bg-[#00805a] hover:bg-[#00704e] text-white rounded-lg text-xs font-bold transition cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
                  >
                    <RotateCw v-if="profilerLoading" class="w-3.5 h-3.5 animate-spin" />
                    <Play v-else class="w-3.5 h-3.5 fill-current" />
                    <span>{{ profilerLoading ? 'Profiling...' : 'Profile Query' }}</span>
                  </button>
                </div>

                <textarea
                  v-model="profilerQueryInput"
                  rows="16"
                  spellcheck="false"
                  class="w-full bg-[#f8fafc] dark:bg-[#0f1420] text-slate-900 dark:text-slate-100 font-mono text-xs p-3.5 rounded-lg border border-slate-200 dark:border-slate-800 focus:outline-none focus:border-teal-500 resize-none selection:bg-teal-500/25"
                ></textarea>
              </div>

              <!-- Profiler Results / Breakdown -->
              <div class="space-y-3">
                <span class="text-xs font-semibold text-slate-800 dark:text-slate-200">Profile Timing Breakdown</span>
                <div class="bg-[#f8fafc] dark:bg-[#0f1420] rounded-lg border border-slate-200 dark:border-slate-800 p-4 min-h-[380px] overflow-auto text-xs font-mono">
                  <div v-if="profilerLoading" class="h-64 flex flex-col items-center justify-center space-y-2 text-slate-400">
                    <RotateCw class="w-6 h-6 animate-spin text-teal-600" />
                    <span>Profiling search execution timings...</span>
                  </div>
                  <pre v-else-if="profilerResult" class="whitespace-pre font-mono leading-relaxed select-text">{{ JSON.stringify(profilerResult, null, 2) }}</pre>
                  <div v-else class="h-64 flex flex-col items-center justify-center text-center text-slate-400 space-y-2">
                    <Terminal class="w-8 h-8 text-slate-300 dark:text-slate-600" />
                    <p class="text-xs">Click <strong class="text-slate-700 dark:text-slate-300">Profile Query</strong> to inspect Lucene query stages, shard timings, and collectors.</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- ================================================================= -->
        <!-- MODAL: HISTORY -->
        <!-- ================================================================= -->
        <div
          v-if="showHistoryModal"
          class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
        >
          <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-2xl shadow-2xl p-5 space-y-4">
            <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
              <div class="flex items-center gap-2">
                <History class="w-4 h-4 text-slate-500 dark:text-slate-400" />
                <h3 class="text-sm font-bold text-slate-900 dark:text-white">Recent Query History</h3>
              </div>
              <button
                @click="showHistoryModal = false"
                class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-1 cursor-pointer"
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <div class="max-h-96 overflow-y-auto space-y-2 pr-1">
              <div
                v-for="(hist, hIdx) in queryHistory"
                :key="'hist-modal-' + hIdx"
                @click="consoleInput = hist.body; showHistoryModal = false"
                class="p-3 rounded-lg border border-slate-200 dark:border-slate-800 bg-[#f8fafc] dark:bg-[#141824] hover:border-teal-500/50 cursor-pointer transition text-xs font-mono space-y-1.5 group"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span
                      class="px-2 py-0.5 rounded text-[10px] font-bold uppercase font-mono"
                      :class="{
                        'bg-blue-500/10 text-blue-600 dark:text-blue-400': hist.method === 'GET',
                        'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400': hist.method === 'POST',
                        'bg-amber-500/10 text-amber-600 dark:text-amber-400': hist.method === 'PUT',
                        'bg-rose-500/10 text-rose-600 dark:text-rose-400': hist.method === 'DELETE',
                        'bg-purple-500/10 text-purple-600 dark:text-purple-400': hist.method === 'HEAD',
                      }"
                    >
                      {{ hist.method }}
                    </span>
                    <span class="font-bold text-slate-800 dark:text-slate-200 group-hover:text-teal-600 dark:group-hover:text-teal-400 transition">{{ hist.endpoint }}</span>
                  </div>
                  <div class="flex items-center gap-2 text-slate-400 text-[11px]">
                    <span class="px-1.5 py-0.2 rounded bg-slate-200 dark:bg-slate-800 text-[10px]">{{ hist.status }}</span>
                    <span>{{ hist.timestamp }}</span>
                  </div>
                </div>
                <div class="text-[11px] text-slate-500 dark:text-slate-400 truncate max-w-xl font-mono">
                  {{ hist.body.replace(/\n/g, ' ') }}
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between pt-2 border-t border-slate-200 dark:border-[#1f283d]">
              <button
                @click="queryHistory = []"
                class="text-xs text-rose-600 dark:text-rose-400 hover:underline cursor-pointer"
              >
                Clear All History
              </button>
              <button
                @click="showHistoryModal = false"
                class="px-4 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 rounded-lg text-xs font-semibold cursor-pointer transition"
              >
                Close
              </button>
            </div>
          </div>
        </div>

        <!-- ================================================================= -->
        <!-- MODAL: SETTINGS & PRESETS -->
        <!-- ================================================================= -->
        <div
          v-if="showSettingsModal"
          class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
        >
          <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-xl shadow-2xl p-5 space-y-4">
            <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
              <div class="flex items-center gap-2">
                <Settings class="w-4 h-4 text-slate-500 dark:text-slate-400" />
                <h3 class="text-sm font-bold text-slate-900 dark:text-white">Console Settings & Query Presets</h3>
              </div>
              <button
                @click="showSettingsModal = false"
                class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-1 cursor-pointer"
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <!-- Predefined Query Templates Catalog -->
            <div class="space-y-2">
              <span class="text-xs font-semibold text-slate-800 dark:text-slate-200">Load Built-in Preset Templates</span>
              <div class="max-h-60 overflow-y-auto space-y-1.5 pr-1">
                <div
                  v-for="preset in QUERY_PRESETS"
                  :key="preset.id"
                  @click="loadPresetToConsole(preset)"
                  class="p-2.5 rounded-lg border border-slate-200 dark:border-slate-800 hover:border-teal-500 bg-[#f8fafc] dark:bg-[#141824] cursor-pointer transition flex items-center justify-between group"
                >
                  <div class="space-y-0.5">
                    <div class="flex items-center gap-2">
                      <span class="px-1.5 py-0.2 rounded text-[10px] font-bold uppercase font-mono bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
                        {{ preset.method }}
                      </span>
                      <span class="text-xs font-bold text-slate-800 dark:text-slate-200 group-hover:text-teal-600 dark:group-hover:text-teal-400">{{ preset.name }}</span>
                    </div>
                    <p class="text-[11px] text-slate-400">{{ preset.description }}</p>
                  </div>
                  <span class="text-[11px] text-teal-600 dark:text-teal-400 font-medium group-hover:underline">Load</span>
                </div>
              </div>
            </div>

            <div class="flex items-center justify-end pt-2 border-t border-slate-200 dark:border-[#1f283d]">
              <button
                @click="showSettingsModal = false"
                class="px-4 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 rounded-lg text-xs font-semibold cursor-pointer transition"
              >
                Done
              </button>
            </div>
          </div>
        </div>

        <!-- ================================================================= -->
        <!-- MODAL: HELP -->
        <!-- ================================================================= -->
        <div
          v-if="showHelpModal"
          class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
        >
          <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-xl shadow-2xl p-5 space-y-4">
            <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
              <div class="flex items-center gap-2">
                <HelpCircle class="w-4 h-4 text-slate-500 dark:text-slate-400" />
                <h3 class="text-sm font-bold text-slate-900 dark:text-white">OpenSearch Dev Tools Console Help</h3>
              </div>
              <button
                @click="showHelpModal = false"
                class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-1 cursor-pointer"
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <div class="space-y-3 text-xs text-slate-600 dark:text-slate-300 font-sans leading-relaxed">
              <div class="space-y-1">
                <h4 class="font-bold text-slate-900 dark:text-white">Request Format</h4>
                <p>The console allows you to compose requests just like in Kibana or OpenSearch Dashboards:</p>
                <pre class="bg-slate-100 dark:bg-[#151b2b] p-2.5 rounded font-mono text-[11px] text-slate-800 dark:text-slate-200">METHOD endpoint
{
  ...optional JSON or NDJSON body...
}</pre>
              </div>

              <div class="space-y-1">
                <h4 class="font-bold text-slate-900 dark:text-white">Keyboard Shortcuts</h4>
                <div class="grid grid-cols-2 gap-2 text-[11px] font-mono">
                  <div class="bg-slate-100 dark:bg-[#151b2b] p-2 rounded flex justify-between">
                    <span>Ctrl + Enter</span>
                    <span class="text-teal-600 dark:text-teal-400 font-bold">Run Request</span>
                  </div>
                  <div class="bg-slate-100 dark:bg-[#151b2b] p-2 rounded flex justify-between">
                    <span>Cmd + Enter</span>
                    <span class="text-teal-600 dark:text-teal-400 font-bold">Run Request (Mac)</span>
                  </div>
                </div>
              </div>

              <div class="space-y-1">
                <h4 class="font-bold text-slate-900 dark:text-white">Supported Operations</h4>
                <p>You can execute all standard OpenSearch HTTP verbs: <code class="bg-slate-100 dark:bg-slate-800 px-1 py-0.5 rounded font-mono font-bold">GET</code>, <code class="bg-slate-100 dark:bg-slate-800 px-1 py-0.5 rounded font-mono font-bold">POST</code>, <code class="bg-slate-100 dark:bg-slate-800 px-1 py-0.5 rounded font-mono font-bold">PUT</code>, <code class="bg-slate-100 dark:bg-slate-800 px-1 py-0.5 rounded font-mono font-bold">DELETE</code>, and <code class="bg-slate-100 dark:bg-slate-800 px-1 py-0.5 rounded font-mono font-bold">HEAD</code>.</p>
              </div>
            </div>

            <div class="flex items-center justify-end pt-2 border-t border-slate-200 dark:border-[#1f283d]">
              <button
                @click="showHelpModal = false"
                class="px-4 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 rounded-lg text-xs font-semibold cursor-pointer transition"
              >
                Close
              </button>
            </div>
          </div>
        </div>

        <!-- ================================================================= -->
        <!-- MODAL: IMPORT -->
        <!-- ================================================================= -->
        <div
          v-if="showImportModal"
          class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
        >
          <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-5 space-y-4">
            <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
              <div class="flex items-center gap-2">
                <Upload class="w-4 h-4 text-slate-500 dark:text-slate-400" />
                <h3 class="text-sm font-bold text-slate-900 dark:text-white">Import Query to Console</h3>
              </div>
              <button
                @click="showImportModal = false"
                class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-1 cursor-pointer"
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <div class="space-y-2">
              <label class="text-xs text-slate-600 dark:text-slate-400">Paste raw query, cURL, or bulk request:</label>
              <textarea
                v-model="importInputText"
                rows="8"
                placeholder="POST _bulk&#10;{ &quot;create&quot;: { &quot;_index&quot;: &quot;test&quot; } }&#10;{ &quot;field&quot;: &quot;value&quot; }"
                class="w-full bg-[#f8fafc] dark:bg-[#141824] border border-slate-300 dark:border-slate-700 rounded-lg p-3 text-xs font-mono text-slate-900 dark:text-slate-100 focus:outline-none focus:border-teal-500"
              ></textarea>
            </div>

            <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-[#1f283d]">
              <button
                @click="showImportModal = false"
                class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                @click="importQueryText"
                :disabled="!importInputText.trim()"
                class="px-4 py-1.5 bg-[#00805a] hover:bg-[#00704e] text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
              >
                Load into Console
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 5: CONNECTION CONFIGURATION -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'connection'" class="max-w-2xl mx-auto space-y-6">
        <div class="p-6 bg-[#1b1e26] border border-slate-800/80 rounded-xl space-y-6 shadow-xl">
          <div class="flex items-center gap-3 border-b border-slate-800 pb-4">
            <Sliders class="w-5 h-5 text-brand-400" />
            <div>
              <h3 class="text-sm font-bold text-white">Cluster Connection Settings</h3>
              <p class="text-xs text-slate-400">Configure connection credentials to your OpenSearch cluster</p>
            </div>
          </div>

          <form @submit.prevent="handleSaveConfig" class="space-y-4 text-xs">
            <div>
              <label class="block text-slate-400 mb-1 font-medium">Cluster Name</label>
              <input v-model="configForm.name" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="e.g. Production Cluster" />
            </div>

            <div class="grid grid-cols-3 gap-4">
              <div class="col-span-2">
                <label class="block text-slate-400 mb-1 font-medium">OpenSearch Host / IP</label>
                <input v-model="configForm.host" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="e.g. 103.171.31.56 or search.example.com" />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-medium">HTTP Port</label>
                <input v-model.number="configForm.port" type="number" required class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-slate-400 mb-1 font-medium">Username (Optional)</label>
                <input v-model="configForm.username" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="admin" />
              </div>
              <div>
                <label class="block text-slate-400 mb-1 font-medium">Password (Encrypted AES-256)</label>
                <input v-model="configForm.password" type="password" class="w-full bg-[#14161b] border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500 font-mono" placeholder="••••••••" />
              </div>
            </div>

            <div class="flex items-center gap-6 pt-2">
              <label class="flex items-center gap-2 cursor-pointer">
                <input v-model="configForm.useSsl" type="checkbox" class="rounded bg-slate-800 border-slate-700 text-brand-500 focus:ring-0" />
                <span class="text-slate-300">Use HTTPS (SSL/TLS)</span>
              </label>
              <label class="flex items-center gap-2 cursor-pointer">
                <input v-model="configForm.verifySsl" type="checkbox" class="rounded bg-slate-800 border-slate-700 text-brand-500 focus:ring-0" />
                <span class="text-slate-300">Verify SSL Certificate</span>
              </label>
            </div>

            <div
              v-if="testResult"
              class="p-3 rounded-lg text-xs"
              :class="isTestingConnection
                ? 'bg-sky-500/10 border border-sky-500/20 text-sky-400 animate-pulse'
                : testSuccess
                ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-400'
                : 'bg-red-500/10 border border-red-500/20 text-red-400'"
            >
              {{ testResult }}
            </div>

            <div class="flex items-center justify-between pt-4 border-t border-slate-800">
              <button
                type="button"
                @click="handleTestConnection"
                :disabled="isTestingConnection"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg transition disabled:opacity-50 cursor-pointer"
              >
                {{ isTestingConnection ? 'Testing...' : 'Test Connection' }}
              </button>

              <button
                v-if="canManage"
                type="submit"
                :disabled="isSavingConfig"
                class="px-5 py-2 bg-brand-500 hover:bg-brand-600 text-white font-medium rounded-lg transition disabled:opacity-50 cursor-pointer"
              >
                {{ isSavingConfig ? 'Saving...' : 'Save Configuration' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- ================================================================= -->
      <!-- TAB 6: LOGS -->
      <!-- ================================================================= -->
      <div v-if="activeTab === 'logs'" class="space-y-4">
        <div class="bg-[#1b1e26] border border-slate-800/80 rounded-xl overflow-hidden shadow-xl">
          <div class="p-4 px-6 border-b border-slate-800 flex items-center justify-between">
            <h3 class="text-xs font-bold text-white tracking-wide">OpenSearch Background Job Logs</h3>
            <span class="text-xs font-mono text-slate-400">{{ logsList.length }} entries</span>
          </div>

          <div class="p-4 max-h-[600px] overflow-y-auto font-mono text-xs space-y-2 bg-[#14161b]">
            <div
              v-for="(log, lIdx) in logsList"
              :key="lIdx"
              class="flex items-start gap-3 p-2 rounded hover:bg-slate-800/30 transition text-slate-300"
            >
              <span class="text-slate-500 shrink-0">{{ log.timestamp ? new Date(log.timestamp).toLocaleTimeString() : '-' }}</span>
              <span
                :class="[
                  'px-1.5 py-0.2 rounded text-[10px] font-bold shrink-0',
                  log.level === 'ERROR' ? 'bg-red-500/20 text-red-400' : log.level === 'WARN' ? 'bg-amber-500/20 text-amber-400' : 'bg-emerald-500/20 text-emerald-400'
                ]"
              >
                {{ log.level || 'INFO' }}
              </span>
              <span class="flex-1 text-slate-300">{{ log.message }}</span>
            </div>
            <p v-if="logsList.length === 0" class="text-center py-8 text-slate-500 italic">No OpenSearch logs recorded.</p>
          </div>
        </div>
      </div>

    </main>

    <!-- ================================================================= -->
    <!-- FLOATING SHARD TOOLTIP (MATCHING USER SCREENSHOT EXACTLY) -->
    <!-- ================================================================= -->
    <div
      v-if="hoveredShard"
      :style="{ left: `${tooltipPosition.x}px`, top: `${tooltipPosition.y}px` }"
      class="fixed z-50 w-80 p-4 bg-[#141b2d] border border-blue-500/50 rounded-xl shadow-2xl backdrop-blur-md pointer-events-none text-xs space-y-2.5 animate-in fade-in zoom-in-95 duration-100"
    >
      <div class="font-bold text-[#60a5fa] text-xs pb-1.5 border-b border-slate-700/60 truncate">
        {{ hoveredShard.index }}
      </div>
      <div class="space-y-1.5 text-[11px] font-sans text-slate-300">
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Index:</span> <span class="font-mono text-slate-200 truncate max-w-[180px]">{{ hoveredShard.index }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Size:</span> <span class="font-mono text-slate-200">{{ hoveredShard.store || hoveredShard['store.size'] || '-' }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Node:</span> <span class="text-slate-200 font-mono">{{ hoveredShard.node || '-' }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">IP:</span> <span class="font-mono text-slate-200">{{ hoveredShard.ip || '-' }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Docs:</span> <span class="font-mono text-slate-200">{{ formatNumber(hoveredShard.docs || 0) }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Shard:</span> <span class="font-mono text-slate-200">{{ hoveredShard.shard || 0 }}</span></div>
        <div class="flex justify-between"><span class="text-slate-400 font-medium">Type:</span> <span :class="hoveredShard.prirep === 'p' || hoveredShard.type === 'Primary' ? 'text-emerald-400 font-bold' : 'text-sky-400 font-bold'">{{ hoveredShard.prirep === 'p' || hoveredShard.type === 'Primary' ? 'Primary' : 'Replica' }}</span></div>
        <div class="flex justify-between">
          <span class="text-slate-400 font-medium">State:</span>
          <span
            :class="[
              'uppercase font-bold tracking-wider',
              hoveredShard.state === 'INITIALIZING' ? 'text-amber-400' : hoveredShard.state === 'RELOCATING' ? 'text-purple-400' : hoveredShard.state === 'UNASSIGNED' ? 'text-rose-400' : 'text-emerald-400'
            ]"
          >
            {{ hoveredShard.state || 'STARTED' }}
          </span>
        </div>
        <div v-if="hoveredShard['unassigned.reason'] || hoveredShard['unassigned.for']" class="pt-1.5 border-t border-slate-700/60">
          <div class="flex justify-between text-rose-400">
            <span class="font-bold">Reason:</span>
            <span class="font-mono text-rose-300 font-semibold">{{ hoveredShard['unassigned.reason'] }}</span>
          </div>
          <div v-if="hoveredShard['unassigned.for']" class="flex justify-between text-slate-400 text-[10px] mt-0.5">
            <span>Unassigned for:</span>
            <span class="font-mono">{{ hoveredShard['unassigned.for'] }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: INDEX DETAILS & SHARDS BREAKDOWN -->
    <!-- ================================================================= -->
    <div
      v-if="selectedIndexModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-2xl p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Database class="w-4 h-4 text-brand-400" />
            <h3 class="text-sm font-bold text-white">{{ selectedIndexModal.index || selectedIndexModal.name }}</h3>
          </div>
          <button @click="selectedIndexModal = null" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="grid grid-cols-3 gap-3 text-xs font-sans">
          <div class="p-3 rounded-lg bg-[#14161b] border border-slate-800">
            <p class="text-[10px] text-slate-500 uppercase font-bold">Health</p>
            <p class="text-sm font-bold text-emerald-400 uppercase">{{ selectedIndexModal.health || 'GREEN' }}</p>
          </div>
          <div class="p-3 rounded-lg bg-[#14161b] border border-slate-800">
            <p class="text-[10px] text-slate-500 uppercase font-bold">Primary Shards</p>
            <p class="text-sm font-bold text-white font-mono">{{ selectedIndexModal.pri || 1 }}</p>
          </div>
          <div class="p-3 rounded-lg bg-[#14161b] border border-slate-800">
            <p class="text-[10px] text-slate-500 uppercase font-bold">Replica Shards</p>
            <p class="text-sm font-bold text-sky-400 font-mono">{{ selectedIndexModal.rep || 0 }}</p>
          </div>
        </div>

        <!-- Allocated Shards for this Index -->
        <div class="space-y-2">
          <h4 class="text-xs font-bold text-slate-400 uppercase tracking-wider">Allocated Shards</h4>
          <div class="overflow-x-auto rounded-lg border border-slate-800">
            <table class="w-full text-left text-xs font-mono">
              <thead class="bg-[#20242e] text-slate-400 text-[10px] uppercase">
                <tr>
                  <th class="p-2.5 px-4">Shard #</th>
                  <th class="p-2.5 px-4">Type</th>
                  <th class="p-2.5 px-4">Node</th>
                  <th class="p-2.5 px-4">IP</th>
                  <th class="p-2.5 px-4">Docs</th>
                  <th class="p-2.5 px-4">Size</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800/60 text-slate-300">
                <tr v-for="(s, sIdx) in selectedIndexShards" :key="sIdx" class="hover:bg-slate-800/30">
                  <td class="p-2.5 px-4 text-white">{{ s.shard }}</td>
                  <td class="p-2.5 px-4 font-sans">
                    <span :class="s.prirep === 'p' || s.type === 'Primary' ? 'text-emerald-400 font-bold' : 'text-sky-400 font-bold'">
                      {{ s.prirep === 'p' || s.type === 'Primary' ? 'Primary' : 'Replica' }}
                    </span>
                  </td>
                  <td class="p-2.5 px-4 text-slate-200">{{ s.node }}</td>
                  <td class="p-2.5 px-4 text-slate-400">{{ s.ip }}</td>
                  <td class="p-2.5 px-4">{{ formatNumber(s.docs || 0) }}</td>
                  <td class="p-2.5 px-4">{{ s.store || '-' }}</td>
                </tr>
                <tr v-if="selectedIndexShards.length === 0">
                  <td colspan="6" class="p-4 text-center text-slate-500 font-sans">No shard allocations discovered.</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div class="flex justify-end pt-2">
          <button @click="selectedIndexModal = null" class="px-4 py-2 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 hover:text-slate-900 dark:text-slate-300 rounded-lg text-xs font-semibold border border-slate-300 dark:border-transparent transition cursor-pointer">
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: AUTO-REFRESH SETTINGS (GEAR ICON) -->
    <!-- ================================================================= -->
    <div
      v-if="isSettingsModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="bg-[#1b1e26] border border-slate-800 rounded-2xl w-full max-w-sm p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <Settings class="w-4 h-4 text-brand-400" />
            <h3 class="text-sm font-bold text-white">Auto-Refresh Interval</h3>
          </div>
          <button @click="isSettingsModalOpen = false" class="text-slate-400 hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-2">
          <button
            v-for="opt in [
              { label: 'Manual Refresh Only', sec: 0 },
              { label: 'Every 5 Seconds', sec: 5 },
              { label: 'Every 10 Seconds', sec: 10 },
              { label: 'Every 15 Seconds', sec: 15 },
              { label: 'Every 30 Seconds (Default)', sec: 30 },
              { label: 'Every 60 Seconds (1 Min)', sec: 60 },
              { label: 'Every 300 Seconds (5 Min)', sec: 300 }
            ]"
            :key="opt.sec"
            @click="setRefreshInterval(opt.sec)"
            :class="[
              'w-full p-2.5 rounded-lg text-xs text-left transition flex items-center justify-between border',
              refreshIntervalSec === opt.sec
                ? 'bg-brand-500/10 border-brand-500/40 text-brand-600 dark:text-brand-400 font-bold'
                : 'bg-slate-50 hover:bg-slate-100 dark:bg-[#14161b] dark:hover:bg-[#1b2234] border-slate-200 dark:border-slate-800 text-slate-700 dark:text-slate-300 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer'
            ]"
          >
            <span>{{ opt.label }}</span>
            <CheckCircle2 v-if="refreshIntervalSec === opt.sec" class="w-3.5 h-3.5 text-brand-400" />
          </button>
        </div>
      </div>
    </div>

    <!-- ================================================================= -->
    <!-- MODAL: SHARE & EMBED OVERVIEW -->
    <!-- ================================================================= -->
    <div
      v-if="isShareModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 overflow-y-auto"
    >
      <div class="bg-white dark:bg-[#1b1e26] border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-xl p-6 space-y-5 shadow-2xl text-slate-800 dark:text-slate-200 my-8">
        <!-- Modal Header -->
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-500/10 border border-blue-200 dark:border-blue-500/30 flex items-center justify-center text-blue-600 dark:text-blue-400 shrink-0">
              <Share2 class="w-4 h-4" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white">Share & Embed Overview</h3>
              <p class="text-[11px] text-slate-500 dark:text-slate-400">Generate standalone URL or iframe snippet for external dashboards & displays</p>
            </div>
          </div>
          <button @click="isShareModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white p-1 rounded-lg">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Direct Embed URL -->
        <div class="space-y-1.5">
          <label class="block text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">Embeddable Direct URL</label>
          <div class="flex items-center gap-2">
            <input
              :value="generatedShareUrl"
              readonly
              class="flex-1 bg-slate-100 dark:bg-[#14161b] border border-slate-300 dark:border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 select-all focus:outline-none focus:border-blue-500"
            />
            <button
              @click="copyShareUrl"
              class="px-3.5 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition shrink-0"
            >
              <Check v-if="copiedUrl" class="w-3.5 h-3.5 text-emerald-300" />
              <Copy v-else class="w-3.5 h-3.5" />
              <span>{{ copiedUrl ? 'Copied!' : 'Copy Link' }}</span>
            </button>
          </div>
        </div>

        <!-- HTML iframe Code Snippet -->
        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <label class="block text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider flex items-center gap-1">
              <Code2 class="w-3.5 h-3.5 text-blue-500" />
              <span>HTML iframe Embed Snippet</span>
            </label>
            <button
              @click="copyIframeCode"
              class="text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 font-medium"
            >
              <Check v-if="copiedIframe" class="w-3 h-3 text-emerald-400" />
              <Copy v-else class="w-3 h-3" />
              <span>{{ copiedIframe ? 'Copied!' : 'Copy iframe' }}</span>
            </button>
          </div>
          <textarea
            :value="generatedIframeCode"
            readonly
            rows="3"
            class="w-full bg-slate-100 dark:bg-[#14161b] border border-slate-300 dark:border-slate-800 rounded-lg p-2.5 text-[11px] font-mono text-amber-600 dark:text-amber-400 select-all resize-none focus:outline-none focus:border-blue-500"
          ></textarea>
        </div>

        <!-- Customization Options -->
        <div class="p-4 bg-slate-50 dark:bg-[#14161b] border border-slate-200 dark:border-slate-800/80 rounded-xl space-y-3 text-xs">
          <div class="text-[11px] font-bold text-slate-600 dark:text-slate-400 uppercase tracking-wider">Embed Options</div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <label class="flex items-center gap-2 cursor-pointer text-slate-700 dark:text-slate-300">
              <input
                type="checkbox"
                v-model="shareEmbedMode"
                class="rounded bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-blue-600 focus:ring-0 w-4 h-4 cursor-pointer"
              />
              <span>Clean Embed (Hide tabs & navigation)</span>
            </label>

            <label class="flex items-center gap-2 cursor-pointer text-slate-700 dark:text-slate-300">
              <input
                type="checkbox"
                v-model="shareHideHeader"
                class="rounded bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-blue-600 focus:ring-0 w-4 h-4 cursor-pointer"
              />
              <span>Full Headless (Hide top bar completely)</span>
            </label>

            <label class="flex items-start gap-2 cursor-pointer text-slate-700 dark:text-slate-300 sm:col-span-2">
              <input
                type="checkbox"
                v-model="shareIncludeToken"
                class="rounded bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-blue-600 focus:ring-0 w-4 h-4 cursor-pointer mt-0.5"
              />
              <div>
                <span class="font-medium text-slate-900 dark:text-white">Include Authentication Session Token</span>
                <p class="text-[10px] text-slate-500">Allows external monitors, TVs, or iframes to load telemetry without manual login prompt.</p>
              </div>
            </label>
          </div>

          <!-- Theme & Refresh Options -->
          <div class="grid grid-cols-2 gap-3 pt-1 border-t border-slate-200 dark:border-slate-800/80">
            <div>
              <label class="block text-[10px] text-slate-500 uppercase font-semibold mb-1">Theme Mode</label>
              <select
                v-model="shareTheme"
                class="w-full bg-white dark:bg-[#1b1e26] border border-slate-300 dark:border-slate-700 rounded px-2 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:outline-none"
              >
                <option value="auto">System / Auto</option>
                <option value="dark">Always Dark</option>
                <option value="light">Always Light</option>
              </select>
            </div>

            <div>
              <label class="block text-[10px] text-slate-500 uppercase font-semibold mb-1">Auto-Refresh Interval</label>
              <select
                v-model.number="shareRefreshSec"
                class="w-full bg-white dark:bg-[#1b1e26] border border-slate-300 dark:border-slate-700 rounded px-2 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:outline-none"
              >
                <option :value="10">Every 10 seconds</option>
                <option :value="30">Every 30 seconds (Default)</option>
                <option :value="60">Every 1 minute</option>
                <option :value="300">Every 5 minutes</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Modal Footer Actions -->
        <div class="flex items-center justify-between pt-2 border-t border-slate-200 dark:border-slate-800">
          <button
            @click="openEmbedPreview"
            class="flex items-center gap-1.5 px-4 py-2 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 rounded-lg text-xs font-semibold transition"
          >
            <ExternalLink class="w-3.5 h-3.5" />
            <span>Open Preview in New Tab</span>
          </button>

          <button
            @click="isShareModalOpen = false"
            class="px-5 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-bold transition"
          >
            Done
          </button>
        </div>
      </div>
    </div>

  </div>
</template>
