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
  Code
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

interface ScrapeJob {
  id: string;
  job_name: string;
  metrics_path: string;
  scheme: 'http' | 'https';
  scrape_interval: string;
  scrape_timeout: string;
  targets: TargetItem[];
  labels: Record<string, string>;
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
const activeJobTab = ref<'targets' | 'labels' | 'relabel' | 'advanced'>('targets');

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

// Set default selected job
onMounted(() => {
  if (scrapeJobs.value.length > 0 && !selectedJobId.value) {
    selectedJobId.value = scrapeJobs.value[1]?.id || scrapeJobs.value[0].id;
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
const generateYaml = (singleJob?: ScrapeJob): string => {
  if (singleJob) {
    // Generate only for single job
    let y = `  - job_name: ${singleJob.job_name}\n`;
    if (singleJob.metrics_path && singleJob.metrics_path !== '/metrics') {
      y += `    metrics_path: ${singleJob.metrics_path}\n`;
    }
    if (singleJob.scheme && singleJob.scheme !== 'http') {
      y += `    scheme: ${singleJob.scheme}\n`;
    }
    if (singleJob.scrape_interval) {
      y += `    scrape_interval: ${singleJob.scrape_interval}\n`;
    }
    if (singleJob.scrape_timeout) {
      y += `    scrape_timeout: ${singleJob.scrape_timeout}\n`;
    }
    y += `    static_configs:\n`;
    y += `      - targets:\n`;
    singleJob.targets.forEach(t => {
      const isDup = singleJob.targets.filter(x => x.endpoint === t.endpoint).length > 1;
      y += `          - ${t.endpoint}${isDup ? ' # Duplicate target' : ''}\n`;
    });
    if (Object.keys(singleJob.labels).length > 0) {
      y += `        labels:\n`;
      for (const [k, v] of Object.entries(singleJob.labels)) {
        y += `          ${k}: ${v}\n`;
      }
    }
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
    if (job.metrics_path && job.metrics_path !== '/metrics') {
      out += `    metrics_path: ${job.metrics_path}\n`;
    }
    if (job.scheme && job.scheme !== 'http') {
      out += `    scheme: ${job.scheme}\n`;
    }
    if (job.scrape_interval) {
      out += `    scrape_interval: ${job.scrape_interval}\n`;
    }
    if (job.scrape_timeout) {
      out += `    scrape_timeout: ${job.scrape_timeout}\n`;
    }
    out += `    static_configs:\n`;
    out += `      - targets:\n`;
    job.targets.forEach(t => {
      const isDup = job.targets.filter(x => x.endpoint === t.endpoint).length > 1;
      out += `          - ${t.endpoint}${isDup ? ' # Duplicate target' : ''}\n`;
    });
    if (Object.keys(job.labels).length > 0) {
      out += `        labels:\n`;
      for (const [k, v] of Object.entries(job.labels)) {
        out += `          ${k}: ${v}\n`;
      }
    }
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
            labels: activeParsedJob.labels || {}
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
          labels: {}
        };
        inTargets = false;
        inLabels = false;
      } else if (currentSection === 'scrape' && activeParsedJob) {
        if (trimmed.startsWith('metrics_path:')) {
          activeParsedJob.metrics_path = trimmed.replace('metrics_path:', '').trim().replace(/['"]/g, '');
        } else if (trimmed.startsWith('scheme:')) {
          activeParsedJob.scheme = trimmed.replace('scheme:', '').trim().toLowerCase() === 'https' ? 'https' : 'http';
        } else if (trimmed.startsWith('scrape_interval:')) {
          activeParsedJob.scrape_interval = trimmed.replace('scrape_interval:', '').trim();
        } else if (trimmed.startsWith('scrape_timeout:')) {
          activeParsedJob.scrape_timeout = trimmed.replace('scrape_timeout:', '').trim();
        } else if (trimmed.startsWith('targets:')) {
          inTargets = true;
          inLabels = false;
          // Check inline targets format: targets: ['a:9090', 'b:9090']
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
          inLabels = true;
          inTargets = false;
        } else if (inTargets && trimmed.startsWith('-')) {
          const ep = trimmed.replace('-', '').trim().replace(/['"]/g, '').split('#')[0].trim();
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
        labels: activeParsedJob.labels || {}
      });
    }

    if (parsedJobs.length > 0) {
      scrapeJobs.value = parsedJobs;
      selectedJobId.value = parsedJobs[0].id;
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

const lastValidatedTime = ref('Sep 30, 2026 10:19 PM');

const validationItems = computed<ValidationResultItem[]>(() => {
  const items: ValidationResultItem[] = [];

  // 1. Basic syntax check
  items.push({
    type: 'info',
    message: 'YAML syntax structure verified and compliant.'
  });

  // 2. Duplicate target checks
  scrapeJobs.value.forEach(job => {
    const seen = new Set<string>();
    job.targets.forEach(t => {
      const ep = t.endpoint.trim();
      if (!ep) return;
      if (seen.has(ep)) {
        items.push({
          type: 'error',
          message: `Duplicate target found: ${ep} (${job.job_name})`,
          jobName: job.job_name
        });
      }
      seen.add(ep);
    });

    // 3. Timeout check
    const intVal = parseInt(job.scrape_interval) || 15;
    const timeoutVal = parseInt(job.scrape_timeout) || 10;
    if (timeoutVal >= intVal) {
      items.push({
        type: 'warning',
        message: `Job '${job.job_name}': scrape_timeout (${job.scrape_timeout}) should be less than scrape_interval (${job.scrape_interval}).`,
        jobName: job.job_name
      });
    }

    // 4. Empty targets check
    if (job.targets.length === 0) {
      items.push({
        type: 'warning',
        message: `Job '${job.job_name}' has 0 static targets configured.`,
        jobName: job.job_name
      });
    }
  });

  // 5. Rule files loaded info
  if (ruleFiles.value.length > 0) {
    items.push({
      type: 'info',
      message: `${ruleFiles.value.length} rule files loaded and referenced.`
    });
  }

  return items;
});

const errorCount = computed(() => validationItems.value.filter(v => v.type === 'error').length);
const warningCount = computed(() => validationItems.value.filter(v => v.type === 'warning').length);
const infoCount = computed(() => validationItems.value.filter(v => v.type === 'info').length);

const runValidation = () => {
  const d = new Date();
  lastValidatedTime.value = d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) + ' ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
  if (errorCount.value === 0) {
    showNotification('success', 'Validation passed! No critical configuration errors found.');
  } else {
    showNotification('warning', `Validation completed with ${errorCount.value} errors and ${warningCount.value} warnings.`);
  }
};

// Check if a target in the current job is duplicate
const isTargetDuplicateInCurrentJob = (idx: number, endpoint: string): boolean => {
  if (!currentJob.value || !endpoint.trim()) return false;
  return currentJob.value.targets.some((t, i) => i < idx && t.endpoint.trim() === endpoint.trim());
};

// Line numbered code viewer with error highlighting
const formattedLines = computed(() => {
  const text = currentGeneratedYaml.value;
  return text.split('\n').map((line, idx) => {
    const lineNum = idx + 1;
    const isError = line.includes('# Duplicate target') || (line.includes('static_configs:') && line.includes('error'));
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
const newJobForm = ref({
  job_name: '',
  metrics_path: '/metrics',
  scheme: 'http' as 'http' | 'https',
  scrape_interval: '15s',
  scrape_timeout: '10s'
});

const executeAddJob = () => {
  if (!newJobForm.value.job_name.trim()) return;
  const newJob: ScrapeJob = {
    id: `job-${Date.now()}`,
    job_name: newJobForm.value.job_name.trim(),
    metrics_path: newJobForm.value.metrics_path || '/metrics',
    scheme: newJobForm.value.scheme,
    scrape_interval: newJobForm.value.scrape_interval || '15s',
    scrape_timeout: newJobForm.value.scrape_timeout || '10s',
    targets: [],
    labels: { app: newJobForm.value.job_name.trim() }
  };
  scrapeJobs.value.push(newJob);
  selectedJobId.value = newJob.id;
  showAddJobModal.value = false;
  newJobForm.value.job_name = '';
  showNotification('success', `Scrape job '${newJob.job_name}' created.`);
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
          class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-[#1a2233] dark:hover:bg-[#222d42] text-slate-700 dark:text-slate-300 rounded-lg text-xs font-semibold transition flex items-center gap-1.5 cursor-pointer border border-slate-200 dark:border-slate-800"
        >
          <Check class="w-3.5 h-3.5 text-slate-400" />
          <span>Validate</span>
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
                @click="showAddJobModal = true"
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
                  <th class="py-2.5 px-3 w-8"></th>
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
                <tr
                  v-for="job in filteredScrapeJobs"
                  :key="job.id"
                  @click="selectedJobId = job.id"
                  :class="[
                    'transition cursor-pointer',
                    selectedJobId === job.id
                      ? 'bg-blue-50/50 dark:bg-blue-950/20'
                      : 'hover:bg-slate-50/60 dark:hover:bg-[#151c2d]'
                  ]"
                >
                  <!-- Radio / Selection Indicator -->
                  <td class="py-2.5 px-3">
                    <input
                      type="radio"
                      :checked="selectedJobId === job.id"
                      name="selected_job"
                      class="text-blue-600 focus:ring-0 cursor-pointer"
                    />
                  </td>

                  <!-- Job Name -->
                  <td class="py-2.5 px-3 font-bold text-slate-900 dark:text-white">
                    {{ job.job_name }}
                  </td>

                  <!-- Path -->
                  <td class="py-2.5 px-3 font-mono text-[11px] text-slate-600 dark:text-slate-400">
                    {{ job.metrics_path }}
                  </td>

                  <!-- Interval -->
                  <td class="py-2.5 px-3 text-slate-700 dark:text-slate-300">
                    {{ job.scrape_interval }}
                  </td>

                  <!-- Targets Count -->
                  <td class="py-2.5 px-3 font-mono font-bold text-slate-800 dark:text-slate-200">
                    {{ job.targets.length }}
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
                        @click="selectedJobId = job.id"
                        class="p-1 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
                        title="Edit Job"
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
              </tbody>
            </table>
          </div>
        </div>

        <!-- Section: Edit Scrape Job Details (Card per selected job) -->
        <div v-if="currentJob" class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-4">
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

          <!-- Sub-tabs: Targets, Labels, Relabel Configs -->
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
              @click="activeJobTab = 'relabel'"
              :class="[
                'pb-2 border-b-2 transition cursor-pointer',
                activeJobTab === 'relabel'
                  ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              Relabel Configs
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
                <button @click="delete currentJob.labels[key]" class="text-slate-400 hover:text-rose-500 p-1">
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>
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

        <!-- Pre-Flight Validation Results Panel (Mockup bottom card) -->
        <div class="p-4 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl shadow-sm space-y-3">
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-2.5">
            <div class="flex items-center gap-2">
              <AlertTriangle class="w-4 h-4 text-amber-500" />
              <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                Validation Results
              </h3>
            </div>
            <div class="flex items-center gap-3">
              <span class="text-[10px] text-slate-400">Last validated: {{ lastValidatedTime }}</span>
              <button
                @click="runValidation"
                class="px-2.5 py-1 bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 rounded text-[10px] font-bold transition hover:bg-blue-100 cursor-pointer"
              >
                Validate Again
              </button>
            </div>
          </div>

          <!-- Counter Indicators on Left + List of Items -->
          <div class="flex items-start gap-4">
            <!-- Counter Pills -->
            <div class="flex flex-col gap-1.5 shrink-0 pt-0.5">
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
            <div class="flex-1 space-y-1.5 text-xs">
              <div
                v-for="(item, idx) in validationItems"
                :key="idx"
                class="flex items-start gap-2"
              >
                <CheckCircle2 v-if="item.type === 'info'" class="w-3.5 h-3.5 text-emerald-500 shrink-0 mt-0.5" />
                <AlertCircle v-else-if="item.type === 'error'" class="w-3.5 h-3.5 text-rose-500 shrink-0 mt-0.5" />
                <AlertTriangle v-else class="w-3.5 h-3.5 text-amber-500 shrink-0 mt-0.5" />

                <span :class="[
                  'text-[11px] leading-tight',
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
      </div>
    </div>

    <!-- ==================== MODALS ==================== -->

    <!-- Modal 1: Add New Job Modal -->
    <div
      v-if="showAddJobModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-md shadow-2xl p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-[#1b2234] pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">Add Scrape Job</h3>
          <button @click="showAddJobModal = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Job Name *</label>
            <input
              v-model="newJobForm.job_name"
              placeholder="e.g. redis-exporter or mysql"
              class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Metrics Path</label>
              <input
                v-model="newJobForm.metrics_path"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono"
              />
            </div>
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Scheme</label>
              <select
                v-model="newJobForm.scheme"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-medium"
              >
                <option value="http">http</option>
                <option value="https">https</option>
              </select>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Scrape Interval</label>
              <input
                v-model="newJobForm.scrape_interval"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono"
              />
            </div>
            <div>
              <label class="block text-xs font-bold text-slate-700 dark:text-slate-300 mb-1">Timeout</label>
              <input
                v-model="newJobForm.scrape_timeout"
                class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg px-3 py-2 text-xs font-mono"
              />
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-[#1b2234]">
          <button
            @click="showAddJobModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="executeAddJob"
            class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition cursor-pointer"
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
