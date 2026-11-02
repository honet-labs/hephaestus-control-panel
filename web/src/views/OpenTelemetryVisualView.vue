<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import InfoButton from '../components/InfoButton.vue';
import {
  Server,
  Plus,
  RotateCw,
  Play,
  Save,
  CheckCircle2,
  AlertTriangle,
  FileCode,
  History,
  Search,
  X,
  Copy,
  Check,
  Layers,
  ArrowRight,
  Info,
  Loader2,
  Radio,
  Sliders,
  Box,
  Boxes,
  FileText,
  Flame,
  Globe,
  Database,
  Cpu,
  ZoomIn,
  ZoomOut,
  Maximize2,
  RefreshCw,
  Sparkles,
  LayoutGrid,
  Columns,
  Eye,
  ExternalLink,
  ChevronDown,
} from 'lucide-vue-next';

// ==================== INTERFACES ====================
interface OTelHost {
  id: string;
  name: string;
  tags?: string[];
  sshHost: string;
  sshPort: number;
  sshUser: string;
  sshAuth: string;
  configPath: string;
  serviceName: string;
  reloadMode: string;
  lastStatus: string; // active, inactive, failed, unreachable, unknown
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

interface OTelHistoryItem {
  id: string;
  otelConfigId: string;
  content: string;
  createdBy?: string;
  changeSummary?: string;
  createdAt: string;
}

interface OTelComponent {
  id: string;
  category: 'receiver' | 'processor' | 'exporter';
  name: string;
  type: string;
  instanceName?: string;
  rawSnippet: string;
  endpoint?: string;
  interval?: string;
  timeout?: string;
  scrapers?: string[];
  tlsInsecure?: boolean;
  paths?: string[];
  actions?: string[];
  pipelines: string[];
  description?: string;
}

interface OTelPipeline {
  id: string;
  name: string;
  signal: 'metrics' | 'logs' | 'traces';
  receivers: string[];
  processors: string[];
  exporters: string[];
}

interface GraphNode {
  id: string;
  category: 'receiver' | 'processor' | 'exporter';
  component: OTelComponent;
  x: number;
  y: number;
  column: number;
}

interface GraphLink {
  id: string;
  pipelineId: string;
  pipelineName: string;
  signal: 'metrics' | 'logs' | 'traces';
  sourceNodeId: string;
  targetNodeId: string;
  sourceX: number;
  sourceY: number;
  targetX: number;
  targetY: number;
  pathD: string;
  badgeLabel?: string;
  midX: number;
  midY: number;
}

// ==================== DEFAULT TEMPLATES ====================
const DEFAULT_SAMPLE_CONFIG = `receivers:
  # FOR METRICS Container Docker
  docker_stats:
    endpoint: "unix:///var/run/docker.sock"
    collection_interval: 30s
    timeout: 10s
    env_vars_to_metric_labels:
      COMPOSE_PROJECT_NAME: docker-dev

  # FOR METRICS Host
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
        metrics:
          system.cpu.logical.count:
            enabled: true
          system.cpu.time:
            enabled: true
          system.cpu.frequency:
            enabled: true
          system.cpu.physical.count:
            enabled: true
          system.cpu.utilization:
            enabled: true
      disk:
        metrics:
          system.disk.io:
            enabled: true
          system.disk.io_time:
            enabled: true
      load:
      filesystem:
      memory:
      network:
      paging:
      processes:
      process:

  # FOR LOGS Container Docker
  filelog/wordpress:
    include: [/var/lib/docker/containers/*/*-json.log]
    include_file_path: true
    operators:
      - type: json_parser
        timestamp:
          parse_from: attributes.time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'

processors:
  batch:
    send_batch_size: 1000
    timeout: 10s
  attributes:
    actions:
      - key: environment
        value: development
        action: insert

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: "host"
    send_timestamps: true
    metric_expiration: 180m

  prometheus/docker:
    endpoint: "0.0.0.0:8886"
    namespace: "docker"
    send_timestamps: true
    metric_expiration: 180m

  otlp/container_docker_metrics:
    endpoint: "10.10.5.174:4940"
    tls:
      insecure: true

  otlp/container_wordpress_logs:
    endpoint: "10.10.5.174:4942"
    tls:
      insecure: true

service:
  pipelines:
    metrics/host_prometheus:
      receivers: [hostmetrics]
      processors: [batch]
      exporters: [prometheus]

    metrics/docker_prometheus:
      receivers: [docker_stats]
      processors: [batch]
      exporters: [prometheus/docker]

    metrics/opensearch:
      receivers: [docker_stats]
      processors: [batch]
      exporters: [otlp/container_docker_metrics]

    logs/wordpress:
      receivers: [filelog/wordpress]
      processors: [attributes, batch]
      exporters: [otlp/container_wordpress_logs]
`;

const MINIMAL_TEMPLATE = `receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      memory:
      disk:
      network:

processors:
  batch:
    send_batch_size: 1024
    timeout: 5s

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: "host"

service:
  pipelines:
    metrics/host:
      receivers: [hostmetrics]
      processors: [batch]
      exporters: [prometheus]
`;

// ==================== AUTH & RBAC ====================
const authStore = useAuthStore();
const canManage = computed(() => authStore.can('opentelemetry_config', 'manage'));

// ==================== FLEET / HOST STATE ====================
const hosts = ref<OTelHost[]>([]);
const selectedHostId = ref<string>('');
const selectedHost = computed(() => hosts.value.find((h) => h.id === selectedHostId.value) || null);
const loadingHosts = ref(false);
const searchQuery = ref('');
const showHostDropdown = ref(false);

// ==================== CONFIGURATION STATE ====================
const yamlContent = ref(DEFAULT_SAMPLE_CONFIG);
const initialLoadedContent = ref(DEFAULT_SAMPLE_CONFIG);
const configSummary = ref('');
const saving = ref(false);
const loadingConfig = ref(false);
const restartingService = ref(false);

// Notification banner
const feedbackMsg = ref<{ type: 'success' | 'error' | 'info'; title: string; detail?: string } | null>(null);
let feedbackTimer: ReturnType<typeof setTimeout> | null = null;
const setFeedback = (type: 'success' | 'error' | 'info', title: string, detail?: string) => {
  if (feedbackTimer) clearTimeout(feedbackTimer);
  feedbackMsg.value = { type, title, detail };
  feedbackTimer = setTimeout(() => {
    feedbackMsg.value = null;
    feedbackTimer = null;
  }, 3000);
};

// Validation state
const validating = ref(false);
const validationResult = ref<{
  valid: boolean;
  message: string;
  output?: string;
  error?: string;
  line?: number;
} | null>(null);

// Real-time YAML validation
const yamlSyntaxCheck = computed(() => {
  if (!yamlContent.value || !yamlContent.value.trim()) {
    return { valid: true, error: null, line: null };
  }
  const lines = yamlContent.value.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (line.includes('\t')) {
      return { valid: false, error: `Line ${i + 1}: Tab characters are forbidden in YAML. Use spaces.`, line: i + 1 };
    }
  }
  return { valid: true, error: null, line: null };
});

const hasUnsavedChanges = computed(() => yamlContent.value !== initialLoadedContent.value);

// View Mode: 'visual' | 'split' | 'code'
const viewMode = ref<'visual' | 'split' | 'code'>('visual');

// Selected pipeline filter: 'all' or pipeline id
const selectedPipelineFilter = ref<string>('all');

// Canvas interaction state
const zoomLevel = ref(1);
const panX = ref(0);
const panY = ref(0);
const isPanning = ref(false);
const panStart = ref({ x: 0, y: 0 });
const animatedFlow = ref(true);

// Node inspection state
const inspectedNode = ref<OTelComponent | null>(null);
const hoveredNodeId = ref<string | null>(null);
const hoveredLinkId = ref<string | null>(null);
const copiedYaml = ref(false);

// Templates & Confirmation Modals
const showTemplateModal = ref(false);
const showConfirmModal = ref(false);
const confirmAction = ref<(() => void) | null>(null);
const confirmTitle = ref('');
const confirmDescription = ref('');

// History Modal
const showHistoryModal = ref(false);
const historyList = ref<OTelHistoryItem[]>([]);
const loadingHistory = ref(false);
const previewHistoryItem = ref<OTelHistoryItem | null>(null);

// Editor ref
const editorRef = ref<HTMLTextAreaElement | null>(null);
const gutterRef = ref<HTMLDivElement | null>(null);

// Line numbers generator
const lineNumbers = computed(() => {
  if (!yamlContent.value) return Array.from({ length: 25 }, (_, i) => i + 1);
  const count = yamlContent.value.split('\n').length;
  return Array.from({ length: Math.max(count, 25) }, (_, i) => i + 1);
});

