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
          path: 'snmp',
          name: 'snmp',
          component: () => import('../views/SnmpView.vue'),
        },
        {
          path: 'opensearch',
          name: 'opensearch',
          component: () => import('../views/OpenSearchView.vue'),
        },
        {
          path: 'grok-debugger',
          name: 'grok-debugger',
          component: () => import('../views/GrokDebuggerView.vue'),
        },
        {
          path: 'vps-control',
          name: 'vps-control',
          component: () => import('../views/VpsControlView.vue'),
        },
        {
          path: 'logs',
          name: 'logs',
          component: () => import('../views/LogsView.vue'),
        },
        {
          path: 'queue',
          name: 'queue',
          component: () => import('../views/QueueView.vue'),
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
