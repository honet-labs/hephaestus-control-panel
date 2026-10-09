<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import {
  Check,
  Save,
  ExternalLink,
  Plus,
  AlertTriangle,
  AlertCircle,
  CheckCircle2,
  Trash2,
  Copy,
  Search,
  FileText,
  Layers,
  Globe,
  Bell,
  Play,
  FileDiff,
  Sliders,
  X,
  ChevronDown,
  ChevronRight,
  Code,
  RotateCw,
  Server
} from 'lucide-vue-next';

const router = useRouter();
const authStore = useAuthStore();
const canManage = computed(() => authStore.can('prometheus_config', 'manage'));

// ==================== INSTANCE STATE ====================
interface PrometheusInstance {
  id: string;
  name: string;
  path?: string;
  reloadUrl?: string;
  sshHost?: string;
  isActive?: boolean;
}

const instances = ref<PrometheusInstance[]>([]);
const selectedInstanceId = ref<string>('');
const configFilePath = ref('/etc/prometheus/prometheus.yml');
const isLoaded = ref(false);
const loading = ref(false);
const saving = ref(false);

// Notification banner
const notification = ref<{ type: 'success' | 'error' | 'warning'; message: string } | null>(null);
let notificationTimer: ReturnType<typeof setTimeout> | null = null;
const showNotification = (type: 'success' | 'error' | 'warning', message: string) => {
  if (notificationTimer) clearTimeout(notificationTimer);
  notification.value = { type, message };
  notificationTimer = setTimeout(() => {
    notification.value = null;
    notificationTimer = null;
  }, 3000);
};

// ==================== DATA MODELS ====================
interface TargetItem {
  id: string;
  endpoint: string;
  labels: Record<string, string>;
}

interface RelabelConfigItem {
  source_labels?: string[];
  separator?: string;
  target_label?: string;
  regex?: string;
  modulus?: number;
  replacement?: string;
  action?: 'replace' | 'keep' | 'drop' | 'hashmod' | 'labelmap' | 'labeldrop' | 'labelkeep';
}

interface ScrapeJob {
  id: string;
  job_name: string;
  metrics_path: string;
  scheme: 'http' | 'https';
  scrape_interval: string;
  scrape_timeout: string;
  targets: TargetItem[];
  labels: Record<string, string>;
  params?: Record<string, string[]>;
  relabel_configs?: RelabelConfigItem[];
  relabel_configs_count?: number;
  rawExtra?: string; // Preserve extra fields
}

interface GlobalSettings {
  scrape_interval: string;
  scrape_timeout: string;
  evaluation_interval: string;
  external_labels: Record<string, string>;
}

interface AlertmanagerSettings {
  scheme: string;
  timeout: string;
  endpoints: string[];
}

// Active Editor State
const viewMode = ref<'visual' | 'raw'>('visual');
const activeRightTab = ref<'preview' | 'diff' | 'history'>('preview');
const previewScope = ref<'full' | 'job'>('full');

// Filter & Selection
const searchJobQuery = ref('');
const selectedJobId = ref<string>('');
const activeJobTab = ref<'targets' | 'labels' | 'params' | 'relabel' | 'advanced'>('targets');

// Modals
const showAddJobModal = ref(false);
const showBulkAddModal = ref(false);
const bulkAddText = ref('');
const showDeleteJobModal = ref(false);
const jobToDelete = ref<ScrapeJob | null>(null);
const showDeleteTargetModal = ref(false);
const targetToDelete = ref<{ job: ScrapeJob; target: TargetItem } | null>(null);

// Accordion Toggles
const openAccordion = ref<'none' | 'global' | 'alertmanager' | 'rules'>('none');
const toggleAccordion = (name: 'global' | 'alertmanager' | 'rules') => {
  openAccordion.value = openAccordion.value === name ? 'none' : name;
};

// ==================== CONFIGURATION STATE ====================
const globalConfig = ref<GlobalSettings>({
  scrape_interval: '15s',
  scrape_timeout: '10s',
  evaluation_interval: '15s',
  external_labels: { monitor: 'hcp-monitor' }
});

const alertmanagerConfig = ref<AlertmanagerSettings>({
  scheme: 'http',
  timeout: '10s',
  endpoints: ['alertmanager:9093']
});

const ruleFiles = ref<string[]>([
  'first_rules.yml',
  'second_rules.yml'
]);

const scrapeJobs = ref<ScrapeJob[]>([
  {
    id: 'job-1',
    job_name: 'alertmanager',
    metrics_path: '/metrics',
    scheme: 'http',
    scrape_interval: '30s',
    scrape_timeout: '10s',
    targets: [
      { id: 't-1-1', endpoint: 'alertmanager:9093', labels: { app: 'alertmanager' } }
    ],
    labels: { app: 'alertmanager' }
  },
  {
    id: 'job-2',
    job_name: 'node-exporter',
    metrics_path: '/metrics',
    scheme: 'http',
    scrape_interval: '15s',
    scrape_timeout: '10s',
    targets: [
      { id: 't-2-1', endpoint: 'localhost:9100', labels: { env: 'prod', instance: 'node1' } },
      { id: 't-2-2', endpoint: 'localhost:9100', labels: { env: 'prod', instance: 'node2' } },
      { id: 't-2-3', endpoint: '192.168.1.10:9100', labels: { env: 'prod', instance: 'node3' } }
    ],
    labels: { app: 'node-exporter', env: 'prod' }
  },
  {
    id: 'job-3',
    job_name: 'prometheus',
    metrics_path: '/metrics',
    scheme: 'http',
    scrape_interval: '15s',
    scrape_timeout: '10s',
    targets: [
      { id: 't-3-1', endpoint: 'localhost:9090', labels: { app: 'prometheus' } }
    ],
    labels: { app: 'prometheus' }
  },
  {
    id: 'job-4',
    job_name: 'kubernetes-nodes',
    metrics_path: '/metrics',
    scheme: 'http',
    scrape_interval: '30s',
    scrape_timeout: '10s',
    targets: [
      { id: 't-4-1', endpoint: 'k8s-node-1:9100', labels: { job: 'k8s', env: 'prod' } },
      { id: 't-4-2', endpoint: 'k8s-node-2:9100', labels: { job: 'k8s', env: 'prod' } }
    ],
    labels: { job: 'k8s', env: 'prod' }
  },
  {
    id: 'job-5',
    job_name: 'kubernetes-pods',
    metrics_path: '/metrics',
    scheme: 'http',
    scrape_interval: '30s',
    scrape_timeout: '10s',
    targets: [
      { id: 't-5-1', endpoint: 'pod-app-1:8080', labels: { job: 'k8s', tier: 'backend' } }
    ],
    labels: { job: 'k8s' }
  }
]);

// Initial Original Content (for diff)
const originalYamlContent = ref('');
const rawEditorYaml = ref('');

// Current Selected Job
const currentJob = computed(() => {
  return scrapeJobs.value.find(j => j.id === selectedJobId.value) || scrapeJobs.value[0] || null;
});

// Expandable Scrape Jobs rows
const expandedJobIds = ref<Set<string>>(new Set());

const toggleExpandJob = (jobId: string) => {
  if (expandedJobIds.value.has(jobId)) {
    expandedJobIds.value.delete(jobId);
  } else {
    expandedJobIds.value.add(jobId);
  }
};

const isJobExpanded = (jobId: string) => expandedJobIds.value.has(jobId);

const expandAllJobs = () => {
  if (expandedJobIds.value.size === scrapeJobs.value.length) {
    expandedJobIds.value.clear();
  } else {
    expandedJobIds.value = new Set(scrapeJobs.value.map(j => j.id));
  }
};

const quickAddTargetToJob = (job: ScrapeJob) => {
  const newTarget: TargetItem = {
    id: `t-${Date.now()}-${Math.random()}`,
    endpoint: '127.0.0.1:9090',
    labels: {}
  };
  job.targets.push(newTarget);
  expandedJobIds.value.add(job.id);
  selectedJobId.value = job.id;
  showNotification('success', `Added target to '${job.job_name}'. Edit IP as needed.`);
};

const scrollToJobEditor = () => {
  const el = document.getElementById('job-details-editor');
  if (el) {
    el.scrollIntoView({ behavior: 'smooth' });
  }
};

// Set default selected job
onMounted(() => {
  if (scrapeJobs.value.length > 0 && !selectedJobId.value) {
    selectedJobId.value = scrapeJobs.value[0].id;
    expandedJobIds.value.add(selectedJobId.value);
  }
});

// Filtered Scrape Jobs
const filteredScrapeJobs = computed(() => {
  const q = searchJobQuery.value.trim().toLowerCase();
  if (!q) return scrapeJobs.value;
  return scrapeJobs.value.filter(j =>
    j.job_name.toLowerCase().includes(q) ||
    j.metrics_path.toLowerCase().includes(q) ||
    j.targets.some(t => t.endpoint.toLowerCase().includes(q))
  );
});

// ==================== YAML PARSER & GENERATOR ====================

// Generate Prometheus YAML from reactive model
// Generate Prometheus YAML snippet for a single scrape job
const generateJobYamlSnippet = (job: ScrapeJob, indent: string = '    '): string => {
  let y = '';
  if (job.metrics_path && job.metrics_path !== '/metrics') {
    y += `${indent}metrics_path: ${job.metrics_path}\n`;
  }
  if (job.scheme && job.scheme !== 'http') {
    y += `${indent}scheme: ${job.scheme}\n`;
  }
  if (job.scrape_interval) {
    y += `${indent}scrape_interval: ${job.scrape_interval}\n`;
  }
  if (job.scrape_timeout) {
    y += `${indent}scrape_timeout: ${job.scrape_timeout}\n`;
  }
  if (job.params && Object.keys(job.params).length > 0) {
    y += `${indent}params:\n`;
    for (const [k, v] of Object.entries(job.params)) {
      if (Array.isArray(v) && v.length > 0) {
        y += `${indent}  ${k}:\n`;
        v.forEach(val => {
          y += `${indent}    - ${val}\n`;
        });
      }
    }
  }
  y += `${indent}static_configs:\n`;
  y += `${indent}  - targets:\n`;
  job.targets.forEach(t => {
    const isDup = job.targets.filter(x => x.endpoint === t.endpoint).length > 1;
    y += `${indent}      - ${t.endpoint}${isDup ? ' # Duplicate target' : ''}\n`;
  });
  if (Object.keys(job.labels).length > 0) {
    y += `${indent}    labels:\n`;
    for (const [k, v] of Object.entries(job.labels)) {
      y += `${indent}      ${k}: ${v}\n`;
    }
  }
  if (job.relabel_configs && job.relabel_configs.length > 0) {
    y += `${indent}relabel_configs:\n`;
    job.relabel_configs.forEach(rc => {
      let first = true;
      if (rc.source_labels && rc.source_labels.length > 0) {
        y += `${indent}  - source_labels: [${rc.source_labels.join(', ')}]\n`;
        first = false;
      }
      if (rc.target_label) {
        y += `${indent}  ${first ? '- ' : '  '}target_label: ${rc.target_label}\n`;
        first = false;
      }
      if (rc.replacement) {
        y += `${indent}  ${first ? '- ' : '  '}replacement: ${rc.replacement}\n`;
        first = false;
      }
      if (rc.regex && rc.regex !== '(.*)') {
        y += `${indent}  ${first ? '- ' : '  '}regex: '${rc.regex}'\n`;
        first = false;
      }
      if (rc.action && rc.action !== 'replace') {
        y += `${indent}  ${first ? '- ' : '  '}action: ${rc.action}\n`;
        first = false;
      }
    });
  }
  return y;
};

// Generate Prometheus YAML from reactive model
const generateYaml = (singleJob?: ScrapeJob): string => {
  if (singleJob) {
    let y = `  - job_name: ${singleJob.job_name}\n`;
    y += generateJobYamlSnippet(singleJob, '    ');
    return y;
  }

  // Full YAML Generation
  let out = `# my global config\nglobal:\n`;
  out += `  scrape_interval: ${globalConfig.value.scrape_interval || '15s'}\n`;
  out += `  evaluation_interval: ${globalConfig.value.evaluation_interval || '15s'}\n`;
  if (globalConfig.value.scrape_timeout) {
    out += `  scrape_timeout: ${globalConfig.value.scrape_timeout}\n`;
  }
  if (Object.keys(globalConfig.value.external_labels).length > 0) {
    out += `  external_labels:\n`;
    for (const [k, v] of Object.entries(globalConfig.value.external_labels)) {
      out += `    ${k}: ${v}\n`;
    }
  }

  out += `\n# Alertmanager configuration\nalerting:\n  alertmanagers:\n`;
  out += `    - static_configs:\n        - targets:\n`;
  alertmanagerConfig.value.endpoints.forEach(ep => {
    out += `            - ${ep}\n`;
  });

  if (ruleFiles.value.length > 0) {
    out += `\n# Load rules once and periodically evaluate them\nrule_files:\n`;
    ruleFiles.value.forEach(rf => {
      out += `  - "${rf}"\n`;
    });
  }

  out += `\nscrape_configs:\n`;
  scrapeJobs.value.forEach(job => {
    out += `  - job_name: ${job.job_name}\n`;
    out += generateJobYamlSnippet(job, '    ');
    out += `\n`;
  });

  return out.trimEnd() + '\n';
};

// Reactive full YAML representation
const currentGeneratedYaml = computed(() => {
  if (viewMode.value === 'raw') {
    return rawEditorYaml.value;
  }
  if (previewScope.value === 'job' && currentJob.value) {
    return generateYaml(currentJob.value);
  }
  return generateYaml();
});