// ==================== ROBUST OPENTELEMETRY YAML PARSER ====================
const parsedModel = computed(() => {
  const content = yamlContent.value || '';
  const lines = content.split('\n');

  const componentsMap = new Map<string, OTelComponent>();
  const pipelinesList: OTelPipeline[] = [];

  let currentSection: 'receivers' | 'processors' | 'exporters' | 'service' | '' = '';
  let inPipelines = false;
  let currentComponent: Partial<OTelComponent> | null = null;
  let componentLines: string[] = [];
  let currentPipeline: Partial<OTelPipeline> | null = null;
  let currentPipelineKey = '';
  let inListField: 'receivers' | 'processors' | 'exporters' | '' = '';

  const toCategory = (sec: string): 'receiver' | 'processor' | 'exporter' => {
    if (sec === 'processors' || sec === 'processor') return 'processor';
    if (sec === 'exporters' || sec === 'exporter') return 'exporter';
    return 'receiver';
  };

  const finalizeComponent = () => {
    if (currentComponent && currentComponent.name && currentComponent.category) {
      const id = `${currentComponent.category}:${currentComponent.name}`;
      currentComponent.rawSnippet = componentLines.join('\n');
      componentsMap.set(id, currentComponent as OTelComponent);
    }
    currentComponent = null;
    componentLines = [];
  };

  const finalizePipeline = () => {
    if (currentPipeline && currentPipeline.name) {
      pipelinesList.push({
        id: currentPipeline.name,
        name: currentPipeline.name,
        signal: currentPipeline.signal || 'metrics',
        receivers: currentPipeline.receivers || [],
        processors: currentPipeline.processors || [],
        exporters: currentPipeline.exporters || [],
      });
    }
    currentPipeline = null;
    currentPipelineKey = '';
    inListField = '';
  };

  for (let i = 0; i < lines.length; i++) {
    const rawLine = lines[i];
    const trimmed = rawLine.trim();
    const lineWithoutComment = rawLine.split('#')[0];
    const trimmedNoComment = lineWithoutComment.trim();

    // Check top-level sections (0 indent)
    if (!rawLine.startsWith(' ') && !rawLine.startsWith('\t') && trimmedNoComment.endsWith(':')) {
      finalizeComponent();
      finalizePipeline();
      inPipelines = false;

      const secName = trimmedNoComment.slice(0, -1).trim();
      if (secName === 'receivers' || secName === 'processors' || secName === 'exporters' || secName === 'service') {
        currentSection = secName;
      } else {
        currentSection = '';
      }
      continue;
    }

    if (!currentSection) continue;

    // Inside receivers, processors, exporters
    if (currentSection === 'receivers' || currentSection === 'processors' || currentSection === 'exporters') {
      const indent = rawLine.match(/^(\s*)/)?.[1].length || 0;
      // 2-space indent indicates new component name
      const isComponentHeader = indent === 2 && trimmedNoComment.endsWith(':');
      if (isComponentHeader) {
        finalizeComponent();
        const rawCompName = trimmedNoComment.slice(0, -1).trim();
        const parts = rawCompName.split('/');
        const compType = parts[0];
        const instanceName = parts.length > 1 ? parts.slice(1).join('/') : undefined;
        const cat = toCategory(currentSection);

        currentComponent = {
          id: `${cat}:${rawCompName}`,
          category: cat,
          name: rawCompName,
          type: compType,
          instanceName,
          rawSnippet: '',
          scrapers: [],
          actions: [],
          paths: [],
          pipelines: [],
        };
        componentLines = [rawLine];
        continue;
      }

      if (currentComponent) {
        componentLines.push(rawLine);

        // Extract key metadata
        if (trimmed.startsWith('endpoint:')) {
          currentComponent.endpoint = trimmed.replace('endpoint:', '').trim().replace(/['"]/g, '');
        } else if (trimmed.startsWith('collection_interval:')) {
          currentComponent.interval = trimmed.replace('collection_interval:', '').trim().replace(/['"]/g, '');
        } else if (trimmed.startsWith('timeout:')) {
          currentComponent.timeout = trimmed.replace('timeout:', '').trim().replace(/['"]/g, '');
        } else if (trimmed.startsWith('insecure:')) {
          currentComponent.tlsInsecure = trimmed.includes('true');
        } else if (trimmed.startsWith('include:')) {
          const m = trimmed.match(/\[(.*)\]/);
          if (m && m[1]) {
            currentComponent.paths = m[1].split(',').map((s) => s.trim().replace(/['"]/g, ''));
          }
        } else if (rawLine.startsWith('      ') && !rawLine.startsWith('        ') && trimmed.endsWith(':') && componentLines.some((l) => l.includes('scrapers:'))) {
          // Scraper item like cpu:, disk:, memory:
          const scraperName = trimmed.slice(0, -1);
          if (scraperName && scraperName !== 'metrics' && !currentComponent.scrapers?.includes(scraperName)) {
            currentComponent.scrapers?.push(scraperName);
          }
        } else if (trimmed.startsWith('- key:') || trimmed.startsWith('value:')) {
          if (!currentComponent.actions) currentComponent.actions = [];
          if (trimmed.startsWith('- key:')) {
            currentComponent.actions.push(trimmed.replace('- key:', '').trim());
          }
        }
      }
    }

    // Inside service -> pipelines
    if (currentSection === 'service') {
      const indent = rawLine.match(/^(\s*)/)?.[1].length || 0;
      if (indent === 2 && trimmedNoComment.startsWith('pipelines:')) {
        inPipelines = true;
        continue;
      }

      if (inPipelines) {
        // Pipeline header (4-space indent)
        if (indent === 4 && trimmedNoComment.endsWith(':')) {
          finalizePipeline();
          currentPipelineKey = trimmedNoComment.slice(0, -1).trim();
          let signal: 'metrics' | 'logs' | 'traces' = 'metrics';
          if (currentPipelineKey.startsWith('logs')) signal = 'logs';
          else if (currentPipelineKey.startsWith('traces')) signal = 'traces';

          currentPipeline = {
            id: currentPipelineKey,
            name: currentPipelineKey,
            signal,
            receivers: [],
            processors: [],
            exporters: [],
          };
          continue;
        }

        if (currentPipeline) {
          // receivers:, processors:, exporters:
          if (trimmed.startsWith('receivers:')) {
            inListField = 'receivers';
            const inlineMatch = trimmed.match(/\[(.*)\]/);
            if (inlineMatch && inlineMatch[1]) {
              currentPipeline.receivers = inlineMatch[1].split(',').map((s) => s.trim().replace(/['"]/g, '')).filter(Boolean);
            }
          } else if (trimmed.startsWith('processors:')) {
            inListField = 'processors';
            const inlineMatch = trimmed.match(/\[(.*)\]/);
            if (inlineMatch && inlineMatch[1]) {
              currentPipeline.processors = inlineMatch[1].split(',').map((s) => s.trim().replace(/['"]/g, '')).filter(Boolean);
            }
          } else if (trimmed.startsWith('exporters:')) {
            inListField = 'exporters';
            const inlineMatch = trimmed.match(/\[(.*)\]/);
            if (inlineMatch && inlineMatch[1]) {
              currentPipeline.exporters = inlineMatch[1].split(',').map((s) => s.trim().replace(/['"]/g, '')).filter(Boolean);
            }
          } else if (trimmed.startsWith('- ') && inListField) {
            const val = trimmed.slice(2).trim().replace(/['"]/g, '');
            if (val) {
              if (inListField === 'receivers' && !currentPipeline.receivers?.includes(val)) currentPipeline.receivers?.push(val);
              if (inListField === 'processors' && !currentPipeline.processors?.includes(val)) currentPipeline.processors?.push(val);
              if (inListField === 'exporters' && !currentPipeline.exporters?.includes(val)) currentPipeline.exporters?.push(val);
            }
          }
        }
      }
    }
  }

  finalizeComponent();
  finalizePipeline();

  // Link components with pipelines using them
  pipelinesList.forEach((pipe) => {
    pipe.receivers.forEach((rName) => {
      const c = componentsMap.get(`receiver:${rName}`);
      if (c && !c.pipelines.includes(pipe.name)) c.pipelines.push(pipe.name);
    });
    pipe.processors.forEach((pName) => {
      const c = componentsMap.get(`processor:${pName}`);
      if (c && !c.pipelines.includes(pipe.name)) c.pipelines.push(pipe.name);
    });
    pipe.exporters.forEach((eName) => {
      const c = componentsMap.get(`exporter:${eName}`);
      if (c && !c.pipelines.includes(pipe.name)) c.pipelines.push(pipe.name);
    });
  });

  return {
    components: Array.from(componentsMap.values()),
    pipelines: pipelinesList,
  };
});

// Available pipeline filters
const availablePipelines = computed(() => parsedModel.value.pipelines);

// Active pipelines based on selected filter
const activePipelines = computed(() => {
  if (selectedPipelineFilter.value === 'all') {
    return parsedModel.value.pipelines;
  }
  return parsedModel.value.pipelines.filter((p) => p.name === selectedPipelineFilter.value);
});

// ==================== TOPOLOGY GRAPH COMPUTATION ====================
// Generates nodes and curved links replicating the user's reference hop topology
const graphData = computed(() => {
  const { components } = parsedModel.value;
  const pipelines = activePipelines.value;

  // Filter components relevant to active pipelines if specific pipeline is selected
  let activeComponents = components;
  if (pipelines.length > 0 && selectedPipelineFilter.value !== 'all') {
    const relevantNames = new Set<string>();
    pipelines.forEach((p) => {
      p.receivers.forEach((r) => relevantNames.add(`receiver:${r}`));
      p.processors.forEach((pr) => relevantNames.add(`processor:${pr}`));
      p.exporters.forEach((e) => relevantNames.add(`exporter:${e}`));
    });
    activeComponents = components.filter((c) => relevantNames.has(`${c.category}:${c.name}`));
  }

  const receivers = activeComponents.filter((c) => c.category === 'receiver');
  const processors = activeComponents.filter((c) => c.category === 'processor');
  const exporters = activeComponents.filter((c) => c.category === 'exporter');

  const maxRows = Math.max(receivers.length, processors.length, exporters.length, 1);
  const rowHeight = 140;
  const canvasHeight = Math.max(540, maxRows * rowHeight + 160);
  const canvasWidth = 1180;

  // X Coordinates for the 3 hop tiers: Receivers -> Processors -> Targets
  const colX = {
    receiver: 180,
    processor: 590,
    exporter: 1000,
  };

  const nodes: GraphNode[] = [];
  const nodeMap = new Map<string, GraphNode>();

  const placeColumn = (items: OTelComponent[], colName: 'receiver' | 'processor' | 'exporter', colIndex: number) => {
    const count = items.length;
    if (count === 0) return;
    const totalSpan = (count - 1) * rowHeight;
    const startY = (canvasHeight - totalSpan) / 2;

    items.forEach((comp, idx) => {
      const node: GraphNode = {
        id: `${comp.category}:${comp.name}`,
        category: comp.category,
        component: comp,
        x: colX[colName],
        y: count === 1 ? canvasHeight / 2 : startY + idx * rowHeight,
        column: colIndex,
      };
      nodes.push(node);
      nodeMap.set(node.id, node);
    });
  };

  placeColumn(receivers, 'receiver', 0);
  placeColumn(processors, 'processor', 1);
  placeColumn(exporters, 'exporter', 2);

  // Generate curved Bézier links connecting Receiver -> Processor -> Exporter
  const links: GraphLink[] = [];
  let linkCounter = 0;

  pipelines.forEach((pipe) => {
    const pipeReceivers = pipe.receivers.map((r) => nodeMap.get(`receiver:${r}`)).filter(Boolean) as GraphNode[];
    const pipeProcessors = pipe.processors.map((p) => nodeMap.get(`processor:${p}`)).filter(Boolean) as GraphNode[];
    const pipeExporters = pipe.exporters.map((e) => nodeMap.get(`exporter:${e}`)).filter(Boolean) as GraphNode[];

    const nodeRadius = 30; // circular badge radius

    if (pipeProcessors.length > 0) {
      // Receiver -> First Processor
      pipeReceivers.forEach((rx) => {
        const firstProc = pipeProcessors[0];
        const sx = rx.x + nodeRadius;
        const sy = rx.y;
        const tx = firstProc.x - nodeRadius;
        const ty = firstProc.y;
        const cx1 = sx + (tx - sx) * 0.5;
        const cy1 = sy;
        const cx2 = sx + (tx - sx) * 0.5;
        const cy2 = ty;
        const pathD = `M ${sx} ${sy} C ${cx1} ${cy1}, ${cx2} ${cy2}, ${tx} ${ty}`;
        const midX = (sx + tx) / 2;
        const midY = (sy + ty) / 2;

        links.push({
          id: `link-${linkCounter++}`,
          pipelineId: pipe.id,
          pipelineName: pipe.name,
          signal: pipe.signal,
          sourceNodeId: rx.id,
          targetNodeId: firstProc.id,
          sourceX: sx,
          sourceY: sy,
          targetX: tx,
          targetY: ty,
          pathD,
          badgeLabel: rx.component.interval || (pipe.signal === 'logs' ? 'filelog' : 'telemetry'),
          midX,
          midY,
        });
      });

      // Sequential Processors (if more than 1)
      for (let pIdx = 0; pIdx < pipeProcessors.length - 1; pIdx++) {
        const p1 = pipeProcessors[pIdx];
        const p2 = pipeProcessors[pIdx + 1];
        const sx = p1.x + nodeRadius;
        const sy = p1.y;
        const tx = p2.x - nodeRadius;
        const ty = p2.y;
        const cx1 = sx + (tx - sx) * 0.5;
        const cy1 = sy;
        const cx2 = sx + (tx - sx) * 0.5;
        const cy2 = ty;
        const pathD = `M ${sx} ${sy} C ${cx1} ${cy1}, ${cx2} ${cy2}, ${tx} ${ty}`;

        links.push({
          id: `link-${linkCounter++}`,
          pipelineId: pipe.id,
          pipelineName: pipe.name,
          signal: pipe.signal,
          sourceNodeId: p1.id,
          targetNodeId: p2.id,
          sourceX: sx,
          sourceY: sy,
          targetX: tx,
          targetY: ty,
          pathD,
          badgeLabel: 'chain',
          midX: (sx + tx) / 2,
          midY: (sy + ty) / 2,
        });
      }

      // Last Processor -> Exporters
      const lastProc = pipeProcessors[pipeProcessors.length - 1];
      pipeExporters.forEach((ex) => {
        const sx = lastProc.x + nodeRadius;
        const sy = lastProc.y;
        const tx = ex.x - nodeRadius;
        const ty = ex.y;
        const cx1 = sx + (tx - sx) * 0.5;
        const cy1 = sy;
        const cx2 = sx + (tx - sx) * 0.5;
        const cy2 = ty;
        const pathD = `M ${sx} ${sy} C ${cx1} ${cy1}, ${cx2} ${cy2}, ${tx} ${ty}`;
        const midX = (sx + tx) / 2;
        const midY = (sy + ty) / 2;

        let badge = ex.component.endpoint || ex.component.type;
        if (badge && badge.includes(':')) {
          const spl = badge.split(':');
          badge = `:${spl[spl.length - 1]}`;
        }

        links.push({
          id: `link-${linkCounter++}`,
          pipelineId: pipe.id,
          pipelineName: pipe.name,
          signal: pipe.signal,
          sourceNodeId: lastProc.id,
          targetNodeId: ex.id,
          sourceX: sx,
          sourceY: sy,
          targetX: tx,
          targetY: ty,
          pathD,
          badgeLabel: badge,
          midX,
          midY,
        });
      });
    } else {
      // Direct Receiver -> Exporters (no processors)
      pipeReceivers.forEach((rx) => {
        pipeExporters.forEach((ex) => {
          const sx = rx.x + nodeRadius;
          const sy = rx.y;
          const tx = ex.x - nodeRadius;
          const ty = ex.y;
          const cx1 = sx + (tx - sx) * 0.5;
          const cy1 = sy;
          const cx2 = sx + (tx - sx) * 0.5;
          const cy2 = ty;
          const pathD = `M ${sx} ${sy} C ${cx1} ${cy1}, ${cx2} ${cy2}, ${tx} ${ty}`;
          const midX = (sx + tx) / 2;
          const midY = (sy + ty) / 2;

          links.push({
            id: `link-${linkCounter++}`,
            pipelineId: pipe.id,
            pipelineName: pipe.name,
            signal: pipe.signal,
            sourceNodeId: rx.id,
            targetNodeId: ex.id,
            sourceX: sx,
            sourceY: sy,
            targetX: tx,
            targetY: ty,
            pathD,
            badgeLabel: ex.component.endpoint || pipe.signal,
            midX,
            midY,
          });
        });
      });
    }
  });

  return {
    nodes,
    links,
    canvasWidth,
    canvasHeight,
  };
});

// Helper for component icons
const getComponentIcon = (comp: OTelComponent) => {
  const t = comp.type.toLowerCase();
  const name = comp.name.toLowerCase();

  if (comp.category === 'receiver') {
    if (t.includes('docker')) return Box;
    if (t.includes('host') || t.includes('metric')) return Server;
    if (t.includes('file') || t.includes('log')) return FileText;
    return Radio;
  }
  if (comp.category === 'processor') {
    if (t.includes('batch')) return Layers;
    if (t.includes('attr')) return Sliders;
    return Cpu;
  }
  if (comp.category === 'exporter') {
    if (t.includes('prometheus') || name.includes('prometheus')) return Flame;
    if (t.includes('otlp') || name.includes('opensearch')) return Globe;
    return Database;
  }
  return Server;
};

// ==================== CANVAS INTERACTION HELPERS ====================
const handleZoomIn = () => {
  zoomLevel.value = Math.min(zoomLevel.value + 0.15, 2.5);
};
const handleZoomOut = () => {
  zoomLevel.value = Math.max(zoomLevel.value - 0.15, 0.4);
};
const handleResetZoom = () => {
  zoomLevel.value = 1;
  panX.value = 0;
  panY.value = 0;
};

const startPan = (e: MouseEvent) => {
  if (e.button !== 0) return;
  // If clicked directly on canvas background
  const target = e.target as HTMLElement;
  if (target.tagName === 'svg' || target.id === 'canvas-bg' || target.classList.contains('canvas-area')) {
    isPanning.value = true;
    panStart.value = { x: e.clientX - panX.value, y: e.clientY - panY.value };
  }
};

const doPan = (e: MouseEvent) => {
  if (!isPanning.value) return;
  panX.value = e.clientX - panStart.value.x;
  panY.value = e.clientY - panStart.value.y;
};

const endPan = () => {
  isPanning.value = false;
};

// Node inspector & highlights
const inspectNode = (comp: OTelComponent) => {
  inspectedNode.value = comp;
};

const closeInspector = () => {
  inspectedNode.value = null;
};

const copyComponentSnippet = async () => {
  if (!inspectedNode.value) return;
  try {
    await navigator.clipboard.writeText(inspectedNode.value.rawSnippet);
    copiedYaml.value = true;
    setTimeout(() => {
      copiedYaml.value = false;
    }, 2000);
  } catch (err) {
    console.error('Failed to copy yaml snippet:', err);
  }
};

// ==================== API ACTIONS ====================
const fetchHosts = async (autoSelect = false) => {
  loadingHosts.value = true;
  try {
    const res = await axios.get('/api/v1/otel/hosts?checkStatus=true');
    if (res.data?.success && Array.isArray(res.data.data)) {
      hosts.value = res.data.data;
      if (hosts.value.length > 0) {
        if (!selectedHostId.value || !hosts.value.some((h) => h.id === selectedHostId.value)) {
          if (autoSelect || !selectedHostId.value) {
            selectHost(hosts.value[0].id);
          }
        }
      }
    }
  } catch (err: any) {
    setFeedback('error', 'Failed to load OTel fleet hosts', err.response?.data?.error || err.message);
  } finally {
    loadingHosts.value = false;
  }
};

const selectHost = async (hostId: string) => {
  selectedHostId.value = hostId;
  showHostDropdown.value = false;
  validationResult.value = null;
  await loadHostConfig(hostId);
};

const loadHostConfig = async (hostId: string) => {
  loadingConfig.value = true;
  try {
    const res = await axios.get(`/api/v1/otel/hosts/${hostId}/config`);
    if (res.data?.success && res.data.data) {
      yamlContent.value = res.data.data;
      initialLoadedContent.value = res.data.data;
      setFeedback('success', 'Host Config Loaded', `Successfully loaded remote config from ${selectedHost.value?.name || 'host'}.`);
    } else {
      // Fallback to sample template if host file is empty
      yamlContent.value = DEFAULT_SAMPLE_CONFIG;
      initialLoadedContent.value = DEFAULT_SAMPLE_CONFIG;
    }
  } catch (err: any) {
    setFeedback('error', 'Failed to fetch remote config', err.response?.data?.error || err.message);
  } finally {
    loadingConfig.value = false;
  }
};

const handleRefresh = async () => {
  await fetchHosts(false);
  if (selectedHostId.value) {
    await loadHostConfig(selectedHostId.value);
  }
  setFeedback('info', 'Refreshed', 'Fleet agent status and configuration refreshed.');
};

const validateRemoteConfig = async () => {
  if (!selectedHost.value) {
    setFeedback('error', 'No Host Selected', 'Please select a remote host to run config validation.');
    return;
  }

  if (!yamlSyntaxCheck.value.valid) {
    setFeedback('error', 'Syntax Error', yamlSyntaxCheck.value.error || 'Please fix YAML syntax errors before validating.');
    return;
  }

  validating.value = true;
  validationResult.value = null;

  try {
    const res = await axios.post(`/api/v1/otel/hosts/${selectedHost.value.id}/validate`, {
      content: yamlContent.value,
    });

    if (res.data?.success && res.data.data) {
      validationResult.value = res.data.data;
      if (res.data.data.valid) {
        setFeedback('success', 'Configuration Valid', 'OpenTelemetry Collector validated the configuration syntax successfully.');
      } else {
        setFeedback('error', 'Validation Failed', res.data.data.error || 'Syntax errors detected.');
      }
    } else {
      setFeedback('error', 'Validation Failed', res.data?.error || 'Validation error');
    }
  } catch (err: any) {
    setFeedback('error', 'Validation Error', err.response?.data?.error || err.message);
  } finally {
    validating.value = false;
  }
};

const deployConfig = async () => {
  if (!selectedHost.value) {
    setFeedback('error', 'No Host Selected', 'Please select a target host before deploying configuration.');
    return;
  }

  if (!yamlSyntaxCheck.value.valid) {
    setFeedback('error', 'Syntax Error', yamlSyntaxCheck.value.error || 'Cannot deploy invalid YAML.');
    return;
  }

  saving.value = true;
  try {
    const res = await axios.post(`/api/v1/otel/hosts/${selectedHost.value.id}/config`, {
      content: yamlContent.value,
      summary: configSummary.value || 'Deployed from Visual Config OpenTelemetry',
      restartAfter: true,
      restart: true,
    });

    if (res.data?.success) {
      initialLoadedContent.value = yamlContent.value;
      configSummary.value = '';
      setFeedback('success', 'Configuration Deployed', 'Configuration was saved to the remote host and agent was restarted.');
      if (selectedHost.value) {
        selectedHost.value.lastStatus = 'active';
      }
    } else {
      setFeedback('error', 'Deployment Failed', res.data?.error || 'Failed to save config.');
    }
  } catch (err: any) {
    setFeedback('error', 'Deployment Error', err.response?.data?.error || err.message);
  } finally {
    saving.value = false;
  }
};

const restartService = async () => {
  if (!selectedHost.value) return;
  restartingService.value = true;
  try {
    const res = await axios.post(`/api/v1/otel/hosts/${selectedHost.value.id}/restart`, {
      mode: 'restart',
    });
    if (res.data?.success) {
      setFeedback('success', 'Service Restarted', `OpenTelemetry agent on ${selectedHost.value.name} restarted successfully.`);
      selectedHost.value.lastStatus = 'active';
    } else {
      setFeedback('error', 'Restart Failed', res.data?.data?.error || 'Failed to restart remote agent.');
    }
  } catch (err: any) {
    setFeedback('error', 'Restart Error', err.response?.data?.error || err.message);
  } finally {
    restartingService.value = false;
  }
};

// ==================== TEMPLATE & CONFIRMATION HELPERS ====================
const promptApplyTemplate = (templateContent: string, templateTitle: string) => {
  confirmTitle.value = `Load ${templateTitle}?`;
  confirmDescription.value = 'This will replace the current visual topology and YAML editor with this template. Any unsaved changes will be overwritten.';
  confirmAction.value = () => {
    yamlContent.value = templateContent;
    showTemplateModal.value = false;
    showConfirmModal.value = false;
    setFeedback('info', 'Template Loaded', `${templateTitle} has been loaded into the visual topology.`);
  };
  showConfirmModal.value = true;
};

const executeConfirm = () => {
  if (confirmAction.value) {
    confirmAction.value();
    confirmAction.value = null;
  }
};

// History modal helpers
const openHistory = async () => {
  if (!selectedHost.value) {
    setFeedback('info', 'Host Required', 'Select a host to view past deployment history.');
    return;
  }
  showHistoryModal.value = true;
  loadingHistory.value = true;
  previewHistoryItem.value = null;
  try {
    const res = await axios.get(`/api/v1/otel/hosts/${selectedHost.value.id}/history`);
    if (res.data?.success && Array.isArray(res.data.data)) {
      historyList.value = res.data.data;
      if (historyList.value.length > 0) {
        previewHistoryItem.value = historyList.value[0];
      }
    } else {
      historyList.value = [];
    }
  } catch {
    historyList.value = [];
  } finally {
    loadingHistory.value = false;
  }
};

const restoreHistoryVersion = (item: OTelHistoryItem) => {
  confirmTitle.value = 'Rollback to Historical Version?';
  confirmDescription.value = `Are you sure you want to revert to the configuration from ${new Date(item.createdAt).toLocaleString()}?`;
  confirmAction.value = () => {
    yamlContent.value = item.content;
    showHistoryModal.value = false;
    showConfirmModal.value = false;
    setFeedback('success', 'Version Restored', 'Past configuration restored into editor. Click Save & Deploy to apply.');
  };
  showConfirmModal.value = true;
};

// Editor helpers
const handleEditorKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Tab') {
    e.preventDefault();
    const textarea = editorRef.value;
    if (!textarea) return;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const spaces = '  ';
    yamlContent.value = yamlContent.value.substring(0, start) + spaces + yamlContent.value.substring(end);
    nextTick(() => {
      textarea.selectionStart = textarea.selectionEnd = start + spaces.length;
    });
  }
};

onMounted(async () => {
  await fetchHosts(true);
});
</script>

<template>
  <!-- Root Container adhering strictly to AGENTS.md (No double padding) -->
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header Section (Standard HCP Header: Clean plain text <h1>, no icons or badges inside <h1>) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Visual Config OpenTelemetry (Beta)
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Interactive visual pipeline topology and remote configuration manager for OpenTelemetry Collector.
        </p>
      </div>

      <!-- Action Toolbar (Neutral monochrome buttons, semantic colors only for critical status) -->
      <div class="flex flex-wrap items-center gap-2 shrink-0">
        <!-- Host Selector Dropdown -->
        <div class="relative">
          <button
            @click="showHostDropdown = !showHostDropdown"
            class="flex items-center gap-2 px-3 py-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] hover:border-slate-300 dark:hover:border-slate-600 rounded-lg text-xs font-medium text-slate-800 dark:text-slate-200 transition shadow-xs cursor-pointer"
          >
            <!-- Critical Semantic Status Indicator -->
            <span
              class="w-2 h-2 rounded-full shrink-0"
              :class="{
                'bg-emerald-500': selectedHost?.lastStatus === 'active',
                'bg-rose-500': selectedHost?.lastStatus === 'failed',
                'bg-amber-500': selectedHost?.lastStatus === 'unreachable' || selectedHost?.lastStatus === 'inactive',
                'bg-slate-400': !selectedHost || selectedHost?.lastStatus === 'unknown',
              }"
            />
            <span class="truncate max-w-[140px] sm:max-w-[180px]">
              {{ selectedHost?.name || 'Select Host...' }}
            </span>
            <ChevronDown class="w-3.5 h-3.5 text-slate-400 shrink-0" />
          </button>

          <!-- Host Options Popover -->
          <div
            v-if="showHostDropdown"
            class="absolute left-0 sm:right-0 sm:left-auto mt-1 w-64 bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-xl shadow-xl z-50 p-2 space-y-1 animate-in fade-in"
          >
            <div class="px-2 py-1 text-[11px] font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">
              OTel Fleet Agents ({{ hosts.length }})
            </div>
            <div class="max-h-56 overflow-y-auto space-y-0.5">
              <button
                v-for="h in hosts"
                :key="h.id"
                @click="selectHost(h.id)"
                class="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs text-left transition hover:bg-slate-100 dark:hover:bg-[#192133] cursor-pointer"
                :class="selectedHostId === h.id ? 'bg-blue-50/70 dark:bg-blue-950/40 text-blue-700 dark:text-blue-300 font-semibold' : 'text-slate-700 dark:text-slate-300'"
              >
                <div class="flex items-center gap-2 truncate">
                  <span
                    class="w-2 h-2 rounded-full shrink-0"
                    :class="{
                      'bg-emerald-500': h.lastStatus === 'active',
                      'bg-rose-500': h.lastStatus === 'failed',
                      'bg-amber-500': h.lastStatus === 'unreachable' || h.lastStatus === 'inactive',
                      'bg-slate-400': h.lastStatus === 'unknown' || !h.lastStatus,
                    }"
                  />
                  <div class="truncate">
                    <div class="truncate text-xs">{{ h.name }}</div>
                    <div class="text-[10px] text-slate-400 font-mono">{{ h.sshHost }}:{{ h.sshPort }}</div>
                  </div>
                </div>
                <span
                  class="text-[9px] px-1.5 py-0.5 rounded font-mono uppercase"
                  :class="{
                    'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400': h.lastStatus === 'active',
                    'bg-rose-500/10 text-rose-600 dark:text-rose-400': h.lastStatus === 'failed',
                    'bg-slate-100 dark:bg-slate-800 text-slate-500': h.lastStatus !== 'active' && h.lastStatus !== 'failed',
                  }"
                >
                  {{ h.lastStatus || 'idle' }}
                </span>
              </button>
              <div v-if="hosts.length === 0" class="px-3 py-3 text-center text-xs text-slate-400">
                No remote hosts registered.
              </div>
            </div>
          </div>
        </div>

        <!-- Refresh Button -->
        <button
          @click="handleRefresh"
          :disabled="loadingHosts || loadingConfig"
          title="Refresh Fleet Status"
          class="p-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#1a2336] rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white transition cursor-pointer disabled:opacity-50"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loadingHosts || loadingConfig }" />
        </button>

        <!-- Template Selector -->
        <button
          @click="showTemplateModal = true"
          title="Load Pipeline Templates"
          class="flex items-center gap-1.5 px-2.5 py-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#1a2336] rounded-lg text-xs font-medium text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white transition cursor-pointer"
        >
          <Sparkles class="w-3.5 h-3.5 text-slate-400" />
          <span class="hidden sm:inline">Templates</span>
        </button>

        <!-- History Button -->
        <button
          @click="openHistory"
          title="Deployment History"
          class="flex items-center gap-1.5 px-2.5 py-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#1a2336] rounded-lg text-xs font-medium text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white transition cursor-pointer"
        >
          <History class="w-3.5 h-3.5 text-slate-400" />
          <span class="hidden sm:inline">History</span>
        </button>

        <!-- Validate Button -->
        <button
          v-if="canManage"
          @click="validateRemoteConfig"
          :disabled="validating || !selectedHost"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#1a2336] rounded-lg text-xs font-semibold text-slate-700 dark:text-slate-300 transition cursor-pointer disabled:opacity-50"
        >
          <CheckCircle2 class="w-3.5 h-3.5 text-slate-400" :class="{ 'animate-spin': validating }" />
          <span>{{ validating ? 'Validating...' : 'Validate' }}</span>
        </button>

        <!-- Save & Deploy Button -->
        <button
          v-if="canManage"
          @click="deployConfig"
          :disabled="saving || !selectedHost"
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shadow-xs cursor-pointer disabled:opacity-50"
        >
          <Save class="w-3.5 h-3.5" :class="{ 'animate-spin': saving }" />
          <span>{{ saving ? 'Deploying...' : 'Save & Deploy' }}</span>
        </button>

        <!-- Restart Service Button -->
        <button
          v-if="canManage"
          @click="restartService"
          :disabled="restartingService || !selectedHost"
          title="Restart remote OpenTelemetry collector daemon"
          class="p-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] hover:bg-slate-50 dark:hover:bg-[#1a2336] rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white transition cursor-pointer disabled:opacity-50"
        >
          <Play class="w-4 h-4" :class="{ 'animate-spin': restartingService }" />
        </button>
      </div>
    </div>

    <!-- Notification Toast / Auto-Dismiss Banner (3000ms per AGENTS.md) -->
    <div
      v-if="feedbackMsg"
      class="p-3 rounded-xl border flex items-center justify-between gap-3 text-xs animate-in fade-in"
      :class="{
        'bg-emerald-500/10 border-emerald-500/30 text-emerald-800 dark:text-emerald-300': feedbackMsg.type === 'success',
        'bg-rose-500/10 border-rose-500/30 text-rose-800 dark:text-rose-300': feedbackMsg.type === 'error',
        'bg-blue-500/10 border-blue-500/30 text-blue-800 dark:text-blue-300': feedbackMsg.type === 'info',
      }"
    >
      <div class="flex items-center gap-2">
        <span class="font-bold">{{ feedbackMsg.title }}:</span>
        <span>{{ feedbackMsg.detail }}</span>
      </div>
      <button @click="feedbackMsg = null" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- View Mode Switcher & Pipeline Filter Tabs Toolbar -->
    <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-3 bg-white dark:bg-[#0e1320] p-3 rounded-2xl border border-slate-200 dark:border-[#1b2234] shadow-xs">
      <!-- Left: Pipeline Filter Tabs -->
      <div class="flex flex-wrap items-center gap-1.5">
        <span class="text-xs font-semibold text-slate-500 dark:text-slate-400 mr-1.5 flex items-center gap-1">
          <Layers class="w-3.5 h-3.5 text-slate-400" />
          <span>Pipelines:</span>
        </span>

        <!-- 'All' Tab -->
        <button
          @click="selectedPipelineFilter = 'all'"
          class="px-2.5 py-1 rounded-lg text-xs font-medium transition cursor-pointer"
          :class="
            selectedPipelineFilter === 'all'
              ? 'bg-slate-900 dark:bg-white text-white dark:text-slate-900 font-bold shadow-xs'
              : 'bg-slate-100 dark:bg-[#161d2d] text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-[#1f283d]'
          "
        >
          All ({{ availablePipelines.length }})
        </button>

        <!-- Pipeline Specific Tabs with Signal Badges -->
        <button
          v-for="p in availablePipelines"
          :key="p.id"
          @click="selectedPipelineFilter = p.id"
          class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium transition cursor-pointer"
          :class="
            selectedPipelineFilter === p.id
              ? 'bg-blue-600 text-white font-bold shadow-xs'
              : 'bg-slate-100 dark:bg-[#161d2d] text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-[#1f283d]'
          "
        >
          <span>{{ p.name }}</span>
          <span
            class="text-[9px] px-1 py-0.2 rounded font-mono uppercase tracking-wider"
            :class="
              selectedPipelineFilter === p.id
                ? 'bg-white/20 text-white'
                : 'bg-slate-200 dark:bg-slate-800 text-slate-500 dark:text-slate-400'
            "
          >
            {{ p.signal }}
          </span>
        </button>
      </div>

      <!-- Right: View Mode Toggle & Canvas Controls -->
      <div class="flex items-center gap-2 self-end lg:self-auto shrink-0">
        <!-- Live Particles Animated Toggle -->
        <button
          v-if="viewMode !== 'code'"
          @click="animatedFlow = !animatedFlow"
          :title="animatedFlow ? 'Disable Flow Animation' : 'Enable Flow Animation'"
          class="flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium border border-slate-200 dark:border-[#1b2234] transition cursor-pointer"
          :class="animatedFlow ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20' : 'text-slate-500 hover:text-slate-700'"
        >
          <span class="w-1.5 h-1.5 rounded-full" :class="animatedFlow ? 'bg-emerald-500 animate-ping' : 'bg-slate-400'" />
          <span class="text-[11px]">Flow</span>
        </button>

        <!-- Zoom Controls (for Visual Mode) -->
        <div v-if="viewMode !== 'code'" class="flex items-center border border-slate-200 dark:border-[#1b2234] rounded-lg overflow-hidden bg-slate-50 dark:bg-[#121826]">
          <button @click="handleZoomOut" title="Zoom Out" class="p-1 hover:bg-slate-200 dark:hover:bg-[#1a2336] text-slate-500 cursor-pointer">
            <ZoomOut class="w-3.5 h-3.5" />
          </button>
          <button @click="handleResetZoom" title="Reset View" class="px-1.5 text-[10px] font-mono text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-[#1a2336] cursor-pointer">
            {{ Math.round(zoomLevel * 100) }}%
          </button>
          <button @click="handleZoomIn" title="Zoom In" class="p-1 hover:bg-slate-200 dark:hover:bg-[#1a2336] text-slate-500 cursor-pointer">
            <ZoomIn class="w-3.5 h-3.5" />
          </button>
        </div>

        <!-- Mode Switcher -->
        <div class="flex items-center bg-slate-100 dark:bg-[#161d2d] p-0.5 rounded-lg border border-slate-200 dark:border-[#1b2234]">
          <button
            @click="viewMode = 'visual'"
            title="Visual Topology Graph"
            class="px-2.5 py-1 rounded-md text-xs font-medium transition cursor-pointer flex items-center gap-1"
            :class="viewMode === 'visual' ? 'bg-white dark:bg-[#0c101a] text-slate-900 dark:text-white font-bold shadow-xs' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800'"
          >
            <Radio class="w-3.5 h-3.5" />
            <span class="hidden sm:inline">Topology</span>
          </button>

          <button
            @click="viewMode = 'split'"
            title="Split: Topology & Code Editor"
            class="px-2.5 py-1 rounded-md text-xs font-medium transition cursor-pointer flex items-center gap-1"
            :class="viewMode === 'split' ? 'bg-white dark:bg-[#0c101a] text-slate-900 dark:text-white font-bold shadow-xs' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800'"
          >
            <Columns class="w-3.5 h-3.5" />
            <span class="hidden sm:inline">Split</span>
          </button>

          <button
            @click="viewMode = 'code'"
            title="Raw YAML Code Editor"
            class="px-2.5 py-1 rounded-md text-xs font-medium transition cursor-pointer flex items-center gap-1"
            :class="viewMode === 'code' ? 'bg-white dark:bg-[#0c101a] text-slate-900 dark:text-white font-bold shadow-xs' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800'"
          >
            <FileCode class="w-3.5 h-3.5" />
            <span class="hidden sm:inline">YAML Editor</span>
          </button>
        </div>
      </div>
    </div>

    <!-- MAIN WORKSPACE: Visual Topology / Split / Code -->
    <div
      class="grid gap-6 transition-all"
      :class="{
        'grid-cols-1': viewMode === 'visual' || viewMode === 'code',
        'grid-cols-1 lg:grid-cols-12': viewMode === 'split',
      }"
    >
      <!-- LEFT/TOP: Interactive Visual Graph Topology Canvas -->
      <div
        v-show="viewMode === 'visual' || viewMode === 'split'"
        class="bg-white dark:bg-[#0a0e17] border border-slate-200 dark:border-[#1b2234] rounded-2xl overflow-hidden shadow-sm relative flex flex-col"
        :class="{
          'h-[640px]': viewMode === 'visual',
          'h-[580px] lg:col-span-7': viewMode === 'split',
        }"
      >
        <!-- Canvas Stage Legend & Metrics Header -->
        <div class="px-4 py-2.5 bg-slate-50/70 dark:bg-[#0f1422] border-b border-slate-200 dark:border-[#1b2234] flex items-center justify-between text-xs z-10 shrink-0">
          <div class="flex items-center gap-4 text-slate-500 dark:text-slate-400">
            <div class="flex items-center gap-1.5">
              <span class="w-2.5 h-2.5 rounded-full bg-emerald-500" />
              <span class="font-semibold text-slate-700 dark:text-slate-300">Receivers ({{ graphData.nodes.filter(n => n.category === 'receiver').length }})</span>
            </div>
            <ArrowRight class="w-3 h-3 text-slate-400" />
            <div class="flex items-center gap-1.5">
              <span class="w-2.5 h-2.5 rounded-full bg-blue-500" />
              <span class="font-semibold text-slate-700 dark:text-slate-300">Processors ({{ graphData.nodes.filter(n => n.category === 'processor').length }})</span>
            </div>
            <ArrowRight class="w-3 h-3 text-slate-400" />
            <div class="flex items-center gap-1.5">
              <span class="w-2.5 h-2.5 rounded-full bg-amber-500" />
              <span class="font-semibold text-slate-700 dark:text-slate-300">Targets / Exporters ({{ graphData.nodes.filter(n => n.category === 'exporter').length }})</span>
            </div>
          </div>

          <div class="text-[11px] text-slate-400">
            <span>{{ graphData.links.length }} active links</span>
          </div>
        </div>

        <!-- SVG Interactive Canvas Area -->
        <div
          id="canvas-bg"
          class="canvas-area flex-1 relative overflow-hidden select-none cursor-grab active:cursor-grabbing bg-slate-50/40 dark:bg-[#080c14]"
          @mousedown="startPan"
          @mousemove="doPan"
          @mouseup="endPan"
          @mouseleave="endPan"
        >
          <!-- Grid Background Pattern -->
          <svg class="absolute inset-0 w-full h-full pointer-events-none" xmlns="http://www.w3.org/2000/svg">
            <defs>
              <pattern id="grid-dots" width="28" height="28" patternUnits="userSpaceOnUse">
                <circle cx="2" cy="2" r="1" class="fill-slate-300 dark:fill-[#1b2336]" />
              </pattern>
            </defs>
            <rect width="100%" height="100%" fill="url(#grid-dots)" />
          </svg>

          <!-- Scalable & Pannable Graph Group -->
          <svg
            class="w-full h-full overflow-visible"
            :viewBox="`0 0 ${graphData.canvasWidth} ${graphData.canvasHeight}`"
            :style="{
              transform: `translate(${panX}px, ${panY}px) scale(${zoomLevel})`,
              transformOrigin: '50% 50%',
              transition: isPanning ? 'none' : 'transform 0.15s ease-out',
            }"
          >
            <defs>
              <!-- Glowing shadow filter for nodes -->
              <filter id="glow-emerald" x="-30%" y="-30%" width="160%" height="160%">
                <feDropShadow dx="0" dy="0" stdDeviation="6" flood-color="#10b981" flood-opacity="0.45" />
              </filter>
              <filter id="glow-blue" x="-30%" y="-30%" width="160%" height="160%">
                <feDropShadow dx="0" dy="0" stdDeviation="6" flood-color="#3b82f6" flood-opacity="0.45" />
              </filter>
              <filter id="glow-amber" x="-30%" y="-30%" width="160%" height="160%">
                <feDropShadow dx="0" dy="0" stdDeviation="6" flood-color="#f59e0b" flood-opacity="0.45" />
              </filter>
            </defs>

            <!-- Column Header Labels inside Graph -->
            <g class="pointer-events-none select-none">
              <text x="180" y="45" text-anchor="middle" class="text-[12px] font-bold fill-slate-400 dark:fill-slate-500 uppercase tracking-widest">
                STAGE 1 • RECEIVERS
              </text>
              <text x="590" y="45" text-anchor="middle" class="text-[12px] font-bold fill-slate-400 dark:fill-slate-500 uppercase tracking-widest">
                STAGE 2 • PROCESSORS
              </text>
              <text x="1000" y="45" text-anchor="middle" class="text-[12px] font-bold fill-slate-400 dark:fill-slate-500 uppercase tracking-widest">
                STAGE 3 • TARGETS
              </text>
            </g>

            <!-- CONNECTING CURVED LINKS (Dashed Bézier curves matching the user's diagram) -->
            <g class="links-layer">
              <g
                v-for="link in graphData.links"
                :key="link.id"
                class="transition-opacity duration-200"
                :class="{
                  'opacity-100': !hoveredNodeId || hoveredNodeId === link.sourceNodeId || hoveredNodeId === link.targetNodeId,
                  'opacity-20': hoveredNodeId && hoveredNodeId !== link.sourceNodeId && hoveredNodeId !== link.targetNodeId,
                }"
              >
                <!-- Outer glow / background track -->
                <path
                  :d="link.pathD"
                  fill="none"
                  class="stroke-slate-200 dark:stroke-[#1b2336]"
                  stroke-width="5"
                  stroke-linecap="round"
                />

                <!-- Active dashed curved line (Green like the user's reference) -->
                <path
                  :d="link.pathD"
                  fill="none"
                  stroke-linecap="round"
                  stroke-width="2.5"
                  class="stroke-emerald-500 transition-all duration-300"
                  stroke-dasharray="7 5"
                />

                <!-- Animated flowing packet particles -->
                <circle
                  v-if="animatedFlow"
                  r="3.5"
                  class="fill-emerald-400 dark:fill-emerald-300"
                >
                  <animateMotion
                    :path="link.pathD"
                    dur="2.5s"
                    repeatCount="indefinite"
                  />
                </circle>

                <!-- Pill badge on curve showing metadata/latency/interval (just like the screenshot's '0.974 ms') -->
                <g
                  v-if="link.badgeLabel"
                  :transform="`translate(${link.midX}, ${link.midY})`"
                  class="cursor-pointer"
                >
                  <rect
                    x="-34"
                    y="-11"
                    width="68"
                    height="22"
                    rx="11"
                    ry="11"
                    class="fill-white dark:fill-[#121826] stroke-slate-200 dark:stroke-[#293681] shadow-xs"
                    stroke-width="1.2"
                  />
                  <text
                    x="0"
                    y="3.5"
                    text-anchor="middle"
                    class="text-[10px] font-bold fill-slate-700 dark:fill-slate-300 font-mono tracking-tight pointer-events-none"
                  >
                    {{ link.badgeLabel.length > 10 ? link.badgeLabel.slice(0, 9) + '..' : link.badgeLabel }}
                  </text>
                </g>
              </g>
            </g>

            <!-- NODES LAYER: Circular hop nodes matching reference image -->
            <g class="nodes-layer">
              <g
                v-for="node in graphData.nodes"
                :key="node.id"
                :transform="`translate(${node.x}, ${node.y})`"
                class="cursor-pointer group"
                @click="inspectNode(node.component)"
                @mouseenter="hoveredNodeId = node.id"
                @mouseleave="hoveredNodeId = null"
              >
                <!-- Category Tag Above Node (RECEIVER, PROCESSOR, TARGET) -->
                <text
                  x="0"
                  y="-42"
                  text-anchor="middle"
                  class="text-[10px] font-extrabold uppercase tracking-wider fill-slate-400 dark:fill-slate-500 transition group-hover:fill-blue-500"
                >
                  {{ node.category === 'exporter' ? 'TARGET' : node.category.toUpperCase() }}
                </text>

                <!-- Outer Glowing Circle -->
                <circle
                  r="32"
                  class="transition-all duration-200"
                  :class="{
                    'fill-emerald-500/10 stroke-emerald-500/40': node.category === 'receiver',
                    'fill-blue-500/10 stroke-blue-500/40': node.category === 'processor',
                    'fill-emerald-500/10 stroke-emerald-500/40': node.category === 'exporter',
                    'scale-110': hoveredNodeId === node.id || inspectedNode?.name === node.component.name,
                  }"
                  stroke-width="2"
                  :filter="node.category === 'processor' ? 'url(#glow-blue)' : 'url(#glow-emerald)'"
                />

                <!-- Core Circular Badge (Green/Emerald like in the reference image) -->
                <circle
                  r="24"
                  class="transition-all duration-200"
                  :class="{
                    'fill-emerald-600 dark:fill-emerald-500': node.category === 'receiver',
                    'fill-blue-600 dark:fill-blue-500': node.category === 'processor',
                    'fill-emerald-600 dark:fill-emerald-500': node.category === 'exporter',
                  }"
                />

                <!-- Icon rendered inside circle -->
                <foreignObject x="-10" y="-10" width="20" height="20" class="pointer-events-none">
                  <component
                    :is="getComponentIcon(node.component)"
                    :size="20"
                    class="text-white"
                  />
                </foreignObject>

                <!-- Label 1: Main Component Name Underneath -->
                <text
                  x="0"
                  y="48"
                  text-anchor="middle"
                  class="text-[12px] font-bold fill-slate-900 dark:fill-white tracking-tight"
                >
                  {{ node.component.name.length > 20 ? node.component.name.slice(0, 18) + '...' : node.component.name }}
                </text>

                <!-- Label 2: Key Sub-metadata (Endpoint, IP or Scrapers) -->
                <text
                  x="0"
                  y="64"
                  text-anchor="middle"
                  class="text-[10px] font-medium fill-slate-500 dark:fill-slate-400 font-mono"
                >
                  <tspan v-if="node.component.endpoint">
                    {{ node.component.endpoint.length > 22 ? node.component.endpoint.slice(0, 20) + '..' : node.component.endpoint }}
                  </tspan>
                  <tspan v-else-if="node.component.interval">
                    {{ node.component.interval }} interval
                  </tspan>
                  <tspan v-else-if="node.component.scrapers && node.component.scrapers.length > 0">
                    {{ node.component.scrapers.length }} scrapers
                  </tspan>
                  <tspan v-else>
                    {{ node.component.type }}
                  </tspan>
                </text>
              </g>
            </g>
          </svg>

          <!-- Floating Helper Note at Bottom of Canvas -->
          <div class="absolute bottom-3 left-3 pointer-events-none text-[11px] text-slate-400 dark:text-slate-500 flex items-center gap-1.5 bg-white/80 dark:bg-black/60 px-2.5 py-1 rounded-lg backdrop-blur-xs border border-slate-200/50 dark:border-slate-800/50">
            <Info class="w-3.5 h-3.5 text-slate-400" />
            <span>Click any node to inspect configuration snippet and pipeline links.</span>
          </div>
        </div>

        <!-- NODE INSPECTOR SLIDE-OVER DRAWER (Bottom or Right) -->
        <div
          v-if="inspectedNode"
          class="p-4 bg-white dark:bg-[#0f1422] border-t border-slate-200 dark:border-[#1b2234] z-20 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 animate-in slide-in-from-bottom-2"
        >
          <div class="space-y-1 min-w-0">
            <div class="flex items-center gap-2">
              <span
                class="text-[10px] font-extrabold uppercase px-2 py-0.5 rounded font-mono"
                :class="{
                  'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400': inspectedNode.category === 'receiver',
                  'bg-blue-500/10 text-blue-600 dark:text-blue-400': inspectedNode.category === 'processor',
                  'bg-amber-500/10 text-amber-600 dark:text-amber-400': inspectedNode.category === 'exporter',
                }"
              >
                {{ inspectedNode.category.toUpperCase() }}
              </span>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white truncate">
                {{ inspectedNode.name }}
              </h3>
              <span class="text-xs text-slate-400 font-mono">({{ inspectedNode.type }})</span>
            </div>

            <!-- Parameters preview -->
            <div class="flex flex-wrap items-center gap-3 text-xs text-slate-600 dark:text-slate-300">
              <span v-if="inspectedNode.endpoint" class="flex items-center gap-1 font-mono">
                <strong class="text-slate-400">Endpoint:</strong> {{ inspectedNode.endpoint }}
              </span>
              <span v-if="inspectedNode.interval" class="flex items-center gap-1 font-mono">
                <strong class="text-slate-400">Interval:</strong> {{ inspectedNode.interval }}
              </span>
              <span v-if="inspectedNode.scrapers && inspectedNode.scrapers.length" class="flex items-center gap-1">
                <strong class="text-slate-400">Scrapers:</strong> {{ inspectedNode.scrapers.join(', ') }}
              </span>
              <span v-if="inspectedNode.pipelines && inspectedNode.pipelines.length" class="flex items-center gap-1">
                <strong class="text-slate-400">Pipelines:</strong> {{ inspectedNode.pipelines.join(', ') }}
              </span>
            </div>
          </div>

          <!-- Inspector Action Buttons -->
          <div class="flex items-center gap-2 shrink-0">
            <button
              @click="copyComponentSnippet"
              class="flex items-center gap-1 px-3 py-1.5 bg-slate-100 dark:bg-[#1a2336] hover:bg-slate-200 dark:hover:bg-[#25324d] rounded-lg text-xs font-semibold text-slate-700 dark:text-slate-200 transition cursor-pointer"
            >
              <Check v-if="copiedYaml" class="w-3.5 h-3.5 text-emerald-500" />
              <Copy v-else class="w-3.5 h-3.5 text-slate-400" />
              <span>{{ copiedYaml ? 'Copied!' : 'Copy YAML' }}</span>
            </button>
            <button
              @click="closeInspector"
              class="p-1.5 hover:bg-slate-100 dark:hover:bg-[#1a2336] rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-white transition cursor-pointer"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      <!-- RIGHT/BOTTOM: Raw YAML Code Editor (for Split View & Code Editor Mode) -->
      <div
        v-show="viewMode === 'code' || viewMode === 'split'"
        class="bg-white dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-2xl overflow-hidden shadow-sm flex flex-col"
        :class="{
          'h-[640px]': viewMode === 'code',
          'h-[580px] lg:col-span-5': viewMode === 'split',
        }"
      >
        <!-- Editor Header -->
        <div class="px-4 py-2.5 bg-slate-50 dark:bg-[#0f1422] border-b border-slate-200 dark:border-[#1b2234] flex items-center justify-between text-xs shrink-0">
          <div class="flex items-center gap-2">
            <FileCode class="w-4 h-4 text-slate-400" />
            <span class="font-bold text-slate-800 dark:text-slate-200">OpenTelemetry YAML Configuration</span>
            <span v-if="hasUnsavedChanges" class="text-[9px] px-1.5 py-0.2 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 font-bold uppercase">
              Unsaved
            </span>
          </div>

          <div class="flex items-center gap-2">
            <span class="text-[11px] font-mono text-slate-400">
              {{ yamlContent.split('\n').length }} lines
            </span>
          </div>
        </div>

        <!-- Syntax Check Warning Banner -->
        <div
          v-if="!yamlSyntaxCheck.valid"
          class="px-3 py-2 bg-rose-500/10 border-b border-rose-500/20 text-rose-700 dark:text-rose-300 text-xs flex items-center justify-between"
        >
          <div class="flex items-center gap-1.5">
            <AlertTriangle class="w-3.5 h-3.5 text-rose-500 shrink-0" />
            <span>{{ yamlSyntaxCheck.error }}</span>
          </div>
        </div>

        <!-- Remote Validation Result Banner -->
        <div
          v-if="validationResult"
          class="px-3 py-2 text-xs border-b flex items-center justify-between"
          :class="validationResult.valid ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-800 dark:text-emerald-300' : 'bg-rose-500/10 border-rose-500/20 text-rose-800 dark:text-rose-300'"
        >
          <div class="flex items-center gap-1.5">
            <CheckCircle2 v-if="validationResult.valid" class="w-3.5 h-3.5 text-emerald-500 shrink-0" />
            <AlertTriangle v-else class="w-3.5 h-3.5 text-rose-500 shrink-0" />
            <span class="font-semibold">{{ validationResult.message }}</span>
          </div>
          <button @click="validationResult = null" class="text-slate-400 hover:text-slate-600">
            <X class="w-3.5 h-3.5" />
          </button>
        </div>

        <!-- Monospaced Code Textarea with Line Numbers -->
        <div class="flex-1 flex overflow-hidden font-mono text-xs relative bg-white dark:bg-[#0c101a]">
          <!-- Gutter / Line Numbers -->
          <div
            ref="gutterRef"
            class="w-11 py-3 bg-slate-50 dark:bg-[#0a0d15] text-slate-400 select-none text-right pr-2 shrink-0 border-r border-slate-200 dark:border-[#1b2234] overflow-hidden"
          >
            <div
              v-for="num in lineNumbers"
              :key="num"
              class="leading-5 h-5 text-[11px]"
              :class="{
                'bg-rose-500/20 text-rose-600 dark:text-rose-400 font-bold': yamlSyntaxCheck.line === num || validationResult?.line === num,
              }"
            >
              {{ num }}
            </div>
          </div>

          <!-- Textarea Editor with real-time reactive model updates -->
          <textarea
            ref="editorRef"
            v-model="yamlContent"
            @keydown="handleEditorKeyDown"
            class="flex-1 p-3 bg-transparent text-slate-800 dark:text-slate-100 font-mono text-xs leading-5 resize-none focus:outline-hidden overflow-auto whitespace-pre selection:bg-blue-500/20"
            placeholder="# Write or paste OpenTelemetry Collector configuration here..."
            spellcheck="false"
          />
        </div>
      </div>
    </div>

    <!-- PRESET TEMPLATES MODAL -->
    <div
      v-if="showTemplateModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
          <div class="flex items-center gap-2">
            <Sparkles class="w-4 h-4 text-slate-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">OpenTelemetry Collector Templates</h3>
          </div>
          <button @click="showTemplateModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3">
          <!-- Template 1: User's Complete Reference Template -->
          <div
            @click="promptApplyTemplate(DEFAULT_SAMPLE_CONFIG, 'Docker + Host + WordPress Pipeline')"
            class="p-3.5 rounded-xl border border-slate-200 dark:border-[#1f283d] hover:border-blue-500 dark:hover:border-blue-500 bg-slate-50 dark:bg-[#151b2a] hover:bg-blue-50/40 dark:hover:bg-blue-950/20 transition cursor-pointer space-y-1.5"
          >
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-slate-900 dark:text-white">
                Docker Containers, Host Metrics & WordPress Logs
              </span>
              <span class="text-[9px] px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-600 dark:text-blue-400 font-mono uppercase">
                4 Pipelines
              </span>
            </div>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              Complete production pipeline parsing hostmetrics, docker_stats, and wordpress JSON filelogs. Dual exporters to Prometheus (:8889, :8886) and OTLP OpenSearch (:4940, :4942).
            </p>
          </div>

          <!-- Template 2: Minimal Hostmetrics -->
          <div
            @click="promptApplyTemplate(MINIMAL_TEMPLATE, 'Basic Hostmetrics & Prometheus')"
            class="p-3.5 rounded-xl border border-slate-200 dark:border-[#1f283d] hover:border-blue-500 dark:hover:border-blue-500 bg-slate-50 dark:bg-[#151b2a] hover:bg-blue-50/40 dark:hover:bg-blue-950/20 transition cursor-pointer space-y-1.5"
          >
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-slate-900 dark:text-white">
                Basic Host Metrics & Prometheus Exporter
              </span>
              <span class="text-[9px] px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 text-slate-600 dark:text-slate-400 font-mono uppercase">
                1 Pipeline
              </span>
            </div>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              Clean starting point with CPU, memory, disk, and network scrapers batched and exposed on port 8889 for Prometheus scraping.
            </p>
          </div>
        </div>

        <div class="flex justify-end pt-2">
          <button
            @click="showTemplateModal = false"
            class="px-4 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>

    <!-- HISTORY & ROLLBACK MODAL -->
    <div
      v-if="showHistoryModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-3xl shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1f283d] pb-3">
          <div class="flex items-center gap-2">
            <History class="w-4 h-4 text-slate-400" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">
              Deployment History • {{ selectedHost?.name }}
            </h3>
          </div>
          <button @click="showHistoryModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div v-if="loadingHistory" class="py-12 text-center text-xs text-slate-400">
          <Loader2 class="w-6 h-6 animate-spin mx-auto text-blue-500 mb-2" />
          <span>Loading historical deployments...</span>
        </div>

        <div v-else-if="historyList.length === 0" class="py-12 text-center text-xs text-slate-400">
          No historical deployments recorded for this host yet.
        </div>

        <div v-else class="grid grid-cols-1 md:grid-cols-12 gap-4 max-h-[460px] overflow-hidden">
          <!-- Left list -->
          <div class="md:col-span-5 space-y-1.5 overflow-y-auto max-h-[420px] pr-1">
            <button
              v-for="item in historyList"
              :key="item.id"
              @click="previewHistoryItem = item"
              class="w-full text-left p-2.5 rounded-xl border text-xs transition cursor-pointer"
              :class="previewHistoryItem?.id === item.id ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/30' : 'border-slate-200 dark:border-[#1f283d] hover:bg-slate-50 dark:hover:bg-[#151b2a]'"
            >
              <div class="font-bold text-slate-800 dark:text-slate-200">
                {{ new Date(item.createdAt).toLocaleString() }}
              </div>
              <div class="text-[11px] text-slate-500 dark:text-slate-400 truncate mt-0.5">
                {{ item.changeSummary || 'Deployed configuration' }}
              </div>
            </button>
          </div>

          <!-- Right preview -->
          <div class="md:col-span-7 flex flex-col border border-slate-200 dark:border-[#1f283d] rounded-xl overflow-hidden bg-slate-50/50 dark:bg-[#0a0e17]">
            <div class="px-3 py-2 bg-slate-100 dark:bg-[#121826] border-b border-slate-200 dark:border-[#1f283d] flex items-center justify-between text-xs">
              <span class="font-semibold text-slate-700 dark:text-slate-300">Configuration Preview</span>
              <button
                v-if="previewHistoryItem"
                @click="restoreHistoryVersion(previewHistoryItem)"
                class="px-2.5 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded-md text-xs font-bold transition cursor-pointer"
              >
                Revert to this Version
              </button>
            </div>
            <pre class="p-3 text-xs font-mono overflow-auto flex-1 max-h-[380px] text-slate-800 dark:text-slate-200">{{ previewHistoryItem?.content }}</pre>
          </div>
        </div>

        <div class="flex justify-end pt-2">
          <button
            @click="showHistoryModal = false"
            class="px-4 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- STANDARD HCP CONFIRMATION MODAL (Strictly adheres to AGENTS.md, NO window.confirm) -->
    <div
      v-if="showConfirmModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-xs animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <!-- Center Icon Circle -->
        <div class="w-12 h-12 rounded-full bg-blue-500/10 text-blue-500 flex items-center justify-center mx-auto">
          <Info class="w-6 h-6" />
        </div>

        <!-- Title & Description -->
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            {{ confirmTitle }}
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            {{ confirmDescription }}
          </p>
        </div>

        <!-- Action Buttons -->
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showConfirmModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeConfirm"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer"
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
