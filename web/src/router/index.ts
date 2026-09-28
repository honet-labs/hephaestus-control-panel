import { createRouter, createWebHistory } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { guestOnly: true },
    },
    {
      path: '/setup',
      name: 'setup',
      component: () => import('../views/SetupView.vue'),
      meta: { guestOnly: true },
    },
    {
      path: '/opensearch-cluster',
      name: 'opensearch-cluster',
      component: () => import('../views/OpenSearchClusterView.vue'),
      meta: { requiresAuth: true, feature: 'opensearch' },
    },
    {
      path: '/remote-host',
      name: 'remote-host',
      component: () => import('../views/RemoteHostView.vue'),
      meta: { requiresAuth: true, feature: 'remote_servers' },
    },
    {
      path: '/remote-server',
      name: 'remote-server',
      component: () => import('../views/RemoteHostView.vue'),
      meta: { requiresAuth: true, feature: 'remote_servers' },
    },
    {
      path: '/network-topology',
      name: 'network-topology',
      component: () => import('../views/TopologyView.vue'),
      meta: { requiresAuth: true, feature: 'network_topology' },
    },
    {
      path: '/security/vaultwarden',
      name: 'vaultwarden',
      component: () => import('../views/VaultwardenView.vue'),
      meta: { requiresAuth: true, feature: 'security' },
    },
    {
      path: '/vaultwarden',
      redirect: '/security/vaultwarden',
    },
    {
      path: '/infrastructure/containers',
      name: 'container-management',
      component: () => import('../views/ContainerManagementView.vue'),
      meta: { requiresAuth: true, feature: 'infrastructure' },
    },
    {
      path: '/containers',
      redirect: '/infrastructure/containers',
    },
    {
      path: '/management-containers',
      redirect: '/infrastructure/containers',
    },
    {
      path: '/status/:slug',
      name: 'public-status-page',
      component: () => import('../views/PublicStatusPageView.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      component: () => import('../layouts/AppLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'dashboard',
          component: () => import('../views/DashboardView.vue'),
          meta: { requiresAuth: true, feature: 'dashboard' },
        },
        {
          path: 'connections',
          name: 'connections',
          component: () => import('../views/ConnectionsView.vue'),
          meta: { requiresAuth: true, feature: 'connections' },
        },
        {
          path: 'inventory-server',
          name: 'inventory-server',
          component: () => import('../views/VmInventoryView.vue'),
          meta: { requiresAuth: true, feature: 'connections' },
        },
        {
          path: 'inventory',
          redirect: '/inventory-server',
        },
        {
          path: 'prometheus-config',
          name: 'prometheus-config',
          component: () => import('../views/PrometheusConfigView.vue'),
          meta: { requiresAuth: true, feature: 'prometheus_config' },
        },
        {
          path: 'dataprepper-config',
          name: 'dataprepper-config',
          component: () => import('../views/DataPrepperConfigView.vue'),
          meta: { requiresAuth: true, feature: 'dataprepper_config' },
        },
        {
          path: 'opentelemetry-config',
          name: 'opentelemetry-config',
          component: () => import('../views/OpenTelemetryConfigView.vue'),
          meta: { requiresAuth: true, feature: 'opentelemetry_config' },
        },
        {
          path: 'otel-config',
          redirect: '/opentelemetry-config',
        },
        {
          path: 'backup',
          name: 'backup',
          component: () => import('../views/BackupView.vue'),
          meta: { requiresAuth: true, feature: 'backup' },
        },
        {
          path: 'snmp',
          name: 'snmp',
          component: () => import('../views/SnmpView.vue'),
          meta: { requiresAuth: true, feature: 'snmp' },
        },
        {
          path: 'grok-debugger',
          name: 'grok-debugger',
          component: () => import('../views/GrokDebuggerView.vue'),
          meta: { requiresAuth: true, feature: 'grok_debugger' },
        },
        {
          path: 'slideshow',
          name: 'slideshow',
          component: () => import('../views/SlideShowView.vue'),
          meta: { requiresAuth: true, feature: 'slideshow' },
        },
        {
          path: 'kiosk/:id?',
          name: 'kiosk',
          component: () => import('../views/SlideShowView.vue'),
          meta: { requiresAuth: true, feature: 'slideshow' },
        },
        {
          path: 'tools',
          redirect: '/snmp',
        },
        {
          path: 'queue',
          redirect: '/settings?tab=services',
        },
        {
          path: 'reports',
          name: 'reports',
          component: () => import('../views/ReportsView.vue'),
          meta: { requiresAuth: true, feature: 'reports' },
        },
        {
          path: 'reports/visual',
          redirect: '/reports',
        },
        {
          path: 'reports/raw',
          name: 'raw-reports',
          component: () => import('../views/RawReportsView.vue'),
          meta: { requiresAuth: true, feature: 'reports' },
        },
        {
          path: 'report',
          redirect: '/reports',
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('../views/SettingsView.vue'),
          meta: { requiresAuth: true, feature: 'settings' },
        },
        {
          path: 'status-pages',
          name: 'status-pages',
          component: () => import('../views/StatusPagesView.vue'),
          meta: { requiresAuth: true, feature: 'status_pages' },
        },
        {
          path: 'status-page',
          redirect: '/status-pages',
        },
        {
          path: 'monitoring/instances',
          name: 'monitoring-instances',
          component: () => import('../views/MonitoringInstancesView.vue'),
          meta: { requiresAuth: true, feature: 'monitoring_instances' },
        },
        {
          path: 'monitoring-instances',
          redirect: '/monitoring/instances',
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../views/NotFoundView.vue'),
      meta: { requiresAuth: false },
    },
  ],
});