// Parse imported raw YAML string into reactive model
const parseYamlIntoModel = (content: string) => {
  try {
    const lines = content.split('\n');
    let currentSection = '';
    const parsedJobs: ScrapeJob[] = [];
    let activeParsedJob: Partial<ScrapeJob> | null = null;
    let inTargets = false;
    let inLabels = false;
    let inParams = false;
    let currentParamKey = '';
    let inRelabel = false;
    let currentRelabelRule: RelabelConfigItem | null = null;

    lines.forEach(line => {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) return;

      if (line.startsWith('global:')) {
        currentSection = 'global';
      } else if (line.startsWith('alerting:')) {
        currentSection = 'alerting';
      } else if (line.startsWith('rule_files:')) {
        currentSection = 'rules';
      } else if (line.startsWith('scrape_configs:')) {
        currentSection = 'scrape';
      } else if (currentSection === 'scrape' && trimmed.startsWith('- job_name:')) {
        if (activeParsedJob && activeParsedJob.job_name) {
          parsedJobs.push({
            id: `job-${Date.now()}-${parsedJobs.length}`,
            job_name: activeParsedJob.job_name,
            metrics_path: activeParsedJob.metrics_path || '/metrics',
            scheme: (activeParsedJob.scheme as any) || 'http',
            scrape_interval: activeParsedJob.scrape_interval || '15s',
            scrape_timeout: activeParsedJob.scrape_timeout || '10s',
            targets: activeParsedJob.targets || [],
            labels: activeParsedJob.labels || {},
            params: activeParsedJob.params && Object.keys(activeParsedJob.params).length > 0 ? activeParsedJob.params : undefined,
            relabel_configs: activeParsedJob.relabel_configs && activeParsedJob.relabel_configs.length > 0 ? activeParsedJob.relabel_configs : undefined
          });
        }
        const jName = trimmed.replace('- job_name:', '').trim().replace(/['"]/g, '');
        activeParsedJob = {
          job_name: jName,
          metrics_path: '/metrics',
          scheme: 'http',
          scrape_interval: '15s',
          scrape_timeout: '10s',
          targets: [],
          labels: {},
          params: {},
          relabel_configs: []
        };
        inTargets = false;
        inLabels = false;
        inParams = false;
        currentParamKey = '';
        inRelabel = false;
        currentRelabelRule = null;
      } else if (currentSection === 'scrape' && activeParsedJob) {
        if (trimmed.startsWith('metrics_path:')) {
          inTargets = false; inLabels = false; inParams = false; inRelabel = false;
          activeParsedJob.metrics_path = trimmed.replace('metrics_path:', '').trim().replace(/['"]/g, '');
        } else if (trimmed.startsWith('scheme:')) {
          inTargets = false; inLabels = false; inParams = false; inRelabel = false;
          activeParsedJob.scheme = trimmed.replace('scheme:', '').trim().toLowerCase() === 'https' ? 'https' : 'http';
        } else if (trimmed.startsWith('scrape_interval:')) {
          inTargets = false; inLabels = false; inParams = false; inRelabel = false;
          activeParsedJob.scrape_interval = trimmed.replace('scrape_interval:', '').trim();
        } else if (trimmed.startsWith('scrape_timeout:')) {
          inTargets = false; inLabels = false; inParams = false; inRelabel = false;
          activeParsedJob.scrape_timeout = trimmed.replace('scrape_timeout:', '').trim();
        } else if (trimmed.startsWith('params:')) {
          inParams = true; inTargets = false; inLabels = false; inRelabel = false;
          activeParsedJob.params = activeParsedJob.params || {};
        } else if (inParams && trimmed.startsWith('- ') && currentParamKey) {
          const val = trimmed.replace(/^-/, '').trim().replace(/['"]/g, '');
          if (val) activeParsedJob.params![currentParamKey].push(val);
        } else if (inParams && trimmed.includes(':') && !trimmed.startsWith('static_configs:') && !trimmed.startsWith('relabel_configs:') && !trimmed.startsWith('metrics_path:')) {
          const parts = trimmed.split(':');
          const pKey = parts[0].trim();
          const pVal = parts.slice(1).join(':').trim();
          if (pVal.startsWith('[') && pVal.endsWith(']')) {
            const arr = pVal.slice(1, -1).split(',').map(s => s.trim().replace(/['"]/g, '')).filter(Boolean);
            activeParsedJob.params![pKey] = arr;
            currentParamKey = '';
          } else if (pVal) {
            activeParsedJob.params![pKey] = [pVal.replace(/['"]/g, '')];
            currentParamKey = '';
          } else {
            currentParamKey = pKey;
            activeParsedJob.params![pKey] = [];
          }
        } else if (trimmed.startsWith('relabel_configs:')) {
          inRelabel = true; inParams = false; inTargets = false; inLabels = false;
          activeParsedJob.relabel_configs = activeParsedJob.relabel_configs || [];
        } else if (inRelabel && (trimmed.startsWith('-') || trimmed.includes(':')) && !trimmed.startsWith('static_configs:') && !trimmed.startsWith('params:') && !trimmed.startsWith('- job_name:')) {
          if (trimmed.startsWith('-')) {
            currentRelabelRule = {};
            activeParsedJob.relabel_configs!.push(currentRelabelRule);
          }
          if (currentRelabelRule) {
            const clean = trimmed.replace(/^-/, '').trim();
            if (clean.startsWith('source_labels:')) {
              const val = clean.replace('source_labels:', '').trim();
              if (val.startsWith('[') && val.endsWith(']')) {
                currentRelabelRule.source_labels = val.slice(1, -1).split(',').map(s => s.trim().replace(/['"]/g, '')).filter(Boolean);
              }
            } else if (clean.startsWith('target_label:')) {
              currentRelabelRule.target_label = clean.replace('target_label:', '').trim().replace(/['"]/g, '');
            } else if (clean.startsWith('replacement:')) {
              currentRelabelRule.replacement = clean.replace('replacement:', '').trim().replace(/['"]/g, '');
            } else if (clean.startsWith('action:')) {
              currentRelabelRule.action = clean.replace('action:', '').trim().replace(/['"]/g, '') as any;
            } else if (clean.startsWith('regex:')) {
              currentRelabelRule.regex = clean.replace('regex:', '').trim().replace(/^['"]|['"]$/g, '');
            }
          }
        } else if (trimmed.startsWith('targets:') || trimmed.startsWith('- targets:')) {
          inTargets = true; inLabels = false; inParams = false; inRelabel = false;
          const inlineMatch = trimmed.match(/\[(.*)\]/);
          if (inlineMatch && inlineMatch[1]) {
            const splitted = inlineMatch[1].split(',').map(s => s.trim().replace(/['"]/g, '')).filter(Boolean);
            splitted.forEach(ep => {
              activeParsedJob!.targets!.push({
                id: `t-${Date.now()}-${Math.random()}`,
                endpoint: ep,
                labels: {}
              });
            });
            inTargets = false;
          }
        } else if (trimmed.startsWith('labels:')) {
          inLabels = true; inTargets = false; inParams = false; inRelabel = false;
        } else if (inTargets && trimmed.startsWith('-') && !trimmed.startsWith('- targets:') && !trimmed.startsWith('- job_name:')) {
          const ep = trimmed.replace(/^-/, '').trim().replace(/['"]/g, '').split('#')[0].trim();
          if (ep) {
            activeParsedJob.targets!.push({
              id: `t-${Date.now()}-${Math.random()}`,
              endpoint: ep,
              labels: {}
            });
          }
        } else if (inLabels && trimmed.includes(':')) {
          const parts = trimmed.split(':');
          const k = parts[0].trim();
          const v = parts.slice(1).join(':').trim().replace(/['"]/g, '');
          if (k) {
            activeParsedJob.labels![k] = v;
          }
        } else if (!trimmed.startsWith('-') && !trimmed.startsWith('#')) {
          inTargets = false;
          inLabels = false;
        }
      }
    });

    if (activeParsedJob && activeParsedJob.job_name) {
      parsedJobs.push({
        id: `job-${Date.now()}-${parsedJobs.length}`,
        job_name: activeParsedJob.job_name,
        metrics_path: activeParsedJob.metrics_path || '/metrics',
        scheme: (activeParsedJob.scheme as any) || 'http',
        scrape_interval: activeParsedJob.scrape_interval || '15s',
        scrape_timeout: activeParsedJob.scrape_timeout || '10s',
        targets: activeParsedJob.targets || [],
        labels: activeParsedJob.labels || {},
        params: activeParsedJob.params && Object.keys(activeParsedJob.params).length > 0 ? activeParsedJob.params : undefined,
        relabel_configs: activeParsedJob.relabel_configs && activeParsedJob.relabel_configs.length > 0 ? activeParsedJob.relabel_configs : undefined
      });
    }

    if (parsedJobs.length > 0) {
      scrapeJobs.value = parsedJobs;
      selectedJobId.value = parsedJobs[0].id;
      expandedJobIds.value.add(parsedJobs[0].id);
    }
  } catch (err) {
    console.error('Failed to parse YAML model', err);
  }
};

// ==================== VALIDATION ENGINE ====================
interface ValidationResultItem {
  type: 'error' | 'warning' | 'info';
  message: string;
  jobName?: string;
  line?: number;
}

const lastValidatedTime = ref('');
const serverValidationIssues = ref<ValidationResultItem[]>([]);
const isValidating = ref(false);
const showValidationResults = ref(false);

const activeYamlContent = computed(() => {
  return viewMode.value === 'raw' ? rawEditorYaml.value : currentGeneratedYaml.value;
});

// Client-side real-time syntax and structural validation
const clientValidationItems = computed<ValidationResultItem[]>(() => {
  const items: ValidationResultItem[] = [];
  const text = activeYamlContent.value;
  if (!text || !text.trim()) {
    items.push({
      type: 'error',
      message: 'Configuration YAML is completely empty.'
    });
    return items;
  }

  const lines = text.split('\n');
  let currentJobName = '';
  let inTargets = false;
  const seenTargets = new Map<string, string>(); // endpoint -> jobName
  const seenJobNames = new Set<string>();

  const validTopLevelDirectives = new Set([
    'global', 'alerting', 'rule_files', 'scrape_configs',
    'storage', 'tracing', 'remote_write', 'remote_read', 'runtime'
  ]);

  lines.forEach((line, idx) => {
    const lineNum = idx + 1;
    const trimmed = line.trim();

    // Skip empty lines & comments
    if (!trimmed || trimmed.startsWith('#')) return;

    // 1. Forbid tab characters (Strict YAML syntax requirement)
    if (line.includes('\t')) {
      items.push({
        type: 'error',
        line: lineNum,
        message: `Line ${lineNum}: YAML forbids tab characters for indentation. Please use spaces.`
      });
    }

    const indent = line.length - line.trimStart().length;

    // 2. Top-level directive check (0 leading spaces)
    if (indent === 0) {
      inTargets = false;
      const colonIdx = trimmed.indexOf(':');
      if (colonIdx === -1) {
        items.push({
          type: 'error',
          line: lineNum,
          message: `Line ${lineNum}: Syntax error on '${trimmed}'. Top-level directives must end with a colon ':'.`
        });
      } else {
        const directive = trimmed.slice(0, colonIdx).trim();
        if (!validTopLevelDirectives.has(directive)) {
          items.push({
            type: 'warning',
            line: lineNum,
            message: `Line ${lineNum}: Unrecognized root directive '${directive}:'.`
          });
        }
      }
      return;
    }

    // 3. Mapping lines (not starting with '-') MUST contain a colon ':'
    if (!trimmed.startsWith('-')) {
      const colonIdx = trimmed.indexOf(':');
      if (colonIdx === -1) {
        items.push({
          type: 'error',
          line: lineNum,
          message: `Line ${lineNum}: Syntax error on '${trimmed}'. Missing colon ':' separating key and value.`
        });
        return;
      }

      const key = trimmed.slice(0, colonIdx).trim();
      const val = trimmed.slice(colonIdx + 1).trim();

      // Check Prometheus duration syntax (e.g. scrape_interval, evaluation_interval, scrape_timeout)
      if (['scrape_interval', 'evaluation_interval', 'scrape_timeout'].includes(key)) {
        if (!val) {
          items.push({
            type: 'error',
            line: lineNum,
            message: `Line ${lineNum}: Missing duration value for '${key}'.`
          });
        } else if (/^\d+$/.test(val)) {
          items.push({
            type: 'error',
            line: lineNum,
            message: `Line ${lineNum}: Invalid duration '${val}' for '${key}'. Prometheus requires a unit (e.g. '${val}s', '1m', '500ms').`
          });
        } else if (!/^\d+(\.\d+)?(ms|s|m|h|d|w|y)$/.test(val)) {
          items.push({
            type: 'error',
            line: lineNum,
            message: `Line ${lineNum}: Invalid duration unit '${val}' for '${key}'. Must end in ms, s, m, h, d, w, or y.`
          });
        }
      }

      if (key === 'targets') {
        inTargets = true;
        const inlineMatch = val.match(/\[(.*)\]/);
        if (inlineMatch && inlineMatch[1]) {
          const endpoints = inlineMatch[1].split(',').map(s => s.trim().replace(/['"]/g, '')).filter(Boolean);
          endpoints.forEach(ep => {
            if (seenTargets.has(ep)) {
              items.push({
                type: 'error',
                line: lineNum,
                message: `Line ${lineNum}: Duplicate target endpoint '${ep}' (already defined in job '${seenTargets.get(ep)}').`
              });
            } else {
              seenTargets.set(ep, currentJobName || 'scrape');
            }
          });
        }
      } else {
        inTargets = false;
      }
    } else {
      // List items starting with '-'
      if (trimmed.startsWith('- job_name:')) {
        inTargets = false;
        const jName = trimmed.replace('- job_name:', '').trim().replace(/['"]/g, '');
        if (!jName) {
          items.push({
            type: 'error',
            line: lineNum,
            message: `Line ${lineNum}: Scrape job has empty job_name.`
          });
        } else if (seenJobNames.has(jName)) {
          items.push({
            type: 'error',
            line: lineNum,
            message: `Line ${lineNum}: Duplicate job_name '${jName}' detected.`
          });
        } else {
          seenJobNames.add(jName);
          currentJobName = jName;
        }
      } else if (trimmed.startsWith('- targets:')) {
        inTargets = true;
        const inlineMatch = trimmed.match(/\[(.*)\]/);
        if (inlineMatch && inlineMatch[1]) {
          const endpoints = inlineMatch[1].split(',').map(s => s.trim().replace(/['"]/g, '')).filter(Boolean);
          endpoints.forEach(ep => {
            if (seenTargets.has(ep)) {
              items.push({
                type: 'error',
                line: lineNum,
                message: `Line ${lineNum}: Duplicate target endpoint '${ep}' (already defined in job '${seenTargets.get(ep)}').`
              });
            } else {
              seenTargets.set(ep, currentJobName || 'scrape');
            }
          });
        }
      } else if (inTargets) {
        const ep = trimmed.replace(/^-/, '').trim().replace(/['"]/g, '').split('#')[0].trim();
        if (ep) {
          if (seenTargets.has(ep)) {
            items.push({
              type: 'error',
              line: lineNum,
              message: `Line ${lineNum}: Duplicate target endpoint '${ep}' (already defined in job '${seenTargets.get(ep)}').`
            });
          } else {
            seenTargets.set(ep, currentJobName || 'scrape');
          }
        }
      }
    }
  });

  return items;
});

// Merged validation items (Client-side real-time checks + Backend compiler issues)
const validationItems = computed<ValidationResultItem[]>(() => {
  const list = [...clientValidationItems.value];

  // Merge in server validation issues
  serverValidationIssues.value.forEach(sItem => {
    if (!list.some(cItem => cItem.message === sItem.message)) {
      list.push(sItem);
    }
  });

  // If completely error-free, add compliant message
  if (list.filter(i => i.type === 'error').length === 0) {
    list.unshift({
      type: 'info',
      message: 'YAML syntax structure verified and compliant.'
    });
  }

  return list;
});

const errorCount = computed(() => validationItems.value.filter(v => v.type === 'error').length);
const warningCount = computed(() => validationItems.value.filter(v => v.type === 'warning').length);
const infoCount = computed(() => validationItems.value.filter(v => v.type === 'info').length);

const runValidation = async () => {
  const d = new Date();
  lastValidatedTime.value = d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) + ' ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
  isValidating.value = true;
  serverValidationIssues.value = [];

  const textToValidate = activeYamlContent.value;

  try {
    const res = await axios.post('/api/v1/prometheus/validate', {
      instanceId: selectedInstanceId.value,
      yaml: textToValidate
    });

    if (res.data?.issues && Array.isArray(res.data.issues)) {
      serverValidationIssues.value = res.data.issues;
    }
  } catch (err: any) {
    const msg = err.response?.data?.error || err.message;
    if (msg) {
      serverValidationIssues.value.push({
        type: 'error',
        message: `Compiler check: ${msg}`
      });
    }
  } finally {
    isValidating.value = false;
    showValidationResults.value = true;
  }

  if (errorCount.value === 0) {
    showNotification('success', 'Validation passed! No critical configuration errors found.');
  } else {
    showNotification('warning', `Validation completed with ${errorCount.value} critical error(s) and ${warningCount.value} warning(s).`);
  }
};

// Check if a target in a job is duplicate
const isTargetDuplicate = (job: ScrapeJob | null, idx: number, endpoint: string): boolean => {
  if (!job || !endpoint.trim()) return false;
  return job.targets.some((t, i) => i < idx && t.endpoint.trim() === endpoint.trim());
};

const isTargetDuplicateInCurrentJob = (idx: number, endpoint: string): boolean => {
  return isTargetDuplicate(currentJob.value, idx, endpoint);
};

// Line numbered code viewer with error highlighting
const formattedLines = computed(() => {
  const text = currentGeneratedYaml.value;
  const errorLines = new Set<number>();
  validationItems.value.forEach(item => {
    if (item.type === 'error' && item.line) {
      errorLines.add(item.line);
    }
  });

  return text.split('\n').map((line, idx) => {
    const lineNum = idx + 1;
    const isError = errorLines.has(lineNum) ||
                    line.includes('# Duplicate target') ||
                    (line.includes('static_configs:') && line.includes('error'));
    return {
      num: lineNum,
      content: line,
      isError
    };
  });
});

// Tokenize YAML line for crisp, high-contrast syntax highlighting in Light & Dark Mode
const formatYamlTokens = (line: string) => {
  const trimmed = line.trimStart();
  const indent = line.slice(0, line.length - trimmed.length);

  // 1. Comment
  if (trimmed.startsWith('#')) {
    return [
      { text: indent, cls: '' },
      { text: trimmed, cls: 'text-amber-700 dark:text-amber-400/90 italic' }
    ];
  }

  // 2. YAML Mapping Key: must have ': ' or end with ':'
  let colonIdx = -1;
  const spaceColonIdx = trimmed.indexOf(': ');
  if (spaceColonIdx !== -1) {
    colonIdx = spaceColonIdx;
  } else if (trimmed.endsWith(':')) {
    colonIdx = trimmed.length - 1;
  }

  if (colonIdx !== -1) {
    const key = trimmed.slice(0, colonIdx + 1);
    const val = trimmed.slice(colonIdx + 1);

    // If key starts with '- ', split the list marker
    if (key.startsWith('- ')) {
      return [
        { text: indent, cls: '' },
        { text: '- ', cls: 'text-slate-400 dark:text-slate-500 font-bold' },
        { text: key.slice(2), cls: 'text-blue-700 dark:text-sky-400 font-semibold' },
        { text: val, cls: 'text-slate-800 dark:text-slate-200' }
      ];
    }

    return [
      { text: indent, cls: '' },
      { text: key, cls: 'text-blue-700 dark:text-sky-400 font-semibold' },
      { text: val, cls: 'text-slate-800 dark:text-slate-200' }
    ];
  }

  // 3. List item with hyphen '- '
  if (trimmed.startsWith('- ')) {
    return [
      { text: indent, cls: '' },
      { text: '- ', cls: 'text-slate-400 dark:text-slate-500 font-bold' },
      { text: trimmed.slice(2), cls: 'text-slate-800 dark:text-slate-200' }
    ];
  }

  // 4. Default plain text
  return [
    { text: line, cls: 'text-slate-800 dark:text-slate-200' }
  ];
};

// ==================== DIFF PREVIEW ENGINE ====================
interface DiffLine {
  type: 'added' | 'removed' | 'unchanged';
  text: string;
}

const diffLines = computed<DiffLine[]>(() => {
  const orig = originalYamlContent.value.split('\n');
  const curr = currentGeneratedYaml.value.split('\n');
  const res: DiffLine[] = [];

  const max = Math.max(orig.length, curr.length);
  for (let i = 0; i < max; i++) {
    const o = orig[i];
    const c = curr[i];
    if (o === undefined) {
      res.push({ type: 'added', text: c });
    } else if (c === undefined) {
      res.push({ type: 'removed', text: o });
    } else if (o !== c) {
      res.push({ type: 'removed', text: o });
      res.push({ type: 'added', text: c });
    } else {
      res.push({ type: 'unchanged', text: o });
    }
  }
  return res;
});

// ==================== TARGET ACTIONS ====================
const addTargetToCurrentJob = () => {
  if (!currentJob.value) return;
  currentJob.value.targets.push({
    id: `t-${Date.now()}-${Math.random()}`,
    endpoint: '',
    labels: { ...currentJob.value.labels }
  });
};

const confirmDeleteTarget = (job: ScrapeJob, target: TargetItem) => {
  targetToDelete.value = { job, target };
  showDeleteTargetModal.value = true;
};

const executeDeleteTarget = () => {
  if (targetToDelete.value) {
    const { job, target } = targetToDelete.value;
    job.targets = job.targets.filter(t => t.id !== target.id);
    showNotification('success', `Target ${target.endpoint || 'entry'} removed.`);
  }
  showDeleteTargetModal.value = false;
  targetToDelete.value = null;
};

const executeBulkAdd = () => {
  if (!currentJob.value || !bulkAddText.value.trim()) return;
  const lines = bulkAddText.value.split(/[\n,]/).map(s => s.trim()).filter(Boolean);
  let added = 0;
  lines.forEach(ep => {
    currentJob.value!.targets.push({
      id: `t-${Date.now()}-${Math.random()}`,
      endpoint: ep,
      labels: { ...currentJob.value!.labels }
    });
    added++;
  });
  bulkAddText.value = '';
  showBulkAddModal.value = false;
  showNotification('success', `Added ${added} targets to '${currentJob.value.job_name}'.`);
};

// ==================== JOB ACTIONS ====================
type JobPreset = 'standard' | 'snmp' | 'blackbox' | 'custom';

const newJobForm = ref({
  preset: 'standard' as JobPreset,
  job_name: '',
  metrics_path: '/metrics',
  scheme: 'http' as 'http' | 'https',
  scrape_interval: '15s',
  scrape_timeout: '10s',
  snmp_module: 'if_mib',
  snmp_custom_module: '',
  blackbox_module: 'http_2xx',
  blackbox_custom_module: '',
  exporter_address: 'localhost:9116',
  targets_text: ''
});

const availableRemoteHosts = ref<Array<{ id: string; name: string; host: string }>>([]);

const fetchAvailableRemoteHosts = async () => {
  try {
    const res = await axios.get('/api/v1/remote-host');
    if (res?.data?.success && Array.isArray(res.data.data)) {
      availableRemoteHosts.value = res.data.data.map((h: any) => ({
        id: h.id,
        name: h.name,
        host: h.host,
      }));
    }
  } catch (_) {}
};

const appendRemoteHostTarget = (hostStr: string) => {
  const current = newJobForm.value.targets_text.trim();
  const targetWithPort = hostStr.includes(':') ? hostStr : `${hostStr}:9100`;
  if (!current) {
    newJobForm.value.targets_text = targetWithPort;
  } else {
    newJobForm.value.targets_text = `${current}\n${targetWithPort}`;
  }
};

const openAddJobModal = () => {
  if (availableRemoteHosts.value.length === 0) {
    fetchAvailableRemoteHosts();
  }
  showAddJobModal.value = true;
};

const setJobPreset = (preset: JobPreset) => {
  newJobForm.value.preset = preset;
  if (preset === 'standard') {
    newJobForm.value.metrics_path = '/metrics';
    newJobForm.value.scrape_interval = '15s';
    newJobForm.value.scrape_timeout = '10s';
  } else if (preset === 'snmp') {
    newJobForm.value.metrics_path = '/snmp';
    newJobForm.value.scrape_interval = '60s';
    newJobForm.value.scrape_timeout = '30s';
    newJobForm.value.snmp_module = 'if_mib';
    newJobForm.value.exporter_address = 'localhost:9116';
  } else if (preset === 'blackbox') {
    newJobForm.value.metrics_path = '/probe';
    newJobForm.value.scrape_interval = '15s';
    newJobForm.value.scrape_timeout = '10s';
    newJobForm.value.blackbox_module = 'http_2xx';
    newJobForm.value.exporter_address = 'localhost:9115';
  } else if (preset === 'custom') {
    newJobForm.value.metrics_path = '/metrics';
    newJobForm.value.scrape_interval = '15s';
    newJobForm.value.scrape_timeout = '10s';
  }
};

const executeAddJob = () => {
  if (!newJobForm.value.job_name.trim()) return;
  const jName = newJobForm.value.job_name.trim();

  let path = newJobForm.value.metrics_path.trim() || '/metrics';
  if (!path.startsWith('/')) path = '/' + path;

  const paramsObj: Record<string, string[]> = {};
  const relabelConfigs: RelabelConfigItem[] = [];

  if (newJobForm.value.preset === 'snmp') {
    path = newJobForm.value.metrics_path.trim() || '/snmp';
    if (!path.startsWith('/')) path = '/' + path;
    const mod = newJobForm.value.snmp_module === 'custom'
      ? (newJobForm.value.snmp_custom_module.trim() || 'if_mib')
      : (newJobForm.value.snmp_module || 'if_mib');
    paramsObj.module = [mod];

    const exporter = newJobForm.value.exporter_address.trim() || 'localhost:9116';
    relabelConfigs.push(
      { source_labels: ['__address__'], target_label: '__param_target__' },
      { source_labels: ['__param_target__'], target_label: 'instance' },
      { target_label: '__address__', replacement: exporter }
    );
  } else if (newJobForm.value.preset === 'blackbox') {
    path = newJobForm.value.metrics_path.trim() || '/probe';
    if (!path.startsWith('/')) path = '/' + path;
    const mod = newJobForm.value.blackbox_module === 'custom'
      ? (newJobForm.value.blackbox_custom_module.trim() || 'http_2xx')
      : (newJobForm.value.blackbox_module || 'http_2xx');
    paramsObj.module = [mod];

    const exporter = newJobForm.value.exporter_address.trim() || 'localhost:9115';
    relabelConfigs.push(
      { source_labels: ['__address__'], target_label: '__param_target__' },
      { source_labels: ['__param_target__'], target_label: 'instance' },
      { target_label: '__address__', replacement: exporter }
    );
  } else if (newJobForm.value.preset === 'custom' && newJobForm.value.exporter_address.trim()) {
    const exporter = newJobForm.value.exporter_address.trim();
    relabelConfigs.push(
      { source_labels: ['__address__'], target_label: '__param_target__' },
      { source_labels: ['__param_target__'], target_label: 'instance' },
      { target_label: '__address__', replacement: exporter }
    );
  }

  // Parse optional initial targets
  const initialTargets: TargetItem[] = [];
  if (newJobForm.value.targets_text.trim()) {
    const lines = newJobForm.value.targets_text.split(/[\n,]+/).map(s => s.trim()).filter(Boolean);
    lines.forEach(ep => {
      initialTargets.push({
        id: `t-${Date.now()}-${Math.random()}`,
        endpoint: ep,
        labels: { app: jName }
      });
    });
  } else {
    const defaultEp = newJobForm.value.preset === 'snmp' ? '192.168.1.1'
      : newJobForm.value.preset === 'blackbox' ? 'https://google.com'
      : 'localhost:9100';
    initialTargets.push({
      id: `t-${Date.now()}-${Math.random()}`,
      endpoint: defaultEp,
      labels: { app: jName }
    });
  }

  const newJob: ScrapeJob = {
    id: `job-${Date.now()}`,
    job_name: jName,
    metrics_path: path,
    scheme: newJobForm.value.scheme,
    scrape_interval: newJobForm.value.scrape_interval || '15s',
    scrape_timeout: newJobForm.value.scrape_timeout || '10s',
    targets: initialTargets,
    labels: { app: jName },
    params: Object.keys(paramsObj).length > 0 ? paramsObj : undefined,
    relabel_configs: relabelConfigs.length > 0 ? relabelConfigs : undefined
  };

  scrapeJobs.value.push(newJob);
  selectedJobId.value = newJob.id;
  showAddJobModal.value = false;

  // Reset form
  newJobForm.value.job_name = '';
  newJobForm.value.targets_text = '';
  newJobForm.value.snmp_custom_module = '';
  newJobForm.value.blackbox_custom_module = '';
  setJobPreset('standard');

  showNotification('success', `Scrape job '${newJob.job_name}' created.`);
};

// ==================== PARAMETER & RELABEL ACTIONS FOR CURRENT JOB ====================
const newParamKey = ref('');
const newParamValue = ref('');

const addParamToCurrentJob = () => {
  if (!currentJob.value || !newParamKey.value.trim() || !newParamValue.value.trim()) return;
  if (!currentJob.value.params) {
    currentJob.value.params = {};
  }
  const k = newParamKey.value.trim();
  const v = newParamValue.value.trim();
  if (!currentJob.value.params[k]) {
    currentJob.value.params[k] = [];
  }
  currentJob.value.params[k].push(v);
  newParamKey.value = '';
  newParamValue.value = '';
};

const removeParamValue = (key: string, idx: number) => {
  if (!currentJob.value?.params?.[key]) return;
  currentJob.value.params[key].splice(idx, 1);
  if (currentJob.value.params[key].length === 0) {
    delete currentJob.value.params[key];
  }
};

const deleteParamKey = (key: string) => {
  if (!currentJob.value?.params) return;
  delete currentJob.value.params[key];
};

const quickAddParam = (key: string, val: string) => {
  if (!currentJob.value) return;
  if (!currentJob.value.params) {
    currentJob.value.params = {};
  }
  currentJob.value.params[key] = [val];
};

const applyExporterRelabelPreset = (preset: 'snmp' | 'blackbox') => {
  if (!currentJob.value) return;
  const address = preset === 'snmp' ? 'localhost:9116' : 'localhost:9115';
  currentJob.value.relabel_configs = [
    { source_labels: ['__address__'], target_label: '__param_target__' },
    { source_labels: ['__param_target__'], target_label: 'instance' },
    { target_label: '__address__', replacement: address }
  ];
  showNotification('success', `Applied ${preset.toUpperCase()} Exporter relabeling rules.`);
};

const addEmptyRelabelRule = () => {
  if (!currentJob.value) return;
  if (!currentJob.value.relabel_configs) {
    currentJob.value.relabel_configs = [];
  }
  currentJob.value.relabel_configs.push({
    source_labels: ['__address__'],
    target_label: '',
    replacement: '',
    action: 'replace'
  });
};

const deleteRelabelRule = (idx: number) => {
  if (!currentJob.value?.relabel_configs) return;
  currentJob.value.relabel_configs.splice(idx, 1);
};

const duplicateCurrentJob = () => {
  if (!currentJob.value) return;
  const clone: ScrapeJob = JSON.parse(JSON.stringify(currentJob.value));
  clone.id = `job-${Date.now()}`;
  clone.job_name = `${clone.job_name}-copy`;
  scrapeJobs.value.push(clone);
  selectedJobId.value = clone.id;
  showNotification('success', `Job duplicated as '${clone.job_name}'.`);
};

const confirmDeleteJob = (job: ScrapeJob) => {
  jobToDelete.value = job;
  showDeleteJobModal.value = true;
};

const executeDeleteJob = () => {
  if (jobToDelete.value) {
    const name = jobToDelete.value.job_name;
    scrapeJobs.value = scrapeJobs.value.filter(j => j.id !== jobToDelete.value!.id);
    if (selectedJobId.value === jobToDelete.value.id && scrapeJobs.value.length > 0) {
      selectedJobId.value = scrapeJobs.value[0].id;
    }
    showNotification('success', `Job '${name}' deleted.`);
  }
  showDeleteJobModal.value = false;
  jobToDelete.value = null;
};

// Clipboard Helper
const copiedText = ref(false);
const copyYamlToClipboard = async () => {
  try {
    await navigator.clipboard.writeText(currentGeneratedYaml.value);
    copiedText.value = true;
    setTimeout(() => { copiedText.value = false; }, 2000);
    showNotification('success', 'YAML copied to clipboard.');
  } catch (e) {
    console.error(e);
  }
};

// ==================== BACKEND API HANDLERS ====================
const fetchPrometheusInstances = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/v1/settings/prometheus?type=prometheus').catch(() => null);
    if (res && res.data && res.data.success && Array.isArray(res.data.data)) {
      const promList = res.data.data.filter((p: any) => {
        const name = (p.name || '').toLowerCase();
        const path = (p.path || '').toLowerCase();
        return !name.includes('data prepper') && !name.includes('dataprepper') && !path.includes('pipeline');
      });

      if (promList.length > 0) {
        instances.value = promList;
        const matched = promList.find((i: any) => i.id === selectedInstanceId.value)
                     || promList.find((i: any) => i.isActive)
                     || promList[0];
        selectedInstanceId.value = matched.id;
        configFilePath.value = matched.path || '/etc/prometheus/prometheus.yml';
        await fetchConfigContent(matched.id);
      } else {
        instances.value = [];
        isLoaded.value = true;
      }
    }
  } catch (err: any) {
    console.error(err);
  } finally {
    loading.value = false;
  }
};

const fetchConfigContent = async (instanceId: string) => {
  loading.value = true;
  try {
    const res = await axios.get(`/api/v1/prometheus/config?instanceId=${instanceId}`);
    if (res.data?.success && res.data.data) {
      const content = res.data.data.content || '';
      originalYamlContent.value = content;
      rawEditorYaml.value = content;
      if (res.data.data.path) {
        configFilePath.value = res.data.data.path;
      }
      if (content.trim()) {
        parseYamlIntoModel(content);
      }
      isLoaded.value = true;
    }
  } catch (err: any) {
    showNotification('error', `Failed to load config: ${err.response?.data?.error || err.message}`);
  } finally {
    loading.value = false;
  }
};

const handleInstanceChange = () => {
  showValidationResults.value = false;
  serverValidationIssues.value = [];
  const current = instances.value.find((i) => i.id === selectedInstanceId.value);
  if (current) {
    if (current.path) configFilePath.value = current.path;
    fetchConfigContent(current.id);
  }
};

// Save Draft to LocalStorage
const saveDraft = () => {
  try {
    const payload = generateYaml();
    localStorage.setItem(`hcp_prom_draft_${selectedInstanceId.value}`, payload);
    showNotification('success', 'Draft saved locally.');
  } catch (e: any) {
    showNotification('error', 'Failed to save draft.');
  }
};

// Save & Restart Prometheus on Server
const handleSaveAndRestart = async () => {
  if (!selectedInstanceId.value) {
    showNotification('error', 'Please select a Prometheus instance first.');
    return;
  }
  if (errorCount.value > 0) {
    showNotification('error', `Cannot deploy: resolve ${errorCount.value} critical errors before restarting.`);
    return;
  }

  saving.value = true;
  const yamlToSave = viewMode.value === 'raw' ? rawEditorYaml.value : generateYaml();

  try {
    const res = await axios.post('/api/v1/prometheus/config', {
      instanceId: selectedInstanceId.value,
      yaml: yamlToSave,
      reload: true
    });

    if (res.data?.success) {
      originalYamlContent.value = yamlToSave;
      showNotification('success', res.data.message || 'Configuration deployed & Prometheus service reloaded successfully.');
    } else {
      showNotification('error', `Deploy failed: ${res.data?.error || 'Unknown error'}`);
    }
  } catch (err: any) {
    showNotification('error', `Deploy failed: ${err.response?.data?.error || err.message}`);
  } finally {
    saving.value = false;
  }
};

onMounted(() => {
  fetchPrometheusInstances();
  fetchAvailableRemoteHosts();
});
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto font-sans">
    <!-- Header (Strictly adheres to AGENTS.md: pure text title, no icon, no emoji) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Prometheus Config
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Create, validate, and deploy prometheus.yml safely.
        </p>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <!-- View Mode Toggle -->
        <div class="flex items-center bg-slate-100 dark:bg-[#121826] p-0.5 rounded-lg border border-slate-200 dark:border-[#1b2234]">
          <button
            @click="viewMode = 'visual'"
            :class="[
              'px-2.5 py-1 text-xs font-semibold rounded-md transition cursor-pointer flex items-center gap-1.5',
              viewMode === 'visual'
                ? 'bg-white dark:bg-[#1a2233] text-blue-600 dark:text-blue-400 shadow-sm'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
            ]"
          >
            <Sliders class="w-3 h-3 text-slate-400" />
            <span>Visual Builder</span>
          </button>
          <button
            @click="viewMode = 'raw'; rawEditorYaml = generateYaml()"
            :class="[
              'px-2.5 py-1 text-xs font-semibold rounded-md transition cursor-pointer flex items-center gap-1.5',
              viewMode === 'raw'
                ? 'bg-white dark:bg-[#1a2233] text-blue-600 dark:text-blue-400 shadow-sm'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
            ]"
          >
            <Code class="w-3 h-3 text-slate-400" />
            <span>Raw YAML</span>
          </button>
        </div>

        <button
          @click="router.push('/connections')"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 text-xs font-semibold transition cursor-pointer shadow-sm"
        >
          <ExternalLink class="w-3.5 h-3.5 text-slate-400" />
          <span>Connections</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div
      v-if="notification"
      :class="[
        'p-3 rounded-xl text-xs font-medium flex items-center gap-2 transition animate-in fade-in',
        notification.type === 'success'
          ? 'bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/50 text-emerald-800 dark:text-emerald-300'
          : notification.type === 'warning'
            ? 'bg-amber-50 dark:bg-amber-950/40 border border-amber-200 dark:border-amber-800/50 text-amber-800 dark:text-amber-300'
            : 'bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-800/50 text-rose-800 dark:text-rose-300'
      ]"
    >
      <CheckCircle2 v-if="notification.type === 'success'" class="w-4 h-4 text-emerald-500 shrink-0" />
      <AlertTriangle v-else-if="notification.type === 'warning'" class="w-4 h-4 text-amber-500 shrink-0" />
      <AlertCircle v-else class="w-4 h-4 text-rose-500 shrink-0" />
      <span>{{ notification.message }}</span>
    </div>

    <!-- Top Controls: Instance Selector, Search & Action Buttons -->
    <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm">
      <!-- Instance Selector & Status -->
      <div class="flex flex-wrap items-center gap-3">
        <label class="text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider shrink-0">
          Prometheus Instance
        </label>
        <select
          v-model="selectedInstanceId"
          @change="handleInstanceChange"
          class="bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
        >
          <option v-for="inst in instances" :key="inst.id" :value="inst.id">
            {{ inst.name }}
          </option>
          <option v-if="instances.length === 0" value="">
            Prometheus-honet-labs (Default)
          </option>
        </select>

        <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-bold bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-800">
          <span class="w-1.5 h-1.5 rounded-full bg-slate-400"></span>
          <span>Loaded</span>
        </span>

        <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-bold bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-500/30">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
          <span>Healthy</span>
        </span>

        <!-- Search Bar -->
        <div class="relative w-full sm:w-60">
          <Search class="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
          <input
            v-model="searchJobQuery"
            placeholder="Search jobs or targets..."
            class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg pl-8 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="flex flex-wrap items-center gap-2 shrink-0">
        <button
          @click="runValidation"
          :disabled="isValidating"
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer border border-slate-200 dark:border-slate-800 disabled:opacity-50"
        >
          <RotateCw v-if="isValidating" class="w-3.5 h-3.5 text-blue-500 animate-spin" />
          <Check v-else class="w-3.5 h-3.5 text-slate-400" />
          <span>{{ isValidating ? 'Validating...' : 'Validate' }}</span>
        </button>

        <button
          @click="activeRightTab = 'diff'"
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer border border-slate-200 dark:border-slate-800"
        >
          <FileDiff class="w-3.5 h-3.5 text-slate-400" />
          <span>Preview Diff</span>
        </button>

        <button
          @click="saveDraft"
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer border border-slate-200 dark:border-slate-800"
        >
          <Save class="w-3.5 h-3.5 text-slate-400" />
          <span>Save Draft</span>
        </button>

        <button
          @click="handleSaveAndRestart"
          :disabled="saving || !canManage"
          :title="!canManage ? 'You do not have permission to modify Prometheus configuration' : ''"
          class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-xs font-bold transition flex items-center gap-1.5 shadow-sm cursor-pointer"
        >
          <Play class="w-3.5 h-3.5" />
          <span>{{ saving ? 'Deploying...' : 'Save & Restart' }}</span>
        </button>
      </div>
    </div>

    <!-- Pre-Flight Validation Results Panel (Appears at top after validation completes) -->
    <transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 -translate-y-2"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100 translate-y-0"
      leave-to-class="opacity-0 -translate-y-2"
    >
      <div
        v-if="showValidationResults"
        :class="[
          'p-4 rounded-xl border shadow-sm space-y-3 transition-all',
          errorCount > 0
            ? 'bg-rose-50/60 dark:bg-[#1a1215] border-rose-200 dark:border-rose-900/50'
            : warningCount > 0
            ? 'bg-amber-50/60 dark:bg-[#1c1811] border-amber-200 dark:border-amber-900/50'
            : 'bg-emerald-50/60 dark:bg-[#0e1c16] border-emerald-200 dark:border-emerald-900/50'
        ]"
      >
        <div class="flex items-center justify-between border-b border-slate-200/80 dark:border-[#1b2234] pb-2.5">
          <div class="flex items-center gap-2">
            <AlertCircle v-if="errorCount > 0" class="w-4 h-4 text-rose-500 shrink-0" />
            <AlertTriangle v-else-if="warningCount > 0" class="w-4 h-4 text-amber-500 shrink-0" />
            <CheckCircle2 v-else class="w-4 h-4 text-emerald-500 shrink-0" />

            <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
              Validation Results
            </h3>

            <span
              v-if="errorCount === 0"
              class="text-[10px] font-semibold text-emerald-700 dark:text-emerald-400 bg-emerald-100 dark:bg-emerald-500/20 px-2 py-0.5 rounded-full"
            >
              Passed
            </span>
            <span
              v-else
              class="text-[10px] font-semibold text-rose-700 dark:text-rose-400 bg-rose-100 dark:bg-rose-500/20 px-2 py-0.5 rounded-full"
            >
              {{ errorCount }} Error(s) Found
            </span>
          </div>

          <div class="flex items-center gap-3">
            <span v-if="lastValidatedTime" class="text-[10px] text-slate-500 dark:text-slate-400">
              Last validated: {{ lastValidatedTime }}
            </span>
            <button
              @click="runValidation"
              :disabled="isValidating"
              class="px-2.5 py-1 bg-white hover:bg-slate-50 dark:bg-[#161d2d] dark:hover:bg-[#1e273d] text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-slate-700/60 rounded text-[10px] font-bold transition cursor-pointer disabled:opacity-50 flex items-center gap-1 shadow-xs"
            >
              <RotateCw v-if="isValidating" class="w-3 h-3 animate-spin" />
              <span>{{ isValidating ? 'Validating...' : 'Validate Again' }}</span>
            </button>
            <button
              @click="showValidationResults = false"
              title="Close validation results"
              class="p-1 text-slate-400 hover:text-slate-600 dark:hover:text-white rounded transition cursor-pointer"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Counter Indicators on Left + List of Items -->
        <div class="flex flex-col sm:flex-row sm:items-start gap-3 sm:gap-4">
          <!-- Counter Pills -->
          <div class="flex sm:flex-col flex-wrap gap-1.5 shrink-0 pt-0.5">
            <div class="flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-bold bg-rose-50 dark:bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-200 dark:border-rose-500/30">
              <span class="w-1.5 h-1.5 rounded-full bg-rose-500"></span>
              <span>{{ errorCount }} Error</span>
            </div>
            <div class="flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-bold bg-amber-50 dark:bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-200 dark:border-amber-500/30">
              <span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
              <span>{{ warningCount }} Warn</span>
            </div>
            <div class="flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-bold bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-500/30">
              <span class="w-1.5 h-1.5 rounded-full bg-blue-500"></span>
              <span>{{ infoCount }} Info</span>
            </div>
          </div>

          <!-- Itemized Messages -->
          <div class="flex-1 space-y-1.5 text-xs max-h-48 overflow-y-auto pr-1">
            <div
              v-for="(item, idx) in validationItems"
              :key="idx"
              class="flex items-start gap-2 py-0.5"
            >
              <CheckCircle2 v-if="item.type === 'info'" class="w-3.5 h-3.5 text-emerald-500 shrink-0 mt-0.5" />
              <AlertCircle v-else-if="item.type === 'error'" class="w-3.5 h-3.5 text-rose-500 shrink-0 mt-0.5" />
              <AlertTriangle v-else class="w-3.5 h-3.5 text-amber-500 shrink-0 mt-0.5" />

              <span :class="[
                'text-[11px] leading-tight font-mono',
                item.type === 'error' ? 'text-rose-700 dark:text-rose-400 font-semibold' :
                item.type === 'warning' ? 'text-amber-700 dark:text-amber-400' :
                'text-slate-600 dark:text-slate-300'
              ]">
                {{ item.message }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </transition>

    <!-- MAIN DUAL-PANE WORKSPACE (60% Form / 40% YAML Preview) -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
      <!-- ==================== LEFT COLUMN: VISUAL BUILDER (7 Cols) ==================== -->
      <div class="lg:col-span-7 space-y-4">
        <!-- Raw YAML Editor Container (shown when viewMode === 'raw') -->
        <div v-if="viewMode === 'raw'" class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
            <div>
              <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                Raw prometheus.yml Editor
              </h2>
              <p class="text-[11px] text-slate-500 dark:text-slate-400">
                Directly edit raw YAML. Changes will automatically update the live preview and pre-flight validation.
              </p>
            </div>
            <button
              @click="parseYamlIntoModel(rawEditorYaml); viewMode = 'visual'; showNotification('success', 'Raw YAML parsed into Visual Builder.')"
              class="px-2.5 py-1 bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 font-semibold rounded text-xs hover:bg-blue-100 cursor-pointer"
            >
              Sync to Visual Builder ▾
            </button>
          </div>

          <div class="relative flex border border-slate-200 dark:border-[#1b2234] rounded-lg overflow-hidden bg-slate-50 dark:bg-[#0a0d14] font-mono text-xs">
            <textarea
              v-model="rawEditorYaml"
              rows="26"
              class="w-full bg-transparent p-3 font-mono text-xs text-slate-900 dark:text-white leading-relaxed focus:outline-none resize-y"
              placeholder="# paste or edit prometheus.yml here"
            ></textarea>
          </div>
        </div>

        <!-- Visual Builder View (shown when viewMode === 'visual') -->
        <div v-else class="space-y-4">
          <!-- Visual Builder Accordion Modules -->
          <div class="space-y-2.5">
            <!-- Accordion 1: Global Settings -->
            <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl overflow-hidden shadow-sm">
            <button
              @click="toggleAccordion('global')"
              class="w-full px-4 py-3.5 flex items-center justify-between text-left hover:bg-slate-50/60 dark:hover:bg-[#121826] transition cursor-pointer"
            >
              <div class="flex items-center gap-3">
                <Globe class="w-4 h-4 text-slate-500 shrink-0" />
                <div>
                  <h3 class="text-xs font-bold text-slate-900 dark:text-white">Global Settings</h3>
                  <p class="text-[11px] text-slate-500 dark:text-slate-400">Scrape and evaluation intervals, external labels, and global configuration.</p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300">
                  {{ Object.keys(globalConfig.external_labels).length + 2 }} settings
                </span>
                <ChevronDown :class="['w-4 h-4 text-slate-400 transition-transform', openAccordion === 'global' ? 'rotate-180' : '']" />
              </div>
            </button>

            <!-- Expanded Global Settings Form -->
            <div v-if="openAccordion === 'global'" class="p-4 border-t border-slate-100 dark:border-[#1b2234] bg-slate-50/40 dark:bg-[#0a0d14] space-y-3 text-xs">
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 uppercase mb-1">Scrape Interval</label>
                  <input
                    v-model="globalConfig.scrape_interval"
                    class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 font-mono text-xs text-slate-900 dark:text-white"
                  />
                </div>
                <div>
                  <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 uppercase mb-1">Evaluation Interval</label>
                  <input
                    v-model="globalConfig.evaluation_interval"
                    class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 font-mono text-xs text-slate-900 dark:text-white"
                  />
                </div>
                <div>
                  <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 uppercase mb-1">Scrape Timeout</label>
                  <input
                    v-model="globalConfig.scrape_timeout"
                    class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 font-mono text-xs text-slate-900 dark:text-white"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Accordion 2: Alertmanager -->
          <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl overflow-hidden shadow-sm">
            <button
              @click="toggleAccordion('alertmanager')"
              class="w-full px-4 py-3.5 flex items-center justify-between text-left hover:bg-slate-50/60 dark:hover:bg-[#121826] transition cursor-pointer"
            >
              <div class="flex items-center gap-3">
                <Bell class="w-4 h-4 text-slate-500 shrink-0" />
                <div>
                  <h3 class="text-xs font-bold text-slate-900 dark:text-white">Alertmanager</h3>
                  <p class="text-[11px] text-slate-500 dark:text-slate-400">Configure alertmanager endpoints and notification settings.</p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300">
                  {{ alertmanagerConfig.endpoints.length }} endpoint{{ alertmanagerConfig.endpoints.length > 1 ? 's' : '' }}
                </span>
                <ChevronDown :class="['w-4 h-4 text-slate-400 transition-transform', openAccordion === 'alertmanager' ? 'rotate-180' : '']" />
              </div>
            </button>

            <div v-if="openAccordion === 'alertmanager'" class="p-4 border-t border-slate-100 dark:border-[#1b2234] bg-slate-50/40 dark:bg-[#0a0d14] space-y-3 text-xs">
              <label class="block text-[10px] font-bold text-slate-600 dark:text-slate-400 uppercase mb-1">Target Endpoints (Comma separated)</label>
              <input
                :value="alertmanagerConfig.endpoints.join(', ')"
                @input="(e: any) => { alertmanagerConfig.endpoints = e.target.value.split(',').map((s: string) => s.trim()).filter(Boolean); }"
                class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 font-mono text-xs text-slate-900 dark:text-white"
                placeholder="alertmanager:9093, 10.0.0.5:9093"
              />
            </div>
          </div>

          <!-- Accordion 3: Rule Files -->
          <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl overflow-hidden shadow-sm">
            <button
              @click="toggleAccordion('rules')"
              class="w-full px-4 py-3.5 flex items-center justify-between text-left hover:bg-slate-50/60 dark:hover:bg-[#121826] transition cursor-pointer"
            >
              <div class="flex items-center gap-3">
                <FileText class="w-4 h-4 text-slate-500 shrink-0" />
                <div>
                  <h3 class="text-xs font-bold text-slate-900 dark:text-white">Rule Files</h3>
                  <p class="text-[11px] text-slate-500 dark:text-slate-400">Load and manage Prometheus recording & alert rule files.</p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300">
                  {{ ruleFiles.length }} rule file{{ ruleFiles.length > 1 ? 's' : '' }}
                </span>
                <ChevronDown :class="['w-4 h-4 text-slate-400 transition-transform', openAccordion === 'rules' ? 'rotate-180' : '']" />
              </div>
            </button>

            <div v-if="openAccordion === 'rules'" class="p-4 border-t border-slate-100 dark:border-[#1b2234] bg-slate-50/40 dark:bg-[#0a0d14] space-y-2 text-xs">
              <div v-for="(rf, idx) in ruleFiles" :key="idx" class="flex items-center gap-2">
                <input
                  v-model="ruleFiles[idx]"
                  class="flex-1 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 font-mono text-xs text-slate-900 dark:text-white"
                />
                <button
                  @click="ruleFiles.splice(idx, 1)"
                  class="p-1.5 text-slate-400 hover:text-rose-500 cursor-pointer"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>
              <button
                @click="ruleFiles.push('new_rules.yml')"
                class="px-2.5 py-1 text-xs text-blue-600 dark:text-blue-400 font-semibold cursor-pointer hover:underline"
              >
                + Add Rule File
              </button>
            </div>
          </div>
        </div>

        <!-- Section: Scrape Jobs Management -->
        <div class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
          <!-- Scrape Jobs Header -->
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-200 dark:border-[#1b2234] pb-3">
            <div class="flex items-center gap-3">
              <Layers class="w-4 h-4 text-slate-500" />
              <div>
                <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                  Scrape Jobs
                </h2>
                <p class="text-[11px] text-slate-500 dark:text-slate-400">
                  Manage scrape configurations for collecting metrics from your targets.
                </p>
              </div>
            </div>

            <div class="flex items-center gap-2">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300">
                {{ scrapeJobs.length }} jobs
              </span>
              <button
                @click="openAddJobModal"
                class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition flex items-center gap-1.5 shadow-sm cursor-pointer"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>Add Job</span>
              </button>
            </div>
          </div>

          <!-- Scrape Jobs Data Table -->
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-slate-50 dark:bg-[#121826] border-y border-slate-200 dark:border-[#1b2234] text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                <tr>
                  <th class="py-2.5 px-3 w-10 text-center">
                    <button
                      @click="expandAllJobs"
                      class="p-1 hover:bg-slate-200 dark:hover:bg-slate-800 rounded text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition cursor-pointer"
                      :title="expandedJobIds.size === scrapeJobs.length ? 'Collapse All Jobs' : 'Expand All Jobs'"
                    >
                      <ChevronDown :class="['w-3.5 h-3.5 transition-transform duration-200', expandedJobIds.size === scrapeJobs.length ? 'rotate-180 text-blue-600 dark:text-blue-400' : '']" />
                    </button>
                  </th>
                  <th class="py-2.5 px-3">Job Name</th>
                  <th class="py-2.5 px-3">Metrics Path</th>
                  <th class="py-2.5 px-3">Scrape Interval</th>
                  <th class="py-2.5 px-3">Targets</th>
                  <th class="py-2.5 px-3">Labels</th>
                  <th class="py-2.5 px-3">Status</th>
                  <th class="py-2.5 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 dark:divide-[#1b2234]/60">
                <template v-for="job in filteredScrapeJobs" :key="job.id">
                  <tr
                    @click="toggleExpandJob(job.id); selectedJobId = job.id"
                    :class="[
                      'transition cursor-pointer select-none',
                      selectedJobId === job.id
                        ? 'bg-blue-50/50 dark:bg-blue-950/20'
                        : 'hover:bg-slate-50/60 dark:hover:bg-[#151c2d]'
                    ]"
                  >
                    <!-- Expand Chevron + Radio Indicator -->
                    <td class="py-2.5 px-3 text-center whitespace-nowrap" @click.stop>
                      <div class="flex items-center gap-1 justify-center">
                        <button
                          @click="toggleExpandJob(job.id)"
                          class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition cursor-pointer"
                          :title="isJobExpanded(job.id) ? 'Collapse targets' : 'Expand targets'"
                        >
                          <ChevronRight
                            :class="[
                              'w-3.5 h-3.5 transition-transform duration-200',
                              isJobExpanded(job.id) ? 'rotate-90 text-blue-600 dark:text-blue-400' : ''
                            ]"
                          />
                        </button>
                        <input
                          type="radio"
                          :checked="selectedJobId === job.id"
                          @change="selectedJobId = job.id"
                          name="selected_job"
                          class="text-blue-600 focus:ring-0 cursor-pointer w-3.5 h-3.5"
                          title="Select job to edit"
                        />
                      </div>
                    </td>

                    <!-- Job Name -->
                    <td class="py-2.5 px-3 font-bold text-slate-900 dark:text-white">
                      <div class="flex items-center gap-2">
                        <span>{{ job.job_name }}</span>
                        <span v-if="isJobExpanded(job.id)" class="text-[10px] text-blue-600 dark:text-blue-400 font-normal">
                          (expanded)
                        </span>
                      </div>
                    </td>

                    <!-- Path -->
                    <td class="py-2.5 px-3 font-mono text-[11px] text-slate-600 dark:text-slate-400">
                      {{ job.metrics_path }}
                    </td>

                    <!-- Interval -->
                    <td class="py-2.5 px-3 text-slate-700 dark:text-slate-300">
                      {{ job.scrape_interval }}
                    </td>

                    <!-- Targets Count Badge (Clickable to expand) -->
                    <td class="py-2.5 px-3">
                      <button
                        @click.stop="toggleExpandJob(job.id)"
                        :class="[
                          'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] font-mono font-bold transition cursor-pointer border',
                          job.targets.length > 0
                            ? 'bg-blue-50 dark:bg-blue-950/40 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-800 hover:bg-blue-100'
                            : 'bg-slate-100 dark:bg-[#1a2233] text-slate-500 dark:text-slate-400 border-slate-200 dark:border-slate-800 hover:bg-slate-200'
                        ]"
                        title="Click to view scraped target IPs"
                      >
                        <span>{{ job.targets.length }}</span>
                        <span class="text-[10px] font-sans font-medium text-slate-500 dark:text-slate-400">target{{ job.targets.length !== 1 ? 's' : '' }}</span>
                      </button>
                    </td>

                    <!-- Labels -->
                    <td class="py-2.5 px-3">
                      <div class="flex flex-wrap gap-1">
                        <span
                          v-for="(val, key) in job.labels"
                          :key="key"
                          class="px-1.5 py-0.2 rounded text-[10px] bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-500/20 font-mono"
                        >
                          {{ key }}: {{ val }}
                        </span>
                        <span v-if="Object.keys(job.labels).length === 0" class="text-slate-400 text-[10px]">-</span>
                      </div>
                    </td>

                    <!-- Status -->
                    <td class="py-2.5 px-3 whitespace-nowrap">
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-500/30">
                        <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                        <span>Healthy</span>
                      </span>
                    </td>

                    <!-- Actions -->
                    <td class="py-2.5 px-3 text-right whitespace-nowrap" @click.stop>
                      <div class="flex items-center justify-end gap-1">
                        <button
                          @click="selectedJobId = job.id; scrollToJobEditor()"
                          class="p-1 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
                          title="Edit Job Details"
                        >
                          <Sliders class="w-3.5 h-3.5" />
                        </button>
                        <button
                          @click="confirmDeleteJob(job)"
                          class="p-1 text-slate-400 hover:text-rose-500 cursor-pointer"
                          title="Delete Job"
                        >
                          <Trash2 class="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>

                  <!-- EXPANDED SUB-ROW: Scraped IPs & Endpoints List -->
                  <tr v-if="isJobExpanded(job.id)" class="bg-slate-50/80 dark:bg-[#070b14]/70 border-b border-slate-200 dark:border-[#1b2234]">
                    <td colspan="8" class="p-3 sm:px-6">
                      <div class="space-y-3 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl p-3.5 shadow-xs animate-in fade-in duration-150">
                        <!-- Sub-row Header -->
                        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-100 dark:border-[#1b2234] pb-2">
                          <div class="flex items-center gap-2 flex-wrap">
                            <span class="text-xs font-bold text-slate-800 dark:text-slate-200">
                              Scraped Target IPs for <strong class="font-mono text-blue-600 dark:text-blue-400">{{ job.job_name }}</strong>:
                            </span>
                            <span class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-400">
                              {{ (job.scheme || 'http').toUpperCase() }} &bull; path: {{ job.metrics_path || '/metrics' }}
                            </span>
                          </div>

                          <div class="flex items-center gap-2">
                            <button
                              @click.stop="quickAddTargetToJob(job)"
                              class="px-2.5 py-1 bg-blue-50 dark:bg-blue-500/10 hover:bg-blue-100 dark:hover:bg-blue-500/20 text-blue-600 dark:text-blue-400 rounded text-[10px] font-bold transition flex items-center gap-1 cursor-pointer"
                            >
                              <Plus class="w-3 h-3" />
                              <span>Add Target IP</span>
                            </button>
                            <button
                              @click.stop="selectedJobId = job.id; scrollToJobEditor()"
                              class="px-2.5 py-1 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-semibold transition flex items-center gap-1 cursor-pointer"
                            >
                              <Sliders class="w-3 h-3 text-slate-400" />
                              <span>Full Job Editor</span>
                            </button>
                          </div>
                        </div>

                        <!-- Target IPs Grid/List -->
                        <div v-if="job.targets.length > 0" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                          <div
                            v-for="(t, tIdx) in job.targets"
                            :key="t.id || tIdx"
                            class="flex items-center justify-between p-2 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/70 dark:bg-[#121826] hover:border-slate-300 dark:hover:border-slate-700 transition"
                          >
                            <div class="flex items-center gap-2 min-w-0 flex-1">
                              <span class="w-2 h-2 rounded-full bg-emerald-500 shrink-0"></span>
                              <div class="min-w-0 flex-1 pr-1">
                                <input
                                  v-model="t.endpoint"
                                  placeholder="e.g. 192.168.1.10:9100"
                                  class="w-full bg-transparent font-mono text-xs font-bold text-slate-900 dark:text-white focus:outline-none focus:bg-white dark:focus:bg-[#0a0d14] px-1 py-0.5 rounded border border-transparent focus:border-blue-500 transition"
                                />
                                <div class="text-[10px] text-slate-400 dark:text-slate-500 font-mono truncate pl-1 flex items-center gap-1">
                                  <span>{{ job.scheme || 'http' }}://{{ t.endpoint || '...' }}{{ job.metrics_path }}</span>
                                  <span v-if="isTargetDuplicate(job, tIdx, t.endpoint)" class="text-rose-500 font-bold text-[9px]">(Duplicate)</span>
                                </div>
                              </div>
                            </div>

                            <button
                              @click.stop="confirmDeleteTarget(job, t)"
                              class="p-1 text-slate-400 hover:text-rose-500 rounded transition cursor-pointer shrink-0"
                              title="Remove IP from Scrape"
                            >
                              <Trash2 class="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </div>

                        <!-- Empty targets state inside expanded sub-row -->
                        <div v-else class="py-4 text-center bg-slate-50 dark:bg-[#070b14] border border-dashed border-slate-200 dark:border-slate-800 rounded-lg space-y-1.5">
                          <p class="text-xs text-slate-500 dark:text-slate-400">
                            No scrape targets configured for <strong>{{ job.job_name }}</strong>.
                          </p>
                          <button
                            @click.stop="quickAddTargetToJob(job)"
                            class="inline-flex items-center gap-1 px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs font-semibold cursor-pointer shadow-xs"
                          >
                            <Plus class="w-3.5 h-3.5" />
                            <span>Add Target IP</span>
                          </button>
                        </div>
                      </div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Section: Edit Scrape Job Details (Card per selected job) -->
        <div v-if="currentJob" id="job-details-editor" class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
          <!-- Job Details Header -->
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
            <div class="flex items-center gap-2">
              <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                Edit Scrape Job: <span class="text-blue-600 dark:text-blue-400 font-mono">{{ currentJob.job_name }}</span>
              </h3>
              <button
                @click="copyYamlToClipboard"
                class="p-1 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
                title="Copy Job Snippet"
              >
                <Copy class="w-3.5 h-3.5" />
              </button>
            </div>

            <div class="flex items-center gap-2">
              <button
                @click="duplicateCurrentJob"
                class="px-2.5 py-1 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded text-[11px] font-semibold transition flex items-center gap-1 cursor-pointer"
              >
                <Copy class="w-3 h-3 text-slate-400" />
                <span>Duplicate Job</span>
              </button>
              <button
                @click="confirmDeleteJob(currentJob)"
                class="px-2.5 py-1 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/20 rounded text-[11px] font-semibold transition flex items-center gap-1 cursor-pointer"
              >
                <Trash2 class="w-3 h-3" />
                <span>Delete Job</span>
              </button>
            </div>
          </div>

          <!-- Basic Job Fields -->
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-12 gap-3.5 text-xs">
            <div class="md:col-span-4">
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Job Name *</label>
              <input
                v-model="currentJob.job_name"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
              <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5">Unique name for this scrape job.</p>
            </div>

            <div class="md:col-span-4">
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Metrics Path</label>
              <input
                v-model="currentJob.metrics_path"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
              <div class="flex items-center gap-1.5 mt-1">
                <span class="text-[10px] text-slate-400">Presets:</span>
                <button
                  type="button"
                  @click="currentJob.metrics_path = '/metrics'"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
                >
                  /metrics
                </button>
                <button
                  type="button"
                  @click="currentJob.metrics_path = '/snmp'"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
                >
                  /snmp
                </button>
                <button
                  type="button"
                  @click="currentJob.metrics_path = '/probe'"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 font-mono cursor-pointer"
                >
                  /probe
                </button>
              </div>
            </div>

            <div class="md:col-span-4">
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Scheme</label>
              <select
                v-model="currentJob.scheme"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
              >
                <option value="http">http</option>
                <option value="https">https</option>
              </select>
            </div>

            <div class="md:col-span-6">
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Scrape Interval</label>
              <input
                v-model="currentJob.scrape_interval"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>

            <div class="md:col-span-6">
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Timeout *</label>
              <input
                v-model="currentJob.scrape_timeout"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <!-- Sub-tabs: Targets, Labels, Parameters, Relabel Configs -->
          <div class="border-b border-slate-200 dark:border-[#1b2234] flex items-center gap-4 text-xs font-semibold pt-2">
            <button
              @click="activeJobTab = 'targets'"
              :class="[
                'pb-2 border-b-2 transition cursor-pointer',
                activeJobTab === 'targets'
                  ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Targets ({{ currentJob.targets.length }})
            </button>
            <button
              @click="activeJobTab = 'labels'"
              :class="[
                'pb-2 border-b-2 transition cursor-pointer',
                activeJobTab === 'labels'
                  ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Labels ({{ Object.keys(currentJob.labels).length }})
            </button>
            <button
              @click="activeJobTab = 'params'"
              :class="[
                'pb-2 border-b-2 transition cursor-pointer',
                activeJobTab === 'params'
                  ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Parameters ({{ currentJob.params ? Object.keys(currentJob.params).length : 0 }})
            </button>
            <button
              @click="activeJobTab = 'relabel'"
              :class="[
                'pb-2 border-b-2 transition cursor-pointer',
                activeJobTab === 'relabel'
                  ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Relabel Configs ({{ currentJob.relabel_configs ? currentJob.relabel_configs.length : 0 }})
            </button>
          </div>

          <!-- Sub-tab Content: TARGETS (with Split Generated YAML for this job) -->
          <div v-if="activeJobTab === 'targets'" class="grid grid-cols-1 md:grid-cols-12 gap-4 items-start pt-1">
            <!-- Left: Static Targets List -->
            <div class="md:col-span-7 space-y-3">
              <div class="flex items-center justify-between">
                <div>
                  <h4 class="text-xs font-bold text-slate-800 dark:text-slate-200">Static Targets</h4>
                  <p class="text-[10px] text-slate-500">List of target endpoints to scrape.</p>
                </div>

                <div class="flex items-center gap-1.5">
                  <button
                    @click="showBulkAddModal = true"
                    class="px-2 py-1 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-semibold transition cursor-pointer"
                  >
                    Bulk Add
                  </button>
                  <button
                    @click="addTargetToCurrentJob"
                    class="px-2.5 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-[10px] font-bold transition flex items-center gap-1 cursor-pointer"
                  >
                    <Plus class="w-3 h-3" />
                    <span>Add Target</span>
                  </button>
                </div>
              </div>

              <!-- Target Rows -->
              <div class="space-y-2">
                <div
                  v-for="(t, idx) in currentJob.targets"
                  :key="t.id"
                  class="space-y-1"
                >
                  <div class="flex items-center gap-2">
                    <span class="text-[10px] font-mono text-slate-400 w-4 text-right">{{ idx + 1 }}</span>
                    <div class="flex-1 relative">
                      <input
                        v-model="t.endpoint"
                        :class="[
                          'w-full bg-slate-50 dark:bg-[#121826] border rounded-lg px-2.5 py-1.5 font-mono text-xs text-slate-900 dark:text-white focus:outline-none',
                          isTargetDuplicateInCurrentJob(idx, t.endpoint)
                            ? 'border-rose-400 dark:border-rose-500/70 bg-rose-50/30 dark:bg-rose-950/20'
                            : 'border-slate-200 dark:border-[#1b2234] focus:border-blue-500'
                        ]"
                        placeholder="localhost:9100"
                      />
                    </div>

                    <!-- Target Inline Labels -->
                    <div class="flex items-center gap-1">
                      <span
                        v-for="(val, k) in t.labels"
                        :key="k"
                        class="px-1.5 py-0.2 rounded text-[10px] bg-slate-100 dark:bg-[#1a2233] text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-800 font-mono"
                      >
                        {{ k }}: {{ val }}
                      </span>
                    </div>

                    <button
                      @click="confirmDeleteTarget(currentJob, t)"
                      class="p-1 text-slate-400 hover:text-rose-500 cursor-pointer"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>

                  <!-- Duplicate Warning Notice -->
                  <div
                    v-if="isTargetDuplicateInCurrentJob(idx, t.endpoint)"
                    class="ml-6 text-[10px] text-rose-600 dark:text-rose-400 font-semibold flex items-center gap-1"
                  >
                    <AlertCircle class="w-3 h-3 text-rose-500" />
                    <span>Duplicate target. This target already exists in earlier rows.</span>
                  </div>
                </div>

                <div v-if="currentJob.targets.length === 0" class="py-6 text-center text-slate-400 text-xs border border-dashed border-slate-200 dark:border-slate-800 rounded-lg">
                  No targets configured. Click <strong>+ Add Target</strong> or <strong>Bulk Add</strong>.
                </div>
              </div>
            </div>

            <!-- Right: Generated YAML (this job only) -->
            <div class="md:col-span-5 bg-slate-50 dark:bg-[#060911] rounded-xl p-3.5 border border-slate-200 dark:border-slate-800 shadow-sm space-y-2">
              <div class="flex items-center justify-between text-slate-500 dark:text-slate-400 text-[10px] uppercase font-bold border-b border-slate-200 dark:border-slate-800/80 pb-1.5">
                <span>Generated YAML (this job)</span>
                <span class="text-blue-600 dark:text-blue-400 font-semibold">Reactive</span>
              </div>
              <pre class="overflow-x-auto text-[11px] leading-relaxed text-slate-800 dark:text-slate-200 font-mono">{{ generateYaml(currentJob) }}</pre>
            </div>
          </div>

          <!-- Sub-tab Content: LABELS -->
          <div v-if="activeJobTab === 'labels'" class="space-y-3 pt-1">
            <p class="text-[11px] text-slate-500">Static labels applied to all targets in this scrape job.</p>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div v-for="(val, key) in currentJob.labels" :key="key" class="flex items-center gap-2 bg-slate-50 dark:bg-[#121826] p-2 rounded-lg border border-slate-200 dark:border-[#1b2234]">
                <span class="font-bold text-xs text-slate-700 dark:text-slate-300 font-mono">{{ key }}:</span>
                <input
                  v-model="currentJob.labels[key]"
                  class="flex-1 bg-white dark:bg-[#0a0d14] border border-slate-200 dark:border-slate-800 rounded px-2 py-1 text-xs font-mono"
                />
                <button @click="delete currentJob.labels[key]" class="text-slate-400 hover:text-rose-500 p-1 cursor-pointer">
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>

          <!-- Sub-tab Content: PARAMETERS (params) -->
          <div v-if="activeJobTab === 'params'" class="space-y-4 pt-1">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h4 class="text-xs font-bold text-slate-800 dark:text-slate-200">Query Parameters (`params`)</h4>
                <p class="text-[10px] text-slate-500 dark:text-slate-400">
                  HTTP GET query parameters sent with scrape requests (required for SNMP Exporter e.g. <code class="font-mono">module: [if_mib]</code> or Blackbox Exporter e.g. <code class="font-mono">module: [http_2xx]</code>).
                </p>
              </div>

              <!-- Quick Presets -->
              <div class="flex items-center gap-1.5 shrink-0 flex-wrap">
                <span class="text-[10px] text-slate-400">Presets:</span>
                <button
                  type="button"
                  @click="quickAddParam('module', 'if_mib')"
                  class="px-2 py-0.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-mono cursor-pointer"
                >
                  + module: if_mib
                </button>
                <button
                  type="button"
                  @click="quickAddParam('module', 'mikrotik')"
                  class="px-2 py-0.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-mono cursor-pointer"
                >
                  + module: mikrotik
                </button>
                <button
                  type="button"
                  @click="quickAddParam('module', 'http_2xx')"
                  class="px-2 py-0.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-mono cursor-pointer"
                >
                  + module: http_2xx
                </button>
              </div>
            </div>

            <!-- Existing Parameters List -->
            <div v-if="currentJob.params && Object.keys(currentJob.params).length > 0" class="space-y-2">
              <div
                v-for="(vals, pKey) in currentJob.params"
                :key="pKey"
                class="p-3 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg space-y-2"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="text-xs font-bold font-mono text-blue-600 dark:text-blue-400">{{ pKey }}</span>
                    <span class="text-[10px] text-slate-400">({{ vals.length }} values)</span>
                  </div>
                  <button
                    @click="deleteParamKey(pKey as string)"
                    class="p-1 text-slate-400 hover:text-rose-500 cursor-pointer"
                    title="Delete Parameter Key"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>

                <div class="flex flex-wrap items-center gap-1.5">
                  <div
                    v-for="(val, idx) in vals"
                    :key="idx"
                    class="flex items-center gap-1 px-2 py-1 bg-white dark:bg-[#0a0d14] border border-slate-200 dark:border-slate-800 rounded text-xs font-mono text-slate-800 dark:text-slate-200"
                  >
                    <span>{{ val }}</span>
                    <button
                      @click="removeParamValue(pKey as string, idx)"
                      class="text-slate-400 hover:text-rose-500 p-0.5 cursor-pointer"
                    >
                      <X class="w-3 h-3" />
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="py-6 text-center text-slate-400 text-xs border border-dashed border-slate-200 dark:border-slate-800 rounded-lg">
              No query parameters defined. Standard exporters do not require parameters. Multi-target exporters use <code class="font-mono">module</code> parameter.
            </div>

            <!-- Add Parameter Input Form -->
            <div class="flex flex-col sm:flex-row items-center gap-2 pt-2 border-t border-slate-100 dark:border-[#1b2234]">
              <input
                v-model="newParamKey"
                placeholder="Parameter key (e.g. module or target)"
                class="w-full sm:w-1/3 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
              <input
                v-model="newParamValue"
                placeholder="Parameter value (e.g. if_mib, http_2xx, mikrotik)"
                class="w-full sm:flex-1 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-1.5 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
              <button
                @click="addParamToCurrentJob"
                class="w-full sm:w-auto px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition flex items-center justify-center gap-1 cursor-pointer shrink-0"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>Add Param</span>
              </button>
            </div>
          </div>

          <!-- Sub-tab Content: RELABEL CONFIGS -->
          <div v-if="activeJobTab === 'relabel'" class="space-y-4 pt-1">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h4 class="text-xs font-bold text-slate-800 dark:text-slate-200">Relabel Configurations (`relabel_configs`)</h4>
                <p class="text-[10px] text-slate-500 dark:text-slate-400">
                  Target relabeling rules executed before scraping. Essential for routing multi-target SNMP and Blackbox probes to the exporter instance.
                </p>
              </div>

              <!-- Presets for SNMP and Blackbox -->
              <div class="flex items-center gap-1.5 shrink-0 flex-wrap">
                <button
                  type="button"
                  @click="applyExporterRelabelPreset('snmp')"
                  class="px-2.5 py-1 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-semibold transition cursor-pointer"
                >
                  SNMP Proxy Rules
                </button>
                <button
                  type="button"
                  @click="applyExporterRelabelPreset('blackbox')"
                  class="px-2.5 py-1 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-semibold transition cursor-pointer"
                >
                  Blackbox Proxy Rules
                </button>
                <button
                  type="button"
                  @click="addEmptyRelabelRule"
                  class="px-2.5 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-[10px] font-bold transition flex items-center gap-1 cursor-pointer"
                >
                  <Plus class="w-3 h-3" />
                  <span>Add Rule</span>
                </button>
              </div>
            </div>

            <!-- List of Relabel Rules -->
            <div v-if="currentJob.relabel_configs && currentJob.relabel_configs.length > 0" class="space-y-2.5">
              <div
                v-for="(rc, rIdx) in currentJob.relabel_configs"
                :key="rIdx"
                class="p-3 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg space-y-2.5"
              >
                <div class="flex items-center justify-between">
                  <span class="text-[11px] font-bold font-mono text-slate-600 dark:text-slate-400">
                    Rule #{{ rIdx + 1 }}
                  </span>
                  <button
                    @click="deleteRelabelRule(rIdx)"
                    class="p-1 text-slate-400 hover:text-rose-500 cursor-pointer"
                    title="Delete Rule"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>

                <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-2 text-xs">
                  <div>
                    <label class="block text-[10px] font-bold text-slate-500 uppercase mb-0.5">Source Labels</label>
                    <input
                      :value="rc.source_labels ? rc.source_labels.join(', ') : ''"
                      @input="(e: any) => rc.source_labels = e.target.value.split(',').map((s: string) => s.trim()).filter(Boolean)"
                      placeholder="e.g. __address__"
                      class="w-full bg-white dark:bg-[#0a0d14] border border-slate-200 dark:border-slate-800 rounded px-2.5 py-1 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label class="block text-[10px] font-bold text-slate-500 uppercase mb-0.5">Target Label</label>
                    <input
                      v-model="rc.target_label"
                      placeholder="e.g. __param_target__ or instance"
                      class="w-full bg-white dark:bg-[#0a0d14] border border-slate-200 dark:border-slate-800 rounded px-2.5 py-1 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label class="block text-[10px] font-bold text-slate-500 uppercase mb-0.5">Replacement</label>
                    <input
                      v-model="rc.replacement"
                      placeholder="e.g. localhost:9116"
                      class="w-full bg-white dark:bg-[#0a0d14] border border-slate-200 dark:border-slate-800 rounded px-2.5 py-1 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label class="block text-[10px] font-bold text-slate-500 uppercase mb-0.5">Action</label>
                    <select
                      v-model="rc.action"
                      class="w-full bg-white dark:bg-[#0a0d14] border border-slate-200 dark:border-slate-800 rounded px-2.5 py-1 text-xs text-slate-900 dark:text-white font-medium focus:outline-none focus:border-blue-500"
                    >
                      <option value="replace">replace (default)</option>
                      <option value="keep">keep</option>
                      <option value="drop">drop</option>
                      <option value="hashmod">hashmod</option>
                      <option value="labelmap">labelmap</option>
                      <option value="labeldrop">labeldrop</option>
                      <option value="labelkeep">labelkeep</option>
                    </select>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="py-6 text-center text-slate-400 text-xs border border-dashed border-slate-200 dark:border-slate-800 rounded-lg">
              No relabel rules defined. Standard direct scrapes do not require relabel configs. Click <strong>SNMP Proxy Rules</strong> or <strong>Blackbox Proxy Rules</strong> to configure multi-target routing.
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- ==================== RIGHT COLUMN: LIVE YAML PREVIEW & VALIDATION (5 Cols) ==================== -->
      <div class="lg:col-span-5 space-y-4">
        <!-- YAML Preview Box -->
        <div class="bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm overflow-hidden flex flex-col">
          <!-- Top Tabs -->
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] px-4 py-2.5 bg-slate-50 dark:bg-[#121826] text-xs">
            <div class="flex items-center gap-2">
              <button
                @click="activeRightTab = 'preview'"
                :class="[
                  'px-2.5 py-1 rounded-md font-semibold transition cursor-pointer flex items-center gap-1.5',
                  activeRightTab === 'preview'
                    ? 'bg-white dark:bg-[#1a2233] text-blue-600 dark:text-blue-400 shadow-sm'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                <Code class="w-3 h-3 text-slate-400" />
                <span>YAML Preview</span>
              </button>

              <button
                @click="activeRightTab = 'diff'"
                :class="[
                  'px-2.5 py-1 rounded-md font-semibold transition cursor-pointer flex items-center gap-1.5',
                  activeRightTab === 'diff'
                    ? 'bg-white dark:bg-[#1a2233] text-blue-600 dark:text-blue-400 shadow-sm'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
                ]"
              >
                <FileDiff class="w-3 h-3 text-slate-400" />
                <span>Diff</span>
              </button>
            </div>

            <div class="flex items-center gap-2">
              <select
                v-model="previewScope"
                class="text-[10px] bg-slate-100 dark:bg-[#1a2233] border border-slate-200 dark:border-slate-800 rounded px-2 py-0.5 font-medium text-slate-700 dark:text-slate-300"
              >
                <option value="full">View: Full Config</option>
                <option value="job">View: This Job Only</option>
              </select>

              <button
                @click="copyYamlToClipboard"
                class="px-2 py-0.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] text-slate-700 dark:text-slate-300 rounded text-[10px] font-semibold transition flex items-center gap-1 cursor-pointer"
                title="Copy YAML"
              >
                <Check v-if="copiedText" class="w-3 h-3 text-emerald-500" />
                <Copy v-else class="w-3 h-3 text-slate-400" />
                <span>{{ copiedText ? 'Copied' : 'Copy' }}</span>
              </button>
            </div>
          </div>

          <!-- Tab 1: Live Line-Numbered YAML Code Block -->
          <div v-if="activeRightTab === 'preview'" class="p-3 bg-slate-50/70 dark:bg-[#060911] border-t border-slate-200 dark:border-[#1b2234] font-mono text-xs overflow-x-auto max-h-[520px] select-text">
            <div class="text-[10px] text-slate-500 dark:text-slate-400 uppercase font-bold pb-2 border-b border-slate-200 dark:border-slate-800/80 mb-2 flex items-center justify-between">
              <span>prometheus.yml (generated)</span>
              <span class="text-[10px] font-normal text-slate-400 dark:text-slate-500 lowercase">{{ formattedLines.length }} lines</span>
            </div>

            <div class="space-y-0.5">
              <div
                v-for="line in formattedLines"
                :key="line.num"
                :class="[
                  'flex items-start gap-2.5 px-1.5 py-0.5 rounded transition font-mono text-[11px]',
                  line.isError
                    ? 'bg-rose-50 dark:bg-rose-950/60 border-l-2 border-rose-500 text-rose-800 dark:text-rose-200'
                    : 'hover:bg-slate-200/50 dark:hover:bg-slate-900/60'
                ]"
              >
                <!-- Line Number with vertical divider -->
                <span class="w-7 text-right text-slate-400 dark:text-slate-600 select-none text-[10px] shrink-0 font-mono pt-0.5 border-r border-slate-200 dark:border-slate-800/60 pr-2">
                  {{ line.num }}
                </span>

                <!-- Line Marker if Error -->
                <span v-if="line.isError" class="text-rose-500 font-bold shrink-0">!</span>

                <!-- Line Content with Color Coding -->
                <span class="whitespace-pre flex-1 select-text">
                  <span
                    v-for="(token, tIdx) in formatYamlTokens(line.content)"
                    :key="tIdx"
                    :class="token.cls"
                  >{{ token.text }}</span>
                </span>
              </div>
            </div>
          </div>

          <!-- Tab 2: Diff Preview -->
          <div v-if="activeRightTab === 'diff'" class="p-3 bg-slate-50/70 dark:bg-[#060911] border-t border-slate-200 dark:border-[#1b2234] font-mono text-xs overflow-x-auto max-h-[520px] select-text space-y-0.5">
            <div class="text-[10px] text-slate-500 dark:text-slate-400 uppercase font-bold pb-2 border-b border-slate-200 dark:border-slate-800/80 mb-2 flex items-center justify-between">
              <span>Diff vs Original Server Config</span>
              <span class="text-emerald-600 dark:text-emerald-400 font-semibold">+ Additions / <span class="text-rose-600 dark:text-rose-400">- Deletions</span></span>
            </div>

            <div
              v-for="(d, idx) in diffLines"
              :key="idx"
              :class="[
                'px-2 py-0.5 font-mono text-[11px] whitespace-pre rounded-sm',
                d.type === 'added' ? 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-800 dark:text-emerald-300 border-l-2 border-emerald-500 font-medium' :
                d.type === 'removed' ? 'bg-rose-50 dark:bg-rose-950/40 text-rose-800 dark:text-rose-300 border-l-2 border-rose-500 line-through opacity-80' :
                'text-slate-600 dark:text-slate-400'
              ]"
            >
              <span class="inline-block w-4 font-bold select-none">{{ d.type === 'added' ? '+' : d.type === 'removed' ? '-' : ' ' }}</span>
              <span>{{ d.text }}</span>
            </div>
          </div>
        </div>


      </div>
    </div>

    <!-- ==================== MODALS ==================== -->

    <!-- Modal 1: Add New Job Modal -->
    <div
      v-if="showAddJobModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        <!-- Modal Header -->
        <div class="flex items-center justify-between px-5 py-3.5 border-b border-slate-100 dark:border-[#1b2234] shrink-0">
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">Add Scrape Job</h3>
            <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">Configure target scrape job, metrics path, and discovery endpoints</p>
          </div>
          <button
            @click="showAddJobModal = false"
            class="text-slate-400 hover:text-slate-600 dark:hover:text-white p-1 rounded-lg hover:bg-slate-100 dark:hover:bg-[#1a2233] cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Modal Body (Scrollable) -->
        <div class="p-5 overflow-y-auto space-y-4 text-xs flex-1">
          <!-- 1. Job Preset / Type -->
          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1.5">Job Preset / Type</label>
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
              <button
                type="button"
                @click="setJobPreset('standard')"
                :class="[
                  'px-3 py-2 rounded-lg border text-xs font-semibold text-left transition cursor-pointer flex flex-col gap-0.5',
                  newJobForm.preset === 'standard'
                    ? 'bg-blue-50 dark:bg-blue-950/30 border-blue-500 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/20'
                    : 'bg-slate-50 dark:bg-[#121826] border-slate-200 dark:border-[#1b2234] text-slate-700 dark:text-slate-300 hover:border-slate-300 dark:hover:border-slate-700'
                ]"
              >
                <span class="font-bold">Standard</span>
                <span class="text-[10px] text-slate-500 dark:text-slate-400">Direct /metrics</span>
              </button>

              <button
                type="button"
                @click="setJobPreset('snmp')"
                :class="[
                  'px-3 py-2 rounded-lg border text-xs font-semibold text-left transition cursor-pointer flex flex-col gap-0.5',
                  newJobForm.preset === 'snmp'
                    ? 'bg-blue-50 dark:bg-blue-950/30 border-blue-500 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/20'
                    : 'bg-slate-50 dark:bg-[#121826] border-slate-200 dark:border-[#1b2234] text-slate-700 dark:text-slate-300 hover:border-slate-300 dark:hover:border-slate-700'
                ]"
              >
                <span class="font-bold">SNMP Exporter</span>
                <span class="text-[10px] text-slate-500 dark:text-slate-400">Switch & Router /snmp</span>
              </button>

              <button
                type="button"
                @click="setJobPreset('blackbox')"
                :class="[
                  'px-3 py-2 rounded-lg border text-xs font-semibold text-left transition cursor-pointer flex flex-col gap-0.5',
                  newJobForm.preset === 'blackbox'
                    ? 'bg-blue-50 dark:bg-blue-950/30 border-blue-500 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/20'
                    : 'bg-slate-50 dark:bg-[#121826] border-slate-200 dark:border-[#1b2234] text-slate-700 dark:text-slate-300 hover:border-slate-300 dark:hover:border-slate-700'
                ]"
              >
                <span class="font-bold">Blackbox Probe</span>
                <span class="text-[10px] text-slate-500 dark:text-slate-400">HTTP/TCP Probe /probe</span>
              </button>

              <button
                type="button"
                @click="setJobPreset('custom')"
                :class="[
                  'px-3 py-2 rounded-lg border text-xs font-semibold text-left transition cursor-pointer flex flex-col gap-0.5',
                  newJobForm.preset === 'custom'
                    ? 'bg-blue-50 dark:bg-blue-950/30 border-blue-500 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/20'
                    : 'bg-slate-50 dark:bg-[#121826] border-slate-200 dark:border-[#1b2234] text-slate-700 dark:text-slate-300 hover:border-slate-300 dark:hover:border-slate-700'
                ]"
              >
                <span class="font-bold">Custom</span>
                <span class="text-[10px] text-slate-500 dark:text-slate-400">Manual Config</span>
              </button>
            </div>
          </div>

          <!-- 2. Job Name -->
          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Job Name *</label>
            <input
              v-model="newJobForm.job_name"
              :placeholder="
                newJobForm.preset === 'snmp'
                  ? 'e.g. snmp_switches or cisco_devices'
                  : newJobForm.preset === 'blackbox'
                  ? 'e.g. http_probe_endpoints'
                  : newJobForm.preset === 'custom'
                  ? 'e.g. custom_service_metrics'
                  : 'e.g. node_exporter or redis_exporter'
              "
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            />
            <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5">
              Unique identifier for this scrape job in prometheus.yml.
            </p>
          </div>

          <!-- 3. Target Endpoints (IP / Host:Port) -->
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300">
                {{
                  newJobForm.preset === 'snmp'
                    ? 'Target Network Devices * (Device IP)'
                    : newJobForm.preset === 'blackbox'
                    ? 'Probe Targets * (URLs or Endpoints)'
                    : 'Target Endpoints * (host:port)'
                }}
              </label>
              <span class="text-[10px] text-slate-500 dark:text-slate-400">One per line or comma-separated</span>
            </div>
            <textarea
              v-model="newJobForm.targets_text"
              rows="3"
              :placeholder="
                newJobForm.preset === 'snmp'
                  ? '192.168.1.1\n192.168.1.254\n10.0.0.1'
                  : newJobForm.preset === 'blackbox'
                  ? 'https://example.com\nhttps://internal.service.local\n10.20.3.1:80'
                  : '10.20.3.5:9100\n10.20.3.6:9100\nlocalhost:9100'
              "
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-3 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            ></textarea>

            <!-- Quick Append from Remote Servers -->
            <div v-if="availableRemoteHosts.length > 0 && (newJobForm.preset === 'standard' || newJobForm.preset === 'custom')" class="space-y-2 pt-1">
              <div class="flex items-center justify-between">
                <span class="flex items-center gap-1.5 text-xs font-bold text-slate-700 dark:text-slate-300">
                  <Server class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
                  Quick add from Remote Servers:
                </span>
                <span class="text-xs text-slate-500 dark:text-slate-400">
                  Click server to append target (:9100)
                </span>
              </div>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-48 overflow-y-auto p-2 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl">
                <button
                  v-for="host in availableRemoteHosts"
                  :key="host.id"
                  type="button"
                  @click="appendRemoteHostTarget(host.host)"
                  class="px-3 py-2 rounded-lg bg-white hover:bg-slate-100 dark:bg-[#161d2d] dark:hover:bg-[#1e273d] border border-slate-200 dark:border-[#26334d] text-slate-800 dark:text-slate-200 transition cursor-pointer flex items-center justify-between gap-2.5 group text-left shadow-2xs"
                  :title="`Click to add ${host.host}:9100 to target endpoints`"
                >
                  <div class="flex items-center gap-2 min-w-0 flex-1">
                    <Plus class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 group-hover:text-slate-700 dark:group-hover:text-slate-200 shrink-0" />
                    <span class="text-xs font-semibold text-slate-800 dark:text-slate-200 truncate">
                      {{ host.name }}
                    </span>
                  </div>
                  <span class="text-xs font-mono text-slate-600 dark:text-slate-400 shrink-0 bg-slate-100 dark:bg-[#0f1422] px-2 py-0.5 rounded border border-slate-200/80 dark:border-[#1e273d]">
                    {{ host.host }}
                  </span>
                </button>
              </div>
            </div>
          </div>

          <!-- 4. Metrics Path & Scheme -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Metrics Path</label>
              <input
                v-model="newJobForm.metrics_path"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
              <div class="flex items-center gap-1.5 mt-1.5 flex-wrap">
                <span class="text-[10px] text-slate-400">Presets:</span>
                <button
                  type="button"
                  @click="newJobForm.metrics_path = '/metrics'"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#253046] text-slate-600 dark:text-slate-300 font-mono transition cursor-pointer"
                >
                  /metrics
                </button>
                <button
                  type="button"
                  @click="newJobForm.metrics_path = '/snmp'"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#253046] text-slate-600 dark:text-slate-300 font-mono transition cursor-pointer"
                >
                  /snmp
                </button>
                <button
                  type="button"
                  @click="newJobForm.metrics_path = '/probe'"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#253046] text-slate-600 dark:text-slate-300 font-mono transition cursor-pointer"
                >
                  /probe
                </button>
                <button
                  type="button"
                  @click="newJobForm.metrics_path = '/actuator/prometheus'"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#253046] text-slate-600 dark:text-slate-300 font-mono transition cursor-pointer"
                >
                  /actuator/prometheus
                </button>
              </div>
            </div>

            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Scheme</label>
              <select
                v-model="newJobForm.scheme"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-medium text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              >
                <option value="http">http</option>
                <option value="https">https</option>
              </select>
            </div>
          </div>

          <!-- 5. Scrape Interval & Timeout -->
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Scrape Interval</label>
              <input
                v-model="newJobForm.scrape_interval"
                placeholder="15s"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Timeout</label>
              <input
                v-model="newJobForm.scrape_timeout"
                placeholder="10s"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <!-- 6. SNMP Configuration Box (Preset SNMP) -->
          <div v-if="newJobForm.preset === 'snmp'" class="p-3.5 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-3">
            <div class="flex items-center justify-between border-b border-slate-200/60 dark:border-[#1b2234] pb-2">
              <span class="text-xs font-bold text-slate-800 dark:text-slate-200">SNMP Exporter Configuration</span>
              <span class="text-[10px] text-slate-500 font-mono">relabeling to localhost:9116</span>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-slate-700 dark:text-slate-300 mb-1">SNMP Module</label>
                <select
                  v-model="newJobForm.snmp_module"
                  class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 text-xs text-slate-900 dark:text-white"
                >
                  <option value="if_mib">if_mib (Network Interfaces)</option>
                  <option value="cisco">cisco (Cisco Devices)</option>
                  <option value="synology">synology (Synology NAS)</option>
                  <option value="apcups">apcups (APC Smart-UPS)</option>
                  <option value="printer">printer (Printers)</option>
                  <option value="custom">custom (Custom Module Name)</option>
                </select>
                <input
                  v-if="newJobForm.snmp_module === 'custom'"
                  v-model="newJobForm.snmp_custom_module"
                  placeholder="Custom module name"
                  class="w-full mt-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1 text-xs font-mono text-slate-900 dark:text-white"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-slate-700 dark:text-slate-300 mb-1">SNMP Exporter Address</label>
                <input
                  v-model="newJobForm.exporter_address"
                  placeholder="localhost:9116"
                  class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 text-xs font-mono text-slate-900 dark:text-white"
                />
                <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5">Address of running snmp_exporter daemon</p>
              </div>
            </div>
          </div>

          <!-- 7. Blackbox Configuration Box (Preset Blackbox) -->
          <div v-if="newJobForm.preset === 'blackbox'" class="p-3.5 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-3">
            <div class="flex items-center justify-between border-b border-slate-200/60 dark:border-[#1b2234] pb-2">
              <span class="text-xs font-bold text-slate-800 dark:text-slate-200">Blackbox Exporter Configuration</span>
              <span class="text-[10px] text-slate-500 font-mono">probe module & proxy relabeling</span>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-slate-700 dark:text-slate-300 mb-1">Probe Module</label>
                <select
                  v-model="newJobForm.blackbox_module"
                  class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 text-xs text-slate-900 dark:text-white"
                >
                  <option value="http_2xx">http_2xx (HTTP 2xx Status)</option>
                  <option value="http_post_2xx">http_post_2xx (HTTP POST Status)</option>
                  <option value="tcp_connect">tcp_connect (TCP Port Check)</option>
                  <option value="icmp">icmp (Ping / ICMP Echo)</option>
                  <option value="ssh_banner">ssh_banner (SSH Banner Handshake)</option>
                  <option value="custom">custom (Custom Probe Module)</option>
                </select>
                <input
                  v-if="newJobForm.blackbox_module === 'custom'"
                  v-model="newJobForm.blackbox_custom_module"
                  placeholder="Custom probe module name"
                  class="w-full mt-1.5 bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1 text-xs font-mono text-slate-900 dark:text-white"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-slate-700 dark:text-slate-300 mb-1">Blackbox Exporter Address</label>
                <input
                  v-model="newJobForm.exporter_address"
                  placeholder="localhost:9115"
                  class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 text-xs font-mono text-slate-900 dark:text-white"
                />
                <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5">Address of running blackbox_exporter daemon</p>
              </div>
            </div>
          </div>

          <!-- 8. Custom Configuration Box (Preset Custom) -->
          <div v-if="newJobForm.preset === 'custom'" class="p-3.5 bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-2">
            <div class="flex items-center justify-between border-b border-slate-200/60 dark:border-[#1b2234] pb-1.5">
              <span class="text-xs font-bold text-slate-800 dark:text-slate-200">Optional Proxy Exporter Relabeling</span>
            </div>
            <div>
              <label class="block text-[11px] font-semibold text-slate-700 dark:text-slate-300 mb-1">Exporter Address (Optional)</label>
              <input
                v-model="newJobForm.exporter_address"
                placeholder="e.g. localhost:9116 (leave blank for direct scrape)"
                class="w-full bg-white dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-2.5 py-1.5 text-xs font-mono text-slate-900 dark:text-white"
              />
              <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5">
                If specified, automatically configures __param_target__ relabeling to proxy through this exporter address.
              </p>
            </div>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="flex items-center justify-end gap-2 px-5 py-3 border-t border-slate-100 dark:border-[#1b2234] bg-slate-50/50 dark:bg-[#0c101a] shrink-0">
          <button
            @click="showAddJobModal = false"
            class="px-3.5 py-1.5 text-xs font-medium text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeAddJob"
            :disabled="!newJobForm.job_name.trim()"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
          >
            Create Job
          </button>
        </div>
      </div>
    </div>

    <!-- Modal 2: Bulk Add Targets Modal -->
    <div
      v-if="showBulkAddModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-lg shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Bulk Add Targets</h3>
          <button @click="showBulkAddModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-2 text-xs">
          <p class="text-slate-500 dark:text-slate-400">
            Paste target host:port endpoints separated by newlines or commas:
          </p>
          <textarea
            v-model="bulkAddText"
            rows="6"
            class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-3 font-mono text-xs text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            placeholder="10.0.0.1:9100&#10;10.0.0.2:9100&#10;192.168.1.50:9100"
          ></textarea>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showBulkAddModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeBulkAdd"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer"
          >
            Import Targets
          </button>
        </div>
      </div>
    </div>

    <!-- Modal 3: Standard Delete Confirmation Modal (Per AGENTS.md) -->
    <div
      v-if="showDeleteJobModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Scrape Job?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ jobToDelete?.job_name }}</strong>? This action cannot be undone.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteJobModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDeleteJob"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer"
          >
            Confirm Delete
          </button>
        </div>
      </div>
    </div>

    <!-- Modal 4: Delete Target Modal (Per AGENTS.md) -->
    <div
      v-if="showDeleteTargetModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
        <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
          <Trash2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete Target Endpoint?</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Remove <strong class="text-slate-800 dark:text-slate-200 font-mono">{{ targetToDelete?.target.endpoint }}</strong> from this job?
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showDeleteTargetModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeDeleteTarget"
            class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer"
          >
            Confirm Delete
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
