<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import {
  Database,
  Cloud,
  Clock,
  Play,
  Plus,
  Trash2,
  Edit2,
  CheckCircle2,
  AlertCircle,
  Calendar,
  RotateCw,
  Server,
  Folder,
  HardDrive,
  Check,
  X,
  FileText,
  Copy,
  RotateCcw,
  Terminal,
  Download,
  ChevronLeft,
  ChevronRight,
  ShieldAlert,
  Box,
  Layers,
  Eye,
  Sliders,
  Archive,
  ExternalLink,
  Info,
  Search,
} from 'lucide-vue-next';

const router = useRouter();
const authStore = useAuthStore();
const canRead = computed(() => authStore.can('backup', 'read'));
const canManage = computed(() => authStore.can('backup', 'manage'));

// Information Popup State
const activeInfoPopup = ref<{ title: string; description: string } | null>(null);
const openInfoPopup = (title: string, description: string) => {
  activeInfoPopup.value = { title, description };
};

const activeTab = ref<'databases' | 'containers' | 'destinations' | 'schedules' | 'history'>('databases');
const activeDbSubTab = ref<'registered' | 'snapshots'>('registered');

const databases = ref<any[]>([]);
const destinations = ref<any[]>([]);
const schedules = ref<any[]>([]);
const history = ref<any[]>([]);
const containerBackups = ref<any[]>([]);
const dockerConnections = ref<any[]>([]);
const availableContainers = ref<any[]>([]);
const loadingDockerContainers = ref(false);
const containerSearch = ref('');
const loading = ref(false);

// Notification Toast State (Auto-dismiss in 3000ms per AGENTS.md)
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

// Filtered Container Backups
const filteredContainerBackups = computed(() => {
  if (!containerSearch.value.trim()) return containerBackups.value;
  const q = containerSearch.value.toLowerCase();
  return containerBackups.value.filter(
    (b: any) =>
      b.containerName?.toLowerCase().includes(q) ||
      b.filename?.toLowerCase().includes(q) ||
      b.image?.toLowerCase().includes(q) ||
      b.sourceImage?.toLowerCase().includes(q) ||
      b.sourceConnectionName?.toLowerCase().includes(q) ||
      b.sourceConnection?.toLowerCase().includes(q)
  );
});

// Database Backups & Filtering (Excludes docker container entries)
const databaseSearch = ref('');
const databaseBackups = computed(() => {
  return history.value.filter(
    (h: any) => h.dbType !== 'docker' && h.destType !== 'container-docker'
  );
});

const filteredDatabaseBackups = computed(() => {
  const list = databaseBackups.value;
  if (!databaseSearch.value.trim()) return list;
  const q = databaseSearch.value.toLowerCase();
  return list.filter(
    (b: any) =>
      b.dbName?.toLowerCase().includes(q) ||
      b.filename?.toLowerCase().includes(q) ||
      b.dbType?.toLowerCase().includes(q) ||
      b.destType?.toLowerCase().includes(q)
  );
});

