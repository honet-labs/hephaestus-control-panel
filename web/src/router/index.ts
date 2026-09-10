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
          path: 'settings',
          name: 'settings',
          component: () => import('../views/SettingsView.vue'),
        },
      ],
    },
  ],
});

// Request Interceptor: Ensure Authorization header is present if token exists
axios.interceptors.request.use((config) => {
  const token = localStorage.getItem('hephaestus_token');
  if (token && !config.headers.Authorization) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response Interceptor: Automatically redirect to login on 401 Unauthorized globally
axios.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 401) {
      const url = error.config?.url || '';
      const isAuthUrl = url.includes('/api/v1/auth/login') || url.includes('/api/v1/setup');
      if (!isAuthUrl) {
        const authStore = useAuthStore();
        authStore.clearAuth();
        if (router.currentRoute.value.name !== 'login') {
          router.push({
            name: 'login',
            query: { redirect: router.currentRoute.value.fullPath },
          });
        }
      }
    }
    return Promise.reject(error);
  }
);

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore();

  // Support auth token passed in query parameter for shared links / embed views
  if (to.query.token && typeof to.query.token === 'string') {
    const qToken = to.query.token;
    authStore.token = qToken;
    authStore.isAuthenticated = true;
    localStorage.setItem('hephaestus_token', qToken);
    axios.defaults.headers.common['Authorization'] = `Bearer ${qToken}`;
    if (!authStore.user) {
      await authStore.fetchUser();
    }
  }

  if (to.meta.requiresAuth) {
    if (!authStore.isAuthenticated) {
      return next({ name: 'login', query: { redirect: to.fullPath } });
    }
    // If authenticated token exists but user object is not yet loaded, load it before rendering
    if (!authStore.user) {
      const u = await authStore.fetchUser();
      if (!u && !authStore.isAuthenticated) {
        return next({ name: 'login', query: { redirect: to.fullPath } });
      }
    }
  }

  if (to.meta.guestOnly && authStore.isAuthenticated) {
    return next({ name: 'dashboard' });
  }

  next();
});

export default router;