// Request Interceptor: Attach Authorization header only if in-memory token is present (H-02)
axios.interceptors.request.use((config) => {
  const authStore = useAuthStore();
  if (authStore.token && !config.headers.Authorization) {
    config.headers.Authorization = `Bearer ${authStore.token}`;
  }
  return config;
});

// Response Interceptor: Automatically redirect to login on 401 Unauthorized globally
axios.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 401) {
      const url = error.config?.url || '';
      // Exclude auth initialization and setup endpoints from calling router.push here,
      // because router.beforeEach is already awaiting fetchUser() and will handle the transition cleanly.
      const isAuthUrl =
        url.includes('/api/v1/auth/login') ||
        url.includes('/api/v1/setup') ||
        url.includes('/api/v1/auth/me');

      if (!isAuthUrl) {
        const authStore = useAuthStore();
        authStore.clearAuth();
        if (router.currentRoute.value.name !== 'login') {
          router
            .push({
              name: 'login',
              query: { redirect: router.currentRoute.value.fullPath },
            })
            .catch(() => {});
        }
      }
    }
    return Promise.reject(error);
  }
);

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore();

  // Initialize session once on initial page load / refresh via HttpOnly cookie (H-02)
  if (!authStore.isInitialized) {
    try {
      await authStore.fetchUser();
    } catch (_) {
      authStore.clearAuth();
    }
  }

  // Support auth token passed in query parameter for shared links / embed views
  if (to.query.token && typeof to.query.token === 'string') {
    const qToken = to.query.token;
    authStore.token = qToken;
    authStore.isAuthenticated = true;
    axios.defaults.headers.common['Authorization'] = `Bearer ${qToken}`;
    if (!authStore.user) {
      await authStore.fetchUser();
    }
  }

  if (to.meta.requiresAuth) {
    if (!authStore.isAuthenticated) {
      return next({ name: 'login', query: { redirect: to.fullPath } });
    }

    // RBAC Feature Permission Enforcement
    if (to.meta.feature && typeof to.meta.feature === 'string') {
      const action = (to.meta.action as 'read' | 'manage') || 'read';
      if (!authStore.can(to.meta.feature, action)) {
        // User lacks permission for this route: avoid blank screen and redirect safely
        if (to.meta.feature !== 'dashboard' && authStore.can('dashboard', 'read')) {
          return next({ path: '/' });
        }
        const accessibleRoutes = [
          { path: '/connections', feature: 'connections' },
          { path: '/infrastructure/containers', feature: 'infrastructure' },
          { path: '/remote-server', feature: 'remote_servers' },
          { path: '/network-topology', feature: 'network_topology' },
          { path: '/status-pages', feature: 'status_pages' },
          { path: '/slideshow', feature: 'slideshow' },
          { path: '/snmp', feature: 'snmp' },
          { path: '/reports', feature: 'reports' },
        ];
        const fallback = accessibleRoutes.find((r) => authStore.can(r.feature, 'read'));
        if (fallback && to.path !== fallback.path) {
          return next({ path: fallback.path });
        }
        return next({ name: 'not-found' });
      }
    }
  }

  if (to.meta.guestOnly && authStore.isAuthenticated) {
    return next({ name: 'dashboard' });
  }

  next();
});

export default router;
