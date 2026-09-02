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
          path: 'terminal',
          name: 'terminal',
          component: () => import('../views/RemoteHostView.vue'),
        },
        {
          path: 'topology',
          name: 'topology',
          component: () => import('../views/TopologyView.vue'),
        },
        {
          path: 'backup',
          name: 'backup',
          component: () => import('../views/BackupView.vue'),
        },
        {
          path: 'tools',
          name: 'tools',
          component: () => import('../views/ToolsView.vue'),
        },
        {
          path: 'snmp',
          redirect: '/tools?tab=snmp',
        },
        {
          path: 'grok-debugger',
          redirect: '/tools?tab=grok',
        },
        {
          path: 'remote-config',
          name: 'remote-config',
          component: () => import('../views/RemoteConfigView.vue'),
        },
        {
          path: 'logs',
          name: 'logs',
          component: () => import('../views/LogsView.vue'),
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
