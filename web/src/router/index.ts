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
      meta: { requiresAuth: true },
    },
    {
      path: '/remote-host',
      name: 'remote-host',
      component: () => import('../views/RemoteHostView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/remote-server',
      name: 'remote-server',
      component: () => import('../views/RemoteHostView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/network-topology',
      name: 'network-topology',
      component: () => import('../views/TopologyView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/security/vaultwarden',
      name: 'vaultwarden',
      component: () => import('../views/VaultwardenView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/vaultwarden',
      redirect: '/security/vaultwarden',
    },
    {
      path: '/infrastructure/containers',
      name: 'container-management',
      component: () => import('../views/ContainerManagementView.vue'),
      meta: { requiresAuth: true },
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
        },
        {
          path: 'connections',
          name: 'connections',
          component: () => import('../views/ConnectionsView.vue'),
        },
        {
          path: 'inventory-server',
          name: 'inventory-server',
          component: () => import('../views/VmInventoryView.vue'),
        },
        {
          path: 'inventory',
          redirect: '/inventory-server',
        },
        {
          path: 'prometheus-config',
          name: 'prometheus-config',
          component: () => import('../views/PrometheusConfigView.vue'),
        },
        {
          path: 'dataprepper-config',
          name: 'dataprepper-config',
          component: () => import('../views/DataPrepperConfigView.vue'),
        },
        {
          path: 'opentelemetry-config',
          name: 'opentelemetry-config',
          component: () => import('../views/OpenTelemetryConfigView.vue'),
        },
        {
          path: 'otel-config',
          redirect: '/opentelemetry-config',
        },
        {
          path: 'backup',
          name: 'backup',
          component: () => import('../views/BackupView.vue'),
        },
        {
          path: 'snmp',
          name: 'snmp',
          component: () => import('../views/SnmpView.vue'),
        },
        {
          path: 'grok-debugger',
          name: 'grok-debugger',
          component: () => import('../views/GrokDebuggerView.vue'),
        },
        {
          path: 'slideshow',
          name: 'slideshow',
          component: () => import('../views/SlideShowView.vue'),
        },
        {
          path: 'kiosk/:id?',
          name: 'kiosk',
          component: () => import('../views/SlideShowView.vue'),
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
        },
        {
          path: 'reports/visual',
          redirect: '/reports',
        },
        {
          path: 'reports/raw',
          name: 'raw-reports',
          component: () => import('../views/RawReportsView.vue'),
        },
        {
          path: 'report',
          redirect: '/reports',
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('../views/SettingsView.vue'),
        },
        {
          path: 'status-pages',
          name: 'status-pages',
          component: () => import('../views/StatusPagesView.vue'),
        },
        {
          path: 'status-page',
          redirect: '/status-pages',
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
  }

  if (to.meta.guestOnly && authStore.isAuthenticated) {
    return next({ name: 'dashboard' });
  }

  next();
});

export default router;