const formatFileSize = (bytes: number) => {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`;
};

const formatDateTime = (dateStr: string) => {
  if (!dateStr) return '-';
  try {
    const d = new Date(dateStr);
    return isNaN(d.getTime()) ? dateStr : d.toLocaleString();
  } catch {
    return dateStr;
  }
};

// Generic Pagination Helper
const getVisiblePages = (current: number, total: number) => {
  const delta = 2;
  const range: number[] = [];
  for (let i = Math.max(2, current - delta); i <= Math.min(total - 1, current + delta); i++) {
    range.push(i);
  }
  if (current - delta > 2) {
    range.unshift(-1);
  }
  if (current + delta < total - 1) {
    range.push(-2);
  }
  range.unshift(1);
  if (total > 1 && !range.includes(total)) {
    range.push(total);
  }
  return range;
};

// 1. History Pagination (15 rows per page)
const historyPage = ref(1);
const historyPageSize = ref(15);
const totalHistoryPages = computed(() => {
  return Math.ceil(history.value.length / historyPageSize.value) || 1;
});
const paginatedHistory = computed(() => {
  const start = (historyPage.value - 1) * historyPageSize.value;
  return history.value.slice(start, start + historyPageSize.value);
});
const visibleHistoryPages = computed(() => getVisiblePages(historyPage.value, totalHistoryPages.value));
watch(totalHistoryPages, (newTotal) => {
  if (historyPage.value > newTotal) {
    historyPage.value = Math.max(1, newTotal);
  }
});

// 2. Database Backup Snapshots Pagination (15 rows per page)
const dbBackupsPage = ref(1);
const dbBackupsPageSize = ref(15);
const totalDbBackupsPages = computed(() => {
  return Math.ceil(filteredDatabaseBackups.value.length / dbBackupsPageSize.value) || 1;
});
const paginatedDatabaseBackups = computed(() => {
  const start = (dbBackupsPage.value - 1) * dbBackupsPageSize.value;
  return filteredDatabaseBackups.value.slice(start, start + dbBackupsPageSize.value);
});
const visibleDbBackupsPages = computed(() => getVisiblePages(dbBackupsPage.value, totalDbBackupsPages.value));
watch(totalDbBackupsPages, (newTotal) => {
  if (dbBackupsPage.value > newTotal) {
    dbBackupsPage.value = Math.max(1, newTotal);
  }
});
watch(databaseSearch, () => {
  dbBackupsPage.value = 1;
});

// 3. Docker Container Backups Pagination (15 rows per page)
const containerPage = ref(1);
const containerPageSize = ref(15);
const totalContainerPages = computed(() => {
  return Math.ceil(filteredContainerBackups.value.length / containerPageSize.value) || 1;
});
const paginatedContainerBackups = computed(() => {
  const start = (containerPage.value - 1) * containerPageSize.value;
  return filteredContainerBackups.value.slice(start, start + containerPageSize.value);
});
const visibleContainerPages = computed(() => getVisiblePages(containerPage.value, totalContainerPages.value));
watch(totalContainerPages, (newTotal) => {
  if (containerPage.value > newTotal) {
    containerPage.value = Math.max(1, newTotal);
  }
});
watch(containerSearch, () => {
  containerPage.value = 1;
});

// 4. Cron Schedules Pagination (15 items per page)
const schedulesPage = ref(1);
const schedulesPageSize = ref(15);
const totalSchedulesPages = computed(() => {
  return Math.ceil(schedules.value.length / schedulesPageSize.value) || 1;
});
const paginatedSchedules = computed(() => {
  const start = (schedulesPage.value - 1) * schedulesPageSize.value;
  return schedules.value.slice(start, start + schedulesPageSize.value);
});
const visibleSchedulesPages = computed(() => getVisiblePages(schedulesPage.value, totalSchedulesPages.value));
watch(totalSchedulesPages, (newTotal) => {
  if (schedulesPage.value > newTotal) {
    schedulesPage.value = Math.max(1, newTotal);
  }
});

// 5. Registered Databases Pagination (15 items per page)
const dbPage = ref(1);
const dbPageSize = ref(15);
const totalDbPages = computed(() => {
  return Math.ceil(databases.value.length / dbPageSize.value) || 1;
});
const paginatedDatabases = computed(() => {
  const start = (dbPage.value - 1) * dbPageSize.value;
  return databases.value.slice(start, start + dbPageSize.value);
});
const visibleDbPages = computed(() => getVisiblePages(dbPage.value, totalDbPages.value));
watch(totalDbPages, (newTotal) => {
  if (dbPage.value > newTotal) {
    dbPage.value = Math.max(1, newTotal);
  }
});

// Run Backup Modal
const isRunBackupModalOpen = ref(false);
const runForm = ref({
  dbConfigId: '',
  dbConfigIds: [] as string[],
  destinationId: '',
});

const toggleSelectAllRunDatabases = () => {
  if (runForm.value.dbConfigIds.length === databases.value.length) {
    runForm.value.dbConfigIds = [];
  } else {
    runForm.value.dbConfigIds = databases.value.map((d: any) => d.id);
  }
};

const isRunDatabaseSelected = (id: string) => {
  return runForm.value.dbConfigIds.includes(id);
};

const toggleRunDatabaseSelection = (id: string) => {
  const idx = runForm.value.dbConfigIds.indexOf(id);
  if (idx >= 0) {
    runForm.value.dbConfigIds.splice(idx, 1);
  } else {
    runForm.value.dbConfigIds.push(id);
  }
};

const openRunBackupModal = () => {
  if (runForm.value.dbConfigIds.length === 0 && databases.value.length > 0) {
    runForm.value.dbConfigIds = databases.value.map((d: any) => d.id);
  }
  if (!runForm.value.destinationId && destinations.value.length > 0) {
    runForm.value.destinationId = destinations.value[0].id;
  }
  isRunBackupModalOpen.value = true;
};

// Database Modal & Edit State
const isDbModalOpen = ref(false);
const editingDbId = ref<string | null>(null);
const dbForm = ref({
  name: '',
  dbType: 'postgresql',
  host: 'localhost',
  port: 5432,
  username: 'postgres',
  password: '',
  databaseName: '',
  useSsh: false,
  sshHost: '',
  sshPort: 22,
  sshUser: 'root',
  sshPassword: '',
});

const openAddDBModal = () => {
  editingDbId.value = null;
  testDBResult.value = null;
  dbForm.value = {
    name: '',
    dbType: 'postgresql',
    host: 'localhost',
    port: 5432,
    username: 'postgres',
    password: '',
    databaseName: '',
    useSsh: false,
    sshHost: '',
    sshPort: 22,
    sshUser: 'root',
    sshPassword: '',
  };
  isDbModalOpen.value = true;
};

const openEditDBModal = (db: any) => {
  editingDbId.value = db.id;
  testDBResult.value = null;
  dbForm.value = {
    name: db.name || '',
    dbType: db.dbType || 'postgresql',
    host: db.host || 'localhost',
    port: Number(db.port) || 5432,
    username: db.username || '',
    password: '',
    databaseName: db.databaseName || '',
    useSsh: !!(db.sshHost && db.sshHost.trim()),
    sshHost: db.sshHost || '',
    sshPort: Number(db.sshPort) || 22,
    sshUser: db.sshUser || 'root',
    sshPassword: '',
  };
  isDbModalOpen.value = true;
};

const testingDB = ref(false);
const testDBResult = ref<{ text: string; success: boolean } | null>(null);
const testingDbId = ref<string | null>(null);

watch(() => dbForm.value.dbType, (newType) => {
  if (newType === 'mysql' || newType === 'mariadb') {
    if (dbForm.value.port === 5432) dbForm.value.port = 3306;
    if (dbForm.value.username === 'postgres') dbForm.value.username = 'root';
  } else if (newType === 'postgresql') {
    if (dbForm.value.port === 3306) dbForm.value.port = 5432;
    if (dbForm.value.username === 'root') dbForm.value.username = 'postgres';
  }
});

// Destination Modal & Edit State
const isDestModalOpen = ref(false);
const editingDestId = ref<string | null>(null);
const testingDestId = ref<string | null>(null);
const destForm = ref({
  name: '',
  destType: 'local',
  host: '',
  port: 22,
  username: 'administrator',
  authType: 'password',
  password: '',
  sshKey: '',
  path: '/app/backups',
  bucket: '',
  endpoint: '',
  accessKeyId: '',
  secretAccessKey: '',
  accountId: '',
});

watch(() => destForm.value.destType, (newType) => {
  if (newType === 'local') {
    if (!destForm.value.path || destForm.value.path === '/opt/backups') {
      destForm.value.path = '/app/backups';
    }
  } else if (newType === 'nas' || newType === 'nfs') {
    if (!destForm.value.path || destForm.value.path === '/app/backups') {
      destForm.value.path = '/opt/backups';
    }
  }
});

const openAddDestModal = () => {
  editingDestId.value = null;
  testDestResult.value = null;
  destForm.value = {
    name: '',
    destType: 'local',
    host: '',
    port: 22,
    username: 'administrator',
    authType: 'password',
    password: '',
    sshKey: '',
    path: '/app/backups',
    bucket: '',
    endpoint: '',
    accessKeyId: '',
    secretAccessKey: '',
    accountId: '',
  };
  isDestModalOpen.value = true;
};

const openEditDestModal = (dest: any) => {
  editingDestId.value = dest.id;
  testDestResult.value = null;
  const cfg = dest.config || {};
  destForm.value = {
    name: dest.name || '',
    destType: dest.destType || 'local',
    host: cfg.host || '',
    port: Number(cfg.port) || 22,
    username: cfg.username || 'administrator',
    authType: cfg.authType || 'password',
    password: '',
    sshKey: '',
    path: cfg.path || (dest.destType === 'local' ? '/app/backups' : '/opt/backups'),
    bucket: cfg.bucket || '',
    endpoint: cfg.endpoint || '',
    accessKeyId: cfg.accessKeyId || '',
    secretAccessKey: '',
    accountId: cfg.accountId || '',
  };
  isDestModalOpen.value = true;
};

// Schedule Modal & Edit State
const isScheduleModalOpen = ref(false);
const editingScheduleId = ref<string | null>(null);
const scheduleForm = ref({
  name: '',
  scheduleType: 'database' as 'database' | 'container',
  dbConfigId: '',
  dbConfigIds: [] as string[],
  dockerConnectionId: '',
  containerIds: [] as string[],
  stopDuringBackup: false,
  destinationId: '',
  cronExpression: '0 2 * * *',
  isActive: true,
});

const getContainerRawName = (c: any) => {
  if (!c) return '';
  let name = c.name;
  if (!name && Array.isArray(c.names) && c.names.length > 0) {
    name = c.names[0].replace(/^\//, '');
  }
  return (name || (c.id ? c.id.slice(0, 12) : 'container')).replace(/^\//, '');
};

const scheduleContainerSearch = ref('');
const scheduleDatabaseSearch = ref('');

const filteredScheduleContainers = computed(() => {
  if (!scheduleContainerSearch.value.trim()) return availableContainers.value;
  const q = scheduleContainerSearch.value.toLowerCase().trim();
  return availableContainers.value.filter((c: any) => {
    const name = getContainerRawName(c).toLowerCase();
    const image = (c.image || c.sourceImage || '').toLowerCase();
    const state = (c.state || c.status || '').toLowerCase();
    return name.includes(q) || image.includes(q) || state.includes(q);
  });
});

const filteredScheduleDatabases = computed(() => {
  if (!scheduleDatabaseSearch.value.trim()) return databases.value;
  const q = scheduleDatabaseSearch.value.toLowerCase().trim();
  return databases.value.filter((db: any) => {
    const name = (db.name || '').toLowerCase();
    const host = (db.host || '').toLowerCase();
    const dbName = (db.databaseName || '').toLowerCase();
    const type = (db.dbType || '').toLowerCase();
    return name.includes(q) || host.includes(q) || dbName.includes(q) || type.includes(q);
  });
});

const toggleSelectAllDatabases = () => {
  const currentList = filteredScheduleDatabases.value;
  if (currentList.length === 0) return;
  const allCurrentSelected = currentList.every((d: any) => scheduleForm.value.dbConfigIds.includes(d.id));
  if (allCurrentSelected) {
    scheduleForm.value.dbConfigIds = scheduleForm.value.dbConfigIds.filter(
      (id) => !currentList.some((d: any) => d.id === id)
    );
  } else {
    const newIds = new Set([...scheduleForm.value.dbConfigIds, ...currentList.map((d: any) => d.id)]);
    scheduleForm.value.dbConfigIds = Array.from(newIds);
  }
};

const isDatabaseSelected = (id: string) => {
  return scheduleForm.value.dbConfigIds.includes(id);
};

const toggleDatabaseSelection = (id: string) => {
  const idx = scheduleForm.value.dbConfigIds.indexOf(id);
  if (idx >= 0) {
    scheduleForm.value.dbConfigIds.splice(idx, 1);
  } else {
    scheduleForm.value.dbConfigIds.push(id);
  }
};

const toggleSelectAllContainers = () => {
  const currentList = filteredScheduleContainers.value;
  if (currentList.length === 0) return;
  const allCurrentSelected = currentList.every((c: any) => scheduleForm.value.containerIds.includes(c.id));
  if (allCurrentSelected) {
    scheduleForm.value.containerIds = scheduleForm.value.containerIds.filter(
      (id) => !currentList.some((c: any) => c.id === id)
    );
  } else {
    const newIds = new Set([...scheduleForm.value.containerIds, ...currentList.map((c: any) => c.id)]);
    scheduleForm.value.containerIds = Array.from(newIds);
  }
};

const isContainerSelected = (id: string) => {
  return scheduleForm.value.containerIds.includes(id);
};

const toggleContainerSelection = (id: string) => {
  const idx = scheduleForm.value.containerIds.indexOf(id);
  if (idx >= 0) {
    scheduleForm.value.containerIds.splice(idx, 1);
  } else {
    scheduleForm.value.containerIds.push(id);
  }
};

const openAddScheduleModal = () => {
  editingScheduleId.value = null;
  const defDockerConn = dockerConnections.value.find((c: any) => c.isDefault)?.id || (dockerConnections.value.length > 0 ? dockerConnections.value[0].id : '');
  scheduleForm.value = {
    name: '',
    scheduleType: 'database',
    dbConfigId: databases.value.length > 0 ? databases.value[0].id : '',
    dbConfigIds: databases.value.map((d: any) => d.id),
    dockerConnectionId: defDockerConn,
    containerIds: [],
    stopDuringBackup: false,
    destinationId: destinations.value.length > 0 ? destinations.value[0].id : '',
    cronExpression: '0 2 * * *',
    isActive: true,
  };
  scheduleContainerSearch.value = '';
  scheduleDatabaseSearch.value = '';
  if (defDockerConn && availableContainers.value.length === 0) {
    fetchAvailableContainers(defDockerConn);
  }
  isScheduleModalOpen.value = true;
};

const openEditScheduleModal = (sched: any) => {
  editingScheduleId.value = sched.id;
  let initialDBs: string[] = [];
  if (Array.isArray(sched.dbConfigIds) && sched.dbConfigIds.length > 0) {
    initialDBs = [...sched.dbConfigIds];
  } else if (sched.dbConfigId) {
    initialDBs = [sched.dbConfigId];
  } else if (databases.value.length > 0) {
    initialDBs = [databases.value[0].id];
  }

  const defDockerConn = sched.dockerConnectionId || (dockerConnections.value.find((c: any) => c.isDefault)?.id || (dockerConnections.value.length > 0 ? dockerConnections.value[0].id : ''));
  const sType = sched.scheduleType || (sched.containerIds && sched.containerIds.length > 0 ? 'container' : 'database');

  scheduleForm.value = {
    name: sched.name || '',
    scheduleType: sType,
    dbConfigId: sched.dbConfigId || '',
    dbConfigIds: initialDBs,
    dockerConnectionId: defDockerConn,
    containerIds: Array.isArray(sched.containerIds) ? [...sched.containerIds] : [],
    stopDuringBackup: !!sched.stopDuringBackup,
    destinationId: sched.destinationId || (destinations.value.length > 0 ? destinations.value[0].id : ''),
    cronExpression: sched.cronExpression || '0 2 * * *',
    isActive: sched.isActive !== undefined ? sched.isActive : true,
  };
  scheduleContainerSearch.value = '';
  scheduleDatabaseSearch.value = '';

  if (defDockerConn) {
    fetchAvailableContainers(defDockerConn);
  }
  isScheduleModalOpen.value = true;
};

const fetchAll = async () => {
  loading.value = true;
  try {
    const [dbRes, destRes, schedRes, histRes, contRes, connRes] = await Promise.all([
      axios.get('/api/v1/backup/databases').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/backup/destinations').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/backup/schedules').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/backup/history?limit=1000').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/backup/containers').catch(() => ({ data: { success: false } })),
      axios.get('/api/v1/docker/connections').catch(() => ({ data: { success: false } })),
    ]);

    if (dbRes.data?.success) databases.value = dbRes.data.data || [];
    if (destRes.data?.success) destinations.value = destRes.data.data || [];
    if (schedRes.data?.success) schedules.value = schedRes.data.data || [];
    if (histRes.data?.success) history.value = histRes.data.data?.history || [];
    if (contRes.data?.success) containerBackups.value = contRes.data.data || [];
    if (connRes.data?.success && Array.isArray(connRes.data.data)) {
      dockerConnections.value = connRes.data.data;
    } else if (Array.isArray(connRes.data)) {
      dockerConnections.value = connRes.data;
    } else {
      dockerConnections.value = [];
    }

    // Pre-select defaults for run modal
    if (databases.value.length > 0 && !runForm.value.dbConfigId) {
      runForm.value.dbConfigId = databases.value[0].id;
    }
    if (destinations.value.length > 0 && !runForm.value.destinationId) {
      runForm.value.destinationId = destinations.value[0].id;
    }
  } catch (err) {
    console.error('Failed to load backup data:', err);
  } finally {
    loading.value = false;
  }
};

const fetchDockerConnections = async () => {
  try {
    const res = await axios.get('/api/v1/docker/connections');
    if (res.data?.success && Array.isArray(res.data.data)) {
      dockerConnections.value = res.data.data;
    } else if (Array.isArray(res.data)) {
      dockerConnections.value = res.data;
    }
  } catch (err) {
    console.error('Failed to load docker connections:', err);
  }
};

const getDockerConnectionLabel = (conn: any) => {
  if (!conn) return 'Local Docker Engine (Local Socket)';
  const name = conn.name || 'Docker Engine';
  let details = '';
  if (conn.sshHost) {
    const user = conn.sshUser || 'root';
    const port = conn.sshPort || 22;
    details = `SSH: ${user}@${conn.sshHost}:${port}`;
  } else if (conn.tcpUrl) {
    details = `TCP: ${conn.tcpUrl}`;
  } else if (conn.socketPath) {
    details = `Socket: ${conn.socketPath}`;
  } else if (conn.hostType === 'local') {
    details = 'Local Socket';
  } else {
    details = conn.driver || conn.hostType || 'Remote Host';
  }
  const isDef = conn.isDefault ? ' (Default)' : '';
  return `${name} — ${details}${isDef}`;
};

const runBackupTargetType = ref<'database' | 'container'>('database');

// Backup Container Form State
const isBackupContainerModalOpen = ref(false);
const backupContainerRunning = ref(false);
const backupContainerForm = ref({
  connectionId: '',
  containerId: '',
  backupName: '',
  stopDuringBackup: false,
});

// Restore Container Form State
const isRestoreContainerModalOpen = ref(false);
const restoreContainerRunning = ref(false);
const restoreBackupTarget = ref<any | null>(null);
const restoreForm = ref({
  filename: '',
  targetConnectionId: '',
  newContainerName: '',
  ports: [] as { host: string; container: string }[],
  env: [] as { key: string; value: string }[],
  restartPolicy: 'unless-stopped',
  startAfterRestore: true,
  replaceExisting: true,
});

// Inspect Container Modal State
const isInspectContainerModalOpen = ref(false);
const inspectedContainerBackup = ref<any | null>(null);

// Live CLI Execution Modal State for Backup & Restore
const isCliModalOpen = ref(false);
const cliModalTitle = ref('Execution Console');
const cliModalTarget = ref('');
const cliModalType = ref<'backup' | 'restore'>('restore');
const cliLogs = ref<{ step: string; status: string; message: string; timestamp: string }[]>([]);
const cliIsRunning = ref(false);
const cliIsSuccess = ref(false);
const cliError = ref('');
const cliTimerSec = ref(0);
const cliTerminalRef = ref<HTMLDivElement | null>(null);
const copiedCliLogs = ref(false);
let cliTimerInterval: ReturnType<typeof setInterval> | null = null;

const addCliLog = (step: string, status: string, message: string, timestamp?: string) => {
  cliLogs.value.push({
    step,
    status,
    message,
    timestamp: timestamp || new Date().toTimeString().split(' ')[0],
  });
  if (cliLogs.value.length > 500) {
    cliLogs.value.shift();
  }
  setTimeout(() => {
    if (cliTerminalRef.value) {
      cliTerminalRef.value.scrollTop = cliTerminalRef.value.scrollHeight;
    }
  }, 20);
};

const copyCliLogs = async () => {
  const fullText = cliLogs.value
    .map((l) => `[${l.timestamp}] [${l.step.toUpperCase()}] ${l.message}`)
    .join('\n');
  try {
    await navigator.clipboard.writeText(fullText);
    copiedCliLogs.value = true;
    setTimeout(() => {
      copiedCliLogs.value = false;
    }, 2000);
  } catch (e) {
    console.error('Failed to copy logs', e);
  }
};

const minimizeCliModal = () => {
  isCliModalOpen.value = false;
  showNotification('success', `${cliModalTitle.value} is running in the background. You can monitor progress anytime in Execution History.`);
  setTimeout(() => {
    fetchAll();
  }, 1000);
};

const closeCliModal = () => {
  if (cliIsRunning.value) {
    minimizeCliModal();
    return;
  }
  isCliModalOpen.value = false;
  if (cliTimerInterval) {
    clearInterval(cliTimerInterval);
    cliTimerInterval = null;
  }
};

const fetchAvailableContainers = async (connectionId?: string) => {
  loadingDockerContainers.value = true;
  try {
    const params: any = { all: true };
    if (connectionId) params.connectionId = connectionId;
    const res = await axios.get('/api/v1/docker/containers', { params });
    if (res.data && Array.isArray(res.data)) {
      availableContainers.value = res.data;
    } else if (res.data?.data && Array.isArray(res.data.data)) {
      availableContainers.value = res.data.data;
    } else {
      availableContainers.value = [];
    }
    if (availableContainers.value.length > 0 && !backupContainerForm.value.containerId) {
      backupContainerForm.value.containerId = availableContainers.value[0].id;
    }
  } catch (err) {
    console.error('Failed to load docker containers:', err);
    availableContainers.value = [];
  } finally {
    loadingDockerContainers.value = false;
  }
};

const getContainerDisplayName = (c: any) => {
  let name = c.name;
  if (!name && Array.isArray(c.names) && c.names.length > 0) {
    name = c.names[0].replace(/^\//, '');
  }
  const image = c.image || 'unknown';
  const state = c.state || (c.status?.includes('Up') ? 'running' : 'stopped');
  return `${name || c.id.slice(0, 12)} (${image}) - [${state}]`;
};

const onBackupConnectionChange = () => {
  backupContainerForm.value.containerId = '';
  fetchAvailableContainers(backupContainerForm.value.connectionId);
};

const selectedTargetConnection = computed(() => {
  if (!restoreForm.value.targetConnectionId) {
    return dockerConnections.value.find((c: any) => c.hostType === 'local' || c.isDefault) || null;
  }
  return dockerConnections.value.find((c: any) => c.id === restoreForm.value.targetConnectionId) || null;
});

const openBackupContainerModal = async (preselectedConnId?: string) => {
  if (!dockerConnections.value || dockerConnections.value.length === 0) {
    await fetchDockerConnections();
  }
  backupContainerForm.value = {
    connectionId: preselectedConnId !== undefined ? preselectedConnId : (dockerConnections.value[0]?.id || ''),
    containerId: '',
    backupName: '',
    stopDuringBackup: false,
  };
  fetchAvailableContainers(backupContainerForm.value.connectionId);
  isBackupContainerModalOpen.value = true;
};

const submitBackupContainer = async () => {
  if (!backupContainerForm.value.containerId) {
    showNotification('error', 'Please select a Docker container to back up.');
    return;
  }

  const selectedConn = dockerConnections.value.find((c) => c.id === backupContainerForm.value.connectionId);
  const connName = selectedConn ? selectedConn.name : 'Docker';
  const selectedCont = availableContainers.value.find((c) => c.id === backupContainerForm.value.containerId);
  const contName = selectedCont ? (selectedCont.name || selectedCont.id.substring(0, 12)) : backupContainerForm.value.containerId;

  // Close form modal & open CLI progress modal
  isBackupContainerModalOpen.value = false;
  isRunBackupModalOpen.value = false;

  cliModalTitle.value = 'Container Backup Console';
  cliModalTarget.value = `${contName} on ${connName}`;
  cliModalType.value = 'backup';
  cliLogs.value = [];
  cliIsRunning.value = true;
  cliIsSuccess.value = false;
  cliError.value = '';
  cliTimerSec.value = 0;
  isCliModalOpen.value = true;

  if (cliTimerInterval) clearInterval(cliTimerInterval);
  cliTimerInterval = setInterval(() => {
    cliTimerSec.value++;
  }, 1000);

  addCliLog('init', 'info', `Initiating backup stream for container '${contName}' on host '${connName}'...`);

  try {
    const token = authStore.token || localStorage.getItem('token') || '';
    const payload = {
      connectionId: backupContainerForm.value.connectionId || undefined,
      containerId: backupContainerForm.value.containerId,
      backupName: backupContainerForm.value.backupName?.trim() || undefined,
      customName: backupContainerForm.value.backupName?.trim() || undefined,
      stopDuringBackup: backupContainerForm.value.stopDuringBackup,
    };

    const response = await fetch('/api/v1/backup/containers/run-stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: token ? `Bearer ${token}` : '',
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      let errMsg = `Server returned HTTP ${response.status} ${response.statusText}`;
      try {
        const errJson = await response.json();
        if (errJson.error) errMsg = errJson.error;
      } catch {}
      throw new Error(errMsg);
    }

    if (!response.body) {
      throw new Error('No streaming response body received from server');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split('\n\n');
      buffer = parts.pop() || '';

      for (const part of parts) {
        const lines = part.split('\n');
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const dataStr = line.slice(6).trim();
            if (!dataStr) continue;
            try {
              const item = JSON.parse(dataStr);
              addCliLog(item.step || 'backup', item.status || 'info', item.message || '', item.timestamp);
              if (item.status === 'error') {
                cliError.value = item.message;
              }
              if (item.step === 'complete' && item.status === 'success') {
                cliIsSuccess.value = true;
              }
            } catch {
              addCliLog('backup', 'info', dataStr);
            }
          }
        }
      }
    }

    if (buffer.trim()) {
      const lines = buffer.split('\n');
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const item = JSON.parse(line.slice(6).trim());
            addCliLog(item.step || 'backup', item.status || 'info', item.message || '', item.timestamp);
            if (item.status === 'error') cliError.value = item.message;
            if (item.step === 'complete' && item.status === 'success') cliIsSuccess.value = true;
          } catch {}
        }
      }
    }

    if (!cliError.value) {
      cliIsSuccess.value = true;
      activeTab.value = 'containers';
      await fetchAll();
    }
  } catch (err: any) {
    const msg = err.message || 'Failed to stream backup';
    cliError.value = msg;
    addCliLog('error', 'error', msg);
  } finally {
    cliIsRunning.value = false;
    if (cliTimerInterval) {
      clearInterval(cliTimerInterval);
      cliTimerInterval = null;
    }
  }
};

const openRestoreContainerModal = async (b: any) => {
  try {
    restoreBackupTarget.value = b;

    if (!dockerConnections.value || dockerConnections.value.length === 0) {
      await fetchDockerConnections();
    }

    // Safely extract port bindings
    const portsList: { host: string; container: string }[] = [];
    if (Array.isArray(b.ports)) {
      for (const p of b.ports) {
        if (typeof p === 'object' && p !== null) {
          const h = p.hostPort || p.host || '';
          const c = p.containerPort || p.container || '';
          if (h || c) {
            portsList.push({ host: String(h), container: String(c) });
          }
        } else if (typeof p === 'string') {
          const parts = p.split(':');
          if (parts.length === 2) {
            portsList.push({ host: parts[0], container: parts[1] });
          }
        }
      }
    } else if (b.portBindings && typeof b.portBindings === 'object') {
      for (const [containerPort, hostPort] of Object.entries(b.portBindings)) {
        portsList.push({ host: String(hostPort), container: String(containerPort) });
      }
    }

    // Safely extract environment variables
    const envList: { key: string; value: string }[] = [];
    if (Array.isArray(b.env)) {
      for (const item of b.env) {
        if (typeof item === 'object' && item !== null) {
          if (item.key !== undefined) {
            envList.push({ key: String(item.key), value: String(item.value || '') });
          }
        } else if (typeof item === 'string') {
          const idx = item.indexOf('=');
          if (idx !== -1) {
            envList.push({ key: item.slice(0, idx), value: item.slice(idx + 1) });
          } else {
            envList.push({ key: item, value: '' });
          }
        }
      }
    }

    // Determine target connection default
    let defaultConnId = '';
    const srcId = b.sourceConnectionId || b.sourceConnectionID;
    if (srcId && dockerConnections.value.some((c: any) => c.id === srcId)) {
      defaultConnId = srcId;
    } else if (dockerConnections.value.length > 0) {
      const defConn = dockerConnections.value.find((c: any) => c.isDefault);
      defaultConnId = defConn ? defConn.id : (dockerConnections.value[0]?.id || '');
    }

    restoreForm.value = {
      filename: b.filename,
      targetConnectionId: defaultConnId,
      newContainerName: b.containerName || '',
      ports: portsList,
      env: envList,
      restartPolicy: b.restartPolicy || 'unless-stopped',
      startAfterRestore: true,
      replaceExisting: true,
    };

    isRestoreContainerModalOpen.value = true;
  } catch (err: any) {
    console.error('Failed to open restore container modal:', err);
    showNotification('error', `Failed to open restore options: ${err.message || err}`);
  }
};

const addRestorePort = () => {
  restoreForm.value.ports.push({ host: '', container: '' });
};

const removeRestorePort = (index: number) => {
  restoreForm.value.ports.splice(index, 1);
};

const addRestoreEnv = () => {
  restoreForm.value.env.push({ key: '', value: '' });
};

const removeRestoreEnv = (index: number) => {
  restoreForm.value.env.splice(index, 1);
};

const submitRestoreContainer = async () => {
  if (!restoreForm.value.filename) {
    showNotification('error', 'Backup archive filename is required.');
    return;
  }
  if (!restoreForm.value.newContainerName.trim()) {
    showNotification('error', 'Target container name is required.');
    return;
  }

  const selectedConn = dockerConnections.value.find((c) => c.id === restoreForm.value.targetConnectionId);
  const targetHostName = selectedConn ? selectedConn.name : 'Target Host';
  const contName = restoreForm.value.newContainerName.trim();

  // Close restore modal & open CLI progress modal
  isRestoreContainerModalOpen.value = false;

  cliModalTitle.value = 'Container Restore Console';
  cliModalTarget.value = `${contName} -> ${targetHostName}`;
  cliModalType.value = 'restore';
  cliLogs.value = [];
  cliIsRunning.value = true;
  cliIsSuccess.value = false;
  cliError.value = '';
  cliTimerSec.value = 0;
  isCliModalOpen.value = true;

  if (cliTimerInterval) clearInterval(cliTimerInterval);
  cliTimerInterval = setInterval(() => {
    cliTimerSec.value++;
  }, 1000);

  addCliLog('init', 'info', `Initiating restore pipeline for '${contName}' into host '${targetHostName}'...`);

  try {
    const portBindings: Record<string, string> = {};
    const portsMapping: { hostPort: string; containerPort: string }[] = [];
    for (const p of restoreForm.value.ports) {
      if (p.container?.trim() && p.host?.trim()) {
        portBindings[p.container.trim()] = p.host.trim();
        portsMapping.push({ hostPort: p.host.trim(), containerPort: p.container.trim() });
      }
    }
    const envArray: string[] = [];
    const envMapping: { key: string; value: string }[] = [];
    for (const e of restoreForm.value.env) {
      if (e.key?.trim()) {
        envArray.push(`${e.key.trim()}=${e.value}`);
        envMapping.push({ key: e.key.trim(), value: e.value });
      }
    }

    const payload = {
      filename: restoreForm.value.filename,
      targetConnectionId: restoreForm.value.targetConnectionId || undefined,
      containerName: contName,
      newContainerName: contName,
      ports: portsMapping.length > 0 ? portsMapping : undefined,
      portBindings: Object.keys(portBindings).length > 0 ? portBindings : undefined,
      env: envMapping.length > 0 ? envMapping : undefined,
      envStrings: envArray.length > 0 ? envArray : undefined,
      restartPolicy: restoreForm.value.restartPolicy || 'unless-stopped',
      startAfter: restoreForm.value.startAfterRestore,
      startAfterRestore: restoreForm.value.startAfterRestore,
      replaceExisting: restoreForm.value.replaceExisting,
    };

    const token = authStore.token || localStorage.getItem('token') || '';
    const response = await fetch('/api/v1/backup/containers/restore-stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: token ? `Bearer ${token}` : '',
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      let errMsg = `Server returned HTTP ${response.status} ${response.statusText}`;
      try {
        const errJson = await response.json();
        if (errJson.error) errMsg = errJson.error;
      } catch {}
      throw new Error(errMsg);
    }

    if (!response.body) {
      throw new Error('No streaming response body received from server');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split('\n\n');
      buffer = parts.pop() || '';

      for (const part of parts) {
        const lines = part.split('\n');
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const dataStr = line.slice(6).trim();
            if (!dataStr) continue;
            try {
              const item = JSON.parse(dataStr);
              addCliLog(item.step || 'restore', item.status || 'info', item.message || '', item.timestamp);
              if (item.status === 'error') {
                cliError.value = item.message;
              }
              if (item.step === 'complete' && item.status === 'success') {
                cliIsSuccess.value = true;
              }
            } catch {
              addCliLog('restore', 'info', dataStr);
            }
          }
        }
      }
    }

    if (buffer.trim()) {
      const lines = buffer.split('\n');
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const item = JSON.parse(line.slice(6).trim());
            addCliLog(item.step || 'restore', item.status || 'info', item.message || '', item.timestamp);
            if (item.status === 'error') cliError.value = item.message;
            if (item.step === 'complete' && item.status === 'success') cliIsSuccess.value = true;
          } catch {}
        }
      }
    }

    if (!cliError.value) {
      cliIsSuccess.value = true;
      activeTab.value = 'containers';
      await fetchAll();
    }
  } catch (err: any) {
    const msg = err.message || 'Failed to stream restore';
    cliError.value = msg;
    addCliLog('error', 'error', msg);
  } finally {
    cliIsRunning.value = false;
    if (cliTimerInterval) {
      clearInterval(cliTimerInterval);
      cliTimerInterval = null;
    }
  }
};

const openInspectContainerModal = async (b: any) => {
  inspectedContainerBackup.value = b;
  isInspectContainerModalOpen.value = true;
  try {
    const res = await axios.get(`/api/v1/backup/containers/${encodeURIComponent(b.filename)}/inspect`);
    if (res.data?.success && res.data.data) {
      inspectedContainerBackup.value = res.data.data;
    }
  } catch (err) {
    console.error('Failed to inspect backup file:', err);
  }
};

const downloadContainerBackup = (b: any) => {
  window.open(`/api/v1/backup/containers/${encodeURIComponent(b.filename)}/download`, '_blank');
};

const triggerBackup = async () => {
  const targets = runForm.value.dbConfigIds.length > 0
    ? runForm.value.dbConfigIds
    : (runForm.value.dbConfigId ? [runForm.value.dbConfigId] : []);

  if (targets.length === 0 || !runForm.value.destinationId) {
    alert('Please select at least one Database and a Storage Destination.');
    return;
  }
  try {
    for (const dbId of targets) {
      await axios.post('/api/v1/backup/run', {
        dbConfigId: dbId,
        destinationId: runForm.value.destinationId,
      });
    }
    isRunBackupModalOpen.value = false;
    alert(`Enqueued backup job(s) for ${targets.length} database(s)!`);
    activeTab.value = 'history';
    fetchAll();
  } catch (err: any) {
    alert(`Failed to trigger backup: ${err.response?.data?.error || err.message}`);
  }
};

const saveDBConfig = async () => {
  if (!dbForm.value.name || !dbForm.value.databaseName) {
    alert('Name and Database Name are required.');
    return;
  }
  try {
    const payload: any = {
      name: dbForm.value.name,
      dbType: dbForm.value.dbType,
      host: dbForm.value.host,
      port: Number(dbForm.value.port),
      username: dbForm.value.username,
      databaseName: dbForm.value.databaseName,
    };
    if (editingDbId.value) {
      payload.id = editingDbId.value;
      payload.password = dbForm.value.password ? dbForm.value.password : '********';
    } else {
      payload.password = dbForm.value.password;
    }
    if (dbForm.value.useSsh) {
      payload.sshHost = dbForm.value.sshHost;
      payload.sshPort = Number(dbForm.value.sshPort);
      payload.sshUser = dbForm.value.sshUser;
      payload.sshPassword = dbForm.value.sshPassword ? dbForm.value.sshPassword : (editingDbId.value ? '********' : '');
      payload.sshAuth = 'password';
    }
    const res = await axios.post('/api/v1/backup/databases', payload);
    if (res.data.success) {
      isDbModalOpen.value = false;
      editingDbId.value = null;
      testDBResult.value = null;
      dbForm.value.name = '';
      dbForm.value.password = '';
      dbForm.value.databaseName = '';
      fetchAll();
    }
  } catch (err: any) {
    alert(`Failed to save database config: ${err.response?.data?.error || err.message}`);
  }
};

const testDBConfig = async () => {
  if (!dbForm.value.host || !dbForm.value.databaseName || !dbForm.value.username) {
    alert('Host, Username, and Database Name are required to test connection.');
    return;
  }
  testingDB.value = true;
  testDBResult.value = null;
  try {
    const payload: any = {
      name: dbForm.value.name || 'Test DB',
      dbType: dbForm.value.dbType,
      host: dbForm.value.host,
      port: Number(dbForm.value.port),
      username: dbForm.value.username,
      password: dbForm.value.password ? dbForm.value.password : (editingDbId.value ? '********' : ''),
      databaseName: dbForm.value.databaseName,
    };
    if (editingDbId.value) {
      payload.id = editingDbId.value;
    }
    if (dbForm.value.useSsh) {
      if (!dbForm.value.sshHost || !dbForm.value.sshUser) {
        alert('SSH Host and SSH User are required when SSH tunnel is enabled.');
        testingDB.value = false;
        return;
      }
      payload.sshHost = dbForm.value.sshHost;
      payload.sshPort = Number(dbForm.value.sshPort) || 22;
      payload.sshUser = dbForm.value.sshUser;
      payload.sshPassword = dbForm.value.sshPassword ? dbForm.value.sshPassword : (editingDbId.value ? '********' : '');
      payload.sshAuth = 'password';
    }
    const res = await axios.post('/api/v1/backup/databases/test', payload);
    if (res.data.success) {
      testDBResult.value = { text: res.data.message || 'Database connection successful!', success: true };
    }
  } catch (err: any) {
    testDBResult.value = {
      text: err.response?.data?.error || err.message || 'Connection test failed',
      success: false,
    };
  } finally {
    testingDB.value = false;
  }
};

const testExistingDB = async (db: any) => {
  testingDbId.value = db.id;
  try {
    const res = await axios.post('/api/v1/backup/databases/test', {
      id: db.id,
      name: db.name,
      dbType: db.dbType,
      host: db.host,
      port: Number(db.port),
      username: db.username,
      databaseName: db.databaseName,
      sshHost: db.sshHost,
      sshPort: db.sshPort,
      sshUser: db.sshUser,
      sshAuth: db.sshAuth,
    });
    if (res.data.success) {
      alert(`[${db.name}] ${res.data.message || 'Connection successful!'}`);
    }
  } catch (err: any) {
    alert(`[${db.name}] Connection test failed: ${err.response?.data?.error || err.message}`);
  } finally {
    testingDbId.value = null;
  }
};

const deleteDBConfig = (db: any) => {
  confirmDeleteDB(db);
};

const testExistingDest = async (dest: any) => {
  testingDestId.value = dest.id;
  try {
    const res = await axios.post('/api/v1/backup/destinations/test', {
      id: dest.id,
      name: dest.name,
      destType: dest.destType,
      config: dest.config || {},
    });
    if (res.data.success) {
      alert(`[${dest.name}] ${res.data.message || 'Connection successful!'}`);
    }
  } catch (err: any) {
    alert(`[${dest.name}] Test failed: ${err.response?.data?.error || err.message}`);
  } finally {
    testingDestId.value = null;
  }
};

const testingDest = ref(false);
const testDestResult = ref<{ text: string; success: boolean } | null>(null);

const prepareDestConfigData = () => {
  const configData: any = {};
  if (destForm.value.destType === 'nas' || destForm.value.destType === 'nfs') {
    configData.host = destForm.value.host;
    configData.port = Number(destForm.value.port) || 22;
    configData.username = destForm.value.username;
    configData.authType = destForm.value.authType;
    configData.password = destForm.value.password ? destForm.value.password : (editingDestId.value ? '********' : '');
    configData.sshKey = destForm.value.sshKey ? destForm.value.sshKey : (editingDestId.value ? '********' : '');
    configData.path = destForm.value.path || '/opt/backups';
  } else if (destForm.value.destType === 'local') {
    configData.path = destForm.value.path || '/opt/backups';
  } else {
    let endpoint = destForm.value.endpoint ? destForm.value.endpoint.trim() : '';
    const bucket = destForm.value.bucket ? destForm.value.bucket.trim() : '';
    const accountId = destForm.value.accountId ? destForm.value.accountId.trim() : '';
    // Auto-generate Cloudflare R2 endpoint if endpoint is omitted
    if (!endpoint && accountId && destForm.value.destType === 'r2') {
      endpoint = `https://${accountId}.r2.cloudflarestorage.com`;
    }
    // Clean trailing slashes and bucket suffix if user pasted full S3 API url from Cloudflare
    if (bucket && endpoint.endsWith('/' + bucket)) {
      endpoint = endpoint.slice(0, -(bucket.length + 1));
    }
    endpoint = endpoint.replace(/\/+$/, '');

    configData.bucket = bucket;
    configData.endpoint = endpoint;
    configData.accessKeyId = destForm.value.accessKeyId ? destForm.value.accessKeyId.trim() : '';
    configData.secretAccessKey = destForm.value.secretAccessKey ? destForm.value.secretAccessKey.trim() : (editingDestId.value ? '********' : '');
    configData.accountId = accountId;
  }
  return configData;
};

