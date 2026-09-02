import { createRouter, createWebHistory } from 'vue-router';
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

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore();

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return next({ name: 'login' });
  }
  if (to.meta.guestOnly && authStore.isAuthenticated) {
    return next({ name: 'dashboard' });
  }

  next();
});

export default router;
