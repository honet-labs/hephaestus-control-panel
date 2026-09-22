import { defineStore } from 'pinia';
import { ref } from 'vue';
import axios from 'axios';

// Ensure withCredentials is true so HttpOnly session cookie is always sent (H-02)
axios.defaults.withCredentials = true;

export interface User {
  id: number;
  username: string;
  role: string;
  permissions?: Record<string, string>;
  forcePasswordChange: boolean;
}

export const useAuthStore = defineStore('auth', () => {
  const savedUserStr = localStorage.getItem('hephaestus_user');
  let initialUser: User | null = null;
  if (savedUserStr) {
    try {
      initialUser = JSON.parse(savedUserStr);
    } catch (_) {}
  }

  // Security Hardening: Remove sensitive tokens from localStorage to prevent XSS extraction (H-02)
  if (localStorage.getItem('hephaestus_token')) {
    localStorage.removeItem('hephaestus_token');
  }

  const user = ref<User | null>(initialUser);
  // Token is stored strictly in-memory during SPA lifecycle
  const token = ref<string | null>(null);
  const isAuthenticated = ref<boolean>(!!initialUser);

  const setAuth = (newUser: User, newToken?: string) => {
    user.value = newUser;
    token.value = newToken || null;
    isAuthenticated.value = true;
    localStorage.setItem('hephaestus_user', JSON.stringify(newUser));
    if (newToken) {
      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;
    }
  };

  const clearAuth = () => {
    user.value = null;
    token.value = null;
    isAuthenticated.value = false;
    localStorage.removeItem('hephaestus_user');
    localStorage.removeItem('hephaestus_token');
    delete axios.defaults.headers.common['Authorization'];
  };

  const fetchUser = async () => {
    if (!token.value) return null;
    try {
      const res = await axios.get('/api/v1/auth/me');
      if (res.data.success) {
        user.value = res.data.data;
        localStorage.setItem('hephaestus_user', JSON.stringify(res.data.data));
        return user.value;
      }
    } catch (err: any) {
      // ONLY clear auth if the server explicitly returned 401 Unauthorized or 403 Forbidden!
      // Do NOT wipe auth if the server is temporarily unreachable (network error, timeout, 502/503 during backend restart)
      if (err.response && (err.response.status === 401 || err.response.status === 403)) {
        clearAuth();
        return null;
      }
      // For network errors or temporary 5xx downtime, keep existing cached user state from localStorage
      console.warn('Backend temporarily unreachable, retaining cached auth session:', err?.message);
      return user.value;
    }
    return null;
  };

  const logout = async () => {
    try {
      await axios.post('/api/v1/auth/logout');
    } catch (_) {}
    clearAuth();
  };

  // RBAC permission check helper
  const can = (feature: string, action: 'read' | 'manage' = 'read'): boolean => {
    if (!user.value) return false;
    // Superadmin has full unrestricted access
    if (user.value.role?.toUpperCase() === 'ADMIN') return true;

    const perms = user.value.permissions || {};
    if (perms['*'] === 'manage') return true;

    const perm = perms[feature] || 'none';
    if (action === 'read') {
      return perm === 'read' || perm === 'manage';
    }
    if (action === 'manage') {
      return perm === 'manage';
    }
    return false;
  };

  return {
    user,
    token,
    isAuthenticated,
    setAuth,
    clearAuth,
    fetchUser,
    logout,
    can,
  };
});