const testDestination = async () => {
  const configData = prepareDestConfigData();
  testingDest.value = true;
  testDestResult.value = null;
  try {
    const payload: any = {
      name: destForm.value.name || 'Test Destination',
      destType: destForm.value.destType,
      config: configData,
    };
    if (editingDestId.value) {
      payload.id = editingDestId.value;
    }
    const res = await axios.post('/api/v1/backup/destinations/test', payload);
    if (res.data.success) {
      testDestResult.value = { text: res.data.message || 'Storage connection succeeded! Test file uploaded.', success: true };
    }
  } catch (err: any) {
    testDestResult.value = { text: err.response?.data?.error || err.message || 'Connection test failed', success: false };
  } finally {
    testingDest.value = false;
  }
};

const saveDestination = async () => {
  if (!destForm.value.name) {
    alert('Destination name is required.');
    return;
  }
  try {
    const configData = prepareDestConfigData();
    const payload: any = {
      name: destForm.value.name,
      destType: destForm.value.destType,
      config: configData,
    };
    if (editingDestId.value) {
      payload.id = editingDestId.value;
    }
    const res = await axios.post('/api/v1/backup/destinations', payload);
    if (res.data.success) {
      isDestModalOpen.value = false;
      editingDestId.value = null;
      testDestResult.value = null;
      destForm.value.name = '';
      destForm.value.host = '';
      destForm.value.password = '';
      destForm.value.sshKey = '';
      destForm.value.path = '/opt/backups';
      fetchAll();
    }
  } catch (err: any) {
    alert(`Failed to save destination: ${err.response?.data?.error || err.message}`);
  }
};

// HCP Standard Delete Confirmation Modal
const showDeleteModal = ref(false);
const isDeleting = ref(false);
const itemToDelete = ref<{
  type: 'database' | 'destination' | 'schedule' | 'history' | 'container';
  id: string;
  name: string;
} | null>(null);

const confirmDeleteDB = (db: any) => {
  itemToDelete.value = {
    type: 'database',
    id: db.id,
    name: db.name || 'Database Config',
  };
  showDeleteModal.value = true;
};

const confirmDeleteDest = (dest: any) => {
  itemToDelete.value = {
    type: 'destination',
    id: dest.id,
    name: dest.name || 'Storage Destination',
  };
  showDeleteModal.value = true;
};

const confirmDeleteSchedule = (sched: any) => {
  itemToDelete.value = {
    type: 'schedule',
    id: sched.id,
    name: sched.name || 'Cron Schedule',
  };
  showDeleteModal.value = true;
};

const confirmDeleteHistory = (h: any) => {
  itemToDelete.value = {
    type: 'history',
    id: h.id,
    name: h.filename || `Execution #${h.id}`,
  };
  showDeleteModal.value = true;
};

const confirmDeleteContainer = (b: any) => {
  itemToDelete.value = {
    type: 'container',
    id: b.filename,
    name: b.filename || b.containerName || 'Container Snapshot',
  };
  showDeleteModal.value = true;
};

const executeDelete = async () => {
  if (!itemToDelete.value) return;
  isDeleting.value = true;
  const { type, id } = itemToDelete.value;
  try {
    if (type === 'history') {
      await axios.delete(`/api/v1/backup/history/${id}`);
    } else if (type === 'database') {
      await axios.delete(`/api/v1/backup/databases/${id}`);
    } else if (type === 'destination') {
      await axios.delete(`/api/v1/backup/destinations/${id}`);
    } else if (type === 'schedule') {
      await axios.delete(`/api/v1/backup/schedules/${id}`);
    } else if (type === 'container') {
      await axios.delete(`/api/v1/backup/containers/${encodeURIComponent(id)}`);
      showNotification('success', `Container backup '${id}' removed from backup/container-docker.`);
    }
    showDeleteModal.value = false;
    itemToDelete.value = null;
    fetchAll();
  } catch (err: any) {
    showNotification('error', err.response?.data?.error || err.message || 'Failed to delete');
  } finally {
    isDeleting.value = false;
  }
};

const deleteDestination = (dest: any) => {
  confirmDeleteDest(dest);
};

const saveSchedule = async () => {
  if (!scheduleForm.value.name.trim()) {
    showNotification('error', 'Schedule name is required.');
    return;
  }
  if (scheduleForm.value.scheduleType === 'database') {
    if (!scheduleForm.value.dbConfigIds || scheduleForm.value.dbConfigIds.length === 0) {
      showNotification('error', 'Please select at least one database to back up.');
      return;
    }
  } else if (scheduleForm.value.scheduleType === 'container') {
    if (!scheduleForm.value.containerIds || scheduleForm.value.containerIds.length === 0) {
      showNotification('error', 'Please select at least one Docker container to back up.');
      return;
    }
  }
  if (!scheduleForm.value.destinationId) {
    showNotification('error', 'Storage destination is required.');
    return;
  }
  try {
    const payload: any = {
      ...scheduleForm.value,
      dbConfigId: scheduleForm.value.dbConfigIds && scheduleForm.value.dbConfigIds.length > 0 ? scheduleForm.value.dbConfigIds[0] : '',
    };
    if (editingScheduleId.value) {
      payload.id = editingScheduleId.value;
    }
    const res = await axios.post('/api/v1/backup/schedules', payload);
    if (res.data.success) {
      isScheduleModalOpen.value = false;
      editingScheduleId.value = null;
      showNotification('success', 'Backup schedule saved successfully.');
      fetchAll();
    }
  } catch (err: any) {
    showNotification('error', `Failed to save schedule: ${err.response?.data?.error || err.message}`);
  }
};

const deleteSchedule = (sched: any) => {
  confirmDeleteSchedule(sched);
};

const deleteHistoryItem = (h: any) => {
  confirmDeleteHistory(h);
};

const downloadBackupFile = (h: any) => {
  if (h.destType === 'container-docker' || h.dbType === 'docker') {
    window.open(`/api/v1/backup/containers/${encodeURIComponent(h.filename)}/download`, '_blank');
  } else {
    window.open(`/api/v1/backup/history/${h.id}/download`, '_blank');
  }
};

const handleRunSingle = (dbId: string) => {
  runForm.value.dbConfigId = dbId;
  isRunBackupModalOpen.value = true;
};

// Execution History Log Modal
const isLogModalOpen = ref(false);
const selectedHistory = ref<any | null>(null);
const selectedHistoryLogs = ref<{ step: string; status: string; message: string; timestamp: string }[]>([]);
const logCopied = ref(false);
let logPollInterval: ReturnType<typeof setInterval> | null = null;

const openLogModal = async (item: any) => {
  selectedHistory.value = item;
  selectedHistoryLogs.value = [];
  isLogModalOpen.value = true;
  logCopied.value = false;

  if (logPollInterval) {
    clearInterval(logPollInterval);
    logPollInterval = null;
  }

  await fetchItemLogs(item.id);

  if (item.status === 'running') {
    logPollInterval = setInterval(async () => {
      if (!isLogModalOpen.value || !selectedHistory.value) {
        if (logPollInterval) clearInterval(logPollInterval);
        return;
      }
      await fetchItemLogs(selectedHistory.value.id);
    }, 2500);
  }
};

const fetchItemLogs = async (id: string) => {
  try {
    const res = await axios.get(`/api/v1/backup/history/${encodeURIComponent(id)}/logs`);
    if (res.data?.success && res.data.data) {
      selectedHistoryLogs.value = res.data.data.logs || [];
      if (selectedHistory.value) {
        selectedHistory.value.status = res.data.data.status;
        if (res.data.data.rawLog) {
          selectedHistory.value.errorMessage = res.data.data.rawLog;
        }
      }
      if (!res.data.data.running && logPollInterval) {
        clearInterval(logPollInterval);
        logPollInterval = null;
        fetchAll();
      }
    }
  } catch (e) {
    console.error('Failed to fetch history logs:', e);
  }
};

const closeLogModal = () => {
  isLogModalOpen.value = false;
  if (logPollInterval) {
    clearInterval(logPollInterval);
    logPollInterval = null;
  }
};

const copyLogText = () => {
  if (!selectedHistory.value) return;
  const content = generateLogText(selectedHistory.value);
  navigator.clipboard.writeText(content);
  logCopied.value = true;
  setTimeout(() => {
    logCopied.value = false;
  }, 2500);
};

const rerunFromHistory = async (h: any) => {
  if (!h.dbConfigId || !h.destinationId) {
    alert('Source database or destination configuration is not specified in this record.');
    return;
  }
  try {
    const res = await axios.post('/api/v1/backup/run', {
      dbConfigId: h.dbConfigId,
      destinationId: h.destinationId,
    });
    if (res.data.success) {
      alert('Backup job enqueued successfully!');
      isLogModalOpen.value = false;
      setTimeout(() => fetchAll(), 1500);
    }
  } catch (err: any) {
    alert(`Failed to trigger backup: ${err.response?.data?.error || err.message}`);
  }
};

const generateLogText = (h: any) => {
  if (!h) return '';
  if (selectedHistoryLogs.value && selectedHistoryLogs.value.length > 0) {
    return selectedHistoryLogs.value
      .map((l) => `[${l.timestamp}] [${(l.step || 'INFO').toUpperCase()}] ${l.message}`)
      .join('\n');
  }

  const started = h.startedAt ? new Date(h.startedAt).toLocaleString() : 'N/A';
  const completed = h.completedAt ? new Date(h.completedAt).toLocaleString() : 'N/A';
  const duration = (h.startedAt && h.completedAt)
    ? `${((new Date(h.completedAt).getTime() - new Date(h.startedAt).getTime()) / 1000).toFixed(2)}s`
    : 'N/A';

  let log = `=======================================================\n`;
  log += ` HEPHAESTUS BACKUP EXECUTION REPORT\n`;
  log += `=======================================================\n`;
  log += `Job ID       : ${h.id}\n`;
  log += `Target       : ${h.dbName} (${h.dbType})\n`;
  log += `Destination  : ${h.destType}\n`;
  log += `File Archive : ${h.filename}\n`;
  log += `Status       : ${h.status.toUpperCase()}\n`;
  log += `Started At   : ${started}\n`;
  log += `Completed At : ${completed}\n`;
  log += `Duration     : ${duration}\n`;
  log += `Archive Size : ${(h.fileSize / 1024 / 1024).toFixed(2)} MB (${h.fileSize} bytes)\n\n`;

  log += `----------------- EXECUTION LOGS ----------------------\n`;
  if (h.errorMessage) {
    log += `${h.errorMessage}\n`;
  } else if (h.status === 'running') {
    log += `[...] Process is currently in progress on background worker...\n`;
  } else {
    log += `[${completed}] Process completed successfully.\n`;
  }
  log += `=======================================================`;
  return log;
};

onMounted(() => {
  if (canRead.value) {
    fetchAll();
  }
});
</script>

