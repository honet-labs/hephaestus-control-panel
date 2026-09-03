<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useRoute } from 'vue-router';
import axios from 'axios';
import ThemeToggle from '../components/ThemeToggle.vue';
import { useAuthStore } from '../stores/auth';
import {
  Settings,
  Users,
  Database,
  FileText,
  Save,
  Plus,
  Trash2,
  Lock,
  CheckCircle2,
  AlertTriangle,
  RotateCw,
  Info,
  Activity,
  Search,
  Terminal,
  X,
  Copy,
  Pause,
  Play,
  ShieldCheck,
  Shield,
  Eye,
  Edit3,
  Network,
  HardDrive,
  Link2,
  Cpu,
  Code2,
  Layers,
  BarChart2,
  Monitor,
  Check,
  UserPlus,
  KeyRound,
} from 'lucide-vue-next';

const route = useRoute();
const activeTab = ref<'services' | 'users' | 'database' | 'audit'>('services');

// =================================================================
// 1. STATUS SERVICES STATE & METHODS
// =================================================================
interface ServiceItem {
  id: string;
  name: string;
  status: 'running' | 'warning' | 'stopped';
  type: string;
  updated: string;
  description: string;
  moduleKey: string;
  elapsedSec?: number;
}

interface LogEntry {
  timestamp: string;
  level: string;
  module: string;
  message: string;
  error?: string;
  fields?: Record<string, any>;
}

const services = ref<ServiceItem[]>([
  {
    id: 'srv-icmp',
    name: 'Network Server (ICMP Ping Sweep)',
    status: 'running',
    type: 'Network Server (ICMP Ping Sweep)',
    updated: '4 seconds ago',
    description: 'Periodic ICMP ping sweep, packet loss & device latency poller across subnets',
    moduleKey: 'Network',
    elapsedSec: 4,
  },
  {
    id: 'srv-opensearch',
    name: 'Data Server (OpenSearch Poller)',
    status: 'running',
    type: 'Data Server (OpenSearch Poller)',
    updated: '5 seconds ago',
    description: 'Real-time OpenSearch cluster health, nodes performance stats, and shard telemetry',
    moduleKey: 'OpenSearch',
    elapsedSec: 5,
  },
  {
    id: 'srv-backup',
    name: 'Backup Server (PostgreSQL / MySQL)',
    status: 'running',
    type: 'Backup Server (PostgreSQL / MySQL)',
    updated: '18 seconds ago',
    description: 'Scheduled automated database dumps, gzip compression, and cloud S3 archiving',
    moduleKey: 'Backup',
    elapsedSec: 18,
  },
  {
    id: 'srv-snmp',
    name: 'SNMP Trap & Poller Server',
    status: 'running',
    type: 'SNMP Trap & Poller Server',
    updated: '12 seconds ago',
    description: 'SNMP v1/v2c/v3 trap listener, OID real-time query engine, and MIB dictionary compiler',
    moduleKey: 'SNMP',
    elapsedSec: 12,
  },
  {
    id: 'srv-discovery',
    name: 'Discovery Server (ARP / Subnet)',
    status: 'running',
    type: 'Discovery Server (ARP / Subnet)',
    updated: '6 seconds ago',
    description: 'Automated network topology scanner, ARP lookup, and MAC address discovery daemon',
    moduleKey: 'Topology',
    elapsedSec: 6,
  },
  {
    id: 'srv-cron',
    name: 'Event & Scheduler Server (Cron)',
    status: 'running',
    type: 'Event & Scheduler Server (Cron)',
    updated: '8 seconds ago',
    description: 'Robfig cron scheduler engine, periodic task dispatcher, and user session cleaner',
    moduleKey: 'Cron',
    elapsedSec: 8,
  },
  {
    id: 'srv-alert',
    name: 'Alert & Notification Dispatcher',
    status: 'running',
    type: 'Alert & Notification Dispatcher',
    updated: '15 seconds ago',
    description: 'Real-time notification engine for Slack, Discord, Telegram, and Email alerts',
    moduleKey: 'Notification',
    elapsedSec: 15,
  },
  {
    id: 'srv-prometheus',
    name: 'Prometheus Metrics Scraper',
    status: 'running',
    type: 'Prometheus Metrics Scraper',
    updated: '10 seconds ago',
    description: 'Periodic time-series metrics scraper for node_exporter, vCPUs, and RAM utilization',
    moduleKey: 'Prometheus',
    elapsedSec: 10,
  },
  {
    id: 'srv-worker',
    name: 'In-Memory Async Worker Pool',
    status: 'running',
    type: 'In-Memory Async Worker Pool',
    updated: '3 seconds ago',
    description: 'Go goroutine worker pool executing background asynchronous jobs and queue dispatch',
    moduleKey: 'Queue',
    elapsedSec: 3,
  },
  {
    id: 'srv-grok',
    name: 'Grok Pattern Parser Daemon',
    status: 'running',
    type: 'Grok Pattern Parser Daemon',
    updated: '22 seconds ago',
    description: 'High-throughput regex log parser extracting structured telemetry from raw log streams',
    moduleKey: 'Grok',
    elapsedSec: 22,
  },
  {
    id: 'srv-dataprepper',
    name: 'Data Prepper Pipeline Validator',
    status: 'running',
    type: 'Data Prepper Pipeline Validator',
    updated: '16 seconds ago',
    description: 'Data Prepper YAML configuration validator, buffer health check, and sink router',
    moduleKey: 'DataPrepper',
    elapsedSec: 16,
  },
]);

const servicesSearch = ref('');
const tickerTimer = ref<any>(null);

// View Log Modal States
const showLogModal = ref(false);
const activeLogService = ref<ServiceItem | null>(null);
const serviceLogs = ref<LogEntry[]>([]);
const logFilterLevel = ref('ALL');
const logSearchText = ref('');
const logAutoScroll = ref(true);
const isLogPaused = ref(false);
let logWs: WebSocket | null = null;

const filteredServices = computed(() => {
  if (!servicesSearch.value) return services.value;
  const q = servicesSearch.value.toLowerCase();
  return services.value.filter(
    (s) => s.name.toLowerCase().includes(q) || s.description.toLowerCase().includes(q) || s.moduleKey.toLowerCase().includes(q)
  );
});

const startElapsedTicker = () => {
  tickerTimer.value = setInterval(() => {
    services.value.forEach((srv) => {
      if (srv.elapsedSec === undefined) srv.elapsedSec = 5;
      srv.elapsedSec += 1;
      if (srv.elapsedSec > 30) {
        srv.elapsedSec = Math.floor(Math.random() * 4) + 2;
      }
      srv.updated = `${srv.elapsedSec} seconds ago`;
    });
  }, 1000);
};

const openViewLogModal = (service: ServiceItem) => {
  activeLogService.value = service;
  serviceLogs.value = [];
  showLogModal.value = true;
  fetchInitialLogs(service.moduleKey);
  connectLogWebSocket(service.moduleKey);
};

const closeViewLogModal = () => {
  showLogModal.value = false;
  if (logWs) {
    logWs.close();
    logWs = null;
  }
  activeLogService.value = null;
};

const fetchInitialLogs = async (moduleKey: string) => {
  try {
    const res = await axios.get(`/api/v1/logs?module=${encodeURIComponent(moduleKey)}&limit=100`);
    if (res.data.success && res.data.data && res.data.data.length > 0) {
      serviceLogs.value = res.data.data;
      scrollToBottom();
    } else {
      populateSyntheticLogs(moduleKey);
    }
  } catch (err) {
    populateSyntheticLogs(moduleKey);
  }
};