<template>
  <!-- Unauthorized Access Restricted State -->
  <div v-if="!canRead" class="space-y-6 max-w-7xl mx-auto font-sans">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Backup Manager
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Automated multi-database & Docker container backups and disaster recovery.
        </p>
      </div>
    </div>
    <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-8 max-w-md mx-auto text-center space-y-4 shadow-sm mt-12">
      <div class="w-12 h-12 rounded-full bg-slate-100 dark:bg-[#1b2234] text-slate-500 flex items-center justify-center mx-auto">
        <ShieldAlert class="w-6 h-6 text-slate-600 dark:text-slate-400" />
      </div>
      <div class="space-y-1">
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">Access Restricted</h3>
        <p class="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
          You do not have permission to view or manage Backups. Please contact your system administrator to request access.
        </p>
      </div>
      <div class="pt-2">
        <router-link
          to="/"
          class="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition"
        >
          <span>Return to Dashboard</span>
        </router-link>
      </div>
    </div>
  </div>

  <div v-else class="space-y-6 max-w-7xl mx-auto font-sans">    <!-- Header -->
    <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 dark:border-slate-800 pb-4">
      <div class="flex items-center gap-2">
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">Backup Manager</h1>
        <button
          type="button"
          @click="openInfoPopup('Backup Manager', 'Automated database & Docker container backups, cross-host container restore, and multi-storage disaster recovery.')"
          class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
          title="Backup Manager Info"
        >
          <Info class="w-4 h-4" />
        </button>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="fetchAll"
          class="p-2 rounded-lg bg-white dark:bg-[#20242e] hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 transition shadow-sm"
          title="Refresh Backup Status"
        >
          <RotateCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </button>

        <button
          v-if="canManage"
          @click="openRunBackupModal"
          class="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white transition shadow-sm cursor-pointer"
        >
          <Play class="w-3.5 h-3.5 fill-current" />
          <span>RUN BACKUP NOW</span>
        </button>
      </div>
    </div>

    <!-- Navigation Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2 text-xs font-medium overflow-x-auto" role="tablist">
      <button
        role="tab"
        :aria-selected="activeTab === 'databases'"
        @click="activeTab = 'databases'"
        :class="[
          activeTab === 'databases'
            ? 'bg-[#4274D9] text-white border-[#4274D9] font-bold shadow-sm'
            : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/30 border-slate-200 dark:border-transparent bg-slate-50/50 dark:bg-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5 cursor-pointer'
        ]"
      >
        <Database class="w-3.5 h-3.5" />
        <span>Databases ({{ databases.length }})</span>
      </button>

      <button
        role="tab"
        :aria-selected="activeTab === 'containers'"
        @click="activeTab = 'containers'"
        :class="[
          activeTab === 'containers'
            ? 'bg-[#4274D9] text-white border-[#4274D9] font-bold shadow-sm'
            : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/30 border-slate-200 dark:border-transparent bg-slate-50/50 dark:bg-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5 cursor-pointer'
        ]"
      >
        <Box class="w-3.5 h-3.5" />
        <span>Docker Containers ({{ containerBackups.length }})</span>
      </button>

      <button
        role="tab"
        :aria-selected="activeTab === 'destinations'"
        @click="activeTab = 'destinations'"
        :class="[
          activeTab === 'destinations'
            ? 'bg-[#4274D9] text-white border-[#4274D9] font-bold shadow-sm'
            : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/30 border-slate-200 dark:border-transparent bg-slate-50/50 dark:bg-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5 cursor-pointer'
        ]"
      >
        <HardDrive class="w-3.5 h-3.5" />
        <span>Storage Destinations ({{ destinations.length }})</span>
      </button>

      <button
        role="tab"
        :aria-selected="activeTab === 'schedules'"
        @click="activeTab = 'schedules'"
        :class="[
          activeTab === 'schedules'
            ? 'bg-[#4274D9] text-white border-[#4274D9] font-bold shadow-sm'
            : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/30 border-slate-200 dark:border-transparent bg-slate-50/50 dark:bg-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5 cursor-pointer'
        ]"
      >
        <Clock class="w-3.5 h-3.5" />
        <span>Cron Schedules ({{ schedules.length }})</span>
      </button>

      <button
        role="tab"
        :aria-selected="activeTab === 'history'"
        @click="activeTab = 'history'"
        :class="[
          activeTab === 'history'
            ? 'bg-[#4274D9] text-white border-[#4274D9] font-bold shadow-sm'
            : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/30 border-slate-200 dark:border-transparent bg-slate-50/50 dark:bg-transparent',
          'px-3.5 py-1.5 rounded-lg border transition flex items-center gap-1.5 cursor-pointer'
        ]"
      >
        <Calendar class="w-3.5 h-3.5" />
        <span>Execution History ({{ history.length }})</span>
      </button>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 1: DATABASES -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'databases'" class="space-y-4">
      <!-- Sub-Tabs Navigation for Databases Section -->
      <div class="flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2 text-xs font-medium">
        <button
          type="button"
          @click="activeDbSubTab = 'registered'"
          :class="[
            activeDbSubTab === 'registered'
              ? 'bg-slate-900 dark:bg-white text-white dark:text-slate-900 font-bold shadow-sm'
              : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/40 border-slate-200 dark:border-transparent',
            'px-3 py-1.5 rounded-lg border transition flex items-center gap-1.5 cursor-pointer'
          ]"
        >
          <Database class="w-3.5 h-3.5" />
          <span>Registered Databases ({{ databases.length }})</span>
        </button>

        <button
          type="button"
          @click="activeDbSubTab = 'snapshots'"
          :class="[
            activeDbSubTab === 'snapshots'
              ? 'bg-slate-900 dark:bg-white text-white dark:text-slate-900 font-bold shadow-sm'
              : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/40 border-slate-200 dark:border-transparent',
            'px-3 py-1.5 rounded-lg border transition flex items-center gap-1.5 cursor-pointer'
          ]"
        >
          <Archive class="w-3.5 h-3.5" />
          <span>Database Backup Snapshots ({{ filteredDatabaseBackups.length }})</span>
        </button>
      </div>

      <!-- SUB-TAB 1.1: REGISTERED DATABASES -->
      <div v-if="activeDbSubTab === 'registered'" class="space-y-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-1.5">
            <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Registered Databases</h2>
            <button
              type="button"
              @click="openInfoPopup('Registered Databases', 'Configured database targets (PostgreSQL, MySQL, MariaDB) registered for automated cron jobs and on-demand dumps. Dumps can be stored locally or shipped to remote destinations (NAS, S3, Cloudflare R2).')"
              class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
              title="Database Targets Info"
            >
              <Info class="w-3.5 h-3.5" />
            </button>
          </div>
          <button
            v-if="canManage"
            @click="openAddDBModal"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow-sm cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Database</span>
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div
            v-for="db in paginatedDatabases"
            :key="db.id"
            class="p-4 rounded-xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700 space-y-3 shadow-sm transition"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Database class="w-4 h-4 text-blue-600 dark:text-brand-400" />
                <h4 class="text-xs font-bold text-slate-900 dark:text-white">{{ db.name }}</h4>
              </div>
              <span class="text-[10px] uppercase font-mono px-2 py-0.5 rounded bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-brand-400 border border-blue-200 dark:border-slate-700/60 font-semibold">
                {{ db.dbType }}
              </span>
            </div>

            <p class="text-[11px] font-mono text-slate-700 dark:text-slate-400 bg-slate-50 dark:bg-[#0f1219] p-2 rounded border border-slate-200 dark:border-slate-800/80 truncate">
              {{ db.username }}@{{ db.host }}:{{ db.port }}/{{ db.databaseName }}
            </p>

            <div class="flex items-center justify-between pt-1 border-t border-slate-100 dark:border-slate-800/80">
              <div class="flex items-center gap-2">
                <button
                  v-if="canManage"
                  @click="handleRunSingle(db.id)"
                  class="flex items-center gap-1 text-[11px] font-bold px-2.5 py-1 rounded-md transition cursor-pointer text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800"
                >
                  <Play class="w-3 h-3 fill-current" />
                  <span>Backup Now</span>
                </button>
                <button
                  @click="testExistingDB(db)"
                  :disabled="testingDbId === db.id"
                  class="flex items-center gap-1 text-[11px] font-semibold px-2.5 py-1 rounded-md transition cursor-pointer disabled:opacity-50 text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800"
                  title="Test Connection"
                >
                  <RotateCw class="w-3 h-3" :class="{ 'animate-spin': testingDbId === db.id }" />
                  <span>{{ testingDbId === db.id ? 'Testing...' : 'Test' }}</span>
                </button>
              </div>

              <div v-if="canManage" class="flex items-center gap-1">
                <button
                  @click="openEditDBModal(db)"
                  class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-blue-600 dark:text-slate-400 dark:hover:text-blue-400 transition cursor-pointer"
                  title="Edit Database Config"
                >
                  <Edit2 class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="deleteDBConfig(db)"
                  class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-rose-600 dark:text-slate-400 dark:hover:text-rose-400 transition cursor-pointer"
                  title="Delete Database Config"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>

          <!-- Empty state -->
          <div v-if="databases.length === 0 && !loading" class="col-span-3 p-12 text-center bg-white dark:bg-[#171a23] border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-xl space-y-2 shadow-sm">
            <Database class="w-8 h-8 text-slate-400 dark:text-slate-600 mx-auto mb-2" />
            <p class="text-sm font-bold text-slate-800 dark:text-slate-300">No Databases Configured</p>
            <p class="text-xs text-slate-600 dark:text-slate-500 max-w-sm mx-auto">
              Click "Add Database" above to configure a PostgreSQL, MySQL, or MariaDB target for automated backups.
            </p>
          </div>
        </div>

        <!-- Pagination for Registered Databases (Limit 15 rows) -->
        <div
          v-if="databases.length > dbPageSize"
          class="px-4 py-3 border border-slate-200 dark:border-slate-800 rounded-xl flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-slate-600 dark:text-slate-400 bg-white dark:bg-[#171a23] font-sans shadow-sm"
        >
          <div>
            Showing
            <span class="font-semibold text-slate-900 dark:text-white">{{ (dbPage - 1) * dbPageSize + 1 }}</span>
            to
            <span class="font-semibold text-slate-900 dark:text-white">{{ Math.min(dbPage * dbPageSize, databases.length) }}</span>
            of
            <span class="font-semibold text-slate-900 dark:text-white">{{ databases.length }}</span>
            databases
          </div>

          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="dbPage--"
              :disabled="dbPage === 1"
              class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
            >
              Previous
            </button>
            <div class="flex items-center gap-1">
              <button
                v-for="p in getVisiblePages(dbPage, totalDbPages)"
                :key="p"
                type="button"
                @click="typeof p === 'number' && (dbPage = p)"
                :disabled="typeof p !== 'number'"
                :class="[
                  p === dbPage
                    ? 'bg-blue-600 text-white font-bold'
                    : 'bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-800',
                  'w-7 h-7 rounded-lg text-xs flex items-center justify-center transition',
                  typeof p !== 'number' ? 'cursor-default border-none !bg-transparent text-slate-400' : 'cursor-pointer'
                ]"
              >
                {{ p }}
              </button>
            </div>
            <button
              type="button"
              @click="dbPage++"
              :disabled="dbPage >= totalDbPages"
              class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
            >
              Next
            </button>
          </div>
        </div>
      </div>

      <!-- SUB-TAB 1.2: DATABASE BACKUP SNAPSHOTS -->
      <div v-if="activeDbSubTab === 'snapshots'" class="space-y-4">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div class="flex items-center gap-1.5">
            <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Database Backup Snapshots</h2>
            <button
              type="button"
              @click="openInfoPopup('Database Backup Snapshots', 'Database dump archives (.sql.gz) generated through scheduled cron jobs or on-demand backups. Includes options to inspect execution details, download archives, and manage retention.')"
              class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
              title="Database Snapshots Info"
            >
              <Info class="w-3.5 h-3.5" />
            </button>
          </div>
          <div class="flex items-center gap-2">
            <input
              v-model="databaseSearch"
              type="text"
              placeholder="Filter database snapshots..."
              class="px-3 py-1.5 text-xs rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#171a23] text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 w-48 sm:w-64"
            />
          </div>
        </div>

        <!-- Database Backups Table -->
        <div v-if="filteredDatabaseBackups.length > 0" class="bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-800 rounded-xl overflow-x-auto shadow-sm">
          <table class="w-full text-left text-xs font-mono">
            <thead class="bg-slate-50 dark:bg-[#1c202b] text-slate-700 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
              <tr>
                <th class="p-3">Archive File</th>
                <th class="p-3">Database</th>
                <th class="p-3">Status</th>
                <th class="p-3">Destination</th>
                <th class="p-3">Size</th>
                <th class="p-3">Created At</th>
                <th class="p-3 w-28 text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-slate-700 dark:text-slate-300 text-[11px]">
              <tr v-for="b in paginatedDatabaseBackups" :key="b.id" class="hover:bg-slate-50/80 dark:hover:bg-slate-800/40 transition">
                <td class="p-3 font-sans">
                  <div class="flex items-center gap-2">
                    <div class="p-1.5 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 shrink-0">
                      <Archive class="w-4 h-4" />
                    </div>
                    <div class="truncate max-w-xs font-mono text-xs font-semibold text-slate-900 dark:text-white" :title="b.filename">
                      {{ b.filename }}
                    </div>
                  </div>
                </td>
                <td class="p-3 font-sans">
                  <div class="flex items-center gap-1.5">
                    <span class="font-bold text-xs text-slate-900 dark:text-white font-mono">{{ b.dbName }}</span>
                    <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[9px] font-semibold bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 uppercase">
                      {{ b.dbType }}
                    </span>
                  </div>
                </td>
                <td class="p-3">
                  <span
                    v-if="b.status === 'failed'"
                    class="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-rose-50 dark:bg-rose-950/40 text-rose-600 dark:text-rose-400 border border-rose-200 dark:border-rose-900/50 inline-flex items-center gap-1"
                  >
                    <AlertCircle class="w-3 h-3" />
                    <span>FAILED</span>
                  </span>
                  <span
                    v-else-if="b.status === 'running'"
                    class="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400 border border-amber-200 dark:border-amber-900/50 animate-pulse inline-flex items-center gap-1"
                  >
                    <RotateCw class="w-3 h-3 animate-spin" />
                    <span>RUNNING</span>
                  </span>
                  <span
                    v-else
                    class="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-900/50 inline-flex items-center gap-1"
                  >
                    <CheckCircle2 class="w-3 h-3" />
                    <span>SUCCESS</span>
                  </span>
                </td>
                <td class="p-3 font-sans">
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 uppercase">
                    <HardDrive class="w-3 h-3 text-slate-400" />
                    <span>{{ b.destType }}</span>
                  </span>
                </td>
                <td class="p-3 text-slate-600 dark:text-slate-400 font-mono">
                  <span class="font-semibold">{{ (b.fileSize / 1024 / 1024).toFixed(2) }} MB</span>
                </td>
                <td class="p-3 text-slate-600 dark:text-slate-400 font-sans">
                  {{ formatDateTime(b.startedAt) }}
                </td>
                <td class="p-3 text-right">
                  <div class="flex items-center justify-end gap-1.5">
                    <button
                      @click="openLogModal(b)"
                      class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white transition cursor-pointer"
                      title="View Backup Details & Execution Log"
                    >
                      <Eye class="w-3.5 h-3.5" />
                    </button>
                    <button
                      v-if="b.status === 'success'"
                      @click="downloadBackupFile(b)"
                      class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-emerald-600 dark:text-slate-400 dark:hover:text-emerald-400 transition cursor-pointer"
                      title="Download Database Archive (.sql.gz)"
                    >
                      <Download class="w-3.5 h-3.5" />
                    </button>
                    <button
                      v-if="canManage"
                      @click="deleteHistoryItem(b)"
                      class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-rose-600 dark:text-slate-400 dark:hover:text-rose-400 transition cursor-pointer"
                      title="Delete Database Backup"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>

          <!-- Pagination Bar for Database Backup Snapshots (15 rows limit) -->
          <div
            v-if="filteredDatabaseBackups.length > dbBackupsPageSize"
            class="px-4 py-3 border-t border-slate-200 dark:border-slate-800 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-slate-600 dark:text-slate-400 bg-slate-50/60 dark:bg-[#1c202b]/60 font-sans"
          >
            <div>
              Showing
              <span class="font-semibold text-slate-900 dark:text-white">{{ (dbBackupsPage - 1) * dbBackupsPageSize + 1 }}</span>
              to
              <span class="font-semibold text-slate-900 dark:text-white">{{ Math.min(dbBackupsPage * dbBackupsPageSize, filteredDatabaseBackups.length) }}</span>
              of
              <span class="font-semibold text-slate-900 dark:text-white">{{ filteredDatabaseBackups.length }}</span>
              snapshots
            </div>

            <div class="flex items-center gap-1.5">
              <button
                type="button"
                @click="dbBackupsPage--"
                :disabled="dbBackupsPage === 1"
                class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
              >
                Previous
              </button>
              <div class="flex items-center gap-1">
                <button
                  v-for="p in getVisiblePages(dbBackupsPage, totalDbBackupsPages)"
                  :key="p"
                  type="button"
                  @click="typeof p === 'number' && (dbBackupsPage = p)"
                  :disabled="typeof p !== 'number'"
                  :class="[
                    p === dbBackupsPage
                      ? 'bg-blue-600 text-white font-bold'
                      : 'bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-800',
                    'w-7 h-7 rounded-lg text-xs flex items-center justify-center transition',
                    typeof p !== 'number' ? 'cursor-default border-none !bg-transparent text-slate-400' : 'cursor-pointer'
                  ]"
                >
                  {{ p }}
                </button>
              </div>
              <button
                type="button"
                @click="dbBackupsPage++"
                :disabled="dbBackupsPage >= totalDbBackupsPages"
                class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
              >
                Next
              </button>
            </div>
          </div>
        </div>

        <!-- Empty state when no snapshots -->
        <div v-else class="p-8 text-center bg-white dark:bg-[#171a23] border border-dashed border-slate-200 dark:border-slate-800 rounded-xl space-y-1 shadow-sm">
          <Database class="w-6 h-6 text-slate-400 dark:text-slate-600 mx-auto mb-1" />
          <p class="text-xs font-bold text-slate-700 dark:text-slate-300">No Database Snapshots Found</p>
          <p class="text-[11px] text-slate-500 dark:text-slate-400">
            Run a database backup using "Backup Now" or wait for scheduled cron executions to generate snapshots.
          </p>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB: DOCKER CONTAINERS -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'containers'" class="space-y-4">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div class="flex items-center gap-1.5">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Docker Container Snapshots</h2>
          <button
            type="button"
            @click="openInfoPopup('Docker Container Snapshots', 'OCI container image archives (.tar.gz) stored in backup/container-docker with complete runtime metadata (environment variables, port mappings, labels, and restart policy) for seamless cross-host restoration.')"
            class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
            title="Snapshots Info"
          >
            <Info class="w-3.5 h-3.5" />
          </button>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="containerSearch"
            type="text"
            placeholder="Filter container snapshots..."
            class="px-3 py-1.5 text-xs rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#171a23] text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 w-48 sm:w-64"
          />
          <button
            v-if="canManage"
            @click="openBackupContainerModal()"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow-sm cursor-pointer shrink-0"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Backup Container</span>
          </button>
        </div>
      </div>

      <!-- Container Backups Table -->
      <div v-if="filteredContainerBackups.length > 0" class="bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-800 rounded-xl overflow-x-auto shadow-sm">
        <table class="w-full text-left text-xs font-mono">
          <thead class="bg-slate-50 dark:bg-[#1c202b] text-slate-700 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
            <tr>
              <th class="p-3">Container / Archive</th>
              <th class="p-3">Source Host</th>
              <th class="p-3">Base Image</th>
              <th class="p-3">Size</th>
              <th class="p-3">Created At</th>
              <th class="p-3 w-40 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-slate-700 dark:text-slate-300 text-[11px]">
            <tr v-for="b in paginatedContainerBackups" :key="b.filename" class="hover:bg-slate-50/80 dark:hover:bg-slate-800/40 transition">
              <td class="p-3 font-sans">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 shrink-0">
                    <Box class="w-4 h-4" />
                  </div>
                  <div>
                    <div class="font-bold text-xs text-slate-900 dark:text-white font-mono">{{ b.containerName }}</div>
                    <div class="text-[10px] text-slate-400 font-mono truncate max-w-xs" :title="b.filename">{{ b.filename }}</div>
                  </div>
                </div>
              </td>
              <td class="p-3 font-sans">
                <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700">
                  <Server class="w-3 h-3 text-slate-400" />
                  <span>{{ b.sourceConnectionName || b.sourceConnection || 'Local Docker' }}</span>
                </span>
              </td>
              <td class="p-3">
                <span class="font-mono text-slate-600 dark:text-slate-400 truncate block max-w-[220px]" :title="b.image || b.sourceImage">
                  {{ b.image || b.sourceImage || '-' }}
                </span>
              </td>
              <td class="p-3 text-slate-600 dark:text-slate-400">
                <span class="font-semibold">{{ formatFileSize(b.fileSize) }}</span>
              </td>
              <td class="p-3 text-slate-600 dark:text-slate-400 font-sans">
                {{ formatDateTime(b.createdAt) }}
              </td>
              <td class="p-3 text-right">
                <div class="flex items-center justify-end gap-1.5">
                  <button
                    v-if="canManage"
                    @click="openRestoreContainerModal(b)"
                    class="flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white shadow-sm transition cursor-pointer"
                    title="Restore Container to Any Docker Host"
                  >
                    <RotateCcw class="w-3 h-3" />
                    <span>Restore</span>
                  </button>
                  <button
                    @click="openInspectContainerModal(b)"
                    class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white transition cursor-pointer"
                    title="Inspect Snapshot Metadata & Ports"
                  >
                    <Eye class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="downloadContainerBackup(b)"
                    class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-emerald-600 dark:text-slate-400 dark:hover:text-emerald-400 transition cursor-pointer"
                    title="Download Archive (.tar.gz)"
                  >
                    <Download class="w-3.5 h-3.5" />
                  </button>
                  <button
                    v-if="canManage"
                    @click="confirmDeleteContainer(b)"
                    class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-rose-600 dark:text-slate-400 dark:hover:text-rose-400 transition cursor-pointer"
                    title="Delete Container Backup"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination Bar for Docker Containers (15 rows limit) -->
        <div
          v-if="filteredContainerBackups.length > containerPageSize"
          class="px-4 py-3 border-t border-slate-200 dark:border-slate-800 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-slate-600 dark:text-slate-400 bg-slate-50/60 dark:bg-[#1c202b]/60 font-sans"
        >
          <div>
            Showing
            <span class="font-semibold text-slate-900 dark:text-white">{{ (containerPage - 1) * containerPageSize + 1 }}</span>
            to
            <span class="font-semibold text-slate-900 dark:text-white">{{ Math.min(containerPage * containerPageSize, filteredContainerBackups.length) }}</span>
            of
            <span class="font-semibold text-slate-900 dark:text-white">{{ filteredContainerBackups.length }}</span>
            container snapshots
          </div>

          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="containerPage--"
              :disabled="containerPage === 1"
              class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
            >
              Previous
            </button>
            <div class="flex items-center gap-1">
              <button
                v-for="p in getVisiblePages(containerPage, totalContainerPages)"
                :key="p"
                type="button"
                @click="typeof p === 'number' && (containerPage = p)"
                :disabled="typeof p !== 'number'"
                :class="[
                  p === containerPage
                    ? 'bg-blue-600 text-white font-bold'
                    : 'bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-800',
                  'w-7 h-7 rounded-lg text-xs flex items-center justify-center transition',
                  typeof p !== 'number' ? 'cursor-default border-none !bg-transparent text-slate-400' : 'cursor-pointer'
                ]"
              >
                {{ p }}
              </button>
            </div>
            <button
              type="button"
              @click="containerPage++"
              :disabled="containerPage >= totalContainerPages"
              class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
            >
              Next
            </button>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="containerBackups.length === 0 && !loading" class="p-12 text-center bg-white dark:bg-[#171a23] border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-xl space-y-3 shadow-sm">
        <div class="w-12 h-12 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center mx-auto text-slate-400">
          <Box class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <p class="text-sm font-bold text-slate-800 dark:text-slate-200">No Docker Container Backups Yet</p>
          <p class="text-xs text-slate-500 dark:text-slate-400 max-w-md mx-auto">
            Take snapshot backups of running or stopped containers across any Docker connection. Images and configurations will be securely archived into <code class="font-mono text-blue-600 dark:text-blue-400">backup/container-docker</code>.
          </p>
        </div>
        <div v-if="canManage" class="pt-2">
          <button
            @click="openBackupContainerModal()"
            class="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-bold transition shadow-sm cursor-pointer"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Create Container Backup</span>
          </button>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 2: STORAGE DESTINATIONS -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'destinations'" class="space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1.5">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Storage Destinations</h2>
          <button
            type="button"
            @click="openInfoPopup('Storage Destinations', 'Configured storage repositories (Local Filesystem, Cloudflare R2, AWS S3, MinIO, or NAS) used for storing and archiving database dumps and backups.')"
            class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
            title="Storage Destinations Info"
          >
            <Info class="w-3.5 h-3.5" />
          </button>
        </div>
        <button
          v-if="canManage"
          @click="openAddDestModal"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow-sm cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Destination</span>
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div
          v-for="dest in destinations"
          :key="dest.id"
          class="p-4 rounded-xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700 space-y-3 shadow-sm transition"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <HardDrive v-if="dest.destType === 'nas' || dest.destType === 'nfs'" class="w-4 h-4 text-slate-600 dark:text-slate-400" />
              <Folder v-else-if="dest.destType === 'local'" class="w-4 h-4 text-slate-600 dark:text-slate-400" />
              <Cloud v-else class="w-4 h-4 text-slate-600 dark:text-slate-400" />
              <h4 class="text-xs font-bold text-slate-900 dark:text-white">{{ dest.name }}</h4>
            </div>
            <span class="text-[10px] uppercase font-mono px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-700/60 font-semibold">
              {{ dest.destType === 'nas' ? 'NAS (SSH)' : dest.destType }}
            </span>
          </div>

          <div class="space-y-1">
            <p class="text-[11px] font-mono text-slate-700 dark:text-slate-400 bg-slate-50 dark:bg-[#0f1219] p-2 rounded border border-slate-200 dark:border-slate-800/80 truncate">
              {{ (dest.destType === 'nas' || dest.destType === 'nfs') ? (dest.config?.host ? `${dest.config.host}:${dest.config.path || '/opt/backups'}` : (dest.config?.path || '/opt/backups')) : (dest.destType === 'local' ? (dest.config?.path || '/app/backups') : (dest.config?.bucket || 'S3 Bucket')) }}
            </p>
            <p v-if="dest.destType === 'local'" class="text-[10px] text-blue-600 dark:text-blue-400 flex items-center gap-1 font-medium px-1">
              <span>Host storage directory:</span>
              <code class="font-mono bg-blue-50 dark:bg-blue-950/40 px-1 rounded">./backups</code>
            </p>
          </div>

          <div class="flex items-center justify-between pt-1 border-t border-slate-100 dark:border-slate-800/80">
            <button
              @click="testExistingDest(dest)"
              :disabled="testingDestId === dest.id"
              class="flex items-center gap-1 text-[11px] font-semibold px-2.5 py-1 rounded-md transition cursor-pointer disabled:opacity-50 text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 bg-slate-100 dark:bg-slate-800/50 hover:bg-slate-200 dark:hover:bg-slate-700/50"
              title="Test Connection"
            >
              <RotateCw class="w-3 h-3" :class="{ 'animate-spin': testingDestId === dest.id }" />
              <span>{{ testingDestId === dest.id ? 'Testing...' : 'Test' }}</span>
            </button>

            <div v-if="canManage" class="flex items-center gap-1">
              <button
                @click="openEditDestModal(dest)"
                class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-blue-600 dark:text-slate-400 dark:hover:text-blue-400 transition cursor-pointer"
                title="Edit Storage Destination"
              >
                <Edit2 class="w-3.5 h-3.5" />
              </button>
              <button
                @click="deleteDestination(dest)"
                class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-rose-600 dark:text-slate-400 dark:hover:text-rose-400 transition cursor-pointer"
                title="Delete Storage Destination"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>

        <div v-if="destinations.length === 0 && !loading" class="col-span-3 p-12 text-center bg-white dark:bg-[#171a23] border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-xl space-y-2 shadow-sm">
          <Cloud class="w-8 h-8 text-slate-400 dark:text-slate-600 mx-auto mb-2" />
          <p class="text-sm font-bold text-slate-800 dark:text-slate-300">No Storage Destinations Configured</p>
          <p class="text-xs text-slate-600 dark:text-slate-500 max-w-sm mx-auto">
            Add a Local folder, Cloudflare R2, or AWS S3 destination to store backup archives.
          </p>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 3: CRON SCHEDULES -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'schedules'" class="space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1.5">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Cron Schedules</h2>
          <button
            type="button"
            @click="openInfoPopup('Cron Schedules', 'Automated background cron backup jobs scheduled to run periodically for databases or Docker containers, with configurable retention and remote destination syncing.')"
            class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
            title="Cron Schedules Info"
          >
            <Info class="w-3.5 h-3.5" />
          </button>
        </div>
        <button
          v-if="canManage"
          @click="openAddScheduleModal"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-blue-600 hover:bg-blue-500 text-white font-bold transition shadow-sm cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Schedule</span>
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div
          v-for="sched in paginatedSchedules"
          :key="sched.id"
          class="p-4 rounded-xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700 space-y-3 shadow-sm transition"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <h4 class="text-xs font-bold text-slate-900 dark:text-white">{{ sched.name }}</h4>
              <span
                v-if="sched.scheduleType === 'container'"
                class="text-[9px] uppercase font-mono px-1.5 py-0.5 rounded bg-cyan-50 dark:bg-cyan-950/40 text-cyan-700 dark:text-cyan-300 border border-cyan-200 dark:border-cyan-800 font-bold"
              >
                Docker Container
              </span>
              <span
                v-else
                class="text-[9px] uppercase font-mono px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-950/40 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800 font-bold"
              >
                Database
              </span>
            </div>
            <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-amber-50 dark:bg-slate-800 text-amber-700 dark:text-amber-400 border border-amber-200 dark:border-slate-700/60 font-semibold">
              {{ sched.cronExpression }}
            </span>
          </div>

          <div class="text-[11px] text-slate-600 dark:text-slate-400 flex items-center justify-between">
            <span>Last Run: {{ sched.lastRun ? new Date(sched.lastRun).toLocaleString() : 'Never' }}</span>
            <span :class="sched.isActive ? 'text-emerald-600 dark:text-emerald-400 font-semibold' : 'text-slate-400 dark:text-slate-500'">
              {{ sched.isActive ? '● Active' : '○ Paused' }}
            </span>
          </div>

          <div class="space-y-1.5 pt-1.5 border-t border-slate-100 dark:border-slate-800/80">
            <!-- If Container Schedule -->
            <template v-if="sched.scheduleType === 'container'">
              <div class="flex items-center gap-1 text-[10px] text-slate-500">
                <span>Docker Host:</span>
                <span class="font-semibold text-slate-700 dark:text-slate-300 font-mono">
                  {{ dockerConnections.find(c => c.id === sched.dockerConnectionId)?.name || 'Local Docker' }}
                </span>
              </div>
              <div class="flex flex-wrap gap-1 items-center">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 mr-1">Containers:</span>
                <template v-if="sched.containerIds && sched.containerIds.length > 0">
                  <span
                    v-for="cid in sched.containerIds"
                    :key="cid"
                    class="text-[10px] px-1.5 py-0.5 rounded bg-cyan-50 dark:bg-cyan-950/40 text-cyan-800 dark:text-cyan-200 border border-cyan-200 dark:border-cyan-800 font-mono font-medium truncate max-w-[140px]"
                    :title="cid"
                  >
                    {{ cid }}
                  </span>
                </template>
                <span v-else class="text-[10px] text-slate-400 font-italic">No containers selected</span>
              </div>
            </template>

            <!-- If Database Schedule -->
            <template v-else>
              <div class="flex flex-wrap gap-1 items-center">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 mr-1">Databases:</span>
                <span
                  v-if="sched.dbConfigIds && sched.dbConfigIds.length > 0 && sched.dbConfigIds.length === databases.length"
                  class="text-[10px] px-2 py-0.5 rounded bg-blue-50 dark:bg-blue-950/50 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-900 font-bold"
                >
                  All Databases ({{ databases.length }})
                </span>
                <template v-else-if="sched.dbConfigIds && sched.dbConfigIds.length > 0">
                  <span
                    v-for="dbId in sched.dbConfigIds"
                    :key="dbId"
                    class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 font-medium"
                  >
                    {{ databases.find(d => d.id === dbId)?.name || dbId }}
                  </span>
                </template>
                <span
                  v-else
                  class="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 font-medium"
                >
                  {{ databases.find(d => d.id === sched.dbConfigId)?.name || sched.dbConfigId }}
                </span>
              </div>
            </template>

            <div class="flex items-center gap-1 text-[10px] text-slate-500">
              <span>Destination:</span>
              <span class="font-semibold text-slate-700 dark:text-slate-300">
                {{ destinations.find(d => d.id === sched.destinationId)?.name || 'Local Storage' }}
              </span>
            </div>
          </div>

          <div v-if="canManage" class="flex items-center justify-end gap-1 pt-1 border-t border-slate-100 dark:border-slate-800/80">
            <button
              @click="openEditScheduleModal(sched)"
              class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-blue-600 dark:text-slate-400 dark:hover:text-blue-400 transition cursor-pointer"
              title="Edit Cron Schedule"
            >
              <Edit2 class="w-3.5 h-3.5" />
            </button>
            <button
              @click="deleteSchedule(sched)"
              class="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-rose-600 dark:text-slate-400 dark:hover:text-rose-400 transition cursor-pointer"
              title="Delete Schedule"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <div v-if="schedules.length === 0 && !loading" class="col-span-2 p-12 text-center bg-white dark:bg-[#171a23] border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-xl space-y-2 shadow-sm">
          <Clock class="w-8 h-8 text-slate-400 dark:text-slate-600 mx-auto mb-2" />
          <p class="text-sm font-bold text-slate-800 dark:text-slate-300">No Scheduled Backups</p>
          <p class="text-xs text-slate-600 dark:text-slate-500 max-w-sm mx-auto">
            Set up a cron schedule to automatically dump and upload databases or containers at scheduled times.
          </p>
        </div>
      </div>

      <!-- Pagination Bar for Cron Schedules (15 rows limit) -->
      <div
        v-if="schedules.length > schedulesPageSize"
        class="px-4 py-3 border border-slate-200 dark:border-slate-800 rounded-xl flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-slate-600 dark:text-slate-400 bg-white dark:bg-[#171a23] font-sans shadow-sm"
      >
        <div>
          Showing
          <span class="font-semibold text-slate-900 dark:text-white">{{ (schedulesPage - 1) * schedulesPageSize + 1 }}</span>
          to
          <span class="font-semibold text-slate-900 dark:text-white">{{ Math.min(schedulesPage * schedulesPageSize, schedules.length) }}</span>
          of
          <span class="font-semibold text-slate-900 dark:text-white">{{ schedules.length }}</span>
          schedules
        </div>

        <div class="flex items-center gap-1.5">
          <button
            type="button"
            @click="schedulesPage--"
            :disabled="schedulesPage === 1"
            class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
          >
            Previous
          </button>
          <div class="flex items-center gap-1">
            <button
              v-for="p in getVisiblePages(schedulesPage, totalSchedulesPages)"
              :key="p"
              type="button"
              @click="typeof p === 'number' && (schedulesPage = p)"
              :disabled="typeof p !== 'number'"
              :class="[
                p === schedulesPage
                  ? 'bg-blue-600 text-white font-bold'
                  : 'bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-800',
                'w-7 h-7 rounded-lg text-xs flex items-center justify-center transition',
                typeof p !== 'number' ? 'cursor-default border-none !bg-transparent text-slate-400' : 'cursor-pointer'
              ]"
            >
              {{ p }}
            </button>
          </div>
          <button
            type="button"
            @click="schedulesPage++"
            :disabled="schedulesPage >= totalSchedulesPages"
            class="px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#1c202b] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
          >
            Next
          </button>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 4: EXECUTION HISTORY -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'history'" class="space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1.5">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Execution History</h2>
          <button
            type="button"
            @click="openInfoPopup('Execution History', 'Comprehensive audit logs of all past backup and restore executions, including status, file size, duration, error logs, and downloadable archives.')"
            class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
            title="Execution History Info"
          >
            <Info class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div class="bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-800 rounded-xl overflow-x-auto shadow-sm">
        <table class="w-full text-left text-xs font-mono">
          <thead class="bg-slate-50 dark:bg-[#1c202b] text-slate-700 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
            <tr>
            <th class="p-3">File Archive</th>
            <th class="p-3">Database</th>
            <th class="p-3">Destination</th>
            <th class="p-3">Size</th>
            <th class="p-3">Status</th>
            <th class="p-3">Executed At</th>
            <th class="p-3 w-28 text-right">Action</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-slate-700 dark:text-slate-300 text-[11px]">
          <tr v-for="h in paginatedHistory" :key="h.id" class="hover:bg-slate-50/80 dark:hover:bg-slate-800/40 transition">
            <td class="p-3">
              <div class="font-semibold text-slate-900 dark:text-white">{{ h.filename }}</div>
              <div
                v-if="h.status === 'failed' && h.errorMessage"
                @click="openLogModal(h)"
                class="text-[10px] text-rose-600 dark:text-rose-400 truncate max-w-xs font-sans mt-0.5 cursor-pointer hover:underline flex items-center gap-1"
                :title="h.errorMessage"
              >
                <AlertCircle class="w-3 h-3 shrink-0" />
                <span class="truncate">{{ h.errorMessage }}</span>
              </div>
            </td>
            <td class="p-3">
              <span v-if="h.destType && h.destType.includes('RESTORE')" class="inline-flex items-center gap-1 font-semibold text-purple-600 dark:text-purple-400">
                <RotateCcw class="w-3 h-3 text-slate-400" />
                <span>{{ h.dbName }} (RESTORE)</span>
              </span>
              <span v-else-if="h.dbType === 'docker'" class="inline-flex items-center gap-1 font-semibold text-blue-600 dark:text-blue-400">
                <Box class="w-3 h-3 text-slate-400" />
                <span>{{ h.dbName }} (SNAPSHOT)</span>
              </span>
              <span v-else>{{ h.dbName }} ({{ h.dbType }})</span>
            </td>
            <td class="p-3 uppercase text-slate-700 dark:text-slate-300 font-semibold">{{ h.destType }}</td>
            <td class="p-3 text-slate-600 dark:text-slate-400">{{ (h.fileSize / 1024 / 1024).toFixed(2) }} MB</td>
            <td class="p-3">
              <span
                v-if="h.status === 'failed'"
                @click="openLogModal(h)"
                role="button"
                class="status-badge cursor-pointer flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold uppercase border bg-rose-50 hover:bg-rose-100 dark:bg-rose-950/40 dark:hover:bg-rose-900/60 text-rose-700 dark:text-rose-400 border-rose-300 dark:border-rose-800/50 transition"
                title="Click to view failure log"
              >
                <AlertCircle class="w-3 h-3 text-rose-600 dark:text-rose-400" />
                <span>FAILED</span>
              </span>
              <span
                v-else
                @click="openLogModal(h)"
                :class="[
                  h.status === 'success' ? 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-400 border-emerald-300 dark:border-emerald-800/50 cursor-pointer' :
                  'bg-amber-50 dark:bg-amber-950/40 text-amber-700 dark:text-amber-400 border-amber-300 dark:border-amber-800/50 cursor-pointer',
                  'px-2 py-0.5 rounded text-[10px] font-bold uppercase border inline-flex items-center gap-1 transition'
                ]"
                :title="h.status === 'running' ? 'Click to view live execution progress' : 'Click to view execution log'"
              >
                <CheckCircle2 v-if="h.status === 'success'" class="w-3 h-3 text-emerald-600 dark:text-emerald-400" />
                <RotateCw v-else class="w-3 h-3 text-amber-600 dark:text-amber-400 animate-spin" />
                <span>{{ h.status }}</span>
              </span>
            </td>
            <td class="p-3 text-slate-600 dark:text-slate-400">{{ new Date(h.startedAt).toLocaleString() }}</td>
            <td class="p-3 text-right">
              <div class="flex items-center justify-end gap-1.5">
                <button
                  v-if="h.status === 'success' && !h.destType?.includes('RESTORE')"
                  @click="downloadBackupFile(h)"
                  class="p-1 rounded text-slate-500 hover:text-emerald-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:text-emerald-400 dark:hover:bg-slate-800 transition cursor-pointer"
                  title="Download Backup Archive"
                >
                  <Download class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="openLogModal(h)"
                  class="p-1 rounded text-slate-500 hover:text-slate-900 hover:bg-slate-100 dark:text-slate-400 dark:hover:text-white dark:hover:bg-slate-800 transition cursor-pointer"
                  title="View Execution Log & Progress"
                >
                  <Eye class="w-3.5 h-3.5" />
                </button>
                <button
                  v-if="canManage"
                  @click="deleteHistoryItem(h)"
                  class="p-1 rounded text-slate-400 hover:text-rose-600 hover:bg-slate-100 dark:text-slate-500 dark:hover:text-rose-400 dark:hover:bg-slate-800 transition cursor-pointer"
                  title="Delete Record"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="history.length === 0">
            <td colspan="7" class="py-12 text-center text-slate-500 text-xs font-sans">
              No backup executions recorded yet.
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Pagination Bar (Active if records > historyPageSize (30 rows)) -->
      <div
        v-if="history.length > historyPageSize"
        class="px-4 py-3 border-t border-slate-200 dark:border-slate-800 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-slate-600 dark:text-slate-400 bg-slate-50/60 dark:bg-[#1c202b]/60 font-sans"
      >
        <div>
          Showing
          <span class="font-semibold text-slate-900 dark:text-white">{{ (historyPage - 1) * historyPageSize + 1 }}</span>
          to
          <span class="font-semibold text-slate-900 dark:text-white">{{ Math.min(historyPage * historyPageSize, history.length) }}</span>
          of
          <span class="font-semibold text-slate-900 dark:text-white">{{ history.length }}</span>
          records
        </div>

        <div class="flex items-center gap-1.5">
          <button
            type="button"
            @click="historyPage--"
            :disabled="historyPage <= 1"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#171a23] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer flex items-center gap-1 text-xs"
          >
            <ChevronLeft class="w-3.5 h-3.5 text-slate-500" />
            <span>Previous</span>
          </button>

          <div class="flex items-center gap-1">
            <template v-for="(p, idx) in visibleHistoryPages" :key="idx">
              <span v-if="p < 0" class="px-1 text-slate-400 select-none">...</span>
              <button
                v-else
                type="button"
                @click="historyPage = p"
                :class="[
                  historyPage === p
                    ? 'bg-blue-600 text-white font-bold border-blue-600'
                    : 'bg-white dark:bg-[#171a23] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-800 hover:bg-slate-100 dark:hover:bg-slate-800',
                  'w-7 h-7 rounded-lg text-xs flex items-center justify-center transition cursor-pointer'
                ]"
              >
                {{ p }}
              </button>
            </template>
          </div>

          <button
            type="button"
            @click="historyPage++"
            :disabled="historyPage >= totalHistoryPages"
            class="px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#171a23] text-slate-700 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer flex items-center gap-1 text-xs"
          >
            <span>Next</span>
            <ChevronRight class="w-3.5 h-3.5 text-slate-500" />
          </button>
        </div>
      </div>
    </div>
  </div>

    <!-- ============================================================= -->
    <!-- MODAL 1: RUN ON-DEMAND BACKUP -->
    <!-- ============================================================= -->
    <div v-if="isRunBackupModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 dark:bg-black/75 backdrop-blur-sm p-4 animate-in fade-in">
      <div class="w-full max-w-2xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans max-h-[92vh] flex flex-col">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3 shrink-0">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
            <Play class="w-4 h-4 text-emerald-600 dark:text-emerald-400 fill-current" />
            <span>Run On-Demand Backup</span>
          </h3>
          <button @click="isRunBackupModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Target Type Switcher -->
        <div class="flex items-center gap-1.5 p-1 bg-slate-100 dark:bg-[#0f1219] rounded-xl border border-slate-200 dark:border-slate-800 shrink-0">
          <button
            type="button"
            @click="runBackupTargetType = 'database'"
            :class="[
              runBackupTargetType === 'database'
                ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white font-bold shadow-sm'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
              'flex-1 py-1.5 text-xs rounded-lg transition cursor-pointer flex items-center justify-center gap-1.5'
            ]"
          >
            <Database class="w-3.5 h-3.5" />
            <span>Database</span>
          </button>
          <button
            type="button"
            @click="runBackupTargetType = 'container'; if (availableContainers.length === 0) fetchAvailableContainers(backupContainerForm.connectionId);"
            :class="[
              runBackupTargetType === 'container'
                ? 'bg-white dark:bg-[#1f283d] text-slate-900 dark:text-white font-bold shadow-sm'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
              'flex-1 py-1.5 text-xs rounded-lg transition cursor-pointer flex items-center justify-center gap-1.5'
            ]"
          >
            <Box class="w-3.5 h-3.5" />
            <span>Docker Container</span>
          </button>
        </div>

        <!-- Form for Database Backup -->
        <div v-if="runBackupTargetType === 'database'" class="space-y-3 text-xs overflow-y-auto pr-1 flex-1">
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-slate-700 dark:text-slate-400 font-bold">
                Select Databases ({{ runForm.dbConfigIds.length }} of {{ databases.length }} selected)
              </label>
              <button
                type="button"
                @click="toggleSelectAllRunDatabases"
                class="text-[11px] font-semibold text-blue-600 hover:text-blue-500 dark:text-blue-400 cursor-pointer"
              >
                {{ runForm.dbConfigIds.length === databases.length ? 'Deselect All' : 'Select All' }}
              </button>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-60 overflow-y-auto pr-1">
              <div
                v-for="db in databases"
                :key="db.id"
                @click="toggleRunDatabaseSelection(db.id)"
                :class="[
                  'p-2.5 rounded-xl border flex items-center justify-between gap-2 transition cursor-pointer',
                  isRunDatabaseSelected(db.id)
                    ? 'bg-blue-50/80 dark:bg-blue-950/40 border-blue-400 dark:border-blue-800 text-blue-900 dark:text-blue-200 shadow-xs'
                    : 'bg-slate-50 dark:bg-[#0f1219] border-slate-200 dark:border-slate-800 text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/50'
                ]"
              >
                <div class="flex items-center gap-2.5 min-w-0">
                  <input
                    type="checkbox"
                    :checked="isRunDatabaseSelected(db.id)"
                    @click.stop="toggleRunDatabaseSelection(db.id)"
                    class="rounded bg-white dark:bg-slate-900 border-slate-300 dark:border-slate-700 text-blue-600 shrink-0 cursor-pointer"
                  />
                  <div class="min-w-0">
                    <p class="font-bold text-xs leading-tight text-slate-900 dark:text-white truncate" :title="db.name">{{ db.name }}</p>
                    <p class="text-[10px] text-slate-500 font-mono truncate" :title="`${db.username}@${db.host}:${db.port}/${db.databaseName}`">
                      {{ db.username }}@{{ db.host }}:{{ db.port }}/{{ db.databaseName }}
                    </p>
                  </div>
                </div>
                <span class="text-[9px] uppercase font-mono px-1.5 py-0.5 rounded bg-white/70 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700/60 font-semibold shrink-0">
                  {{ db.dbType }}
                </span>
              </div>
            </div>
            <p v-if="runForm.dbConfigIds.length === 0" class="text-[11px] text-rose-500 mt-1 font-medium">
              * Please select at least one database.
            </p>
          </div>

          <div>
            <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Select Storage Destination</label>
            <select v-model="runForm.destinationId" class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-brand-500">
              <option v-for="d in destinations" :key="d.id" :value="d.id">
                {{ d.name }} ({{ d.destType }})
              </option>
            </select>
          </div>
        </div>

        <!-- Form for Docker Container Backup -->
        <div v-else class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-bold mb-1">Docker Connection</label>
            <select
              v-model="backupContainerForm.connectionId"
              @change="onBackupConnectionChange"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
            >
              <option value="">Local Docker Engine</option>
              <option v-for="conn in dockerConnections" :key="conn.id" :value="conn.id">
                {{ conn.name }} ({{ conn.host || 'local' }})
              </option>
            </select>
          </div>

          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-bold mb-1">Target Container</label>
            <select
              v-model="backupContainerForm.containerId"
              :disabled="loadingDockerContainers"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500 disabled:opacity-50"
            >
              <option v-if="loadingDockerContainers" value="">Loading containers...</option>
              <option v-else-if="availableContainers.length === 0" value="">No containers found on this host</option>
              <option v-for="c in availableContainers" :key="c.id" :value="c.id">
                {{ getContainerDisplayName(c) }}
              </option>
            </select>
          </div>

          <div>
            <div class="flex items-center gap-1.5 mb-1">
              <label class="block text-slate-700 dark:text-slate-300 font-bold">Custom Snapshot Name (Optional)</label>
              <button
                type="button"
                @click="openInfoPopup('Custom Snapshot Name', 'Optional custom name for this snapshot. If left blank, it defaults to the container name with a timestamp suffix (e.g. container_20261026_143000.tar.gz).\n\nArchive is saved locally to backup/container-docker.')"
                class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                title="Naming Details"
              >
                <Info class="w-3.5 h-3.5" />
              </button>
            </div>
            <input
              v-model="backupContainerForm.backupName"
              type="text"
              placeholder="e.g. production-snapshot-v1"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <div class="flex justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
          <button @click="isRunBackupModalOpen = false" class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer">
            Cancel
          </button>
          <button
            v-if="runBackupTargetType === 'database'"
            @click="triggerBackup"
            class="px-4 py-2 text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-lg shadow cursor-pointer"
          >
            Start Backup
          </button>
          <button
            v-else
            @click="submitBackupContainer"
            :disabled="backupContainerRunning || !backupContainerForm.containerId"
            class="flex items-center gap-1.5 px-4 py-2 text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-lg shadow disabled:opacity-50 transition cursor-pointer"
          >
            <RotateCw v-if="backupContainerRunning" class="w-3.5 h-3.5 animate-spin" />
            <Box v-else class="w-3.5 h-3.5" />
            <span>{{ backupContainerRunning ? 'Creating Snapshot...' : 'Start Backup' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 2: ADD DATABASE CONFIG -->
    <!-- ============================================================= -->
    <div v-if="isDbModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 dark:bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-lg bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
            <Database class="w-4 h-4 text-blue-600 dark:text-brand-400" />
            <span>{{ editingDbId ? 'Edit Database Configuration' : 'Add Database Configuration' }}</span>
          </h3>
          <button @click="isDbModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveDBConfig" class="space-y-3 text-xs">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Config Label</label>
              <input v-model="dbForm.name" required placeholder="e.g. Primary Postgres" class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white" />
            </div>
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Database Type</label>
              <select v-model="dbForm.dbType" class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white">
                <option value="postgresql">PostgreSQL</option>
                <option value="mysql">MySQL</option>
                <option value="mariadb">MariaDB</option>
              </select>
            </div>
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div class="col-span-2">
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Host / IP</label>
              <input v-model="dbForm.host" required class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono" />
            </div>
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Port</label>
              <input v-model.number="dbForm.port" type="number" required class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Username</label>
              <input v-model="dbForm.username" required class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono" />
            </div>
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Password</label>
              <input v-model="dbForm.password" type="password" :placeholder="editingDbId ? '•••••••• (leave blank to keep current)' : '••••••'" class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono" />
            </div>
          </div>

          <div>
            <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Database Name</label>
            <input v-model="dbForm.databaseName" required placeholder="e.g. hephaestus_db" class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono" />
          </div>

          <!-- SSH Tunnel Option -->
          <div class="pt-2 border-t border-slate-200 dark:border-slate-800">
            <label class="flex items-center gap-2 cursor-pointer text-slate-700 dark:text-slate-300">
              <input type="checkbox" v-model="dbForm.useSsh" class="rounded bg-slate-100 dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-blue-600" />
              <span>Connect via Remote SSH Host</span>
            </label>

            <div v-if="dbForm.useSsh" class="mt-2 space-y-2 p-3 bg-slate-50 dark:bg-[#0f1219] rounded-lg border border-slate-200 dark:border-slate-800">
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block text-slate-500 text-[10px]">SSH Host</label>
                  <input v-model="dbForm.sshHost" placeholder="10.20.3.1" class="w-full bg-white dark:bg-[#171a23] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white text-xs font-mono" />
                </div>
                <div>
                  <label class="block text-slate-500 text-[10px]">SSH Port</label>
                  <input v-model.number="dbForm.sshPort" type="number" class="w-full bg-white dark:bg-[#171a23] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white text-xs font-mono" />
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-slate-500 text-[10px]">SSH User</label>
                  <input v-model="dbForm.sshUser" placeholder="root" class="w-full bg-white dark:bg-[#171a23] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white text-xs font-mono" />
                </div>
                <div>
                  <label class="block text-slate-500 text-[10px]">SSH Password</label>
                  <input v-model="dbForm.sshPassword" type="password" :placeholder="editingDbId ? '•••••••• (leave blank to keep current)' : '••••••'" class="w-full bg-white dark:bg-[#171a23] border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-slate-900 dark:text-white text-xs font-mono" />
                </div>
              </div>
            </div>
          </div>

          <!-- Connection Test Feedback Alert -->
          <div
            v-if="testDBResult"
            class="p-2.5 rounded-lg border text-xs flex items-center gap-2"
            :class="testDBResult.success
              ? 'bg-emerald-50 dark:bg-emerald-950/40 border-emerald-300 dark:border-emerald-800 text-emerald-800 dark:text-emerald-300'
              : 'bg-rose-50 dark:bg-rose-950/40 border-rose-300 dark:border-rose-800 text-rose-800 dark:text-rose-300'"
          >
            <CheckCircle2 v-if="testDBResult.success" class="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
            <AlertCircle v-else class="w-4 h-4 text-rose-600 dark:text-rose-400 shrink-0" />
            <span class="break-all">{{ testDBResult.text }}</span>
          </div>

          <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
            <button
              type="button"
              @click="isDbModalOpen = false; testDBResult = null"
              class="px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-transparent text-xs font-semibold transition cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="button"
              @click="testDBConfig"
              :disabled="testingDB"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 border border-slate-300 dark:border-slate-700 text-xs font-semibold shadow-sm transition cursor-pointer disabled:opacity-50"
            >
              <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': testingDB }" />
              <span>{{ testingDB ? 'Testing...' : 'Test Connection' }}</span>
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow transition cursor-pointer text-xs"
            >
              {{ editingDbId ? 'Update Database' : 'Save Database' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 3: ADD STORAGE DESTINATION -->
    <!-- ============================================================= -->
    <div v-if="isDestModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 dark:bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-lg bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
            <Cloud class="w-4 h-4 text-slate-500 dark:text-slate-400" />
            <span>{{ editingDestId ? 'Edit Storage Destination' : 'Add Storage Destination' }}</span>
          </h3>
          <button @click="isDestModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveDestination" class="space-y-3 text-xs">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Destination Name</label>
              <input v-model="destForm.name" required placeholder="e.g. Local Storage, Cloudflare R2" class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white" />
            </div>
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Storage Type</label>
              <select v-model="destForm.destType" class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white">
                <option value="local">Local Storage (Host ./backups)</option>
                <option value="nas">NAS (SMB/SSH)</option>
                <option value="r2">Cloudflare R2 Object Storage</option>
                <option value="s3">AWS S3 / MinIO Storage</option>
              </select>
            </div>
          </div>

          <!-- ================= NAS (SMB/SSH) ================= -->
          <template v-if="destForm.destType === 'nas' || destForm.destType === 'nfs'">
            <div class="grid grid-cols-3 gap-3">
              <div class="col-span-2">
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">NAS HOST</label>
                <input
                  v-model="destForm.host"
                  required
                  placeholder="10.3.16.184"
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
                />
              </div>
              <div>
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">SSH PORT</label>
                <input
                  v-model.number="destForm.port"
                  type="number"
                  required
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
                />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">USERNAME</label>
                <input
                  v-model="destForm.username"
                  required
                  placeholder="administrator"
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
                />
              </div>
              <div>
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">AUTH</label>
                <select
                  v-model="destForm.authType"
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white"
                >
                  <option value="password">Password</option>
                  <option value="key">SSH Key</option>
                </select>
              </div>
            </div>

            <div v-if="destForm.authType === 'password'">
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">PASSWORD</label>
              <input
                v-model="destForm.password"
                type="password"
                :placeholder="editingDestId ? '•••••••• (leave blank to keep current)' : '••••••••'"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
              />
            </div>
            <div v-else>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">SSH PRIVATE KEY</label>
              <textarea
                v-model="destForm.sshKey"
                rows="3"
                :placeholder="editingDestId ? '(leave blank to keep current private key)' : '-----BEGIN OPENSSH PRIVATE KEY-----'"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono text-[11px]"
              ></textarea>
            </div>

            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">BACKUP PATH</label>
              <input
                v-model="destForm.path"
                required
                placeholder="/opt/backups-wordpress"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
              />
            </div>
          </template>

          <!-- ================= LOCAL FILESYSTEM ================= -->
          <template v-else-if="destForm.destType === 'local'">
            <div class="space-y-1.5">
              <label class="block text-slate-700 dark:text-slate-400 font-bold">Local Directory Path</label>
              <input
                v-model="destForm.path"
                required
                placeholder="/app/backups or database"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
              />
              <div class="p-2.5 rounded-lg bg-blue-50/70 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-900/50 text-[11px] text-blue-900 dark:text-blue-300 space-y-1">
                <p class="font-semibold flex items-center gap-1">
                  <span>Persistent Host Mount:</span>
                  <code class="px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/60 rounded font-mono font-bold">./backups</code>
                </p>
                <p class="text-[10px] leading-relaxed text-blue-700 dark:text-blue-400">
                  Files are saved directly to the <code class="font-bold">./backups</code> directory on your host server (container path: <code class="font-bold">/app/backups</code>). Any subfolder such as <code class="font-bold">database</code> or custom name is safely stored inside this persistent volume.
                </p>
              </div>
            </div>
          </template>

          <template v-else>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">
                  {{ destForm.destType === 'r2' ? 'R2 Bucket Name' : 'S3 Bucket Name' }}
                </label>
                <input
                  v-model="destForm.bucket"
                  required
                  :placeholder="destForm.destType === 'r2' ? 'e.g. hcp-storage' : 'hephaestus-backups'"
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
                />
              </div>
              <div>
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">
                  {{ destForm.destType === 'r2' ? 'Cloudflare Account ID' : 'Region' }}
                </label>
                <input
                  v-model="destForm.accountId"
                  :placeholder="destForm.destType === 'r2' ? 'e.g. 1f133699545b4bb29b45928a5ed86724' : 'auto'"
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
                />
              </div>
            </div>

            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">
                {{ destForm.destType === 'r2' ? 'S3 API Endpoint URL' : 'S3 Endpoint URL' }}
                <span v-if="destForm.destType === 'r2'" class="text-[10px] font-normal text-slate-500">
                  (Omit bucket name from URL)
                </span>
              </label>
              <input
                v-model="destForm.endpoint"
                :placeholder="destForm.destType === 'r2' ? 'https://<account-id>.r2.cloudflarestorage.com' : 'https://s3.amazonaws.com'"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
              />
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">
                  {{ destForm.destType === 'r2' ? 'R2 Access Key ID' : 'Access Key ID' }}
                </label>
                <input
                  v-model="destForm.accessKeyId"
                  required
                  placeholder="Access Key ID from R2 Token"
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
                />
              </div>
              <div>
                <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">
                  {{ destForm.destType === 'r2' ? 'R2 Secret Access Key' : 'Secret Access Key' }}
                </label>
                <input
                  v-model="destForm.secretAccessKey"
                  type="password"
                  :required="!editingDestId"
                  :placeholder="editingDestId ? '•••••••• (leave blank to keep current)' : 'Secret Access Key from R2 Token'"
                  class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono"
                />
              </div>
            </div>
          </template>

          <!-- Connection Test Feedback Banner -->
          <div
            v-if="testDestResult"
            :class="[
              'p-2.5 rounded-lg text-xs font-mono flex items-center gap-2 border transition',
              testDestResult.success
                ? 'bg-emerald-50 text-emerald-800 border-emerald-300 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800'
                : 'bg-rose-50 text-rose-800 border-rose-300 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-800'
            ]"
          >
            <CheckCircle2 v-if="testDestResult.success" class="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
            <AlertCircle v-else class="w-4 h-4 text-rose-600 dark:text-rose-400 shrink-0" />
            <span>{{ testDestResult.text }}</span>
          </div>

          <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
            <button
              type="button"
              @click="isDestModalOpen = false; testDestResult = null"
              class="px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-transparent text-xs font-semibold transition cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="button"
              @click="testDestination"
              :disabled="testingDest"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 border border-slate-300 dark:border-slate-700 text-xs font-semibold shadow-sm transition cursor-pointer disabled:opacity-50"
            >
              <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': testingDest }" />
              <span>{{ testingDest ? 'Testing...' : 'Test Connection' }}</span>
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-bold rounded-lg shadow transition cursor-pointer text-xs"
            >
              {{ editingDestId ? 'Update Destination' : 'Save Destination' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 4: ADD CRON SCHEDULE -->
    <!-- ============================================================= -->
    <div v-if="isScheduleModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 dark:bg-black/75 backdrop-blur-sm p-4">
      <div class="w-full max-w-3xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans max-h-[92vh] flex flex-col">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3 shrink-0">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
            <Clock class="w-4 h-4 text-amber-600 dark:text-amber-400" />
            <span>{{ editingScheduleId ? 'Edit Backup Schedule' : 'Add Backup Schedule' }}</span>
          </h3>
          <button @click="isScheduleModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="saveSchedule" class="space-y-4 text-xs overflow-y-auto pr-1 flex-1">
          <!-- Top Row: Schedule Name and Target Type side-by-side -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Schedule Name</label>
              <input
                v-model="scheduleForm.name"
                required
                placeholder="Daily Midnight Backup"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              />
            </div>

            <!-- Backup Target Type Toggle -->
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Target Type</label>
              <div class="grid grid-cols-2 gap-2 p-1 bg-slate-100 dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-800">
                <button
                  type="button"
                  @click="scheduleForm.scheduleType = 'database'"
                  :class="[
                    scheduleForm.scheduleType === 'database'
                      ? 'bg-white dark:bg-[#1f2430] text-blue-600 dark:text-white font-bold shadow-xs'
                      : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200',
                    'py-1.5 px-3 rounded-md text-xs transition cursor-pointer flex items-center justify-center gap-1.5'
                  ]"
                >
                  <Database class="w-3.5 h-3.5" />
                  <span>Database Backup</span>
                </button>
                <button
                  type="button"
                  @click="scheduleForm.scheduleType = 'container'"
                  :class="[
                    scheduleForm.scheduleType === 'container'
                      ? 'bg-white dark:bg-[#1f2430] text-blue-600 dark:text-white font-bold shadow-xs'
                      : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200',
                    'py-1.5 px-3 rounded-md text-xs transition cursor-pointer flex items-center justify-center gap-1.5'
                  ]"
                >
                  <Box class="w-3.5 h-3.5" />
                  <span>Docker Container</span>
                </button>
              </div>
            </div>
          </div>

          <!-- ================= DATABASE TARGET SELECTOR ================= -->
          <div v-if="scheduleForm.scheduleType === 'database'" class="space-y-2">
            <div class="flex items-center justify-between gap-3 flex-wrap">
              <label class="text-slate-700 dark:text-slate-400 font-bold">
                Select Databases ({{ scheduleForm.dbConfigIds.length }} of {{ databases.length }} selected)
              </label>
              <div class="flex items-center gap-2">
                <div class="relative">
                  <Search class="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                  <input
                    v-model="scheduleDatabaseSearch"
                    type="text"
                    placeholder="Search databases..."
                    class="pl-8 pr-2.5 py-1 text-[11px] bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-md text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 w-36 sm:w-48"
                  />
                </div>
                <button
                  type="button"
                  @click="toggleSelectAllDatabases"
                  class="text-[11px] font-semibold text-blue-600 hover:text-blue-500 dark:text-blue-400 cursor-pointer whitespace-nowrap"
                >
                  {{ scheduleForm.dbConfigIds.length === filteredScheduleDatabases.length && filteredScheduleDatabases.length > 0 ? 'Deselect All' : 'Select All' }}
                </button>
              </div>
            </div>

            <div v-if="filteredScheduleDatabases.length === 0" class="p-6 text-center text-slate-400 bg-slate-50 dark:bg-[#0f1219] rounded-xl border border-slate-200 dark:border-slate-800 text-xs">
              No databases match your search.
            </div>
            <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-64 sm:max-h-72 overflow-y-auto pr-1">
              <div
                v-for="db in filteredScheduleDatabases"
                :key="db.id"
                @click="toggleDatabaseSelection(db.id)"
                :class="[
                  'p-2.5 rounded-xl border flex items-center justify-between gap-2 transition cursor-pointer',
                  isDatabaseSelected(db.id)
                    ? 'bg-blue-50/80 dark:bg-blue-950/40 border-blue-400 dark:border-blue-800 text-blue-900 dark:text-blue-200 shadow-xs'
                    : 'bg-slate-50 dark:bg-[#0f1219] border-slate-200 dark:border-slate-800 text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/50'
                ]"
              >
                <div class="flex items-center gap-2.5 min-w-0">
                  <input
                    type="checkbox"
                    :checked="isDatabaseSelected(db.id)"
                    @click.stop="toggleDatabaseSelection(db.id)"
                    class="rounded bg-white dark:bg-slate-900 border-slate-300 dark:border-slate-700 text-blue-600 shrink-0 cursor-pointer"
                  />
                  <div class="min-w-0">
                    <p class="font-bold text-xs leading-tight text-slate-900 dark:text-white truncate" :title="db.name">{{ db.name }}</p>
                    <p class="text-[10px] text-slate-500 font-mono truncate" :title="`${db.username}@${db.host}:${db.port}/${db.databaseName}`">
                      {{ db.username }}@{{ db.host }}:{{ db.port }}/{{ db.databaseName }}
                    </p>
                  </div>
                </div>
                <span class="text-[9px] uppercase font-mono px-1.5 py-0.5 rounded bg-white/70 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700/60 font-semibold shrink-0">
                  {{ db.dbType }}
                </span>
              </div>
            </div>
            <p v-if="scheduleForm.dbConfigIds.length === 0" class="text-[11px] text-rose-500 mt-1 font-medium">
              * Please select at least one database.
            </p>
          </div>

          <!-- ================= CONTAINER TARGET SELECTOR ================= -->
          <div v-else-if="scheduleForm.scheduleType === 'container'" class="space-y-3">
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Docker Host Connection</label>
              <select
                v-model="scheduleForm.dockerConnectionId"
                @change="fetchAvailableContainers(scheduleForm.dockerConnectionId)"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              >
                <option value="">Local Docker Engine</option>
                <option v-for="conn in dockerConnections" :key="conn.id" :value="conn.id">
                  {{ conn.name }} ({{ conn.host || 'local' }})
                </option>
              </select>
            </div>

            <div class="space-y-2">
              <div class="flex items-center justify-between gap-3 flex-wrap">
                <label class="text-slate-700 dark:text-slate-400 font-bold">
                  Select Containers ({{ scheduleForm.containerIds.length }} of {{ availableContainers.length }} selected)
                </label>
                <div class="flex items-center gap-2">
                  <div class="relative">
                    <Search class="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                    <input
                      v-model="scheduleContainerSearch"
                      type="text"
                      placeholder="Search container..."
                      class="pl-8 pr-2.5 py-1 text-[11px] bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-md text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 w-36 sm:w-48"
                    />
                  </div>
                  <button
                    type="button"
                    @click="toggleSelectAllContainers"
                    class="text-[11px] font-semibold text-blue-600 hover:text-blue-500 dark:text-blue-400 cursor-pointer whitespace-nowrap"
                  >
                    {{ scheduleForm.containerIds.length === filteredScheduleContainers.length && filteredScheduleContainers.length > 0 ? 'Deselect All' : 'Select All' }}
                  </button>
                </div>
              </div>

              <div v-if="loadingDockerContainers" class="p-6 text-center text-slate-400 bg-slate-50 dark:bg-[#0f1219] rounded-xl border border-slate-200 dark:border-slate-800">
                <RotateCw class="w-4 h-4 animate-spin mx-auto mb-1" />
                <span>Loading containers...</span>
              </div>
              <div v-else-if="availableContainers.length === 0" class="p-6 text-center text-slate-400 bg-slate-50 dark:bg-[#0f1219] rounded-xl border border-slate-200 dark:border-slate-800 text-xs">
                No containers found on selected Docker host.
              </div>
              <div v-else-if="filteredScheduleContainers.length === 0" class="p-6 text-center text-slate-400 bg-slate-50 dark:bg-[#0f1219] rounded-xl border border-slate-200 dark:border-slate-800 text-xs">
                No containers match your search.
              </div>
              <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-64 sm:max-h-72 overflow-y-auto pr-1">
                <div
                  v-for="c in filteredScheduleContainers"
                  :key="c.id"
                  @click="toggleContainerSelection(c.id)"
                  :class="[
                    'p-2.5 rounded-xl border flex items-center justify-between gap-2 transition cursor-pointer',
                    isContainerSelected(c.id)
                      ? 'bg-blue-50/80 dark:bg-blue-950/40 border-blue-400 dark:border-blue-800 text-blue-900 dark:text-blue-200 shadow-xs'
                      : 'bg-slate-50 dark:bg-[#0f1219] border-slate-200 dark:border-slate-800 text-slate-700 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/50'
                  ]"
                >
                  <div class="flex items-center gap-2.5 min-w-0">
                    <input
                      type="checkbox"
                      :checked="isContainerSelected(c.id)"
                      @click.stop="toggleContainerSelection(c.id)"
                      class="rounded bg-white dark:bg-slate-900 border-slate-300 dark:border-slate-700 text-blue-600 shrink-0 cursor-pointer"
                    />
                    <div class="min-w-0">
                      <p class="font-bold text-xs leading-tight text-slate-900 dark:text-white font-mono truncate" :title="getContainerRawName(c)">
                        {{ getContainerRawName(c) }}
                      </p>
                      <p class="text-[10px] text-slate-500 font-mono truncate" :title="c.image || c.sourceImage">
                        {{ c.image || c.sourceImage || '-' }}
                      </p>
                    </div>
                  </div>
                  <span
                    :class="[
                      c.state === 'running' || c.status?.includes('Up')
                        ? 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800'
                        : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 border-slate-200 dark:border-slate-700',
                      'text-[9px] uppercase font-mono px-1.5 py-0.5 rounded border font-semibold shrink-0'
                    ]"
                  >
                    {{ c.state || (c.status?.includes('Up') ? 'running' : 'exited') }}
                  </span>
                </div>
              </div>
              <p v-if="scheduleForm.containerIds.length === 0" class="text-[11px] text-rose-500 mt-1 font-medium">
                * Please select at least one container.
              </p>
            </div>
          </div>

          <!-- Destination & Cron in a 2-column grid -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label class="block text-slate-700 dark:text-slate-400 mb-1 font-bold">Select Destination</label>
              <select
                v-model="scheduleForm.destinationId"
                required
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500"
              >
                <option v-for="d in destinations" :key="d.id" :value="d.id">
                  {{ d.name }} ({{ d.destType }})
                </option>
              </select>
            </div>

            <div>
              <div class="flex items-center gap-1.5 mb-1">
                <label class="block text-slate-700 dark:text-slate-400 font-bold">Cron Expression</label>
                <button
                  type="button"
                  @click="openInfoPopup('Cron Expression Format', 'Standard 5-field cron syntax:\n• min (0-59)\n• hour (0-23)\n• dom (day of month, 1-31)\n• mon (month, 1-12)\n• dow (day of week, 0-6, 0=Sun)\n\nPresets:\n• 0 2 * * * : Every day at 02:00 AM\n• 0 */6 * * * : Every 6 hours\n• 0 0 * * 0 : Weekly on Sunday midnight')"
                  class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                  title="Cron Format Info"
                >
                  <Info class="w-3.5 h-3.5" />
                </button>
              </div>
              <input
                v-model="scheduleForm.cronExpression"
                required
                placeholder="0 2 * * *"
                class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <!-- Bottom Footer Bar: Options & Actions -->
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-3 border-t border-slate-100 dark:border-slate-800">
            <div class="flex flex-col sm:flex-row items-start sm:items-center gap-3 text-xs">
              <label class="flex items-center gap-2 cursor-pointer text-slate-700 dark:text-slate-300">
                <input
                  type="checkbox"
                  v-model="scheduleForm.isActive"
                  class="rounded bg-slate-100 dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-blue-600"
                />
                <span class="font-semibold">Schedule is Active</span>
              </label>

              <label
                v-if="scheduleForm.scheduleType === 'container'"
                class="flex items-center gap-2 cursor-pointer text-slate-700 dark:text-slate-300"
              >
                <input
                  type="checkbox"
                  v-model="scheduleForm.stopDuringBackup"
                  class="rounded bg-slate-100 dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-blue-600"
                />
                <span class="font-semibold">Stop container during snapshot</span>
              </label>
            </div>

            <div class="flex items-center justify-end gap-2 shrink-0">
              <button
                type="button"
                @click="isScheduleModalOpen = false"
                class="px-4 py-2 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white rounded-lg transition cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="submit"
                class="px-5 py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-lg shadow-sm transition cursor-pointer"
              >
                {{ editingScheduleId ? 'Update Schedule' : 'Save Schedule' }}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 5: VIEW EXECUTION LOG -->
    <!-- ============================================================= -->
    <div v-if="isLogModalOpen && selectedHistory" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm p-4">
      <div class="w-full max-w-2xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans max-h-[90vh] flex flex-col">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3 shrink-0">
          <div class="flex items-center gap-2.5">
            <div class="p-2 rounded-lg" :class="selectedHistory.status === 'failed' ? 'bg-rose-50 dark:bg-rose-500/10 text-rose-600 dark:text-rose-400' : 'bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-brand-400'">
              <Terminal class="w-4 h-4" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <span>Backup Execution Log</span>
                <span
                  :class="[
                    selectedHistory.status === 'success' ? 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-300 dark:border-emerald-500/30' :
                    selectedHistory.status === 'running' ? 'bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-300 dark:border-amber-500/30' :
                    'bg-rose-50 dark:bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-300 dark:border-rose-500/30',
                    'px-2 py-0.5 rounded text-[10px] font-bold uppercase border'
                  ]"
                >
                  {{ selectedHistory.status }}
                </span>
              </h3>
              <p class="text-[11px] font-mono text-slate-500 truncate max-w-md">
                {{ selectedHistory.filename }}
              </p>
            </div>
          </div>
          <button @click="closeLogModal" class="text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Meta info pills -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs shrink-0">
          <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">Target</span>
            <span class="font-semibold text-slate-900 dark:text-white truncate block">{{ selectedHistory.dbName }} ({{ selectedHistory.dbType }})</span>
          </div>
          <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">Destination</span>
            <span class="font-semibold text-slate-900 dark:text-white uppercase truncate block">{{ selectedHistory.destType }}</span>
          </div>
          <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">File Size</span>
            <span class="font-semibold text-slate-900 dark:text-white block">{{ (selectedHistory.fileSize / 1024 / 1024).toFixed(2) }} MB</span>
          </div>
          <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">Executed At</span>
            <span class="font-semibold text-slate-900 dark:text-white truncate block">{{ new Date(selectedHistory.startedAt).toLocaleTimeString() }}</span>
          </div>
        </div>

        <!-- Error Summary Box (If failed) -->
        <div
          v-if="selectedHistory.status === 'failed' && selectedHistory.errorMessage"
          class="p-3 rounded-xl bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900/60 text-rose-800 dark:text-rose-300 text-xs shrink-0 flex items-start gap-2.5"
        >
          <AlertCircle class="w-4 h-4 text-rose-600 dark:text-rose-400 shrink-0 mt-0.5" />
          <div class="space-y-1 overflow-hidden">
            <p class="font-bold text-rose-900 dark:text-rose-200">Failure Reason:</p>
            <p class="font-mono text-[11px] break-all leading-relaxed whitespace-pre-wrap">{{ selectedHistory.errorMessage }}</p>
          </div>
        </div>

        <!-- Log Terminal Output -->
        <div class="flex-1 min-h-[220px] max-h-[460px] flex flex-col overflow-hidden">
          <div class="flex items-center justify-between pb-1.5 text-[10px] font-mono text-slate-500">
            <span class="flex items-center gap-2">
              <span>EXECUTION LOG</span>
              <span v-if="selectedHistory.status === 'running'" class="inline-flex items-center gap-1 text-amber-500 animate-pulse font-sans">
                <span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
                <span>Live streaming...</span>
              </span>
            </span>
            <span>UTF-8</span>
          </div>

          <!-- Structured Step Viewer if available -->
          <div
            v-if="selectedHistoryLogs.length > 0"
            data-terminal="true"
            class="cli-terminal-box terminal-log-viewer flex-1 overflow-y-auto p-4 rounded-xl border border-slate-800 shadow-inner font-mono text-[11px] leading-relaxed select-text space-y-1.5"
            style="background-color: #080b14 !important; color: #f8fafc !important;"
          >
            <div
              v-for="(log, idx) in selectedHistoryLogs"
              :key="idx"
              class="flex items-start gap-2.5 font-mono"
            >
              <span class="shrink-0 select-none text-[10px]" style="color: #94a3b8 !important;">
                [{{ log.timestamp }}]
              </span>
              <span
                class="uppercase text-[9px] px-1.5 py-0.5 rounded shrink-0 font-bold tracking-wider"
                :style="
                  log.status === 'error'
                    ? 'background-color: #4c0519 !important; color: #fb7185 !important; border: 1px solid #e11d48 !important;'
                    : log.status === 'success'
                    ? 'background-color: #064e3b !important; color: #4ade80 !important; border: 1px solid #059669 !important;'
                    : log.status === 'progress'
                    ? 'background-color: #1e1b4b !important; color: #a5b4fc !important; border: 1px solid #4f46e5 !important;'
                    : 'background-color: #1e293b !important; color: #38bdf8 !important; border: 1px solid #0284c7 !important;'
                "
              >
                {{ log.step }}
              </span>
              <span
                class="flex-1 break-all"
                :style="
                  log.status === 'error'
                    ? 'color: #fb7185 !important; font-weight: 600;'
                    : log.status === 'success'
                    ? 'color: #4ade80 !important; font-weight: 600;'
                    : log.status === 'progress'
                    ? 'color: #93c5fd !important;'
                    : 'color: #f1f5f9 !important;'
                "
              >
                {{ log.message }}
              </span>
            </div>

            <div
              v-if="selectedHistory.status === 'running'"
              class="flex items-center gap-2 pt-2 animate-pulse font-mono text-[10px]"
              style="color: #94a3b8 !important;"
            >
              <span class="inline-block w-2 h-3.5" style="background-color: #38bdf8 !important;"></span>
              <span>Process is actively executing...</span>
            </div>
          </div>

          <!-- Raw Log text fallback -->
          <div
            v-else
            data-terminal="true"
            class="terminal-log-viewer flex-1 overflow-y-auto font-mono text-[11px] p-4 rounded-xl border border-slate-800 shadow-inner whitespace-pre-wrap select-text leading-relaxed"
            style="background-color: #080b14 !important; color: #4ade80 !important;"
          >
{{ generateLogText(selectedHistory) }}
          </div>
        </div>

        <!-- Footer Actions -->
        <div class="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-slate-800 shrink-0">
          <div class="flex items-center gap-2">
            <button
              v-if="selectedHistory.dbConfigId && selectedHistory.destinationId"
              @click="rerunFromHistory(selectedHistory)"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-bold text-xs shadow transition cursor-pointer"
            >
              <RotateCcw class="w-3.5 h-3.5" />
              <span>Retry Backup</span>
            </button>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="copyLogText"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 text-xs font-semibold transition cursor-pointer"
            >
              <Check v-if="logCopied" class="w-3.5 h-3.5 text-emerald-600" />
              <Copy v-else class="w-3.5 h-3.5" />
              <span>{{ logCopied ? 'Copied!' : 'Copy Log' }}</span>
            </button>
            <button
              @click="closeLogModal"
              class="px-4 py-1.5 rounded-lg bg-slate-200 hover:bg-slate-300 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-white font-bold text-xs transition cursor-pointer"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL: BACKUP DOCKER CONTAINER -->
    <!-- ============================================================= -->
    <div v-if="isBackupContainerModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm p-4 animate-in fade-in">
      <div class="w-full max-w-lg bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2.5">
            <div class="p-2 rounded-lg bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400">
              <Box class="w-5 h-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-sm font-bold text-slate-900 dark:text-white">Backup Docker Container</h3>
                <button
                  type="button"
                  @click="openInfoPopup('Backup Docker Container', 'Creates an OCI container snapshot image (.tar.gz) and saves runtime environment variables, port configurations, and labels to backup/container-docker.')"
                  class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                  title="Container Backup Info"
                >
                  <Info class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>
          <button @click="isBackupContainerModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form @submit.prevent="submitBackupContainer" class="space-y-4 text-xs">
          <!-- Connection / Host Selection -->
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <label class="block text-slate-700 dark:text-slate-300 font-bold">Docker Connection</label>
              <router-link
                to="/connections"
                target="_blank"
                class="text-[11px] text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 cursor-pointer"
                title="View or configure Docker connections in Connections menu"
              >
                <span>Manage in Connections</span>
                <ExternalLink class="w-3 h-3" />
              </router-link>
            </div>
            <select
              v-model="backupContainerForm.connectionId"
              @change="onBackupConnectionChange"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-medium"
            >
              <option value="">Local Docker Engine (Local Socket: /var/run/docker.sock)</option>
              <option v-for="conn in dockerConnections" :key="conn.id" :value="conn.id">
                {{ getDockerConnectionLabel(conn) }}
              </option>
            </select>
          </div>

          <!-- Container Selection -->
          <div>
            <div class="flex items-center justify-between mb-1">
              <label class="text-slate-700 dark:text-slate-300 font-bold">Target Container</label>
              <button
                type="button"
                @click="fetchAvailableContainers(backupContainerForm.connectionId)"
                class="text-[11px] text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 cursor-pointer"
              >
                <RotateCw class="w-3 h-3" :class="{ 'animate-spin': loadingDockerContainers }" />
                <span>Refresh</span>
              </button>
            </div>
            <select
              v-model="backupContainerForm.containerId"
              :disabled="loadingDockerContainers"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-mono disabled:opacity-50"
            >
              <option v-if="loadingDockerContainers" value="">Loading containers...</option>
              <option v-else-if="availableContainers.length === 0" value="">No containers found on this host</option>
              <option v-for="c in availableContainers" :key="c.id" :value="c.id">
                {{ getContainerDisplayName(c) }}
              </option>
            </select>
          </div>

          <!-- Custom Snapshot Name -->
          <div>
            <div class="flex items-center gap-1.5 mb-1">
              <label class="block text-slate-700 dark:text-slate-300 font-bold">Custom Snapshot Name (Optional)</label>
              <button
                type="button"
                @click="openInfoPopup('Snapshot Name', 'Custom filename prefix for this backup. If left blank, defaults to container name with the current timestamp (e.g. container_20261026_143000.tar.gz).')"
                class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                title="Naming Guide"
              >
                <Info class="w-3.5 h-3.5" />
              </button>
            </div>
            <input
              v-model="backupContainerForm.backupName"
              type="text"
              placeholder="e.g. production-snapshot-v1"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Quiescence Toggle -->
          <div class="p-3 rounded-xl bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800 flex items-center justify-between">
            <label class="flex items-center gap-2.5 cursor-pointer">
              <input
                v-model="backupContainerForm.stopDuringBackup"
                type="checkbox"
                class="rounded border-slate-300 dark:border-slate-700 text-blue-600"
              />
              <span class="font-bold text-slate-800 dark:text-slate-200 text-xs">Stop container during snapshot</span>
            </label>
            <button
              type="button"
              @click.prevent.stop="openInfoPopup('Container Filesystem Quiescence', 'Temporarily stops the container during image commit to guarantee filesystem quiescence (no active write transactions or corrupted database state), and automatically restarts it once snapshot creation finishes.')"
              class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-1 rounded transition cursor-pointer"
              title="About Quiescence"
            >
              <Info class="w-3.5 h-3.5" />
            </button>
          </div>

          <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
            <button
              type="button"
              @click="isBackupContainerModalOpen = false"
              class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="backupContainerRunning || !backupContainerForm.containerId"
              class="flex items-center gap-1.5 px-4 py-2 text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-lg shadow disabled:opacity-50 transition cursor-pointer"
            >
              <RotateCw v-if="backupContainerRunning" class="w-3.5 h-3.5 animate-spin" />
              <Box v-else class="w-3.5 h-3.5" />
              <span>{{ backupContainerRunning ? 'Creating Snapshot & Streaming...' : 'Start Container Backup' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL: RESTORE DOCKER CONTAINER -->
    <!-- ============================================================= -->
    <div v-if="isRestoreContainerModalOpen && restoreBackupTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm p-4 animate-in fade-in">
      <div class="w-full max-w-2xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans max-h-[92vh] flex flex-col">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3 shrink-0">
          <div class="flex items-center gap-2.5">
            <div class="p-2 rounded-lg bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
              <RotateCcw class="w-5 h-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-sm font-bold text-slate-900 dark:text-white">Restore Docker Container</h3>
                <button
                  type="button"
                  @click="openInfoPopup('Restore Docker Container', 'Deploys an OCI snapshot image (.tar.gz) and its configuration to a local or remote Docker host. Cross-host restore extracts the image directly and launches the container with your configured network ports, environment variables, and restart policy.')"
                  class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                  title="About Container Restore"
                >
                  <Info class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>
          <button @click="isRestoreContainerModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Snapshot Info Banner -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs shrink-0">
          <div class="p-2 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">Source Container</span>
            <span class="font-bold text-slate-900 dark:text-white truncate block font-mono">{{ restoreBackupTarget.containerName }}</span>
          </div>
          <div class="p-2 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">Source Host</span>
            <span class="font-semibold text-slate-900 dark:text-white truncate block">{{ restoreBackupTarget.sourceConnection || 'Local Docker' }}</span>
          </div>
          <div class="p-2 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">Archive Size</span>
            <span class="font-semibold text-slate-900 dark:text-white block">{{ formatFileSize(restoreBackupTarget.fileSize) }}</span>
          </div>
          <div class="p-2 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
            <span class="text-[10px] text-slate-500 block uppercase font-bold">Snapshot Date</span>
            <span class="font-semibold text-slate-900 dark:text-white truncate block">{{ formatDateTime(restoreBackupTarget.createdAt) }}</span>
          </div>
        </div>

        <form @submit.prevent="submitRestoreContainer" class="flex-1 overflow-y-auto space-y-4 pr-1 text-xs">
          <!-- Target Docker Connection -->
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-1.5">
                <label class="block text-slate-700 dark:text-slate-300 font-bold">
                  Target Docker Connection (Restore Destination)
                </label>
                <button
                  type="button"
                  @click="openInfoPopup('Target Docker Connection', 'Select the destination Docker host where this container will be deployed. Cross-host restoration automatically transfers and extracts the OCI snapshot image via Docker API or direct SSH streaming.')"
                  class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                  title="Connection Info"
                >
                  <Info class="w-3.5 h-3.5" />
                </button>
              </div>
              <router-link
                to="/connections"
                target="_blank"
                class="text-[11px] text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 cursor-pointer"
                title="View or add Docker connections in Connections menu"
              >
                <span>Manage in Connections</span>
                <ExternalLink class="w-3 h-3" />
              </router-link>
            </div>
            <select
              v-model="restoreForm.targetConnectionId"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-medium"
            >
              <option value="">Local Docker Engine (Local Socket: /var/run/docker.sock)</option>
              <option v-for="conn in dockerConnections" :key="conn.id" :value="conn.id">
                {{ getDockerConnectionLabel(conn) }}
              </option>
            </select>

            <!-- Target Host Details Badge -->
            <div
              v-if="selectedTargetConnection"
              class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800 flex items-center justify-between text-[11px] mt-2"
            >
              <div class="flex items-center gap-2">
                <Server class="w-4 h-4 text-slate-500 shrink-0" />
                <div>
                  <span class="font-bold text-slate-900 dark:text-white">{{ selectedTargetConnection.name }}</span>
                  <span class="text-slate-500 dark:text-slate-400 block font-mono text-[10px]">
                    {{ selectedTargetConnection.sshHost ? `${selectedTargetConnection.sshUser || 'root'}@${selectedTargetConnection.sshHost}:${selectedTargetConnection.sshPort || 22}` : (selectedTargetConnection.tcpUrl || selectedTargetConnection.socketPath || 'Local Engine') }}
                  </span>
                </div>
              </div>
              <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700">
                {{ selectedTargetConnection.hostType || selectedTargetConnection.driver || 'DOCKER' }}
              </span>
            </div>
          </div>

          <!-- New Container Name -->
          <div>
            <label class="block text-slate-700 dark:text-slate-300 font-bold mb-1">Restored Container Name</label>
            <input
              v-model="restoreForm.newContainerName"
              type="text"
              required
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <!-- Port Bindings Configuration -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <div class="flex items-center gap-1.5">
                <label class="text-slate-700 dark:text-slate-300 font-bold">Port Mappings (Host : Container)</label>
                <button
                  type="button"
                  @click="openInfoPopup('Port Mappings', 'Maps the host network port to the container internal port (HostPort : ContainerPort). Adjust the host port if the original port is already in use on the target machine.')"
                  class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                  title="Port Mappings Info"
                >
                  <Info class="w-3.5 h-3.5" />
                </button>
              </div>
              <button
                type="button"
                @click="addRestorePort"
                class="text-[11px] font-semibold text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 cursor-pointer"
              >
                <Plus class="w-3 h-3" />
                <span>Add Port</span>
              </button>
            </div>
            <div v-if="restoreForm.ports.length === 0" class="text-[11px] text-slate-400 italic p-2 bg-slate-50 dark:bg-[#0f1219] rounded-lg border border-slate-200 dark:border-slate-800">
              No port mappings defined. Click "Add Port" if needed.
            </div>
            <div v-else class="space-y-1.5">
              <div v-for="(p, idx) in restoreForm.ports" :key="idx" class="flex items-center gap-2">
                <input
                  v-model="p.host"
                  type="text"
                  placeholder="Host port (e.g. 8080)"
                  class="flex-1 bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
                />
                <span class="text-slate-400 font-bold">:</span>
                <input
                  v-model="p.container"
                  type="text"
                  placeholder="Container port (e.g. 80/tcp)"
                  class="flex-1 bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-1.5 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
                />
                <button
                  type="button"
                  @click="removeRestorePort(idx)"
                  class="p-1.5 text-slate-400 hover:text-rose-500 cursor-pointer"
                  title="Remove Port Mapping"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>

          <!-- Environment Variables -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <div class="flex items-center gap-1.5">
                <label class="text-slate-700 dark:text-slate-300 font-bold">Environment Variables</label>
                <button
                  type="button"
                  @click="openInfoPopup('Environment Variables', 'Key-value pairs supplied to the container runtime process. You can edit existing snapshot variables, override credentials, or add new configuration settings.')"
                  class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                  title="Environment Variables Info"
                >
                  <Info class="w-3.5 h-3.5" />
                </button>
              </div>
              <button
                type="button"
                @click="addRestoreEnv"
                class="text-[11px] font-semibold text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 cursor-pointer"
              >
                <Plus class="w-3 h-3" />
                <span>Add Variable</span>
              </button>
            </div>
            <div v-if="restoreForm.env.length === 0" class="text-[11px] text-slate-400 italic p-2 bg-slate-50 dark:bg-[#0f1219] rounded-lg border border-slate-200 dark:border-slate-800">
              No custom environment variables.
            </div>
            <div v-else class="space-y-1.5 max-h-40 overflow-y-auto pr-1">
              <div v-for="(e, idx) in restoreForm.env" :key="idx" class="flex items-center gap-2">
                <input
                  v-model="e.key"
                  type="text"
                  placeholder="KEY"
                  class="w-1/3 bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-2.5 py-1 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
                />
                <span class="text-slate-400 font-bold">=</span>
                <input
                  v-model="e.value"
                  type="text"
                  placeholder="VALUE"
                  class="flex-1 bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-2.5 py-1 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
                />
                <button
                  type="button"
                  @click="removeRestoreEnv(idx)"
                  class="p-1 text-slate-400 hover:text-rose-500 cursor-pointer"
                  title="Remove Variable"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>

          <!-- Restart Policy -->
          <div>
            <div class="flex items-center gap-1.5 mb-1">
              <label class="block text-slate-700 dark:text-slate-300 font-bold">Restart Policy</label>
              <button
                type="button"
                @click="openInfoPopup('Restart Policy', 'Defines whether Docker automatically restarts this container when it exits or when the host daemon restarts:\n\n• unless-stopped: Automatically restarts unless explicitly stopped by user (recommended).\n• always: Always restarts regardless of exit state or manual stop.\n• on-failure: Restarts only if exited with a non-zero exit code.\n• no: Do not restart automatically.')"
                class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                title="Restart Policy Guide"
              >
                <Info class="w-3.5 h-3.5" />
              </button>
            </div>
            <select
              v-model="restoreForm.restartPolicy"
              class="w-full bg-slate-50 dark:bg-[#0f1219] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 font-mono"
            >
              <option value="unless-stopped">unless-stopped (recommended)</option>
              <option value="always">always</option>
              <option value="on-failure">on-failure</option>
              <option value="no">no</option>
            </select>
          </div>

          <!-- Toggles -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
            <label class="flex items-center justify-between p-2.5 rounded-xl bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800 cursor-pointer">
              <div class="flex items-center gap-2">
                <input v-model="restoreForm.startAfterRestore" type="checkbox" class="rounded border-slate-300 dark:border-slate-700 text-blue-600" />
                <span class="font-bold text-slate-800 dark:text-slate-200">Start container after restore</span>
              </div>
              <button
                type="button"
                @click.prevent.stop="openInfoPopup('Start Container After Restore', 'When enabled, the restored container is started immediately upon creation. If unchecked, the container remains created but in an exited/stopped state.')"
                class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                title="Details"
              >
                <Info class="w-3.5 h-3.5" />
              </button>
            </label>
            <label class="flex items-center justify-between p-2.5 rounded-xl bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800 cursor-pointer">
              <div class="flex items-center gap-2">
                <input v-model="restoreForm.replaceExisting" type="checkbox" class="rounded border-slate-300 dark:border-slate-700 text-blue-600" />
                <span class="font-bold text-slate-800 dark:text-slate-200">Replace existing container if exists</span>
              </div>
              <button
                type="button"
                @click.prevent.stop="openInfoPopup('Replace Existing Container', 'If a container with the same name already exists on the target host, it will be safely stopped and removed before the restored container is created.')"
                class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
                title="Details"
              >
                <Info class="w-3.5 h-3.5" />
              </button>
            </label>
          </div>

          <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-slate-800 shrink-0">
            <button
              type="button"
              @click="isRestoreContainerModalOpen = false"
              class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="restoreContainerRunning"
              class="flex items-center gap-1.5 px-4 py-2 text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-lg shadow disabled:opacity-50 transition cursor-pointer"
            >
              <RotateCw v-if="restoreContainerRunning" class="w-3.5 h-3.5 animate-spin" />
              <RotateCcw v-else class="w-3.5 h-3.5" />
              <span>{{ restoreContainerRunning ? 'Restoring & Loading Image...' : 'Restore Container' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL: INSPECT DOCKER CONTAINER BACKUP -->
    <!-- ============================================================= -->
    <div v-if="isInspectContainerModalOpen && inspectedContainerBackup" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm p-4 animate-in fade-in">
      <div class="w-full max-w-xl bg-white dark:bg-[#171a23] border border-slate-200 dark:border-slate-700 rounded-2xl p-6 shadow-2xl space-y-4 font-sans max-h-[90vh] flex flex-col">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3 shrink-0">
          <div class="flex items-center gap-2.5">
            <div class="p-2 rounded-lg bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400">
              <Eye class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white">Container Snapshot Metadata</h3>
              <p class="text-[11px] font-mono text-slate-500 truncate max-w-md">{{ inspectedContainerBackup.filename }}</p>
            </div>
          </div>
          <button @click="isInspectContainerModalOpen = false" class="text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="flex-1 overflow-y-auto space-y-3 text-xs pr-1">
          <!-- Overview Cards -->
          <div class="grid grid-cols-2 gap-2">
            <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
              <span class="text-[10px] text-slate-500 block uppercase font-bold">Container Name</span>
              <span class="font-bold text-slate-900 dark:text-white font-mono">{{ inspectedContainerBackup.containerName }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
              <span class="text-[10px] text-slate-500 block uppercase font-bold">Source Image</span>
              <span class="font-mono text-slate-900 dark:text-white truncate block">{{ inspectedContainerBackup.sourceImage }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
              <span class="text-[10px] text-slate-500 block uppercase font-bold">Storage Path</span>
              <span class="font-mono text-slate-700 dark:text-slate-300 truncate block">backup/container-docker/{{ inspectedContainerBackup.filename }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800">
              <span class="text-[10px] text-slate-500 block uppercase font-bold">Archive Size</span>
              <span class="font-bold text-slate-900 dark:text-white">{{ formatFileSize(inspectedContainerBackup.fileSize) }}</span>
            </div>
          </div>

          <!-- Port Bindings -->
          <div class="p-3 rounded-xl bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800 space-y-1.5">
            <span class="text-[10px] text-slate-500 uppercase font-bold block">Port Mappings</span>
            <div v-if="(!inspectedContainerBackup.portBindings || Object.keys(inspectedContainerBackup.portBindings).length === 0) && (!inspectedContainerBackup.ports || inspectedContainerBackup.ports.length === 0)" class="text-slate-400 italic">
              No exposed ports recorded.
            </div>
            <div v-else class="space-y-1 font-mono text-[11px]">
              <template v-if="Array.isArray(inspectedContainerBackup.ports) && inspectedContainerBackup.ports.length > 0">
                <div v-for="(p, idx) in inspectedContainerBackup.ports" :key="idx" class="flex items-center justify-between bg-white dark:bg-[#171a23] px-2.5 py-1 rounded border border-slate-200 dark:border-slate-800">
                  <span class="text-slate-500">Container: {{ p.containerPort || p.container }}</span>
                  <span class="font-bold text-blue-600 dark:text-blue-400">Host: {{ p.hostPort || p.host }}</span>
                </div>
              </template>
              <template v-else-if="inspectedContainerBackup.portBindings">
                <div v-for="(hostP, contP) in inspectedContainerBackup.portBindings" :key="contP" class="flex items-center justify-between bg-white dark:bg-[#171a23] px-2.5 py-1 rounded border border-slate-200 dark:border-slate-800">
                  <span class="text-slate-500">Container: {{ contP }}</span>
                  <span class="font-bold text-blue-600 dark:text-blue-400">Host: {{ hostP }}</span>
                </div>
              </template>
            </div>
          </div>

          <!-- Environment Variables -->
          <div class="p-3 rounded-xl bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800 space-y-1.5">
            <span class="text-[10px] text-slate-500 uppercase font-bold block">Environment Variables ({{ inspectedContainerBackup.env?.length || 0 }})</span>
            <div v-if="!inspectedContainerBackup.env || inspectedContainerBackup.env.length === 0" class="text-slate-400 italic">
              No environment variables recorded.
            </div>
            <div
              v-else
              data-terminal="true"
              class="terminal-log-viewer max-h-36 overflow-y-auto space-y-1 font-mono text-[10px] p-2.5 rounded-lg whitespace-pre-wrap select-text"
              style="background-color: #080b14 !important; color: #4ade80 !important;"
            >
              <div v-for="(e, idx) in inspectedContainerBackup.env" :key="idx">
                {{ typeof e === 'object' && e !== null ? (e.key ? `${e.key}=${e.value || ''}` : JSON.stringify(e)) : e }}
              </div>
            </div>
          </div>

          <!-- Volumes / Mounts -->
          <div v-if="inspectedContainerBackup.volumes && inspectedContainerBackup.volumes.length > 0" class="p-3 rounded-xl bg-slate-50 dark:bg-[#0f1219] border border-slate-200 dark:border-slate-800 space-y-1.5">
            <span class="text-[10px] text-slate-500 uppercase font-bold block">Mounted Volumes</span>
            <div class="space-y-1 font-mono text-[11px]">
              <div v-for="(v, idx) in inspectedContainerBackup.volumes" :key="idx" class="bg-white dark:bg-[#171a23] px-2.5 py-1 rounded border border-slate-200 dark:border-slate-800 truncate">
                {{ v }}
              </div>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-slate-800 shrink-0">
          <button
            @click="isInspectContainerModalOpen = false"
            class="px-4 py-1.5 rounded-lg bg-slate-200 hover:bg-slate-300 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-white font-bold text-xs transition cursor-pointer"
          >
            Close
          </button>
          <button
            v-if="canManage"
            @click="isInspectContainerModalOpen = false; openRestoreContainerModal(inspectedContainerBackup)"
            class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-bold text-xs shadow transition cursor-pointer"
          >
            <RotateCcw class="w-3.5 h-3.5" />
            <span>Proceed to Restore</span>
          </button>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 3: STANDARD DELETE CONFIRMATION MODAL (Strictly AGENTS.md) -->
    <!-- ============================================================= -->
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
            Delete {{ itemToDelete.type === 'database' ? 'Database Config' : itemToDelete.type === 'destination' ? 'Storage Destination' : itemToDelete.type === 'schedule' ? 'Cron Schedule' : itemToDelete.type === 'container' ? 'Container Backup' : 'Database Backup' }}?
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

    <!-- ============================================================= -->
    <!-- INFORMATION POPUP MODAL (Quick Help Guide) -->
    <!-- ============================================================= -->
    <div
      v-if="activeInfoPopup"
      class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
      @click.self="activeInfoPopup = null"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4">
        <div class="flex items-start justify-between gap-3 border-b border-slate-100 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 flex items-center justify-center shrink-0">
              <Info class="w-4 h-4" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white leading-tight">
                {{ activeInfoPopup.title }}
              </h3>
              <span class="text-[10px] text-slate-400 font-medium">Information Guide</span>
            </div>
          </div>
          <button
            type="button"
            @click="activeInfoPopup = null"
            class="text-slate-400 hover:text-slate-700 dark:hover:text-white p-1 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="text-xs text-slate-600 dark:text-slate-300 leading-relaxed whitespace-pre-line">
          {{ activeInfoPopup.description }}
        </div>

        <div class="flex justify-end pt-2 border-t border-slate-100 dark:border-slate-800">
          <button
            type="button"
            @click="activeInfoPopup = null"
            class="px-4 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-200 rounded-lg text-xs font-semibold transition cursor-pointer"
          >
            Close
          </button>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- LIVE CLI EXECUTION PROGRESS CONSOLE MODAL -->
    <!-- ============================================================= -->
    <div
      v-if="isCliModalOpen"
      class="fixed inset-0 z-[70] flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh]">
        <!-- Modal Header -->
        <div class="flex items-center justify-between px-5 py-4 border-b border-slate-100 dark:border-[#1c2438]">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-200 flex items-center justify-center shrink-0">
              <Terminal class="w-4 h-4" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-sm font-bold text-slate-900 dark:text-white">
                  {{ cliModalTitle }}
                </h3>
                <span
                  v-if="cliIsRunning"
                  class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/10 text-emerald-500 border border-emerald-500/20"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                  Processing ({{ cliTimerSec }}s)
                </span>
                <span
                  v-else-if="cliIsSuccess"
                  class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/10 text-emerald-500 border border-emerald-500/20"
                >
                  Completed in {{ cliTimerSec }}s
                </span>
                <span
                  v-else-if="cliError"
                  class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium bg-rose-500/10 text-rose-500 border border-rose-500/20"
                >
                  Failed after {{ cliTimerSec }}s
                </span>
              </div>
              <p class="text-[11px] text-slate-500 dark:text-slate-400 font-mono mt-0.5">
                Target: {{ cliModalTarget }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="copyCliLogs"
              :disabled="cliLogs.length === 0"
              class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 rounded-lg transition cursor-pointer disabled:opacity-40"
              title="Copy console output"
            >
              <Check v-if="copiedCliLogs" class="w-3.5 h-3.5 text-emerald-500" />
              <Copy v-else class="w-3.5 h-3.5" />
              <span>{{ copiedCliLogs ? 'Copied' : 'Copy' }}</span>
            </button>
            <button
              @click="closeCliModal"
              class="text-slate-400 hover:text-slate-700 dark:hover:text-white p-1 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
              :title="cliIsRunning ? 'Minimize to background' : 'Close'"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Terminal Output Window -->
        <div
          ref="cliTerminalRef"
          data-terminal="true"
          class="cli-terminal-box terminal-log-viewer flex-1 overflow-y-auto p-4 font-mono text-[11px] leading-relaxed select-text space-y-1.5 min-h-[300px] max-h-[460px]"
          style="background-color: #080b14 !important; color: #f8fafc !important;"
        >
          <div
            v-for="(log, idx) in cliLogs"
            :key="idx"
            class="flex items-start gap-2.5 font-mono"
          >
            <span
              class="shrink-0 select-none text-[10px]"
              style="color: #94a3b8 !important;"
            >
              [{{ log.timestamp }}]
            </span>
            <span
              class="uppercase text-[9px] px-1.5 py-0.5 rounded shrink-0 font-bold tracking-wider"
              :style="
                log.status === 'error'
                  ? 'background-color: #4c0519 !important; color: #fb7185 !important; border: 1px solid #e11d48 !important;'
                  : log.status === 'success'
                  ? 'background-color: #064e3b !important; color: #4ade80 !important; border: 1px solid #059669 !important;'
                  : log.status === 'progress'
                  ? 'background-color: #1e1b4b !important; color: #a5b4fc !important; border: 1px solid #4f46e5 !important;'
                  : 'background-color: #1e293b !important; color: #38bdf8 !important; border: 1px solid #0284c7 !important;'
              "
            >
              {{ log.step }}
            </span>
            <span
              class="flex-1 break-all"
              :style="
                log.status === 'error'
                  ? 'color: #fb7185 !important; font-weight: 600;'
                  : log.status === 'success'
                  ? 'color: #4ade80 !important; font-weight: 600;'
                  : log.status === 'progress'
                  ? 'color: #93c5fd !important;'
                  : 'color: #f1f5f9 !important;'
              "
            >
              {{ log.message }}
            </span>
          </div>

          <div
            v-if="cliIsRunning"
            class="flex items-center gap-2 pt-2 animate-pulse font-mono text-[10px]"
            style="color: #94a3b8 !important;"
          >
            <span class="inline-block w-2 h-3.5" style="background-color: #38bdf8 !important;"></span>
            <span>Running background task...</span>
          </div>
        </div>

        <!-- Error Summary Box (If failed) -->
        <div
          v-if="cliError"
          class="px-5 py-3 bg-rose-50 dark:bg-rose-950/40 border-t border-rose-200 dark:border-rose-900/50 flex items-start gap-3"
        >
          <AlertCircle class="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
          <div class="text-xs text-rose-800 dark:text-rose-300 space-y-0.5">
            <span class="font-bold">Process Encountered An Error:</span>
            <p class="font-mono text-[11px] break-all">{{ cliError }}</p>
          </div>
        </div>

        <!-- Success Summary Box (If completed) -->
        <div
          v-else-if="cliIsSuccess"
          class="px-5 py-3 bg-emerald-50 dark:bg-emerald-950/40 border-t border-emerald-200 dark:border-emerald-900/50 flex items-center gap-3"
        >
          <CheckCircle2 class="w-4 h-4 text-emerald-500 shrink-0" />
          <span class="text-xs font-semibold text-emerald-800 dark:text-emerald-300">
            {{ cliModalType === 'restore' ? 'Container restored and running successfully!' : 'Container backup snapshot created successfully!' }}
          </span>
        </div>

        <!-- Modal Footer -->
        <div class="flex items-center justify-between px-5 py-3 bg-slate-50 dark:bg-[#0c101c] border-t border-slate-100 dark:border-[#1c2438]">
          <span class="text-[11px] text-slate-500 dark:text-slate-400">
            {{ cliIsRunning ? 'Task will safely continue running in the background if closed.' : 'Process execution finished.' }}
          </span>
          <div class="flex items-center gap-2">
            <button
              v-if="cliIsRunning"
              type="button"
              @click="minimizeCliModal"
              class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition cursor-pointer shadow-sm"
            >
              Run in Background
            </button>
            <button
              v-else
              type="button"
              @click="closeCliModal"
              class="px-4 py-1.5 bg-slate-800 hover:bg-slate-700 text-white rounded-lg text-xs font-bold transition cursor-pointer"
            >
              {{ cliIsSuccess ? 'Done' : 'Close' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Notification Toast (3000ms auto-dismiss per AGENTS.md) -->
    <div
      v-if="notification"
      class="fixed bottom-6 right-6 z-50 flex items-center gap-2.5 px-4 py-3 rounded-xl border shadow-xl transition-all animate-in fade-in slide-in-from-bottom-2 text-xs font-semibold backdrop-blur-sm"
      :class="[
        notification.type === 'success' ? 'bg-emerald-50/95 dark:bg-emerald-950/90 text-emerald-900 dark:text-emerald-200 border-emerald-300 dark:border-emerald-800' :
        notification.type === 'warning' ? 'bg-amber-50/95 dark:bg-amber-950/90 text-amber-900 dark:text-amber-200 border-amber-300 dark:border-amber-800' :
        'bg-rose-50/95 dark:bg-rose-950/90 text-rose-900 dark:text-rose-200 border-rose-300 dark:border-rose-800'
      ]"
    >
      <CheckCircle2 v-if="notification.type === 'success'" class="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
      <AlertCircle v-else class="w-4 h-4 text-rose-600 dark:text-rose-400 shrink-0" />
      <span>{{ notification.message }}</span>
      <button @click="notification = null" class="ml-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

  </div>
</template>