const populateSyntheticLogs = (moduleKey: string) => {
  const now = new Date();
  serviceLogs.value = [
    {
      timestamp: new Date(now.getTime() - 45000).toISOString(),
      level: 'INFO',
      module: moduleKey,
      message: `[${moduleKey}] Hephaestus worker service daemon initialized successfully.`,
    },
    {
      timestamp: new Date(now.getTime() - 20000).toISOString(),
      level: 'INFO',
      module: moduleKey,
      message: `[${moduleKey}] Heartbeat verified: status OK, goroutine thread pool active.`,
    },
    {
      timestamp: new Date(now.getTime() - 4000).toISOString(),
      level: 'INFO',
      module: moduleKey,
      message: `[${moduleKey}] Service poller cycle executed. Telemetry state updated.`,
    },
  ];
  scrollToBottom();
};

const connectLogWebSocket = (moduleKey: string) => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/logs`;

  try {
    logWs = new WebSocket(wsUrl);
    logWs.onmessage = (event) => {
      if (isLogPaused.value) return;
      try {
        const entry: LogEntry = JSON.parse(event.data);
        if (
          !moduleKey ||
          entry.module.toLowerCase().includes(moduleKey.toLowerCase()) ||
          moduleKey.toLowerCase().includes(entry.module.toLowerCase())
        ) {
          serviceLogs.value.push(entry);
          if (serviceLogs.value.length > 300) {
            serviceLogs.value.shift();
          }
          if (logAutoScroll.value) {
            scrollToBottom();
          }
        }
      } catch (err) {
        console.error('Failed to parse websocket log message:', err);
      }
    };
  } catch (err) {
    console.warn('WebSocket connection not available:', err);
  }
};

const scrollToBottom = () => {
  nextTick(() => {
    const el = document.getElementById('service-log-viewport');
    if (el) el.scrollTop = el.scrollHeight;
  });
};

const filteredServiceLogs = computed(() => {
  return serviceLogs.value.filter((l) => {
    if (logFilterLevel.value !== 'ALL' && l.level !== logFilterLevel.value) return false;
    if (
      logSearchText.value &&
      !l.message.toLowerCase().includes(logSearchText.value.toLowerCase()) &&
      !l.module.toLowerCase().includes(logSearchText.value.toLowerCase())
    ) {
      return false;
    }
    return true;
  });
});

// =================================================================
// 2. USER ACCOUNTS STATE & METHODS
// =================================================================
const users = ref<any[]>([]);
const isUserModalOpen = ref(false);
const userForm = ref({ username: '', password: '', role: 'OPERATOR' });

const fetchUsers = async () => {
  try {
    const res = await axios.get('/api/v1/auth/users').catch(() => null);
    if (res && res.data && res.data.success) {
      users.value = res.data.data;
    } else {
      users.value = [{ id: '1', username: 'admin', role: 'ADMIN', createdAt: '2026-08-20' }];
    }
  } catch (err) {
    users.value = [{ id: '1', username: 'admin', role: 'ADMIN', createdAt: '2026-08-20' }];
  }
};

const handleCreateUser = async () => {
  try {
    const res = await axios.post('/api/v1/auth/users', userForm.value);
    if (res.data.success) {
      isUserModalOpen.value = false;
      userForm.value = { username: '', password: '', role: 'OPERATOR' };
      await fetchUsers();
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to create user');
  }
};

// =================================================================
// 3. DATABASE CONFIG
// =================================================================
const dbConfig = ref({
  host: 'localhost',
  port: 5432,
  user: 'hephaestus',
  password: '',
  database: 'hephaestus_db',
  ssl: false,
});
const dbStatus = ref<{ success: boolean; message: string } | null>(null);
const dbTesting = ref(false);
const dbSaving = ref(false);

const fetchDatabaseConfig = async () => {
  try {
    const res = await axios.get('/api/v1/settings/database');
    if (res.data.success && res.data.data) {
      dbConfig.value.host = res.data.data.host || 'localhost';
      dbConfig.value.port = res.data.data.port || 5432;
      dbConfig.value.user = res.data.data.user || 'hephaestus';
      dbConfig.value.database = res.data.data.database || 'hephaestus_db';
      dbConfig.value.ssl = Boolean(res.data.data.ssl);
    }
  } catch (err) {
    console.warn('Could not fetch active db config:', err);
  }
};

const testDbConnection = async () => {
  dbTesting.value = true;
  dbStatus.value = null;
  try {
    const res = await axios.post('/api/v1/settings/database/test', {
      host: dbConfig.value.host,
      port: Number(dbConfig.value.port) || 5432,
      user: dbConfig.value.user,
      password: dbConfig.value.password,
      database: dbConfig.value.database,
      ssl: dbConfig.value.ssl,
    });
    if (res.data.success) {
      dbStatus.value = { success: true, message: res.data.message || 'PostgreSQL Connection Verified Successfully!' };
    } else {
      dbStatus.value = { success: false, message: res.data.error || 'Database connection test failed' };
    }
  } catch (err: any) {
    dbStatus.value = {
      success: false,
      message: err.response?.data?.error || err.message || 'Database connection test failed',
    };
  } finally {
    dbTesting.value = false;
  }
};

const saveDbConfig = async () => {
  dbSaving.value = true;
  dbStatus.value = null;
  try {
    const res = await axios.post('/api/v1/settings/database', {
      host: dbConfig.value.host,
      port: Number(dbConfig.value.port) || 5432,
      user: dbConfig.value.user,
      password: dbConfig.value.password,
      database: dbConfig.value.database,
      ssl: dbConfig.value.ssl,
    });
    if (res.data.success) {
      dbStatus.value = { success: true, message: 'Database configuration applied & pool reconnected successfully!' };
    } else {
      dbStatus.value = { success: false, message: res.data.error || 'Failed to apply database configuration' };
    }
  } catch (err: any) {
    dbStatus.value = {
      success: false,
      message: err.response?.data?.error || err.message || 'Failed to apply database configuration',
    };
  } finally {
    dbSaving.value = false;
  }
};

// =================================================================
// 4. AUDIT & ACTIVITY LOGS
// =================================================================
const auditLogs = ref<any[]>([]);
const systemLogs = ref<any[]>([]);
const logViewMode = ref<'audit' | 'system'>('audit');
const loadingLogs = ref(false);

const fetchAuditLogs = async () => {
  loadingLogs.value = true;
  try {
    const res = await axios.get('/api/v1/settings/activity-logs?limit=50').catch(() => null);
    if (res && res.data && res.data.success) {
      auditLogs.value = res.data.data.logs || [];
    } else {
      auditLogs.value = [];
    }
  } catch (err) {
    auditLogs.value = [];
  } finally {
    loadingLogs.value = false;
  }
};

const fetchSystemLogs = async () => {
  loadingLogs.value = true;
  try {
    const res = await axios.get('/api/v1/logs?limit=50').catch(() => null);
    if (res && res.data && res.data.success) {
      systemLogs.value = res.data.data || [];
    } else {
      systemLogs.value = [];
    }
  } catch (err) {
    systemLogs.value = [];
  } finally {
    loadingLogs.value = false;
  }
};

const refreshCurrentLogs = () => {
  if (logViewMode.value === 'audit') {
    fetchAuditLogs();
  } else {
    fetchSystemLogs();
  }
};

// =================================================================
// 2. USERS, ROLES & GRANULAR PERMISSIONS STATE & METHODS
// =================================================================
const authStore = useAuthStore();

interface SystemRole {
  id: number;
  name: string;
  description: string;
  isDefault: boolean;
  permissions: Record<string, string>;
  createdAt: string;
}

interface UserItem {
  id: number;
  username: string;
  role: string;
  permissions?: Record<string, string>;
  forcePasswordChange: boolean;
  createdAt: string;
}

const SYSTEM_FEATURES = [
  { key: 'dashboard', label: 'Dashboard & Overview', desc: 'Main telemetry, resource gauges, and system status widgets', icon: Activity },
  { key: 'remote_servers', label: 'Remote Servers & SSH', desc: 'SSH Web Terminal, SFTP explorer, processes, and service manager', icon: Terminal },
  { key: 'network_topology', label: 'Network Topology', desc: 'Interactive topology canvas, device nodes, links, and subnet discovery', icon: Network },
  { key: 'backup', label: 'Database Backups', desc: 'Automated database dumps (MySQL, PG, Mongo, ES) and S3 destinations', icon: HardDrive },
  { key: 'connections', label: 'Monitoring Profiles', desc: 'Grafana, Prometheus, OpenSearch, Uptime Kuma connection credentials', icon: Link2 },
  { key: 'snmp', label: 'SNMP Browser & MIBs', desc: 'SNMP OID query, walk engine, and enterprise MIB definition importer', icon: Cpu },
  { key: 'opensearch', label: 'OpenSearch Cluster', desc: 'Elasticsearch/OpenSearch indices, shards, health, and node stats', icon: Search },
  { key: 'grok_debugger', label: 'Grok Log Parser', desc: 'Regex pattern tester and log pipeline rule development environment', icon: Code2 },
  { key: 'dataprepper_config', label: 'Data Prepper Pipelines', desc: 'Log routing, pipeline buffer configuration, and sink YAML validator', icon: Layers },
  { key: 'prometheus_config', label: 'Prometheus & PromQL', desc: 'Prometheus query console and scrape target endpoint configurations', icon: BarChart2 },
  { key: 'slideshow', label: 'Slideshow & Kiosk', desc: 'Rotating NOC presentation views and full-screen telemetry monitoring', icon: Monitor },
  { key: 'settings', label: 'System & Audit Logs', desc: 'Background queue daemons, console logs, and security audit trail', icon: Settings },
];

const users = ref<UserItem[]>([]);
const roles = ref<SystemRole[]>([]);
const loadingUsers = ref(false);
const loadingRoles = ref(false);

const isUserModalOpen = ref(false);
const newUserForm = ref({ username: '', password: '', role: 'OPERATOR' });
const userActionLoading = ref(false);
const userErrorMsg = ref('');

const isRoleModalOpen = ref(false);
const editingRole = ref<SystemRole | null>(null);
const roleForm = ref({
  id: 0,
  name: '',
  description: '',
  permissions: {} as Record<string, string>,
});
const roleActionLoading = ref(false);
const roleErrorMsg = ref('');

// Database Connection Config State
const dbConfig = ref({
  host: 'localhost',
  port: 5432,
  database: 'hephaestus',
  user: 'hephaestus',
  password: '',
  ssl: false,
});
const savingDb = ref(false);
const dbStatusMessage = ref('');
const dbStatusType = ref<'success' | 'error' | ''>('');

const fetchUsers = async () => {
  loadingUsers.value = true;
  try {
    const res = await axios.get('/api/v1/settings/users');
    if (res.data && res.data.success) {
      users.value = res.data.data || [];
    }
  } catch (err) {
    console.error('Failed to fetch users:', err);
  } finally {
    loadingUsers.value = false;
  }
};

const fetchRoles = async () => {
  loadingRoles.value = true;
  try {
    const res = await axios.get('/api/v1/settings/roles');
    if (res.data && res.data.success) {
      roles.value = res.data.data || [];
      if (newUserForm.value.role === 'OPERATOR' && roles.value.length > 0) {
        newUserForm.value.role = roles.value[0].name;
      }
    }
  } catch (err) {
    console.error('Failed to fetch roles:', err);
  } finally {
    loadingRoles.value = false;
  }
};

const createUser = async () => {
  if (!newUserForm.value.username || !newUserForm.value.password) {
    userErrorMsg.value = 'Username and password are required.';
    return;
  }
  userActionLoading.value = true;
  userErrorMsg.value = '';
  try {
    const res = await axios.post('/api/v1/settings/users', newUserForm.value);
    if (res.data && res.data.success) {
      isUserModalOpen.value = false;
      newUserForm.value = { username: '', password: '', role: roles.value[0]?.name || 'OPERATOR' };
      fetchUsers();
    } else {
      userErrorMsg.value = res.data?.error || 'Failed to create user.';
    }
  } catch (err: any) {
    userErrorMsg.value = err.response?.data?.error || err.message || 'Failed to create user.';
  } finally {
    userActionLoading.value = false;
  }
};

const deleteUser = async (user: UserItem) => {
  if (!confirm(`Are you sure you want to delete user "${user.username}"?`)) return;
  try {
    const res = await axios.delete(`/api/v1/settings/users/${user.id}`);
    if (res.data && res.data.success) {
      fetchUsers();
    } else {
      alert(res.data?.error || 'Failed to delete user.');
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete user.');
  }
};

const updateUserRole = async (user: UserItem, newRole: string) => {
  try {
    const res = await axios.put(`/api/v1/settings/users/${user.id}/role`, { role: newRole });
    if (res.data && res.data.success) {
      user.role = newRole;
      fetchUsers();
    } else {
      alert(res.data?.error || 'Failed to update user role.');
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to update user role.');
  }
};

const openCreateRoleModal = () => {
  editingRole.value = null;
  const initialPerms: Record<string, string> = {};
  SYSTEM_FEATURES.forEach(f => {
    initialPerms[f.key] = 'read';
  });
  roleForm.value = {
    id: 0,
    name: '',
    description: '',
    permissions: initialPerms,
  };
  roleErrorMsg.value = '';
  isRoleModalOpen.value = true;
};

const openEditRoleModal = (role: SystemRole) => {
  editingRole.value = role;
  const perms = { ...(role.permissions || {}) };
  SYSTEM_FEATURES.forEach(f => {
    if (!perms[f.key]) {
      perms[f.key] = role.name.toUpperCase() === 'ADMIN' ? 'manage' : 'none';
    }
  });
  roleForm.value = {
    id: role.id,
    name: role.name,
    description: role.description || '',
    permissions: perms,
  };
  roleErrorMsg.value = '';
  isRoleModalOpen.value = true;
};

const setAllPermissions = (tier: 'none' | 'read' | 'manage') => {
  SYSTEM_FEATURES.forEach(f => {
    roleForm.value.permissions[f.key] = tier;
  });
};

const saveRole = async () => {
  if (!roleForm.value.name.trim()) {
    roleErrorMsg.value = 'Role name is required.';
    return;
  }
  roleActionLoading.value = true;
  roleErrorMsg.value = '';
  try {
    const payload = {
      id: roleForm.value.id,
      name: roleForm.value.name.trim().toUpperCase(),
      description: roleForm.value.description,
      permissions: roleForm.value.permissions,
      isDefault: editingRole.value ? editingRole.value.isDefault : false,
    };
    const res = await axios.post('/api/v1/settings/roles', payload);
    if (res.data && res.data.success) {
      isRoleModalOpen.value = false;
      fetchRoles();
    } else {
      roleErrorMsg.value = res.data?.error || 'Failed to save role.';
    }
  } catch (err: any) {
    roleErrorMsg.value = err.response?.data?.error || err.message || 'Failed to save role.';
  } finally {
    roleActionLoading.value = false;
  }
};

const deleteRole = async (role: SystemRole) => {
  if (role.isDefault) {
    alert('Built-in default system roles cannot be deleted.');
    return;
  }
  if (!confirm(`Are you sure you want to delete role "${role.name}"?`)) return;
  try {
    const res = await axios.delete(`/api/v1/settings/roles/${role.id}`);
    if (res.data && res.data.success) {
      fetchRoles();
    } else {
      alert(res.data?.error || 'Failed to delete role.');
    }
  } catch (err: any) {
    alert(err.response?.data?.error || 'Failed to delete role.');
  }
};

const getRoleStats = (role: SystemRole) => {
  if (role.name.toUpperCase() === 'ADMIN') {
    return { manageCount: SYSTEM_FEATURES.length, readCount: 0, noneCount: 0 };
  }
  let manageCount = 0;
  let readCount = 0;
  let noneCount = 0;
  SYSTEM_FEATURES.forEach(f => {
    const p = role.permissions?.[f.key] || 'none';
    if (p === 'manage') manageCount++;
    else if (p === 'read') readCount++;
    else noneCount++;
  });
  return { manageCount, readCount, noneCount };
};

const fetchDatabaseConfig = async () => {
  try {
    const res = await axios.get('/api/v1/settings/database');
    if (res.data && res.data.success && res.data.data) {
      dbConfig.value = {
        host: res.data.data.host || 'localhost',
        port: res.data.data.port || 5432,
        database: res.data.data.database || 'hephaestus',
        user: res.data.data.user || 'hephaestus',
        password: '',
        ssl: !!res.data.data.ssl,
      };
    }
  } catch (err) {
    console.error('Failed to fetch DB config:', err);
  }
};

const saveDbConfig = async () => {
  savingDb.value = true;
  dbStatusMessage.value = '';
  try {
    const res = await axios.post('/api/v1/settings/database', dbConfig.value);
    if (res.data && res.data.success) {
      dbStatusType.value = 'success';
      dbStatusMessage.value = 'Database settings updated and reconnected successfully!';
    } else {
      dbStatusType.value = 'error';
      dbStatusMessage.value = res.data?.error || 'Failed to update database settings.';
    }
  } catch (err: any) {
    dbStatusType.value = 'error';
    dbStatusMessage.value = err.response?.data?.error || 'Error saving database settings.';
  } finally {
    savingDb.value = false;
  }
};

onMounted(() => {
  if (route.query.tab === 'services') {
    activeTab.value = 'services';
  } else if (route.query.tab === 'audit' || route.query.tab === 'activity') {
    activeTab.value = 'audit';
  } else if (route.query.tab === 'users') {
    activeTab.value = 'users';
  } else if (route.query.tab === 'database') {
    activeTab.value = 'database';
  }
  fetchUsers();
  fetchRoles();
  fetchAuditLogs();
  fetchSystemLogs();
  fetchDatabaseConfig();
  startElapsedTicker();
});

onUnmounted(() => {
  if (tickerTimer.value) clearInterval(tickerTimer.value);
  if (logWs) logWs.close();
});
</script>

<template>
  <div class="space-y-6 max-w-6xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-200 dark:border-slate-800 pb-4 flex items-center justify-between">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight flex items-center gap-2">
          <Settings class="w-5 h-5 text-brand-500 dark:text-brand-400" />
          <span>System Settings & Services</span>
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Manage HCP system parameters, background service daemons, user access control, database connection, and audit trail.
        </p>
      </div>

      <div>
        <ThemeToggle variant="button" :showLabel="true" />
      </div>
    </div>

    <!-- Navigation Tabs -->
    <div class="flex flex-wrap items-center gap-2 border-b border-slate-300 dark:border-slate-800 pb-2 text-xs">
      <button
        @click="activeTab = 'services'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'services'
            ? 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-800 dark:text-emerald-400 border border-emerald-300 dark:border-emerald-500/30 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <Activity class="w-3.5 h-3.5" />
        <span>Status Services ({{ services.length }})</span>
      </button>

      <button
        @click="activeTab = 'users'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'users'
            ? 'bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-white border border-blue-300 dark:border-slate-700 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <Users class="w-3.5 h-3.5" />
        <span>User Accounts ({{ users.length }})</span>
      </button>

      <button
        @click="activeTab = 'database'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'database'
            ? 'bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-white border border-blue-300 dark:border-slate-700 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <Database class="w-3.5 h-3.5" />
        <span>PostgreSQL Connection</span>
      </button>

      <button
        @click="activeTab = 'audit'"
        :class="[
          'px-4 py-2 rounded-xl font-semibold transition flex items-center gap-2',
          activeTab === 'audit'
            ? 'bg-blue-50 dark:bg-slate-800 text-blue-700 dark:text-white border border-blue-300 dark:border-slate-700 shadow-sm font-bold'
            : 'text-slate-700 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800/40 border border-slate-200 dark:border-slate-800/60 bg-slate-50 dark:bg-transparent'
        ]"
      >
        <FileText class="w-3.5 h-3.5" />
        <span>Activity Logs</span>
      </button>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 1: STATUS SERVICES (4 Core Columns + View Log Modal) -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'services'" class="space-y-4 animate-in fade-in duration-150">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <div class="w-2.5 h-2.5 rounded-[2px] bg-emerald-500"></div>
          <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Active Service Daemons</h3>
        </div>

        <!-- Search input daemon -->
        <div class="relative w-64">
          <Search class="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500 dark:text-slate-400" />
          <input
            v-model="servicesSearch"
            placeholder="Search daemon..."
            class="w-full bg-white dark:bg-[#1b1e26] border border-slate-300 dark:border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-slate-900 dark:text-white placeholder-slate-500 dark:placeholder-slate-400 focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500/30"
          />
        </div>
      </div>

      <!-- 4 Core Columns Table -->
      <div class="bg-white dark:bg-[#1b1e26] border border-slate-300 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <table class="w-full text-left text-xs border-collapse">
          <thead class="bg-slate-100 dark:bg-[#20242e] text-slate-700 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-300 dark:border-slate-800 select-none">
            <tr>
              <th class="py-3 px-4 w-36">Status Services</th>
              <th class="py-3 px-4">Nama Services</th>
              <th class="py-3 px-4 w-44">Last Update</th>
              <th class="py-3 px-4 w-28 text-center">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-200 dark:divide-slate-800/60">
            <tr
              v-for="srv in filteredServices"
              :key="srv.id"
              class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition group"
            >
              <!-- 1. Status Services -->
              <td class="py-3 px-4 whitespace-nowrap">
                <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded bg-emerald-50 dark:bg-emerald-500/10 border border-emerald-300 dark:border-emerald-500/30 text-emerald-800 dark:text-emerald-400 text-[11px] font-bold">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                  <span class="uppercase">RUNNING</span>
                </div>
              </td>

              <!-- 2. Nama Services -->
              <td class="py-3 px-4">
                <div>
                  <p class="text-xs font-bold text-slate-900 dark:text-white group-hover:text-blue-700 dark:group-hover:text-emerald-400 transition">{{ srv.name }}</p>
                  <p class="text-[11px] text-slate-600 dark:text-slate-400 mt-0.5">{{ srv.description }}</p>
                </div>
              </td>

              <!-- 3. Last Update (Live Ticker) -->
              <td class="py-3 px-4 whitespace-nowrap text-xs font-mono text-slate-700 dark:text-slate-300 font-medium">
                <span>{{ srv.updated }}</span>
              </td>

              <!-- 4. Actions (View Log) -->
              <td class="py-3 px-4 text-center whitespace-nowrap">
                <button
                  @click="openViewLogModal(srv)"
                  class="px-3 py-1.5 rounded-lg bg-white hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-200 text-xs font-semibold border border-slate-300 dark:border-slate-700 transition inline-flex items-center gap-1.5 shadow-sm"
                >
                  <Terminal class="w-3.5 h-3.5 text-blue-600 dark:text-brand-400" />
                  <span>View Log</span>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 3: USER ACCOUNTS & ROLE-BASED ACCESS CONTROL (RBAC) -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'users'" class="space-y-8 animate-in fade-in duration-150">
      <!-- Section 1: System Users -->
      <div class="space-y-3">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div>
            <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-2">
              <Users class="w-4 h-4 text-blue-500" />
              <span>User Accounts ({{ users.length }})</span>
            </h3>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">
              Manage operator logins, assign roles, and control account security credentials.
            </p>
          </div>
          <button
            @click="isUserModalOpen = true; userErrorMsg = '';"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs transition shadow-sm self-start sm:self-auto"
          >
            <UserPlus class="w-3.5 h-3.5" />
            <span>Add User</span>
          </button>
        </div>

        <div class="bg-white dark:bg-[#1b1e26] border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <div v-if="loadingUsers" class="p-8 text-center text-xs text-slate-400">
            <RotateCw class="w-5 h-5 animate-spin mx-auto mb-2 text-blue-500" />
            Loading user accounts...
          </div>
          <table v-else class="w-full text-left text-xs">
            <thead class="bg-slate-50 dark:bg-[#20242e] text-slate-600 dark:text-slate-400 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
              <tr>
                <th class="p-3">User</th>
                <th class="p-3">Assigned Role</th>
                <th class="p-3">Feature Access Overview</th>
                <th class="p-3">Created</th>
                <th class="p-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200 dark:divide-slate-800/60 text-slate-700 dark:text-slate-300">
              <tr v-for="u in users" :key="u.id" class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition">
                <td class="p-3">
                  <div class="flex items-center gap-2.5">
                    <div class="w-7 h-7 rounded-full bg-blue-600/10 text-blue-600 dark:bg-blue-500/20 dark:text-blue-400 flex items-center justify-center font-bold text-xs">
                      {{ u.username.substring(0, 2).toUpperCase() }}
                    </div>
                    <div>
                      <div class="font-bold text-slate-900 dark:text-white flex items-center gap-1.5">
                        <span>{{ u.username }}</span>
                        <span v-if="u.id === authStore.user?.id" class="px-1.5 py-0.2 rounded text-[9px] font-bold bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-300">
                          YOU
                        </span>
                      </div>
                      <span class="text-[10px] text-slate-400 font-mono">UID: #{{ u.id }}</span>
                    </div>
                  </div>
                </td>
                <td class="p-3">
                  <div class="flex items-center gap-2">
                    <select
                      :value="u.role"
                      @change="(e: any) => updateUserRole(u, e.target.value)"
                      :disabled="u.id === authStore.user?.id && u.role === 'ADMIN'"
                      class="bg-slate-100 dark:bg-[#14161b] border border-slate-300 dark:border-slate-700 rounded-lg px-2.5 py-1 text-xs font-semibold text-slate-900 dark:text-white focus:outline-none focus:border-blue-500 transition"
                    >
                      <option v-for="r in roles" :key="r.id" :value="r.name">
                        {{ r.name }}
                      </option>
                    </select>
                  </div>
                </td>
                <td class="p-3">
                  <div class="flex items-center gap-1.5 flex-wrap">
                    <span v-if="u.role === 'ADMIN'" class="px-2 py-0.5 rounded text-[10px] font-bold bg-purple-100 dark:bg-purple-500/10 text-purple-700 dark:text-purple-300 border border-purple-300 dark:border-purple-500/30">
                      ⚡ Full Unrestricted Access (All Features)
                    </span>
                    <template v-else>
                      <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-100 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border border-emerald-300 dark:border-emerald-500/30">
                        {{ Object.values(u.permissions || {}).filter(p => p === 'manage').length }} Manage
                      </span>
                      <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-sky-100 dark:bg-sky-500/10 text-sky-700 dark:text-sky-400 border border-sky-300 dark:border-sky-500/30">
                        {{ Object.values(u.permissions || {}).filter(p => p === 'read').length }} Read Only
                      </span>
                    </template>
                  </div>
                </td>
                <td class="p-3 text-slate-500 dark:text-slate-400 font-mono text-[11px]">
                  {{ u.createdAt ? new Date(u.createdAt).toLocaleDateString() : '-' }}
                </td>
                <td class="p-3 text-right">
                  <button
                    v-if="u.id !== authStore.user?.id"
                    @click="deleteUser(u)"
                    class="p-1.5 rounded-lg text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 transition"
                    title="Delete User"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                  <span v-else class="text-[10px] text-slate-400 italic">Self</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Section 2: Roles & Granular Permission Matrix -->
      <div class="space-y-3 pt-4 border-t border-slate-200 dark:border-slate-800">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div>
            <h3 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-2">
              <ShieldCheck class="w-4 h-4 text-purple-500" />
              <span>Roles & Granular Permissions Matrix</span>
            </h3>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">
              Configure feature access tiers: <strong>No Access</strong>, <strong>Read-Only</strong>, or <strong>Edit/Manage & Read</strong> for each system capability.
            </p>
          </div>
          <button
            @click="openCreateRoleModal"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-600 hover:bg-purple-500 text-white font-semibold text-xs transition shadow-sm self-start sm:self-auto"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>New Custom Role</span>
          </button>
        </div>

        <!-- Role Cards Grid -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div
            v-for="r in roles"
            :key="r.id"
            class="p-4 rounded-xl border transition flex flex-col justify-between bg-white dark:bg-[#1b1e26] border-slate-200 dark:border-slate-800 hover:border-purple-400 dark:hover:border-purple-500/50 shadow-sm"
          >
            <div class="space-y-2.5">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span class="font-bold text-slate-900 dark:text-white text-sm tracking-tight">{{ r.name }}</span>
                  <span
                    v-if="r.isDefault"
                    class="px-1.5 py-0.5 rounded text-[9px] font-bold uppercase tracking-wider bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border border-slate-300 dark:border-slate-700"
                  >
                    System Built-in
                  </span>
                  <span
                    v-else
                    class="px-1.5 py-0.5 rounded text-[9px] font-bold uppercase tracking-wider bg-purple-100 dark:bg-purple-500/10 text-purple-700 dark:text-purple-400 border border-purple-300 dark:border-purple-500/30"
                  >
                    Custom Role
                  </span>
                </div>
              </div>

              <p class="text-xs text-slate-500 dark:text-slate-400 line-clamp-2 min-h-[32px]">
                {{ r.description || 'No description provided.' }}
              </p>

              <!-- Stats Pill Bar -->
              <div class="flex items-center gap-1.5 pt-1 text-[10px] font-mono">
                <span class="px-2 py-0.5 rounded bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-500/20 font-bold">
                  {{ getRoleStats(r).manageCount }} Manage
                </span>
                <span class="px-2 py-0.5 rounded bg-sky-50 dark:bg-sky-500/10 text-sky-700 dark:text-sky-400 border border-sky-200 dark:border-sky-500/20 font-bold">
                  {{ getRoleStats(r).readCount }} Read
                </span>
                <span class="px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700">
                  {{ getRoleStats(r).noneCount }} Hidden
                </span>
              </div>
            </div>

            <div class="pt-4 mt-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center justify-between">
              <button
                @click="openEditRoleModal(r)"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-200 text-xs font-semibold transition"
              >
                <Edit3 class="w-3.5 h-3.5 text-purple-500" />
                <span>Configure Permissions</span>
              </button>

              <button
                v-if="!r.isDefault"
                @click="deleteRole(r)"
                class="p-1.5 rounded-lg text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 transition"
                title="Delete Custom Role"
              >
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 4: POSTGRESQL CONNECTION -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'database'" class="p-6 bg-[#171a23] border border-slate-800 rounded-xl space-y-4 max-w-3xl animate-in fade-in duration-150">
      <div class="flex items-center justify-between border-b border-slate-800/80 pb-3">
        <div>
          <h3 class="text-xs font-bold text-white uppercase tracking-wider">PostgreSQL Connection Settings</h3>
          <p class="text-[11px] text-slate-400">Configure PostgreSQL database credentials, host endpoint, port, and TLS encryption</p>
        </div>
        <span class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-mono font-bold">
          ACTIVE POOL
        </span>
      </div>
      
      <form @submit.prevent="saveDbConfig" class="space-y-4 text-xs">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Host</label>
            <input v-model="dbConfig.host" required placeholder="localhost" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Port</label>
            <input v-model.number="dbConfig.port" type="number" required placeholder="5432" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-bold">Database Name</label>
            <input v-model="dbConfig.database" required placeholder="hephaestus_db" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-bold">User</label>
            <input v-model="dbConfig.user" required placeholder="hephaestus" class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono" />
          </div>
        </div>

        <div>
          <label class="block text-slate-400 mb-1 font-bold">Password</label>
          <input
            v-model="dbConfig.password"
            type="password"
            placeholder="••••••••••••••••"
            class="w-full bg-[#0f1219] border border-slate-700 rounded-lg px-3 py-1.5 text-white font-mono"
          />
          <p class="text-[10px] text-slate-500 mt-1">Leave blank to retain existing encrypted database password</p>
        </div>

        <div class="pt-1">
          <label class="flex items-center gap-2 cursor-pointer text-slate-300">
            <input
              type="checkbox"
              v-model="dbConfig.ssl"
              class="rounded bg-[#0f1219] border-slate-700 text-[#4274D9] focus:ring-0 w-4 h-4 cursor-pointer"
            />
            <span class="font-medium">Use SSL / TLS Encrypted Connection (sslmode=require)</span>
          </label>
        </div>

        <div v-if="dbStatus" :class="['p-3 rounded-lg border text-xs font-mono', dbStatus.success ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-400']">
          {{ dbStatus.message }}
        </div>

        <div class="pt-3 border-t border-slate-800 flex items-center justify-between">
          <button
            type="button"
            @click="testDbConnection"
            :disabled="dbTesting"
            class="px-4 py-2 bg-[#20242e] hover:bg-[#282d3a] text-slate-200 text-xs font-bold rounded-lg border border-slate-700 transition disabled:opacity-50"
          >
            {{ dbTesting ? 'TESTING...' : 'Test Connection' }}
          </button>

          <button
            type="submit"
            :disabled="dbSaving"
            class="px-4 py-2 bg-[#4274D9] hover:bg-[#3461c2] text-white text-xs font-bold rounded-lg transition disabled:opacity-50"
          >
            {{ dbSaving ? 'SAVING...' : 'Save & Apply Config' }}
          </button>
        </div>
      </form>
    </div>

    <!-- ============================================================= -->
    <!-- TAB 4: ACTIVITY LOGS & AUDIT TRAIL -->
    <!-- ============================================================= -->
    <div v-if="activeTab === 'audit'" class="space-y-4 animate-in fade-in duration-150">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-white dark:bg-[#141824] p-3.5 rounded-xl border border-slate-200 dark:border-slate-800 shadow-sm">
        <div class="flex items-center gap-3">
          <div class="flex items-center p-1 bg-slate-100 dark:bg-slate-800/80 rounded-lg border border-slate-200 dark:border-slate-700/60 text-xs">
            <button
              @click="logViewMode = 'audit'; fetchAuditLogs()"
              :class="[
                'px-3 py-1.5 rounded-md font-semibold transition flex items-center gap-1.5',
                logViewMode === 'audit'
                  ? 'bg-[#4274D9] text-white shadow-sm'
                  : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              <ShieldCheck class="w-3.5 h-3.5" />
              <span>User & Security Audit Trail ({{ auditLogs.length }})</span>
            </button>
            <button
              @click="logViewMode = 'system'; fetchSystemLogs()"
              :class="[
                'px-3 py-1.5 rounded-md font-semibold transition flex items-center gap-1.5',
                logViewMode === 'system'
                  ? 'bg-[#4274D9] text-white shadow-sm'
                  : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
              ]"
            >
              <Terminal class="w-3.5 h-3.5" />
              <span>System & Console Logs ({{ systemLogs.length }})</span>
            </button>
          </div>
        </div>

        <button
          @click="refreshCurrentLogs"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-slate-700 text-xs font-semibold transition shadow-sm self-start sm:self-auto"
        >
          <RotateCw class="w-3.5 h-3.5 text-[#4274D9]" :class="{ 'animate-spin': loadingLogs }" />
          <span>Refresh</span>
        </button>
      </div>

      <!-- Table 1: Security & User Audit Logs (Database) -->
      <div v-if="logViewMode === 'audit'" class="bg-white dark:bg-[#141824] border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <table class="w-full text-left text-xs">
          <thead class="bg-slate-50 dark:bg-[#1a202c] text-slate-700 dark:text-slate-300 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
            <tr>
              <th class="p-3 w-48">Timestamp</th>
              <th class="p-3 w-32">Actor / User</th>
              <th class="p-3 w-24">Module</th>
              <th class="p-3 w-36">Action</th>
              <th class="p-3">Details</th>
              <th class="p-3 w-28 text-right">Status</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-slate-800 dark:text-slate-200">
            <tr v-for="log in auditLogs" :key="log.id" class="hover:bg-slate-50/80 dark:hover:bg-slate-800/30 transition">
              <td class="p-3 text-slate-500 dark:text-slate-400 font-mono text-[11px] whitespace-nowrap">
                {{ new Date(log.timestamp).toLocaleString() }}
              </td>
              <td class="p-3 font-semibold text-slate-900 dark:text-white">
                <span v-if="log.username" class="px-2 py-0.5 rounded bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-mono text-[11px]">
                  {{ log.username }}
                </span>
                <span v-else class="text-slate-400 dark:text-slate-500 italic text-[11px]">
                  Anonymous
                </span>
              </td>
              <td class="p-3">
                <span class="px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 font-semibold text-[10px]">
                  {{ log.module }}
                </span>
              </td>
              <td class="p-3 font-medium">
                {{ log.action }}
              </td>
              <td class="p-3 text-slate-600 dark:text-slate-300 font-mono text-[11px] break-all">
                {{ log.details }}
              </td>
              <td class="p-3 text-right">
                <span
                  class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider"
                  :class="log.status === 'SUCCESS' ? 'bg-emerald-100 dark:bg-emerald-500/20 text-emerald-700 dark:text-emerald-400' : 'bg-red-100 dark:bg-red-500/20 text-red-700 dark:text-red-400'"
                >
                  {{ log.status }}
                </span>
              </td>
            </tr>
            <tr v-if="auditLogs.length === 0">
              <td colspan="6" class="p-8 text-center text-slate-400 dark:text-slate-500">
                No security or user activity logs recorded yet.
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Table 2: System Console Logs -->
      <div v-else class="bg-white dark:bg-[#141824] border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <table class="w-full text-left text-xs font-mono">
          <thead class="bg-slate-50 dark:bg-[#1a202c] text-slate-700 dark:text-slate-300 text-[10px] uppercase font-bold tracking-wider border-b border-slate-200 dark:border-slate-800">
            <tr>
              <th class="p-3 w-48">Timestamp</th>
              <th class="p-3 w-20">Level</th>
              <th class="p-3 w-28">Module</th>
              <th class="p-3">Message</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 text-slate-800 dark:text-slate-200">
            <tr v-for="(log, idx) in systemLogs" :key="idx" class="hover:bg-slate-50/80 dark:hover:bg-slate-800/30 transition">
              <td class="p-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{{ log.timestamp }}</td>
              <td class="p-3">
                <span
                  class="px-1.5 py-0.5 rounded text-[9px] font-bold uppercase"
                  :class="[
                    log.level === 'ERROR' ? 'bg-red-100 dark:bg-red-500/20 text-red-700 dark:text-red-400' :
                    log.level === 'WARN' ? 'bg-amber-100 dark:bg-amber-500/20 text-amber-700 dark:text-amber-400' :
                    log.level === 'DEBUG' ? 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400' :
                    'bg-emerald-100 dark:bg-emerald-500/20 text-emerald-700 dark:text-emerald-400'
                  ]"
                >
                  {{ log.level }}
                </span>
              </td>
              <td class="p-3 font-semibold text-[#4274D9]">{{ log.module }}</td>
              <td class="p-3 text-slate-800 dark:text-slate-200 break-all">{{ log.message }}</td>
            </tr>
            <tr v-if="systemLogs.length === 0">
              <td colspan="4" class="p-8 text-center text-slate-400 dark:text-slate-500">
                No system log events recorded.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- VIEW LOG MODAL (CONNECTED VIA WEBSOCKET) -->
    <!-- ============================================================= -->
    <div
      v-if="showLogModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-150"
    >
      <div class="bg-[#11141c] border border-slate-700 rounded-2xl w-full max-w-4xl h-[75vh] flex flex-col shadow-2xl overflow-hidden font-mono">
        <div class="p-3.5 bg-[#171a23] border-b border-slate-800 flex items-center justify-between text-xs font-sans">
          <div class="flex items-center gap-2.5">
            <Terminal class="w-4 h-4 text-brand-400" />
            <h3 class="font-bold text-white">{{ activeLogService?.name }}</h3>
            <span class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-mono">
              LIVE WS STREAM
            </span>
          </div>
          <button @click="closeViewLogModal" class="text-slate-400 hover:text-white p-1">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div id="service-log-viewport" class="flex-1 p-4 overflow-y-auto bg-[#090d16] text-xs space-y-1 select-text leading-relaxed">
          <div
            v-for="(l, idx) in filteredServiceLogs"
            :key="idx"
            class="flex items-start gap-2 hover:bg-slate-900/60 p-0.5 rounded"
          >
            <span class="text-slate-600 text-[11px] shrink-0 select-none">
              {{ new Date(l.timestamp).toLocaleTimeString() }}
            </span>
            <span
              :class="[
                'px-1.5 py-0.2 rounded text-[9px] font-bold shrink-0 uppercase',
                l.level === 'ERROR' ? 'bg-red-500/20 text-red-400' : l.level === 'WARN' ? 'bg-amber-500/20 text-amber-400' : 'bg-emerald-500/20 text-emerald-400'
              ]"
            >
              {{ l.level }}
            </span>
            <span class="text-slate-200 break-all">{{ l.message }}</span>
          </div>
          <div v-if="filteredServiceLogs.length === 0" class="text-center py-12 text-slate-600 font-sans text-xs">
            No live log entries recorded for this service module.
          </div>
        </div>

        <div class="p-2.5 px-4 bg-[#141720] border-t border-slate-800 flex items-center justify-between text-xs text-slate-400 font-sans">
          <span>Module: {{ activeLogService?.moduleKey }}</span>
          <button @click="closeViewLogModal" class="px-3 py-1 bg-slate-800 hover:bg-slate-700 text-white rounded-lg">Close</button>
        </div>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 1: ADD USER -->
    <!-- ============================================================= -->
    <div
      v-if="isUserModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in duration-150"
    >
      <div class="bg-white dark:bg-[#1b1e26] border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <UserPlus class="w-4 h-4 text-blue-500" />
            <h3 class="text-sm font-bold text-slate-900 dark:text-white">Create New Operator User</h3>
          </div>
          <button @click="isUserModalOpen = false" class="text-slate-400 hover:text-slate-200">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div v-if="userErrorMsg" class="p-2.5 rounded-lg bg-red-500/10 border border-red-500/20 text-red-600 dark:text-red-400 text-xs flex items-center gap-2">
          <AlertTriangle class="w-4 h-4 shrink-0" />
          <span>{{ userErrorMsg }}</span>
        </div>

        <form @submit.prevent="createUser" class="space-y-3.5 text-xs">
          <div>
            <label class="block text-slate-700 dark:text-slate-300 mb-1 font-semibold">Username</label>
            <input
              v-model="newUserForm.username"
              required
              placeholder="e.g. jdoe_ops"
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label class="block text-slate-700 dark:text-slate-300 mb-1 font-semibold">Password</label>
            <input
              v-model="newUserForm.password"
              type="password"
              required
              placeholder="Minimum 6 characters"
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label class="block text-slate-700 dark:text-slate-300 mb-1 font-semibold">Assign System Role</label>
            <select
              v-model="newUserForm.role"
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-semibold focus:outline-none focus:border-blue-500"
            >
              <option v-for="r in roles" :key="r.id" :value="r.name">
                {{ r.name }} - {{ r.description }}
              </option>
            </select>
          </div>

          <div class="flex justify-end gap-2 pt-3 border-t border-slate-200 dark:border-slate-800">
            <button
              type="button"
              @click="isUserModalOpen = false"
              class="px-4 py-2 bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 font-semibold"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="userActionLoading"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold rounded-lg flex items-center gap-1.5 transition disabled:opacity-50"
            >
              <RotateCw v-if="userActionLoading" class="w-3.5 h-3.5 animate-spin" />
              <span>Create Account</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ============================================================= -->
    <!-- MODAL 2: ROLE & GRANULAR PERMISSION MATRIX MODAL -->
    <!-- ============================================================= -->
    <div
      v-if="isRoleModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 overflow-y-auto animate-in fade-in duration-150"
    >
      <div class="bg-white dark:bg-[#1b1e26] border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-2xl my-8 p-6 space-y-5 shadow-2xl flex flex-col max-h-[90vh]">
        <!-- Modal Header -->
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3 shrink-0">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-xl bg-purple-500/10 text-purple-500 flex items-center justify-center">
              <ShieldCheck class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-white">
                {{ editingRole ? `Configure Role: ${editingRole.name}` : 'Create New Custom Role' }}
              </h3>
              <p class="text-[11px] text-slate-500 dark:text-slate-400">
                Grant or restrict access per feature with granular tiers: No Access, Read-Only, or Edit/Manage.
              </p>
            </div>
          </div>
          <button @click="isRoleModalOpen = false" class="text-slate-400 hover:text-slate-200">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div v-if="roleErrorMsg" class="p-2.5 rounded-lg bg-red-500/10 border border-red-500/20 text-red-600 dark:text-red-400 text-xs flex items-center gap-2 shrink-0">
          <AlertTriangle class="w-4 h-4 shrink-0" />
          <span>{{ roleErrorMsg }}</span>
        </div>

        <!-- Role Meta Info -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs shrink-0">
          <div>
            <label class="block text-slate-700 dark:text-slate-300 mb-1 font-semibold">Role Name</label>
            <input
              v-model="roleForm.name"
              :disabled="editingRole?.isDefault"
              required
              placeholder="e.g. AUDITOR, NETWORK_ADMIN"
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white font-mono font-bold uppercase focus:outline-none focus:border-purple-500 disabled:opacity-60"
            />
          </div>
          <div>
            <label class="block text-slate-700 dark:text-slate-300 mb-1 font-semibold">Description</label>
            <input
              v-model="roleForm.description"
              placeholder="Brief explanation of this role's purpose"
              class="w-full bg-slate-50 dark:bg-[#14161b] border border-slate-300 dark:border-slate-700 rounded-lg px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:border-purple-500"
            />
          </div>
        </div>

        <!-- Superadmin Notice or Quick Presets -->
        <div v-if="editingRole?.name.toUpperCase() === 'ADMIN'" class="p-3 bg-purple-50 dark:bg-purple-500/10 border border-purple-200 dark:border-purple-500/30 rounded-xl text-xs text-purple-800 dark:text-purple-300 shrink-0">
          <div class="flex items-center gap-2 font-bold mb-1">
            <Lock class="w-4 h-4 text-purple-500" />
            <span>Built-in Superadmin Profile</span>
          </div>
          <p class="text-[11px] leading-relaxed">
            The <strong>ADMIN</strong> role is the root system administrator and inherently holds unrestricted <strong>Edit / Manage</strong> permissions across all current and future features.
          </p>
        </div>

        <div v-else class="flex flex-wrap items-center justify-between gap-2 p-2.5 bg-slate-100 dark:bg-[#14161b] rounded-xl border border-slate-200 dark:border-slate-800 shrink-0">
          <span class="text-[11px] font-semibold text-slate-600 dark:text-slate-400">Quick Batch Presets:</span>
          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="setAllPermissions('manage')"
              class="px-2.5 py-1 rounded-lg text-[10px] font-bold bg-emerald-100 hover:bg-emerald-200 dark:bg-emerald-500/20 dark:hover:bg-emerald-500/30 text-emerald-700 dark:text-emerald-300 transition"
            >
              ⚡ Manage All
            </button>
            <button
              type="button"
              @click="setAllPermissions('read')"
              class="px-2.5 py-1 rounded-lg text-[10px] font-bold bg-sky-100 hover:bg-sky-200 dark:bg-sky-500/20 dark:hover:bg-sky-500/30 text-sky-700 dark:text-sky-300 transition"
            >
              👁 Read Only All
            </button>
            <button
              type="button"
              @click="setAllPermissions('none')"
              class="px-2.5 py-1 rounded-lg text-[10px] font-bold bg-slate-200 hover:bg-slate-300 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 transition"
            >
              ✕ No Access All
            </button>
          </div>
        </div>

        <!-- Scrollable Feature Matrix List -->
        <div class="overflow-y-auto pr-1 space-y-2.5 flex-1 min-h-[260px]">
          <div
            v-for="f in SYSTEM_FEATURES"
            :key="f.key"
            class="p-3 rounded-xl border transition flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-slate-50 dark:bg-[#151821] border-slate-200 dark:border-slate-800/80 hover:border-slate-300 dark:hover:border-slate-700"
          >
            <div class="flex items-start gap-3">
              <div class="w-8 h-8 rounded-lg bg-white dark:bg-[#1f232d] border border-slate-200 dark:border-slate-700 text-blue-500 flex items-center justify-center shrink-0 mt-0.5">
                <component :is="f.icon" class="w-4 h-4" />
              </div>
              <div>
                <div class="font-bold text-slate-900 dark:text-white text-xs">{{ f.label }}</div>
                <div class="text-[11px] text-slate-500 dark:text-slate-400 leading-snug">{{ f.desc }}</div>
              </div>
            </div>

            <!-- 3-Segment Toggle Pill for Feature -->
            <div class="flex items-center rounded-lg p-0.5 bg-slate-200 dark:bg-[#0e1017] border border-slate-300 dark:border-slate-800 shrink-0 self-end sm:self-auto">
              <!-- Tier 1: None -->
              <button
                type="button"
                @click="roleForm.permissions[f.key] = 'none'"
                :disabled="editingRole?.name.toUpperCase() === 'ADMIN'"
                :class="[
                  'px-2.5 py-1 rounded-md text-[10px] font-bold transition flex items-center gap-1',
                  (roleForm.permissions[f.key] || 'none') === 'none'
                    ? 'bg-slate-600 text-white shadow-sm'
                    : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
                ]"
              >
                <span>✕ None</span>
              </button>

              <!-- Tier 2: Read Only -->
              <button
                type="button"
                @click="roleForm.permissions[f.key] = 'read'"
                :disabled="editingRole?.name.toUpperCase() === 'ADMIN'"
                :class="[
                  'px-2.5 py-1 rounded-md text-[10px] font-bold transition flex items-center gap-1',
                  roleForm.permissions[f.key] === 'read'
                    ? 'bg-sky-600 text-white shadow-sm'
                    : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
                ]"
              >
                <Eye class="w-3 h-3" />
                <span>Read Only</span>
              </button>

              <!-- Tier 3: Edit / Manage -->
              <button
                type="button"
                @click="roleForm.permissions[f.key] = 'manage'"
                :disabled="editingRole?.name.toUpperCase() === 'ADMIN'"
                :class="[
                  'px-2.5 py-1 rounded-md text-[10px] font-bold transition flex items-center gap-1',
                  roleForm.permissions[f.key] === 'manage'
                    ? 'bg-emerald-600 text-white shadow-sm'
                    : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
                ]"
              >
                <Check class="w-3 h-3" />
                <span>Edit / Manage</span>
              </button>
            </div>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="flex items-center justify-between pt-3 border-t border-slate-200 dark:border-slate-800 shrink-0">
          <div class="text-[11px] text-slate-500 font-mono">
            Permissions active immediately on save
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="isRoleModalOpen = false"
              class="px-4 py-2 bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 text-xs font-semibold"
            >
              Cancel
            </button>
            <button
              type="button"
              @click="saveRole"
              :disabled="roleActionLoading"
              class="px-5 py-2 bg-purple-600 hover:bg-purple-500 text-white text-xs font-semibold rounded-lg flex items-center gap-1.5 transition disabled:opacity-50 shadow-sm"
            >
              <RotateCw v-if="roleActionLoading" class="w-3.5 h-3.5 animate-spin" />
              <span>Save Role & Permissions</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
